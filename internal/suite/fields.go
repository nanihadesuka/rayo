package suite

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/nanihadesuka/rayo/internal/adb"
	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/mock"
	"github.com/nanihadesuka/rayo/internal/report"
	"github.com/nanihadesuka/rayo/internal/schema"
	"gopkg.in/yaml.v3"
)

// Every key of a suite, in tables: to add one, add it here.

// AppFields are those of app.
var AppFields = []schema.Field[app]{
	schema.TextField("id", "package", "The app's", func(a *app) *string { return &a.ID }).Needed(),
	schema.TextField("apk", "file", "What to install, if the device has another. rayo doesn't build it", func(a *app) *string { return &a.APK }).Needed(),
}

// DeviceFields are those of device.
var DeviceFields = []schema.Field[adb.Setup]{
	{Name: "size", Takes: "pixels, such as `1080x2400`", About: "Of the screen", Schema: schema.Pattern(adb.ScreenSize.String()), Read: func(s *adb.Setup, n *yaml.Node) error {
		if n.Kind != yaml.ScalarNode || !adb.ScreenSize.MatchString(n.Value) {
			return fmt.Errorf("takes pixels, such as 1080x2400, not %q", n.Value)
		}
		s.Size = n.Value
		return nil
	}},
	schema.NumberField("dpi", "72 to 1000", "Its density. With `size`, it says how many dp there are: 1080 pixels at 420 dpi are 411 dp", 72, 1000, func(s *adb.Setup, v int) { s.DPI = v }),
	schema.NumberField("fontScale", "0.5 to 3", "Of the text; 1 is normal", 0.5, 3.0, func(s *adb.Setup, v float64) { s.FontScale = &v }),
	{Name: "darkMode", Takes: "`true` or `false`", About: "The dark theme", Schema: schema.Type("boolean"), Read: func(s *adb.Setup, n *yaml.Node) error {
		s.DarkMode = new(bool)
		return schema.FlagField("", "", func(b *bool) *bool { return b }).Read(s.DarkMode, n)
	}},
}

// IgnoredFields are those of a call of unmocked's ignore, when not written as --ignore-unmocked takes it.
var IgnoredFields = []schema.Field[mock.Ignored]{
	schema.TextField("host", "host", "Of the call, or of its subdomains", func(i *mock.Ignored) *string { return &i.Host }).Optional("any"),
	schema.TextField("path", "a path", "Of the call, without the host; `*` is any text but `/`, `**` any", func(i *mock.Ignored) *string { return &i.Path }).Optional("any"),
	{Name: "graphql", Takes: "a name or a list", About: "GraphQL operations", Default: "any call",
		Schema: schema.AnyOf(schema.Type("string"), schema.List(schema.Type("string"))), Read: func(i *mock.Ignored, n *yaml.Node) (err error) {
			i.GraphQL, err = readNames(n)
			return err
		}},
}

// UnmockedFields are those of unmocked.
var UnmockedFields = []schema.Field[mock.Unmocked]{
	{Name: "ignore", Takes: "a list", Required: true, About: "Calls that no mock answers but that don't fail the flow: each `graphql:Operation`, `host/path`, `host` or `/path`, as `--ignore-unmocked`, or a mapping with host, path and graphql, see [Calls not worth mocking](#calls-not-worth-mocking)",
		Schema: schema.List(schema.AnyOf(schema.Type("string"), schema.Of(IgnoredFields))), Read: readIgnored},
	schema.NumberField("status", "100 to 599", "What those calls are answered with", 100, 599, func(u *mock.Unmocked, v int) { u.Status = v }).Optional("404"),
	schema.TextField("body", "text", "Of the answer", func(u *mock.Unmocked) *string { return &u.Body }).Optional("`not mocked: ` and the call"),
}

