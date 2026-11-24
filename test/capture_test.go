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

// REST and GraphQL calls in one capture: GraphQL ones are told apart by their operation, so their
// mocks answer whatever the order, and only a mutation moves the scenario on.
func TestCaptureGraphQL(t *testing.T) {
	booked := false
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if q.URL.Path != "/graphql" {
			fmt.Fprintf(w, `{"rest":%q}`, q.URL.Path)
			return
		}
		var b struct {
			OperationName string `json:"operationName"`
			Variables     struct{ Id int }
		}
		_ = json.NewDecoder(q.Body).Decode(&b)
		switch b.OperationName {
		case "Book":
			booked = true
			fmt.Fprint(w, `{"data":{"book":true}}`)
		case "Wallet":
			fmt.Fprintf(w, `{"data":{"wallet":{"booked":%t}}}`, booked)
		case "Place":
			fmt.Fprintf(w, `{"data":{"place":%d}}`, b.Variables.Id)
		default:
			fmt.Fprintf(w, `{"data":{"%s":1}}`, b.OperationName)
		}
	}))
	defer s.Close()
	dir := t.TempDir()
	gql := func(op, query string, vars string) string {
		return fmt.Sprintf("- tapOn: { id: 'POST /graphql {\"operationName\":\"%s\",\"query\":\"%s %s { x }\",\"variables\":%s}' }\n", op, query, op, vars)
	}
	rest := "- tapOn: { id: \"GET /profile\" }\n"
	geo, wallet := gql("Geocode", "query", "{}"), gql("Wallet", "query", "{}")
	place := func(id int) string { return gql("Place", "query", fmt.Sprintf(`{"id":%d}`, id)) }
	book := gql("Book", "mutation", "{}")
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
			if strings.HasPrefix(l, "  app: ") {
				_, a, _ := strings.Cut(l, " -> ")
				answers = append(answers, strings.Join(strings.Fields(a), ""))
			}
		}
		return answers
	}
	write(t, filepath.Join(dir, "real.yaml"), "appId: a.b\n---\n"+rest+geo+wallet+place(1)+place(2)+book+wallet+rest)
	run("test", "--capture", "got", "--capture-from", s.URL, "--graphql", "operation+variables", "real.yaml")
	mocks, _ := os.ReadFile(filepath.Join(dir, "got", "mappings", "capture.json"))
	for _, want := range []string{`"equalTo": "Geocode"`, `"expression": "$.operationName"`, `"expression": "$.variables"`, `"id": 2`, `"urlPath": "/profile"`} {
		if !strings.Contains(string(mocks), want) {
			t.Errorf("no %s in\n%s", want, mocks)
		}
	}
	// Another order: wallet before geocode, the places swapped.
	write(t, filepath.Join(dir, "mocked.yaml"), "appId: a.b\nmock: got\n---\n"+wallet+rest+place(2)+geo+place(1)+book+rest+wallet)
	got := run("test", "mocked.yaml")
	want := []string{
		`200application/json{"data":{"wallet":{"booked":false}}}`,
		`200application/json{"rest":"/profile"}`,
		`200application/json{"data":{"place":2}}`,
		`200application/json{"data":{"Geocode":1}}`,
		`200application/json{"data":{"place":1}}`,
		`200application/json{"data":{"book":true}}`,
		`200application/json{"rest":"/profile"}`,
		`200application/json{"data":{"wallet":{"booked":true}}}`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	write(t, filepath.Join(dir, "missing.yaml"), "appId: a.b\nmock: got\n---\n"+gql("Unknown", "query", "{}"))
	d := startDevice(t, "emulator-5554\tdevice\n", false)
	out, err := d.command(dir, "test", "--retries", "0", "missing.yaml").CombinedOutput()
	if err == nil || !strings.Contains(string(out), "POST /graphql (GraphQL operation Unknown)") {
		t.Errorf("%v\n%s", err, out)
	}
}

