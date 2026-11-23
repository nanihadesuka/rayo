package mock

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
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
	data, err := io.ReadAll(io.LimitReader(q.Body, 10<<20))
	if err != nil {
		return // the app hung up
	}
	if m.real != nil {
		m.pass(w, q, data)
		return
	}
	a := m.answer(q, string(data))
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
	w.Header()["Content-Type"] = nil // or Go guesses one
	for k, v := range a.headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(a.status)
	_, _ = io.WriteString(w, a.body) // the app may have hung up
}

type answer struct {
	status  int
	headers map[string]string
	body    string
	delay   time.Duration
	drop    bool
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
	var best *Rule
	for _, r := range m.rules {
		// Of the same priority, the last one added wins.
		if best != nil && r.priority() > best.priority() {
			continue
		}
		if r.RequiredScenarioState != "" && m.state(r.ScenarioName) != r.RequiredScenarioState {
			continue
		}
		if r.fits(m.vm, q, body) {
			best = r
		}
	}
	if best == nil {
		call.Outcome, call.Unmocked = "not mocked", true
		a.status, a.body = 404, "not mocked: "+q.Method+" "+call.URI
		return a
	}
	if best.NewScenarioState != "" {
		m.states[best.ScenarioName] = best.NewScenarioState
	}
	call.Outcome = strconv.Itoa(best.Response.Status)
	a.status, a.headers, a.body = best.Response.Status, best.Response.Headers, best.body.render(q, body)
	a.delay += time.Duration(best.Response.FixedDelayMilliseconds) * time.Millisecond
	return a
}

func (m *Mock) state(scenario string) string {
	if s, ok := m.states[scenario]; ok {
		return s
	}
	return "Started"
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
