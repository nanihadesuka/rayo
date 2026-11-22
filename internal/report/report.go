// Package report is how a run went: on the screen, as text, JSON and JUnit.
package report

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/flow"
)

// Status is how a flow went. Its name is what is printed.
type Status string

// How a flow can go.
const (
	Passed      Status = "PASS"
	Failed      Status = "FAIL"
	Interrupted Status = "INTERRUPTED"
	NotRun      Status = "NOT_RUN"
)

// Outcome is a flow's, after all its tries.
type Outcome struct {
	flow.Result // of the last try
	Flow        *flow.Flow
	Status      Status
	Tries       []flow.Result
	Retries     int    // allowed
	Why         string // it didn't run
}

// Judge takes a try into account.
func (o *Outcome) Judge(ctx context.Context, try flow.Result) {
	o.Result = try
	o.Tries = append(o.Tries, try)
	o.Status = judge(ctx, try)
}

func judge(ctx context.Context, o flow.Result) Status {
	switch {
	case o.Err == nil:
		return Passed
	case ctx.Err() != nil:
		return Interrupted
	}
	return Failed
}

func (o *Outcome) took() (total time.Duration) {
	for _, t := range o.Tries {
		total += t.Elapsed
	}
	return total
}

// TimesFailed counts its tries that failed.
func (o *Outcome) TimesFailed() int {
	n := 0
	for _, t := range o.Tries {
		if t.Err != nil {
			n++
		}
	}
	return n
}

// Line is one line: the status is its first word.
func (o *Outcome) Line() string {
	if o.Status == NotRun {
		return fmt.Sprintf("%s %s  (%s)", o.Status, o.Flow.Name, o.Why)
	}
	line := fmt.Sprintf("%s %s  %.1f s", o.Status, o.Flow.Name, o.took().Seconds())
	if n := o.TimesFailed(); len(o.Tries) > 1 {
		times := map[bool]string{true: "once", false: fmt.Sprintf("%d times", n)}[n == 1]
		line += "  (failed " + times + map[bool]string{true: ", then passed"}[o.Status == Passed] + ")"
	}
	return line
}

