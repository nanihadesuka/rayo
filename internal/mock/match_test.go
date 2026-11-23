package mock

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// mockOf is a mock of mappings written here.
func mockOf(t *testing.T, mappings ...string) *Mock {
	t.Helper()
	m := New()
	for _, text := range mappings {
		r, err := ParseRule([]byte(text))
		if err != nil {
			t.Fatalf("%s: %v", text, err)
		}
		if err := m.Add(r); err != nil {
			t.Fatal(err)
		}
	}
	return m
}

// ask makes a call to the mock, and returns the status and the body.
func ask(m *Mock, method, uri, body string, headers ...string) (int, string) {
	q := httptest.NewRequest(method, uri, strings.NewReader(body))
	for i := 0; i+1 < len(headers); i += 2 {
		q.Header.Add(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	m.ServeHTTP(w, q)
	return w.Code, w.Body.String()
}

func TestWireMockRecordingLoads(t *testing.T) {
	dir := t.TempDir()
	// As WireMock's recorder writes them.
	write := func(name, text string) {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("mappings/products-abc.json", `{
	  "id" : "5c2d6f6a-1d7e-4f5a-9b7e-1b7f6d6b1a01",
	  "name" : "products",
	  "request" : { "url" : "/products?page=1", "method" : "GET" },
	  "response" : { "status" : 200, "bodyFileName" : "products-abc.json",
	    "headers" : { "Content-Type" : "application/json", "Set-Cookie" : [ "a=1", "b=2" ] } },
	  "uuid" : "5c2d6f6a-1d7e-4f5a-9b7e-1b7f6d6b1a01",
	  "persistent" : true,
	  "insertionIndex" : 3
	}`)
	write("mappings/login.json", `{
	  "request" : { "urlPath" : "/login", "method" : "POST",
	    "bodyPatterns" : [ { "equalToJson" : "{\"user\":\"a\",\"remember\":true}", "ignoreArrayOrder" : true, "ignoreExtraElements" : true } ] },
	  "response" : { "status" : 200, "jsonBody" : { "token" : "{{randomValue length=8 type='ALPHANUMERIC'}}", "user" : "{{jsonPath request.body '$.user'}}" },
	    "transformers" : [ "response-template" ] }
	}`)
	write("mappings/logo.json", `{ "request" : { "urlPath" : "/logo" }, "response" : { "base64Body" : "iVBORw0KGgo=" } }`)
	write("__files/products-abc.json", `{"items":[1,2]}`)
	m := New()
	if err := m.Load(dir); err != nil {
		t.Fatal(err)
	}
	q := httptest.NewRequest("GET", "/products?page=1", nil)
	w := httptest.NewRecorder()
	m.ServeHTTP(w, q)
	if w.Code != 200 || w.Body.String() != `{"items":[1,2]}` || len(w.Header().Values("Set-Cookie")) != 2 {
		t.Errorf("products: %d %s %v", w.Code, w.Body, w.Header())
	}
	status, body := ask(m, "POST", "/login", `{"remember":true,"user":"a","extra":1}`)
	var got struct{ Token, User string }
	if err := json.Unmarshal([]byte(body), &got); status != 200 || err != nil || len(got.Token) != 8 || got.User != "a" {
		t.Errorf("login: %d %s", status, body)
	}
	if status, _ := ask(m, "POST", "/login", `{"user":"b","remember":true}`); status != 404 {
		t.Errorf("another user: %d", status)
	}
	if _, body := ask(m, "GET", "/logo", ""); body != "\x89PNG\r\n\x1a\n" {
		t.Errorf("logo: %q", body)
	}
}

func TestMatchers(t *testing.T) {
	for _, c := range []struct {
		matcher string
		value   string
		want    bool
	}{
		{`{"equalTo":"Abc","caseInsensitive":true}`, "aBC", true},
		{`{"doesNotContain":"x"}`, "abc", true},
		{`{"doesNotContain":"b"}`, "abc", false},
		{`{"doesNotMatch":"a.*"}`, "abc", false},
		{`{"and":[{"contains":"a"},{"contains":"c"}]}`, "abc", true},
		{`{"or":[{"equalTo":"x"},{"equalTo":"abc"}]}`, "abc", true},
		{`{"not":{"equalTo":"abc"}}`, "abc", false},
		{`{"anything":"anything"}`, "abc", true},
		{`{"equalToJson":{"a":1,"b":[1,2]}}`, `{"b":[1,2],"a":1.0}`, true},
		{`{"equalToJson":{"a":1}}`, `{"a":1,"b":2}`, false},
		{`{"equalToJson":{"a":1},"ignoreExtraElements":true}`, `{"a":1,"b":2}`, true},
		{`{"equalToJson":{"b":[1,2]}}`, `{"b":[2,1]}`, false},
		{`{"equalToJson":{"b":[1,2]},"ignoreArrayOrder":true}`, `{"b":[2,1]}`, true},
		{`{"equalToJson":{"b":[1]},"ignoreExtraElements":true}`, `{"b":[1,3]}`, true}, // as JsonUnit: in order, extra ones after
		{`{"equalToJson":{"b":[1]},"ignoreExtraElements":true}`, `{"b":[3,1]}`, false},
		{`{"equalToJson":{"b":[1]},"ignoreExtraElements":true,"ignoreArrayOrder":true}`, `{"b":[3,1]}`, true},
		// each wanted item needs one of its own: [x,y] fits x and y, but y only fits [x,y]
		{`{"equalToJson":[{"a":1},{"a":1,"b":2}],"ignoreArrayOrder":true,"ignoreExtraElements":true}`, `[{"a":1,"b":2},{"a":1}]`, true},
		{`{"equalToJson":[{"a":1},{"a":1}],"ignoreArrayOrder":true,"ignoreExtraElements":true}`, `[{"a":1,"b":2}]`, false},
		{`{"equalToJson":{"id":"${json-unit.any-string}","n":"${json-unit.any-number}","at":"${json-unit.regex}\\d{4}"}}`, `{"id":"x","n":5,"at":"2026"}`, true},
		{`{"equalToJson":{"id":"${json-unit.any-string}"}}`, `{"id":5}`, false},
		{`{"equalToJson":{"id":"${json-unit.ignore}"}}`, `{}`, false},
		{`{"equalToJson":{"id":"${json-unit.ignore-element}"}}`, `{}`, true},
		{`{"equalToJson":"{\"a\": [1, 2]}"}`, `{"a":[1,2]}`, true},
		{`{"equalToXml":"<a x=\"1\" y=\"2\"><b>t</b></a>"}`, "<a y='2' x='1'>\n  <b> t </b>\n</a>", true},
		{`{"equalToXml":"<a><b>t</b></a>"}`, "<a><b>u</b></a>", false},
		{`{"binaryEqualTo":"AAEC"}`, "\x00\x01\x02", true},
		{`{"matchesJsonPath":"$.a"}`, `{"a":1}`, true},
		{`{"matchesJsonPath":"$.a"}`, `{"a":[]}`, false},
		{`{"matchesJsonPath":"$[?(@.a == 1)]"}`, `{"a":1}`, true},
		{`{"matchesJsonPath":"$.items[?(@.price > 5)].name"}`, `{"items":[{"price":3,"name":"x"},{"price":9,"name":"y"}]}`, true},
		{`{"matchesJsonPath":"$.items[?(@.price > 50)].name"}`, `{"items":[{"price":3,"name":"x"}]}`, false},
		{`{"matchesJsonPath":"$..name"}`, `{"a":{"b":{"name":"x"}}}`, true},
		{`{"matchesJsonPath":"$[?(@.name =~ /ab.*/i)]"}`, `{"name":"ABC"}`, true},
		{`{"matchesJsonPath":"$[?(@.kind in ['a','b'])]"}`, `{"kind":"b"}`, true},
		{`{"matchesJsonPath":"$[?(@.kind nin ['a','b'])]"}`, `{"kind":"b"}`, false},
		{`{"matchesJsonPath":"$[?(@.ids size 2)]"}`, `{"ids":[1,2]}`, true},
		{`{"matchesJsonPath":"$[?(@.ids.length() == 2)]"}`, `{"ids":[1,2]}`, true},
		{`{"matchesJsonPath":"$[?(@.x-y == 1)]"}`, `{"x-y":1}`, true},
		{`{"matchesJsonPath":"$['a b'][-1]"}`, `{"a b":[1,2]}`, true},
		{`{"matchesJsonPath":{"expression":"$.operationName","equalTo":"Wallet"}}`, `{"operationName":"Wallet"}`, true},
		{`{"matchesJsonPath":{"expression":"$.operationName","equalTo":"Wallet"}}`, `{"operationName":"Geo"}`, false},
		{`{"matchesJsonPath":{"expression":"$.query","matches":".*\\bquery\\s+Wallet\\b.*"}}`, `{"query":"fragment F on X { a }\nquery Wallet { ...F }"}`, true},
		{`{"matchesJsonPath":{"expression":"$.variables","equalToJson":{"id":2},"ignoreExtraElements":true}}`, `{"variables":{"id":2,"x":1}}`, true},
		{`{"matchesJsonPath":{"expression":"$.variables","equalToJson":{"id":2}}}`, `{"variables":{"id":3}}`, false},
		{`{"matchesJsonPath":{"expression":"$.missing","absent":true}}`, `{}`, true},
		// as rayo has always written them: JavaScript in a filter
		{`{"matchesJsonPath":"$[?(/^\\s*query\\s+Wallet\\b/.test(@.query))]"}`, `{"query":"query Wallet { a }"}`, true},
	} {
		m := &matcher{}
		if err := strict([]byte(c.matcher), m); err != nil {
			t.Errorf("%s: %v", c.matcher, err)
			continue
		}
		if err := m.prepare(); err != nil {
			t.Errorf("%s: %v", c.matcher, err)
			continue
		}
		if why := m.miss(New().vm, c.value, true); (why == "") != c.want {
			t.Errorf("%s on %s: want %v (%s)", c.matcher, c.value, c.want, why)
		}
	}
}

func TestRequestParts(t *testing.T) {
	m := mockOf(t,
		`{"request":{"method":"GET","urlPathTemplate":"/users/{id}/orders/{order}","pathParameters":{"id":{"matches":"\\d+"}}},"response":{"body":"user {{request.pathSegments.[1]}}"}}`,
		`{"request":{"method":"GET","urlPath":"/q","queryParameters":{"tag":{"hasExactly":[{"equalTo":"a"},{"equalTo":"b"}]}}},"response":{"body":"exactly"}}`,
		`{"request":{"method":"GET","urlPath":"/i","queryParameters":{"tag":{"includes":[{"equalTo":"a"}]}}},"response":{"body":"includes"}}`,
		`{"request":{"method":"GET","urlPath":"/me","cookies":{"session":{"equalTo":"s1"}},"basicAuthCredentials":{"username":"u","password":"p"}},"response":{"body":"me"}}`,
		`{"request":{"method":"POST","urlPath":"/form","formParameters":{"name":{"equalTo":"a b"}}},"response":{"body":"form"}}`,
		`{"request":{"method":"GET","urlPath":"/api.example.com/h","host":{"equalTo":"api.example.com"}},"response":{"body":"host"}}`,
	)
	for _, c := range []struct {
		method, uri, body string
		headers           []string
		want              string
	}{
		{"GET", "/users/42/orders/7", "", nil, "user 42"},
		{"GET", "/users/x/orders/7", "", nil, "not mocked"},
		{"GET", "/q?tag=b&tag=a", "", nil, "exactly"},
		{"GET", "/q?tag=a", "", nil, "not mocked"},
		{"GET", "/i?tag=c&tag=a", "", nil, "includes"},
		{"GET", "/me", "", []string{"Cookie", "session=s1", "Authorization", "Basic dTpw"}, "me"},
		{"GET", "/me", "", []string{"Cookie", "session=s2", "Authorization", "Basic dTpw"}, "not mocked"},
		{"POST", "/form", "x=1&name=a+b", []string{"Content-Type", "application/x-www-form-urlencoded"}, "form"},
		{"GET", "/api.example.com/h", "", nil, "host"},
	} {
		if _, got := ask(m, c.method, c.uri, c.body, c.headers...); !strings.HasPrefix(got, c.want) {
			t.Errorf("%s %s: %q, want %q", c.method, c.uri, got, c.want)
		}
	}
}

func TestNotSupported(t *testing.T) {
	for text, want := range map[string]string{
		`{"request":{"urlPath":"/a"},"response":{"proxyBaseUrl":"https://x"}}`:                             "proxyBaseUrl isn't supported",
		`{"request":{"urlPath":"/a"},"response":{},"postServeActions":[{"name":"webhook"}]}`:               "postServeActions isn't supported",
		`{"request":{"urlPath":"/a","multipartPatterns":[{}]},"response":{}}`:                              "multipartPatterns isn't supported",
		`{"request":{"urlPath":"/a","bodyPatterns":[{"matchesXPath":"/a"}]},"response":{}}`:                "matchesXPath isn't supported",
		`{"request":{"urlPath":"/a"},"response":{"transformers":["my-own"]}}`:                              "transformer \"my-own\" isn't supported",
		`{"request":{"urlPath":"/a"},"response":{"fault":"OOPS"}}`:                                         "fault takes one of",
		`{"request":{"urlPath":"/a"},"response":{"body":"{{nope}}","transformers":["response-template"]}}`: "unknown template",
	} {
		if _, err := ParseRule([]byte(text)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", text, err, want)
		}
	}
	// What WireMock writes and doesn't change what is answered is taken.
	if _, err := ParseRule([]byte(`{"request":{"urlPath":"/a"},"response":{},"postServeActions":[],"metadata":{"x":1}}`)); err != nil {
		t.Error(err)
	}
}

func TestTemplates(t *testing.T) {
	m := mockOf(t,
		`{"request":{"urlPath":"/html"},"response":{"body":"{{request.query.q}} {{{request.query.q}}}","transformers":["response-template"]}}`,
		`{"request":{"urlPath":"/rayo"},"response":{"body":"{{request.query.q}} {{request.method}} {{request.headers.[X-A]}} {{request.query.a.b}}"}}`,
		`{"request":{"urlPath":"/plain"},"response":{"body":"{{nope}}","transformers":[]}}`,
		`{"request":{"urlPath":"/params"},"response":{"body":"{{parameters.who}}","transformers":["response-template"],"transformerParameters":{"who":"me"}}}`,
		`{"request":{"urlPath":"/now"},"response":{"body":"{{now format='yyyy-MM-dd' timezone='UTC'}}|{{now format='epoch'}}|{{randomValue type='UUID'}}"}}`,
		`{"request":{"urlPath":"/first"},"response":{"body":"{{regexExtract request.path '[a-z]+'}}"}}`,
	)
	check := func(uri, want string, headers ...string) {
		t.Helper()
		if _, got := ask(m, "GET", uri, "", headers...); got != want {
			t.Errorf("%s: %q, want %q", uri, got, want)
		}
	}
	check("/html?q=%3Cb%3E", "&lt;b&gt; <b>")
	check("/rayo?q=%3Cb%3E&a.b=dot", "<b> GET x dot", "X-A", "x")
	check("/plain", "{{nope}}")
	check("/params", "me")
	check("/first", "first")
	_, got := ask(m, "GET", "/now", "")
	parts := strings.Split(got, "|")
	if len(parts) != 3 || parts[0] != time.Now().UTC().Format("2006-01-02") || len(parts[1]) != 13 ||
		!regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(parts[2]) {
		t.Errorf("now: %q", got)
	}
	format, err := javaDate("yyyy-MM-dd'T'HH:mm:ss.SSSXXX EEE MMM")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 1, 2, 3, 4, 5, 678000000, time.UTC)
	if got := format(at); got != "2026-01-02T03:04:05.678Z Fri Jan" {
		t.Errorf("java date: %q", got)
	}
}

func TestServing(t *testing.T) {
	m := mockOf(t,
		`{"request":{"method":"POST","urlPath":"/zip","bodyPatterns":[{"equalToJson":{"a":1}}]},"response":{"body":"unzipped"}}`,
		`{"request":{"urlPath":"/empty"},"response":{"fault":"EMPTY_RESPONSE"}}`,
		`{"request":{"urlPath":"/drip"},"response":{"body":"abcdef","chunkedDribbleDelay":{"numberOfChunks":3,"totalDuration":30}}}`,
		`{"request":{"urlPath":"/products"},"response":{"body":"no host"}}`,
		`{"request":{"urlPath":"/known.example.com/a"},"response":{"body":"known"}}`,
	)
	port, err := m.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	base := "http://127.0.0.1:" + itoa(port)
	var zipped bytes.Buffer
	z := gzip.NewWriter(&zipped)
	_, _ = z.Write([]byte(`{"a":1}`))
	_ = z.Close()
	q, _ := http.NewRequest("POST", base+"/zip", &zipped)
	q.Header.Set("Content-Encoding", "gzip")
	if got := do(t, q); got != "200 unzipped" {
		t.Errorf("gzip: %s", got)
	}
	if _, err := http.Get(base + "/empty"); err == nil {
		t.Error("EMPTY_RESPONSE answered")
	}
	q, _ = http.NewRequest("GET", base+"/drip", nil)
	if got := do(t, q); got != "200 abcdef" {
		t.Errorf("drip: %s", got)
	}
	// Through the proxy a call is /host/path: a mapping without the host answers it, unless one has the host.
	q, _ = http.NewRequest("GET", base+"/api.example.com/products", nil)
	if got := do(t, q); got != "200 no host" {
		t.Errorf("without the host: %s", got)
	}
	q, _ = http.NewRequest("GET", base+"/known.example.com/products", nil)
	if got := do(t, q); !strings.HasPrefix(got, "404") {
		t.Errorf("a host the mocks know: %s", got)
	}
}

func itoa(n int) string { return strings.TrimSpace(strings.ReplaceAll(jsonNumber(n), " ", "")) }

func jsonNumber(n int) string { b, _ := json.Marshal(n); return string(b) }

func do(t *testing.T, q *http.Request) string {
	t.Helper()
	res, err := http.DefaultClient.Do(q)
	if err != nil {
		return "error: " + err.Error()
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return itoa(res.StatusCode) + " " + string(data)
}

func TestNearMiss(t *testing.T) {
	m := mockOf(t,
		`{"name":"availability","request":{"method":"POST","urlPath":"/availability","bodyPatterns":[{"equalToJson":{"category":"ondemand"},"ignoreExtraElements":true}]},"response":{}}`,
		`{"scenarioName":"s","requiredScenarioState":"paid","request":{"method":"GET","urlPath":"/order"},"response":{}}`,
	)
	if _, got := ask(m, "POST", "/availability", `{"category":"prebooked"}`); !strings.Contains(got, `nearest availability: body: $.category: "prebooked", expected "ondemand"`) {
		t.Errorf("body: %s", got)
	}
	if _, got := ask(m, "GET", "/order", ""); !strings.Contains(got, "scenario s is in Started, it needs paid") {
		t.Errorf("state: %s", got)
	}
}

func TestDefinitions(t *testing.T) {
	for doc, want := range map[string]string{
		"query Wallet { a }": "query Wallet",
		"{ a }":              "query ",
		"# mutation Fake\nmutation Pay($id: ID!) { pay(id: $id) }": "mutation Pay",
		"fragment F on X { a }\nmutation Pay { ...F }":             "mutation Pay",
		`query Q { a(text: "mutation M { }") }`:                    "query Q",
		"query A { a } mutation B { b }":                           "query A,mutation B",
		`query Q($s: String = """a { mutation """) { a }`:          "query Q",
	} {
		var got []string
		for _, d := range definitions(doc) {
			got = append(got, d.kind+" "+d.name)
		}
		if strings.Join(got, ",") != want {
			t.Errorf("%q: %v, want %s", doc, got, want)
		}
	}
	if d, ok := chosen("query A { a } mutation B { b }", "B"); !ok || d.kind != "mutation" {
		t.Errorf("chosen: %v", d)
	}
}

// capture makes mocks of calls as a capture would, and loads them.
func capture(t *testing.T, s Settings, calls ...captured) (*Mock, string) {
	t.Helper()
	dir := t.TempDir()
	r := &real{settings: s}
	for i := range calls {
		c := &calls[i]
		if c.request == nil {
			c.request = &request{}
		}
		if c.contentType == "" {
			c.contentType = "application/json"
		}
		q := httptest.NewRequest(c.method, c.uri, nil)
		q.Header.Set("Content-Type", c.request.ContentType)
		for name, values := range c.request.Headers {
			q.Header[name] = values
		}
		c.request = r.cleanRequest(q, []byte(c.request.Body))
		c.uri = r.cleanURI(q.URL)
	}
	if _, err := saveCaptured(dir, calls, s, nil, nil, func([]string) {}); err != nil {
		t.Fatal(err)
	}
	m := New()
	if err := m.Load(dir); err != nil {
		mappings, _ := os.ReadFile(filepath.Join(dir, "mappings", "capture.json"))
		t.Fatalf("%v\n%s", err, mappings)
	}
	mappings, _ := os.ReadFile(filepath.Join(dir, "mappings", "capture.json"))
	return m, string(mappings)
}

func post(uri, body, answer string) captured {
	return captured{method: "POST", uri: uri, status: 200, body: answer, request: &request{ContentType: "application/json", Body: body}}
}

func get(uri, answer string) captured {
	return captured{method: "GET", uri: uri, status: 200, body: answer}
}

// The case of the issue: the same POST with other bodies, and a moment in the query.
func TestCaptureByBody(t *testing.T) {
	m, mappings := capture(t, Settings{},
		post("/availability", `{"category":"ondemand","at":"2026-11-23T18:14:29Z"}`, `{"available":false}`),
		post("/availability", `{"category":"prebooked","at":"2026-11-23T18:14:31Z"}`, `{"available":true}`),
		get("/directions?from=a&depart_at=2026-11-23T18:14:29", `{"route":1}`),
	)
	if strings.Contains(mappings, "scenarioName") {
		t.Errorf("states, for calls told apart by their bodies:\n%s", mappings)
	}
	// Another order, other moments.
	for _, c := range [][3]string{
		{"POST", `{"category":"prebooked","at":"2027-01-01T00:00:00Z"}`, `{"available":true}`},
		{"POST", `{"at":"2027-01-01T00:00:01Z","category":"ondemand"}`, `{"available":false}`},
	} {
		if _, got := ask(m, c[0], "/availability", c[1]); strings.Join(strings.Fields(got), "") != c[2] {
			t.Errorf("%s: %s, want %s", c[1], got, c[2])
		}
	}
	if status, got := ask(m, "GET", "/directions?depart_at=2027-02-02T10:00:00&from=a", ""); status != 200 {
		t.Errorf("directions: %d %s", status, got)
	}
	if status, _ := ask(m, "GET", "/directions?depart_at=2027-02-02T10:00:00&from=b", ""); status != 404 {
		t.Error("from=b was answered")
	}
}

// Of two that fit, with extra elements ignored, the closest answers.
func TestCaptureClosest(t *testing.T) {
	m, mappings := capture(t, Settings{},
		post("/search", `{"q":"a","filters":{"color":"red"}}`, `{"n":1}`),
		post("/search", `{"q":"a"}`, `{"n":2}`),
		get("/items?page=2", `{"page":2}`),
		get("/items", `{"page":1}`),
	)
	for _, c := range [][3]string{
		{"POST /search", `{"q":"a"}`, `{"n":2}`},
		{"POST /search", `{"q":"a","filters":{"color":"red"}}`, `{"n":1}`},
		{"GET /items?page=2", "", `{"page":2}`},
		{"GET /items", "", `{"page":1}`},
	} {
		method, uri, _ := strings.Cut(c[0], " ")
		if _, got := ask(m, method, uri, c[1]); strings.Join(strings.Fields(got), "") != c[2] {
			t.Errorf("%s %s: %s, want %s\n%s", c[0], c[1], got, c[2], mappings)
		}
	}
}

// Ignored and private parts aren't matched; the same call made twice with other answers still gets states.
func TestCaptureIgnoreAndStates(t *testing.T) {
	m, mappings := capture(t, Settings{IgnoreParams: []string{"session"}, IgnoreBody: []string{"nonce", "trip.clientTime"}},
		post("/quote?session=1", `{"trip":{"from":"a","clientTime":5},"nonce":1,"password":"x"}`, `{"price":5}`),
		post("/book", `{"id":1}`, `{"ok":true}`),
		post("/quote?session=2", `{"trip":{"from":"a","clientTime":6},"nonce":2,"password":"y"}`, `{"price":7}`),
	)
	if strings.Contains(mappings, "password") || strings.Contains(mappings, "nonce") || strings.Contains(mappings, "session") {
		t.Errorf("matched what was ignored or private:\n%s", mappings)
	}
	for _, want := range [][2]string{{"/quote?session=9", `{"price":5}`}, {"/book", `{"ok":true}`}, {"/quote?session=8", `{"price":7}`}} {
		body := `{"trip":{"from":"a","clientTime":1},"nonce":3,"password":"z"}`
		if want[0] == "/book" {
			body = `{"id":1}`
		}
		path, query, _ := strings.Cut(want[0], "?")
		if _, got := ask(m, "POST", path+"?"+query, body); strings.Join(strings.Fields(got), "") != want[1] {
			t.Errorf("%s: %s, want %s\n%s", want[0], got, want[1], mappings)
		}
	}
}

// GraphQL: variables, persisted queries asked first by their hash, batches and GET.
func TestCaptureGraphQLCases(t *testing.T) {
	gql := func(body, answer string) captured { return post("/graphql", body, answer) }
	m, mappings := capture(t, Settings{},
		gql(`{"operationName":"Place","query":"query Place($id: ID!) { place(id: $id) { name } }","variables":{"id":1}}`, `{"data":{"place":1}}`),
		gql(`{"operationName":"Place","query":"query Place($id: ID!) { place(id: $id) { name } }","variables":{"id":2}}`, `{"data":{"place":2}}`),
		gql(`{"operationName":"Feed","extensions":{"persistedQuery":{"version":1,"sha256Hash":"abc"}}}`, `{"errors":[{"message":"PersistedQueryNotFound"}]}`),
		gql(`{"operationName":"Feed","query":"query Feed { feed }","extensions":{"persistedQuery":{"version":1,"sha256Hash":"abc"}}}`, `{"data":{"feed":[]}}`),
		gql(`[{"operationName":"A","query":"query A { a }"},{"operationName":"B","query":"mutation B { b }"}]`, `[{"data":{"a":1}},{"data":{"b":1}}]`),
		get("/graphql?operationName=Me&query=query%20Me%20%7B%20me%20%7D", `{"data":{"me":1}}`),
	)
	if strings.Contains(mappings, ".test(") || strings.Contains(mappings, ".replace(") || strings.Contains(mappings, ".length") {
		t.Errorf("JavaScript that WireMock can't read:\n%s", mappings)
	}
	for _, c := range [][2]string{
		{`{"operationName":"Place","query":"query Place($id: ID!) { place(id: $id) { name } }","variables":{"id":2}}`, `{"data":{"place":2}}`},
		{`{"operationName":"Place","query":"query Place($id: ID!) { place(id: $id) { name } }","variables":{"id":1}}`, `{"data":{"place":1}}`},
		{`{"operationName":"Feed","query":"query Feed { feed }","extensions":{"persistedQuery":{"version":1,"sha256Hash":"abc"}}}`, `{"data":{"feed":[]}}`},
		{`{"operationName":"Feed","extensions":{"persistedQuery":{"version":1,"sha256Hash":"abc"}}}`, `{"errors":[{"message":"PersistedQueryNotFound"}]}`},
		{`[{"operationName":"A","query":"query A { a }"},{"operationName":"B","query":"mutation B { b }"}]`, `[{"data":{"a":1}},{"data":{"b":1}}]`},
	} {
		if _, got := ask(m, "POST", "/graphql", c[0]); strings.Join(strings.Fields(got), "") != c[1] {
			t.Errorf("%s: %s, want %s\n%s", c[0], got, c[1], mappings)
		}
	}
	if _, got := ask(m, "GET", "/graphql?query=query%20Me%20%7B%20me%20%7D&operationName=Me", ""); strings.Join(strings.Fields(got), "") != `{"data":{"me":1}}` {
		t.Errorf("GET: %s", got)
	}
	if !strings.Contains(mappings, `"name": "POST /graphql graphql:Feed (hash only)"`) {
		t.Errorf("no hash only:\n%s", mappings)
	}
}

// A body of JSON that is a text, and a batch, whose order counts.
func TestCaptureScalarsAndBatches(t *testing.T) {
	batch := func(a, b string) string {
		return `[{"operationName":"` + a + `","query":"query ` + a + ` { x }"},{"operationName":"` + b + `","query":"query ` + b + ` { x }"}]`
	}
	m, mappings := capture(t, Settings{},
		post("/echo", `"hello"`, `{"said":"hello"}`),
		post("/echo", `42`, `{"said":42}`),
		post("/graphql", batch("A", "B"), `[{"a":1},{"b":1}]`),
		post("/graphql", batch("B", "A"), `[{"b":1},{"a":1}]`),
	)
	for _, c := range [][3]string{
		{"/echo", `"hello"`, `{"said":"hello"}`},
		{"/echo", `42.0`, `{"said":42}`},
		{"/graphql", batch("B", "A"), `[{"b":1},{"a":1}]`},
		{"/graphql", batch("A", "B"), `[{"a":1},{"b":1}]`},
	} {
		if _, got := ask(m, "POST", c[0], c[1]); strings.Join(strings.Fields(got), "") != c[2] {
			t.Errorf("%s: %s, want %s\n%s", c[1], got, c[2], mappings)
		}
	}
}

func TestCaptureKeepsAnswers(t *testing.T) {
	png := "\x89PNG\r\n\x1a\n\x00\xff"
	c := get("/logo", base64.StdEncoding.EncodeToString([]byte(png)))
	c.binary, c.contentType = true, "image/png"
	c.headers = map[string][]string{"X-Total-Count": {"9"}, "Set-Cookie": {"sid=abc; Path=/", "lang=es"}}
	text := get("/page", "Hello {{name}}")
	text.contentType = "text/plain"
	m, mappings := capture(t, Settings{}, c, text)
	q := httptest.NewRequest("GET", "/logo", nil)
	w := httptest.NewRecorder()
	m.ServeHTTP(w, q)
	if w.Body.String() != png || w.Header().Get("X-Total-Count") != "9" || len(w.Header().Values("Set-Cookie")) != 2 {
		t.Errorf("logo: %q %v\n%s", w.Body.String(), w.Header(), mappings)
	}
	if _, got := ask(m, "GET", "/page", ""); got != "Hello {{name}}" {
		t.Errorf("text: %q", got)
	}
}

func TestImportHAR(t *testing.T) {
	dir := t.TempDir()
	har := `{"log":{"entries":[
	  {"startedDateTime":"2026-11-23T10:00:01.000Z",
	   "request":{"method":"POST","url":"https://api.example.com/availability","headers":[{"name":"Content-Type","value":"application/json"}],
	     "postData":{"mimeType":"application/json","text":"{\"category\":\"prebooked\",\"email\":\"a@b.com\"}"}},
	   "response":{"status":200,"headers":[{"name":"Content-Type","value":"application/json"},{"name":"Date","value":"x"}],
	     "content":{"mimeType":"application/json","text":"{\"available\":true}"}}},
	  {"startedDateTime":"2026-11-23T10:00:00.000Z",
	   "request":{"method":"POST","url":"https://api.example.com/availability","headers":[],
	     "postData":{"mimeType":"application/json","text":"{\"category\":\"ondemand\"}"}},
	   "response":{"status":200,"headers":[],"content":{"mimeType":"application/json","text":"eyJhdmFpbGFibGUiOmZhbHNlfQ==","encoding":"base64"}}},
	  {"startedDateTime":"2026-11-23T10:00:02.000Z",
	   "request":{"method":"GET","url":"https://www.google-analytics.com/collect","headers":[]},
	   "response":{"status":200,"headers":[],"content":{"text":""}}}
	]}}`
	file := filepath.Join(dir, "flows.har")
	if err := os.WriteFile(file, []byte(har), 0o644); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(dir, "mock")
	notes, err := ImportHAR(file, folder, "", Settings{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(notes, "\n"), "1 calls to Google, Firebase, analytics or crash reports") {
		t.Errorf("notes: %v", notes)
	}
	m := New()
	if err := m.Load(folder); err != nil {
		t.Fatal(err)
	}
	for body, want := range map[string]string{`{"category":"ondemand"}`: `{"available":false}`, `{"email":"x@y.z","category":"prebooked"}`: `{"available":true}`} {
		if _, got := ask(m, "POST", "/api.example.com/availability", body); strings.Join(strings.Fields(got), "") != want {
			t.Errorf("%s: %s, want %s", body, got, want)
		}
	}
	mappings, _ := os.ReadFile(filepath.Join(folder, "mappings", "capture.json"))
	if strings.Contains(string(mappings), "a@b.com") || strings.Contains(string(mappings), `"Date"`) {
		t.Errorf("private or volatile:\n%s", mappings)
	}
}
