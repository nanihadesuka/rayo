package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/live"
	"github.com/nanihadesuka/rayo/internal/mock"
	"github.com/nanihadesuka/rayo/internal/report"
	"github.com/nanihadesuka/rayo/internal/schema"
	"github.com/nanihadesuka/rayo/internal/suite"
)

// options are those of all the commands: each takes only its own, see the table below.
type options struct {
	args   []string // what isn't an option
	format string   // of what is printed; empty if not said

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
	shard, shards          int // --shard shard/shards; 0 if not said
	ofAll                  int // with --shard, the flows of all the parts
	minFlows               int // 0 if not said
	shuffle                bool
	seed                   uint64
	retries                int // negative if not said
	failFast, keepGoing    bool
	flakyFails             bool     // a flow that passed only on a retry fails the run
	detail                 string   // empty if not said
	window                 int      // failure-window, seconds; 0 if not said
	results                []string // files
	show                   []string // nil if not said
	slowest                int      // 0 if not said
	resultsTemplate        string
	keep                   string // empty if not said
	clean                  string // empty if not said
	ignoreLock             bool
	runID                  string // of the next run, as the lock says it; empty for a new one
	video                  bool   // --video
	suite                  *suite.Suite
	updateOwn              bool // --capture-update of the flow's own mock
	resume                 bool
	from, until            string          // steps, file:line
	mockState              []flow.Scenario // nil if not said
	starts                 map[*flow.Flow]*flow.Start
	leftOut                []string // by --resume, as they can't go on from where they failed
	onFail                 string   // what a failure does: empty to go on, else hold
	holdTimeout            time.Duration

	// Of live.
	app          string
	mocks        []string
	noRun, serve bool
	idle         time.Duration // 0 for the default

	// Of do.
	screen, screenChanges bool
	ignoreScreen          string        // a regular expression
	doTimeout             time.Duration // 0 to wait as long as it takes
	discard               bool

	// Of explain.
	latest bool

	// Of hierarchy and do.
	find string

	// Of test and check.
	since string // a git ref; empty for all the flows

	// Of mocks.
	take  []string
	prune bool

	// Of skills.
	agents  []string
	project string
}

// option is all there is to know about one: to add one, add it to the table below.
type option struct {
	short, name string
	value       string // what it takes; empty if nothing
	optional    bool   // the value, only as --name=value
	about       string
	key         string // of a suite, that it overrides, as output.keep; empty if none
	set         func(o *options, value string) error
}

// section is the options of some commands.
type section struct {
	commands []string
	title    string // of a part of them
	options  []option
}

// What --on-fail takes.
const (
	onFailGoOn = "go-on"
	onFailHold = "hold"
)

const (
	cmdTest      = "test"
	cmdCheck     = "check"
	cmdLive      = "live"
	cmdDo        = "do"
	cmdHierarchy = "hierarchy"
	cmdDevices   = "devices"
	cmdCA        = "ca"
	cmdMocks     = "mocks"
	cmdExplain   = "explain"
	cmdStatus    = "status"
	cmdSkills    = "skills"
)