// Details is everything that helps to know why it failed: "  key: value" lines, and more indented
// ones for what doesn't fit in a line.
func (o *Outcome) Details(full bool) string {
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
	for i, t := range o.Tries {
		if t.Err == nil || o.Status == Interrupted {
			continue
		}
		if len(o.Tries) > 1 {
			add("try", fmt.Sprint(i+1))
		}
		var step *flow.StepError
		if errors.As(t.Err, &step) {
			add("why", step.Err.Error())
			o.addCause(add, t)
			add("step", step.Step.Text())
			add("at", step.Step.Place())
			for _, from := range step.From {
				add("from", from.Place()+" "+from.Text())
			}
		} else {
			add("why", t.Err.Error())
			o.addCause(add, t)
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
	if o.Status != Passed {
		b.WriteString(o.videos())
	}
	b.WriteString(o.WarningLines())
	return b.String()
}

func (o *Outcome) addCause(add func(key, value string), t flow.Result) {
	if f := t.Failure; f != nil && f.Cause != "" {
		because := string(f.Cause)
		if f.Because != "" {
			because += ", " + f.Because
		}
		add("cause", because)
	}
}

// WarningLines are its warnings, to print.
func (o *Outcome) WarningLines() string {
	text := ""
	for _, w := range o.Warnings {
		text += "  warning: " + strings.ReplaceAll(strings.TrimSpace(w), "\n", "\n    ") + "\n"
	}
	return text
}

// Notes is a line for each failed try of a flow that passed at last: where it failed, and why.
func (o *Outcome) Notes() string {
	var b strings.Builder
	for i, t := range o.Tries {
		if t.Err == nil || o.Status != Passed {
			continue
		}
		fmt.Fprintf(&b, "  try %d: %s", i+1, strings.Join(strings.Fields(t.Err.Error()), " "))
		if f := t.Failure; f != nil {
			fmt.Fprintf(&b, "  (%s)", f.Cause)
			if f.Saved != "" {
				fmt.Fprintf(&b, "  %s", f.Saved)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// videos are those of every try.
func (o *Outcome) videos() string {
	text := ""
	for _, t := range o.Tries {
		for _, file := range t.Recordings {
			text += "  recording: " + file + "\n"
		}
	}
	return text
}

// Summary is how a run went.
type Summary struct {
	Title, Device, Order string
	Stopped              string // why, if the rest didn't run for it
	Began                time.Time
	Took                 time.Duration
	Outcomes             []Outcome
}

func (s *Summary) count(is Status) int {
	n := 0
	for _, o := range s.Outcomes {
		if o.Status == is {
			n++
		}
	}
	return n
}

// cause is of the last try of a flow that failed.
func (o *Outcome) cause() flow.Cause {
	if o.Status == Failed && o.Failure != nil {
		return o.Failure.Cause
	}
	return ""
}

func (s *Summary) caused(by flow.Cause) int {
	n := 0
	for _, o := range s.Outcomes {
		if o.cause() == by {
			n++
		}
	}
	return n
}

// retried is how many flows were tried again, whether they passed or not.
func (s *Summary) retried() int {
	n := 0
	for _, o := range s.Outcomes {
		if len(o.Tries) > 1 {
			n++
		}
	}
	return n
}

// Text is the results. Brief is for the screen, where the details have been shown already.
func (s *Summary) Text(failuresOnly, brief bool) string {
	var b strings.Builder
	if !brief {
		fmt.Fprintf(&b, "run: %s\ndevice: %s\nbegan: %s\n", s.Title, s.Device, s.Began.UTC().Format("2006-01-02T15:04:05Z"))
		if s.Order != "" {
			fmt.Fprintf(&b, "order: %s\n", s.Order)
		}
		b.WriteString("\n")
	}
	if !brief || len(s.Outcomes) > 1 && s.count(Passed) < len(s.Outcomes) { // or it would only repeat
		for _, o := range s.Outcomes {
			if failuresOnly && o.Status == Passed && o.TimesFailed() == 0 {
				continue
			}
			b.WriteString(o.Line() + "\n")
			if !brief {
				fmt.Fprintf(&b, "  file: %s\n", filepath.ToSlash(o.Flow.Path))
				if o.Status == Passed { // or it is among the details
					b.WriteString(o.videos())
				}
				if o.Status == Passed {
					b.WriteString(o.Notes() + o.WarningLines())
				} else {
					b.WriteString(o.Details(true))
				}
			}
		}
		b.WriteString("\n")
	}
	var parts []string
	for _, c := range []struct {
		status Status
		name   string
	}{{Passed, "passed"}, {Failed, "failed"}, {Interrupted, "interrupted"}, {NotRun, "not run"}} {
		if n := s.count(c.status); n > 0 || c.status == Passed || c.status == Failed {
			parts = append(parts, fmt.Sprintf("%d %s", n, c.name))
		}
	}
	if s.Stopped != "" {
		fmt.Fprintf(&b, "STOPPED %s\n", s.Stopped)
	}
	var causes []string
	for _, c := range []flow.Cause{flow.App, flow.Dev} {
		if n := s.caused(c); n > 0 {
			causes = append(causes, fmt.Sprintf("%d %s", n, c))
		}
	}
	if len(causes) > 0 {
		parts = append(parts, "of those failed "+strings.Join(causes, " and "))
	}
	if n := s.retried(); n > 0 {
		parts = append(parts, fmt.Sprintf("%d retried", n))
	}
	fmt.Fprintf(&b, "TOTAL %d: %s  %.1f s\n", len(s.Outcomes), strings.Join(parts, ", "), s.Took.Seconds())
	return b.String()
}

// Formats of the results.
const (
	Text  = "txt"
	JSON  = "json"
	JUnit = "junit"
)

// FormatOf is that of a results file, by how its name ends.
func FormatOf(path string) (string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".txt":
		return Text, nil
	case ".json":
		return JSON, nil
	case ".xml":
		return JUnit, nil
	}
	return "", fmt.Errorf("takes a .txt, .json or .xml (JUnit) file, not %q", filepath.ToSlash(path))
}

// Write saves the results, in the format of the file.
func (s *Summary) Write(path string, failuresOnly bool) error {
	format, err := FormatOf(path)
	if err != nil {
		return err
	}
	var data []byte
	switch format {
	case JUnit: // whole, as what reads it counts
		data, err = s.junit()
	case JSON:
		data, err = s.json(failuresOnly)
	case Text:
		data = []byte(s.Text(failuresOnly, false))
	}
	if err != nil {
		return err
	}
	return WriteFile(path, data)
}

// WriteFile also makes the folder.
func WriteFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Err says which flows didn't pass.
func (s *Summary) Err() error {
	var bad []string
	for _, o := range s.Outcomes {
		switch o.Status {
		case Failed, Interrupted:
			bad = append(bad, o.Flow.Name)
		case NotRun:
			bad = append(bad, o.Flow.Name+" (not run)")
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("failed: %s", strings.Join(bad, ", "))
	}
	return nil
}
