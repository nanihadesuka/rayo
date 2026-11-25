package mock

import (
	"bytes"
	"cmp"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	mrand "math/rand/v2"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Start serves on a free port of this computer.
func (m *Mock) Start() (port int, err error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	m.server = &http.Server{Handler: m, ErrorLog: log.New(io.Discard, "", 0)}
	go func() {
		if err := m.server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
			m.fail(fmt.Errorf("the mock stopped: %w", err))
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// Close also cuts the calls being answered.
func (m *Mock) Close() error {
	if m.server == nil { // behind a proxy
		return nil
	}
	return m.server.Close()
}

// Err is what went wrong while serving.
func (m *Mock) Err() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.err
}

func (m *Mock) fail(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = errors.Join(m.err, err)
}

func (m *Mock) ServeHTTP(w http.ResponseWriter, q *http.Request) {
	data, err := io.ReadAll(io.LimitReader(q.Body, maxRequest))
	if err != nil {
		return // the app hung up
	}
	if m.real != nil {
		m.pass(w, q, data)
		return
	}
	a := m.answer(q, decoded(q.Header, data))
	if a.delay > 0 {
		select {
		case <-time.After(a.delay):
		case <-q.Context().Done():
			return
		}
	}
	if a.drop {
		panic(http.ErrAbortHandler) // closes the connection without an answer
	}
	if a.fault != "" {
		fault(w, a.fault)
		return
	}
	w.Header()["Content-Type"] = nil // or Go guesses one
	for k, v := range a.headers {
		if k = http.CanonicalHeaderKey(k); k != "Content-Length" && k != "Transfer-Encoding" { // Go's to say, of the body as filled in
			w.Header()[k] = v
		}
	}
	w.WriteHeader(a.status)
	if a.chunks <= 1 {
		_, _ = io.WriteString(w, a.body) // the app may have hung up
		return
	}
	flusher, _ := w.(http.Flusher)
	size := max((len(a.body)+a.chunks-1)/a.chunks, 1)
	for i := 0; i < len(a.body); i += size {
		if i > 0 {
			select {
			case <-time.After(a.every):
			case <-q.Context().Done():
				return
			}
		}
		if _, err := io.WriteString(w, a.body[i:min(i+size, len(a.body))]); err != nil {
			return
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
}

const maxRequest = 10 << 20

// decoded is a body as compared: not compressed, if the app compressed it.
func decoded(h http.Header, data []byte) string {
	var r io.Reader
	switch strings.ToLower(strings.TrimSpace(h.Get("Content-Encoding"))) {
	case "gzip", "x-gzip":
		z, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return string(data)
		}
		r = z
	case "deflate":
		r = flate.NewReader(bytes.NewReader(data))
		if z, err := zlib.NewReader(bytes.NewReader(data)); err == nil { // as most send it
			r = z
		}
	default:
		return string(data)
	}
	plain, err := io.ReadAll(io.LimitReader(r, maxRequest))
	if err != nil {
		return string(data)
	}
	return string(plain)
}

// fault breaks the connection as WireMock's faults do.
func fault(w http.ResponseWriter, kind string) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		panic(http.ErrAbortHandler)
	}
	conn, buf, err := hj.Hijack()
	if err != nil {
		return
	}
	defer conn.Close()
	switch kind {
	case "CONNECTION_RESET_BY_PEER":
		if tcp, ok := conn.(*net.TCPConn); ok {
			_ = tcp.SetLinger(0) // a reset, not a close
		}
	case "MALFORMED_RESPONSE_CHUNK":
		_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\nlskdu018973t09sylgasjkfg1][]'./.sdlv")
	case "RANDOM_DATA_THEN_CLOSE":
		junk := make([]byte, 64)
		_, _ = rand.Read(junk)
		_, _ = buf.Write(junk)
	}
	_ = buf.Flush()
}

type answer struct {
	status  int
	headers map[string][]string
	body    string
	delay   time.Duration
	drop    bool
	fault   string
	chunks  int           // the body is sent in so many pieces
	every   time.Duration // between them
}

func (m *Mock) answer(q *http.Request, body string) (a answer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	call := &Call{At: time.Now(), Method: q.Method, Path: q.URL.EscapedPath(), URI: q.URL.RequestURI(), Body: body}
	m.calls = append(m.calls, call)
	if n := m.network; n.Path == nil || n.Path.MatchString(call.Path) {
		a.delay = n.Delay
		if n.Offline {
			call.Outcome = "offline"
			a.drop = true
			return a
		}
		if n.Fail != 0 {
			call.Outcome = fmt.Sprintf("%d (network)", n.Fail)
			a.status = n.Fail
			return a
		}
	}
	asked := q
	best := m.pick(q, body)
	if best == nil {
		if bare := m.withoutHost(q); bare != nil {
			if best = m.pick(bare, body); best != nil {
				asked = bare
			}
		}
	}
	if best == nil && m.unmocked.ignores(q, body) {
		call.Outcome, call.Ignored = "not mocked, ignored", true
		a.status, a.body = cmp.Or(m.unmocked.Status, 404), cmp.Or(m.unmocked.Body, "not mocked: "+q.Method+" "+call.URI)
		return a
	}
	if best == nil {
		call.Outcome, call.Unmocked = "not mocked", true
		a.status, a.body = 404, "not mocked: "+q.Method+" "+call.URI
		if op := OperationOf(body); op != "" && q.Method == http.MethodPost {
			call.Outcome = "not mocked: GraphQL operation " + op
			a.body += ", GraphQL operation " + op
		}
		if near := m.nearest(q, body); near != "" {
			call.Outcome += "; nearest " + near
			a.body += "\nnearest " + near
		}
		return a
	}
	call.Mapping = best.title()
	if best.source != "" {
		call.Mapping += " (" + best.source + ")"
	}
	if best.ScenarioName != "" {
		before := m.state(best.ScenarioName)
		after := cmp.Or(best.NewScenarioState, before)
		call.Scenario = best.ScenarioName + ": " + before
		if after != before {
			call.Scenario += " -> " + after
		}
	}
	if best.NewScenarioState != "" {
		m.states[best.ScenarioName] = best.NewScenarioState
	}
	r := &best.Response
	call.Outcome = strconv.Itoa(r.Status)
	a.status, a.headers, a.body = r.Status, r.Headers, best.body.render(asked, body, r.TransformerParameters, m.vm)
	a.delay += r.delay(mrand.Float64, mrand.NormFloat64)
	a.fault = r.Fault
	if d := r.ChunkedDribbleDelay; d != nil {
		a.chunks, a.every = d.NumberOfChunks, time.Duration(d.TotalDuration)*time.Millisecond/time.Duration(d.NumberOfChunks)
	}
	return a
}

