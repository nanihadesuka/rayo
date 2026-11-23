// Package suite finds the flows to run, also those that a suite lists.
package suite

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nanihadesuka/rayo/internal/adb"
	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/schema"
	"gopkg.in/yaml.v3"
)

// suiteEnd is how the name of a suite ends.
const suiteEnd = ".rayo.yaml"

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
	TimeoutScale float64 // 0 if not said

	Path string
}

// LowestScale and HighestScale bound timeoutScale and --timeout-scale.
const LowestScale, HighestScale = 0.1, 10.0

// DefaultRetries is how many times a flow is tried again, if nothing says.
const DefaultRetries = 1

// entry is a flow of a suite: its file, or a mapping with what is only for it.
type entry struct {
	file    string
	retries *int // nil for the suite's
	line    int
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
