// Package suite finds the flows to run, also those that a suite lists.
package suite

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nanihadesuka/rayo/internal/adb"
	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/mock"
	"github.com/nanihadesuka/rayo/internal/schema"
	"gopkg.in/yaml.v3"
)

// suiteEnd is how the name of a suite ends.
const suiteEnd = ".suite.yaml"

// oldEnd is how it ended before; such a file is no longer read, but said how to rename.
const oldEnd = ".rayo.yaml"

// IsSuite says if a path is the name of a suite.
func IsSuite(path string) bool { return strings.HasSuffix(path, suiteEnd) }

// renamed is the mistake of a suite with the old name: the exact rename.
func renamed(path string) error {
	name := filepath.Base(path)
	return fmt.Errorf("%s: suites are now *%s, rename it: %s -> %s", filepath.ToSlash(path), suiteEnd, name, strings.TrimSuffix(name, oldEnd)+suiteEnd)
}

// inFolder is what a folder means: the flows directly in it. Its subfolders hold what flows share,
// so they are left out, and a suite in it runs only when its file is given: it is skipped, saying so.
func inFolder(folder string) (flows []string, err error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, err
	}
	for _, d := range entries {
		path, ext := filepath.Join(folder, d.Name()), filepath.Ext(d.Name())
		switch {
		case d.IsDir():
		case strings.HasSuffix(path, oldEnd):
			fmt.Fprintf(os.Stderr, "skipped %s (a suite; rename it to %s)\n", filepath.ToSlash(path), strings.TrimSuffix(d.Name(), oldEnd)+suiteEnd)
		case IsSuite(path):
			fmt.Fprintf(os.Stderr, "skipped %s (a suite; name it to run it)\n", filepath.ToSlash(path))
		case (ext == ".yaml" || ext == ".yml") && d.Name() != "config.yaml":
			flows = append(flows, path)
		}
	}
	return flows, nil
}

// Suite is a file that says which flows run together. Its paths are relative to it.
type Suite struct {
	Name            string
	App             *app
	Flows           []entry
	Retries         *int
	Output          Output
	Results         []string
	Format          string // of what is printed; --format wins
	Device          *adb.Setup
	Detail          string   // empty if not said
	FailureWindow   int      // seconds; 0 if not said
	Show            []string // nil if not said
	Slowest         int      // 0 if not said
	MinFlows        int      // 0 if not said
	ResultsTemplate string   // empty if not said
	FlakyFailsBuild bool
	TimeoutScale    float64           // 0 if not said
	Env             map[string]string // for every flow; -e wins
	Network         string            // for every flow that doesn't say; --network wins
	Unmocked        *mock.Unmocked    // calls without a mock that don't fail any flow; nil for none
	Capture         *Capture          // for every flow; nil if not said
	CA              string            // the folder of the CA the proxy signs with, relative to the suite; --ca wins

	Path string
}

// Output is what a run writes, and where.
type Output struct {
	Folder       string   // empty if not said
	Keep         string   // empty if not said
	Video        *bool    // nil if not said
	Artifacts    []string // nil if not said
	NameTemplate string   // empty if not said
}

// LowestScale and HighestScale bound timeoutScale and --timeout-scale.
const LowestScale, HighestScale = 0.1, 10.0

// DefaultRetries is how many times a flow is tried again, if nothing says.
const DefaultRetries = 1

// entry is a flow of a suite: its file, or a mapping with what is only for it.
type entry struct {
	file     string
	retries  *int           // nil for the suite's
	mock     []string       // for the flow, if its header has none
	network  string         // for the flow, if its header doesn't say; else the suite's
	unmocked *mock.Unmocked // added to the suite's, for the flow
	capture  *Capture       // over the suite's, for the flow
	line     int
}

