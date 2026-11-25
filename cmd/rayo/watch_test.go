package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nanihadesuka/rayo/internal/suite"
)

// A file written several times in a row starts one run, once it stays the same.
func TestWatchWaitsUntilWritingEnds(t *testing.T) {
	file := filepath.Join(t.TempDir(), "flow.yaml")
	write := func(n int) {
		if err := os.WriteFile(file, []byte{byte(n)}, 0o644); err != nil {
			t.Error(err)
		}
		stamp := time.Now().Add(time.Duration(n) * time.Second) // coarse clocks still see a change
		_ = os.Chtimes(file, stamp, stamp)
	}
	write(0)
	w := watcher{file: true}
	lastWrite := make(chan time.Time, 1)
	go func() {
		time.Sleep(250 * time.Millisecond)
		for i := 1; i <= 4; i++ { // closer together than settle
			write(i)
			time.Sleep(settle / 3)
		}
		lastWrite <- time.Now()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := w.await(ctx, nil); err != nil {
		t.Fatal(err)
	}
	returned := time.Now()
	select {
	case last := <-lastWrite:
		if returned.Sub(last) < settle-50*time.Millisecond {
			t.Errorf("returned %v after the last write, before it settled", returned.Sub(last))
		}
	default:
		t.Error("returned while the file was still being written")
	}
}

func TestWatchStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (watcher{}).await(ctx, nil); err == nil {
		t.Error("no error once cancelled")
	}
}

func TestWatchWithoutAnApp(t *testing.T) {
	dir := t.TempDir()
	w := watcher{}
	w.app(nil)
	w.app(&suite.Suite{Path: filepath.Join(dir, "all.suite.yaml")})
	if len(w) != 0 {
		t.Errorf("watches %v without an app", w)
	}
}
