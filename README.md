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
[3/3] FAIL pay  9.1 s  (failed 2 times)
  ...

TOTAL 3: 1 passed, 1 flaky, 1 failed, 2 retried  68.3 s
```

## Why

- **Fast.** Made for Android only: it waits on the device itself and reacts to the screen, without fixed pauses.
- **One executable.** Windows, Linux or macOS. It only needs adb, and installs nothing on the device.
- **[Mock backend](MOCKING.md) built in.** WireMock's format: its mocks load as they are. States, slow or
  broken networks; a call without a mock fails the test, and says which mock came nearest.
- **[Capture](REFERENCE.md#capture).** Run once on the real backend, and its answers become mocks, matched
  as WireMock's recorder does: REST by body too, and [GraphQL](REFERENCE.md#graphql) by operation and
  variables, so the order doesn't matter; dates and what you name left out. Or import a HAR file of mitmproxy's.
- **[Update mocks](REFERENCE.md#updating-mocks).** See what the backend changed since, take only the changes
  you want, or fail CI with `--check` when the mocks are behind.
- **[Videos](REFERENCE.md#startrecording-stoprecording)**, small, of each test or some steps.
- **Pretend location and trips.** [`setLocation`](REFERENCE.md#setlocation) puts the device anywhere;
  [`travel`](REFERENCE.md#travel) moves it along a route at a real speed. For any app, without changing it.
- **[Live](REFERENCE.md#live-sessions).** Write a test while it runs, by hand or from another program.
- **[Honest results](REFERENCE.md#results)** as text, JSON, JUnit or Markdown: why, where, the screen, the calls,
  the crash, and whose fault it is. [`rayo explain`](REFERENCE.md#explain) repeats it later.
- **[Retries](REFERENCE.md#retries)**, with [flaky](REFERENCE.md#flaky-flows) tests shown as such, and it
  [stops early](REFERENCE.md#blame) if the device or the app is what fails.
- **[For programs](REFERENCE.md#for-a-program).** JSON output and clear exit codes, for CI, scripts and AI agents.
- **And** QR for the camera, permissions, airplane mode, screen rotation, size and font, scripts, tags,
  random order, [schemas](schema) for your editor.

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
