// Package flow reads flows and runs them.
package flow

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/nanihadesuka/rayo/internal/mock"
	"github.com/nanihadesuka/rayo/internal/schema"
	"gopkg.in/yaml.v3"
)

// Flow is one file, checked before anything runs.
type Flow struct {
	Path        string
	Name        string
	App         string
	Tags        []string
	Env         map[string]string
	Mock        []*yaml.Node // paths
	Network     string       // interceptor or proxy, or ${...} that gives one; empty if the flow doesn't say
	NetworkLine int
	Unmocked    *mock.Unmocked // calls without a mock that don't fail it, as its suite says; nil for none
	Steps       []*Step
	OnStart     []*Step
	OnComplete  []*Step
	Recording   *Recording // nil if the flow doesn't say
}

// Step is a command in a flow. It keeps its YAML: ${...} is only known when it runs.
type Step struct {
	Kind  string
	Value *yaml.Node // nil when there is none
	Path  string
	Line  int
	Sub   []*Step // repeat, retry and inline runFlow
}

func problem(path string, line int, format string, a ...any) error {
	return &Problem{Path: path, Line: line, What: fmt.Sprintf(format, a...)}
}

// Problem is something wrong in a file, a flow's or a suite's, and what would be right.
type Problem struct {
	Path     string
	Line     int    // 0 if not known
	What     string // is wrong
	Expected string // what it takes, if that can be said
	Nearest  string // of that, the closest to what is written
	Fix      string // what to write instead, if that can be said
	Err      error  // what it comes from, if anything
}

func (p *Problem) Error() string {
	if p.Line == 0 {
		return fmt.Sprintf("%s: %s", filepath.ToSlash(p.Path), p.What)
	}
	return fmt.Sprintf("%s:%d: %s", filepath.ToSlash(p.Path), p.Line, p.What)
}

func (p *Problem) Unwrap() error { return p.Err }

// Load reads a flow and checks it and those it includes.
func Load(path string) (*Flow, error) { return load(path, 0) }

var errCircle = errors.New("flows run each other in a circle")

func load(path string, depth int) (*Flow, error) {
	if depth > 20 {
		return nil, &Problem{Path: path, What: errCircle.Error(), Err: errCircle}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parse(path, data, depth)
}

func parse(path string, data []byte, depth int) (*Flow, error) {
	var docs []*yaml.Node
	for d := yaml.NewDecoder(bytes.NewReader(data)); ; {
		var n yaml.Node
		if err := d.Decode(&n); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, &Problem{Path: path, What: err.Error()}
		}
		if len(n.Content) > 0 {
			docs = append(docs, n.Content[0])
		}
	}
	if len(docs) != 2 || docs[0].Kind != yaml.MappingNode || docs[1].Kind != yaml.SequenceNode {
		return nil, &Problem{Path: path, What: "expected 'appId: ...', then '---', then a list of commands", Expected: "appId: ..., then ---, then a list of commands"}
	}
	f := &Flow{Path: path, Name: flowName(path), Env: map[string]string{}}
	p := &reader{path: path, depth: depth}
	if err := p.readHeader(f, docs[0]); err != nil {
		return nil, err
	}
	var err error
	f.Steps, err = p.steps(docs[1])
	return f, err
}

// flowName is the file's name, or the test's folder when the file is just "flow.yaml".
func flowName(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if abs, err := filepath.Abs(path); err == nil && name == "flow" {
		dir := filepath.Dir(abs)
		if b := filepath.Base(dir); b == "rayo" {
			dir = filepath.Dir(dir)
		}
		return filepath.Base(dir)
	}
	return name
}

// --- Checking ---

type reader struct {
	path   string
	depth  int
	filled bool // each ${...} has its value by now
}

func (p *reader) fail(n *yaml.Node, format string, a ...any) error {
	return problem(p.path, n.Line, format, a...)
}

// at gives an error its place; none stays none.
func (p *reader) at(n *yaml.Node, err error) error {
	if err == nil {
		return nil
	}
	return p.fail(n, "%v", err)
}

