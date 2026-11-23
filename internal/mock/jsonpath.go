package mock

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/dop251/goja"
)

// JSONPath, as Jayway's that WireMock uses: $.a, $['a'], $['a','b'], $[0], $[-1], $[0,2], $[1:3], $.*,
// $[*], $..a, filters such as [?(@.a == 1 && @.b =~ /x.*/)] anywhere, and length() at the end. A filter
// is run as JavaScript once Jayway's operators (=~, in, nin, size, empty, subsetof, anyof, noneof) are
// turned into it, so a filter may also use JavaScript of its own.

type jsonStep struct {
	names  []string // children by name
	index  []int    // elements by index; from the end if negative
	slice  *[3]*int // start, end, step
	all    bool     // * : every child
	deep   bool     // .. : this step applies to the node and all below it
	filter *goja.Program
	length bool // length() or size()
}

func (s jsonStep) definite() bool {
	return !s.all && !s.deep && s.filter == nil && s.slice == nil && len(s.names)+len(s.index) <= 1
}

var (
	jsonDotName   = regexp.MustCompile(`^\.([\w-]+|\*)`)
	jsonDeepName  = regexp.MustCompile(`^\.\.([\w-]+|\*)`)
	jsonFunction  = regexp.MustCompile(`^\.(length|size)\(\)$`)
	jsonQuoted    = regexp.MustCompile(`^\s*(?:'((?:[^'\\]|\\.)*)'|"((?:[^"\\]|\\.)*)")\s*`)
	jsonSliceExpr = regexp.MustCompile(`^(-?\d*):(-?\d*)(?::(-?\d+))?$`)
)

func parseJSONPath(path string) ([]jsonStep, error) {
	bad := func(why string) error { return fmt.Errorf("can't read the JSONPath %q: %s", path, why) }
	rest, ok := strings.CutPrefix(strings.TrimSpace(path), "$")
	if !ok {
		return nil, bad("it begins with $")
	}
	var steps []jsonStep
	for rest != "" {
		deep := false
		if m := jsonFunction.FindStringSubmatch(rest); m != nil {
			steps = append(steps, jsonStep{length: true})
			break
		}
		if m := jsonDeepName.FindStringSubmatch(rest); m != nil {
			steps = append(steps, named(m[1], true))
			rest = rest[len(m[0]):]
			continue
		}
		if strings.HasPrefix(rest, "..[") {
			deep, rest = true, rest[2:]
		}
		if m := jsonDotName.FindStringSubmatch(rest); m != nil {
			steps = append(steps, named(m[1], false))
			rest = rest[len(m[0]):]
			continue
		}
		if !strings.HasPrefix(rest, "[") {
			return nil, bad("at " + rest)
		}
		end := closingBracket(rest)
		if end < 0 {
			return nil, bad("a [ isn't closed")
		}
		inside := rest[1:end]
		rest = rest[end+1:]
		s, err := bracket(inside)
		if err != nil {
			return nil, bad(err.Error())
		}
		s.deep = deep
		steps = append(steps, s)
	}
	return steps, nil
}

func named(name string, deep bool) jsonStep {
	if name == "*" {
		return jsonStep{all: true, deep: deep}
	}
	return jsonStep{names: []string{name}, deep: deep}
}

// closingBracket is where the ] of the [ at the start is, past quotes and inner brackets.
func closingBracket(s string) int {
	depth := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '[' || c == '(':
			depth++
		case c == ']' || c == ')':
			if depth--; depth == 0 {
				return i
			}
		}
	}
	return -1
}

func bracket(inside string) (jsonStep, error) {
	t := strings.TrimSpace(inside)
	switch {
	case t == "*":
		return jsonStep{all: true}, nil
	case strings.HasPrefix(t, "?(") && strings.HasSuffix(t, ")"):
		program, err := goja.Compile("JSONPath", filterJS(t[2:len(t)-1]), false)
		if err != nil {
			return jsonStep{}, err
		}
		return jsonStep{filter: program}, nil
	case strings.HasPrefix(t, "'") || strings.HasPrefix(t, `"`):
		var s jsonStep
		for t != "" {
			m := jsonQuoted.FindStringSubmatch(t)
			if m == nil {
				return s, fmt.Errorf("in [%s]", inside)
			}
			s.names = append(s.names, unescapeQuoted(m[1]+m[2]))
			t = strings.TrimPrefix(t[len(m[0]):], ",")
		}
		return s, nil
	}
	if m := jsonSliceExpr.FindStringSubmatch(t); m != nil {
		var parts [3]*int
		for i, v := range m[1:] {
			if v != "" {
				n, _ := strconv.Atoi(v)
				parts[i] = &n
			}
		}
		return jsonStep{slice: &parts}, nil
	}
	var s jsonStep
	for _, part := range strings.Split(t, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return s, fmt.Errorf("in [%s]", inside)
		}
		s.index = append(s.index, n)
	}
	return s, nil
}