// commandNames are rayo's own, as rayo help <command> takes them.
var commandNames = []string{cmdTest, cmdCheck, cmdExplain, cmdStatus, cmdLive, cmdDo, cmdHierarchy, cmdDevices, cmdCA, cmdMocks, cmdSkills, "schema"}

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
		text("device", "serial", "when several are connected; else ANDROID_SERIAL, else the only one", func(o *options) *string { return &o.device }),
		flag("emulator", "never a real device, even if it is the only one", func(o *options) *bool { return &o.emulator }),
	}},
	{[]string{cmdTest, cmdCheck}, "which flows", []option{
		{name: "changed-only", about: "only the flows of which a file changed: their own, one they run, or a mock; as --since HEAD", set: func(o *options, _ string) error {
			if o.since == "" {
				o.since = "HEAD"
			}
			return nil
		}},
		text("since", "git ref", "only the flows of which a file changed since then, also not committed", func(o *options) *string { return &o.since }),
	}},
	{[]string{cmdHierarchy, cmdDo}, "", []option{
		text("find", "text", "of the screen, only the elements most like it, instead of all", func(o *options) *string { return &o.find }),
	}},
	{[]string{cmdTest, cmdLive, cmdDo, cmdExplain, cmdStatus}, "", []option{
		text("output", "folder", "where failures and screenshots go (default .rayo)", func(o *options) *string { return &o.output }),
	}},
	{[]string{cmdTest, cmdLive}, "", []option{
		{name: "timeout-scale", value: "factor", about: fmt.Sprintf("multiply the default waits, not a step's own timeout, from %v to %v (default 1)", suite.LowestScale, suite.HighestScale), set: func(o *options, v string) error {
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
	{[]string{cmdTest, cmdCheck}, "which flows of them", []option{
		list("only", "only the flows with these names", func(o *options) *[]string { return &o.only }),
		list("include-tags", "only flows with one of these tags", func(o *options) *[]string { return &o.include }),
		list("exclude-tags", "not flows with one of these tags", func(o *options) *[]string { return &o.exclude }),
		flag("rerun-failed", "only those that haven't passed since they last failed", func(o *options) *bool { return &o.rerunFailed }),
		{name: "min-flows", value: "n", about: "fail (exit 2) if fewer flows than this are chosen, as a typo in a tag or a name would; of all the parts with --shard, and not with --rerun-failed or --resume (default: the suite's minFlows, else none)", set: func(o *options, v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("--min-flows takes a number from 1, not %q", v)
			}
			o.minFlows = n
			return nil
		}},
		{name: "shard", value: "i/n", about: "only part i of n, for n machines that run the same flows: each flow is in one part, by its path, whatever the order; it writes to .rayo/shard-i-of-n unless --output or the suite says", set: func(o *options, v string) error {
			i, n, ok := strings.Cut(v, "/")
			var bad1, bad2 error
			o.shard, bad1 = strconv.Atoi(i)
			o.shards, bad2 = strconv.Atoi(n)
			if !ok || bad1 != nil || bad2 != nil || o.shards < 1 || o.shard < 1 || o.shard > o.shards {
				o.shard, o.shards = 0, 0
				return fmt.Errorf("--shard takes i/n, a part of how many, such as 2/4, not %q", v)
			}
			return nil
		}},
	}},
	{[]string{cmdTest, cmdCheck, cmdLive}, "", []option{
		{short: "e", name: "env", value: "KEY=VALUE", about: "a variable for the flows; over the suite's env, which is over a flow's own", set: func(o *options, v string) error {
			k, value, ok := strings.Cut(v, "=")
			if !ok {
				return errors.New("--env takes KEY=VALUE")
			}
			o.runner.Env[k] = value
			return nil
		}},
	}},
	{[]string{cmdTest, cmdCheck}, "as the suite says, for one run", []option{
		{name: "retries", value: "n", about: fmt.Sprintf("try again what fails, every flow, also one the suite gives its own retries (default %d, or the suite's)", suite.DefaultRetries), set: func(o *options, v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return fmt.Errorf("--retries takes a number, not %q", v)
			}
			o.retries = n
			return nil
		}},
	}},
	{[]string{cmdTest}, "how they run", []option{
		flag("fail-fast", "stop at the first failure", func(o *options) *bool { return &o.failFast }),
		flag("flaky-fails-build", "exit 4 if every flow passed but some only on a retry (default: exit 0)", func(o *options) *bool { return &o.flakyFails }),
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
		{name: "screens-before", value: "n", about: "keep the screen after each of the last n steps that passed, and save them with a failure as screens-before.txt: how the app got there; each costs a read of the screen (default 0)", set: func(o *options, v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 || n > 20 {
				return fmt.Errorf("--screens-before takes 0 to 20, not %q", v)
			}
			o.runner.ScreensBefore = n
			return nil
		}},
		flag("continuous", "run again whenever a file changes", func(o *options) *bool { return &o.continuous }).withShort("c"),
		choice("on-fail", "what", "go-on (default): retry it as allowed, then the next flow; hold: keep the flow where it failed, the app as it is, until rayo do says how to go on, so that it is fixed in its files and run again from there; see rayo help hold", func(o *options) *string { return &o.onFail }, onFailGoOn, onFailHold),
		{name: "hold-timeout", value: "duration", about: "with --on-fail hold, leave a flow failed if no command comes for this long, such as 30m (default 5m)", set: func(o *options, v string) error {
			d, err := time.ParseDuration(v)
			if err != nil || d <= 0 {
				return fmt.Errorf("--hold-timeout takes how long, such as 20m, not %q", v)
			}
			o.holdTimeout = d
			return nil
		}},
	}},
	{[]string{cmdTest}, "from where, after a fix; see rayo help resume", []option{
		flag("resume", "each flow that failed last time, from the step that failed, with the app as it is and the mocks' scenarios as they were then", func(o *options) *bool { return &o.resume }),
		{name: "from", value: "step", about: "begin one flow at a step: file:line as rayo explain's at: says, a line of its own file, or a text of one step such as pickDate; the app is as it is, onFlowStart doesn't run", set: func(o *options, v string) error {
			o.from = v
			return flow.CheckStepID(v)
		}},
		{name: "until", value: "step", about: "stop one flow after a step, given as --from takes it, leaving the app there; onFlowComplete doesn't run", set: func(o *options, v string) error {
			o.until = v
			return flow.CheckStepID(v)
		}},
		{name: "mock-state", value: "a=s,b=t", about: "with --resume or --from, the scenarios' states to begin with, over those of the last run", set: func(o *options, v string) error {
			for _, part := range strings.Split(v, ",") {
				name, state, ok := strings.Cut(part, "=")
				if !ok || name == "" || state == "" {
					return fmt.Errorf("--mock-state takes scenario=state, such as booking=booked, not %q", part)
				}
				o.mockState = append(o.mockState, flow.Scenario{Name: name, State: state})
			}
			return nil
		}},
	}},
	{[]string{cmdTest, cmdCheck}, "what is written, as the suite's output", []option{
		flag("video", "a video of each flow whose header doesn't say record, as output: { video: true }", func(o *options) *bool { return &o.video }),
		choice("keep", "when", "the files of every try (all, default), only of the tries that failed, or none", func(o *options) *string { return &o.keep }, suite.KeepAll, suite.KeepFailed, suite.KeepNone),
		choice("clean", "when", "remove the files of earlier runs: none (default), keeps them; auto, a whole run begins with none, a partial one removes a flow's as it starts; all, every run begins with none", func(o *options) *string { return &o.clean }, report.Cleans...),
		{name: "artifacts", value: "a,b", about: "the kinds of file written: video, screenshot, logcat, screen, requests (default all)", set: func(o *options, v string) error {
			kinds := strings.Split(v, ",")
			if err := flow.CheckArtifacts(kinds); err != nil {
				return fmt.Errorf("--artifacts %w", err)
			}
			o.runner.Artifacts = kinds
			return nil
		}},
		{name: "name-template", value: "template", about: "how the files are named in the output folder (default {flow}_{attempt}_{kind}.{ext})", set: func(o *options, v string) error {
			if err := flow.CheckTemplate(v); err != nil {
				return fmt.Errorf("--name-template %w", err)
			}
			o.runner.NameTemplate = v
			return nil
		}},
	}},
	{[]string{cmdTest}, "what comes out", []option{
		flag("ignore-lock", "run even if another rayo seems to write to the output folder, as when its running.json was left by one that is gone", func(o *options) *bool { return &o.ignoreLock }),
		choice("detail", "how much", "printed: brief, the verdict of a failure; normal (default), also why, where, the nearest elements and the calls, and every step of a single flow; full, also the screen, every step, and whole files", func(o *options) *string { return &o.detail }, flow.Details...),
		{name: "failure-window", value: "seconds", about: fmt.Sprintf("a failure's log and calls begin this long before the failing step (default %d)", int(flow.DefaultWindow.Seconds())), set: func(o *options, v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("--failure-window takes a number of seconds from 1, not %q", v)
			}
			o.window = n
			return nil
		}},
		{name: "results", value: "file", about: "write the results: .txt, .json, .xml (JUnit) or .md; may be repeated", set: func(o *options, v string) error {
			o.results = append(o.results, v)
			if _, err := report.FormatOf(v); err != nil {
				return fmt.Errorf("--results %w", err)
			}
			return nil
		}},
		{name: "results-template", value: "file", about: "write the results with that extension as this file says, with {suite}, {passed}, {failed}, {flaky} and the like, and {flows}...{end}", set: func(o *options, v string) error {
			if err := report.CheckTemplate(v); err != nil {
				return fmt.Errorf("--results-template %w", err)
			}
			o.resultsTemplate = v
			return nil
		}},
		{name: "show", value: "a,b", about: "the flows the results list: failed, flaky, slow or all (default all)", set: func(o *options, v string) error {
			list := strings.Split(v, ",")
			if err := report.CheckShow(list); err != nil {
				return fmt.Errorf("--show %w", err)
			}
			o.show = list
			return nil
		}},
		{name: "slowest", value: "n", about: fmt.Sprintf("how many flows the slow list has (default %d)", report.DefaultSlowest), set: func(o *options, v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("--slowest takes a number from 1, not %q", v)
			}
			o.slowest = n
			return nil
		}},
		flag("timings", "while it runs: each step's time, each runFlow's total, the slowest steps and flows, and a note while a step hangs", func(o *options) *bool { return &o.runner.Timings }),
	}},
	{[]string{cmdTest, cmdLive}, "mocks from the real backend", []option{
		text("capture", "folder", "call the real backend, and write what it answers as mocks; in live, on .quit, without the calls of steps that failed", func(o *options) *string { return &o.runner.Capture }),
		{name: "capture-update", value: "folder", about: "capture into folder/.update, and say what changed from the mocks in folder; rayo mocks apply takes it. Given a suite instead, the folder is the flow's mock", set: func(o *options, v string) error {
			if strings.HasSuffix(v, ".suite.yaml") || strings.HasSuffix(v, ".rayo.yaml") { // the flow's mock, once it is known
				o.args, o.updateOwn = append(o.args, v), true
				return nil
			}
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
		flag("ignore-path-ids", "match UUIDs and ULIDs in paths by their form, as they are new on each run", func(o *options) *bool { return &o.runner.Matching.PathIDs }),
		{name: "stateless", value: "a,b", about: "paths such as api.shop.com/quotes/** whose calls only read, whatever the method: they don't change the state (* any text but /, ** any)", set: func(o *options, v string) error {
			for _, g := range strings.Split(v, ",") {
				if _, err := mock.Glob(g); err != nil {
					return fmt.Errorf("--stateless: %w", err)
				}
			}
			o.runner.Matching.Stateless = strings.Split(v, ",")
			return nil
		}},
	}},
	{[]string{cmdTest, cmdLive, cmdCA}, "how calls reach the mock", []option{
		{name: "network", value: "way", about: "interceptor, the app sends them, with code in its debug build (default, or the flow's); proxy, the device's proxy does, no code; see rayo help connect-mocks", set: func(o *options, v string) error {
			if v != flow.Interceptor && v != flow.Proxy {
				return fmt.Errorf("--network takes interceptor or proxy, not %q", v)
			}
			o.runner.Network = v
			return nil
		}},
		{name: "ignore-unmocked", value: "a,b", about: "calls without a mock that don't fail the flow, answered 404: graphql:Operation, host/path, host or /path (* any text but /, ** any)", set: func(o *options, v string) error {
			u := &mock.Unmocked{}
			for _, part := range strings.Split(v, ",") {
				i, err := mock.ParseIgnored(part)
				if err != nil {
					return fmt.Errorf("--ignore-unmocked: %w", err)
				}
				u.Ignore = append(u.Ignore, i)
			}
			o.runner.Unmocked = o.runner.Unmocked.Merge(u)
			return nil
		}},
		text("ca", "folder", "with a proxy, the CA it signs with, its ca.pem and ca-key.pem (default .rayo/ca next to the suite or above it, else ~/.rayo); see rayo ca", func(o *options) *string { return &o.runner.CA }),
		list("hosts", "with a proxy, the hosts to open, and their subdomains (default: capturing, all but Google's, Firebase's, analytics and crash reports; else those of the mocks)", func(o *options) *[]string { return &o.runner.Hosts }),
	}},
	{[]string{cmdLive}, "", []option{
		text("app", "package", "of a flow that isn't there yet", func(o *options) *string { return &o.app }),
		list("mock", "its mocks, relative to it", func(o *options) *[]string { return &o.mocks }),
		flag("no-run", "don't begin by running the steps it has", func(o *options) *bool { return &o.noRun }),
		flag("serve", "only take commands from rayo do, until .quit", func(o *options) *bool { return &o.serve }),
		{name: "idle-timeout", value: "duration", about: "end the session if no command comes for this long, such as 1h; steps not saved are printed (default 5m)", set: func(o *options, v string) error {
			d, err := time.ParseDuration(v)
			if err != nil || d <= 0 {
				return fmt.Errorf("--idle-timeout takes how long, such as 30m, not %q", v)
			}
			o.idle = d
			return nil
		}},
	}},
	{[]string{cmdExplain}, "", []option{
		flag("latest", "only the flows of the newest run in failed.json, not those still there from earlier ones", func(o *options) *bool { return &o.latest }),
	}},
	{[]string{cmdMocks}, "", []option{
		list("only", "the changes whose name has one of these, such as graphql:Wallet or GET /profile (default all)", func(o *options) *[]string { return &o.take }),
		flag("prune", "also take out the mocks that weren't called", func(o *options) *bool { return &o.prune }),
	}},
	{[]string{cmdSkills}, "", []option{
		list("agent", "only for these: claude, codex (default every one found)", func(o *options) *[]string { return &o.agents }),
		text("project", "folder", "in a project's .claude or .codex, not your home's", func(o *options) *string { return &o.project }),
	}},
	{[]string{cmdDo}, "", []option{
		{name: "screen", value: "changes", optional: true, about: "also what is on screen afterwards; --screen=changes, only the lines that appeared (+) or went (-) since the screen the session last gave", set: func(o *options, v string) error {
			if v != "" && v != "changes" {
				return fmt.Errorf("--screen takes nothing, or =changes, not %q", v)
			}
			o.screen, o.screenChanges = true, v == "changes"
			return nil
		}},
		{name: "ignore-screen", value: "regex", about: "with --screen, leave out the lines that match, such as a clock's; says how many", set: func(o *options, v string) error {
			if _, err := regexp.Compile(v); err != nil || v == "" {
				return fmt.Errorf("--ignore-screen takes a regular expression, not %q", v)
			}
			o.ignoreScreen = v
			return nil
		}},
		{name: "timeout", value: "duration", about: "how long to wait for the answer, such as 30s (default as long as it takes); the session goes on", set: func(o *options, v string) error {
			d, err := time.ParseDuration(v)
			if err != nil || d <= 0 {
				return fmt.Errorf("--timeout takes how long, such as 30s or 2m, not %q", v)
			}
			o.doTimeout = d
			return nil
		}},
		flag("discard", "with .quit, end even if steps were not saved", func(o *options) *bool { return &o.discard }),
	}},
	{[]string{cmdTest, cmdCheck, cmdExplain, cmdStatus, cmdDevices, cmdHierarchy, cmdDo, cmdMocks}, "", []option{
		choice("format", "how", "human (default); json, for a program, see rayo schema; ndjson, the same on one line, and for test an event a line as it runs",
			func(o *options) *string { return &o.format }, report.Formats...),
	}},
}

const usage = `rayo - runs flows on Android, fast

New here, or driving rayo from a program? Start with: rayo help agents

  rayo test [options] <flow, folder or suite>...   run flows; a suite is a *.suite.yaml
  rayo check <flow, folder or suite>...            check flows without a device, and say which would run
  rayo explain [<run folder, .zip or suite>] [<flow>]  why the flows of the last run failed, in a few lines
  rayo status [options] <flow, folder or suite>... whether each flow passed the last time it ran, in any run
  rayo live [options] <flow>                       write a flow as it runs, command by command
  rayo live [options] <suite> <flow>               the same, with the suite's env, output and CA
  rayo do [options] <commands>                     send commands to the live session; rayo do .help
  rayo hierarchy [options]                         what is on screen, and how to refer to it
  rayo devices                                     connected devices
  rayo mocks apply [options] <folder>              take the changes of rayo test --capture-update into the mocks
  rayo mocks import [options] <file.har> <folder>  write the calls of a HAR file, such as mitmproxy's, as mocks
  rayo mocks check <folder>                        mappings that never answer, and bodies no mapping uses
  rayo ca new [<folder>]                           a CA for the project, in .rayo/ca, to commit with the flows
  rayo ca export <file>                            the certificate the proxy signs with, for the app's debug build
  rayo skills add [options]                        the rayo skill, for Claude Code and Codex: how to write, run, fix and mock
  rayo skills remove [options]                     take it out
  rayo skills list [options]                       where it is, and whether it is of this version
  rayo help <command>                              what a command takes, of a flow or of rayo; or <command> --help
  rayo help <topic>                                flow, suite, json, all (everything on one page), or a topic below
  rayo help search <word>...                       the lines of all the help that have the words
  rayo schema <name>                               a JSON Schema: flow, suite, results, or of --format json
  rayo version
`

// suiteKeys are the keys of a suite that options override, by the option's name. A test checks
// that each is a suite's, and that every key of a suite has an option or a reason not to.
var suiteKeys = map[string]string{
	"output": "output.folder", "timeout-scale": "timeoutScale", "env": "env", "retries": "retries", "min-flows": "minFlows",
	"video": "output.video", "keep": "output.keep", "artifacts": "output.artifacts", "name-template": "output.nameTemplate", "clean": "output.clean",
	"detail": "detail", "failure-window": "failureWindow", "results": "results", "results-template": "resultsTemplate",
	"show": "show", "slowest": "slowest", "flaky-fails-build": "flakyFailsBuild", "format": "format",
	"capture-from": "capture.from", "graphql": "capture.graphql", "graphql-path": "capture.graphqlPath", "rest": "capture.rest",
	"ignore-params": "capture.ignoreParams", "ignore-body": "capture.ignoreBody", "timestamps": "capture.timestamps",
	"capture-headers": "capture.captureHeaders", "ignore-host": "capture.ignoreHost", "ignore-path-ids": "capture.ignorePathIds",
	"stateless": "capture.stateless", "redact-value": "capture.redactValue", "hosts": "capture.hosts",
	"network": "network", "ignore-unmocked": "unmocked.ignore", "ca": "ca", "screens-before": "output.screensBefore",
}

func init() {
	for i := range sections {
		for j := range sections[i].options {
			p := &sections[i].options[j]
			if slices.Contains(sections[i].commands, cmdTest) || slices.Contains(sections[i].commands, cmdCheck) {
				p.key = suiteKeys[p.name]
			}
		}
	}
}

// help is made from the table.
func help() string {
	var b strings.Builder
	b.WriteString(usage)
	b.WriteString("\nTopics, rayo help <topic>\n")
	for _, t := range topics {
		fmt.Fprintf(&b, "  %-26s %s\n", t.name, t.about)
	}
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

// helpOf is how to use a command of rayo's, and its options; empty if it isn't one.
func helpOf(command string) string {
	if !slices.Contains(commandNames, command) {
		return ""
	}
	var lines strings.Builder
	for _, line := range strings.Split(usage, "\n") {
		if strings.HasPrefix(line, "  rayo "+command+" ") {
			lines.WriteString(line + "\n")
		}
	}
	var b strings.Builder
	for _, s := range sections {
		if slices.Contains(s.commands, command) {
			for _, p := range s.options {
				b.WriteString(p.help())
			}
		}
	}
	session := ""
	if command == cmdLive || command == cmdDo {
		session = "\nIn a session, typed or as rayo do <command>:\n" + live.Help
	}
	if b.Len() == 0 {
		return lines.String() + session
	}
	return lines.String() + "\nOptions of " + command + ":\n" + b.String() + session
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
	about := p.about
	if p.key != "" {
		about += " (suite: " + p.key + ")"
	}
	return fmt.Sprintf("  %-26s %s\n", name, about)
}

// find is the option with a name as written, if the command takes it.
func find(command, written string) (option, error) {
	var takenBy, own []string // the commands that take it; the options of this command
	for _, s := range sections {
		for _, p := range s.options {
			if slices.Contains(s.commands, command) {
				own = append(own, "--"+p.name)
			}
			if written != "--"+p.name && (p.short == "" || written != "-"+p.short) {
				continue
			}
			if slices.Contains(s.commands, command) {
				return p, nil
			}
			takenBy = append(takenBy, s.commands...)
		}
	}
	slices.Sort(takenBy)
	if takenBy = slices.Compact(takenBy); command == cmdCheck && slices.Contains(takenBy, cmdTest) {
		return option{}, fmt.Errorf("%s only matters while flows run on a device; rayo check runs nothing, so drop it there and keep it for rayo test", written)
	}
	if len(takenBy) > 0 {
		return option{}, fmt.Errorf("%s is not an option of %s, but of %s; see rayo help %s", written, command, strings.Join(takenBy, ", "), command)
	}
	near := schema.Nearest(written, own)
	for _, name := range own { // --from-step, --resume-failed: the option and a word more
		if near == "" && len(name) > 4 && (strings.HasPrefix(written, name+"-") || strings.HasPrefix(name, written+"-")) {
			near = name
		}
	}
	if near != "" && strings.HasPrefix(written, "--") {
		return option{}, fmt.Errorf("unknown option %s, did you mean %s? See rayo help %s", written, near, command)
	}
	return option{}, fmt.Errorf("unknown option %s; see rayo help %s", written, command)
}

func parse(command string, args []string) (*options, error) {
	o := &options{retries: -1, runner: flow.Options{Env: map[string]string{}, MockPort: 8080}}
	for i, a := range args { // first, so that what is wrong with the rest is told as asked
		if v, ok := strings.CutPrefix(a, "--format="); ok || a == "--format" && i+1 < len(args) {
			if !ok {
				v = args[i+1]
			}
			if slices.Contains(report.Formats, v) {
				printing = v
			}
		}
	}
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

// printing is the format of what is printed: of --format, or else of the suite.
var printing = report.Human

// human is where what is for people goes: nowhere, if a program reads it.
func human() io.Writer {
	if printing == report.Human {
		return os.Stdout
	}
	return io.Discard
}

// emit prints something for a program, as --format says.
func emit(v any) error {
	data, err := report.Encode(v, printing == report.JSON)
	if err == nil {
		_, err = os.Stdout.Write(data)
	}
	return err
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
	if o.detail == flow.Brief {
		return false
	}
	return o.detail == flow.Full || o.runner.Profile || o.runner.Timings || len(flows) == 1
}

// into settles the output folder: the option, or else the suite's.
func (o *options) into(folder string) {
	if o.output == "" {
		o.output = folder
	}
	if o.output == "" && o.shards > 0 { // two parts on one machine don't write over each other
		o.output = filepath.Join(".rayo", fmt.Sprintf("shard-%d-of-%d", o.shard, o.shards))
	}
	if o.output == "" {
		o.output = ".rayo"
	}
}
