package test

import (
	"maps"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/report"
	"github.com/nanihadesuka/rayo/internal/schema"
	"github.com/nanihadesuka/rayo/internal/suite"
)

// What is generated from the tables in the code: -update writes it.

func unix(data []byte) string { return strings.ReplaceAll(string(data), "\r\n", "\n") }

func generated(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	switch {
	case *update:
		write(t, path, want)
	case err != nil:
		t.Error(err)
	case unix(data) != want:
		t.Errorf("%s is out of date: go test ./test -update", path)
	}
}

func TestSchemas(t *testing.T) {
	generated(t, "../schema/flow.json", string(flow.Schema()))
	generated(t, "../schema/suite.json", string(suite.Schema()))
	generated(t, "../schema/results.json", string(report.Schema()))
}

func TestReadmeHasTheHelp(t *testing.T) {
	out, err := exec.Command(rayo, "help").Output()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile("(?s)```\nrayo - runs.*?```\n")
	if !block.Match([]byte(unix(data))) {
		t.Fatal("README.md doesn't have the help")
	}
	generated(t, "../README.md", block.ReplaceAllLiteralString(unix(data), "```\n"+unix(out)+"```\n"))
}

func TestReferenceTables(t *testing.T) {
	data, err := os.ReadFile("../REFERENCE.md")
	if err != nil {
		t.Fatal(err)
	}
	tables := flow.Tables()
	maps.Copy(tables, report.Tables())
	tables["suite"] = schema.Table(suite.Fields)
	tables["suite flow"] = schema.Table(suite.FlowFields)
	tables["suite app"] = schema.Table(suite.AppFields)
	tables["device"] = schema.Table(suite.DeviceFields)
	used := map[string]bool{}
	block := regexp.MustCompile("(?s)<!-- generated: (.+?) -->\n.*?<!-- end -->")
	text := block.ReplaceAllStringFunc(unix(data), func(old string) string {
		name := block.FindStringSubmatch(old)[1]
		used[name] = true
		if tables[name] == "" {
			t.Errorf("there is no %q to generate", name)
			return old
		}
		return "<!-- generated: " + name + " -->\n" + tables[name] + "<!-- end -->"
	})
	generated(t, "../REFERENCE.md", text)
	same := map[string]string{"killApp": "stopApp", "inputRandomNumber": "", "inputRandomText": ""} // told together
	for name := range tables {
		if command, ok := strings.CutPrefix(name, "keys "); ok {
			if other, told := same[command]; told && (other == "" || used["keys "+other]) {
				continue
			}
		}
		if !used[name] {
			t.Errorf("REFERENCE.md has no table of %s", name)
		}
	}
}

// Each command has its part in the reference, marked if it hasn't been tested on a device.
func TestReferenceHasEveryCommand(t *testing.T) {
	data, err := os.ReadFile("../REFERENCE.md")
	if err != nil {
		t.Fatal(err)
	}
	marked := map[string]bool{}
	for _, m := range regexp.MustCompile("(?m)^### (.+)\n\n(.)").FindAllStringSubmatch(unix(data), -1) {
		for _, name := range strings.Split(m[1], ", ") {
			marked[strings.TrimSuffix(name, " step")] = m[2] == "🧪"
		}
	}
	for _, name := range flow.Commands() {
		heading := name
		if strings.HasPrefix(name, "inputRandom") {
			heading = "inputRandom…"
		}
		mark, there := marked[heading]
		if !there {
			t.Errorf("REFERENCE.md says nothing of %s", name)
		} else if mark == flow.Tested(name) {
			t.Errorf("%s: REFERENCE.md and commands.go don't agree on whether it was tested on a device", name)
		}
	}
}