func unescapeQuoted(s string) string {
	return strings.NewReplacer(`\'`, `'`, `\"`, `"`, `\\`, `\`).Replace(s)
}

// pathResult is what a JSONPath found.
type pathResult struct {
	values   []any
	definite bool
}

func jsonPathEval(vm *goja.Runtime, steps []jsonStep, root any) pathResult {
	nodes, definite := []any{root}, true
	for _, s := range steps {
		definite = definite && s.definite()
		var next []any
		for _, n := range nodes {
			targets := []any{n}
			if s.deep {
				targets = descendants(n, nil)
			}
			for _, t := range targets {
				next = append(next, s.apply(vm, t, root)...)
			}
		}
		nodes = next
	}
	return pathResult{nodes, definite}
}

// descendants are a node and all below it, in order.
func descendants(v any, out []any) []any {
	out = append(out, v)
	switch v := v.(type) {
	case map[string]any:
		for _, k := range sortedKeys(v) {
			out = descendants(v[k], out)
		}
	case []any:
		for _, child := range v {
			out = descendants(child, out)
		}
	}
	return out
}

func (s jsonStep) apply(vm *goja.Runtime, v any, root any) []any {
	object, isObject := v.(map[string]any)
	list, isList := v.([]any)
	var out []any
	switch {
	case s.length:
		switch v := v.(type) {
		case []any:
			return []any{json.Number(strconv.Itoa(len(v)))}
		case map[string]any:
			return []any{json.Number(strconv.Itoa(len(v)))}
		case string:
			return []any{json.Number(strconv.Itoa(len([]rune(v))))}
		}
	case s.all:
		if isObject {
			for _, k := range sortedKeys(object) {
				out = append(out, object[k])
			}
		}
		out = append(out, list...)
	case s.names != nil:
		for _, name := range s.names {
			if child, ok := object[name]; ok && isObject {
				out = append(out, child)
			}
		}
	case s.index != nil:
		for _, i := range s.index {
			if i < 0 {
				i += len(list)
			}
			if isList && i >= 0 && i < len(list) {
				out = append(out, list[i])
			}
		}
	case s.slice != nil:
		if !isList {
			return nil
		}
		start, end, step := 0, len(list), 1
		at := func(p *int, def int) int {
			if p == nil {
				return def
			}
			n := *p
			if n < 0 {
				n += len(list)
			}
			return max(0, min(n, len(list)))
		}
		start, end = at(s.slice[0], start), at(s.slice[1], end)
		if s.slice[2] != nil && *s.slice[2] > 0 {
			step = *s.slice[2]
		}
		for i := start; i < end; i += step {
			out = append(out, list[i])
		}
	case s.filter != nil:
		items := list
		if !isList {
			items = []any{v} // as Jayway: a filter on an object tests the object
		}
		for _, item := range items {
			if vm.Set("it_", plain(item)) != nil || vm.Set("root_", plain(root)) != nil {
				continue
			}
			ok, err := vm.RunProgram(s.filter) // one that fails, such as on a missing field, doesn't match
			if err == nil && ok.ToBoolean() {
				out = append(out, item)
			}
		}
	}
	return out
}

// plain gives JavaScript numbers instead of json.Number, so that @.a == 1 holds.
func plain(v any) any {
	switch v := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, child := range v {
			out[k] = plain(child)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = plain(child)
		}
		return out
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f
		}
		return v.String()
	}
	return v
}

// jsonPathTrue is whether a matchesJsonPath of an expression alone holds: as in WireMock, something
// empty or null isn't there.
func jsonPathTrue(f pathResult) bool {
	if !f.definite {
		return len(f.values) > 0
	}
	if len(f.values) == 0 {
		return false
	}
	switch v := f.values[0].(type) {
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	}
	return f.values[0] != nil
}

// text is what was found as a matcher sees it: a text as it is, anything else as JSON.
func (f pathResult) text() (string, bool) {
	var v any
	switch {
	case len(f.values) == 0:
		return "", false
	case len(f.values) == 1:
		v = f.values[0]
	default:
		v = f.values
	}
	switch v := v.(type) {
	case nil:
		return "", false
	case string:
		return v, true
	}
	return compactJSON(v), true
}

func compactJSON(v any) string {
	var b strings.Builder
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	_ = e.Encode(v)
	return strings.TrimSuffix(b.String(), "\n")
}

var (
	jsOperand  = `(@(?:\.[\w-]+|\[[^\]]*\])*(?:\.length\(\))?|\$(?:\.[\w-]+|\[[^\]]*\])*|'(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*"|-?\d+(?:\.\d+)?|true|false|null)`
	jsList     = `(\[[^\]]*\])`
	jsRegexOp  = regexp.MustCompile(jsOperand + `\s*=~\s*(/(?:\\.|[^/\\])*/[a-z]*)`)
	jsNin      = regexp.MustCompile(jsOperand + `\s+nin\s+` + jsList)
	jsIn       = regexp.MustCompile(jsOperand + `\s+in\s+` + jsList)
	jsSubsetOf = regexp.MustCompile(jsOperand + `\s+subsetof\s+` + jsList)
	jsAnyOf    = regexp.MustCompile(jsOperand + `\s+anyof\s+` + jsList)
	jsNoneOf   = regexp.MustCompile(jsOperand + `\s+noneof\s+` + jsList)
	jsSize     = regexp.MustCompile(jsOperand + `\s+size\s+(\d+)`)
	jsEmpty    = regexp.MustCompile(jsOperand + `\s+empty\s+(true|false)`)
	jsFunction = regexp.MustCompile(`\.(?:length|size)\(\)`)
)

