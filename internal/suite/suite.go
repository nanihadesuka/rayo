// Package suite finds the flows to run, also those that a suite lists.
package suite

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nanihadesuka/rayo/internal/adb"
	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/report"
	"gopkg.in/yaml.v3"
)

// suiteEnd is how the name of a suite ends.
const suiteEnd = ".rayo.yaml"

// Suite is a file that says which flows run together. Its paths are relative to it.
type Suite struct {
	Name    string  `yaml:"name"`
	App     *app    `yaml:"app"`
	Flows   []entry `yaml:"flows"`
	Retries *int    `yaml:"retries"`
	Output  string  `yaml:"output"`
	Results files   `yaml:"results"`

	Device       *adb.Setup `yaml:"device"`
	Record       bool       `yaml:"record"`
	RecordFolder string     `yaml:"recordFolder"`

	Path string `yaml:"-"`
}

// files is one or a list.
type files []string

// UnmarshalYAML reads it.
func (f *files) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*f = files{n.Value}
		return nil
	}
	return n.Decode((*[]string)(f))
}

// DefaultRetries is how many times a flow is tried again, if nothing says.
const DefaultRetries = 1

// entry is a flow of a suite: its file, or a mapping with what is only for it.
type entry struct {
	file    string
	retries *int // nil for the suite's
}

// UnmarshalYAML reads it. Errors say the line, as yaml's own do.
func (e *entry) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		e.file = n.Value
		return nil
	}
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: a flow is a file, or file and retries", n.Line)
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, v := n.Content[i], n.Content[i+1]
		var err error
		switch key.Value {
		case "file":
			err = v.Decode(&e.file)
		case "retries":
			e.retries = new(int)
			err = v.Decode(e.retries)
		default:
			err = fmt.Errorf("line %d: a flow takes file and retries, not %q", key.Line, key.Value)
		}
		if err != nil {
			return err
		}
	}
	if e.file == "" {
		return fmt.Errorf("line %d: a flow needs its file", n.Line)
	}
	return nil
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

type app struct {
	ID  string `yaml:"id"`
	APK string `yaml:"apk"`
}

func read(path string) (*Suite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := &Suite{Path: path}
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	where := filepath.ToSlash(path)
	if err := d.Decode(s); err != nil {
		return nil, fmt.Errorf("%s: %w", where, err)
	}
	if len(s.Flows) == 0 {
		return nil, fmt.Errorf("%s: needs flows, a list of files", where)
	}
	for _, file := range s.Results {
		if _, err := report.FormatOf(file); err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
	}
	if s.Device != nil {
		if err := s.Device.Check(); err != nil {
			return nil, fmt.Errorf("%s: device: %w", where, err)
		}
	}
	if s.RecordFolder != "" && !s.Record {
		return nil, fmt.Errorf("%s: recordFolder needs record: true", where)
	}
	if a := s.App; a != nil && (a.APK == "" || a.ID == "") {
		return nil, fmt.Errorf("%s: app needs id and apk", where)
	}
	if s.Retries != nil && *s.Retries < 0 {
		return nil, fmt.Errorf("%s: retries can't be %d", where, *s.Retries)
	}
	for i, e := range s.Flows {
		if slices.ContainsFunc(s.Flows[:i], func(o entry) bool { return o.file == e.file }) {
			return nil, fmt.Errorf("%s: %s is listed twice", where, e.file)
		}
		if info, err := os.Stat(s.Relative(e.file)); err != nil || info.IsDir() {
			return nil, fmt.Errorf("%s: no such flow: %s", where, e.file)
		}
		if e.retries != nil && *e.retries < 0 {
			return nil, fmt.Errorf("%s: %s: retries can't be %d", where, e.file, *e.retries)
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
		if strings.HasSuffix(p, suiteEnd) && !info.IsDir() {
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
		if !info.IsDir() {
			files = append(files, p)
			continue
		}
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			ext := filepath.Ext(path)
			if err == nil && !d.IsDir() && (ext == ".yaml" || ext == ".yml") && d.Name() != "config.yaml" && !strings.HasSuffix(path, suiteEnd) {
				files = append(files, path)
			}
			return err
		})
		if err != nil {
			return nil, nil, err
		}
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
	return flows, s, nil
}
