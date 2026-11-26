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
	Flaky       Status = "FLAKY" // failed, then passed on a retry
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
	Retries     int       // allowed
	Why         string    // it didn't run
	Began       time.Time // its first try; zero if it didn't run
	Partial     bool      // it began or stopped at a step, so a pass isn't the whole flow's
}

// Judge takes a try into account.
func (o *Outcome) Judge(ctx context.Context, try flow.Result) {
	o.Result = try
	o.Tries = append(o.Tries, try)
	o.Status = judge(ctx, try)
	if o.Status == Passed && o.TimesFailed() > 0 {
		o.Status = Flaky
	}
}

// Passed says whether it passed at last, at once or on a retry.
func (o *Outcome) Passed() bool { return o.Status == Passed || o.Status == Flaky }

func judge(ctx context.Context, o flow.Result) Status {
	switch {
	case o.Err == nil:
		return Passed
	case ctx.Err() != nil:
		return Interrupted
	}
	return Failed
}

// Took is how long all its tries took.
func (o *Outcome) Took() time.Duration { return o.took() }

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
	if o.Status == Interrupted {
		return line + "  (" + o.Interruption() + ")"
	}
	if n := o.TimesFailed(); len(o.Tries) > 1 {
		times := map[bool]string{true: "once", false: fmt.Sprintf("%d times", n)}[n == 1]
		line += "  (failed " + times + map[bool]string{true: ", then passed"}[o.Passed()] + ")"
	}
	return line
}

// Interruption is where an interrupted flow was: "interrupted at <step> (file:line)", or "interrupted".
func (o *Outcome) Interruption() string {
	var step *flow.StepError
	if errors.As(o.Err, &step) {
		return fmt.Sprintf("interrupted at %s (%s)", step.Step.Text(), step.Step.Place())
	}
	return "interrupted"
}

// Verdict is a failed try in a line: "<flow> failed at <step>: <cause>", the cause one of flow.Causes. Of
// step failed, it goes on with the first line of why and where the step is written, so the line is enough
// on its own. A failure that isn't of a step says so.
func (o *Outcome) Verdict(t flow.Result) string {
	at := "outside a step"
	if t.Failure == nil || len(t.Failure.Before) == 0 {
		at = "before its first step"
	}
	var step *flow.StepError
	if errors.As(t.Err, &step) {
		at = "at " + step.Step.Text()
	}
	kind := causeOf(t)
	line := fmt.Sprintf("%s failed %s: %s", o.Flow.Name, at, kind)
	if kind == flow.StepFailed && t.Err != nil {
		why := t.Err
		if step != nil {
			why = step.Err
		}
		first, _, _ := strings.Cut(strings.TrimSpace(why.Error()), "\n")
		line += ": " + strings.TrimSpace(first)
		if step != nil {
			line += fmt.Sprintf(" (%s:%d)", filepath.Base(step.Step.Path), step.Step.Line)
		}
	}
	return line
}

// causeOf is what went wrong in a failed try, one of flow.Causes.
func causeOf(t flow.Result) string {
	if t.Failure != nil && t.Failure.Cause != "" {
		return t.Failure.Cause
	}
	return flow.StepFailed
}

// lastFailed is its last try that failed; nil if none did.
func (o *Outcome) lastFailed() *flow.Result {
	for i := len(o.Tries) - 1; i >= 0; i-- {
		if o.Tries[i].Err != nil {
			return &o.Tries[i]
		}
	}
	return nil
}

// blameOf is whose fault a failed try is, and what shows it; empty if nothing says.
func blameOf(t flow.Result) string {
	f := t.Failure
	if f == nil || f.Blame == "" {
		return ""
	}
	if f.Because != "" {
		return string(f.Blame) + ", " + f.Because
	}
	return string(f.Blame)
}

