package flow

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/agent"
	"github.com/nanihadesuka/rayo/internal/mock"
	"github.com/nanihadesuka/rayo/internal/schema"
	"gopkg.in/yaml.v3"
)

// kind is what a key takes. The same function checks a value and reads it, so they always agree.
type kind[T any] struct {
	name  string
	shape func() schema.S // its JSON Schema, without ${...}
	read  func(n *yaml.Node) (T, error)
	check func(p *reader, key string, n *yaml.Node) error // for those with keys inside; nil if reading is enough
}

// plain is a kind written as a single value.
func plain[T any](name string, shape schema.S, read func(text string) (T, error)) kind[T] {
	return kind[T]{name: name, shape: shapeOf(shape), read: func(n *yaml.Node) (T, error) {
		if n.Kind != yaml.ScalarNode {
			var zero T
			return zero, errors.New("takes a single value")
		}
		return read(n.Value)
	}}
}

func shapeOf(s schema.S) func() schema.S { return func() schema.S { return s } }

// oneOf takes one of some words, in any case.
func oneOf[T any](words []string, values []T) kind[T] {
	var shape []any
	for _, w := range words {
		switch w {
		case "true":
			shape = append(shape, true)
		case "false":
			shape = append(shape, false)
		default:
			shape = append(shape, w)
			if lower := strings.ToLower(w); lower != w {
				shape = append(shape, lower)
			}
		}
	}
	return plain(strings.Join(words, "/"), schema.Enum(shape...), func(text string) (T, error) {
		for i, w := range words {
			if strings.EqualFold(w, text) {
				return values[i], nil
			}
		}
		var zero T
		return zero, fmt.Errorf("not one of %s: %q", strings.Join(words, ", "), text)
	})
}

// raw is YAML that the command reads itself.
func raw(name string, shape schema.S) kind[*yaml.Node] {
	return kind[*yaml.Node]{name: name, shape: shapeOf(shape), read: func(n *yaml.Node) (*yaml.Node, error) { return n, nil }}
}

// mappingOf is YAML that the command reads itself, with each value checked.
func mappingOf(name string, shape schema.S, each func(v *yaml.Node) error) kind[*yaml.Node] {
	return kind[*yaml.Node]{name: name, shape: shapeOf(schema.Map(shape)), read: raw(name, nil).read, check: func(p *reader, key string, n *yaml.Node) error {
		if n.Kind != yaml.MappingNode {
			return p.fail(n, "%s: takes a mapping", key)
		}
		return pairs(n, func(k, v *yaml.Node) error {
			if v.Kind != yaml.ScalarNode {
				return p.fail(v, "%s: %s takes a single value", key, k.Value)
			}
			if err := each(v); err != nil && (p.filled || !dynamic(v)) {
				return p.fail(v, "%s: %s: %v", key, k.Value, err)
			}
			return nil
		})
	}}
}

func asWritten(text string) (string, error) { return text, nil }

