package flow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Attempt is one try of a flow, for what it writes: into Out, named after Flow and Try.
type Attempt struct {
	Out  string // the output folder
	Flow string // its name, which no other flow of the run has
	Try  int    // 0 for the first
}

// DefaultTemplate is how the files of an attempt are named, all in one folder.
const DefaultTemplate = "{flow}_{attempt}_{kind}.{ext}"

// The kinds of file an attempt writes.
const (
	Video      = "video"      // recordings
	Screenshot = "screenshot" // of a failure, and those of takeScreenshot
	Logcat     = "logcat"     // the device's log, of a failure
	Screen     = "screen"     // what was on it, of a failure
	Requests   = "requests"   // the calls to the mock, of a failure
)

// Artifacts are all the kinds, in the order they are told.
var Artifacts = []string{Video, Screenshot, Logcat, Screen, Requests}

// CheckTemplate says whether a name template names each file of a run apart.
func CheckTemplate(t string) error {
	for _, part := range []string{"{flow}", "{attempt}", "{kind}", "{ext}"} {
		if !strings.Contains(t, part) {
			return fmt.Errorf("needs {flow}, {attempt}, {kind} and {ext}, so that no two files have one name, not %q", t)
		}
	}
	if filepath.IsAbs(t) || strings.HasPrefix(t, "/") || slices.Contains(strings.Split(filepath.ToSlash(t), "/"), "..") {
		return fmt.Errorf("is a path in the output folder, not %q", t)
	}
	return nil
}

// CheckArtifacts says whether each is a kind of file.
func CheckArtifacts(kinds []string) error {
	for _, k := range kinds {
		if !slices.Contains(Artifacts, k) {
			return fmt.Errorf("takes %s, not %q", strings.Join(Artifacts, ", "), k)
		}
	}
	return nil
}

// wants says whether files of a kind are written.
func (r *Runner) wants(kind string) bool {
	return r.Artifacts == nil || slices.Contains(r.Artifacts, kind)
}

// named is where a file of the attempt goes: kind tells it from the others, such as failure or logcat, and is empty
// for its video. A part of the path that is left without a name takes that of the attempt.
func (r *Runner) named(kind, ext string) string {
	a := r.attempt
	attempt := ""
	if a.Try > 0 {
		attempt = fmt.Sprintf("retry%d", a.Try)
	}
	fill := strings.NewReplacer("{flow}", a.Flow, "{attempt}", attempt, "{kind}", kind, "{ext}", ext).Replace
	template := r.NameTemplate
	if template == "" {
		template = DefaultTemplate
	}
	own := tidy(fill("{flow}_{attempt}"))
	parts := strings.Split(fill(template), "/")
	for i, p := range parts {
		if p = tidy(p); p == "" || strings.HasPrefix(p, ".") {
			p = own + p
		}
		parts[i] = p
	}
	return filepath.Join(a.Out, filepath.FromSlash(strings.Join(parts, "/")))
}

var separators = regexp.MustCompile(`[_-]+`)

// tidy takes out the separators that were around what is empty, as {attempt} of the first.
func tidy(part string) string {
	part = separators.ReplaceAllStringFunc(part, func(s string) string { return s[:1] })
	part = strings.NewReplacer("_.", ".", "-.", ".").Replace(part)
	return strings.Trim(part, "_-")
}

// wrote notes a file of the attempt, which goes as Keep says.
func (r *Runner) wrote(path string) { r.files = append(r.files, path) }

// Remove takes out the files of an attempt, and the folders they leave empty in out.
func Remove(out string, files []string) error {
	var err error
	for _, file := range files {
		if e := os.Remove(file); e != nil && !os.IsNotExist(e) {
			err = e
			continue
		}
		for dir := filepath.Dir(file); ; dir = filepath.Dir(dir) {
			rel, e := filepath.Rel(out, dir)
			if e != nil || rel == "." || strings.HasPrefix(rel, "..") || os.Remove(dir) != nil { // not empty
				break
			}
		}
	}
	return err
}
