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

// A run that holds a flow where it failed: it is fixed in its own file, run again from there, and goes
// on with the next flow; the fixed one is a partial pass, and the files say so.
func TestHold(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "all.suite.yaml"), "flows: [a.yaml, b.yaml, c.yaml]\noutput: results\nenv: { WHO: me }\n")
	write(t, filepath.Join(dir, "a.yaml"), "appId: a.b\nonFlowComplete:\n  - tapOn: done\n---\n- tapOn: first\n- tapOn: { id: missing }\n- assertVisible: last\n")
	write(t, filepath.Join(dir, "b.yaml"), "appId: a.b\n---\n- tapOn: ${WHO}\n- tapOn: { id: missing }\n")
	write(t, filepath.Join(dir, "c.yaml"), "appId: a.b\n---\n- tapOn: { id: missing }\n")
	d := startDevice(t, "emulator-5554\tdevice\n", false)
	run := d.command(dir, "test", "--on-fail", "hold", "all.suite.yaml")
	var printed bytes.Buffer
	run.Stdout, run.Stderr = &printed, &printed
	if err := run.Start(); err != nil {
		t.Fatal(err)
	}
	ended := make(chan error, 1)
	go func() { ended <- run.Wait() }()
	defer func() { _ = run.Process.Kill() }()
	door := filepath.Join(dir, "results", "live.json")
	await := func(gone bool) {
		t.Helper()
		for limit := time.Now().Add(15 * time.Second); ; time.Sleep(20 * time.Millisecond) {
			if _, err := os.Stat(door); (err == nil) != gone {
				return
			}
			if time.Now().After(limit) {
				t.Fatalf("no hold\n%s", printed.String())
			}
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

	await(false)
	has(do(0, ".why"), "a failed at tapOn missing: not on screen", "at: a.yaml:6", "nor anything like it")
	var a struct {
		OK   bool
		Held struct {
			Flow, At string
			Fixed    bool
		}
	}
	if out := do(0, "--format", "json", ".steps"); json.Unmarshal([]byte(out), &a) != nil || a.Held.Flow != "a" || a.Held.At != "a.yaml:6" || a.Held.Fixed {
		t.Errorf("%+v\n%s", a, out)
	}
	has(do(0, ".steps"), "1. a.yaml:5  tapOn first", "failed at a.yaml:6")
	has(do(1, ".continue"), ".continue goes on once the flow's steps pass")
	has(do(0, ".probe 2"), "0 of 2 passed", "it always fails as it is")
	has(do(0, ".diff"), "the screen is as it was")
	has(do(0, ".find", "a"), "a screen") // its words as one command
	has(do(1, ".retry"), "not on screen: missing")
	has(do(1, ".nope"), "unknown .nope")
	// A line above it, and the step itself changed: it is found by its place among the file's steps.
	write(t, filepath.Join(dir, "a.yaml"), "appId: a.b\nonFlowComplete:\n  - tapOn: done\n---\n# the fix\n- tapOn: first\n- tapOn: { id: there }\n- assertVisible: last\n")
	has(do(0, ".retry"), "the step that failed, tapOn missing, is at a.yaml:7 now", "✓ tapOn there", "✓ assertVisible last", "passed from a.yaml:7")
	if out := do(0, "--format", "json", ".why"); json.Unmarshal([]byte(out), &a) != nil || !a.Held.Fixed {
		t.Errorf("%+v\n%s", a, out)
	}
	has(do(0, ".continue"), "going on")

	await(true)
	await(false)
	has(do(0, ".why"), "b failed at tapOn missing", "at: b.yaml:4")
	has(do(1, ".mark bug"), ".mark takes app-bug, test-bug, flaky")
	has(do(0, ".mark app-bug it never shows"), "marked app-bug")
	has(do(0, ".skip"), "left failed")

	await(true)
	await(false)
	has(do(0, ".abort"), "no more flows run")
	select {
	case err := <-ended:
		if err == nil {
			t.Errorf("it passed\n%s", printed.String())
		}
	case <-time.After(15 * time.Second):
		t.Fatalf("it didn't end\n%s", printed.String())
	}
	has(printed.String(), "HELD a", "PARTIAL a", "FAIL b", "FAIL c", "NOT WHOLE: 1 flow ran only in part", "STOPPED aborted while c was held")
	d.mu.Lock()
	has(strings.Join(d.log, "\n"), `"done"`) // onFlowComplete, once fixed
	d.mu.Unlock()
	last, _ := os.ReadFile(filepath.Join(dir, "results", "failed.json"))
	has(string(last), `"status": "PARTIAL"`, `"as": "app-bug"`, `"note": "it never shows"`)
	if _, err := os.Stat(door); err == nil {
		t.Error("left live.json")
	}
	cmd := d.command(dir, "explain", "--latest", "b")
	out, _ := cmd.CombinedOutput()
	has(string(out), "(from results, the output of all.suite.yaml)", "marked: app-bug: it never shows")
}

// A fix that checks less than the flow did as it failed is told: WEAKENED.
func TestHoldWeakened(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.yaml"), "appId: a.b\n---\n- tapOn: first\n- assertVisible: { id: missing }\n- assertVisible: last\n")
	d := startDevice(t, "emulator-5554\tdevice\n", false)
	run := d.command(dir, "test", "--on-fail", "hold", "a.yaml")
	var printed bytes.Buffer
	run.Stdout, run.Stderr = &printed, &printed
	if err := run.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = run.Process.Kill() }()
	door := filepath.Join(dir, ".rayo", "live.json")
	for limit := time.Now().Add(15 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if _, err := os.Stat(door); err == nil {
			break
		}
		if time.Now().After(limit) {
			t.Fatalf("no hold\n%s", printed.String())
		}
	}
	write(t, filepath.Join(dir, "a.yaml"), "appId: a.b\n---\n- tapOn: first\n- assertVisible: last\n")
	if out, err := d.command(dir, "do", ".from a.yaml:4").CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	out, _ := d.command(dir, "do", ".continue").CombinedOutput()
	if !strings.Contains(string(out), "WEAKENED") {
		t.Errorf("%s", out)
	}
	if err := run.Wait(); err != nil {
		t.Errorf("%v\n%s", err, printed.String())
	}
	if !strings.Contains(printed.String(), "PARTIAL") || !strings.Contains(printed.String(), "WEAKENED") {
		t.Errorf("%s", printed.String())
	}
}
