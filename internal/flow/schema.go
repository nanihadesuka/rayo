package flow

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/nanihadesuka/rayo/internal/schema"
	"gopkg.in/yaml.v3"
)

// header is what the first part of a flow fills in.
type header struct {
	flow *Flow
	p    *reader
}

// headerFields are every key of it: to add one, add it here. Filled in below because steps hold flows.
var headerFields []schema.Field[header]

func init() {
	headerFields = []schema.Field[header]{
		schema.TextField("appId", "package", "The app's", func(h *header) *string { return &h.flow.App }),
		schema.TextField("name", "text", "Shown in the output", func(h *header) *string { return &h.flow.Name }).Optional("the file's, or the folder's for `flow.yaml`"),
		{Name: "tags", Takes: "a list", About: "To choose flows, see [Tags](#tags)", Schema: schema.List(schema.Text()), Read: func(h *header, n *yaml.Node) error {
			if n.Decode(&h.flow.Tags) != nil {
				return errors.New("takes a list")
			}
			return nil
		}},
		{Name: "env", Takes: "values", About: "Variables for it, see [Variables](#variables)", Schema: schema.Map(schema.Text()), Read: func(h *header, n *yaml.Node) (err error) {
			h.flow.Env, err = variablesIn(n)
			return err
		}},
		{Name: "mock", Takes: "a path or a list", About: "Who answers as the backend, see [Mock](#mock)", Schema: schema.AnyOf(schema.Type("string"), schema.List(schema.Type("string"))), Read: func(h *header, n *yaml.Node) (err error) {
			h.flow.Mock, err = h.p.mocks(n)
			return schema.Placed(err)
		}},
		{Name: "network", Takes: "`interceptor` or `proxy`", Default: "`interceptor`, or `--network`", About: "How the app's calls reach the mock, see [Proxy](#proxy)", Schema: schema.Enum(Interceptor, Proxy), Read: func(h *header, n *yaml.Node) error {
			if n.Value != Interceptor && n.Value != Proxy || n.Kind != yaml.ScalarNode {
				return errors.New("takes interceptor or proxy")
			}
			h.flow.Network = n.Value
			return nil
		}},
		{Name: "record", Takes: "`true`, `false` or a file", Default: "the suite's", About: "A video of the flow, see [Recording a whole flow](#recording-a-whole-flow)", Schema: schema.Type("boolean", "string"), Read: func(h *header, n *yaml.Node) (err error) {
			h.flow.Recording, err = readRecording(n)
			return err
		}},
		{Name: "onFlowStart", Takes: "commands", About: "Run first", Schema: schema.Ref("commands"), Read: func(h *header, n *yaml.Node) (err error) {
			h.flow.OnStart, err = h.p.steps(n)
			return schema.Placed(err)
		}},
		{Name: "onFlowComplete", Takes: "commands", About: "Run last, also after a failure", Schema: schema.Ref("commands"), Read: func(h *header, n *yaml.Node) (err error) {
			h.flow.OnComplete, err = h.p.steps(n)
			return schema.Placed(err)
		}},
	}
}

// readHeader fills in the flow. An error that has its place keeps it.
func (p *reader) readHeader(f *Flow, n *yaml.Node) error {
	err := schema.Read(headerFields, &header{f, p}, n)
	var here *schema.Error
	if errors.As(err, &here) {
		return problem(p.path, here.Line, "%s", here.What)
	}
	return err
}

// Schema is a flow's JSON Schema. Its two parts are two documents, and an editor checks each alone.
func Schema() []byte {
	defs := map[string]schema.S{
		"commands": schema.List(schema.Ref("command")),
		"selector": schema.AnyOf(schema.Text(), keysSchema(selectorKeys, commonKeys)).About("An element of the screen"),
		"when":     keysSchema(whenKeys),
	}
	var alone []any
	each := map[string]schema.S{}
	for _, name := range Commands() {
		c := commands[name]
		if c.alone {
			alone = append(alone, name)
		}
		each[name] = c.schema().About(c.about)
	}
	one := schema.Object(each)
	one["minProperties"], one["maxProperties"] = 1, 1
	defs["command"] = schema.AnyOf(schema.Enum(alone...), one)
	return schema.AnyOf(schema.Of(headerFields), schema.Ref("commands")).File("A rayo flow", defs)
}

func (k command) schema() schema.S {
	var ways []schema.S
	if k.alone {
		ways = append(ways, schema.Type("null"))
	}
	if k.plain != nil {
		ways = append(ways, k.plain.schema())
	}
	switch {
	case k.free:
		ways = append(ways, schema.Map(schema.Text()))
	default:
		ways = append(ways, keysSchema(k.accepts(), commonKeys))
	}
	return schema.AnyOf(ways...)
}

func keysSchema(lists ...[]anyKey) schema.S {
	properties := map[string]schema.S{}
	for _, k := range slices.Concat(lists...) {
		if _, there := properties[k.title()]; !there { // a command's own key wins
			properties[k.title()] = k.schema().About(k.describe())
		}
	}
	return schema.Object(properties)
}

// Commands are the names of all of them.
func Commands() []string {
	var list []string
	for name := range commands {
		list = append(list, name)
	}
	sort.Strings(list)
	return list
}

// Tested says whether a command has run on a device.
func Tested(name string) bool { return !commands[name].untested }

// HeaderDocs describes it.
func HeaderDocs() []schema.Doc { return schema.Docs(headerFields) }

// HeaderHelp is for the terminal.
func HeaderHelp() string { return schema.Help(headerFields) }

// Tables are those of the docs: of each command, "keys " and its name.
func Tables() map[string]string {
	tables := map[string]string{
		"header":   schema.Table(headerFields),
		"selector": keyTable(selectorKeys, nil),
		"when":     keyTable(whenKeys, nil),
		"options":  keyTable(commonKeys, nil),
	}
	for name, c := range commands {
		if len(c.keys) > 0 {
			tables["keys "+name] = keyTable(c.keys, c.plain)
		}
	}
	return tables
}

func keyTable(keys []anyKey, plain anyKey) string {
	var b strings.Builder
	b.WriteString("| Key | Takes | |\n|---|---|---|\n")
	for _, k := range keys {
		about := k.describe()
		if k == plain {
			about += ". Short form"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", k.title(), k.takes(), about)
	}
	return b.String()
}

// Variables reads a mapping of variables, as a header's env.
func Variables(n *yaml.Node) (map[string]string, error) { return variablesIn(n) }
