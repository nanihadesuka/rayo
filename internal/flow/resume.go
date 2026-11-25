package flow

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Start is where a flow begins instead of at its first step, and where it stops: of --resume, --from
// and --until. The app is left as it is, so onFlowStart doesn't run.
type Start struct {
	From   string     // a step, file:line; empty for the first
	Until  string     // the step after which the flow stops, leaving the app as it is; empty for none
	States []Scenario // set once From is reached
	Whence string     // where the states come from, as said then
}

// Scenario is a scenario of the mocks and the state it is in.
type Scenario struct{ Name, State string }

// StepStates is the scenarios as a step began.
type StepStates struct {
	At     string // file:line
	States []Scenario
}

// trailMost is how many StepStates a failure keeps: a long flow's last ones.
const trailMost = 2000

var errUntil = errors.New("stopped as --until says")

// scenarios are a mock's, in order of name; none without a mock.
func (b *backend) scenarios() []Scenario {
	if b.mock == nil {
		return nil
	}
	var list []Scenario
	for name, state := range b.mock.States() {
		list = append(list, Scenario{name, state})
	}
	slices.SortFunc(list, func(a, b Scenario) int { return strings.Compare(a.Name, b.Name) })
	return list
}

// Is says whether a step is the one an id names: file:line, the file as given or only its name; or a
// line alone, of the flow's own file.
func (s *Step) Is(id, flowPath string) bool {
	file, line := flowPath, id
	if i := strings.LastIndex(id, ":"); i >= 0 {
		file, line = id[:i], id[i+1:]
	}
	if n, err := strconv.Atoi(line); err != nil || n != s.Line {
		return false
	}
	if !strings.ContainsAny(file, `/\`) && filepath.Base(s.Path) == file {
		return true
	}
	a, errA := filepath.Abs(file)
	b, errB := filepath.Abs(s.Path)
	return errA == nil && errB == nil && strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// CheckStepID says whether an id may name a step: file:line, a line, or a text of one step.
func CheckStepID(id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("a step is file:line, such as flows/book.yaml:42, a line of the flow's own file, or a text of one step such as pickDate")
	}
	return nil
}

// isPlace says whether an id is file:line or a line, and not a text.
func isPlace(id string) bool {
	n, err := strconv.Atoi(id[strings.LastIndex(id, ":")+1:])
	return err == nil && n > 0
}

// Resolve turns an id that is a text into the file:line of the one step whose text has it, as rayo
// shows a step; one that is already a place stays as it is.
func (f *Flow) Resolve(id string) (string, error) {
	if isPlace(id) {
		if !f.Find(id) {
			return "", fmt.Errorf("%s has no step at %s (not of onFlowStart or onFlowComplete); its steps:\n%s", f.Name, id, f.StepList())
		}
		return id, nil
	}
	var found []*Step
	var walk func(steps []*Step)
	walk = func(steps []*Step) {
		for _, s := range steps {
			if strings.Contains(strings.ToLower(s.describe()), strings.ToLower(id)) {
				found = append(found, s)
			}
			walk(s.Sub)
		}
	}
	walk(f.Steps)
	switch len(found) {
	case 1:
		return found[0].Place(), nil
	case 0:
		return "", fmt.Errorf("no step of %s has %q; give file:line, from its steps:\n%s", f.Name, id, f.StepList())
	}
	var list []string
	for _, s := range found {
		list = append(list, "  "+s.Place()+"  "+s.describe())
	}
	return "", fmt.Errorf("%d steps of %s have %q; give file:line of one:\n%s", len(found), f.Name, id, strings.Join(list, "\n"))
}

// StepList is the flow's own steps, a line each with its place, as an option takes it.
func (f *Flow) StepList() string {
	var list []string
	var walk func(steps []*Step, depth int)
	walk = func(steps []*Step, depth int) {
		for _, s := range steps {
			list = append(list, strings.Repeat("  ", depth+1)+s.Place()+"  "+s.describe())
			walk(s.Sub, depth+1)
		}
	}
	walk(f.Steps, 0)
	return strings.Join(list, "\n")
}

// Find says whether a flow has a step, among its steps and those of the flows it runs; not those of
// onFlowStart, which a flow begun in the middle doesn't run, nor of onFlowComplete. A file given by ${...} can't be looked into.
func (f *Flow) Find(id string) bool {
	seen := map[string]bool{}
	var walk func(steps []*Step) bool
	walk = func(steps []*Step) bool {
		for _, s := range steps {
			if s.Is(id, f.Path) || walk(s.Sub) {
				return true
			}
			if file := keyFile.of(s.Value); file != "" && !strings.Contains(file, "${") {
				path := relative(s.Path, file)
				if !seen[path] {
					seen[path] = true
					if included, err := Load(path); err == nil && walk(slices.Concat(included.OnStart, included.Steps, included.OnComplete)) {
						return true
					}
				}
			}
		}
		return false
	}
	return walk(f.Steps)
}

// leadsTo says whether the step to begin from is in a step: in its commands, or in the flow it runs.
func (r *Runner) leadsTo(s *Step, depth int) bool {
	if depth > 20 {
		return false
	}
	for _, sub := range s.Sub {
		if sub.Is(r.from, r.flow.Path) || r.leadsTo(sub, depth+1) {
			return true
		}
	}
	included := r.included(s)
	if included == nil {
		return false
	}
	for _, sub := range slices.Concat(included.OnStart, included.Steps, included.OnComplete) {
		if sub.Is(r.from, r.flow.Path) || r.leadsTo(sub, depth+1) {
			return true
		}
	}
	return false
}

// included is the flow a step runs from a file; nil if none, or if it can't be read.
func (r *Runner) included(s *Step) *Flow {
	file := keyFile.of(s.Value)
	if file == "" || s.Kind != "runFlow" {
		return nil
	}
	file, err := r.script.fill(file)
	if err != nil || file == "" {
		return nil
	}
	f, err := Load(relative(s.Path, file))
	if err != nil {
		return nil
	}
	return f
}

// pass goes by a step before the one to begin from. Mocks are still added, as later steps need them;
// all else is left as the last run left it.
func (r *Runner) pass(ctx context.Context, s *Step, depth int) error {
	switch {
	case s.Kind == "mock":
		return r.step(ctx, s, depth)
	case len(s.Sub) > 0:
		for _, sub := range s.Sub {
			if err := r.pass(ctx, sub, depth); err != nil {
				return err
			}
		}
		return nil
	}
	if f := r.included(s); f != nil {
		if err := r.loadMocks(f); err != nil {
			return err
		}
		for _, sub := range slices.Concat(f.OnStart, f.Steps, f.OnComplete) {
			if err := r.pass(ctx, sub, depth); err != nil {
				return err
			}
		}
		return nil
	}
	r.passed++
	return nil
}

// reached begins where the flow was asked to: it sets the scenarios, and says so.
func (r *Runner) reached(s *Step) error {
	r.from = ""
	skipped := fmt.Sprintf("%d steps", r.passed)
	if len(r.flow.OnStart) > 0 {
		skipped = "onFlowStart and " + skipped
	}
	text := fmt.Sprintf("from %s %s: skipped %s before it, the app is as the last run left it", s.Place(), s.describe(), skipped)
	if len(r.Start.States) == 0 {
		r.reporter.Note(0, text)
		if r.Start.Whence != "" {
			r.reporter.Note(0, r.Start.Whence)
		}
		return nil
	}
	if r.backend.mock == nil {
		return fmt.Errorf("can't set the mocks' scenarios: %w", errNoMock)
	}
	var set []string
	for _, sc := range r.Start.States {
		if err := r.backend.mock.SetState(sc.Name, sc.State); err != nil {
			return fmt.Errorf("can't set the mocks' scenarios: %w", err)
		}
		set = append(set, sc.Name+"="+sc.State)
	}
	r.reporter.Note(0, text)
	r.reporter.Note(0, fmt.Sprintf("scenarios %s%s", strings.Join(set, ","), r.Start.Whence))
	return nil
}

// stopsAfter says whether the flow stops after a step, as --until says.
func (r *Runner) stopsAfter(s *Step) bool {
	return r.Start != nil && r.Start.Until != "" && s.Is(r.Start.Until, r.flow.Path)
}

// StatesAt is the scenarios as a step began, the last time it did; nil if it never did.
func StatesAt(trail []StepStates, id, flowPath string) []Scenario {
	for i := len(trail) - 1; i >= 0; i-- {
		at := trail[i].At
		cut := strings.LastIndex(at, ":")
		if cut < 0 {
			continue
		}
		file := at[:cut]
		n, _ := strconv.Atoi(at[cut+1:])
		if (&Step{Path: file, Line: n}).Is(id, flowPath) {
			return trail[i].States
		}
	}
	return nil
}

// Before says whether step a comes before step b among the flow's own steps; false if either isn't one.
func (f *Flow) Before(a, b string) bool {
	ia := slices.IndexFunc(f.Steps, func(s *Step) bool { return s.Is(a, f.Path) })
	ib := slices.IndexFunc(f.Steps, func(s *Step) bool { return s.Is(b, f.Path) })
	return ia >= 0 && ib >= 0 && ia < ib
}
