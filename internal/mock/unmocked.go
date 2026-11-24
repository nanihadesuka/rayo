package mock

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
)

// Unmocked says which calls that no mock answers don't fail the flow, and how they are answered.
type Unmocked struct {
	Ignore []Ignored
	Status int    // 0 for 404
	Body   string // empty for the usual "not mocked: ..."
}

// Ignored is calls not worth mocking: of a host, of a path, or GraphQL operations. What is empty
// matches anything; all that is said must match.
type Ignored struct {
	Host    string   // and its subdomains
	Path    string   // a glob, as --stateless: * any text but /, ** any
	GraphQL []string // operation names
	path    *regexp.Regexp
}

// ParseIgnored reads one written as --ignore-unmocked takes it: graphql:Op, or host/path, host alone, or
// /path for any host. A path may have * and **.
func ParseIgnored(text string) (Ignored, error) {
	text = strings.TrimSpace(text)
	if op, ok := strings.CutPrefix(text, "graphql:"); ok {
		if op == "" {
			return Ignored{}, fmt.Errorf("graphql: needs an operation, as graphql:Geocode")
		}
		return Ignored{GraphQL: []string{op}}, nil
	}
	if text == "" {
		return Ignored{}, fmt.Errorf("takes graphql:Operation, host/path, host or /path")
	}
	var i Ignored
	if !strings.HasPrefix(text, "/") {
		host, path, _ := strings.Cut(text, "/")
		i.Host = host
		if path != "" {
			i.Path = "/" + path
		}
	} else {
		i.Path = text
	}
	return i, i.compile()
}

func (i *Ignored) compile() error {
	if i.Host == "" && i.Path == "" && len(i.GraphQL) == 0 {
		return fmt.Errorf("an ignored call needs a host, a path or graphql")
	}
	if i.Host != "" && !hostName.MatchString(i.Host) {
		return fmt.Errorf("not a host: %q", i.Host)
	}
	if i.Path != "" {
		g, err := Glob(i.Path)
		if err != nil {
			return err
		}
		i.path = g
	}
	return nil
}

// Check makes it ready, and is an error if it can't be used.
func (u *Unmocked) Check() error {
	for k := range u.Ignore {
		if err := u.Ignore[k].compile(); err != nil {
			return err
		}
	}
	if u.Status != 0 && (u.Status < 100 || u.Status > 599) {
		return fmt.Errorf("status takes 100 to 599, not %d", u.Status)
	}
	return nil
}

// Merge is u with what other adds: its calls too, and its status and body where it says them.
func (u *Unmocked) Merge(other *Unmocked) *Unmocked {
	if other == nil {
		return u
	}
	if u == nil {
		return other
	}
	m := &Unmocked{Ignore: slices.Concat(u.Ignore, other.Ignore), Status: u.Status, Body: u.Body}
	if other.Status != 0 {
		m.Status = other.Status
	}
	if other.Body != "" {
		m.Body = other.Body
	}
	return m
}

// ignores says whether a call is one of those ignored. Through the proxy, the path begins with the host.
func (u *Unmocked) ignores(q *http.Request, body string) bool {
	if u == nil {
		return false
	}
	path, host := q.URL.Path, q.Host
	if h, rest, ok := strings.Cut(strings.TrimPrefix(path, "/"), "/"); ok && hostName.MatchString(h) {
		host, path = h, "/"+rest
	}
	host, _, _ = strings.Cut(host, ":")
	op := ""
	if q.Method == http.MethodPost {
		op = OperationOf(body)
	}
	for _, i := range u.Ignore {
		switch {
		case i.Host != "" && !Matches(host, []string{i.Host}):
		case i.path != nil && !i.path.MatchString(path):
		case len(i.GraphQL) > 0 && !slices.Contains(i.GraphQL, op):
		default:
			return true
		}
	}
	return false
}

// SetUnmocked says which calls without a mock are answered anyway, and don't fail the flow.
func (m *Mock) SetUnmocked(u *Unmocked) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unmocked = u
}
