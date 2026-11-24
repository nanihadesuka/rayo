// Package schema describes what files may hold, in one place: from it come the check of a file, its
// JSON Schema, its help and its tables in the docs.
package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// S is a JSON Schema.
type S map[string]any

// Type is a value of one of some JSON types.
func Type(names ...string) S {
	if len(names) == 1 {
		return S{"type": names[0]}
	}
	return S{"type": names}
}

// Text is a single value, however YAML reads it: 1234 is a text too.
func Text() S { return Type("string", "number", "boolean") }

// Enum is one of some values.
func Enum(values ...any) S { return S{"enum": values} }

// Pattern is a text that fits.
func Pattern(p string) S { return S{"type": "string", "pattern": p} }

// List is of any length.
func List(of S) S { return S{"type": "array", "items": of} }

// AnyOf is one of several.
func AnyOf(list ...S) S { return S{"anyOf": list} }

// Ref is another one, by its name in $defs.
func Ref(name string) S { return S{"$ref": "#/$defs/" + name} }

// Map is of any keys.
func Map(of S) S { return S{"type": "object", "additionalProperties": of} }

// Object has only these keys.
func Object(properties map[string]S, required ...string) S {
	s := S{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

// Dynamic also takes a ${...}, whose value isn't known yet.
func Dynamic(s S) S { return AnyOf(s, Pattern(`\$\{`)) }

// About adds what it is.
func (s S) About(about string) S {
	c := S{"description": about}
	for k, v := range s {
		c[k] = v
	}
	return c
}

// File is a whole schema, as it is written.
func (s S) File(title string, defs map[string]S) []byte {
	top := S{"$schema": "http://json-schema.org/draft-07/schema#", "title": title}
	for k, v := range s {
		top[k] = v
	}
	if len(defs) > 0 {
		top["$defs"] = defs
	}
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	e.SetIndent("", "  ")
	if err := e.Encode(top); err != nil {
		panic(err) // it only has what can be
	}
	return b.Bytes()
}

// Field is a key of a mapping that fills in a T.
type Field[T any] struct {
	Name     string
	Takes    string // for people
	Default  string // for people; empty if there is none
	Required bool
	About    string
	Schema   S
	Read     func(t *T, n *yaml.Node) error
}

// Error is a mistake and its line.
type Error struct {
	Line     int
	What     string
	Expected string // the keys it takes, of an unknown one
}

func (e *Error) Error() string { return e.What }

// placed is an error that says its place already.
type placed struct{ err error }

func (e *placed) Error() string { return e.err.Error() }
func (e *placed) Unwrap() error { return e.err }

// Placed keeps an error as it is; none stays none.
func Placed(err error) error {
	if err == nil {
		return nil
	}
	return &placed{err}
}

// Read checks a mapping and fills in a T.
func Read[T any](fields []Field[T], t *T, n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return &Error{n.Line, "takes " + names(fields), names(fields)}
	}
	var seen []string
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, v := n.Content[i], n.Content[i+1]
		at := slices.IndexFunc(fields, func(f Field[T]) bool { return f.Name == key.Value })
		if at < 0 {
			return &Error{key.Line, fmt.Sprintf("unknown key %q (it takes %s)", key.Value, names(fields)), names(fields)}
		}
		if slices.Contains(seen, key.Value) {
			return &Error{key.Line, key.Value + " is there twice", ""}
		}
		seen = append(seen, key.Value)
		if err := fields[at].Read(t, v); err != nil {
			switch err := err.(type) {
			case *placed:
				return err
			case *Error:
				return &Error{err.Line, key.Value + ": " + err.What, err.Expected}
			}
			return &Error{v.Line, key.Value + ": " + err.Error(), ""}
		}
	}
	for _, f := range fields {
		if f.Required && !slices.Contains(seen, f.Name) {
			return &Error{n.Line, "needs " + f.Name, ""}
		}
	}
	return nil
}

func names[T any](fields []Field[T]) string {
	var list []string
	for _, f := range fields {
		list = append(list, f.Name)
	}
	return strings.Join(list, ", ")
}

// Of is the schema of a mapping.
func Of[T any](fields []Field[T]) S {
	properties := map[string]S{}
	var required []string
	for _, f := range fields {
		properties[f.Name] = f.Schema.About(bare(f.full()))
		if f.Required {
			required = append(required, f.Name)
		}
	}
	return Object(properties, required...)
}

func (f Field[T]) full() string {
	switch {
	case f.Required:
		return f.About + " (required)"
	case f.Default != "":
		return fmt.Sprintf("%s (default: %s)", f.About, f.Default)
	}
	return f.About
}

// Table is for the docs.
func Table[T any](fields []Field[T]) string {
	var b strings.Builder
	b.WriteString("| Key | Takes | |\n|---|---|---|\n")
	for _, f := range fields {
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", f.Name, f.Takes, f.full())
	}
	return b.String()
}

// Help is for the terminal.
func Help[T any](fields []Field[T]) string {
	var b strings.Builder
	for _, f := range fields {
		fmt.Fprintf(&b, "  %-16s %-26s %s\n", f.Name, bare(f.Takes), bare(f.full()))
	}
	return b.String()
}

// --- Kinds of fields ---

func scalar(n *yaml.Node, into any, what string) error {
	if n.Kind != yaml.ScalarNode || n.Decode(into) != nil {
		return fmt.Errorf("takes %s", what)
	}
	return nil
}

