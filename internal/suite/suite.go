// Package suite finds the flows to run, also those that a suite lists.
package suite

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

// inFolder is what a folder means: its suite, if it holds one, else its flows. A file with the
// old name of a suite fails, saying the rename.
func inFolder(folder string) (suites, flows []string, err error) {
	err = filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := filepath.Ext(path)
		switch {
		case strings.HasSuffix(path, oldEnd):
			return renamed(path)
		case IsSuite(path):
			suites = append(suites, path)
		case (ext == ".yaml" || ext == ".yml") && d.Name() != "config.yaml":
			flows = append(flows, path)
		}
		return nil
	})
	return suites, flows, err
}

// Suite is a file that says which flows run together. Its paths are relative to it.
type Suite struct {
	Name         string
	App          *app
	Flows        []entry
	Retries      *int
	Output       string
	Results      []string
	Device       *adb.Setup
	Record       bool
	RecordFolder string
	Keep         string            // empty if not said
	Artifacts    []string          // nil if not said
	NameTemplate string            // empty if not said
	TimeoutScale float64           // 0 if not said
	Env          map[string]string // for every flow; -e wins
	Network      string            // for every flow that doesn't say; --network wins
	Unmocked     *mock.Unmocked    // calls without a mock that don't fail any flow; nil for none
	Capture      *Capture          // for every flow; nil if not said

	Path string
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
	where := filepath.ToSlash(path)
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", where, err)
	}
	if len(doc.Content) == 0 {
		return nil, fmt.Errorf("%s: is empty", where)
	}
	s := &Suite{Path: path}
	if err := schema.Read(Fields, s, doc.Content[0]); err != nil {
		var here *schema.Error
		if errors.As(err, &here) {
			return nil, fmt.Errorf("%s:%d: %s", where, here.Line, here.What)
		}
		return nil, fmt.Errorf("%s: %w", where, err)
	}
	if s.RecordFolder != "" && !s.Record {
		return nil, fmt.Errorf("%s: recordFolder needs record: true", where)
	}
	for _, e := range s.Flows {
		if info, err := os.Stat(s.Relative(e.file)); err != nil || info.IsDir() {
			return nil, fmt.Errorf("%s:%d: no such flow: %s", where, e.line, e.file)
		}
		for _, m := range e.mock {
			if err := mock.New().Load(s.Relative(m)); err != nil {
				return nil, fmt.Errorf("%s:%d: mock: %v", where, e.line, err)
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
		if err != nil {
			return nil, nil, err
		}
		if strings.HasSuffix(p, oldEnd) && !info.IsDir() {
			return nil, nil, renamed(p)
		}
		if info.IsDir() {
			suites, inside, err := inFolder(p)
			if err != nil {
				return nil, nil, err
			}
			switch {
			case len(suites) > 1:
				for i := range suites {
					suites[i] = filepath.ToSlash(suites[i])
				}
				return nil, nil, fmt.Errorf("%s holds %d suites, give one: %s", filepath.ToSlash(p), len(suites), strings.Join(suites, ", "))
			case len(suites) == 0:
				files = append(files, inside...)
				continue
			}
			p = suites[0]
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
	var bad []string
	for _, file := range files {
		f, err := flow.Load(file)
		if err != nil {
			bad = append(bad, err.Error())
			continue
		}
		flows = append(flows, f)
	}
	if len(bad) > 0 {
		return nil, nil, errors.New(strings.Join(bad, "\n       "))
	}
	if len(flows) == 0 {
		return nil, nil, fmt.Errorf("no flows in %s", strings.Join(paths, ", "))
	}
	if s != nil {
		if err := s.fill(flows); err != nil {
			return nil, nil, err
		}
	}
	return flows, s, nil
}
