package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/flow"
)

const help = `rayo - runs flows on Android, fast

  rayo test [options] <flow, folder or suite>...   run flows; a suite is a *.rayo.yaml
  rayo check <flow, folder or suite>...            check flows without a device
  rayo live [options] <flow>                       write a flow as it runs, command by command
  rayo do [--screen] [--json] <commands>           send commands to the live session; rayo do .help
  rayo hierarchy                                   what is on screen, and how to refer to it
  rayo devices                                     connected devices
  rayo help <command>                              what a command takes
  rayo version

Options of test:
  --device <serial>        when several are connected
  --emulator               never a real device, even if it is the only one
  -e, --env KEY=VALUE      a variable for the flows
  -c, --continuous         run again whenever a file changes

  Which flows, and how:
  --only a,b               only the flows with these names
  --include-tags a,b       only flows with one of these tags
  --exclude-tags a,b       not flows with one of these tags
  --rerun-failed           only those that haven't passed since they last failed
  --shuffle[=seed]         in a random order, or in that of an earlier run
  --retries <n>            try again what fails, whatever the suite says (default 1)
  --fail-fast              stop at the first failure
  --keep-going             don't stop when the device or the app fails over and over

  What comes out:
  -v, --verbose            every step (always, for a single flow)
  --profile                where the time of each step went
  --results <file>         write the results
  --results-format <f>     txt (the default), json or junit
  --failures-only          leave the flows that passed out of the results
  --format junit           write a report
  --output <file>          where (default report.xml)
  --debug-output <folder>  where failures and screenshots go (default .rayo)

  --capture <folder>       call the real backend, and write what it answers as mocks
  --capture-from <url>     the backend, if calls don't begin with /host/

Options of live, besides --device, --emulator, -e, --mock-port, --debug-output and --animations:
  --app <package>          of a flow that isn't there yet
  --mock a,b               its mocks, relative to it
  --no-run                 don't begin by running the steps it has
  --serve                  only take commands from rayo do, until .quit
  --mock-port <port>       where the app finds the mock on the device (default 8080)
  --animations             leave the device's animations on
`

type options struct {
	device, format, output string
	results, resultsFormat string
	include, exclude, only []string
	continuous, animations bool
	verbose, failuresOnly  bool
	rerunFailed, failFast  bool
	shuffle, emulator      bool
	noRun, serve           bool
	keepGoing              bool
	app                    string
	mocks                  []string
	seed                   uint64
	retries                int // negative if not said
	suite                  *suite
	runner                 flow.Options
	paths                  []string
}

func parse(args []string) (*options, error) {
	o := &options{output: "report.xml", retries: -1, runner: flow.Options{Env: map[string]string{}, MockPort: 8080}}
	for i := 0; i < len(args); i++ {
		name, value, joined := strings.Cut(args[i], "=")
		if !strings.HasPrefix(args[i], "-") {
			o.paths = append(o.paths, args[i])
			continue
		}
		next := func() (string, error) {
			if joined {
				return value, nil
			}
			if i++; i >= len(args) {
				return "", fmt.Errorf("%s needs a value", name)
			}
			return args[i], nil
		}
		var err error
		switch name {
		case "--device", "--udid":
			o.device, err = next()
		case "-e", "--env":
			var pair string
			if pair, err = next(); err == nil {
				k, v, ok := strings.Cut(pair, "=")
				if !ok {
					err = fmt.Errorf("%s takes KEY=VALUE", name)
				}
				o.runner.Env[k] = v
			}
		case "--include-tags":
			o.include, err = tags(next)
		case "--exclude-tags":
			o.exclude, err = tags(next)
		case "--only":
			o.only, err = tags(next)
		case "--results":
			o.results, err = next()
		case "--results-format":
			if o.resultsFormat, err = next(); err == nil {
				err = checkFormat(o.resultsFormat)
			}
		case "--capture":
			if o.runner.Capture, err = next(); err == nil && o.runner.Capture == "" {
				err = errors.New("--capture takes a folder")
			}
		case "--capture-from":
			o.runner.Backend, err = next()
		case "--retries":
			var n string
			if n, err = next(); err == nil {
				if o.retries, err = strconv.Atoi(n); err != nil || o.retries < 0 {
					err = fmt.Errorf("--retries takes a number, not %q", n)
				}
			}
		case "--shuffle":
			o.shuffle, o.seed = true, uint64(time.Now().UnixMilli())
			if joined {
				if o.seed, err = strconv.ParseUint(value, 10, 64); err != nil {
					err = fmt.Errorf("--shuffle takes a number, not %q", value)
				}
			}
		case "--rerun-failed":
			o.rerunFailed = true
		case "--app":
			o.app, err = next()
		case "--mock":
			o.mocks, err = tags(next)
		case "--no-run":
			o.noRun = true
		case "--serve":
			o.serve = true
		case "--emulator":
			o.emulator = true
		case "--keep-going":
			o.keepGoing = true
		case "--fail-fast":
			o.failFast = true
		case "--failures-only":
			o.failuresOnly = true
		case "-v", "--verbose":
			o.verbose = true
		case "--format":
			if o.format, err = next(); err == nil && !strings.EqualFold(o.format, "junit") {
				err = fmt.Errorf("unknown format %q (use junit)", o.format)
			}
		case "--output":
			o.output, err = next()
		case "--debug-output", "--test-output-dir":
			o.runner.Out, err = next()
		case "--mock-port":
			var port string
			if port, err = next(); err == nil {
				n, e := strconv.ParseUint(port, 10, 16)
				if e != nil || n == 0 {
					err = fmt.Errorf("not a port: %q", port)
				}
				o.runner.MockPort = int(n)
			}
		case "-c", "--continuous":
			o.continuous = true
		case "--profile":
			o.runner.Profile, o.verbose = true, true
		case "--animations":
			o.animations = true
		default:
			err = fmt.Errorf("unknown option %s", name)
		}
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

func tags(next func() (string, error)) ([]string, error) {
	v, err := next()
	return strings.Split(v, ","), err
}
