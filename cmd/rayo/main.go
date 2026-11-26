// Command rayo runs end-to-end flows on Android, fast.
package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/nanihadesuka/rayo/internal/adb"
	"github.com/nanihadesuka/rayo/internal/agent"
	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/live"
	"github.com/nanihadesuka/rayo/internal/mock"
	"github.com/nanihadesuka/rayo/internal/report"
	"github.com/nanihadesuka/rayo/internal/schema"
	"github.com/nanihadesuka/rayo/internal/suite"
)

const version = "3.3.1"

func main() {
	if len(os.Args) < 2 {
		fmt.Print(help())
		os.Exit(report.ExitConfig)
	}
	// SIGTERM is how CI cancels a job, and docker stop a container: as Ctrl-C, the device is put back and the results written.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	context.AfterFunc(ctx, stop) // a second one doesn't wait
	err := run(ctx, os.Args[1], os.Args[2:])
	code := exitOf(err)
	switch {
	case ctx.Err() != nil:
		fmt.Fprintln(os.Stderr, "interrupted")
		os.Exit(report.ExitInterrupted)
	case err == nil || errors.Is(err, errShown):
	case printing == report.JSON:
		_ = emit(report.ErrorsOf(err, code))
	case printing == report.NDJSON:
		_ = emit(report.ErrorEvent(err, code))
	default:
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	os.Exit(code)
}

// exitError says which exit code an error is for, see report.Exits.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

// exiting gives an error its exit code, unless it has one; none stays none.
func exiting(code int, err error) error {
	var known *exitError
	if err == nil || errors.As(err, &known) {
		return err
	}
	return &exitError{code, err}
}

// written settles what a run writes, and where: an option overrides the suite's output.
func (o *options) written(group *suite.Suite) {
	video := o.video
	if group != nil {
		g := group.Output
		if g.Folder != "" {
			o.into(group.Relative(g.Folder))
		}
		o.keep = cmp.Or(o.keep, g.Keep)
		o.runner.NameTemplate = cmp.Or(o.runner.NameTemplate, g.NameTemplate)
		if o.runner.Artifacts == nil {
			o.runner.Artifacts = g.Artifacts
		}
		video = video || g.Video != nil && *g.Video
	}
	o.into("")
	o.keep = cmp.Or(o.keep, suite.KeepAll)
	o.runner.Recording = &flow.Recording{On: video}
}

// ofDevice is an error of the device, or of reaching it.
func ofDevice(err error) error {
	if errors.Is(err, os.ErrDeadlineExceeded) { // the device took the command and never answered
		err = fmt.Errorf("the device didn't answer, so it is hung or very slow: restart it (an emulator: a cold boot), check it with rayo devices, then run again (%w)", err)
	}
	return exiting(report.ExitDevice, err)
}

// exitOf is the exit code for an error: one that doesn't say is of something written.
func exitOf(err error) int {
	var known *exitError
	switch {
	case err == nil:
		return report.ExitPassed
	case errors.As(err, &known):
		return known.code
	case errors.Is(err, errShown):
		return report.ExitFailed
	}
	return report.ExitConfig
}

func run(ctx context.Context, command string, args []string) error {
	if asksHelp(command, args) {
		text, err := helpOn(command)
		fmt.Print(text)
		return err
	}
	switch command {
	case cmdTest:
		return test(ctx, args)
	case cmdCheck:
		return check(args)
	case cmdExplain:
		return explain(args)
	case cmdLive:
		return runLive(ctx, args)
	case cmdDo:
		return runDo(args)
	case cmdHierarchy:
		return hierarchy(ctx, args)
	case cmdDevices:
		o, err := parse(cmdDevices, args)
		if err == nil {
			err = o.none(cmdDevices)
		}
		if err != nil {
			return err
		}
		list, err := adb.Devices()
		switch {
		case err != nil:
			return ofDevice(err)
		case printing != report.Human:
			return emit(report.Devices{SchemaVersion: report.SchemaVersion, Devices: report.List(list)})
		}
		fmt.Println(strings.Join(list, "\n"))
		return nil
	case cmdCA:
		return commandCA(args)
	case cmdMocks:
		return applyMocks(args)
	case "version", "--version", "-v":
		fmt.Println("rayo " + version)
		return nil
	case "help", "--help", "-h":
		if len(args) == 0 || args[0] == "topics" {
			fmt.Print(help())
			return nil
		}
		if args[0] == "search" {
			text, err := helpSearch(args[1:])
			fmt.Print(text)
			return err
		}
		text, err := helpOn(args[0])
		fmt.Print(text)
		return err
	case "schema":
		return printSchema(args)
	}
	if near := schema.Nearest(command, append(slices.Clone(commandNames), "help", "version")); near != "" {
		return fmt.Errorf("unknown command %q, did you mean %s? See rayo help", command, near)
	}
	return fmt.Errorf("unknown command %q, see rayo help", command)
}

func check(args []string) error {
	o, err := parse(cmdCheck, args)
	if err != nil {
		return err
	}
	flows, group, err := suite.Load(o.args)
	if err == nil {
		flows, err = o.changed(flows)
	}
	if err != nil {
		return err
	}
	o.written(group)
	o.suite = group
	if flows, err = o.choose(flows); err != nil { // those rayo test would run
		return err
	}
	// what would do nothing, as rayo test says it
	if err := o.checkKept(); err != nil {
		return err
	}
	if err := o.checkUseful(flows); err != nil {
		return err
	}
	list := plan(flows, group)
	if o.retries >= 0 { // as rayo test would
		for i := range list {
			list[i].Retries = o.retries
		}
	}
	if printing != report.Human {
		return emit(report.Check{SchemaVersion: report.SchemaVersion, OK: true, Count: len(flows), Flows: list})
	}
	if len(flows) == 1 {
		fmt.Println("1 flow is fine")
	} else {
		fmt.Printf("%d flows are fine\n", len(flows))
	}
	env := maps.Clone(o.runner.Env)
	if group != nil {
		for k, v := range group.Env {
			if _, set := env[k]; !set {
				env[k] = v
			}
		}
	}
	pairs := func(vars map[string]string) string {
		var all []string
		for _, k := range slices.Sorted(maps.Keys(vars)) {
			all = append(all, k+"="+vars[k])
		}
		return strings.Join(all, ", ")
	}
	for i, p := range list {
		line := fmt.Sprintf("  %s  %s  retries: %d", p.Name, p.File, p.Retries)
		if len(p.Tags) > 0 {
			line += "  tags: " + strings.Join(p.Tags, ", ")
		}
		if len(p.Mocks) > 0 {
			line += "  mocks: " + strings.Join(p.Mocks, ", ")
		}
		own := map[string]string{}
		for k, v := range flows[i].Env {
			if _, over := env[k]; !over {
				own[k] = v
			}
		}
		if len(own) > 0 {
			line += "  own env: " + pairs(own)
		}
		fmt.Println(line)
	}
	kept := map[string]string{suite.KeepAll: "keeps the files of every try", suite.KeepFailed: "keeps only the files of the tries that failed", suite.KeepNone: "keeps no files"}[o.keep]
	if o.keep != suite.KeepNone && o.runner.Artifacts != nil {
		kept += " (only " + strings.Join(o.runner.Artifacts, ", ") + ")"
	}
	video := "a video only of flows whose header says record"
	switch {
	case o.runner.Recording.On && o.keep == suite.KeepFailed:
		video = "a video of each flow, kept only if its try failed"
	case o.runner.Recording.On:
		video = "a video of each flow"
	}
	fmt.Printf("a run writes to %s, %s, with %s\n", o.output, kept, video)
	if o.keep != suite.KeepNone && len(flows) > 0 {
		a := flow.Attempt{Out: o.output, Flow: flow.FolderName(flows[0].Name)}
		retry := a
		retry.Try = 1
		fmt.Printf("such as %s, and of a retry %s\n", filepath.ToSlash(flow.FileName(o.runner.NameTemplate, a, "failure", "png")),
			filepath.ToSlash(flow.FileName(o.runner.NameTemplate, retry, "logcat", "txt")))
	}
	results := o.results
	if results == nil && group != nil {
		for _, file := range group.Results {
			results = append(results, group.Relative(file))
		}
	}
	if len(results) > 0 {
		fmt.Println("results:", filepath.ToSlash(strings.Join(results, ", ")))
	}
	if len(env) > 0 {
		fmt.Println("env, over each flow's own:", pairs(env))
	}
	return nil
}

func hierarchy(ctx context.Context, args []string) error {
	o, err := parse(cmdHierarchy, args)
	if err == nil {
		err = o.none(cmdHierarchy)
	}
	if err != nil {
		return err
	}
	d, err := adb.Pick(o.device, o.emulator)
	if err != nil {
		return ofDevice(err)
	}
	a, err := agent.Start(d)
	if err != nil {
		return ofDevice(err)
	}
	defer a.Close()
	r, err := a.Do(ctx, "tree", nil, 10*time.Second)
	switch {
	case err != nil:
		return ofDevice(err)
	}
	if o.find != "" {
		r.Out = findOn(r.Out, o.find)
	}
	r.Out = flow.CapScreen(r.Out, flow.MostShown)
	if printing != report.Human {
		return emit(report.Hierarchy{SchemaVersion: report.SchemaVersion, Screen: report.Lines(r.Out)})
	}
	fmt.Print(r.Out)
	return nil
}

func test(ctx context.Context, args []string) (err error) {
	o, err := parse(cmdTest, args)
	if err != nil {
		return err
	}
	all, group, err := suite.Load(o.args)
	if err != nil {
		return err
	}
	o.suite = group
	if all, err = o.changed(all); err != nil {
		return err
	}
	title := strings.Join(o.args, " ")
	if group != nil {
		if group.Name != "" {
			title = group.Name
		}
		if o.format == "" && group.Format != "" {
			printing = group.Format
		}
		if o.runner.Scale == 0 {
			o.runner.Scale = group.TimeoutScale
		}
		for k, v := range group.Env {
			if _, set := o.runner.Env[k]; !set {
				o.runner.Env[k] = v
			}
		}
		o.flakyFails = o.flakyFails || group.FlakyFailsBuild
		o.detail = cmp.Or(o.detail, group.Detail)
		if o.window == 0 {
			o.window = group.FailureWindow
		}
		if o.show == nil {
			o.show = group.Show
		}
		if o.slowest == 0 {
			o.slowest = group.Slowest
		}
		if o.runner.CA == "" && group.CA != "" {
			o.runner.CA = group.Relative(group.CA)
		}
		if o.resultsTemplate == "" && group.ResultsTemplate != "" {
			o.resultsTemplate = group.Relative(group.ResultsTemplate)
		}
		if o.results == nil {
			for _, file := range group.Results {
				o.results = append(o.results, group.Relative(file))
			}
		}
	}
	if len(all) == 0 {
		if err := o.enough(0); err != nil {
			return err
		}
		return o.noneChosen(title, "no flow changed since "+o.since)
	}
	for _, file := range o.results {
		if format, _ := report.FormatOf(file); format == report.JSON { // it has been checked
			o.runner.JSON = true
		}
	}
	o.written(group)
	if err := o.findCA(group, true); err != nil {
		return err
	}
	o.detail = cmp.Or(o.detail, flow.Normal)
	o.runner.Detail, o.runner.Window = o.detail, time.Duration(o.window)*time.Second
	if err := o.checkKept(); err != nil {
		return err
	}
	if o.keep == suite.KeepNone {
		o.runner.Artifacts = []string{}
	}
	if err := o.checkStarts(); err != nil {
		return err
	}
	flows, err := o.choose(all)
	if err != nil {
		return err
	}
	if len(flows) == 0 { // only with --rerun-failed or --shard
		switch {
		case o.shards > 0 && !o.rerunFailed:
			return o.noneChosen(title, fmt.Sprintf("no flows in part %d of %d", o.shard, o.shards))
		case o.resume:
			return o.nothingToResume()
		case o.shards > 0:
			return o.noneChosen(title, fmt.Sprintf("nothing failed last time in part %d of %d", o.shard, o.shards))
		}
		return o.noneChosen(title, "nothing failed last time")
	}
	if err := o.settleStarts(flows); err != nil {
		return err
	}
	if err := o.suiteCapture(flows); err != nil {
		return err
	}
	if err := o.checkCapture(flows); err != nil {
		return err
	}
	if err := o.checkUseful(flows); err != nil {
		return err
	}
	began := time.Now()
	d, err := adb.Pick(o.device, o.emulator)
	if err != nil {
		return ofDevice(err)
	}
	picked := time.Since(began)
	if group != nil {
		if err := group.Install(d, o.steps(flows) && printing == report.Human); err != nil {
			return ofDevice(err)
		}
	}
	if group != nil && group.Device != nil {
		undo, e := d.Apply(group.Device)
		if e != nil {
			return ofDevice(e)
		}
		defer func() { // after the agent, which begins and ends with the screen like this
			if e := undo(); e != nil {
				err = errors.Join(err, fmt.Errorf("couldn't put the device back: %w", e))
			}
		}()
	}
	start := time.Now()
	installed := start.Sub(began) - picked
	s := &session{o: o, device: d, reporter: newConsole(human())}
	if err := s.start(); err != nil {
		return ofDevice(err)
	}
	defer func() { s.agent.Close() }() // the one there is by then
	if o.steps(flows) {
		fmt.Fprintf(human(), "%s (%s), ready in %d ms\n", d.Serial, d.Why, time.Since(start).Milliseconds())
	}
	if o.runner.Timings {
		fmt.Fprintf(human(), "  setup: pick %d ms, install and device %d ms, agent and mocks %d ms; %d adb calls\n",
			picked.Milliseconds(), installed.Milliseconds(), time.Since(start).Milliseconds(), adb.Calls())
	}

	watch := watcher{}
	if group != nil {
		watch[group.Path] = true
	}
	watch.app(group)
	for {
		err = s.runAll(ctx, flows, title)
		if !o.continuous || ctx.Err() != nil {
			return err
		}
		if err != nil && !errors.Is(err, errShown) {
			fmt.Fprintln(human(), err)
		}
		if s.agent.Dead() != nil {
			return ofDevice(s.agent.Dead())
		}
		fmt.Fprintln(human(), "\nwaiting for changes...")
		for {
			if err := watch.await(ctx, flows); err != nil {
				return err
			}
			again, changed, err := suite.Load(o.args)
			if err == nil {
				o.suite = changed
				again, err = o.choose(again)
			}
			if err == nil && changed != nil { // the same APK isn't installed again
				watch.app(changed)
				if err = changed.Install(d, printing == report.Human); err != nil {
					err = fmt.Errorf("couldn't install the app: %w", err)
				}
			}
			if err == nil {
				flows = again
				break
			}
			fmt.Fprintln(human(), "error:", err)
		}
	}
}

// suiteCapture settles a capture of a suite's flow: into its own mock, for --capture-update of a suite,
// and as the suite says for what the options don't.
func (o *options) suiteCapture(flows []*flow.Flow) error {
	if o.updateOwn {
		if len(flows) != 1 {
			return errors.New("--capture-update runs one flow, once: say which with --only")
		}
		f := flows[0]
		if len(f.Mock) == 0 {
			return fmt.Errorf("--capture-update: %s has no mock to update", f.Name)
		}
		folder := f.Mock[len(f.Mock)-1].Value // its own, after those shared
		if strings.Contains(folder, "${") {
			return fmt.Errorf("--capture-update: the mock of %s is %s; give the folder", f.Name, folder)
		}
		if !filepath.IsAbs(folder) {
			folder = filepath.Join(filepath.Dir(f.Path), filepath.FromSlash(folder))
		}
		wd, _ := os.Getwd()
		if rel, err := filepath.Rel(wd, folder); err == nil && !strings.HasPrefix(rel, "..") {
			folder = rel
		}
		o.runner.CaptureUpdate, o.runner.Capture = folder, filepath.Join(folder, mock.UpdateFolder)
	}
	if o.runner.Capture == "" || o.suite == nil || len(flows) != 1 {
		return nil
	}
	c := o.suite.CaptureOf(flows[0])
	if c == nil {
		return nil
	}
	o.runner.Matching = o.runner.Matching.Or(c.Settings)
	if len(o.runner.Hosts) == 0 {
		o.runner.Hosts = c.Hosts
	}
	if o.runner.Backend == "" {
		o.runner.Backend = c.From
	}
	return nil
}

// checkCapture refuses what would call the real backend more than once, or lose an earlier capture.
func (o *options) checkCapture(flows []*flow.Flow) error {
	if o.runner.Capture != "" && (len(flows) != 1 || o.retries > 0 || o.continuous) {
		return errors.New("--capture runs one flow, once")
	}
	return o.checkCaptureFolder()
}

// checkCaptureFolder refuses a capture that would write over another, or go on from none.
func (o *options) checkCaptureFolder() error {
	folder := o.runner.Capture
	if folder == "" {
		if o.runner.Backend != "" {
			return errors.New("--capture-from needs --capture")
		}
		if !reflect.DeepEqual(o.runner.Matching, mock.Settings{}) {
			return errors.New("how a capture becomes mocks (--graphql, --rest, --ignore-params and the like) needs --capture")
		}
		if o.runner.CaptureAppend {
			return errors.New("--capture-append needs --capture")
		}
		return nil
	}
	if o.runner.CaptureCheck && o.runner.CaptureUpdate == "" {
		return errors.New("--check needs --capture-update")
	}
	if o.runner.CaptureUpdate != "" {
		if o.runner.CaptureAppend {
			return errors.New("--capture-update makes a capture of its own; --capture-append goes on with one")
		}
		if _, err := os.Stat(filepath.Join(o.runner.CaptureUpdate, "mappings")); err != nil {
			return fmt.Errorf("--capture-update: %s has no mocks to compare with", filepath.ToSlash(o.runner.CaptureUpdate))
		}
		if err := mock.ClearUpdate(o.runner.CaptureUpdate); err != nil {
			return err
		}
	}
	list, err := os.ReadDir(folder)
	switch {
	case o.runner.CaptureAppend && err != nil:
		return fmt.Errorf("--capture-append: %s isn't there", filepath.ToSlash(folder))
	case !o.runner.CaptureAppend && err == nil && len(list) > 0:
		return fmt.Errorf("%s is there already; --capture-append goes on with it", filepath.ToSlash(folder))
	}
	return nil
}

func runLive(ctx context.Context, args []string) error {
	o, err := parse(cmdLive, args)
	if err != nil {
		return err
	}
	if len(o.args) != 1 {
		return errors.New("which flow? give a file, there already or not")
	}
	o.into("")
	if err := o.checkCaptureFolder(); err != nil {
		return err
	}
	if err := o.findCA(nil, true); err != nil {
		return err
	}
	p := live.Params{Path: o.args[0], App: o.app, Mocks: o.mocks, Output: o.output, NoRun: o.noRun, Serve: o.serve}
	return live.Run(ctx, p, func(reporter flow.Reporter) (*flow.Runner, func(), error) {
		d, err := adb.Pick(o.device, o.emulator)
		if err != nil {
			return nil, nil, ofDevice(err)
		}
		s := &session{o: o, device: d, reporter: reporter}
		if err := s.start(); err != nil {
			return nil, nil, ofDevice(err)
		}
		return s.runner, func() { s.agent.Close() }, nil
	})
}

// findCA settles the CA the proxy signs with: --ca, else the suite's, else .rayo/ca where the suite or
// the flow is, or above it, else the computer's. One said must be there.
func (o *options) findCA(group *suite.Suite, flows bool) error {
	if o.runner.CA != "" {
		if _, err := os.Stat(filepath.Join(o.runner.CA, "ca.pem")); err != nil {
			return fmt.Errorf("there is no CA in %s: it takes ca.pem and ca-key.pem; rayo ca new %s makes one", filepath.ToSlash(o.runner.CA), filepath.ToSlash(o.runner.CA))
		}
		return nil
	}
	from := "."
	switch {
	case group != nil:
		from = filepath.Dir(group.Path)
	case flows && len(o.args) > 0:
		from = o.args[0]
		if info, err := os.Stat(from); err != nil || !info.IsDir() {
			from = filepath.Dir(from)
		}
	}
	o.runner.CA = mock.FindCA(from)
	return nil
}

// errShown is a failure that has been printed.
var errShown = errors.New("")

// runDo sends commands to the live session.
func runDo(args []string) error {
	o, err := parse(cmdDo, args)
	if err != nil {
		return err
	}
	o.into("")
	for i, text := range o.args {
		if text == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return err
			}
			o.args[i] = string(data)
		}
	}
	if len(o.args) == 0 {
		return errors.New("do what? such as rayo do '- tapOn: Save', or rayo do .help")
	}
	if o.find != "" && !o.screen {
		return errors.New("--find needs --screen")
	}
	if o.find != "" && o.screenChanges {
		return errors.New("--find looks through the whole screen: use --screen, not --screen=changes")
	}
	if o.ignoreScreen != "" && !o.screen {
		return errors.New("--ignore-screen needs --screen")
	}
	text := strings.Join(o.args, "\n")
	if o.discard {
		if strings.TrimSpace(text) != ".quit" {
			return errors.New("--discard is of .quit")
		}
		text = ".quit --discard"
	}
	a, err := live.Ask(o.output, live.Request{Text: text, Screen: o.screen, Changes: o.screenChanges, Ignore: o.ignoreScreen}, o.doTimeout)
	if err != nil {
		return err
	}
	if o.find != "" {
		a.Screen = report.Lines(findOn(strings.Join(a.Screen, "\n"), o.find))
	}
	if printing == report.Human {
		fmt.Print(a.Text())
		switch {
		case a.Failure != nil:
		case o.screenChanges && a.Baseline:
			fmt.Println("(whole screen: nothing before)")
		case o.screenChanges && len(a.Screen) == 0:
			fmt.Println("screen: the same")
		}
		if a.Ignored > 0 {
			fmt.Printf("screen: %d ignored\n", a.Ignored)
		}
	} else {
		err = emit(a)
	}
	if err == nil && !a.OK {
		err = errShown
	}
	return err
}