// RetriesOf says how many times a flow may be tried again.
func (s *Suite) RetriesOf(f *flow.Flow) int {
	for _, e := range s.Flows {
		if e.retries != nil && s.Relative(e.file) == f.Path {
			return *e.retries
		}
	}
	if s.Retries != nil {
		return *s.Retries
	}
	return DefaultRetries
}

// CaptureOf is how a capture of a flow is made, as its entry says over the suite; nil if neither says.
func (s *Suite) CaptureOf(f *flow.Flow) *Capture {
	for _, e := range s.Flows {
		if s.Relative(e.file) == f.Path {
			return e.capture.Or(s.Capture)
		}
	}
	return s.Capture
}

// fill gives each flow of the suite the mock and network the suite says for it, unless its own
// header says: a flow's header wins over its entry, and that over the suite's network. --network
// wins over all of them when the flow runs.
func (s *Suite) fill(flows []*flow.Flow) error {
	for _, f := range flows {
		var e entry
		for _, o := range s.Flows {
			if s.Relative(o.file) == f.Path {
				e = o
			}
		}
		if len(f.Mock) == 0 {
			for _, m := range e.mock {
				path, err := filepath.Abs(s.Relative(m))
				if err != nil {
					return err
				}
				f.Mock = append(f.Mock, &yaml.Node{Kind: yaml.ScalarNode, Value: path, Line: e.line})
			}
		}
		if f.Network == "" {
			f.Network = cmp.Or(e.network, s.Network)
		}
		f.Unmocked = s.Unmocked.Merge(e.unmocked)
	}
	return nil
}

type app struct{ ID, APK string }

func read(path string) (*Suite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, &flow.Problem{Path: path, What: err.Error(), Err: err}
	}
	if len(doc.Content) == 0 {
		return nil, &flow.Problem{Path: path, What: "is empty"}
	}
	s := &Suite{Path: path}
	if top := doc.Content[0]; top.Kind == yaml.MappingNode {
		for i := 0; i < len(top.Content); i += 2 {
			key := top.Content[i]
			if to, ok := Moved[key.Value]; ok {
				return nil, &flow.Problem{Path: path, Line: key.Line, What: fmt.Sprintf("%s is now %s (rayo 3.0 put what a run writes under output, and spells every key in camelCase)", key.Value, to)}
			}
			if under := elsewhere(key.Value); under != "" {
				return nil, &flow.Problem{Path: path, Line: key.Line, What: fmt.Sprintf("unknown key %q, did you mean %s? It belongs under %s: { %s: ... }", key.Value, under+"."+key.Value, under, key.Value)}
			}
		}
	}
	if err := schema.Read(Fields, s, doc.Content[0]); err != nil {
		var here *schema.Error
		if errors.As(err, &here) {
			return nil, &flow.Problem{Path: path, Line: here.Line, What: here.What, Expected: here.Expected}
		}
		return nil, &flow.Problem{Path: path, What: err.Error(), Err: err}
	}
	for _, e := range s.Flows {
		if info, err := os.Stat(s.Relative(e.file)); err != nil || info.IsDir() {
			return nil, &flow.Problem{Path: path, Line: e.line, What: "no such flow: " + e.file}
		}
		for _, m := range e.mock {
			if err := mock.New().Load(s.Relative(m)); err != nil {
				return nil, &flow.Problem{Path: path, Line: e.line, What: "mock: " + err.Error(), Err: err}
			}
		}
	}
	return s, nil
}

// Relative is a path written in the suite.
func (s *Suite) Relative(path string) string {
	path = filepath.FromSlash(path)
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(filepath.Dir(s.Path), path)
}

// Install puts the app on the device, unless that same one is there.
func (s *Suite) Install(d *adb.Device, say bool) error {
	if s.App == nil {
		return nil
	}
	apk, err := os.ReadFile(s.Relative(s.App.APK))
	if err != nil {
		return err
	}
	did, err := d.Install(s.App.ID, apk)
	if err != nil {
		return fmt.Errorf("couldn't install %s: %w", s.App.ID, err)
	}
	if did && say {
		fmt.Println("installed", s.App.ID)
	}
	return nil
}

