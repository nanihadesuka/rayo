package mock

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/dop251/goja"
)

// Rule is a WireMock mapping: what to answer to which calls.
type Rule struct {
	Id                    string      `json:"id"`
	Name                  string      `json:"name"`
	Priority              *int        `json:"priority"`
	ScenarioName          string      `json:"scenarioName"`
	RequiredScenarioState string      `json:"requiredScenarioState"`
	NewScenarioState      string      `json:"newScenarioState"`
	Request               ruleRequest `json:"request"`
	Response              ruleAnswer  `json:"response"`

	body *template
}

type ruleRequest struct {
	Method          string              `json:"method"`
	Url             string              `json:"url"`
	UrlPattern      string              `json:"urlPattern"`
	UrlPath         string              `json:"urlPath"`
	UrlPathPattern  string              `json:"urlPathPattern"`
	QueryParameters map[string]*matcher `json:"queryParameters"`
	Headers         map[string]*matcher `json:"headers"`
	BodyPatterns    []*matcher          `json:"bodyPatterns"`

	urlPattern, pathPattern *regexp.Regexp
}

type ruleAnswer struct {
	Status                 int               `json:"status"`
	Headers                map[string]string `json:"headers"`
	Body                   string            `json:"body"`
	JsonBody               json.RawMessage   `json:"jsonBody"`
	BodyFileName           string            `json:"bodyFileName"`
	FixedDelayMilliseconds int               `json:"fixedDelayMilliseconds"`
}

type matcher struct {
	EqualTo         *string `json:"equalTo"`
	Contains        *string `json:"contains"`
	Matches         *string `json:"matches"`
	MatchesJsonPath *string `json:"matchesJsonPath"`
	Absent          bool    `json:"absent"`

	pattern *regexp.Regexp
	path    []jsonStep
}

// Pattern is a regular expression for a whole value.
func Pattern(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile(`(?s)^(?:` + pattern + `)$`)
}

func strict(data []byte, into any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	return d.Decode(into)
}

// ParseRule reads one mapping.
func ParseRule(data []byte) (*Rule, error) {
	r := &Rule{}
	if err := strict(data, r); err != nil {
		return nil, err
	}
	q := &r.Request
	if q.Method == "" {
		q.Method = "ANY"
	}
	addresses := 0
	for _, a := range []string{q.Url, q.UrlPattern, q.UrlPath, q.UrlPathPattern} {
		if a != "" {
			addresses++
		}
	}
	if addresses != 1 {
		return nil, errors.New("a request needs one of url, urlPattern, urlPath, urlPathPattern")
	}
	var err error
	if q.UrlPattern != "" {
		if q.urlPattern, err = Pattern(q.UrlPattern); err != nil {
			return nil, err
		}
	}
	if q.UrlPathPattern != "" {
		if q.pathPattern, err = Pattern(q.UrlPathPattern); err != nil {
			return nil, err
		}
	}
	all := append([]*matcher{}, q.BodyPatterns...)
	for _, named := range []map[string]*matcher{q.QueryParameters, q.Headers} {
		for _, m := range named {
			all = append(all, m)
		}
	}
	for _, m := range all {
		if err := m.prepare(); err != nil {
			return nil, err
		}
	}
	if (r.RequiredScenarioState != "" || r.NewScenarioState != "") && r.ScenarioName == "" {
		return nil, errors.New("a state needs a scenarioName")
	}
	a := &r.Response
	if a.Status == 0 {
		a.Status = 200
	}
	if a.Status < 200 || a.Status > 599 {
		return nil, fmt.Errorf("not a status: %d", a.Status)
	}
	bodies := 0
	for _, set := range []bool{a.Body != "", a.JsonBody != nil, a.BodyFileName != ""} {
		if set {
			bodies++
		}
	}
	if bodies > 1 {
		return nil, errors.New("a response takes one of body, jsonBody, bodyFileName")
	}
	if a.JsonBody != nil {
		a.Body = string(a.JsonBody)
	}
	if a.BodyFileName == "" {
		r.body, err = parseTemplate(a.Body)
	}
	return r, err
}

func (m *matcher) prepare() (err error) {
	if m == nil {
		return errors.New("an empty matcher")
	}
	n := 0
	for _, set := range []bool{m.EqualTo != nil, m.Contains != nil, m.Matches != nil, m.MatchesJsonPath != nil, m.Absent} {
		if set {
			n++
		}
	}
	if n != 1 {
		return errors.New("a matcher takes one of equalTo, contains, matches, matchesJsonPath, absent")
	}
	if m.Matches != nil {
		m.pattern, err = Pattern(*m.Matches)
	}
	if m.MatchesJsonPath != nil {
		m.path, err = parseJSONPath(*m.MatchesJsonPath)
	}
	return err
}

func (m *matcher) fits(vm *goja.Runtime, value string, there bool) bool {
	switch {
	case m.Absent || !there:
		return m.Absent && !there
	case m.EqualTo != nil:
		return value == *m.EqualTo
	case m.Contains != nil:
		return strings.Contains(value, *m.Contains)
	case m.Matches != nil:
		return m.pattern.MatchString(value)
	}
	var document any
	return json.Unmarshal([]byte(value), &document) == nil && jsonPathFinds(vm, m.path, document)
}

func (m *matcher) fitsAny(vm *goja.Runtime, values []string) bool {
	for _, v := range values {
		if m.fits(vm, v, true) {
			return true
		}
	}
	return len(values) == 0 && m.fits(vm, "", false)
}

func (r *Rule) fits(vm *goja.Runtime, q *http.Request, body string) bool {
	w := &r.Request
	if w.Method != "ANY" && !strings.EqualFold(w.Method, q.Method) {
		return false
	}
	uri := q.URL.RequestURI()
	switch {
	case w.Url != "" && w.Url != uri,
		w.urlPattern != nil && !w.urlPattern.MatchString(uri),
		w.UrlPath != "" && w.UrlPath != q.URL.EscapedPath(),
		w.pathPattern != nil && !w.pathPattern.MatchString(q.URL.EscapedPath()):
		return false
	}
	query := q.URL.Query()
	for name, m := range w.QueryParameters {
		if !m.fitsAny(vm, query[name]) {
			return false
		}
	}
	for name, m := range w.Headers {
		if !m.fitsAny(vm, q.Header.Values(name)) {
			return false
		}
	}
	for _, m := range w.BodyPatterns {
		if !m.fits(vm, body, body != "") {
			return false
		}
	}
	return true
}

func (r *Rule) priority() int {
	if r.Priority != nil {
		return *r.Priority
	}
	return 5
}
