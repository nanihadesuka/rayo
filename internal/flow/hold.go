package flow

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Holding a flow where it failed, so that it can be fixed in its files and go on from there: of
// rayo test --on-fail hold.

// Hold is given a flow that failed, as it failed, before onFlowComplete runs and with the app as it
// is. It returns nil once the flow's steps have passed from some step to their end, so that the flow goes
// on with onFlowComplete, and a pass that is only part of it; else the error the flow fails with.
type Hold func(ctx context.Context, s *Session, f *Flow, err error) error

// ranMost is how many steps that passed a flow keeps, for .rewind.
const ranMost = 500

// Failure is what is known of the last failure, as it failed; nil if none.
func (s *Session) Failure() *Failure { return s.r.failure }

// Warn adds a warning to the flow's, as of its run.
func (s *Session) Warn(text string) { s.r.warn(0, text) }

// Telling has what the flow says go to a reporter too, until the function it returns is called.
func (s *Session) Telling(also Reporter) (restore func()) {
	before := s.r.reporter
	s.r.reporter = both{before, also}
	return func() { s.r.reporter = before }
}

type both [2]Reporter

func (b both) Step(s StepReport) { b[0].Step(s); b[1].Step(s) }
func (b both) Note(depth int, text string) {
	b[0].Note(depth, text)
	b[1].Note(depth, text)
}

// Ran is the places of the steps that passed, the last ones, oldest first.
func (s *Session) Ran() []string { return slices.Clone(s.r.ran) }

// From runs a flow's steps from one on, as --from does, to their end: not onFlowComplete, which
// runs once the hold ends. The flow may have been read again, with changes.
func (s *Session) From(ctx context.Context, f *Flow, from string) Done {
	r := s.r
	kept := r.warnings // of the flow so far
	start := r.Start
	r.flow, r.from, r.passed, r.Start = f, from, 0, &Start{From: from}
	defer func() { r.Start, r.from = start, "" }()
	r.began, r.failure, r.warnings = time.Now(), nil, nil
	before := 0
	if r.backend.mock != nil {
		before = len(r.backend.mock.Calls())
	}
	var d Done
	d.Err = r.steps(ctx, f.Steps, 0)
	if d.Err == nil && r.from != "" {
		d.Err = fmt.Errorf("never reached the step %s to begin from", r.from)
	}
	if d.Err == nil && r.backend.mock != nil {
		d.Err = r.backend.check()
	}
	if d.Err != nil && ctx.Err() == nil {
		r.evidence(ctx)
		r.judge(d.Err)
		d.Failure = r.failure
	}
	if r.backend.mock != nil {
		d.Calls = r.backend.mock.Calls()[before:]
		for i := range d.Calls {
			d.Calls[i].Body = ""
		}
	}
	d.Warnings = r.warnings
	r.warnings = append(kept, r.warnings...)
	return d
}

// Probe runs one step again and again, as it is now, and says how each try went: whether it fails at
// times, or always.
func (s *Session) Probe(ctx context.Context, step *Step, times int) []Try {
	var tries []Try
	for i := 0; i < times && ctx.Err() == nil && s.Dead() == nil; i++ {
		began := time.Now()
		err := s.r.step(ctx, step, 0)
		tries = append(tries, Try{Err: err, Took: time.Since(began)})
	}
	return tries
}

// Try is one of a Probe.
type Try struct {
	Err  error
	Took time.Duration
}

// At is the step at a place, file:line, among a flow's own and those of the flows it runs; nil if none.
func (f *Flow) At(place string) *Step {
	var found *Step
	seen := map[string]bool{}
	var walk func(steps []*Step)
	walk = func(steps []*Step) {
		for _, s := range steps {
			if found != nil {
				return
			}
			if s.Is(place, f.Path) {
				found = s
				return
			}
			walk(s.Sub)
			if file := keyFile.of(s.Value); file != "" && !strings.Contains(file, "${") {
				path := relative(s.Path, file)
				if !seen[path] {
					seen[path] = true
					if included, err := Load(path); err == nil {
						walk(slices.Concat(included.OnStart, included.Steps, included.OnComplete))
					}
				}
			}
		}
	}
	walk(slices.Concat(f.Steps, f.OnStart, f.OnComplete))
	return found
}

