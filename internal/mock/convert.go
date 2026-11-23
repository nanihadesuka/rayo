package mock

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Turning a capture into mocks. What is always answered the same needs no more than that. For the
// rest, the capture is cut into states of one scenario: a call that isn't a GET begins a state, and
// so does a GET whose answer changes by itself. States that make no difference are joined.

const scenario = "capture"

// state is a stretch of the capture in which each GET is always answered the same.
type state struct {
	by      *captured            // the call that began it; nil if the backend changed by itself
	first   captured             // what showed that, if so
	answers map[string]*captured // of each GET seen in it
}

// rule is a mapping as it is written: Rule can't tell what was left out.
type rule struct {
	Name     string         `json:"name"`
	Priority int            `json:"priority,omitempty"`
	Scenario string         `json:"scenarioName,omitempty"`
	Required string         `json:"requiredScenarioState,omitempty"`
	Next     string         `json:"newScenarioState,omitempty"`
	Request  map[string]any `json:"request"`
	Response map[string]any `json:"response"`
}

func stateName(i int) string {
	if i == 0 {
		return "Started"
	}
	return strconv.Itoa(i)
}

// varying are the endpoints with more than one answer.
func varying(calls []captured) map[string]bool {
	first, vary := map[string]string{}, map[string]bool{}
	for _, c := range calls {
		a, seen := first[c.endpoint()]
		if !seen {
			first[c.endpoint()] = c.answer()
		} else if a != c.answer() {
			vary[c.endpoint()] = true
		}
	}
	return vary
}

func cut(calls []captured, vary map[string]bool) []*state {
	all := []*state{{answers: map[string]*captured{}}}
	for i := range calls {
		c := &calls[i]
		if !c.read() {
			all = append(all, &state{by: c, answers: map[string]*captured{}})
			continue
		}
		if !vary[c.endpoint()] {
			continue
		}
		now := all[len(all)-1]
		if before := now.answers[c.endpoint()]; before != nil && before.answer() != c.answer() {
			now = &state{first: *c, answers: map[string]*captured{}}
			all = append(all, now)
		}
		now.answers[c.endpoint()] = c
	}
	return join(all, vary, calls)
}

// join joins the states that make no difference. When an answer changed, the call behind it is one of
// those since it was last seen: the rarest, as one that is made all the time seldom changes anything.
func join(all []*state, vary map[string]bool, calls []captured) []*state {
	times := map[string]int{}
	for _, c := range calls {
		times[c.endpoint()]++
	}
	joined := all[:1:1]
	var writes []*captured    // in the last state of those
	since := map[string]int{} // how many of them there were when a GET was last seen
	for _, s := range all[1:] {
		last := joined[len(joined)-1]
		if s.by != nil {
			writes = append(writes, s.by)
		}
		differs, from := s.by == nil, 0
		for e, c := range s.answers {
			if before := last.answers[e]; before != nil && before.answer() != c.answer() {
				differs, from = true, max(from, since[e])
			}
		}
		if s.by != nil && vary[s.by.endpoint()] { // it is told apart by its own answer
			differs, from = true, len(writes)-1
		}
		if differs {
			if s.by != nil {
				for _, w := range writes[from:] {
					if times[w.endpoint()] < times[s.by.endpoint()] {
						s.by = w
					}
				}
			}
			joined = append(joined, s)
			writes, since = nil, map[string]int{}
			continue
		}
		maps.Copy(last.answers, s.answers)
		for e := range s.answers {
			since[e] = len(writes)
		}
	}
	return joined
}

// answerIn is a GET's in a state: the one seen, or the last before it, or the first after.
func answerIn(states []*state, at int, endpoint string) *captured {
	for i := at; i >= 0; i-- {
		if c := states[i].answers[endpoint]; c != nil {
			return c
		}
	}
	for _, s := range states[at:] {
		if c := s.answers[endpoint]; c != nil {
			return c
		}
	}
	return nil
}

// nextOf is the first state after one that this endpoint begins, and whether it may go there: not
// past a state that another call has to begin.
func nextOf(states []*state, at int, endpoint string) (next int, reachable bool) {
	reachable = true
	for i := at + 1; i < len(states); i++ {
		if by := states[i].by; by != nil {
			if by.endpoint() == endpoint {
				return i, reachable
			}
			reachable = false
		}
	}
	return 0, false
}

// callsFile keeps the calls of a capture as they were, so that another can go on from it.
const callsFile = "capture.calls.json"

type keptCapture struct {
	Calls []kept   `json:"calls"`
	Files []string `json:"files"` // in __files, written from them
}

