package main

import (
	"context"
	"maps"
	"os"
	"time"

	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/suite"
)

// watcher remembers the files of the flows, also of those that no longer load.
type watcher map[string]bool

// await returns when one of the files changes.
func (w watcher) await(ctx context.Context, flows []*flow.Flow) error {
	for _, f := range flows {
		for _, file := range f.Files() {
			w[file] = true
		}
	}
	before := w.stamp()
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for maps.Equal(before, w.stamp()) {
		select {
		case <-tick.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	// until nothing more changes, as an editor or a build may write several times
	for now := w.stamp(); ; {
		select {
		case <-time.After(settle):
		case <-ctx.Done():
			return ctx.Err()
		}
		later := w.stamp()
		if maps.Equal(now, later) {
			return nil
		}
		now = later
	}
}

// settle is how long the files must stay the same before a run begins.
const settle = 300 * time.Millisecond

// app adds a suite's APK, so that a new build is installed and run.
func (w watcher) app(s *suite.Suite) {
	if s != nil && s.App != nil && s.App.APK != "" {
		w[s.Relative(s.App.APK)] = true
	}
}

// stamp is when each file that is there last changed.
func (w watcher) stamp() map[string]time.Time {
	stamps := map[string]time.Time{}
	for file := range w {
		if info, err := os.Stat(file); err == nil {
			stamps[file] = info.ModTime()
		}
	}
	return stamps
}
