package flow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/adb"
	"github.com/nanihadesuka/rayo/internal/agent"
	"github.com/nanihadesuka/rayo/internal/mock"
	"gopkg.in/yaml.v3"
)

// backend is the flow's mock, reached through a port of the device.
type backend struct {
	device *adb.Device
	port   int
	mock   *mock.Mock // nil until the flow uses one

	capturing bool // the real backend answers
	real      string
	matching  mock.Settings
	secrets   map[string]string

	proxy   bool     // the device's proxy, and not the app, sends calls to the mock
	hosts   []string // those the proxy opens; empty for the default
	proxied *mock.Proxy
	ca      string // the folder of its CA; empty for the computer's
	caNoted bool
	unset   func() error // puts the device's proxy back

	unmocked *mock.Unmocked // of the flow that runs
}

func newBackend(d *adb.Device, o Options) *backend {
	return &backend{device: d, port: o.MockPort, capturing: o.Capture != "", real: o.Backend, secrets: o.Env, hosts: o.Hosts, matching: o.Matching, ca: o.CA}
}

// opens says which hosts the proxy opens: those asked for; else, capturing, all but the passthrough list;
// else those the mocks know, so that a call to another host passes untouched.
func (b *backend) opens(m *mock.Mock) func(host string) bool {
	return func(host string) bool {
		switch {
		case len(b.hosts) > 0:
			return mock.Matches(host, b.hosts)
		case b.capturing:
			return !mock.Matches(host, mock.Passthrough)
		}
		return m.Hosts()[strings.ToLower(host)]
	}
}

// during says which step the calls from now on belong to.
func (b *backend) during(s *Step) {
	if b.capturing && b.mock != nil {
		b.mock.During(s.Place() + " " + s.Text())
	}
}

// start starts it the first time.
func (b *backend) start() (*mock.Mock, error) {
	if b.mock != nil {
		return b.mock, nil
	}
	m := mock.New()
	if b.capturing {
		m.Forward(b.real, b.secrets, b.matching)
	} else {
		m.SetUnmocked(b.unmocked)
	}
	if b.proxy {
		return b.startProxy(m)
	}
	port, err := m.Start()
	if err != nil {
		return nil, err
	}
	if err := b.device.Reverse(b.port, port); err != nil {
		return nil, errors.Join(err, m.Close())
	}
	b.mock = m
	return m, nil
}

func (b *backend) startProxy(m *mock.Mock) (*mock.Mock, error) {
	ca, err := mock.LoadCA(b.ca)
	if err != nil {
		return nil, err
	}
	p := mock.NewProxy(m, ca, b.opens(m))
	p.Asked = func(host string) bool { return len(b.hosts) > 0 && mock.Matches(host, b.hosts) }
	port, err := p.Start()
	if err != nil {
		return nil, err
	}
	if err := b.device.Reverse(b.port, port); err != nil {
		return nil, errors.Join(err, p.Close())
	}
	unset, err := b.device.SetProxy(fmt.Sprintf("127.0.0.1:%d", b.port))
	if err != nil {
		return nil, errors.Join(err, b.device.Unreverse(b.port), p.Close())
	}
	b.mock, b.proxied, b.unset = m, p, unset
	return m, nil
}

func (b *backend) close() error {
	if b.mock == nil {
		return nil
	}
	var err error
	if b.unset != nil { // first, or the device calls a port with nothing behind it
		err = b.unset()
	}
	err = errors.Join(err, b.device.Unreverse(b.port), b.proxied.Close(), b.mock.Close())
	b.mock, b.proxied, b.unset = nil, nil, nil
	return err
}

// noteProxy says which hosts the proxy left alone.
func (r *Runner) noteProxy() {
	if r.backend.proxied == nil {
		return
	}
	if !r.backend.caNoted { // so that a CA the app doesn't trust is easy to see
		r.backend.caNoted = true
		r.reporter.Note(1, "the proxy signs with the CA in "+filepath.ToSlash(r.backend.proxied.CA()))
	}
	for _, n := range r.backend.proxied.Notes() {
		r.reporter.Note(1, n)
	}
}

// noteIgnored says which calls had no mock but were let through, so that nothing is hidden.
func (r *Runner) noteIgnored() {
	if r.backend.mock == nil {
		return
	}
	var list []string
	for _, c := range r.backend.mock.Calls() {
		if call := c.What(); c.Ignored && !slices.Contains(list, call) {
			list = append(list, call)
		}
	}
	if len(list) > 0 {
		r.reporter.Note(1, fmt.Sprintf("ignored %d unmocked calls: %s", len(list), strings.Join(list, ", ")))
	}
}

