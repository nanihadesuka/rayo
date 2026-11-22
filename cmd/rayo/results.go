package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/flow"
)

// status is how a flow went. Its name is what is printed.
type status string

const (
	passed      status = "PASS"
	flaky       status = "FLAKY" // passed, but not the first time
	failed      status = "FAIL"
	interrupted status = "INTERRUPTED"
	notRun      status = "NOT_RUN"
)

func (s status) String() string { return string(s) }

func (s status) good() bool { return s == passed || s == flaky }

// outcome is a flow's, after all its tries.
type outcome struct {
	flow.Result // of the last try
	flow        *flow.Flow
	status      status
	tries       []flow.Result
	why         string // it didn't run
}

func (o *outcome) judge(ctx context.Context) status {
	switch {
	case o.Err == nil && len(o.tries) > 1:
		return flaky
	case o.Err == nil:
		return passed
	case ctx.Err() != nil:
		return interrupted
	}
	return failed
}

func (o *outcome) took() (total time.Duration) {
	for _, t := range o.tries {
		total += t.Elapsed
	}
	return total
}

// line is one line: the status is its first word.
func (o *outcome) line() string {
	if o.status == notRun {
		return fmt.Sprintf("%s %s  (%s)", o.status, o.flow.Name, o.why)
	}
	line := fmt.Sprintf("%s %s  %.1f s", o.status, o.flow.Name, o.took().Seconds())
	if len(o.tries) > 1 {
		line += fmt.Sprintf("  (%d tries)", len(o.tries))
	}
	return line
}

// details is everything that helps to know why it failed: "  key: value" lines, and more indented
// ones for what doesn't fit in a line.
func (o *outcome) details(full bool) string {
	var b strings.Builder
	add := func(key, value string) {
		if value = strings.TrimSpace(value); value != "" {
			fmt.Fprintf(&b, "  %s: %s\n", key, strings.ReplaceAll(value, "\n", "\n    "))
		}
	}
	list := func(key string, values []string) {
		if len(values) > 0 {
			fmt.Fprintf(&b, "  %s:\n", key)
		}
		for _, v := range values {
			fmt.Fprintf(&b, "    %s\n", v)
		}
	}
	for i, t := range o.tries {
		if t.Err == nil || o.status == interrupted {
			continue
		}
		if len(o.tries) > 1 {
			add("try", fmt.Sprint(i+1))
		}
		var step *flow.StepError
		if errors.As(t.Err, &step) {
			add("why", step.Err.Error())
			add("step", step.Step.Text())
			add("at", step.Step.Place())
			for _, from := range step.From {
				add("from", from.Place()+" "+from.Text())
			}
		} else {
			add("why", t.Err.Error())
		}
		f := t.Failure
		if f == nil {
			continue
		}
		add("crash", f.Crash)
		list("before", f.Before)
		if full {
			var calls []string
			for _, c := range f.Calls {
				calls = append(calls, c.String())
			}
			list("calls", calls)
			const most = 40
			screen := strings.Split(strings.TrimRight(f.Screen, "\r\n"), "\n")
			if f.Screen == "" {
				screen = nil
			} else if len(screen) > most {
				screen = append(screen[:most:most], fmt.Sprintf("... %d more in screen.txt", len(screen)-most))
			}
			list("screen", screen)
		}
		add("saved", f.Saved)
	}
	if !o.status.good() {
		b.WriteString(o.videos())
	}
	for _, w := range o.Warnings {
		add("warning", w)
	}
	return b.String()
}

// videos are those of every try.
func (o *outcome) videos() string {
	text := ""
	for _, t := range o.tries {
		for _, file := range t.Recordings {
			text += "  recording: " + file + "\n"
		}
	}
	return text
}

// summary is how a run went.
type summary struct {
	title, device, order string
	began                time.Time
	took                 time.Duration
	outcomes             []outcome
}

func (s *summary) count(is status) int {
	n := 0
	for _, o := range s.outcomes {
		if o.status == is {
			n++
		}
	}
	return n
}

// text is the results. Brief is for the screen, where the details have been shown already.
func (s *summary) text(failuresOnly, brief bool) string {
	var b strings.Builder
	if !brief {
		fmt.Fprintf(&b, "run: %s\ndevice: %s\nbegan: %s\n", s.title, s.device, s.began.UTC().Format("2006-01-02T15:04:05Z"))
		if s.order != "" {
			fmt.Fprintf(&b, "order: %s\n", s.order)
		}
		b.WriteString("\n")
	}
	if !brief || len(s.outcomes) > 1 && s.count(passed) < len(s.outcomes) { // or it would only repeat
		for _, o := range s.outcomes {
			if failuresOnly && o.status == passed {
				continue
			}
			b.WriteString(o.line() + "\n")
			if !brief {
				fmt.Fprintf(&b, "  file: %s\n", filepath.ToSlash(o.flow.Path))
				if o.status.good() { // or it is among the details
					b.WriteString(o.videos())
				}
				b.WriteString(o.details(true))
			}
		}
		b.WriteString("\n")
	}
	var parts []string
	for _, c := range []struct {
		status status
		name   string
	}{{passed, "passed"}, {flaky, "flaky"}, {failed, "failed"}, {interrupted, "interrupted"}, {notRun, "not run"}} {
		if n := s.count(c.status); n > 0 || c.status == passed || c.status == failed {
			parts = append(parts, fmt.Sprintf("%d %s", n, c.name))
		}
	}
	fmt.Fprintf(&b, "TOTAL %d: %s  %.1f s\n", len(s.outcomes), strings.Join(parts, ", "), s.took.Seconds())
	return b.String()
}

// save writes what was asked for, and what --rerun-failed needs.
func (s *summary) save(o *options) error {
	err := s.saveLast(o)
	if o.format != "" {
		err = errors.Join(err, writeJUnit(o.output, s))
	}
	if o.results != "" {
		err = errors.Join(err, s.write(o))
	}
	return err
}

// Formats of the results file.
const (
	formatText  = "txt"
	formatJSON  = "json"
	formatJUnit = "junit"
)

func checkFormat(name string) error {
	if name != formatText && name != formatJSON && name != formatJUnit {
		return fmt.Errorf("unknown results format %q (use txt, json or junit)", name)
	}
	return nil
}

func (s *summary) write(o *options) error {
	switch o.resultsFormat {
	case formatJUnit: // whole, as what reads it counts
		return writeJUnit(o.results, s)
	case formatJSON:
		data, err := s.json(o.failuresOnly)
		if err != nil {
			return err
		}
		return writeFile(o.results, data)
	}
	return writeFile(o.results, []byte(s.text(o.failuresOnly, false)))
}

func (s *summary) err() error {
	var bad []string
	for _, o := range s.outcomes {
		switch o.status {
		case failed, interrupted:
			bad = append(bad, o.flow.Name)
		case notRun:
			bad = append(bad, o.flow.Name+" (not run)")
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("failed: %s", strings.Join(bad, ", "))
	}
	return nil
}