// unmockedField is the suite's unmocked, and that of one of its flows.
func unmockedField[T any](to func(*T) **mock.Unmocked) schema.Field[T] {
	return schema.Field[T]{Name: "unmocked", Takes: "a mapping", About: "Calls without a mock that don't fail the flow, see [Calls not worth mocking](#calls-not-worth-mocking)",
		Schema: schema.Of(UnmockedFields), Read: func(t *T, n *yaml.Node) error {
			u := &mock.Unmocked{}
			if err := schema.Read(UnmockedFields, u, n); err != nil {
				return err
			}
			if err := u.Check(); err != nil {
				return &schema.Error{Line: n.Line, What: err.Error()}
			}
			*to(t) = u
			return nil
		}}
}

func readIgnored(u *mock.Unmocked, n *yaml.Node) error {
	if n.Kind != yaml.SequenceNode {
		return &schema.Error{Line: n.Line, What: "takes a list of calls"}
	}
	for _, c := range n.Content {
		if c.Kind == yaml.ScalarNode {
			i, err := mock.ParseIgnored(c.Value)
			if err != nil {
				return &schema.Error{Line: c.Line, What: err.Error()}
			}
			u.Ignore = append(u.Ignore, i)
			continue
		}
		var i mock.Ignored
		if err := schema.Read(IgnoredFields, &i, c); err != nil {
			return err
		}
		u.Ignore = append(u.Ignore, i)
	}
	return nil
}

// readNames reads a name or a list of them.
func readNames(n *yaml.Node) ([]string, error) {
	list := []*yaml.Node{n}
	if n.Kind == yaml.SequenceNode {
		list = n.Content
	}
	var names []string
	for _, c := range list {
		if c.Kind != yaml.ScalarNode || c.Value == "" {
			return nil, &schema.Error{Line: c.Line, What: "takes a name, or a list of them"}
		}
		names = append(names, c.Value)
	}
	return names, nil
}

// Capture is how a capture of a suite's flows calls the backend and becomes mocks: the options of
// the same names, which win.
type Capture struct {
	Settings mock.Settings
	Hosts    []string
	From     string
}

// Or fills what c doesn't say with what other does.
func (c *Capture) Or(other *Capture) *Capture {
	switch {
	case c == nil:
		return other
	case other == nil:
		return c
	}
	hosts := c.Hosts
	if len(hosts) == 0 {
		hosts = other.Hosts
	}
	return &Capture{Settings: c.Settings.Or(other.Settings), Hosts: hosts, From: cmp.Or(c.From, other.From)}
}

func names[T any](name, about string, to func(*T) *[]string) schema.Field[T] {
	return schema.Field[T]{Name: name, Takes: "a name or a list", About: about, Schema: schema.AnyOf(schema.Type("string"), schema.List(schema.Type("string"))),
		Read: func(t *T, n *yaml.Node) (err error) {
			*to(t), err = readNames(n)
			return err
		}}
}

func choiceField[T any](name, about string, to func(*T) *string, values ...string) schema.Field[T] {
	return schema.Field[T]{Name: name, Takes: "`" + strings.Join(values, "`, `") + "`", About: about, Schema: schema.Enum(anys(values)...), Read: func(t *T, n *yaml.Node) error {
		if n.Kind != yaml.ScalarNode || !slices.Contains(values, n.Value) {
			return fmt.Errorf("takes %s, not %q", strings.Join(values, ", "), n.Value)
		}
		*to(t) = n.Value
		return nil
	}}
}

func anys(list []string) []any {
	var out []any
	for _, s := range list {
		out = append(out, s)
	}
	return out
}

