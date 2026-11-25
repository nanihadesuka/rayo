package mock

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

func writeMappings(t *testing.T, folder, mappings string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(folder, "mappings"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "mappings", "m.json"), []byte(`{"mappings":[`+mappings+`]}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCompare(t *testing.T) {
	var many, other []string
	for i := range 30 {
		many = append(many, fmt.Sprintf(`"k%02d":1`, i))
		other = append(other, fmt.Sprintf(`"k%02d":2`, i))
	}
	for name, c := range map[string]struct{ before, after, want string }{
		"request": {
			`{"name":"a","request":{"method":"GET","urlPath":"/a"},"response":{"status":200}}`,
			`{"name":"a","request":{"method":"GET","urlPath":"/b"},"response":{"status":200}}`,
			"request.urlPath: \"/a\" -> \"/b\"",
		},
		"headers by name": {
			`{"name":"a","request":{"method":"GET"},"response":{"status":200,"headers":{"A":"1","B":"2"}}}`,
			`{"name":"a","request":{"method":"GET"},"response":{"status":200,"headers":{"A":"1","B":"3"}}}`,
			"headers.B: \"2\" -> \"3\"",
		},
		"delay, scenario, priority": {
			`{"name":"a","request":{"method":"GET"},"response":{"status":200}}`,
			`{"name":"a","priority":2,"newScenarioState":"x","request":{"method":"GET"},"response":{"status":200,"fixedDelayMilliseconds":500}}`,
			"response.fixedDelayMilliseconds: null -> 500\npriority: null -> 2\nnewScenarioState: null -> \"x\"",
		},
		"text body": {
			`{"name":"a","request":{"method":"GET"},"response":{"status":200,"body":"one\ntwo\nthree"}}`,
			`{"name":"a","request":{"method":"GET"},"response":{"status":200,"body":"one\n2\n3"}}`,
			"body:2: \"two\" -> \"2\"\nbody: 1 more lines differ",
		},
		"many": {
			`{"name":"a","request":{"method":"GET"},"response":{"status":200,"jsonBody":{` + strings.Join(many, ",") + `}}}`,
			`{"name":"a","request":{"method":"GET"},"response":{"status":200,"jsonBody":{` + strings.Join(other, ",") + `}}}`,
			"... and 10 more",
		},
	} {
		dir := t.TempDir()
		writeMappings(t, filepath.Join(dir, "old"), c.before)
		writeMappings(t, filepath.Join(dir, "new"), c.after)
		changes, err := Compare(filepath.Join(dir, "old"), filepath.Join(dir, "new"))
		if err != nil {
			t.Fatal(err)
		}
		if len(changes) != 1 || changes[0].Kind != KindChanged {
			t.Errorf("%s: %+v", name, changes)
			continue
		}
		got := strings.Join(changes[0].Diff, "\n")
		if !strings.Contains(got, c.want) {
			t.Errorf("%s:\n%s\nwant\n%s", name, got, c.want)
		}
		if name == "many" && len(changes[0].Diff) != maxDiff+1 {
			t.Errorf("many: %d lines", len(changes[0].Diff))
		}
	}
}

func TestShortKeepsRunes(t *testing.T) {
	s := short(strings.Repeat("é", 100))
	if !utf8.ValidString(s) || !strings.HasSuffix(s, "...") {
		t.Errorf("%q", s)
	}
}

func TestMissedAndTooBig(t *testing.T) {
	m := New()
	if got := m.Missed("", regexp.MustCompile("/a"), ""); got[0] != "the mock received no call" {
		t.Errorf("%v", got)
	}
	ask(m, "GET", "/b", "")
	ask(m, "POST", "/a", `{"x":1}`)
	got := strings.Join(m.Missed("GET", regexp.MustCompile("^/a$"), "y"), "\n")
	for _, want := range []string{"the last 2 of 2 calls", "GET /b: path /b doesn't match ^/a$", "POST /a: method POST, not GET; body hasn't \"y\""} {
		if !strings.Contains(got, want) {
			t.Errorf("%s\nwant %s", got, want)
		}
	}
	if code, _ := ask(m, "POST", "/big", strings.Repeat("x", maxRequest+1)); code != 413 {
		t.Errorf("too big: %d", code)
	}
	calls := m.Calls()
	if c := calls[len(calls)-1]; !c.Unmocked || !strings.HasPrefix(c.Why(), "413") {
		t.Errorf("too big: %+v", c)
	}
}

func TestLoadSaysEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := New().Load(dir); err == nil || !strings.Contains(err.Error(), "no mappings in") {
		t.Errorf("%v", err)
	}
}
