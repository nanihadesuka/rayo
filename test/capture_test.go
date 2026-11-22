package test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
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
