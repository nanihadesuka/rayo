package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/adb"
	"github.com/nanihadesuka/rayo/internal/agent"
	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/live"
	"github.com/nanihadesuka/rayo/internal/report"
	"github.com/nanihadesuka/rayo/internal/schema"
	"github.com/nanihadesuka/rayo/internal/suite"
)

// session is a device ready to run flows.
type session struct {
	o        *options
	device   *adb.Device
	agent    *agent.Agent
	runner   *flow.Runner
	reporter flow.Reporter

	// Of the flow running now, with --on-fail hold.
	index, of int
	mark      *report.Mark // what was said of it while held
	abort     bool         // .abort: no more flows
}

func (s *session) start() error {
	a, err := agent.Start(s.device)
	if err != nil {
		return err
	}
	s.agent = a
	if !s.o.animations {
		if _, err := a.Do(context.Background(), "noAnimations", nil, 10*time.Second); err != nil {
			a.Close()
			return err
		}
	}
	s.runner = flow.NewRunner(s.device, a, s.reporter, s.o.runner)
	if s.o.onFail == onFailHold {
		s.runner.Hold = s.hold
	}
	return nil
}

// hold keeps a flow where it failed until rayo do says how to go on.
func (s *session) hold(ctx context.Context, held *flow.Session, f *flow.Flow, failed error) error {
	p := live.HoldParams{Output: s.o.output, Say: human(), Typed: printing == report.Human, Timeout: s.o.holdTimeout,
		Held: func() { stream(report.Held(f, s.index, s.of, failed, s.o.output)) }}
	end, err := live.Hold(ctx, p, held, f, failed)
	s.mark = end.Mark
	s.abort = s.abort || end.Abort
	return err
}

// restart replaces an agent that was lost.
func (s *session) restart() error {
	s.agent.Close()
	return s.start()
}

// choose is the flows asked for, in the order to run them.
func (o *options) choose(flows []*flow.Flow) ([]*flow.Flow, error) {
	var names, tags []string
	for _, f := range flows {
		names = append(names, f.Name)
		for _, t := range f.Tags {
			if !slices.Contains(tags, t) {
				tags = append(tags, t)
			}
		}
	}
	slices.Sort(tags)
	for _, name := range o.only {
		if !slices.Contains(names, name) {
			if near := schema.Nearest(name, names); near != "" {
				return nil, fmt.Errorf("no flow is called %s, did you mean %s? There are: %s", name, near, strings.Join(names, ", "))
			}
			return nil, fmt.Errorf("no flow is called %s; there are: %s", name, strings.Join(names, ", "))
		}
	}
	// A tag no flow has may be one of flows to come, unless it is all that is asked; one like a tag
	// in use is a typo.
	for i, tag := range slices.Concat(o.include, o.exclude) {
		if slices.Contains(tags, tag) {
			continue
		}
		inUse := "no flow has tags"
		if len(tags) > 0 {
			inUse = "the flows' tags are: " + strings.Join(tags, ", ")
		}
		if near := schema.Nearest(tag, tags); near != "" {
			return nil, fmt.Errorf("no flow has the tag %s, did you mean %s? %s", tag, near, inUse)
		}
		if i < len(o.include) && !slices.ContainsFunc(o.include, func(t string) bool { return slices.Contains(tags, t) }) {
			return nil, fmt.Errorf("no flow has the tag %s; %s", tag, inUse)
		}
	}
	var last map[string]report.Unpassed
	if o.rerunFailed {
		var err error
		if last, err = report.Last(report.LastFile(o.output)); err != nil {
			return nil, err
		}
	}
	has := func(f *flow.Flow, list []string) bool {
		return slices.ContainsFunc(f.Tags, func(tag string) bool { return slices.Contains(list, tag) })
	}
	var chosen []*flow.Flow
	for _, f := range flows {
		if len(o.only) > 0 && !slices.Contains(o.only, f.Name) {
			continue
		}
		if len(o.include) > 0 && !has(f, o.include) || has(f, o.exclude) {
			continue
		}
		if _, failed := last[report.Identity(f)]; o.rerunFailed && !failed {
			continue
		}
		chosen = append(chosen, f)
	}
	if len(chosen) == 0 && !o.rerunFailed {
		var asked []string
		if len(o.only) > 0 {
			asked = append(asked, "--only "+strings.Join(o.only, ","))
		}
		if len(o.include) > 0 {
			asked = append(asked, "--include-tags "+strings.Join(o.include, ","))
		}
		if len(o.exclude) > 0 {
			asked = append(asked, "--exclude-tags "+strings.Join(o.exclude, ","))
		}
		return nil, fmt.Errorf("none of the %d flows is left by %s", len(flows), strings.Join(asked, " with "))
	}
	if err := o.enough(len(chosen)); err != nil {
		return nil, err
	}
	if o.shards > 0 {
		o.ofAll = len(chosen)
		chosen = o.part(chosen)
	}
	if o.shuffle {
		// By path first, so that a seed always means the same order.
		slices.SortStableFunc(chosen, func(a, b *flow.Flow) int { return strings.Compare(report.Identity(a), report.Identity(b)) })
		rand.New(rand.NewPCG(o.seed, 0)).Shuffle(len(chosen), func(i, j int) { chosen[i], chosen[j] = chosen[j], chosen[i] })
	}
	return chosen, nil
}

