package flow

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// A flow or a script saved on Windows, with \r\n, reads as one with \n.
func TestCRLF(t *testing.T) {
	dir := t.TempDir()
	js := filepath.Join(dir, "a.js")
	os.WriteFile(js, []byte("output.text = `one\r\ntwo`;\r\noutput.n = output.text.length;\r\n"), 0o644)
	s := newScript(context.Background(), func(string) {})
	if _, err := s.eval("setup", "var output = {}"); err != nil {
		t.Fatal(err)
	}
	if err := s.run(js); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.eval("check", "output.text"); v.String() != "one\ntwo" {
		t.Errorf("template literal: %q", v.String())
	}

	f, err := Parse(filepath.Join(dir, "f.yaml"), []byte("appId: a.b\r\n---\r\n- evalScript: |\r\n    a = 1\r\n    b = 2\r\n- inputText: \"x\"\r\n- back\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	steps := f.Steps
	if len(steps) != 3 {
		t.Fatalf("%d steps", len(steps))
	}
	if got := steps[2].Place(); got != filepath.ToSlash(filepath.Join(dir, "f.yaml"))+":7" {
		t.Errorf("line of the last step: %s", got)
	}
}