// check is an error if the app called something without a mock.
func (b *backend) check() error {
	if b.mock == nil {
		return nil
	}
	if err := b.mock.Err(); err != nil {
		return err
	}
	var list []string
	for _, c := range b.mock.Calls() {
		call := c.What()
		if why := c.Why(); why != "" {
			call += "\n      " + why
		}
		if c.Unmocked && !slices.Contains(list, call) {
			list = append(list, call)
		}
	}
	if list == nil {
		return nil
	}
	text := "the app called what isn't mocked:\n    " + strings.Join(list, "\n    ")
	if loaded := b.mock.Loaded(); len(loaded) > 0 {
		text += "\n  mocks loaded: " + strings.Join(loaded, ", ")
	} else {
		text += "\n  no mocks were loaded: the flow's mock: says which"
	}
	return errors.New(text)
}

// noteUnreached says, of a flow that failed with mocks no call reached, why that may be.
func (r *Runner) noteUnreached(err error) {
	b := r.backend
	if err == nil || b.mock == nil || b.capturing || b.proxy || b.mock.Rules() == 0 || len(b.mock.Calls()) > 0 {
		return
	}
	r.reporter.Note(1, "no call reached the mock: is the app's interceptor on in this build? else try network: proxy")
}

const fewCalls = 5

// calls is every call since a time, as text or JSON.
func (b *backend) calls(reporter Reporter, asJSON bool, since time.Time) ([]byte, error) {
	calls := b.mock.Calls()
	for len(calls) > 0 && calls[0].At.Before(since) {
		calls = calls[1:]
	}
	for i := range calls {
		if b.capturing {
			calls[i].Body = "" // a real one may hold secrets
		}
	}
	var text bytes.Buffer
	if !asJSON {
		for _, c := range calls {
			fmt.Fprintln(&text, c)
			if c.Body != "" {
				fmt.Fprintf(&text, "    %.1000s\n", c.Body)
			}
		}
		return text.Bytes(), nil
	}
	e := json.NewEncoder(&text)
	e.SetEscapeHTML(false)
	e.SetIndent("", "  ")
	err := e.Encode(calls)
	return text.Bytes(), err
}

// loadMocks adds those of a flow's header.
func (r *Runner) loadMocks(f *Flow) error {
	if r.backend.capturing {
		m, err := r.backend.start()
		if err == nil && r.CaptureUpdate != "" { // made as the mocks were
			err = m.UseSettingsOf(r.CaptureUpdate)
		}
		if err == nil && r.CaptureAppend {
			var n int
			if n, err = m.Continue(r.Capture); err == nil {
				r.reporter.Note(1, fmt.Sprintf("going on from the %d calls captured in %s", n, filepath.ToSlash(r.Capture)))
			}
		}
		return err
	}
	for _, n := range f.Mock {
		path, err := r.script.fill(n.Value)
		if err != nil {
			return problem(f.Path, n.Line, "%v", err)
		}
		if path == "" { // ${...} said none, such as against the real backend
			continue
		}
		m, err := r.backend.start()
		if err != nil {
			return err
		}
		if err := m.Load(relative(f.Path, path)); err != nil {
			return problem(f.Path, n.Line, "%v", err)
		}
	}
	return nil
}

// saveCapture writes what the real backend answered as mocks, also after a failure: it can't always be had again.
func (r *Runner) saveCapture() error {
	if !r.backend.capturing || r.backend.mock == nil {
		return nil
	}
	notes, err := r.backend.mock.SaveCapture(r.Capture)
	if err != nil {
		return fmt.Errorf("couldn't save the capture: %w", err)
	}
	if r.CaptureUpdate != "" && len(r.backend.mock.Calls()) > 0 {
		changes, err := mock.Compare(r.CaptureUpdate, r.Capture)
		if err != nil {
			return fmt.Errorf("couldn't compare the capture with %s: %w", filepath.ToSlash(r.CaptureUpdate), err)
		}
		report := mock.Report(changes)
		if r.CaptureCheck {
			for _, n := range append(notes, report...) {
				r.reporter.Note(0, n)
			}
			if differ := len(changes) - countSame(changes); differ > 0 {
				return fmt.Errorf("the mocks in %s differ from the backend: %d changes", filepath.ToSlash(r.CaptureUpdate), differ)
			}
			return nil
		}
		report = append(report, "to take them: rayo mocks apply "+filepath.ToSlash(r.CaptureUpdate)+"   (--only a,b for some; --prune to take out those gone)")
		notes = append(notes, report...)
		if err := os.WriteFile(filepath.Join(r.Capture, "changes.txt"), []byte(strings.Join(report, "\n")+"\n"), 0o644); err != nil {
			return err
		}
	}
	for _, n := range notes {
		r.reporter.Note(0, n)
	}
	return nil
}

// skippedWhileCapturing says so of a step that changes the mock, as there is none.
func skippedWhileCapturing(c *call) bool {
	if c.r.backend.capturing {
		c.r.reporter.Note(c.depth+1, c.s.Kind+" does nothing while capturing")
	}
	return c.r.backend.capturing
}