var (
	aText    = plain("text", schema.Text(), asWritten)
	aScript  = plain("JavaScript", schema.Text(), asWritten)
	aTime    = plain("time", schema.AnyOf(schema.S{"type": "integer", "minimum": 0}, schema.Pattern(`^[0-9_.]+(ms|s|m|h)?([0-9.]+(ms|s|m))*$`)), duration)
	aPoint   = plain("point", schema.Pattern(`^\s*[0-9.]+%?\s*,\s*[0-9.]+%?\s*$`), point)
	aKeyCode = plain("key", schema.Type("string", "integer"), keyCode)
	aFlag    = oneOf([]string{"true", "false"}, []bool{true, false})
	onOff    = oneOf([]string{"enabled", "disabled", "true", "false"}, []bool{true, false, true, false})
	aState   = oneOf([]string{"offline", "online"}, []bool{true, false}) // whether it is offline
	aNumber  = plain("number", schema.Type("integer"), func(text string) (int, error) {
		n, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			return 0, fmt.Errorf("not a whole number: %q", text)
		}
		return n, nil
	})
	aDecimal = plain("number", schema.Type("number"), decimal)
	aSpeed   = plain("number", schema.S{"type": "number", "exclusiveMinimum": 0}, func(text string) (float64, error) {
		n, err := decimal(text)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("not a speed: %q", text)
		}
		return n, nil
	})
	aStatus = plain("400 to 599", schema.S{"type": "integer", "minimum": 400, "maximum": 599}, func(text string) (int, error) {
		n, err := strconv.Atoi(text)
		if err != nil || n < 400 || n > 599 {
			return 0, fmt.Errorf("not a status from 400 to 599: %q", text)
		}
		return n, nil
	})
	aPattern = plain("pattern", schema.S{"type": "string", "minLength": 1}, func(text string) (*regexp.Regexp, error) {
		if text == "" {
			return nil, errors.New("is empty")
		}
		return mock.Pattern(text)
	})
	aDirection = oneOf([]string{"UP", "DOWN", "LEFT", "RIGHT"}, []Direction{{0, -1}, {0, 1}, {-1, 0}, {1, 0}})

	anElement = kind[agent.Args]{name: "selector", shape: shapeOf(schema.Ref("selector")),
		read:  func(n *yaml.Node) (agent.Args, error) { return selector(n), nil },
		check: func(p *reader, _ string, n *yaml.Node) error { return p.selector(n) },
	}
	elements = kind[[]agent.Args]{name: "selectors", shape: shapeOf(schema.List(schema.Ref("selector"))), read: func(n *yaml.Node) (list []agent.Args, err error) {
		for _, c := range n.Content {
			list = append(list, selector(c))
		}
		return list, nil
	}, check: func(p *reader, key string, n *yaml.Node) error {
		if n.Kind != yaml.SequenceNode {
			return p.fail(n, "%s: takes a list", key)
		}
		for _, c := range n.Content {
			if err := p.selector(c); err != nil {
				return err
			}
		}
		return nil
	}}
	conditions = kind[*yaml.Node]{name: "conditions", shape: shapeOf(schema.Ref("when")), read: raw("", nil).read, check: func(p *reader, key string, n *yaml.Node) error {
		later := *p
		later.filled = false // conditions are filled in when they are looked at
		return later.mapping(n, key, whenKeys, nil)
	}}
	variables = kind[map[string]string]{name: "values", shape: shapeOf(schema.Map(schema.Text())), read: variablesIn}
	places    = kind[[]Place]{name: "locations", shape: shapeOf(schema.List(schema.Type("string"))), read: func(n *yaml.Node) (list []Place, err error) {
		for _, c := range n.Content {
			p, err := place(c.Value)
			if err != nil {
				return nil, err
			}
			list = append(list, p)
		}
		return list, nil
	}, check: func(p *reader, key string, n *yaml.Node) error {
		if n.Kind != yaml.SequenceNode {
			return p.fail(n, "%s: takes a list", key)
		}
		for _, c := range n.Content {
			if _, err := place(c.Value); err != nil && (p.filled || !dynamic(c)) {
				return p.fail(c, "%s: %v", key, err)
			}
		}
		return nil
	}}
	aPlatform   = oneOf([]string{"Android", "iOS", "Web"}, []bool{true, false, false})
	aPermission = oneOf([]string{"allow", "deny", "unset"}, []bool{true, false, false})
	arguments   = mappingOf("values", schema.Text(), func(*yaml.Node) error { return nil })
	permissions = mappingOf("permissions", schema.Dynamic(schema.Enum("allow", "deny", "unset")), func(v *yaml.Node) error {
		_, err := aPermission.read(v)
		return err
	})
)

// Direction is where a finger goes, in x and y.
type Direction struct{ x, y float64 }

// Point is in % of something, or in pixels.
type Point struct {
	x, y    float64
	percent bool
}

func decimal(text string) (float64, error) {
	n, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil {
		return 0, fmt.Errorf("not a number: %q", text)
	}
	return n, nil
}

// point reads "35%, 30%" or "540, 1200".
func point(text string) (p Point, err error) {
	bad := fmt.Errorf("not a point: %q (use '35%%, 30%%' or '540, 1200')", text)
	parts := strings.Split(text, ",")
	if len(parts) != 2 {
		return Point{}, bad
	}
	var v [2]float64
	percent := strings.Contains(text, "%")
	for i := range parts {
		s := strings.TrimSpace(parts[i])
		if strings.HasSuffix(s, "%") != percent {
			return Point{}, bad
		}
		if v[i], err = strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64); err != nil || v[i] < 0 {
			return Point{}, bad
		}
	}
	return Point{v[0], v[1], percent}, nil
}

// duration reads milliseconds or "10s".
func duration(text string) (time.Duration, error) {
	text = strings.ReplaceAll(strings.TrimSpace(text), "_", "")
	if ms, err := strconv.ParseInt(text, 10, 64); err == nil && ms >= 0 {
		return time.Duration(ms) * time.Millisecond, nil
	}
	if d, err := time.ParseDuration(text); err == nil && d >= 0 {
		return d, nil
	}
	return 0, fmt.Errorf("not a time: %q (use 500 or 10s)", text)
}