// CaptureFields are those of capture.
var CaptureFields = []schema.Field[Capture]{
	names("hosts", "As `--hosts`", func(c *Capture) *[]string { return &c.Hosts }),
	schema.TextField("from", "url", "As `--capture-from`", func(c *Capture) *string { return &c.From }),
	choiceField("graphql", "As `--graphql`", func(c *Capture) *string { return &c.Settings.GraphQL }, mock.GraphQLVariables, mock.GraphQLOperation, mock.GraphQLOff),
	schema.TextField("graphqlPath", "regex", "As `--graphql-path`", func(c *Capture) *string { return &c.Settings.GraphQLPath }),
	choiceField("rest", "As `--rest`", func(c *Capture) *string { return &c.Settings.REST }, mock.RESTBody, mock.RESTOff),
	names("ignoreParams", "As `--ignore-params`", func(c *Capture) *[]string { return &c.Settings.IgnoreParams }),
	names("ignoreBody", "As `--ignore-body`", func(c *Capture) *[]string { return &c.Settings.IgnoreBody }),
	choiceField("timestamps", "As `--timestamps`", func(c *Capture) *string { return &c.Settings.Timestamps }, mock.TimestampsFormat, mock.TimestampsExact),
	names("captureHeaders", "As `--capture-headers`", func(c *Capture) *[]string { return &c.Settings.Headers }),
	schema.FlagField("ignoreHost", "As `--ignore-host`", func(c *Capture) *bool { return &c.Settings.IgnoreHost }),
	schema.FlagField("ignorePathIds", "As `--ignore-path-ids`", func(c *Capture) *bool { return &c.Settings.PathIDs }),
	{Name: "stateless", Takes: "a path or a list", About: "As `--stateless`", Schema: schema.AnyOf(schema.Type("string"), schema.List(schema.Type("string"))), Read: func(c *Capture, n *yaml.Node) (err error) {
		if c.Settings.Stateless, err = readNames(n); err != nil {
			return err
		}
		for _, g := range c.Settings.Stateless {
			if _, err := mock.Glob(g); err != nil {
				return &schema.Error{Line: n.Line, What: err.Error()}
			}
		}
		return nil
	}},
	{Name: "redactValue", Takes: "values", About: "As `--redact-value`: what a private field of a body becomes, such as `phone: \"+4915100000000\"`", Schema: schema.Map(schema.Text()), Read: func(c *Capture, n *yaml.Node) error {
		values, err := flow.Variables(n)
		if err != nil {
			return err
		}
		for _, k := range slices.Sorted(maps.Keys(values)) {
			c.Settings.RedactValues = append(c.Settings.RedactValues, k+"="+values[k])
		}
		return nil
	}},
}

// captureField is the suite's capture, and that of one of its flows.
func captureField[T any](to func(*T) **Capture) schema.Field[T] {
	return schema.Field[T]{Name: "capture", Takes: "a mapping", About: "How a capture calls the backend and becomes mocks, as the options of the same names, which win, see [Capturing a suite's flow](#capturing-a-suites-flow)",
		Schema: schema.Of(CaptureFields), Read: func(t *T, n *yaml.Node) error {
			c := &Capture{}
			if err := schema.Read(CaptureFields, c, n); err != nil {
				return err
			}
			if err := c.Settings.Check(); err != nil {
				return &schema.Error{Line: n.Line, What: err.Error()}
			}
			*to(t) = c
			return nil
		}}
}

// FlowFields are those of a flow of the list that isn't only its file.
var FlowFields = []schema.Field[entry]{
	schema.TextField("file", "file", "The flow", func(e *entry) *string { return &e.file }).Needed(),
	schema.NumberField("retries", "a number from 0", "How many times it is tried again if it fails", 0, 0, func(e *entry, v int) { e.retries = &v }).Optional("the suite's"),
	{Name: "mock", Takes: "a path or a list", About: "Who answers as the backend, as a flow's `mock`, relative to the suite; the flow's own `mock` wins, see [Mocks of a suite](#mocks-of-a-suite)",
		Schema: schema.AnyOf(schema.Type("string"), schema.List(schema.Type("string"))), Read: func(e *entry, n *yaml.Node) (err error) {
			e.mock, err = readMocks(n)
			return err
		}},
	networkField(func(e *entry) *string { return &e.network }).Optional("the suite's"),
	unmockedField(func(e *entry) **mock.Unmocked { return &e.unmocked }).Optional("the suite's; its calls are added to them"),
	captureField(func(e *entry) **Capture { return &e.capture }).Optional("the suite's; what it says wins over it"),
}

