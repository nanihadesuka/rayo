package mock

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// Check loads mocks, a folder or a file, as a flow would, and says what can be told without running
// them: mappings that never answer, as a later one answers the same calls, and bodies in __files that
// no mapping uses. An error is of mocks that don't load.
func Check(path string) (mappings int, problems []string, err error) {
	m := New()
	if err := m.Load(path); err != nil {
		return 0, nil, err
	}
	for i, r := range m.rules {
		for _, later := range m.rules[i+1:] {
			if sameCalls(r, later) { // of the same priority, the last one wins
				problems = append(problems, fmt.Sprintf("%s never answers: %s, after it, answers the same calls (%s)", r.source, later.source, r.title()))
				break
			}
		}
	}
	files := filepath.Join(path, "__files")
	if info, err := os.Stat(files); err != nil || !info.IsDir() {
		return len(m.rules), problems, nil
	}
	used := map[string]bool{}
	for _, r := range m.rules {
		if name := r.Response.BodyFileName; name != "" {
			used[filepath.Clean(filepath.FromSlash(name))] = true
		}
	}
	var unused []string
	err = filepath.WalkDir(files, func(file string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name, err := filepath.Rel(files, file)
		if err == nil && !used[name] {
			unused = append(unused, fmt.Sprintf("%s: no mapping uses it", filepath.ToSlash(file)))
		}
		return err
	})
	slices.Sort(unused)
	return len(m.rules), append(problems, unused...), err
}

// sameCalls says whether two mappings answer the same calls, in the same states.
func sameCalls(a, b *Rule) bool {
	if a.priority() != b.priority() || a.ScenarioName != b.ScenarioName || a.RequiredScenarioState != b.RequiredScenarioState {
		return false
	}
	x, errA := json.Marshal(a.Request)
	y, errB := json.Marshal(b.Request)
	return errA == nil && errB == nil && string(x) == string(y)
}
