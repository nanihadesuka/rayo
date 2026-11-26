package flow

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/mock"
	"gopkg.in/yaml.v3"
)

// Written is a step with its YAML.
type Written struct {
	Step *Step
	Text string
}

// ParseSteps reads a command or a list of them, as if it were in the flow with that path.
func ParseSteps(path, text string) ([]Written, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		return nil, errors.New("no command")
	}
	list := doc.Content
	if list[0].Kind == yaml.SequenceNode {
		list = list[0].Content
	}
	p := &reader{path: path}
	var steps []Written
	for _, n := range list {
		var b bytes.Buffer // before the step is read, as that changes it
		e := yaml.NewEncoder(&b)
		e.SetIndent(2)
		if err := e.Encode(&yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{n}}); err != nil {
			return nil, err
		}
		s, err := p.step(n)
		if err != nil {
			return nil, err
		}
		steps = append(steps, Written{s, b.String()})
	}
	return steps, nil
}

// Parse reads a flow that may not be in a file yet.
func Parse(path string, data []byte) (*Flow, error) { return parse(path, data, 0) }

// Session is a flow whose steps are given as it runs.
type Session struct{ r *Runner }

// Live runs a flow for as long as talk lasts, and then puts back what it changed.
func (r *Runner) Live(ctx context.Context, f *Flow, a Attempt, talk func(s *Session) error) error {
	r.prepare(ctx, f, a)
	err := r.withEnv(f, func() error {
		err := r.begin(f)
		if err == nil {
			err = r.steps(ctx, f.OnStart, 0)
		}
		if err == nil {
			err = talk(&Session{r})
		}
		if ctx.Err() == nil && r.agent.Dead() == nil {
			err = errors.Join(err, r.steps(ctx, f.OnComplete, 0))
		}
		return err
	})
	if e := r.saveCapture(); e != nil {
		err = errors.Join(err, e)
	}
	r.noteProxy()
	r.reset()
	return err
}

// Done is how some steps went.
type Done struct {
	Passed   int   // how many, from the first
	Marks    []int // of each that passed, what Captured was before it
	Err      error
	Failure  *Failure
	Warnings []string
	Calls    []mock.Call // made meanwhile
}

// Do runs steps until one fails.
func (s *Session) Do(ctx context.Context, steps []*Step) Done {
	r := s.r
	r.began, r.failure, r.warnings = time.Now(), nil, nil
	before := 0
	if r.backend.mock != nil {
		before = len(r.backend.mock.Calls())
	}
	var d Done
	for _, step := range steps {
		mark := s.Captured()
		if d.Err = r.step(ctx, step, 0); d.Err != nil {
			if n := s.Drop(mark); n > 0 {
				r.reporter.Note(1, fmt.Sprintf("the %d calls of the step that failed aren't kept in the capture", n))
			}
			break
		}
		d.Passed++
		d.Marks = append(d.Marks, mark)
	}
	if d.Err != nil && ctx.Err() == nil {
		r.evidence(ctx)
		d.Failure = r.failure
	}
	if r.backend.mock != nil {
		d.Calls = r.backend.mock.Calls()[before:]
	}
	for i, c := range d.Calls {
		d.Calls[i].Body = ""
		if c.Unmocked {
			r.warnings = append(r.warnings, "the app called what isn't mocked: "+c.What())
		}
	}
	d.Warnings = r.warnings
	return d
}

// Capturing says whether the real backend answers, and what it does is kept.
func (s *Session) Capturing() bool { return s.r.backend.capturing }

// Captured is how many calls have been captured so far: a mark for Drop. 0 when not capturing.
func (s *Session) Captured() int {
	if !s.Capturing() || s.r.backend.mock == nil {
		return 0
	}
	return s.r.backend.mock.Captured()
}

// Drop forgets the calls captured since a mark, and returns how many.
func (s *Session) Drop(mark int) int {
	if !s.Capturing() || s.r.backend.mock == nil {
		return 0
	}
	return s.r.backend.mock.Drop(mark)
}

// Screen is the elements on it.
func (s *Session) Screen(ctx context.Context) (string, error) {
	a, err := s.r.agent.Do(ctx, "tree", nil, 10*time.Second)
	return CapScreen(a.Out, MostShown), err
}

// MostShown is how many elements of a screen are printed: a huge list would flood whoever reads it.
const MostShown = 500

// CapScreen keeps the first most lines of a screen, and says how many it left out.
func CapScreen(screen string, most int) string {
	lines := strings.SplitAfter(screen, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= most {
		return screen
	}
	return strings.Join(lines[:most], "") + fmt.Sprintf("... %d more elements: --find, or a selector with an id, narrows it\n", len(lines)-most)
}

// States is the mocks' scenarios, a line each with its state.
func (s *Session) States() (string, error) {
	if s.r.backend.mock == nil {
		return "", errNoMock
	}
	list := s.r.backend.scenarios()
	if len(list) == 0 {
		return "no mock has a scenario\n", nil
	}
	var b strings.Builder
	for _, sc := range list {
		fmt.Fprintf(&b, "%s: %s\n", sc.Name, sc.State)
	}
	return b.String(), nil
}

// Dead is why the session can't go on; nil while it can.
func (s *Session) Dead() error { return s.r.agent.Dead() }

// NewFlow is the text of a flow without steps yet.
func NewFlow(app string, mocks []string) string {
	text := fmt.Sprintf("appId: %s\n", app)
	if len(mocks) > 0 {
		text += fmt.Sprintf("mock: [%s]\n", strings.Join(mocks, ", "))
	}
	return text + "---\n"
}
