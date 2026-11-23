package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/mock"
	"github.com/nanihadesuka/rayo/internal/report"
	"github.com/nanihadesuka/rayo/internal/suite"
)

// options are those of all the commands: each takes only its own, see the table below.
type options struct {
	args []string // what isn't an option

	// Of those that use a device.
	device     string
	emulator   bool
	animations bool
	output     string // folder; empty if not said
	runner     flow.Options

	// Of test.
	continuous             bool
	only, include, exclude []string
	rerunFailed            bool
	shuffle                bool
	seed                   uint64
	retries                int // negative if not said
	failFast, keepGoing    bool
	verbose                bool
	results                []string // files
	failuresOnly           bool
	suite                  *suite.Suite

	// Of live.
	app          string
	mocks        []string
	noRun, serve bool

	// Of do.
	screen, json bool

	// Of mocks.
	take  []string
	prune bool
}

// option is all there is to know about one: to add one, add it to the table below.
type option struct {
	short, name string
	value       string // what it takes; empty if nothing
	optional    bool   // the value, only as --name=value
	about       string
	set         func(o *options, value string) error
}

// section is the options of some commands.
type section struct {
	commands []string
	title    string // of a part of them
	options  []option
}

const (
	cmdTest      = "test"
	cmdCheck     = "check"
	cmdLive      = "live"
	cmdDo        = "do"
	cmdHierarchy = "hierarchy"
	cmdDevices   = "devices"
	cmdCA        = "ca"
	cmdMocks     = "mocks"
)

func flag(name, about string, at func(o *options) *bool) option {
	return option{name: name, about: about, set: func(o *options, _ string) error { *at(o) = true; return nil }}
}

func text(name, value, about string, at func(o *options) *string) option {
	return option{name: name, value: value, about: about, set: func(o *options, v string) error {
		if v == "" {
			return fmt.Errorf("--%s takes a %s", name, value)
		}
		*at(o) = v
		return nil
	}}
}

func choice(name, value, about string, at func(o *options) *string, values ...string) option {
	return option{name: name, value: value, about: about, set: func(o *options, v string) error {
		if !slices.Contains(values, v) {
			return fmt.Errorf("--%s takes %s or %s, not %q", name, strings.Join(values[:len(values)-1], ", "), values[len(values)-1], v)
		}
		*at(o) = v
		return nil
	}}
}

func list(name, about string, at func(o *options) *[]string) option {
	return option{name: name, value: "a,b", about: about, set: func(o *options, v string) error {
		*at(o) = strings.Split(v, ",")
		return nil
	}}
}

func (p option) withShort(short string) option { p.short = short; return p }

