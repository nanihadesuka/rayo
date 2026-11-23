package mock

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

// GraphQL: every call is POST /graphql, so a capture tells them apart by their operation, and
// queries are taken as reads: only mutations move the scenario on. What a mock matches is written with
// WireMock's own matchers, so WireMock reads the mocks as rayo does.

// GraphQL modes.
const (
	GraphQLOff         = "off"
	GraphQLOperation   = "operation"
	GraphQLVariables   = "operation+variables"
	DefaultGraphQLPath = `.*/graphql`
)

// operation is what a GraphQL call is told apart by. Only what has nothing private is kept.
type operation struct {
	Name      string `json:"name,omitempty"`      // operationName, or the name in the query
	Named     bool   `json:"named,omitempty"`     // the body has operationName
	Hash      string `json:"hash,omitempty"`      // of a persisted query
	Mutation  bool   `json:"mutation,omitempty"`  // it changes something
	Shape     string `json:"shape,omitempty"`     // the query, on one line: the same name may ask for other fields
	Query     bool   `json:"query,omitempty"`     // the body has the query, and not only its hash
	Variables any    `json:"variables,omitempty"` // as matched, for operation+variables
	Batch     bool   `json:"batch,omitempty"`     // several operations in one call: matched by its whole body
	Get       bool   `json:"get,omitempty"`       // sent in the address: matched by it

	distinct   bool // others of its name have another shape, so it is told apart by its own
	hashedOnly bool // others of its name come with the query, so that it is told apart by having one
}

// gqlSpaces are what doesn't count between the words of a query.
var gqlSpaces = regexp.MustCompile(`[\s,]+`)

func oneLine(query string) string { return strings.TrimSpace(gqlSpaces.ReplaceAllString(query, " ")) }

// shapes marks the operations whose name comes with more than one query, or with and without one.
func shapes(calls []captured) {
	seen, queried, hashed := map[string]map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, c := range calls {
		if o := c.op; o != nil && !o.Batch && !o.Get {
			if o.Shape != "" {
				if seen[o.Name] == nil {
					seen[o.Name] = map[string]bool{}
				}
				seen[o.Name][o.Shape] = true
			}
			if o.Query {
				queried[o.Name] = true
			} else {
				hashed[o.Name] = true
			}
		}
	}
	for i := range calls {
		if o := calls[i].op; o != nil && !o.Batch && !o.Get {
			copied := *o
			copied.distinct = len(seen[o.Name]) > 1
			copied.hashedOnly = queried[o.Name] && hashed[o.Name]
			calls[i].op = &copied
		}
	}
}

// definition is an operation of a query document.
type definition struct{ kind, name string }

