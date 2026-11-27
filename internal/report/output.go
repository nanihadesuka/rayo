package report

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/schema"
)

// What rayo prints for a program, with --format json or ndjson. Every key is always there.

// SchemaVersion is of all of it, and of the results: it changes if the meaning of a key does.
const SchemaVersion = 3

// Formats of what is printed, besides JSON.
const (
	Human  = "human"
	NDJSON = "ndjson"
)

// Formats are all of them, the default first.
var Formats = []string{Human, JSON, NDJSON}

// Exit codes: this is the only place they are told.
const (
	ExitPassed      = 0
	ExitFailed      = 1
	ExitConfig      = 2
	ExitDevice      = 3
	ExitFlaky       = 4
	ExitInterrupted = 130
)

// Exit is a code, for the docs.
type Exit struct {
	Code int    `json:"code"`
	Name string `json:"name"`
	What string `json:"what"`
}

// Exits are all the codes.
var Exits = []Exit{
	{ExitPassed, "passed", "Every flow passed, or the command did what it was asked"},
	{ExitFailed, "failed", "A flow failed, was interrupted or didn't run; or a step sent with `rayo do` failed"},
	{ExitConfig, "configuration", "Something written is wrong: an option, a flow, a suite or a mock; nothing ran"},
	{ExitDevice, "device", "No device to run on, or it was lost or unusable, so the flows couldn't run or didn't all run"},
	{ExitFlaky, "flaky", "Every flow passed, some only on a retry, and `flaky-fails-build` is `true`; else such a run exits `0`"},
	{ExitInterrupted, "interrupted", "Ctrl-C, or SIGTERM as CI sends when it cancels a job: what ran is in the results, each interrupted flow with the step it was at"},
}

func exitTable() string {
	var b strings.Builder
	b.WriteString("| Code | | |\n|---|---|---|\n")
	for _, e := range Exits {
		fmt.Fprintf(&b, "| `%d` | %s | %s |\n", e.Code, e.Name, e.What)
	}
	return b.String()
}

// Problem is an error as data: of something written, or of a flow that failed.
type Problem struct {
	File     string `json:"file" about:"Where it is written; empty if it isn't of a file"`
	Line     int    `json:"line" about:"0 if not known"`
	Step     string `json:"step" about:"Of a flow that failed, the step"`
	Message  string `json:"message" about:"What is wrong"`
	Expected string `json:"expected" about:"What it takes, if that can be said"`
	Nearest  string `json:"nearest" about:"Of that, the closest to what is written"`
	Fix      string `json:"fix" about:"What to write instead, if that can be said"`
}

// ProblemOf describes an error.
func ProblemOf(err error) Problem {
	p := Problem{Message: err.Error()}
	var step *flow.StepError
	var wrong *flow.Problem
	switch {
	case errors.As(err, &step):
		p.Step, p.File, p.Line, p.Message = step.Step.Text(), filepath.ToSlash(step.Step.Path), step.Step.Line, step.Err.Error()
	case errors.As(err, &wrong):
		p.File, p.Line, p.Message = filepath.ToSlash(wrong.Path), wrong.Line, wrong.What
		p.Expected, p.Nearest, p.Fix = wrong.Expected, wrong.Nearest, wrong.Fix
	}
	return p
}

// ProblemsOf describes an error, or each of several.
func ProblemsOf(err error) []Problem {
	var several interface{ Unwrap() []error }
	if !errors.As(err, &several) {
		return []Problem{ProblemOf(err)}
	}
	list := []Problem{}
	for _, e := range several.Unwrap() {
		list = append(list, ProblemsOf(e)...)
	}
	return list
}

// Errors is what any command prints instead, if it can't do what it was asked.
type Errors struct {
	SchemaVersion int       `json:"schemaVersion" about:"Changes if the meaning of a key does"`
	OK            bool      `json:"ok" about:"false"`
	Exit          int       `json:"exit" about:"The exit code, see [Exit codes](#exit-codes)"`
	Errors        []Problem `json:"errors" about:"One for each, see [Errors](#errors)"`
}

// ErrorsOf is an error, for a program.
func ErrorsOf(err error, exit int) Errors {
	return Errors{SchemaVersion: SchemaVersion, Exit: exit, Errors: ProblemsOf(err)}
}

// Check is what rayo check prints: the plan.
type Check struct {
	SchemaVersion int       `json:"schemaVersion" about:"Changes if the meaning of a key does"`
	OK            bool      `json:"ok" about:"true: else it prints the errors"`
	Count         int       `json:"count" about:"How many flows are fine, and would run"`
	Flows         []Planned `json:"flows" about:"Those that would run, in that order"`
}

// Planned is a flow that would run.
type Planned struct {
	Name    string   `json:"name" about:"The flow's"`
	File    string   `json:"file" about:"The flow's"`
	Tags    []string `json:"tags" about:""`
	Mocks   []string `json:"mocks" about:"As written in it, or in its suite"`
	Retries int      `json:"retries" about:"How many times it is tried again if it fails"`
}

// Devices is what rayo devices prints.
type Devices struct {
	SchemaVersion int      `json:"schemaVersion" about:"Changes if the meaning of a key does"`
	Devices       []string `json:"devices" about:"The serials of those connected and ready"`
}

