<p align="center"><img src="docs/icon.svg" width="140" alt=""></p>

<h1 align="center">rayo</h1>

<p align="center"><b>End-to-end tests for Android, fast as lightning.</b><br>
Android only. One portable executable. Flows in YAML.</p>

<p align="center"><a href="https://github.com/nanihadesuka/rayo/actions/workflows/check.yml?query=branch%3Amain"><img src="https://github.com/nanihadesuka/rayo/actions/workflows/check.yml/badge.svg?branch=main" alt="check"></a>
<a href="https://github.com/nanihadesuka/rayo/releases/latest"><img src="https://img.shields.io/github/v/release/nanihadesuka/rayo" alt="release"></a></p>

<p align="center"><a href="https://github.com/nanihadesuka/rayo/releases/latest/download/rayo-windows-amd64.zip"><img src="https://img.shields.io/badge/download-Windows%20x64-0078D6" alt="Download for Windows x64"></a>
<a href="https://github.com/nanihadesuka/rayo/releases/latest/download/rayo-linux-amd64.tar.gz"><img src="https://img.shields.io/badge/download-Linux%20x64-E95420" alt="Download for Linux x64"></a>
<a href="https://github.com/nanihadesuka/rayo/releases/latest/download/rayo-macos-arm64.tar.gz"><img src="https://img.shields.io/badge/download-macOS%20Apple%20silicon-555555" alt="Download for macOS Apple silicon"></a><br>
<sub>Others (arm64 Windows and Linux, Intel Macs) are in the <a href="https://github.com/nanihadesuka/rayo/releases/latest">release</a>.</sub></p>

<p align="center"><a href="LICENSE">MIT License</a> © 2026 <a href="https://github.com/nanihadesuka">Sergio Nettleton</a></p>

<br>

<p align="center"><a href="https://nanihadesuka.github.io/rayo/"><img src="https://img.shields.io/badge/Documentation-nanihadesuka.github.io%2Frayo-F5B800?style=for-the-badge" alt="Documentation" height="40"></a><br>
<sub>Get started, every command, mocking, suites and results, with search.</sub></p>

<br>

```yaml
appId: com.example.app
mock: mock
---
- launchApp: { clearState: true, arguments: { mock: true } }
- tapOn: { id: login }
- inputText: me@example.com
- extendedWaitUntil: { visible: { id: home }, timeout: 20s }
- waitForRequest: { method: POST, path: ".*/session" }
```

```
$ rayo test --emulator all.suite.yaml
[1/3] PASS book  18.2 s
[2/3] FLAKY scan  41.0 s  (failed once, then passed)
  try 1: scan.yaml:12: tapOn scan: not on screen: scan  (test)  .results/scan
[3/3] FAIL pay  9.1 s  (failed all 2 tries)
  ...

TOTAL 3: 1 passed, 1 flaky, 1 failed, 2 retried  68.3 s
```

## Features