// withoutHost is a call through the proxy, /host/path, as /path: for mocks written without the host, as
// WireMock's are. Only for a host that no mock names, so that those that do are never passed over.
func (m *Mock) withoutHost(q *http.Request) *http.Request {
	host, _, ok := strings.Cut(strings.TrimPrefix(q.URL.Path, "/"), "/")
	if !ok || !hostName.MatchString(host) || m.hosts()[strings.ToLower(host)] {
		return nil
	}
	bare := q.Clone(q.Context())
	bare.URL.Path = strings.TrimPrefix(q.URL.Path, "/"+host)
	bare.URL.RawPath = ""
	if raw := q.URL.RawPath; raw != "" {
		bare.URL.RawPath = strings.TrimPrefix(raw, "/"+host)
	}
	return bare
}

// pick is the rule that answers a call: of those that fit, the lowest priority, then the last added.
func (m *Mock) pick(q *http.Request, body string) *Rule {
	var best *Rule
	for _, r := range m.rules {
		if best != nil && r.priority() > best.priority() {
			continue
		}
		if r.RequiredScenarioState != "" && m.state(r.ScenarioName) != r.RequiredScenarioState {
			continue
		}
		if r.miss(m.vm, q, body) == "" {
			best = r
		}
	}
	return best
}

// nearest says, of the mappings for the call's method and address, the one that would answer but for
// one thing, and what it is: as WireMock's near misses.
func (m *Mock) nearest(q *http.Request, body string) string {
	var best *Rule
	why := ""
	for _, r := range m.rules {
		if r.missAddress(q) != "" || best != nil && r.priority() > best.priority() {
			continue
		}
		reason := r.missRest(m.vm, q, body)
		if reason == "" && r.RequiredScenarioState != "" {
			reason = fmt.Sprintf("scenario %s is in %s, it needs %s", r.ScenarioName, m.state(r.ScenarioName), r.RequiredScenarioState)
		}
		if reason != "" {
			best, why = r, reason
		}
	}
	if best == nil {
		return ""
	}
	return best.title() + ": " + why
}

// title is how a mapping is called in what rayo says.
func (r *Rule) title() string {
	if r.Name != "" {
		return r.Name
	}
	w := &r.Request
	return w.Method + " " + w.Url + w.UrlPattern + w.UrlPath + w.UrlPathPattern + w.UrlPathTemplate
}

func (m *Mock) state(scenario string) string {
	if s, ok := m.states[scenario]; ok {
		return s
	}
	return "Started"
}

// States is every scenario the mappings have, and the state it is in.
func (m *Mock) States() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	states := map[string]string{}
	for _, r := range m.rules {
		if r.ScenarioName != "" {
			states[r.ScenarioName] = m.state(r.ScenarioName)
		}
	}
	return states
}

// SetState moves a scenario.
func (m *Mock) SetState(scenario, state string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	known := map[string]bool{}
	for _, r := range m.rules {
		if r.ScenarioName == scenario {
			known["Started"] = true
			known[r.RequiredScenarioState] = true
			known[r.NewScenarioState] = true
		}
	}
	delete(known, "")
	if len(known) == 0 {
		return fmt.Errorf("no mock has a scenario %q", scenario)
	}
	if !known[state] {
		var names []string
		for name := range known {
			names = append(names, name)
		}
		sort.Strings(names)
		return fmt.Errorf("%s has no state %q (it has %s)", scenario, state, strings.Join(names, ", "))
	}
	m.states[scenario] = state
	return nil
}

// SetNetwork replaces how the connection behaves.
func (m *Mock) SetNetwork(n Network) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.network = n
}

// Await waits for a call that no other Await has matched. Earlier calls of the flow count.
func (m *Mock) Await(ctx context.Context, method string, path *regexp.Regexp, body string, limit time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	matched := func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		for _, c := range m.calls {
			if !c.awaited && (method == "" || strings.EqualFold(method, c.Method)) &&
				path.MatchString(c.Path) && strings.Contains(c.Body, body) {
				c.awaited = true
				return true
			}
		}
		return false
	}
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for !matched() {
		select {
		case <-tick.C:
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return errors.New("the app didn't call it")
			}
			return ctx.Err()
		}
	}
	return nil
}

// Calls are those received so far.
func (m *Mock) Calls() []Call {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := make([]Call, len(m.calls))
	for i, c := range m.calls {
		list[i] = *c
	}
	return list
}
