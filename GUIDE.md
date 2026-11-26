# Guide

## Install

Download it for your system from the [releases](https://github.com/nanihadesuka/rayo/releases/latest), and unpack it:
it is a single file. Check it against `SHA256SUMS`:

```
sha256sum -c SHA256SUMS --ignore-missing                   # Linux
shasum -a 256 -c SHA256SUMS --ignore-missing               # macOS
Get-FileHash rayo-windows-amd64.zip                        # Windows: compare with the list
```

On macOS, a file from a browser also needs `xattr -d com.apple.quarantine rayo`.

It needs adb (in Android Studio, or the SDK's platform-tools), and a device or emulator. `rayo devices`
shows what is connected. Nothing is installed on the device.

## Your first test

A test is a **flow**: a file with a header, `---`, and a list of commands.

```yaml
appId: com.example.app
---
- launchApp: { clearState: true }
- tapOn: Log in
- inputText: me@example.com
- assertVisible: "Welcome.*"
```

```
rayo check login.yaml              # mistakes, without a device
rayo test --emulator login.yaml    # run it
```

There are no pauses to add: every command waits for what it needs, and goes on as soon as it is there.

## Find elements

Open the app where you want and run `rayo hierarchy`. It shows what is on screen, and how to refer to it:

```
id: login_email  [63,1060][1017,1228]
  text: Email  [105,1123][216,1186]
id: login_submit | disabled  [63,1481][1017,1607]
```

```yaml
- tapOn: { id: login_email }
- tapOn: Continue
```

Prefer an `id`: texts change with the language. In Compose, set `testTag`, and
`testTagsAsResourceId = true` at the root. See [Selectors](REFERENCE.md#selectors).

## Write a test while it runs

```
rayo live --emulator --app com.example.app tests/login.yaml
```

Type commands and see what each does. Those that work are kept, and `.save` writes them to the file.
From a script or another program, use `rayo live --serve` and `rayo do`. See
[Live sessions](REFERENCE.md#live-sessions).

## Mock the backend

rayo answers as your backend, so tests don't depend on a server. The app sends its calls to
`http://127.0.0.1:8080` on the device, with a small [interceptor](MOCKING.md#in-your-app) that can be only in the
debug build. Can't change the app's code? Use `--network proxy` instead, see [Two ways to connect the app](MOCKING.md#two-ways-to-connect-the-app).

```yaml
appId: com.example.app
mock: mock
---
- launchApp: { clearState: true, arguments: { mock: true } }
- waitForRequest: { method: POST, path: ".*/login" }
```

See [Mocking](MOCKING.md). To begin from what the real backend answers, use
[Capture](REFERENCE.md#capture).

## Run many tests

A **suite**, `all.suite.yaml`, lists what runs together:

```yaml
name: Everything
flows:
  - tests/login.yaml
  - { file: tests/pay.yaml, retries: 3 }
app: { id: com.example.app, apk: app-debug.apk }
results: [results/results.txt, results/junit.xml]
output: { folder: results, video: true }   # a video of every flow, and every file, in results/
```

Every key of a suite has an option of the same name, in kebab-case, that overrides it for one run:
`output: { keep: failed }` in the suite, `--keep failed` on the command line. `rayo check all.suite.yaml`
says what it will write, and where.

```
rayo test --emulator all.suite.yaml
```

See [Suites](REFERENCE.md#suites), [Retries](REFERENCE.md#retries) and [Results](REFERENCE.md#results).

## When a test fails

It says what went wrong first, then whose fault it is, where, what came before, and what on screen
looked most like what it wanted:

```
[2/3] FAIL pay  9.1 s  (failed 2 times)
  pay failed at tapOn pay_button: not on screen
  why: not on screen: pay_button
  blame: test
  step: tapOn pay_button
  at: tests/pay.yaml:12
  before:
    inputText 4242 4242 4242 4242
  nearest:
    id: com.example.app:id/pay | text: Pay  [40,2100][1040,2220]
  saved: .rayo/pay
```

Here `nearest` shows the button's id is `pay`, not `pay_button`: fix the test.

`blame` says whose fault it is: the `test`, the `app` (it crashed or hung) or the `device`. If the
app or the device keeps failing, rayo stops early, so you don't wait for every test to fail the same way.

In `saved`, the screen, `pay_screen.txt`, `pay_logcat.txt` and `pay_requests.txt`: what was on
screen, the device log and the calls to the backend. Those of every try are kept; `--keep failed` keeps only those of the tries that failed, see [What is kept](REFERENCE.md#what-is-kept).

Later, to see again why it failed, without running anything:

```
rayo explain
```

Then `rayo test --rerun-failed` runs only what failed.

A test that fails and then passes on a retry is `FLAKY`. It doesn't fail the run unless you add
`--flaky-fails-build`, but it is shown, so you can fix it. See [Flaky flows](REFERENCE.md#flaky-flows).

# How do I...

## Log in before each test

Put it in a flow of its own, and run it from the others:

```yaml
- runFlow: { file: login.yaml, env: { EMAIL: me@example.com } }
```

## Keep passwords out of the files

```
rayo test -e PASSWORD=secret login.yaml
```

```yaml
- type: { id: password, value: "${PASSWORD}" }
```

## Give a subflow a default

A name nobody set is `undefined`, so the header can fall back:

```yaml
env:
  PICKUP: ${ PICKUP || "Main Street 1" }
```

## Set variables for every flow

In the suite, under `env`; `-e` overrides one. Values may use `${...}`:

```yaml
env:
  API: staging
  MAESTRO_FILENAME: ${rayo.file}   # flows from Maestro expect it: the file's name, without extension
```

See [Variables](REFERENCE.md#variables).

## Close a dialog that only appears sometimes

```yaml
- runFlow:
    when: { visible: Rate this app }
    commands:
      - tapOn: Not now
```

Or let a step fail without failing the test:

```yaml
- tapOn: { text: Not now, optional: true }
```

## Find something further down

```yaml
- scrollUntilVisible: { element: { text: Terms } }
- tapOn: Terms
```

## Test without a connection

```yaml
- network: offline
- tapOn: { id: refresh }
- assertVisible: You are offline
- network: online
```

## Test an error of the backend

```yaml
- network: { fail: 503, path: ".*/payments" }
- tapOn: { id: pay }
- assertVisible: Try again
```

## Check what the app sent

```yaml
- tapOn: { id: pay }
- waitForRequest: { method: POST, path: ".*/payments", body: "\"amount\":5" }
- assertRequest: { method: POST, path: ".*/payments", times: 1 }   # and only once
- assertRequest: { path: ".*/analytics", times: 0 }                # never this one
```

## Keep a text for later

```yaml
- copyTextFrom: { id: order_number, output: orderId }
- tapOn: { id: search }
- inputText: ${output.orderId}
```

## Check that the screen survives a rotation

```yaml
- inputText: half-written
- setOrientation: LANDSCAPE_LEFT
- assertVisible: half-written
- setOrientation: PORTRAIT        # the end of the flow puts it back anyway
```

## Split the tests between CI machines

```sh
rayo test --shard 1/4 all.suite.yaml   # on the first of 4 machines; 2/4 on the second…
```

Each test is in one part, by its path, so all machines agree whatever the order.

## Refuse a permission

```yaml
- launchApp: { clearState: true, permissions: { all: allow, camera: deny } }
```

## Pretend to be somewhere

```yaml
- setLocation: { latitude: 41.3785, longitude: 2.1765 }
- travel: { points: ["41.3785,2.1765", "41.3805,2.1748"], speed: 6 }
```

## Scan a QR

```yaml
- camera: qr.png
- tapOn: { id: scan }
```

The emulator's back camera must be *VirtualScene*.

## Record a video

```yaml
- startRecording: checkout
- tapOn: { id: pay }
- stopRecording
```

For a whole flow, or all of a suite, see [Recording a whole flow](REFERENCE.md#recording-a-whole-flow).

## Test other screen sizes

Write a suite for each:

```yaml
flows: [tests/login.yaml]
device: { size: 720x1600, dpi: 280, fontScale: 1.3, darkMode: true }
```

## Run only some tests

```
rayo test --only login,pay all.suite.yaml
rayo test --include-tags smoke tests
rayo test --rerun-failed all.suite.yaml
rayo test --changed-only all.suite.yaml
rayo test --since origin/main all.suite.yaml
```

`--changed-only` runs only the tests that changed since the last commit, also changes not committed
yet. A test counts as changed if its own file, a flow it runs with `runFlow`, or one of its mocks
changed. `--since` compares with any branch or commit instead: in a pull request, `--since origin/main`.

To see which tests would run, without a device:

```
rayo check --changed-only all.suite.yaml
```

## Run in a CI

Any CI with an emulator. `.xml` results are JUnit, and `.md` a short summary for a pull request:

```
rayo check all.suite.yaml
rayo test --emulator --results junit.xml --results summary.md all.suite.yaml
```

The exit code says what happened, so the CI can tell a broken test from a broken emulator:

<!-- generated: exit codes -->
| Code | | |
|---|---|---|
| `0` | passed | Every flow passed, or the command did what it was asked |
| `1` | failed | A flow failed, was interrupted or didn't run; or a step sent with `rayo do` failed |
| `2` | configuration | Something written is wrong: an option, a flow, a suite or a mock; nothing ran |
| `3` | device | No device to run on, or it was lost or unusable, so the flows couldn't run or didn't all run |
| `4` | flaky | Every flow passed, some only on a retry, and `flaky-fails-build` is `true`; else such a run exits `0` |
| `130` | interrupted | Ctrl-C |
<!-- end -->

The JSON results say it too, as `summary.exit`, and JUnit gives each failure the verdict as its `message`
and the cause as its `type`, so CI pages show the headline.

## Use rayo from a script or an AI agent

Add `--format json` to any command, and read its [schema](REFERENCE.md#for-a-program) with
`rayo schema`. `rayo help agents` explains the cheapest way to work: check first, run one flow,
read the cause.

```
rayo check --format json all.suite.yaml
rayo test --format ndjson --detail brief --only pay all.suite.yaml
rayo explain --format json
```

## Find one element on a busy screen

```
rayo hierarchy --find "Pay now"
```

Shows only the elements that look most like the text, the closest first, with how to refer to them.
In a [live session](REFERENCE.md#live-sessions), `rayo do --screen --find "Pay now" '- tapOn: Pay now'` does the same after a step.

## Complete and check in my editor

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/nanihadesuka/rayo/main/schema/flow.json
```

It needs the repository to be public; until then, use the path of your copy. See
[Schemas](REFERENCE.md#schemas).
