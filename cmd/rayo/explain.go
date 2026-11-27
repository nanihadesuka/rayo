package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

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
	if len(o.args) > 2 {
		return errors.New("rayo explain takes a run's folder, a .zip of one, or a suite; and a flow")
	}
	folder, which, from := o.output, "", ""
	for _, arg := range o.args {
		info, err := os.Stat(arg)
		switch {
		case err == nil && info.IsDir() && folder == "":
			folder = arg
		case err == nil && strings.EqualFold(filepath.Ext(arg), ".zip") && folder == "":
			folder = arg
		case err == nil && strings.HasSuffix(strings.ToLower(arg), suite.Ext) && folder == "":
			_, group, err := suite.Load([]string{arg})
			if err != nil {
				return err
			}
			folder = ".rayo"
			if group.Output.Folder != "" {
				folder = group.Relative(group.Output.Folder)
			}
		case which == "":
			which = arg
		default:
			return fmt.Errorf("rayo explain takes one flow, and %s is another", arg)
		}
	}
	if folder == "" {
		folder, from = folderWith("failed.json")
	}
	read := readFolder(folder)
	data, err := read("failed.json")
	if err != nil {
		if which != "" {
			return fmt.Errorf("%s hasn't failed: %s has nothing of it", which, filepath.ToSlash(report.LastFile(folder)))
		}
		return fmt.Errorf("nothing failed: there is no %s%s", filepath.ToSlash(report.LastFile(folder)), from)
	}
	last, err := report.ParseLast(data)
	if err != nil {
		return fmt.Errorf("%s: %w", filepath.ToSlash(report.LastFile(folder)), err)
	}
	if o.latest {
		last = report.Latest(last)
	}
	var history []report.Ran
	if data, err := read("history.jsonl"); err == nil {
		history = report.ParseHistory(data)
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
		u := last[k]
		if u.Failure != nil { // what --resume reads, and long
			f := *u.Failure
			f.Trail = []report.StepStates{}
			u.Failure = &f
		}
		u.History = report.Told(history, k)
		out.Flows = append(out.Flows, u)
	}
	if printing != report.Human {
		return emit(out)
	}
	if from != "" {
		fmt.Printf("(%s)\n", strings.TrimPrefix(from, ": "))
	}
	for i, u := range out.Flows {
		if i > 0 {
			fmt.Println()
		}
		fmt.Print(u.Explain())
	}
	return nil
}

// folderWith is the output folder that has a file, when none is said: .rayo, or else that of a suite
// here, or a folder down, whose file is newest. It also says which, if not .rayo.
func folderWith(name string) (folder, from string) {
	folder = ".rayo"
	if _, err := os.Stat(filepath.Join(folder, name)); err == nil {
		return folder, ""
	}
	suites, _ := filepath.Glob("*" + suite.Ext)
	more, _ := filepath.Glob(filepath.Join("*", "*"+suite.Ext))
	var newest time.Time
	for _, path := range append(suites, more...) {
		_, group, err := suite.Load([]string{path})
		if err != nil || group.Output.Folder == "" {
			continue
		}
		there := group.Relative(group.Output.Folder)
		if info, err := os.Stat(filepath.Join(there, name)); err == nil && info.ModTime().After(newest) {
			newest, folder, from = info.ModTime(), there, fmt.Sprintf(": from %s, the output of %s", filepath.ToSlash(there), filepath.ToSlash(path))
		}
	}
	return folder, from
}

// readFolder reads the files of a run's folder, or of a .zip of one, such as CI keeps: there, the
// first file of that name, in whatever folder.
func readFolder(folder string) func(name string) ([]byte, error) {
	if !strings.EqualFold(filepath.Ext(folder), ".zip") {
		return func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(folder, name)) }
	}
	return func(name string) ([]byte, error) {
		z, err := zip.OpenReader(folder)
		if err != nil {
			return nil, err
		}
		defer z.Close()
		for _, f := range z.File {
			if path.Base(f.Name) != name {
				continue
			}
			r, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer r.Close()
			return io.ReadAll(io.LimitReader(r, 256<<20))
		}
		return nil, os.ErrNotExist
	}
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
			if errors.Is(err, exec.ErrNotFound) {
				return nil, fmt.Errorf("--since needs git: %w", err)
			}
			return nil, fmt.Errorf("--since %s: git %s: %w", ref, strings.Join(args, " "), err)
		}
		var lines []string
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				lines = append(lines, line)
			}
		}
		return lines, nil
	}
	root, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, errors.New("--since and --changed-only need a git repository; outside one, name the flows with --only <name>, or give the flow files")
	}
	if len(root) == 0 {
		return nil, errors.New("--since needs a git repository")
	}
	top := filepath.FromSlash(root[0])
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

// pathKey is a path as files are told apart: through links and, on Windows, short names, whatever the case.
func pathKey(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	} else if dir, err := filepath.EvalSymlinks(filepath.Dir(path)); err == nil {
		path = filepath.Join(dir, filepath.Base(path))
	}
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