// mocks reads one path or a list, and checks them together: a file may use the bodies of a folder before it.
func (p *reader) mocks(v *yaml.Node) ([]*yaml.Node, error) {
	list := []*yaml.Node{v}
	if v.Kind == yaml.SequenceNode {
		list = v.Content
	}
	m := mock.New()
	for _, n := range list {
		if n.Kind != yaml.ScalarNode || n.Value == "" {
			return nil, p.fail(n, "expected the path of a mock")
		}
		if hasExpression(n) {
			continue
		}
		if err := m.Load(relative(p.path, n.Value)); err != nil {
			return nil, p.fail(n, "%v", err)
		}
	}
	return list, nil
}

func (p *reader) steps(n *yaml.Node) ([]*Step, error) {
	if n.Kind != yaml.SequenceNode {
		return nil, p.fail(n, "expected a list of commands")
	}
	var steps []*Step
	for _, c := range n.Content {
		s, err := p.step(c)
		if err != nil {
			return nil, err
		}
		steps = append(steps, s)
	}
	return steps, nil
}

func (p *reader) step(n *yaml.Node) (*Step, error) {
	s := &Step{Path: p.path, Line: n.Line}
	switch {
	case n.Kind == yaml.ScalarNode:
		s.Kind = n.Value
	case n.Kind == yaml.MappingNode && len(n.Content) == 2:
		s.Kind, s.Value = n.Content[0].Value, n.Content[1]
		if s.Value.Tag == "!!null" {
			s.Value = nil
		}
	default:
		if n.Kind == yaml.MappingNode && len(n.Content) > 2 {
			if k, ok := commands[n.Content[0].Value]; ok {
				for i := 2; i < len(n.Content); i += 2 {
					name := n.Content[i].Value
					if _, other := commands[name]; !other && (find(k.accepts(), name) != nil || find(commonKeys, name) != nil) {
						return nil, p.fail(n.Content[i], "%s belongs under %s: indent it, as '- %s: { ..., %s: ... }'", name, n.Content[0].Value, n.Content[0].Value, name)
					}
				}
			}
		}
		return nil, p.fail(n, "a command is one action, such as '- tapOn: Login'")
	}
	k, ok := commands[s.Kind]
	if !ok {
		e := p.fail(n, "unknown command %q%s", s.Kind, suggest(s.Kind)).(*Problem)
		if e.Nearest = nearest(s.Kind); e.Nearest != "" {
			e.Fix = e.Nearest // as the command's name
		}
		return nil, e
	}
	v := s.Value
	switch {
	case v == nil:
		if !k.alone {
			return nil, p.fail(n, "%s needs a value", s.Kind)
		}
		return s, nil
	case v.Kind == yaml.ScalarNode:
		if k.plain == nil {
			if len(k.accepts()) == 0 {
				return nil, p.fail(v, "%s doesn't take a plain value", s.Kind)
			}
			return nil, p.fail(v, "%s takes %s", s.Kind, names(k.accepts(), ", "))
		}
		// From here on, every value is a mapping.
		s.Value = &yaml.Node{Kind: yaml.MappingNode, Line: v.Line, Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: k.plain.title(), Line: v.Line}, v,
		}}
	case v.Kind != yaml.MappingNode:
		return nil, p.fail(v, "%s doesn't take a list", s.Kind)
	}
	return s, p.check(s, k)
}

func suggest(name string) string {
	if best := nearest(name); best != "" {
		return fmt.Sprintf(", did you mean %s?", best)
	}
	return ""
}

// NearestCommand is the command closest to a name that isn't one; empty if none is close.
func NearestCommand(name string) string { return nearest(name) }

