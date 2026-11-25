package flow

import (
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/mock"
)

// Status is how a step went.
type Status int

// How a step can go. Started is a group's, whose commands come next; Finished is its end, with
// --timings.
const (
	Passed Status = iota
	Failed
	Optional // failed, but that is fine
	Skipped
	Started
	Finished
)

// StepReport is what there is to say about a step.
type StepReport struct {
	Depth   int
	Status  Status
	Text    string
	Took    time.Duration
	Profile string // where the time went, if asked for
}

var marks = map[Status]string{Passed: "✓", Failed: "✗", Optional: "–", Skipped: "–", Started: "▸", Finished: "◂"}

// Line is how it is printed.
func (s StepReport) Line() string {
	note := fmt.Sprintf("  %d ms%s", s.Took.Milliseconds(), s.Profile)
	switch s.Status {
	case Optional:
		note += " (optional)"
	case Skipped:
		note = "  skipped"
	case Started:
		note = ""
	}
	return fmt.Sprintf("%s%s %s%s\n", strings.Repeat("  ", s.Depth+1), marks[s.Status], s.Text, note)
}

// Reporter is told what happens as flows run.
type Reporter interface {
	Step(StepReport)
	Note(depth int, text string)
}

// Result is how a flow went.
type Result struct {
	Flow       *Flow
	Err        error
	Elapsed    time.Duration
	Failure    *Failure // nil if there was none
	Warnings   []string
	Recordings []string // files
	Files      []string // all it wrote, recordings too
}

// Failure is what helps to understand one.
type Failure struct {
	Before    []string    // the last steps that passed
	Calls     []mock.Call // the last ones to the backend
	Screen    string      // the elements on it
	Crash     string      // of the app, from the device log
	Blame     Blame
	Because   string       // of a cause that isn't the test
	Unusable  bool         // nothing can run on the device
	Saved     string       // the first file of the rest; the others are named alike
	Cause     string       // what went wrong, one of Causes
	Nearest   []string     // of not on screen, the elements most like what was looked for
	Scenarios []Scenario   // of the mocks, as it failed
	Trail     []StepStates // the scenarios as each step began, the last ones
}

// crash is the app's, if it crashed or hung since the flow began.
func (r *Runner) crash(log string) string {
	if r.flow.App == "" || (!strings.Contains(log, "FATAL EXCEPTION") && !strings.Contains(log, "ANR in ")) {
		return ""
	}
	var found []string
	for line := range r.sinceBegan(log) {
		_, message, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		switch {
		case strings.Contains(message, "ANR in "+r.flow.App):
			return message
		case strings.Contains(message, "FATAL EXCEPTION"):
			found = []string{message}
		case len(found) > 0 && len(found) < 12 && strings.Contains(line, "AndroidRuntime"):
			found = append(found, message)
			if len(found) == 2 && !strings.Contains(message, "Process: "+r.flow.App+",") {
				found = nil // another app's
			}
		}
	}
	return strings.Join(found, "\n")
}

// sinceBegan is the lines of the device log from when the flow began; none if that can't be told.
func (r *Runner) sinceBegan(log string) iter.Seq[string] {
	// The log has the device's clock.
	const layout = "01-02 15:04:05"
	return func(yield func(string) bool) {
		out, err := r.device.Exec("date '+%m-%d %H:%M:%S'")
		if err != nil {
			return
		}
		now, err := time.Parse(layout, strings.TrimSpace(string(out)))
		if err != nil {
			return
		}
		since := now.Add(-time.Since(r.began) - time.Second).Format(layout)
		for line := range strings.Lines(log) {
			line = strings.TrimSpace(line)
			if len(line) >= len(layout) && line[:len(layout)] >= since && !yield(line) {
				return
			}
		}
	}
}
