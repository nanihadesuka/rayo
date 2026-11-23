package mock

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"slices"
	"strings"
)

// equalToJson, as WireMock's (JsonUnit): objects in any order of keys; with ignoreExtraElements, other
// keys and other items of arrays may be there too; with ignoreArrayOrder, the items in any order. Numbers
// are equal by value. Placeholders stand for a value: ${json-unit.ignore}, ${json-unit.ignore-element} (which may be absent), ${json-unit.any-string},
// ${json-unit.any-number}, ${json-unit.any-boolean} and ${json-unit.regex}pattern.

const (
	placeholderIgnore        = "${json-unit.ignore}"
	placeholderIgnoreElement = "${json-unit.ignore-element}"
	placeholderString        = "${json-unit.any-string}"
	placeholderNumber        = "${json-unit.any-number}"
	placeholderBoolean       = "${json-unit.any-boolean}"
	placeholderRegex         = "${json-unit.regex}"
)

// jsonDiffers says where got differs from want, or "" if it doesn't.
func jsonDiffers(at string, want, got any, anyOrder, extra bool) string {
	if s, ok := want.(string); ok && strings.HasPrefix(s, "${json-unit.") {
		return placeholderMiss(at, s, got)
	}
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return fmt.Sprintf("%s: %s, expected an object", at, short(got))
		}
		for _, k := range sortedKeys(w) {
			child := at + pathPart(k)
			v, there := g[k]
			if !there {
				if s, _ := w[k].(string); s == placeholderIgnoreElement {
					continue // as JsonUnit: the only one that may be absent
				}
				return child + ": absent, expected " + short(w[k])
			}
			if why := jsonDiffers(child, w[k], v, anyOrder, extra); why != "" {
				return why
			}
		}
		if !extra {
			for _, k := range sortedKeys(g) {
				if _, wanted := w[k]; !wanted {
					return at + pathPart(k) + ": not expected"
				}
			}
		}
		return ""
	case []any:
		g, ok := got.([]any)
		if !ok {
			return fmt.Sprintf("%s: %s, expected an array", at, short(got))
		}
		if len(g) < len(w) || !extra && len(g) != len(w) {
			return fmt.Sprintf("%s: %d items, expected %d", at, len(g), len(w))
		}
		if !anyOrder {
			for i := range w {
				if why := jsonDiffers(fmt.Sprintf("%s[%d]", at, i), w[i], g[i], anyOrder, extra); why != "" {
					return why
				}
			}
			return ""
		}
		fits := func(i, j int) bool { return jsonDiffers("", w[i], g[j], anyOrder, extra) == "" }
		if pairs(len(w), len(g), fits) < len(w) {
			for i := range w { // say which one, if one alone fits nothing
				if !slices.ContainsFunc(g, func(v any) bool { return jsonDiffers("", w[i], v, anyOrder, extra) == "" }) {
					return fmt.Sprintf("%s: no item like %s", at, short(w[i]))
				}
			}
			return at + ": the items don't pair up"
		}
		return ""
	case json.Number:
		if n, ok := got.(json.Number); ok && sameNumber(w, n) {
			return ""
		}
	default:
		if w == got {
			return ""
		}
	}
	return fmt.Sprintf("%s: %s, expected %s", at, short(got), short(want))
}

func placeholderMiss(at, p string, got any) string {
	ok := false
	switch {
	case p == placeholderIgnore, p == placeholderIgnoreElement:
		ok = true
	case p == placeholderString:
		_, ok = got.(string)
	case p == placeholderNumber:
		_, ok = got.(json.Number)
	case p == placeholderBoolean:
		_, ok = got.(bool)
	case strings.HasPrefix(p, placeholderRegex):
		s, isText := got.(string)
		re, err := Pattern(strings.TrimPrefix(p, placeholderRegex))
		ok = isText && err == nil && re.MatchString(s)
	default:
		s, _ := got.(string)
		ok = s == p // not a placeholder after all
	}
	if ok {
		return ""
	}
	return fmt.Sprintf("%s: %s, expected %s", at, short(got), p)
}

func sameNumber(a, b json.Number) bool {
	if a == b {
		return true
	}
	x, okX := new(big.Rat).SetString(a.String())
	y, okY := new(big.Rat).SetString(b.String())
	return okX && okY && x.Cmp(y) == 0
}

var jsonKeyName = regexp.MustCompile(`^[A-Za-z_][\w]*$`)

func pathPart(k string) string {
	if jsonKeyName.MatchString(k) {
		return "." + k
	}
	return "['" + strings.ReplaceAll(k, "'", `\'`) + "']"
}

// equalToXml: the same elements, attributes in any order, the same text but for the spaces around it;
// spaces between elements, comments and processing instructions don't count. Names are compared by
// their namespace, not their prefix.

type xmlToken struct {
	kind  byte // <, >, or t for text
	name  xml.Name
	attrs []xml.Attr
	text  string
}

func xmlTokens(text string) ([]xmlToken, error) {
	d := xml.NewDecoder(strings.NewReader(text))
	d.Strict = true
	var list []xmlToken
	for {
		t, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := t.(type) {
		case xml.StartElement:
			attrs := slices.DeleteFunc(slices.Clone(t.Attr), func(a xml.Attr) bool {
				return a.Name.Space == "xmlns" || a.Name.Space == "" && a.Name.Local == "xmlns"
			})
			slices.SortFunc(attrs, func(a, b xml.Attr) int {
				return strings.Compare(a.Name.Space+" "+a.Name.Local, b.Name.Space+" "+b.Name.Local)
			})
			list = append(list, xmlToken{kind: '<', name: t.Name, attrs: attrs})
		case xml.EndElement:
			list = append(list, xmlToken{kind: '>', name: t.Name})
		case xml.CharData:
			if s := strings.TrimSpace(string(t)); s != "" {
				if n := len(list); n > 0 && list[n-1].kind == 't' { // text broken by a comment
					list[n-1].text += s
				} else {
					list = append(list, xmlToken{kind: 't', text: s})
				}
			}
		}
	}
	if len(list) == 0 {
		return nil, errors.New("no element")
	}
	return list, nil
}

func xmlDiffers(want, got []xmlToken) string {
	var path []string
	for i := range max(len(want), len(got)) {
		if i >= len(want) || i >= len(got) {
			return "/" + strings.Join(path, "/") + ": not the same elements"
		}
		w, g := want[i], got[i]
		if w.kind != g.kind || w.name != g.name {
			return "/" + strings.Join(path, "/") + ": not the same elements"
		}
		switch w.kind {
		case '<':
			path = append(path, w.name.Local)
			if !slices.Equal(w.attrs, g.attrs) {
				return "/" + strings.Join(path, "/") + ": not the same attributes"
			}
		case '>':
			path = path[:len(path)-1]
		case 't':
			if w.text != g.text {
				return fmt.Sprintf("/%s: %s, expected %s", strings.Join(path, "/"), short(g.text), short(w.text))
			}
		}
	}
	return ""
}