// nearest is the command closest to a name that isn't one; empty if none is close.
func nearest(name string) string {
	best := ""
	for k := range commands { // tap: tapOn
		if strings.HasPrefix(strings.ToLower(k), strings.ToLower(name)) && len(name) >= 3 && (best == "" || len(k) < len(best)) {
			best = k
		}
	}
	if best != "" {
		return best
	}
	for k := range commands {
		near := strings.EqualFold(k, name) ||
			(len(name) > 3 && strings.HasPrefix(strings.ToLower(k), strings.ToLower(name[:4])))
		if near && (best == "" || k < best) {
			best = k
		}
	}
	return best
}

// mapping checks the keys of a mapping and their values.
func (p *reader) mapping(n *yaml.Node, what string, keys, also []anyKey) error {
	if n.Kind != yaml.MappingNode {
		return p.fail(n, "%s: takes %s", what, names(keys, ", "))
	}
	seen := map[string]int{}
	return pairs(n, func(name, v *yaml.Node) error {
		if line, twice := seen[name.Value]; twice && line == name.Line {
			return p.fail(name, "%s appears twice; keep one", name.Value)
		} else if twice {
			return p.fail(name, "%s appears twice (lines %d and %d); keep one", name.Value, line, name.Line)
		}
		seen[name.Value] = name.Line
		k := find(keys, name.Value)
		if k == nil {
			k = find(also, name.Value)
		}
		if k != nil {
			return k.check(p, v)
		}
		var all []string
		for _, k := range keys {
			all = append(all, k.title())
		}
		sort.Strings(all)
		near := schema.Nearest(name.Value, all)
		if near == "" {
			for _, k := range also {
				all = append(all, k.title())
			}
			near = schema.Nearest(name.Value, all)
			all = all[:len(keys)]
		}
		hint := ""
		if near != "" {
			hint = ", did you mean " + near + "?"
		}
		e := p.fail(name, "%s doesn't take %q%s (it takes %s)", what, name.Value, hint, strings.Join(all, ", ")).(*Problem)
		e.Expected, e.Nearest = strings.Join(all, ", "), near
		return e
	})
}

func (p *reader) selector(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		return nil
	case yaml.MappingNode:
		if err := p.mapping(n, "a selector", selectorKeys, commonKeys); err != nil {
			return err
		}
		return p.matched(n)
	}
	return p.fail(n, "expected a text or a selector")
}

// matched refuses a text or an id that can't be what is meant: empty, which matches only what has
// none, or an id with spaces around it, which no id has.
func (p *reader) matched(n *yaml.Node) error {
	for _, k := range []anyKey{keyText, keyID} {
		v := get(n, k)
		if v == nil || v.Kind != yaml.ScalarNode || v.Tag == "!!null" {
			continue
		}
		switch {
		case strings.TrimSpace(v.Value) == "":
			return p.fail(v, "%s is empty; say what to look for, or leave %s out", k.title(), k.title())
		case k == keyID && strings.TrimSpace(v.Value) != v.Value:
			return p.fail(v, "id %q has spaces around it, and no id has; write %s", v.Value, strings.TrimSpace(v.Value))
		}
	}
	return nil
}

func (p *reader) check(s *Step, c command) error {
	v := s.Value
	if c.free {
		return c.check(p, s)
	}
	if err := p.mapping(v, s.Kind, c.accepts(), commonKeys); err != nil {
		return err
	}
	if c.element {
		found := get(v, keyPoint) != nil
		for _, k := range selectorKeys {
			found = found || get(v, k) != nil
		}
		if !found {
			return p.fail(v, "%s needs an element, such as a text or an id", s.Kind)
		}
		if err := p.matched(v); err != nil {
			return err
		}
	}
	if n := get(v, keyCommands); n != nil {
		sub, err := p.steps(n)
		if err != nil {
			return err
		}
		s.Sub = sub
	}
	if c.check != nil {
		return c.check(p, s)
	}
	return nil
}

// nearFile is ", did you mean ...?" with the file next to a missing one most like it, as written; empty if none is.
func nearFile(path, written string) string {
	entries, _ := os.ReadDir(filepath.Dir(path))
	var names []string
	for _, d := range entries {
		names = append(names, d.Name())
	}
	if near := schema.Nearest(filepath.Base(path), names); near != "" {
		return fmt.Sprintf(", did you mean %s?", filepath.ToSlash(filepath.Join(filepath.Dir(filepath.FromSlash(written)), near)))
	}
	return ""
}

