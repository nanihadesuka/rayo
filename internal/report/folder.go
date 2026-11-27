package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// What output.clean and --clean take.
const (
	CleanNone = "none" // nothing is removed: files of older runs stay beside the new
	CleanAuto = "auto" // a whole run begins with none; a partial one removes a flow's when it starts
	CleanAll  = "all"  // every run begins with none
)

// Cleans are what output.clean takes.
var Cleans = []string{CleanNone, CleanAuto, CleanAll}

// FilesFile is where the files each flow's last run wrote are listed, with the run's id.
func FilesFile(output string) string { return filepath.Join(output, "files.json") }

// Written is the files a flow's last run wrote, as files.json keeps them.
type Written struct {
	Name  string   `json:"name" about:"The flow's"`
	File  string   `json:"file" about:"The flow's, as given"`
	Run   string   `json:"run" about:"The id of the run that wrote them, as the results' run"`
	Files []string `json:"files" about:"Relative to the output folder; those of every try that is kept"`
	Older []string `json:"older,omitempty" about:"Files of its earlier runs still there, with output: { clean: none }"`
}

// Manifest is what files.json has, by Identity.
type Manifest map[string]Written

// ReadManifest is what files.json has; none if there is none or it can't be read.
func ReadManifest(output string) Manifest {
	m := Manifest{}
	if data, err := os.ReadFile(FilesFile(output)); err == nil {
		_ = json.Unmarshal(data, &m)
	}
	return m
}

// Save writes it, or removes the file if it is empty.
func (m Manifest) Save(output string) error {
	if len(m) == 0 {
		if err := os.Remove(FilesFile(output)); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := Encode(m, true)
	if err != nil {
		return err
	}
	return WriteFile(FilesFile(output), data)
}

// Note sets the files a flow wrote in a run, those that are still there.
func (m Manifest) Note(output, identity, name, file, run string, files []string) {
	var kept []string
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			continue
		}
		if rel, err := filepath.Rel(output, f); err == nil && !strings.HasPrefix(rel, "..") {
			f = rel
		}
		if f = filepath.ToSlash(f); !slices.Contains(kept, f) {
			kept = append(kept, f)
		}
	}
	var older []string
	if w, there := m[identity]; there && w.Run != run {
		for _, f := range append(w.Older, w.Files...) {
			path := f
			if !filepath.IsAbs(path) {
				path = filepath.Join(output, filepath.FromSlash(f))
			}
			if _, err := os.Stat(path); err == nil && !slices.Contains(kept, f) && !slices.Contains(older, f) {
				older = append(older, f)
			}
		}
	}
	if len(kept) == 0 && len(older) == 0 {
		delete(m, identity)
		return
	}
	if kept == nil {
		kept = []string{}
	}
	m[identity] = Written{Name: name, File: filepath.ToSlash(file), Run: run, Files: kept, Older: older}
}

// Clean removes the files an earlier run wrote for a flow, and the folders they leave empty.
func (m Manifest) Clean(output, identity string) error {
	w, there := m[identity]
	if !there {
		return nil
	}
	delete(m, identity)
	var err error
	for _, f := range append(w.Older, w.Files...) {
		path := f
		if !filepath.IsAbs(path) {
			path = filepath.Join(output, filepath.FromSlash(f))
		}
		if e := os.Remove(path); e != nil && !os.IsNotExist(e) {
			err = errors.Join(err, e)
			continue
		}
		for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
			rel, e := filepath.Rel(output, dir)
			if e != nil || rel == "." || strings.HasPrefix(rel, "..") || os.Remove(dir) != nil {
				break
			}
		}
	}
	return err
}

// CleanAllFiles removes the files of every flow it lists.
func (m Manifest) CleanAllFiles(output string) error {
	var err error
	for id := range m {
		err = errors.Join(err, m.Clean(output, id))
	}
	return err
}

// LockFile is there while a run writes to the folder.
func LockFile(output string) string { return filepath.Join(output, "running.json") }

// Lock is a run that writes to a folder.
type Lock struct {
	Run    string `json:"run"`
	Pid    int    `json:"pid"`
	Device string `json:"device"`
	Began  string `json:"began"`
}

// ReadLock is the run that holds the folder; nil if none.
func ReadLock(output string) *Lock {
	data, err := os.ReadFile(LockFile(output))
	if err != nil {
		return nil
	}
	var l Lock
	if json.Unmarshal(data, &l) != nil {
		return &Lock{} // half written: its run was killed as it began
	}
	return &l
}

// Take holds the folder for a run: an error if another run that is still going holds it. One that was
// killed left its lock, and is said to have been interrupted.
func Take(output string, l Lock, alive func(pid int) bool, override bool) (stale *Lock, err error) {
	if old := ReadLock(output); old != nil {
		if old.Pid != 0 && old.Pid != os.Getpid() && alive(old.Pid) && !override {
			return nil, fmt.Errorf("another rayo (pid %d, run %s, on %s, since %s) writes to %s: wait for it, use another --output, or --ignore-lock if it is gone",
				old.Pid, old.Run, cmpOr(old.Device, "a device"), cmpOr(old.Began, "?"), filepath.ToSlash(output))
		}
		if old.Pid != os.Getpid() {
			stale = old
		}
	}
	l.Pid = os.Getpid()
	if l.Began == "" {
		l.Began = stamp(time.Now())
	}
	data, err := json.Marshal(l)
	if err != nil {
		return nil, err
	}
	return stale, WriteFile(LockFile(output), data)
}

// Release lets the folder go, if this process holds it.
func Release(output string) {
	if l := ReadLock(output); l != nil && l.Pid == os.Getpid() {
		_ = os.Remove(LockFile(output))
	}
}

func cmpOr(s, or string) string {
	if s == "" {
		return or
	}
	return s
}

// Signature is where and why a flow failed, the same for flows that failed alike; empty if it didn't.
func (o *Outcome) Signature() string {
	if o.Status != Failed {
		return ""
	}
	_, at, why, _, _ := o.failing()
	if at == "" && why == "" {
		return ""
	}
	return at + ": " + why
}

// Unfinished is the results of a run that has begun: every flow interrupted, so that a run that is killed
// leaves results that say so, not those of an earlier run, nor none.
func (s *Summary) Unfinished() *Summary {
	u := *s
	u.Outcomes = nil
	for _, o := range s.Outcomes {
		o.Status, o.Why = Interrupted, "the run didn't finish: it was killed, or is still going"
		u.Outcomes = append(u.Outcomes, o)
	}
	u.Stopped = "interrupted: the run didn't finish"
	u.Exit = ExitInterrupted
	return &u
}
