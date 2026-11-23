// Command rayo runs end-to-end flows on Android, fast.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
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

const version = "1.0.13"

// Exit codes.
const (
	exitFailed      = 1
	exitUsage       = 2
	exitInterrupted = 130
)

func main() {
	if len(os.Args) < 2 {
		fmt.Print(help())
		os.Exit(exitUsage)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	context.AfterFunc(ctx, stop) // a second Ctrl-C doesn't wait
	err := run(ctx, os.Args[1], os.Args[2:])
	switch {
	case ctx.Err() != nil:
		fmt.Fprintln(os.Stderr, "interrupted")
		os.Exit(exitInterrupted)
	case err == errShown:
		os.Exit(exitFailed)
	case err != nil:
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(exitFailed)
	}
}

func run(ctx context.Context, command string, args []string) error {
	switch command {
	case cmdTest:
		return test(ctx, args)
	case cmdCheck:
		return check(args)
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
		if err == nil {
			fmt.Println(strings.Join(list, "\n"))
		}
		return err
	case cmdCA:
		return exportCA(args)
	case cmdMocks:
		return applyMocks(args)
	case "version", "--version", "-v":
		fmt.Println("rayo " + version)
		return nil
	case "help", "--help", "-h":
		if len(args) == 0 {
			fmt.Print(help())
			return nil
		}
		text, err := helpOn(args[0])
		fmt.Print(text)
		return err
	case "schema":
		return printSchema(args)
	}
	return fmt.Errorf("unknown command %q, see rayo help", command)
}

func check(args []string) error {
	o, err := parse(cmdCheck, args)
	if err != nil {
		return err
	}
	flows, _, err := suite.Load(o.args)
	if err == nil {
		fmt.Printf("%d flows are fine\n", len(flows))
	}
	return err
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
		return err
	}
	a, err := agent.Start(d)
	if err != nil {
		return err
	}
	defer a.Close()
	r, err := a.Do(ctx, "tree", nil, 10*time.Second)
	if err == nil {
		fmt.Print(r.Out)
	}
	return err
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
	title := strings.Join(o.args, " ")
	if group != nil {
		if group.Name != "" {
			title = group.Name
		}
		o.into(group.Relative(group.Output))
		if o.runner.Scale == 0 {
			o.runner.Scale = group.TimeoutScale
		}
		for k, v := range group.Env {
			if _, set := o.runner.Env[k]; !set {
				o.runner.Env[k] = v
			}
		}
		o.runner.Recording = &flow.Recording{On: group.Record, Path: group.Relative(group.RecordFolder)}
		if o.results == nil {
			for _, file := range group.Results {
				o.results = append(o.results, group.Relative(file))
			}
		}
	}
	for _, file := range o.results {
		if format, _ := report.FormatOf(file); format == report.JSON { // it has been checked
			o.runner.JSON = true
		}
	}
	o.into("")
	flows, err := o.choose(all)
	if err != nil {
		return err
	}
	if len(flows) == 0 { // only with --rerun-failed
		fmt.Println("nothing failed last time")
		return nil
	}
	if err := o.checkCapture(flows); err != nil {
		return err
	}
	d, err := adb.Pick(o.device, o.emulator)
	if err != nil {
		return err
	}
	if group != nil {
		if err := group.Install(d, o.steps(flows)); err != nil {
			return err
		}
	}
	if group != nil && group.Device != nil {
		undo, e := d.Apply(group.Device)
		if e != nil {
			return e
		}
		defer func() { // after the agent, which begins and ends with the screen like this
			if e := undo(); e != nil {
				err = errors.Join(err, fmt.Errorf("couldn't put the device back: %w", e))
			}
		}()
	}
	start := time.Now()
	s := &session{o: o, device: d, reporter: newConsole()}
	if err := s.start(); err != nil {
		return err
	}
	defer func() { s.agent.Close() }() // the one there is by then
	if o.steps(flows) {
		fmt.Printf("%s, ready in %d ms\n", d.Serial, time.Since(start).Milliseconds())
	}

	watch := watcher{}
	if group != nil {
		watch[group.Path] = true
	}
	for {
		err = s.runAll(ctx, flows, title)
		if !o.continuous || ctx.Err() != nil {
			return err
		}
		if err != nil {
			fmt.Println(err)
		}
		if s.agent.Dead() != nil {
			return s.agent.Dead()
		}
		fmt.Println("\nwaiting for changes...")
		for {
			if err := watch.await(ctx, flows); err != nil {
				return err
			}
			again, changed, err := suite.Load(o.args)
			if err == nil {
				o.suite = changed
				again, err = o.choose(again)
			}
			if err == nil {
				flows = again
				break
			}
			fmt.Println("error:", err)
		}
	}
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
	p := live.Params{Path: o.args[0], App: o.app, Mocks: o.mocks, Output: o.output, NoRun: o.noRun, Serve: o.serve}
	return live.Run(ctx, p, func(reporter flow.Reporter) (*flow.Runner, func(), error) {
		d, err := adb.Pick(o.device, o.emulator)
		if err != nil {
			return nil, nil, err
		}
		s := &session{o: o, device: d, reporter: reporter}
		if err := s.start(); err != nil {
			return nil, nil, err
		}
		return s.runner, func() { s.agent.Close() }, nil
	})
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
	a, err := live.Ask(o.output, live.Request{Text: strings.Join(o.args, "\n"), Screen: o.screen})
	if err != nil {
		return err
	}
	if o.json {
		e := json.NewEncoder(os.Stdout)
		e.SetEscapeHTML(false)
		e.SetIndent("", "  ")
		err = e.Encode(a)
	} else {
		fmt.Print(a.Text())
	}
	if err == nil && !a.OK {
		err = errShown
	}
	return err
}