// networkField is the suite's network, and that of one of its flows.
func networkField[T any](to func(*T) *string) schema.Field[T] {
	return schema.Field[T]{Name: "network", Takes: "`interceptor` or `proxy`", About: "How the app's calls reach the mock, as a flow's `network`; the flow's own and `--network` override it, see [Mocks of a suite](#mocks-of-a-suite)",
		Schema: schema.Enum(flow.Interceptor, flow.Proxy), Read: func(t *T, n *yaml.Node) error {
			if n.Kind != yaml.ScalarNode || n.Value != flow.Interceptor && n.Value != flow.Proxy {
				return errors.New("takes interceptor or proxy")
			}
			*to(t) = n.Value
			return nil
		}}
}

// readMocks reads a path or a list of them, and checks that each loads.
func readMocks(n *yaml.Node) ([]string, error) {
	list := []*yaml.Node{n}
	if n.Kind == yaml.SequenceNode {
		list = n.Content
	}
	var paths []string
	for _, c := range list {
		if c.Kind != yaml.ScalarNode || c.Value == "" {
			return nil, &schema.Error{Line: c.Line, What: "takes the path of a mock, or a list of them"}
		}
		paths = append(paths, c.Value)
	}
	return paths, nil
}

// Fields are those of a suite.
var Fields = []schema.Field[Suite]{
	schema.TextField("name", "text", "Shown in the results", func(s *Suite) *string { return &s.Name }).Optional("the file"),
	{Name: "flows", Takes: "a list", Required: true, About: "Files, run in this order; or mappings with file, retries, mock, network, unmocked and capture, see [below](#a-flow-of-a-suite)",
		Schema: schema.S{"type": "array", "minItems": 1, "items": schema.AnyOf(schema.Type("string"), schema.Of(FlowFields))}, Read: readFlows},
	schema.NumberField("retries", "a number from 0", "How many times a flow that fails is tried again, see [Retries](#retries)", 0, 0, func(s *Suite, v int) { s.Retries = &v }).Optional(fmt.Sprint(DefaultRetries)),
	{Name: "app", Takes: "a mapping", About: "Installed before the flows, see [below](#the-app-of-a-suite)", Schema: schema.Of(AppFields), Read: func(s *Suite, n *yaml.Node) error {
		s.App = &app{}
		return schema.Read(AppFields, s.App, n)
	}},
	{Name: "device", Takes: "a mapping", About: "How it is while the flows run, see [Device setup](#device-setup)", Schema: schema.Of(DeviceFields), Read: func(s *Suite, n *yaml.Node) error {
		s.Device = &adb.Setup{}
		return schema.Read(DeviceFields, s.Device, n)
	}},
	{Name: "env", Takes: "values", About: "Variables for every flow, over a flow's own `env`; `-e` overrides one; they may use `${...}`, such as `${rayo.file}`, see [Variables](#variables)", Schema: schema.Map(schema.Text()), Read: func(s *Suite, n *yaml.Node) (err error) {
		s.Env, err = flow.Variables(n)
		return err
	}},
	schema.NumberField("timeoutScale", "0.1 to 10", "Multiplies the default waits, see [Default times](#default-times); `--timeout-scale` overrides it", LowestScale, HighestScale, func(s *Suite, v float64) { s.TimeoutScale = v }).Optional("1"),
	networkField(func(s *Suite) *string { return &s.Network }).Optional("each flow's, else `interceptor`"),
	unmockedField(func(s *Suite) **mock.Unmocked { return &s.Unmocked }).Optional("none: each fails the flow"),
	captureField(func(s *Suite) **Capture { return &s.Capture }),
	{Name: "ca", Takes: "folder", About: "`--ca` overrides it: the CA the proxy signs with, its ca.pem and ca-key.pem, relative to the suite; `rayo ca new` makes one, see [Mocks of a suite](#mocks-of-a-suite)",
		Default: "`.rayo/ca` next to the suite or above it, up to the repository's root; else `~/.rayo`",
		Schema:  schema.Type("string"), Read: func(s *Suite, n *yaml.Node) error {
			if n.Kind != yaml.ScalarNode || n.Value == "" {
				return errors.New("takes a folder")
			}
			s.CA = n.Value
			return nil
		}},
	{Name: "output", Takes: "a folder, or a mapping", About: "What a run writes, and where: a folder, or folder, keep, video, artifacts and nameTemplate, see [What is kept](#what-is-kept)", Default: "`.rayo` where rayo runs",
		Schema: schema.AnyOf(schema.Type("string"), schema.Of(OutputFields)), Read: func(s *Suite, n *yaml.Node) error {
			if n.Kind == yaml.ScalarNode {
				s.Output.Folder = n.Value
				return nil
			}
			return schema.Read(OutputFields, &s.Output, n)
		}},
	{Name: "results", Takes: "a file or a list", About: "`--results` replaces them all: `.txt`, `.json`, `.xml` (JUnit) or `.md`, see [Results](#results)",
		Schema: func() schema.S { f := schema.Pattern(`\.(txt|json|xml|md)$`); return schema.AnyOf(f, schema.List(f)) }(), Read: readResults},
	choiceField("detail", "`--detail` overrides it: how much is printed, see [Results](#results)", func(s *Suite) *string { return &s.Detail }, flow.Details...).Optional("`normal`"),
	schema.NumberField("failureWindow", "seconds, from 1", "`--failure-window` overrides it: a failure's log and calls begin this long before the failing step, see [Failures](#failures)", 1, 0, func(s *Suite, v int) { s.FailureWindow = v }).Optional("10"),
	{Name: "resultsTemplate", Takes: "file", About: "`--results-template` overrides it: the results with its extension are written as it says, see [A template of your own](#a-template-of-your-own)",
		Schema: schema.Type("string"), Read: func(s *Suite, n *yaml.Node) error {
			if n.Kind != yaml.ScalarNode {
				return errors.New("takes a file")
			}
			if err := report.CheckTemplate(s.Relative(n.Value)); err != nil {
				return &schema.Error{Line: n.Line, What: err.Error()}
			}
			s.ResultsTemplate = n.Value
			return nil
		}},
	{Name: "show", Takes: "a name or a list", About: "`--show` overrides it: the flows the results list, of `" + strings.Join(report.Shows, "`, `") + "`, see [Results](#results)",
		Schema: schema.AnyOf(schema.Enum(anys(report.Shows)...), schema.List(schema.Enum(anys(report.Shows)...))), Default: "`all`", Read: func(s *Suite, n *yaml.Node) (err error) {
			if s.Show, err = readNames(n); err != nil {
				return err
			}
			if err := report.CheckShow(s.Show); err != nil {
				return &schema.Error{Line: n.Line, What: err.Error()}
			}
			return nil
		}},
	schema.NumberField("slowest", "a number from 1", "`--slowest` overrides it: how many flows the slow list has", 1, 0, func(s *Suite, v int) { s.Slowest = v }).Optional(fmt.Sprint(report.DefaultSlowest)),
	{Name: "format", Takes: "`human`, `json` or `ndjson`", About: "What is printed; `--format` overrides it, see [For a program](#for-a-program)", Default: "`human`",
		Schema: schema.Enum(report.Human, report.JSON, report.NDJSON), Read: func(s *Suite, n *yaml.Node) error {
			if n.Kind != yaml.ScalarNode || !slices.Contains(report.Formats, n.Value) {
				return errors.New("takes human, json or ndjson")
			}
			s.Format = n.Value
			return nil
		}},
	schema.NumberField("minFlows", "a number from 1", "`--min-flows` overrides it: fewer flows chosen, by tags, names or what changed, is an error (exit `2`), not a green run of nothing", 1, 0, func(s *Suite, v int) { s.MinFlows = v }).Optional("none"),
	schema.FlagField("flakyFailsBuild", "`--flaky-fails-build` too: a run where every flow passed, some only on a retry, exits `4`, see [Flaky flows](#flaky-flows)", func(s *Suite) *bool { return &s.FlakyFailsBuild }).Optional("`false`: it exits `0`"),
}