// Load loads every flow, so a mistake in any shows before the first one runs. The suite is nil
// unless one was given.
func Load(paths []string) ([]*flow.Flow, *Suite, error) {
	if len(paths) == 0 {
		return nil, nil, errors.New("which flow? give a file, a folder or a suite")
	}
	var s *Suite
	var files []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, missing(p)
		}
		if err != nil {
			return nil, nil, err
		}
		if strings.HasSuffix(p, oldEnd) && !info.IsDir() {
			return nil, nil, renamed(p)
		}
		if info.IsDir() {
			inside, err := inFolder(p)
			if err != nil {
				return nil, nil, err
			}
			files = append(files, inside...)
			continue
		}
		if IsSuite(p) {
			if s != nil {
				return nil, nil, errors.New("one suite at a time")
			}
			if s, err = read(p); err != nil {
				return nil, nil, err
			}
			for _, f := range s.Flows {
				files = append(files, s.Relative(f.file))
			}
			continue
		}
		files = append(files, p)
	}
	var flows []*flow.Flow
	var bad Problems
	for _, file := range files {
		f, err := flow.Load(file)
		if err != nil {
			bad = append(bad, err)
			continue
		}
		flows = append(flows, f)
	}
	if len(bad) > 0 {
		return nil, nil, bad
	}
	if len(flows) == 0 {
		return nil, nil, fmt.Errorf("no flows in %s", strings.Join(paths, ", "))
	}
	named := map[string]*flow.Flow{} // by the name of their files' folder, which a file system may not tell apart by case
	for _, f := range flows {
		key := strings.ToLower(flow.FolderName(f.Name))
		if other, ok := named[key]; ok && other.Path != f.Path {
			bad = append(bad, &flow.Problem{Path: f.Path, What: fmt.Sprintf("is named %s, as %s is: --only and the output folder can't tell them apart; give one a name: in its header", f.Name, filepath.ToSlash(other.Path))})
		}
		named[key] = f
	}
	if len(bad) > 0 {
		return nil, nil, bad
	}
	if s != nil {
		if err := s.fill(flows); err != nil {
			return nil, nil, err
		}
	}
	return flows, s, nil
}

// Problems are those of several flows, each on a line of its own.
type Problems []error

func (p Problems) Error() string {
	var lines []string
	for _, err := range p {
		lines = append(lines, err.Error())
	}
	return strings.Join(lines, "\n       ")
}

func (p Problems) Unwrap() []error { return p }

// elsewhere is the key of a suite that has a key of that name, when the suite itself has none;
// empty if none has.
func elsewhere(name string) string {
	if slices.ContainsFunc(Fields, func(f schema.Field[Suite]) bool { return f.Name == name }) {
		return ""
	}
	for _, in := range []struct {
		key   string
		names []string
	}{{"output", fieldNames(OutputFields)}, {"capture", fieldNames(CaptureFields)}, {"device", fieldNames(DeviceFields)}, {"app", fieldNames(AppFields)}} {
		if slices.Contains(in.names, name) {
			return in.key
		}
	}
	return ""
}

func fieldNames[T any](fields []schema.Field[T]) []string {
	var list []string
	for _, f := range fields {
		list = append(list, f.Name)
	}
	return list
}

// missing is the error of a path that isn't there, and the nearest of what is next to it.
func missing(path string) error {
	err := fmt.Errorf("%s isn't there; give a flow file, a folder or a *%s", filepath.ToSlash(path), suiteEnd)
	entries, _ := os.ReadDir(filepath.Dir(path))
	var names []string
	for _, d := range entries {
		names = append(names, d.Name())
	}
	if near := schema.Nearest(filepath.Base(path), names); near != "" {
		return fmt.Errorf("%w; did you mean %s?", err, filepath.ToSlash(filepath.Join(filepath.Dir(path), near)))
	}
	return err
}