// helpOn explains a command, of flows or rayo's own, or a kind of file.
func helpOn(name string) (string, error) {
	switch name {
	case "json":
		return helpJSON()
	case "flow":
		return "A flow begins with a header, then ---, then a list of commands. The header:\n\n" + flow.HeaderHelp() +
			"\nCommands: " + strings.Join(flow.Commands(), ", ") + "\n", nil
	case "suite":
		return "A suite, *.rayo.yaml:\n\n" + schema.Help(suite.Fields) + "\nflows, when not only a file:\n\n" + schema.Help(suite.FlowFields) +
			"\napp:\n\n" + schema.Help(suite.AppFields) + "\ndevice:\n\n" + schema.Help(suite.DeviceFields), nil
	}
	if text := helpOf(name); text != "" {
		return text, nil
	}
	return flow.Help(name)
}

func printSchema(args []string) error {
	makers := map[string]func() []byte{"flow": flow.Schema, "suite": suite.Schema, "results": report.Schema}
	if len(args) == 1 && makers[args[0]] != nil {
		_, err := os.Stdout.Write(makers[args[0]]())
		return err
	}
	return errors.New("rayo schema takes flow, suite or results")
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
		notes, err = mock.ImportHAR(o.args[1], folder, o.runner.Backend, o.runner.Matching)
	default:
		return errors.New("rayo mocks apply [--only a,b] [--prune] <folder>, after rayo test --capture-update <folder>;\n" +
			"or rayo mocks import [options] <file.har> <folder>")
	}
	if err == nil {
		fmt.Println(strings.Join(notes, "\n"))
	}
	return err
}

// exportCA writes the certificate of this computer's CA, for an app to trust in its debug build.
func exportCA(args []string) error {
	o, err := parse(cmdCA, args)
	if err != nil {
		return err
	}
	if len(o.args) != 2 || o.args[0] != "export" {
		return errors.New("rayo ca export <file>, such as app/src/debug/res/raw/rayo_ca.pem")
	}
	ca, err := mock.LoadCA()
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
