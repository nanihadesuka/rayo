package mock

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// What a body may hold: {{now}}, {{request...}} and {{regexExtract}}.

type template struct {
	parts []func(c *templateContext) string
}

type templateContext struct {
	request *http.Request
	query   url.Values
	body    string
	groups  map[string][]string
}

var (
	templateTag   = regexp.MustCompile(`\{\{(.*?)\}\}`)
	templateWords = regexp.MustCompile(`(\w+=)?'[^']*'|[^\s']+`)
	templateAgo   = regexp.MustCompile(`^offset='(-?\d+) (seconds?|minutes?|hours?|days?)'$`)
	templateUnits = map[string]time.Duration{"second": time.Second, "minute": time.Minute, "hour": time.Hour, "day": 24 * time.Hour}
)

func parseTemplate(text string) (*template, error) {
	t := &template{}
	names := map[string]bool{}
	literal := func(s string) {
		t.parts = append(t.parts, func(*templateContext) string { return s })
	}
	last := 0
	for _, at := range templateTag.FindAllStringSubmatchIndex(text, -1) {
		literal(text[last:at[0]])
		last = at[1]
		part, err := parseTag(text[at[2]:at[3]], names)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", text[at[0]:at[1]], err)
		}
		t.parts = append(t.parts, part)
	}
	literal(text[last:])
	return t, nil
}

func parseTag(tag string, names map[string]bool) (func(*templateContext) string, error) {
	words := templateWords.FindAllString(tag, -1)
	switch {
	case len(words) == 0:
		return nil, errors.New("an empty template")
	case words[0] == "now":
		var offset time.Duration
		if len(words) == 2 {
			m := templateAgo.FindStringSubmatch(words[1])
			if m == nil {
				return nil, errors.New("use offset='-8 minutes'")
			}
			n, err := strconv.Atoi(m[1])
			if err != nil {
				return nil, err
			}
			offset = time.Duration(n) * templateUnits[strings.TrimSuffix(m[2], "s")]
		} else if len(words) > 2 {
			return nil, errors.New("now only takes an offset")
		}
		return func(*templateContext) string {
			return time.Now().Add(offset).UTC().Format("2006-01-02T15:04:05Z")
		}, nil
	case words[0] == "regexExtract" && len(words) == 4:
		value, err := parseTag(words[1], names)
		if err != nil {
			return nil, err
		}
		pattern, err := regexp.Compile(strings.Trim(words[2], "'"))
		if err != nil {
			return nil, err
		}
		name := strings.Trim(words[3], "'")
		names[name] = true
		return func(c *templateContext) string {
			if m := pattern.FindStringSubmatch(value(c)); m != nil {
				c.groups[name] = m[1:]
			}
			return ""
		}, nil
	case len(words) != 1:
		return nil, errors.New("unknown template")
	}
	word := words[0]
	if name, ok := strings.CutPrefix(word, "request.query."); ok {
		return func(c *templateContext) string { return c.query.Get(name) }, nil
	}
	if name, ok := strings.CutPrefix(word, "request.headers."); ok {
		return func(c *templateContext) string { return c.request.Header.Get(name) }, nil
	}
	switch word {
	case "request.path":
		return func(c *templateContext) string { return c.request.URL.Path }, nil
	case "request.url":
		return func(c *templateContext) string { return c.request.URL.RequestURI() }, nil
	case "request.body":
		return func(c *templateContext) string { return c.body }, nil
	}
	name, index, _ := strings.Cut(word, ".")
	i, err := strconv.Atoi(index)
	if err != nil || i < 0 || !names[name] {
		return nil, errors.New("unknown template")
	}
	return func(c *templateContext) string {
		if g := c.groups[name]; i < len(g) {
			return g[i]
		}
		return ""
	}, nil
}

func (t *template) render(q *http.Request, body string) string {
	c := &templateContext{request: q, query: q.URL.Query(), body: body, groups: map[string][]string{}}
	var b strings.Builder
	for _, part := range t.parts {
		b.WriteString(part(c))
	}
	return b.String()
}