// asksHelp says whether a command of rayo's has --help or -h, which are rayo help <command>.
func asksHelp(command string, args []string) bool {
	if !slices.Contains(commandNames, command) {
		return false
	}
	for _, a := range args {
		// In do, -h may be a command of a list; --help never is.
		if a == "--help" || a == "-h" && command != cmdDo {
			return true
		}
	}
	return false
}

// helpOn explains a command, of flows or rayo's own, or a kind of file.
func helpOn(name string) (string, error) {
	switch name {
	case "json":
		return helpJSON()
	case "all":
		return helpAll()
	case "flow":
		return "A flow begins with a header, then ---, then a list of commands. The header:\n\n" + flow.HeaderHelp() +
			"\nCommands: " + strings.Join(flow.Commands(), ", ") + "\n", nil
	case "suite":
		return "A suite, *.suite.yaml, lists flows that run together, and is where a run's settings go. Paths in it are relative to it. An option of the same name in kebab-case overrides a key for one run (failureWindow, --failure-window). A folder is the flows directly in it; a suite runs only when its file is given:\n\n" + schema.Help(suite.Fields) + "\nflows, when not only a file:\n\n" + schema.Help(suite.FlowFields) +
			"\nunmocked:\n\n" + schema.Help(suite.UnmockedFields) + "\nan ignored call, when not text:\n\n" + schema.Help(suite.IgnoredFields) +
			"\noutput, when a mapping (rayo check says what a run would write):\n\n" + schema.Help(suite.OutputFields) +
			"\ncapture:\n\n" + schema.Help(suite.CaptureFields) +
			"\napp:\n\n" + schema.Help(suite.AppFields) + "\ndevice:\n\n" + schema.Help(suite.DeviceFields), nil
	}
	if text := helpTopic(name); text != "" {
		return text, nil
	}
	if text := helpOf(name); text != "" {
		return text, nil
	}
	text, err := flow.Help(name)
	if err != nil && flow.NearestCommand(name) == "" {
		all := append(slices.Clone(commandNames), "json", "all", "flow", "suite", "search")
		for _, t := range topics {
			all = append(all, t.name)
		}
		if near := schema.Nearest(name, all); near != "" {
			return "", fmt.Errorf("no help on %q, did you mean rayo help %s?", name, near)
		}
	}
	return text, err
}

