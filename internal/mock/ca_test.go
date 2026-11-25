package mock

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindCA(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	deep := filepath.Join(repo, "flows", "login")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := FindCA(deep); got != "" {
		t.Fatalf("no CA yet, found %s", got)
	}
	outside := filepath.Join(root, filepath.FromSlash(ProjectCA)) // above the repository: not its own
	if err := MakeCA(outside); err != nil {
		t.Fatal(err)
	}
	if got := FindCA(deep); got != "" {
		t.Fatalf("a CA above the repository's root was taken: %s", got)
	}
	project := filepath.Join(repo, filepath.FromSlash(ProjectCA))
	if err := MakeCA(project); err != nil {
		t.Fatal(err)
	}
	if got := FindCA(deep); got != project {
		t.Fatalf("found %q, not %q", got, project)
	}
	if err := MakeCA(project); err == nil {
		t.Fatal("a CA that is there was made again")
	}
	ca, err := LoadCA(project)
	if err != nil || ca.Path != filepath.Join(project, "ca.pem") {
		t.Fatalf("LoadCA: %v %v", ca, err)
	}
	if _, err := LoadCA(filepath.Join(root, "none")); err == nil {
		t.Fatal("a folder without a CA was loaded")
	}
}
