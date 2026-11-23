package mock

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// Capturing: the real backend answers, and what it says is kept to be mocked later.

// Settings are how a capture becomes mocks: what tells calls apart, and what doesn't. They are kept
// with the capture, so that one that goes on from it, or updates it, makes the same mocks.
type Settings struct {
	GraphQL      string   `json:"graphql,omitempty"`      // GraphQLOff, GraphQLOperation or GraphQLVariables
	GraphQLPath  string   `json:"graphqlPath,omitempty"`  // a pattern of paths of GraphQL calls
	REST         string   `json:"rest,omitempty"`         // RESTBody or RESTOff
	IgnoreParams []string `json:"ignoreParams,omitempty"` // names of the query not to match
	IgnoreBody   []string `json:"ignoreBody,omitempty"`   // names, or dotted paths, of the body not to match
	Timestamps   string   `json:"timestamps,omitempty"`   // TimestampsFormat or TimestampsExact
	Headers      []string `json:"headers,omitempty"`      // of the request, to match
	IgnoreHost   bool     `json:"ignoreHost,omitempty"`   // any host, for paths that begin with one
	Stateless    []string `json:"stateless,omitempty"`    // globs of host/path whose calls only read, whatever the method
	RedactValues []string `json:"redactValues,omitempty"` // name=value: what a private field of a body becomes
}

// Values of Settings.
const (
	RESTBody         = "body"
	RESTOff          = "off"
	TimestampsFormat = "format"
	TimestampsExact  = "exact"
)

// Or fills what isn't set with what other has.
func (s Settings) Or(other Settings) Settings {
	pick := func(a, b string) string {
		if a == "" {
			return b
		}
		return a
	}
	list := func(a, b []string) []string {
		if len(a) == 0 {
			return b
		}
		return a
	}
	return Settings{
		GraphQL: pick(s.GraphQL, other.GraphQL), GraphQLPath: pick(s.GraphQLPath, other.GraphQLPath), REST: pick(s.REST, other.REST),
		IgnoreParams: list(s.IgnoreParams, other.IgnoreParams), IgnoreBody: list(s.IgnoreBody, other.IgnoreBody),
		Timestamps: pick(s.Timestamps, other.Timestamps), Headers: list(s.Headers, other.Headers), IgnoreHost: s.IgnoreHost || other.IgnoreHost,
		RedactValues: list(s.RedactValues, other.RedactValues), Stateless: list(s.Stateless, other.Stateless),
	}
}

// Defaults are what a capture does unless told otherwise.
var Defaults = Settings{GraphQL: GraphQLVariables, GraphQLPath: DefaultGraphQLPath, REST: RESTBody, Timestamps: TimestampsFormat}

// Check says what is wrong with them.
func (s Settings) Check() error {
	switch {
	case s.GraphQL != "" && s.GraphQL != GraphQLOff && s.GraphQL != GraphQLOperation && s.GraphQL != GraphQLVariables:
		return fmt.Errorf("graphql takes operation, operation+variables or off, not %q", s.GraphQL)
	case s.REST != "" && s.REST != RESTBody && s.REST != RESTOff:
		return fmt.Errorf("rest takes body or off, not %q", s.REST)
	case s.Timestamps != "" && s.Timestamps != TimestampsFormat && s.Timestamps != TimestampsExact:
		return fmt.Errorf("timestamps takes format or exact, not %q", s.Timestamps)
	}
	if s.GraphQLPath != "" {
		if _, err := Pattern(s.GraphQLPath); err != nil {
			return fmt.Errorf("graphql-path: %w", err)
		}
	}
	return nil
}

// real is the backend calls are passed to.
type real struct {
	base     string // empty when the path begins with the host
	client   *http.Client
	secrets  []string
	settings Settings // as given; the saved ones fill the rest
	step     string
	captured []captured
	skipped  []string
	earlier  []string // files in __files of the capture gone on from
}