func printSchema(args []string) error {
	makers := report.Schemas()
	makers["flow"], makers["suite"] = flow.Schema, suite.Schema
	makers["do"] = func() []byte { return schema.Written(live.Reply{}).File("What rayo do prints, --format json", nil) }
	if len(args) == 1 && makers[args[0]] != nil {
		_, err := os.Stdout.Write(makers[args[0]]())
		return err
	}
	return errors.New("rayo schema takes flow, suite, results, check, explain, devices, hierarchy, do, mocks, event or errors")
}

// helpJSON is all of it, for programs and the documentation's page.
func helpJSON() (string, error) {
	type optionDoc struct {
		Name     string   `json:"name"`
		Short    string   `json:"short"`
		Takes    string   `json:"takes"`
		About    string   `json:"about"`
		Commands []string `json:"commands"`
	}
	var all []optionDoc
	for _, s := range sections {
		for _, p := range s.options {
			all = append(all, optionDoc{p.name, p.short, p.value, p.about, s.commands})
		}
	}
	var b strings.Builder
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	e.SetIndent("", " ")
	err := e.Encode(map[string]any{
		"options": all, "flow": flow.Describe(), "header": flow.HeaderDocs(), "suite": schema.Docs(suite.Fields),
		"suiteFlow": schema.Docs(suite.FlowFields), "app": schema.Docs(suite.AppFields), "device": schema.Docs(suite.DeviceFields),
		"unmocked": schema.Docs(suite.UnmockedFields), "ignored": schema.Docs(suite.IgnoredFields), "capture": schema.Docs(suite.CaptureFields),
		"exitCodes": report.Exits,
	})
	return b.String(), err
}

