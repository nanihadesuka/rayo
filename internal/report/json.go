package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/mock"
	"github.com/nanihadesuka/rayo/internal/schema"
)

// The results as JSON. Every key is always there, so that whoever reads it needn't check; its meaning
// doesn't change within a version, see SchemaVersion.

type jsonRun struct {
	SchemaVersion int         `json:"schemaVersion" about:"Changes if the meaning of a key does"`
	Run           string      `json:"run" about:"The suite's name, or what was run"`
	Device        string      `json:"device" about:"Its serial"`
	Order         string      `json:"order" about:"'--shuffle=<seed>', if shuffled"`
	Shard         *jsonShard  `json:"shard" about:"The part of --shard i/n this run was; null if all"`
	Stopped       string      `json:"stopped" about:"Why, if it stopped before the end, see [Blame](#blame)"`
	OK            bool        `json:"ok" about:"Nothing failed, was interrupted or didn't run, nor was flaky with flaky-fails-build: as the exit code"`
	Summary       jsonSummary `json:"summary" about:"The run at a glance, see below"`
	Flows         []jsonFlow  `json:"flows" about:"Those that show says, see below"`
}

type jsonShard struct {
	Index int `json:"index" about:"i, from 1"`
	Of    int `json:"of" about:"n"`
}

type jsonFlow struct {
	Name         string     `json:"name" about:"The flow's"`
	File         string     `json:"file" about:"The flow's"`
	Status       Status     `json:"status" about:"One of the [outcomes](#outcomes)" enum:"PASS,FLAKY,FAIL,INTERRUPTED,NOT_RUN"`
	Blame        flow.Blame `json:"blame" about:"Of a flow that failed, whose fault it is, or empty, see [Blame](#blame)" enum:"test,app,device,"`
	Began        string     `json:"began" about:"Its first try, in UTC; empty if it didn't run"`
	Seconds      float64    `json:"seconds" about:"All its tries"`
	Attempts     string     `json:"attempts" about:"Such as 'passed on retry 2' or 'failed all 3 tries'; empty for one try"`
	Step         string     `json:"step" about:"Of its last try that failed, the step that failed"`
	At           string     `json:"at" about:"Where that step is written, file:line"`
	Why          string     `json:"why" about:"What went wrong in that try"`
	Tries        int        `json:"tries" about:"Those it took"`
	TimesFailed  int        `json:"timesFailed" about:"Of those"`
	Retries      int        `json:"retries" about:"Those allowed"`
	NotRunReason string     `json:"notRunReason" about:"If it is 'NOT_RUN'"`
	Warnings     []string   `json:"warnings" about:"What may explain this, or the next flow"`
	Recordings   []string   `json:"recordings" about:"Videos"`
	Failures     []Failure  `json:"failures" about:"One for each try that failed, also of a flow that passed at last"`
}

// Failure is one, for a program.
type Failure struct {
	Try       int          `json:"try" about:"From 1"`
	Verdict   string       `json:"verdict" about:"In a line: '<flow> failed at <step>: <cause>'; of 'step failed', then the first line of why and '(<file>:<line>)'; 'failed before its first step' or 'failed outside a step' if it isn't of a step; of an interrupted flow, '<flow> interrupted at <step> (<file:line>)'"`
	Cause     string       `json:"cause" about:"What went wrong, see [Results](#results)" enum:"not on screen,timeout,app crashed,unmocked call,device error,step failed"`
	Error     Problem      `json:"error" about:"What went wrong, at which step, see [Errors](#errors)"`
	Blame     flow.Blame   `json:"blame" about:"Whose fault it is: 'test', 'app' or 'device', see [Blame](#blame)" enum:"test,app,device"`
	Because   string       `json:"because" about:"What shows that it isn't the test"`
	From      []jsonStep   `json:"from" about:"The step that ran that flow, and so on outwards"`
	Before    []string     `json:"before" about:"The last 3 steps that passed"`
	Nearest   []string     `json:"nearest" about:"Of 'not on screen', the elements on it most like what was looked for"`
	Calls     []mock.Call  `json:"calls" about:"The last 5 to the mock, and before them those that went wrong in the failure window"`
	Screen    []string     `json:"screen" about:"The elements on it"`
	Crash     string       `json:"crash" about:"The app crashed or hung during the flow: what the device log says"`
	Saved     string       `json:"saved" about:"The first file of the rest, which are named alike, see [Failures](#failures)"`
	Scenarios []Scenario   `json:"scenarios" about:"Of the mocks, the state each was in as it failed; empty without scenarios"`
	Trail     []StepStates `json:"trail" about:"The scenarios as each step began, the last 2000 steps: what --resume and --from set, see [Resume](#resume)"`
}

// Scenario is one of the mocks, and its state.
type Scenario struct {
	Name  string `json:"name" about:"Its scenarioName"`
	State string `json:"state" about:""`
}

// StepStates is the scenarios as a step began.
type StepStates struct {
	At        string     `json:"at" about:"The step, file:line"`
	Scenarios []Scenario `json:"scenarios" about:""`
}

