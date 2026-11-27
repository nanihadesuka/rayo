package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nanihadesuka/rayo/internal/flow"
)

// LastFile is where the flows that haven't passed are kept.
func LastFile(output string) string { return filepath.Join(output, "failed.json") }

// Identity tells flows apart in different runs.
func Identity(f *flow.Flow) string {
	path := f.Path
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return filepath.ToSlash(path)
}

// Unpassed is a flow that hasn't passed since it last failed: how it went, and its last failure, for rayo explain.
type Unpassed struct {
	Status  Status   `json:"status" about:"One of the [outcomes](#outcomes)" enum:"FAIL,PARTIAL,INTERRUPTED,NOT_RUN"`
	Name    string   `json:"name" about:"The flow's"`
	File    string   `json:"file" about:"The flow's, as given"`
	Why     string   `json:"why" about:"Of NOT_RUN, why it didn't run"`
	Run     string   `json:"run" about:"The id of the run it was last in, as the results' run; empty if written by an older rayo"`
	Mark    *Mark    `json:"mark" about:"What was said of it while held, with .mark; null if nothing"`
	History string   `json:"history" about:"Of rayo explain, how its last runs went, from history.jsonl, such as 'failed 2 of the last 5 runs, at login.yaml:12 each time'; empty if none are known"`
	Failure *Failure `json:"failure" about:"Of its last try that failed, as in the [results](#results); null if none"`
}

// Last is the flows that haven't passed since they last failed, by Identity.
func Last(file string) (map[string]Unpassed, error) {
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Unpassed{}, nil
	}
	var last map[string]Unpassed
	if err == nil {
		last, err = ParseLast(data)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.ToSlash(file), err)
	}
	return last, nil
}

// ParseLast reads what a failed.json has.
func ParseLast(data []byte) (map[string]Unpassed, error) {
	last := map[string]Unpassed{}
	if err := json.Unmarshal(data, &last); err != nil { // as it was written before: a status each
		old := map[string]Status{}
		if json.Unmarshal(data, &old) != nil {
			return nil, err
		}
		for k, s := range old {
			last[k] = Unpassed{Status: s}
		}
	}
	return last, nil
}

// SaveLast adds this run to it.
func (s *Summary) SaveLast(file string) error {
	last, err := Last(file)
	if err != nil {
		last = map[string]Unpassed{} // unreadable, so written again
	}
	before := len(last)
	for _, o := range s.Outcomes {
		if o.Status == Partial { // from a step on, it may still fail before it: kept, so that it runs whole
			u, there := last[Identity(o.Flow)]
			if !there {
				u = Unpassed{Name: o.Flow.Name, File: filepath.ToSlash(o.Flow.Path)}
			}
			u.Status, u.Run, u.Mark = Partial, s.Run, o.Mark
			last[Identity(o.Flow)] = u
			continue
		}
		if o.Passed() {
			delete(last, Identity(o.Flow))
			continue
		}
		u := Unpassed{Status: o.Status, Name: o.Flow.Name, File: filepath.ToSlash(o.Flow.Path), Why: o.Why, Run: s.Run, Mark: o.Mark}
		if j := o.json(); len(j.Failures) > 0 {
			u.Failure = &j.Failures[len(j.Failures)-1]
			u.Failure.Screen = []string{} // it is in screen.txt, and long
		}
		last[Identity(o.Flow)] = u
	}
	if len(last) == 0 {
		if before > 0 || err != nil {
			return os.Remove(file)
		}
		return nil
	}
	data, err := Encode(last, true)
	if err != nil {
		return err
	}
	return WriteFile(file, data)
}

// Latest keeps only the flows of the newest run there is among them; all, if none says its run.
func Latest(last map[string]Unpassed) map[string]Unpassed {
	newest := ""
	for _, u := range last {
		newest = max(newest, u.Run) // ids begin with the time
	}
	if newest == "" {
		return last
	}
	kept := map[string]Unpassed{}
	for k, u := range last {
		if u.Run == newest {
			kept[k] = u
		}
	}
	return kept
}

// Explained is what rayo explain prints, --format json.
type Explained struct {
	SchemaVersion int        `json:"schemaVersion" about:"Changes if the meaning of a key does"`
	Flows         []Unpassed `json:"flows" about:"Those that haven't passed since they last failed, see [Explain](#explain)"`
}

// Explain is a flow that hasn't passed, in a few lines: the verdict, why, where, the nearest elements and the calls.
func (u Unpassed) Explain() string {
	f := u.Failure
	switch {
	case u.Status == NotRun:
		return fmt.Sprintf("%s didn't run: %s\n", u.Name, u.Why)
	case u.Status == Partial:
		text := fmt.Sprintf("%s passed only in part: it began at a step, or was fixed while held\n  next: run it whole, rayo test --rerun-failed <the suite or flow you ran>\n", u.Name)
		if u.Mark != nil {
			text += "  marked: " + u.Mark.As + ": " + u.Mark.Note + "\n"
		}
		return text
	case f == nil:
		return fmt.Sprintf("%s: %s\n", u.Name, u.Status)
	}
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
	b.WriteString(f.Verdict + "\n")
	add("why", f.Error.Message)
	if f.Error.File != "" {
		add("at", fmt.Sprintf("%s:%d", f.Error.File, f.Error.Line))
	}
	for _, from := range f.From {
		add("from", fmt.Sprintf("%s:%d %s", from.File, from.Line, from.Step))
	}
	if f.Because != "" {
		add("because", f.Because)
	}
	add("crash", f.Crash)
	list("nearest", f.Nearest)
	var states []string
	for _, sc := range f.Scenarios {
		states = append(states, sc.Name+": "+sc.State)
	}
	list("scenarios", states)
	var calls []string
	for _, c := range f.Calls {
		calls = append(calls, c.String())
	}
	list("calls", calls)
	add("saved", f.Saved)
	if f.Blame == flow.Test && (f.Cause == flow.NotOnScreen || f.Cause == flow.UnmockedCall) {
		add("rerun", "unchanged it fails the same way: fix the flow or the mocks first")
	}
	if u.Mark != nil {
		add("marked", u.Mark.As+": "+u.Mark.Note)
	}
	add("history", u.History)
	next := f.Next
	if next == nil { // written by an older rayo
		next = f.next()
	}
	list("next", next)
	return b.String()
}
