package flow

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Reading YAML and the values in it.

func pairs(n *yaml.Node, each func(key, value *yaml.Node) error) error {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if err := each(n.Content[i], n.Content[i+1]); err != nil {
			return err
		}
	}
	return nil
}

func get(n *yaml.Node, wanted anyKey) *yaml.Node {
	var found *yaml.Node
	_ = pairs(n, func(k, v *yaml.Node) error {
		if k.Value == wanted.title() {
			found = v
		}
		return nil
	})
	return found
}

func variablesIn(n *yaml.Node) (map[string]string, error) {
	if n.Kind != yaml.MappingNode {
		return nil, errors.New("takes a mapping")
	}
	m := map[string]string{}
	return m, pairs(n, func(k, v *yaml.Node) error {
		if v.Kind != yaml.ScalarNode {
			return fmt.Errorf("%s takes a single value", k.Value)
		}
		m[k.Value] = v.Value
		return nil
	})
}

func dynamic(n *yaml.Node) bool { return n != nil && strings.Contains(n.Value, "${") }

func hasExpression(n *yaml.Node) bool {
	if n == nil || dynamic(n) {
		return n != nil
	}
	for _, c := range n.Content {
		if hasExpression(c) {
			return true
		}
	}
	return false
}