// definitions reads the operations of a document, past strings, comments and fragments.
func definitions(doc string) []definition {
	var out []definition
	depth, pending := 0, ""
	for i := 0; i < len(doc); {
		c := doc[i]
		switch {
		case c == '#':
			for i < len(doc) && doc[i] != '\n' {
				i++
			}
			continue
		case strings.HasPrefix(doc[i:], `"""`):
			end := strings.Index(doc[i+3:], `"""`)
			if end < 0 {
				return out
			}
			i += end + 6
			continue
		case c == '"':
			i++
			for i < len(doc) && doc[i] != '"' {
				if doc[i] == '\\' {
					i++
				}
				i++
			}
			i++
			continue
		case c == '{':
			if depth == 0 && pending == "" {
				out = append(out, definition{kind: "query"}) // the short form
			}
			depth++
			pending = ""
		case c == '}':
			depth--
		case depth == 0 && (c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'):
			j := i
			for j < len(doc) && (doc[j] == '_' || doc[j] >= 'A' && doc[j] <= 'Z' || doc[j] >= 'a' && doc[j] <= 'z' || doc[j] >= '0' && doc[j] <= '9') {
				j++
			}
			word := doc[i:j]
			switch {
			case pending == "query" || pending == "mutation" || pending == "subscription":
				out[len(out)-1].name = word
				pending = "named"
			case word == "query" || word == "mutation" || word == "subscription":
				out = append(out, definition{kind: word})
				pending = word
			case word == "fragment":
				pending = "fragment"
			}
			i = j
			continue
		case depth == 0 && c == '(':
			if pending != "fragment" { // the variables: none of the words in them is the name
				for i < len(doc) && doc[i] != ')' {
					i++
				}
			}
		}
		i++
	}
	return out
}

// chosen is the operation a call runs: the one operationName names, else the only one.
func chosen(doc, name string) (definition, bool) {
	defs := definitions(doc)
	for _, d := range defs {
		if name != "" && d.name == name {
			return d, true
		}
	}
	if len(defs) == 1 && name == "" {
		return defs[0], true
	}
	if len(defs) > 0 && name == "" {
		return defs[0], true
	}
	return definition{}, false
}

// graphQL reads the operation of a call, or nil if it isn't a GraphQL one. The body is as kept: without
// what is private.
func graphQL(s Settings, method, uri string, req *request, loose func(string)) *operation {
	if s.GraphQL == GraphQLOff || req == nil {
		return nil
	}
	path, query, _ := strings.Cut(uri, "?")
	pathPattern, _ := Pattern(s.GraphQLPath)
	if method == "GET" {
		v, err := url.ParseQuery(query)
		if err != nil || v.Get("query") == "" && v.Get("operationName") == "" && v.Get("extensions") == "" {
			return nil
		}
		o := operationOf(map[string]any{"query": v.Get("query"), "operationName": v.Get("operationName")})
		if o == nil {
			o = &operation{}
		}
		o.Get, o.Mutation = true, false // a mutation is never sent with GET
		return o
	}
	if method != "POST" || req.Base64 || req.Body == "" {
		return nil
	}
	v, err := decodeJSON([]byte(req.Body))
	if err != nil {
		return nil
	}
	if list, ok := v.([]any); ok { // a batch
		var names []string
		o := &operation{Batch: true}
		for _, one := range list {
			b, _ := one.(map[string]any)
			each := operationOf(b)
			if each == nil {
				return nil
			}
			names = append(names, each.label())
			o.Mutation = o.Mutation || each.Mutation
		}
		o.Name = strings.Join(names, "+")
		return o
	}
	b, _ := v.(map[string]any)
	if b == nil {
		return nil
	}
	_, hasQuery := b["query"]
	_, hasName := b["operationName"]
	if !hasQuery && !hasName && hashOf(b) == "" && (pathPattern == nil || !pathPattern.MatchString(path)) {
		return nil
	}
	o := operationOf(b)
	if o == nil {
		return nil
	}
	if s.GraphQL == GraphQLVariables {
		o.Variables = variablesOf(s, b["variables"], loose)
	}
	return o
}

func hashOf(b map[string]any) string {
	ext, _ := b["extensions"].(map[string]any)
	pq, _ := ext["persistedQuery"].(map[string]any)
	hash, _ := pq["sha256Hash"].(string)
	return hash
}

// operationOf is the operation of one body, or nil if there is nothing to tell it apart by.
func operationOf(b map[string]any) *operation {
	query, _ := b["query"].(string)
	name, _ := b["operationName"].(string)
	o := &operation{Name: name, Named: name != "", Query: query != ""}
	if d, ok := chosen(query, name); ok {
		o.Mutation = d.kind == "mutation"
		if o.Name == "" {
			o.Name = d.name
		}
	} else if query == "" && strings.HasPrefix(strings.ToLower(name), "mutation") {
		o.Mutation = true // only a hash: its name is all there is to go by
	}
	if shape := oneLine(query); shape != "" && !hides(shape) {
		o.Shape = shape // a query with something private in it is told apart by its name only
	}
	if o.Name == "" {
		o.Hash = hashOf(b)
	}
	if o.Name == "" && o.Hash == "" && o.Shape == "" {
		return nil // nothing to tell it apart by: as any other call
	}
	return o
}

// variablesOf are the variables as matched: those that aren't private, nor to be ignored.
func variablesOf(s Settings, v any, loose func(string)) any {
	switch v := v.(type) {
	case map[string]any:
		matched := shapeJSON(s, v, "", func(what string) {
			if loose != nil {
				loose("variables " + strings.TrimPrefix(what, "body "))
			}
		})
		if m, _ := matched.(map[string]any); len(m) == 0 {
			return nil
		}
		return matched
	case string: // sent as a text of JSON
		if v != "" && !hides(v) {
			return v
		}
	}
	return nil
}

// label tells the operation apart in names.
func (o *operation) label() string {
	l := "graphql:" + o.Name
	switch {
	case o.Name == "" && o.Hash != "":
		l = "graphql:#" + o.Hash[:min(len(o.Hash), 12)]
	case o.Name == "":
		l = "graphql:anonymous #" + hash8(o.Shape)
	}
	if o.distinct && o.Name != "" {
		l += " #" + hash8(o.Shape)
	}
	if o.hashedOnly && !o.Query {
		l += " (hash only)"
	}
	if o.Variables != nil {
		l += " (" + summary(o.Variables) + ")"
	}
	return l
}

func hash8(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:4])
}

// shapePattern matches a query of this shape, whatever the spaces and commas between its words.
func shapePattern(shape string) string {
	words := strings.Split(shape, " ")
	for i, w := range words {
		words[i] = regexp.QuoteMeta(w)
	}
	return `[\s,]*` + strings.Join(words, `[\s,]+`) + `[\s,]*`
}

// patterns are the bodyPatterns of its calls, and how much they match.
func (o *operation) patterns() ([]map[string]any, int) {
	at := func(expression string, m map[string]any) map[string]any {
		m["expression"] = expression
		return map[string]any{"matchesJsonPath": m}
	}
	var list []map[string]any
	switch {
	case o.Named:
		list = append(list, at("$.operationName", map[string]any{"equalTo": o.Name}))
	case o.Name != "":
		list = append(list, at("$.query", map[string]any{"matches": `.*\b(query|mutation|subscription)\s+` + o.Name + `\b.*`}))
	case o.Hash != "":
		list = append(list, at("$.extensions.persistedQuery.sha256Hash", map[string]any{"equalTo": o.Hash}))
	}
	if o.Shape != "" && (o.distinct || o.Name == "" && o.Hash == "") {
		list = append(list, at("$.query", map[string]any{"matches": shapePattern(o.Shape)}))
	}
	if o.hashedOnly && o.Query {
		list = append(list, map[string]any{"matchesJsonPath": "$.query"})
	}
	weight := len(list)
	switch v := o.Variables.(type) {
	case nil:
	case string:
		list = append(list, at("$.variables", map[string]any{"equalTo": v}))
		weight++
	default:
		list = append(list, at("$.variables", map[string]any{"equalToJson": v, "ignoreArrayOrder": true, "ignoreExtraElements": true}))
		weight += nodes(v)
	}
	return list, weight
}

// OperationOf is the GraphQL operation of a call's body, to say which one wasn't mocked.
func OperationOf(body string) string {
	var b map[string]any
	if json.Unmarshal([]byte(body), &b) != nil {
		return ""
	}
	if o := operationOf(b); o != nil {
		return o.Name
	}
	return ""
}
