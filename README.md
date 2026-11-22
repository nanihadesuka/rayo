# rayo

Runs end-to-end tests of Android apps, fast: flows in YAML, with a mock backend built in. One
executable, nothing to install on the device.

All commands: **[REFERENCE.md](REFERENCE.md)**.

## Use

```
rayo test flow.yaml              run a flow, or every flow in a folder
rayo test all.rayo.yaml          run a suite: the flows it lists, together
rayo test --capture mock f.yaml  run on the real backend, and keep its answers as mocks
rayo test -c flow.yaml           run again whenever a file changes
rayo live new.yaml --app a.b     write a flow as it runs, see REFERENCE.md
rayo do '- tapOn: Save'          send a command to that, also from a script or an LLM
rayo check flows/                check flows without a device
rayo hierarchy                   what is on screen, and how to refer to it
rayo devices
rayo help tapOn                  what a command takes
```

Needs adb and a device or emulator. All the options, as `rayo help` prints them:

```
rayo - runs flows on Android, fast

  rayo test [options] <flow, folder or suite>...   run flows; a suite is a *.rayo.yaml
  rayo check <flow, folder or suite>...            check flows without a device
  rayo live [options] <flow>                       write a flow as it runs, command by command
  rayo do [options] <commands>                     send commands to the live session; rayo do .help
  rayo hierarchy [options]                         what is on screen, and how to refer to it
  rayo devices                                     connected devices
  rayo help <command>                              what a command takes, of a flow or of rayo; also flow and suite
  rayo schema flow, suite or results               its JSON Schema, for editors and programs
  rayo version

Options of test, live and hierarchy
  --device <serial>          when several are connected
  --emulator                 never a real device, even if it is the only one

Options of test, live and do
  --output <folder>          where failures and screenshots go (default .rayo)

Options of test and live
  -e, --env <KEY=VALUE>      a variable for the flows
  --mock-port <port>         where the app finds the mock on the device (default 8080)
  --animations               leave the device's animations on
  --profile                  where the time of each step went

Options of test: which flows
  --only <a,b>               only the flows with these names
  --include-tags <a,b>       only flows with one of these tags
  --exclude-tags <a,b>       not flows with one of these tags
  --rerun-failed             only those that haven't passed since they last failed

Options of test: how they run
  --retries <n>              try again what fails, whatever the suite says (default 1)
  --fail-fast                stop at the first failure
  --keep-going               don't stop when the device or the app fails over and over
  --shuffle[=seed]           in a random order, or in that of an earlier run
  -c, --continuous           run again whenever a file changes

Options of test: what comes out
  -v, --verbose              every step (always, for a single flow)
  --results <file>           write the results: .txt, .json or .xml (JUnit); may be repeated
  --failures-only            leave the flows that passed out of the results

Options of test: mocks from the real backend
  --capture <folder>         call the real backend, and write what it answers as mocks
  --capture-from <url>       the backend, if calls don't begin with /host/

Options of live
  --app <package>            of a flow that isn't there yet
  --mock <a,b>               its mocks, relative to it
  --no-run                   don't begin by running the steps it has
  --serve                    only take commands from rayo do, until .quit

Options of do
  --screen                   also what is on screen afterwards
  --json                     for a program
```

