// Package mock is a pretend backend. It knows nothing of flows or devices.
package mock

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/dop251/goja"
)

// Mock answers as the backend during one flow. Its files are WireMock's, or the part of them listed
// in the reference: anything else is an error.
type Mock struct {
	server *http.Server

	mu       sync.Mutex
	rules    []*Rule
	folders  []string // with bodies, newest first
	states   map[string]string
	network  Network
	calls    []*Call
	err      error
	vm       *goja.Runtime
	real     *real     // nil unless capturing
	unmocked *Unmocked // calls without a mock that are answered anyway; nil for none
	loaded   []string  // each folder or file loaded, with how many mappings it had
}

// Network is how the connection to the backend behaves.
type Network struct {
	Offline bool
	Delay   time.Duration
	Fail    int
	Path    *regexp.Regexp // nil for every call
}

// Call is one the app made.
type Call struct {
	At           time.Time
	Method, Path string
	URI, Body    string
	Outcome      string
	Unmocked     bool
	Ignored      bool   // not mocked, but answered as Unmocked says: it doesn't fail the flow
	Mapping      string // the one that answered: its name, and where it is written
	Scenario     string // what it did to a scenario, such as "booking: 20 -> 21"
	awaited      bool   // already matched by an Await
}

// MarshalJSON gives what is worth telling of it.
func (c Call) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	err := e.Encode(jsonCall{c.At.UTC().Format(callTime), c.Method, c.URI, c.Outcome, !c.Unmocked, c.Mapping, c.Scenario, c.Body})
	return b.Bytes(), err
}

// UnmarshalJSON reads what MarshalJSON writes.
func (c *Call) UnmarshalJSON(data []byte) error {
	var j jsonCall
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	at, err := time.Parse(callTime, j.At)
	*c = Call{At: at, Method: j.Method, URI: j.URI, Outcome: j.Outcome, Unmocked: !j.Mocked, Mapping: j.Mapping, Scenario: j.Scenario, Body: j.Body}
	return err
}

const callTime = "2006-01-02T15:04:05.000Z"

// Shown is how it is written.
func (c Call) Shown() any { return jsonCall{} }

type jsonCall struct {
	At       string `json:"at" about:"In UTC"`
	Method   string `json:"method" about:""`
	URI      string `json:"uri" about:""`
	Outcome  string `json:"outcome" about:"The status, or what happened"`
	Mocked   bool   `json:"mocked" about:"Whether a mock fitted"`
	Mapping  string `json:"mapping" about:"The one that answered: its name, or method and address, and file#n where it is written; empty if none"`
	Scenario string `json:"scenario" about:"Of a mapping with a scenario, its state before and after, such as 'booking: 20 -> 21'; empty if none"`
	Body     string `json:"body" about:"Only in a requests.json"`
}

func (c Call) String() string {
	s := fmt.Sprintf("%s %s %s -> %s", c.At.Format("15:04:05.000"), c.Method, c.URI, c.Outcome)
	if c.Mapping != "" {
		s += "  by " + c.Mapping
	}
	if c.Scenario != "" {
		s += "  [" + c.Scenario + "]"
	}
	return s
}

func New() *Mock {
	return &Mock{states: map[string]string{}, vm: goja.New()}
}

// Load adds a folder (mappings/ and __files/) or a file of mappings.
func (m *Mock) Load(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	files, root := []string{path}, filepath.Dir(path)
	if info.IsDir() {
		files, root = nil, path
		err := filepath.WalkDir(filepath.Join(path, "mappings"), func(file string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && filepath.Ext(file) == ".json" {
				files = append(files, file)
			}
			return err
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) { // without mappings/ is said below
			return err
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.folders = append([]string{filepath.Join(root, "__files")}, m.folders...)
	had := len(m.rules)
	defer func() {
		m.loaded = append(m.loaded, fmt.Sprintf("%s (%d)", filepath.ToSlash(path), len(m.rules)-had))
	}()
	if len(files) == 0 {
		return fmt.Errorf("no mappings in %s: a folder of mocks has them in mappings/*.json", filepath.ToSlash(path))
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		where := filepath.ToSlash(file)
		var top map[string]json.RawMessage
		if err := json.Unmarshal(data, &top); err != nil {
			return fmt.Errorf("%s: %w", where, err)
		}
		list := []json.RawMessage{data}
		if several, ok := top["mappings"]; ok {
			if len(top) > 1 {
				return fmt.Errorf("%s: expected only mappings", where)
			}
			list = nil
			if err := json.Unmarshal(several, &list); err != nil {
				return fmt.Errorf("%s: %w", where, err)
			}
		}
		for i, one := range list {
			r, err := ParseRule(one)
			if err == nil {
				r.source = where
				if len(list) > 1 {
					r.source = fmt.Sprintf("%s#%d", where, i+1)
				}
				err = m.add(r)
			}
			if err != nil {
				return fmt.Errorf("%s: mapping %d: %w", where, i+1, err)
			}
		}
	}
	return nil
}

// Add adds a rule; of the same priority, the last one wins.
func (m *Mock) Add(r *Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.add(r)
}

func (m *Mock) add(r *Rule) error {
	if name := r.Response.BodyFileName; name != "" {
		var data []byte
		err := errors.New("no folder")
		for _, folder := range m.folders {
			if data, err = os.ReadFile(filepath.Join(folder, filepath.FromSlash(name))); err == nil {
				break
			}
		}
		if err != nil {
			return fmt.Errorf("can't read the body %s: %w", name, err)
		}
		templating := r.Response.templating()
		if !utf8.Valid(data) { // an image, say: never a template
			templating = templatedNot
		}
		if r.body, err = parseTemplate(string(data), templating); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	m.rules = append(m.rules, r)
	return nil
}

// Hosts are those the mocks answer for: where a call's path begins with /host/.
func (m *Mock) Hosts() map[string]bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.hosts()
}

func (m *Mock) hosts() map[string]bool {
	hosts := map[string]bool{}
	for _, r := range m.rules {
		if h := r.Request.Host; h != nil && h.EqualTo != nil {
			hosts[strings.ToLower(*h.EqualTo)] = true
		}
		for _, u := range []string{r.Request.Url, r.Request.UrlPath, r.Request.UrlPattern, r.Request.UrlPathPattern, r.Request.UrlPathTemplate} {
			u = strings.ReplaceAll(strings.TrimPrefix(u, "^"), `\.`, ".")
			if host, _, _ := strings.Cut(strings.TrimPrefix(u, "/"), "/"); hostName.MatchString(host) {
				hosts[strings.ToLower(host)] = true
			}
		}
	}
	return hosts
}

// Loaded is each folder or file of mappings loaded, with how many it had, such as "mocks/a (12)".
func (m *Mock) Loaded() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.loaded)
}

// Rules is how many mappings there are.
func (m *Mock) Rules() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.rules)
}

// Why is what was wrong with a call that isn't mocked, beyond that: the mapping nearest to it, or that
// its body was too big; empty if nothing.
func (c *Call) Why() string {
	if strings.HasPrefix(c.Outcome, "413") {
		return c.Outcome
	}
	if _, near, ok := strings.Cut(c.Outcome, "; nearest "); ok {
		return "nearest " + near
	}
	return ""
}

// What is the call as said when it isn't mocked: with its GraphQL operation, if it has one.
func (c *Call) What() string {
	if op := OperationOf(c.Body); op != "" && c.Method == "POST" {
		return c.Method + " " + c.URI + " (GraphQL operation " + op + ")"
	}
	return c.Method + " " + c.URI
}