- **Testing**
  - **Fast execution:** built exclusively for Android, rayo synchronizes with the device's UI state directly
    instead of relying on fixed sleeps or polling delays.
  - **[Live sessions](REFERENCE.md#live-sessions):** build a test interactively while it runs on the device,
    either manually or driven by an external program.
  - **Test organization:** suites, scripting, tags, randomized execution order, and [JSON schemas](schema)
    for editor autocompletion and validation.
  - **[Retries and flaky detection](REFERENCE.md#retries):** failed tests can be retried, and
    [flaky tests](REFERENCE.md#flaky-flows) are reported separately.
- **Mocking**
  - **[Built-in mock server](MOCKING.md):** supports stateful scenarios and simulated network latency or failures.
    Any unmatched request fails the test and reports the closest matching stub.
  - **[Traffic capture](REFERENCE.md#capture):** record responses from the real backend and turn them into mocks.
    REST requests are matched on body content, [GraphQL](REFERENCE.md#graphql) requests on operation name and
    variables, independent of call order.
  - **[Mock updates](REFERENCE.md#updating-mocks):** diff your mocks against the live backend, apply changes
    selectively, or use `--check` to fail CI when mocks are out of date.
  - **[Two connection modes](MOCKING.md#two-ways-to-connect-the-app):** a small interceptor in the app's debug
    build, or a device-wide proxy that needs no code changes.
- **Debugging**
  - **Failure analysis:** each failure reports its cause and location, the screen state, network calls and
    crash logs, and whether the test, the app or the device is at fault.
  - **[Explain](REFERENCE.md#explain):** `rayo explain` summarizes why the last run failed.
  - **Fix as it fails:** hold a run at a failure, fix the files, and retry from the failing step without
    rerunning the whole flow.
  - **[Early abort](REFERENCE.md#blame):** the run stops early when the device or the app itself is at fault.
- **Reporting**
  - **[Result formats](REFERENCE.md#results):** text, JSON, JUnit or Markdown.
  - **[Video recording](REFERENCE.md#startrecording-stoprecording):** compact screen recordings per test or for
    specific steps.
  - **[Automation friendly](REFERENCE.md#for-a-program):** machine-readable JSON output and well-defined exit
    codes for CI pipelines, scripts and AI agents.
- **More**
  - **Single binary:** a self-contained executable for Windows, Linux and macOS. The only dependency is `adb`;
    nothing is installed on the device.
  - **Location simulation:** [`setLocation`](REFERENCE.md#setlocation) sets a fixed GPS position, and
    [`travel`](REFERENCE.md#travel) moves the device along a route at a realistic speed, with no app changes.
  - **Device control:** camera QR code injection, runtime permissions, airplane mode, screen rotation,
    display size and font scale.
  - **AI agent skill:** `rayo skills add` installs a skill for Claude Code and Codex covering writing,
    running, mocking and fixing tests.
- **Compatibility**
  - **Maestro:** existing Maestro flows run with little to no modification, using the same YAML commands,
    parameter names and default behaviors.
  - **WireMock:** stub mappings, including stateful scenarios, are loaded as-is.
  - **mitmproxy, Charles and browser dev tools:** HAR exports can be imported and converted into mocks.

## Start

[Download it](https://github.com/nanihadesuka/rayo/releases/latest), unpack it, and check it:

```
sha256sum -c SHA256SUMS --ignore-missing
```

Or build it, with Go: `bash build.sh`.

| | |
|---|---|
| `rayo hierarchy` | What is on screen, and how to refer to it |
| `rayo test flow.yaml` | Run a flow, a folder or a suite |
| `rayo hierarchy --find Pay` | Only the elements that look like "Pay" |
| `rayo check flow.yaml` | Find mistakes, and see what would run, without a device |
| `rayo test --changed-only all.suite.yaml` | Run only the tests you changed |
| `rayo explain` | Why the last run failed |
| `rayo live new.yaml --app com.example.app` | Write a test while it runs |
| `rayo do '- tapOn: Save'` | The same, from another program |
| `rayo test --capture mock flow.yaml` | Make mocks from the real backend |
| `rayo test --capture-update mock flow.yaml` | What changed in the backend since; `rayo mocks apply mock` takes it |
| `rayo mocks import flows.har mock` | Make mocks of mitmproxy's, a browser's or Charles' calls |
| `rayo mocks check mock` | Mappings that never answer, and bodies no mapping uses |
| `rayo test --shard 2/4 all.suite.yaml` | Run a quarter of the tests, on each of 4 CI machines |
| `rayo skills add` | Install the rayo skill for Claude Code and Codex |
| `rayo help agents` | How a script or an AI agent should use rayo |
| `rayo help` | Everything else |

## Connecting the app to the mocks

To mock or capture the backend, the app's calls have to reach rayo. There are [two ways](MOCKING.md#two-ways-to-connect-the-app):

| | How | Needs |
|---|---|---|
| **Interceptor** (default) | The app itself sends its calls to rayo | A few lines of code, only in the debug build |
| **Proxy** (`--network proxy`) | The phone's network settings send every call through rayo | No code; the debug build trusts rayo's certificate |

Start with the interceptor if you can change the app. Use the proxy if you can't, or if an SDK in the app
makes calls of its own. Everything else in rayo works with any app as it is.

## Many tests

A suite is a file named `*.suite.yaml`. It says which tests run together and how; only `flows` is
required. Give its file to run it. Give a folder instead, and the tests directly in it run.

```yaml
flows:
  - tests/book/flow.yaml
  - { file: tests/scan/flow.yaml, retries: 3 }
app: { id: com.example.app, apk: app-debug.apk }
results: [.results/results.txt, .results/junit.xml]
output: { folder: .results, video: true, keep: failed }          # what a run writes; --output, --video, --keep override
env: { CITY: Springfield, MAESTRO_FILENAME: "${rayo.file}" }   # for every flow; -e overrides
device: { size: 1080x2400, dpi: 420, darkMode: true }
```

## More

**[📖 Documentation](https://nanihadesuka.github.io/rayo/)**, the easiest way to read all of it, with search.

Or here: [Guide](GUIDE.md) · [What's new](https://github.com/nanihadesuka/rayo/releases/latest) · [Full example](EXAMPLE.md) · [Reference](REFERENCE.md) · [Mocking](MOCKING.md) · [Working on rayo](CONTRIBUTING.md)