// Details is what helps to know why it failed, as much as detail says: the verdict, then "  key: value"
// lines, and more indented ones for what doesn't fit in a line.
func (o *Outcome) Details(detail string) string {
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
		b.WriteString("  " + o.Verdict(t) + "\n")
		f := t.Failure
		if detail == flow.Brief {
			if f != nil {
				add("saved", f.Saved)
			}
			continue
		}
		var step *flow.StepError
		if errors.As(t.Err, &step) {
			add("why", step.Err.Error())
			o.addBlame(add, t)
			add("step", step.Step.Text())
			add("at", step.Step.Place())
			for _, from := range step.From {
				add("from", from.Place()+" "+from.Text())
			}
		} else {
			add("why", t.Err.Error())
			o.addBlame(add, t)
		}
		if f == nil {
			continue
		}
		add("crash", f.Crash)
		list("before", f.Before)
		list("nearest", f.Nearest)
		var calls []string
		for _, c := range f.Calls {
			calls = append(calls, c.String())
		}
		list("calls", calls)
		if detail == flow.Full {
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
	if !o.Passed() {
		b.WriteString(o.videos())
	}
	b.WriteString(o.WarningLines())
	return b.String()
}

func (o *Outcome) addBlame(add func(key, value string), t flow.Result) {
	add("blame", blameOf(t))
}

type flakyFailure struct{ message, cause, text string }

// flakyFailures are the tries that failed of a flow that passed at last: the error, the cause and the details.
func (o *Outcome) flakyFailures(detail string) []flakyFailure {
	var list []flakyFailure
	for i, t := range o.Tries {
		if t.Err == nil {
			continue
		}
		one := Outcome{Flow: o.Flow, Status: Failed, Tries: []flow.Result{t}}
		f := flakyFailure{message: fmt.Sprintf("try %d: %s", i+1, strings.Join(strings.Fields(t.Err.Error()), " ")), text: one.Details(detail)}
		f.cause = causeOf(t)
		if blame := blameOf(t); blame != "" {
			f.message += " (" + blame + ")"
		}
		list = append(list, f)
	}
	return list
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
		if t.Err == nil || !o.Passed() {
			continue
		}
		fmt.Fprintf(&b, "  try %d: %s", i+1, strings.Join(strings.Fields(t.Err.Error()), " "))
		if f := t.Failure; f != nil {
			if f.Blame != "" {
				fmt.Fprintf(&b, "  (%s)", f.Blame)
			}
			if f.Saved != "" {
				fmt.Fprintf(&b, "  %s", f.Saved)
			}
		}
		b.WriteString("\n")
	}
	if o.Status == Flaky {
		b.WriteString("  next: " + o.next() + "\n")
	}
	return b.String()
}