// One operation name asked with two shapes: each shape gets its own answer, whatever the order.
func TestCaptureGraphQLShapes(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var b struct{ Query string }
		_ = json.NewDecoder(q.Body).Decode(&b)
		if strings.Contains(b.Query, "balance") {
			fmt.Fprint(w, `{"data":{"wallet":{"balance":5}}}`)
		} else {
			fmt.Fprint(w, `{"data":{"wallet":{"cards":[]}}}`)
		}
	}))
	defer s.Close()
	dir := t.TempDir()
	ask := func(fields string) string {
		return fmt.Sprintf("- tapOn: { id: 'POST /api/graphql {\"operationName\":\"Wallet\",\"query\":\"query Wallet {  wallet { %s } }\"}' }\n", fields)
	}
	run := func(flow string, args ...string) []string {
		write(t, filepath.Join(dir, "f.yaml"), flow)
		d := startDevice(t, "emulator-5554\tdevice\n", false)
		out, err := d.command(dir, append(append([]string{"test"}, args...), "f.yaml")...).CombinedOutput()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		d.mu.Lock()
		defer d.mu.Unlock()
		var answers []string
		for _, l := range d.log {
			if _, a, ok := strings.Cut(l, "  app: "); ok {
				_, a, _ = strings.Cut(a, " -> ")
				answers = append(answers, strings.Join(strings.Fields(a), ""))
			}
		}
		return answers
	}
	run("appId: a.b\n---\n"+ask("balance")+ask("cards"), "--capture", "got", "--capture-from", s.URL)
	got := run("appId: a.b\nmock: got\n---\n" + ask("cards") + ask("balance"))
	want := `200application/json{"data":{"wallet":{"cards":[]}}} 200application/json{"data":{"wallet":{"balance":5}}}`
	if strings.Join(got, " ") != want {
		t.Errorf("got %v", got)
	}
}