func scenariosOf(list []flow.Scenario) []Scenario {
	out := []Scenario{}
	for _, s := range list {
		out = append(out, Scenario{s.Name, s.State})
	}
	return out
}

// Flow is them as the runner takes them.
func (l StepStates) Flow() flow.StepStates {
	s := flow.StepStates{At: l.At}
	for _, x := range l.Scenarios {
		s.States = append(s.States, flow.Scenario{Name: x.Name, State: x.State})
	}
	return s
}

type jsonStep struct {
	Step string `json:"step" about:""`
	File string `json:"file" about:""`
	Line int    `json:"line" about:""`
}

func seconds(d time.Duration) float64 { return math.Round(d.Seconds()*1000) / 1000 }

// List is never null.
func List[T any](l []T) []T {
	if l == nil {
		return []T{}
	}
	return l
}

func stepOf(s *flow.Step) jsonStep { return jsonStep{s.Text(), filepath.ToSlash(s.Path), s.Line} }

// Lines are those of a text; never null.
func Lines(text string) []string {
	if text == "" {
		return []string{}
	}
	return strings.Split(strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), "\n")
}

// FailureOf describes a failure.
func FailureOf(err error, e *flow.Failure) Failure {
	x := Failure{Try: 1, Error: ProblemOf(err), From: []jsonStep{}, Before: []string{}, Calls: []mock.Call{}, Screen: []string{}, Nearest: []string{}, Cause: flow.StepFailed,
		Scenarios: []Scenario{}, Trail: []StepStates{}}
	var step *flow.StepError
	if errors.As(err, &step) {
		for _, s := range step.From {
			x.From = append(x.From, stepOf(s))
		}
	}
	if e != nil {
		x.Blame, x.Because = e.Blame, e.Because
		x.Before, x.Calls, x.Crash, x.Saved = List(e.Before), List(e.Calls), e.Crash, e.Saved
		x.Screen = Lines(e.Screen)
		x.Cause, x.Nearest = e.Cause, List(e.Nearest)
		x.Scenarios = scenariosOf(e.Scenarios)
		for _, t := range e.Trail {
			x.Trail = append(x.Trail, StepStates{t.At, scenariosOf(t.States)})
		}
	}
	return x
}

func (o *Outcome) json() jsonFlow {
	f := jsonFlow{Name: o.Flow.Name, File: filepath.ToSlash(o.Flow.Path), Status: o.Status, Blame: o.blame(), Seconds: seconds(o.took()),
		Began: stamp(o.Began), Attempts: o.Attempts(), Tries: len(o.Tries), TimesFailed: o.TimesFailed(), Retries: o.Retries, NotRunReason: o.Why, Warnings: List(o.Warnings), Recordings: []string{}, Failures: []Failure{}}
	for i, t := range o.Tries {
		f.Recordings = append(f.Recordings, t.Recordings...)
		if t.Err == nil {
			continue
		}
		x := FailureOf(t.Err, t.Failure)
		x.Try, x.Verdict = i+1, o.Verdict(t)
		if o.Status == Interrupted && i == len(o.Tries)-1 {
			x.Verdict = o.Flow.Name + " " + o.Interruption()
		}
		f.Failures = append(f.Failures, x)
	}
	f.Step, f.At, f.Why, _, _ = o.failing()
	return f
}

func (s *Summary) json() ([]byte, error) {
	return Encode(s.run(), true)
}

// run is the results, as written.
func (s *Summary) run() jsonRun {
	r := jsonRun{SchemaVersion: SchemaVersion, Run: s.Title, Device: s.Device, Order: s.Order, Shard: s.shard(), Stopped: s.Stopped, OK: s.Err() == nil,
		Summary: s.summary(), Flows: []jsonFlow{}}
	for _, o := range s.Shown() {
		r.Flows = append(r.Flows, o.json())
	}
	return r
}

func (s *Summary) shard() *jsonShard {
	if s.Shards == 0 {
		return nil
	}
	return &jsonShard{s.Shard, s.Shards}
}

// Encode is JSON for a program: indented, or else on one line.
func Encode(v any, indent bool) ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if indent {
		e.SetIndent("", "  ")
	}
	err := e.Encode(v)
	return b.Bytes(), err
}

// Schema is the JSON Schema of the results.
func Schema() []byte { return schema.Written(jsonRun{}).File("Results of rayo", nil) }

// Tables are those of the docs.
func Tables() map[string]string {
	return map[string]string{
		"results":         schema.ColumnTable(jsonRun{}),
		"results summary": schema.ColumnTable(jsonSummary{}),
		"results flow":    schema.ColumnTable(jsonFlow{}),
		"results failure": schema.ColumnTable(Failure{}),
		"error":           schema.ColumnTable(Problem{}),
		"errors":          schema.ColumnTable(Errors{}),
		"event":           schema.ColumnTable(Event{}),
		"exit codes":      exitTable(),
	}
}
