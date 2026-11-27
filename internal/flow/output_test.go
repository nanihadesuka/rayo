package flow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileNameRun(t *testing.T) {
	a := Attempt{Out: "out", Flow: "login", Try: 1, Run: "20261127T100000-ab12"}
	if got := filepath.ToSlash(FileName("{run}/{flow}_{attempt}_{kind}.{ext}", a, "logcat", "txt")); got != "out/20261127T100000-ab12/login_retry1_logcat.txt" {
		t.Error(got)
	}
	if got := filepath.ToSlash(FileName("{flow}_{run}_{attempt}_{kind}.{ext}", a, "logcat", "txt")); got != "out/login_20261127T100000-ab12_retry1_logcat.txt" {
		t.Error(got)
	}
	if got := filepath.ToSlash(FileName("", a, "logcat", "txt")); got != "out/login_retry1_logcat.txt" {
		t.Errorf("the default has no run: %s", got)
	}
	a.Run, a.Try = "", 0
	if got := filepath.ToSlash(FileName("{flow}_{run}_{attempt}_{kind}.{ext}", a, "logcat", "txt")); got != "out/login_logcat.txt" {
		t.Errorf("an empty run leaves no separators: %s", got)
	}
}

func TestRecordsElsewhere(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	write("task.yaml", "appId: a.b\n---\n- startRecording: seeding\n- stopRecording\n")
	for _, c := range []struct {
		text               string
		elsewhere, records bool
	}{
		{"appId: a.b\n---\n- tapOn: x\n", false, false},
		{"appId: a.b\n---\n- startRecording\n- stopRecording\n", false, true},
		{"appId: a.b\n---\n- startRecording: mine\n", true, true},
		{"appId: a.b\nrecord: true\n---\n- tapOn: x\n", false, true},
		{"appId: a.b\n---\n- runFlow: task.yaml\n", true, true},
	} {
		f, err := Load(write("f.yaml", c.text))
		if err != nil {
			t.Fatal(err)
		}
		if f.RecordsElsewhere() != c.elsewhere || f.Records() != c.records {
			t.Errorf("%q: elsewhere %v, records %v", c.text, f.RecordsElsewhere(), f.Records())
		}
	}
}