// Dead says whether the device was lost.
func (r *Runner) Dead() bool { return r.agent.Dead() != nil }

// --- mock ---

func checkMock(p *reader, s *Step) error {
	if hasExpression(s.Value) {
		return nil
	}
	var err error
	file := keyFile.of(s.Value)
	switch {
	case (file == "") == (get(s.Value, keyRequest) == nil):
		err = errors.New("mock needs either a file or a request and a response")
	case file != "":
		_, err = os.Stat(relative(p.path, file))
	case get(s.Value, keyResponse) == nil:
		err = errors.New("mock needs a response")
	default:
		_, err = ruleFrom(s.Value)
	}
	return p.at(s.Value, err)
}

func addMock(c *call) error {
	if skippedWhileCapturing(c) {
		return nil
	}
	m, err := c.r.backend.start()
	if err != nil {
		return err
	}
	if file := arg(c, keyFile); file != "" {
		return m.Load(relative(c.s.Path, file))
	}
	rule, err := ruleFrom(c.v)
	if err != nil {
		return err
	}
	return m.Add(rule)
}

func sayMock(v *yaml.Node) string {
	if q := get(v, keyRequest); q != nil {
		return short(q)
	}
	return short(v)
}

func ruleFrom(v *yaml.Node) (*mock.Rule, error) {
	top := agent.Args{}
	err := pairs(v, func(k, n *yaml.Node) error {
		if isOption(k.Value) {
			return nil
		}
		var value any
		err := n.Decode(&value)
		top[k.Value] = value
		return err
	})
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(top)
	if err != nil {
		return nil, err
	}
	r, err := mock.ParseRule(data)
	if err == nil && r.Priority == nil {
		r.Priority = new(int) // what a flow says now wins over the files
	}
	return r, err
}

// --- mockState ---

func checkMockState(p *reader, s *Step) error {
	if listing(s.Value) == "" {
		return p.fail(s.Value, "mockState takes a scenario and its state, such as { ride: docked }")
	}
	return nil
}

func setMockState(c *call) error {
	if skippedWhileCapturing(c) {
		return nil
	}
	if c.r.backend.mock == nil {
		return errNoMock
	}
	return pairs(c.v, func(k, n *yaml.Node) error {
		if isOption(k.Value) {
			return nil
		}
		return c.r.backend.mock.SetState(k.Value, n.Value)
	})
}

var errNoMock = errors.New("this flow has no mock")

// listing is "key: value" for each key that isn't an option.
func listing(v *yaml.Node) string {
	var parts []string
	_ = pairs(v, func(k, n *yaml.Node) error { // never fails
		if !isOption(k.Value) {
			parts = append(parts, k.Value+": "+n.Value)
		}
		return nil
	})
	return strings.Join(parts, ", ")
}

// --- network ---

func checkNetwork(p *reader, s *Step) error {
	changes := get(s.Value, keyDelay) != nil || get(s.Value, keyFail) != nil
	if changes == (get(s.Value, keyState) != nil) {
		return p.fail(s.Value, "network takes online or offline, or else delay and fail")
	}
	return nil
}

func setNetwork(c *call) error {
	if skippedWhileCapturing(c) {
		return nil
	}
	if c.r.backend.mock == nil {
		return errNoMock
	}
	c.r.backend.mock.SetNetwork(mock.Network{
		Offline: arg(c, keyState), Delay: arg(c, keyDelay), Fail: arg(c, keyFail), Path: arg(c, keyCallPath),
	})
	return nil
}

// --- waitForRequest ---

func awaitRequest(c *call) error {
	if c.r.backend.mock == nil {
		return errNoMock
	}
	m := c.r.backend.mock
	err := m.Await(c.ctx, arg(c, keyMethod), arg(c, keyCallPath), arg(c, keyBody), c.limit)
	if err != nil && c.ctx.Err() == nil {
		err = fmt.Errorf("%w\n    %s", err, strings.Join(m.Missed(arg(c, keyMethod), arg(c, keyCallPath), arg(c, keyBody)), "\n    "))
	}
	return err
}

// --- assertRequest ---

func assertRequest(c *call) error {
	if c.r.backend.mock == nil {
		return errNoMock
	}
	want := arg(c, keyCalls)
	if n := c.r.backend.mock.Count(arg(c, keyMethod), arg(c, keyCallPath), arg(c, keyBody)); n != want {
		text := fmt.Sprintf("the app made it %d times, not %d", n, want)
		if n < want {
			text += "\n    " + strings.Join(c.r.backend.mock.Missed(arg(c, keyMethod), arg(c, keyCallPath), arg(c, keyBody)), "\n    ")
		}
		return errors.New(text)
	}
	return nil
}

func countSame(changes []mock.Change) int {
	n := 0
	for _, c := range changes {
		if c.Kind == mock.KindSame {
			n++
		}
	}
	return n
}
