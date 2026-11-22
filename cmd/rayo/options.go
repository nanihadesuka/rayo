package main

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/flow"
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
		flag("failures-only", "leave the flows that passed out of the results", func(o *options) *bool { return &o.failuresOnly }),
	}},
	{[]string{cmdTest}, "mocks from the real backend", []option{
		text("capture", "folder", "call the real backend, and write what it answers as mocks", func(o *options) *string { return &o.runner.Capture }),
		text("capture-from", "url", "the backend, if calls don't begin with /host/", func(o *options) *string { return &o.runner.Backend }),
	}},
	{[]string{cmdLive}, "", []option{
		text("app", "package", "of a flow that isn't there yet", func(o *options) *string { return &o.app }),
		list("mock", "its mocks, relative to it", func(o *options) *[]string { return &o.mocks }),
		flag("no-run", "don't begin by running the steps it has", func(o *options) *bool { return &o.noRun }),
		flag("serve", "only take commands from rayo do, until .quit", func(o *options) *bool { return &o.serve }),
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
	return o.verbose || o.runner.Profile || len(flows) == 1
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