// OutputFields are those of output, when a mapping. Each has an option of the same name that overrides it.
var OutputFields = []schema.Field[Output]{
	schema.TextField("folder", "folder", "Where everything goes; `--output` overrides it", func(o *Output) *string { return &o.Folder }).Optional("`.rayo` where rayo runs"),
	choiceField("keep", "The files of every try (`all`), only of the tries that failed (`failed`), or none; `--keep` overrides it", func(o *Output) *string { return &o.Keep }, KeepAll, KeepFailed, KeepNone).Optional("`all`"),
	{Name: "video", Takes: "`true` or `false`", About: "A video of each flow whose header doesn't say `record`; `--video` overrides it", Default: "`false`", Schema: schema.Type("boolean"), Read: func(o *Output, n *yaml.Node) error {
		o.Video = new(bool)
		return schema.FlagField("", "", func(b *bool) *bool { return b }).Read(o.Video, n)
	}},
	{Name: "artifacts", Takes: "a name or a list", About: "The kinds of file written, of `" + strings.Join(flow.Artifacts, "`, `") + "`; `--artifacts` overrides it",
		Schema: schema.AnyOf(schema.Enum(anys(flow.Artifacts)...), schema.List(schema.Enum(anys(flow.Artifacts)...))), Default: "all", Read: func(o *Output, n *yaml.Node) (err error) {
			if o.Artifacts, err = readNames(n); err != nil {
				return err
			}
			if err := flow.CheckArtifacts(o.Artifacts); err != nil {
				return &schema.Error{Line: n.Line, What: err.Error()}
			}
			return nil
		}},
	{Name: "nameTemplate", Takes: "a template", About: "How the files are named in the folder; `--name-template` overrides it",
		Schema: schema.Type("string"), Default: "`" + flow.DefaultTemplate + "`", Read: func(o *Output, n *yaml.Node) error {
			if n.Kind != yaml.ScalarNode {
				return errors.New("takes a template")
			}
			if err := flow.CheckTemplate(n.Value); err != nil {
				return &schema.Error{Line: n.Line, What: err.Error()}
			}
			o.NameTemplate = n.Value
			return nil
		}},
}