An option of another command is an error, never ignored. More on [retries](REFERENCE.md#retries),
[when it stops](REFERENCE.md#causes), [results](REFERENCE.md#results),
[capture](REFERENCE.md#capture) and [live sessions](REFERENCE.md#live-sessions).

The exit code is not 0 if a flow fails or doesn't run. A failure saves a screenshot, the elements on screen and
the device log.

## Build

```
bash build.sh            # bin/rayo, needs Go
bash build.sh --all      # for Windows, Linux and macOS
bash build.sh --agent    # also the device part, needs a JDK and the Android SDK
```

`internal/agent/agent.jar` is committed, so Go is enough unless `internal/agent/java` changes.

## A flow

```yaml
appId: com.example.app
---
- launchApp: { clearState: true, arguments: { mock: true } }
- tapOn: { id: login }
- inputText: me@example.com
- extendedWaitUntil: { visible: { id: home }, timeout: 20s }
- assertVisible: "Welcome.*"
```

## Many tests

A suite, `*.rayo.yaml`, lists the flows that run together and how:

```yaml
name: Rides
app: { id: com.example.app, apk: app-debug.apk }    # installed if the device has another
output: .results                  # what is saved of each failure
results: [.results/results.txt, .results/junit.xml]   # how it went: .txt, .json or .xml (JUnit)
record: true                      # a small video of each flow
recordFolder: .results/videos     # where; in output if not said
retries: 1                        # of a whole flow that fails; 1 if not said
device: { size: 1080x2400, dpi: 420, fontScale: 1.0, darkMode: false }   # put back at the end

flows:
  - tests/book/flow.yaml
  - file: tests/scan/flow.yaml
    retries: 3                    # only this one
```

```
$ rayo test --emulator all.rayo.yaml
[1/2] PASS book  18.2 s
[2/2] PASS scan  41.0 s  (failed once, then passed)
  try 1: tests/scan/flow.yaml:12: tapOn scan: not on screen: scan  (test)  .results/scan

TOTAL 2: 2 passed, 0 failed, 1 retried  59.3 s
```

| | |
|---|---|
| [Clean state](REFERENCE.md#clean-state) | Between flows, what each changed is undone |
| [Retries](REFERENCE.md#retries) | What passes at last is a `PASS`, with a note of where it failed before |
| [Causes](REFERENCE.md#causes) | Each failure is of the `test`, the `app` or the `device`. If the device or the app fails over and over, the run stops and says why |
| [Outcomes](REFERENCE.md#outcomes) | Every flow has one, also those that didn't run |
| [Results](REFERENCE.md#results) | For a failure: why, where, what came before, the last calls, the screen and the crash |
| [Recordings](REFERENCE.md#startrecording-stoprecording), [device setup](REFERENCE.md#device-setup) | For a suite; recordings also for a flow or some steps |
| [Live sessions](REFERENCE.md#live-sessions) | Write a flow as it runs, by hand or from another program |

## Mocking

rayo answers as the backend. A mock is a folder: `mappings/` says **when** to answer, and `__files/`
holds **what** to answer. The names are fixed.

```
mock/
├─ mappings/                 the rules: if the app calls this, answer that
│   ├─ products.json
│   └─ order.json
└─ __files/                  the bodies, as the backend would send them
    ├─ products/all.json
    └─ order/shipped.json
```

A flow names its mocks, in order, in its header:

```yaml
mock: [../shared/mock, mock/overrides.json]
```

### What happens on a call

1. The app calls `http://127.0.0.1:8080/...` on the device, which reaches rayo.
2. rayo looks for a mapping that fits: method, address, query, headers, body and state.

   ```json
   {
     "request": { "method": "GET", "urlPath": "/products" },
     "response": { "status": 200, "headers": { "Content-Type": "application/json" }, "bodyFileName": "products/all.json" }
   }
   ```

   If several fit, the lowest `priority` wins (5 by default), and then the last one loaded. So
   `overrides.json` wins over `shared/mock`, as it comes after.
3. It answers with `__files/products/all.json`, with what is between `{{ }}` filled in, such as
   `{{now offset='-8 minutes'}}`.
4. If none fits, the app gets a 404 and **the flow fails**.

### State

The same call can be answered differently as things advance. A mapping may belong to a scenario,
which begins in `Started`:

```json
{ "mappings": [
  { "scenarioName": "order", "requiredScenarioState": "Started", "newScenarioState": "paid",
    "request": { "method": "POST", "urlPath": "/payments" },
    "response": { "status": 200 } },
  { "scenarioName": "order", "requiredScenarioState": "shipped",
    "request": { "method": "GET", "urlPath": "/order" },
    "response": { "status": 200, "bodyFileName": "order/shipped.json" } },
  { "priority": 10,
    "request": { "method": "GET", "urlPath": "/order" },
    "response": { "status": 200, "jsonBody": { "state": "waiting" } } }
] }
```

Here paying moves `order` to `paid`. What the backend does by itself, the flow says:
`- mockState: { order: shipped }`. The last mapping is for any other state.

### Why two folders

- **Reuse**: several mappings can answer with the same body.
- **Readable**: the logic isn't buried in long bodies.
- **Easy to replace**: a body is a real answer, to paste without touching the rules.

It isn't required: an answer can be in its mapping, with `body` or `jsonBody`. A few lines go there;
what is long or shared goes in `__files/`.

`rayo test --capture` writes such a folder from a real backend. All that a mapping can say, and
the commands `mock`, `mockState`, `network` and `waitForRequest`, are in the
[reference](REFERENCE.md#mock).

## Limitations

- Not supported: `assertWithAI` and the other AI commands, `addMedia`,
  `setOrientation`, sharding across devices, a
  `config.yaml` (see [suites](REFERENCE.md#suites)), and selectors by `css`, `width`, `height` or `traits`. An unknown command is an
  error, never ignored.
- `setLocation` and `travel` need Android 12 or newer.
- WebViews show only what they expose to accessibility.
- Only one tool can automate a device at a time: not while another runs.
- Tested on Windows with an Android 17 emulator, and only some of the commands. Scrolling,
  swiping, `repeat`, `retry`, `travel`, random input, relative selectors, tags and reports are
  **written but not tested yet**.

## How it works

| | |
|---|---|
| `cmd/rayo` | Command line |
| `  options.go` | Every option, in one table: the commands that take it, its check and its help |
| `  run.go` | Runs all the flows, losing none |
| `internal/suite` | Which flows, also those of a suite |
| `internal/report` | How a run went: on the screen, as text, JSON and JUnit |
| `internal/schema` | What a table of keys is, and all that is made from one |
| `internal/live` | Live sessions, and how `do` reaches them |
| `internal/flow` | Flows: reading, checking and running them |
| `  keys.go`, `kinds.go` | Every key a flow may use; the kinds of value, each checked and read by one function |
| `  commands.go` | Every command, in one table: its keys, its check and what it does |
| `  runner.go`, `report.go` | Runs a flow, and tells a `Reporter` |
| `  control.go`, `touch.go`, `input.go`, `apps.go`, `device.go`, `recording.go`, `cause.go`, `camera.go`, `mocking.go`, `script.go` | What the commands do |
| `internal/mock` | The mock backend, on its own; `capture.go` and `convert.go` make mocks from a real one |
| `internal/agent` | Starts the device part (`java/`) and talks to it |
| `internal/adb` | Talks to the adb server, sets up the device, and talks to the emulator's console |

What files and commands may hold is said once, in tables of the code (`options.go`, `keys.go`,
`commands.go`, `flow/schema.go`, `suite/fields.go`). From them come the checks, `rayo help`, the JSON Schemas
in `schema/` and the tables of the reference; a test fails if any is out of date.

`test/` runs the real rayo against a pretend device and compares all that comes out, also every
trip to the device: `go test ./test` (5 s), `-update` to accept a change. What only a real one can tell:
`go test ./test -run Real -emulator emulator-5554`.

The agent runs as the shell user with `app_process`. Waiting happens there, reacting to screen
events, so there is no polling over adb. A tap waits until the element is enabled and its
position holds still. When the connection closes, for whatever reason, the agent restores the
animations and the real location, and exits.

## License

[MIT](LICENSE). Copyright (c) 2026 [Sergio Nettleton](https://github.com/nanihadesuka).
