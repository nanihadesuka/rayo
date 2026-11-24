package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/report"
	"github.com/nanihadesuka/rayo/internal/suite"
)

// explain prints why the flows of the last run failed, in a few lines each, from <output>/failed.json.
func explain(args []string) error {
	o, err := parse(cmdExplain, args)
	if err != nil {
		return err
	}
	if len(o.args) > 1 {
		return errors.New("rayo explain takes a run's folder, or a flow")
	}
	folder, which := o.output, ""
	if len(o.args) == 1 {
		if info, err := os.Stat(o.args[0]); err == nil && info.IsDir() {
			folder = o.args[0]
		} else {
			which = o.args[0]
		}
	}
	if folder == "" {
		folder = ".rayo"
	}
	file := report.LastFile(folder)
	if _, err := os.Stat(file); err != nil {
		if which != "" {
			return fmt.Errorf("%s hasn't failed: %s has nothing of it", which, filepath.ToSlash(file))
		}
		return fmt.Errorf("nothing failed: there is no %s", filepath.ToSlash(file))
	}
	last, err := report.Last(file)
	if err != nil {
		return err
	}
	id := ""
	if which != "" {
		id = report.Identity(&flow.Flow{Path: which})
	}
	var keys []string
	for k, u := range last {
		if which == "" || k == id || u.Name == which {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return fmt.Errorf("%s hasn't failed since it last passed", which)
	}
	slices.Sort(keys)
	out := report.Explained{SchemaVersion: report.SchemaVersion, Flows: []report.Unpassed{}}
	for _, k := range keys {
		out.Flows = append(out.Flows, last[k])
	}
	if printing != report.Human {
		return emit(out)
	}
	for i, u := range out.Flows {
		if i > 0 {
			fmt.Println()
		}
		fmt.Print(u.Explain())
	}
	return nil
}

// changed keeps the flows of which a file changed since a git ref: their own, those they run, or their mocks.
func (o *options) changed(flows []*flow.Flow) ([]*flow.Flow, error) {
	if o.since == "" {
		return flows, nil
	}
	files, err := changedFiles(o.since)
	if err != nil {
		return nil, err
	}
	var kept []*flow.Flow
	for _, f := range flows {
		if slices.ContainsFunc(f.Files(), func(path string) bool {
			abs, err := filepath.Abs(path)
			return err == nil && files[pathKey(abs)]
		}) {
			kept = append(kept, f)
		}
	}
	return kept, nil
}

// changedFiles are those that differ from a git ref, also those not committed and those git doesn't
// know yet, as absolute paths.
func changedFiles(ref string) (map[string]bool, error) {
	git := func(args ...string) ([]string, error) {
		out, err := exec.Command("git", args...).Output()
		if err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) && len(exit.Stderr) > 0 {
				return nil, fmt.Errorf("--since %s: %s", ref, strings.TrimSpace(string(exit.Stderr)))
			}
			return nil, fmt.Errorf("--since needs git: %w", err)
		}
		return strings.Fields(strings.TrimSpace(string(out))), nil
	}
	root, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, errors.New("--since needs a git repository")
	}
	if len(root) == 0 {
		return nil, errors.New("--since needs a git repository")
	}
	top := strings.Join(root, " ")
	diff, err := git("-C", top, "diff", "--name-only", ref)
	if err != nil {
		return nil, err
	}
	fresh, err := git("-C", top, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	files := map[string]bool{}
	for _, f := range append(diff, fresh...) {
		files[pathKey(filepath.Join(top, filepath.FromSlash(f)))] = true
	}
	return files, nil
}

// pathKey is a path as files are told apart: on Windows, whatever the case.
func pathKey(path string) string {
	path = filepath.ToSlash(path)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return path
}

// findOn keeps the lines of a screen that match a text, or are near it.
func findOn(screen, text string) string {
	lines := flow.Nearest(screen, text, 20)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// plan is what rayo check prints of the flows.
func plan(flows []*flow.Flow, s *suite.Suite) []report.Planned {
	list := []report.Planned{}
	for _, f := range flows {
		p := report.Planned{Name: f.Name, File: filepath.ToSlash(f.Path), Tags: report.List(f.Tags), Mocks: []string{}, Retries: suite.DefaultRetries}
		for _, m := range f.Mock {
			p.Mocks = append(p.Mocks, filepath.ToSlash(m.Value))
		}
		if s != nil {
			p.Retries = s.RetriesOf(f)
		}
		list = append(list, p)
	}
	return list
}