// Moved is where a step is now, after its file changed: the same place if the step there is still the
// same; else that of the step of the same file with the same text that is nearest its old line; else, if
// its file has as many steps as before, the one in its place among them; else the same place.
func (f *Flow) Moved(place, text string, ordinal int) string {
	if s := f.At(place); s != nil && s.describe() == text {
		return place
	}
	file, line := place, 0
	if i := strings.LastIndex(place, ":"); i >= 0 {
		file = place[:i]
		fmt.Sscan(place[i+1:], &line)
	}
	mine := f.stepsOf(file)
	best, far := "", 1<<30
	for _, s := range mine {
		if d := abs(s.Line - line); s.describe() == text && d < far {
			best, far = s.Place(), d
		}
	}
	switch {
	case best != "":
		return best
	case ordinal >= 0 && ordinal < len(mine):
		return mine[ordinal].Place()
	}
	return place
}

// Ordinal is the place of a step among those of its file, as the flow runs them; -1 if it isn't there.
func (f *Flow) Ordinal(place string) int {
	file := place
	if i := strings.LastIndex(place, ":"); i >= 0 {
		file = place[:i]
	}
	return slices.IndexFunc(f.stepsOf(file), func(s *Step) bool { return s.Is(place, f.Path) })
}

// stepsOf is the steps written in one file, of the flow and those it runs, in order.
func (f *Flow) stepsOf(file string) []*Step {
	var list []*Step
	seen := map[string]bool{}
	var walk func(steps []*Step)
	walk = func(steps []*Step) {
		for _, s := range steps {
			if sameFile(s.Path, file, f.Path) {
				list = append(list, s)
			}
			walk(s.Sub)
			if path := keyFile.of(s.Value); path != "" && !strings.Contains(path, "${") {
				path = relative(s.Path, path)
				if !seen[path] {
					seen[path] = true
					if included, err := Load(path); err == nil {
						walk(slices.Concat(included.OnStart, included.Steps, included.OnComplete))
					}
				}
			}
		}
	}
	walk(f.Steps)
	return list
}

// sameFile says whether a step's file is the one of a place.
func sameFile(path, file, flowPath string) bool {
	return (&Step{Path: path, Line: 1}).Is(file+":1", flowPath)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Checks counts what a flow checks, in its files and those it runs: the steps that assert, and those
// that may fail without failing it. Fewer asserts, or more that are optional, after a fix, is a test
// made to pass, not a flow fixed.
func (f *Flow) Checks() (asserts, optional int) {
	seen := map[string]bool{}
	var walk func(steps []*Step)
	walk = func(steps []*Step) {
		for _, s := range steps {
			if strings.HasPrefix(s.Kind, "assert") || s.Kind == "extendedWaitUntil" {
				asserts++
			}
			if n := get(s.Value, keyOptional); n != nil && n.Value != "false" {
				optional++
			}
			walk(s.Sub)
			if path := keyFile.of(s.Value); path != "" && !strings.Contains(path, "${") {
				path = relative(s.Path, path)
				if !seen[path] {
					seen[path] = true
					if included, err := Load(path); err == nil {
						walk(slices.Concat(included.OnStart, included.Steps, included.OnComplete))
					}
				}
			}
		}
	}
	walk(slices.Concat(f.OnStart, f.Steps, f.OnComplete))
	return asserts, optional
}

// FailedStep is the step an error is of, the innermost; nil if it isn't of one.
func FailedStep(err error) *Step {
	var at *StepError
	if !errors.As(err, &at) {
		return nil
	}
	return at.Step
}
