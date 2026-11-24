package test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
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

func TestReferenceHasTheHelp(t *testing.T) {
	out, err := exec.Command(rayo, "help").Output()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../REFERENCE.md")
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile("(?s)```\nrayo - runs.*?```\n")
	if !block.Match([]byte(unix(data))) {
		t.Fatal("REFERENCE.md doesn't have the help")
	}
	generated(t, "../REFERENCE.md", block.ReplaceAllLiteralString(unix(data), "```\n"+unix(out)+"```\n"))
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
	tables["suite unmocked"] = schema.Table(suite.UnmockedFields)
	tables["suite ignored"] = schema.Table(suite.IgnoredFields)
	tables["suite capture"] = schema.Table(suite.CaptureFields)
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

// The documentation's page has the Markdown files and the tables of the code.
func TestPage(t *testing.T) {
	out, err := exec.Command(rayo, "help", "json").Output()
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	names := []string{"GUIDE.md", "EXAMPLE.md", "REFERENCE.md", "MOCKING.md", "CONTRIBUTING.md", "schema/suite.json", "schema/results.json"}
	example, err := os.ReadFile("../EXAMPLE.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range regexp.MustCompile("<!-- file: (.+?) -->").FindAllStringSubmatch(unix(example), -1) {
		names = append(names, m[1])
	}
	for _, name := range names {
		data, err := os.ReadFile("../" + name)
		if err != nil {
			t.Fatal(err)
		}
		files[name] = unix(data)
	}
	var b strings.Builder
	e := json.NewEncoder(&b) // which escapes what could end a script
	e.SetIndent("", " ")
	if err := e.Encode(map[string]any{"markdown": files, "rayo": json.RawMessage(out)}); err != nil {
		t.Fatal(err)
	}
	generated(t, "../docs/data.js", "// Written by go test ./test -update\nwindow.RAYO = "+b.String())
}

// The man page and llms.txt are rayo help all, as each wants it.
func TestHelpAll(t *testing.T) {
	out, err := exec.Command(rayo, "help", "all").Output()
	if err != nil {
		t.Fatal(err)
	}
	all := unix(out)
	parts := strings.Split(strings.TrimPrefix(all, "## "), "\n## ")

	var man strings.Builder
	man.WriteString(".\\\" Written by go test ./test -update, from rayo help all\n.TH RAYO 1 \"\" rayo \"rayo help all\"\n")
	for _, part := range parts {
		title, text, _ := strings.Cut(part, "\n")
		fmt.Fprintf(&man, ".SH %s\n.nf\n", strings.ToUpper(title))
		for _, line := range strings.Split(strings.Trim(text, "\n"), "\n") {
			line = strings.ReplaceAll(line, "\\", "\\e")
			if strings.HasPrefix(line, ".") || strings.HasPrefix(line, "'") {
				line = "\\&" + line
			}
			man.WriteString(line + "\n")
		}
		man.WriteString(".fi\n")
	}
	generated(t, "../man/rayo.1", man.String())

	base := "https://github.com/nanihadesuka/rayo/blob/main/"
	generated(t, "../llms.txt", "# rayo\n\n> End-to-end tests for Android: one executable, flows in YAML, a mock backend built in.\n"+
		"> Written by go test ./test -update: below is rayo help all, the same as rayo help <topic> part by part.\n\n"+
		"## Docs\n\n- ["+"Guide]("+base+"GUIDE.md)\n- [Full example]("+base+"EXAMPLE.md)\n- [Reference]("+base+"REFERENCE.md)\n"+
		"- [Mocking]("+base+"MOCKING.md)\n\n## rayo help all\n\n```\n"+all+"```\n")
}

// The full example is right, and all its files are told of.
func TestFullExample(t *testing.T) {
	for _, what := range []string{"../examples/shop/all.rayo.yaml", "../examples"} {
		if out, err := exec.Command(rayo, "check", what).CombinedOutput(); err != nil {
			t.Errorf("%s", out)
		}
	}
	example, err := os.ReadFile("../EXAMPLE.md")
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir("../examples", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !strings.Contains(string(example), filepath.ToSlash(strings.TrimPrefix(path, ".."+string(filepath.Separator)))) {
			t.Errorf("EXAMPLE.md doesn't show %s", path)
		}
		return err
	})
	if err != nil {
		t.Error(err)
	}
}

// Every example can be copied: rayo accepts it.
func TestExamples(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "login.yaml"), "appId: a.b\n---\n- back\n")
	write(t, filepath.Join(dir, "create_user.js"), "output.name = 1")
	write(t, filepath.Join(dir, "qr.png"), "png")
	write(t, filepath.Join(dir, "overrides.json"), `{"mappings":[]}`)
	try := func(what, text string) {
		if _, err := flow.ParseSteps(filepath.Join(dir, "example.yaml"), text); err != nil {
			t.Errorf("%s: %v\n%s", what, err, text)
		}
	}
	for _, c := range flow.Describe().Commands {
		if !strings.Contains(c.Example, "- "+c.Name) {
			t.Errorf("%s has no example", c.Name)
		}
		try(c.Name, c.Example)
	}
	guide, err := os.ReadFile("../GUIDE.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range regexp.MustCompile("(?s)```yaml\n(- .*?)```").FindAllStringSubmatch(unix(guide), -1) {
		try("GUIDE.md", m[1])
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
