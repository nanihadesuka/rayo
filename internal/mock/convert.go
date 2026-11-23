package mock

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Turning a capture into mocks. Each call is matched as WireMock's recorder does by default: by its
// method, its path, its query and its body. Calls that match the same are the same call: what is always
// answered the same needs no more than that. For the rest, the capture is cut into states of one
// scenario: a call that isn't a read begins a state, and so does a read whose answer changes by
// itself. States that make no difference are joined.

const scenario = "capture"

// spec is how a captured call is matched.
type spec struct {
	request map[string]any // as written in the mapping
	key     string         // the same for calls that are matched the same
	label   string         // its name, for people
	group   string         // the method and address: of those whose matchers may overlap
	weight  int            // how much it matches: of two that fit a call, the heavier is the closer
}

// builder works out the specs of a capture's calls.
type builder struct {
	s         Settings
	loosened  map[string]bool   // what was left out, or matched by its form
	unmatched map[string]string // the label of a call whose body isn't matched, and why
}

func (b *builder) loose(what string) { b.loosened[what] = true }

// timestampPattern is an ISO 8601 date and time: a value of the moment, matched by its form.
const timestampPattern = `\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}(:\d{2}([.,]\d+)?)?(Z|[+-]\d{2}(:?\d{2})?)?`

var timestamp = regexp.MustCompile(`^` + timestampPattern + `$`)

var hiddenParts = regexp.MustCompile(regexp.QuoteMeta(redacted) + "|" + regexp.QuoteMeta(nobody))

// hidden is a pattern of a text that had something private taken out of it.
func hidden(text string) string {
	parts := hiddenParts.Split(text, -1)
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	return strings.Join(parts, ".*")
}

// value is the matcher of a value of the query, a header or a form, and how it is shown.
func (b *builder) value(where, v string) (map[string]any, string) {
	switch {
	case v == redacted:
		return map[string]any{"matches": ".*"}, "*"
	case hides(v):
		return map[string]any{"matches": hidden(v)}, strings.NewReplacer(redacted, "*", nobody, "*").Replace(v)
	case b.s.Timestamps != TimestampsExact && timestamp.MatchString(v):
		b.loose(where)
		return map[string]any{"matches": timestampPattern}, "<time>"
	}
	return map[string]any{"equalTo": v}, v
}

// params are the matchers of a query or a form, leaving out those ignored; and how they are shown.
func (b *builder) params(kind string, values url.Values, ignored func(string) bool) (map[string]any, string, int) {
	out, weight := map[string]any{}, 0
	var shown []string
	for _, name := range sortedKeys(values) {
		if ignored(name) {
			b.loose(kind + " " + name)
			continue
		}
		var ms []any
		for _, v := range values[name] {
			m, s := b.value(kind+" "+name, v)
			ms = append(ms, m)
			shown = append(shown, name+"="+s)
		}
		if len(ms) == 1 {
			out[name] = ms[0]
		} else {
			out[name] = map[string]any{"hasExactly": ms}
		}
		weight += len(ms)
	}
	return out, strings.Join(shown, "&"), weight
}

func (b *builder) ignoredBody(name, path string) bool {
	for _, e := range b.s.IgnoreBody {
		e = strings.TrimPrefix(strings.TrimPrefix(e, "$"), ".")
		if strings.Contains(e, ".") && e == path || !strings.Contains(e, ".") && e == name {
			return true
		}
	}
	return false
}

// shapeJSON is a JSON body as matched: without what is private, nor what is ignored; with what had
// something private in it, or is a moment, matched by its form.
func shapeJSON(s Settings, v any, path string, loose func(string)) any {
	b := &builder{s: s}
	return b.shapeJSON(v, path, loose)
}

func (b *builder) shapeJSON(v any, path string, loose func(string)) any {
	switch v := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, child := range v {
			p := k
			if path != "" {
				p = path + "." + k
			}
			if private(k) {
				continue
			}
			if b.ignoredBody(k, p) {
				if loose != nil {
					loose("body " + p)
				}
				continue
			}
			out[k] = b.shapeJSON(child, p, loose)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = b.shapeJSON(child, path, loose)
		}
		return out
	case string:
		switch {
		case v == redacted:
			return placeholderString
		case hides(v):
			return placeholderRegex + hidden(v)
		case b.s.Timestamps != TimestampsExact && timestamp.MatchString(v):
			if loose != nil {
				loose("body " + path)
			}
			return placeholderRegex + timestampPattern
		}
	}
	return v
}