// enough is an error if fewer flows were chosen than --min-flows, or the suite's minFlows, says.
func (o *options) enough(chosen int) error {
	least := o.minFlows
	if least == 0 && o.suite != nil {
		least = o.suite.MinFlows
	}
	if chosen >= least || o.rerunFailed || o.resume {
		return nil
	}
	what := "--min-flows"
	if o.minFlows == 0 {
		what = "the suite's minFlows"
	}
	return fmt.Errorf("%d flows chosen, fewer than the %d of %s: a tag, a name or --since may be wrong", chosen, least, what)
}

// noneChosen says why no flow runs, and writes the results all the same, with none, so that CI doesn't read those of an earlier run.
func (o *options) noneChosen(title, why string) error {
	fmt.Fprintln(human(), why)
	sum := &report.Summary{Title: title, Began: time.Now(), Stopped: why, Shard: o.shard, Shards: o.shards, Template: o.resultsTemplate}
	var err error
	for _, file := range o.results {
		err = errors.Join(err, sum.Write(file))
	}
	return err
}

// part is those of --shard: each flow's place among all by path settles its part, so that every
// machine agrees whatever the order or the other options.
func (o *options) part(flows []*flow.Flow) []*flow.Flow {
	byPath := slices.Clone(flows)
	slices.SortStableFunc(byPath, func(a, b *flow.Flow) int { return strings.Compare(report.Identity(a), report.Identity(b)) })
	mine := map[*flow.Flow]bool{}
	for i, f := range byPath {
		if i%o.shards == o.shard-1 {
			mine[f] = true
		}
	}
	return slices.DeleteFunc(flows, func(f *flow.Flow) bool { return !mine[f] })
}

// streak is the failures in a row that aren't the tests' own.
type streak struct {
	device int    // tries
	app    int    // flows, as one alone may crash it by itself
	crash  string // the app's
	last   *flow.Flow
}

// How many in a row are enough to think that the rest would fail too.
const (
	deviceRow = 2
	appRow    = 3
)

// after takes a try into account, and returns why to stop, if so.
func (r *streak) after(o *report.Outcome, keepGoing bool) string {
	f := o.Failure
	if o.Err == nil || f == nil || f.Blame == flow.Test {
		*r = streak{}
		return ""
	}
	if f.Blame == flow.Dev {
		r.device++
		r.app, r.crash = 0, ""
		switch {
		case f.Unusable:
			return "device: " + f.Because
		case r.device >= deviceRow && !keepGoing:
			return fmt.Sprintf("device: %d failures in a row, the last: %s", r.device, f.Because)
		}
		return ""
	}
	if r.crash != f.Because {
		r.app = 0
	}
	if r.app == 0 || r.last != o.Flow {
		r.app++
	}
	r.device, r.crash, r.last = 0, f.Because, o.Flow
	if r.app >= appRow && !keepGoing {
		return fmt.Sprintf("app: %d flows in a row, %s", r.app, f.Because)
	}
	return ""
}

// retriesOf says how many times a flow may be tried again. A capture calls the real backend, so only once.
func (o *options) retriesOf(f *flow.Flow) int {
	switch {
	case o.runner.Capture != "", o.starts != nil, o.onFail == onFailHold: // a retry wouldn't begin where the app is
		return 0
	case o.retries >= 0:
		return o.retries
	case o.suite != nil:
		return o.suite.RetriesOf(f)
	case o.continuous: // someone is looking, and changing it
		return 0
	}
	return suite.DefaultRetries
}

