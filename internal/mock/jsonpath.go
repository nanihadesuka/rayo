package mock

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/dop251/goja"
)

// JSONPath, as far as: $.a.b, $[0], $['a'], $[*] and, at the end, $[?(a condition on @)].

type jsonStep struct {
	name   string
	index  int // when name and filter are empty; -1 for every one
	filter *goja.Program
}

var (
	jsonName  = regexp.MustCompile(`^(?:\.([\w-]+)|\['([^']*)'\])`)
	jsonIndex = regexp.MustCompile(`^\[(\d+|\*)\]`)
)

func parseJSONPath(path string) ([]jsonStep, error) {
	bad := fmt.Errorf("can't read the JSONPath %q", path)
	rest, ok := strings.CutPrefix(strings.TrimSpace(path), "$")
	if !ok {
		return nil, bad
	}
	var steps []jsonStep
	for rest != "" {
		if m := jsonName.FindStringSubmatch(rest); m != nil {
			steps = append(steps, jsonStep{name: m[1] + m[2]})
			rest = rest[len(m[0]):]
		} else if m := jsonIndex.FindStringSubmatch(rest); m != nil {
			index, err := strconv.Atoi(m[1])
			if err != nil {
				index = -1 // *
			}
			steps = append(steps, jsonStep{index: index})
			rest = rest[len(m[0]):]
		} else if strings.HasPrefix(rest, "[?(") && strings.HasSuffix(rest, ")]") { // only at the end
			program, err := goja.Compile("JSONPath", current(rest[3:len(rest)-2]), false)
			if err != nil {
				return nil, fmt.Errorf("%w: %v", bad, err)
			}
			steps = append(steps, jsonStep{filter: program})
			rest = ""
		} else {
			return nil, bad
		}
	}
	return steps, nil
}

// current turns the @ of a filter into a JavaScript variable, except inside quotes.
func current(filter string) string {
	var b strings.Builder
	var quoted rune
	for _, c := range filter {
		switch {
		case quoted != 0:
			if c == quoted {
				quoted = 0
			}
		case c == '\'' || c == '"':
			quoted = c
		case c == '@':
			b.WriteString("it_")
			continue
		}
		b.WriteRune(c)
	}
	return b.String()
}

func jsonPathFinds(vm *goja.Runtime, steps []jsonStep, v any) bool {
	if len(steps) == 0 { // as in WireMock, something empty isn't there
		switch v := v.(type) {
		case []any:
			return len(v) > 0
		case map[string]any:
			return len(v) > 0
		}
		return v != nil
	}
	s, rest := steps[0], steps[1:]
	list, isList := v.([]any)
	switch {
	case s.filter != nil:
		if !isList {
			list = []any{v}
		}
		for _, item := range list {
			if vm.Set("it_", item) != nil {
				return false
			}
			ok, err := vm.RunProgram(s.filter) // one that fails, such as on a missing field, doesn't match
			if err == nil && ok.ToBoolean() {
				return true
			}
		}
		return false
	case s.name != "":
		object, _ := v.(map[string]any)
		child, ok := object[s.name]
		return ok && jsonPathFinds(vm, rest, child)
	case s.index >= 0:
		return s.index < len(list) && jsonPathFinds(vm, rest, list[s.index])
	}
	for _, item := range list {
		if jsonPathFinds(vm, rest, item) {
			return true
		}
	}
	return false
}
