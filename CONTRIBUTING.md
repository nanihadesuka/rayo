# Working on rayo

## Build

```
bash build.sh            # bin/rayo, needs Go
bash build.sh --all      # for Windows, Linux and macOS
bash build.sh --release  # archives in dist/, with SHA256SUMS
bash build.sh --agent    # also the device part, needs a JDK and the Android SDK
```

`internal/agent/agent.jar` is committed, so Go is enough unless `internal/agent/java` changes.

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
in `schema/`, the tables of the reference and the page in `docs/`; a test fails if any is out of date.

`test/` runs the real rayo against a pretend device and compares all that comes out, also every
trip to the device: `go test ./test` (5 s), `-update` to accept a change. What only a real one can tell:
`go test ./test -run Real -emulator emulator-5554`.

The agent runs as the shell user with `app_process`. Waiting happens there, reacting to screen
events, so there is no polling over adb. A tap waits until the element is enabled and its
position holds still. When the connection closes, for whatever reason, the agent restores the
animations and the real location, and exits.

## The documentation's page

`docs/` is a page for GitHub Pages, without anything to build: `data.js` holds `GUIDE.md`, `EXAMPLE.md`,
`REFERENCE.md`, `MOCKING.md`, this file and `rayo help json`. `app.js` says which section goes on which page. Change those, and run `go test ./test -update`. To see it,
open `docs/index.html`.
