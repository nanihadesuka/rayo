package report

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nanihadesuka/rayo/internal/flow"
)

// FlowState is how a flow went the last time it ran, in whichever run that was.
type FlowState struct {
	Name   string   `json:"name" about:"The flow's"`
	File   string   `json:"file" about:"The flow's, as given"`
	Status Status   `json:"status" about:"Of its last run: one of the [outcomes](#outcomes), or NEVER if it never ran into this folder" enum:"PASS,FLAKY,PARTIAL,FAIL,NEVER"`
	Run    string   `json:"run" about:"The id of that run; empty if it never ran"`
	At     string   `json:"at" about:"Of a failure, the step that failed, file:line"`
	Files  []string `json:"files" about:"The files that run wrote for it and are kept, relative to the output folder, from files.json"`
}

// SuiteState is what rayo status prints, --format json.
type SuiteState struct {
	SchemaVersion int         `json:"schemaVersion" about:"Changes if the meaning of a key does"`
	Output        string      `json:"output" about:"The folder it was read from"`
	Green         bool        `json:"green" about:"Whether every flow passed whole the last time it ran: PASS or FLAKY"`
	Passed        int         `json:"passed" about:"Flows whose last run was PASS or FLAKY"`
	Total         int         `json:"total" about:"Flows of the suite, or those chosen"`
	Runs          []string    `json:"runs" about:"The ids of the runs the flows' states come from, oldest first"`
	Running       *Lock       `json:"running" about:"The run that holds the folder now, or one that was killed and left its lock; null if none"`
	Flows         []FlowState `json:"flows" about:"Each flow, in the suite's order"`
}

// Never is the state of a flow that never ran into the folder.
const Never Status = "NEVER"

// State merges the newest run of each flow from the history: so a suite fixed with partial runs is known
// green without running it all again.
func State(output string, flows []*flow.Flow) SuiteState {
	history := History(HistoryFile(output))
	files := ReadManifest(output)
	st := SuiteState{SchemaVersion: SchemaVersion, Output: filepath.ToSlash(output), Total: len(flows), Running: ReadLock(output), Flows: []FlowState{}, Runs: []string{}}
	for _, f := range flows {
		id := Identity(f)
		fs := FlowState{Name: f.Name, File: filepath.ToSlash(f.Path), Status: Never, Files: []string{}}
		for i := len(history) - 1; i >= 0; i-- {
			if r := history[i]; r.Flow == id {
				fs.Status, fs.Run, fs.At = r.Status, r.Run, r.At
				break
			}
		}
		if w, there := files[id]; there && w.Run == fs.Run {
			fs.Files = w.Files
		}
		if fs.Status == Passed || fs.Status == Flaky {
			st.Passed++
		}
		if fs.Run != "" && !containsString(st.Runs, fs.Run) {
			st.Runs = append(st.Runs, fs.Run)
		}
		st.Flows = append(st.Flows, fs)
	}
	sortStrings(st.Runs)
	st.Green = st.Total > 0 && st.Passed == st.Total
	return st
}

// Text is the state for a person.
func (st SuiteState) Text() string {
	var b strings.Builder
	width := 0
	for _, f := range st.Flows {
		width = max(width, len(f.Name))
	}
	for _, f := range st.Flows {
		line := fmt.Sprintf("  %-7s %-*s", f.Status, width, f.Name)
		if f.Run != "" {
			line += "  run " + f.Run
		}
		if f.At != "" {
			line += "  at " + f.At
		}
		if f.Status == Partial {
			line += "  (only part of it ran: run it whole)"
		}
		b.WriteString(strings.TrimRight(line, " ") + "\n")
	}
	verdict := "not green"
	if st.Green {
		verdict = "green"
	}
	fmt.Fprintf(&b, "%d of %d passed the last time they ran, from %d run(s): %s\n", st.Passed, st.Total, len(st.Runs), verdict)
	if l := st.Running; l != nil {
		fmt.Fprintf(&b, "%s holds %s: run %s, pid %d; if no rayo is running, it was killed\n", "running.json", st.Output, cmpOr(l.Run, "?"), l.Pid)
	}
	return b.String()
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func sortStrings(list []string) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && list[j] < list[j-1]; j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}
