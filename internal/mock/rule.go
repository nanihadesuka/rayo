package mock

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/dop251/goja"
)

// Rule is a WireMock mapping: what to answer to which calls.
type Rule struct {
	Id                    string      `json:"id"`
	Uuid                  string      `json:"uuid"`
	Name                  string      `json:"name"`
	Priority              *int        `json:"priority"`
	ScenarioName          string      `json:"scenarioName"`
	RequiredScenarioState string      `json:"requiredScenarioState"`
	NewScenarioState      string      `json:"newScenarioState"`
	Request               ruleRequest `json:"request"`
	Response              ruleAnswer  `json:"response"`

	// Written by WireMock, and of no effect here.
	Persistent     *bool           `json:"persistent"`
	InsertionIndex *int            `json:"insertionIndex"`
	Metadata       json.RawMessage `json:"metadata"`

	// WireMock's, and not done here: an error rather than a mock that does less.
	PostServeActions    json.RawMessage `json:"postServeActions"`
	ServeEventListeners json.RawMessage `json:"serveEventListeners"`

	body   *template
	source string // where it was written: file#n, or empty for a flow's step
}

type ruleRequest struct {
	Method               string              `json:"method"`
	Url                  string              `json:"url"`
	UrlPattern           string              `json:"urlPattern"`
	UrlPath              string              `json:"urlPath"`
	UrlPathPattern       string              `json:"urlPathPattern"`
	UrlPathTemplate      string              `json:"urlPathTemplate"`
	Host                 *matcher            `json:"host"`
	PathParameters       map[string]*matcher `json:"pathParameters"`
	QueryParameters      map[string]*matcher `json:"queryParameters"`
	FormParameters       map[string]*matcher `json:"formParameters"`
	Headers              map[string]*matcher `json:"headers"`
	Cookies              map[string]*matcher `json:"cookies"`
	BasicAuthCredentials *struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"basicAuthCredentials"`
	BodyPatterns []*matcher `json:"bodyPatterns"`

	MultipartPatterns json.RawMessage `json:"multipartPatterns"`
	CustomMatcher     json.RawMessage `json:"customMatcher"`
	Scheme            json.RawMessage `json:"scheme"`
	Port              json.RawMessage `json:"port"`
	ClientIp          json.RawMessage `json:"clientIp"`

	urlPattern, pathPattern *regexp.Regexp
	pathNames               []string // of urlPathTemplate, in order
}

type ruleAnswer struct {
	Status                 int             `json:"status"`
	StatusMessage          string          `json:"statusMessage"` // HTTP/1.1 apps don't read it
	Headers                headers         `json:"headers"`
	Body                   string          `json:"body"`
	JsonBody               json.RawMessage `json:"jsonBody"`
	Base64Body             string          `json:"base64Body"`
	BodyFileName           string          `json:"bodyFileName"`
	FixedDelayMilliseconds int             `json:"fixedDelayMilliseconds"`
	DelayDistribution      *distribution   `json:"delayDistribution"`
	ChunkedDribbleDelay    *dribble        `json:"chunkedDribbleDelay"`
	Fault                  string          `json:"fault"`
	Transformers           []string        `json:"transformers"`
	TransformerParameters  map[string]any  `json:"transformerParameters"`
	ProxyBaseUrl           json.RawMessage `json:"proxyBaseUrl"`
	AdditionalProxyHeaders json.RawMessage `json:"additionalProxyRequestHeaders"`
	RemoveProxyHeaders     json.RawMessage `json:"removeProxyRequestHeaders"`
	ProxyUrlPrefixToRemove json.RawMessage `json:"proxyUrlPrefixToRemove"`
}

// headers are an answer's: each a text, or a list of them, as WireMock writes them.
type headers map[string][]string

func (h *headers) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*h = headers{}
	for name, v := range raw {
		var one string
		if json.Unmarshal(v, &one) == nil {
			(*h)[name] = []string{one}
			continue
		}
		var several []string
		if err := json.Unmarshal(v, &several); err != nil {
			return fmt.Errorf("header %s: a text or a list of texts", name)
		}
		(*h)[name] = several
	}
	return nil
}

// distribution is WireMock's random delay.
type distribution struct {
	Type         string  `json:"type"`
	Median       float64 `json:"median"`
	Sigma        float64 `json:"sigma"`
	Lower        float64 `json:"lower"`
	Upper        float64 `json:"upper"`
	Milliseconds float64 `json:"milliseconds"`
}

// dribble sends the body in pieces over a time.
type dribble struct {
	NumberOfChunks int `json:"numberOfChunks"`
	TotalDuration  int `json:"totalDuration"`
}

// Faults, as WireMock's.
var faults = []string{"CONNECTION_RESET_BY_PEER", "EMPTY_RESPONSE", "MALFORMED_RESPONSE_CHUNK", "RANDOM_DATA_THEN_CLOSE"}

// The kinds of templating a body can have.
const (
	templatedRayo       = iota // no transformers: rayo's, as ever
	templatedHandlebars        // transformers has response-template: as WireMock's
	templatedNot               // transformers without it: as it is
)

// templating is how the body is filled in.
func (a *ruleAnswer) templating() int {
	switch {
	case a.Transformers == nil:
		return templatedRayo
	case slices.Contains(a.Transformers, "response-template"):
		return templatedHandlebars
	}
	return templatedNot
}

type matcher struct {
	EqualTo             *string         `json:"equalTo"`
	CaseInsensitive     bool            `json:"caseInsensitive"`
	Contains            *string         `json:"contains"`
	DoesNotContain      *string         `json:"doesNotContain"`
	Matches             *string         `json:"matches"`
	DoesNotMatch        *string         `json:"doesNotMatch"`
	EqualToJson         json.RawMessage `json:"equalToJson"`
	IgnoreArrayOrder    bool            `json:"ignoreArrayOrder"`
	IgnoreExtraElements bool            `json:"ignoreExtraElements"`
	EqualToXml          *string         `json:"equalToXml"`
	EnablePlaceholders  bool            `json:"enablePlaceholders"`
	BinaryEqualTo       *string         `json:"binaryEqualTo"`
	MatchesJsonPath     json.RawMessage `json:"matchesJsonPath"`
	Absent              bool            `json:"absent"`
	Anything            *string         `json:"anything"`
	And                 []*matcher      `json:"and"`
	Or                  []*matcher      `json:"or"`
	Not                 *matcher        `json:"not"`
	HasExactly          []*matcher      `json:"hasExactly"`
	Includes            []*matcher      `json:"includes"`

	MatchesXPath    json.RawMessage `json:"matchesXPath"`
	EqualToDateTime json.RawMessage `json:"equalToDateTime"`
	Before          json.RawMessage `json:"before"`
	After           json.RawMessage `json:"after"`

	pattern    *regexp.Regexp
	json       any        // equalToJson, decoded
	xml        []xmlToken // equalToXml, as compared
	binary     []byte
	path       []jsonStep
	sub        *matcher // of matchesJsonPath's value
	expression string   // of matchesJsonPath
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

func set(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s != "" && s != "null" && s != "[]" && s != "{}"
}

// notDone says which parts of WireMock's a mapping has that rayo can't do.
func notDone(parts map[string]json.RawMessage) error {
	for _, name := range slices.Sorted(func(yield func(string) bool) {
		for k := range parts {
			if !yield(k) {
				return
			}
		}
	}) {
		if set(parts[name]) {
			return fmt.Errorf("WireMock's %s isn't supported", name)
		}
	}
	return nil
}

// ParseRule reads one mapping.
func ParseRule(data []byte) (*Rule, error) {
	r := &Rule{}
	if err := strict(data, r); err != nil {
		return nil, err
	}
	q, a := &r.Request, &r.Response
	if err := notDone(map[string]json.RawMessage{
		"postServeActions": r.PostServeActions, "serveEventListeners": r.ServeEventListeners,
		"multipartPatterns": q.MultipartPatterns, "customMatcher": q.CustomMatcher, "scheme": q.Scheme, "port": q.Port, "clientIp": q.ClientIp,
		"proxyBaseUrl": a.ProxyBaseUrl, "additionalProxyRequestHeaders": a.AdditionalProxyHeaders,
		"removeProxyRequestHeaders": a.RemoveProxyHeaders, "proxyUrlPrefixToRemove": a.ProxyUrlPrefixToRemove,
	}); err != nil {
		return nil, err
	}
	if q.Method == "" {
		q.Method = "ANY"
	}
	addresses := 0
	for _, a := range []string{q.Url, q.UrlPattern, q.UrlPath, q.UrlPathPattern, q.UrlPathTemplate} {
		if a != "" {
			addresses++
		}
	}
	if addresses != 1 {
		return nil, errors.New("a request needs one of url, urlPattern, urlPath, urlPathPattern, urlPathTemplate")
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
	if q.UrlPathTemplate != "" {
		if q.pathPattern, q.pathNames, err = pathTemplate(q.UrlPathTemplate); err != nil {
			return nil, err
		}
	}
	for name := range q.PathParameters {
		if !slices.Contains(q.pathNames, name) {
			return nil, fmt.Errorf("pathParameters: %s isn't in the urlPathTemplate", name)
		}
	}
	all := append([]*matcher{q.Host}, q.BodyPatterns...)
	for _, named := range []map[string]*matcher{q.PathParameters, q.QueryParameters, q.FormParameters, q.Headers, q.Cookies} {
		for _, m := range named {
			all = append(all, m)
		}
	}
	for i, m := range all {
		if i == 0 && m == nil { // no host
			continue
		}
		if err := m.prepare(); err != nil {
			return nil, err
		}
	}
	if (r.RequiredScenarioState != "" || r.NewScenarioState != "") && r.ScenarioName == "" {
		return nil, errors.New("a state needs a scenarioName")
	}
	if a.Status == 0 {
		a.Status = 200
	}
	if a.Status < 200 || a.Status > 599 {
		return nil, fmt.Errorf("not a status: %d", a.Status)
	}
	bodies := 0
	for _, set := range []bool{a.Body != "", a.JsonBody != nil, a.BodyFileName != "", a.Base64Body != ""} {
		if set {
			bodies++
		}
	}
	if bodies > 1 {
		return nil, errors.New("a response takes one of body, jsonBody, base64Body, bodyFileName")
	}
	if a.Fault != "" && !slices.Contains(faults, a.Fault) {
		return nil, fmt.Errorf("fault takes one of %s, not %q", strings.Join(faults, ", "), a.Fault)
	}
	for _, t := range a.Transformers {
		if t != "response-template" {
			return nil, fmt.Errorf("WireMock's transformer %q isn't supported, only response-template", t)
		}
	}
	if d := a.DelayDistribution; d != nil && d.Type != "lognormal" && d.Type != "uniform" && d.Type != "fixed" {
		return nil, fmt.Errorf("delayDistribution takes the type lognormal, uniform or fixed, not %q", d.Type)
	}
	if d := a.ChunkedDribbleDelay; d != nil && (d.NumberOfChunks < 1 || d.TotalDuration < 0) {
		return nil, errors.New("chunkedDribbleDelay needs numberOfChunks of 1 or more, and a totalDuration")
	}
	if a.JsonBody != nil {
		a.Body = string(a.JsonBody)
	}
	switch {
	case a.Base64Body != "":
		data, err := base64.StdEncoding.DecodeString(a.Base64Body)
		if err != nil {
			return nil, fmt.Errorf("base64Body: %w", err)
		}
		r.body = literal(string(data))
	case a.BodyFileName == "":
		r.body, err = parseTemplate(a.Body, a.templating())
	}
	return r, err
}

var templateName = regexp.MustCompile(`\{([^{}/]+)\}`)

// pathTemplate is a pattern of a urlPathTemplate such as /users/{id}, and the names in it.
func pathTemplate(t string) (*regexp.Regexp, []string, error) {
	var names []string
	var b strings.Builder
	last := 0
	for _, at := range templateName.FindAllStringSubmatchIndex(t, -1) {
		b.WriteString(regexp.QuoteMeta(t[last:at[0]]))
		names = append(names, t[at[2]:at[3]])
		b.WriteString(`([^/]+)`)
		last = at[1]
	}
	b.WriteString(regexp.QuoteMeta(t[last:]))
	p, err := Pattern(b.String())
	return p, names, err
}

func (m *matcher) prepare() (err error) {
	if m == nil {
		return errors.New("an empty matcher")
	}
	if err := notDone(map[string]json.RawMessage{"matchesXPath": m.MatchesXPath, "equalToDateTime": m.EqualToDateTime, "before": m.Before, "after": m.After}); err != nil {
		return err
	}
	n := 0
	for _, set := range []bool{m.EqualTo != nil, m.Contains != nil, m.DoesNotContain != nil, m.Matches != nil, m.DoesNotMatch != nil,
		m.EqualToJson != nil, m.EqualToXml != nil, m.BinaryEqualTo != nil, m.MatchesJsonPath != nil, m.Absent, m.Anything != nil,
		m.And != nil, m.Or != nil, m.Not != nil, m.HasExactly != nil, m.Includes != nil} {
		if set {
			n++
		}
	}
	if n != 1 {
		return errors.New("a matcher takes one of equalTo, contains, doesNotContain, matches, doesNotMatch, equalToJson, equalToXml, " +
			"binaryEqualTo, matchesJsonPath, absent, anything, and, or, not, hasExactly, includes")
	}
	if m.CaseInsensitive && m.EqualTo == nil {
		return errors.New("caseInsensitive goes with equalTo")
	}
	if (m.IgnoreArrayOrder || m.IgnoreExtraElements) && m.EqualToJson == nil {
		return errors.New("ignoreArrayOrder and ignoreExtraElements go with equalToJson")
	}
	if m.EnablePlaceholders {
		return errors.New("enablePlaceholders of equalToXml isn't supported")
	}
	switch {
	case m.Matches != nil:
		m.pattern, err = Pattern(*m.Matches)
	case m.DoesNotMatch != nil:
		m.pattern, err = Pattern(*m.DoesNotMatch)
	case m.EqualToJson != nil:
		m.json, err = expectedJSON(m.EqualToJson)
	case m.EqualToXml != nil:
		m.xml, err = xmlTokens(*m.EqualToXml)
		if err != nil {
			err = fmt.Errorf("equalToXml: %w", err)
		}
	case m.BinaryEqualTo != nil:
		m.binary, err = base64.StdEncoding.DecodeString(*m.BinaryEqualTo)
		if err != nil {
			err = fmt.Errorf("binaryEqualTo: %w", err)
		}
	case m.MatchesJsonPath != nil:
		err = m.prepareJSONPath()
	}
	if err != nil {
		return err
	}
	subs := slices.Concat(m.And, m.Or, m.HasExactly, m.Includes)
	if m.Not != nil {
		subs = append(subs, m.Not)
	}
	for _, sub := range subs {
		if err := sub.prepare(); err != nil {
			return err
		}
	}
	return nil
}

// expectedJSON reads equalToJson: a JSON value, or a text holding one.
func expectedJSON(raw json.RawMessage) (any, error) {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		raw = json.RawMessage(text)
	}
	v, err := decodeJSON([]byte(raw))
	if err != nil {
		return nil, fmt.Errorf("equalToJson: %w", err)
	}
	return v, nil
}

func decodeJSON(data []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	if d.More() {
		return nil, errors.New("more than one JSON value")
	}
	return v, nil
}

// prepareJSONPath reads matchesJsonPath: an expression, or one with a matcher for what it finds.
func (m *matcher) prepareJSONPath() (err error) {
	var expression string
	if json.Unmarshal(m.MatchesJsonPath, &expression) != nil {
		var parts map[string]json.RawMessage
		if err := json.Unmarshal(m.MatchesJsonPath, &parts); err != nil {
			return errors.New("matchesJsonPath takes an expression, or one with a matcher")
		}
		if err := json.Unmarshal(parts["expression"], &expression); err != nil {
			return errors.New("matchesJsonPath needs an expression")
		}
		delete(parts, "expression")
		if len(parts) > 0 {
			rest, _ := json.Marshal(parts)
			m.sub = &matcher{}
			if err := strict(rest, m.sub); err != nil {
				return fmt.Errorf("matchesJsonPath: %w", err)
			}
			if err := m.sub.prepare(); err != nil {
				return fmt.Errorf("matchesJsonPath: %w", err)
			}
		}
	}
	m.expression = expression
	m.path, err = parseJSONPath(expression)
	return err
}

// miss says why a value doesn't fit, or "" if it does. A value that isn't there is told by there.
func (m *matcher) miss(vm *goja.Runtime, value string, there bool) string {
	fits := func(ok bool, wanted string) string {
		if ok {
			return ""
		}
		if !there {
			return "absent, expected " + wanted
		}
		return fmt.Sprintf("%s, expected %s", short(value), wanted)
	}
	switch {
	case m.Absent:
		return fits(!there, "absent")
	case m.Not != nil:
		return fits(m.Not.miss(vm, value, there) != "", "not "+m.Not.what())
	case m.And != nil:
		for _, sub := range m.And {
			if why := sub.miss(vm, value, there); why != "" {
				return why
			}
		}
		return ""
	case m.Or != nil:
		why := ""
		for _, sub := range m.Or {
			if why = sub.miss(vm, value, there); why == "" {
				return ""
			}
		}
		return why
	case m.HasExactly != nil || m.Includes != nil:
		var values []string
		if there {
			values = []string{value}
		}
		return m.missValues(vm, values)
	case !there: // as WireMock: only absent fits what isn't there
		return "absent, expected " + m.what()
	case m.Anything != nil:
		return ""
	case m.EqualTo != nil:
		return fits(value == *m.EqualTo || m.CaseInsensitive && strings.EqualFold(value, *m.EqualTo), m.what())
	case m.Contains != nil:
		return fits(strings.Contains(value, *m.Contains), m.what())
	case m.DoesNotContain != nil:
		return fits(!strings.Contains(value, *m.DoesNotContain), m.what())
	case m.Matches != nil:
		return fits(m.pattern.MatchString(value), m.what())
	case m.DoesNotMatch != nil:
		return fits(!m.pattern.MatchString(value), m.what())
	case m.BinaryEqualTo != nil:
		return fits(value == string(m.binary), "the binaryEqualTo")
	case m.EqualToXml != nil:
		got, err := xmlTokens(value)
		if err != nil {
			return "not XML: " + err.Error()
		}
		return xmlDiffers(m.xml, got)
	case m.EqualToJson != nil:
		got, err := decodeJSON([]byte(value))
		if err != nil {
			return "not JSON"
		}
		return jsonDiffers("$", m.json, got, m.IgnoreArrayOrder, m.IgnoreExtraElements)
	}
	document, err := decodeJSON([]byte(value))
	if err != nil {
		return "not JSON"
	}
	found := jsonPathEval(vm, m.path, document)
	if m.sub == nil {
		if jsonPathTrue(found) {
			return ""
		}
		return "nothing at " + m.expression
	}
	text, ok := found.text()
	if why := m.sub.miss(vm, text, ok); why != "" {
		return m.expression + ": " + why
	}
	return ""
}

// what a matcher wants, as said in a near miss.
func (m *matcher) what() string {
	q := func(s *string) string { return short(*s) }
	switch {
	case m.EqualTo != nil:
		return q(m.EqualTo)
	case m.Contains != nil:
		return "one that contains " + q(m.Contains)
	case m.DoesNotContain != nil:
		return "one without " + q(m.DoesNotContain)
	case m.Matches != nil:
		return "one that matches " + q(m.Matches)
	case m.DoesNotMatch != nil:
		return "one that doesn't match " + q(m.DoesNotMatch)
	case m.Absent:
		return "absent"
	case m.EqualToJson != nil:
		return "the equalToJson"
	case m.EqualToXml != nil:
		return "the equalToXml"
	case m.MatchesJsonPath != nil:
		return "matchesJsonPath " + m.expression
	}
	return "a value"
}

// missValues says why a list of values, such as a header's, doesn't fit. A plain matcher fits if any does.
func (m *matcher) missValues(vm *goja.Runtime, values []string) string {
	switch {
	case m.HasExactly != nil || m.Includes != nil:
		want := m.HasExactly
		if want == nil {
			want = m.Includes
		}
		if m.HasExactly != nil && len(values) != len(want) {
			return fmt.Sprintf("%d values, expected %d", len(values), len(want))
		}
		fits := func(i, j int) bool { return want[i].miss(vm, values[j], true) == "" }
		if pairs(len(want), len(values), fits) < len(want) {
			return fmt.Sprintf("%s fit no %s", short(strings.Join(values, ", ")), "list wanted")
		}
		return ""
	case len(values) == 0:
		return m.miss(vm, "", false)
	}
	why := ""
	for _, v := range values {
		if why = m.miss(vm, v, true); why == "" {
			return ""
		}
	}
	return why
}

// pairs is how many of a can be paired each with a different b it fits: a bipartite matching.
func pairs(a, b int, fits func(i, j int) bool) int {
	taken := make([]int, b) // the a of each b, +1
	var try func(i int, seen []bool) bool
	try = func(i int, seen []bool) bool {
		for j := range b {
			if seen[j] || !fits(i, j) {
				continue
			}
			seen[j] = true
			if taken[j] == 0 || try(taken[j]-1, seen) {
				taken[j] = i + 1
				return true
			}
		}
		return false
	}
	n := 0
	for i := range a {
		if try(i, make([]bool, b)) {
			n++
		}
	}
	return n
}

// miss says why a call doesn't fit the rule, or "" if it does.
func (r *Rule) miss(vm *goja.Runtime, q *http.Request, body string) string {
	if why := r.missAddress(q); why != "" {
		return why
	}
	return r.missRest(vm, q, body)
}

// missAddress is about the method and the address alone.
func (r *Rule) missAddress(q *http.Request) string {
	w := &r.Request
	if w.Method != "ANY" && !strings.EqualFold(w.Method, q.Method) {
		return "method " + q.Method + ", expected " + w.Method
	}
	uri, path := q.URL.RequestURI(), q.URL.EscapedPath()
	switch {
	case w.Url != "" && w.Url != uri,
		w.urlPattern != nil && !w.urlPattern.MatchString(uri),
		w.UrlPath != "" && w.UrlPath != path,
		w.pathPattern != nil && !w.pathPattern.MatchString(path):
		return "the address"
	}
	return ""
}

func (r *Rule) missRest(vm *goja.Runtime, q *http.Request, body string) string {
	w := &r.Request
	if w.Host != nil {
		if why := w.Host.miss(vm, hostOf(q), true); why != "" {
			return "host: " + why
		}
	}
	if len(w.PathParameters) > 0 {
		m := w.pathPattern.FindStringSubmatch(q.URL.EscapedPath())
		for i, name := range w.pathNames {
			if p := w.PathParameters[name]; p != nil {
				value, _ := url.PathUnescape(m[i+1])
				if why := p.miss(vm, value, true); why != "" {
					return "path parameter " + name + ": " + why
				}
			}
		}
	}
	query := q.URL.Query()
	for _, name := range sortedKeys(w.QueryParameters) {
		if why := w.QueryParameters[name].missValues(vm, query[name]); why != "" {
			return "query " + name + ": " + why
		}
	}
	for _, name := range sortedKeys(w.Headers) {
		if why := w.Headers[name].missValues(vm, q.Header.Values(name)); why != "" {
			return "header " + name + ": " + why
		}
	}
	if len(w.Cookies) > 0 {
		cookies := map[string][]string{}
		for _, c := range q.Cookies() {
			cookies[c.Name] = append(cookies[c.Name], c.Value)
		}
		for _, name := range sortedKeys(w.Cookies) {
			if why := w.Cookies[name].missValues(vm, cookies[name]); why != "" {
				return "cookie " + name + ": " + why
			}
		}
	}
	if c := w.BasicAuthCredentials; c != nil {
		if user, password, ok := q.BasicAuth(); !ok || user != c.Username || password != c.Password {
			return "basicAuthCredentials"
		}
	}
	if len(w.FormParameters) > 0 {
		form, _ := url.ParseQuery(body)
		for _, name := range sortedKeys(w.FormParameters) {
			if why := w.FormParameters[name].missValues(vm, form[name]); why != "" {
				return "form " + name + ": " + why
			}
		}
	}
	for _, m := range w.BodyPatterns { // a body is always there, if empty, except to absent
		if why := m.miss(vm, body, body != "" || !m.Absent); why != "" {
			return "body: " + why
		}
	}
	return ""
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// hostOf is the host a call was for: the one at the start of its path, else the one it was sent to.
func hostOf(q *http.Request) string {
	if host, _, _ := strings.Cut(strings.TrimPrefix(q.URL.Path, "/"), "/"); hostName.MatchString(host) {
		return strings.ToLower(host)
	}
	host := q.Host
	if h, _, ok := strings.Cut(host, ":"); ok {
		host = h
	}
	return strings.ToLower(host)
}

func (r *Rule) priority() int {
	if r.Priority != nil {
		return *r.Priority
	}
	return 5
}

// delay is the rule's, with its random part.
func (a *ruleAnswer) delay(random func() float64, normal func() float64) time.Duration {
	ms := float64(a.FixedDelayMilliseconds)
	if d := a.DelayDistribution; d != nil {
		switch d.Type {
		case "fixed":
			ms += d.Milliseconds
		case "uniform":
			ms += d.Lower + random()*(d.Upper-d.Lower)
		case "lognormal":
			ms += d.Median * math.Exp(d.Sigma*normal())
		}
	}
	return time.Duration(ms * float64(time.Millisecond))
}
