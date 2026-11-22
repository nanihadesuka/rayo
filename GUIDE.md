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
From a script or an LLM, use `rayo live --serve` and `rayo do`. See
[Live sessions](REFERENCE.md#live-sessions).

## Mock the backend

rayo answers as your backend, so tests don't depend on a server. The app sends its calls to
`http://127.0.0.1:8080` on the device, with a small [interceptor](MOCKING.md#in-your-app) that can be only in the
debug build.

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

A **suite**, `all.rayo.yaml`, lists what runs together:

```yaml
name: Everything
flows:
  - tests/login.yaml
  - { file: tests/pay.yaml, retries: 3 }
app: { id: com.example.app, apk: app-debug.apk }
results: [results/results.txt, results/junit.xml]
record: true
recordFolder: results/videos
```

```
rayo test --emulator all.rayo.yaml
```

See [Suites](REFERENCE.md#suites), [Retries](REFERENCE.md#retries) and [Results](REFERENCE.md#results).

## When a test fails

It says why, where, what came before, and what was on screen:

```
[2/3] FAIL pay  9.1 s  (failed 2 times)
  why: not on screen: pay_button
  cause: test
  step: tapOn pay_button
  at: tests/pay.yaml:12
  screen:
    id: cart
  saved: .rayo/pay
```

The folder has a screenshot, the elements, the device log and the calls to the backend.
`rayo test --rerun-failed` runs only what failed.

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
```

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
rayo test --only login,pay all.rayo.yaml
rayo test --include-tags smoke tests
rayo test --rerun-failed all.rayo.yaml
```

## Run in a CI

Any CI with an emulator. The exit code isn't 0 if a test fails, and `.xml` results are JUnit:

```
rayo check all.rayo.yaml
rayo test --emulator --results junit.xml all.rayo.yaml
```

## Complete and check in my editor

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/nanihadesuka/rayo/main/schema/flow.json
```

It needs the repository to be public; until then, use the path of your copy. See
[Schemas](REFERENCE.md#schemas).
