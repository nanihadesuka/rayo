package mock

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
)

// Updating mocks: a new capture is compared with the mocks there are, mapping by mapping, and the
// changes chosen are taken into them.

// UpdateFolder is where a capture for an update is written, inside the mocks' folder.
const UpdateFolder = ".update"

// Change is what happened to one mapping.
type Change struct {
	Key  string // its name, and the state it needs
	Kind string // same, changed, new or gone
	Diff []string
}

// Change kinds.
const (
	KindSame    = "same"
	KindChanged = "changed"
	KindNew     = "new"
	KindGone    = "gone"
)

// found is a mapping as compared: what it matches and what it answers.
type found struct {
	file    string // of mappings, where it is
	index   int    // in it
	raw     map[string]any
	request any
	status  any
	headers any
	body    any // decoded if JSON
	bodyRaw string
	bodyOf  string // its file in __files, if any
}

func keyOf(m map[string]any) string {
	name, _ := m["name"].(string)
	if name == "" {
		q, _ := m["request"].(map[string]any)
		name = fmt.Sprint(q["method"], " ", firstOf(q, "url", "urlPattern", "urlPath", "urlPathPattern"))
	}
	if state, _ := m["requiredScenarioState"].(string); state != "" {
		name += " @" + fmt.Sprint(m["scenarioName"]) + ":" + state
	}
	return name
}

func firstOf(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return ""
}

// readMappings reads all the mappings of a folder, by key.
func readMappings(folder string) (map[string]*found, []string, error) {
	var files []string
	err := filepath.WalkDir(filepath.Join(folder, "mappings"), func(file string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(file), ".json") {
			files = append(files, file)
		}
		return err
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, err
	}
	all, order := map[string]*found{}, []string{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, nil, err
		}
		var doc struct {
			Mappings []map[string]any `json:"mappings"`
		}
		d := json.NewDecoder(bytes.NewReader(data))
		d.UseNumber()
		if err := d.Decode(&doc); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", filepath.ToSlash(file), err)
		}
		if doc.Mappings == nil { // a file of one mapping
			var one map[string]any
			d := json.NewDecoder(bytes.NewReader(data))
			d.UseNumber()
			if d.Decode(&one) == nil && one["request"] != nil {
				doc.Mappings = []map[string]any{one}
			}
		}
		for i, m := range doc.Mappings {
			f := &found{file: file, index: i, raw: m, request: m["request"]}
			r, _ := m["response"].(map[string]any)
			f.status, f.headers = r["status"], r["headers"]
			switch {
			case r["bodyFileName"] != nil:
				f.bodyOf = fmt.Sprint(r["bodyFileName"])
				b, err := os.ReadFile(filepath.Join(folder, "__files", f.bodyOf))
				if err != nil {
					return nil, nil, err
				}
				f.bodyRaw = string(b)
			case r["jsonBody"] != nil:
				b, _ := json.Marshal(r["jsonBody"])
				f.bodyRaw = string(b)
			default:
				f.bodyRaw, _ = r["body"].(string)
			}
			d := json.NewDecoder(strings.NewReader(f.bodyRaw))
			d.UseNumber()
			if d.Decode(&f.body) != nil {
				f.body = f.bodyRaw
			}
			k := keyOf(m)
			if _, twice := all[k]; !twice {
				order = append(order, k)
			}
			all[k] = f // as in a mock, the last one wins
		}
	}
	return all, order, nil
}

// Compare tells what changed from the mocks in a folder to those of a new capture.
func Compare(folder, update string) ([]Change, error) {
	before, order, err := readMappings(folder)
	if err != nil {
		return nil, err
	}
	after, newOrder, err := readMappings(update)
	if err != nil {
		return nil, err
	}
	var changes []Change
	for _, k := range newOrder {
		a, b := after[k], before[k]
		switch {
		case b == nil:
			changes = append(changes, Change{Key: k, Kind: KindNew})
		default:
			var diff []string
			if !reflect.DeepEqual(normal(a.request), normal(b.request)) {
				diff = append(diff, "request: what it matches changed")
			}
			if fmt.Sprint(a.status) != fmt.Sprint(b.status) {
				diff = append(diff, fmt.Sprintf("status: %v -> %v", b.status, a.status))
			}
			if !reflect.DeepEqual(normal(a.headers), normal(b.headers)) {
				diff = append(diff, fmt.Sprintf("headers: %v -> %v", compact(b.headers), compact(a.headers)))
			}
			diffJSON("body", b.body, a.body, &diff)
			kind := KindSame
			if len(diff) > 0 {
				kind = KindChanged
			}
			changes = append(changes, Change{Key: k, Kind: kind, Diff: diff})
		}
	}
	for _, k := range order {
		if after[k] == nil {
			changes = append(changes, Change{Key: k, Kind: KindGone})
		}
	}
	return changes, nil
}

func normal(v any) any {
	b, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}

