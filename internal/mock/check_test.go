package mock

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func mockFolder(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCheckFine(t *testing.T) {
	dir := mockFolder(t, map[string]string{
		"mappings/a.json": `{"mappings": [
			{"request": {"method": "GET", "urlPath": "/a"}, "response": {"bodyFileName": "a.json"}},
			{"request": {"method": "POST", "urlPath": "/a"}, "response": {"status": 201}},
			{"priority": 1, "request": {"method": "GET", "urlPath": "/a"}, "response": {"status": 500}},
			{"scenarioName": "s", "requiredScenarioState": "Started", "request": {"method": "GET", "urlPath": "/a"}, "response": {"status": 202}},
			{"scenarioName": "s", "requiredScenarioState": "done", "request": {"method": "GET", "urlPath": "/a"}, "response": {"status": 203}}
		]}`,
		"__files/a.json": `{}`,
	})
	n, problems, err := Check(dir)
	if err != nil || n != 5 || len(problems) > 0 {
		t.Errorf("Check = %d, %q, %v; want 5 mappings and no problems", n, problems, err)
	}
}

func TestCheckFindsHiddenAndUnused(t *testing.T) {
	dir := mockFolder(t, map[string]string{
		"mappings/a.json":        `{"request": {"method": "GET", "urlPath": "/a", "queryParameters": {"q": {"equalTo": "1"}}}, "response": {"body": "first"}}`,
		"mappings/b.json":        `{"request": {"method": "GET", "urlPath": "/a", "queryParameters": {"q": {"equalTo": "1"}}}, "response": {"body": "second"}}`,
		"__files/old.json":       `{}`,
		"__files/deep/older.png": `x`,
	})
	_, problems, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"a.json never answers: ", "b.json, after it, answers the same calls (GET /a)",
		"__files/deep/older.png: no mapping uses it",
		"__files/old.json: no mapping uses it",
	}
	all := strings.Join(problems, "\n")
	for _, w := range want {
		if !strings.Contains(all, w) {
			t.Errorf("no %q in\n%s", w, all)
		}
	}
	if len(problems) != 3 {
		t.Errorf("%d problems, want 3:\n%s", len(problems), all)
	}
	if !slices.IsSorted(problems[1:]) {
		t.Errorf("unused bodies not in order: %q", problems[1:])
	}
}

func TestCheckOfMocksThatDontLoad(t *testing.T) {
	dir := mockFolder(t, map[string]string{"mappings/a.json": `{"request": {"urlPath": "/a"}, "response": {"bodyFileName": "missing.json"}}`})
	if _, _, err := Check(dir); err == nil || !strings.Contains(err.Error(), "missing.json") {
		t.Errorf("err = %v, want one about missing.json", err)
	}
	if _, _, err := Check(filepath.Join(dir, "nope")); err == nil {
		t.Error("no error for a folder that isn't there")
	}
}

func TestCheckOfAFile(t *testing.T) {
	dir := mockFolder(t, map[string]string{"one.json": `{"mappings": [
		{"request": {"urlPath": "/a"}, "response": {"body": "1"}},
		{"request": {"urlPath": "/a"}, "response": {"body": "2"}}
	]}`})
	n, problems, err := Check(filepath.Join(dir, "one.json"))
	if err != nil || n != 2 || len(problems) != 1 || !strings.Contains(problems[0], "one.json#1 never answers: ") {
		t.Errorf("Check = %d, %q, %v", n, problems, err)
	}
}
