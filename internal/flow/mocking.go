package flow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

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
	secrets   map[string]string
}

func newBackend(d *adb.Device, o Options) *backend {
	return &backend{device: d, port: o.MockPort, capturing: o.Capture != "", real: o.Backend, secrets: o.Env}
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
		m.Forward(b.real, b.secrets)
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

func (b *backend) close() error {
	if b.mock == nil {
		return nil
	}
	err := errors.Join(b.device.Unreverse(b.port), b.mock.Close())
	b.mock = nil
	return err
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
		if call := c.Method + " " + c.URI; c.Unmocked && !slices.Contains(list, call) {
			list = append(list, call)
		}
	}
	if list == nil {
		return nil
	}
	return fmt.Errorf("the app called what isn't mocked:\n    %s", strings.Join(list, "\n    "))
}

const fewCalls = 5

// calls is every call, as text or JSON, and reports the last ones.
func (b *backend) calls(reporter Reporter, asJSON bool) ([]byte, error) {
	calls := b.mock.Calls()
	first := max(0, len(calls)-fewCalls)
	for i, c := range calls {
		if b.capturing {
			calls[i].Body = "" // a real one may hold secrets
		}
		if i == first {
			reporter.Note(1, "last calls:")
		}
		if i >= first {
			reporter.Note(2, c.String())
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
		_, err := r.backend.start()
		return err
	}
	for _, n := range f.Mock {
		path, err := r.script.fill(n.Value)
		if err != nil {
			return problem(f.Path, n.Line, "%v", err)
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
	return c.r.backend.mock.Await(c.ctx, arg(c, keyMethod), arg(c, keyCallPath), arg(c, keyBody), c.limit)
}
