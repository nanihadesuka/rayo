package main

import (
	"context"
	"encoding/json"
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
			return nil, fmt.Errorf("no flow is called %s", name)
		}
	}
	var last map[string]string
	if o.rerunFailed {
		var err error
		if last, err = o.lastRun(); err != nil {
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
		if _, failed := last[identity(f)]; o.rerunFailed && !failed {
			continue
		}
		chosen = append(chosen, f)
	}
	if len(chosen) == 0 && !o.rerunFailed {
		return nil, errors.New("no flows have those tags")
	}
	if o.shuffle {
		// By path first, so that a seed always means the same order.
		slices.SortStableFunc(chosen, func(a, b *flow.Flow) int { return strings.Compare(identity(a), identity(b)) })
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
func (r *streak) after(o *outcome, keepGoing bool) string {
	f := o.Failure
	if o.Err == nil || f == nil || f.Cause == flow.Test {
		*r = streak{}
		return ""
	}
	if f.Cause == flow.Dev {
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
	if r.app == 0 || r.last != o.flow {
		r.app++
	}
	r.device, r.crash, r.last = 0, f.Because, o.flow
	if r.app >= appRow && !keepGoing {
		return fmt.Sprintf("app: %d flows in a row, %s", r.app, f.Because)
	}
	return ""
}

// retriesOf says how many times a flow may be tried again. A capture calls the real backend, so only once.
func (o *options) retriesOf(f *flow.Flow) int {
	switch {
	case o.runner.Capture != "":
		return 0
	case o.retries >= 0:
		return o.retries
	case o.suite != nil:
		return o.suite.retriesOf(f)
	case o.continuous: // someone is looking, and changing it
		return 0
	}
	return defaultRetries
}

const defaultRetries = 1

// steps says whether to show every step, and not only how each flow went.
func (o *options) steps(flows []*flow.Flow) bool { return o.verbose || len(flows) == 1 }

// runAll runs flows and says how each went, also those that didn't run.
func (s *session) runAll(ctx context.Context, flows []*flow.Flow, title string) error {
	o := s.o
	sum := &summary{title: title, device: s.device.Serial, began: time.Now()}
	if o.shuffle {
		sum.order = fmt.Sprintf("--shuffle=%d", o.seed)
		fmt.Println("order:", sum.order)
	}
	folders := map[string]int{}
	var row streak
	stop := "" // why the rest won't run
	verbose := o.steps(flows)
	s.reporter.(*console).verbose = verbose
	for i, f := range flows {
		out := outcome{flow: f, retries: o.retriesOf(f)}
		if stop != "" {
			out.status, out.why = notRun, stop
		} else {
			name := flow.FolderName(f.Name)
			if folders[strings.ToLower(name)]++; folders[strings.ToLower(name)] > 1 { // two flows with one name
				name = fmt.Sprintf("%s_%d", name, folders[strings.ToLower(name)])
			}
			s.reporter.(*console).begin(i+1, len(flows), f.Name)
			for try := 0; ; try++ {
				folder := filepath.Join(o.runner.Out, name)
				if try > 0 {
					folder += fmt.Sprintf("_retry%d", try)
				}
				out.Result = s.runner.Run(ctx, f, folder)
				out.tries = append(out.tries, out.Result)
				out.status = out.judge(ctx)
				if ctx.Err() == nil {
					stop = row.after(&out, o.keepGoing)
					if stop == "" && s.agent.Dead() != nil {
						if err := s.restart(); err != nil {
							stop = fmt.Sprintf("the device was lost: %v", err)
						}
					}
					sum.stopped = stop
				}
				if out.status != failed || try >= out.retries || stop != "" {
					break
				}
				if verbose {
					fmt.Printf("  failed, retry %d of %d: %v\n", try+1, out.retries, out.Err)
				}
			}
			switch {
			case ctx.Err() != nil:
				stop = "interrupted"
			case stop == "" && out.status == failed && o.failFast:
				stop = "--fail-fast, after " + f.Name
			}
		}
		sum.outcomes = append(sum.outcomes, out)
		s.reporter.(*console).end(i+1, len(flows), &out)
	}
	sum.took = time.Since(sum.began)
	fmt.Println()
	fmt.Print(sum.text(o.failuresOnly, true))
	return errors.Join(sum.save(o), sum.err())
}

// --- The last run ---

// identity tells flows apart in different runs.
func identity(f *flow.Flow) string {
	path := f.Path
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return filepath.ToSlash(path)
}

func (o *options) lastFile() string { return filepath.Join(o.runner.Out, "failed.json") }

// lastRun is the flows that haven't passed since they last failed, and how they went.
func (o *options) lastRun() (map[string]string, error) {
	last := map[string]string{}
	data, err := os.ReadFile(o.lastFile())
	if errors.Is(err, os.ErrNotExist) {
		return last, nil
	}
	if err == nil {
		err = json.Unmarshal(data, &last)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.ToSlash(o.lastFile()), err)
	}
	return last, nil
}

func (s *summary) saveLast(o *options) error {
	last, err := o.lastRun()
	if err != nil {
		last = map[string]string{} // unreadable, so written again
	}
	before := len(last)
	for _, out := range s.outcomes {
		if out.status == passed {
			delete(last, identity(out.flow))
		} else {
			last[identity(out.flow)] = out.status.String()
		}
	}
	if len(last) == 0 {
		if before > 0 || err != nil {
			return os.Remove(o.lastFile())
		}
		return nil
	}
	data, err := json.MarshalIndent(last, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(o.lastFile(), append(data, '\n'))
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