// applyMocks takes the changes of an update into the mocks, or writes those of a HAR file.
func applyMocks(args []string) error {
	o, err := parse(cmdMocks, args)
	if err != nil {
		return err
	}
	var notes []string
	switch {
	case len(o.args) == 2 && o.args[0] == "check":
		if !reflect.DeepEqual(o.runner.Matching, mock.Settings{}) || o.runner.Backend != "" || len(o.take) > 0 || o.prune {
			return errors.New("rayo mocks check takes only the mocks: a folder or a file")
		}
		return checkMocks(o.args[1])
	case len(o.args) == 2 && o.args[0] == "apply":
		if !reflect.DeepEqual(o.runner.Matching, mock.Settings{}) || o.runner.Backend != "" {
			return errors.New("rayo mocks apply takes the mocks as they were captured: --graphql, --rest and the like are of rayo mocks import")
		}
		notes, err = mock.Apply(o.args[1], o.take, o.prune)
	case len(o.args) == 3 && o.args[0] == "import":
		if len(o.take) > 0 || o.prune {
			return errors.New("--only and --prune are of rayo mocks apply")
		}
		folder := o.args[2]
		if list, err := os.ReadDir(folder); err == nil && len(list) > 0 {
			return fmt.Errorf("%s is there already: import into a new folder", filepath.ToSlash(folder))
		}
		notes, err = mock.ImportHAR(o.args[1], folder, o.runner.Backend, o.runner.Env, o.runner.Matching)
	default:
		if near := nearSub(o.args, "check", "apply", "import"); near != "" {
			return fmt.Errorf("unknown rayo mocks %s, did you mean %s?", o.args[0], near)
		}
		return errors.New("rayo mocks apply [--only a,b] [--prune] <folder>, after rayo test --capture-update <folder>;\n" +
			"or rayo mocks import [options] <file.har> <folder>;\nor rayo mocks check <folder>")
	}
	switch {
	case err != nil:
		return err
	case printing != report.Human:
		return emit(report.Notes{SchemaVersion: report.SchemaVersion, Notes: report.List(notes)})
	}
	fmt.Println(strings.Join(notes, "\n"))
	return nil
}

