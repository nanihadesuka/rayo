package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/report"
)

func failedAt(line int, msg string) *report.Outcome {
	o := &report.Outcome{Flow: &flow.Flow{Name: "f"}}
	step := &flow.Step{Path: filepath.Join("e2e", "login.yaml"), Line: line}
	o.Judge(context.Background(), flow.Result{Err: &flow.StepError{Step: step, Err: errors.New(msg)}, Failure: &flow.Failure{}})
	return o
}

func TestSetupStreak(t *testing.T) {
	var s setupStreak
	if s.after(failedAt(3, "x")) != "" || s.after(failedAt(3, "x")) != "" {
		t.Fatal("stopped too soon")
	}
	why := s.after(failedAt(3, "x"))
	if !strings.HasPrefix(why, "setup: the first 3 flows") || !strings.Contains(why, "login.yaml:3") {
		t.Fatalf("%q", why)
	}
	if s.after(failedAt(3, "x")) != "" {
		t.Error("said twice")
	}

	s = setupStreak{}
	s.after(failedAt(3, "x"))
	s.after(failedAt(4, "x"))
	s.after(failedAt(4, "x"))
	if s.after(failedAt(4, "x")) != "" {
		t.Error("failures that differ, then alike, aren't of the setup")
	}

	s = setupStreak{}
	passed := &report.Outcome{Flow: &flow.Flow{Name: "p"}}
	passed.Judge(context.Background(), flow.Result{})
	s.after(passed)
	for range 3 {
		if s.after(failedAt(3, "x")) != "" {
			t.Fatal("after a pass, the setup works")
		}
	}
}

func TestPartial(t *testing.T) {
	for _, c := range []struct {
		o    options
		want bool
	}{
		{options{}, false},
		{options{only: []string{"a"}}, true},
		{options{include: []string{"t"}}, true},
		{options{exclude: []string{"t"}}, true},
		{options{rerunFailed: true}, true},
		{options{resume: true}, true},
		{options{shards: 2}, true},
		{options{since: "HEAD"}, true},
		{options{onFail: onFailHold}, true},
		{options{starts: map[*flow.Flow]*flow.Start{}}, true},
	} {
		if got := c.o.partial(); got != c.want {
			t.Errorf("%+v: %v", c.o, got)
		}
	}
}