// captured is a call and its answer, with nothing private in them.
type captured struct {
	method, uri string
	status      int
	contentType string
	headers     map[string][]string // of the answer, but Content-Type
	body        string
	binary      bool       // body is in base64
	step        string     // when it was made
	request     *request   // nil in a capture made before requests were kept
	op          *operation // of a GraphQL call, worked out from request; or kept, in a capture made before

	spec      *spec // how it is matched: worked out when the mocks are written
	stateless bool  // it only reads, as Settings.Stateless says
}

// request is what the app sent, as kept: only what isn't private.
type request struct {
	ContentType string              `json:"contentType,omitempty"`
	Body        string              `json:"body,omitempty"`
	Base64      bool                `json:"base64,omitempty"`    // a body that isn't text
	Unmatched   string              `json:"unmatched,omitempty"` // why the body isn't kept, if it isn't
	Headers     map[string][]string `json:"headers,omitempty"`   // those of Settings.Headers
}

// kept is how a captured call is written, to go on with the capture later.
type kept struct {
	Method      string              `json:"method"`
	URI         string              `json:"uri"`
	Request     *request            `json:"request,omitempty"`
	Status      int                 `json:"status"`
	ContentType string              `json:"contentType,omitempty"`
	Headers     map[string][]string `json:"headers,omitempty"`
	Body        string              `json:"body"`
	Base64      bool                `json:"base64,omitempty"`
	Step        string              `json:"step,omitempty"`
	GraphQL     *operation          `json:"graphql,omitempty"`
}

func (c captured) toKept() kept {
	r := c.request
	if r != nil && r.Headers == nil && r.Body == "" && r.ContentType == "" && r.Unmatched == "" {
		r = nil // nothing sent
	}
	return kept{c.method, c.uri, r, c.status, c.contentType, c.headers, c.body, c.binary, c.step, c.op}
}

// toCaptured reads it as written by a version of kept: before 2, without what the app sent, the
// operation is all there is.
func (k kept) toCaptured(version int) captured {
	c := captured{method: k.Method, uri: k.URI, request: k.Request, status: k.Status, contentType: k.ContentType, headers: k.Headers, body: k.Body, binary: k.Base64, step: k.Step}
	switch {
	case version < 2:
		c.op = k.GraphQL
	case c.request == nil:
		c.request = &request{}
	}
	return c
}

// Captured is how many calls have been captured: a mark for Drop.
func (m *Mock) Captured() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.real.captured)
}

// Drop forgets the calls captured since a mark, and returns how many there were.
func (m *Mock) Drop(mark int) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.real.captured) - mark
	if n <= 0 {
		return 0
	}
	m.real.captured = m.real.captured[:mark]
	return n
}

// read is a call that changes nothing: a GET, or a GraphQL query.
func (c captured) read() bool {
	if c.stateless {
		return true
	}
	if c.op != nil {
		return !c.op.Mutation
	}
	return c.method == http.MethodGet || c.method == http.MethodHead || c.method == http.MethodOptions
}

// endpoint tells calls apart: those that are the same call have the same one. Those that change
// something are told apart by their order too.
func (c captured) endpoint() string {
	if c.spec != nil {
		return c.spec.key
	}
	return c.method + " " + c.uri
}

func (c captured) answer() string {
	return fmt.Sprintf("%d\n%s\n%t\n%s", c.status, c.contentType, c.binary, c.body)
}

