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
	"gopkg.in/yaml.v3"
)

// kind is what a key takes. The same function checks a value and reads it, so they always agree.
type kind[T any] struct {
	name  string
	read  func(n *yaml.Node) (T, error)
	check func(p *reader, key string, n *yaml.Node) error // for those with keys inside; nil if reading is enough
}

// plain is a kind written as a single value.
func plain[T any](name string, read func(text string) (T, error)) kind[T] {
	return kind[T]{name: name, read: func(n *yaml.Node) (T, error) {
		if n.Kind != yaml.ScalarNode {
			var zero T
			return zero, errors.New("takes a single value")
		}
		return read(n.Value)
	}}
}

// oneOf takes one of some words, in any case.
func oneOf[T any](words []string, values []T) kind[T] {
	return plain(strings.Join(words, "/"), func(text string) (T, error) {
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
func raw(name string) kind[*yaml.Node] {
	return kind[*yaml.Node]{name: name, read: func(n *yaml.Node) (*yaml.Node, error) { return n, nil }}
}

// mappingOf is YAML that the command reads itself, with each value checked.
func mappingOf(name string, each func(v *yaml.Node) error) kind[*yaml.Node] {
	return kind[*yaml.Node]{name: name, read: raw(name).read, check: func(p *reader, key string, n *yaml.Node) error {
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
	aText    = plain("text", asWritten)
	aScript  = plain("JavaScript", asWritten)
	aTime    = plain("time", duration)
	aPoint   = plain("point", point)
	aKeyCode = plain("key", keyCode)
	aFlag    = oneOf([]string{"true", "false"}, []bool{true, false})
	onOff    = oneOf([]string{"enabled", "disabled", "true", "false"}, []bool{true, false, true, false})
	aState   = oneOf([]string{"offline", "online"}, []bool{true, false}) // whether it is offline
	aNumber  = plain("number", func(text string) (int, error) {
		n, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			return 0, fmt.Errorf("not a whole number: %q", text)
		}
		return n, nil
	})
	aDecimal = plain("number", decimal)
	aSpeed   = plain("number", func(text string) (float64, error) {
		n, err := decimal(text)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("not a speed: %q", text)
		}
		return n, nil
	})
	aStatus = plain("400 to 599", func(text string) (int, error) {
		n, err := strconv.Atoi(text)
		if err != nil || n < 400 || n > 599 {
			return 0, fmt.Errorf("not a status from 400 to 599: %q", text)
		}
		return n, nil
	})
	aPattern = plain("pattern", func(text string) (*regexp.Regexp, error) {
		if text == "" {
			return nil, errors.New("is empty")
		}
		return mock.Pattern(text)
	})
	aDirection = oneOf([]string{"UP", "DOWN", "LEFT", "RIGHT"}, []Direction{{0, -1}, {0, 1}, {-1, 0}, {1, 0}})

	anElement = kind[agent.Args]{name: "selector",
		read:  func(n *yaml.Node) (agent.Args, error) { return selector(n), nil },
		check: func(p *reader, _ string, n *yaml.Node) error { return p.selector(n) },
	}
	elements = kind[[]agent.Args]{name: "selectors", read: func(n *yaml.Node) (list []agent.Args, err error) {
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
	conditions = kind[*yaml.Node]{name: "conditions", read: raw("").read, check: func(p *reader, key string, n *yaml.Node) error {
		later := *p
		later.filled = false // conditions are filled in when they are looked at
		return later.mapping(n, key, whenKeys, nil)
	}}
	variables = kind[map[string]string]{name: "values", read: variablesIn}
	places    = kind[[]Place]{name: "locations", read: func(n *yaml.Node) (list []Place, err error) {
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
	arguments   = mappingOf("values", func(*yaml.Node) error { return nil })
	permissions = mappingOf("permissions", func(v *yaml.Node) error {
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
