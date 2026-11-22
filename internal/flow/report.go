package flow

import (
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/mock"
)

// Status is how a step went.
type Status int

// How a step can go. Started is a group's, whose commands come next.
const (
	Passed Status = iota
	Failed
	Optional // failed, but that is fine
	Skipped
	Started
)

// StepReport is what there is to say about a step.
type StepReport struct {
	Depth   int
	Status  Status
	Text    string
	Took    time.Duration
	Profile string // where the time went, if asked for
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
}

// Failure is what helps to understand one.
type Failure struct {
	Before []string    // the last steps that passed
	Calls  []mock.Call // the last ones to the backend
	Screen string      // the elements on it
	Crash  string      // of the app, from the device log
	Saved  string      // the folder with the rest
}

// crash is the app's, if it crashed or hung since the flow began.
func (r *Runner) crash(log string) string {
	if r.flow.App == "" || (!strings.Contains(log, "FATAL EXCEPTION") && !strings.Contains(log, "ANR in ")) {
		return ""
	}
	// The log has the device's clock.
	const layout = "01-02 15:04:05"
	out, err := r.device.Exec("date '+%m-%d %H:%M:%S'")
	if err != nil {
		return ""
	}
	now, err := time.Parse(layout, strings.TrimSpace(string(out)))
	if err != nil {
		return ""
	}
	since := now.Add(-time.Since(r.began) - time.Second).Format(layout)
	var found []string
	for line := range strings.Lines(log) {
		line = strings.TrimSpace(line)
		_, message, ok := strings.Cut(line, ": ")
		if !ok || len(line) < len(layout) || line[:len(layout)] < since {
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
