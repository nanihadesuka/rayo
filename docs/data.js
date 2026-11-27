// Written by go test ./test -update
window.RAYO = {
 "markdown": {
  "CONTRIBUTING.md": "# Working on rayo\n\n## Build\n\n```\nbash build.sh            # bin/rayo, needs Go\nbash build.sh --all      # for Windows, Linux and macOS\nbash build.sh --release  # archives in dist/, with SHA256SUMS\nbash build.sh --agent    # also the device part, needs a JDK and the Android SDK\n```\n\n`internal/agent/agent.jar` is committed, so Go is enough unless `internal/agent/java` changes.\n\n## How it works\n\n| | |\n|---|---|\n| `cmd/rayo` | Command line |\n| `  options.go` | Every option, in one table: the commands that take it, its check and its help |\n| `  run.go` | Runs all the flows, losing none |\n| `internal/suite` | Which flows, also those of a suite |\n| `internal/report` | How a run went: on the screen, as text, JSON and JUnit |\n| `internal/schema` | What a table of keys is, and all that is made from one |\n| `internal/live` | Live sessions, and how `do` reaches them |\n| `internal/flow` | Flows: reading, checking and running them |\n| `  keys.go`, `kinds.go` | Every key a flow may use; the kinds of value, each checked and read by one function |\n| `  commands.go` | Every command, in one table: its keys, its check and what it does |\n| `  runner.go`, `report.go` | Runs a flow, and tells a `Reporter` |\n| `  control.go`, `touch.go`, `input.go`, `apps.go`, `device.go`, `recording.go`, `cause.go`, `camera.go`, `mocking.go`, `script.go` | What the commands do |\n| `internal/mock` | The mock backend, on its own; `capture.go` and `convert.go` make mocks from a real one |\n| `internal/agent` | Starts the device part (`java/`) and talks to it |\n| `internal/adb` | Talks to the adb server, sets up the device, and talks to the emulator's console |\n\nWhat files and commands may hold is said once, in tables of the code (`options.go`, `keys.go`,\n`commands.go`, `flow/schema.go`, `suite/fields.go`). From them come the checks, `rayo help`, the JSON Schemas\nin `schema/`, the tables of the reference, the page in `docs/`, and `rayo help all` as `man/rayo.1` and `llms.txt`; a test fails if any is out of date.\n\n`test/` runs the real rayo against a pretend device and compares all that comes out, also every\ntrip to the device: `go test ./test` (5 s), `-update` to accept a change. What only a real one can tell:\n`go test ./test -run Real -emulator emulator-5554`.\n\nThe agent runs as the shell user with `app_process`. Waiting happens there, reacting to screen\nevents, so there is no polling over adb. A tap waits until the element is enabled and its\nposition holds still. When the connection closes, for whatever reason, the agent restores the\nanimations and the real location, and exits.\n\n## The documentation's page\n\n`docs/` is a page for GitHub Pages, without anything to build: `data.js` holds `GUIDE.md`, `EXAMPLE.md`,\n`REFERENCE.md`, `MOCKING.md`, this file and `rayo help json`. `app.js` says which section goes on which page. Change those, and run `go test ./test -update`. To see it,\nopen `docs/index.html`.\n",
  "EXAMPLE.md": "# A full example\n\nA shop: log in, buy something, the bank refuses the card, and no connection. These are all its files, in\n[examples/shop](examples/shop). The app is made up; the files are real, and rayo checks them.\n\n## The folders\n\n```\nshop/\n├─ all.suite.yaml               what runs together, and how\n├─ steps/login.yaml            steps that several tests use\n├─ tests/\n│   ├─ browse/flow.yaml\n│   ├─ pay/flow.yaml\n│   ├─ declined/flow.yaml      and declined.json, an answer only for it\n│   └─ offline/flow.yaml\n└─ mock/\n    ├─ mappings/shop.json      which call gets which answer\n    └─ __files/products.json   a long answer\n```\n\n## The suite\n\n\u003c!-- file: examples/shop/all.suite.yaml --\u003e\n\n## Steps that are shared\n\nEach test begins by running it, so each begins clean and can be run alone.\n\n\u003c!-- file: examples/shop/steps/login.yaml --\u003e\n\n## The tests\n\nThe simplest one:\n\n\u003c!-- file: examples/shop/tests/browse/flow.yaml --\u003e\n\nThis one checks what the app sent, and changes what the backend says:\n\n\u003c!-- file: examples/shop/tests/pay/flow.yaml --\u003e\n\nThis one adds an answer that wins over the normal one:\n\n\u003c!-- file: examples/shop/tests/declined/flow.yaml --\u003e\n\u003c!-- file: examples/shop/tests/declined/declined.json --\u003e\n\nAnd this one cuts the connection:\n\n\u003c!-- file: examples/shop/tests/offline/flow.yaml --\u003e\n\n## The backend\n\n\u003c!-- file: examples/shop/mock/mappings/shop.json --\u003e\n\u003c!-- file: examples/shop/mock/__files/products.json --\u003e\n\n## Run it\n\n```\nrayo check all.suite.yaml                 # mistakes, without a device\nrayo test --emulator all.suite.yaml       # all of it\nrayo test --only pay all.suite.yaml       # one\n```\n\n## What comes out\n\nHow it would look, if `pay` failed once and the app had a bug in `offline`:\n\n```\n[1/4] PASS browse  9.8 s\n[2/4] PASS pay  31.4 s  (failed once, then passed)\n  try 1: tests/pay/flow.yaml:13: tapOn refresh: not on screen: refresh  (test)  results/pay_failure.jpg\n[3/4] PASS declined  11.2 s\n[4/4] FAIL offline  41.9 s  (failed all 2 tries)\n  try: 1\n  why: not on screen: You are offline\n  blame: app, the app crashed: java.net.ConnectException: unexpected end of stream\n  step: assertVisible You are offline\n  at: tests/offline/flow.yaml:7\n  crash: FATAL EXCEPTION: main\n    Process: com.example.shop, PID: 4711\n    java.net.ConnectException: unexpected end of stream\n  before:\n    extendedWaitUntil products\n    network offline\n    tapOn refresh\n  saved: results/offline_failure.jpg\n  recording: results/videos/offline.mp4\n  ...\n\nTOTAL 4: 3 passed, 1 failed, of those failed 1 app, 2 retried  94.3 s\n```\n\nAnd in `results/`:\n\n```\nresults/\n├─ results.txt                 the same, with the details of every try\n├─ junit.xml                   for a CI\n├─ failed.json                 what --rerun-failed runs\n├─ videos/                     pay.mp4, offline.mp4, offline_retry1.mp4: of the tries that failed\n├─ offline_failure.jpg         of each failed try: the screen\n├─ offline_screen.txt          the elements on it\n├─ offline_logcat.txt          the device log\n├─ offline_requests.txt        the calls to the backend\n└─ offline_retry1_failure.jpg  ... and so on for the second try\n```\n",
  "GUIDE.md": "# Guide\n\n## Install\n\nDownload it for your system from the [releases](https://github.com/nanihadesuka/rayo/releases/latest), and unpack it:\nit is a single file. Check it against `SHA256SUMS`:\n\n```\nsha256sum -c SHA256SUMS --ignore-missing                   # Linux\nshasum -a 256 -c SHA256SUMS --ignore-missing               # macOS\nGet-FileHash rayo-windows-amd64.zip                        # Windows: compare with the list\n```\n\nOn macOS, a file from a browser also needs `xattr -d com.apple.quarantine rayo`.\n\nIt needs adb (in Android Studio, or the SDK's platform-tools), and a device or emulator. `rayo devices`\nshows what is connected. Nothing is installed on the device.\n\n## Your first test\n\nA test is a **flow**: a file with a header, `---`, and a list of commands.\n\n```yaml\nappId: com.example.app\n---\n- launchApp: { clearState: true }\n- tapOn: Log in\n- inputText: me@example.com\n- assertVisible: \"Welcome.*\"\n```\n\n```\nrayo check login.yaml              # mistakes, without a device\nrayo test --emulator login.yaml    # run it\n```\n\nThere are no pauses to add: every command waits for what it needs, and goes on as soon as it is there.\n\n## Find elements\n\nOpen the app where you want and run `rayo hierarchy`. It shows what is on screen, and how to refer to it:\n\n```\nid: login_email  [63,1060][1017,1228]\n  text: Email  [105,1123][216,1186]\nid: login_submit | disabled  [63,1481][1017,1607]\n```\n\n```yaml\n- tapOn: { id: login_email }\n- tapOn: Continue\n```\n\nPrefer an `id`: texts change with the language. In Compose, set `testTag`, and\n`testTagsAsResourceId = true` at the root. See [Selectors](REFERENCE.md#selectors).\n\n## Write a test while it runs\n\n```\nrayo live --emulator --app com.example.app tests/login.yaml\n```\n\nType commands and see what each does. Those that work are kept, and `.save` writes them to the file.\nFrom a script or another program, use `rayo live --serve` and `rayo do`. See\n[Live sessions](REFERENCE.md#live-sessions).\n\n## Mock the backend\n\nrayo answers as your backend, so tests don't depend on a server. The app sends its calls to\n`http://127.0.0.1:8080` on the device, with a small [interceptor](MOCKING.md#in-your-app) that can be only in the\ndebug build. Can't change the app's code? Use `--network proxy` instead, see [Two ways to connect the app](MOCKING.md#two-ways-to-connect-the-app).\n\n```yaml\nappId: com.example.app\nmock: mock\n---\n- launchApp: { clearState: true, arguments: { mock: true } }\n- waitForRequest: { method: POST, path: \".*/login\" }\n```\n\nSee [Mocking](MOCKING.md). To begin from what the real backend answers, use\n[Capture](REFERENCE.md#capture).\n\n## Run many tests\n\nA **suite**, `all.suite.yaml`, lists what runs together:\n\n```yaml\nname: Everything\nflows:\n  - tests/login.yaml\n  - { file: tests/pay.yaml, retries: 3 }\napp: { id: com.example.app, apk: app-debug.apk }\nresults: [results/results.txt, results/junit.xml]\noutput: { folder: results, video: true }   # a video of every flow, and every file, in results/\n```\n\nEvery key of a suite has an option of the same name, in kebab-case, that overrides it for one run:\n`output: { keep: failed }` in the suite, `--keep failed` on the command line. `rayo check all.suite.yaml`\nsays what it will write, and where.\n\n```\nrayo test --emulator all.suite.yaml\n```\n\nSee [Suites](REFERENCE.md#suites), [Retries](REFERENCE.md#retries) and [Results](REFERENCE.md#results).\n\n## When a test fails\n\nIt says what went wrong first, then whose fault it is, where, what came before, and what on screen\nlooked most like what it wanted:\n\n```\n[2/3] FAIL pay  9.1 s  (failed all 2 tries)\n  pay failed at tapOn pay_button: not on screen\n  why: not on screen: pay_button\n  blame: test\n  step: tapOn pay_button\n  at: tests/pay.yaml:12\n  before:\n    inputText 4242 4242 4242 4242\n  nearest:\n    id: com.example.app:id/pay | text: Pay  [40,2100][1040,2220]\n  saved: .rayo/pay\n```\n\nHere `nearest` shows the button's id is `pay`, not `pay_button`: fix the test.\n\n`blame` says whose fault it is: the `test`, the `app` (it crashed, hung, or showed nothing readable)\nor the `device`. If the app or the device keeps failing, rayo stops early, so you don't wait for\nevery test to fail the same way.\n\nIn `saved`, the screen, `pay_screen.txt`, `pay_logcat.txt` and `pay_requests.txt`: what was on\nscreen, the device log and the calls to the backend. Those of every try are kept; `--keep failed` keeps only those of the tries that failed, see [What is kept](REFERENCE.md#what-is-kept).\n\nLater, to see again why it failed, without running anything:\n\n```\nrayo explain\n```\n\nThen `rayo test --rerun-failed` runs only what failed.\n\nA test that fails and then passes on a retry is `FLAKY`. It doesn't fail the run unless you add\n`--flaky-fails-build`, but it is shown, so you can fix it. See [Flaky flows](REFERENCE.md#flaky-flows),\nand [Fix a flaky test](#fix-a-flaky-test).\n\n# How do I...\n\n## Log in before each test\n\nPut it in a flow of its own, and run it from the others:\n\n```yaml\n- runFlow: { file: login.yaml, env: { EMAIL: me@example.com } }\n```\n\n## Keep passwords out of the files\n\n```\nrayo test -e PASSWORD=secret login.yaml\n```\n\n```yaml\n- type: { id: password, value: \"${PASSWORD}\" }\n```\n\n## Give a subflow a default\n\nA name nobody set is `undefined`, so the header can fall back:\n\n```yaml\nenv:\n  PICKUP: ${ PICKUP || \"Main Street 1\" }\n```\n\n## Set variables for every flow\n\nIn the suite, under `env`; `-e` overrides one. Values may use `${...}`:\n\n```yaml\nenv:\n  API: staging\n  MAESTRO_FILENAME: ${rayo.file}   # flows from Maestro expect it: the file's name, without extension\n```\n\nSee [Variables](REFERENCE.md#variables).\n\n## Close a dialog that only appears sometimes\n\n```yaml\n- runFlow:\n    when: { visible: Rate this app }\n    commands:\n      - tapOn: Not now\n```\n\nOr let a step fail without failing the test:\n\n```yaml\n- tapOn: { text: Not now, optional: true }\n```\n\n## Find something further down\n\n```yaml\n- scrollUntilVisible: { element: { text: Terms } }\n- tapOn: Terms\n```\n\n## Test without a connection\n\n```yaml\n- network: offline\n- tapOn: { id: refresh }\n- assertVisible: You are offline\n- network: online\n```\n\n## Test an error of the backend\n\n```yaml\n- network: { fail: 503, path: \".*/payments\" }\n- tapOn: { id: pay }\n- assertVisible: Try again\n```\n\n## Check what the app sent\n\n```yaml\n- tapOn: { id: pay }\n- waitForRequest: { method: POST, path: \".*/payments\", body: \"\\\"amount\\\":5\" }\n- assertRequest: { method: POST, path: \".*/payments\", times: 1 }   # and only once\n- assertRequest: { path: \".*/analytics\", times: 0 }                # never this one\n```\n\n## Keep a text for later\n\n```yaml\n- copyTextFrom: { id: order_number, output: orderId }\n- tapOn: { id: search }\n- inputText: ${output.orderId}\n```\n\n## Check that the screen survives a rotation\n\n```yaml\n- inputText: half-written\n- setOrientation: LANDSCAPE_LEFT\n- assertVisible: half-written\n- setOrientation: PORTRAIT        # the end of the flow puts it back anyway\n```\n\n## Split the tests between CI machines\n\n```sh\nrayo test --shard 1/4 all.suite.yaml   # on the first of 4 machines; 2/4 on the second…\n```\n\nEach test is in one part, by its path, so all machines agree whatever the order. Each part writes to\n`.rayo/shard-1-of-4` unless `--output` says, and its results say which part they are: `part 1 of 4` in the text\nand `.md`, `rayo-shard-1-of-4` as the JUnit suite's name, and `shard` in the JSON. A part with no tests still\nwrites its results, with none.\n\n## Refuse a permission\n\n```yaml\n- launchApp: { clearState: true, permissions: { all: allow, camera: deny } }\n```\n\n## Pretend to be somewhere\n\n```yaml\n- setLocation: { latitude: 41.3785, longitude: 2.1765 }\n- travel: { points: [\"41.3785,2.1765\", \"41.3805,2.1748\"], speed: 6 }\n```\n\n## Scan a QR\n\n```yaml\n- camera: qr.png\n- tapOn: { id: scan }\n```\n\nThe emulator's back camera must be *VirtualScene*.\n\n## Record a video\n\n```yaml\n- startRecording: checkout\n- tapOn: { id: pay }\n- stopRecording\n```\n\nFor a whole flow, or all of a suite, see [Recording a whole flow](REFERENCE.md#recording-a-whole-flow).\n\n## Test other screen sizes\n\nWrite a suite for each:\n\n```yaml\nflows: [tests/login.yaml]\ndevice: { size: 720x1600, dpi: 280, fontScale: 1.3, darkMode: true }\n```\n\n## Run only some tests\n\n```\nrayo test --only login,pay all.suite.yaml\nrayo test --include-tags smoke tests\nrayo test --rerun-failed all.suite.yaml\nrayo test --changed-only all.suite.yaml\nrayo test --since origin/main all.suite.yaml\n```\n\n`--changed-only` runs only the tests that changed since the last commit, also changes not committed\nyet. A test counts as changed if its own file, a flow it runs with `runFlow`, or one of its mocks\nchanged. `--since` compares with any branch or commit instead: in a pull request, `--since origin/main`.\n\nTo see which tests would run, without a device:\n\n```\nrayo check --changed-only all.suite.yaml\n```\n\nAdd `--min-flows 20` (or `minFlows: 20` in the suite) so that a typo in a tag runs fewer tests and fails,\nwith exit `2`, instead of a green run of nothing.\n\n## Fix a flaky test\n\nA `FLAKY` test comes with a `next:` line, by how its first try failed. Most often the screen wasn't there in\ntime: wait for what comes before it, with `assertVisible` or `waitForRequest`, rather than a longer timeout.\n`rayo test --timings` lists as `tight` the steps that passed using more than 70% of their timeout: those\nfail on a slower day. To see where it fails, run it alone a few times with `--only name --retries 0`.\nA `retry:` step says in its output which round it is and what failed before.\n\n## Run in a CI\n\nAny CI with an emulator. `.xml` results are JUnit, and `.md` a short summary for a pull request:\n\n```\nrayo check all.suite.yaml\nrayo test --emulator --results junit.xml --results summary.md all.suite.yaml\n```\n\nThe exit code says what happened, so the CI can tell a broken test from a broken emulator:\n\n\u003c!-- generated: exit codes --\u003e\n| Code | | |\n|---|---|---|\n| `0` | passed | Every flow passed, or the command did what it was asked |\n| `1` | failed | A flow failed, was interrupted or didn't run; or a step sent with `rayo do` failed |\n| `2` | configuration | Something written is wrong: an option, a flow, a suite or a mock; nothing ran |\n| `3` | device | No device to run on, or it was lost or unusable, so the flows couldn't run or didn't all run |\n| `4` | flaky | Every flow passed, some only on a retry, and `flaky-fails-build` is `true`; else such a run exits `0` |\n| `130` | interrupted | Ctrl-C, or SIGTERM as CI sends when it cancels a job: what ran is in the results, each interrupted flow with the step it was at |\n\u003c!-- end --\u003e\n\nThe JSON results say it too, as `summary.exit`, and JUnit gives each failure the verdict as its `message`\nand the cause as its `type`, so CI pages show the headline.\n\n## Use rayo from a script or an AI agent\n\nAdd `--format json` to any command, and read its [schema](REFERENCE.md#for-a-program) with\n`rayo schema`. `rayo help agents` explains the cheapest way to work: check first, run one flow,\nread the cause.\n\n```\nrayo check --format json all.suite.yaml\nrayo test --format ndjson --detail brief --only pay all.suite.yaml\nrayo explain --format json\n```\n\n## Find one element on a busy screen\n\n```\nrayo hierarchy --find \"Pay now\"\n```\n\nShows only the elements that look most like the text, the closest first, with how to refer to them.\nIn a [live session](REFERENCE.md#live-sessions), `rayo do --screen --find \"Pay now\" '- tapOn: Pay now'` does the same after a step.\n\n## Complete and check in my editor\n\n```yaml\n# yaml-language-server: $schema=https://raw.githubusercontent.com/nanihadesuka/rayo/main/schema/flow.json\n```\n\nFor a suite, `schema/suite.json` the same way. Offline, or to pin your version, use the path of\nyour copy instead, such as `# yaml-language-server: $schema=../rayo/schema/flow.json`. See\n[Schemas](REFERENCE.md#schemas).\n",
  "MOCKING.md": "# Mocking\n\nrayo answers as the backend. A mock is a folder: `mappings/` says **when** to answer, and `__files/`\nholds **what** to answer. The names are fixed.\n\n```\nmock/\n├─ mappings/                 the rules: if the app calls this, answer that\n│   ├─ products.json\n│   └─ order.json\n└─ __files/                  the bodies, as the backend would send them\n    ├─ products/all.json\n    └─ order/shipped.json\n```\n\nA flow names its mocks, in order, in its header:\n\n```yaml\nmock: [../shared/mock, mock/overrides.json]\n```\n\n## Two ways to connect the app\n\nrayo can only answer calls that reach it. There are two ways to send them there:\n\n- **Interceptor** (the default). The app itself sends its calls to rayo instead of the server. You add\n  [a few lines](#in-your-app) to the app, only in its debug build. It sees the calls made through the\n  app's own HTTP client.\n- **Proxy** (`--network proxy`). rayo sets the phone's network proxy while the test runs, so every call goes\n  through rayo, also those of SDKs in the app. [No code](#proxy): the debug build only has to trust rayo's\n  certificate, from `rayo ca export`.\n\nWhich one:\n\n- You can change the app, and all its calls go through one client: **interceptor**. Simplest, nothing on the phone changes.\n- You can't change the app's code, or an SDK (payments, maps, chat…) makes calls of its own: **proxy**.\n- The app pins certificates: **interceptor**, or turn pinning off in the debug build.\n\nBoth read and write the same mock folders, so you can capture one way and replay the other.\nThe [table](#proxy) below compares them in detail.\n\n## In your app\n\nrayo can't reach inside an app: the app has to send its calls to `http://127.0.0.1:8080` on the device.\nWith OkHttp, an interceptor does it. It can be **only in the debug build** (`src/debug`), and only on\nwhen a test says so, so nothing of it reaches your users.\n\n```kotlin\n/** https://host/path becomes http://127.0.0.1:8080/host/path, when a test asked for it. */\nobject MockRedirect : Interceptor {\n  @Volatile var on = false\n\n  override fun intercept(chain: Interceptor.Chain): Response {\n    val request = chain.request()\n    if (!on) return chain.proceed(request)\n    val url = request.url.newBuilder()\n      .scheme(\"http\").host(\"127.0.0.1\").port(8080)\n      .encodedPath(\"/${request.url.host}${request.url.encodedPath}\")\n      .build()\n    return chain.proceed(request.newBuilder().url(url).build())\n  }\n}\n```\n\nAdd it to the client with `addInterceptor(MockRedirect)`, and turn it on before the first call, from\nwhat `launchApp: { arguments: { mock: true } }` gives:\n\n```kotlin\nMockRedirect.on = intent.getBooleanExtra(\"mock\", false)\n```\n\nAndroid refuses calls that aren't HTTPS, so allow that one, also in `src/debug`, with\n`android:networkSecurityConfig=\"@xml/network_security_config\"`:\n\n```xml\n\u003cnetwork-security-config\u003e\n  \u003cdomain-config cleartextTrafficPermitted=\"true\"\u003e\n    \u003cdomain includeSubdomains=\"false\"\u003e127.0.0.1\u003c/domain\u003e\n  \u003c/domain-config\u003e\n\u003c/network-security-config\u003e\n```\n\nThe host goes in the path so that several can be mocked, and so that a capture knows whom to call:\na mapping for `https://api.example.com/products` has the path `/api.example.com/products`. An app\nwith one backend may leave it out, as the examples here do; to capture it, use `--capture-from`.\n\n## Proxy\n\nThe interceptor only sees the calls made through the app's own client. A third-party SDK that brings its\nown client, such as a payment SDK that polls a URL from an answer, still calls the real server: a capture\nmisses those calls without saying so, and a mock run sends them live. With `--network proxy`, or\n`network: proxy` in a flow's header, the device's HTTP proxy sends every call to rayo instead.\n\n```\nrayo ca new                                           # once per project: .rayo/ca, committed\nrayo ca export app/src/debug/res/raw/rayo_ca.pem      # once, for the app to trust it\nrayo test --capture mock/ --network proxy flow.yaml   # capture\nrayo test --network proxy flow.yaml                   # mock\n```\n\n| | `interceptor` (default) | `proxy` |\n|---|---|---|\n| What it sees | Calls made through the app's own client | Every HTTP(S) call on the device: the app, SDKs; WebViews of `--hosts` |\n| In the app | The interceptor, and plain HTTP allowed to `127.0.0.1` | No code: the debug build trusts rayo's certificate (CA) |\n| On the device | `adb reverse`, so `127.0.0.1:8080` on the phone reaches rayo | The same, and the phone's proxy set while the flow runs, then put back |\n| Misses | Calls of an SDK with its own client | Hosts the app pins certificates for, or doesn't trust rayo's CA for: the flow fails at once |\n\nBoth write and read the same folders, with the host in the path, so a capture made one way replays the\nother way. An app that still rewrites its URLs works behind the proxy too.\n\n- **The CA** (`ca.pem` and `ca-key.pem`) is, in this order: the folder of `--ca`, or of `ca:` in the\n  suite; else `.rayo/ca` next to the suite or the flow, or in a folder above it up to the repository's\n  root; else that of the computer, in `~/.rayo`, made the first time. `rayo ca new [\u003cfolder\u003e]` makes one\n  for the project, in `.rayo/ca` if not said: committed with the flows and the mocks, a fresh checkout\n  or CI signs as the app expects, with no setup. Its key signs only for test builds that trust it.\n  The run says which CA the proxy signed with. `rayo ca export` writes the certificate and the `network_security_config` that trusts it in\n  `debug-overrides` only. A release build ignores it.\n- **Hosts.** The proxy decrypts only some hosts and tunnels the rest untouched: those of `--hosts`, and\n  their subdomains; else, capturing, all but Google's, Firebase's, analytics and crash reports; else\n  those the mocks have in their paths. Then a call to a host of the mocks that none fits fails as\n  `not mocked`, and a call to another host goes to the real one. The hosts left alone are listed after the flow.\n- **Browsers.** A Custom Tab or the browser, such as for a login page, trusts only the system's CAs,\n  and would show \"Your connection is not private\". The proxy reads the start of each TLS connection\n  first: Chrome and what is made from it (WebViews, Cronet) send GREASE values, Android's own TLS\n  doesn't. Such a connection is passed untouched, and its hosts listed after the flow, unless the host\n  is in `--hosts`: name it there to capture or mock a WebView's calls.\n- **Pinning.** A host the proxy opens whose TLS the app refuses fails the flow at the step it happens,\n  as `pinned or untrusted CA: \u003chost\u003e rejected rayo's CA`: its calls would reach the real backend,\n  neither captured nor mocked, so they are refused, never sent on. A mocked run never reaches the real\n  backend of a host it mocks. The app needs rayo's CA in its debug `network_security_config`, from `rayo ca\n  export`; a build that pins certificates needs the pins off in debug. A host meant to reach the real\n  one is left out of `--hosts`, and passes untouched.\n- **Plain `http://`** passes untouched, and is listed.\n- **Clean-up.** The proxy is set by a shell on the device that puts the one before back when the flow\n  ends, and also when the connection closes, however rayo stopped: a device isn't left with a proxy\n  with nothing behind it.\n\n## What happens on a call\n\n1. The app calls `http://127.0.0.1:8080/...` on the device, which reaches rayo.\n2. rayo looks for a mapping that fits: method, address, query, headers, body and state.\n\n   ```json\n   {\n     \"request\": { \"method\": \"GET\", \"urlPath\": \"/products\" },\n     \"response\": { \"status\": 200, \"headers\": { \"Content-Type\": \"application/json\" }, \"bodyFileName\": \"products/all.json\" }\n   }\n   ```\n\n   If several fit, the lowest `priority` wins (5 by default), and then the last one loaded. So\n   `overrides.json` wins over `shared/mock`, as it comes after.\n3. It answers with `__files/products/all.json`, with what is between `{{ }}` filled in, such as\n   `{{now offset='-8 minutes'}}`.\n4. If none fits, the app gets a 404 and **the flow fails**.\n\n## State\n\nThe same call can be answered differently as things advance. A mapping may belong to a scenario,\nwhich begins in `Started`:\n\n```json\n{ \"mappings\": [\n  { \"scenarioName\": \"order\", \"requiredScenarioState\": \"Started\", \"newScenarioState\": \"paid\",\n    \"request\": { \"method\": \"POST\", \"urlPath\": \"/payments\" },\n    \"response\": { \"status\": 200 } },\n  { \"scenarioName\": \"order\", \"requiredScenarioState\": \"shipped\",\n    \"request\": { \"method\": \"GET\", \"urlPath\": \"/order\" },\n    \"response\": { \"status\": 200, \"bodyFileName\": \"order/shipped.json\" } },\n  { \"priority\": 10,\n    \"request\": { \"method\": \"GET\", \"urlPath\": \"/order\" },\n    \"response\": { \"status\": 200, \"jsonBody\": { \"state\": \"waiting\" } } }\n] }\n```\n\nHere paying moves `order` to `paid`. What the backend does by itself, the flow says:\n`- mockState: { order: shipped }`. The last mapping is for any other state.\n\n## Why two folders\n\n- **Reuse**: several mappings can answer with the same body.\n- **Readable**: the logic isn't buried in long bodies.\n- **Easy to replace**: a body is a real answer, to paste without touching the rules.\n\nIt isn't required: an answer can be in its mapping, with `body` or `jsonBody`. A few lines go there;\nwhat is long or shared goes in `__files/`.\n\n## From the real backend\n\n`rayo test --capture` writes such a folder from a real backend, through the interceptor or the proxy.\nREST and GraphQL calls go in the same folder. A REST call is matched by its method, address, query and\nbody, as WireMock's recorder does, so calls to one endpoint with other bodies get their own answers. A\nGraphQL call is matched by its operation and variables (and, if one\nname is sent with several query documents, by its shape), so a `Wallet` query is never answered with a\n`Geocode` answer, whatever order the app makes them in. See [GraphQL](REFERENCE.md#graphql).\n\nWhen the backend changes, `rayo test --capture-update mock` captures again beside the mocks and says,\nmapping by mapping, what changed; `rayo mocks apply mock` takes all of it, or `--only` some. With\n`--check` it fails instead, for CI. See [Updating mocks](REFERENCE.md#updating-mocks).\n\n## Keeping mocks up to date\n\nA capture is made once. When the app or the backend changes after it, don't capture again from\nnothing: these are the ways to bring the mocks up to date, keeping what still holds and the mappings\nwritten by hand.\n\n**Knowing that they are out of date.** A flow run on its mocks fails when the app calls what no\nmapping answers, and says which calls, and the nearest mapping:\n\n```\nthe app called what isn't mocked:\n    GET /api/users/me/badges\n    GET /api/users/me?lang=en\n      nearest GET /api/users/me: query lang: \"en\", expected \"de\"\n  mocks loaded: mock (12)\n```\n\nA folder without mappings is an error when it is loaded (`no mappings in \u003cpath\u003e`). A flow that fails\nwhen no call reached its mocks at all says so: the app's interceptor may be off in that build, else try\n`network: proxy`. A request body over 10 MB is answered 413 and fails the flow as unmocked, never\ncompared cut short. When `waitForRequest` or `assertRequest` doesn't find its call, the error lists the\nlast 5 calls and why each didn't fit (method, path, body, or taken by an earlier `waitForRequest`).\n\nA mapping that is no longer called doesn't fail anything: an update shows it as `gone`.\n\n**The app makes a call more, or one less, or the backend answers otherwise.** Update from the real\nbackend, only the flows that changed, then take what you want:\n\n```sh\nrayo test --capture-update mock --only checkout flows/   # captures into mock/.update, and says what changed\nrayo mocks apply mock                                    # takes the changed and the new\nrayo mocks apply --only \"GET /api/users/me/badges\" mock  # or only these\nrayo mocks apply --prune mock                            # also takes out those gone\n```\n\nThe update makes its mocks as the capture made them (with the settings kept in\n`capture.calls.json`), so a mapping that didn't change is the same, and isn't touched. Look at what\nis printed, or at `mock/.update/changes.txt`, before applying.\n\n**A new flow, on mocks that are there.** Go on with the capture, as if it had been made in one go:\n\n```sh\nrayo test --capture mock --capture-append --only newFlow flows/\n```\n\n**In CI.** Find out, without changing anything, whether the backend still answers as the mocks say:\n\n```sh\nrayo test --capture-update mock --check flows/\n```\n\nEach of these calls the real backend, for the flows it runs: it has to be reachable, with the\naccounts and data the flows need. `--only` keeps that to the flows that changed. Commit the mocks\nafter `rayo mocks apply`, and leave `mock/.update/` out (add it to `.gitignore`).\n\n**Tidy.** `rayo mocks check mock` says which mappings never answer, as a later one matches the very\nsame calls, and which bodies in `__files` no mapping uses: often leftovers of edits by hand.\n\nAll that a mapping can say, and\nthe commands `mock`, `mockState`, `network`, `waitForRequest` and `assertRequest`, are in the\n[reference](REFERENCE.md#mock).\n\n## Coming from WireMock or mitmproxy\n\nA mocks folder of WireMock's, `mappings/` and `__files/`, is one of rayo's: give it to a flow's `mock`\nas it is. rayo's captures load in WireMock too: they use only its matchers. What rayo can't do, such as\nwebhooks or proxying to a real backend, is an error when the mocks load, not a mock that does less.\nAn app that calls one backend, with `--capture-from`, has paths without the host, as WireMock's; through\nthe proxy, a mapping without the host answers too.\n\n| WireMock recording | rayo |\n|---|---|\n| `POST /__admin/recordings/start`, then `snapshot` | `rayo test --capture mock flow.yaml` |\n| `requestBodyPattern: auto` (`equalToJson` with `ignoreArrayOrder` and `ignoreExtraElements`, `equalToXml`, `equalTo`) | The default; `--rest off` for none |\n| `repeatsAsScenarios: true` | Always: the same call answered otherwise gets states, of the scenario `capture` |\n| `captureHeaders` | `--capture-headers Accept-Language,X-Api-Version` |\n| `extractBodyCriteria` | Always: every body goes in `__files/` |\n| `persist`, `--root-dir` | The folder of `--capture` |\n| Near misses | Said when a call isn't mocked: the nearest mapping, and what of it didn't fit |\n| `--global-response-templating` | Mappings without `transformers`; with `\"transformers\": [\"response-template\"]`, as WireMock's local mode |\n\n| mitmproxy server replay | rayo |\n|---|---|\n| `mitmdump -w flows` | `rayo test --capture mock --network proxy flow.yaml`, or `rayo mocks import flows.har mock` with `mitmdump -nr flows --set hardump=flows.har` |\n| `mitmdump --server-replay flows` | `rayo test flow.yaml`, with `mock: mock` in its header |\n| `server_replay_ignore_params` | `--ignore-params a,b` |\n| `server_replay_ignore_payload_params` | `--ignore-body a,b`, also for JSON and GraphQL variables |\n| `server_replay_ignore_content` | `--rest off` |\n| `server_replay_ignore_host` | `--ignore-host` |\n| Answers given once each, in order (`server_replay_reuse` off) | States: a call made three times is answered as the three times were; after that, as the last |\n| `server_replay_kill_extra` | Always: a call that isn't mocked fails the flow |\n| `server_replay_refresh` | `{{now offset='...'}}` in a body |\n\nDates and times are matched by their form, not their value, so a call made with the time of the run\nstill fits; `--timestamps exact` to match them as they were. All these settings are kept with the\ncapture, and used again by `--capture-append` and `--capture-update`.\n",
  "REFERENCE.md": "# rayo reference\n\nEvery command and feature. To begin, see the [README](README.md).\n\n🧪 not tested on a device yet. `rayo help \u003ccommand\u003e` shows what one takes.\n\nThe tables between `generated` marks come from the code, as do `rayo help` and the JSON Schemas\n(`rayo schema flow`, `suite` or `results`; also in [schema/](schema)). `go test ./test -update` writes them.\n\n## Index\n\n| Topic | |\n|---|---|\n| [Options](#options) | As `rayo help` prints them |\n| [Flow file](#flow-file) | [Header](#header) · [Common options](#common-options) · [Values](#values) |\n| [Selectors](#selectors) | [Conditions](#conditions) · [Relations](#relations) |\n| [Apps](#apps) | [launchApp](#launchapp) · [stopApp](#stopapp-killapp) · [killApp](#stopapp-killapp) · [clearState](#clearstate) · [setPermissions](#setpermissions) · [openLink](#openlink) |\n| [Touch](#touch) | [tapOn](#tapon) · [doubleTapOn](#doubletapon) · [longPressOn](#longpresson) · [swipe](#swipe) · [scroll](#scroll) · [scrollUntilVisible](#scrolluntilvisible) |\n| [Text and keys](#text-and-keys) | [type](#type) · [inputText](#inputtext) · [inputRandom…](#inputrandom) · [eraseText](#erasetext) · [copyTextFrom](#copytextfrom-pastetext) · [pasteText](#copytextfrom-pastetext) · [hideKeyboard](#hidekeyboard) · [pressKey](#presskey) · [back](#back) |\n| [Checks and waits](#checks-and-waits) | [assertVisible](#assertvisible) · [assertNotVisible](#assertnotvisible) · [extendedWaitUntil](#extendedwaituntil) · [assertTrue](#asserttrue) · [waitForAnimationToEnd](#waitforanimationtoend) |\n| [Control](#control) | [runFlow](#runflow) · [repeat](#repeat) · [retry](#retry) · [when](#when) |\n| [Scripts](#scripts) | [Expressions](#expressions) · [runScript](#runscript) · [evalScript](#evalscript) · [JavaScript API](#javascript-api) |\n| [Device](#device) | [setLocation](#setlocation) · [travel](#travel) · [setAirplaneMode](#setairplanemode) · [setOrientation](#setorientation) · [takeScreenshot](#takescreenshot) · [startRecording](#startrecording-stoprecording) · [stopRecording](#startrecording-stoprecording) · [camera](#camera) · [shell](#shell) |\n| [Server](#server) | [http](#http) |\n| [Mock](#mock) | [Files](#mock-files) · [mock](#mock-step) · [mockState](#mockstate) · [network](#network) · [waitForRequest](#waitforrequest) · [assertRequest](#assertrequest) · [Capture](#capture) |\n| [Running several](#running-several) | [Suites](#suites) · [Clean state](#clean-state) · [Retries](#retries) · [Blame](#blame) · [Outcomes](#outcomes) · [Results](#results) · [For a program](#for-a-program) · [Exit codes](#exit-codes) · [Errors](#errors) · [Events](#events) |\n| [Live sessions](#live-sessions) | [Commands](#live-commands) · [From other programs](#from-other-programs) |\n| [Features](#features) | [Schemas](#schemas) · [Variables](#variables) · [Tags](#tags) · [Continuous mode](#continuous-mode) · [Resume](#resume) · [Failures](#failures) · [Timings](#timings) · [Profile](#profile) · [Report](#report) · [Environment](#environment) |\n| [Default times](#default-times) · [Limitations](#limitations) | |\n\n---\n\n## Options\n\nAs `rayo help` prints them. An option of another command is an error, never ignored.\n\n```\nrayo - runs flows on Android, fast\n\nNew here, or driving rayo from a program? Start with: rayo help agents\n\n  rayo test [options] \u003cflow, folder or suite\u003e...   run flows; a suite is a *.suite.yaml\n  rayo check \u003cflow, folder or suite\u003e...            check flows without a device, and say which would run\n  rayo explain [\u003crun folder, .zip or suite\u003e] [\u003cflow\u003e]  why the flows of the last run failed, in a few lines\n  rayo live [options] \u003cflow\u003e                       write a flow as it runs, command by command\n  rayo live [options] \u003csuite\u003e \u003cflow\u003e               the same, with the suite's env, output and CA\n  rayo do [options] \u003ccommands\u003e                     send commands to the live session; rayo do .help\n  rayo hierarchy [options]                         what is on screen, and how to refer to it\n  rayo devices                                     connected devices\n  rayo mocks apply [options] \u003cfolder\u003e              take the changes of rayo test --capture-update into the mocks\n  rayo mocks import [options] \u003cfile.har\u003e \u003cfolder\u003e  write the calls of a HAR file, such as mitmproxy's, as mocks\n  rayo mocks check \u003cfolder\u003e                        mappings that never answer, and bodies no mapping uses\n  rayo ca new [\u003cfolder\u003e]                           a CA for the project, in .rayo/ca, to commit with the flows\n  rayo ca export \u003cfile\u003e                            the certificate the proxy signs with, for the app's debug build\n  rayo help \u003ccommand\u003e                              what a command takes, of a flow or of rayo; or \u003ccommand\u003e --help\n  rayo help \u003ctopic\u003e                                flow, suite, json, all (everything on one page), or a topic below\n  rayo help search \u003cword\u003e...                       the lines of all the help that have the words\n  rayo schema \u003cname\u003e                               a JSON Schema: flow, suite, results, or of --format json\n  rayo version\n\nTopics, rayo help \u003ctopic\u003e\n  agents                     start here: the fast, cheap way to use rayo\n  write-a-flow               write a flow\n  debug-a-flow               debug a failing flow\n  resume                     go on from where a flow failed, after a fix, without running it all again\n  hold                       fix failing flows as they fail: hold the run at a failure, fix the files, run again from there, go on\n  record-mocks               record mocks from the real backend\n  connect-mocks              connect the app to the mocks: interceptor or proxy\n  run-in-ci                  run in a CI\n  recipes                    short answers: video, logcat, reset, log in, deeplink, what a run leaves\n  limits                     limitations and gotchas\n  exit-codes                 exit codes\n\nOptions of test, live and hierarchy\n  --device \u003cserial\u003e          when several are connected; else ANDROID_SERIAL, else the only one\n  --emulator                 never a real device, even if it is the only one\n\nOptions of test and check: which flows\n  --changed-only             only the flows of which a file changed: their own, one they run, or a mock; as --since HEAD\n  --since \u003cgit ref\u003e          only the flows of which a file changed since then, also not committed\n\nOptions of hierarchy and do\n  --find \u003ctext\u003e              of the screen, only the elements most like it, instead of all\n\nOptions of test, live, do and explain\n  --output \u003cfolder\u003e          where failures and screenshots go (default .rayo) (suite: output.folder)\n\nOptions of test and live\n  --timeout-scale \u003cfactor\u003e   multiply the default waits, not a step's own timeout, from 0.1 to 10 (default 1) (suite: timeoutScale)\n  --mock-port \u003cport\u003e         where the app finds the mock on the device (default 8080)\n  --animations               leave the device's animations on\n  --profile                  where the time of each step went\n\nOptions of test and check: which flows of them\n  --only \u003ca,b\u003e               only the flows with these names\n  --include-tags \u003ca,b\u003e       only flows with one of these tags\n  --exclude-tags \u003ca,b\u003e       not flows with one of these tags\n  --rerun-failed             only those that haven't passed since they last failed\n  --min-flows \u003cn\u003e            fail (exit 2) if fewer flows than this are chosen, as a typo in a tag or a name would; of all the parts with --shard, and not with --rerun-failed or --resume (default: the suite's minFlows, else none) (suite: minFlows)\n  --shard \u003ci/n\u003e              only part i of n, for n machines that run the same flows: each flow is in one part, by its path, whatever the order; it writes to .rayo/shard-i-of-n unless --output or the suite says\n\nOptions of test, check and live\n  -e, --env \u003cKEY=VALUE\u003e      a variable for the flows; over the suite's env, which is over a flow's own (suite: env)\n\nOptions of test and check: as the suite says, for one run\n  --retries \u003cn\u003e              try again what fails, every flow, also one the suite gives its own retries (default 1, or the suite's) (suite: retries)\n\nOptions of test: how they run\n  --fail-fast                stop at the first failure\n  --flaky-fails-build        exit 4 if every flow passed but some only on a retry (default: exit 0) (suite: flakyFailsBuild)\n  --keep-going               don't stop when the device or the app fails over and over\n  --shuffle[=seed]           in a random order, or in that of an earlier run\n  --screens-before \u003cn\u003e       keep the screen after each of the last n steps that passed, and save them with a failure as screens-before.txt: how the app got there; each costs a read of the screen (default 0) (suite: output.screensBefore)\n  -c, --continuous           run again whenever a file changes\n  --on-fail \u003cwhat\u003e           go-on (default): retry it as allowed, then the next flow; hold: keep the flow where it failed, the app as it is, until rayo do says how to go on, so that it is fixed in its files and run again from there; see rayo help hold\n  --hold-timeout \u003cduration\u003e  with --on-fail hold, leave a flow failed if no command comes for this long, such as 20m (default 1h)\n\nOptions of test: from where, after a fix; see rayo help resume\n  --resume                   each flow that failed last time, from the step that failed, with the app as it is and the mocks' scenarios as they were then\n  --from \u003cstep\u003e              begin one flow at a step: file:line as rayo explain's at: says, a line of its own file, or a text of one step such as pickDate; the app is as it is, onFlowStart doesn't run\n  --until \u003cstep\u003e             stop one flow after a step, given as --from takes it, leaving the app there; onFlowComplete doesn't run\n  --mock-state \u003ca=s,b=t\u003e     with --resume or --from, the scenarios' states to begin with, over those of the last run\n\nOptions of test and check: what is written, as the suite's output\n  --video                    a video of each flow whose header doesn't say record, as output: { video: true } (suite: output.video)\n  --keep \u003cwhen\u003e              the files of every try (all, default), only of the tries that failed, or none (suite: output.keep)\n  --artifacts \u003ca,b\u003e          the kinds of file written: video, screenshot, logcat, screen, requests (default all) (suite: output.artifacts)\n  --name-template \u003ctemplate\u003e how the files are named in the output folder (default {flow}_{attempt}_{kind}.{ext}) (suite: output.nameTemplate)\n\nOptions of test: what comes out\n  --detail \u003chow much\u003e        printed: brief, the verdict of a failure; normal (default), also why, where, the nearest elements and the calls, and every step of a single flow; full, also the screen, every step, and whole files (suite: detail)\n  --failure-window \u003cseconds\u003e a failure's log and calls begin this long before the failing step (default 10) (suite: failureWindow)\n  --results \u003cfile\u003e           write the results: .txt, .json, .xml (JUnit) or .md; may be repeated (suite: results)\n  --results-template \u003cfile\u003e  write the results with that extension as this file says, with {suite}, {passed}, {failed}, {flaky} and the like, and {flows}...{end} (suite: resultsTemplate)\n  --show \u003ca,b\u003e               the flows the results list: failed, flaky, slow or all (default all) (suite: show)\n  --slowest \u003cn\u003e              how many flows the slow list has (default 5) (suite: slowest)\n  --timings                  while it runs: each step's time, each runFlow's total, the slowest steps and flows, and a note while a step hangs\n\nOptions of test and live: mocks from the real backend\n  --capture \u003cfolder\u003e         call the real backend, and write what it answers as mocks; in live, on .quit, without the calls of steps that failed\n  --capture-update \u003cfolder\u003e  capture into folder/.update, and say what changed from the mocks in folder; rayo mocks apply takes it. Given a suite instead, the folder is the flow's mock\n  --check                    with --capture-update, fail if the mocks differ from the backend, and change nothing\n  --capture-append           go on with the capture already in the folder, as if in one go\n  --redact-value \u003cname=value\u003e what a private field of a body becomes, such as phone=+4915100000000 (default one of the same form: a date stays a date); again for more (suite: capture.redactValue)\n\nOptions of test, live and mocks: how a capture or an import becomes mocks\n  --capture-from \u003curl\u003e       the backend, if calls don't begin with /host/ (suite: capture.from)\n  --graphql \u003cmatch\u003e          tell GraphQL calls apart by operation+variables (default), by operation, or off, as any call (suite: capture.graphql)\n  --graphql-path \u003cregex\u003e     paths of GraphQL calls, besides those whose body has query or operationName (default .*/graphql) (suite: capture.graphqlPath)\n  --rest \u003cmatch\u003e             match other calls by their body too (body, default), or by method and address alone (off) (suite: capture.rest)\n  --ignore-params \u003ca,b\u003e      names in the query not to match, such as session or depart_at (suite: capture.ignoreParams)\n  --ignore-body \u003ca,b\u003e        names, or paths such as trip.departAt, in JSON bodies and GraphQL variables, and names in forms, not to match (suite: capture.ignoreBody)\n  --timestamps \u003cmatch\u003e       match dates and times in the query and bodies by their form (format, default), or exactly (suite: capture.timestamps)\n  --capture-headers \u003ca,b\u003e    headers of the request to match too, such as Accept-Language (suite: capture.captureHeaders)\n  --ignore-host              match calls whose path begins with a host, whatever the host (suite: capture.ignoreHost)\n  --ignore-path-ids          match UUIDs and ULIDs in paths by their form, as they are new on each run (suite: capture.ignorePathIds)\n  --stateless \u003ca,b\u003e          paths such as api.shop.com/quotes/** whose calls only read, whatever the method: they don't change the state (* any text but /, ** any) (suite: capture.stateless)\n\nOptions of test, live and ca: how calls reach the mock\n  --network \u003cway\u003e            interceptor, the app sends them, with code in its debug build (default, or the flow's); proxy, the device's proxy does, no code; see rayo help connect-mocks (suite: network)\n  --ignore-unmocked \u003ca,b\u003e    calls without a mock that don't fail the flow, answered 404: graphql:Operation, host/path, host or /path (* any text but /, ** any) (suite: unmocked.ignore)\n  --ca \u003cfolder\u003e              with a proxy, the CA it signs with, its ca.pem and ca-key.pem (default .rayo/ca next to the suite or above it, else ~/.rayo); see rayo ca (suite: ca)\n  --hosts \u003ca,b\u003e              with a proxy, the hosts to open, and their subdomains (default: capturing, all but Google's, Firebase's, analytics and crash reports; else those of the mocks) (suite: capture.hosts)\n\nOptions of live\n  --app \u003cpackage\u003e            of a flow that isn't there yet\n  --mock \u003ca,b\u003e               its mocks, relative to it\n  --no-run                   don't begin by running the steps it has\n  --serve                    only take commands from rayo do, until .quit\n\nOptions of explain\n  --latest                   only the flows of the newest run in failed.json, not those still there from earlier ones\n\nOptions of mocks\n  --only \u003ca,b\u003e               the changes whose name has one of these, such as graphql:Wallet or GET /profile (default all)\n  --prune                    also take out the mocks that weren't called\n\nOptions of do\n  --screen[=changes]         also what is on screen afterwards; --screen=changes, only the lines that appeared (+) or went (-) since the screen the session last gave\n  --ignore-screen \u003cregex\u003e    with --screen, leave out the lines that match, such as a clock's; says how many\n  --timeout \u003cduration\u003e       how long to wait for the answer, such as 30s (default as long as it takes); the session goes on\n  --discard                  with .quit, end even if steps were not saved\n\nOptions of test, check, explain, devices, hierarchy, do and mocks\n  --format \u003chow\u003e             human (default); json, for a program, see rayo schema; ndjson, the same on one line, and for test an event a line as it runs (suite: format)\n```\n\n---\n\n## Flow file\n\nTwo parts separated by `---`: a header and a list of commands.\n\n```yaml\nappId: com.example.app\n---\n- launchApp\n- tapOn: Log in\n```\n\n### Header\n\n\u003c!-- generated: header --\u003e\n| Key | Takes | |\n|---|---|---|\n| `appId` | package | The app's |\n| `name` | text | Shown in the output (default: the file's, or the folder's for `flow.yaml`) |\n| `tags` | a list | To choose flows, see [Tags](#tags) |\n| `env` | values | Variables for it, see [Variables](#variables) |\n| `mock` | a path or a list | Who answers as the backend, see [Mock](#mock); `${...}` too, where an empty path loads nothing |\n| `network` | `interceptor` or `proxy` | How the app's calls reach the mock, see [Proxy](#proxy); `${...}` too, such as `${NETWORK}` from `-e`, where empty is the default (default: `interceptor`, or `--network`) |\n| `record` | `true`, `false` or a file | A video of the flow, see [Recording a whole flow](#recording-a-whole-flow) (default: the suite's) |\n| `onFlowStart` | commands | Run first |\n| `onFlowComplete` | commands | Run last, also after a failure |\n\u003c!-- end --\u003e\n\n### Common options\n\nAccepted by every command that takes a mapping.\n\n\u003c!-- generated: options --\u003e\n| Key | Takes | |\n|---|---|---|\n| `label` | text | What to show instead of the command |\n| `optional` | true/false | A failure doesn't fail the flow |\n| `timeout` | time | How long it may take |\n\u003c!-- end --\u003e\n\n### Values\n\n| Value | Written as |\n|---|---|\n| Time | `500` (milliseconds) or `10s` |\n| Point | `35%, 30%` of the screen, or `540, 1200` pixels |\n| Location | `41.3785, 2.1765` (latitude, longitude) |\n| Short form | `- tapOn: Log in` is `- tapOn: { text: Log in }` |\n\n---\n\n## Selectors\n\nHow a command refers to an element. All the conditions given must hold. Run `rayo hierarchy` to\nsee what is on screen.\n\n### Conditions\n\n\u003c!-- generated: selector --\u003e\n| Key | Takes | |\n|---|---|---|\n| `id` | text | The resource id, whole or after :id/; in Compose, a testTag with testTagsAsResourceId. A regular expression of the whole |\n| `text` | text | The text, description or hint. A regular expression of the whole |\n| `index` | number | The n-th match, from 0, top to bottom and left to right; negative counts from the end |\n| `enabled` | true/false | Whether it is enabled |\n| `checked` | true/false | Whether it is checked |\n| `focused` | true/false | Whether it has the focus |\n| `selected` | true/false | Whether it is selected |\n| `below` | selector | Entirely below another |\n| `above` | selector | Entirely above another |\n| `leftOf` | selector | Entirely to the left of another |\n| `rightOf` | selector | Entirely to the right of another |\n| `containsChild` | selector | The direct parent of another |\n| `childOf` | selector | Anywhere inside another |\n| `containsDescendants` | selectors | Around all of these |\n\u003c!-- end --\u003e\n\n`id` and `text` are regular expressions that must match the **whole** value, ignoring case. An\nexact value always matches too, so `Price (€)` needs no escaping.\n\nWith several matches and no `index`, the first in drawing order of the topmost window is used. If one match contains\nanother, the inner one wins.\n\n### Relations\n\n🧪 Each takes another selector.\n\n```yaml\n- tapOn: { text: Delete, below: { id: row_title, text: Invoice 7 } }\n```\n\n---\n\n## Apps\n\n### launchApp\n\nOpens the app and waits until Android has it in front.\n\n\u003c!-- generated: keys launchApp --\u003e\n| Key | Takes | |\n|---|---|---|\n| `appId` | text | The app; by default the header's. Short form |\n| `clearState` | true/false | Erase its data first (false) |\n| `stopApp` | true/false | Close it first (true); false brings it back as it was |\n| `permissions` | permissions | Name: allow, deny or unset. With clearState, all are allowed if not said |\n| `arguments` | values | Intent extras; booleans, integers and decimals keep their type |\n\u003c!-- end --\u003e\n\nPermission names: `all`, `location`, `camera`, `notifications`, `contacts`, `microphone`,\n`bluetooth`, `storage`, `phone`, `calendar`, `sms`, `medialibrary`, or Android's own, such as\n`READ_CONTACTS`. Only `all` has been tested. On an Android older than the permission, `notifications` and\n`medialibrary` before 13, `bluetooth` before 12, it is skipped with a note such as `notifications: not\nasked before Android 13, ignored`.\n\n```yaml\n- launchApp: { clearState: true, permissions: { all: allow }, arguments: { mock: true } }\n```\n\n### stopApp, killApp\n\n🧪 Both close the app. Short form: the package.\n\n\u003c!-- generated: keys stopApp --\u003e\n| Key | Takes | |\n|---|---|---|\n| `appId` | text | The app; by default the header's. Short form |\n\u003c!-- end --\u003e\n\n### clearState\n\n🧪 Erases the app's data. Short form: the package.\n\n\u003c!-- generated: keys clearState --\u003e\n| Key | Takes | |\n|---|---|---|\n| `appId` | text | The app; by default the header's. Short form |\n\u003c!-- end --\u003e\n\n### setPermissions\n\nChanges permissions without opening the app, in the order given. Takes `permissions`, as in\n[launchApp](#launchapp), and `appId`. Fails if the app has no such permission. Android closes a\nrunning app when it loses one.\n\n\u003c!-- generated: keys setPermissions --\u003e\n| Key | Takes | |\n|---|---|---|\n| `appId` | text | The app; by default the header's |\n| `permissions` | permissions | Name: allow, deny or unset. With clearState, all are allowed if not said |\n\u003c!-- end --\u003e\n\n```yaml\n- setPermissions: { permissions: { all: deny, camera: allow } }\n```\n\n### openLink\n\n🧪 Opens a link in whichever app handles it.\n\n\u003c!-- generated: keys openLink --\u003e\n| Key | Takes | |\n|---|---|---|\n| `link` | text | What to open. Short form |\n\u003c!-- end --\u003e\n\n```yaml\n- openLink: https://example.com/item/7\n```\n\n---\n\n## Touch\n\n### tapOn\n\nWaits until the element is on screen, enabled and not moving, then taps its centre.\n\n\u003c!-- generated: keys tapOn --\u003e\n| Key | Takes | |\n|---|---|---|\n| `point` | point | Alone, of the screen; with a selector, of the element in % |\n| `repeat` | number | Taps (1) |\n| `delay` | time | Between taps (100) |\n\u003c!-- end --\u003e\n\n```yaml\n- tapOn: { id: save }\n- tapOn: { point: \"35%, 30%\" }\n```\n\n### doubleTapOn\n\n🧪 Like `tapOn`, twice. `delay` is 100 ms by default.\n\n\u003c!-- generated: keys doubleTapOn --\u003e\n| Key | Takes | |\n|---|---|---|\n| `point` | point | Alone, of the screen; with a selector, of the element in % |\n| `delay` | time | Between taps (100) |\n\u003c!-- end --\u003e\n\n### longPressOn\n\n🧪 Like `tapOn`, held for a second.\n\n\u003c!-- generated: keys longPressOn --\u003e\n| Key | Takes | |\n|---|---|---|\n| `point` | point | Alone, of the screen; with a selector, of the element in % |\n\u003c!-- end --\u003e\n\n### swipe\n\n🧪\n\n\u003c!-- generated: keys swipe --\u003e\n| Key | Takes | |\n|---|---|---|\n| `direction` | UP/DOWN/LEFT/RIGHT | Where the finger or the search goes (DOWN when searching) |\n| `start` | point | Where the finger goes down, instead of direction |\n| `end` | point | Where it goes up |\n| `duration` | time | Of the swipe (250) |\n| `from` | selector | Start on this element; with direction |\n| `fling` | true/false | Lift the finger still moving, so the list goes on by itself (true); false stops it where the finger does |\n\u003c!-- end --\u003e\n\n### scroll\n\n🧪 Scrolls down once.\n\n### scrollUntilVisible\n\n🧪 Scrolls until the element is on screen. If the screen stops changing, the end of the list, it fails\nthen, saying so, instead of waiting out the timeout.\n\n\u003c!-- generated: keys scrollUntilVisible --\u003e\n| Key | Takes | |\n|---|---|---|\n| `element` | selector | What to look for |\n| `direction` | UP/DOWN/LEFT/RIGHT | Where the finger or the search goes (DOWN when searching) |\n| `centerElement` | true/false | Go on until it is in the middle of the list, or the list ends (false) |\n| `speed` | number | 0 to 100, as Maestro: each scroll takes (100 - speed) * 10 ms (40) |\n\u003c!-- end --\u003e\n\n---\n\n## Text and keys\n\n### type\n\nReplaces the text of a field at once, without focus or keyboard. The quickest way to fill a\nform; `pressKey: Enter` won't follow it, as the field isn't focused.\n\n\u003c!-- generated: keys type --\u003e\n| Key | Takes | |\n|---|---|---|\n| `value` | text | The new text |\n\u003c!-- end --\u003e\n\n```yaml\n- type: { id: email, value: me@example.com }\n```\n\n### inputText\n\nAdds text to the focused field, waiting for one to be focused.\n\n\u003c!-- generated: keys inputText --\u003e\n| Key | Takes | |\n|---|---|---|\n| `text` | text | The text to add. Short form |\n\u003c!-- end --\u003e\n\n```yaml\n- tapOn: { id: email }\n- inputText: me@example.com\n```\n\n### inputRandom…\n\n🧪 Like `inputText`, with something random.\n\n| Command | |\n|---|---|\n| `inputRandomEmail` | |\n| `inputRandomPersonName` | |\n| `inputRandomNumber` | `length`, 8 by default |\n| `inputRandomText` | `length`, 8 by default |\n\n### eraseText\n\n🧪 Deletes characters from the end of the focused field: 50, or the number given.\n\n\u003c!-- generated: keys eraseText --\u003e\n| Key | Takes | |\n|---|---|---|\n| `charactersToErase` | number | How many (50). Short form |\n\u003c!-- end --\u003e\n\n### copyTextFrom, pasteText\n\n🧪 `copyTextFrom` keeps an element's text, or its description if it has none, in\n`rayo.copiedText`. `pasteText` adds it to the focused field.\n\n`output` names where else to keep it, so that the next copy doesn't replace it: `output: orderId`\nkeeps it in `${output.orderId}` too, for the rest of the flow and the flows it runs.\n\n```yaml\n- copyTextFrom: { id: order_number, output: orderId }\n- copyTextFrom: { id: total, output: total }\n- inputText: ${output.orderId}\n- assertTrue: ${output.total == '5 €'}\n```\n\n\u003c!-- generated: keys copyTextFrom --\u003e\n| Key | Takes | |\n|---|---|---|\n| `output` | name | Also keeps it in output.\u003cthis name\u003e, which the next copy doesn't replace |\n\u003c!-- end --\u003e\n\n### hideKeyboard\n\n🧪 Closes the keyboard if it is open with one Back, and waits until it has. Does nothing when none is\nopen. It never sends a second Back, which could close the window under the keyboard, such as a\nCustom Tab: if the keyboard is still open at the `timeout`, it fails. To try again, use `back` or\n`retry`. A keyboard is open when the input method's window is at least an eighth of the screen: with\na hardware keyboard, the small toolbar the input method may show isn't one, so nothing is sent.\n\n### pressKey\n\n`Home`, `Back`, `Enter`, `Tab`, `Backspace`, `Delete`, `Escape`, `Space`, `Menu`, `Search`,\n`Recents`, `Power`, `Lock`, `Volume Up`, `Volume Down`, `Remote Dpad …`, `Remote Media …`, or an\nAndroid key code. Only `Home` and `Enter` have been tested.\n\n\u003c!-- generated: keys pressKey --\u003e\n| Key | Takes | |\n|---|---|---|\n| `key` | key | A name, such as Enter, or an Android key code. Short form |\n\u003c!-- end --\u003e\n\n`Home` also waits until the app has really stopped, so that opening it again counts.\n\n### back\n\n`pressKey: Back`.\n\n---\n\n## Checks and waits\n\nEach returns as soon as it holds; none needs a fixed pause.\n\n### assertVisible\n\n🧪 The element is on screen, or appears in time.\n\n### assertNotVisible\n\n🧪 The element isn't on screen, or goes away in time.\n\n### extendedWaitUntil\n\nThe same two, usually with a longer `timeout`.\n\n\u003c!-- generated: keys extendedWaitUntil --\u003e\n| Key | Takes | |\n|---|---|---|\n| `visible` | selector | On screen |\n| `notVisible` | selector | Not on screen |\n\u003c!-- end --\u003e\n\n```yaml\n- extendedWaitUntil: { visible: { id: home }, timeout: 20s }\n- extendedWaitUntil: { notVisible: Loading.* }\n```\n\n### assertTrue\n\n🧪 An [expression](#expressions) is true.\n\n\u003c!-- generated: keys assertTrue --\u003e\n| Key | Takes | |\n|---|---|---|\n| `condition` | JavaScript | An expression. Short form |\n\u003c!-- end --\u003e\n\n```yaml\n- assertTrue: ${output.total == 3}\n```\n\n### waitForAnimationToEnd\n\n🧪 Waits until nothing on screen has changed for `quiet` (1 s). Goes on when the `timeout` is\nover, without failing.\n\nA quiet screen isn't a finished transition: while the app waits for the network, nothing moves, so\nthis returns, and a dialog that comes after may still be missing. It can't know about what hasn't\nbegun. To wait for something late, wait for it by name, as `extendedWaitUntil`, or a `when` with a\nlonger `timeout`:\n\n```yaml\n- tapOn: Continue\n- runFlow:\n    when: { visible: Allow notifications, timeout: 5s }\n    commands: [ { tapOn: Not now } ]\n- assertVisible: Home\n```\n\n\u003c!-- generated: keys waitForAnimationToEnd --\u003e\n| Key | Takes | |\n|---|---|---|\n| `quiet` | time | How long nothing may change (1s) |\n\u003c!-- end --\u003e\n\n```yaml\n- waitForAnimationToEnd: { quiet: 1s, timeout: 5s }\n```\n\n---\n\n## Control\n\n### runFlow\n\n\u003c!-- generated: keys runFlow --\u003e\n| Key | Takes | |\n|---|---|---|\n| `file` | text | A path relative to this flow. Short form |\n| `env` | values | Variables for it |\n| `when` | conditions | Only if these hold |\n| `commands` | commands | What to run |\n\u003c!-- end --\u003e\n\n```yaml\n- runFlow: ../shared/login.yaml\n```\n\n### repeat\n\n🧪 Runs `commands` again, up to `times`, while [`while`](#when) holds. One of the two is required.\n\n\u003c!-- generated: keys repeat --\u003e\n| Key | Takes | |\n|---|---|---|\n| `times` | number | At most |\n| `while` | conditions | As long as these hold; without times, it fails after 1000 rounds that it still holds |\n| `commands` | commands | What to run |\n\u003c!-- end --\u003e\n\n### retry\n\n🧪 Runs `commands` or a `file`, again if it fails, up to `retries` (1) more times.\n\n\u003c!-- generated: keys retry --\u003e\n| Key | Takes | |\n|---|---|---|\n| `retries` | number | How many more times (1) |\n| `commands` | commands | What to run |\n| `file` | text | A path relative to this flow |\n| `env` | values | Variables for it |\n\u003c!-- end --\u003e\n\n### when\n\n🧪 A condition for `runFlow`, `runScript` and, as `while`, `repeat`. All must hold. `visible` and\n`notVisible` wait up to `timeout` (0.7 s) for it; `timeout: 0` looks once, for a guard that\nusually doesn't hold.\n\n```yaml\n- runFlow:\n    when: { visible: \"Allow notifications\", timeout: 3s }\n    commands: [ { tapOn: \"Not now\" } ]\n```\n\n\u003c!-- generated: when --\u003e\n| Key | Takes | |\n|---|---|---|\n| `visible` | selector | On screen |\n| `notVisible` | selector | Not on screen |\n| `platform` | Android/iOS/Web | Android |\n| `true` | JavaScript | An expression that holds |\n| `timeout` | time | How long visible and notVisible may take (700ms); 0 looks once |\n\u003c!-- end --\u003e\n\n---\n\n## Scripts\n\n### Expressions\n\n`${…}` is JavaScript, anywhere in a value. Something undefined is an error, not the word\n\"undefined\".\n\n```yaml\n- inputText: ${output.user.name}\n```\n\n### runScript\n\nRuns a file. `env` gives it variables. What it declares stays inside: share through `output`.\n\n\u003c!-- generated: keys runScript --\u003e\n| Key | Takes | |\n|---|---|---|\n| `file` | text | A path relative to this flow. Short form |\n| `env` | values | Variables for it |\n| `when` | conditions | Only if these hold |\n\u003c!-- end --\u003e\n\n```yaml\n- runScript: { file: create_user.js, env: { NAME: Ana } }\n```\n\n### evalScript\n\n🧪 Runs a line.\n\n\u003c!-- generated: keys evalScript --\u003e\n| Key | Takes | |\n|---|---|---|\n| `script` | JavaScript | A line of it. Short form |\n\u003c!-- end --\u003e\n\n```yaml\n- evalScript: ${output.count = 1}\n```\n\n### JavaScript API\n\n| | |\n|---|---|\n| `output` | An object shared by the whole flow |\n| `http.get/post/put/delete(url, options)`, `http.request(url, options)` | `options`: `headers`, `body`, `method`. Returns `ok`, `status`, `body` |\n| `json(text)` | Parses |\n| `console.log(…)` | Prints |\n| `rayo.platform`, `rayo.copiedText` | |\n\nOnly `http.put` has been tested. A script may take up to 60 seconds. There is no `require`, `fetch` or `async`.\n\n---\n\n## Device\n\n### setLocation\n\nStays at a place, sending a fix every second like a real GPS, until the flow ends. Needs\nAndroid 12.\n\n\u003c!-- generated: keys setLocation --\u003e\n| Key | Takes | |\n|---|---|---|\n| `latitude` | number | In degrees |\n| `longitude` | number | In degrees |\n\u003c!-- end --\u003e\n\n```yaml\n- setLocation: { latitude: 41.3785, longitude: 2.1765 }\n```\n\n### travel\n\n🧪 Moves along `points` at `speed` metres per second (7 by default), then stays at the last.\n\n\u003c!-- generated: keys travel --\u003e\n| Key | Takes | |\n|---|---|---|\n| `points` | locations | A list of 'latitude,longitude' |\n| `speed` | number | Metres per second (7) |\n\u003c!-- end --\u003e\n\n```yaml\n- travel: { points: [\"41.3785,2.1765\", \"41.3805,2.1748\"], speed: 6 }\n```\n\n### setAirplaneMode\n\n🧪 `enabled` or `disabled`. The end of the flow puts it back as it was.\n\n\u003c!-- generated: keys setAirplaneMode --\u003e\n| Key | Takes | |\n|---|---|---|\n| `value` | enabled/disabled/true/false | enabled or disabled. Short form |\n\u003c!-- end --\u003e\n\n### setOrientation\n\n🧪 `PORTRAIT`, `LANDSCAPE_LEFT`, `UPSIDE_DOWN` or `LANDSCAPE_RIGHT`, as Maestro names them. The screen stops\nturning by itself until the end of the flow, which puts both settings back as they were. The quickest\nway to see whether the app keeps what was typed and where it was when the activity is made again.\n\n\u003c!-- generated: keys setOrientation --\u003e\n| Key | Takes | |\n|---|---|---|\n| `orientation` | PORTRAIT/LANDSCAPE_LEFT/UPSIDE_DOWN/LANDSCAPE_RIGHT | How the screen is turned. Short form |\n\u003c!-- end --\u003e\n\n### takeScreenshot\n\n🧪 Saves `\u003coutput\u003e/\u003cflow\u003e_\u003cname\u003e.png`, named as [the other files](#what-is-kept).\n\n\u003c!-- generated: keys takeScreenshot --\u003e\n| Key | Takes | |\n|---|---|---|\n| `path` | text | The file, in the debug output. Short form |\n\u003c!-- end --\u003e\n\n### startRecording, stopRecording\n\nRecords the screen as an mp4. The recording ends with `stopRecording` or with the flow, also if it\nfails. Without a path it has the flow's name.\n\n\u003c!-- generated: keys startRecording --\u003e\n| Key | Takes | |\n|---|---|---|\n| `path` | text | The file, in the debug output; by default the flow's name. Short form |\n\u003c!-- end --\u003e\n\n```yaml\n- startRecording                      # \u003coutput\u003e/\u003cflow\u003e.mp4\n- startRecording: login               # \u003coutput\u003e/\u003cflow\u003e_login.mp4\n- startRecording: { path: login }     # the same\n- stopRecording\n```\n\n#### Recording a whole flow\n\n`record` in its header; or `video` in a [suite's `output`](#what-is-kept), for those of its flows that don't say:\n\n| Where | | Saved as |\n|---|---|---|\n| Flow header | `record: true` | `\u003coutput\u003e/\u003cflow\u003e.mp4` |\n| | `record: name`, or an absolute path | `\u003coutput\u003e/\u003cflow\u003e_name.mp4`, or that path |\n| | `record: false` | Not recorded, whatever the suite says |\n| Suite | `output: { video: true }`, or `--video` | `\u003coutput\u003e/\u003cflow\u003e.mp4` |\n\nEach try of a flow has its own, such as `\u003cflow\u003e_retry1.mp4`. All are kept, unless\n[`keep`](#what-is-kept) says otherwise: `keep: failed` keeps only those of the tries that failed. One recording at a time: with one in\nthe header, another needs `stopRecording` first. At most 30 minutes, or 3 before Android 11 or so;\nthe rest isn't recorded. Without sound, and small on purpose: 480 pixels on the short side, 0.6 Mbps,\nwhich is under 5 MB a minute. The device encodes it as it records.\n\n### camera\n\nHangs a picture on the wall of the emulator's virtual room and walks up to it, such as a QR to\nscan. The walk starts 3 seconds later, while the flow goes on, so use it early. `none` puts the\nroom back, and so does the end of the flow.\n\n\u003c!-- generated: keys camera --\u003e\n| Key | Takes | |\n|---|---|---|\n| `image` | text | What it sees, a path relative to this flow; or none. Short form |\n\u003c!-- end --\u003e\n\n```yaml\n- camera: device/qr.png\n```\n\nThe emulator's back camera must be set to *VirtualScene*. On a real device, or if the emulator\ndoesn't answer, the step says so and passes.\n\n### shell\n\nRuns a command on the device as the shell user. Fails if its exit code isn't 0.\n\n\u003c!-- generated: keys shell --\u003e\n| Key | Takes | |\n|---|---|---|\n| `command` | text | Run as the shell user. Short form |\n\u003c!-- end --\u003e\n\n```yaml\n- shell: am broadcast -a com.example.REFRESH -p com.example.app\n```\n\n---\n\n## Server\n\n### http\n\n🧪 Calls a server from the computer, not the device.\n\n\u003c!-- generated: keys http --\u003e\n| Key | Takes | |\n|---|---|---|\n| `method` | text | Such as POST (GET) |\n| `url` | text | What to call, from the computer. Short form |\n| `json` | values | The body |\n| `expect` | number | The status (any below 300) |\n\u003c!-- end --\u003e\n\n```yaml\n- http: { method: PUT, url: \"http://127.0.0.1:8080/state\", json: { state: docked } }\n```\n\n---\n\n## Mock\n\nrayo answers as the backend while a flow runs. The app must send its calls to\n`http://127.0.0.1:8080` on the device (another port with `--mock-port`); rayo sets up\n`adb reverse` and removes it. HTTP only. Every flow starts afresh, and **fails if the app calls\nsomething without a mock**.\n\n```yaml\nappId: com.example.app\nmock:\n  - ../shared/mock          # the normal answers\n  - overrides.json          # what differs here\n---\n- tapOn: { id: pay }\n- waitForRequest: { method: POST, path: \".*/payments\" }\n- mockState: { order: shipped }\n- network: offline\n```\n\nAn included flow's mocks are added when it runs. `mock` and `network` may use `${...}`, from `-e`\nor a suite's `env`, so that one suite runs both mocked and against the real backend, with no flow\naround each: a path that comes out empty loads nothing, and an empty `network` is the default.\n\n```yaml\nmock: ${MOCKS ? MOCKS + '/' + rayo.file : ''}   # rayo test -e MOCKS=mocks -e NETWORK=proxy all.suite.yaml\nnetwork: ${NETWORK || ''}\n```\n\nHow mocks work, with examples:\n[Mocking](MOCKING.md).\n\n### Mock files\n\nA folder with `mappings/` and `__files/`, or one JSON file of mappings, in WireMock's format: a folder\nWireMock recorded loads as it is, and WireMock loads rayo's captures. Only the following is supported;\nanything else, such as webhooks or `proxyBaseUrl`, is an error rather than a mock that does less.\n\n| Part | |\n|---|---|\n| Mapping | `id`, `uuid`, `name`, `priority`, `scenarioName`, `requiredScenarioState`, `newScenarioState`, `request`, `response`; `persistent`, `insertionIndex` and `metadata` are taken and do nothing |\n| `request` | `method`; one of `url` `urlPattern` `urlPath` `urlPathPattern` `urlPathTemplate` (`/users/{id}`, with `pathParameters`); `queryParameters`, `headers`, `cookies`, `formParameters`, `basicAuthCredentials`, `host`, `bodyPatterns` |\n| Matchers | `equalTo` (and `caseInsensitive`), `contains`, `doesNotContain`, `matches`, `doesNotMatch`, `absent`, `anything`, `equalToJson` (and `ignoreArrayOrder`, `ignoreExtraElements`), `equalToXml`, `binaryEqualTo`, `matchesJsonPath`, `and`, `or`, `not`; for a query, header, cookie or form, also `hasExactly` and `includes` |\n| `equalToJson` | A JSON value, or a text of one. Keys in any order, numbers by value. Placeholders: `${json-unit.ignore}`, `${json-unit.ignore-element}` (may be absent), `${json-unit.any-string}`, `${json-unit.any-number}`, `${json-unit.any-boolean}`, `${json-unit.regex}pattern` |\n| `matchesJsonPath` | An expression, true if it finds something not empty; or `{ \"expression\": \"$.a\", \"equalTo\": \"x\" }` with any matcher for what it finds. Jayway's JSONPath: `$.a`, `$['a']`, `$[0]`, `$[-1]`, `$[0,2]`, `$[1:3]`, `$.*`, `$..a`, `length()`, and filters anywhere, with `==` `!=` `\u003c` `\u003e` `\u0026\u0026` `=~ /re/` `in` `nin` `size` `empty` `subsetof` `anyof` `noneof` |\n| `response` | `status`, `statusMessage`, `headers` (a value, or a list of them), one of `body` `jsonBody` `base64Body` `bodyFileName`, `fixedDelayMilliseconds`, `delayDistribution` (`lognormal`, `uniform`, `fixed`), `chunkedDribbleDelay`, `fault` (`CONNECTION_RESET_BY_PEER`, `EMPTY_RESPONSE`, `MALFORMED_RESPONSE_CHUNK`, `RANDOM_DATA_THEN_CLOSE`), `transformers`, `transformerParameters` |\n| In a body | `{{now}}` (with `offset='-8 minutes'`, `format='yyyy-MM-dd'` or `epoch` or `unix`, `timezone='Europe/Madrid'`), `{{request.method}}` `.path` `.url` `.body` `.query.x` `.query.x.[1]` `.headers.x` `.cookies.x` `.pathSegments.[0]`, `{{jsonPath request.body '$.a'}}`, `{{randomValue length=8 type='ALPHANUMERIC'}}` (or `UUID`, `NUMERIC`...), `{{regexExtract request.path '[a-z]+'}}`, `{{regexExtract request.query.x '(.+)' 'name'}}` then `{{name.0}}`, `{{parameters.x}}` |\n\nThe lowest `priority` wins (5 by default), then the last one added. Scenarios begin in `Started`.\nRegular expressions are Go's and match the whole value, except in `regexExtract`; those WireMock\nwrites mean the same in both, but Java's lookarounds aren't supported.\n\nA body is filled in as `transformers` says. Without it, as rayo always has: every `{{ }}` is filled in,\nand one that isn't known is an error. With `\"transformers\": [\"response-template\"]`, as WireMock does:\n`{{x}}` is escaped for HTML and `{{{x}}}` isn't. With `\"transformers\": []`, it is sent as it is. A body\nthat isn't text, such as an image, is always sent as it is.\n\nA call whose body is compressed (`Content-Encoding: gzip` or `deflate`) is matched by what it holds.\nThrough the proxy, a call is `/host/path`; a mapping without the host, as WireMock's are, answers it\ntoo when no mapping names that host.\n\nWhen no mapping answers, the flow fails and says which came nearest, and why, as WireMock's near\nmisses do: `POST /availability -\u003e not mocked; nearest POST /availability {\"category\":\"ondemand\"}: body:\n$.category: \"prebooked\", expected \"ondemand\"`.\n\n### mock step\n\nAdds a path, or a mapping written here. Its priority is 0 by default, so it wins over the files.\n\n\u003c!-- generated: keys mock --\u003e\n| Key | Takes | |\n|---|---|---|\n| `file` | text | A path relative to this flow. Short form |\n| `id` | text | A name for it |\n| `name` | text | A name for it |\n| `priority` | number | The lowest wins (0) |\n| `scenarioName` | text | The scenario it belongs to |\n| `requiredScenarioState` | text | Only in this state |\n| `newScenarioState` | text | Moves to this state |\n| `request` | values | Which calls |\n| `response` | values | What to answer |\n\u003c!-- end --\u003e\n\n```yaml\n- mock: overrides.json\n- mock: { request: { method: POST, urlPathPattern: \".*/payments\" }, response: { status: 503 } }\n```\n\n### mockState\n\nMoves scenarios to a state. A scenario or state that no mock mentions is an error.\n\n### network\n\n| Form | |\n|---|---|\n| `network: offline` | Connections close without an answer. Android still believes it is connected: see [setAirplaneMode](#setairplanemode) |\n| `network: { delay: 2s }` | Every answer takes longer |\n| `network: { fail: 503 }` | Every answer is this status |\n| `network: online` | Back to normal |\n\n\u003c!-- generated: keys network --\u003e\n| Key | Takes | |\n|---|---|---|\n| `state` | offline/online | online or offline. Short form |\n| `delay` | time | Every answer takes longer |\n| `fail` | 400 to 599 | Every answer is this status, 400 to 599 |\n| `path` | pattern | Only these calls, a regular expression of the whole path |\n\u003c!-- end --\u003e\n\n`path`, a regular expression, limits it to some calls. Each `network` replaces the one before.\n\n### waitForRequest\n\n\u003c!-- generated: keys waitForRequest --\u003e\n| Key | Takes | |\n|---|---|---|\n| `method` | text | Such as POST (GET) |\n| `path` | pattern | Only these calls, a regular expression of the whole path. Short form |\n| `body` | text | A text in the body |\n\u003c!-- end --\u003e\n\nWaits until the app calls `path`, a regular expression of the whole path. Optional: `method`, and\n`body`, a text in it. Calls already made in this flow count, but each is matched only once.\n\n### assertRequest\n\n\u003c!-- generated: keys assertRequest --\u003e\n| Key | Takes | |\n|---|---|---|\n| `method` | text | Such as POST (GET) |\n| `path` | pattern | Only these calls, a regular expression of the whole path |\n| `body` | text | A text in the body |\n| `times` | number | Exactly so many calls by now; 0 for none |\n\u003c!-- end --\u003e\n\nChecks, without waiting, that the app has called `path` exactly `times` times so far in this flow:\n`times: 0` for a call it must never make. `method` and `body` narrow it as in waitForRequest; calls\nthat a waitForRequest matched count too. For a call that may still be on its way, waitForRequest first.\n\n```yaml\n- tapOn: Pay\n- waitForRequest: { method: POST, path: \".*/payments\" }\n- assertRequest: { method: POST, path: \".*/payments\", times: 1 }   # not twice\n- assertRequest: { path: \".*/analytics\", times: 0 }\n```\n\n### Capture\n\nMakes mocks from a real backend. Run the flow once with\n\n```\nrayo test --capture mock -e PASSWORD=... flow.yaml\n```\n\nThe app needs nothing new: it sends its calls to rayo as with a mock, and rayo passes them on and\nkeeps the answers. **The calls are real, and do what they do.** One flow, once, into a new folder.\n\n| The app calls | Passed on to |\n|---|---|\n| `http://127.0.0.1:8080/api.example.com/path` | `https://api.example.com/path` |\n| `http://127.0.0.1:8080/path`, with `--capture-from https://api.example.com` | `https://api.example.com/path` |\n\nWhile capturing, the flow's mocks are left out and `mock`, `mockState` and `network` do nothing, so\na flow that has them can be captured again. `waitForRequest` works. What was captured is also kept\nif the flow fails or is interrupted.\n\nA long flow whose calls do something real (an order, a payment) is better captured in a\n[live session](#live-sessions), which a failure doesn't end:\n\n```\nrayo live --serve --capture mock -e PASSWORD=... flow.yaml \u0026\nrayo do .retry          # after fixing the step that failed, in the file\nrayo do .rewind 2       # forget the last 2 steps and their calls; move the app back by hand\nrayo do .quit           # writes the capture\n```\n\nThe calls of a step that fails, and of those rewound, are left out, so what is written is one\ncapture, as if the flow had run once without them. Without the session, `--capture-append` goes on\nwith a capture that is there already, in another run: its calls come first and the states go on\nfrom them.\n\nIn the folder:\n\n| | |\n|---|---|\n| `mappings/capture.json`, `__files/` | The mocks: give the folder to the flow's `mock` |\n| `capture.txt` | What to do next, as it was printed |\n| `capture.calls.json` | The calls, and what the app sent, hidden as in the mocks; and how they were made into mocks. For `--capture-append` and `--capture-update` |\n\nHow it becomes mocks, as WireMock's recorder does by default:\n\n- A call is matched by its method, its path, its query and its body. The query is matched with\n  `queryParameters`, so the order of its parameters doesn't count. The body as WireMock's\n  `requestBodyPattern: auto`: JSON with `equalToJson`, `ignoreArrayOrder` and `ignoreExtraElements`;\n  a form with `formParameters`; XML with `equalToXml`; other text with `equalTo`; anything else with\n  `binaryEqualTo`. A multipart body, or one over 64 KB, isn't matched. `--rest off` matches by method\n  and address alone. GraphQL calls are told apart [their own way](#graphql).\n- Calls matched the same are the same call. One always answered the same is always answered so.\n- Otherwise there are states, of a scenario called `capture`: `Started`, `1`, `2`... A call that\n  isn't a read (a GET, HEAD, OPTIONS or GraphQL query) moves to the next, if a later answer changed\n  after it. Of several it may be, the one made the fewest times is taken. So the same call is\n  answered as often, and in the order, as during the capture: WireMock's `repeatsAsScenarios`.\n- If an answer changed **by itself**, the flow has to say when: `capture.txt` has the `mockState` to\n  add, and before which step.\n- As `ignoreExtraElements` lets a mapping fit a call with more in it, the mappings that match more\n  get a lower `priority`, from 5: a call is answered by the closest. A mapping written by hand without\n  a priority, loaded after, wins over all of them.\n- What changes from run to run can't be matched as it was. A date and time (ISO 8601) in the query\n  or in a JSON body is matched by its form; `--timestamps exact` matches it as it was. `--ignore-params\n  depart_at,session` leaves parameters of the query out; `--ignore-body nonce,trip.clientTime` leaves\n  names, at any depth, or paths, out of JSON bodies, GraphQL variables and forms; `--ignore-host`\n  matches the path of any host (then, through the proxy, `--hosts` says which to open);\n  `--ignore-path-ids` matches a UUID or ULID in a path, such as `/executions/\u003cuuid\u003e/status`, by its form.\n  `capture.txt` lists what was left out.\n- A call that changes nothing, as GET, keeps the state; any other may change it. `--stateless\n  'api.shop.com/quotes/**,*/availability'` says which paths only read, whatever the method, such as a\n  POST that searches: they answer in any state, as often as the app calls them. `*` is any text but\n  `/`, `**` any text, from the host when calls begin with one.\n- When calls go to several hosts, `capture.txt` says which of them change the state: one that isn't\n  the app's backend, such as analytics, is best left out with `--stateless 'its.host/**'`.\n- A read that answers one way when the app sends `Authorization` and another when it doesn't, such\n  as `GET /users/me` before and after logging in, is matched by whether the header is there, not by\n  a state: `(signed in)` and `(signed out)` in its name.\n- `--capture-headers Accept-Language` matches these headers of the request too.\n- The headers of the answer are kept, but for those that change by themselves (`Date`, `ETag`, trace\n  ids...).\n- These settings are kept in `capture.calls.json`: `--capture-append` and `--capture-update` make the\n  mocks the same way, unless told otherwise.\n\n`rayo mocks import flows.har mock` makes mocks of a HAR file in the same way, as if captured: one of\nmitmproxy (`mitmdump -nr flows --set hardump=flows.har`), Charles, or a browser. With `--capture-from\nhttps://api.example.com`, only the calls to it are taken, without the host in the path; else every\ncall, with its host, but those to Google, Firebase, analytics and crash reports. The values of `--env`\nand the suite's `env` are hidden as in a capture. Calls are taken in the order they started (those\nwithout a time last, then as written), so the same file always makes the same mocks.\n\n#### Updating mocks\n\nWhen the backend changes, capture again beside the mocks instead of over them:\n\n```sh\nrayo test --capture-update mock --capture-from https://api.example.com flow.yaml\n```\n\nThe new capture goes to `mock/.update/`, and each of its mappings is compared with the mapping of\nthe same name (and state) in `mock/`, including hand-written ones. What is printed, and kept in\n`mock/.update/changes.txt`, is each mapping that is `changed` (the paths that differ in what it\nmatches, such as `request.urlPath: \"/a\" -\u003e \"/b\"`; status; each header by name; delay, fault,\n`priority`, `scenarioName` and `newScenarioState`; the paths in the body that differ, such as\n`body.data.wallet.price: 5 -\u003e 7`, or for a body that isn't JSON its first line that does, such as\n`body:12: ... -\u003e ...`), `new` or `gone`. Past 20 differences of a mapping, `... and N more`. Then\ntake what you want:\n\n```sh\nrayo mocks apply mock                                   # all the changes and new ones\nrayo mocks apply --only graphql:Wallet,\"GET /profile\" mock   # those whose name has one of these\nrayo mocks apply --prune mock                           # also take out those gone\n```\n\nIn CI, `--check` fails the run when anything differs, and changes nothing:\n\n```sh\nrayo test --capture-update mock --check --capture-from https://api.example.com flow.yaml\n```\n\nA changed mapping is written where it was, with its body in the same file of `__files`, so hand-written\nmappings in other files stay where they are. It works the same with the interceptor and the proxy.\nMocks captured before REST bodies and GraphQL variables were matched have other names: the first\nupdate of them shows each as `gone` and `new`; `rayo mocks apply --prune` takes the new ones.\n\n#### Checking mocks\n\n`rayo check` already fails on a mock that doesn't load. `rayo mocks check \u003cfolder\u003e` also says what\nloads but is never used, and exits 1 if anything is:\n\n- a mapping that never answers, as a later one of the same priority and scenario state matches the\n  very same calls: the later one wins, so the earlier is dead, often a leftover of an edit;\n- a file of `__files` that no mapping's `bodyFileName` names.\n\n```\n$ rayo mocks check mock\nmock/mappings/profile.json never answers: mock/mappings/profile-new.json, after it, answers the same calls (GET /profile)\nmock/__files/old-profile.json: no mapping uses it\n```\n\n#### GraphQL\n\nGraphQL and REST calls are captured together, through the interceptor or the proxy. A call is taken\nas GraphQL when it is a JSON `POST` whose body has `query`, `operationName` or\n`extensions.persistedQuery.sha256Hash`, or one to a path matching `--graphql-path` (default\n`.*/graphql`). Anything else is captured as before.\n\n- Each GraphQL call is told apart by its **operation**, with a `bodyPatterns` entry\n  `matchesJsonPath: { expression: $.operationName, equalTo: Geocode }`. Mocks therefore don't depend\n  on the order of the calls: a `Wallet` query is never answered with a `Geocode` answer. Without\n  `operationName`, the name in the query is matched; with neither, the hash of a persisted query, or\n  else the query itself. Only WireMock's matchers are written, so WireMock reads the mocks the same.\n- A **query** counts like a GET and a **mutation** like any other call: only mutations move the\n  scenario on, and states whose answers don't differ are joined.\n- Which operation a document runs is read past comments, strings and fragments, also when it has\n  several: the one `operationName` names. That tells a query from a mutation.\n- When the same operation name is sent with **different query documents** (other fields, other\n  fragments), each document gets its own mappings, matched on the query whatever its spaces and\n  commas. When there is only one document per name, only the name is matched, so a small edit to a\n  query doesn't break the mocks.\n- An automatic persisted query, sent first with its hash alone and then again with the query, gets a\n  mapping for each.\n- The **variables** are matched too, with `equalToJson` on `$.variables`, but for those under private\n  names, those of `--ignore-body`, and dates and times, matched by their form. `--graphql operation`\n  matches the operation alone; `--graphql off` takes GraphQL calls as any other.\n- A batch, an array of operations, is matched by its whole body, and is a read if all of them are\n  queries. A query sent with GET is matched by its address.\n- Only `operationName`, the query text and, if asked for, the variables are kept from the request.\n  A query with something private written into it is matched by its name only.\n- When mocking, a GraphQL call that nothing matches is reported with its operation:\n  `POST /graphql (GraphQL operation Unknown)`.\n\nLeft out, as far as it can be recognised: the values of `-e`; emails and JWTs anywhere; and what is\nunder a name such as `access_token`, `password`, `email`, `phone` or `firstName`, also in the address\nand in what the app sends. In an address or a body, anything goes in its place. The value of every\ncookie, and headers such as `Authorization`, are hidden. In a JSON answer, a hidden value keeps its\nform, so that a typed client still reads it: a date or time becomes `2000-01-01T00:00:00Z` in the\nsame layout, a phone `+12025550123`, a first name `Alex`, a last name `Doe`, digits zeros, a number\n`0`; what has no form, such as a token, `REDACTED`. `--redact-value phone=+4915100000000` (again for\nmore) says what a name becomes. An answer that isn't text is kept as it was;\na body the app sent that isn't text isn't kept if a secret is found in it. **Names that aren't in that\nlist are kept, so read the files before committing them.** A failure's screenshot and device log are\nnot touched.\n\n---\n\n## Running several\n\n### Suites\n\nA file whose name ends in `.suite.yaml` lists the flows that run together. It is the place to say\nwhat each is for. Paths are relative to it.\n\n```yaml\nname: Rides\napp:\n  id: com.example.app\n  apk: ../app/build/outputs/apk/debug/app-debug.apk\noutput: .results\nresults: .results/results.txt   # relative to the suite, as every path in it\nflows:\n  # Books a bike and rides it.\n  - tests/book/flow.yaml\n  # The same without booking.\n  - tests/scan/flow.yaml\n```\n\n\u003c!-- generated: suite --\u003e\n| Key | Takes | |\n|---|---|---|\n| `name` | text | Shown in the results (default: the file) |\n| `flows` | a list | Files, run in this order; or mappings with file, retries, mock, network, unmocked and capture, see [below](#a-flow-of-a-suite) (required) |\n| `retries` | a number from 0 | How many times a flow that fails is tried again, see [Retries](#retries) (default: 1) |\n| `app` | a mapping | Installed before the flows, see [below](#the-app-of-a-suite) |\n| `device` | a mapping | How it is while the flows run, see [Device setup](#device-setup) |\n| `env` | values | Variables for every flow, over a flow's own `env`; `-e` overrides one; they may use `${...}`, such as `${rayo.file}`, see [Variables](#variables) |\n| `timeoutScale` | 0.1 to 10 | Multiplies the default waits, see [Default times](#default-times); `--timeout-scale` overrides it (default: 1) |\n| `network` | `interceptor` or `proxy` | How the app's calls reach the mock, as a flow's `network`; the flow's own and `--network` override it, see [Mocks of a suite](#mocks-of-a-suite) (default: each flow's, else `interceptor`) |\n| `unmocked` | a mapping | Calls without a mock that don't fail the flow, see [Calls not worth mocking](#calls-not-worth-mocking) (default: none: each fails the flow) |\n| `capture` | a mapping | How a capture calls the backend and becomes mocks, as the options of the same names, which win, see [Capturing a suite's flow](#capturing-a-suites-flow) |\n| `ca` | folder | `--ca` overrides it: the CA the proxy signs with, its ca.pem and ca-key.pem, relative to the suite; `rayo ca new` makes one, see [Mocks of a suite](#mocks-of-a-suite) (default: `.rayo/ca` next to the suite or above it, up to the repository's root; else `~/.rayo`) |\n| `output` | a folder, or a mapping | What a run writes, and where: a folder, or folder, keep, video, artifacts and nameTemplate, see [What is kept](#what-is-kept) (default: `.rayo` where rayo runs) |\n| `results` | a file or a list | `--results` replaces them all: `.txt`, `.json`, `.xml` (JUnit) or `.md`, see [Results](#results) |\n| `detail` | `brief`, `normal`, `full` | `--detail` overrides it: how much is printed, see [Results](#results) (default: `normal`) |\n| `failureWindow` | seconds, from 1 | `--failure-window` overrides it: a failure's log and calls begin this long before the failing step, see [Failures](#failures) (default: 10) |\n| `resultsTemplate` | file | `--results-template` overrides it: the results with its extension are written as it says, see [A template of your own](#a-template-of-your-own) |\n| `show` | a name or a list | `--show` overrides it: the flows the results list, of `failed`, `flaky`, `slow`, `all`, see [Results](#results) (default: `all`) |\n| `slowest` | a number from 1 | `--slowest` overrides it: how many flows the slow list has (default: 5) |\n| `format` | `human`, `json` or `ndjson` | What is printed; `--format` overrides it, see [For a program](#for-a-program) (default: `human`) |\n| `minFlows` | a number from 1 | `--min-flows` overrides it: fewer flows chosen, by tags, names or what changed, is an error (exit `2`), not a green run of nothing (default: none) |\n| `flakyFailsBuild` | `true` or `false` | `--flaky-fails-build` too: a run where every flow passed, some only on a retry, exits `4`, see [Flaky flows](#flaky-flows) (default: `false`: it exits `0`) |\n\u003c!-- end --\u003e\n\n#### A flow of a suite\n\n\u003c!-- generated: suite flow --\u003e\n| Key | Takes | |\n|---|---|---|\n| `file` | file | The flow (required) |\n| `retries` | a number from 0 | How many times it is tried again if it fails (default: the suite's) |\n| `mock` | a path or a list | Who answers as the backend, as a flow's `mock`, relative to the suite; the flow's own `mock` wins, see [Mocks of a suite](#mocks-of-a-suite) |\n| `network` | `interceptor` or `proxy` | How the app's calls reach the mock, as a flow's `network`; the flow's own and `--network` override it, see [Mocks of a suite](#mocks-of-a-suite) (default: the suite's) |\n| `unmocked` | a mapping | Calls without a mock that don't fail the flow, see [Calls not worth mocking](#calls-not-worth-mocking) (default: the suite's; its calls are added to them) |\n| `capture` | a mapping | How a capture calls the backend and becomes mocks, as the options of the same names, which win, see [Capturing a suite's flow](#capturing-a-suites-flow) (default: the suite's; what it says wins over it) |\n\u003c!-- end --\u003e\n\n#### Mocks of a suite\n\nA suite can say which mocks each flow runs on, and how the app's calls reach them, so that the same\nflow files run unchanged against the real backend and on mocks, with no wrapper flows:\n\n```yaml\nname: Shop (mocked)\nnetwork: proxy                  # for every flow; --network wins\nflows:\n  - file: tests/checkout.yaml\n    mock: mocks/checkout        # a path or a list, relative to the suite\n    retries: 2\n  - file: tests/login.yaml\n    mock: [mocks/common, mocks/login]\n  - file: tests/pinned_host.yaml\n    mock: mocks/pinned_host\n    network: interceptor        # this flow, the other way\n```\n\nWhat wins, first to last: `--network`; the flow's own `mock` and `network`; its entry in the suite;\nthe suite's `network`. A flow whose header has a `mock` runs on that one only, not on its entry's.\nThe paths are checked when the suite is read, as a flow's are.\n\n#### Calls not worth mocking\n\nA call that no mock answers fails the flow. Some aren't worth recording: those the app makes on its\nown at startup, or one cancelled during the capture. `unmocked` lets them through, answered, in the\nsuite for every flow or in a flow's entry, which adds its own; `--ignore-unmocked` adds more:\n\n```yaml\nunmocked:\n  ignore:\n    - host: guest-api.example.com\n      path: /api/v2/locations/favorites/dismiss\n    - graphql: [Geocode, ReverseGeocode]\n    - api.example.com/pings/**  # as --ignore-unmocked takes them\n  status: 200                   # what they are answered with (default 404)\n  body: \"{}\"\n```\n\n```sh\nrayo test --ignore-unmocked 'graphql:Geocode,api.example.com/pings/**' staging.suite.yaml\n```\n\nEvery other call without a mock still fails the flow. Those let through are said at the end of the\nflow, as `ignored 2 unmocked calls: ...`, so that nothing is hidden. They change nothing while\ncapturing, as the real backend answers.\n\n\u003c!-- generated: suite unmocked --\u003e\n| Key | Takes | |\n|---|---|---|\n| `ignore` | a list | Calls that no mock answers but that don't fail the flow: each `graphql:Operation`, `host/path`, `host` or `/path`, as `--ignore-unmocked`, or a mapping with host, path and graphql, see [Calls not worth mocking](#calls-not-worth-mocking) (required) |\n| `status` | 100 to 599 | What those calls are answered with (default: 404) |\n| `body` | text | Of the answer (default: `not mocked: ` and the call) |\n\u003c!-- end --\u003e\n\nAn ignored call, when a mapping:\n\n\u003c!-- generated: suite ignored --\u003e\n| Key | Takes | |\n|---|---|---|\n| `host` | host | Of the call, or of its subdomains (default: any) |\n| `path` | a path | Of the call, without the host; `*` is any text but `/`, `**` any (default: any) |\n| `graphql` | a name or a list | GraphQL operations (default: any call) |\n\u003c!-- end --\u003e\n\n#### Capturing a suite's flow\n\nHow a capture calls the backend and becomes mocks belongs to the suite, as it describes its backend.\n`capture` says it for every flow, and a flow's entry over it; each key is the option of the same\nname, which wins over both:\n\n```yaml\nnetwork: proxy\ncapture:\n  hosts: [a.example.com, b.example.com]\n  ignoreParams: [_pollTs]\n  ignoreBody: [nonce, deviceData]\n  ignorePathIds: true\n  stateless: [a.example.com/graphql]\n  redactValue: { phone: \"+4915100000000\" }\nflows:\n  - file: flows/checkout.yaml\n    mock: mocks/checkout\n    capture:                    # what it says wins over the suite's, key by key\n      ignoreBody: [nonce, deviceData, cartId]\n```\n\nThen re-recording a flow's mock needs no script: `--capture-update` with the suite, instead of a\nfolder, updates the flow's own mock, the last of its `mock`:\n\n```sh\nrayo test --capture-update staging.suite.yaml --only checkout\nrayo mocks apply mocks/checkout\n```\n\n`capture` is read only while capturing.\n\n\u003c!-- generated: suite capture --\u003e\n| Key | Takes | |\n|---|---|---|\n| `hosts` | a name or a list | As `--hosts` |\n| `from` | url | As `--capture-from` |\n| `graphql` | `operation+variables`, `operation`, `off` | As `--graphql` |\n| `graphqlPath` | regex | As `--graphql-path` |\n| `rest` | `body`, `off` | As `--rest` |\n| `ignoreParams` | a name or a list | As `--ignore-params` |\n| `ignoreBody` | a name or a list | As `--ignore-body` |\n| `timestamps` | `format`, `exact` | As `--timestamps` |\n| `captureHeaders` | a name or a list | As `--capture-headers` |\n| `ignoreHost` | `true` or `false` | As `--ignore-host` |\n| `ignorePathIds` | `true` or `false` | As `--ignore-path-ids` |\n| `stateless` | a path or a list | As `--stateless` |\n| `redactValue` | values | As `--redact-value`: what a private field of a body becomes, such as `phone: \"+4915100000000\"` |\n\u003c!-- end --\u003e\n\n#### The app of a suite\n\n\u003c!-- generated: suite app --\u003e\n| Key | Takes | |\n|---|---|---|\n| `id` | package | The app's (required) |\n| `apk` | file | What to install, if the device has another. rayo doesn't build it (required) |\n\u003c!-- end --\u003e\n\n#### What goes where\n\nShared steps and permissions belong in each flow, so that it can be read and run alone; mocks too,\nunless the same flow also runs without them, see [Mocks of a suite](#mocks-of-a-suite). The\ndevice is chosen as always: use `--emulator` if the flows must never run on a real one.\n\n`rayo check` takes suites too. A folder is the flows directly in it; a suite runs only when its\nfile is given. Subfolders are left out, as they hold what flows share: sub-flows for `runFlow`,\nmocks and the like; give several folders to run the flows of each. A suite in a given folder is\nskipped, saying so on stderr: `skipped e2e/all.suite.yaml (a suite; name it to run it)`.\n\nA file ending in `.rayo.yaml`, the name suites had before, is no longer read: given, it fails,\nsaying the rename, such as `staging.rayo.yaml -\u003e staging.suite.yaml`; in a folder, it is skipped,\nsaying the same.\n\n### Device setup\n\n```yaml\ndevice:\n  size: 1080x2400\n  dpi: 420\n  fontScale: 1.3\n  darkMode: true\n```\n\n\u003c!-- generated: device --\u003e\n| Key | Takes | |\n|---|---|---|\n| `size` | pixels, such as `1080x2400` | Of the screen |\n| `dpi` | 72 to 1000 | Its density. With `size`, it says how many dp there are: 1080 pixels at 420 dpi are 411 dp |\n| `fontScale` | 0.5 to 3 | Of the text; 1 is normal |\n| `darkMode` | `true` or `false` | The dark theme |\n\u003c!-- end --\u003e\n\nWhat isn't said stays as it is. It is only for a suite: all its flows run on the same device, and\none flow alone runs on the device as it is. To run the same flows on two, write two suites.\n\nIt is put back at the end, also after Ctrl-C or if rayo is killed, as the device itself does it. If\nthe device is restarted meanwhile, use `wm size reset` and `wm density reset`. It works on real\ndevices too. The size doesn't change the hardware: a notch stays where it is.\n\n### Clean state\n\nAfter each flow, whether it passed or not, rayo undoes what it changed: the mock and `network`, the\n`camera`, a recording, the location and airplane mode. If it can't, the flow has a `warning`, as the next may be\naffected. The app's data and permissions stay: begin with `launchApp: { clearState: true }`. What a\n`shell` or `http` step changed stays too.\n\n`--shuffle` finds flows that depend on others. The order is printed as `--shuffle=\u003cseed\u003e`, which\nrepeats it.\n\n### Retries\n\nA flow that fails can be run again, whole, after the [clean state](#clean-state). The suite says how\nmany times, for all its flows and for each:\n\n```yaml\nretries: 1                          # all, if they don't say\nflows:\n  - tests/book/flow.yaml            # 1\n  - file: tests/scan/flow.yaml      # 3\n    retries: 3\n  - { file: tests/pay/flow.yaml, retries: 0 }   # never\n```\n\n`--retries \u003cn\u003e` wins over both. **Without any of them it is 1**, also for flows without a suite;\n`--retries 0` turns it off. With `-c` and no suite it is 0. It is retried at once, before the next flow. `NOT_RUN` and `INTERRUPTED` aren't, nor when capturing.\n\n`retries: 3` means up to 4 tries: the first and 3 more. The line says `passed on retry 2` when the\nthird try passed, and `failed all 4 tries` when none did.\n\nIf it passes at last it is `FLAKY`, not `PASS`, with a note of where each try failed, its\n[blame](#blame) and the folder with the rest. Each has its own video too.\n\n```\n[2/3] FLAKY scan  31.0 s  (passed on retry 1)\n  try 1: tests/scan/flow.yaml:12: tapOn scan: not on screen: scan  (test)  .results/scan\n[3/3] FAIL pay  27.2 s  (failed all 4 tries)\n  try: 1\n  ...\n\nTOTAL 3: 1 passed, 1 flaky, 1 failed, 2 retried  78.1 s\n```\n\n| | |\n|---|---|\n| `retried` | Flows tried more than once, whether they passed or not |\n| JSON | `flaky` of the run; of a flow, `status` `FLAKY`, `tries`, `timesFailed`, `retries` allowed, and a failure for each failed try |\n| JUnit | See [Flaky flows](#flaky-flows) |\n\n#### Flaky flows\n\nA run whose flows all passed, some of them `FLAKY`, exits `0`; with `--flaky-fails-build`, or\n`flakyFailsBuild: true` in the suite, it exits `4`, see [Exit codes](#exit-codes).\n\nIn JUnit, as Maven Surefire writes it, which Jenkins, GitLab and most CI read: the `testcase` passes, and\nhas a `flakyFailure` for each try that failed, its `message` the error and the [blame](#blame), its `type` the\ncause and its `stackTrace` all there is to know, as in a `.txt`. It isn't counted in `failures`. A `failure`\nhas the verdict as its `message` and the cause as its `type`; an `error` (interrupted) has `interrupted`. The\n`classname` of each `testcase` is its folder from the one all flows are in, with dots, so CI pages group\nthem; `file` is from where rayo runs, with slashes. What XML can't hold is left out, and a text over 256 KiB\nkeeps its beginning and its end.\n\n```xml\n\u003ctestcase name=\"scan\" classname=\"scan\" file=\"tests/scan/flow.yaml\" time=\"31.000\"\u003e\n  \u003cflakyFailure message=\"try 1: tests/scan/flow.yaml:12: tapOn scan: not on screen: scan (test)\" type=\"not on screen\"\u003e\n    \u003cstackTrace\u003e  why: not on screen: scan ...\u003c/stackTrace\u003e\n  \u003c/flakyFailure\u003e\n\u003c/testcase\u003e\n```\n\nTo run only some steps again, use [retry](#retry).\n\n### Blame\n\nNot every failure is the test's. Each says whose fault it is, its `blame`:\n\n| Blame | When |\n|---|---|\n| `app` | The app crashed or hung during the flow, or isn't installed, or an element wasn't found while the app was in front with nothing readable on it: still starting, stuck, or not readable by accessibility. That one's cause stays `not on screen` |\n| `device` | It doesn't answer, is starting, restarted, its interface hung, or the connection was lost |\n| `test` | Anything else. It is only `app` or `device` if there is proof of it, so something slow to appear on a screen that shows other things is `test` |\n\nThe run stops, so as not to waste time on flows that would fail the same, when:\n\n| | |\n|---|---|\n| The device can't be used: it doesn't answer, is starting, or Android isn't running | At once, always |\n| 2 failures in a row are `device` | Unless `--keep-going` |\n| 3 different flows in a row fail with `app`, and the same crash | Unless `--keep-going` |\n\nFor `device`, each try counts. A failure that is the test's, or a pass, starts the count again. The rest are `NOT_RUN`, and the results say why the run stopped:\n\n```\n[3/9] FAIL pay  4.1 s\n  why: not on screen: home\n  blame: app, the app crashed: java.lang.IllegalStateException: oops\n  ...\n[4/9] NOT_RUN scan  (app: 3 flows in a row, the app crashed: ...)\n\nSTOPPED app: 3 flows in a row, the app crashed: java.lang.IllegalStateException: oops\nTOTAL 9: 0 passed, 3 failed, 6 not run, of those failed 3 app  14.0 s\n```\n\nJSON has `stopped` for the run, `cause` for a flow that failed, and `cause` and `because` for each failure.\n\n### Outcomes\n\nEvery flow asked for has one; none is left out of the count.\n\n| | | Exit code |\n|---|---|---|\n| `PASS` | At the first try | 0 |\n| `PARTIAL` | It passed, but only part of it ran: it began or stopped at a step ([`--resume`, `--from`, `--until`](#resume)), or was fixed [while held](#hold). It stays in `failed.json`, and the summary says `NOT WHOLE` | 0 |\n| `FLAKY` | It failed, and passed when [retried](#retries) | 0, or 4 with [`flakyFailsBuild`](#flaky-flows) |\n| `FAIL` | | 1 |\n| `INTERRUPTED` | Ctrl-C while it ran | 130 |\n| `NOT_RUN` | With the reason: interrupted, `--fail-fast`, or the run [stopped](#blame) | 1 |\n\nIf the connection is lost, the flow fails and the next goes on with a new one. `--rerun-failed` runs what is in `\u003coutput\u003e/failed.json`: what\nfailed, was interrupted or didn't run, until it passes; and what passed only in part, until it passes whole.\n\n### Results\n\nWith several flows, one line each, and for a failure what helps to know why: first the verdict,\n`\u003cflow\u003e failed at \u003cstep\u003e: \u003ccause\u003e`, then the rest. Of `step failed`, the verdict goes on with the first line\nof why and where the step is written, such as `login failed at assertTrue ${ok}: step failed: it is false (login.yaml:12)`,\nso the line is enough on its own; a failure that isn't of a step says `failed before its first step` or\n`failed outside a step`. A single flow shows every step. The last line says where the results went:\n`results: .rayo  (rayo explain)`, when a flow didn't pass.\n\nHow much is printed is `--detail`, or `detail:` in a suite:\n\n| | |\n|---|---|\n| `brief` | Only the verdict and `saved` of a failure; no steps |\n| `normal` | The default: as below; every step of a single flow |\n| `full` | Also `screen`, and every step of every flow; the [files](#failures) of a failure are whole |\n\nThe cause of a verdict, `cause` in JSON, is what went wrong; [`blame`](#blame) is whose fault it is, and `message` of an [error](#errors) is the text:\n\n| | |\n|---|---|\n| `not on screen` | What a step looked for wasn't there; `nearest` says what was most like it |\n| `timeout` | Something didn't happen in time, or stayed on screen |\n| `app crashed` | The app crashed, hung or isn't installed, see [Blame](#blame) |\n| `unmocked call` | The app called what no [mock](#mock) answers |\n| `device error` | The device was lost, restarted or hung, see [Blame](#blame) |\n| `step failed` | Anything else, such as a script or a command that failed |\n\n```\n[1/3] PASS book  18.2 s\n[2/3] FAIL scan  9.1 s\n  scan failed at tapOn finished_close: not on screen\n  why: not on screen: finished_close\n  step: tapOn finished_close\n  at: shared/flows/dock.yaml:8\n  from: tests/scan/flow.yaml:23 runFlow ../../shared/flows/dock.yaml\n  before:\n    extendedWaitUntil finished\n  nearest:\n    id: com.rides:id/finish_close | text: Close  [40,2100][1040,2220]\n  calls:\n    12:00:01.250 GET /trips -\u003e 200\n  saved: .results/scan\n[3/3] NOT_RUN long  (--fail-fast, after scan)\n\nPASS book  18.2 s\nFAIL scan  9.1 s\nNOT_RUN long  (--fail-fast, after scan)\n\nTOTAL 3: 1 passed, 1 failed, 1 not run  27.4 s\n```\n\nThe list at the end is only there if something didn't pass. Under a failure:\n\n| Key | |\n|---|---|\n| `try` | Which, if there were several |\n| | The verdict, a line without a key |\n| `why` | The error |\n| `blame` | `test`, `app` or `device`, and what shows it, see [Blame](#blame) |\n| `step`, `at` | The step that failed, and where it is written |\n| `from` | The step that ran that flow, and so on outwards |\n| `crash` | The app crashed or hung during the flow: what the device log says |\n| `before` | The last 3 steps that passed |\n| `nearest` | Of `not on screen`, at most 5 elements on it most like what was looked for, the closest first |\n| `calls` | The last 5 to the [mock](#mock), and before them those of the failure window that weren't mocked or weren't answered 2xx or 3xx |\n| `screen` | With `--detail full`, the elements on it, at most 40 lines; they are all in `screen.txt` |\n| `saved` | The folder with the [rest](#failures) |\n| `recording` | A video; of flows that passed, only in the file |\n| `warning` | Something that may explain this, or the next flow |\n\n`--results \u003cfile\u003e` writes it, as the file's name says. It may be repeated, for several formats at once:\n\n```\nrayo test --results out.txt --results junit.xml all.suite.yaml\n```\n\n| File | |\n|---|---|\n| `.txt` | As above for every flow, with `run`, `device` and `began` at the top and each flow's `file` |\n| `.json` | For a program |\n| `.xml` | JUnit, for a CI, see [Report](#report) |\n| `.md` | Short, for a chat or a mail: the verdict and counts, then what failed, what was flaky and what was slow |\n\nA path is as written: from where rayo runs, or in a suite from the suite. rayo sends nothing anywhere;\nit writes the report, and you send it.\n\n`--show`, or `show:` in a suite, says which flows the results list, the screen's list at the end too:\n`failed` (also interrupted and not run), `flaky`, `slow`, `all` (default), or several, such as\n`--show failed,flaky`. They are all counted anyway. `slow` is the slowest, as many as `--slowest \u003cn\u003e` says (default 5),\nwhich is also the length of the slow list of `.md` and of `summary.slowest` in `.json`. JUnit always has every flow,\nas what reads it counts them. Which files stay is [`keep`](#what-is-kept)'s.\n\nEach flow has its attempts (`passed on retry 2`, `failed all 3 tries`), its time, when it began, and the\nstep that failed and why. `--timings` and `--profile` are only for the screen, while it runs.\n\n```markdown\n# Rides\n\n**FAIL** · 3 flows: 1 passed, 1 flaky, 1 failed · 78.1 s · 2026-11-22T21:11:18Z · emulator-5554\n\n## Failed\n\n- **pay** 27.2 s, failed all 2 tries: `tapOn Pay` at tests/pay/flow.yaml:12: not on screen: Pay (test)\n\n## Flaky\n\n- **scan** 31.0 s, passed on retry 1: `tapOn scan` at tests/scan/flow.yaml:9: not on screen: scan (test)\n\n## Slowest\n\n- scan 31.0 s\n- pay 27.2 s\n- book 19.9 s\n```\n\n#### A template of your own\n\n`--results-template \u003cfile\u003e`, or `resultsTemplate:` in a suite, writes the results whose name ends as its own does\n(`report.md` writes every `--results *.md`) as it says, instead of the format built in:\n\n```\n{verdict} {suite}: {passed} passed, {flaky} flaky, {failed} failed, {skipped} not run in {duration}, from {started}\n{flows}- {status} {name} {duration} {attempts} {step} {error}\n{end}\n```\n\n| Run | |\n|---|---|\n| `{suite}` | The suite's name, or what was run |\n| `{verdict}` | `PASS`, `FAIL`, `FLAKY` or `PASS, with flaky flows` |\n| `{total}`, `{passed}`, `{flaky}`, `{failed}`, `{interrupted}`, `{skipped}` | How many; `{skipped}` didn't run |\n| `{duration}`, `{started}` | Such as `78.1 s`; in UTC |\n| `{device}` | Its serial |\n| `{flows}`...`{end}` | Written once for each flow that `show` says |\n\n| A flow, between `{flows}` and `{end}` | |\n|---|---|\n| `{name}`, `{file}`, `{status}` | Its [outcome](#outcomes) |\n| `{attempts}`, `{tries}` | Such as `passed on retry 2`, empty for one try; how many |\n| `{duration}`, `{started}` | All its tries; when the first began |\n| `{step}`, `{at}`, `{error}` | Of its last try that failed: the step, where it is written, and what went wrong |\n| `{cause}`, `{blame}` | Of that try: the cause of its verdict, such as `not on screen`; `test`, `app` or `device`, see [Blame](#blame) |\n\nIn a `.txt`, a line that begins with an outcome is a flow: `grep -E '^(FAIL|NOT_RUN)' results.txt`. What\nfollows is about it: `  key: value`, or `  key:` and lines with four spaces. `TOTAL` is the last line.\n\nIn a `.json`, every key is always there: what there is none of is `\"\"`, `0` or `[]`.\n\n```json\n{\n  \"schemaVersion\": 3,\n  \"run\": \"Rides\",\n  \"device\": \"emulator-5554\",\n  \"order\": \"\",\n  \"stopped\": \"\",\n  \"ok\": false,\n  \"summary\": {\n    \"began\": \"2026-11-22T21:11:18Z\", \"seconds\": 27.402,\n    \"total\": 3, \"passed\": 1, \"flaky\": 0, \"failed\": 1, \"interrupted\": 0, \"notRun\": 1, \"retried\": 0,\n    \"failedFlows\": [\"scan\", \"long\"], \"flakyFlows\": [],\n    \"slowest\": [{ \"name\": \"book\", \"seconds\": 18.21 }, { \"name\": \"scan\", \"seconds\": 9.114 }]\n  },\n  \"flows\": [\n    {\n      \"name\": \"scan\",\n      \"file\": \"tests/scan/flow.yaml\",\n      \"status\": \"FAIL\",\n      \"blame\": \"test\",\n      \"began\": \"2026-11-22T21:11:36Z\",\n      \"seconds\": 9.114,\n      \"attempts\": \"\",\n      \"step\": \"tapOn finished_close\",\n      \"at\": \"shared/flows/dock.yaml:8\",\n      \"why\": \"not on screen: finished_close\",\n      \"tries\": 1,\n      \"timesFailed\": 1,\n      \"retries\": 0,\n      \"notRunReason\": \"\",\n      \"warnings\": [],\n      \"recordings\": [],\n      \"failures\": [\n        {\n          \"try\": 1,\n          \"error\": {\n            \"file\": \"shared/flows/dock.yaml\", \"line\": 8, \"step\": \"tapOn finished_close\",\n            \"message\": \"not on screen: finished_close\", \"expected\": \"\", \"nearest\": \"\", \"fix\": \"\"\n          },\n          \"verdict\": \"scan failed at tapOn finished_close: not on screen\",\n          \"cause\": \"not on screen\",\n          \"blame\": \"test\",\n          \"because\": \"\",\n          \"from\": [{ \"step\": \"runFlow ../../shared/flows/dock.yaml\", \"file\": \"tests/scan/flow.yaml\", \"line\": 23 }],\n          \"before\": [\"extendedWaitUntil finished\"],\n          \"calls\": [{ \"at\": \"2026-11-22T21:11:40.250Z\", \"method\": \"GET\", \"uri\": \"/trips\", \"outcome\": \"200\", \"mocked\": true, \"body\": \"\" }],\n          \"screen\": [\"...\"],\n          \"crash\": \"\",\n          \"saved\": \".results/scan\"\n        }\n      ]\n    }\n  ]\n}\n```\n\nThe run:\n\n\u003c!-- generated: results --\u003e\n| Key | | |\n|---|---|---|\n| `schemaVersion` | number | Changes if the meaning of a key does |\n| `run` | text | The suite's name, or what was run |\n| `id` | text | The run's own, as failed.json and history.jsonl give it: when it began, in UTC, then 4 hex digits |\n| `device` | text | Its serial |\n| `order` | text | `--shuffle=\u003cseed\u003e`, if shuffled |\n| `shard` | mapping or null | The part of --shard i/n this run was; null if all |\n| `stopped` | text | Why, if it stopped before the end, see [Blame](#blame) |\n| `ok` | true or false | Nothing failed, was interrupted or didn't run, nor was flaky with flaky-fails-build: as the exit code |\n| `summary` | mapping | The run at a glance, see below |\n| `flows` | list | Those that show says, see below |\n\u003c!-- end --\u003e\n\n`summary`:\n\n\u003c!-- generated: results summary --\u003e\n| Key | | |\n|---|---|---|\n| `began` | text | In UTC |\n| `seconds` | number | All of it |\n| `total` | number | Flows |\n| `passed` | number | At the first try |\n| `flaky` | number | Failed, then passed on a retry |\n| `partial` | number | Passed, but began or stopped at a step (--resume, --from, --until, or fixed while held): not a pass of the whole flow |\n| `failed` | number |  |\n| `interrupted` | number |  |\n| `notRun` | number |  |\n| `retried` | number | Flows tried more than once, whether they passed or not |\n| `failedFlows` | list | The names of those that failed, were interrupted or didn't run |\n| `flakyFlows` | list | The names of those that were flaky |\n| `slowest` | list | The flows that took longest, as many as slowest says |\n| `exit` | mapping | The exit code of the run, see [Exit codes](#exit-codes) |\n\u003c!-- end --\u003e\n\nEach of `flows`:\n\n\u003c!-- generated: results flow --\u003e\n| Key | | |\n|---|---|---|\n| `name` | text | The flow's |\n| `file` | text | The flow's |\n| `status` | text | One of the [outcomes](#outcomes) |\n| `blame` | text | Of a flow that failed, whose fault it is, or empty, see [Blame](#blame) |\n| `began` | text | Its first try, in UTC; empty if it didn't run |\n| `seconds` | number | All its tries |\n| `attempts` | text | Such as 'passed on retry 2' or 'failed all 3 tries'; empty for one try |\n| `step` | text | Of its last try that failed, the step that failed |\n| `at` | text | Where that step is written, file:line |\n| `why` | text | What went wrong in that try |\n| `tries` | number | Those it took |\n| `timesFailed` | number | Of those |\n| `retries` | number | Those allowed |\n| `notRunReason` | text | If it is `NOT_RUN` |\n| `warnings` | list | What may explain this, or the next flow |\n| `recordings` | list | Videos |\n| `mark` | mapping or null | What was said of it while held, with .mark; null if nothing |\n| `failures` | list | One for each try that failed, also of a flow that passed at last |\n\u003c!-- end --\u003e\n\nEach of `failures`:\n\n\u003c!-- generated: results failure --\u003e\n| Key | | |\n|---|---|---|\n| `try` | number | From 1 |\n| `verdict` | text | In a line: '\u003cflow\u003e failed at \u003cstep\u003e: \u003ccause\u003e'; of 'step failed', then the first line of why and `(\u003cfile\u003e:\u003cline\u003e)`; 'failed before its first step' or 'failed outside a step' if it isn't of a step; of an interrupted flow, '\u003cflow\u003e interrupted at \u003cstep\u003e (\u003cfile:line\u003e)' |\n| `cause` | text | What went wrong, see [Results](#results) |\n| `error` | mapping | What went wrong, at which step, see [Errors](#errors) |\n| `blame` | text | Whose fault it is: `test`, `app` or `device`, see [Blame](#blame) |\n| `because` | text | What shows that it isn't the test |\n| `from` | list | The step that ran that flow, and so on outwards |\n| `before` | list | The last 3 steps that passed |\n| `nearest` | list | Of 'not on screen', the elements on it most like what was looked for |\n| `calls` | list | The last 5 to the mock, and before them those that went wrong in the failure window |\n| `screen` | list | The elements on it |\n| `crash` | text | The app crashed or hung during the flow: what the device log says |\n| `saved` | text | The first file of the rest, which are named alike, see [Failures](#failures) |\n| `scenarios` | list | Of the mocks, the state each was in as it failed; empty without scenarios |\n| `trail` | list | The scenarios as each step began, the last 2000 steps: what --resume and --from set, see [Resume](#resume) |\n| `next` | list | What to do about it, a line each, by its cause and blame: the same for the same failure, for a program to follow |\n\u003c!-- end --\u003e\n\n```\njq -r '.flows[] | select(.status == \"FAIL\") | \"\\(.name): \\(.failures[-1].verdict)\"' results.json\n(Get-Content results.json -Raw | ConvertFrom-Json).flows | Where-Object status -eq FAIL\n```\n\n### Explain\n\n`rayo explain` prints, for each flow of the last run that hasn't passed since, only what tells why:\nthe verdict, `why`, `at`, `from`, the `nearest` elements, the `calls` and `saved`, as a failure does. It\nreads `\u003coutput\u003e/failed.json`, which keeps the last failure of each. Give it a run's folder, a `.zip` of one as\nCI keeps it, or a suite, whose `output` it reads; and a flow's file or name. With none, it is `.rayo`, or else\nthe output folder of a suite here, or a folder down, whose `failed.json` is newest, and it says which:\n\n```\nrayo explain\nrayo explain pay\nrayo explain all.suite.yaml pay\nrayo explain rayo-results.zip --format json\nrayo explain --latest\n```\n\n`failed.json` keeps a flow until it passes, so after a few runs it has flows of more than one; each says\nits `run`, and `--latest` keeps those of the newest. Each run adds a line for each flow to\n`\u003coutput\u003e/history.jsonl`, the last 5000: `history:` says how a flow's last runs went, such as\n`failed 3 of its last 10 runs, at the same step each time: it passes at times, so it may be flaky`.\n`next:` says what to do about the failure, by its cause and whose it is, the same each time, also in\nthe results as `next`.\n\n`--find \u003ctext\u003e`, on `rayo hierarchy` and `rayo do --screen`, prints only the elements of the screen most\nlike the text, the closest first, instead of all of them.\n\n`--changed-only`, on `rayo test` and `rayo check`, keeps only the flows of which a file changed since the last\ncommit, also those not committed yet: the flow's own, one it runs with `runFlow`, or a file of its mocks.\n`--since \u003cgit ref\u003e` does so since that ref, such as `--since origin/main`.\n\n`rayo check` says which flows would run, in that order, with their tags and mocks; with `--format json`, also the retries of each.\nTwo flows with the same name are an error, as `--only` and the output folder can't tell them apart: give\none a `name:` in its header. A name of `--only`, or a tag, that no flow has is an error that says the\nnearest one and those there are; a tag no flow has is let be when another of the list is there. A key\nwritten twice in a step, a `when` or `while` without a condition, an empty `text` or `id`, and an `id`\nwith spaces around it are errors too. Most mistakes of a name, a key or a word end in `did you mean ...?`;\n`rayo help test` says the key of a suite that each option overrides, as `(suite: output.keep)`.\n\n### For a program\n\n`--format` says what is printed: `human`, the default; `json`, for a program; `ndjson`, the same on\none line. `test`, `check`, `devices`, `hierarchy`, `do` and `mocks` take it, and a suite's `format`\ntoo, which `--format` wins over. With `json` or `ndjson`, only that goes to the standard output.\n\n| Command | `--format json` prints | Schema |\n|---|---|---|\n| `test` | The [results](#results), as a `.json` of `--results` | `rayo schema results` |\n| `check` | `schemaVersion`, `ok` and how many `flows` are fine | `rayo schema check` |\n| `devices` | `schemaVersion` and the `devices` | `rayo schema devices` |\n| `hierarchy` | `schemaVersion` and the `screen`, a line each | `rayo schema hierarchy` |\n| `do` | See [From other programs](#from-other-programs) | `rayo schema do` |\n| `mocks` | `schemaVersion` and the `notes` | `rayo schema mocks` |\n\nIf a command can't do what it was asked, it prints instead:\n\n\u003c!-- generated: errors --\u003e\n| Key | | |\n|---|---|---|\n| `schemaVersion` | number | Changes if the meaning of a key does |\n| `ok` | true or false | false |\n| `exit` | number | The exit code, see [Exit codes](#exit-codes) |\n| `errors` | list | One for each, see [Errors](#errors) |\n\u003c!-- end --\u003e\n\n`schemaVersion` changes if the meaning of a key does; every key is always there.\n\n### Exit codes\n\nThe same whatever `--format` says:\n\n\u003c!-- generated: exit codes --\u003e\n| Code | | |\n|---|---|---|\n| `0` | passed | Every flow passed, or the command did what it was asked |\n| `1` | failed | A flow failed, was interrupted or didn't run; or a step sent with `rayo do` failed |\n| `2` | configuration | Something written is wrong: an option, a flow, a suite or a mock; nothing ran |\n| `3` | device | No device to run on, or it was lost or unusable, so the flows couldn't run or didn't all run |\n| `4` | flaky | Every flow passed, some only on a retry, and `flaky-fails-build` is `true`; else such a run exits `0` |\n| `130` | interrupted | Ctrl-C, or SIGTERM as CI sends when it cancels a job: what ran is in the results, each interrupted flow with the step it was at |\n\u003c!-- end --\u003e\n\n### Errors\n\nMistakes in what is written and flows that failed are told the same way, as data:\n\n\u003c!-- generated: error --\u003e\n| Key | | |\n|---|---|---|\n| `file` | text | Where it is written; empty if it isn't of a file |\n| `line` | number | 0 if not known |\n| `step` | text | Of a flow that failed, the step |\n| `message` | text | What is wrong |\n| `expected` | text | What it takes, if that can be said |\n| `nearest` | text | Of that, the closest to what is written |\n| `fix` | text | What to write instead, if that can be said |\n\u003c!-- end --\u003e\n\n```json\n{ \"file\": \"flows/login.yaml\", \"line\": 3, \"step\": \"\", \"message\": \"unknown command \\\"tapOm\\\", did you mean tapOn?\",\n  \"expected\": \"\", \"nearest\": \"tapOn\", \"fix\": \"tapOn\" }\n```\n\n### Events\n\n`rayo test --format ndjson` prints a line as each thing happens, so a program needn't wait for the\nend: `flowStarted`; for each try, `stepFailed` if a step failed, then `attempt`; with `--on-fail hold`,\n`held` as a flow is [held](#hold), with the `session`'s folder; `flowFinished`, after all its tries; and last\n`outcome`, with the results. If the run can't begin, the only line is `error`.\n\n\u003c!-- generated: event --\u003e\n| Key | | |\n|---|---|---|\n| `schemaVersion` | number | Changes if the meaning of a key does |\n| `event` | text | What happened, see [Events](#events) |\n| `flow` | text | Its name; empty in outcome and error |\n| `file` | text | The flow's |\n| `index` | number | Of the flow, from 1 |\n| `of` | number | Flows to run |\n| `try` | number | Of stepFailed and attempt, from 1 |\n| `status` | text | Of attempt, PASS or FAIL; of flowFinished, the flow's [outcome](#outcomes) |\n| `error` | mapping or null | Of stepFailed and of an attempt that failed, see [Errors](#errors) |\n| `result` | mapping or null | Of flowFinished, the flow as in the [results](#results) |\n| `results` | mapping or null | Of outcome, the [results](#results), as --format json |\n| `exit` | number | Of outcome and error, the exit code, see [Exit codes](#exit-codes) |\n| `errors` | list | Of error, as rayo prints them with --format json |\n| `session` | text | Of held, the output folder of the session that holds the flow: rayo do --output \u003cit\u003e .help, or rayo do .help there |\n\u003c!-- end --\u003e\n\n```\nrayo test --format ndjson all.suite.yaml | jq -c 'select(.event == \"stepFailed\") | .error'\n```\n\n---\n\n## Live sessions\n\nWrite a test by running it, a command at a time. Good for other programs too: they see what each\nstep did before choosing the next.\n\n```\n$ rayo live --emulator --app com.example.app --mock ../mock tests/new/flow.yaml\nlive: type commands, or send them with rayo do; .help\n- launchApp: { clearState: true }\n  ✓ launchApp clearState  812 ms\n- tapOn: { id: nothing }\n  ✗ tapOn nothing  15012 ms\nerror: tests/new/flow.yaml:1: tapOn nothing: not on screen: nothing\nscreen:\n  ...\n- tapOn: { id: login }\n  ✓ tapOn login  95 ms\n.save\nsaved 2 steps in tests/new/flow.yaml:4-5\nrayo check: the flow is fine\n```\n\nIf the flow is there already, its steps run first (`--no-run` to skip them) and `.save` adds to its\nend, leaving the rest as it is. A new one needs `--app`, and `--mock` takes paths relative to it.\n`rayo live all.suite.yaml login` is the suite's flow with the suite's `env`, `output`, `timeoutScale`, `ca`\nand `unmocked`; `-e KEY=VALUE` sets a variable over them, as for `rayo test`.\nThe mock, the variables and `output` last for the whole session. When it ends, all is\n[put back](#clean-state).\n\n### Live commands\n\n| | |\n|---|---|\n| Any command, or a list of them | Runs them up to the first that fails. Those that pass are kept |\n| `.screen` | What is on screen, and how to refer to it |\n| `.steps` | The steps done, numbered as `.rewind` counts them, each saved (with its step and line in the file) or unsaved |\n| `.undo` | Forgets the last one, and says how many are left. **The device stays as it is** |\n| `.save` | Adds them to the file, says on which lines, and runs `rayo check` on it |\n| `.retry` | Reads the file again and runs its steps from the one that failed, so a step can be fixed in the file and tried again |\n| `.from \u003cn\u003e` | The same, from its n-th step |\n| `.rewind [n]` | Forgets the last n steps done (1), typed or of the file, and the calls they made to a [capture](#capture). **The device stays as it is**: move the app back by hand, then `.retry` |\n| `.help`, `.quit [--discard]` | With steps not saved, `.quit` is an error and the session goes on: `.save` them, or `.quit --discard` ends and prints what is lost. The end of what is typed discards |\n\nTyped, a command is a line; one that ends in `:` goes on until an empty line. The leading `- ` may\nbe left out.\n\n### From other programs\n\n`rayo do` sends commands to the live session, from the same folder or with the same `--output`.\nEach call answers when the steps have run; its exit code is 1 if one failed, see [Exit codes](#exit-codes).\n\n```\nrayo live --serve --emulator tests/new/flow.yaml \u0026     # until .quit, or an hour unused\nrayo do .screen\nrayo do --screen '- tapOn: { id: login }'\nrayo do --format json - \u003c steps.yaml\nrayo do .save\nrayo do .quit\n```\n\n| Option | |\n|---|---|\n| `--screen` | Also what is on screen afterwards. After a failure it always comes |\n| `--screen=changes` | Only the lines that appeared (`+`) or went (`-`) since the screen the session last gave. The first has no screen before: it is all of it, said as `(whole screen: nothing before)`, and `baseline: true` |\n| `--ignore-screen \u003cregex\u003e` | With `--screen`, leaves out the lines that match, such as a clock's, and says how many (`ignored`) |\n| `--timeout \u003cduration\u003e` | Waits for the answer that long at most, then fails; the session goes on with the steps |\n| `--discard` | With `.quit`, ends though steps were not saved |\n| `--format json` | `schemaVersion`, `ok`, `output`, `error`, `failure` (as in the [results](#results), or `null`), `warnings`, `calls` to the mock meanwhile, `screen`, `baseline`, `ignored`, `unsaved` steps; `rayo schema do` |\n| `-` | Commands from standard input |\n\nCalls the app makes without a mock are warnings here: the session goes on. A run of the saved\nflow would fail.\n\nOnly this computer can use a session, with a token kept in `\u003coutput\u003e/live.json`.\n\n---\n\n## Features\n\n### Schemas\n\nFor editors to complete and check what is written, and for programs to know what is valid. At the\ntop of a file, for those that understand it (such as VS Code with the YAML extension):\n\n```yaml\n# yaml-language-server: $schema=../../rayo/schema/flow.json\n```\n\nThey know the keys and values. What depends on other files, or on several keys at once, only\n`rayo check` tells.\n\n### Variables\n\nUsed as `${NAME}`. The first that has it wins:\n\n1. `env` of the `runFlow` or `runScript` running now\n2. `-e NAME=value`\n3. `env` of the suite, for all its flows\n4. What is set where the file is included\n5. `env` in the file's header: a default, for what nothing above sets\n\nIn `${...}` and conditions, a name that nothing sets is `undefined`, not an error, so defaults work\nwith `||` or `??`. Only a whole `${...}` that ends up `undefined` fails, as there is nothing to write:\n\n```yaml\nenv:\n  CITY: ${ CITY || \"Springfield\" }\n```\n\nValues of `-e`, of a suite's `env` and of a header may all use `${...}`. Besides the variables,\nrayo gives `rayo.file`, the flow's file name without its extension, and `rayo.platform`. To give one of\nthem a name that flows from another tool expect, set it once for all of them:\n\n```yaml\n# all.suite.yaml\nflows: [login.yaml, book.yaml]\nenv:\n  MAESTRO_FILENAME: ${rayo.file}\n```\n\nor, without a suite, `rayo test -e 'MAESTRO_FILENAME=${rayo.file}' flows/`.\n\n### Tags\n\n🧪 `--include-tags smoke,login` runs flows with any of them; `--exclude-tags slow` leaves flows out.\n\n### Continuous mode\n\n🧪 `rayo test -c` runs again when a flow, or a file it includes, is saved.\n\n### Resume\n\nAfter a fix to the mocks or a flow, go on from the step that failed instead of running the flow again:\n`rayo test --resume \u003csuite\u003e`. The files are read again; the step that failed runs first. `onFlowStart`\nand the steps before it don't run, so the app must be where the failure left it; their `mock:` steps\nand the mocks of the flows they `runFlow` are still loaded. The mocks' scenarios are set as they were\nas that step began: each failure in `failed.json` keeps them in `trail`, and `scenarios` as it failed.\nEach call says the `mapping` that answered it and what it did to its `scenario`.\n\n`--from \u003cfile:line\u003e` begins one flow at any step, before or after the failure, and `--until \u003cfile:line\u003e`\nstops it after one, leaving the app there, without `onFlowComplete`. `--mock-state booking=booked` sets\nscenarios over those of the last run. Such a run has no retries, and a pass is `PARTIAL`, which leaves the flow in\n`failed.json`: run it whole to be sure. In a live session, `.states` lists the scenarios.\n\nA step inside a `runFlow` with `when`, or a `repeat` with `while`, can be begun from: the condition is\ntaken as holding to reach it, and says so. The `startRecording` steps of `onFlowStart` still run, so\nthat a `stopRecording` finds its recording; a `stopRecording` of a recording the flow began before the step\nonly says so. `--resume` leaves out a flow that failed in `onFlowStart` or `onFlowComplete`, and says\nso, and goes on with the others.\n\n### Hold\n\n`rayo test --on-fail hold \u003csuite\u003e` stops at the first failure, before `onFlowComplete` and without\nretries, with the device, the app and the mocks as they are, and says `HELD \u003cflow\u003e` with the verdict,\n`why`, `at`, `from` and `nearest`. The flow is fixed in its own files, run again from where it failed,\nand the run goes on: nothing is copied and no flow is written for it. `rayo do` reaches it, in the\nsuite's output folder by itself; typed commands work too.\n\n| | |\n|---|---|\n| `.why` | The failure again |\n| `.screen`, `.find \u003ctext\u003e` | What is on screen now; or only what is most like a text |\n| `.diff` | What of the screen changed since it failed: `+` appeared, `-` went |\n| `.retry` | Reads the flow and the flows it runs again, and runs from the step that failed to the end of its steps. A step that moved, as lines were added above it, is found by its text, or else by its place among the file's steps |\n| `.from \u003cstep\u003e` | The same from another step: `file:line`, a text of one, or n of `.steps`; also inside a `when` or a `repeat` |\n| `.rewind [n]` | The same from the n-th step before the one that failed (1). The app stays as it is: move it back first if those steps changed it |\n| `.steps` | The last steps that passed, numbered as `.from` and `.rewind` count them |\n| `.probe [n]` | Runs the step that failed n times (5), as it is now, and says how often it passes and how long each took: a flake, or always |\n| `.states` | The mocks' scenarios and the state of each |\n| Any command, or a list | Runs as it is, to move the app; it isn't added to the flow |\n| `.mark \u003cas\u003e [why]` | `app-bug`, `test-bug` or `flaky`, kept in the results and `failed.json` as `mark` |\n| `.continue` | Once its steps passed: `onFlowComplete`, then the next flows |\n| `.skip`, `.abort` | Leave it failed and go on; or run no more flows |\n\nA flow fixed while held is `PARTIAL`, not `PASS`: it stays in `failed.json` and `rayo test --rerun-failed`\nruns it whole. If it checks less than when it failed, fewer asserts or more `optional` steps, its warnings\nsay `WEAKENED`. With no command for an hour, or `--hold-timeout`, the flow is left failed and the run goes on.\n`rayo do --format json` answers with `held`: the flow, `at`, and `fixed` once its steps passed.\n\n### Failures\n\nSaved in `\u003coutput\u003e`, next to `failed.json`, named after the flow and the try: `\u003cflow\u003e_failure.jpg`,\nand `\u003cflow\u003e_retry1_failure.jpg` for the second try; `\u003cflow\u003e_2_...` for a second flow with that name.\nThose of an earlier run are removed when the flow runs again. `saved` is the first of them.\nWith `--screens-before n`, or the suite's `output.screensBefore`, the screen after each of the last n steps that\npassed is in `\u003cflow\u003e_screens-before.txt`: how the app got there, such as a dialog that came and went.\n\n| File | Kind | |\n|---|---|---|\n| `failure.jpg` | `screenshot` | The screen, small: 480 pixels on the short side |\n| `screen.txt` | `screen` | The elements on it, as `rayo hierarchy` shows them |\n| `logcat.txt` | `logcat` | The device log from the failure window to the failure, of the app and of crashes; whole with `--detail full` |\n| `requests.txt` | `requests` | The calls the app made to the [mock](#mock) in the failure window, with their bodies; all with `--detail full`; `requests.json`, a list like `calls`, if the results are also written to a `.json` |\n\nThe failure window begins 10 s before the failing step did; `--failure-window \u003cseconds\u003e`, or\n`failureWindow:` in a suite, says how long.\n\n### What is kept\n\nWhat a run writes, and where, is said in one place: `output` in a [suite](#suites). Each key has an\noption of `rayo test` with the same name, in kebab-case, that overrides it for one run; `rayo check`\ntakes them too, and says what the run would write.\n\n```yaml\noutput: .rayo                                  # only the folder\noutput: { folder: out, keep: failed, video: true }\n```\n\n| | Takes | |\n|---|---|---|\n| `folder`, `--output` | a folder, relative to the suite (default `.rayo` where rayo runs) | Where everything goes |\n| `video`, `--video` | `true` or `false` (default `false`) | A video of each flow whose header doesn't say `record`, see [Recording a whole flow](#recording-a-whole-flow) |\n| `keep`, `--keep` | `all` (default), `failed`, `none` | `all`: the files of every try. `failed`: the files of a try that passed are removed, also its videos and screenshots; a try that failed keeps its own, even if a retry then passes. `none` writes none. `failed.json` and the [results](#results) are always written |\n| `artifacts`, `--artifacts` | `video`, `screenshot`, `logcat`, `screen`, `requests` (default all) | The kinds of file written; `screenshot` is also that of `takeScreenshot` |\n| `nameTemplate`, `--name-template` | `{flow}`, `{attempt}`, `{kind}` and `{ext}` (default `{flow}_{attempt}_{kind}.{ext}`) | How the files are named in `\u003coutput\u003e`. `{attempt}` is empty for the first try, then `retry1`..., `{kind}` is `failure`, `screen`, `logcat`, `requests`, a screenshot's name, and empty for the flow's video; the `_` or `-` around what is empty goes. A `/` makes folders, and a part of the path left without a name takes the try's |\n\nWhat would change nothing is an error, before the run, and `rayo check` says the same of a suite:\n`video` in `artifacts` when no flow is recorded (`video: true`, `record: true` in a flow, or `startRecording`),\n`artifacts` or `nameTemplate` with `keep: none`. So are `--flaky-fails-build` with `--retries 0`,\nand `--hosts` when no flow goes through the proxy.\n\n```\nrayo test --keep failed --artifacts video,screenshot all.suite.yaml   # with output: { video: true } in the suite\nrayo test --name-template '{flow}_{attempt}/{kind}.{ext}' all.suite.yaml   # a folder for each try: pay/failure.jpg, pay/pay.mp4, pay_retry1/...\n```\n\n| Exit code | |\n|---|---|\n| 0 | Everything passed |\n| 1 | A flow failed or didn't run, or an error |\n| 2 | No command given |\n| 130 | Interrupted with Ctrl-C; what was set up on the device is undone |\n\n### Timings\n\n`rayo test --timings` shows where a run is slow or stuck, without the internals of `--profile`:\n\n- before the flows, where the setup went: `setup: pick 3 ms, install and device 0 ms, agent and mocks\n  900 ms; 4 adb calls` (picking the device, installing the app and the suite's device settings,\n  starting the agent and the mocks, and how many times adb was asked so far)\n- every step with its time, and `◂` with the total of each `runFlow`, `repeat` and `retry`\n- `… still on \u003cstep\u003e after 10 s` every 5 s while a step runs, so a hang shows where it is\n- after each flow, its 5 slowest steps; at the end, the 10 slowest flows\n- `default waits ×2` at the start, with a [`--timeout-scale`](#default-times) other than 1\n\n### Profile\n\n`--profile` adds to each step where its time went, in milliseconds:\n\n| | |\n|---|---|\n| `adb+host` | The computer and adb |\n| `read` | Reading the screen; long when the app's main thread is busy |\n| `cache` | Looking in what Android remembers of the screen |\n| `check` | Reading one element again, to confirm it |\n| `wait` | Waiting for the screen to change |\n| `still` | Waiting to see that an element doesn't move |\n| `inject` | Touches and keys, until the app takes them |\n| `shell` | Commands on the device |\n\n### Report\n\nJUnit: `--results report.xml`. What didn't run is `skipped`, what was interrupted is an `error`,\nand a flow that failed first has its failed tries in `system-out`.\n\n### Environment\n\n| Variable | |\n|---|---|\n| `ANDROID_SERIAL` | The device, without `--device` |\n| `ANDROID_HOME`, `ANDROID_SDK_ROOT` | Where to find adb if it isn't in the `PATH`; the usual places are tried too |\n| `ANDROID_ADB_SERVER_PORT` | 5037 by default |\n\n---\n\n## Default times\n\n| | |\n|---|---|\n| Finding an element, `extendedWaitUntil`, `launchApp`, `waitForRequest` | 15 s |\n| `assertNotVisible` | 5 s |\n| `scrollUntilVisible` | 20 s |\n| With `optional: true` | 3 s, except the two above |\n| `when` and `while` | 0.7 s each for `visible` and `notVisible`, or their `timeout` |\n\n`--timeout-scale \u003cfactor\u003e`, or `timeoutScale:` in a suite, multiplies all of these: `2` for a slow CI emulator,\n`0.5` for a fast local one. It takes 0.1 to 10; the option wins over the suite, and the default is 1. A step's own\n`timeout:` is not scaled, and `timeout: 0` stays 0. With `--timings`, a factor other than 1 is shown at the start.\n\n---\n\n## Limitations\n\n- Not supported: `addMedia`, running on several devices at once (`--shard i/n` splits the flows\n  between machines instead), a\n  `config.yaml` (see [suites](#suites)), and selectors by `css`, `width`, `height` or `traits`. An unknown command is an\n  error, never ignored.\n- `setLocation` and `travel` need Android 12 or newer.\n- WebViews show only what they expose to accessibility.\n- What a dialog, sheet or other window covers is not on screen: only what is on top can be found\n  or tapped. Close the dialog first, for example with a `when` guard.\n- Only one tool can automate a device at a time: not while another runs.\n- What is marked 🧪 is **written but not tested on a\n  device yet**, and so are relative selectors and tags.\n",
  "examples/shop/all.suite.yaml": "# What runs together, in this order.\nname: Shop\n\napp:\n  id: com.example.shop\n  apk: app-debug.apk\n\noutput: { folder: results, video: true }\nresults: [results/results.txt, results/junit.xml]\nretries: 1\ndevice: { size: 1080x2400, dpi: 420, fontScale: 1.0, darkMode: false }\n\nflows:\n  # Log in and find a product.\n  - tests/browse/flow.yaml\n  # Buy it, and follow the order until it is shipped.\n  - { file: tests/pay/flow.yaml, retries: 2 }\n  # The bank refuses the card.\n  - tests/declined/flow.yaml\n  # No connection, and back.\n  - tests/offline/flow.yaml\n",
  "examples/shop/mock/__files/products.json": "[\n  { \"id\": 1, \"name\": \"Red bike\", \"price\": 249.0 },\n  { \"id\": 2, \"name\": \"Helmet\", \"price\": 39.0 }\n]\n",
  "examples/shop/mock/mappings/shop.json": "{\n  \"mappings\": [\n    {\n      \"request\": { \"method\": \"POST\", \"urlPath\": \"/login\" },\n      \"response\": { \"status\": 200, \"jsonBody\": { \"token\": \"a-token\" } }\n    },\n    {\n      \"request\": { \"method\": \"GET\", \"urlPath\": \"/products\" },\n      \"response\": { \"status\": 200, \"headers\": { \"Content-Type\": \"application/json\" }, \"bodyFileName\": \"products.json\" }\n    },\n    {\n      \"scenarioName\": \"order\", \"requiredScenarioState\": \"Started\", \"newScenarioState\": \"paid\",\n      \"request\": { \"method\": \"POST\", \"urlPath\": \"/orders\" },\n      \"response\": { \"status\": 201, \"jsonBody\": { \"id\": 7 } }\n    },\n    {\n      \"scenarioName\": \"order\", \"requiredScenarioState\": \"paid\",\n      \"request\": { \"method\": \"GET\", \"urlPath\": \"/orders/7\" },\n      \"response\": { \"status\": 200, \"jsonBody\": { \"id\": 7, \"state\": \"preparing\" } }\n    },\n    {\n      \"scenarioName\": \"order\", \"requiredScenarioState\": \"shipped\",\n      \"request\": { \"method\": \"GET\", \"urlPath\": \"/orders/7\" },\n      \"response\": { \"status\": 200, \"jsonBody\": { \"id\": 7, \"state\": \"shipped\" } }\n    }\n  ]\n}\n",
  "examples/shop/steps/login.yaml": "appId: com.example.shop\nenv: { EMAIL: me@example.com }\n---\n# Begins clean, against the mock, and logs in.\n- launchApp: { clearState: true, permissions: { all: allow }, arguments: { mock: true } }\n- type: { id: login_email, value: \"${EMAIL}\" }\n- type: { id: login_password, value: secret }\n- tapOn: { id: login_button }\n- extendedWaitUntil: { visible: { id: products }, timeout: 20s }\n",
  "examples/shop/tests/browse/flow.yaml": "appId: com.example.shop\nmock: ../../mock\ntags: [smoke]\n---\n- runFlow: ../../steps/login.yaml\n- scrollUntilVisible: { element: { text: Red bike } }\n- tapOn: Red bike\n- assertVisible: { id: price, text: 249.00.* }\n",
  "examples/shop/tests/declined/declined.json": "{\n  \"priority\": 1,\n  \"request\": { \"method\": \"POST\", \"urlPath\": \"/orders\" },\n  \"response\": { \"status\": 402, \"jsonBody\": { \"error\": \"card_declined\" } }\n}\n",
  "examples/shop/tests/declined/flow.yaml": "appId: com.example.shop\nmock: [../../mock, declined.json]     # the normal answers, and the one that differs\n---\n- runFlow: ../../steps/login.yaml\n- tapOn: Red bike\n- tapOn: { id: buy }\n- assertVisible: Your card was declined\n- assertNotVisible: Preparing your order\n",
  "examples/shop/tests/offline/flow.yaml": "appId: com.example.shop\nmock: ../../mock\n---\n- runFlow: ../../steps/login.yaml\n- network: offline\n- tapOn: { id: refresh }\n- assertVisible: You are offline\n- network: online\n- tapOn: Try again\n- assertVisible: Red bike\n",
  "examples/shop/tests/pay/flow.yaml": "appId: com.example.shop\nmock: ../../mock\ntags: [smoke]\n---\n- runFlow: ../../steps/login.yaml\n- tapOn: Red bike\n- tapOn: { id: buy }\n- waitForRequest: { method: POST, path: /orders, body: \"\\\"product\\\":1\" }\n- assertVisible: Preparing your order\n\n# The shop ships it: something the backend does by itself.\n- mockState: { order: shipped }\n- tapOn: { id: refresh }\n- assertVisible: On its way\n",
  "schema/results.json": "{\n  \"$schema\": \"http://json-schema.org/draft-07/schema#\",\n  \"additionalProperties\": false,\n  \"properties\": {\n    \"device\": {\n      \"description\": \"Its serial\",\n      \"type\": \"string\"\n    },\n    \"flows\": {\n      \"description\": \"Those that show says, see below\",\n      \"items\": {\n        \"additionalProperties\": false,\n        \"properties\": {\n          \"at\": {\n            \"description\": \"Where that step is written, file:line\",\n            \"type\": \"string\"\n          },\n          \"attempts\": {\n            \"description\": \"Such as 'passed on retry 2' or 'failed all 3 tries'; empty for one try\",\n            \"type\": \"string\"\n          },\n          \"began\": {\n            \"description\": \"Its first try, in UTC; empty if it didn't run\",\n            \"type\": \"string\"\n          },\n          \"blame\": {\n            \"description\": \"Of a flow that failed, whose fault it is, or empty\",\n            \"enum\": [\n              \"test\",\n              \"app\",\n              \"device\",\n              \"\"\n            ]\n          },\n          \"failures\": {\n            \"description\": \"One for each try that failed, also of a flow that passed at last\",\n            \"items\": {\n              \"additionalProperties\": false,\n              \"properties\": {\n                \"because\": {\n                  \"description\": \"What shows that it isn't the test\",\n                  \"type\": \"string\"\n                },\n                \"before\": {\n                  \"description\": \"The last 3 steps that passed\",\n                  \"items\": {\n                    \"type\": \"string\"\n                  },\n                  \"type\": \"array\"\n                },\n                \"blame\": {\n                  \"description\": \"Whose fault it is: test, app or device\",\n                  \"enum\": [\n                    \"test\",\n                    \"app\",\n                    \"device\"\n                  ]\n                },\n                \"calls\": {\n                  \"description\": \"The last 5 to the mock, and before them those that went wrong in the failure window\",\n                  \"items\": {\n                    \"additionalProperties\": false,\n                    \"properties\": {\n                      \"at\": {\n                        \"description\": \"In UTC\",\n                        \"type\": \"string\"\n                      },\n                      \"body\": {\n                        \"description\": \"Only in a requests.json\",\n                        \"type\": \"string\"\n                      },\n                      \"mapping\": {\n                        \"description\": \"The one that answered: its name, or method and address, and file#n where it is written; empty if none\",\n                        \"type\": \"string\"\n                      },\n                      \"method\": {\n                        \"description\": \"\",\n                        \"type\": \"string\"\n                      },\n                      \"mocked\": {\n                        \"description\": \"Whether a mock fitted\",\n                        \"type\": \"boolean\"\n                      },\n                      \"outcome\": {\n                        \"description\": \"The status, or what happened\",\n                        \"type\": \"string\"\n                      },\n                      \"scenario\": {\n                        \"description\": \"Of a mapping with a scenario, its state before and after, such as 'booking: 20 -\u003e 21'; empty if none\",\n                        \"type\": \"string\"\n                      },\n                      \"uri\": {\n                        \"description\": \"\",\n                        \"type\": \"string\"\n                      }\n                    },\n                    \"required\": [\n                      \"at\",\n                      \"method\",\n                      \"uri\",\n                      \"outcome\",\n                      \"mocked\",\n                      \"mapping\",\n                      \"scenario\",\n                      \"body\"\n                    ],\n                    \"type\": \"object\"\n                  },\n                  \"type\": \"array\"\n                },\n                \"cause\": {\n                  \"description\": \"What went wrong\",\n                  \"enum\": [\n                    \"not on screen\",\n                    \"timeout\",\n                    \"app crashed\",\n                    \"unmocked call\",\n                    \"device error\",\n                    \"step failed\"\n                  ]\n                },\n                \"crash\": {\n                  \"description\": \"The app crashed or hung during the flow: what the device log says\",\n                  \"type\": \"string\"\n                },\n                \"error\": {\n                  \"additionalProperties\": false,\n                  \"description\": \"What went wrong, at which step\",\n                  \"properties\": {\n                    \"expected\": {\n                      \"description\": \"What it takes, if that can be said\",\n                      \"type\": \"string\"\n                    },\n                    \"file\": {\n                      \"description\": \"Where it is written; empty if it isn't of a file\",\n                      \"type\": \"string\"\n                    },\n                    \"fix\": {\n                      \"description\": \"What to write instead, if that can be said\",\n                      \"type\": \"string\"\n                    },\n                    \"line\": {\n                      \"description\": \"0 if not known\",\n                      \"type\": \"integer\"\n                    },\n                    \"message\": {\n                      \"description\": \"What is wrong\",\n                      \"type\": \"string\"\n                    },\n                    \"nearest\": {\n                      \"description\": \"Of that, the closest to what is written\",\n                      \"type\": \"string\"\n                    },\n                    \"step\": {\n                      \"description\": \"Of a flow that failed, the step\",\n                      \"type\": \"string\"\n                    }\n                  },\n                  \"required\": [\n                    \"file\",\n                    \"line\",\n                    \"step\",\n                    \"message\",\n                    \"expected\",\n                    \"nearest\",\n                    \"fix\"\n                  ],\n                  \"type\": \"object\"\n                },\n                \"from\": {\n                  \"description\": \"The step that ran that flow, and so on outwards\",\n                  \"items\": {\n                    \"additionalProperties\": false,\n                    \"properties\": {\n                      \"file\": {\n                        \"description\": \"\",\n                        \"type\": \"string\"\n                      },\n                      \"line\": {\n                        \"description\": \"\",\n                        \"type\": \"integer\"\n                      },\n                      \"step\": {\n                        \"description\": \"\",\n                        \"type\": \"string\"\n                      }\n                    },\n                    \"required\": [\n                      \"step\",\n                      \"file\",\n                      \"line\"\n                    ],\n                    \"type\": \"object\"\n                  },\n                  \"type\": \"array\"\n                },\n                \"nearest\": {\n                  \"description\": \"Of 'not on screen', the elements on it most like what was looked for\",\n                  \"items\": {\n                    \"type\": \"string\"\n                  },\n                  \"type\": \"array\"\n                },\n                \"next\": {\n                  \"description\": \"What to do about it, a line each, by its cause and blame: the same for the same failure, for a program to follow\",\n                  \"items\": {\n                    \"type\": \"string\"\n                  },\n                  \"type\": \"array\"\n                },\n                \"saved\": {\n                  \"description\": \"The first file of the rest, which are named alike\",\n                  \"type\": \"string\"\n                },\n                \"scenarios\": {\n                  \"description\": \"Of the mocks, the state each was in as it failed; empty without scenarios\",\n                  \"items\": {\n                    \"additionalProperties\": false,\n                    \"properties\": {\n                      \"name\": {\n                        \"description\": \"Its scenarioName\",\n                        \"type\": \"string\"\n                      },\n                      \"state\": {\n                        \"description\": \"\",\n                        \"type\": \"string\"\n                      }\n                    },\n                    \"required\": [\n                      \"name\",\n                      \"state\"\n                    ],\n                    \"type\": \"object\"\n                  },\n                  \"type\": \"array\"\n                },\n                \"screen\": {\n                  \"description\": \"The elements on it\",\n                  \"items\": {\n                    \"type\": \"string\"\n                  },\n                  \"type\": \"array\"\n                },\n                \"trail\": {\n                  \"description\": \"The scenarios as each step began, the last 2000 steps: what --resume and --from set\",\n                  \"items\": {\n                    \"additionalProperties\": false,\n                    \"properties\": {\n                      \"at\": {\n                        \"description\": \"The step, file:line\",\n                        \"type\": \"string\"\n                      },\n                      \"scenarios\": {\n                        \"description\": \"\",\n                        \"items\": {\n                          \"additionalProperties\": false,\n                          \"properties\": {\n                            \"name\": {\n                              \"description\": \"Its scenarioName\",\n                              \"type\": \"string\"\n                            },\n                            \"state\": {\n                              \"description\": \"\",\n                              \"type\": \"string\"\n                            }\n                          },\n                          \"required\": [\n                            \"name\",\n                            \"state\"\n                          ],\n                          \"type\": \"object\"\n                        },\n                        \"type\": \"array\"\n                      }\n                    },\n                    \"required\": [\n                      \"at\",\n                      \"scenarios\"\n                    ],\n                    \"type\": \"object\"\n                  },\n                  \"type\": \"array\"\n                },\n                \"try\": {\n                  \"description\": \"From 1\",\n                  \"type\": \"integer\"\n                },\n                \"verdict\": {\n                  \"description\": \"In a line: '\u003cflow\u003e failed at \u003cstep\u003e: \u003ccause\u003e'; of 'step failed', then the first line of why and (\u003cfile\u003e:\u003cline\u003e); 'failed before its first step' or 'failed outside a step' if it isn't of a step; of an interrupted flow, '\u003cflow\u003e interrupted at \u003cstep\u003e (\u003cfile:line\u003e)'\",\n                  \"type\": \"string\"\n                }\n              },\n              \"required\": [\n                \"try\",\n                \"verdict\",\n                \"cause\",\n                \"error\",\n                \"blame\",\n                \"because\",\n                \"from\",\n                \"before\",\n                \"nearest\",\n                \"calls\",\n                \"screen\",\n                \"crash\",\n                \"saved\",\n                \"scenarios\",\n                \"trail\",\n                \"next\"\n              ],\n              \"type\": \"object\"\n            },\n            \"type\": \"array\"\n          },\n          \"file\": {\n            \"description\": \"The flow's\",\n            \"type\": \"string\"\n          },\n          \"mark\": {\n            \"anyOf\": [\n              {\n                \"additionalProperties\": false,\n                \"properties\": {\n                  \"as\": {\n                    \"description\": \"app-bug, test-bug or flaky\",\n                    \"enum\": [\n                      \"app-bug\",\n                      \"test-bug\",\n                      \"flaky\"\n                    ]\n                  },\n                  \"note\": {\n                    \"description\": \"Why, as given\",\n                    \"type\": \"string\"\n                  }\n                },\n                \"required\": [\n                  \"as\",\n                  \"note\"\n                ],\n                \"type\": \"object\"\n              },\n              {\n                \"type\": \"null\"\n              }\n            ],\n            \"description\": \"What was said of it while held, with .mark; null if nothing\"\n          },\n          \"name\": {\n            \"description\": \"The flow's\",\n            \"type\": \"string\"\n          },\n          \"notRunReason\": {\n            \"description\": \"If it is NOT_RUN\",\n            \"type\": \"string\"\n          },\n          \"recordings\": {\n            \"description\": \"Videos\",\n            \"items\": {\n              \"type\": \"string\"\n            },\n            \"type\": \"array\"\n          },\n          \"retries\": {\n            \"description\": \"Those allowed\",\n            \"type\": \"integer\"\n          },\n          \"seconds\": {\n            \"description\": \"All its tries\",\n            \"type\": \"number\"\n          },\n          \"status\": {\n            \"description\": \"One of the outcomes\",\n            \"enum\": [\n              \"PASS\",\n              \"PARTIAL\",\n              \"FLAKY\",\n              \"FAIL\",\n              \"INTERRUPTED\",\n              \"NOT_RUN\"\n            ]\n          },\n          \"step\": {\n            \"description\": \"Of its last try that failed, the step that failed\",\n            \"type\": \"string\"\n          },\n          \"timesFailed\": {\n            \"description\": \"Of those\",\n            \"type\": \"integer\"\n          },\n          \"tries\": {\n            \"description\": \"Those it took\",\n            \"type\": \"integer\"\n          },\n          \"warnings\": {\n            \"description\": \"What may explain this, or the next flow\",\n            \"items\": {\n              \"type\": \"string\"\n            },\n            \"type\": \"array\"\n          },\n          \"why\": {\n            \"description\": \"What went wrong in that try\",\n            \"type\": \"string\"\n          }\n        },\n        \"required\": [\n          \"name\",\n          \"file\",\n          \"status\",\n          \"blame\",\n          \"began\",\n          \"seconds\",\n          \"attempts\",\n          \"step\",\n          \"at\",\n          \"why\",\n          \"tries\",\n          \"timesFailed\",\n          \"retries\",\n          \"notRunReason\",\n          \"warnings\",\n          \"recordings\",\n          \"mark\",\n          \"failures\"\n        ],\n        \"type\": \"object\"\n      },\n      \"type\": \"array\"\n    },\n    \"id\": {\n      \"description\": \"The run's own, as failed.json and history.jsonl give it: when it began, in UTC, then 4 hex digits\",\n      \"type\": \"string\"\n    },\n    \"ok\": {\n      \"description\": \"Nothing failed, was interrupted or didn't run, nor was flaky with flaky-fails-build: as the exit code\",\n      \"type\": \"boolean\"\n    },\n    \"order\": {\n      \"description\": \"--shuffle=\u003cseed\u003e, if shuffled\",\n      \"type\": \"string\"\n    },\n    \"run\": {\n      \"description\": \"The suite's name, or what was run\",\n      \"type\": \"string\"\n    },\n    \"schemaVersion\": {\n      \"description\": \"Changes if the meaning of a key does\",\n      \"type\": \"integer\"\n    },\n    \"shard\": {\n      \"anyOf\": [\n        {\n          \"additionalProperties\": false,\n          \"properties\": {\n            \"index\": {\n              \"description\": \"i, from 1\",\n              \"type\": \"integer\"\n            },\n            \"of\": {\n              \"description\": \"n\",\n              \"type\": \"integer\"\n            }\n          },\n          \"required\": [\n            \"index\",\n            \"of\"\n          ],\n          \"type\": \"object\"\n        },\n        {\n          \"type\": \"null\"\n        }\n      ],\n      \"description\": \"The part of --shard i/n this run was; null if all\"\n    },\n    \"stopped\": {\n      \"description\": \"Why, if it stopped before the end\",\n      \"type\": \"string\"\n    },\n    \"summary\": {\n      \"additionalProperties\": false,\n      \"description\": \"The run at a glance, see below\",\n      \"properties\": {\n        \"began\": {\n          \"description\": \"In UTC\",\n          \"type\": \"string\"\n        },\n        \"exit\": {\n          \"additionalProperties\": false,\n          \"description\": \"The exit code of the run\",\n          \"properties\": {\n            \"code\": {\n              \"description\": \"\",\n              \"type\": \"integer\"\n            },\n            \"name\": {\n              \"description\": \"Such as failed\",\n              \"type\": \"string\"\n            }\n          },\n          \"required\": [\n            \"code\",\n            \"name\"\n          ],\n          \"type\": \"object\"\n        },\n        \"failed\": {\n          \"description\": \"\",\n          \"type\": \"integer\"\n        },\n        \"failedFlows\": {\n          \"description\": \"The names of those that failed, were interrupted or didn't run\",\n          \"items\": {\n            \"type\": \"string\"\n          },\n          \"type\": \"array\"\n        },\n        \"flaky\": {\n          \"description\": \"Failed, then passed on a retry\",\n          \"type\": \"integer\"\n        },\n        \"flakyFlows\": {\n          \"description\": \"The names of those that were flaky\",\n          \"items\": {\n            \"type\": \"string\"\n          },\n          \"type\": \"array\"\n        },\n        \"interrupted\": {\n          \"description\": \"\",\n          \"type\": \"integer\"\n        },\n        \"notRun\": {\n          \"description\": \"\",\n          \"type\": \"integer\"\n        },\n        \"partial\": {\n          \"description\": \"Passed, but began or stopped at a step (--resume, --from, --until, or fixed while held): not a pass of the whole flow\",\n          \"type\": \"integer\"\n        },\n        \"passed\": {\n          \"description\": \"At the first try\",\n          \"type\": \"integer\"\n        },\n        \"retried\": {\n          \"description\": \"Flows tried more than once, whether they passed or not\",\n          \"type\": \"integer\"\n        },\n        \"seconds\": {\n          \"description\": \"All of it\",\n          \"type\": \"number\"\n        },\n        \"slowest\": {\n          \"description\": \"The flows that took longest, as many as slowest says\",\n          \"items\": {\n            \"additionalProperties\": false,\n            \"properties\": {\n              \"name\": {\n                \"description\": \"\",\n                \"type\": \"string\"\n              },\n              \"seconds\": {\n                \"description\": \"\",\n                \"type\": \"number\"\n              }\n            },\n            \"required\": [\n              \"name\",\n              \"seconds\"\n            ],\n            \"type\": \"object\"\n          },\n          \"type\": \"array\"\n        },\n        \"total\": {\n          \"description\": \"Flows\",\n          \"type\": \"integer\"\n        }\n      },\n      \"required\": [\n        \"began\",\n        \"seconds\",\n        \"total\",\n        \"passed\",\n        \"flaky\",\n        \"partial\",\n        \"failed\",\n        \"interrupted\",\n        \"notRun\",\n        \"retried\",\n        \"failedFlows\",\n        \"flakyFlows\",\n        \"slowest\",\n        \"exit\"\n      ],\n      \"type\": \"object\"\n    }\n  },\n  \"required\": [\n    \"schemaVersion\",\n    \"run\",\n    \"id\",\n    \"device\",\n    \"order\",\n    \"shard\",\n    \"stopped\",\n    \"ok\",\n    \"summary\",\n    \"flows\"\n  ],\n  \"title\": \"Results of rayo\",\n  \"type\": \"object\"\n}\n",
  "schema/suite.json": "{\n  \"$schema\": \"http://json-schema.org/draft-07/schema#\",\n  \"additionalProperties\": false,\n  \"properties\": {\n    \"app\": {\n      \"additionalProperties\": false,\n      \"description\": \"Installed before the flows\",\n      \"properties\": {\n        \"apk\": {\n          \"description\": \"What to install, if the device has another. rayo doesn't build it (required)\",\n          \"type\": [\n            \"string\",\n            \"number\",\n            \"boolean\"\n          ]\n        },\n        \"id\": {\n          \"description\": \"The app's (required)\",\n          \"type\": [\n            \"string\",\n            \"number\",\n            \"boolean\"\n          ]\n        }\n      },\n      \"required\": [\n        \"id\",\n        \"apk\"\n      ],\n      \"type\": \"object\"\n    },\n    \"ca\": {\n      \"description\": \"--ca overrides it: the CA the proxy signs with, its ca.pem and ca-key.pem, relative to the suite; rayo ca new makes one (default: .rayo/ca next to the suite or above it, up to the repository's root; else ~/.rayo)\",\n      \"type\": \"string\"\n    },\n    \"capture\": {\n      \"additionalProperties\": false,\n      \"description\": \"How a capture calls the backend and becomes mocks, as the options of the same names, which win\",\n      \"properties\": {\n        \"captureHeaders\": {\n          \"anyOf\": [\n            {\n              \"type\": \"string\"\n            },\n            {\n              \"items\": {\n                \"type\": \"string\"\n              },\n              \"type\": \"array\"\n            }\n          ],\n          \"description\": \"As --capture-headers\"\n        },\n        \"from\": {\n          \"description\": \"As --capture-from\",\n          \"type\": [\n            \"string\",\n            \"number\",\n            \"boolean\"\n          ]\n        },\n        \"graphql\": {\n          \"description\": \"As --graphql\",\n          \"enum\": [\n            \"operation+variables\",\n            \"operation\",\n            \"off\"\n          ]\n        },\n        \"graphqlPath\": {\n          \"description\": \"As --graphql-path\",\n          \"type\": [\n            \"string\",\n            \"number\",\n            \"boolean\"\n          ]\n        },\n        \"hosts\": {\n          \"anyOf\": [\n            {\n              \"type\": \"string\"\n            },\n            {\n              \"items\": {\n                \"type\": \"string\"\n              },\n              \"type\": \"array\"\n            }\n          ],\n          \"description\": \"As --hosts\"\n        },\n        \"ignoreBody\": {\n          \"anyOf\": [\n            {\n              \"type\": \"string\"\n            },\n            {\n              \"items\": {\n                \"type\": \"string\"\n              },\n              \"type\": \"array\"\n            }\n          ],\n          \"description\": \"As --ignore-body\"\n        },\n        \"ignoreHost\": {\n          \"description\": \"As --ignore-host\",\n          \"type\": \"boolean\"\n        },\n        \"ignoreParams\": {\n          \"anyOf\": [\n            {\n              \"type\": \"string\"\n            },\n            {\n              \"items\": {\n                \"type\": \"string\"\n              },\n              \"type\": \"array\"\n            }\n          ],\n          \"description\": \"As --ignore-params\"\n        },\n        \"ignorePathIds\": {\n          \"description\": \"As --ignore-path-ids\",\n          \"type\": \"boolean\"\n        },\n        \"redactValue\": {\n          \"additionalProperties\": {\n            \"type\": [\n              \"string\",\n              \"number\",\n              \"boolean\"\n            ]\n          },\n          \"description\": \"As --redact-value: what a private field of a body becomes, such as phone: \\\"+4915100000000\\\"\",\n          \"type\": \"object\"\n        },\n        \"rest\": {\n          \"description\": \"As --rest\",\n          \"enum\": [\n            \"body\",\n            \"off\"\n          ]\n        },\n        \"stateless\": {\n          \"anyOf\": [\n            {\n              \"type\": \"string\"\n            },\n            {\n              \"items\": {\n                \"type\": \"string\"\n              },\n              \"type\": \"array\"\n            }\n          ],\n          \"description\": \"As --stateless\"\n        },\n        \"timestamps\": {\n          \"description\": \"As --timestamps\",\n          \"enum\": [\n            \"format\",\n            \"exact\"\n          ]\n        }\n      },\n      \"type\": \"object\"\n    },\n    \"detail\": {\n      \"description\": \"--detail overrides it: how much is printed (default: normal)\",\n      \"enum\": [\n        \"brief\",\n        \"normal\",\n        \"full\"\n      ]\n    },\n    \"device\": {\n      \"additionalProperties\": false,\n      \"description\": \"How it is while the flows run\",\n      \"properties\": {\n        \"darkMode\": {\n          \"description\": \"The dark theme\",\n          \"type\": \"boolean\"\n        },\n        \"dpi\": {\n          \"description\": \"Its density. With size, it says how many dp there are: 1080 pixels at 420 dpi are 411 dp\",\n          \"maximum\": 1000,\n          \"minimum\": 72,\n          \"type\": \"integer\"\n        },\n        \"fontScale\": {\n          \"description\": \"Of the text; 1 is normal\",\n          \"maximum\": 3,\n          \"minimum\": 0.5,\n          \"type\": \"number\"\n        },\n        \"size\": {\n          \"description\": \"Of the screen\",\n          \"pattern\": \"^[1-9]\\\\d{2,3}x[1-9]\\\\d{2,3}$\",\n          \"type\": \"string\"\n        }\n      },\n      \"type\": \"object\"\n    },\n    \"env\": {\n      \"additionalProperties\": {\n        \"type\": [\n          \"string\",\n          \"number\",\n          \"boolean\"\n        ]\n      },\n      \"description\": \"Variables for every flow, over a flow's own env; -e overrides one; they may use ${...}, such as ${rayo.file}\",\n      \"type\": \"object\"\n    },\n    \"failureWindow\": {\n      \"description\": \"--failure-window overrides it: a failure's log and calls begin this long before the failing step (default: 10)\",\n      \"minimum\": 1,\n      \"type\": \"integer\"\n    },\n    \"flakyFailsBuild\": {\n      \"description\": \"--flaky-fails-build too: a run where every flow passed, some only on a retry, exits 4 (default: false: it exits 0)\",\n      \"type\": \"boolean\"\n    },\n    \"flows\": {\n      \"description\": \"Files, run in this order; or mappings with file, retries, mock, network, unmocked and capture (required)\",\n      \"items\": {\n        \"anyOf\": [\n          {\n            \"type\": \"string\"\n          },\n          {\n            \"additionalProperties\": false,\n            \"properties\": {\n              \"capture\": {\n                \"additionalProperties\": false,\n                \"description\": \"How a capture calls the backend and becomes mocks, as the options of the same names, which win (default: the suite's; what it says wins over it)\",\n                \"properties\": {\n                  \"captureHeaders\": {\n                    \"anyOf\": [\n                      {\n                        \"type\": \"string\"\n                      },\n                      {\n                        \"items\": {\n                          \"type\": \"string\"\n                        },\n                        \"type\": \"array\"\n                      }\n                    ],\n                    \"description\": \"As --capture-headers\"\n                  },\n                  \"from\": {\n                    \"description\": \"As --capture-from\",\n                    \"type\": [\n                      \"string\",\n                      \"number\",\n                      \"boolean\"\n                    ]\n                  },\n                  \"graphql\": {\n                    \"description\": \"As --graphql\",\n                    \"enum\": [\n                      \"operation+variables\",\n                      \"operation\",\n                      \"off\"\n                    ]\n                  },\n                  \"graphqlPath\": {\n                    \"description\": \"As --graphql-path\",\n                    \"type\": [\n                      \"string\",\n                      \"number\",\n                      \"boolean\"\n                    ]\n                  },\n                  \"hosts\": {\n                    \"anyOf\": [\n                      {\n                        \"type\": \"string\"\n                      },\n                      {\n                        \"items\": {\n                          \"type\": \"string\"\n                        },\n                        \"type\": \"array\"\n                      }\n                    ],\n                    \"description\": \"As --hosts\"\n                  },\n                  \"ignoreBody\": {\n                    \"anyOf\": [\n                      {\n                        \"type\": \"string\"\n                      },\n                      {\n                        \"items\": {\n                          \"type\": \"string\"\n                        },\n                        \"type\": \"array\"\n                      }\n                    ],\n                    \"description\": \"As --ignore-body\"\n                  },\n                  \"ignoreHost\": {\n                    \"description\": \"As --ignore-host\",\n                    \"type\": \"boolean\"\n                  },\n                  \"ignoreParams\": {\n                    \"anyOf\": [\n                      {\n                        \"type\": \"string\"\n                      },\n                      {\n                        \"items\": {\n                          \"type\": \"string\"\n                        },\n                        \"type\": \"array\"\n                      }\n                    ],\n                    \"description\": \"As --ignore-params\"\n                  },\n                  \"ignorePathIds\": {\n                    \"description\": \"As --ignore-path-ids\",\n                    \"type\": \"boolean\"\n                  },\n                  \"redactValue\": {\n                    \"additionalProperties\": {\n                      \"type\": [\n                        \"string\",\n                        \"number\",\n                        \"boolean\"\n                      ]\n                    },\n                    \"description\": \"As --redact-value: what a private field of a body becomes, such as phone: \\\"+4915100000000\\\"\",\n                    \"type\": \"object\"\n                  },\n                  \"rest\": {\n                    \"description\": \"As --rest\",\n                    \"enum\": [\n                      \"body\",\n                      \"off\"\n                    ]\n                  },\n                  \"stateless\": {\n                    \"anyOf\": [\n                      {\n                        \"type\": \"string\"\n                      },\n                      {\n                        \"items\": {\n                          \"type\": \"string\"\n                        },\n                        \"type\": \"array\"\n                      }\n                    ],\n                    \"description\": \"As --stateless\"\n                  },\n                  \"timestamps\": {\n                    \"description\": \"As --timestamps\",\n                    \"enum\": [\n                      \"format\",\n                      \"exact\"\n                    ]\n                  }\n                },\n                \"type\": \"object\"\n              },\n              \"file\": {\n                \"description\": \"The flow (required)\",\n                \"type\": [\n                  \"string\",\n                  \"number\",\n                  \"boolean\"\n                ]\n              },\n              \"mock\": {\n                \"anyOf\": [\n                  {\n                    \"type\": \"string\"\n                  },\n                  {\n                    \"items\": {\n                      \"type\": \"string\"\n                    },\n                    \"type\": \"array\"\n                  }\n                ],\n                \"description\": \"Who answers as the backend, as a flow's mock, relative to the suite; the flow's own mock wins\"\n              },\n              \"network\": {\n                \"description\": \"How the app's calls reach the mock, as a flow's network; the flow's own and --network override it (default: the suite's)\",\n                \"enum\": [\n                  \"interceptor\",\n                  \"proxy\"\n                ]\n              },\n              \"retries\": {\n                \"description\": \"How many times it is tried again if it fails (default: the suite's)\",\n                \"minimum\": 0,\n                \"type\": \"integer\"\n              },\n              \"unmocked\": {\n                \"additionalProperties\": false,\n                \"description\": \"Calls without a mock that don't fail the flow (default: the suite's; its calls are added to them)\",\n                \"properties\": {\n                  \"body\": {\n                    \"description\": \"Of the answer (default: not mocked:  and the call)\",\n                    \"type\": [\n                      \"string\",\n                      \"number\",\n                      \"boolean\"\n                    ]\n                  },\n                  \"ignore\": {\n                    \"description\": \"Calls that no mock answers but that don't fail the flow: each graphql:Operation, host/path, host or /path, as --ignore-unmocked, or a mapping with host, path and graphql (required)\",\n                    \"items\": {\n                      \"anyOf\": [\n                        {\n                          \"type\": \"string\"\n                        },\n                        {\n                          \"additionalProperties\": false,\n                          \"properties\": {\n                            \"graphql\": {\n                              \"anyOf\": [\n                                {\n                                  \"type\": \"string\"\n                                },\n                                {\n                                  \"items\": {\n                                    \"type\": \"string\"\n                                  },\n                                  \"type\": \"array\"\n                                }\n                              ],\n                              \"description\": \"GraphQL operations (default: any call)\"\n                            },\n                            \"host\": {\n                              \"description\": \"Of the call, or of its subdomains (default: any)\",\n                              \"type\": [\n                                \"string\",\n                                \"number\",\n                                \"boolean\"\n                              ]\n                            },\n                            \"path\": {\n                              \"description\": \"Of the call, without the host; * is any text but /, ** any (default: any)\",\n                              \"type\": [\n                                \"string\",\n                                \"number\",\n                                \"boolean\"\n                              ]\n                            }\n                          },\n                          \"type\": \"object\"\n                        }\n                      ]\n                    },\n                    \"type\": \"array\"\n                  },\n                  \"status\": {\n                    \"description\": \"What those calls are answered with (default: 404)\",\n                    \"maximum\": 599,\n                    \"minimum\": 100,\n                    \"type\": \"integer\"\n                  }\n                },\n                \"required\": [\n                  \"ignore\"\n                ],\n                \"type\": \"object\"\n              }\n            },\n            \"required\": [\n              \"file\"\n            ],\n            \"type\": \"object\"\n          }\n        ]\n      },\n      \"minItems\": 1,\n      \"type\": \"array\"\n    },\n    \"format\": {\n      \"description\": \"What is printed; --format overrides it (default: human)\",\n      \"enum\": [\n        \"human\",\n        \"json\",\n        \"ndjson\"\n      ]\n    },\n    \"minFlows\": {\n      \"description\": \"--min-flows overrides it: fewer flows chosen, by tags, names or what changed, is an error (exit 2), not a green run of nothing (default: none)\",\n      \"minimum\": 1,\n      \"type\": \"integer\"\n    },\n    \"name\": {\n      \"description\": \"Shown in the results (default: the file)\",\n      \"type\": [\n        \"string\",\n        \"number\",\n        \"boolean\"\n      ]\n    },\n    \"network\": {\n      \"description\": \"How the app's calls reach the mock, as a flow's network; the flow's own and --network override it (default: each flow's, else interceptor)\",\n      \"enum\": [\n        \"interceptor\",\n        \"proxy\"\n      ]\n    },\n    \"output\": {\n      \"anyOf\": [\n        {\n          \"type\": \"string\"\n        },\n        {\n          \"additionalProperties\": false,\n          \"properties\": {\n            \"artifacts\": {\n              \"anyOf\": [\n                {\n                  \"enum\": [\n                    \"video\",\n                    \"screenshot\",\n                    \"logcat\",\n                    \"screen\",\n                    \"requests\"\n                  ]\n                },\n                {\n                  \"items\": {\n                    \"enum\": [\n                      \"video\",\n                      \"screenshot\",\n                      \"logcat\",\n                      \"screen\",\n                      \"requests\"\n                    ]\n                  },\n                  \"type\": \"array\"\n                }\n              ],\n              \"description\": \"The kinds of file written, of video, screenshot, logcat, screen, requests; --artifacts overrides it (default: all)\"\n            },\n            \"folder\": {\n              \"description\": \"Where everything goes; --output overrides it (default: .rayo where rayo runs)\",\n              \"type\": [\n                \"string\",\n                \"number\",\n                \"boolean\"\n              ]\n            },\n            \"keep\": {\n              \"description\": \"The files of every try (all), only of the tries that failed (failed), or none; --keep overrides it (default: all)\",\n              \"enum\": [\n                \"all\",\n                \"failed\",\n                \"none\"\n              ]\n            },\n            \"nameTemplate\": {\n              \"description\": \"How the files are named in the folder; --name-template overrides it (default: {flow}_{attempt}_{kind}.{ext})\",\n              \"type\": \"string\"\n            },\n            \"screensBefore\": {\n              \"description\": \"The screen after each of the last steps that passed, saved with a failure as screens-before.txt: how the app got there, such as a dialog that came and went; each costs a read of the screen. --screens-before overrides it (default: 0)\",\n              \"maximum\": 20,\n              \"minimum\": 0,\n              \"type\": \"integer\"\n            },\n            \"video\": {\n              \"description\": \"A video of each flow whose header doesn't say record; --video overrides it (default: false)\",\n              \"type\": \"boolean\"\n            }\n          },\n          \"type\": \"object\"\n        }\n      ],\n      \"description\": \"What a run writes, and where: a folder, or folder, keep, video, artifacts and nameTemplate (default: .rayo where rayo runs)\"\n    },\n    \"results\": {\n      \"anyOf\": [\n        {\n          \"pattern\": \"\\\\.(txt|json|xml|md)$\",\n          \"type\": \"string\"\n        },\n        {\n          \"items\": {\n            \"pattern\": \"\\\\.(txt|json|xml|md)$\",\n            \"type\": \"string\"\n          },\n          \"type\": \"array\"\n        }\n      ],\n      \"description\": \"--results replaces them all: .txt, .json, .xml (JUnit) or .md\"\n    },\n    \"resultsTemplate\": {\n      \"description\": \"--results-template overrides it: the results with its extension are written as it says\",\n      \"type\": \"string\"\n    },\n    \"retries\": {\n      \"description\": \"How many times a flow that fails is tried again (default: 1)\",\n      \"minimum\": 0,\n      \"type\": \"integer\"\n    },\n    \"show\": {\n      \"anyOf\": [\n        {\n          \"enum\": [\n            \"failed\",\n            \"flaky\",\n            \"slow\",\n            \"all\"\n          ]\n        },\n        {\n          \"items\": {\n            \"enum\": [\n              \"failed\",\n              \"flaky\",\n              \"slow\",\n              \"all\"\n            ]\n          },\n          \"type\": \"array\"\n        }\n      ],\n      \"description\": \"--show overrides it: the flows the results list, of failed, flaky, slow, all (default: all)\"\n    },\n    \"slowest\": {\n      \"description\": \"--slowest overrides it: how many flows the slow list has (default: 5)\",\n      \"minimum\": 1,\n      \"type\": \"integer\"\n    },\n    \"timeoutScale\": {\n      \"description\": \"Multiplies the default waits; --timeout-scale overrides it (default: 1)\",\n      \"maximum\": 10,\n      \"minimum\": 0.1,\n      \"type\": \"number\"\n    },\n    \"unmocked\": {\n      \"additionalProperties\": false,\n      \"description\": \"Calls without a mock that don't fail the flow (default: none: each fails the flow)\",\n      \"properties\": {\n        \"body\": {\n          \"description\": \"Of the answer (default: not mocked:  and the call)\",\n          \"type\": [\n            \"string\",\n            \"number\",\n            \"boolean\"\n          ]\n        },\n        \"ignore\": {\n          \"description\": \"Calls that no mock answers but that don't fail the flow: each graphql:Operation, host/path, host or /path, as --ignore-unmocked, or a mapping with host, path and graphql (required)\",\n          \"items\": {\n            \"anyOf\": [\n              {\n                \"type\": \"string\"\n              },\n              {\n                \"additionalProperties\": false,\n                \"properties\": {\n                  \"graphql\": {\n                    \"anyOf\": [\n                      {\n                        \"type\": \"string\"\n                      },\n                      {\n                        \"items\": {\n                          \"type\": \"string\"\n                        },\n                        \"type\": \"array\"\n                      }\n                    ],\n                    \"description\": \"GraphQL operations (default: any call)\"\n                  },\n                  \"host\": {\n                    \"description\": \"Of the call, or of its subdomains (default: any)\",\n                    \"type\": [\n                      \"string\",\n                      \"number\",\n                      \"boolean\"\n                    ]\n                  },\n                  \"path\": {\n                    \"description\": \"Of the call, without the host; * is any text but /, ** any (default: any)\",\n                    \"type\": [\n                      \"string\",\n                      \"number\",\n                      \"boolean\"\n                    ]\n                  }\n                },\n                \"type\": \"object\"\n              }\n            ]\n          },\n          \"type\": \"array\"\n        },\n        \"status\": {\n          \"description\": \"What those calls are answered with (default: 404)\",\n          \"maximum\": 599,\n          \"minimum\": 100,\n          \"type\": \"integer\"\n        }\n      },\n      \"required\": [\n        \"ignore\"\n      ],\n      \"type\": \"object\"\n    }\n  },\n  \"required\": [\n    \"flows\"\n  ],\n  \"title\": \"A rayo suite, *.suite.yaml\",\n  \"type\": \"object\"\n}\n"
 },
 "rayo": {
  "app": [
   {
    "name": "id",
    "takes": "package",
    "about": "The app's (required)",
    "required": true
   },
   {
    "name": "apk",
    "takes": "file",
    "about": "What to install, if the device has another. rayo doesn't build it (required)",
    "required": true
   }
  ],
  "capture": [
   {
    "name": "hosts",
    "takes": "a name or a list",
    "about": "As --hosts",
    "required": false
   },
   {
    "name": "from",
    "takes": "url",
    "about": "As --capture-from",
    "required": false
   },
   {
    "name": "graphql",
    "takes": "operation+variables, operation, off",
    "about": "As --graphql",
    "required": false
   },
   {
    "name": "graphqlPath",
    "takes": "regex",
    "about": "As --graphql-path",
    "required": false
   },
   {
    "name": "rest",
    "takes": "body, off",
    "about": "As --rest",
    "required": false
   },
   {
    "name": "ignoreParams",
    "takes": "a name or a list",
    "about": "As --ignore-params",
    "required": false
   },
   {
    "name": "ignoreBody",
    "takes": "a name or a list",
    "about": "As --ignore-body",
    "required": false
   },
   {
    "name": "timestamps",
    "takes": "format, exact",
    "about": "As --timestamps",
    "required": false
   },
   {
    "name": "captureHeaders",
    "takes": "a name or a list",
    "about": "As --capture-headers",
    "required": false
   },
   {
    "name": "ignoreHost",
    "takes": "true or false",
    "about": "As --ignore-host",
    "required": false
   },
   {
    "name": "ignorePathIds",
    "takes": "true or false",
    "about": "As --ignore-path-ids",
    "required": false
   },
   {
    "name": "stateless",
    "takes": "a path or a list",
    "about": "As --stateless",
    "required": false
   },
   {
    "name": "redactValue",
    "takes": "values",
    "about": "As --redact-value: what a private field of a body becomes, such as phone: \"+4915100000000\"",
    "required": false
   }
  ],
  "device": [
   {
    "name": "size",
    "takes": "pixels, such as 1080x2400",
    "about": "Of the screen",
    "required": false
   },
   {
    "name": "dpi",
    "takes": "72 to 1000",
    "about": "Its density. With size, it says how many dp there are: 1080 pixels at 420 dpi are 411 dp",
    "required": false
   },
   {
    "name": "fontScale",
    "takes": "0.5 to 3",
    "about": "Of the text; 1 is normal",
    "required": false
   },
   {
    "name": "darkMode",
    "takes": "true or false",
    "about": "The dark theme",
    "required": false
   }
  ],
  "exitCodes": [
   {
    "code": 0,
    "name": "passed",
    "what": "Every flow passed, or the command did what it was asked"
   },
   {
    "code": 1,
    "name": "failed",
    "what": "A flow failed, was interrupted or didn't run; or a step sent with `rayo do` failed"
   },
   {
    "code": 2,
    "name": "configuration",
    "what": "Something written is wrong: an option, a flow, a suite or a mock; nothing ran"
   },
   {
    "code": 3,
    "name": "device",
    "what": "No device to run on, or it was lost or unusable, so the flows couldn't run or didn't all run"
   },
   {
    "code": 4,
    "name": "flaky",
    "what": "Every flow passed, some only on a retry, and `flaky-fails-build` is `true`; else such a run exits `0`"
   },
   {
    "code": 130,
    "name": "interrupted",
    "what": "Ctrl-C, or SIGTERM as CI sends when it cancels a job: what ran is in the results, each interrupted flow with the step it was at"
   }
  ],
  "flow": {
   "commands": [
    {
     "name": "assertNotVisible",
     "about": "The element isn't on screen, or goes away in time",
     "example": "- assertNotVisible: Loading.*\n- assertNotVisible: { id: spinner, timeout: 10s }",
     "tested": false,
     "alone": false,
     "element": true,
     "keys": []
    },
    {
     "name": "assertRequest",
     "about": "The app made a call exactly so many times by now, without waiting: times: 0 for none; waitForRequest first for one still on its way",
     "example": "- waitForRequest: { method: POST, path: \".*/payments\" }\n- assertRequest: { method: POST, path: \".*/payments\", times: 1 }\n- assertRequest: { path: \".*/analytics\", times: 0 }",
     "tested": true,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "method",
       "takes": "text",
       "about": "Such as POST (GET)",
       "values": [],
       "short": false
      },
      {
       "name": "path",
       "takes": "pattern",
       "about": "Only these calls, a regular expression of the whole path",
       "values": [],
       "short": false
      },
      {
       "name": "body",
       "takes": "text",
       "about": "A text in the body",
       "values": [],
       "short": false
      },
      {
       "name": "times",
       "takes": "number",
       "about": "Exactly so many calls by now; 0 for none",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "assertTrue",
     "about": "An expression is true",
     "example": "- evalScript: ${output.total = 3}\n- assertTrue: ${output.total == 3}",
     "tested": false,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "condition",
       "takes": "JavaScript",
       "about": "An expression",
       "values": [],
       "short": true
      }
     ]
    },
    {
     "name": "assertVisible",
     "about": "The element is on screen, or appears in time",
     "example": "- assertVisible: \"Welcome.*\"\n- assertVisible: { id: price, text: \"Price (€)\" }",
     "tested": false,
     "alone": false,
     "element": true,
     "keys": []
    },
    {
     "name": "back",
     "about": "Presses Back",
     "example": "- back",
     "tested": true,
     "alone": true,
     "element": false,
     "keys": []
    },
    {
     "name": "camera",
     "about": "What an emulator's camera sees; does nothing on other devices",
     "example": "- camera: qr.png      # what the emulator's camera sees\n- camera: none        # the room as it was",
     "tested": true,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "image",
       "takes": "text",
       "about": "What it sees, a path relative to this flow; or none",
       "values": [],
       "short": true
      }
     ]
    },
    {
     "name": "clearState",
     "about": "Erases the app's data",
     "example": "- clearState\n- clearState: com.example.other",
     "tested": false,
     "alone": true,
     "element": false,
     "keys": [
      {
       "name": "appId",
       "takes": "text",
       "about": "The app; by default the header's",
       "values": [],
       "short": true
      }
     ]
    },
    {
     "name": "copyTextFrom",
     "about": "Keeps an element's text in rayo.copiedText, and in output.\u003cname\u003e if output says one",
     "example": "- copyTextFrom: { id: order_number }\n- tapOn: { id: search }\n- pasteText\n- copyTextFrom: { id: total, output: total }\n- assertTrue: ${output.total == '5 €'}",
     "tested": false,
     "alone": false,
     "element": true,
     "keys": [
      {
       "name": "output",
       "takes": "name",
       "about": "Also keeps it in output.\u003cthis name\u003e, which the next copy doesn't replace",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "doubleTapOn",
     "about": "Like tapOn, twice",
     "example": "- doubleTapOn: { id: photo }",
     "tested": false,
     "alone": false,
     "element": true,
     "keys": [
      {
       "name": "point",
       "takes": "point",
       "about": "Alone, of the screen; with a selector, of the element in %",
       "values": [],
       "short": false
      },
      {
       "name": "delay",
       "takes": "time",
       "about": "Between taps (100)",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "eraseText",
     "about": "Deletes from the end of the focused field",
     "example": "- tapOn: { id: email }\n- eraseText           # 50 characters\n- eraseText: 10",
     "tested": false,
     "alone": true,
     "element": false,
     "keys": [
      {
       "name": "charactersToErase",
       "takes": "number",
       "about": "How many (50)",
       "values": [],
       "short": true
      }
     ]
    },
    {
     "name": "evalScript",
     "about": "Runs a line of JavaScript",
     "example": "- evalScript: ${output.count = 1}",
     "tested": false,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "script",
       "takes": "JavaScript",
       "about": "A line of it",
       "values": [],
       "short": true
      }
     ]
    },
    {
     "name": "extendedWaitUntil",
     "about": "Waits for an element to appear or go away",
     "example": "- extendedWaitUntil: { visible: { id: home }, timeout: 20s }\n- extendedWaitUntil: { notVisible: Loading.* }",
     "tested": true,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "visible",
       "takes": "selector",
       "about": "On screen",
       "values": [],
       "short": false
      },
      {
       "name": "notVisible",
       "takes": "selector",
       "about": "Not on screen",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "hideKeyboard",
     "about": "Closes the keyboard",
     "example": "- hideKeyboard",
     "tested": false,
     "alone": true,
     "element": false,
     "keys": []
    },
    {
     "name": "http",
     "about": "Calls a server from the computer",
     "example": "- http: https://api.example.com/reset\n- http: { method: PUT, url: \"https://api.example.com/state\", json: { state: ready }, expect: 204 }",
     "tested": false,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "method",
       "takes": "text",
       "about": "Such as POST (GET)",
       "values": [],
       "short": false
      },
      {
       "name": "url",
       "takes": "text",
       "about": "What to call, from the computer",
       "values": [],
       "short": true
      },
      {
       "name": "json",
       "takes": "values",
       "about": "The body",
       "values": [],
       "short": false
      },
      {
       "name": "expect",
       "takes": "number",
       "about": "The status (any below 300)",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "inputRandomEmail",
     "about": "Adds a random email",
     "example": "- tapOn: { id: email }\n- inputRandomEmail",
     "tested": false,
     "alone": true,
     "element": false,
     "keys": []
    },
    {
     "name": "inputRandomNumber",
     "about": "Adds a random number",
     "example": "- inputRandomNumber\n- inputRandomNumber: { length: 4 }",
     "tested": false,
     "alone": true,
     "element": false,
     "keys": [
      {
       "name": "length",
       "takes": "number",
       "about": "How long (8)",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "inputRandomPersonName",
     "about": "Adds a random name",
     "example": "- inputRandomPersonName",
     "tested": false,
     "alone": true,
     "element": false,
     "keys": []
    },
    {
     "name": "inputRandomText",
     "about": "Adds random letters",
     "example": "- inputRandomText: { length: 20 }",
     "tested": false,
     "alone": true,
     "element": false,
     "keys": [
      {
       "name": "length",
       "takes": "number",
       "about": "How long (8)",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "inputText",
     "about": "Adds text to the focused field",
     "example": "- tapOn: { id: email }\n- inputText: me@example.com\n- inputText: ${EMAIL}",
     "tested": true,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "text",
       "takes": "text",
       "about": "The text to add",
       "values": [],
       "short": true
      }
     ]
    },
    {
     "name": "killApp",
     "about": "Closes the app",
     "example": "- killApp",
     "tested": false,
     "alone": true,
     "element": false,
     "keys": [
      {
       "name": "appId",
       "takes": "text",
       "about": "The app; by default the header's",
       "values": [],
       "short": true
      }
     ]
    },
    {
     "name": "launchApp",
     "about": "Opens the app and waits until Android has it in front",
     "example": "- launchApp\n- launchApp: { clearState: true, permissions: { all: allow }, arguments: { mock: true } }\n- launchApp: { stopApp: false }     # back as it was",
     "tested": true,
     "alone": true,
     "element": false,
     "keys": [
      {
       "name": "appId",
       "takes": "text",
       "about": "The app; by default the header's",
       "values": [],
       "short": true
      },
      {
       "name": "clearState",
       "takes": "true/false",
       "about": "Erase its data first (false)",
       "values": [
        "true",
        "false"
       ],
       "short": false
      },
      {
       "name": "stopApp",
       "takes": "true/false",
       "about": "Close it first (true); false brings it back as it was",
       "values": [
        "true",
        "false"
       ],
       "short": false
      },
      {
       "name": "permissions",
       "takes": "permissions",
       "about": "Name: allow, deny or unset. With clearState, all are allowed if not said",
       "values": [],
       "short": false
      },
      {
       "name": "arguments",
       "takes": "values",
       "about": "Intent extras; booleans, integers and decimals keep their type",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "longPressOn",
     "about": "Like tapOn, held for a second",
     "example": "- longPressOn: { id: message }",
     "tested": false,
     "alone": false,
     "element": true,
     "keys": [
      {
       "name": "point",
       "takes": "point",
       "about": "Alone, of the screen; with a selector, of the element in %",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "mock",
     "about": "Adds mocks: a path, or one written here",
     "example": "- mock: overrides.json\n- mock: { request: { method: POST, urlPathPattern: \".*/payments\" }, response: { status: 503 } }",
     "tested": true,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "file",
       "takes": "text",
       "about": "A path relative to this flow",
       "values": [],
       "short": true
      },
      {
       "name": "id",
       "takes": "text",
       "about": "A name for it",
       "values": [],
       "short": false
      },
      {
       "name": "name",
       "takes": "text",
       "about": "A name for it",
       "values": [],
       "short": false
      },
      {
       "name": "priority",
       "takes": "number",
       "about": "The lowest wins (0)",
       "values": [],
       "short": false
      },
      {
       "name": "scenarioName",
       "takes": "text",
       "about": "The scenario it belongs to",
       "values": [],
       "short": false
      },
      {
       "name": "requiredScenarioState",
       "takes": "text",
       "about": "Only in this state",
       "values": [],
       "short": false
      },
      {
       "name": "newScenarioState",
       "takes": "text",
       "about": "Moves to this state",
       "values": [],
       "short": false
      },
      {
       "name": "request",
       "takes": "values",
       "about": "Which calls",
       "values": [],
       "short": false
      },
      {
       "name": "response",
       "takes": "values",
       "about": "What to answer",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "mockState",
     "about": "Moves scenarios to a state, such as { ride: docked }; in rayo live, .states lists them; rayo test --mock-state sets them where --resume or --from begins",
     "example": "- mockState: { order: shipped }",
     "tested": true,
     "alone": false,
     "element": false,
     "keys": []
    },
    {
     "name": "network",
     "about": "How the connection to the mock behaves",
     "example": "- network: offline\n- network: { delay: 2s }\n- network: { fail: 503, path: \".*/payments\" }\n- network: online",
     "tested": true,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "state",
       "takes": "offline/online",
       "about": "online or offline",
       "values": [
        "offline",
        "online"
       ],
       "short": true
      },
      {
       "name": "delay",
       "takes": "time",
       "about": "Every answer takes longer",
       "values": [],
       "short": false
      },
      {
       "name": "fail",
       "takes": "400 to 599",
       "about": "Every answer is this status, 400 to 599",
       "values": [],
       "short": false
      },
      {
       "name": "path",
       "takes": "pattern",
       "about": "Only these calls, a regular expression of the whole path",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "openLink",
     "about": "Opens a link in whichever app handles it",
     "example": "- openLink: https://example.com/item/7",
     "tested": false,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "link",
       "takes": "text",
       "about": "What to open",
       "values": [],
       "short": true
      }
     ]
    },
    {
     "name": "pasteText",
     "about": "Adds rayo.copiedText to the focused field",
     "example": "- copyTextFrom: { id: code }\n- tapOn: { id: code_input }\n- pasteText",
     "tested": false,
     "alone": true,
     "element": false,
     "keys": []
    },
    {
     "name": "pressKey",
     "about": "Presses a key",
     "example": "- pressKey: Enter\n- pressKey: Home",
     "tested": true,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "key",
       "takes": "key",
       "about": "A name, such as Enter, or an Android key code",
       "values": [],
       "short": true
      }
     ]
    },
    {
     "name": "repeat",
     "about": "Runs commands several times",
     "example": "- repeat:\n    times: 3\n    commands:\n      - tapOn: { id: next }\n- repeat:\n    while: { visible: { id: next } }\n    times: 10\n    commands:\n      - tapOn: { id: next }",
     "tested": false,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "times",
       "takes": "number",
       "about": "At most",
       "values": [],
       "short": false
      },
      {
       "name": "while",
       "takes": "conditions",
       "about": "As long as these hold; without times, it fails after 1000 rounds that it still holds",
       "values": [],
       "short": false
      },
      {
       "name": "commands",
       "takes": "commands",
       "about": "What to run",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "retry",
     "about": "Runs again what fails",
     "example": "- retry:\n    retries: 2\n    commands:\n      - tapOn: { id: refresh }\n      - assertVisible: Updated",
     "tested": false,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "retries",
       "takes": "number",
       "about": "How many more times (1)",
       "values": [],
       "short": false
      },
      {
       "name": "commands",
       "takes": "commands",
       "about": "What to run",
       "values": [],
       "short": false
      },
      {
       "name": "file",
       "takes": "text",
       "about": "A path relative to this flow",
       "values": [],
       "short": false
      },
      {
       "name": "env",
       "takes": "values",
       "about": "Variables for it",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "runFlow",
     "about": "Runs another flow, or commands right here",
     "example": "- runFlow: login.yaml\n- runFlow: { file: login.yaml, env: { EMAIL: me@example.com } }\n- runFlow:\n    when: { visible: Accept cookies }\n    commands:\n      - tapOn: Accept",
     "tested": true,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "file",
       "takes": "text",
       "about": "A path relative to this flow",
       "values": [],
       "short": true
      },
      {
       "name": "env",
       "takes": "values",
       "about": "Variables for it",
       "values": [],
       "short": false
      },
      {
       "name": "when",
       "takes": "conditions",
       "about": "Only if these hold",
       "values": [],
       "short": false
      },
      {
       "name": "commands",
       "takes": "commands",
       "about": "What to run",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "runScript",
     "about": "Runs a JavaScript file",
     "example": "- runScript: create_user.js\n- runScript: { file: create_user.js, env: { NAME: Ana } }",
     "tested": true,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "file",
       "takes": "text",
       "about": "A path relative to this flow",
       "values": [],
       "short": true
      },
      {
       "name": "env",
       "takes": "values",
       "about": "Variables for it",
       "values": [],
       "short": false
      },
      {
       "name": "when",
       "takes": "conditions",
       "about": "Only if these hold",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "scroll",
     "about": "Scrolls down once",
     "example": "- scroll",
     "tested": false,
     "alone": true,
     "element": false,
     "keys": []
    },
    {
     "name": "scrollUntilVisible",
     "about": "Scrolls until the whole element is on screen",
     "example": "- scrollUntilVisible: { element: { text: Terms }, timeout: 30s }",
     "tested": false,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "element",
       "takes": "selector",
       "about": "What to look for",
       "values": [],
       "short": false
      },
      {
       "name": "direction",
       "takes": "UP/DOWN/LEFT/RIGHT",
       "about": "Where the finger or the search goes (DOWN when searching)",
       "values": [
        "UP",
        "DOWN",
        "LEFT",
        "RIGHT"
       ],
       "short": false
      },
      {
       "name": "centerElement",
       "takes": "true/false",
       "about": "Go on until it is in the middle of the list, or the list ends (false)",
       "values": [
        "true",
        "false"
       ],
       "short": false
      },
      {
       "name": "speed",
       "takes": "number",
       "about": "0 to 100, as Maestro: each scroll takes (100 - speed) * 10 ms (40)",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "setAirplaneMode",
     "about": "Turns airplane mode on or off",
     "example": "- setAirplaneMode: enabled\n- setAirplaneMode: disabled",
     "tested": false,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "value",
       "takes": "enabled/disabled/true/false",
       "about": "enabled or disabled",
       "values": [
        "enabled",
        "disabled",
        "true",
        "false"
       ],
       "short": true
      }
     ]
    },
    {
     "name": "setLocation",
     "about": "Stays at a place",
     "example": "- setLocation: { latitude: 41.3785, longitude: 2.1765 }",
     "tested": true,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "latitude",
       "takes": "number",
       "about": "In degrees",
       "values": [],
       "short": false
      },
      {
       "name": "longitude",
       "takes": "number",
       "about": "In degrees",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "setOrientation",
     "about": "Turns the screen, and keeps it so until the flow ends",
     "example": "- setOrientation: LANDSCAPE_LEFT\n- assertVisible: { id: player }\n- setOrientation: PORTRAIT",
     "tested": false,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "orientation",
       "takes": "PORTRAIT/LANDSCAPE_LEFT/UPSIDE_DOWN/LANDSCAPE_RIGHT",
       "about": "How the screen is turned",
       "values": [
        "PORTRAIT",
        "LANDSCAPE_LEFT",
        "UPSIDE_DOWN",
        "LANDSCAPE_RIGHT"
       ],
       "short": true
      }
     ]
    },
    {
     "name": "setPermissions",
     "about": "Allows or denies permissions",
     "example": "- setPermissions: { permissions: { all: deny, camera: allow } }",
     "tested": true,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "appId",
       "takes": "text",
       "about": "The app; by default the header's",
       "values": [],
       "short": false
      },
      {
       "name": "permissions",
       "takes": "permissions",
       "about": "Name: allow, deny or unset. With clearState, all are allowed if not said",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "shell",
     "about": "Runs a command on the device",
     "example": "- shell: am broadcast -a com.example.REFRESH -p com.example.app",
     "tested": true,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "command",
       "takes": "text",
       "about": "Run as the shell user",
       "values": [],
       "short": true
      }
     ]
    },
    {
     "name": "startRecording",
     "about": "Records the screen, until stopRecording or the end of the flow",
     "example": "- startRecording          # with the flow's name\n- tapOn: { id: login }\n- stopRecording\n- startRecording: checkout",
     "tested": true,
     "alone": true,
     "element": false,
     "keys": [
      {
       "name": "path",
       "takes": "text",
       "about": "The file, in the debug output; by default the flow's name",
       "values": [],
       "short": true
      }
     ]
    },
    {
     "name": "stopApp",
     "about": "Closes the app",
     "example": "- stopApp\n- stopApp: com.example.other",
     "tested": false,
     "alone": true,
     "element": false,
     "keys": [
      {
       "name": "appId",
       "takes": "text",
       "about": "The app; by default the header's",
       "values": [],
       "short": true
      }
     ]
    },
    {
     "name": "stopRecording",
     "about": "Saves the recording",
     "example": "- startRecording: login\n- tapOn: { id: login }\n- stopRecording",
     "tested": true,
     "alone": true,
     "element": false,
     "keys": []
    },
    {
     "name": "swipe",
     "about": "Moves a finger across the screen",
     "example": "- swipe: { direction: LEFT }\n- swipe: { direction: UP, from: { id: sheet } }\n- swipe: { start: \"50%, 80%\", end: \"50%, 20%\", duration: 400 }",
     "tested": false,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "direction",
       "takes": "UP/DOWN/LEFT/RIGHT",
       "about": "Where the finger or the search goes (DOWN when searching)",
       "values": [
        "UP",
        "DOWN",
        "LEFT",
        "RIGHT"
       ],
       "short": false
      },
      {
       "name": "start",
       "takes": "point",
       "about": "Where the finger goes down, instead of direction",
       "values": [],
       "short": false
      },
      {
       "name": "end",
       "takes": "point",
       "about": "Where it goes up",
       "values": [],
       "short": false
      },
      {
       "name": "duration",
       "takes": "time",
       "about": "Of the swipe (250)",
       "values": [],
       "short": false
      },
      {
       "name": "from",
       "takes": "selector",
       "about": "Start on this element; with direction",
       "values": [],
       "short": false
      },
      {
       "name": "fling",
       "takes": "true/false",
       "about": "Lift the finger still moving, so the list goes on by itself (true); false stops it where the finger does",
       "values": [
        "true",
        "false"
       ],
       "short": false
      }
     ]
    },
    {
     "name": "takeScreenshot",
     "about": "Saves the screen",
     "example": "- takeScreenshot: home",
     "tested": false,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "path",
       "takes": "text",
       "about": "The file, in the debug output",
       "values": [],
       "short": true
      }
     ]
    },
    {
     "name": "tapOn",
     "about": "Waits until the element is on screen, enabled and still, then taps it",
     "example": "- tapOn: Log in\n- tapOn: { id: save }\n- tapOn: { text: Delete, below: { id: row, text: Invoice 7 } }\n- tapOn: { point: \"35%, 30%\" }",
     "tested": true,
     "alone": false,
     "element": true,
     "keys": [
      {
       "name": "point",
       "takes": "point",
       "about": "Alone, of the screen; with a selector, of the element in %",
       "values": [],
       "short": false
      },
      {
       "name": "repeat",
       "takes": "number",
       "about": "Taps (1)",
       "values": [],
       "short": false
      },
      {
       "name": "delay",
       "takes": "time",
       "about": "Between taps (100)",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "travel",
     "about": "Moves along points at a real speed",
     "example": "- travel: { points: [\"41.3785,2.1765\", \"41.3805,2.1748\"], speed: 6 }",
     "tested": false,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "points",
       "takes": "locations",
       "about": "A list of 'latitude,longitude'",
       "values": [],
       "short": false
      },
      {
       "name": "speed",
       "takes": "number",
       "about": "Metres per second (7)",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "type",
     "about": "Replaces the text of a field at once, without the keyboard",
     "example": "- type: { id: email, value: me@example.com }",
     "tested": true,
     "alone": false,
     "element": true,
     "keys": [
      {
       "name": "value",
       "takes": "text",
       "about": "The new text",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "waitForAnimationToEnd",
     "about": "Waits until nothing has changed for a while",
     "example": "- waitForAnimationToEnd\n- waitForAnimationToEnd: { timeout: 3s }",
     "tested": false,
     "alone": true,
     "element": false,
     "keys": [
      {
       "name": "quiet",
       "takes": "time",
       "about": "How long nothing may change (1s)",
       "values": [],
       "short": false
      }
     ]
    },
    {
     "name": "waitForRequest",
     "about": "Waits until the app makes a call",
     "example": "- waitForRequest: \".*/payments\"\n- waitForRequest: { method: POST, path: \".*/payments\", body: \"\\\"amount\\\":5\" }",
     "tested": true,
     "alone": false,
     "element": false,
     "keys": [
      {
       "name": "method",
       "takes": "text",
       "about": "Such as POST (GET)",
       "values": [],
       "short": false
      },
      {
       "name": "path",
       "takes": "pattern",
       "about": "Only these calls, a regular expression of the whole path",
       "values": [],
       "short": true
      },
      {
       "name": "body",
       "takes": "text",
       "about": "A text in the body",
       "values": [],
       "short": false
      }
     ]
    }
   ],
   "selector": [
    {
     "name": "id",
     "takes": "text",
     "about": "The resource id, whole or after :id/; in Compose, a testTag with testTagsAsResourceId. A regular expression of the whole",
     "values": [],
     "short": false
    },
    {
     "name": "text",
     "takes": "text",
     "about": "The text, description or hint. A regular expression of the whole",
     "values": [],
     "short": false
    },
    {
     "name": "index",
     "takes": "number",
     "about": "The n-th match, from 0, top to bottom and left to right; negative counts from the end",
     "values": [],
     "short": false
    },
    {
     "name": "enabled",
     "takes": "true/false",
     "about": "Whether it is enabled",
     "values": [
      "true",
      "false"
     ],
     "short": false
    },
    {
     "name": "checked",
     "takes": "true/false",
     "about": "Whether it is checked",
     "values": [
      "true",
      "false"
     ],
     "short": false
    },
    {
     "name": "focused",
     "takes": "true/false",
     "about": "Whether it has the focus",
     "values": [
      "true",
      "false"
     ],
     "short": false
    },
    {
     "name": "selected",
     "takes": "true/false",
     "about": "Whether it is selected",
     "values": [
      "true",
      "false"
     ],
     "short": false
    },
    {
     "name": "below",
     "takes": "selector",
     "about": "Entirely below another",
     "values": [],
     "short": false
    },
    {
     "name": "above",
     "takes": "selector",
     "about": "Entirely above another",
     "values": [],
     "short": false
    },
    {
     "name": "leftOf",
     "takes": "selector",
     "about": "Entirely to the left of another",
     "values": [],
     "short": false
    },
    {
     "name": "rightOf",
     "takes": "selector",
     "about": "Entirely to the right of another",
     "values": [],
     "short": false
    },
    {
     "name": "containsChild",
     "takes": "selector",
     "about": "The direct parent of another",
     "values": [],
     "short": false
    },
    {
     "name": "childOf",
     "takes": "selector",
     "about": "Anywhere inside another",
     "values": [],
     "short": false
    },
    {
     "name": "containsDescendants",
     "takes": "selectors",
     "about": "Around all of these",
     "values": [],
     "short": false
    }
   ],
   "when": [
    {
     "name": "visible",
     "takes": "selector",
     "about": "On screen",
     "values": [],
     "short": false
    },
    {
     "name": "notVisible",
     "takes": "selector",
     "about": "Not on screen",
     "values": [],
     "short": false
    },
    {
     "name": "platform",
     "takes": "Android/iOS/Web",
     "about": "Android",
     "values": [
      "Android",
      "iOS",
      "Web"
     ],
     "short": false
    },
    {
     "name": "true",
     "takes": "JavaScript",
     "about": "An expression that holds",
     "values": [],
     "short": false
    },
    {
     "name": "timeout",
     "takes": "time",
     "about": "How long visible and notVisible may take (700ms); 0 looks once",
     "values": [],
     "short": false
    }
   ],
   "options": [
    {
     "name": "label",
     "takes": "text",
     "about": "What to show instead of the command",
     "values": [],
     "short": false
    },
    {
     "name": "optional",
     "takes": "true/false",
     "about": "A failure doesn't fail the flow",
     "values": [
      "true",
      "false"
     ],
     "short": false
    },
    {
     "name": "timeout",
     "takes": "time",
     "about": "How long it may take",
     "values": [],
     "short": false
    }
   ]
  },
  "header": [
   {
    "name": "appId",
    "takes": "package",
    "about": "The app's",
    "required": false
   },
   {
    "name": "name",
    "takes": "text",
    "about": "Shown in the output (default: the file's, or the folder's for flow.yaml)",
    "required": false
   },
   {
    "name": "tags",
    "takes": "a list",
    "about": "To choose flows",
    "required": false
   },
   {
    "name": "env",
    "takes": "values",
    "about": "Variables for it",
    "required": false
   },
   {
    "name": "mock",
    "takes": "a path or a list",
    "about": "Who answers as the backend; ${...} too, where an empty path loads nothing",
    "required": false
   },
   {
    "name": "network",
    "takes": "interceptor or proxy",
    "about": "How the app's calls reach the mock; ${...} too, such as ${NETWORK} from -e, where empty is the default (default: interceptor, or --network)",
    "required": false
   },
   {
    "name": "record",
    "takes": "true, false or a file",
    "about": "A video of the flow (default: the suite's)",
    "required": false
   },
   {
    "name": "onFlowStart",
    "takes": "commands",
    "about": "Run first",
    "required": false
   },
   {
    "name": "onFlowComplete",
    "takes": "commands",
    "about": "Run last, also after a failure",
    "required": false
   }
  ],
  "ignored": [
   {
    "name": "host",
    "takes": "host",
    "about": "Of the call, or of its subdomains (default: any)",
    "required": false
   },
   {
    "name": "path",
    "takes": "a path",
    "about": "Of the call, without the host; * is any text but /, ** any (default: any)",
    "required": false
   },
   {
    "name": "graphql",
    "takes": "a name or a list",
    "about": "GraphQL operations (default: any call)",
    "required": false
   }
  ],
  "options": [
   {
    "name": "device",
    "short": "",
    "takes": "serial",
    "about": "when several are connected; else ANDROID_SERIAL, else the only one",
    "commands": [
     "test",
     "live",
     "hierarchy"
    ]
   },
   {
    "name": "emulator",
    "short": "",
    "takes": "",
    "about": "never a real device, even if it is the only one",
    "commands": [
     "test",
     "live",
     "hierarchy"
    ]
   },
   {
    "name": "changed-only",
    "short": "",
    "takes": "",
    "about": "only the flows of which a file changed: their own, one they run, or a mock; as --since HEAD",
    "commands": [
     "test",
     "check"
    ]
   },
   {
    "name": "since",
    "short": "",
    "takes": "git ref",
    "about": "only the flows of which a file changed since then, also not committed",
    "commands": [
     "test",
     "check"
    ]
   },
   {
    "name": "find",
    "short": "",
    "takes": "text",
    "about": "of the screen, only the elements most like it, instead of all",
    "commands": [
     "hierarchy",
     "do"
    ]
   },
   {
    "name": "output",
    "short": "",
    "takes": "folder",
    "about": "where failures and screenshots go (default .rayo)",
    "commands": [
     "test",
     "live",
     "do",
     "explain"
    ]
   },
   {
    "name": "timeout-scale",
    "short": "",
    "takes": "factor",
    "about": "multiply the default waits, not a step's own timeout, from 0.1 to 10 (default 1)",
    "commands": [
     "test",
     "live"
    ]
   },
   {
    "name": "mock-port",
    "short": "",
    "takes": "port",
    "about": "where the app finds the mock on the device (default 8080)",
    "commands": [
     "test",
     "live"
    ]
   },
   {
    "name": "animations",
    "short": "",
    "takes": "",
    "about": "leave the device's animations on",
    "commands": [
     "test",
     "live"
    ]
   },
   {
    "name": "profile",
    "short": "",
    "takes": "",
    "about": "where the time of each step went",
    "commands": [
     "test",
     "live"
    ]
   },
   {
    "name": "only",
    "short": "",
    "takes": "a,b",
    "about": "only the flows with these names",
    "commands": [
     "test",
     "check"
    ]
   },
   {
    "name": "include-tags",
    "short": "",
    "takes": "a,b",
    "about": "only flows with one of these tags",
    "commands": [
     "test",
     "check"
    ]
   },
   {
    "name": "exclude-tags",
    "short": "",
    "takes": "a,b",
    "about": "not flows with one of these tags",
    "commands": [
     "test",
     "check"
    ]
   },
   {
    "name": "rerun-failed",
    "short": "",
    "takes": "",
    "about": "only those that haven't passed since they last failed",
    "commands": [
     "test",
     "check"
    ]
   },
   {
    "name": "min-flows",
    "short": "",
    "takes": "n",
    "about": "fail (exit 2) if fewer flows than this are chosen, as a typo in a tag or a name would; of all the parts with --shard, and not with --rerun-failed or --resume (default: the suite's minFlows, else none)",
    "commands": [
     "test",
     "check"
    ]
   },
   {
    "name": "shard",
    "short": "",
    "takes": "i/n",
    "about": "only part i of n, for n machines that run the same flows: each flow is in one part, by its path, whatever the order; it writes to .rayo/shard-i-of-n unless --output or the suite says",
    "commands": [
     "test",
     "check"
    ]
   },
   {
    "name": "env",
    "short": "e",
    "takes": "KEY=VALUE",
    "about": "a variable for the flows; over the suite's env, which is over a flow's own",
    "commands": [
     "test",
     "check",
     "live"
    ]
   },
   {
    "name": "retries",
    "short": "",
    "takes": "n",
    "about": "try again what fails, every flow, also one the suite gives its own retries (default 1, or the suite's)",
    "commands": [
     "test",
     "check"
    ]
   },
   {
    "name": "fail-fast",
    "short": "",
    "takes": "",
    "about": "stop at the first failure",
    "commands": [
     "test"
    ]
   },
   {
    "name": "flaky-fails-build",
    "short": "",
    "takes": "",
    "about": "exit 4 if every flow passed but some only on a retry (default: exit 0)",
    "commands": [
     "test"
    ]
   },
   {
    "name": "keep-going",
    "short": "",
    "takes": "",
    "about": "don't stop when the device or the app fails over and over",
    "commands": [
     "test"
    ]
   },
   {
    "name": "shuffle",
    "short": "",
    "takes": "seed",
    "about": "in a random order, or in that of an earlier run",
    "commands": [
     "test"
    ]
   },
   {
    "name": "screens-before",
    "short": "",
    "takes": "n",
    "about": "keep the screen after each of the last n steps that passed, and save them with a failure as screens-before.txt: how the app got there; each costs a read of the screen (default 0)",
    "commands": [
     "test"
    ]
   },
   {
    "name": "continuous",
    "short": "c",
    "takes": "",
    "about": "run again whenever a file changes",
    "commands": [
     "test"
    ]
   },
   {
    "name": "on-fail",
    "short": "",
    "takes": "what",
    "about": "go-on (default): retry it as allowed, then the next flow; hold: keep the flow where it failed, the app as it is, until rayo do says how to go on, so that it is fixed in its files and run again from there; see rayo help hold",
    "commands": [
     "test"
    ]
   },
   {
    "name": "hold-timeout",
    "short": "",
    "takes": "duration",
    "about": "with --on-fail hold, leave a flow failed if no command comes for this long, such as 20m (default 1h)",
    "commands": [
     "test"
    ]
   },
   {
    "name": "resume",
    "short": "",
    "takes": "",
    "about": "each flow that failed last time, from the step that failed, with the app as it is and the mocks' scenarios as they were then",
    "commands": [
     "test"
    ]
   },
   {
    "name": "from",
    "short": "",
    "takes": "step",
    "about": "begin one flow at a step: file:line as rayo explain's at: says, a line of its own file, or a text of one step such as pickDate; the app is as it is, onFlowStart doesn't run",
    "commands": [
     "test"
    ]
   },
   {
    "name": "until",
    "short": "",
    "takes": "step",
    "about": "stop one flow after a step, given as --from takes it, leaving the app there; onFlowComplete doesn't run",
    "commands": [
     "test"
    ]
   },
   {
    "name": "mock-state",
    "short": "",
    "takes": "a=s,b=t",
    "about": "with --resume or --from, the scenarios' states to begin with, over those of the last run",
    "commands": [
     "test"
    ]
   },
   {
    "name": "video",
    "short": "",
    "takes": "",
    "about": "a video of each flow whose header doesn't say record, as output: { video: true }",
    "commands": [
     "test",
     "check"
    ]
   },
   {
    "name": "keep",
    "short": "",
    "takes": "when",
    "about": "the files of every try (all, default), only of the tries that failed, or none",
    "commands": [
     "test",
     "check"
    ]
   },
   {
    "name": "artifacts",
    "short": "",
    "takes": "a,b",
    "about": "the kinds of file written: video, screenshot, logcat, screen, requests (default all)",
    "commands": [
     "test",
     "check"
    ]
   },
   {
    "name": "name-template",
    "short": "",
    "takes": "template",
    "about": "how the files are named in the output folder (default {flow}_{attempt}_{kind}.{ext})",
    "commands": [
     "test",
     "check"
    ]
   },
   {
    "name": "detail",
    "short": "",
    "takes": "how much",
    "about": "printed: brief, the verdict of a failure; normal (default), also why, where, the nearest elements and the calls, and every step of a single flow; full, also the screen, every step, and whole files",
    "commands": [
     "test"
    ]
   },
   {
    "name": "failure-window",
    "short": "",
    "takes": "seconds",
    "about": "a failure's log and calls begin this long before the failing step (default 10)",
    "commands": [
     "test"
    ]
   },
   {
    "name": "results",
    "short": "",
    "takes": "file",
    "about": "write the results: .txt, .json, .xml (JUnit) or .md; may be repeated",
    "commands": [
     "test"
    ]
   },
   {
    "name": "results-template",
    "short": "",
    "takes": "file",
    "about": "write the results with that extension as this file says, with {suite}, {passed}, {failed}, {flaky} and the like, and {flows}...{end}",
    "commands": [
     "test"
    ]
   },
   {
    "name": "show",
    "short": "",
    "takes": "a,b",
    "about": "the flows the results list: failed, flaky, slow or all (default all)",
    "commands": [
     "test"
    ]
   },
   {
    "name": "slowest",
    "short": "",
    "takes": "n",
    "about": "how many flows the slow list has (default 5)",
    "commands": [
     "test"
    ]
   },
   {
    "name": "timings",
    "short": "",
    "takes": "",
    "about": "while it runs: each step's time, each runFlow's total, the slowest steps and flows, and a note while a step hangs",
    "commands": [
     "test"
    ]
   },
   {
    "name": "capture",
    "short": "",
    "takes": "folder",
    "about": "call the real backend, and write what it answers as mocks; in live, on .quit, without the calls of steps that failed",
    "commands": [
     "test",
     "live"
    ]
   },
   {
    "name": "capture-update",
    "short": "",
    "takes": "folder",
    "about": "capture into folder/.update, and say what changed from the mocks in folder; rayo mocks apply takes it. Given a suite instead, the folder is the flow's mock",
    "commands": [
     "test",
     "live"
    ]
   },
   {
    "name": "check",
    "short": "",
    "takes": "",
    "about": "with --capture-update, fail if the mocks differ from the backend, and change nothing",
    "commands": [
     "test",
     "live"
    ]
   },
   {
    "name": "capture-append",
    "short": "",
    "takes": "",
    "about": "go on with the capture already in the folder, as if in one go",
    "commands": [
     "test",
     "live"
    ]
   },
   {
    "name": "redact-value",
    "short": "",
    "takes": "name=value",
    "about": "what a private field of a body becomes, such as phone=+4915100000000 (default one of the same form: a date stays a date); again for more",
    "commands": [
     "test",
     "live"
    ]
   },
   {
    "name": "capture-from",
    "short": "",
    "takes": "url",
    "about": "the backend, if calls don't begin with /host/",
    "commands": [
     "test",
     "live",
     "mocks"
    ]
   },
   {
    "name": "graphql",
    "short": "",
    "takes": "match",
    "about": "tell GraphQL calls apart by operation+variables (default), by operation, or off, as any call",
    "commands": [
     "test",
     "live",
     "mocks"
    ]
   },
   {
    "name": "graphql-path",
    "short": "",
    "takes": "regex",
    "about": "paths of GraphQL calls, besides those whose body has query or operationName (default .*/graphql)",
    "commands": [
     "test",
     "live",
     "mocks"
    ]
   },
   {
    "name": "rest",
    "short": "",
    "takes": "match",
    "about": "match other calls by their body too (body, default), or by method and address alone (off)",
    "commands": [
     "test",
     "live",
     "mocks"
    ]
   },
   {
    "name": "ignore-params",
    "short": "",
    "takes": "a,b",
    "about": "names in the query not to match, such as session or depart_at",
    "commands": [
     "test",
     "live",
     "mocks"
    ]
   },
   {
    "name": "ignore-body",
    "short": "",
    "takes": "a,b",
    "about": "names, or paths such as trip.departAt, in JSON bodies and GraphQL variables, and names in forms, not to match",
    "commands": [
     "test",
     "live",
     "mocks"
    ]
   },
   {
    "name": "timestamps",
    "short": "",
    "takes": "match",
    "about": "match dates and times in the query and bodies by their form (format, default), or exactly",
    "commands": [
     "test",
     "live",
     "mocks"
    ]
   },
   {
    "name": "capture-headers",
    "short": "",
    "takes": "a,b",
    "about": "headers of the request to match too, such as Accept-Language",
    "commands": [
     "test",
     "live",
     "mocks"
    ]
   },
   {
    "name": "ignore-host",
    "short": "",
    "takes": "",
    "about": "match calls whose path begins with a host, whatever the host",
    "commands": [
     "test",
     "live",
     "mocks"
    ]
   },
   {
    "name": "ignore-path-ids",
    "short": "",
    "takes": "",
    "about": "match UUIDs and ULIDs in paths by their form, as they are new on each run",
    "commands": [
     "test",
     "live",
     "mocks"
    ]
   },
   {
    "name": "stateless",
    "short": "",
    "takes": "a,b",
    "about": "paths such as api.shop.com/quotes/** whose calls only read, whatever the method: they don't change the state (* any text but /, ** any)",
    "commands": [
     "test",
     "live",
     "mocks"
    ]
   },
   {
    "name": "network",
    "short": "",
    "takes": "way",
    "about": "interceptor, the app sends them, with code in its debug build (default, or the flow's); proxy, the device's proxy does, no code; see rayo help connect-mocks",
    "commands": [
     "test",
     "live",
     "ca"
    ]
   },
   {
    "name": "ignore-unmocked",
    "short": "",
    "takes": "a,b",
    "about": "calls without a mock that don't fail the flow, answered 404: graphql:Operation, host/path, host or /path (* any text but /, ** any)",
    "commands": [
     "test",
     "live",
     "ca"
    ]
   },
   {
    "name": "ca",
    "short": "",
    "takes": "folder",
    "about": "with a proxy, the CA it signs with, its ca.pem and ca-key.pem (default .rayo/ca next to the suite or above it, else ~/.rayo); see rayo ca",
    "commands": [
     "test",
     "live",
     "ca"
    ]
   },
   {
    "name": "hosts",
    "short": "",
    "takes": "a,b",
    "about": "with a proxy, the hosts to open, and their subdomains (default: capturing, all but Google's, Firebase's, analytics and crash reports; else those of the mocks)",
    "commands": [
     "test",
     "live",
     "ca"
    ]
   },
   {
    "name": "app",
    "short": "",
    "takes": "package",
    "about": "of a flow that isn't there yet",
    "commands": [
     "live"
    ]
   },
   {
    "name": "mock",
    "short": "",
    "takes": "a,b",
    "about": "its mocks, relative to it",
    "commands": [
     "live"
    ]
   },
   {
    "name": "no-run",
    "short": "",
    "takes": "",
    "about": "don't begin by running the steps it has",
    "commands": [
     "live"
    ]
   },
   {
    "name": "serve",
    "short": "",
    "takes": "",
    "about": "only take commands from rayo do, until .quit",
    "commands": [
     "live"
    ]
   },
   {
    "name": "latest",
    "short": "",
    "takes": "",
    "about": "only the flows of the newest run in failed.json, not those still there from earlier ones",
    "commands": [
     "explain"
    ]
   },
   {
    "name": "only",
    "short": "",
    "takes": "a,b",
    "about": "the changes whose name has one of these, such as graphql:Wallet or GET /profile (default all)",
    "commands": [
     "mocks"
    ]
   },
   {
    "name": "prune",
    "short": "",
    "takes": "",
    "about": "also take out the mocks that weren't called",
    "commands": [
     "mocks"
    ]
   },
   {
    "name": "screen",
    "short": "",
    "takes": "changes",
    "about": "also what is on screen afterwards; --screen=changes, only the lines that appeared (+) or went (-) since the screen the session last gave",
    "commands": [
     "do"
    ]
   },
   {
    "name": "ignore-screen",
    "short": "",
    "takes": "regex",
    "about": "with --screen, leave out the lines that match, such as a clock's; says how many",
    "commands": [
     "do"
    ]
   },
   {
    "name": "timeout",
    "short": "",
    "takes": "duration",
    "about": "how long to wait for the answer, such as 30s (default as long as it takes); the session goes on",
    "commands": [
     "do"
    ]
   },
   {
    "name": "discard",
    "short": "",
    "takes": "",
    "about": "with .quit, end even if steps were not saved",
    "commands": [
     "do"
    ]
   },
   {
    "name": "format",
    "short": "",
    "takes": "how",
    "about": "human (default); json, for a program, see rayo schema; ndjson, the same on one line, and for test an event a line as it runs",
    "commands": [
     "test",
     "check",
     "explain",
     "devices",
     "hierarchy",
     "do",
     "mocks"
    ]
   }
  ],
  "suite": [
   {
    "name": "name",
    "takes": "text",
    "about": "Shown in the results (default: the file)",
    "required": false
   },
   {
    "name": "flows",
    "takes": "a list",
    "about": "Files, run in this order; or mappings with file, retries, mock, network, unmocked and capture (required)",
    "required": true
   },
   {
    "name": "retries",
    "takes": "a number from 0",
    "about": "How many times a flow that fails is tried again (default: 1)",
    "required": false
   },
   {
    "name": "app",
    "takes": "a mapping",
    "about": "Installed before the flows",
    "required": false
   },
   {
    "name": "device",
    "takes": "a mapping",
    "about": "How it is while the flows run",
    "required": false
   },
   {
    "name": "env",
    "takes": "values",
    "about": "Variables for every flow, over a flow's own env; -e overrides one; they may use ${...}, such as ${rayo.file}",
    "required": false
   },
   {
    "name": "timeoutScale",
    "takes": "0.1 to 10",
    "about": "Multiplies the default waits; --timeout-scale overrides it (default: 1)",
    "required": false
   },
   {
    "name": "network",
    "takes": "interceptor or proxy",
    "about": "How the app's calls reach the mock, as a flow's network; the flow's own and --network override it (default: each flow's, else interceptor)",
    "required": false
   },
   {
    "name": "unmocked",
    "takes": "a mapping",
    "about": "Calls without a mock that don't fail the flow (default: none: each fails the flow)",
    "required": false
   },
   {
    "name": "capture",
    "takes": "a mapping",
    "about": "How a capture calls the backend and becomes mocks, as the options of the same names, which win",
    "required": false
   },
   {
    "name": "ca",
    "takes": "folder",
    "about": "--ca overrides it: the CA the proxy signs with, its ca.pem and ca-key.pem, relative to the suite; rayo ca new makes one (default: .rayo/ca next to the suite or above it, up to the repository's root; else ~/.rayo)",
    "required": false
   },
   {
    "name": "output",
    "takes": "a folder, or a mapping",
    "about": "What a run writes, and where: a folder, or folder, keep, video, artifacts and nameTemplate (default: .rayo where rayo runs)",
    "required": false
   },
   {
    "name": "results",
    "takes": "a file or a list",
    "about": "--results replaces them all: .txt, .json, .xml (JUnit) or .md",
    "required": false
   },
   {
    "name": "detail",
    "takes": "brief, normal, full",
    "about": "--detail overrides it: how much is printed (default: normal)",
    "required": false
   },
   {
    "name": "failureWindow",
    "takes": "seconds, from 1",
    "about": "--failure-window overrides it: a failure's log and calls begin this long before the failing step (default: 10)",
    "required": false
   },
   {
    "name": "resultsTemplate",
    "takes": "file",
    "about": "--results-template overrides it: the results with its extension are written as it says",
    "required": false
   },
   {
    "name": "show",
    "takes": "a name or a list",
    "about": "--show overrides it: the flows the results list, of failed, flaky, slow, all (default: all)",
    "required": false
   },
   {
    "name": "slowest",
    "takes": "a number from 1",
    "about": "--slowest overrides it: how many flows the slow list has (default: 5)",
    "required": false
   },
   {
    "name": "format",
    "takes": "human, json or ndjson",
    "about": "What is printed; --format overrides it (default: human)",
    "required": false
   },
   {
    "name": "minFlows",
    "takes": "a number from 1",
    "about": "--min-flows overrides it: fewer flows chosen, by tags, names or what changed, is an error (exit 2), not a green run of nothing (default: none)",
    "required": false
   },
   {
    "name": "flakyFailsBuild",
    "takes": "true or false",
    "about": "--flaky-fails-build too: a run where every flow passed, some only on a retry, exits 4 (default: false: it exits 0)",
    "required": false
   }
  ],
  "suiteFlow": [
   {
    "name": "file",
    "takes": "file",
    "about": "The flow (required)",
    "required": true
   },
   {
    "name": "retries",
    "takes": "a number from 0",
    "about": "How many times it is tried again if it fails (default: the suite's)",
    "required": false
   },
   {
    "name": "mock",
    "takes": "a path or a list",
    "about": "Who answers as the backend, as a flow's mock, relative to the suite; the flow's own mock wins",
    "required": false
   },
   {
    "name": "network",
    "takes": "interceptor or proxy",
    "about": "How the app's calls reach the mock, as a flow's network; the flow's own and --network override it (default: the suite's)",
    "required": false
   },
   {
    "name": "unmocked",
    "takes": "a mapping",
    "about": "Calls without a mock that don't fail the flow (default: the suite's; its calls are added to them)",
    "required": false
   },
   {
    "name": "capture",
    "takes": "a mapping",
    "about": "How a capture calls the backend and becomes mocks, as the options of the same names, which win (default: the suite's; what it says wins over it)",
    "required": false
   }
  ],
  "unmocked": [
   {
    "name": "ignore",
    "takes": "a list",
    "about": "Calls that no mock answers but that don't fail the flow: each graphql:Operation, host/path, host or /path, as --ignore-unmocked, or a mapping with host, path and graphql (required)",
    "required": true
   },
   {
    "name": "status",
    "takes": "100 to 599",
    "about": "What those calls are answered with (default: 404)",
    "required": false
   },
   {
    "name": "body",
    "takes": "text",
    "about": "Of the answer (default: not mocked:  and the call)",
    "required": false
   }
  ]
 }
}
