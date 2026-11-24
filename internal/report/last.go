package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nanihadesuka/rayo/internal/flow"
)

// LastFile is where the flows that haven't passed are kept.
func LastFile(output string) string { return filepath.Join(output, "failed.json") }

// Identity tells flows apart in different runs.
func Identity(f *flow.Flow) string {
	path := f.Path
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return filepath.ToSlash(path)
}

// Last is the flows that haven't passed since they last failed, and how they went.
func Last(file string) (map[string]Status, error) {
	last := map[string]Status{}
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return last, nil
	}
	if err == nil {
		err = json.Unmarshal(data, &last)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.ToSlash(file), err)
	}
	return last, nil
}

// SaveLast adds this run to it.
func (s *Summary) SaveLast(file string) error {
	last, err := Last(file)
	if err != nil {
		last = map[string]Status{} // unreadable, so written again
	}
	before := len(last)
	for _, o := range s.Outcomes {
		if o.Passed() {
			delete(last, Identity(o.Flow))
		} else {
			last[Identity(o.Flow)] = o.Status
		}
	}
	if len(last) == 0 {
		if before > 0 || err != nil {
			return os.Remove(file)
		}
		return nil
	}
	data, err := json.MarshalIndent(last, "", "  ")
	if err != nil {
		return err
	}
	return WriteFile(file, append(data, '\n'))
}
