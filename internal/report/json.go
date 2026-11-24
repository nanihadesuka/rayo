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
// doesn't change within a version.
const resultsVersion = 1

type jsonRun struct {
	Version     int        `json:"version" about:"Changes if the meaning of a key does"`
	Run         string     `json:"run" about:"The suite's name, or what was run"`
	Device      string     `json:"device" about:"Its serial"`
	Began       string     `json:"began" about:"In UTC"`
	Seconds     float64    `json:"seconds" about:"All of it"`
	Order       string     `json:"order" about:"'--shuffle=<seed>', if shuffled"`
	Stopped     string     `json:"stopped" about:"Why, if it stopped before the end, see [Causes](#causes)"`
	OK          bool       `json:"ok" about:"Nothing failed, was interrupted or didn't run: as the exit code"`
	Total       int        `json:"total" about:"Flows"`
	Passed      int        `json:"passed" about:"Also those that failed first"`
	Failed      int        `json:"failed" about:""`
	Interrupted int        `json:"interrupted" about:""`
	NotRun      int        `json:"notRun" about:""`
	Retried     int        `json:"retried" about:"Flows tried more than once, whether they passed or not"`
	Flows       []jsonFlow `json:"flows" about:"See below"`
}

type jsonFlow struct {
	Name         string     `json:"name" about:"The flow's"`
	File         string     `json:"file" about:"The flow's"`
	Status       Status     `json:"status" about:"One of the [outcomes](#outcomes)" enum:"PASS,FAIL,INTERRUPTED,NOT_RUN"`
	Cause        flow.Cause `json:"cause" about:"Of a flow that failed, or empty, see [Causes](#causes)" enum:"test,app,device,"`
	Seconds      float64    `json:"seconds" about:"All its tries"`
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
	Try     int         `json:"try" about:"From 1"`
	Why     string      `json:"why" about:"The error"`
	Cause   flow.Cause  `json:"cause" about:"'test', 'app' or 'device', see [Causes](#causes)" enum:"test,app,device"`
	Because string      `json:"because" about:"What shows that it isn't the test"`
	Step    string      `json:"step" about:"The step that failed"`
	File    string      `json:"file" about:"Where it is written"`
	Line    int         `json:"line" about:""`
	From    []jsonStep  `json:"from" about:"The step that ran that flow, and so on outwards"`
	Before  []string    `json:"before" about:"The last 3 steps that passed"`
	Calls   []mock.Call `json:"calls" about:"The last 5 to the mock"`
	Screen  []string    `json:"screen" about:"The elements on it"`
	Crash   string      `json:"crash" about:"The app crashed or hung during the flow: what the device log says"`
	Saved   string      `json:"saved" about:"The first file of the rest, which are named alike, see [Failures](#failures)"`
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
	x := Failure{Try: 1, Why: err.Error(), From: []jsonStep{}, Before: []string{}, Calls: []mock.Call{}, Screen: []string{}}
	var step *flow.StepError
	if errors.As(err, &step) {
		at := stepOf(step.Step)
		x.Why, x.Step, x.File, x.Line = step.Err.Error(), at.Step, at.File, at.Line
		for _, s := range step.From {
			x.From = append(x.From, stepOf(s))
		}
	}
	if e != nil {
		x.Cause, x.Because = e.Cause, e.Because
		x.Before, x.Calls, x.Crash, x.Saved = List(e.Before), List(e.Calls), e.Crash, e.Saved
		x.Screen = Lines(e.Screen)
	}
	return x
}

func (o *Outcome) json() jsonFlow {
	f := jsonFlow{Name: o.Flow.Name, File: filepath.ToSlash(o.Flow.Path), Status: o.Status, Cause: o.cause(), Seconds: seconds(o.took()),
		Tries: len(o.Tries), TimesFailed: o.TimesFailed(), Retries: o.Retries, NotRunReason: o.Why, Warnings: List(o.Warnings), Recordings: []string{}, Failures: []Failure{}}
	for i, t := range o.Tries {
		f.Recordings = append(f.Recordings, t.Recordings...)
		if t.Err == nil || o.Status == Interrupted {
			continue
		}
		x := FailureOf(t.Err, t.Failure)
		x.Try = i + 1
		f.Failures = append(f.Failures, x)
	}
	return f
}

func (s *Summary) json(failuresOnly bool) ([]byte, error) {
	r := jsonRun{Version: resultsVersion, Run: s.Title, Device: s.Device, Began: s.Began.UTC().Format("2006-01-02T15:04:05Z"),
		Seconds: seconds(s.Took), Order: s.Order, Stopped: s.Stopped, OK: s.Err() == nil, Total: len(s.Outcomes), Passed: s.count(Passed),
		Failed: s.count(Failed), Interrupted: s.count(Interrupted), NotRun: s.count(NotRun), Retried: s.retried(), Flows: []jsonFlow{}}
	for _, o := range s.Outcomes {
		if !failuresOnly || o.Status != Passed || o.TimesFailed() > 0 {
			r.Flows = append(r.Flows, o.json())
		}
	}
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	e.SetIndent("", "  ")
	err := e.Encode(r)
	return b.Bytes(), err
}

// Schema is the JSON Schema of the results.
func Schema() []byte { return schema.Written(jsonRun{}).File("Results of rayo", nil) }

// Tables are those of the docs.
func Tables() map[string]string {
	return map[string]string{
		"results":         schema.ColumnTable(jsonRun{}),
		"results flow":    schema.ColumnTable(jsonFlow{}),
		"results failure": schema.ColumnTable(Failure{}),
	}
}