// next is what to do about a flaky flow, by how its first failed try went.
func (o *Outcome) next() string {
	for _, t := range o.Tries {
		if t.Err == nil || t.Failure == nil {
			continue
		}
		switch {
		case t.Failure.Blame == flow.Dev:
			return "the device failed, not the flow: check it with rayo devices; if it happens often, a cold boot of the emulator helps"
		case t.Failure.Blame == flow.App || t.Failure.Cause == flow.AppCrashed:
			return "the app crashed or hung: the crash is in the try's logcat; it is a bug of the app, not of the flow"
		case t.Failure.Cause == flow.UnmockedCall:
			return "a call had no mock only sometimes: mock it, or ignore it with ignoreUnmocked"
		case t.Failure.Cause == flow.NotOnScreen, t.Failure.Cause == flow.Timeout:
			return "the screen wasn't there in time: wait for what comes before it (assertVisible, waitForRequest) instead of a longer timeout; rayo test --timings lists the waits near their limit"
		}
		break
	}
	return "run it alone a few times (--only " + o.Flow.Name + " --retries 0) to see where it fails; see Fix a flaky test in the guide"
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
	FlakyFails           bool     // flaky-fails-build: a flow that passed only on a retry fails the run
	Show                 []string // which flows the results list, see Shows; all if empty
	Slowest              int      // how many the slow list has
	Template             string   // results-template: a file, for results with its extension
	Detail               string   // how much a failure says: full in the files too, else normal
	Exit                 int      // the exit code of the run, one of Exits
	Shard, Shards        int      // the part of --shard i/n; 0 if all
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

// AllPassed says whether every flow passed, at once or on a retry: else failed.json says which didn't.
func (s *Summary) AllPassed() bool {
	for _, o := range s.Outcomes {
		if !o.Passed() {
			return false
		}
	}
	return true
}

// blame is of the last try of a flow that failed.
func (o *Outcome) blame() flow.Blame {
	if o.Status == Failed && o.Failure != nil {
		return o.Failure.Blame
	}
	return ""
}

func (s *Summary) blamed(by flow.Blame) int {
	n := 0
	for _, o := range s.Outcomes {
		if o.blame() == by {
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
func (s *Summary) Text(brief bool) string {
	var b strings.Builder
	if !brief {
		fmt.Fprintf(&b, "run: %s\ndevice: %s\nbegan: %s\n", s.Title, s.Device, s.Began.UTC().Format("2006-01-02T15:04:05Z"))
		if s.Order != "" {
			fmt.Fprintf(&b, "order: %s\n", s.Order)
		}
		if s.Shards > 0 {
			fmt.Fprintf(&b, "part: %d of %d\n", s.Shard, s.Shards)
		}
		b.WriteString("\n")
	}
	if !brief || len(s.Outcomes) > 1 && s.count(Passed) < len(s.Outcomes) { // or it would only repeat
		for _, o := range s.Shown() {
			b.WriteString(o.Line() + "\n")
			if !brief {
				fmt.Fprintf(&b, "  file: %s\n", filepath.ToSlash(o.Flow.Path))
				if o.Passed() { // or it is among the details
					b.WriteString(o.videos())
					b.WriteString(o.Notes() + o.WarningLines())
				} else {
					b.WriteString(o.Details(s.fileDetail()))
				}
			}
		}
		b.WriteString("\n")
	}
	var parts []string
	for _, c := range []struct {
		status Status
		name   string
	}{{Passed, "passed"}, {Flaky, "flaky"}, {Failed, "failed"}, {Interrupted, "interrupted"}, {NotRun, "not run"}} {
		if n := s.count(c.status); n > 0 || c.status == Passed || c.status == Failed {
			parts = append(parts, fmt.Sprintf("%d %s", n, c.name))
		}
	}
	if s.Stopped != "" {
		fmt.Fprintf(&b, "STOPPED %s\n", s.Stopped)
	}
	var causes []string
	for _, c := range []flow.Blame{flow.App, flow.Dev} {
		if n := s.blamed(c); n > 0 {
			causes = append(causes, fmt.Sprintf("%d %s", n, c))
		}
	}
	if len(causes) > 0 {
		parts = append(parts, "of those failed "+strings.Join(causes, " and "))
	}
	if n := s.retried(); n > 0 {
		parts = append(parts, fmt.Sprintf("%d retried", n))
	}
	total := fmt.Sprint(len(s.Outcomes))
	if s.Shards > 0 {
		total += " in " + s.Part()
	}
	fmt.Fprintf(&b, "TOTAL %s: %s  %.1f s\n", total, strings.Join(parts, ", "), s.Took.Seconds())
	return b.String()
}

// Part is "part 2 of 4", of --shard.
func (s *Summary) Part() string { return fmt.Sprintf("part %d of %d", s.Shard, s.Shards) }

// Formats of the results.
const (
	Text     = "txt"
	JSON     = "json"
	JUnit    = "junit"
	Markdown = "md"
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
	case ".md":
		return Markdown, nil
	}
	return "", fmt.Errorf("takes a .txt, .json, .xml (JUnit) or .md file, not %q", filepath.ToSlash(path))
}

// Write saves the results, in the format of the file.
func (s *Summary) Write(path string) error {
	format, err := FormatOf(path)
	if err != nil {
		return err
	}
	var data []byte
	switch {
	case s.Template != "" && strings.EqualFold(filepath.Ext(path), filepath.Ext(s.Template)):
		data, err = s.fromTemplate()
	case format == JUnit: // whole, as what reads it counts
		data, err = s.junit()
	case format == JSON:
		data, err = s.json()
	case format == Text:
		data = []byte(s.Text(false))
	case format == Markdown:
		data = []byte(s.markdown())
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
	if s.FlakyFails {
		for _, o := range s.Outcomes {
			if o.Status == Flaky {
				bad = append(bad, o.Flow.Name)
			}
		}
		if len(bad) > 0 {
			return &FlakyError{bad}
		}
	}
	return nil
}

// FlakyError is of a run where every flow passed, some only on a retry, with flaky-fails-build.
type FlakyError struct{ Flows []string }

func (e *FlakyError) Error() string { return "flaky: " + strings.Join(e.Flows, ", ") }

// fileDetail is how much a failure says in the results: normal, or full if asked.
func (s *Summary) fileDetail() string {
	if s.Detail == flow.Full {
		return flow.Full
	}
	return flow.Normal
}
