package report

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nanihadesuka/rayo/internal/flow"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func TestManifestNoteKeepsOnlyFilesThatAreThere(t *testing.T) {
	out := t.TempDir()
	touch(t, filepath.Join(out, "a_failure.png"))
	m := Manifest{}
	m.Note(out, "id", "a", "a.yaml", "r1", []string{filepath.Join(out, "a_failure.png"), filepath.Join(out, "gone.txt"), filepath.Join(out, "a_failure.png")})
	if w := m["id"]; w.Run != "r1" || !slices.Equal(w.Files, []string{"a_failure.png"}) {
		t.Errorf("%+v", w)
	}
	m.Note(out, "none", "b", "b.yaml", "r1", nil)
	if _, there := m["none"]; there {
		t.Error("a flow that wrote nothing is listed")
	}
}

func TestManifestOlderRunsAreKeptUntilCleaned(t *testing.T) {
	out := t.TempDir()
	old, now := filepath.Join(out, "a", "old.mp4"), filepath.Join(out, "a_new.mp4")
	touch(t, old)
	touch(t, now)
	m := Manifest{}
	m.Note(out, "id", "a", "a.yaml", "r1", []string{old})
	m.Note(out, "id", "a", "a.yaml", "r2", []string{now})
	w := m["id"]
	if w.Run != "r2" || !slices.Equal(w.Files, []string{"a_new.mp4"}) || !slices.Equal(w.Older, []string{"a/old.mp4"}) {
		t.Fatalf("%+v", w)
	}
	other := filepath.Join(out, "b_failure.png")
	touch(t, other)
	m.Note(out, "b", "b", "b.yaml", "r2", []string{other})
	if err := m.Clean(out, "id"); err != nil {
		t.Fatal(err)
	}
	if exists(old) || exists(now) || exists(filepath.Join(out, "a")) {
		t.Error("files of the flow, or the folder they left empty, are still there")
	}
	if !exists(other) {
		t.Error("another flow's file was removed")
	}
	if _, there := m["id"]; there {
		t.Error("still listed")
	}
}

func TestManifestSaveReadAndCleanAll(t *testing.T) {
	out := t.TempDir()
	a, b := filepath.Join(out, "a.png"), filepath.Join(out, "b.png")
	touch(t, a)
	touch(t, b)
	m := Manifest{}
	m.Note(out, "a", "a", "a.yaml", "r", []string{a})
	m.Note(out, "b", "b", "b.yaml", "r", []string{b})
	if err := m.Save(out); err != nil {
		t.Fatal(err)
	}
	read := ReadManifest(out)
	if len(read) != 2 || read["a"].Files[0] != "a.png" {
		t.Fatalf("%+v", read)
	}
	if err := read.CleanAllFiles(out); err != nil {
		t.Fatal(err)
	}
	if err := read.Save(out); err != nil {
		t.Fatal(err)
	}
	if exists(a) || exists(b) || exists(FilesFile(out)) {
		t.Error("files, or files.json, are still there")
	}
}

func TestReadManifestOfGarbage(t *testing.T) {
	out := t.TempDir()
	touch(t, FilesFile(out))
	if m := ReadManifest(out); len(m) != 0 {
		t.Errorf("%+v", m)
	}
}

