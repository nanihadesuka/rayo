package mock

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // for now timezone=, wherever rayo runs

	"github.com/dop251/goja"
)

// What a body may hold, as in WireMock's response templating: {{now}}, {{request...}}, {{jsonPath}},
// {{randomValue}}, {{regexExtract}} and {{parameters...}}. Without transformers a body is filled in as
// rayo always has; with transformers: [response-template] as WireMock does, {{x}} escaped for HTML and
// {{{x}}} as it is; with other transformers, or none listed, it is sent as it is.

type template struct {
	parts []func(c *templateContext) string
}

type templateContext struct {
	request *http.Request
	query   url.Values
	body    string
	groups  map[string][]string
	params  map[string]any
	vm      *goja.Runtime
}

var (
	templateTag   = regexp.MustCompile(`(?s)\{\{\{(.*?)\}\}\}|\{\{(.*?)\}\}`)
	templateWords = regexp.MustCompile(`(\w+=)?'[^']*'|(\w+=)?"[^"]*"|[^\s'"]+`)
	templateAgo   = regexp.MustCompile(`^(-?\d+) (seconds?|minutes?|hours?|days?|weeks?|months?|years?)$`)
)

func literal(s string) *template {
	return &template{parts: []func(*templateContext) string{func(*templateContext) string { return s }}}
}

func parseTemplate(text string, templating int) (*template, error) {
	if templating == templatedNot {
		return literal(text), nil
	}
	t := &template{}
	names := map[string]bool{}
	last := 0
	for _, at := range templateTag.FindAllStringSubmatchIndex(text, -1) {
		t.parts = append(t.parts, literal(text[last:at[0]]).parts...)
		last = at[1]
		raw, inside := at[2] >= 0, ""
		if raw {
			inside = text[at[2]:at[3]]
		} else {
			inside = text[at[4]:at[5]]
		}
		part, err := parseTag(strings.TrimSpace(inside), names)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", text[at[0]:at[1]], err)
		}
		if templating == templatedHandlebars && !raw {
			escaped := part
			part = func(c *templateContext) string { return htmlEscaper.Replace(escaped(c)) }
		}
		t.parts = append(t.parts, part)
	}
	t.parts = append(t.parts, literal(text[last:]).parts...)
	return t, nil
}

// htmlEscaper is Handlebars'.
var htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#x27;", "`", "&#x60;", "=", "&#x3D;")

// arguments are the name='value' words of a tag.
func arguments(words []string) (map[string]string, error) {
	args := map[string]string{}
	for _, w := range words {
		name, value, ok := strings.Cut(w, "=")
		if !ok || name == "" {
			return nil, fmt.Errorf("expected name='value', not %s", w)
		}
		args[name] = unquote(value)
	}
	return args, nil
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '\'' || s[0] == '"') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

func parseTag(tag string, names map[string]bool) (func(*templateContext) string, error) {
	words := templateWords.FindAllString(tag, -1)
	if len(words) == 0 {
		return nil, errors.New("an empty template")
	}
	switch words[0] {
	case "now":
		return parseNow(words[1:])
	case "randomValue":
		return parseRandom(words[1:])
	case "jsonPath":
		if len(words) != 3 {
			return nil, errors.New("use jsonPath request.body '$.a'")
		}
		value, err := parseValue(words[1], names)
		if err != nil {
			return nil, err
		}
		steps, err := parseJSONPath(unquote(words[2]))
		if err != nil {
			return nil, err
		}
		return func(c *templateContext) string {
			doc, err := decodeJSON([]byte(value(c)))
			if err != nil {
				return ""
			}
			text, _ := jsonPathEval(c.vm, steps, doc).text()
			return text
		}, nil
	case "regexExtract":
		if len(words) != 3 && len(words) != 4 {
			return nil, errors.New("use regexExtract request.path '(.+)', or with a name to keep the groups in")
		}
		value, err := parseValue(words[1], names)
		if err != nil {
			return nil, err
		}
		pattern, err := regexp.Compile(unquote(words[2]))
		if err != nil {
			return nil, err
		}
		if len(words) == 3 { // the first match
			return func(c *templateContext) string { return pattern.FindString(value(c)) }, nil
		}
		name := unquote(words[3])
		names[name] = true
		return func(c *templateContext) string {
			if m := pattern.FindStringSubmatch(value(c)); m != nil {
				c.groups[name] = m[1:]
			}
			return ""
		}, nil
	}
	if len(words) != 1 {
		return nil, errors.New("unknown template")
	}
	return parseValue(words[0], names)
}

// selector reads a.b, a.[b] and a.[0] alike.
var selector = regexp.MustCompile(`\.\[([^\]]*)\]|\.([^.\[]+)`)

