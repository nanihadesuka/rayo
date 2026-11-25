package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/mock"
	"github.com/nanihadesuka/rayo/internal/report"
)

// checkStarts refuses --resume, --from, --until and --mock-state where they can't do what they say.
// Before the flows are chosen, as --resume chooses those that failed.
func (o *options) checkStarts() error {
	switch {
	case !o.resume && o.from == "" && o.until == "":
		if o.mockState != nil {
			return errors.New("--mock-state sets the scenarios where a flow begins in the middle: add --resume or --from; see rayo help resume")
		}
		return nil
	case o.resume && o.from != "":
		return errors.New("--resume begins at the step that failed, --from at another: use one; see rayo help resume")
	case o.continuous:
		return errors.New("--continuous runs the flows whole again on each change, so --resume, --from and --until don't go with it")
	case o.runner.Capture != "":
		return errors.New("a capture is of whole flows, so --resume, --from and --until don't go with it")
	}
	if o.resume {
		o.rerunFailed = true
	}
	return nil
}

// settleStarts says where each flow begins and stops, from the options and the last run.
func (o *options) settleStarts(flows []*flow.Flow) error {
	if !o.resume && o.from == "" && o.until == "" {
		return nil
	}
	if !o.resume && len(flows) != 1 {
		return fmt.Errorf("--from and --until are of one flow, and %d would run: say which with --only", len(flows))
	}
	last, err := report.Last(report.LastFile(o.output))
	if err != nil {
		return err
	}
	o.starts = map[*flow.Flow]*flow.Start{}
	for _, f := range flows {
		start := &flow.Start{}
		var err error
		if o.from != "" {
			if start.From, err = f.Resolve(o.from); err != nil {
				return fmt.Errorf("--from: %w", err)
			}
		}
		if o.until != "" {
			if start.Until, err = f.Resolve(o.until); err != nil {
				return fmt.Errorf("--until: %w", err)
			}
		}
		if start.From != "" && start.Until != "" && f.Before(start.Until, start.From) {
			return fmt.Errorf("--until %s comes before --from %s, so nothing would run", start.Until, start.From)
		}
		failure := last[report.Identity(f)].Failure
		if o.resume {
			if failure == nil || failure.Error.File == "" {
				return fmt.Errorf("--resume: the last run of %s says no step failed, so there is nowhere to go on from: run it whole", f.Name)
			}
			start.From = fmt.Sprintf("%s:%d", failure.Error.File, failure.Error.Line)
			if !f.Find(start.From) {
				return fmt.Errorf("--resume: %s failed at %s, in onFlowStart or onFlowComplete, which a flow can't begin from: run it whole", f.Name, start.From)
			}
		}
		var trail []flow.StepStates
		if failure != nil {
			for _, t := range failure.Trail {
				trail = append(trail, t.Flow())
			}
		}
		if start.From != "" {
			start.States = flow.StatesAt(trail, start.From, f.Path)
			switch {
			case start.States != nil:
				start.Whence = ", as the last run had them as that step began"
			case len(f.Mock) > 0 && o.mockState == nil:
				start.Whence = "the last run has no scenarios for that step: they are as the mocks begin; --mock-state sets them"
			}
		}
		if err := checkStates(f, o.mockState); err != nil {
			return err
		}
		for _, set := range o.mockState {
			start.Whence = ", as the last run had them, and --mock-state"
			if start.States == nil {
				start.Whence = ", as --mock-state says"
			}
			i := slices.IndexFunc(start.States, func(s flow.Scenario) bool { return s.Name == set.Name })
			if i < 0 {
				start.States = append(start.States, set)
			} else {
				start.States = slices.Clone(start.States)
				start.States[i] = set
			}
		}
		o.starts[f] = start
	}
	return nil
}

// checkStates refuses a scenario or a state that the flow's own mocks don't have, before the device is
// used; those of mocks given by ${...} or added by steps are only known as it runs.
func checkStates(f *flow.Flow, states []flow.Scenario) error {
	if len(states) == 0 {
		return nil
	}
	m := mock.New()
	for _, n := range f.Mock {
		if strings.Contains(n.Value, "${") {
			return nil
		}
		path := n.Value
		if !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(f.Path), filepath.FromSlash(path))
		}
		if m.Load(path) != nil {
			return nil // rayo check says why
		}
	}
	if len(f.Mock) == 0 {
		return fmt.Errorf("--mock-state: %s has no mock: in its header", f.Name)
	}
	for _, s := range states {
		if err := m.SetState(s.Name, s.State); err != nil {
			return fmt.Errorf("--mock-state: %w", err)
		}
	}
	return nil
}

// nothingToResume says why --resume found no flow: none failed, or what failed isn't among these flows.
func (o *options) nothingToResume() error {
	file := report.LastFile(o.output)
	last, err := report.Last(file)
	if err != nil {
		return err
	}
	if len(last) == 0 {
		return fmt.Errorf("--resume: nothing failed last time (no %s): run the flows whole", filepath.ToSlash(file))
	}
	var failed []string
	for key, u := range last {
		failed = append(failed, fmt.Sprintf("%s (%s)", u.Name, key))
	}
	slices.Sort(failed)
	return fmt.Errorf("--resume: none of these flows failed last time; %s has %s. Give the suite or flow that ran, from the same folder, and the same --output", filepath.ToSlash(file), strings.Join(failed, ", "))
}