func TestTakeAndRelease(t *testing.T) {
	out := t.TempDir()
	never := func(int) bool { return false }
	always := func(int) bool { return true }
	if stale, err := Take(out, Lock{Run: "r1", Device: "emulator-5554"}, never, false); err != nil || stale != nil {
		t.Fatalf("%v %v", stale, err)
	}
	l := ReadLock(out)
	if l == nil || l.Pid != os.Getpid() || l.Run != "r1" || l.Began == "" {
		t.Fatalf("%+v", l)
	}
	Release(out)
	if exists(LockFile(out)) {
		t.Fatal("not released")
	}

	other, _ := json.Marshal(Lock{Run: "r0", Pid: os.Getpid() + 1, Device: "d"})
	write := func(data []byte) {
		if err := os.WriteFile(LockFile(out), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(other)
	_, err := Take(out, Lock{Run: "r1"}, always, false)
	if err == nil || !strings.Contains(err.Error(), "pid") || !strings.Contains(err.Error(), "r0") || !strings.Contains(err.Error(), "--ignore-lock") {
		t.Fatalf("a running one isn't refused: %v", err)
	}
	stale, err := Take(out, Lock{Run: "r1"}, always, true)
	if err != nil || stale == nil || stale.Run != "r0" {
		t.Fatalf("--ignore-lock: %v %v", stale, err)
	}
	write(other)
	stale, err = Take(out, Lock{Run: "r1"}, never, false)
	if err != nil || stale == nil || stale.Pid != os.Getpid()+1 {
		t.Fatalf("one that is gone: %v %v", stale, err)
	}

	write(other)
	Release(out)
	if !exists(LockFile(out)) {
		t.Error("released another's lock")
	}
	write([]byte("{half"))
	if stale, err := Take(out, Lock{Run: "r2"}, always, false); err != nil || stale == nil {
		t.Errorf("half written: %v %v", stale, err)
	}
}

func TestSignature(t *testing.T) {
	step := &flow.Step{Path: filepath.Join("e2e", "login.yaml"), Line: 3}
	fail := func(msg string) Outcome {
		o := Outcome{Flow: &flow.Flow{Name: "f"}}
		o.Judge(t0(), flow.Result{Err: &flow.StepError{Step: step, Err: errors.New(msg)}, Failure: &flow.Failure{}})
		return o
	}
	a, b, c := fail("not on screen: x"), fail("not on screen:   x"), fail("other")
	if a.Signature() == "" || a.Signature() != b.Signature() || a.Signature() == c.Signature() {
		t.Errorf("%q %q %q", a.Signature(), b.Signature(), c.Signature())
	}
	passed := Outcome{Flow: &flow.Flow{Name: "f"}}
	passed.Judge(t0(), flow.Result{})
	if passed.Signature() != "" {
		t.Error("a pass has one")
	}
}

func TestUnfinished(t *testing.T) {
	s := &Summary{Run: "r", Outcomes: []Outcome{{Flow: &flow.Flow{Name: "a", Path: "a.yaml"}}, {Flow: &flow.Flow{Name: "b", Path: "b.yaml"}}}}
	u := s.Unfinished()
	if u.Exit != ExitInterrupted || u.Stopped == "" || len(u.Outcomes) != 2 || u.Outcomes[1].Status != Interrupted {
		t.Fatalf("%+v", u)
	}
	if s.Outcomes[0].Status != "" {
		t.Error("changed the run's own")
	}
	data, err := u.json()
	if err != nil || !strings.Contains(string(data), `"INTERRUPTED"`) {
		t.Errorf("%v %s", err, data)
	}
}

func TestState(t *testing.T) {
	out := t.TempDir()
	a, b, c := &flow.Flow{Name: "a", Path: "a.yaml"}, &flow.Flow{Name: "b", Path: "b.yaml"}, &flow.Flow{Name: "c", Path: "c.yaml"}
	run := func(id string, err error, flows ...*flow.Flow) {
		s := &Summary{Run: id}
		for _, f := range flows {
			o := Outcome{Flow: f}
			o.Judge(t0(), flow.Result{Err: err})
			s.Outcomes = append(s.Outcomes, o)
		}
		if err := s.SaveHistory(HistoryFile(out)); err != nil {
			t.Fatal(err)
		}
	}
	run("20260101T000000-0001", errors.New("boom"), a, b)
	st := State(out, []*flow.Flow{a, b, c})
	if st.Green || st.Passed != 0 || st.Flows[2].Status != Never {
		t.Fatalf("%+v", st)
	}
	run("20260101T010000-0002", nil, a, b)
	st = State(out, []*flow.Flow{a, b})
	if !st.Green || st.Passed != 2 || !slices.Equal(st.Runs, []string{"20260101T010000-0002"}) {
		t.Fatalf("%+v", st)
	}
	run("20260101T020000-0003", errors.New("boom"), b) // only b again: a stays green from the run before
	st = State(out, []*flow.Flow{a, b})
	if st.Green || st.Passed != 1 || st.Flows[0].Run != "20260101T010000-0002" || st.Flows[1].Status != Failed || len(st.Runs) != 2 {
		t.Fatalf("%+v", st)
	}
	if text := st.Text(); !strings.Contains(text, "1 of 2 passed") || !strings.Contains(text, "not green") {
		t.Error(text)
	}
}
