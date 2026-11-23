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
