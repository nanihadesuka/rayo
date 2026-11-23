package mock

import (
	"crypto/sha256"
	"encoding/hex"
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

// GraphQL: every call is POST /graphql, so a capture tells them apart by their operation, and
// queries are taken as reads: only mutations move the scenario on.

// GraphQL modes.
const (
	GraphQLOff         = "off"
	GraphQLOperation   = "operation"
	GraphQLVariables   = "operation+variables"
	DefaultGraphQLPath = `.*/graphql`
)

// operation is what a GraphQL call is told apart by. Only what has nothing private is kept.
type operation struct {
	Name     string `json:"name,omitempty"`     // operationName, or the name in the query
	Named    bool   `json:"named,omitempty"`    // the body has operationName
	Hash     string `json:"hash,omitempty"`     // of a persisted query, when there is no name
	Mutation bool   `json:"mutation,omitempty"` // it changes something
	Vars     string `json:"vars,omitempty"`     // conditions on its variables, for operation+variables
	Shape    string `json:"shape,omitempty"`    // the query, on one line: the same name may ask for other fields

	distinct bool // others of its name have another shape, so it is told apart by its own
}

// oneLine is a query as compared: whatever the spaces and commas between its words.
var gqlSpaces = regexp.MustCompile(`[\s,]+`)

func oneLine(query string) string { return strings.TrimSpace(gqlSpaces.ReplaceAllString(query, " ")) }

// shapes marks the operations whose name comes with more than one query.
func shapes(calls []captured) {
	seen := map[string]map[string]bool{}
	for _, c := range calls {
		if c.op != nil && c.op.Name != "" && c.op.Shape != "" {
			if seen[c.op.Name] == nil {
				seen[c.op.Name] = map[string]bool{}
			}
			seen[c.op.Name][c.op.Shape] = true
		}
	}
	for i := range calls {
		if o := calls[i].op; o != nil && len(seen[o.Name]) > 1 {
			copied := *o
			copied.distinct = true
			calls[i].op = &copied
		}
	}
}

var (
	gqlOperation = regexp.MustCompile(`^\s*(?:#[^\n]*\n\s*)*(query|mutation|subscription)\b\s*([_A-Za-z][_0-9A-Za-z]*)?`)
	jsIdentifier = regexp.MustCompile(`^[_A-Za-z][_0-9A-Za-z]*$`)
)

// graphQL reads the operation of a call, or nil if it isn't a GraphQL one.
func (r *real) graphQL(method, path string, body []byte) *operation {
	if r.graphql == GraphQLOff || method != "POST" {
		return nil
	}
	var b map[string]any
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	if d.Decode(&b) != nil || b == nil {
		return nil
	}
	query, _ := b["query"].(string)
	name, _ := b["operationName"].(string)
	hash := ""
	if ext, ok := b["extensions"].(map[string]any); ok {
		if pq, ok := ext["persistedQuery"].(map[string]any); ok {
			hash, _ = pq["sha256Hash"].(string)
		}
	}
	_, hasQuery := b["query"]
	_, hasName := b["operationName"]
	if !hasQuery && !hasName && hash == "" && !r.graphqlPath.MatchString(path) {
		return nil
	}
	o := &operation{Name: name, Named: name != ""}
	if m := gqlOperation.FindStringSubmatch(query); m != nil {
		o.Mutation = m[1] == "mutation"
		if o.Name == "" {
			o.Name = m[2]
		}
	} else if query == "" && strings.HasPrefix(strings.ToLower(name), "mutation") {
		o.Mutation = true // only a hash: its name is all there is to go by
	}
	if o.Name == "" {
		o.Hash = hash
	}
	if shape := oneLine(query); shape != "" && r.cleanText(shape) == shape && !strings.Contains(shape, ")]") {
		o.Shape = shape // a query with something private in it is told apart by its name only
	}
	if o.Name == "" && o.Hash == "" {
		return nil // nothing to tell it apart by: as any other call
	}
	if r.graphql == GraphQLVariables {
		var conditions []string
		r.varConditions("@.variables", b["variables"], &conditions)
		o.Vars = strings.Join(conditions, " && ")
	}
	return o
}

// varConditions are on what of the variables isn't private: on each value as it is.
func (r *real) varConditions(at string, v any, out *[]string) {
	switch v := v.(type) {
	case map[string]any:
		for _, k := range slices.Sorted(maps.Keys(v)) {
			if private(k) {
				continue
			}
			child := at + "['" + k + "']"
			if jsIdentifier.MatchString(k) {
				child = at + "." + k
			}
			r.varConditions(child, v[k], out)
		}
	case []any:
		*out = append(*out, fmt.Sprintf("%s.length == %d", at, len(v)))
		for i, child := range v {
			r.varConditions(fmt.Sprintf("%s[%d]", at, i), child, out)
		}
	case string:
		if r.cleanText(v) != v || strings.Contains(v, ")]") {
			return
		}
		literal, _ := json.Marshal(v)
		*out = append(*out, at+" == "+string(literal))
	case json.Number:
		*out = append(*out, at+" == "+v.String())
	case bool:
		*out = append(*out, fmt.Sprintf("%s == %t", at, v))
	case nil:
		*out = append(*out, at+" == null")
	}
}

// label tells the operation apart in names and endpoints.
func (o *operation) label() string {
	l := "graphql:" + o.Name
	if o.Name == "" {
		l = "graphql:#" + o.Hash[:min(len(o.Hash), 12)]
	}
	if o.distinct {
		sum := sha256.Sum256([]byte(o.Shape))
		l += " #" + hex.EncodeToString(sum[:4])
	}
	if o.Vars != "" {
		l += " (" + o.Vars + ")"
	}
	return l
}

// condition is the JSONPath that matches the body of its calls.
func (o *operation) condition() string {
	var c string
	switch {
	case o.Named:
		c = "@.operationName == " + quote(o.Name)
	case o.Name != "":
		c = "/^\\s*(#[^\\n]*\\n\\s*)*(query|mutation|subscription)\\s+" + o.Name + "\\b/.test(@.query)"
	default:
		c = "@.extensions.persistedQuery.sha256Hash == " + quote(o.Hash)
	}
	if o.distinct {
		c += ` && @.query.replace(/[\s,]+/g, ' ').trim() == ` + quote(o.Shape)
	}
	if o.Vars != "" {
		c += " && " + o.Vars
	}
	return "$[?(" + c + ")]"
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// OperationOf is the GraphQL operation of a call's body, to say which one wasn't mocked.
func OperationOf(body string) string {
	var b struct {
		OperationName string `json:"operationName"`
		Query         string `json:"query"`
	}
	if json.Unmarshal([]byte(body), &b) != nil {
		return ""
	}
	if b.OperationName != "" {
		return b.OperationName
	}
	if m := gqlOperation.FindStringSubmatch(b.Query); m != nil {
		return m[2]
	}
	return ""
}