// checkMocks says what of mocks is never used; a mock that doesn't load is an error, as for rayo check.
func checkMocks(path string) error {
	n, problems, err := mock.Check(path)
	if err != nil {
		return err
	}
	if printing != report.Human {
		err = emit(report.Notes{SchemaVersion: report.SchemaVersion, Notes: report.List(problems)})
	} else if len(problems) == 0 && n == 1 {
		fmt.Println("1 mapping is fine")
	} else if len(problems) == 0 {
		fmt.Println(n, "mappings are fine")
	} else {
		fmt.Println(strings.Join(problems, "\n"))
	}
	if err == nil && len(problems) > 0 {
		err = errShown
	}
	return err
}

// caUsage is what rayo ca takes.
const caUsage = "rayo ca new [<folder>], a CA for the project, in .rayo/ca if not said;\n" +
	"or rayo ca export [--ca <folder>] <file>, such as app/src/debug/res/raw/rayo_ca.pem"

// commandCA makes a project's CA, or writes the certificate of the CA the proxy would sign with, for an
// app to trust in its debug build.
// nearSub is the subcommand most like the first argument, when that isn't one; empty if none is.
func nearSub(args []string, subs ...string) string {
	if len(args) == 0 || slices.Contains(subs, args[0]) {
		return ""
	}
	return schema.Nearest(args[0], subs)
}

