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
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/adb"
	"github.com/nanihadesuka/rayo/internal/agent"
	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/live"
	"github.com/nanihadesuka/rayo/internal/report"
	"github.com/nanihadesuka/rayo/internal/suite"
)

const version = "0.1.0"

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
	case "version", "--version", "-v":
		fmt.Println("rayo " + version)
		return nil
	case "help", "--help", "-h":
		if len(args) == 0 {
			fmt.Print(help())
			return nil
		}
		text, err := flow.Help(args[0])
		fmt.Print(text)
		return err
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
	folder := o.runner.Capture
	if folder == "" {
		if o.runner.Backend != "" {
			return errors.New("--capture-from needs --capture")
		}
		return nil
	}
	if len(flows) != 1 || o.retries > 0 || o.continuous {
		return errors.New("--capture runs one flow, once")
	}
	if list, err := os.ReadDir(folder); err == nil && len(list) > 0 {
		return fmt.Errorf("%s is there already", filepath.ToSlash(folder))
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