// notScalar says whether a JSON value is an object or an array.
func notScalar(v any) bool {
	switch v.(type) {
	case map[string]any, []any:
		return true
	}
	return false
}

// nodes is how many values a JSON value has, itself among them.
func nodes(v any) int {
	n := 1
	switch v := v.(type) {
	case map[string]any:
		for _, child := range v {
			n += nodes(child)
		}
	case []any:
		for _, child := range v {
			n += nodes(child)
		}
	}
	return n
}

// canonical is a JSON value written the same for all that are matched the same: keys sorted, items
// sorted as their order doesn't count, numbers by their value.
func canonical(v any) string {
	switch v := v.(type) {
	case map[string]any:
		parts := make([]string, 0, len(v))
		for _, k := range sortedKeys(v) {
			parts = append(parts, quote(k)+":"+canonical(v[k]))
		}
		return "{" + strings.Join(parts, ",") + "}"
	case []any:
		parts := make([]string, len(v))
		for i, child := range v {
			parts[i] = canonical(child)
		}
		slices.Sort(parts)
		return "[" + strings.Join(parts, ",") + "]"
	case json.Number:
		if r, ok := new(big.Rat).SetString(v.String()); ok {
			return r.RatString()
		}
	}
	return compactJSON(v)
}

// shown is a JSON value as matched, for people: what is matched by its form as * or <time>.
func shown(v any) any {
	switch v := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, child := range v {
			out[k] = shown(child)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = shown(child)
		}
		return out
	case string:
		switch {
		case v == placeholderRegex+timestampPattern:
			return "<time>"
		case strings.HasPrefix(v, "${json-unit."):
			return "*"
		}
	}
	return v
}

