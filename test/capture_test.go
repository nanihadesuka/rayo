package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// What was captured from a real backend, mocked, answers the same to the same flow.
func TestCaptureReplay(t *testing.T) {
	reads, ride, logins := 0, "none", 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch q.Method + " " + q.URL.Path {
		case "POST /login":
			if logins++; logins == 1 {
				w.WriteHeader(401)
			}
			fmt.Fprintf(w, `{"n":%d}`, logins)
		case "POST /book":
			ride = "booked"
			fmt.Fprint(w, `{"ok":true}`)
		case "POST /ping":
			fmt.Fprint(w, `{}`)
		case "GET /ride":
			if reads++; reads == 5 { // by itself
				ride = "docked"
			}
			fmt.Fprintf(w, `{"ride":%q,"text":"{{x}}"}`, ride)
		default:
			fmt.Fprint(w, `{"same":true}`)
		}
	}))
	defer s.Close()
	dir := t.TempDir()
	put := func(name, text string) { write(t, filepath.Join(dir, name), text) }
	calls := func(list ...string) string {
		var b strings.Builder
		for _, c := range list {
			fmt.Fprintf(&b, "- tapOn: { id: %q }\n", c)
		}
		return b.String()
	}
	before := calls("POST /login", "POST /login", "POST /ping", "GET /ride", "GET /same?a=1", "POST /ping", "POST /book", "POST /ping", "GET /ride", "GET /ride", "GET /ride")
	after := calls("GET /ride", "GET /same?a=1", "POST /ping")
	run := func(args ...string) []string {
		d := startDevice(t, "emulator-5554\tdevice\n", false)
		out, err := d.command(dir, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		d.mu.Lock()
		defer d.mu.Unlock()
		var answers []string
		for _, l := range d.log {
			if strings.HasPrefix(l, "  app: ") { // a capture is written with other spaces
				answers = append(answers, strings.Join(strings.Fields(l), ""))
			}
		}
		return answers
	}
	put("real.yaml", "appId: a.b\n---\n"+before+after)
	real := run("test", "--capture", "got", "--capture-from", s.URL, "real.yaml")
	put("mocked.yaml", "appId: a.b\nmock: got\n---\n"+before+"- mockState: { capture: 4 }\n"+after)
	mocked := run("test", "mocked.yaml")
	if len(real) != 14 || len(real) != len(mocked) {
		t.Fatalf("%d calls, then %d", len(real), len(mocked))
	}
	for i := range real {
		if want := strings.ReplaceAll(real[i], "{{x}}", "{\\u007bx}}"); mocked[i] != want {
			t.Errorf("call %d: %s\nwas %s", i+1, mocked[i], want)
		}
	}
}

// A live capture survives a failure: the failed step's calls are dropped, the file's steps are run
// again from there, a step is rewound, and what is saved on .quit is one capture, which another goes on with.
func TestCaptureLive(t *testing.T) {
	var mu sync.Mutex
	got := map[string]int{}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		mu.Lock()
		got[q.Method+" "+q.URL.Path]++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"path":%q}`, q.URL.Path)
	}))
	defer s.Close()
	dir := t.TempDir()
	const failing = "appId: a.b\n---\n- tapOn: { id: \"POST /login\" }\n- runFlow:\n    commands:\n" +
		"      - tapOn: { id: \"POST /order\" }\n      - tapOn: { id: missing }\n- tapOn: { id: \"GET /ride\" }\n"
	write(t, filepath.Join(dir, "flow.yaml"), failing)
	d := startDevice(t, "emulator-5554\tdevice\n", false)
	server := d.command(dir, "live", "--serve", "--capture", "got", "--capture-from", s.URL, "flow.yaml")
	var printed bytes.Buffer
	server.Stdout, server.Stderr = &printed, &printed
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	ended := make(chan error, 1)
	go func() { ended <- server.Wait() }()
	defer func() { _ = server.Process.Kill() }()
	for limit := time.Now().Add(10 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if _, err := os.Stat(filepath.Join(dir, ".rayo", "live.json")); err == nil {
			break
		}
		if time.Now().After(limit) {
			t.Fatalf("it didn't start\n%s", printed.String())
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
	write(t, filepath.Join(dir, "flow.yaml"), strings.Replace(failing, "      - tapOn: { id: missing }\n", "", 1))
	has(do(0, ".retry"), "✓ tapOn POST /order", "✓ tapOn GET /ride")
	do(1, ".retry")
	has(do(0, "- tapOn: { id: \"POST /extra\" }"), "✓ tapOn POST /extra")
	has(do(0, ".rewind"), "forgot - tapOn", "dropped 1 calls")
	has(do(1, ".rewind 9"), "only 3 steps")
	do(0, ".quit")
	select {
	case err := <-ended:
		if err != nil {
			t.Errorf("%v\n%s", err, printed.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("it didn't end")
	}
	has(printed.String(), "the 1 calls of the step that failed aren't kept", "captured 3 calls")
	kept := func() string {
		t.Helper()
		var k struct {
			Calls []struct{ Method, URI string }
		}
		data, err := os.ReadFile(filepath.Join(dir, "got", "capture.calls.json"))
		if err == nil {
			err = json.Unmarshal(data, &k)
		}
		if err != nil {
			t.Fatal(err)
		}
		var list []string
		for _, c := range k.Calls {
			list = append(list, c.Method+" "+c.URI)
		}
		return strings.Join(list, ", ")
	}
	if k := kept(); k != "POST /login, POST /order, GET /ride" {
		t.Errorf("kept %s", k)
	}
	mu.Lock()
	if got["POST /order"] != 2 || got["POST /extra"] != 1 {
		t.Errorf("the backend got %v", got)
	}
	mu.Unlock()

	write(t, filepath.Join(dir, "more.yaml"), "appId: a.b\n---\n- tapOn: { id: \"POST /pay\" }\n")
	if out, err := d.command(dir, "test", "--capture", "got", "more.yaml").CombinedOutput(); err == nil || !strings.Contains(string(out), "--capture-append") {
		t.Errorf("%v\n%s", err, out)
	}
	out, err := d.command(dir, "test", "--capture", "got", "--capture-append", "--capture-from", s.URL, "more.yaml").CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	has(string(out), "going on from the 3 calls captured in got", "captured 4 calls")
	if k := kept(); k != "POST /login, POST /order, GET /ride, POST /pay" {
		t.Errorf("kept %s", k)
	}
	files, _ := os.ReadDir(filepath.Join(dir, "got", "__files"))
	if len(files) != 4 {
		t.Errorf("%d files in __files", len(files))
	}
	mocks, _ := os.ReadFile(filepath.Join(dir, "got", "mappings", "capture.json"))
	has(string(mocks), "POST /login", "POST /pay")
}