// runAll runs flows and says how each went, also those that didn't run.
func (s *session) runAll(ctx context.Context, flows []*flow.Flow, title string) error {
	o := s.o
	sum := &report.Summary{Title: title, Device: s.device.Serial, Began: time.Now(), Run: runID(), FlakyFails: o.flakyFails,
		Detail: o.detail, Show: o.show, Slowest: o.slowest, Template: o.resultsTemplate, Shard: o.shard, Shards: o.shards}
	say := human()
	if o.shards > 0 {
		fmt.Fprintf(say, "%s: %d of %d flows\n", sum.Part(), len(flows), o.ofAll)
	}
	if o.shuffle {
		sum.Order = fmt.Sprintf("--shuffle=%d", o.seed)
		fmt.Fprintln(say, "order:", sum.Order)
	}
	if o.runner.Timings && o.runner.Scale > 0 && o.runner.Scale != 1 {
		fmt.Fprintf(say, "default waits ×%v\n", o.runner.Scale)
	}
	folders := map[string]int{}
	var row streak
	stop := ""    // why the rest won't run
	lost := false // the device, for that
	verbose := o.steps(flows)
	s.reporter.(*console).verbose = verbose
	s.reporter.(*console).detail = o.detail
	s.reporter.(*console).timings = o.runner.Timings
	for i, f := range flows {
		out := report.Outcome{Flow: f, Retries: o.retriesOf(f), Began: time.Now(), Partial: o.starts[f] != nil}
		s.runner.Start = o.starts[f]
		if stop != "" {
			out.Status, out.Why, out.Began = report.NotRun, stop, time.Time{}
		} else {
			name := flow.FolderName(f.Name)
			if folders[strings.ToLower(name)]++; folders[strings.ToLower(name)] > 1 { // two flows with one name
				name = fmt.Sprintf("%s_%d", name, folders[strings.ToLower(name)])
			}
			s.index, s.of, s.mark = i+1, len(flows), nil
			s.reporter.(*console).begin(i+1, len(flows), f.Name)
			stream(report.Started(f, i+1, len(flows)))
			for try := 0; ; try++ {
				result := s.runner.Run(ctx, f, flow.Attempt{Out: o.output, Flow: name, Try: try})
				if result.Err == nil && o.keep == suite.KeepFailed { // what is kept is to see why it failed
					if err := flow.Remove(o.output, result.Files); err != nil {
						fmt.Printf("  couldn't remove the files of the attempt: %v\n", err)
					}
					result.Recordings = nil
				}
				out.Judge(ctx, result)
				out.Mark = s.mark
				for _, e := range report.Tried(&out, i+1, len(flows)) {
					stream(e)
				}
				if ctx.Err() == nil {
					stop = row.after(&out, o.keepGoing)
					lost = strings.HasPrefix(stop, "device:")
					if stop == "" && s.agent.Dead() != nil {
						if err := s.restart(); err != nil {
							stop, lost = fmt.Sprintf("the device was lost: %v", err), true
						}
					}
					sum.Stopped = stop
				}
				if s.abort && stop == "" {
					stop = "aborted while " + f.Name + " was held"
					sum.Stopped = stop
				}
				if out.Status != report.Failed || try >= out.Retries || stop != "" {
					break
				}
				if verbose {
					fmt.Fprintf(say, "  failed, retry %d of %d: %v\n", try+1, out.Retries, out.Err)
				}
			}
			switch {
			case ctx.Err() != nil:
				stop = "interrupted"
			case stop == "" && out.Status == report.Failed && o.failFast:
				stop = "--fail-fast, after " + f.Name
			}
		}
		sum.Outcomes = append(sum.Outcomes, out)
		s.reporter.(*console).end(i+1, len(flows), &out)
		stream(report.Finished(&out, i+1, len(flows)))
	}
	sum.Took = time.Since(sum.Began)
	fmt.Fprintln(say)
	fmt.Fprint(say, sum.Text(true))
	s.reporter.(*console).slowestFlows(sum.Outcomes)
	if lost {
		fmt.Fprintln(say, "next: rayo devices says if the device is there and ready; restart the emulator or reconnect it, then rayo test --rerun-failed")
	}
	failed := sum.Err()
	if failed != nil && lost {
		failed = ofDevice(failed)
	}
	var flaky *report.FlakyError
	if errors.As(failed, &flaky) {
		failed = exiting(report.ExitFlaky, failed)
	}
	failed = exiting(report.ExitFailed, failed)
	sum.Exit = exitOf(failed)
	if ctx.Err() != nil {
		sum.Exit = report.ExitInterrupted
	}
	err := errors.Join(sum.SaveLast(report.LastFile(o.output)), sum.SaveHistory(report.HistoryFile(o.output)))
	for _, file := range o.results {
		err = errors.Join(err, sum.Write(file))
	}
	if !sum.AllPassed() {
		fmt.Fprintf(say, "results: %s  (rayo explain)\n", filepath.ToSlash(o.output))
	}
	if printing != report.Human && failed != nil {
		failed = &exitError{exitOf(failed), errShown} // the results say it
	}
	switch printing {
	case report.JSON:
		data, e := sum.JSON()
		if e == nil {
			_, e = os.Stdout.Write(data)
		}
		err = errors.Join(err, e)
	case report.NDJSON:
		err = errors.Join(err, emit(sum.Outcome(exitOf(failed))))
	}
	return errors.Join(err, failed)
}

// stream prints an event, with --format ndjson.
func stream(e report.Event) {
	if printing == report.NDJSON {
		_ = emit(e)
	}
}

// runID tells a run apart from the others: when it began, then some of the process, so that ids sort by time.
func runID() string {
	now := time.Now().UTC()
	return fmt.Sprintf("%s-%04x", now.Format("20060102T150405"), uint16(os.Getpid()^now.Nanosecond()))
}