func relative(from, file string) string {
	file = filepath.FromSlash(file)
	if filepath.IsAbs(file) {
		return file
	}
	return filepath.Join(filepath.Dir(from), file)
}

// --- Showing ---

// describe is how a step shows in the output.
func (s *Step) describe() string {
	if l := keyLabel.of(s.Value); l != "" {
		return l
	}
	if s.Value == nil {
		return s.Kind
	}
	if say := commands[s.Kind].say; say != nil {
		return s.Kind + " " + say(s.Value)
	}
	return strings.TrimSpace(s.Kind + " " + short(s.Value))
}

func short(n *yaml.Node) string {
	switch n.Kind {
	case yaml.ScalarNode:
		return n.Value
	case yaml.SequenceNode:
		return fmt.Sprintf("[%d]", len(n.Content))
	}
	var parts []string
	_ = pairs(n, func(k, v *yaml.Node) error {
		switch {
		case isOption(k.Value) || k.Value == keyCommands.name || k.Value == keyEnv.name:
		case k.Value == keyID.name || len(n.Content) == 2:
			parts = append(parts, short(v))
		case v.Value == "true":
			parts = append(parts, k.Value)
		case v.Kind == yaml.ScalarNode:
			parts = append(parts, k.Value+": "+v.Value)
		default:
			parts = append(parts, strings.TrimSpace(short(v)))
		}
		return nil
	})
	return strings.Join(parts, ", ")
}

// RecordsElsewhere says whether the flow, or one it runs, starts a recording to a file of its own:
// startRecording with a path. A recording of the whole flow, which goes to the flow's name, can't go on with it.
func (f *Flow) RecordsElsewhere() bool { return f.walkFlows(false) }

// Records says whether the flow, or one it runs, makes a video: with record in its header, or startRecording.
func (f *Flow) Records() bool { return f.walkFlows(true) }

// walkFlows looks for startRecording in the flow and those it runs, and for record in their headers if
// header says so; else only for a startRecording with a path.
func (f *Flow) walkFlows(header bool) bool {
	seen := map[string]bool{}
	var of func(f *Flow) bool
	var walk func(steps []*Step) bool
	walk = func(steps []*Step) bool {
		for _, s := range steps {
			if s.Kind == "startRecording" && (header || keyVideo.of(s.Value) != "") || walk(s.Sub) {
				return true
			}
			if file := keyFile.of(s.Value); file != "" {
				path := relative(s.Path, file)
				if !seen[path] {
					seen[path] = true
					if included, err := Load(path); err == nil && of(included) {
						return true
					}
				}
			}
		}
		return false
	}
	of = func(f *Flow) bool {
		if header && f.Recording != nil && f.Recording.On {
			return true
		}
		return walk(f.OnStart) || walk(f.Steps) || walk(f.OnComplete)
	}
	return of(f)
}

// Files are those that can change how the flow goes: itself, what it includes and its mocks.
func (f *Flow) Files() []string {
	var files []string
	var add func(path string)
	var walk func(steps []*Step)
	walk = func(steps []*Step) {
		for _, s := range steps {
			if file := keyFile.of(s.Value); file != "" {
				add(relative(s.Path, file))
			}
			walk(s.Sub)
		}
	}
	from := func(f *Flow) {
		for _, n := range f.Mock {
			_ = filepath.WalkDir(relative(f.Path, n.Value), func(path string, d fs.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					files = append(files, path)
				}
				return nil // what can't be read can't be watched
			})
		}
		walk(f.OnStart)
		walk(f.Steps)
		walk(f.OnComplete)
	}
	add = func(path string) {
		if slices.Contains(files, path) {
			return
		}
		files = append(files, path)
		if included, err := Load(path); err == nil {
			from(included)
		}
	}
	files = append(files, f.Path)
	from(f)
	return files
}
