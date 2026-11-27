package report

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HistoryFile is where each run adds a line for each flow that ran, so that a failure can be told a
// flake or new; kept at historyMost lines.
func HistoryFile(output string) string { return filepath.Join(output, "history.jsonl") }

const historyMost = 5000

// Ran is a flow in a run, as history.jsonl keeps it.
type Ran struct {
	Run    string `json:"run" about:"The run's id"`
	Flow   string `json:"flow" about:"Its file, absolute, as failed.json tells flows apart"`
	Name   string `json:"name" about:"The flow's"`
	Status Status `json:"status" about:"Its outcome"`
	At     string `json:"at" about:"Of a failure, the step that failed, file:line; empty if it passed"`
}

// SaveHistory adds this run's flows to the history, those that ran.
func (s *Summary) SaveHistory(file string) error {
	var add bytes.Buffer
	for _, o := range s.Outcomes {
		if o.Status == NotRun || o.Status == Interrupted {
			continue
		}
		r := Ran{Run: s.Run, Flow: Identity(o.Flow), Name: o.Flow.Name, Status: o.Status}
		if !o.Passed() {
			_, r.At, _, _, _ = o.failing()
		}
		line, err := json.Marshal(r)
		if err != nil {
			return err
		}
		add.Write(append(line, '\n'))
	}
	if add.Len() == 0 {
		return nil
	}
	old, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lines := strings.SplitAfter(string(old)+add.String(), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	lines = lines[max(0, len(lines)-historyMost):]
	return WriteFile(file, []byte(strings.Join(lines, "")))
}

// History is what a history file has, oldest first; none if there is none.
func History(file string) []Ran {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	return ParseHistory(data)
}

// ParseHistory reads what a history file has; a line it can't read is left out.
func ParseHistory(data []byte) []Ran {
	var list []Ran
	lines := bufio.NewScanner(bytes.NewReader(data))
	lines.Buffer(nil, 1<<20)
	for lines.Scan() {
		var r Ran
		if json.Unmarshal(lines.Bytes(), &r) == nil {
			list = append(list, r)
		}
	}
	return list
}

// historyRuns is how many of a flow's last runs a summary of its history looks at.
const historyRuns = 10

// Told is a flow's last runs in a line, such as "failed 2 of its last 5 runs, at login.yaml:12 each
// time; passed the 3 before"; empty if it ran only once, or never.
func Told(history []Ran, identity string) string {
	var mine []Ran
	for _, r := range history {
		if r.Flow == identity {
			mine = append(mine, r)
		}
	}
	mine = mine[max(0, len(mine)-historyRuns):]
	if len(mine) < 2 {
		return ""
	}
	failed, places := 0, map[string]bool{}
	for _, r := range mine {
		if r.Status == Failed {
			failed++
			places[r.At] = true
		}
	}
	text := fmt.Sprintf("failed %d of its last %d runs", failed, len(mine))
	if failed > 1 && len(places) == 1 {
		text += ", at the same step each time"
	}
	switch {
	case failed == len(mine):
		text += ": it fails every time, so it isn't a flake"
	case failed == 1 && mine[len(mine)-1].Status == Failed:
		text += ": only the last, so something changed since it passed"
	case failed > 1:
		text += ": it passes at times, so it may be flaky; a wait, or what an earlier flow left, is the usual reason"
	}
	return text
}