// An update captures again beside the mocks, says what changed, and takes only what is chosen.
func TestCaptureUpdate(t *testing.T) {
	price, name := 5, "old"
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if q.URL.Path == "/graphql" {
			fmt.Fprintf(w, `{"data":{"wallet":{"price":%d}}}`, price)
			return
		}
		fmt.Fprintf(w, `{"name":%q}`, name)
	}))
	defer s.Close()
	dir := t.TempDir()
	flow := "appId: a.b\n---\n- tapOn: { id: \"GET /profile\" }\n" +
		"- tapOn: { id: 'POST /graphql {\"operationName\":\"Wallet\",\"query\":\"query Wallet { x }\"}' }\n"
	write(t, filepath.Join(dir, "f.yaml"), flow)
	rayo := func(args ...string) string {
		t.Helper()
		d := startDevice(t, "emulator-5554\tdevice\n", false)
		out, err := d.command(dir, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
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
	rayo("test", "--capture", "mock", "--capture-from", s.URL, "f.yaml")
	price, name = 7, "new"
	out := rayo("test", "--capture-update", "mock", "--capture-from", s.URL, "f.yaml")
	has(out, "0 same, 2 changed, 0 new, 0 gone", "changed POST /graphql graphql:Wallet", "body.data.wallet.price: 5 -> 7", `body.name: "old" -> "new"`)
	report, _ := os.ReadFile(filepath.Join(dir, "mock", ".update", "changes.txt"))
	has(string(report), "rayo mocks apply mock")
	has(rayo("mocks", "apply", "--only", "graphql:Wallet", "mock"), "took 1 changes", "graphql:Wallet")
	has(rayo("test", "--capture-update", "mock", "--capture-from", s.URL, "f.yaml"), "1 same, 1 changed", "changed GET /profile")
	write(t, filepath.Join(dir, "m.yaml"), strings.Replace(flow, "---", "mock: mock\n---", 1))
	rayo("test", "m.yaml") // the mocks still load
	d := startDevice(t, "emulator-5554\tdevice\n", false)
	check, err := d.command(dir, "test", "--capture-update", "mock", "--check", "--capture-from", s.URL, "f.yaml").CombinedOutput()
	if err == nil || !strings.Contains(string(check), "differ from the backend: 1 changes") {
		t.Errorf("--check: %v\n%s", err, check)
	}
	has(rayo("mocks", "apply", "mock"), "took 1 changes")
	has(rayo("test", "--capture-update", "mock", "--check", "--capture-from", s.URL, "f.yaml"), "2 same, 0 changed")
	wallet, _ := os.ReadFile(filepath.Join(dir, "mock", "__files", "002_post_Wallet.json"))
	profile, _ := os.ReadFile(filepath.Join(dir, "mock", "__files", "001_get_profile.json"))
	has(string(wallet), `"price": 7`)
	has(string(profile), `"name": "new"`) // after taking them all
}

// The case of issue 18: a suite says how its flows are captured, so re-recording one's mock needs only
// --capture-update with the suite; the flow's entry wins over the suite, and an option over both.
func TestCaptureUpdateOfSuite(t *testing.T) {
	name := "old"
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"name":%q}`, name)
	}))
	defer s.Close()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "f.yaml"), "appId: a.b\n---\n- tapOn: { id: \"GET /profile?_ts=1\" }\n")
	write(t, filepath.Join(dir, "s.suite.yaml"), "capture:\n  from: "+s.URL+"\n  ignoreParams: [nope]\nflows:\n"+
		"  - file: f.yaml\n    mock: [common, mock]\n    capture:\n      ignoreParams: [_ts]\n")
	write(t, filepath.Join(dir, "common", "mappings", "x.json"), `{"mappings":[]}`)
	rayo := func(args ...string) string {
		t.Helper()
		d := startDevice(t, "emulator-5554\tdevice\n", false)
		out, err := d.command(dir, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return string(out)
	}
	rayo("test", "--capture", "mock", "--capture-from", s.URL, "--ignore-params", "_ts", "f.yaml")
	name = "new"
	out := rayo("test", "--capture-update", "s.suite.yaml")
	if !strings.Contains(out, "0 same, 1 changed") {
		t.Errorf("no change in\n%s", out)
	}
	settings, _ := os.ReadFile(filepath.Join(dir, "mock", ".update", "mappings", "capture.json"))
	if strings.Contains(string(settings), "queryParameters") {
		t.Errorf("not the flow's settings:\n%s", settings)
	}
	rayo("mocks", "apply", "mock")
	rayo("test", "s.suite.yaml") // replays, with ?_ts ignored
}

// The case of issue 8: the same POST with other bodies is answered by its body, whatever the order and
// the timing, and a moment in the query, or a parameter asked to be ignored, doesn't break a replay.
func TestCaptureRESTBodies(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", "2")
		var b struct{ Category string }
		_ = json.NewDecoder(q.Body).Decode(&b)
		switch q.URL.Path {
		case "/availability":
			fmt.Fprintf(w, `{"available":%t}`, b.Category == "prebooked")
		default:
			fmt.Fprintf(w, `{"route":%q}`, q.URL.Query().Get("from"))
		}
	}))
	defer s.Close()
	dir := t.TempDir()
	call := func(category string) string {
		return fmt.Sprintf("- tapOn: { id: 'POST /availability {\"category\":\"%s\",\"session_token\":\"t%d\"}' }\n", category, len(category))
	}
	directions := func(at string) string {
		return fmt.Sprintf("- tapOn: { id: \"GET /directions?from=a&depart_at=%s&trace=%s\" }\n", at, at[len(at)-2:])
	}
	run := func(flow string, args ...string) []string {
		t.Helper()
		write(t, filepath.Join(dir, "f.yaml"), flow)
		d := startDevice(t, "emulator-5554\tdevice\n", false)
		out, err := d.command(dir, append(append([]string{"test"}, args...), "f.yaml")...).CombinedOutput()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		d.mu.Lock()
		defer d.mu.Unlock()
		var answers []string
		for _, l := range d.log {
			if _, a, ok := strings.Cut(l, "  app: "); ok {
				_, a, _ = strings.Cut(a, " -> ")
				answers = append(answers, strings.Join(strings.Fields(a), ""))
			}
		}
		return answers
	}
	run("appId: a.b\n---\n"+call("ondemand")+call("prebooked")+directions("2026-11-23T18:14:29"),
		"--capture", "got", "--capture-from", s.URL, "--ignore-params", "trace")
	mocks, _ := os.ReadFile(filepath.Join(dir, "got", "mappings", "capture.json"))
	if strings.Contains(string(mocks), "scenarioName") || strings.Contains(string(mocks), "session_token") || strings.Contains(string(mocks), "trace") {
		t.Errorf("states, a private value or an ignored one:\n%s", mocks)
	}
	got := run("appId: a.b\nmock: got\n---\n- network: { delay: 10ms }\n" + call("prebooked") + call("ondemand") + call("prebooked") + directions("2027-01-01T09:00:00"))
	want := []string{
		`200application/json{"available":true}`,
		`200application/json{"available":false}`,
		`200application/json{"available":true}`,
		`200application/json{"route":"a"}`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	kept, _ := os.ReadFile(filepath.Join(dir, "got", "capture.calls.json"))
	for _, w := range []string{`"ignoreParams": [`, `\"category\":\"prebooked\"`} {
		if !strings.Contains(string(kept), w) {
			t.Errorf("no %s in\n%s", w, kept)
		}
	}
	// A HAR file, as mitmproxy writes one, becomes the same mocks.
	har := fmt.Sprintf(`{"log":{"entries":[{"startedDateTime":"2026-11-23T10:00:00Z",
		"request":{"method":"POST","url":"%s/availability","headers":[],"postData":{"mimeType":"application/json","text":"{\"category\":\"prebooked\"}"}},
		"response":{"status":200,"headers":[{"name":"Content-Type","value":"application/json"}],"content":{"mimeType":"application/json","text":"{\"available\":true}"}}}]}}`, s.URL)
	write(t, filepath.Join(dir, "flows.har"), har)
	d := startDevice(t, "emulator-5554\tdevice\n", false)
	if out, err := d.command(dir, "mocks", "import", "--capture-from", s.URL, "flows.har", "imported").CombinedOutput(); err != nil || !strings.Contains(string(out), "captured 1 calls") {
		t.Fatalf("%v\n%s", err, out)
	}
	if got := run("appId: a.b\nmock: imported\n---\n" + call("prebooked")); len(got) != 1 || got[0] != `200application/json{"available":true}` {
		t.Errorf("imported: %v", got)
	}
}
