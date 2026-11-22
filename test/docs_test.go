package test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The options are in one table: the help is made from it, and the README has to hold that.
func TestReadmeHasTheHelp(t *testing.T) {
	out, err := exec.Command(rayo, "help").Output()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatal(err)
	}
	unix := func(b []byte) string { return strings.ReplaceAll(string(b), "\r\n", "\n") }
	if !strings.Contains(unix(data), "```\n"+unix(out)+"```\n") {
		t.Error("README.md doesn't have what rayo help prints: paste it there")
	}
}