// Moved are the keys a suite had before 3.0, and where they are now.
var Moved = map[string]string{
	"record":            "output: { video: true }",
	"recordFolder":      "output: { folder: ... }, or a nameTemplate with a /",
	"keep":              "output: { keep: ... }",
	"artifacts":         "output: { artifacts: ... }",
	"name-template":     "output: { nameTemplate: ... }",
	"failure-window":    "failureWindow",
	"results-template":  "resultsTemplate",
	"flaky-fails-build": "flakyFailsBuild",
}

// What keep takes.
const (
	KeepFailed = "failed"
	KeepAll    = "all"
	KeepNone   = "none"
)

// Schema is a suite's JSON Schema.
func Schema() []byte { return schema.Of(Fields).File("A rayo suite, *.suite.yaml", nil) }

func readFlows(s *Suite, n *yaml.Node) error {
	if n.Kind != yaml.SequenceNode || len(n.Content) == 0 {
		return fmt.Errorf("takes a list of files")
	}
	for _, c := range n.Content {
		e := entry{line: c.Line}
		if c.Kind == yaml.ScalarNode {
			e.file = c.Value
		} else if err := schema.Read(FlowFields, &e, c); err != nil {
			return err
		}
		if slices.ContainsFunc(s.Flows, func(o entry) bool { return o.file == e.file }) {
			return &schema.Error{Line: c.Line, What: e.file + " is listed twice"}
		}
		s.Flows = append(s.Flows, e)
	}
	return nil
}

func readResults(s *Suite, n *yaml.Node) error {
	list := []*yaml.Node{n}
	if n.Kind == yaml.SequenceNode {
		list = n.Content
	}
	for _, c := range list {
		if c.Kind != yaml.ScalarNode {
			return &schema.Error{Line: c.Line, What: "takes a file or a list of files"}
		}
		if _, err := report.FormatOf(c.Value); err != nil {
			return err
		}
		s.Results = append(s.Results, c.Value)
	}
	return nil
}
