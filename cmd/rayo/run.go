package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/adb"
	"github.com/nanihadesuka/rayo/internal/agent"
	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/report"
	"github.com/nanihadesuka/rayo/internal/suite"
)

// session is a device ready to run flows.
type session struct {
	o        *options
	device   *adb.Device
	agent    *agent.Agent
	runner   *flow.Runner
	reporter flow.Reporter
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
	return nil
}

// restart replaces an agent that was lost.
func (s *session) restart() error {
	s.agent.Close()
	return s.start()
}

// choose is the flows asked for, in the order to run them.
func (o *options) choose(flows []*flow.Flow) ([]*flow.Flow, error) {
	for _, name := range o.only {
		if !slices.ContainsFunc(flows, func(f *flow.Flow) bool { return f.Name == name }) {
			var names []string
			for _, f := range flows {
				names = append(names, f.Name)
			}
			return nil, fmt.Errorf("no flow is called %s; there are: %s", name, strings.Join(names, ", "))
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
		return nil, errors.New("no flows have those tags")
	}
	if o.shuffle {
		// By path first, so that a seed always means the same order.
		slices.SortStableFunc(chosen, func(a, b *flow.Flow) int { return strings.Compare(report.Identity(a), report.Identity(b)) })
		rand.New(rand.NewPCG(o.seed, 0)).Shuffle(len(chosen), func(i, j int) { chosen[i], chosen[j] = chosen[j], chosen[i] })
	}
	return chosen, nil
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
	case o.runner.Capture != "", o.starts != nil: // a retry wouldn't begin where the app is
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
	sum := &report.Summary{Title: title, Device: s.device.Serial, Began: time.Now(), FlakyFails: o.flakyFails,
		Detail: o.detail, Show: o.show, Slowest: o.slowest, Template: o.resultsTemplate}
	say := human()
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
	err := sum.SaveLast(report.LastFile(o.output))
	for _, file := range o.results {
		err = errors.Join(err, sum.Write(file))
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
