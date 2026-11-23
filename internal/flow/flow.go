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
	"gopkg.in/yaml.v3"
)

// Flow is one file, checked before anything runs.
type Flow struct {
	Path       string
	Name       string
	App        string
	Tags       []string
	Env        map[string]string
	Mock       []*yaml.Node // paths
	Network    string       // interceptor or proxy; empty if the flow doesn't say
	Steps      []*Step
	OnStart    []*Step
	OnComplete []*Step
	Recording  *Recording // nil if the flow doesn't say
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
	return &flaw{path, line, fmt.Sprintf(format, a...)}
}

// flaw is something wrong in a flow.
type flaw struct {
	path string
	line int
	what string
}

func (f *flaw) Error() string {
	return fmt.Sprintf("%s:%d: %s", filepath.ToSlash(f.path), f.line, f.what)
}

// Load reads a flow and checks it and those it includes.
func Load(path string) (*Flow, error) { return load(path, 0) }

var errCircle = errors.New("flows run each other in a circle")

func load(path string, depth int) (*Flow, error) {
	if depth > 20 {
		return nil, fmt.Errorf("%s: %w", filepath.ToSlash(path), errCircle)
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
			return nil, fmt.Errorf("%s: %v", path, err)
		}
		if len(n.Content) > 0 {
			docs = append(docs, n.Content[0])
		}
	}
	if len(docs) != 2 || docs[0].Kind != yaml.MappingNode || docs[1].Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s: expected 'appId: ...', then '---', then a list of commands", path)
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
		if hasExpression(v) {
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
		return nil, p.fail(n, "a command is one action, such as '- tapOn: Login'")
	}
	k, ok := commands[s.Kind]
	if !ok {
		return nil, p.fail(n, "unknown command %q%s", s.Kind, suggest(s.Kind))
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
	best := ""
	for k := range commands {
		near := strings.EqualFold(k, name) ||
			(len(name) > 3 && strings.HasPrefix(strings.ToLower(k), strings.ToLower(name[:4])))
		if near && (best == "" || k < best) {
			best = k
		}
	}
	if best == "" {
		return ""
	}
	return fmt.Sprintf(", did you mean %s?", best)
}

// mapping checks the keys of a mapping and their values.
func (p *reader) mapping(n *yaml.Node, what string, keys, also []anyKey) error {
	if n.Kind != yaml.MappingNode {
		return p.fail(n, "%s: takes %s", what, names(keys, ", "))
	}
	return pairs(n, func(name, v *yaml.Node) error {
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
		return p.fail(name, "%s doesn't take %q (it takes %s)", what, name.Value, strings.Join(all, ", "))
	})
}

func (p *reader) selector(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		return nil
	case yaml.MappingNode:
		return p.mapping(n, "a selector", selectorKeys, commonKeys)
	}
	return p.fail(n, "expected a text or a selector")
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