// summary is a value short enough for a name; one cut short ends with a hash of all of it.
func summary(v any) string {
	s, ok := v.(string)
	full := s
	if !ok {
		s, full = compactJSON(shown(v)), compactJSON(v)
	}
	if r := []rune(s); len(r) > 48 {
		return string(r[:40]) + "… #" + hash8(full)
	}
	return s
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// prepare works out how each call is matched.
func (b *builder) prepare(calls []captured) {
	for i := range calls {
		c := &calls[i]
		if c.request != nil {
			c.op = graphQL(b.s, c.method, c.uri, c.request, b.loose)
		}
		c.stateless = stateless(b.s.Stateless, c.uri)
	}
	shapes(calls)
	keys := map[string]map[string]bool{} // of each label
	for i := range calls {
		sp := b.spec(&calls[i])
		calls[i].spec = sp
		if keys[sp.label] == nil {
			keys[sp.label] = map[string]bool{}
		}
		keys[sp.label][sp.key] = true
	}
	// A name is the key of a mapping in an update: two calls matched otherwise never share one.
	for i := range calls {
		if sp := calls[i].spec; len(keys[sp.label]) > 1 {
			sp.label += " #" + hash8(sp.key)
		}
	}
}

func (b *builder) spec(c *captured) *spec {
	q := map[string]any{"method": c.method}
	weight := 0
	path, rawQuery, hasQuery := strings.Cut(c.uri, "?")
	segments := strings.Split(path, "/")
	patterns, shown := make([]string, len(segments)), make([]string, len(segments))
	patterned := false
	for i, s := range segments {
		patterns[i], shown[i] = regexp.QuoteMeta(s), s
		switch {
		case s == redacted:
			patterns[i], shown[i], patterned = `[^/]*`, "*", true
		case b.s.IgnoreHost && i == 1 && hostName.MatchString(s):
			patterns[i], shown[i], patterned = `[^/]+`, "*", true
		}
	}
	address := path
	if patterned {
		address = strings.Join(patterns, "/")
		q["urlPathPattern"] = address
	} else {
		q["urlPath"] = path
	}
	label := c.method + " " + strings.Join(shown, "/")
	key := label
	if hasQuery {
		values, err := url.ParseQuery(rawQuery)
		if err != nil { // not a query url can read: the whole address as it was
			delete(q, "urlPath")
			delete(q, "urlPathPattern")
			address = strings.ReplaceAll(regexp.QuoteMeta(c.uri), redacted, `[^&]*`)
			q["urlPattern"] = address
			label, key = c.method+" "+c.uri, c.method+" "+c.uri
		} else {
			ignored := func(name string) bool { return slices.Contains(b.s.IgnoreParams, name) }
			params, shownQuery, w := b.params("query", values, ignored)
			if len(params) > 0 {
				q["queryParameters"] = params
				label += "?" + shownQuery
				key += "?" + shownQuery
			}
			weight += w
		}
	}
	group := c.method + " " + address
	if r := c.request; r != nil && len(r.Headers) > 0 {
		headers := map[string]any{}
		for _, name := range sortedKeys(r.Headers) {
			var ms []any
			for _, v := range r.Headers[name] {
				m, s := b.value("header "+name, v)
				ms = append(ms, m)
				key += " " + name + ":" + s
			}
			headers[name] = ms[0]
			if len(ms) > 1 {
				headers[name] = map[string]any{"hasExactly": ms}
			}
			weight += len(ms)
		}
		q["headers"] = headers
	}
	if o := c.op; o != nil && !o.Batch && !o.Get {
		patterns, w := o.patterns()
		q["bodyPatterns"] = patterns
		weight += w
		label += " " + o.label()
		key += " " + o.label() + " " + canonical(o.Variables)
		group += " " + o.Name + "#" + o.Hash
	} else {
		if o != nil {
			label += " " + o.label()
		}
		if b.s.REST != RESTOff && c.request != nil {
			what, keyed, w := b.body(c, q)
			if what != "" {
				label += " " + what
				key += " " + keyed
			}
			weight += w
			if why := c.request.Unmatched; why != "" {
				b.unmatched[label] = why
			}
		}
	}
	return &spec{request: q, key: key, label: label, group: group, weight: weight}
}

// body puts the matcher of a REST body in the request, and says how it is shown and told apart.
func (b *builder) body(c *captured, q map[string]any) (shown, key string, weight int) {
	r := c.request
	if r.Body == "" {
		return "", "", 0
	}
	kind, _, _ := mime.ParseMediaType(r.ContentType)
	one := func(m map[string]any) { q["bodyPatterns"] = []map[string]any{m} }
	switch {
	case r.Base64:
		one(map[string]any{"binaryEqualTo": r.Body})
		return "binary #" + hash8(r.Body), "binary " + r.Body, 1
	case kind == "application/x-www-form-urlencoded":
		values, err := url.ParseQuery(r.Body)
		if err != nil {
			break
		}
		params, s, w := b.params("form", values, func(name string) bool { return b.ignoredBody(name, name) })
		if len(params) > 0 {
			q["formParameters"] = params
		}
		return summary(s), "form " + s, w
	case json.Valid([]byte(r.Body)):
		v, err := decodeJSON([]byte(r.Body))
		if err != nil {
			break
		}
		expected := b.shapeJSON(v, "", b.loose)
		var written any = expected
		if _, isText := expected.(string); isText || !notScalar(expected) {
			written = compactJSON(expected) // a text in equalToJson is one of JSON, to WireMock too
		}
		if c.op != nil && c.op.Batch { // its answers go in the order of its operations
			one(map[string]any{"equalToJson": written, "ignoreExtraElements": true})
			return summary(expected), compactJSON(expected), nodes(expected)
		}
		one(map[string]any{"equalToJson": written, "ignoreArrayOrder": true, "ignoreExtraElements": true})
		return summary(expected), canonical(expected), nodes(expected)
	case strings.Contains(kind, "xml") && !hides(r.Body):
		if _, err := xmlTokens(r.Body); err == nil {
			one(map[string]any{"equalToXml": r.Body})
			return "xml #" + hash8(r.Body), "xml " + r.Body, 1
		}
	}
	if hides(r.Body) {
		one(map[string]any{"matches": hidden(r.Body)})
	} else {
		one(map[string]any{"equalTo": r.Body})
	}
	return summary(r.Body), "text " + r.Body, 1
}

// state is a stretch of the capture in which each read is always answered the same.
type state struct {
	by      *captured            // the call that began it; nil if the backend changed by itself
	first   captured             // what showed that, if so
	answers map[string]*captured // of each read seen in it
}

// rule is a mapping as it is written: Rule can't tell what was left out.
type rule struct {
	Name     string         `json:"name"`
	Priority *int           `json:"priority,omitempty"`
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
	since := map[string]int{} // how many of them there were when a read was last seen
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

// answerIn is a read's in a state: the one seen, or the last before it, or the first after.
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

// keptVersion is that of the calls kept: 2 since what the app sent is kept.
const keptVersion = 2

// callsFile keeps the calls of a capture as they were, so that another can go on from it.
const callsFile = "capture.calls.json"

type keptCapture struct {
	Version  int      `json:"version"` // 2 since requests are kept; none before
	Settings Settings `json:"settings"`
	Calls    []kept   `json:"calls"`
	Files    []string `json:"files"` // in __files, written from them
}

// readKept reads the calls kept in a folder; nil if there are none.
func readKept(folder string) (*keptCapture, error) {
	data, err := os.ReadFile(filepath.Join(folder, callsFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var k keptCapture
	if err := json.Unmarshal(data, &k); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.ToSlash(filepath.Join(folder, callsFile)), err)
	}
	return &k, nil
}

// Continue goes on with the capture in a folder: its calls come before those made from now on, as if
// in one go, and SaveCapture writes all of them there again. Its settings are kept, but for those given.
func (m *Mock) Continue(folder string) (int, error) {
	k, err := readKept(folder)
	if err != nil {
		return 0, err
	}
	if k == nil {
		return 0, fmt.Errorf("%s has no %s to go on from: it isn't a capture, or one of rayo before 1.0.5", filepath.ToSlash(folder), callsFile)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	before := make([]captured, 0, len(k.Calls)+len(m.real.captured))
	for _, c := range k.Calls {
		before = append(before, c.toCaptured(k.Version))
	}
	m.real.captured = append(before, m.real.captured...)
	m.real.earlier = k.Files
	m.real.settings = m.real.settings.Or(k.Settings)
	return len(k.Calls), nil
}

// SaveCapture writes a folder of mocks, and returns what to say about it.
func (m *Mock) SaveCapture(folder string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return saveCaptured(folder, slices.Clone(m.real.captured), m.real.settings, m.real.earlier, m.real.skipped, func(files []string) { m.real.earlier = files })
}

func saveCaptured(folder string, calls []captured, given Settings, earlier, skipped []string, written func([]string)) ([]string, error) {
	var notes []string
	s := given.Or(Defaults)
	if len(calls) == 0 {
		notes = append(notes, "nothing captured: no call of the app was answered")
	} else {
		for _, name := range earlier { // written again from all the calls
			if err := os.Remove(filepath.Join(folder, "__files", name)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
		}
		b := &builder{s: s, loosened: map[string]bool{}, unmatched: map[string]string{}}
		states, files, err := writeMocks(folder, calls, b)
		if err != nil {
			return nil, err
		}
		k := keptCapture{Version: keptVersion, Settings: s, Files: files}
		for _, c := range calls {
			k.Calls = append(k.Calls, c.toKept())
		}
		if err := writeJSON(filepath.Join(folder, callsFile), k); err != nil {
			return nil, err
		}
		written(files)
		notes = report(folder, calls, states, b)
	}
	if len(skipped) > 0 {
		notes = append(notes, "not captured:")
	}
	for _, s := range skipped {
		notes = append(notes, "  "+s)
	}
	if len(calls) == 0 {
		return notes, nil // so that the folder is free for another try
	}
	return notes, os.WriteFile(filepath.Join(folder, "capture.txt"), []byte(strings.Join(notes, "\n")+"\n"), 0o644)
}

// ranks put the heavier of the specs that may overlap first: a call is answered by the closest.
func ranks(calls []captured) map[*spec]int {
	weights := map[string][]int{}
	for _, c := range calls {
		if w := weights[c.spec.group]; !slices.Contains(w, c.spec.weight) {
			weights[c.spec.group] = append(w, c.spec.weight)
		}
	}
	out := map[*spec]int{}
	for _, c := range calls {
		w := weights[c.spec.group]
		slices.Sort(w)
		slices.Reverse(w)
		out[c.spec] = slices.Index(w, c.spec.weight)
	}
	return out
}

func writeMocks(folder string, calls []captured, b *builder) ([]*state, []string, error) {
	b.prepare(calls)
	rank := ranks(calls)
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
	add := func(c *captured, besides bool, required, next string) error {
		response, err := files.response(c)
		// 5, WireMock's default, for the closest: a mapping written by hand with no priority, loaded after,
		// wins over all of them.
		priority := 5 + 2*rank[c.spec]
		if besides {
			priority++
		}
		r := rule{Name: c.spec.label, Required: required, Next: next, Request: c.spec.request, Response: response}
		if priority != 5 {
			r.Priority = &priority
		}
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
				if err := add(answerIn(states, i, e), false, stateName(i), ""); err != nil {
					return nil, nil, err
				}
			}
			continue
		}
		besides := false
		for i := range states {
			next, reachable := nextOf(states, i, e)
			if next == 0 {
				break
			}
			besides = true // its answer whenever it is called besides
			to := ""
			if reachable {
				to = stateName(next)
			}
			if reachable || vary[e] {
				if err := add(states[next].by, false, stateName(i), to); err != nil {
					return nil, nil, err
				}
			}
		}
		if err := add(c, besides, "", ""); err != nil {
			return nil, nil, err
		}
	}
	written := slices.Sorted(maps.Values(files.names))
	return states, written, writeJSON(filepath.Join(folder, "mappings", "capture.json"), map[string]any{"mappings": rules})
}

func report(folder string, calls []captured, states []*state, b *builder) []string {
	notes := []string{
		fmt.Sprintf("captured %d calls, in %d states: %s", len(calls), len(states), filepath.ToSlash(folder)),
		"to use it, set mock in the flow's header to its path, relative to the flow",
		"read it before committing: what looks private is hidden, but not all that is may look so",
	}
	if len(b.loosened) > 0 {
		notes = append(notes, "matched by their form, or not at all, as they change from run to run: "+strings.Join(slices.Sorted(maps.Keys(b.loosened)), ", "))
	}
	for _, label := range slices.Sorted(maps.Keys(b.unmatched)) {
		notes = append(notes, fmt.Sprintf("the body isn't matched (%s): %s", b.unmatched[label], label))
	}
	binaries := 0
	for _, c := range calls {
		if c.binary {
			binaries++
		}
	}
	if binaries > 0 {
		notes = append(notes, fmt.Sprintf("%d answers aren't text, and are kept as they were: nothing in them is hidden", binaries))
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
		notes = append(notes, fmt.Sprintf("  - mockState: { %s: %d }   # %s  (%s)", scenario, i, s.first.step, s.first.spec.label))
	}
	return notes
}

// bodies are kept one file each, only once.
type bodies struct {
	folder string
	names  map[string]string
}

var notInNames = regexp.MustCompile(`[^A-Za-z0-9]+`)

func (b *bodies) response(c *captured) (map[string]any, error) {
	r := map[string]any{"status": c.status}
	h := map[string]any{}
	if c.contentType != "" {
		h["Content-Type"] = c.contentType
	}
	for name, values := range c.headers {
		if len(values) == 1 {
			h[name] = values[0]
		} else {
			h[name] = values
		}
	}
	if len(h) > 0 {
		r["headers"] = h
	}
	if c.body == "" {
		return r, nil
	}
	data := []byte(c.body)
	if c.binary {
		var err error
		if data, err = base64.StdEncoding.DecodeString(c.body); err != nil {
			return nil, err
		}
	} else if kind, _, _ := mime.ParseMediaType(c.contentType); !strings.HasSuffix(kind, "json") && strings.Contains(c.body, "{{") {
		r["transformers"] = []string{} // as it is, also in WireMock
	}
	same := fmt.Sprint(c.binary, c.body) // answers with the same body share a file
	name, ok := b.names[same]
	if !ok {
		path, _, _ := strings.Cut(c.uri, "?")
		path = strings.TrimRight(path, "/")
		if c.op != nil && c.op.Name != "" && !c.op.Batch {
			path = c.op.Name
		}
		what := strings.Trim(notInNames.ReplaceAllString(path[strings.LastIndex(path, "/")+1:], "_"), "_")
		name = fmt.Sprintf("%03d_%s_%.40s%s", len(b.names)+1, strings.ToLower(c.method), what, extension(c.contentType, c.binary))
		b.names[same] = name
		if err := os.MkdirAll(b.folder, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(b.folder, name), data, 0o644); err != nil {
			return nil, err
		}
	}
	r["bodyFileName"] = name
	return r, nil
}

func extension(contentType string, binary bool) string {
	kind, _, _ := mime.ParseMediaType(contentType)
	switch {
	case strings.HasSuffix(kind, "json"):
		return ".json"
	case strings.HasSuffix(kind, "xml"):
		return ".xml"
	case kind == "text/html":
		return ".html"
	case !binary:
		return ".txt"
	}
	if list, _ := mime.ExtensionsByType(kind); len(list) > 0 {
		return list[0]
	}
	return ".bin"
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