// filterJS turns a Jayway filter into JavaScript of it_ (the @) and root_ (the $).
func filterJS(filter string) string {
	f := jsRegexOp.ReplaceAllString(filter, "$2.test($1)")
	f = jsNin.ReplaceAllString(f, "!$2.includes($1)")
	f = jsIn.ReplaceAllString(f, "$2.includes($1)")
	f = jsSubsetOf.ReplaceAllString(f, "($1).every(x_ => $2.includes(x_))")
	f = jsAnyOf.ReplaceAllString(f, "($1).some(x_ => $2.includes(x_))")
	f = jsNoneOf.ReplaceAllString(f, "!($1).some(x_ => $2.includes(x_))")
	f = jsSize.ReplaceAllString(f, "($1).length == $2")
	f = jsEmpty.ReplaceAllString(f, "(($1).length == 0) == $2")
	f = jsFunction.ReplaceAllString(f, ".length")
	return current(f)
}

var jsPathName = regexp.MustCompile(`^\.([\w-]+)`)

// current turns the @ and $ of a filter into JavaScript variables, outside quotes and regular
// expressions; a name with - in it is written as ['a-b'].
func current(filter string) string {
	var b strings.Builder
	var quoted byte
	last := byte('(') // before the filter: where a regular expression may begin
	for i := 0; i < len(filter); i++ {
		c := filter[i]
		switch {
		case quoted != 0:
			b.WriteByte(c)
			if c == '\\' && i+1 < len(filter) {
				i++
				b.WriteByte(filter[i])
			} else if c == quoted {
				quoted = 0
			}
			continue
		case c == '\'' || c == '"':
			quoted = c
		case c == '/' && strings.IndexByte("(,=!&|?:~[{;", last) >= 0:
			quoted = '/'
		case c == '@' || c == '$' && i+1 < len(filter) && (filter[i+1] == '.' || filter[i+1] == '['):
			if c == '@' {
				b.WriteString("it_")
			} else {
				b.WriteString("root_")
			}
			for {
				m := jsPathName.FindStringSubmatch(filter[i+1:])
				if m == nil {
					break
				}
				if strings.Contains(m[1], "-") {
					b.WriteString("['" + m[1] + "']")
				} else {
					b.WriteString(m[0])
				}
				i += len(m[0])
			}
			last = 'a'
			continue
		}
		b.WriteByte(c)
		if c != ' ' && c != '\t' {
			last = c
		}
	}
	return b.String()
}
