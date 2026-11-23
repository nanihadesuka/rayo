package suite

import (
	"fmt"
	"slices"

	"github.com/nanihadesuka/rayo/internal/adb"
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

// FlowFields are those of a flow of the list that isn't only its file.
var FlowFields = []schema.Field[entry]{
	schema.TextField("file", "file", "The flow", func(e *entry) *string { return &e.file }).Needed(),
	schema.NumberField("retries", "a number from 0", "How many times it is tried again if it fails", 0, 0, func(e *entry, v int) { e.retries = &v }).Optional("the suite's"),
}

// Fields are those of a suite.
var Fields = []schema.Field[Suite]{
	schema.TextField("name", "text", "Shown in the results", func(s *Suite) *string { return &s.Name }).Optional("the file"),
	{Name: "flows", Takes: "a list", Required: true, About: "Files, run in this order; or mappings with file and retries, see [below](#a-flow-of-a-suite)",
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
	schema.NumberField("timeoutScale", "0.1 to 10", "Multiplies the default waits, see [Default times](#default-times); `--timeout-scale` wins", LowestScale, HighestScale, func(s *Suite, v float64) { s.TimeoutScale = v }).Optional("1"),
	schema.TextField("output", "folder", "As `--output`, which wins", func(s *Suite) *string { return &s.Output }).Optional("`.rayo` where rayo runs"),
	{Name: "results", Takes: "a file or a list", About: "As `--results`, which wins over all of them: `.txt`, `.json` or `.xml` (JUnit)",
		Schema: func() schema.S { f := schema.Pattern(`\.(txt|json|xml)$`); return schema.AnyOf(f, schema.List(f)) }(), Read: readResults},
	schema.FlagField("record", "A video of each flow that doesn't say, see [Recording a whole flow](#recording-a-whole-flow)", func(s *Suite) *bool { return &s.Record }).Optional("`false`"),
	schema.TextField("recordFolder", "folder", "Where the videos go; needs `record: true`", func(s *Suite) *string { return &s.RecordFolder }).Optional("each flow's, in `output`"),
}

// Schema is a suite's JSON Schema.
func Schema() []byte { return schema.Of(Fields).File("A rayo suite, *.rayo.yaml", nil) }

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
