package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nanihadesuka/rayo/internal/flow"
)

func TestChanged(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	dir := t.TempDir()
	write := func(name, text string) {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write("a.yaml", "appId: a.b\n---\n- runFlow: shared/sub.yaml\n")
	write("b.yaml", "appId: a.b\nmock: m\n---\n- back\n")
	write("c.yaml", "appId: a.b\n---\n- back\n")
	write("shared/sub.yaml", "appId: a.b\n---\n- back\n")
	write("m/mappings/x.json", `{"request":{"urlPath":"/a"},"response":{"body":"ok"}}`)
	git("init", "-q")
	git("add", ".")
	git("commit", "-qm", "x")
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	var flows []*flow.Flow
	for _, name := range []string{"a.yaml", "b.yaml", "c.yaml"} {
		f, err := flow.Load(name)
		if err != nil {
			t.Fatal(err)
		}
		flows = append(flows, f)
	}
	write("shared/sub.yaml", "appId: a.b\n---\n- back\n- back\n")
	write("m/mappings/y.json", `{"request":{"urlPath":"/b"},"response":{"body":"ok"}}`)
	got, err := (&options{since: "HEAD"}).changed(flows)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range got {
		names = append(names, f.Name)
	}
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Errorf("changed: %v, want [a b]", names)
	}
}