// Continue goes on with the capture in a folder: its calls come before those made from now on, as if
// in one go, and SaveCapture writes all of them there again.
func (m *Mock) Continue(folder string) (int, error) {
	data, err := os.ReadFile(filepath.Join(folder, callsFile))
	if errors.Is(err, os.ErrNotExist) {
		return 0, fmt.Errorf("%s has no %s to go on from: it isn't a capture, or one of rayo before 1.0.5", filepath.ToSlash(folder), callsFile)
	}
	if err != nil {
		return 0, err
	}
	var k keptCapture
	if err := json.Unmarshal(data, &k); err != nil {
		return 0, fmt.Errorf("%s: %w", filepath.ToSlash(filepath.Join(folder, callsFile)), err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	before := make([]captured, 0, len(k.Calls)+len(m.real.captured))
	for _, c := range k.Calls {
		before = append(before, c.toCaptured())
	}
	m.real.captured = append(before, m.real.captured...)
	m.real.earlier = k.Files
	return len(k.Calls), nil
}

// SaveCapture writes a folder of mocks, and returns what to say about it.
func (m *Mock) SaveCapture(folder string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := slices.Clone(m.real.captured)
	var notes []string
	if len(calls) == 0 {
		notes = append(notes, "nothing captured: no call of the app was answered")
	} else {
		for _, name := range m.real.earlier { // written again from all the calls
			if err := os.Remove(filepath.Join(folder, "__files", name)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
		}
		states, files, err := writeMocks(folder, calls)
		if err != nil {
			return nil, err
		}
		k := keptCapture{Files: files}
		for _, c := range calls {
			k.Calls = append(k.Calls, c.toKept())
		}
		if err := writeJSON(filepath.Join(folder, callsFile), k); err != nil {
			return nil, err
		}
		m.real.earlier = files
		notes = report(folder, calls, states)
	}
	if len(m.real.skipped) > 0 {
		notes = append(notes, "not captured:")
	}
	for _, s := range m.real.skipped {
		notes = append(notes, "  "+s)
	}
	if len(calls) == 0 {
		return notes, nil // so that the folder is free for another try
	}
	return notes, os.WriteFile(filepath.Join(folder, "capture.txt"), []byte(strings.Join(notes, "\n")+"\n"), 0o644)
}

func writeMocks(folder string, calls []captured) ([]*state, []string, error) {
	shapes(calls)
	vary := varying(calls)
	states := cut(calls, vary)
	last := map[string]*captured{}
	var endpoints []string
	for i := range calls {
		e := calls[i].endpoint()
		if last[e] == nil {
			endpoints = append(endpoints, e)
		}
		last[e] = &calls[i]
	}
	files := &bodies{folder: filepath.Join(folder, "__files"), names: map[string]string{}}
	var rules []rule
	add := func(c *captured, priority int, required, next string) error {
		response, err := files.response(c)
		r := rule{Name: c.endpoint(), Priority: priority, Required: required, Next: next, Request: request(c), Response: response}
		if required != "" {
			r.Scenario = scenario
		}
		rules = append(rules, r)
		return err
	}
	for _, e := range endpoints {
		c := last[e]
		if c.read() && vary[e] {
			for i := range states {
				if err := add(answerIn(states, i, e), 0, stateName(i), ""); err != nil {
					return nil, nil, err
				}
			}
			continue
		}
		priority := 0
		for i := range states {
			next, reachable := nextOf(states, i, e)
			if next == 0 {
				break
			}
			priority = 6 // of its answer whenever it is called besides
			to := ""
			if reachable {
				to = stateName(next)
			}
			if reachable || vary[e] {
				if err := add(states[next].by, 0, stateName(i), to); err != nil {
					return nil, nil, err
				}
			}
		}
		if err := add(c, priority, "", ""); err != nil {
			return nil, nil, err
		}
	}
	written := slices.Sorted(maps.Values(files.names))
	return states, written, writeJSON(filepath.Join(folder, "mappings", "capture.json"), map[string]any{"mappings": rules})
}

func report(folder string, calls []captured, states []*state) []string {
	notes := []string{
		fmt.Sprintf("captured %d calls, in %d states: %s", len(calls), len(states), filepath.ToSlash(folder)),
		"to use it, set mock in the flow's header to its path, relative to the flow",
		"read it before committing: what looks private is hidden, but not all that is may look so",
	}
	alone := false
	for i, s := range states {
		if i == 0 || s.by != nil {
			continue
		}
		if !alone {
			notes = append(notes, "the backend changed by itself; to repeat it, add before these steps:")
			alone = true
		}
		notes = append(notes, fmt.Sprintf("  - mockState: { %s: %d }   # %s  (%s)", scenario, i, s.first.step, s.first.endpoint()))
	}
	return notes
}

// request matches the whole address, except what was hidden of it.
func request(c *captured) map[string]any {
	q := map[string]any{"method": c.method}
	if p := bodyPatterns(c); p != nil {
		q["bodyPatterns"] = p
	}
	if strings.Contains(c.uri, redacted) {
		q["urlPattern"] = strings.ReplaceAll(regexp.QuoteMeta(c.uri), redacted, `[^/?&]*`)
	} else {
		q["url"] = c.uri
	}
	return q
}

// bodyPatterns tell a GraphQL call apart by its operation.
func bodyPatterns(c *captured) []map[string]string {
	if c.op == nil {
		return nil
	}
	return []map[string]string{{"matchesJsonPath": c.op.condition()}}
}

// bodies are kept one file each, only once.
type bodies struct {
	folder string
	names  map[string]string
}

var notInNames = regexp.MustCompile(`[^A-Za-z0-9]+`)

func (b *bodies) response(c *captured) (map[string]any, error) {
	r := map[string]any{"status": c.status}
	if c.contentType != "" {
		r["headers"] = map[string]string{"Content-Type": c.contentType}
	}
	if c.body == "" {
		return r, nil
	}
	name, ok := b.names[c.body]
	if !ok {
		path, _, _ := strings.Cut(c.uri, "?")
		path = strings.TrimRight(path, "/")
		if c.op != nil && c.op.Name != "" {
			path = c.op.Name
		}
		what := strings.Trim(notInNames.ReplaceAllString(path[strings.LastIndex(path, "/")+1:], "_"), "_")
		ext := ".txt"
		if kind, _, _ := mime.ParseMediaType(c.contentType); strings.HasSuffix(kind, "json") {
			ext = ".json"
		}
		name = fmt.Sprintf("%03d_%s_%.40s%s", len(b.names)+1, strings.ToLower(c.method), what, ext)
		b.names[c.body] = name
		if err := os.MkdirAll(b.folder, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(b.folder, name), []byte(c.body), 0o644); err != nil {
			return nil, err
		}
	}
	r["bodyFileName"] = name
	return r, nil
}

func writeJSON(path string, value any) error {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	e.SetIndent("", "  ")
	if err := e.Encode(value); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}