func commandCA(args []string) error {
	o, err := parse(cmdCA, args)
	if err != nil {
		return err
	}
	switch {
	case len(o.args) >= 1 && len(o.args) <= 2 && o.args[0] == "new":
		if o.runner.CA != "" {
			return errors.New("rayo ca new takes the folder, not --ca")
		}
		folder := filepath.FromSlash(mock.ProjectCA)
		if len(o.args) == 2 {
			folder = o.args[1]
		}
		if err := mock.MakeCA(folder); err != nil {
			return err
		}
		fmt.Printf(`made a CA in %[1]s: commit it with the flows, so that every checkout and CI signs alike.
rayo finds .rayo/ca next to a suite or above it; else a suite says ca: %[1]s, or --ca %[1]s.
Then rayo ca export app/src/debug/res/raw/rayo_ca.pem, for the app to trust it.
`, filepath.ToSlash(folder))
		return nil
	case len(o.args) != 2 || o.args[0] != "export":
		if near := nearSub(o.args, "new", "export"); near != "" {
			return fmt.Errorf("unknown rayo ca %s, did you mean %s?", o.args[0], near)
		}
		return errors.New(caUsage)
	}
	if err := o.findCA(nil, false); err != nil {
		return err
	}
	ca, err := mock.LoadCA(o.runner.CA)
	if err != nil {
		return err
	}
	file := o.args[1]
	if dir := filepath.Dir(file); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(file, ca.PEM(), 0o644); err != nil {
		return err
	}
	name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	fmt.Printf(`wrote %s, the certificate of %s; its key stays there.

For --network proxy, trust it in the debug build only, in app/src/debug/res/xml/network_security_config.xml:

  <network-security-config>
      <debug-overrides>
          <trust-anchors>
              <certificates src="@raw/%s" />
          </trust-anchors>
      </debug-overrides>
  </network-security-config>

and android:networkSecurityConfig="@xml/network_security_config" on <application> in the manifest.
`, filepath.ToSlash(file), filepath.ToSlash(ca.Path), name)
	return nil
}
