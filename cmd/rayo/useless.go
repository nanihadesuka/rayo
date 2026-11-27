package main

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/suite"
)

// Options that are right on their own but would change nothing are errors: a run that seems to have
// done what was asked, and didn't, is worse than one that doesn't begin.

// checkKept refuses files asked for that keep none would not write. Before keep none clears the kinds.
func (o *options) checkKept() error {
	if o.keep != suite.KeepNone {
		return nil
	}
	if len(o.runner.Artifacts) > 0 {
		return errors.New("--keep none writes no files, so --artifacts (or the suite's output: { artifacts }) does nothing; see rayo help test")
	}
	if o.runner.Recording.On {
		return errors.New("--keep none writes no files, so --video (or the suite's output: { video: true }) records for nothing; see rayo help test")
	}
	if o.runner.NameTemplate != "" {
		return errors.New("--keep none writes no files, so --name-template (or the suite's output: { nameTemplate }) does nothing; see rayo help test")
	}
	return nil
}

// checkUseful refuses options that the flows to run would not use.
func (o *options) checkUseful(flows []*flow.Flow) error {
	if slices.Contains(o.runner.Artifacts, flow.Video) && !o.records(flows) {
		return errors.New("artifacts has video, but no flow is recorded: set record: true in the suite, record: true in a flow, or use startRecording; see rayo help suite")
	}
	if o.runner.Recording != nil && o.runner.Recording.On {
		for _, f := range flows {
			if (f.Recording == nil || f.Recording.On) && f.RecordsElsewhere() {
				return fmt.Errorf("--video (or the suite's output: { video: true }) records each flow whole, and %s starts a recording of its own, startRecording with a path: only one can go on at a time, so every try would fail. Leave out --video, give the flow record: false in its header, or use startRecording without a path", f.Name)
			}
		}
	}
	if o.retries == 0 && o.flakyFails {
		return errors.New("--flaky-fails-build with --retries 0: without a retry no flow can be flaky")
	}
	if o.runner.Hosts != nil && o.runner.Network != flow.Proxy && !slices.ContainsFunc(flows, mayProxy) {
		return errors.New("--hosts is for the proxy, and no flow uses it: add --network proxy, or network: proxy to the flows; see rayo help connect-mocks")
	}
	return nil
}

// records says whether a video can be made: of every flow, as the suite says, or of one of them.
func (o *options) records(flows []*flow.Flow) bool {
	if r := o.runner.Recording; r != nil && r.On {
		return true
	}
	return slices.ContainsFunc(flows, (*flow.Flow).Records)
}

// mayProxy says whether a flow goes through the proxy, or may: its network is only known when it runs.
func mayProxy(f *flow.Flow) bool {
	return f.Network == flow.Proxy || strings.Contains(f.Network, "${")
}