func parts(word string) []string {
	var out []string
	for _, m := range selector.FindAllStringSubmatch("."+word, -1) {
		out = append(out, m[1]+m[2])
	}
	return out
}

func index(list []string, at string) string {
	i, err := strconv.Atoi(at)
	if err != nil || i < 0 || i >= len(list) {
		return ""
	}
	return list[i]
}

// parseValue reads one value: of the request, of the parameters, or of a group of regexExtract.
func parseValue(word string, names map[string]bool) (func(*templateContext) string, error) {
	if q := unquote(word); q != word {
		return func(*templateContext) string { return q }, nil
	}
	p := parts(word)
	unknown := errors.New("unknown template")
	if len(p) == 0 {
		return nil, unknown
	}
	if p[0] == "parameters" && len(p) > 1 {
		return func(c *templateContext) string {
			var v any = c.params
			for _, k := range p[1:] {
				m, _ := v.(map[string]any)
				v = m[k]
			}
			switch v := v.(type) {
			case nil:
				return ""
			case string:
				return v
			}
			return compactJSON(v)
		}, nil
	}
	if p[0] != "request" {
		if names[p[0]] && len(p) == 2 {
			return func(c *templateContext) string { return index(c.groups[p[0]], p[1]) }, nil
		}
		return nil, unknown
	}
	at := func(i int) string {
		if i < len(p) {
			return p[i]
		}
		return ""
	}
	switch {
	case len(p) == 2 && p[1] == "method":
		return func(c *templateContext) string { return c.request.Method }, nil
	case len(p) == 2 && p[1] == "path":
		return func(c *templateContext) string { return c.request.URL.Path }, nil
	case len(p) == 2 && p[1] == "url":
		return func(c *templateContext) string { return c.request.URL.RequestURI() }, nil
	case len(p) == 2 && p[1] == "body":
		return func(c *templateContext) string { return c.body }, nil
	case len(p) >= 3 && (p[1] == "query" || p[1] == "headers"):
		// request.query.a.[0] is the first a; request.query.a.b is the one named a.b
		name, which := strings.Join(p[2:], "."), ""
		if _, err := strconv.Atoi(p[len(p)-1]); err == nil && len(p) >= 4 {
			name, which = strings.Join(p[2:len(p)-1], "."), p[len(p)-1]
		}
		values := func(c *templateContext) []string { return c.query[name] }
		if p[1] == "headers" {
			values = func(c *templateContext) []string { return c.request.Header.Values(name) }
		}
		return func(c *templateContext) string {
			if which == "" {
				return index(values(c), "0")
			}
			return index(values(c), which)
		}, nil
	case len(p) == 3 && p[1] == "cookies":
		return func(c *templateContext) string {
			if cookie, err := c.request.Cookie(p[2]); err == nil {
				return cookie.Value
			}
			return ""
		}, nil
	case len(p) == 3 && p[1] == "pathSegments":
		return func(c *templateContext) string {
			return index(strings.Split(strings.Trim(c.request.URL.Path, "/"), "/"), at(2))
		}, nil
	}
	return nil, unknown
}

func parseNow(words []string) (func(*templateContext) string, error) {
	args, err := arguments(words)
	if err != nil {
		return nil, err
	}
	shift := func(t time.Time) time.Time { return t }
	if o, ok := args["offset"]; ok {
		m := templateAgo.FindStringSubmatch(o)
		if m == nil {
			return nil, errors.New("use offset='-8 minutes'")
		}
		n, _ := strconv.Atoi(m[1])
		switch unit := strings.TrimSuffix(m[2], "s"); unit {
		case "month":
			shift = func(t time.Time) time.Time { return t.AddDate(0, n, 0) }
		case "year":
			shift = func(t time.Time) time.Time { return t.AddDate(n, 0, 0) }
		default:
			d := map[string]time.Duration{"second": time.Second, "minute": time.Minute, "hour": time.Hour, "day": 24 * time.Hour, "week": 7 * 24 * time.Hour}[unit]
			shift = func(t time.Time) time.Time { return t.Add(time.Duration(n) * d) }
		}
	}
	zone := time.UTC
	if z, ok := args["timezone"]; ok {
		if zone, err = time.LoadLocation(z); err != nil {
			return nil, fmt.Errorf("timezone: %w", err)
		}
	}
	format := func(t time.Time) string { return t.Format("2006-01-02T15:04:05Z07:00") }
	switch f := args["format"]; f {
	case "":
	case "epoch":
		format = func(t time.Time) string { return strconv.FormatInt(t.UnixMilli(), 10) }
	case "unix":
		format = func(t time.Time) string { return strconv.FormatInt(t.Unix(), 10) }
	default:
		if format, err = javaDate(f); err != nil {
			return nil, err
		}
	}
	for k := range args {
		if k != "offset" && k != "timezone" && k != "format" {
			return nil, fmt.Errorf("now takes offset, format and timezone, not %s", k)
		}
	}
	return func(*templateContext) string { return format(shift(time.Now()).In(zone)) }, nil
}

