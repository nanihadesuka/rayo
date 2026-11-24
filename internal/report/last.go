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
	Status  Status   `json:"status" about:"One of the [outcomes](#outcomes)" enum:"FAIL,INTERRUPTED,NOT_RUN"`
	Name    string   `json:"name" about:"The flow's"`
	File    string   `json:"file" about:"The flow's, as given"`
	Why     string   `json:"why" about:"Of NOT_RUN, why it didn't run"`
	Failure *Failure `json:"failure" about:"Of its last try that failed, as in the [results](#results); null if none"`
}

// Last is the flows that haven't passed since they last failed, by Identity.
func Last(file string) (map[string]Unpassed, error) {
	last := map[string]Unpassed{}
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return last, nil
	}
	if err == nil {
		if err = json.Unmarshal(data, &last); err != nil { // as it was written before: a status each
			old := map[string]Status{}
			if json.Unmarshal(data, &old) == nil {
				for k, s := range old {
					last[k] = Unpassed{Status: s}
				}
				err = nil
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.ToSlash(file), err)
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
		if o.Passed() {
			delete(last, Identity(o.Flow))
			continue
		}
		u := Unpassed{Status: o.Status, Name: o.Flow.Name, File: filepath.ToSlash(o.Flow.Path), Why: o.Why}
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
	var calls []string
	for _, c := range f.Calls {
		calls = append(calls, c.String())
	}
	list("calls", calls)
	add("saved", f.Saved)
	return b.String()
}