func compact(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

const maxDiff = 20

// diffJSON says where two values differ, as paths.
func diffJSON(at string, before, after any, out *[]string) {
	if len(*out) >= maxDiff {
		return
	}
	switch b := before.(type) {
	case map[string]any:
		if a, ok := after.(map[string]any); ok {
			keys := slices.Sorted(maps.Keys(b))
			for k := range a {
				if _, in := b[k]; !in {
					keys = append(keys, k)
				}
			}
			for _, k := range keys {
				bv, inB := b[k]
				av, inA := a[k]
				switch {
				case !inA:
					*out = append(*out, fmt.Sprintf("%s.%s: gone (was %s)", at, k, short(bv)))
				case !inB:
					*out = append(*out, fmt.Sprintf("%s.%s: new %s", at, k, short(av)))
				default:
					diffJSON(at+"."+k, bv, av, out)
				}
				if len(*out) >= maxDiff {
					*out = append(*out, "…")
					return
				}
			}
			return
		}
	case []any:
		if a, ok := after.([]any); ok {
			if len(a) != len(b) {
				*out = append(*out, fmt.Sprintf("%s: %d items -> %d", at, len(b), len(a)))
			}
			for i := range min(len(a), len(b)) {
				diffJSON(fmt.Sprintf("%s[%d]", at, i), b[i], a[i], out)
			}
			return
		}
	}
	if !reflect.DeepEqual(normal(before), normal(after)) {
		*out = append(*out, fmt.Sprintf("%s: %s -> %s", at, short(before), short(after)))
	}
}

func short(v any) string {
	s := compact(v)
	if len(s) > 60 {
		s = s[:57] + "..."
	}
	return s
}

// Report is the changes as said, the same ones left out.
func Report(changes []Change) []string {
	count := map[string]int{}
	var lines []string
	for _, c := range changes {
		count[c.Kind]++
		if c.Kind == KindSame {
			continue
		}
		lines = append(lines, fmt.Sprintf("  %-7s %s", c.Kind, c.Key))
		for _, d := range c.Diff {
			lines = append(lines, "            "+d)
		}
	}
	head := fmt.Sprintf("compared with the mocks: %d same, %d changed, %d new, %d gone", count[KindSame], count[KindChanged], count[KindNew], count[KindGone])
	return append([]string{head}, lines...)
}

// Apply takes changes from the update into the mocks: those whose key has one of only, or all if
// only is empty. A mapping that is gone is taken out only with prune. It returns what it did.
func Apply(folder string, only []string, prune bool) ([]string, error) {
	update := filepath.Join(folder, UpdateFolder)
	if _, err := os.Stat(filepath.Join(update, "mappings")); err != nil {
		return nil, fmt.Errorf("%s has no update: make one with rayo test --capture-update %s", filepath.ToSlash(folder), filepath.ToSlash(folder))
	}
	changes, err := Compare(folder, update)
	if err != nil {
		return nil, err
	}
	before, _, _ := readMappings(folder)
	after, _, _ := readMappings(update)
	chosen := func(key string) bool {
		if len(only) == 0 {
			return true
		}
		for _, o := range only {
			if strings.Contains(key, o) {
				return true
			}
		}
		return false
	}
	docs := map[string][]map[string]any{} // of each file of mappings, as it will be
	load := func(file string) []map[string]any {
		if list, ok := docs[file]; ok {
			return list
		}
		var list []map[string]any
		for _, f := range before {
			if f.file == file {
				for len(list) <= f.index {
					list = append(list, nil)
				}
				list[f.index] = f.raw
			}
		}
		docs[file] = list
		return list
	}
	target := filepath.Join(folder, "mappings", "capture.json")
	var did []string
	for _, c := range changes {
		if c.Kind == KindSame || !chosen(c.Key) || (c.Kind == KindGone && !prune) {
			continue
		}
		var file string
		var index int
		if b := before[c.Key]; b != nil {
			file, index = b.file, b.index
		} else {
			file, index = target, -1
		}
		list := load(file)
		switch c.Kind {
		case KindGone:
			list[index] = nil
		default:
			a := after[c.Key]
			m := a.raw
			if a.bodyOf != "" {
				name := a.bodyOf
				if b := before[c.Key]; b != nil && b.bodyOf != "" {
					name = b.bodyOf // where it was: hand-written mocks may point at it
				} else if _, err := os.Stat(filepath.Join(folder, "__files", name)); err == nil {
					name = "updated_" + name
				}
				if err := os.MkdirAll(filepath.Join(folder, "__files"), 0o755); err != nil {
					return nil, err
				}
				if err := os.WriteFile(filepath.Join(folder, "__files", name), []byte(a.bodyRaw), 0o644); err != nil {
					return nil, err
				}
				m["response"].(map[string]any)["bodyFileName"] = name
			}
			if index < 0 {
				list = append(list, m)
			} else {
				list[index] = m
			}
		}
		docs[file] = list
		did = append(did, fmt.Sprintf("  %-7s %s", c.Kind, c.Key))
	}
	for file, list := range docs {
		kept := slices.DeleteFunc(slices.Clone(list), func(m map[string]any) bool { return m == nil })
		if err := writeJSON(file, map[string]any{"mappings": kept}); err != nil {
			return nil, err
		}
	}
	if len(did) == 0 {
		return []string{"nothing to take: no change was chosen"}, nil
	}
	return append([]string{fmt.Sprintf("took %d changes into %s:", len(did), filepath.ToSlash(folder))}, did...), nil
}

// ClearUpdate takes away an update made before, for a new one.
func ClearUpdate(folder string) error {
	err := os.RemoveAll(filepath.Join(folder, UpdateFolder))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
