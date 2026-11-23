package mock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func call(method, uri, body string, status int) captured {
	return captured{method: method, uri: uri, status: status, contentType: "application/json", body: body,
		request: &request{ContentType: "application/json"}}
}

// mappingsOf writes the mocks of calls, and reads them back.
func mappingsOf(t *testing.T, s Settings, calls ...captured) []rule {
	t.Helper()
	dir := t.TempDir()
	b := &builder{s: s, loosened: map[string]bool{}, unmatched: map[string]string{}}
	if _, _, err := writeMocks(dir, calls, b); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "mappings", "capture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct{ Mappings []rule }
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	return file.Mappings
}

func TestStatelessPOST(t *testing.T) {
	calls := []captured{
		call("POST", "/api.x.com/quote", `{"p":1}`, 200),
		call("GET", "/api.x.com/cart", `{"n":0}`, 200),
		call("POST", "/api.x.com/quote", `{"p":1}`, 200),
		call("GET", "/api.x.com/cart", `{"n":1}`, 200),
	}
	for _, r := range mappingsOf(t, Settings{Stateless: []string{"api.x.com/quote"}}, calls...) {
		if strings.Contains(r.Name, "quote") && (r.Next != "" || r.Required != "") {
			t.Errorf("a stateless call has a state: %+v", r)
		}
	}
	moved := false
	for _, r := range mappingsOf(t, Settings{}, calls...) {
		moved = moved || strings.Contains(r.Name, "quote") && r.Next != ""
	}
	if !moved {
		t.Error("without --stateless, the POST should move the state")
	}
}

func TestIgnorePathIDs(t *testing.T) {
	calls := []captured{call("GET", "/api.x.com/executions/3f2b8c1e-9a4d-4e2b-8c1e-9a4d4e2b8c1e/status", `{}`, 200)}
	r := mappingsOf(t, Settings{PathIDs: true}, calls...)[0]
	if p, _ := r.Request["urlPathPattern"].(string); !uuidID.MatchString("0f2b8c1e-9a4d-4e2b-8c1e-9a4d4e2b8c1f") || !strings.Contains(p, uuidPattern) {
		t.Errorf("not matched by its form: %v", r.Request)
	}
	if r := mappingsOf(t, Settings{}, calls...)[0]; r.Request["urlPath"] == nil {
		t.Errorf("matched by its form without the option: %v", r.Request)
	}
	if !ulidID.MatchString("01ARZ3NDEKTSV4RRFFQ69G5FAV") || ulidID.MatchString("ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		t.Error("ulid")
	}
}
