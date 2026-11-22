package mock

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Capturing: the real backend answers, and what it says is kept to be mocked later.

// real is the backend calls are passed to.
type real struct {
	base     string // empty when the path begins with the host
	client   *http.Client
	secrets  []string
	step     string
	captured []captured
	skipped  []string
}

// captured is a call and its answer, with nothing private in them.
type captured struct {
	method, uri string
	status      int
	contentType string
	body        string
	step        string // when it was made
}

func (c captured) read() bool { return c.method == http.MethodGet || c.method == http.MethodHead }

// endpoint tells calls apart. Those that change something are told apart by their order too.
func (c captured) endpoint() string { return c.method + " " + c.uri }

func (c captured) answer() string { return fmt.Sprintf("%d\n%s\n%s", c.status, c.contentType, c.body) }

// Forward passes every call to the real backend. Without a base, /host/path goes to https://host/path.
func (m *Mock) Forward(base string, secrets map[string]string) {
	m.real = &real{base: strings.TrimSuffix(base, "/"), client: &http.Client{Timeout: time.Minute}}
	for _, v := range secrets {
		if len(v) >= 4 { // anything shorter would be found everywhere
			m.real.secrets = append(m.real.secrets, v)
		}
	}
}

// During says which step the calls from now on belong to.
func (m *Mock) During(step string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.real.step = step
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
	for _, name := range []string{"Accept-Encoding", "Connection", "If-None-Match", "If-Modified-Since"} {
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

func (m *Mock) pass(w http.ResponseWriter, q *http.Request, body []byte) {
	r := m.real
	uri := r.cleanURI(q.URL)
	m.mu.Lock()
	step := r.step
	call := &Call{At: time.Now(), Method: q.Method, Path: q.URL.EscapedPath(), URI: uri, Body: string(body), Outcome: "no answer yet"}
	m.calls = append(m.calls, call)
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
	c := captured{method: q.Method, uri: uri, status: res.StatusCode, contentType: res.Header.Get("Content-Type"), step: step}
	if c.body, err = r.cleanBody(data); err != nil {
		r.skipped = append(r.skipped, c.endpoint()+": "+err.Error())
	} else {
		r.captured = append(r.captured, c)
	}
	m.mu.Unlock()

	w.Header()["Content-Type"] = nil // or Go guesses one
	for _, name := range []string{"Content-Type", "Content-Language", "Cache-Control", "Etag", "Last-Modified", "Set-Cookie", "Www-Authenticate", "Retry-After"} {
		for _, v := range res.Header.Values(name) {
			w.Header().Add(name, v)
		}
	}
	w.WriteHeader(res.StatusCode)
	_, _ = w.Write(data) // the app may have hung up
}

// --- Leaving out what is private ---

const (
	redacted = "REDACTED"
	nobody   = "someone@example.com"
)

var (
	privateNames = regexp.MustCompile(`^(access|refresh|id|auth|session|device|push|csrf)?token$|^jwt$|password|secret|^pass$|` +
		`^verificationpass$|^authorization$|^(session|cookie)(id)?$|^apikey$|e?mail|phone|^mobile(number)?$|` +
		`^(first|last|full|given|family|middle)name$|^surnames?$|^user(name)?$|^(nif|nie|dni|passport|iban|pan)$|` +
		`^documentnumber$|^(card|account)number$|^(home|billing|postal|shipping)address$|^zip(code)?$|^postalcode$|birth`)
	emails = regexp.MustCompile(`[\w.+%-]+@[\w-]+(\.[\w-]+)+`)
	jwts   = regexp.MustCompile(`eyJ[\w-]+\.[\w-]+\.[\w-]*`)
)

func private(name string) bool {
	return privateNames.MatchString(strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(name)))
}

func (r *real) cleanText(text string) string {
	for _, s := range r.secrets {
		text = strings.ReplaceAll(text, s, redacted)
	}
	return jwts.ReplaceAllString(emails.ReplaceAllString(text, nobody), redacted)
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
	pairs := strings.Split(u.RawQuery, "&")
	for i, p := range pairs {
		if name, value, ok := strings.Cut(p, "="); ok {
			pairs[i] = name + "=" + hide(value, private(name))
		}
	}
	return uri + "?" + strings.Join(pairs, "&")
}

func (r *real) cleanBody(data []byte) (string, error) {
	var value any
	if json.Valid(data) {
		d := json.NewDecoder(bytes.NewReader(data))
		d.UseNumber() // as it was written
		if err := d.Decode(&value); err != nil {
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
	text := r.cleanText(string(data))
	if strings.Contains(text, "{{") {
		return "", errors.New("it has {{, which a mock would take as a template")
	}
	return text, nil
}

// cleanValue keeps the type of what it hides, so that the app can still read it.
func (r *real) cleanValue(v any, hide bool) any {
	switch v := v.(type) {
	case map[string]any:
		for k, child := range v {
			v[k] = r.cleanValue(child, hide || private(k))
		}
	case []any:
		for i, child := range v {
			v[i] = r.cleanValue(child, hide)
		}
	case json.Number:
		if hide {
			return json.Number("0")
		}
	case string:
		switch {
		case hide && emails.MatchString(v):
			return nobody
		case hide && v != "":
			return redacted
		}
		return r.cleanText(v)
	}
	return v
}