var sections = []section{
	{[]string{cmdTest, cmdLive, cmdHierarchy}, "", []option{
		text("device", "serial", "when several are connected", func(o *options) *string { return &o.device }),
		flag("emulator", "never a real device, even if it is the only one", func(o *options) *bool { return &o.emulator }),
	}},
	{[]string{cmdTest, cmdLive, cmdDo}, "", []option{
		text("output", "folder", "where failures and screenshots go (default .rayo)", func(o *options) *string { return &o.output }),
	}},
	{[]string{cmdTest, cmdLive}, "", []option{
		{short: "e", name: "env", value: "KEY=VALUE", about: "a variable for the flows", set: func(o *options, v string) error {
			k, value, ok := strings.Cut(v, "=")
			if !ok {
				return errors.New("--env takes KEY=VALUE")
			}
			o.runner.Env[k] = value
			return nil
		}},
		{name: "timeout-scale", value: "factor", about: "multiply the default waits, not a step's own timeout (default 1)", set: func(o *options, v string) error {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil || f < suite.LowestScale || f > suite.HighestScale {
				return fmt.Errorf("--timeout-scale takes %v to %v, not %q", suite.LowestScale, suite.HighestScale, v)
			}
			o.runner.Scale = f
			return nil
		}},
		{name: "mock-port", value: "port", about: "where the app finds the mock on the device (default 8080)", set: func(o *options, v string) error {
			n, err := strconv.ParseUint(v, 10, 16)
			if err != nil || n == 0 {
				return fmt.Errorf("not a port: %q", v)
			}
			o.runner.MockPort = int(n)
			return nil
		}},
		flag("animations", "leave the device's animations on", func(o *options) *bool { return &o.animations }),
		flag("profile", "where the time of each step went", func(o *options) *bool { return &o.runner.Profile }),
	}},
	{[]string{cmdTest}, "which flows", []option{
		list("only", "only the flows with these names", func(o *options) *[]string { return &o.only }),
		list("include-tags", "only flows with one of these tags", func(o *options) *[]string { return &o.include }),
		list("exclude-tags", "not flows with one of these tags", func(o *options) *[]string { return &o.exclude }),
		flag("rerun-failed", "only those that haven't passed since they last failed", func(o *options) *bool { return &o.rerunFailed }),
	}},
	{[]string{cmdTest}, "how they run", []option{
		{name: "retries", value: "n", about: "try again what fails, whatever the suite says (default 1)", set: func(o *options, v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return fmt.Errorf("--retries takes a number, not %q", v)
			}
			o.retries = n
			return nil
		}},
		flag("fail-fast", "stop at the first failure", func(o *options) *bool { return &o.failFast }),
		flag("keep-going", "don't stop when the device or the app fails over and over", func(o *options) *bool { return &o.keepGoing }),
		{name: "shuffle", value: "seed", optional: true, about: "in a random order, or in that of an earlier run", set: func(o *options, v string) error {
			o.shuffle, o.seed = true, uint64(time.Now().UnixMilli())
			if v == "" {
				return nil
			}
			var err error
			if o.seed, err = strconv.ParseUint(v, 10, 64); err != nil {
				return fmt.Errorf("--shuffle takes a number, not %q", v)
			}
			return nil
		}},
		flag("continuous", "run again whenever a file changes", func(o *options) *bool { return &o.continuous }).withShort("c"),
	}},
	{[]string{cmdTest}, "what comes out", []option{
		flag("verbose", "every step (always, for a single flow)", func(o *options) *bool { return &o.verbose }).withShort("v"),
		{name: "results", value: "file", about: "write the results: .txt, .json or .xml (JUnit); may be repeated", set: func(o *options, v string) error {
			o.results = append(o.results, v)
			if _, err := report.FormatOf(v); err != nil {
				return fmt.Errorf("--results %w", err)
			}
			return nil
		}},
		flag("timings", "each step's time, each runFlow's total, the slowest steps and flows, and a note while a step hangs", func(o *options) *bool { return &o.runner.Timings }),
		flag("failures-only", "leave the flows that passed out of the results", func(o *options) *bool { return &o.failuresOnly }),
	}},
	{[]string{cmdTest, cmdLive}, "mocks from the real backend", []option{
		text("capture", "folder", "call the real backend, and write what it answers as mocks; in live, on .quit, without the calls of steps that failed", func(o *options) *string { return &o.runner.Capture }),
		{name: "capture-update", value: "folder", about: "capture into folder/.update, and say what changed from the mocks in folder; rayo mocks apply takes it", set: func(o *options, v string) error {
			o.runner.CaptureUpdate, o.runner.Capture = v, filepath.Join(v, mock.UpdateFolder)
			return nil
		}},
		flag("check", "with --capture-update, fail if the mocks differ from the backend, and change nothing", func(o *options) *bool { return &o.runner.CaptureCheck }),
		flag("capture-append", "go on with the capture already in the folder, as if in one go", func(o *options) *bool { return &o.runner.CaptureAppend }),
		{name: "redact-value", value: "name=value", about: "what a private field of a body becomes, such as phone=+4915100000000 (default one of the same form: a date stays a date); again for more", set: func(o *options, v string) error {
			if n, _, ok := strings.Cut(v, "="); !ok || n == "" {
				return fmt.Errorf("--redact-value takes name=value, not %q", v)
			}
			o.runner.Matching.RedactValues = append(o.runner.Matching.RedactValues, v)
			return nil
		}},
	}},
	{[]string{cmdTest, cmdLive, cmdMocks}, "how a capture or an import becomes mocks", []option{
		text("capture-from", "url", "the backend, if calls don't begin with /host/", func(o *options) *string { return &o.runner.Backend }),
		choice("graphql", "match", "tell GraphQL calls apart by operation+variables (default), by operation, or off, as any call", func(o *options) *string { return &o.runner.Matching.GraphQL }, mock.GraphQLVariables, mock.GraphQLOperation, mock.GraphQLOff),
		{name: "graphql-path", value: "regex", about: "paths of GraphQL calls, besides those whose body has query or operationName (default .*/graphql)", set: func(o *options, v string) error {
			if _, err := mock.Pattern(v); err != nil {
				return fmt.Errorf("--graphql-path: %w", err)
			}
			o.runner.Matching.GraphQLPath = v
			return nil
		}},
		choice("rest", "match", "match other calls by their body too (body, default), or by method and address alone (off)", func(o *options) *string { return &o.runner.Matching.REST }, mock.RESTBody, mock.RESTOff),
		list("ignore-params", "names in the query not to match, such as session or depart_at", func(o *options) *[]string { return &o.runner.Matching.IgnoreParams }),
		list("ignore-body", "names, or paths such as trip.departAt, in JSON bodies and GraphQL variables, and names in forms, not to match", func(o *options) *[]string { return &o.runner.Matching.IgnoreBody }),
		choice("timestamps", "match", "match dates and times in the query and bodies by their form (format, default), or exactly", func(o *options) *string { return &o.runner.Matching.Timestamps }, mock.TimestampsFormat, mock.TimestampsExact),
		list("capture-headers", "headers of the request to match too, such as Accept-Language", func(o *options) *[]string { return &o.runner.Matching.Headers }),
		flag("ignore-host", "match calls whose path begins with a host, whatever the host", func(o *options) *bool { return &o.runner.Matching.IgnoreHost }),
	}},
	{[]string{cmdTest, cmdLive}, "how calls reach the mock", []option{
		{name: "network", value: "way", about: "interceptor, the app sends them (default, or the flow's); proxy, the device's proxy does", set: func(o *options, v string) error {
			if v != flow.Interceptor && v != flow.Proxy {
				return fmt.Errorf("--network takes interceptor or proxy, not %q", v)
			}
			o.runner.Network = v
			return nil
		}},
		list("hosts", "with a proxy, the hosts to open, and their subdomains (default: capturing, all but Google's, Firebase's, analytics and crash reports; else those of the mocks)", func(o *options) *[]string { return &o.runner.Hosts }),
	}},
	{[]string{cmdLive}, "", []option{
		text("app", "package", "of a flow that isn't there yet", func(o *options) *string { return &o.app }),
		list("mock", "its mocks, relative to it", func(o *options) *[]string { return &o.mocks }),
		flag("no-run", "don't begin by running the steps it has", func(o *options) *bool { return &o.noRun }),
		flag("serve", "only take commands from rayo do, until .quit", func(o *options) *bool { return &o.serve }),
	}},
	{[]string{cmdMocks}, "", []option{
		list("only", "the changes whose name has one of these, such as graphql:Wallet or GET /profile (default all)", func(o *options) *[]string { return &o.take }),
		flag("prune", "also take out the mocks that weren't called", func(o *options) *bool { return &o.prune }),
	}},
	{[]string{cmdDo}, "", []option{
		flag("screen", "also what is on screen afterwards", func(o *options) *bool { return &o.screen }),
		flag("json", "for a program", func(o *options) *bool { return &o.json }),
	}},
}

