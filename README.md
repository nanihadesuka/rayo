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

Needs adb and a device or emulator. Options of `test`:

| Option | |
|---|---|
| `--device <serial>` | When several are connected |
| `--emulator` | Never a real device, even if it is the only one |
| `-e KEY=VALUE` | A variable for the flows |
| `--include-tags a,b`, `--exclude-tags a,b` | Choose flows by tag |
| `--only a,b` | Choose flows by name |
| `--rerun-failed` | Only those that haven't passed since they last failed |
| `--shuffle`, `--shuffle=<seed>` | In a random order, or that of an earlier run |
| `--retries <n>`, `--fail-fast` | Try again what fails; stop at the first failure |
| `-v` | Every step, also with several flows |
| `--results <file>`, `--results-format` | Write the [results](REFERENCE.md#results) as `txt` (the default), `json` or `junit` |
| `--failures-only` | Results without what passed |
| `--format junit --output report.xml` | Write a report |
| `--capture <folder>`, `--capture-from <url>` | Make mocks from the real backend, see [Capture](REFERENCE.md#capture) |
| `--debug-output <folder>` | Where failures and screenshots go (default `.rayo`) |
| `--profile` | Where the time of each step went |
| `--mock-port <port>` | Where the app finds the [mock](REFERENCE.md#mock) on the device (8080) |
| `--animations` | Leave the device's animations on |

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
| `cmd/rayo` | Command line, continuous mode |
| `  suite.go`, `run.go` | Which flows; running them all, losing none |
| `  live.go` | Live sessions and `do` |
| `  console.go`, `results.go`, `json.go`, `junit.go` | What it prints and writes |
| `internal/flow` | Flows: reading, checking and running them |
| `  keys.go`, `kinds.go` | Every key a flow may use; the kinds of value, each checked and read by one function |
| `  commands.go` | Every command, in one table: its keys, its check and what it does |
| `  runner.go`, `report.go` | Runs a flow, and tells a `Reporter` |
| `  control.go`, `touch.go`, `input.go`, `apps.go`, `device.go`, `recording.go`, `camera.go`, `mocking.go`, `script.go` | What the commands do |
| `internal/mock` | The mock backend, on its own; `capture.go` and `convert.go` make mocks from a real one |
| `internal/agent` | Starts the device part (`java/`) and talks to it |
| `internal/adb` | Talks to the adb server, sets up the device, and talks to the emulator's console |

`test/` runs the real rayo against a pretend device and compares all that comes out, also every
trip to the device: `go test ./test` (3 s), `-update` to accept a change.

The agent runs as the shell user with `app_process`. Waiting happens there, reacting to screen
events, so there is no polling over adb. A tap waits until the element is enabled and its
position holds still. When the connection closes, for whatever reason, the agent restores the
animations and the real location, and exits.

## License

[MIT](LICENSE). Copyright (c) 2026 [Sergio Nettleton](https://github.com/nanihadesuka).