// Hierarchy is what rayo hierarchy prints.
type Hierarchy struct {
	SchemaVersion int      `json:"schemaVersion" about:"Changes if the meaning of a key does"`
	Screen        []string `json:"screen" about:"The elements on it, a line each, as rayo hierarchy prints them"`
}

// Notes is what rayo mocks prints.
type Notes struct {
	SchemaVersion int      `json:"schemaVersion" about:"Changes if the meaning of a key does"`
	Notes         []string `json:"notes" about:"What was done, a line each"`
}

// Event is a line of rayo test --format ndjson, as it happens.
type Event struct {
	SchemaVersion int       `json:"schemaVersion" about:"Changes if the meaning of a key does"`
	Event         string    `json:"event" about:"What happened, see [Events](#events)" enum:"flowStarted,stepFailed,attempt,held,flowFinished,outcome,error"`
	Flow          string    `json:"flow" about:"Its name; empty in outcome and error"`
	File          string    `json:"file" about:"The flow's"`
	Index         int       `json:"index" about:"Of the flow, from 1"`
	Of            int       `json:"of" about:"Flows to run"`
	Try           int       `json:"try" about:"Of stepFailed and attempt, from 1"`
	Status        Status    `json:"status" about:"Of attempt, PASS or FAIL; of flowFinished, the flow's [outcome](#outcomes)" enum:"PASS,PARTIAL,FLAKY,FAIL,INTERRUPTED,NOT_RUN,"`
	Error         *Problem  `json:"error" about:"Of stepFailed and of an attempt that failed, see [Errors](#errors)"`
	Result        *jsonFlow `json:"result" about:"Of flowFinished, the flow as in the [results](#results)"`
	Results       *jsonRun  `json:"results" about:"Of outcome, the [results](#results), as --format json"`
	Exit          int       `json:"exit" about:"Of outcome and error, the exit code, see [Exit codes](#exit-codes)"`
	Errors        []Problem `json:"errors" about:"Of error, as rayo prints them with --format json"`
	Session       string    `json:"session" about:"Of held, the output folder of the session that holds the flow: rayo do --output <it> .help, or rayo do .help there"`
}

func event(kind string, f *flow.Flow, i, of int) Event {
	e := Event{SchemaVersion: SchemaVersion, Event: kind, Index: i, Of: of, Errors: []Problem{}}
	if f != nil {
		e.Flow, e.File = f.Name, filepath.ToSlash(f.Path)
	}
	return e
}

// Started is the event of a flow that begins.
func Started(f *flow.Flow, i, of int) Event { return event("flowStarted", f, i, of) }

// Tried is the events of the last try of a flow: the step that failed, if one did, and the try.
func Tried(o *Outcome, i, of int) []Event {
	try := event("attempt", o.Flow, i, of)
	try.Try, try.Status = len(o.Tries), o.Status
	if o.Err == nil {
		try.Status = Passed
		return []Event{try}
	}
	if try.Status != Interrupted {
		try.Status = Failed
	}
	p := ProblemOf(o.Err)
	try.Error = &p
	var step *flow.StepError
	if !errors.As(o.Err, &step) {
		return []Event{try}
	}
	failed := event("stepFailed", o.Flow, i, of)
	failed.Try, failed.Error = try.Try, &p
	return []Event{failed, try}
}

// Held is the event of a flow held where it failed, with --on-fail hold, until rayo do says how to go on.
func Held(f *flow.Flow, i, of int, err error, folder string) Event {
	e := event("held", f, i, of)
	p := ProblemOf(err)
	e.Error, e.Session = &p, filepath.ToSlash(folder)
	return e
}

// Finished is the event of a flow after all its tries, or that didn't run.
func Finished(o *Outcome, i, of int) Event {
	e := event("flowFinished", o.Flow, i, of)
	e.Status, e.Try = o.Status, len(o.Tries)
	r := o.json()
	e.Result = &r
	return e
}

// Outcome is the last event: the results.
func (s *Summary) Outcome(exit int) Event {
	e := event("outcome", nil, 0, len(s.Outcomes))
	r := s.run()
	e.Results, e.Exit = &r, exit
	return e
}

// JSON is the results, as --format json prints them.
func (s *Summary) JSON() ([]byte, error) { return s.json() }

// ErrorEvent is an error of the run, as the last event.
func ErrorEvent(err error, exit int) Event {
	e := event("error", nil, 0, 0)
	e.Exit, e.Errors = exit, ProblemsOf(err)
	return e
}

// Schemas are those of what is printed for a program, by name.
func Schemas() map[string]func() []byte {
	file := func(v any, title string) func() []byte {
		return func() []byte { return schema.Written(v).File(title, nil) }
	}
	return map[string]func() []byte{
		"results":   Schema,
		"check":     file(Check{}, "What rayo check prints, --format json"),
		"devices":   file(Devices{}, "What rayo devices prints, --format json"),
		"hierarchy": file(Hierarchy{}, "What rayo hierarchy prints, --format json"),
		"mocks":     file(Notes{}, "What rayo mocks prints, --format json"),
		"explain":   file(Explained{}, "What rayo explain prints, --format json"),
		"event":     file(Event{}, "A line of rayo test --format ndjson"),
		"errors":    file(Errors{}, "What rayo prints, --format json, if it can't do what was asked"),
	}
}
