<p align="center"><img src="docs/icon.svg" width="140" alt=""></p>

<h1 align="center">rayo</h1>

<p align="center"><b>End-to-end tests for Android, fast as lightning.</b><br>
Android only. One portable executable. Flows in YAML.</p>

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
$ rayo test --emulator all.rayo.yaml
[1/3] PASS book  18.2 s
[2/3] PASS scan  41.0 s  (failed once, then passed)
  try 1: scan.yaml:12: tapOn scan: not on screen: scan  (test)  .results/scan
[3/3] FAIL pay  9.1 s  (failed 2 times)
  ...

TOTAL 3: 2 passed, 1 failed, 2 retried  68.3 s
```

## Why

- **Fast.** Made for Android only: it waits on the device itself and reacts to the screen, without fixed pauses.
- **One executable.** Windows, Linux or macOS. It only needs adb, and installs nothing on the device.
- **[Mock backend](MOCKING.md) built in.** States, slow or broken networks; a call without a mock fails the test.
- **[Capture](REFERENCE.md#capture).** Run once on the real backend, and its answers become mocks.
- **[Videos](REFERENCE.md#startrecording-stoprecording)**, small, of each test or some steps.
- **[Live](REFERENCE.md#live-sessions).** Write a test while it runs, by hand, from a script or an LLM.
- **[Honest results](REFERENCE.md#results)** as text, JSON or JUnit: why, where, the screen, the calls and the crash.
- **[Retries](REFERENCE.md#retries)**, and it [stops early](REFERENCE.md#causes) if the device or the app is what fails.
- **And** location and trips, QR for the camera, permissions, airplane mode, screen size and font, scripts, tags,
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
| `rayo check flow.yaml` | Find mistakes, without a device |
| `rayo live new.yaml --app com.example.app` | Write a test while it runs |
| `rayo do '- tapOn: Save'` | The same, from a script or an LLM |
| `rayo test --capture mock flow.yaml` | Make mocks from the real backend |
| `rayo help` | Everything else |

## Mocking needs an interceptor

For mocking and capturing, the app has to send its calls to rayo, at `http://127.0.0.1:8080` on the
device. That takes a small [interceptor](MOCKING.md#in-your-app) in the app, which can be **only in the
debug build**. Everything else works with any app as it is.

## Many tests

A suite, `*.rayo.yaml`, says what runs together and how. All but `flows` is optional.

```yaml
flows:
  - tests/book/flow.yaml
  - { file: tests/scan/flow.yaml, retries: 3 }
app: { id: com.example.app, apk: app-debug.apk }
results: [.results/results.txt, .results/junit.xml]
record: true
device: { size: 1080x2400, dpi: 420, darkMode: true }
```

## More

[Guide](GUIDE.md) · [Reference](REFERENCE.md) · [Mocking](MOCKING.md) · [Working on rayo](CONTRIBUTING.md)

## License

[MIT](LICENSE). Copyright (c) 2026 [Sergio Nettleton](https://github.com/nanihadesuka).
