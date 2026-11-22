package main

import (
	"context"
	"maps"
	"os"
	"time"

	"github.com/nanihadesuka/rayo/internal/flow"
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
	return nil
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
