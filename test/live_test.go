package test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const liveMock = `{"request":{"urlPath":"/a"},"response":{"body":"ok"}}`

// Commands typed into a session run, and those that pass are saved.
func TestLiveTyped(t *testing.T) {
	dir := t.TempDir()
	d := startDevice(t, "emulator-5554\tdevice\n", false)
	cmd := d.command(dir, "live", "--app", "a.b", "flows/new.yaml")
	cmd.Stdin = strings.NewReader("- tapOn: one\n" +
		"tapOn: { id: missing }\n" +
		"- extendedWaitUntil:\n    visible: { id: home }\n\n" +
		"- tapOm: x\n" +
		"- setLocation: { latitude: 1, longitude: 2 }\n" +
		"- back\n.undo\n.steps\n.save\n.nothing\n- tapOn: late\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for _, want := range []string{"✓ tapOn one", "✗ tapOn missing", "not on screen: missing", "  a screen", `unknown command "tapOm"`, "forgot - back", "saved 3 steps in flows/new.yaml", "unknown .nothing"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	got, _ := os.ReadFile(filepath.Join(dir, "flows", "new.yaml"))
	want := "appId: a.b\n---\n- tapOn: one\n- extendedWaitUntil:\n    visible: {id: home}\n- setLocation: {latitude: 1, longitude: 2}\n"
	if string(got) != want {
		t.Errorf("saved\n%s", got)
	}
	if out, err := d.command(dir, "check", "flows/new.yaml").CombinedOutput(); err != nil {
		t.Errorf("%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".rayo", "live.json")); err == nil {
		t.Error("left live.json")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if !strings.Contains(strings.Join(d.log, "\n"), "resetLocation") {
		t.Error("didn't reset the location")
	}
}

// Other processes use a session that is served, going on from a flow with its mock.
func TestLiveServed(t *testing.T) {
	dir := t.TempDir()
	const before = "appId: a.b\nmock: mock.json\n---\n# as written\n- tapOn: first\n"
	write(t, filepath.Join(dir, "flow.yaml"), before)
	write(t, filepath.Join(dir, "mock.json"), liveMock)
	d := startDevice(t, "emulator-5554\tdevice\n", false)
	if out, err := d.command(dir, "do", "- back").CombinedOutput(); err == nil || !strings.Contains(string(out), "no session is live") {
		t.Fatalf("%v\n%s", err, out)
	}
	server := d.command(dir, "live", "--serve", "flow.yaml")
	var printed bytes.Buffer
	server.Stdout, server.Stderr = &printed, &printed
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	ended := make(chan error, 1)
	go func() { ended <- server.Wait() }()
	defer func() { _ = server.Process.Kill() }()
	door := filepath.Join(dir, ".rayo", "live.json")
	for limit := time.Now().Add(10 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if _, err := os.Stat(door); err == nil {
			break
		}
		if time.Now().After(limit) {
			t.Fatal("it didn't start")
		}
	}
	do := func(code int, args ...string) string {
		t.Helper()
		cmd := d.command(dir, append([]string{"do"}, args...)...)
		out, _ := cmd.CombinedOutput()
		if cmd.ProcessState.ExitCode() != code {
			t.Errorf("%v: exit %d\n%s", args, cmd.ProcessState.ExitCode(), out)
		}
		return string(out)
	}
	has := func(out string, wants ...string) {
		t.Helper()
		for _, w := range wants {
			if !strings.Contains(out, w) {
				t.Errorf("no %q in\n%s", w, out)
			}
		}
	}
	has(do(0, "--screen", "- tapOn: { id: GET /a }\n- tapOn: { id: GET /no }"), "✓ tapOn GET /a", "isn't mocked: GET /no", "  a screen")
	var a struct {
		OK            bool
		Failure       struct{ Error struct{ Cause, Step string } }
		SchemaVersion int
		Calls         []any
		Screen        []string
		Unsaved       int
	}
	out := do(1, "--format", "json", "- back\n- tapOn: { id: missing }\n- tapOn: never")
	if err := json.Unmarshal([]byte(out), &a); err != nil || a.OK || a.Failure.Error.Cause != "not on screen: missing" || a.SchemaVersion != 2 || a.Unsaved != 3 {
		t.Errorf("%v %+v\n%s", err, a, out)
	}
	has(do(1, "- tapOm: x"), "unknown command")
	has(do(0, ".screen"), "a screen")
	has(do(0, ".save"), "saved 3 steps")
	second := d.command(dir, "live", "flow.yaml")
	if out, err := second.CombinedOutput(); err == nil || !strings.Contains(string(out), "live already") {
		t.Errorf("%v\n%s", err, out)
	}
	do(0, ".quit")
	select {
	case err := <-ended:
		if err != nil {
			t.Errorf("%v\n%s", err, printed.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("it didn't end")
	}
	has(printed.String(), "✓ tapOn first")
	got, _ := os.ReadFile(filepath.Join(dir, "flow.yaml"))
	if string(got) != before+"- tapOn: {id: GET /a}\n- tapOn: {id: GET /no}\n- back\n" {
		t.Errorf("saved\n%s", got)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.forwards) > 0 {
		t.Error("left the port forwarded")
	}
}