// TextField is a field that takes a text.
func TextField[T any](name, takes, about string, at func(t *T) *string) Field[T] {
	return Field[T]{Name: name, Takes: takes, About: about, Schema: Text(), Read: func(t *T, n *yaml.Node) error {
		if err := scalar(n, at(t), "a "+takes); err != nil {
			return err
		}
		if *at(t) == "" {
			return fmt.Errorf("is empty")
		}
		return nil
	}}
}

// FlagField is a field that takes true or false.
func FlagField[T any](name, about string, at func(t *T) *bool) Field[T] {
	return Field[T]{Name: name, Takes: "`true` or `false`", About: about, Schema: Type("boolean"), Read: func(t *T, n *yaml.Node) error {
		return scalar(n, at(t), "true or false")
	}}
}

// NumberField is a field that takes a number within limits.
func NumberField[T any, N int | float64](name, takes, about string, low, high N, set func(t *T, v N)) Field[T] {
	kind := "number"
	if _, whole := any(low).(int); whole {
		kind = "integer"
	}
	s := S{"type": kind, "minimum": low}
	if high > low {
		s["maximum"] = high
	}
	return Field[T]{Name: name, Takes: takes, About: about, Schema: s, Read: func(t *T, n *yaml.Node) error {
		var v N
		if err := scalar(n, &v, takes); err != nil {
			return err
		}
		if v < low || high > low && v > high {
			return fmt.Errorf("takes %s, not %v", takes, v)
		}
		set(t, v)
		return nil
	}}
}

// Optional marks the default of a field.
func (f Field[T]) Optional(otherwise string) Field[T] { f.Default = otherwise; return f }

// Needed marks a field as required.
func (f Field[T]) Needed() Field[T] { f.Required = true; return f }

// --- What is written, from Go types ---

// Shower is a type that is written as another.
type Shower interface{ Shown() any }

// Column is a key of what is written.
type Column struct{ Key, Takes, About string }

// Written is the schema of a struct as it is written. The tag about says what each key is.
func Written(v any) S { return writtenType(reflect.TypeOf(v)) }

func writtenType(t reflect.Type) S {
	if s, ok := reflect.Zero(t).Interface().(Shower); ok {
		return Written(s.Shown())
	}
	switch t.Kind() {
	case reflect.Bool:
		return Type("boolean")
	case reflect.Int:
		return Type("integer")
	case reflect.Float64:
		return Type("number")
	case reflect.Slice:
		return List(writtenType(t.Elem()))
	case reflect.Pointer: // null when there is none
		return AnyOf(writtenType(t.Elem()), Type("null"))
	case reflect.Struct:
		properties := map[string]S{}
		var required []string
		for i := range t.NumField() {
			f := t.Field(i)
			s := writtenType(f.Type)
			if values := f.Tag.Get("enum"); values != "" {
				var list []any
				for _, v := range strings.Split(values, ",") {
					list = append(list, v)
				}
				s = Enum(list...)
			}
			properties[f.Tag.Get("json")] = s.About(bare(code(f.Tag.Get("about"))))
			required = append(required, f.Tag.Get("json"))
		}
		return Object(properties, required...)
	}
	return Type("string")
}

// Columns are the keys of a struct as it is written.
func Columns(v any) []Column {
	var list []Column
	t := reflect.TypeOf(v)
	for i := range t.NumField() {
		f := t.Field(i)
		takes := f.Type.Kind().String()
		switch f.Type.Kind() {
		case reflect.Slice:
			takes = "list"
		case reflect.Int, reflect.Float64:
			takes = "number"
		case reflect.Bool:
			takes = "true or false"
		case reflect.String:
			takes = "text"
		case reflect.Struct:
			takes = "mapping"
		case reflect.Pointer:
			takes = "mapping or null"
		}
		list = append(list, Column{f.Tag.Get("json"), takes, f.Tag.Get("about")})
	}
	return list
}

// ColumnTable is for the docs.
func ColumnTable(v any) string {
	var b strings.Builder
	b.WriteString("| Key | | |\n|---|---|---|\n")
	for _, c := range Columns(v) {
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", c.Key, c.Takes, code(c.About))
	}
	return b.String()
}

// code turns 'this' into `this`: a tag can't hold the latter. An apostrophe is left alone.
var quoted = regexp.MustCompile(`'([^' ]+)'`)

func code(text string) string { return quoted.ReplaceAllString(text, "`$1`") }

var (
	seeLinks = regexp.MustCompile(`,? ?[sS]ee \[[^\]]+\]\([^)]+\)`)
	links    = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
)

// bare is a text of the docs where there is no Markdown, nor anywhere to go to.
func bare(text string) string {
	text = links.ReplaceAllString(seeLinks.ReplaceAllString(text, ""), "$1")
	return strings.ReplaceAll(text, "`", "")
}

// Doc is a field, for who reads about it.
type Doc struct {
	Name     string `json:"name"`
	Takes    string `json:"takes"`
	About    string `json:"about"`
	Required bool   `json:"required"`
}

// Docs describes fields.
func Docs[T any](fields []Field[T]) []Doc {
	list := []Doc{}
	for _, f := range fields {
		list = append(list, Doc{f.Name, bare(f.Takes), bare(f.full()), f.Required})
	}
	return list
}
