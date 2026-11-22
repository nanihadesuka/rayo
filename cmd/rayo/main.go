// Command rayo runs end-to-end flows on Android, fast.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/adb"
	"github.com/nanihadesuka/rayo/internal/agent"
	"github.com/nanihadesuka/rayo/internal/flow"
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
		fmt.Print(help)
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
	case "test":
		return test(ctx, args)
	case "check":
		return check(args)
	case "live":
		return runLive(ctx, args)
	case "do":
		return runDo(args)
	case "hierarchy", "tree":
		return hierarchy(ctx, args)
	case "devices":
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
			fmt.Print(help)
			return nil
		}
		text, err := flow.Help(args[0])
		fmt.Print(text)
		return err
	}
	return fmt.Errorf("unknown command %q, see rayo help", command)
}

func check(args []string) error {
	flows, _, err := load(args)
	if err == nil {
		fmt.Printf("%d flows are fine\n", len(flows))
	}
	return err
}

func hierarchy(ctx context.Context, args []string) error {
	o, err := parse(args)
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
	o, err := parse(args)
	if err != nil {
		return err
	}
	all, group, err := load(o.paths)
	if err != nil {
		return err
	}
	title := strings.Join(o.paths, " ")
	if group != nil {
		if group.Name != "" {
			title = group.Name
		}
		if o.runner.Out == "" {
			o.runner.Out = group.relative(group.Output)
		}
		o.runner.Recording = group.recording
		if o.results == "" {
			o.results = group.relative(group.Results)
		}
		if o.resultsFormat == "" {
			o.resultsFormat = group.Format
		}
	}
	o.runner.JSON = o.resultsFormat == formatJSON
	if o.runner.Out == "" {
		o.runner.Out = ".rayo"
	}
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
		if err := group.install(d, o.steps(flows)); err != nil {
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
		watch[group.path] = true
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
			again, _, err := load(o.paths)
			if err == nil {
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