const usage = `rayo - runs flows on Android, fast

  rayo test [options] <flow, folder or suite>...   run flows; a suite is a *.rayo.yaml
  rayo check <flow, folder or suite>...            check flows without a device
  rayo live [options] <flow>                       write a flow as it runs, command by command
  rayo do [options] <commands>                     send commands to the live session; rayo do .help
  rayo hierarchy [options]                         what is on screen, and how to refer to it
  rayo devices                                     connected devices
  rayo mocks apply [options] <folder>              take the changes of rayo test --capture-update into the mocks
  rayo mocks import [options] <file.har> <folder>  write the calls of a HAR file, such as mitmproxy's, as mocks
  rayo ca export <file>                            the certificate the proxy signs with, for the app's debug build
  rayo help <command>                              what a command takes, of a flow or of rayo; also flow, suite and json
  rayo schema flow, suite or results               its JSON Schema, for editors and programs
  rayo version
`

// help is made from the table.
func help() string {
	var b strings.Builder
	b.WriteString(usage)
	for _, s := range sections {
		names := strings.Join(s.commands, ", ")
		if i := strings.LastIndex(names, ", "); i >= 0 {
			names = names[:i] + " and " + names[i+2:]
		}
		fmt.Fprintf(&b, "\nOptions of %s", names)
		if s.title != "" {
			fmt.Fprintf(&b, ": %s", s.title)
		}
		b.WriteString("\n")
		for _, p := range s.options {
			b.WriteString(p.help())
		}
	}
	return b.String()
}

