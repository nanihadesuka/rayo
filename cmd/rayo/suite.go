package main

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
	"gopkg.in/yaml.v3"
)

// suiteEnd is how the name of a suite ends.
const suiteEnd = ".rayo.yaml"

// suite is a file that says which flows run together. Its paths are relative to it.
type suite struct {
	Name    string   `yaml:"name"`
	App     *app     `yaml:"app"`
	Flows   []string `yaml:"flows"`
	Output  string   `yaml:"output"`
	Results string   `yaml:"results"`
	Format  string   `yaml:"resultsFormat"`

	Device    *adb.Setup `yaml:"device"`
	Recording yaml.Node  `yaml:"startRecording"`

	path      string
	recording *flow.Recording
}

type app struct {
	ID  string `yaml:"id"`
	APK string `yaml:"apk"`
}

func loadSuite(path string) (*suite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := &suite{path: path}
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	where := filepath.ToSlash(path)
	if err := d.Decode(s); err != nil {
		return nil, fmt.Errorf("%s: %w", where, err)
	}
	if len(s.Flows) == 0 {
		return nil, fmt.Errorf("%s: needs flows, a list of files", where)
	}
	if s.Format != "" {
		if err := checkFormat(s.Format); err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
	}
	if s.Device != nil {
		if err := s.Device.Check(); err != nil {
			return nil, fmt.Errorf("%s: device: %w", where, err)
		}
	}
	if s.Recording.Kind != 0 {
		if s.recording, err = flow.ReadRecording(&s.Recording); err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
		s.recording.Path = s.relative(s.recording.Path)
	}
	if a := s.App; a != nil && (a.APK == "" || a.ID == "") {
		return nil, fmt.Errorf("%s: app needs id and apk", where)
	}
	for i, f := range s.Flows {
		if slices.Contains(s.Flows[:i], f) {
			return nil, fmt.Errorf("%s: %s is listed twice", where, f)
		}
		if info, err := os.Stat(s.relative(f)); err != nil || info.IsDir() {
			return nil, fmt.Errorf("%s: no such flow: %s", where, f)
		}
	}
	return s, nil
}

func (s *suite) relative(path string) string {
	path = filepath.FromSlash(path)
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(filepath.Dir(s.path), path)
}

// install puts the app on the device, unless that same one is there.
func (s *suite) install(d *adb.Device, say bool) error {
	if s.App == nil {
		return nil
	}
	apk, err := os.ReadFile(s.relative(s.App.APK))
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

// load loads every flow, so a mistake in any shows before the first one runs. The suite is nil
// unless one was given.
func load(paths []string) ([]*flow.Flow, *suite, error) {
	if len(paths) == 0 {
		return nil, nil, errors.New("which flow? give a file, a folder or a suite")
	}
	var s *suite
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
			if s, err = loadSuite(p); err != nil {
				return nil, nil, err
			}
			for _, f := range s.Flows {
				files = append(files, s.relative(f))
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