const apostrophe = '\''

// javaFields cuts a Java date pattern into runs of one letter, quoted texts, and what is between.
func javaFields(pattern string) []string {
	var out []string
	letter := func(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
	for i := 0; i < len(pattern); {
		j := i + 1
		switch c := pattern[i]; {
		case c == apostrophe:
			if k := strings.IndexByte(pattern[j:], apostrophe); k >= 0 {
				j += k + 1
			} else {
				j = len(pattern)
			}
		case letter(c):
			for j < len(pattern) && pattern[j] == c {
				j++
			}
		default:
			for j < len(pattern) && !letter(pattern[j]) && pattern[j] != apostrophe {
				j++
			}
		}
		out = append(out, pattern[i:j])
		i = j
	}
	return out
}

// javaDate formats as a Java date pattern such as yyyy-MM-dd'T'HH:mm:ss.SSSXXX, field by field.
func javaDate(pattern string) (func(time.Time) string, error) {
	var fields []func(time.Time) string
	for _, f := range javaFields(pattern) {
		layout := map[string]string{
			"yyyy": "2006", "YYYY": "2006", "yy": "06", "YY": "06", "MMMM": "January", "MMM": "Jan", "MM": "01", "M": "1",
			"dd": "02", "d": "2", "HH": "15", "hh": "03", "h": "3", "mm": "04", "m": "4", "ss": "05", "s": "5",
			"EEEE": "Monday", "EEE": "Mon", "E": "Mon", "a": "PM", "z": "MST", "Z": "-0700", "X": "Z07", "XX": "Z0700", "XXX": "Z07:00",
		}[f]
		switch {
		case strings.HasPrefix(f, "'"):
			text := strings.Trim(f, "'")
			if f == "''" {
				text = "'"
			}
			fields = append(fields, func(time.Time) string { return text })
		case f[0] == 'S':
			n := len(f)
			fields = append(fields, func(t time.Time) string { return fmt.Sprintf("%09d", t.Nanosecond())[:min(n, 9)] })
		case f == "H":
			fields = append(fields, func(t time.Time) string { return strconv.Itoa(t.Hour()) })
		case layout != "":
			fields = append(fields, func(t time.Time) string { return t.Format(layout) })
		case f[0] >= 'a' && f[0] <= 'z' || f[0] >= 'A' && f[0] <= 'Z':
			return nil, fmt.Errorf("format: %s isn't supported", f)
		default:
			text := f
			fields = append(fields, func(time.Time) string { return text })
		}
	}
	return func(t time.Time) string {
		var b strings.Builder
		for _, f := range fields {
			b.WriteString(f(t))
		}
		return b.String()
	}, nil
}

const (
	lettersLower = "abcdefghijklmnopqrstuvwxyz"
	digits       = "0123456789"
)

func parseRandom(words []string) (func(*templateContext) string, error) {
	args, err := arguments(words)
	if err != nil {
		return nil, err
	}
	kind := args["type"]
	if kind == "UUID" {
		return func(*templateContext) string { return uuid() }, nil
	}
	set := map[string]string{
		"ALPHANUMERIC": lettersLower + strings.ToUpper(lettersLower) + digits, "ALPHABETIC": lettersLower + strings.ToUpper(lettersLower),
		"NUMERIC": digits, "HEXADECIMAL": digits + "abcdef", "ALPHANUMERIC_AND_SYMBOLS": lettersLower + strings.ToUpper(lettersLower) + digits + "!#$%&()*+,-./:;<=>?@[]^_{|}~",
	}[kind]
	if set == "" {
		return nil, errors.New("randomValue takes type= ALPHANUMERIC, ALPHABETIC, NUMERIC, HEXADECIMAL, ALPHANUMERIC_AND_SYMBOLS or UUID")
	}
	n, err := strconv.Atoi(args["length"])
	if err != nil || n < 1 {
		return nil, errors.New("randomValue needs length=, a number")
	}
	upper := args["uppercase"] == "true"
	return func(*templateContext) string {
		b := make([]byte, n)
		for i := range b {
			k, _ := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
			b[i] = set[k.Int64()]
		}
		if upper {
			return strings.ToUpper(string(b))
		}
		return string(b)
	}, nil
}

func uuid() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (t *template) render(q *http.Request, body string, params map[string]any, vm *goja.Runtime) string {
	c := &templateContext{request: q, query: q.URL.Query(), body: body, groups: map[string][]string{}, params: params, vm: vm}
	var b strings.Builder
	for _, part := range t.parts {
		b.WriteString(part(c))
	}
	return b.String()
}