// helpOf is the options of a command; empty if it has none.
func helpOf(command string) string {
	var b strings.Builder
	for _, s := range sections {
		if slices.Contains(s.commands, command) {
			for _, p := range s.options {
				b.WriteString(p.help())
			}
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return "Options of " + command + ":\n" + b.String()
}

func (p option) help() string {
	name := "--" + p.name
	switch {
	case p.optional:
		name += "[=" + p.value + "]"
	case p.value != "":
		name += " <" + p.value + ">"
	}
	if p.short != "" {
		name = "-" + p.short + ", " + name
	}
	return fmt.Sprintf("  %-26s %s\n", name, p.about)
}

// find is the option with a name as written, if the command takes it.
func find(command, written string) (option, error) {
	known := false
	for _, s := range sections {
		for _, p := range s.options {
			if written != "--"+p.name && (p.short == "" || written != "-"+p.short) {
				continue
			}
			if slices.Contains(s.commands, command) {
				return p, nil
			}
			known = true
		}
	}
	if known {
		return option{}, fmt.Errorf("%s is not an option of %s", written, command)
	}
	return option{}, fmt.Errorf("unknown option %s", written)
}

func parse(command string, args []string) (*options, error) {
	o := &options{retries: -1, runner: flow.Options{Env: map[string]string{}, MockPort: 8080}}
	for i := 0; i < len(args); i++ {
		// A command for do may begin with a dash, as in a list.
		if !strings.HasPrefix(args[i], "-") || command == cmdDo && !strings.HasPrefix(args[i], "--") {
			o.args = append(o.args, args[i])
			continue
		}
		name, value, joined := strings.Cut(args[i], "=")
		p, err := find(command, name)
		if err != nil {
			return nil, err
		}
		switch {
		case p.value == "" && joined:
			return nil, fmt.Errorf("%s takes no value", name)
		case p.value != "" && !joined && !p.optional:
			if i++; i >= len(args) {
				return nil, fmt.Errorf("%s needs a value", name)
			}
			value = args[i]
		}
		if err := p.set(o, value); err != nil {
			return nil, err
		}
	}
	if o.failFast && o.keepGoing {
		return nil, errors.New("--fail-fast and --keep-going don't go together")
	}
	return o, nil
}

// none is an error if there is something else than options.
func (o *options) none(command string) error {
	if len(o.args) > 0 {
		return fmt.Errorf("%s takes no %q", command, o.args[0])
	}
	return nil
}

// steps says whether to show every step, and not only how each flow went.
func (o *options) steps(flows []*flow.Flow) bool {
	return o.verbose || o.runner.Profile || o.runner.Timings || len(flows) == 1
}

// into settles the output folder: the option, or else the suite's.
func (o *options) into(folder string) {
	if o.output == "" {
		o.output = folder
	}
	if o.output == "" {
		o.output = ".rayo"
	}
}