// Glob is a pattern of a path, such as api.shop.com/quotes/** : * is any text but /, ** any text.
func Glob(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^/?")
	for i, part := range strings.Split(strings.TrimPrefix(pattern, "/"), "**") {
		if i > 0 {
			b.WriteString(".*")
		}
		for j, p := range strings.Split(part, "*") {
			if j > 0 {
				b.WriteString("[^/]*")
			}
			b.WriteString(regexp.QuoteMeta(p))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

// stateless says whether a call's path is one of the globs.
func stateless(globs []string, uri string) bool {
	path, _, _ := strings.Cut(uri, "?")
	for _, g := range globs {
		if re, err := Glob(g); err == nil && re.MatchString(path) {
			return true
		}
	}
	return false
}

// Forward passes every call to the real backend. Without a base, /host/path goes to https://host/path.
func (m *Mock) Forward(base string, secrets map[string]string, settings Settings) {
	m.real = &real{base: strings.TrimSuffix(base, "/"), client: &http.Client{Timeout: time.Minute}, settings: settings}
	for _, v := range secrets {
		if len(v) >= 4 { // anything shorter would be found everywhere
			m.real.secrets = append(m.real.secrets, v)
		}
	}
}

// UseSettingsOf takes the settings of the capture in a folder, for what wasn't given: so that an update
// of mocks is made as they were.
func (m *Mock) UseSettingsOf(folder string) error {
	k, err := readKept(folder)
	if err != nil || k == nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.real.settings = m.real.settings.Or(k.Settings)
	return nil
}

// During says which step the calls from now on belong to.
func (m *Mock) During(step string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.real.step = m.real.cleanText(step) // it is kept with the calls
}

var hostName = regexp.MustCompile(`^[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}$`)

func (r *real) address(q *http.Request) (string, error) {
	uri := q.URL.RequestURI()
	if r.base != "" {
		return r.base + uri, nil
	}
	host, rest, _ := strings.Cut(strings.TrimPrefix(uri, "/"), "/")
	if !hostName.MatchString(host) {
		return "", errors.New("the path doesn't begin with a host")
	}
	return "https://" + host + "/" + rest, nil
}

const maxAnswer = 50 << 20

// fetch asks for a whole answer: not compressed, and not "as you already have it".
func (r *real) fetch(q *http.Request, body []byte) (*http.Response, []byte, error) {
	address, err := r.address(q)
	if err != nil {
		return nil, nil, err
	}
	out, err := http.NewRequestWithContext(q.Context(), q.Method, address, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	out.Header = q.Header.Clone()
	for _, name := range append([]string{"Accept-Encoding", "If-None-Match", "If-Modified-Since"}, hopByHop...) {
		out.Header.Del(name)
	}
	res, err := r.client.Do(out)
	if err != nil {
		var u *url.Error
		if errors.As(err, &u) {
			err = u.Err // without the address, which may hold secrets
		}
		return nil, nil, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, maxAnswer+1))
	if err == nil && len(data) > maxAnswer {
		err = errors.New("the answer is too long")
	}
	return res, data, err
}

// hopByHop are headers of one connection, not of the call.
var hopByHop = []string{"Connection", "Keep-Alive", "Proxy-Connection", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade"}

// volatile are headers of an answer that change by themselves, or say how it was sent: a mock leaves
// them out. Those that end in - are prefixes.
var volatile = []string{"Date", "Age", "Expires", "Etag", "Last-Modified", "Server", "Via", "Alt-Svc", "Strict-Transport-Security",
	"Report-To", "Nel", "Server-Timing", "Content-Length", "Content-Encoding", "Content-Type", "X-Request-Id", "X-Correlation-Id",
	"Request-Id", "Traceparent", "Tracestate", "X-Trace-Id", "X-Runtime", "X-Response-Time", "X-Served-By", "X-Timer", "X-Cache",
	"X-Cache-Hits", "Request-Context", "X-Amz-", "X-Amzn-", "X-B3-", "X-Datadog-", "X-Envoy-", "Cf-", "X-Azure-", "X-Ms-", "X-Github-Request-Id"}

func isVolatile(name string) bool {
	name = http.CanonicalHeaderKey(name)
	for _, v := range append(volatile, hopByHop...) {
		if name == v || strings.HasSuffix(v, "-") && strings.HasPrefix(name, v) {
			return true
		}
	}
	return false
}

// noFollow keeps a redirect for the app to follow: behind the proxy, it calls through it again.
func noFollow(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

func (m *Mock) pass(w http.ResponseWriter, q *http.Request, body []byte) {
	r := m.real
	uri := r.cleanURI(q.URL)
	m.mu.Lock()
	step := r.step
	call := &Call{At: time.Now(), Method: q.Method, Path: q.URL.EscapedPath(), URI: uri, Body: decoded(q.Header, body), Outcome: "no answer yet"}
	m.calls = append(m.calls, call)
	sent := r.cleanRequest(q, []byte(call.Body))
	m.mu.Unlock()

	res, data, err := r.fetch(q, body)
	m.mu.Lock()
	if err != nil {
		call.Outcome = "failed: " + r.cleanText(err.Error())
		r.skipped = append(r.skipped, q.Method+" "+uri+": "+call.Outcome)
		m.mu.Unlock()
		panic(http.ErrAbortHandler) // closes the connection without an answer
	}
	call.Outcome = fmt.Sprint(res.StatusCode)
	c := captured{method: q.Method, uri: uri, status: res.StatusCode, contentType: res.Header.Get("Content-Type"), headers: r.cleanHeaders(res.Header), step: step, request: sent}
	if binary(c.contentType, data) {
		c.body, c.binary = base64.StdEncoding.EncodeToString(data), true
	} else {
		c.body, err = r.cleanBody(data)
	}
	if err != nil {
		r.skipped = append(r.skipped, q.Method+" "+uri+": "+err.Error())
	} else {
		r.captured = append(r.captured, c)
	}
	m.mu.Unlock()

	w.Header()["Content-Type"] = nil // or Go guesses one
	for name, values := range res.Header {
		if !slices.Contains(hopByHop, name) && name != "Content-Length" {
			w.Header()[name] = values
		}
	}
	w.WriteHeader(res.StatusCode)
	_, _ = w.Write(data) // the app may have hung up
}

// binary says whether a body isn't text, so that it is kept as it is.
func binary(contentType string, data []byte) bool {
	kind, _, _ := mime.ParseMediaType(contentType)
	switch {
	case !utf8.Valid(data):
		return true
	case strings.HasPrefix(kind, "image/"), strings.HasPrefix(kind, "audio/"), strings.HasPrefix(kind, "video/"), strings.HasPrefix(kind, "font/"):
		return true
	}
	return slices.Contains([]string{"application/octet-stream", "application/pdf", "application/zip", "application/protobuf",
		"application/x-protobuf", "application/vnd.google.protobuf", "application/grpc", "application/grpc-web", "application/grpc-web+proto"}, kind)
}

// --- Leaving out what is private ---

const (
	redacted = "REDACTED"
	nobody   = "someone@example.com"
)

var (
	privateNames = regexp.MustCompile(`^(access|refresh|id|auth|session|device|push|csrf|xsrf|api)?token$|^jwt$|password|secret|^pass$|` +
		`^verificationpass$|^authorization$|^(session|cookie)(id)?$|^apikey$|e?mail|phone|^mobile(number)?$|` +
		`^(first|last|full|given|family|middle)name$|^surnames?$|^user(name)?$|^(nif|nie|dni|passport|iban|pan|ssn|cvv|cvc)$|` +
		`^documentnumber$|^(card|account)number$|^(home|billing|postal|shipping)address$|^zip(code)?$|^postalcode$|birth`)
	emails = regexp.MustCompile(`[\w.+%-]+@[\w-]+(\.[\w-]+)+`)
	jwts   = regexp.MustCompile(`eyJ[\w-]+\.[\w-]+\.[\w-]*`)
)

func private(name string) bool {
	return privateNames.MatchString(strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(name)))
}

// privateHeader is a header that holds who the app is: always hidden.
func privateHeader(name string) bool {
	name = strings.ToLower(name)
	return name == "cookie" || name == "set-cookie" || name == "authorization" || name == "proxy-authorization" ||
		private(strings.TrimPrefix(name, "x-"))
}

func (r *real) cleanText(text string) string {
	for _, s := range r.secrets {
		for _, form := range []string{s, url.QueryEscape(s), url.PathEscape(s)} { // as in an address too
			text = strings.ReplaceAll(text, form, redacted)
		}
	}
	return jwts.ReplaceAllString(emails.ReplaceAllString(text, nobody), redacted)
}

// hides says whether something private was taken out of a text.
func hides(text string) bool {
	return strings.Contains(text, redacted) || strings.Contains(text, nobody)
}

// cleanURI keeps the order and the encoding, so that later calls can be compared with it.
func (r *real) cleanURI(u *url.URL) string {
	hide := func(text string, all bool) string {
		plain, err := url.QueryUnescape(text)
		if err != nil {
			plain = text
		}
		if text != "" && (all || r.cleanText(plain) != plain) {
			return redacted
		}
		return text
	}
	parts := strings.Split(u.EscapedPath(), "/")
	for i := range parts {
		parts[i] = hide(parts[i], false)
	}
	uri := strings.Join(parts, "/")
	if u.RawQuery == "" {
		return uri
	}
	return uri + "?" + r.cleanPairs(u.RawQuery)
}

// cleanPairs hides the values of a query, or of a form, that are private.
func (r *real) cleanPairs(query string) string {
	pairs := strings.Split(query, "&")
	for i, p := range pairs {
		name, value, ok := strings.Cut(p, "=")
		if !ok {
			continue
		}
		plain, err := url.QueryUnescape(value)
		if err != nil {
			plain = value
		}
		plainName, _ := url.QueryUnescape(name)
		if value != "" && (private(plainName) || r.cleanText(plain) != plain) {
			value = redacted
		}
		pairs[i] = name + "=" + value
	}
	return strings.Join(pairs, "&")
}

// cleanHeaders are those of an answer worth mocking, with what is private hidden: every cookie's value too,
// as most are who the app is.
func (r *real) cleanHeaders(h http.Header) map[string][]string {
	out := map[string][]string{}
	for name, values := range h {
		if isVolatile(name) {
			continue
		}
		for _, v := range values {
			switch {
			case name == "Set-Cookie":
				cookie, attributes, _ := strings.Cut(v, ";")
				if n, _, ok := strings.Cut(cookie, "="); ok {
					v = strings.TrimSpace(n) + "=" + redacted
					if attributes != "" {
						v += ";" + attributes
					}
				}
			case privateHeader(name):
				v = redacted
			default:
				v = r.cleanText(v)
			}
			out[name] = append(out[name], v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// maxMatched is the longest body a mock matches: one longer, such as an upload, is matched by its address.
const maxMatched = 64 << 10

// cleanRequest keeps what the app sent, without what is private, to match it later.
func (r *real) cleanRequest(q *http.Request, body []byte) *request {
	k := &request{ContentType: q.Header.Get("Content-Type")}
	for _, name := range r.settings.Headers {
		for _, v := range q.Header.Values(name) {
			if privateHeader(name) {
				v = redacted
			} else {
				v = r.cleanText(v)
			}
			if k.Headers == nil {
				k.Headers = map[string][]string{}
			}
			k.Headers[http.CanonicalHeaderKey(name)] = append(k.Headers[http.CanonicalHeaderKey(name)], v)
		}
	}
	kind, _, _ := mime.ParseMediaType(k.ContentType)
	switch {
	case len(body) == 0:
	case len(body) > maxMatched:
		k.Unmatched = fmt.Sprintf("longer than %d KB", maxMatched>>10)
	case strings.HasPrefix(kind, "multipart/"):
		k.Unmatched = "multipart, its boundary changes"
	case !utf8.Valid(body):
		if r.cleanText(string(body)) != string(body) {
			k.Unmatched = "binary, with something private in it"
		} else {
			k.Body, k.Base64 = base64.StdEncoding.EncodeToString(body), true
		}
	case kind == "application/x-www-form-urlencoded":
		k.Body = r.cleanPairs(string(body))
	case json.Valid(body):
		v, err := decodeJSON(body)
		if err != nil {
			k.Unmatched = "JSON of more than one value"
			break
		}
		k.Body = compactJSON(r.cleanValue(v, false))
	default:
		k.Body = r.cleanText(string(body))
	}
	return k
}

func (r *real) cleanBody(data []byte) (string, error) {
	if json.Valid(data) {
		value, err := decodeJSON(data)
		if err != nil {
			return "", err
		}
		var b bytes.Buffer
		e := json.NewEncoder(&b)
		e.SetEscapeHTML(false)
		e.SetIndent("", "  ")
		if err := e.Encode(r.cleanValue(value, false)); err != nil {
			return "", err
		}
		// A mock would take it as a template. In JSON it can only be inside a text.
		return strings.ReplaceAll(b.String(), "{{", "{\\u007b"), nil
	}
	return r.cleanText(string(data)), nil
}

// cleanValue keeps the type of what it hides, so that the app can still read it.
func (r *real) cleanValue(v any, hide bool) any { return r.cleanNamed("", v, hide) }

func (r *real) cleanNamed(name string, v any, hide bool) any {
	switch v := v.(type) {
	case map[string]any:
		for k, child := range v {
			v[k] = r.cleanNamed(k, child, hide || private(k))
		}
	case []any:
		for i, child := range v {
			v[i] = r.cleanNamed(name, child, hide)
		}
	case json.Number:
		if hide {
			return json.Number("0")
		}
	case string:
		switch {
		case hide && v != "":
			return r.fake(name, v)
		}
		return r.cleanText(v)
	}
	return v
}

var (
	dates  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	phones = regexp.MustCompile(`^\+?[\d\s().-]{6,}$`)
	numeric = regexp.MustCompile(`^[\d\s-]+$`)
)

// fake is what a private text becomes: of the same form, so that the app reads it as it read the real one.
func (r *real) fake(name, v string) string {
	key := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(name))
	for _, kv := range r.settings.RedactValues {
		if n, value, ok := strings.Cut(kv, "="); ok && strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(n)) == key {
			return value
		}
	}
	switch {
	case emails.MatchString(v):
		return nobody
	case timestamp.MatchString(v) || dates.MatchString(v):
		// The same layout, with the time zone it had: 2000-01-01, 00:00:00.000, +00:00.
		out := []byte(v)
		copy(out, "2000-01-01")
		for i := 10; i < len(out); i++ {
			if out[i] >= '0' && out[i] <= '9' {
				out[i] = '0'
			}
		}
		return string(out)
	case strings.Contains(key, "phone") || strings.Contains(key, "mobile") || phones.MatchString(v) && !numeric.MatchString(v):
		if strings.HasPrefix(v, "+") {
			return "+12025550123" // a valid E.164 number no one has
		}
		return sameDigits(v, "2025550123")
	case numeric.MatchString(v): // such as a postal code or a card's number
		return sameDigits(v, "0")
	case strings.Contains(key, "first") || strings.Contains(key, "given") || key == "name":
		return "Alex"
	case strings.Contains(key, "last") || strings.Contains(key, "family") || strings.Contains(key, "surname"):
		return "Doe"
	case strings.Contains(key, "full") || strings.Contains(key, "middle"):
		return "Alex Doe"
	case strings.HasPrefix(key, "user"):
		return "alex"
	}
	return redacted
}

// sameDigits keeps where the digits and the rest are, with the digits of with.
func sameDigits(v, with string) string {
	out, n := []byte(v), 0
	for i, c := range out {
		if c >= '0' && c <= '9' {
			out[i] = with[n%len(with)]
			n++
		}
	}
	return string(out)
}
