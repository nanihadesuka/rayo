package main

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
)

// The results as JSON. Every key is always there, so that whoever reads it needn't check; its meaning
// doesn't change within a version.
const resultsVersion = 1

type jsonRun struct {
	Version     int        `json:"version"`
	Run         string     `json:"run"`
	Device      string     `json:"device"`
	Began       string     `json:"began"`
	Seconds     float64    `json:"seconds"`
	Order       string     `json:"order"`
	Stopped     string     `json:"stopped"` // why, if it did
	OK          bool       `json:"ok"`
	Total       int        `json:"total"`
	Passed      int        `json:"passed"`
	Failed      int        `json:"failed"`
	Interrupted int        `json:"interrupted"`
	NotRun      int        `json:"notRun"`
	Retried     int        `json:"retried"`
	Flows       []jsonFlow `json:"flows"`
}

type jsonFlow struct {
	Name         string        `json:"name"`
	File         string        `json:"file"`
	Status       status        `json:"status"`
	Cause        flow.Cause    `json:"cause"` // if it failed
	Seconds      float64       `json:"seconds"`
	Tries        int           `json:"tries"`
	TimesFailed  int           `json:"timesFailed"`
	Retries      int           `json:"retries"` // allowed
	NotRunReason string        `json:"notRunReason"`
	Warnings     []string      `json:"warnings"`
	Recordings   []string      `json:"recordings"`
	Failures     []jsonFailure `json:"failures"` // one for each try that failed
}

type jsonFailure struct {
	Try     int         `json:"try"`
	Why     string      `json:"why"`
	Cause   flow.Cause  `json:"cause"`
	Because string      `json:"because"`
	Step    string      `json:"step"`
	File    string      `json:"file"`
	Line    int         `json:"line"`
	From    []jsonStep  `json:"from"`
	Before  []string    `json:"before"`
	Calls   []mock.Call `json:"calls"`
	Screen  []string    `json:"screen"`
	Crash   string      `json:"crash"`
	Saved   string      `json:"saved"`
}

type jsonStep struct {
	Step string `json:"step"`
	File string `json:"file"`
	Line int    `json:"line"`
}

func seconds(d time.Duration) float64 { return math.Round(d.Seconds()*1000) / 1000 }

// list is never null.
func list[T any](l []T) []T {
	if l == nil {
		return []T{}
	}
	return l
}

func stepOf(s *flow.Step) jsonStep { return jsonStep{s.Text(), filepath.ToSlash(s.Path), s.Line} }

func linesOf(text string) []string {
	if text == "" {
		return []string{}
	}
	return strings.Split(strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), "\n")
}

func failureOf(err error, e *flow.Failure) jsonFailure {
	x := jsonFailure{Try: 1, Why: err.Error(), From: []jsonStep{}, Before: []string{}, Calls: []mock.Call{}, Screen: []string{}}
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
		x.Before, x.Calls, x.Crash, x.Saved = list(e.Before), list(e.Calls), e.Crash, e.Saved
		x.Screen = linesOf(e.Screen)
	}
	return x
}

func (o *outcome) json() jsonFlow {
	f := jsonFlow{Name: o.flow.Name, File: filepath.ToSlash(o.flow.Path), Status: o.status, Cause: o.cause(), Seconds: seconds(o.took()),
		Tries: len(o.tries), TimesFailed: o.timesFailed(), Retries: o.retries, NotRunReason: o.why, Warnings: list(o.Warnings), Recordings: []string{}, Failures: []jsonFailure{}}
	for i, t := range o.tries {
		f.Recordings = append(f.Recordings, t.Recordings...)
		if t.Err == nil || o.status == interrupted {
			continue
		}
		x := failureOf(t.Err, t.Failure)
		x.Try = i + 1
		f.Failures = append(f.Failures, x)
	}
	return f
}

func (s *summary) json(failuresOnly bool) ([]byte, error) {
	r := jsonRun{Version: resultsVersion, Run: s.title, Device: s.device, Began: s.began.UTC().Format("2006-01-02T15:04:05Z"),
		Seconds: seconds(s.took), Order: s.order, Stopped: s.stopped, OK: s.err() == nil, Total: len(s.outcomes), Passed: s.count(passed),
		Failed: s.count(failed), Interrupted: s.count(interrupted), NotRun: s.count(notRun), Retried: s.retried(), Flows: []jsonFlow{}}
	for _, o := range s.outcomes {
		if !failuresOnly || o.status != passed || o.timesFailed() > 0 {
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
