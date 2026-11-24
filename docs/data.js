// Written by go test ./test -update
window.RAYO = {
 "markdown": {
  "CONTRIBUTING.md": "# Working on rayo\n\n## Build\n\n```\nbash build.sh            # bin/rayo, needs Go\nbash build.sh --all      # for Windows, Linux and macOS\nbash build.sh --release  # archives in dist/, with SHA256SUMS\nbash build.sh --agent    # also the device part, needs a JDK and the Android SDK\n```\n\n`internal/agent/agent.jar` is committed, so Go is enough unless `internal/agent/java` changes.\n\n## How it works\n\n| | |\n|---|---|\n| `cmd/rayo` | Command line |\n| `  options.go` | Every option, in one table: the commands that take it, its check and its help |\n| `  run.go` | Runs all the flows, losing none |\n| `internal/suite` | Which flows, also those of a suite |\n| `internal/report` | How a run went: on the screen, as text, JSON and JUnit |\n| `internal/schema` | What a table of keys is, and all that is made from one |\n| `internal/live` | Live sessions, and how `do` reaches them |\n| `internal/flow` | Flows: reading, checking and running them |\n| `  keys.go`, `kinds.go` | Every key a flow may use; the kinds of value, each checked and read by one function |\n| `  commands.go` | Every command, in one table: its keys, its check and what it does |\n| `  runner.go`, `report.go` | Runs a flow, and tells a `Reporter` |\n| `  control.go`, `touch.go`, `input.go`, `apps.go`, `device.go`, `recording.go`, `cause.go`, `camera.go`, `mocking.go`, `script.go` | What the commands do |\n| `internal/mock` | The mock backend, on its own; `capture.go` and `convert.go` make mocks from a real one |\n| `internal/agent` | Starts the device part (`java/`) and talks to it |\n| `internal/adb` | Talks to the adb server, sets up the device, and talks to the emulator's console |\n\nWhat files and commands may hold is said once, in tables of the code (`options.go`, `keys.go`,\n`commands.go`, `flow/schema.go`, `suite/fields.go`). From them come the checks, `rayo help`, the JSON Schemas\nin `schema/`, the tables of the reference and the page in `docs/`; a test fails if any is out of date.\n\n`test/` runs the real rayo against a pretend device and compares all that comes out, also every\ntrip to the device: `go test ./test` (5 s), `-update` to accept a change. What only a real one can tell:\n`go test ./test -run Real -emulator emulator-5554`.\n\nThe agent runs as the shell user with `app_process`. Waiting happens there, reacting to screen\nevents, so there is no polling over adb. A tap waits until the element is enabled and its\nposition holds still. When the connection closes, for whatever reason, the agent restores the\nanimations and the real location, and exits.\n\n## The documentation's page\n\n`docs/` is a page for GitHub Pages, without anything to build: `data.js` holds `GUIDE.md`, `EXAMPLE.md`,\n`REFERENCE.md`, `MOCKING.md`, this file and `rayo help json`. `app.js` says which section goes on which page. Change those, and run `go test ./test -update`. To see it,\nopen `docs/index.html`.\n",
  "EXAMPLE.md": "# A full example\n\nA shop: log in, buy something, the bank refuses the card, and no connection. These are all its files, in\n[examples/shop](examples/shop). The app is made up; the files are real, and rayo checks them.\n\n## The folders\n\n```\nshop/\n├─ all.rayo.yaml               what runs together, and how\n├─ steps/login.yaml            steps that several tests use\n├─ tests/\n│   ├─ browse/flow.yaml\n│   ├─ pay/flow.yaml\n│   ├─ declined/flow.yaml      and declined.json, an answer only for it\n│   └─ offline/flow.yaml\n└─ mock/\n    ├─ mappings/shop.json      which call gets which answer\n    └─ __files/products.json   a long answer\n```\n\n## The suite\n\n\u003c!-- file: examples/shop/all.rayo.yaml --\u003e\n\n## Steps that are shared\n\nEach test begins by running it, so each begins clean and can be run alone.\n\n\u003c!-- file: examples/shop/steps/login.yaml --\u003e\n\n## The tests\n\nThe simplest one:\n\n\u003c!-- file: examples/shop/tests/browse/flow.yaml --\u003e\n\nThis one checks what the app sent, and changes what the backend says:\n\n\u003c!-- file: examples/shop/tests/pay/flow.yaml --\u003e\n\nThis one adds an answer that wins over the normal one:\n\n\u003c!-- file: examples/shop/tests/declined/flow.yaml --\u003e\n\u003c!-- file: examples/shop/tests/declined/declined.json --\u003e\n\nAnd this one cuts the connection:\n\n\u003c!-- file: examples/shop/tests/offline/flow.yaml --\u003e\n\n## The backend\n\n\u003c!-- file: examples/shop/mock/mappings/shop.json --\u003e\n\u003c!-- file: examples/shop/mock/__files/products.json --\u003e\n\n## Run it\n\n```\nrayo check all.rayo.yaml                 # mistakes, without a device\nrayo test --emulator all.rayo.yaml       # all of it\nrayo test --only pay all.rayo.yaml       # one\n```\n\n## What comes out\n\nHow it would look, if `pay` failed once and the app had a bug in `offline`:\n\n```\n[1/4] PASS browse  9.8 s\n[2/4] PASS pay  31.4 s  (failed once, then passed)\n  try 1: tests/pay/flow.yaml:13: tapOn refresh: not on screen: refresh  (test)  results/pay\n[3/4] PASS declined  11.2 s\n[4/4] FAIL offline  41.9 s  (failed 2 times)\n  try: 1\n  why: not on screen: You are offline\n  cause: app, the app crashed: java.net.ConnectException: unexpected end of stream\n  step: assertVisible You are offline\n  at: tests/offline/flow.yaml:7\n  crash: FATAL EXCEPTION: main\n    Process: com.example.shop, PID: 4711\n    java.net.ConnectException: unexpected end of stream\n  before:\n    extendedWaitUntil products\n    network offline\n    tapOn refresh\n  saved: results/offline\n  recording: results/videos/offline.mp4\n  ...\n\nTOTAL 4: 3 passed, 1 failed, of those failed 1 app, 2 retried  94.3 s\n```\n\nAnd in `results/`:\n\n```\nresults/\n├─ results.txt                 the same, with the details of every try\n├─ junit.xml                   for a CI\n├─ failed.json                 what --rerun-failed runs\n├─ videos/                     browse.mp4, pay.mp4, pay_retry1.mp4...\n└─ offline/                    of each failure:\n    ├─ failure.jpg             the screen\n    ├─ screen.txt              the elements on it\n    ├─ logcat.txt              the device log\n    └─ requests.txt            the calls to the backend\n```\n",
  "GUIDE.md": "# Guide\n\n## Install\n\nDownload it for your system from the [releases](https://github.com/nanihadesuka/rayo/releases/latest), and unpack it:\nit is a single file. Check it against `SHA256SUMS`:\n\n```\nsha256sum -c SHA256SUMS --ignore-missing                   # Linux\nshasum -a 256 -c SHA256SUMS --ignore-missing               # macOS\nGet-FileHash rayo-windows-amd64.zip                        # Windows: compare with the list\n```\n\nOn macOS, a file from a browser also needs `xattr -d com.apple.quarantine rayo`.\n\nIt needs adb (in Android Studio, or the SDK's platform-tools), and a device or emulator. `rayo devices`\nshows what is connected. Nothing is installed on the device.\n\n## Your first test\n\nA test is a **flow**: a file with a header, `---`, and a list of commands.\n\n```yaml\nappId: com.example.app\n---\n- launchApp: { clearState: true }\n- tapOn: Log in\n- inputText: me@example.com\n- assertVisible: \"Welcome.*\"\n```\n\n```\nrayo check login.yaml              # mistakes, without a device\nrayo test --emulator login.yaml    # run it\n```\n\nThere are no pauses to add: every command waits for what it needs, and goes on as soon as it is there.\n\n## Find elements\n\nOpen the app where you want and run `rayo hierarchy`. It shows what is on screen, and how to refer to it:\n\n```\nid: login_email  [63,1060][1017,1228]\n  text: Email  [105,1123][216,1186]\nid: login_submit | disabled  [63,1481][1017,1607]\n```\n\n```yaml\n- tapOn: { id: login_email }\n- tapOn: Continue\n```\n\nPrefer an `id`: texts change with the language. In Compose, set `testTag`, and\n`testTagsAsResourceId = true` at the root. See [Selectors](REFERENCE.md#selectors).\n\n## Write a test while it runs\n\n```\nrayo live --emulator --app com.example.app tests/login.yaml\n```\n\nType commands and see what each does. Those that work are kept, and `.save` writes them to the file.\nFrom a script or another program, use `rayo live --serve` and `rayo do`. See\n[Live sessions](REFERENCE.md#live-sessions).\n\n## Mock the backend\n\nrayo answers as your backend, so tests don't depend on a server. The app sends its calls to\n`http://127.0.0.1:8080` on the device, with a small [interceptor](MOCKING.md#in-your-app) that can be only in the\ndebug build.\n\n```yaml\nappId: com.example.app\nmock: mock\n---\n- launchApp: { clearState: true, arguments: { mock: true } }\n- waitForRequest: { method: POST, path: \".*/login\" }\n```\n\nSee [Mocking](MOCKING.md). To begin from what the real backend answers, use\n[Capture](REFERENCE.md#capture).\n\n## Run many tests\n\nA **suite**, `all.rayo.yaml`, lists what runs together:\n\n```yaml\nname: Everything\nflows:\n  - tests/login.yaml\n  - { file: tests/pay.yaml, retries: 3 }\napp: { id: com.example.app, apk: app-debug.apk }\nresults: [results/results.txt, results/junit.xml]\nrecord: true\nrecordFolder: results/videos\n```\n\n```\nrayo test --emulator all.rayo.yaml\n```\n\nSee [Suites](REFERENCE.md#suites), [Retries](REFERENCE.md#retries) and [Results](REFERENCE.md#results).\n\n## When a test fails\n\nIt says why, where, what came before, and what was on screen:\n\n```\n[2/3] FAIL pay  9.1 s  (failed 2 times)\n  why: not on screen: pay_button\n  cause: test\n  step: tapOn pay_button\n  at: tests/pay.yaml:12\n  screen:\n    id: cart\n  saved: .rayo/pay\n```\n\nThe folder has a screenshot, the elements, the device log and the calls to the backend.\n`rayo test --rerun-failed` runs only what failed.\n\n# How do I...\n\n## Log in before each test\n\nPut it in a flow of its own, and run it from the others:\n\n```yaml\n- runFlow: { file: login.yaml, env: { EMAIL: me@example.com } }\n```\n\n## Keep passwords out of the files\n\n```\nrayo test -e PASSWORD=secret login.yaml\n```\n\n```yaml\n- type: { id: password, value: \"${PASSWORD}\" }\n```\n\n## Give a subflow a default\n\nA name nobody set is `undefined`, so the header can fall back:\n\n```yaml\nenv:\n  PICKUP: ${ PICKUP || \"Main Street 1\" }\n```\n\n## Set variables for every flow\n\nIn the suite, under `-e`, which wins; values may use `${...}`:\n\n```yaml\nenv:\n  API: staging\n  MAESTRO_FILENAME: ${rayo.file}   # flows from Maestro expect it: the file's name, without extension\n```\n\nSee [Variables](REFERENCE.md#variables).\n\n## Close a dialog that only appears sometimes\n\n```yaml\n- runFlow:\n    when: { visible: Rate this app }\n    commands:\n      - tapOn: Not now\n```\n\nOr let a step fail without failing the test:\n\n```yaml\n- tapOn: { text: Not now, optional: true }\n```\n\n## Find something further down\n\n```yaml\n- scrollUntilVisible: { element: { text: Terms } }\n- tapOn: Terms\n```\n\n## Test without a connection\n\n```yaml\n- network: offline\n- tapOn: { id: refresh }\n- assertVisible: You are offline\n- network: online\n```\n\n## Test an error of the backend\n\n```yaml\n- network: { fail: 503, path: \".*/payments\" }\n- tapOn: { id: pay }\n- assertVisible: Try again\n```\n\n## Check what the app sent\n\n```yaml\n- tapOn: { id: pay }\n- waitForRequest: { method: POST, path: \".*/payments\", body: \"\\\"amount\\\":5\" }\n```\n\n## Refuse a permission\n\n```yaml\n- launchApp: { clearState: true, permissions: { all: allow, camera: deny } }\n```\n\n## Pretend to be somewhere\n\n```yaml\n- setLocation: { latitude: 41.3785, longitude: 2.1765 }\n- travel: { points: [\"41.3785,2.1765\", \"41.3805,2.1748\"], speed: 6 }\n```\n\n## Scan a QR\n\n```yaml\n- camera: qr.png\n- tapOn: { id: scan }\n```\n\nThe emulator's back camera must be *VirtualScene*.\n\n## Record a video\n\n```yaml\n- startRecording: checkout\n- tapOn: { id: pay }\n- stopRecording\n```\n\nFor a whole flow, or all of a suite, see [Recording a whole flow](REFERENCE.md#recording-a-whole-flow).\n\n## Test other screen sizes\n\nWrite a suite for each:\n\n```yaml\nflows: [tests/login.yaml]\ndevice: { size: 720x1600, dpi: 280, fontScale: 1.3, darkMode: true }\n```\n\n## Run only some tests\n\n```\nrayo test --only login,pay all.rayo.yaml\nrayo test --include-tags smoke tests\nrayo test --rerun-failed all.rayo.yaml\n```\n\n## Run in a CI\n\nAny CI with an emulator. The exit code isn't 0 if a test fails, and `.xml` results are JUnit:\n\n```\nrayo check all.rayo.yaml\nrayo test --emulator --results junit.xml all.rayo.yaml\n```\n\n## Complete and check in my editor\n\n```yaml\n# yaml-language-server: $schema=https://raw.githubusercontent.com/nanihadesuka/rayo/main/schema/flow.json\n```\n\nIt needs the repository to be public; until then, use the path of your copy. See\n[Schemas](REFERENCE.md#schemas).\n",
  "MOCKING.md": "# Mocking\n\nrayo answers as the backend. A mock is a folder: `mappings/` says **when** to answer, and `__files/`\nholds **what** to answer. The names are fixed.\n\n```\nmock/\n├─ mappings/                 the rules: if the app calls this, answer that\n│   ├─ products.json\n│   └─ order.json\n└─ __files/                  the bodies, as the backend would send them\n    ├─ products/all.json\n    └─ order/shipped.json\n```\n\nA flow names its mocks, in order, in its header:\n\n```yaml\nmock: [../shared/mock, mock/overrides.json]\n```\n\n## In your app\n\nrayo can't reach inside an app: the app has to send its calls to `http://127.0.0.1:8080` on the device.\nWith OkHttp, an interceptor does it. It can be **only in the debug build** (`src/debug`), and only on\nwhen a test says so, so nothing of it reaches your users.\n\n```kotlin\n/** https://host/path becomes http://127.0.0.1:8080/host/path, when a test asked for it. */\nobject MockRedirect : Interceptor {\n  @Volatile var on = false\n\n  override fun intercept(chain: Interceptor.Chain): Response {\n    val request = chain.request()\n    if (!on) return chain.proceed(request)\n    val url = request.url.newBuilder()\n      .scheme(\"http\").host(\"127.0.0.1\").port(8080)\n      .encodedPath(\"/${request.url.host}${request.url.encodedPath}\")\n      .build()\n    return chain.proceed(request.newBuilder().url(url).build())\n  }\n}\n```\n\nAdd it to the client with `addInterceptor(MockRedirect)`, and turn it on before the first call, from\nwhat `launchApp: { arguments: { mock: true } }` gives:\n\n```kotlin\nMockRedirect.on = intent.getBooleanExtra(\"mock\", false)\n```\n\nAndroid refuses calls that aren't HTTPS, so allow that one, also in `src/debug`, with\n`android:networkSecurityConfig=\"@xml/network_security_config\"`:\n\n```xml\n\u003cnetwork-security-config\u003e\n  \u003cdomain-config cleartextTrafficPermitted=\"true\"\u003e\n    \u003cdomain includeSubdomains=\"false\"\u003e127.0.0.1\u003c/domain\u003e\n  \u003c/domain-config\u003e\n\u003c/network-security-config\u003e\n```\n\nThe host goes in the path so that several can be mocked, and so that a capture knows whom to call:\na mapping for `https://api.example.com/products` has the path `/api.example.com/products`. An app\nwith one backend may leave it out, as the examples here do; to capture it, use `--capture-from`.\n\n## Proxy\n\nThe interceptor only sees the calls made through the app's own client. A third-party SDK that brings its\nown client, such as a payment SDK that polls a URL from an answer, still calls the real server: a capture\nmisses those calls without saying so, and a mock run sends them live. With `--network proxy`, or\n`network: proxy` in a flow's header, the device's HTTP proxy sends every call to rayo instead.\n\n```\nrayo ca export app/src/debug/res/raw/rayo_ca.pem      # once per computer\nrayo test --capture mock/ --network proxy flow.yaml   # capture\nrayo test --network proxy flow.yaml                   # mock\n```\n\n| | `interceptor` (default) | `proxy` |\n|---|---|---|\n| What it sees | Calls made through the app's own client | Every HTTP(S) call on the device: the app, SDKs; WebViews of `--hosts` |\n| In the app | The interceptor, and cleartext to `127.0.0.1` | No code: the debug build trusts rayo's CA |\n| On the device | `adb reverse` | `adb reverse`, and `http_proxy` while the flow runs |\n| Fails when | An SDK uses its own client | A library pins certificates: that host is tunnelled, and said so |\n\nBoth write and read the same folders, with the host in the path, so a capture made one way replays the\nother way. An app that still rewrites its URLs works behind the proxy too.\n\n- **The CA** is made the first time, in `~/.rayo` (`ca.pem`, and `ca-key.pem`, which never leaves it).\n  `rayo ca export` writes the certificate and the `network_security_config` that trusts it in\n  `debug-overrides` only. A release build ignores it.\n- **Hosts.** The proxy decrypts only some hosts and tunnels the rest untouched: those of `--hosts`, and\n  their subdomains; else, capturing, all but Google's, Firebase's, analytics and crash reports; else\n  those the mocks have in their paths. Then a call to a host of the mocks that none fits fails as\n  `not mocked`, and a call to another host goes to the real one. The hosts left alone are listed after the flow.\n- **Browsers.** A Custom Tab or the browser, such as for a login page, trusts only the system's CAs,\n  and would show \"Your connection is not private\". The proxy reads the start of each TLS connection\n  first: Chrome and what is made from it (WebViews, Cronet) send GREASE values, Android's own TLS\n  doesn't. Such a connection is passed untouched, and its hosts listed after the flow, unless the host\n  is in `--hosts`: name it there to capture or mock a WebView's calls.\n- **Pinning.** A host whose TLS the app refuses is listed after the flow as `pinned or untrusted CA:\n  \u003chost\u003e rejected rayo's CA`, its calls neither captured nor mocked, and tunnelled from then on, instead\n  of failing the run. The app needs rayo's CA in its debug `network_security_config`, from `rayo ca\n  export`; a build that pins certificates needs the pins off in debug.\n- **Plain `http://`** passes untouched, and is listed.\n- **Clean-up.** The proxy is set by a shell on the device that puts the one before back when the flow\n  ends, and also when the connection closes, however rayo stopped: a device isn't left with a proxy\n  with nothing behind it.\n\n## What happens on a call\n\n1. The app calls `http://127.0.0.1:8080/...` on the device, which reaches rayo.\n2. rayo looks for a mapping that fits: method, address, query, headers, body and state.\n\n   ```json\n   {\n     \"request\": { \"method\": \"GET\", \"urlPath\": \"/products\" },\n     \"response\": { \"status\": 200, \"headers\": { \"Content-Type\": \"application/json\" }, \"bodyFileName\": \"products/all.json\" }\n   }\n   ```\n\n   If several fit, the lowest `priority` wins (5 by default), and then the last one loaded. So\n   `overrides.json` wins over `shared/mock`, as it comes after.\n3. It answers with `__files/products/all.json`, with what is between `{{ }}` filled in, such as\n   `{{now offset='-8 minutes'}}`.\n4. If none fits, the app gets a 404 and **the flow fails**.\n\n## State\n\nThe same call can be answered differently as things advance. A mapping may belong to a scenario,\nwhich begins in `Started`:\n\n```json\n{ \"mappings\": [\n  { \"scenarioName\": \"order\", \"requiredScenarioState\": \"Started\", \"newScenarioState\": \"paid\",\n    \"request\": { \"method\": \"POST\", \"urlPath\": \"/payments\" },\n    \"response\": { \"status\": 200 } },\n  { \"scenarioName\": \"order\", \"requiredScenarioState\": \"shipped\",\n    \"request\": { \"method\": \"GET\", \"urlPath\": \"/order\" },\n    \"response\": { \"status\": 200, \"bodyFileName\": \"order/shipped.json\" } },\n  { \"priority\": 10,\n    \"request\": { \"method\": \"GET\", \"urlPath\": \"/order\" },\n    \"response\": { \"status\": 200, \"jsonBody\": { \"state\": \"waiting\" } } }\n] }\n```\n\nHere paying moves `order` to `paid`. What the backend does by itself, the flow says:\n`- mockState: { order: shipped }`. The last mapping is for any other state.\n\n## Why two folders\n\n- **Reuse**: several mappings can answer with the same body.\n- **Readable**: the logic isn't buried in long bodies.\n- **Easy to replace**: a body is a real answer, to paste without touching the rules.\n\nIt isn't required: an answer can be in its mapping, with `body` or `jsonBody`. A few lines go there;\nwhat is long or shared goes in `__files/`.\n\n## From the real backend\n\n`rayo test --capture` writes such a folder from a real backend, through the interceptor or the proxy.\nREST and GraphQL calls go in the same folder. A REST call is matched by its method, address, query and\nbody, as WireMock's recorder does, so calls to one endpoint with other bodies get their own answers. A\nGraphQL call is matched by its operation and variables (and, if one\nname is sent with several query documents, by its shape), so a `Wallet` query is never answered with a\n`Geocode` answer, whatever order the app makes them in. See [GraphQL](REFERENCE.md#graphql).\n\nWhen the backend changes, `rayo test --capture-update mock` captures again beside the mocks and says,\nmapping by mapping, what changed; `rayo mocks apply mock` takes all of it, or `--only` some. With\n`--check` it fails instead, for CI. See [Updating mocks](REFERENCE.md#updating-mocks).\n\n## Keeping mocks up to date\n\nA capture is made once. When the app or the backend changes after it, don't capture again from\nnothing: these are the ways to bring the mocks up to date, keeping what still holds and the mappings\nwritten by hand.\n\n**Knowing that they are out of date.** A flow run on its mocks fails when the app calls what no\nmapping answers, and says which calls, and the nearest mapping:\n\n```\nthe app called what isn't mocked:\n    GET /api/users/me/badges\n```\n\nA mapping that is no longer called doesn't fail anything: an update shows it as `gone`.\n\n**The app makes a call more, or one less, or the backend answers otherwise.** Update from the real\nbackend, only the flows that changed, then take what you want:\n\n```sh\nrayo test --capture-update mock --only checkout flows/   # captures into mock/.update, and says what changed\nrayo mocks apply mock                                    # takes the changed and the new\nrayo mocks apply --only \"GET /api/users/me/badges\" mock  # or only these\nrayo mocks apply --prune mock                            # also takes out those gone\n```\n\nThe update makes its mocks as the capture made them (with the settings kept in\n`capture.calls.json`), so a mapping that didn't change is the same, and isn't touched. Look at what\nis printed, or at `mock/.update/changes.txt`, before applying.\n\n**A new flow, on mocks that are there.** Go on with the capture, as if it had been made in one go:\n\n```sh\nrayo test --capture mock --capture-append --only newFlow flows/\n```\n\n**In CI.** Find out, without changing anything, whether the backend still answers as the mocks say:\n\n```sh\nrayo test --capture-update mock --check flows/\n```\n\nEach of these calls the real backend, for the flows it runs: it has to be reachable, with the\naccounts and data the flows need. `--only` keeps that to the flows that changed. Commit the mocks\nafter `rayo mocks apply`, and leave `mock/.update/` out (add it to `.gitignore`).\n\nAll that a mapping can say, and\nthe commands `mock`, `mockState`, `network` and `waitForRequest`, are in the\n[reference](REFERENCE.md#mock).\n\n## Coming from WireMock or mitmproxy\n\nA mocks folder of WireMock's, `mappings/` and `__files/`, is one of rayo's: give it to a flow's `mock`\nas it is. rayo's captures load in WireMock too: they use only its matchers. What rayo can't do, such as\nwebhooks or proxying to a real backend, is an error when the mocks load, not a mock that does less.\nAn app that calls one backend, with `--capture-from`, has paths without the host, as WireMock's; through\nthe proxy, a mapping without the host answers too.\n\n| WireMock recording | rayo |\n|---|---|\n| `POST /__admin/recordings/start`, then `snapshot` | `rayo test --capture mock flow.yaml` |\n| `requestBodyPattern: auto` (`equalToJson` with `ignoreArrayOrder` and `ignoreExtraElements`, `equalToXml`, `equalTo`) | The default; `--rest off` for none |\n| `repeatsAsScenarios: true` | Always: the same call answered otherwise gets states, of the scenario `capture` |\n| `captureHeaders` | `--capture-headers Accept-Language,X-Api-Version` |\n| `extractBodyCriteria` | Always: every body goes in `__files/` |\n| `persist`, `--root-dir` | The folder of `--capture` |\n| Near misses | Said when a call isn't mocked: the nearest mapping, and what of it didn't fit |\n| `--global-response-templating` | Mappings without `transformers`; with `\"transformers\": [\"response-template\"]`, as WireMock's local mode |\n\n| mitmproxy server replay | rayo |\n|---|---|\n| `mitmdump -w flows` | `rayo test --capture mock --network proxy flow.yaml`, or `rayo mocks import flows.har mock` with `mitmdump -nr flows --set hardump=flows.har` |\n| `mitmdump --server-replay flows` | `rayo test flow.yaml`, with `mock: mock` in its header |\n| `server_replay_ignore_params` | `--ignore-params a,b` |\n| `server_replay_ignore_payload_params` | `--ignore-body a,b`, also for JSON and GraphQL variables |\n| `server_replay_ignore_content` | `--rest off` |\n| `server_replay_ignore_host` | `--ignore-host` |\n| Answers given once each, in order (`server_replay_reuse` off) | States: a call made three times is answered as the three times were; after that, as the last |\n| `server_replay_kill_extra` | Always: a call that isn't mocked fails the flow |\n| `server_replay_refresh` | `{{now offset='...'}}` in a body |\n\nDates and times are matched by their form, not their value, so a call made with the time of the run\nstill fits; `--timestamps exact` to match them as they were. All these settings are kept with the\ncapture, and used again by `--capture-append` and `--capture-update`.\n",
  "REFERENCE.md": "# rayo reference\n\nEvery command and feature. To begin, see the [README](README.md).\n\n🧪 not tested on a device yet. `rayo help \u003ccommand\u003e` shows what one takes.\n\nThe tables between `generated` marks come from the code, as do `rayo help` and the JSON Schemas\n(`rayo schema flow`, `suite` or `results`; also in [schema/](schema)). `go test ./test -update` writes them.\n\n## Index\n\n| Topic | |\n|---|---|\n| [Options](#options) | As `rayo help` prints them |\n| [Flow file](#flow-file) | [Header](#header) · [Common options](#common-options) · [Values](#values) |\n| [Selectors](#selectors) | [Conditions](#conditions) · [Relations](#relations) |\n| [Apps](#apps) | [launchApp](#launchapp) · [stopApp](#stopapp-killapp) · [killApp](#stopapp-killapp) · [clearState](#clearstate) · [setPermissions](#setpermissions) · [openLink](#openlink) |\n| [Touch](#touch) | [tapOn](#tapon) · [doubleTapOn](#doubletapon) · [longPressOn](#longpresson) · [swipe](#swipe) · [scroll](#scroll) · [scrollUntilVisible](#scrolluntilvisible) |\n| [Text and keys](#text-and-keys) | [type](#type) · [inputText](#inputtext) · [inputRandom…](#inputrandom) · [eraseText](#erasetext) · [copyTextFrom](#copytextfrom-pastetext) · [pasteText](#copytextfrom-pastetext) · [hideKeyboard](#hidekeyboard) · [pressKey](#presskey) · [back](#back) |\n| [Checks and waits](#checks-and-waits) | [assertVisible](#assertvisible) · [assertNotVisible](#assertnotvisible) · [extendedWaitUntil](#extendedwaituntil) · [assertTrue](#asserttrue) · [waitForAnimationToEnd](#waitforanimationtoend) |\n| [Control](#control) | [runFlow](#runflow) · [repeat](#repeat) · [retry](#retry) · [when](#when) |\n| [Scripts](#scripts) | [Expressions](#expressions) · [runScript](#runscript) · [evalScript](#evalscript) · [JavaScript API](#javascript-api) |\n| [Device](#device) | [setLocation](#setlocation) · [travel](#travel) · [setAirplaneMode](#setairplanemode) · [takeScreenshot](#takescreenshot) · [startRecording](#startrecording-stoprecording) · [stopRecording](#startrecording-stoprecording) · [camera](#camera) · [shell](#shell) |\n| [Server](#server) | [http](#http) |\n| [Mock](#mock) | [Files](#mock-files) · [mock](#mock-step) · [mockState](#mockstate) · [network](#network) · [waitForRequest](#waitforrequest) · [Capture](#capture) |\n| [Running several](#running-several) | [Suites](#suites) · [Clean state](#clean-state) · [Retries](#retries) · [Causes](#causes) · [Outcomes](#outcomes) · [Results](#results) |\n| [Live sessions](#live-sessions) | [Commands](#live-commands) · [From other programs](#from-other-programs) |\n| [Features](#features) | [Schemas](#schemas) · [Variables](#variables) · [Tags](#tags) · [Continuous mode](#continuous-mode) · [Failures](#failures) · [Timings](#timings) · [Profile](#profile) · [Report](#report) · [Environment](#environment) |\n| [Default times](#default-times) · [Limitations](#limitations) | |\n\n---\n\n## Options\n\nAs `rayo help` prints them. An option of another command is an error, never ignored.\n\n```\nrayo - runs flows on Android, fast\n\n  rayo test [options] \u003cflow, folder or suite\u003e...   run flows; a suite is a *.rayo.yaml\n  rayo check \u003cflow, folder or suite\u003e...            check flows without a device\n  rayo live [options] \u003cflow\u003e                       write a flow as it runs, command by command\n  rayo do [options] \u003ccommands\u003e                     send commands to the live session; rayo do .help\n  rayo hierarchy [options]                         what is on screen, and how to refer to it\n  rayo devices                                     connected devices\n  rayo mocks apply [options] \u003cfolder\u003e              take the changes of rayo test --capture-update into the mocks\n  rayo mocks import [options] \u003cfile.har\u003e \u003cfolder\u003e  write the calls of a HAR file, such as mitmproxy's, as mocks\n  rayo ca export \u003cfile\u003e                            the certificate the proxy signs with, for the app's debug build\n  rayo help \u003ccommand\u003e                              what a command takes, of a flow or of rayo; also flow, suite and json\n  rayo schema flow, suite or results               its JSON Schema, for editors and programs\n  rayo version\n\nOptions of test, live and hierarchy\n  --device \u003cserial\u003e          when several are connected\n  --emulator                 never a real device, even if it is the only one\n\nOptions of test, live and do\n  --output \u003cfolder\u003e          where failures and screenshots go (default .rayo)\n\nOptions of test and live\n  -e, --env \u003cKEY=VALUE\u003e      a variable for the flows\n  --timeout-scale \u003cfactor\u003e   multiply the default waits, not a step's own timeout (default 1)\n  --mock-port \u003cport\u003e         where the app finds the mock on the device (default 8080)\n  --animations               leave the device's animations on\n  --profile                  where the time of each step went\n\nOptions of test: which flows\n  --only \u003ca,b\u003e               only the flows with these names\n  --include-tags \u003ca,b\u003e       only flows with one of these tags\n  --exclude-tags \u003ca,b\u003e       not flows with one of these tags\n  --rerun-failed             only those that haven't passed since they last failed\n\nOptions of test: how they run\n  --retries \u003cn\u003e              try again what fails, whatever the suite says (default 1)\n  --fail-fast                stop at the first failure\n  --keep-going               don't stop when the device or the app fails over and over\n  --shuffle[=seed]           in a random order, or in that of an earlier run\n  -c, --continuous           run again whenever a file changes\n\nOptions of test: what comes out\n  -v, --verbose              every step (always, for a single flow)\n  --results \u003cfile\u003e           write the results: .txt, .json or .xml (JUnit); may be repeated\n  --timings                  each step's time, each runFlow's total, the slowest steps and flows, and a note while a step hangs\n  --failures-only            leave the flows that passed out of the results\n\nOptions of test and live: mocks from the real backend\n  --capture \u003cfolder\u003e         call the real backend, and write what it answers as mocks; in live, on .quit, without the calls of steps that failed\n  --capture-update \u003cfolder\u003e  capture into folder/.update, and say what changed from the mocks in folder; rayo mocks apply takes it\n  --check                    with --capture-update, fail if the mocks differ from the backend, and change nothing\n  --capture-append           go on with the capture already in the folder, as if in one go\n  --redact-value \u003cname=value\u003e what a private field of a body becomes, such as phone=+4915100000000 (default one of the same form: a date stays a date); again for more\n\nOptions of test, live and mocks: how a capture or an import becomes mocks\n  --capture-from \u003curl\u003e       the backend, if calls don't begin with /host/\n  --graphql \u003cmatch\u003e          tell GraphQL calls apart by operation+variables (default), by operation, or off, as any call\n  --graphql-path \u003cregex\u003e     paths of GraphQL calls, besides those whose body has query or operationName (default .*/graphql)\n  --rest \u003cmatch\u003e             match other calls by their body too (body, default), or by method and address alone (off)\n  --ignore-params \u003ca,b\u003e      names in the query not to match, such as session or depart_at\n  --ignore-body \u003ca,b\u003e        names, or paths such as trip.departAt, in JSON bodies and GraphQL variables, and names in forms, not to match\n  --timestamps \u003cmatch\u003e       match dates and times in the query and bodies by their form (format, default), or exactly\n  --capture-headers \u003ca,b\u003e    headers of the request to match too, such as Accept-Language\n  --ignore-host              match calls whose path begins with a host, whatever the host\n  --ignore-path-ids          match UUIDs and ULIDs in paths by their form, as they are new on each run\n  --stateless \u003ca,b\u003e          paths such as api.shop.com/quotes/** whose calls only read, whatever the method: they don't change the state (* any text but /, ** any)\n\nOptions of test and live: how calls reach the mock\n  --network \u003cway\u003e            interceptor, the app sends them (default, or the flow's); proxy, the device's proxy does\n  --ignore-unmocked \u003ca,b\u003e    calls without a mock that don't fail the flow, answered 404: graphql:Operation, host/path, host or /path (* any text but /, ** any)\n  --hosts \u003ca,b\u003e              with a proxy, the hosts to open, and their subdomains (default: capturing, all but Google's, Firebase's, analytics and crash reports; else those of the mocks)\n\nOptions of live\n  --app \u003cpackage\u003e            of a flow that isn't there yet\n  --mock \u003ca,b\u003e               its mocks, relative to it\n  --no-run                   don't begin by running the steps it has\n  --serve                    only take commands from rayo do, until .quit\n\nOptions of mocks\n  --only \u003ca,b\u003e               the changes whose name has one of these, such as graphql:Wallet or GET /profile (default all)\n  --prune                    also take out the mocks that weren't called\n\nOptions of do\n  --screen                   also what is on screen afterwards\n  --json                     for a program\n```\n\n---\n\n## Flow file\n\nTwo parts separated by `---`: a header and a list of commands.\n\n```yaml\nappId: com.example.app\n---\n- launchApp\n- tapOn: Log in\n```\n\n### Header\n\n\u003c!-- generated: header --\u003e\n| Key | Takes | |\n|---|---|---|\n| `appId` | package | The app's |\n| `name` | text | Shown in the output (default: the file's, or the folder's for `flow.yaml`) |\n| `tags` | a list | To choose flows, see [Tags](#tags) |\n| `env` | values | Variables for it, see [Variables](#variables) |\n| `mock` | a path or a list | Who answers as the backend, see [Mock](#mock); `${...}` too, where an empty path loads nothing |\n| `network` | `interceptor` or `proxy` | How the app's calls reach the mock, see [Proxy](#proxy); `${...}` too, such as `${NETWORK}` from `-e`, where empty is the default (default: `interceptor`, or `--network`) |\n| `record` | `true`, `false` or a file | A video of the flow, see [Recording a whole flow](#recording-a-whole-flow) (default: the suite's) |\n| `onFlowStart` | commands | Run first |\n| `onFlowComplete` | commands | Run last, also after a failure |\n\u003c!-- end --\u003e\n\n### Common options\n\nAccepted by every command that takes a mapping.\n\n\u003c!-- generated: options --\u003e\n| Key | Takes | |\n|---|---|---|\n| `label` | text | What to show instead of the command |\n| `optional` | true/false | A failure doesn't fail the flow |\n| `timeout` | time | How long it may take |\n\u003c!-- end --\u003e\n\n### Values\n\n| Value | Written as |\n|---|---|\n| Time | `500` (milliseconds) or `10s` |\n| Point | `35%, 30%` of the screen, or `540, 1200` pixels |\n| Location | `41.3785, 2.1765` (latitude, longitude) |\n| Short form | `- tapOn: Log in` is `- tapOn: { text: Log in }` |\n\n---\n\n## Selectors\n\nHow a command refers to an element. All the conditions given must hold. Run `rayo hierarchy` to\nsee what is on screen.\n\n### Conditions\n\n\u003c!-- generated: selector --\u003e\n| Key | Takes | |\n|---|---|---|\n| `id` | text | The resource id, whole or after :id/; in Compose, a testTag with testTagsAsResourceId. A regular expression of the whole |\n| `text` | text | The text, description or hint. A regular expression of the whole |\n| `index` | number | The n-th match, from 0, top to bottom and left to right; negative counts from the end |\n| `enabled` | true/false | Whether it is enabled |\n| `checked` | true/false | Whether it is checked |\n| `focused` | true/false | Whether it has the focus |\n| `selected` | true/false | Whether it is selected |\n| `below` | selector | Entirely below another |\n| `above` | selector | Entirely above another |\n| `leftOf` | selector | Entirely to the left of another |\n| `rightOf` | selector | Entirely to the right of another |\n| `containsChild` | selector | The direct parent of another |\n| `childOf` | selector | Anywhere inside another |\n| `containsDescendants` | selectors | Around all of these |\n\u003c!-- end --\u003e\n\n`id` and `text` are regular expressions that must match the **whole** value, ignoring case. An\nexact value always matches too, so `Price (€)` needs no escaping.\n\nWith several matches and no `index`, the first in drawing order of the topmost window is used. If one match contains\nanother, the inner one wins.\n\n### Relations\n\n🧪 Each takes another selector.\n\n```yaml\n- tapOn: { text: Delete, below: { id: row_title, text: Invoice 7 } }\n```\n\n---\n\n## Apps\n\n### launchApp\n\nOpens the app and waits until Android has it in front.\n\n\u003c!-- generated: keys launchApp --\u003e\n| Key | Takes | |\n|---|---|---|\n| `appId` | text | The app; by default the header's. Short form |\n| `clearState` | true/false | Erase its data first (false) |\n| `stopApp` | true/false | Close it first (true); false brings it back as it was |\n| `permissions` | permissions | Name: allow, deny or unset. With clearState, all are allowed if not said |\n| `arguments` | values | Intent extras; booleans, integers and decimals keep their type |\n\u003c!-- end --\u003e\n\nPermission names: `all`, `location`, `camera`, `notifications`, `contacts`, `microphone`,\n`bluetooth`, `storage`, `phone`, `calendar`, `sms`, `medialibrary`, or Android's own, such as\n`READ_CONTACTS`. Only `all` has been tested.\n\n```yaml\n- launchApp: { clearState: true, permissions: { all: allow }, arguments: { mock: true } }\n```\n\n### stopApp, killApp\n\n🧪 Both close the app. Short form: the package.\n\n\u003c!-- generated: keys stopApp --\u003e\n| Key | Takes | |\n|---|---|---|\n| `appId` | text | The app; by default the header's. Short form |\n\u003c!-- end --\u003e\n\n### clearState\n\n🧪 Erases the app's data. Short form: the package.\n\n\u003c!-- generated: keys clearState --\u003e\n| Key | Takes | |\n|---|---|---|\n| `appId` | text | The app; by default the header's. Short form |\n\u003c!-- end --\u003e\n\n### setPermissions\n\nChanges permissions without opening the app, in the order given. Takes `permissions`, as in\n[launchApp](#launchapp), and `appId`. Fails if the app has no such permission. Android closes a\nrunning app when it loses one.\n\n\u003c!-- generated: keys setPermissions --\u003e\n| Key | Takes | |\n|---|---|---|\n| `appId` | text | The app; by default the header's |\n| `permissions` | permissions | Name: allow, deny or unset. With clearState, all are allowed if not said |\n\u003c!-- end --\u003e\n\n```yaml\n- setPermissions: { permissions: { all: deny, camera: allow } }\n```\n\n### openLink\n\n🧪 Opens a link in whichever app handles it.\n\n\u003c!-- generated: keys openLink --\u003e\n| Key | Takes | |\n|---|---|---|\n| `link` | text | What to open. Short form |\n\u003c!-- end --\u003e\n\n```yaml\n- openLink: https://example.com/item/7\n```\n\n---\n\n## Touch\n\n### tapOn\n\nWaits until the element is on screen, enabled and not moving, then taps its centre.\n\n\u003c!-- generated: keys tapOn --\u003e\n| Key | Takes | |\n|---|---|---|\n| `point` | point | Alone, of the screen; with a selector, of the element in % |\n| `repeat` | number | Taps (1) |\n| `delay` | time | Between taps (100) |\n\u003c!-- end --\u003e\n\n```yaml\n- tapOn: { id: save }\n- tapOn: { point: \"35%, 30%\" }\n```\n\n### doubleTapOn\n\n🧪 Like `tapOn`, twice. `delay` is 100 ms by default.\n\n\u003c!-- generated: keys doubleTapOn --\u003e\n| Key | Takes | |\n|---|---|---|\n| `point` | point | Alone, of the screen; with a selector, of the element in % |\n| `delay` | time | Between taps (100) |\n\u003c!-- end --\u003e\n\n### longPressOn\n\n🧪 Like `tapOn`, held for a second.\n\n\u003c!-- generated: keys longPressOn --\u003e\n| Key | Takes | |\n|---|---|---|\n| `point` | point | Alone, of the screen; with a selector, of the element in % |\n\u003c!-- end --\u003e\n\n### swipe\n\n🧪\n\n\u003c!-- generated: keys swipe --\u003e\n| Key | Takes | |\n|---|---|---|\n| `direction` | UP/DOWN/LEFT/RIGHT | Where the finger or the search goes (DOWN when searching) |\n| `start` | point | Where the finger goes down, instead of direction |\n| `end` | point | Where it goes up |\n| `duration` | time | Of the swipe (250) |\n| `from` | selector | Start on this element; with direction |\n\u003c!-- end --\u003e\n\n### scroll\n\n🧪 Scrolls down once.\n\n### scrollUntilVisible\n\n🧪 Scrolls until the element is on screen.\n\n\u003c!-- generated: keys scrollUntilVisible --\u003e\n| Key | Takes | |\n|---|---|---|\n| `element` | selector | What to look for |\n| `direction` | UP/DOWN/LEFT/RIGHT | Where the finger or the search goes (DOWN when searching) |\n\u003c!-- end --\u003e\n\n---\n\n## Text and keys\n\n### type\n\nReplaces the text of a field at once, without focus or keyboard. The quickest way to fill a\nform; `pressKey: Enter` won't follow it, as the field isn't focused.\n\n\u003c!-- generated: keys type --\u003e\n| Key | Takes | |\n|---|---|---|\n| `value` | text | The new text |\n\u003c!-- end --\u003e\n\n```yaml\n- type: { id: email, value: me@example.com }\n```\n\n### inputText\n\nAdds text to the focused field, waiting for one to be focused.\n\n\u003c!-- generated: keys inputText --\u003e\n| Key | Takes | |\n|---|---|---|\n| `text` | text | The text to add. Short form |\n\u003c!-- end --\u003e\n\n```yaml\n- tapOn: { id: email }\n- inputText: me@example.com\n```\n\n### inputRandom…\n\n🧪 Like `inputText`, with something random.\n\n| Command | |\n|---|---|\n| `inputRandomEmail` | |\n| `inputRandomPersonName` | |\n| `inputRandomNumber` | `length`, 8 by default |\n| `inputRandomText` | `length`, 8 by default |\n\n### eraseText\n\n🧪 Deletes characters from the end of the focused field: 50, or the number given.\n\n\u003c!-- generated: keys eraseText --\u003e\n| Key | Takes | |\n|---|---|---|\n| `charactersToErase` | number | How many (50). Short form |\n\u003c!-- end --\u003e\n\n### copyTextFrom, pasteText\n\n🧪 `copyTextFrom` keeps an element's text, or its description if it has none, in\n`rayo.copiedText`. `pasteText` adds it to the focused field.\n\n### hideKeyboard\n\n🧪 Closes the keyboard if it is open with one Back, and waits until it has. Does nothing when none is\nopen. It never sends a second Back, which could close the window under the keyboard, such as a\nCustom Tab: if the keyboard is still open at the `timeout`, it fails. To try again, use `back` or\n`retry`. A keyboard is open when the input method's window is at least an eighth of the screen: with\na hardware keyboard, the small toolbar the input method may show isn't one, so nothing is sent.\n\n### pressKey\n\n`Home`, `Back`, `Enter`, `Tab`, `Backspace`, `Delete`, `Escape`, `Space`, `Menu`, `Search`,\n`Recents`, `Power`, `Lock`, `Volume Up`, `Volume Down`, `Remote Dpad …`, `Remote Media …`, or an\nAndroid key code. Only `Home` and `Enter` have been tested.\n\n\u003c!-- generated: keys pressKey --\u003e\n| Key | Takes | |\n|---|---|---|\n| `key` | key | A name, such as Enter, or an Android key code. Short form |\n\u003c!-- end --\u003e\n\n`Home` also waits until the app has really stopped, so that opening it again counts.\n\n### back\n\n`pressKey: Back`.\n\n---\n\n## Checks and waits\n\nEach returns as soon as it holds; none needs a fixed pause.\n\n### assertVisible\n\n🧪 The element is on screen, or appears in time.\n\n### assertNotVisible\n\n🧪 The element isn't on screen, or goes away in time.\n\n### extendedWaitUntil\n\nThe same two, usually with a longer `timeout`.\n\n\u003c!-- generated: keys extendedWaitUntil --\u003e\n| Key | Takes | |\n|---|---|---|\n| `visible` | selector | On screen |\n| `notVisible` | selector | Not on screen |\n\u003c!-- end --\u003e\n\n```yaml\n- extendedWaitUntil: { visible: { id: home }, timeout: 20s }\n- extendedWaitUntil: { notVisible: Loading.* }\n```\n\n### assertTrue\n\n🧪 An [expression](#expressions) is true.\n\n\u003c!-- generated: keys assertTrue --\u003e\n| Key | Takes | |\n|---|---|---|\n| `condition` | JavaScript | An expression. Short form |\n\u003c!-- end --\u003e\n\n```yaml\n- assertTrue: ${output.total == 3}\n```\n\n### waitForAnimationToEnd\n\n🧪 Waits until nothing on screen has changed for `quiet` (1 s). Goes on when the `timeout` is\nover, without failing.\n\nA quiet screen isn't a finished transition: while the app waits for the network, nothing moves, so\nthis returns, and a dialog that comes after may still be missing. It can't know about what hasn't\nbegun. To wait for something late, wait for it by name, as `extendedWaitUntil`, or a `when` with a\nlonger `timeout`:\n\n```yaml\n- tapOn: Continue\n- runFlow:\n    when: { visible: Allow notifications, timeout: 5s }\n    commands: [ { tapOn: Not now } ]\n- assertVisible: Home\n```\n\n\u003c!-- generated: keys waitForAnimationToEnd --\u003e\n| Key | Takes | |\n|---|---|---|\n| `quiet` | time | How long nothing may change (1s) |\n\u003c!-- end --\u003e\n\n```yaml\n- waitForAnimationToEnd: { quiet: 1s, timeout: 5s }\n```\n\n---\n\n## Control\n\n### runFlow\n\n\u003c!-- generated: keys runFlow --\u003e\n| Key | Takes | |\n|---|---|---|\n| `file` | text | A path relative to this flow. Short form |\n| `env` | values | Variables for it |\n| `when` | conditions | Only if these hold |\n| `commands` | commands | What to run |\n\u003c!-- end --\u003e\n\n```yaml\n- runFlow: ../shared/login.yaml\n```\n\n### repeat\n\n🧪 Runs `commands` again, up to `times`, while [`while`](#when) holds. One of the two is required.\n\n\u003c!-- generated: keys repeat --\u003e\n| Key | Takes | |\n|---|---|---|\n| `times` | number | At most |\n| `while` | conditions | As long as these hold |\n| `commands` | commands | What to run |\n\u003c!-- end --\u003e\n\n### retry\n\n🧪 Runs `commands` or a `file`, again if it fails, up to `retries` (1) more times.\n\n\u003c!-- generated: keys retry --\u003e\n| Key | Takes | |\n|---|---|---|\n| `retries` | number | How many more times (1) |\n| `commands` | commands | What to run |\n| `file` | text | A path relative to this flow |\n| `env` | values | Variables for it |\n\u003c!-- end --\u003e\n\n### when\n\n🧪 A condition for `runFlow`, `runScript` and, as `while`, `repeat`. All must hold. `visible` and\n`notVisible` wait up to `timeout` (0.7 s) for it; `timeout: 0` looks once, for a guard that\nusually doesn't hold.\n\n```yaml\n- runFlow:\n    when: { visible: \"Allow notifications\", timeout: 3s }\n    commands: [ { tapOn: \"Not now\" } ]\n```\n\n\u003c!-- generated: when --\u003e\n| Key | Takes | |\n|---|---|---|\n| `visible` | selector | On screen |\n| `notVisible` | selector | Not on screen |\n| `platform` | Android/iOS/Web | Android |\n| `true` | JavaScript | An expression that holds |\n| `timeout` | time | How long visible and notVisible may take (700ms); 0 looks once |\n\u003c!-- end --\u003e\n\n---\n\n## Scripts\n\n### Expressions\n\n`${…}` is JavaScript, anywhere in a value. Something undefined is an error, not the word\n\"undefined\".\n\n```yaml\n- inputText: ${output.user.name}\n```\n\n### runScript\n\nRuns a file. `env` gives it variables. What it declares stays inside: share through `output`.\n\n\u003c!-- generated: keys runScript --\u003e\n| Key | Takes | |\n|---|---|---|\n| `file` | text | A path relative to this flow. Short form |\n| `env` | values | Variables for it |\n| `when` | conditions | Only if these hold |\n\u003c!-- end --\u003e\n\n```yaml\n- runScript: { file: create_user.js, env: { NAME: Ana } }\n```\n\n### evalScript\n\n🧪 Runs a line.\n\n\u003c!-- generated: keys evalScript --\u003e\n| Key | Takes | |\n|---|---|---|\n| `script` | JavaScript | A line of it. Short form |\n\u003c!-- end --\u003e\n\n```yaml\n- evalScript: ${output.count = 1}\n```\n\n### JavaScript API\n\n| | |\n|---|---|\n| `output` | An object shared by the whole flow |\n| `http.get/post/put/delete(url, options)`, `http.request(url, options)` | `options`: `headers`, `body`, `method`. Returns `ok`, `status`, `body` |\n| `json(text)` | Parses |\n| `console.log(…)` | Prints |\n| `rayo.platform`, `rayo.copiedText` | |\n\nOnly `http.put` has been tested. A script may take up to 60 seconds. There is no `require`, `fetch` or `async`.\n\n---\n\n## Device\n\n### setLocation\n\nStays at a place, sending a fix every second like a real GPS, until the flow ends. Needs\nAndroid 12.\n\n\u003c!-- generated: keys setLocation --\u003e\n| Key | Takes | |\n|---|---|---|\n| `latitude` | number | In degrees |\n| `longitude` | number | In degrees |\n\u003c!-- end --\u003e\n\n```yaml\n- setLocation: { latitude: 41.3785, longitude: 2.1765 }\n```\n\n### travel\n\n🧪 Moves along `points` at `speed` metres per second (7 by default), then stays at the last.\n\n\u003c!-- generated: keys travel --\u003e\n| Key | Takes | |\n|---|---|---|\n| `points` | locations | A list of 'latitude,longitude' |\n| `speed` | number | Metres per second (7) |\n\u003c!-- end --\u003e\n\n```yaml\n- travel: { points: [\"41.3785,2.1765\", \"41.3805,2.1748\"], speed: 6 }\n```\n\n### setAirplaneMode\n\n🧪 `enabled` or `disabled`. The end of the flow puts it back as it was.\n\n\u003c!-- generated: keys setAirplaneMode --\u003e\n| Key | Takes | |\n|---|---|---|\n| `value` | enabled/disabled/true/false | enabled or disabled. Short form |\n\u003c!-- end --\u003e\n\n### takeScreenshot\n\n🧪 Saves `\u003coutput\u003e/\u003cflow\u003e/\u003cname\u003e.png`.\n\n\u003c!-- generated: keys takeScreenshot --\u003e\n| Key | Takes | |\n|---|---|---|\n| `path` | text | The file, in the debug output. Short form |\n\u003c!-- end --\u003e\n\n### startRecording, stopRecording\n\nRecords the screen as an mp4. The recording ends with `stopRecording` or with the flow, also if it\nfails. Without a path it has the flow's name.\n\n\u003c!-- generated: keys startRecording --\u003e\n| Key | Takes | |\n|---|---|---|\n| `path` | text | The file, in the debug output; by default the flow's name. Short form |\n\u003c!-- end --\u003e\n\n```yaml\n- startRecording                      # \u003coutput\u003e/\u003cflow\u003e/\u003cflow\u003e.mp4\n- startRecording: login               # \u003coutput\u003e/\u003cflow\u003e/login.mp4\n- startRecording: { path: login }     # the same\n- stopRecording\n```\n\n#### Recording a whole flow\n\n`record` in its header; or in a [suite](#suites), for those of its flows that don't say:\n\n| Where | | Saved as |\n|---|---|---|\n| Flow header | `record: true` | `\u003coutput\u003e/\u003cflow\u003e/\u003cflow\u003e.mp4` |\n| | `record: name`, or an absolute path | `\u003coutput\u003e/\u003cflow\u003e/name.mp4`, or that path |\n| | `record: false` | Not recorded, whatever the suite says |\n| Suite | `record: true` | `\u003coutput\u003e/\u003cflow\u003e/\u003cflow\u003e.mp4` |\n| | and `recordFolder: videos`, relative to the suite | `videos/\u003cflow\u003e.mp4` |\n\nEach try of a flow has its own, such as `\u003cflow\u003e_retry1.mp4`. One recording at a time: with one in\nthe header, another needs `stopRecording` first. At most 30 minutes, or 3 before Android 11 or so;\nthe rest isn't recorded. Without sound, and small on purpose: 480 pixels on the short side, 0.6 Mbps,\nwhich is under 5 MB a minute. The device encodes it as it records.\n\n### camera\n\nHangs a picture on the wall of the emulator's virtual room and walks up to it, such as a QR to\nscan. The walk starts 3 seconds later, while the flow goes on, so use it early. `none` puts the\nroom back, and so does the end of the flow.\n\n\u003c!-- generated: keys camera --\u003e\n| Key | Takes | |\n|---|---|---|\n| `image` | text | What it sees, a path relative to this flow; or none. Short form |\n\u003c!-- end --\u003e\n\n```yaml\n- camera: device/qr.png\n```\n\nThe emulator's back camera must be set to *VirtualScene*. On a real device, or if the emulator\ndoesn't answer, the step says so and passes.\n\n### shell\n\nRuns a command on the device as the shell user. Fails if its exit code isn't 0.\n\n\u003c!-- generated: keys shell --\u003e\n| Key | Takes | |\n|---|---|---|\n| `command` | text | Run as the shell user. Short form |\n\u003c!-- end --\u003e\n\n```yaml\n- shell: am broadcast -a com.example.REFRESH -p com.example.app\n```\n\n---\n\n## Server\n\n### http\n\n🧪 Calls a server from the computer, not the device.\n\n\u003c!-- generated: keys http --\u003e\n| Key | Takes | |\n|---|---|---|\n| `method` | text | Such as POST (GET) |\n| `url` | text | What to call, from the computer. Short form |\n| `json` | values | The body |\n| `expect` | number | The status (any below 300) |\n\u003c!-- end --\u003e\n\n```yaml\n- http: { method: PUT, url: \"http://127.0.0.1:8080/state\", json: { state: docked } }\n```\n\n---\n\n## Mock\n\nrayo answers as the backend while a flow runs. The app must send its calls to\n`http://127.0.0.1:8080` on the device (another port with `--mock-port`); rayo sets up\n`adb reverse` and removes it. HTTP only. Every flow starts afresh, and **fails if the app calls\nsomething without a mock**.\n\n```yaml\nappId: com.example.app\nmock:\n  - ../shared/mock          # the normal answers\n  - overrides.json          # what differs here\n---\n- tapOn: { id: pay }\n- waitForRequest: { method: POST, path: \".*/payments\" }\n- mockState: { order: shipped }\n- network: offline\n```\n\nAn included flow's mocks are added when it runs. `mock` and `network` may use `${...}`, from `-e`\nor a suite's `env`, so that one suite runs both mocked and against the real backend, with no flow\naround each: a path that comes out empty loads nothing, and an empty `network` is the default.\n\n```yaml\nmock: ${MOCKS ? MOCKS + '/' + rayo.file : ''}   # rayo test -e MOCKS=mocks -e NETWORK=proxy all.rayo.yaml\nnetwork: ${NETWORK || ''}\n```\n\nHow mocks work, with examples:\n[Mocking](MOCKING.md).\n\n### Mock files\n\nA folder with `mappings/` and `__files/`, or one JSON file of mappings, in WireMock's format: a folder\nWireMock recorded loads as it is, and WireMock loads rayo's captures. Only the following is supported;\nanything else, such as webhooks or `proxyBaseUrl`, is an error rather than a mock that does less.\n\n| Part | |\n|---|---|\n| Mapping | `id`, `uuid`, `name`, `priority`, `scenarioName`, `requiredScenarioState`, `newScenarioState`, `request`, `response`; `persistent`, `insertionIndex` and `metadata` are taken and do nothing |\n| `request` | `method`; one of `url` `urlPattern` `urlPath` `urlPathPattern` `urlPathTemplate` (`/users/{id}`, with `pathParameters`); `queryParameters`, `headers`, `cookies`, `formParameters`, `basicAuthCredentials`, `host`, `bodyPatterns` |\n| Matchers | `equalTo` (and `caseInsensitive`), `contains`, `doesNotContain`, `matches`, `doesNotMatch`, `absent`, `anything`, `equalToJson` (and `ignoreArrayOrder`, `ignoreExtraElements`), `equalToXml`, `binaryEqualTo`, `matchesJsonPath`, `and`, `or`, `not`; for a query, header, cookie or form, also `hasExactly` and `includes` |\n| `equalToJson` | A JSON value, or a text of one. Keys in any order, numbers by value. Placeholders: `${json-unit.ignore}`, `${json-unit.ignore-element}` (may be absent), `${json-unit.any-string}`, `${json-unit.any-number}`, `${json-unit.any-boolean}`, `${json-unit.regex}pattern` |\n| `matchesJsonPath` | An expression, true if it finds something not empty; or `{ \"expression\": \"$.a\", \"equalTo\": \"x\" }` with any matcher for what it finds. Jayway's JSONPath: `$.a`, `$['a']`, `$[0]`, `$[-1]`, `$[0,2]`, `$[1:3]`, `$.*`, `$..a`, `length()`, and filters anywhere, with `==` `!=` `\u003c` `\u003e` `\u0026\u0026` `=~ /re/` `in` `nin` `size` `empty` `subsetof` `anyof` `noneof` |\n| `response` | `status`, `statusMessage`, `headers` (a value, or a list of them), one of `body` `jsonBody` `base64Body` `bodyFileName`, `fixedDelayMilliseconds`, `delayDistribution` (`lognormal`, `uniform`, `fixed`), `chunkedDribbleDelay`, `fault` (`CONNECTION_RESET_BY_PEER`, `EMPTY_RESPONSE`, `MALFORMED_RESPONSE_CHUNK`, `RANDOM_DATA_THEN_CLOSE`), `transformers`, `transformerParameters` |\n| In a body | `{{now}}` (with `offset='-8 minutes'`, `format='yyyy-MM-dd'` or `epoch` or `unix`, `timezone='Europe/Madrid'`), `{{request.method}}` `.path` `.url` `.body` `.query.x` `.query.x.[1]` `.headers.x` `.cookies.x` `.pathSegments.[0]`, `{{jsonPath request.body '$.a'}}`, `{{randomValue length=8 type='ALPHANUMERIC'}}` (or `UUID`, `NUMERIC`...), `{{regexExtract request.path '[a-z]+'}}`, `{{regexExtract request.query.x '(.+)' 'name'}}` then `{{name.0}}`, `{{parameters.x}}` |\n\nThe lowest `priority` wins (5 by default), then the last one added. Scenarios begin in `Started`.\nRegular expressions are Go's and match the whole value, except in `regexExtract`; those WireMock\nwrites mean the same in both, but Java's lookarounds aren't supported.\n\nA body is filled in as `transformers` says. Without it, as rayo always has: every `{{ }}` is filled in,\nand one that isn't known is an error. With `\"transformers\": [\"response-template\"]`, as WireMock does:\n`{{x}}` is escaped for HTML and `{{{x}}}` isn't. With `\"transformers\": []`, it is sent as it is. A body\nthat isn't text, such as an image, is always sent as it is.\n\nA call whose body is compressed (`Content-Encoding: gzip` or `deflate`) is matched by what it holds.\nThrough the proxy, a call is `/host/path`; a mapping without the host, as WireMock's are, answers it\ntoo when no mapping names that host.\n\nWhen no mapping answers, the flow fails and says which came nearest, and why, as WireMock's near\nmisses do: `POST /availability -\u003e not mocked; nearest POST /availability {\"category\":\"ondemand\"}: body:\n$.category: \"prebooked\", expected \"ondemand\"`.\n\n### mock step\n\nAdds a path, or a mapping written here. Its priority is 0 by default, so it wins over the files.\n\n\u003c!-- generated: keys mock --\u003e\n| Key | Takes | |\n|---|---|---|\n| `file` | text | A path relative to this flow. Short form |\n| `id` | text | A name for it |\n| `name` | text | A name for it |\n| `priority` | number | The lowest wins (0) |\n| `scenarioName` | text | The scenario it belongs to |\n| `requiredScenarioState` | text | Only in this state |\n| `newScenarioState` | text | Moves to this state |\n| `request` | values | Which calls |\n| `response` | values | What to answer |\n\u003c!-- end --\u003e\n\n```yaml\n- mock: overrides.json\n- mock: { request: { method: POST, urlPathPattern: \".*/payments\" }, response: { status: 503 } }\n```\n\n### mockState\n\nMoves scenarios to a state. A scenario or state that no mock mentions is an error.\n\n### network\n\n| Form | |\n|---|---|\n| `network: offline` | Connections close without an answer. Android still believes it is connected: see [setAirplaneMode](#setairplanemode) |\n| `network: { delay: 2s }` | Every answer takes longer |\n| `network: { fail: 503 }` | Every answer is this status |\n| `network: online` | Back to normal |\n\n\u003c!-- generated: keys network --\u003e\n| Key | Takes | |\n|---|---|---|\n| `state` | offline/online | online or offline. Short form |\n| `delay` | time | Every answer takes longer |\n| `fail` | 400 to 599 | Every answer is this status, 400 to 599 |\n| `path` | pattern | Only these calls, a regular expression of the whole path |\n\u003c!-- end --\u003e\n\n`path`, a regular expression, limits it to some calls. Each `network` replaces the one before.\n\n### waitForRequest\n\n\u003c!-- generated: keys waitForRequest --\u003e\n| Key | Takes | |\n|---|---|---|\n| `method` | text | Such as POST (GET) |\n| `path` | pattern | Only these calls, a regular expression of the whole path. Short form |\n| `body` | text | A text in the body |\n\u003c!-- end --\u003e\n\nWaits until the app calls `path`, a regular expression of the whole path. Optional: `method`, and\n`body`, a text in it. Calls already made in this flow count, but each is matched only once.\n\n### Capture\n\nMakes mocks from a real backend. Run the flow once with\n\n```\nrayo test --capture mock -e PASSWORD=... flow.yaml\n```\n\nThe app needs nothing new: it sends its calls to rayo as with a mock, and rayo passes them on and\nkeeps the answers. **The calls are real, and do what they do.** One flow, once, into a new folder.\n\n| The app calls | Passed on to |\n|---|---|\n| `http://127.0.0.1:8080/api.example.com/path` | `https://api.example.com/path` |\n| `http://127.0.0.1:8080/path`, with `--capture-from https://api.example.com` | `https://api.example.com/path` |\n\nWhile capturing, the flow's mocks are left out and `mock`, `mockState` and `network` do nothing, so\na flow that has them can be captured again. `waitForRequest` works. What was captured is also kept\nif the flow fails or is interrupted.\n\nA long flow whose calls do something real (an order, a payment) is better captured in a\n[live session](#live-sessions), which a failure doesn't end:\n\n```\nrayo live --serve --capture mock -e PASSWORD=... flow.yaml \u0026\nrayo do .retry          # after fixing the step that failed, in the file\nrayo do .rewind 2       # forget the last 2 steps and their calls; move the app back by hand\nrayo do .quit           # writes the capture\n```\n\nThe calls of a step that fails, and of those rewound, are left out, so what is written is one\ncapture, as if the flow had run once without them. Without the session, `--capture-append` goes on\nwith a capture that is there already, in another run: its calls come first and the states go on\nfrom them.\n\nIn the folder:\n\n| | |\n|---|---|\n| `mappings/capture.json`, `__files/` | The mocks: give the folder to the flow's `mock` |\n| `capture.txt` | What to do next, as it was printed |\n| `capture.calls.json` | The calls, and what the app sent, hidden as in the mocks; and how they were made into mocks. For `--capture-append` and `--capture-update` |\n\nHow it becomes mocks, as WireMock's recorder does by default:\n\n- A call is matched by its method, its path, its query and its body. The query is matched with\n  `queryParameters`, so the order of its parameters doesn't count. The body as WireMock's\n  `requestBodyPattern: auto`: JSON with `equalToJson`, `ignoreArrayOrder` and `ignoreExtraElements`;\n  a form with `formParameters`; XML with `equalToXml`; other text with `equalTo`; anything else with\n  `binaryEqualTo`. A multipart body, or one over 64 KB, isn't matched. `--rest off` matches by method\n  and address alone. GraphQL calls are told apart [their own way](#graphql).\n- Calls matched the same are the same call. One always answered the same is always answered so.\n- Otherwise there are states, of a scenario called `capture`: `Started`, `1`, `2`... A call that\n  isn't a read (a GET, HEAD, OPTIONS or GraphQL query) moves to the next, if a later answer changed\n  after it. Of several it may be, the one made the fewest times is taken. So the same call is\n  answered as often, and in the order, as during the capture: WireMock's `repeatsAsScenarios`.\n- If an answer changed **by itself**, the flow has to say when: `capture.txt` has the `mockState` to\n  add, and before which step.\n- As `ignoreExtraElements` lets a mapping fit a call with more in it, the mappings that match more\n  get a lower `priority`, from 5: a call is answered by the closest. A mapping written by hand without\n  a priority, loaded after, wins over all of them.\n- What changes from run to run can't be matched as it was. A date and time (ISO 8601) in the query\n  or in a JSON body is matched by its form; `--timestamps exact` matches it as it was. `--ignore-params\n  depart_at,session` leaves parameters of the query out; `--ignore-body nonce,trip.clientTime` leaves\n  names, at any depth, or paths, out of JSON bodies, GraphQL variables and forms; `--ignore-host`\n  matches the path of any host (then, through the proxy, `--hosts` says which to open);\n  `--ignore-path-ids` matches a UUID or ULID in a path, such as `/executions/\u003cuuid\u003e/status`, by its form.\n  `capture.txt` lists what was left out.\n- A call that changes nothing, as GET, keeps the state; any other may change it. `--stateless\n  'api.shop.com/quotes/**,*/availability'` says which paths only read, whatever the method, such as a\n  POST that searches: they answer in any state, as often as the app calls them. `*` is any text but\n  `/`, `**` any text, from the host when calls begin with one.\n- When calls go to several hosts, `capture.txt` says which of them change the state: one that isn't\n  the app's backend, such as analytics, is best left out with `--stateless 'its.host/**'`.\n- A read that answers one way when the app sends `Authorization` and another when it doesn't, such\n  as `GET /users/me` before and after logging in, is matched by whether the header is there, not by\n  a state: `(signed in)` and `(signed out)` in its name.\n- `--capture-headers Accept-Language` matches these headers of the request too.\n- The headers of the answer are kept, but for those that change by themselves (`Date`, `ETag`, trace\n  ids...).\n- These settings are kept in `capture.calls.json`: `--capture-append` and `--capture-update` make the\n  mocks the same way, unless told otherwise.\n\n`rayo mocks import flows.har mock` makes mocks of a HAR file in the same way, as if captured: one of\nmitmproxy (`mitmdump -nr flows --set hardump=flows.har`), Charles, or a browser. With `--capture-from\nhttps://api.example.com`, only the calls to it are taken, without the host in the path; else every\ncall, with its host, but those to Google, Firebase, analytics and crash reports.\n\n#### Updating mocks\n\nWhen the backend changes, capture again beside the mocks instead of over them:\n\n```sh\nrayo test --capture-update mock --capture-from https://api.example.com flow.yaml\n```\n\nThe new capture goes to `mock/.update/`, and each of its mappings is compared with the mapping of\nthe same name (and state) in `mock/`, including hand-written ones. What is printed, and kept in\n`mock/.update/changes.txt`, is each mapping that is `changed` (status, headers, and the paths in the\nbody that differ, such as `body.data.wallet.price: 5 -\u003e 7`), `new` or `gone`. Then take what you\nwant:\n\n```sh\nrayo mocks apply mock                                   # all the changes and new ones\nrayo mocks apply --only graphql:Wallet,\"GET /profile\" mock   # those whose name has one of these\nrayo mocks apply --prune mock                           # also take out those gone\n```\n\nIn CI, `--check` fails the run when anything differs, and changes nothing:\n\n```sh\nrayo test --capture-update mock --check --capture-from https://api.example.com flow.yaml\n```\n\nA changed mapping is written where it was, with its body in the same file of `__files`, so hand-written\nmappings in other files stay where they are. It works the same with the interceptor and the proxy.\nMocks captured before REST bodies and GraphQL variables were matched have other names: the first\nupdate of them shows each as `gone` and `new`; `rayo mocks apply --prune` takes the new ones.\n\n#### GraphQL\n\nGraphQL and REST calls are captured together, through the interceptor or the proxy. A call is taken\nas GraphQL when it is a JSON `POST` whose body has `query`, `operationName` or\n`extensions.persistedQuery.sha256Hash`, or one to a path matching `--graphql-path` (default\n`.*/graphql`). Anything else is captured as before.\n\n- Each GraphQL call is told apart by its **operation**, with a `bodyPatterns` entry\n  `matchesJsonPath: { expression: $.operationName, equalTo: Geocode }`. Mocks therefore don't depend\n  on the order of the calls: a `Wallet` query is never answered with a `Geocode` answer. Without\n  `operationName`, the name in the query is matched; with neither, the hash of a persisted query, or\n  else the query itself. Only WireMock's matchers are written, so WireMock reads the mocks the same.\n- A **query** counts like a GET and a **mutation** like any other call: only mutations move the\n  scenario on, and states whose answers don't differ are joined.\n- Which operation a document runs is read past comments, strings and fragments, also when it has\n  several: the one `operationName` names. That tells a query from a mutation.\n- When the same operation name is sent with **different query documents** (other fields, other\n  fragments), each document gets its own mappings, matched on the query whatever its spaces and\n  commas. When there is only one document per name, only the name is matched, so a small edit to a\n  query doesn't break the mocks.\n- An automatic persisted query, sent first with its hash alone and then again with the query, gets a\n  mapping for each.\n- The **variables** are matched too, with `equalToJson` on `$.variables`, but for those under private\n  names, those of `--ignore-body`, and dates and times, matched by their form. `--graphql operation`\n  matches the operation alone; `--graphql off` takes GraphQL calls as any other.\n- A batch, an array of operations, is matched by its whole body, and is a read if all of them are\n  queries. A query sent with GET is matched by its address.\n- Only `operationName`, the query text and, if asked for, the variables are kept from the request.\n  A query with something private written into it is matched by its name only.\n- When mocking, a GraphQL call that nothing matches is reported with its operation:\n  `POST /graphql (GraphQL operation Unknown)`.\n\nLeft out, as far as it can be recognised: the values of `-e`; emails and JWTs anywhere; and what is\nunder a name such as `access_token`, `password`, `email`, `phone` or `firstName`, also in the address\nand in what the app sends. In an address or a body, anything goes in its place. The value of every\ncookie, and headers such as `Authorization`, are hidden. In a JSON answer, a hidden value keeps its\nform, so that a typed client still reads it: a date or time becomes `2000-01-01T00:00:00Z` in the\nsame layout, a phone `+12025550123`, a first name `Alex`, a last name `Doe`, digits zeros, a number\n`0`; what has no form, such as a token, `REDACTED`. `--redact-value phone=+4915100000000` (again for\nmore) says what a name becomes. An answer that isn't text is kept as it was;\na body the app sent that isn't text isn't kept if a secret is found in it. **Names that aren't in that\nlist are kept, so read the files before committing them.** A failure's screenshot and device log are\nnot touched.\n\n---\n\n## Running several\n\n### Suites\n\nA file whose name ends in `.rayo.yaml` lists the flows that run together. It is the place to say\nwhat each is for. Paths are relative to it.\n\n```yaml\nname: Rides\napp:\n  id: com.example.app\n  apk: ../app/build/outputs/apk/debug/app-debug.apk\noutput: .results\nresults: .results/results.txt\nflows:\n  # Books a bike and rides it.\n  - tests/book/flow.yaml\n  # The same without booking.\n  - tests/scan/flow.yaml\n```\n\n\u003c!-- generated: suite --\u003e\n| Key | Takes | |\n|---|---|---|\n| `name` | text | Shown in the results (default: the file) |\n| `flows` | a list | Files, run in this order; or mappings with file, retries, mock, network and unmocked, see [below](#a-flow-of-a-suite) (required) |\n| `retries` | a number from 0 | How many times a flow that fails is tried again, see [Retries](#retries) (default: 1) |\n| `app` | a mapping | Installed before the flows, see [below](#the-app-of-a-suite) |\n| `device` | a mapping | How it is while the flows run, see [Device setup](#device-setup) |\n| `env` | values | Variables for every flow, as `-e`, which wins; they may use `${...}`, such as `${rayo.file}`, see [Variables](#variables) |\n| `timeoutScale` | 0.1 to 10 | Multiplies the default waits, see [Default times](#default-times); `--timeout-scale` wins (default: 1) |\n| `network` | `interceptor` or `proxy` | How the app's calls reach the mock, as a flow's `network`; the flow's own and `--network` win, see [Mocks of a suite](#mocks-of-a-suite) (default: each flow's, else `interceptor`) |\n| `unmocked` | a mapping | Calls without a mock that don't fail the flow, see [Calls not worth mocking](#calls-not-worth-mocking) (default: none: each fails the flow) |\n| `output` | folder | As `--output`, which wins (default: `.rayo` where rayo runs) |\n| `results` | a file or a list | As `--results`, which wins over all of them: `.txt`, `.json` or `.xml` (JUnit) |\n| `record` | `true` or `false` | A video of each flow that doesn't say, see [Recording a whole flow](#recording-a-whole-flow) (default: `false`) |\n| `recordFolder` | folder | Where the videos go; needs `record: true` (default: each flow's, in `output`) |\n\u003c!-- end --\u003e\n\n#### A flow of a suite\n\n\u003c!-- generated: suite flow --\u003e\n| Key | Takes | |\n|---|---|---|\n| `file` | file | The flow (required) |\n| `retries` | a number from 0 | How many times it is tried again if it fails (default: the suite's) |\n| `mock` | a path or a list | Who answers as the backend, as a flow's `mock`, relative to the suite; the flow's own `mock` wins, see [Mocks of a suite](#mocks-of-a-suite) |\n| `network` | `interceptor` or `proxy` | How the app's calls reach the mock, as a flow's `network`; the flow's own and `--network` win, see [Mocks of a suite](#mocks-of-a-suite) (default: the suite's) |\n| `unmocked` | a mapping | Calls without a mock that don't fail the flow, see [Calls not worth mocking](#calls-not-worth-mocking) (default: the suite's; its calls are added to them) |\n\u003c!-- end --\u003e\n\n#### Mocks of a suite\n\nA suite can say which mocks each flow runs on, and how the app's calls reach them, so that the same\nflow files run unchanged against the real backend and on mocks, with no wrapper flows:\n\n```yaml\nname: Shop (mocked)\nnetwork: proxy                  # for every flow; --network wins\nflows:\n  - file: tests/checkout.yaml\n    mock: mocks/checkout        # a path or a list, relative to the suite\n    retries: 2\n  - file: tests/login.yaml\n    mock: [mocks/common, mocks/login]\n  - file: tests/pinned_host.yaml\n    mock: mocks/pinned_host\n    network: interceptor        # this flow, the other way\n```\n\nWhat wins, first to last: `--network`; the flow's own `mock` and `network`; its entry in the suite;\nthe suite's `network`. A flow whose header has a `mock` runs on that one only, not on its entry's.\nThe paths are checked when the suite is read, as a flow's are.\n\n#### Calls not worth mocking\n\nA call that no mock answers fails the flow. Some aren't worth recording: those the app makes on its\nown at startup, or one cancelled during the capture. `unmocked` lets them through, answered, in the\nsuite for every flow or in a flow's entry, which adds its own; `--ignore-unmocked` adds more:\n\n```yaml\nunmocked:\n  ignore:\n    - host: guest-api.example.com\n      path: /api/v2/locations/favorites/dismiss\n    - graphql: [Geocode, ReverseGeocode]\n    - api.example.com/pings/**  # as --ignore-unmocked takes them\n  status: 200                   # what they are answered with (default 404)\n  body: \"{}\"\n```\n\n```sh\nrayo test --ignore-unmocked 'graphql:Geocode,api.example.com/pings/**' suite.rayo.yaml\n```\n\nEvery other call without a mock still fails the flow. Those let through are said at the end of the\nflow, as `ignored 2 unmocked calls: ...`, so that nothing is hidden. They change nothing while\ncapturing, as the real backend answers.\n\n\u003c!-- generated: suite unmocked --\u003e\n| Key | Takes | |\n|---|---|---|\n| `ignore` | a list | Calls that no mock answers but that don't fail the flow: each `graphql:Operation`, `host/path`, `host` or `/path`, as `--ignore-unmocked`, or a mapping with host, path and graphql, see [Calls not worth mocking](#calls-not-worth-mocking) (required) |\n| `status` | 100 to 599 | What those calls are answered with (default: 404) |\n| `body` | text | Of the answer (default: `not mocked: ` and the call) |\n\u003c!-- end --\u003e\n\nAn ignored call, when a mapping:\n\n\u003c!-- generated: suite ignored --\u003e\n| Key | Takes | |\n|---|---|---|\n| `host` | host | Of the call, or of its subdomains (default: any) |\n| `path` | a path | Of the call, without the host; `*` is any text but `/`, `**` any (default: any) |\n| `graphql` | a name or a list | GraphQL operations (default: any call) |\n\u003c!-- end --\u003e\n\n#### The app of a suite\n\n\u003c!-- generated: suite app --\u003e\n| Key | Takes | |\n|---|---|---|\n| `id` | package | The app's (required) |\n| `apk` | file | What to install, if the device has another. rayo doesn't build it (required) |\n\u003c!-- end --\u003e\n\n#### What goes where\n\nShared steps and permissions belong in each flow, so that it can be read and run alone; mocks too,\nunless the same flow also runs without them, see [Mocks of a suite](#mocks-of-a-suite). The\ndevice is chosen as always: use `--emulator` if the flows must never run on a real one.\n\n`rayo check` takes suites too. When a folder is given, its suites aren't run.\n\n### Device setup\n\n```yaml\ndevice:\n  size: 1080x2400\n  dpi: 420\n  fontScale: 1.3\n  darkMode: true\n```\n\n\u003c!-- generated: device --\u003e\n| Key | Takes | |\n|---|---|---|\n| `size` | pixels, such as `1080x2400` | Of the screen |\n| `dpi` | 72 to 1000 | Its density. With `size`, it says how many dp there are: 1080 pixels at 420 dpi are 411 dp |\n| `fontScale` | 0.5 to 3 | Of the text; 1 is normal |\n| `darkMode` | `true` or `false` | The dark theme |\n\u003c!-- end --\u003e\n\nWhat isn't said stays as it is. It is only for a suite: all its flows run on the same device, and\none flow alone runs on the device as it is. To run the same flows on two, write two suites.\n\nIt is put back at the end, also after Ctrl-C or if rayo is killed, as the device itself does it. If\nthe device is restarted meanwhile, use `wm size reset` and `wm density reset`. It works on real\ndevices too. The size doesn't change the hardware: a notch stays where it is.\n\n### Clean state\n\nAfter each flow, whether it passed or not, rayo undoes what it changed: the mock and `network`, the\n`camera`, a recording, the location and airplane mode. If it can't, the flow has a `warning`, as the next may be\naffected. The app's data and permissions stay: begin with `launchApp: { clearState: true }`. What a\n`shell` or `http` step changed stays too.\n\n`--shuffle` finds flows that depend on others. The order is printed as `--shuffle=\u003cseed\u003e`, which\nrepeats it.\n\n### Retries\n\nA flow that fails can be run again, whole, after the [clean state](#clean-state). The suite says how\nmany times, for all its flows and for each:\n\n```yaml\nretries: 1                          # all, if they don't say\nflows:\n  - tests/book/flow.yaml            # 1\n  - file: tests/scan/flow.yaml      # 3\n    retries: 3\n  - { file: tests/pay/flow.yaml, retries: 0 }   # never\n```\n\n`--retries \u003cn\u003e` wins over both. **Without any of them it is 1**, also for flows without a suite;\n`--retries 0` turns it off. With `-c` and no suite it is 0. It is retried at once, before the next flow. `NOT_RUN` and `INTERRUPTED` aren't, nor when capturing.\n\nIf it passes at last it is a `PASS`, as any other, with a note of where each try failed, its\n[cause](#causes) and the folder with the rest. Each has its own video too.\n\n```\n[2/3] PASS scan  31.0 s  (failed once, then passed)\n  try 1: tests/scan/flow.yaml:12: tapOn scan: not on screen: scan  (test)  .results/scan\n[3/3] FAIL pay  27.2 s  (failed 3 times)\n  try: 1\n  ...\n\nTOTAL 3: 2 passed, 1 failed, 2 retried  78.1 s\n```\n\n| | |\n|---|---|\n| `retried` | Flows tried more than once, whether they passed or not |\n| JSON | `retried` of the run; of a flow, `tries`, `timesFailed`, `retries` allowed, and a failure for each failed try |\n| JUnit | One that passed at last passes, with its failed tries in `system-out` |\n| `--failures-only` | Keeps those that failed at first |\n\nTo run only some steps again, use [retry](#retry).\n\n### Causes\n\nNot every failure is the test's. Each has a `cause`:\n\n| Cause | When |\n|---|---|\n| `app` | The app crashed or hung during the flow, or isn't installed |\n| `device` | It doesn't answer, is starting, restarted, its interface hung, or the connection was lost |\n| `test` | Anything else. It is only `app` or `device` if there is proof of it, so something slow is `test` |\n\nThe run stops, so as not to waste time on flows that would fail the same, when:\n\n| | |\n|---|---|\n| The device can't be used: it doesn't answer, is starting, or Android isn't running | At once, always |\n| 2 failures in a row are `device` | Unless `--keep-going` |\n| 3 different flows in a row fail with `app`, and the same crash | Unless `--keep-going` |\n\nFor `device`, each try counts. A failure that is the test's, or a pass, starts the count again. The rest are `NOT_RUN`, and the results say why the run stopped:\n\n```\n[3/9] FAIL pay  4.1 s\n  why: not on screen: home\n  cause: app, the app crashed: java.lang.IllegalStateException: oops\n  ...\n[4/9] NOT_RUN scan  (app: 3 flows in a row, the app crashed: ...)\n\nSTOPPED app: 3 flows in a row, the app crashed: java.lang.IllegalStateException: oops\nTOTAL 9: 0 passed, 3 failed, 6 not run, of those failed 3 app  14.0 s\n```\n\nJSON has `stopped` for the run, `cause` for a flow that failed, and `cause` and `because` for each failure.\n\n### Outcomes\n\nEvery flow asked for has one; none is left out of the count.\n\n| | | Exit code |\n|---|---|---|\n| `PASS` | Also if it failed first, and passed when [retried](#retries) | 0 |\n| `FAIL` | | 1 |\n| `INTERRUPTED` | Ctrl-C while it ran | 130 |\n| `NOT_RUN` | With the reason: interrupted, `--fail-fast`, or the run [stopped](#causes) | 1 |\n\nIf the connection is lost, the flow fails and the next goes on with a new one. `--rerun-failed` runs what is in `\u003coutput\u003e/failed.json`: what\nfailed, was interrupted or didn't run, until it passes.\n\n### Results\n\nWith several flows, one line each, and for a failure all there is to know. A single flow, or `-v`,\nshows every step.\n\n```\n[1/3] PASS book  18.2 s\n[2/3] FAIL scan  9.1 s\n  why: not on screen: finished_close\n  step: tapOn finished_close\n  at: shared/flows/dock.yaml:8\n  from: tests/scan/flow.yaml:23 runFlow ../../shared/flows/dock.yaml\n  before:\n    extendedWaitUntil finished\n  calls:\n    12:00:01.250 GET /trips -\u003e 200\n  screen:\n    ...\n  saved: .results/scan\n[3/3] NOT_RUN long  (--fail-fast, after scan)\n\nPASS book  18.2 s\nFAIL scan  9.1 s\nNOT_RUN long  (--fail-fast, after scan)\n\nTOTAL 3: 1 passed, 1 failed, 1 not run  27.4 s\n```\n\nThe list at the end is only there if something didn't pass. Under a failure:\n\n| Key | |\n|---|---|\n| `try` | Which, if there were several |\n| `why` | The error |\n| `cause` | `test`, `app` or `device`, see [Causes](#causes) |\n| `step`, `at` | The step that failed, and where it is written |\n| `from` | The step that ran that flow, and so on outwards |\n| `crash` | The app crashed or hung during the flow: what the device log says |\n| `before` | The last 3 steps that passed |\n| `calls` | The last 5 to the [mock](#mock) |\n| `screen` | The elements on it, at most 40 lines |\n| `saved` | The folder with the [rest](#failures) |\n| `recording` | A video; of flows that passed, only in the file |\n| `warning` | Something that may explain this, or the next flow |\n\n`--results \u003cfile\u003e` writes it, as the file's name says. It may be repeated, for several formats at once:\n\n```\nrayo test --results out.txt --results junit.xml all.rayo.yaml\n```\n\n| File | |\n|---|---|\n| `.txt` | As above for every flow, with `run`, `device` and `began` at the top and each flow's `file` |\n| `.json` | For a program |\n| `.xml` | JUnit, for a CI, see [Report](#report) |\n\n`--failures-only` leaves out the flows that passed, but counts them; not from JUnit.\n\nIn a `.txt`, a line that begins with an outcome is a flow: `grep -E '^(FAIL|NOT_RUN)' results.txt`. What\nfollows is about it: `  key: value`, or `  key:` and lines with four spaces. `TOTAL` is the last line.\n\nIn a `.json`, every key is always there: what there is none of is `\"\"`, `0` or `[]`.\n\n```json\n{\n  \"version\": 1,\n  \"run\": \"Rides\",\n  \"device\": \"emulator-5554\",\n  \"began\": \"2026-11-22T21:11:18Z\",\n  \"seconds\": 27.402,\n  \"order\": \"\",\n  \"stopped\": \"\",\n  \"ok\": false,\n  \"total\": 3, \"passed\": 1, \"failed\": 1, \"interrupted\": 0, \"notRun\": 1, \"retried\": 0,\n  \"flows\": [\n    {\n      \"name\": \"scan\",\n      \"file\": \"tests/scan/flow.yaml\",\n      \"status\": \"FAIL\",\n      \"cause\": \"test\",\n      \"seconds\": 9.114,\n      \"tries\": 1,\n      \"timesFailed\": 1,\n      \"retries\": 0,\n      \"notRunReason\": \"\",\n      \"warnings\": [],\n      \"recordings\": [],\n      \"failures\": [\n        {\n          \"try\": 1,\n          \"why\": \"not on screen: finished_close\",\n          \"cause\": \"test\",\n          \"because\": \"\",\n          \"step\": \"tapOn finished_close\",\n          \"file\": \"shared/flows/dock.yaml\",\n          \"line\": 8,\n          \"from\": [{ \"step\": \"runFlow ../../shared/flows/dock.yaml\", \"file\": \"tests/scan/flow.yaml\", \"line\": 23 }],\n          \"before\": [\"extendedWaitUntil finished\"],\n          \"calls\": [{ \"at\": \"2026-11-22T21:11:40.250Z\", \"method\": \"GET\", \"uri\": \"/trips\", \"outcome\": \"200\", \"mocked\": true, \"body\": \"\" }],\n          \"screen\": [\"...\"],\n          \"crash\": \"\",\n          \"saved\": \".results/scan\"\n        }\n      ]\n    }\n  ]\n}\n```\n\nThe run:\n\n\u003c!-- generated: results --\u003e\n| Key | | |\n|---|---|---|\n| `version` | number | Changes if the meaning of a key does |\n| `run` | text | The suite's name, or what was run |\n| `device` | text | Its serial |\n| `began` | text | In UTC |\n| `seconds` | number | All of it |\n| `order` | text | `--shuffle=\u003cseed\u003e`, if shuffled |\n| `stopped` | text | Why, if it stopped before the end, see [Causes](#causes) |\n| `ok` | true or false | Nothing failed, was interrupted or didn't run: as the exit code |\n| `total` | number | Flows |\n| `passed` | number | Also those that failed first |\n| `failed` | number |  |\n| `interrupted` | number |  |\n| `notRun` | number |  |\n| `retried` | number | Flows tried more than once, whether they passed or not |\n| `flows` | list | See below |\n\u003c!-- end --\u003e\n\nEach of `flows`:\n\n\u003c!-- generated: results flow --\u003e\n| Key | | |\n|---|---|---|\n| `name` | text | The flow's |\n| `file` | text | The flow's |\n| `status` | text | One of the [outcomes](#outcomes) |\n| `cause` | text | Of a flow that failed, or empty, see [Causes](#causes) |\n| `seconds` | number | All its tries |\n| `tries` | number | Those it took |\n| `timesFailed` | number | Of those |\n| `retries` | number | Those allowed |\n| `notRunReason` | text | If it is `NOT_RUN` |\n| `warnings` | list | What may explain this, or the next flow |\n| `recordings` | list | Videos |\n| `failures` | list | One for each try that failed, also of a flow that passed at last |\n\u003c!-- end --\u003e\n\nEach of `failures`:\n\n\u003c!-- generated: results failure --\u003e\n| Key | | |\n|---|---|---|\n| `try` | number | From 1 |\n| `why` | text | The error |\n| `cause` | text | `test`, `app` or `device`, see [Causes](#causes) |\n| `because` | text | What shows that it isn't the test |\n| `step` | text | The step that failed |\n| `file` | text | Where it is written |\n| `line` | number |  |\n| `from` | list | The step that ran that flow, and so on outwards |\n| `before` | list | The last 3 steps that passed |\n| `calls` | list | The last 5 to the mock |\n| `screen` | list | The elements on it |\n| `crash` | text | The app crashed or hung during the flow: what the device log says |\n| `saved` | text | The folder with the rest, see [Failures](#failures) |\n\u003c!-- end --\u003e\n\n```\njq -r '.flows[] | select(.status == \"FAIL\") | \"\\(.name): \\(.failures[-1].why)\"' results.json\n(Get-Content results.json -Raw | ConvertFrom-Json).flows | Where-Object status -eq FAIL\n```\n\n---\n\n## Live sessions\n\nWrite a test by running it, a command at a time. Good for other programs too: they see what each\nstep did before choosing the next.\n\n```\n$ rayo live --emulator --app com.example.app --mock ../mock tests/new/flow.yaml\nlive: type commands, or send them with rayo do; .help\n- launchApp: { clearState: true }\n  ✓ launchApp clearState  812 ms\n- tapOn: { id: nothing }\n  ✗ tapOn nothing  15012 ms\nerror: tests/new/flow.yaml:1: tapOn nothing: not on screen: nothing\nscreen:\n  ...\n- tapOn: { id: login }\n  ✓ tapOn login  95 ms\n.save\nsaved 2 steps in tests/new/flow.yaml\n```\n\nIf the flow is there already, its steps run first (`--no-run` to skip them) and `.save` adds to its\nend, leaving the rest as it is. A new one needs `--app`, and `--mock` takes paths relative to it.\nThe mock, the variables and `output` last for the whole session. When it ends, all is\n[put back](#clean-state).\n\n### Live commands\n\n| | |\n|---|---|\n| Any command, or a list of them | Runs them up to the first that fails. Those that pass are kept |\n| `.screen` | What is on screen, and how to refer to it |\n| `.steps` | What is kept and not saved yet |\n| `.undo` | Forgets the last one. **The device stays as it is** |\n| `.save` | Adds them to the file |\n| `.retry` | Reads the file again and runs its steps from the one that failed, so a step can be fixed in the file and tried again |\n| `.from \u003cn\u003e` | The same, from its n-th step |\n| `.rewind [n]` | Forgets the last n steps done (1), typed or of the file, and the calls they made to a [capture](#capture). **The device stays as it is**: move the app back by hand, then `.retry` |\n| `.help`, `.quit` | `.quit` doesn't save, but prints what is lost |\n\nTyped, a command is a line; one that ends in `:` goes on until an empty line. The leading `- ` may\nbe left out.\n\n### From other programs\n\n`rayo do` sends commands to the live session, from the same folder or with the same `--output`.\nEach call answers when the steps have run; its exit code is 1 if one failed.\n\n```\nrayo live --serve --emulator tests/new/flow.yaml \u0026     # until .quit, or an hour unused\nrayo do .screen\nrayo do --screen '- tapOn: { id: login }'\nrayo do --json - \u003c steps.yaml\nrayo do .save\nrayo do .quit\n```\n\n| Option | |\n|---|---|\n| `--screen` | Also what is on screen afterwards. After a failure it always comes |\n| `--json` | `ok`, `output`, `error`, `failure` (as in the [results](#results), or `null`), `warnings`, `calls` to the mock meanwhile, `screen`, `unsaved` steps |\n| `-` | Commands from standard input |\n\nCalls the app makes without a mock are warnings here: the session goes on. A run of the saved\nflow would fail.\n\nOnly this computer can use a session, with a token kept in `\u003coutput\u003e/live.json`.\n\n---\n\n## Features\n\n### Schemas\n\nFor editors to complete and check what is written, and for programs to know what is valid. At the\ntop of a file, for those that understand it (such as VS Code with the YAML extension):\n\n```yaml\n# yaml-language-server: $schema=../../rayo/schema/flow.json\n```\n\nThey know the keys and values. What depends on other files, or on several keys at once, only\n`rayo check` tells.\n\n### Variables\n\nUsed as `${NAME}`. The first that has it wins:\n\n1. `env` of the `runFlow` or `runScript` running now\n2. `-e NAME=value`\n3. `env` of the suite, for all its flows\n4. What is set where the file is included\n5. `env` in the file's header: a default, for what nothing above sets\n\nIn `${...}` and conditions, a name that nothing sets is `undefined`, not an error, so defaults work\nwith `||` or `??`. Only a whole `${...}` that ends up `undefined` fails, as there is nothing to write:\n\n```yaml\nenv:\n  CITY: ${ CITY || \"Springfield\" }\n```\n\nValues of `-e`, of a suite's `env` and of a header may all use `${...}`. Besides the variables,\nrayo gives `rayo.file`, the flow's file name without its extension, and `rayo.platform`. To give one of\nthem a name that flows from another tool expect, set it once for all of them:\n\n```yaml\n# all.rayo.yaml\nflows: [login.yaml, book.yaml]\nenv:\n  MAESTRO_FILENAME: ${rayo.file}\n```\n\nor, without a suite, `rayo test -e 'MAESTRO_FILENAME=${rayo.file}' flows/`.\n\n### Tags\n\n🧪 `--include-tags smoke,login` runs flows with any of them; `--exclude-tags slow` leaves flows out.\n\n### Continuous mode\n\n🧪 `rayo test -c` runs again when a flow, or a file it includes, is saved.\n\n### Failures\n\nSaved in `\u003coutput\u003e/\u003cflow\u003e/`, or `\u003cflow\u003e_2` for a second one with that name; the tries after the\nfirst in `\u003cflow\u003e_retry1` and so on. Those of an earlier run are removed when the flow runs again.\n\n| File | |\n|---|---|\n| `failure.jpg` | The screen, small: 480 pixels on the short side |\n| `screen.txt` | The elements on it, as `rayo hierarchy` shows them |\n| `logcat.txt` | The end of the device log |\n| `requests.txt` | Every call the app made to the [mock](#mock), with its body; `requests.json`, a list like `calls`, if the results are also written to a `.json`. The last five are printed |\n\n| Exit code | |\n|---|---|\n| 0 | Everything passed |\n| 1 | A flow failed or didn't run, or an error |\n| 2 | No command given |\n| 130 | Interrupted with Ctrl-C; what was set up on the device is undone |\n\n### Timings\n\n`rayo test --timings` shows where a run is slow or stuck, without the internals of `--profile`:\n\n- every step with its time, and `◂` with the total of each `runFlow`, `repeat` and `retry`\n- `… still on \u003cstep\u003e after 10 s` every 5 s while a step runs, so a hang shows where it is\n- after each flow, its 5 slowest steps; at the end, the 10 slowest flows\n- `default waits ×2` at the start, with a [`--timeout-scale`](#default-times) other than 1\n\n### Profile\n\n`--profile` adds to each step where its time went, in milliseconds:\n\n| | |\n|---|---|\n| `adb+host` | The computer and adb |\n| `read` | Reading the screen; long when the app's main thread is busy |\n| `cache` | Looking in what Android remembers of the screen |\n| `check` | Reading one element again, to confirm it |\n| `wait` | Waiting for the screen to change |\n| `still` | Waiting to see that an element doesn't move |\n| `inject` | Touches and keys, until the app takes them |\n| `shell` | Commands on the device |\n\n### Report\n\nJUnit: `--results report.xml`. What didn't run is `skipped`, what was interrupted is an `error`,\nand a flow that failed first has its failed tries in `system-out`.\n\n### Environment\n\n| Variable | |\n|---|---|\n| `ANDROID_SERIAL` | The device, without `--device` |\n| `ANDROID_HOME`, `ANDROID_SDK_ROOT` | Where to find adb if it isn't in the `PATH`; the usual places are tried too |\n| `ANDROID_ADB_SERVER_PORT` | 5037 by default |\n\n---\n\n## Default times\n\n| | |\n|---|---|\n| Finding an element, `extendedWaitUntil`, `launchApp`, `waitForRequest` | 15 s |\n| `assertNotVisible` | 5 s |\n| `scrollUntilVisible` | 20 s |\n| With `optional: true` | 3 s, except the two above |\n| `when` and `while` | 0.7 s each for `visible` and `notVisible`, or their `timeout` |\n\n`--timeout-scale \u003cfactor\u003e`, or `timeoutScale:` in a suite, multiplies all of these: `2` for a slow CI emulator,\n`0.5` for a fast local one. It takes 0.1 to 10; the option wins over the suite, and the default is 1. A step's own\n`timeout:` is not scaled, and `timeout: 0` stays 0. With `--timings`, a factor other than 1 is shown at the start.\n\n---\n\n## Limitations\n\n- Not supported: `addMedia`,\n  `setOrientation`, sharding across devices, a\n  `config.yaml` (see [suites](#suites)), and selectors by `css`, `width`, `height` or `traits`. An unknown command is an\n  error, never ignored.\n- `setLocation` and `travel` need Android 12 or newer.\n- WebViews show only what they expose to accessibility.\n- What a dialog, sheet or other window covers is not on screen: only what is on top can be found\n  or tapped. Close the dialog first, for example with a `when` guard.\n- Only one tool can automate a device at a time: not while another runs.\n- What is marked 🧪 is **written but not tested on a\n  device yet**, and so are relative selectors and tags.\n",
  "examples/shop/all.rayo.yaml": "# What runs together, in this order.\nname: Shop\n\napp:\n  id: com.example.shop\n  apk: app-debug.apk\n\noutput: results\nresults: [results/results.txt, results/junit.xml]\nrecord: true\nrecordFolder: results/videos\nretries: 1\ndevice: { size: 1080x2400, dpi: 420, fontScale: 1.0, darkMode: false }\n\nflows:\n  # Log in and find a product.\n  - tests/browse/flow.yaml\n  # Buy it, and follow the order until it is shipped.\n  - { file: tests/pay/flow.yaml, retries: 2 }\n  # The bank refuses the card.\n  - tests/declined/flow.yaml\n  # No connection, and back.\n  - tests/offline/flow.yaml\n",
  "examples/shop/mock/__files/products.json": "[\n  { \"id\": 1, \"name\": \"Red bike\", \"price\": 249.0 },\n  { \"id\": 2, \"name\": \"Helmet\", \"price\": 39.0 }\n]\n",
  "examples/shop/mock/mappings/shop.json": "{\n  \"mappings\": [\n    {\n      \"request\": { \"method\": \"POST\", \"urlPath\": \"/login\" },\n      \"response\": { \"status\": 200, \"jsonBody\": { \"token\": \"a-token\" } }\n    },\n    {\n      \"request\": { \"method\": \"GET\", \"urlPath\": \"/products\" },\n      \"response\": { \"status\": 200, \"headers\": { \"Content-Type\": \"application/json\" }, \"bodyFileName\": \"products.json\" }\n    },\n    {\n      \"scenarioName\": \"order\", \"requiredScenarioState\": \"Started\", \"newScenarioState\": \"paid\",\n      \"request\": { \"method\": \"POST\", \"urlPath\": \"/orders\" },\n      \"response\": { \"status\": 201, \"jsonBody\": { \"id\": 7 } }\n    },\n    {\n      \"scenarioName\": \"order\", \"requiredScenarioState\": \"paid\",\n      \"request\": { \"method\": \"GET\", \"urlPath\": \"/orders/7\" },\n      \"response\": { \"status\": 200, \"jsonBody\": { \"id\": 7, \"state\": \"preparing\" } }\n    },\n    {\n      \"scenarioName\": \"order\", \"requiredScenarioState\": \"shipped\",\n      \"request\": { \"method\": \"GET\", \"urlPath\": \"/orders/7\" },\n      \"response\": { \"status\": 200, \"jsonBody\": { \"id\": 7, \"state\": \"shipped\" } }\n    }\n  ]\n}\n",
  "examples/shop/steps/login.yaml": "appId: com.example.shop\nenv: { EMAIL: me@example.com }\n---\n# Begins clean, against the mock, and logs in.\n- launchApp: { clearState: true, permissions: { all: allow }, arguments: { mock: true } }\n- type: { id: login_email, value: \"${EMAIL}\" }\n- type: { id: login_password, value: secret }\n- tapOn: { id: login_button }\n- extendedWaitUntil: { visible: { id: products }, timeout: 20s }\n",
  "examples/shop/tests/browse/flow.yaml": "appId: com.example.shop\nmock: ../../mock\ntags: [smoke]\n---\n- runFlow: ../../steps/login.yaml\n- scrollUntilVisible: { element: { text: Red bike } }\n- tapOn: Red bike\n- assertVisible: { id: price, text: 249.00.* }\n",
  "examples/shop/tests/declined/declined.json": "{\n  \"priority\": 1,\n  \"request\": { \"method\": \"POST\", \"urlPath\": \"/orders\" },\n  \"response\": { \"status\": 402, \"jsonBody\": { \"error\": \"card_declined\" } }\n}\n",
  "examples/shop/tests/declined/flow.yaml": "appId: com.example.shop\nmock: [../../mock, declined.json]     # the normal answers, and the one that differs\n---\n- runFlow: ../../steps/login.yaml\n- tapOn: Red bike\n- tapOn: { id: buy }\n- assertVisible: Your card was declined\n- assertNotVisible: Preparing your order\n",
  "examples/shop/tests/offline/flow.yaml": "appId: com.example.shop\nmock: ../../mock\n---\n- runFlow: ../../steps/login.yaml\n- network: offline\n- tapOn: { id: refresh }\n- assertVisible: You are offline\n- network: online\n- tapOn: Try again\n- assertVisible: Red bike\n",
  "examples/shop/tests/pay/flow.yaml": "appId: com.example.shop\nmock: ../../mock\ntags: [smoke]\n---\n- runFlow: ../../steps/login.yaml\n- tapOn: Red bike\n- tapOn: { id: buy }\n- waitForRequest: { method: POST, path: /orders, body: \"\\\"product\\\":1\" }\n- assertVisible: Preparing your order\n\n# The shop ships it: something the backend does by itself.\n- mockState: { order: shipped }\n- tapOn: { id: refresh }\n- assertVisible: On its way\n",
  "schema/results.json": "{\n  \"$schema\": \"http://json-schema.org/draft-07/schema#\",\n  \"additionalProperties\": false,\n  \"properties\": {\n    \"began\": {\n      \"description\": \"In UTC\",\n      \"type\": \"string\"\n    },\n    \"device\": {\n      \"description\": \"Its serial\",\n      \"type\": \"string\"\n    },\n    \"failed\": {\n      \"description\": \"\",\n      \"type\": \"integer\"\n    },\n    \"flows\": {\n      \"description\": \"See below\",\n      \"items\": {\n        \"additionalProperties\": false,\n        \"properties\": {\n          \"cause\": {\n            \"description\": \"Of a flow that failed, or empty\",\n            \"enum\": [\n              \"test\",\n              \"app\",\n              \"device\",\n              \"\"\n            ]\n          },\n          \"failures\": {\n            \"description\": \"One for each try that failed, also of a flow that passed at last\",\n            \"items\": {\n              \"additionalProperties\": false,\n              \"properties\": {\n                \"because\": {\n                  \"description\": \"What shows that it isn't the test\",\n                  \"type\": \"string\"\n                },\n                \"before\": {\n                  \"description\": \"The last 3 steps that passed\",\n                  \"items\": {\n                    \"type\": \"string\"\n                  },\n                  \"type\": \"array\"\n                },\n                \"calls\": {\n                  \"description\": \"The last 5 to the mock\",\n                  \"items\": {\n                    \"additionalProperties\": false,\n                    \"properties\": {\n                      \"at\": {\n                        \"description\": \"In UTC\",\n                        \"type\": \"string\"\n                      },\n                      \"body\": {\n                        \"description\": \"Only in a requests.json\",\n                        \"type\": \"string\"\n                      },\n                      \"method\": {\n                        \"description\": \"\",\n                        \"type\": \"string\"\n                      },\n                      \"mocked\": {\n                        \"description\": \"Whether a mock fitted\",\n                        \"type\": \"boolean\"\n                      },\n                      \"outcome\": {\n                        \"description\": \"The status, or what happened\",\n                        \"type\": \"string\"\n                      },\n                      \"uri\": {\n                        \"description\": \"\",\n                        \"type\": \"string\"\n                      }\n                    },\n                    \"required\": [\n                      \"at\",\n                      \"method\",\n                      \"uri\",\n                      \"outcome\",\n                      \"mocked\",\n                      \"body\"\n                    ],\n                    \"type\": \"object\"\n                  },\n                  \"type\": \"array\"\n                },\n                \"cause\": {\n                  \"description\": \"test, app or device\",\n                  \"enum\": [\n                    \"test\",\n                    \"app\",\n                    \"device\"\n                  ]\n                },\n                \"crash\": {\n                  \"description\": \"The app crashed or hung during the flow: what the device log says\",\n                  \"type\": \"string\"\n                },\n                \"file\": {\n                  \"description\": \"Where it is written\",\n                  \"type\": \"string\"\n                },\n                \"from\": {\n                  \"description\": \"The step that ran that flow, and so on outwards\",\n                  \"items\": {\n                    \"additionalProperties\": false,\n                    \"properties\": {\n                      \"file\": {\n                        \"description\": \"\",\n                        \"type\": \"string\"\n                      },\n                      \"line\": {\n                        \"description\": \"\",\n                        \"type\": \"integer\"\n                      },\n                      \"step\": {\n                        \"description\": \"\",\n                        \"type\": \"string\"\n                      }\n                    },\n                    \"required\": [\n                      \"step\",\n                      \"file\",\n                      \"line\"\n                    ],\n                    \"type\": \"object\"\n                  },\n                  \"type\": \"array\"\n                },\n                \"line\": {\n                  \"description\": \"\",\n                  \"type\": \"integer\"\n                },\n                \"saved\": {\n                  \"description\": \"The folder with the rest\",\n                  \"type\": \"string\"\n                },\n                \"screen\": {\n                  \"description\": \"The elements on it\",\n                  \"items\": {\n                    \"type\": \"string\"\n                  },\n                  \"type\": \"array\"\n                },\n                \"step\": {\n                  \"description\": \"The step that failed\",\n                  \"type\": \"string\"\n                },\n                \"try\": {\n                  \"description\": \"From 1\",\n                  \"type\": \"integer\"\n                },\n                \"why\": {\n                  \"description\": \"The error\",\n                  \"type\": \"string\"\n                }\n              },\n              \"required\": [\n                \"try\",\n                \"why\",\n                \"cause\",\n                \"because\",\n                \"step\",\n                \"file\",\n                \"line\",\n                \"from\",\n                \"before\",\n                \"calls\",\n                \"screen\",\n                \"crash\",\n                \"saved\"\n              ],\n              \"type\": \"object\"\n            },\n            \"type\": \"array\"\n          },\n          \"file\": {\n            \"description\": \"The flow's\",\n            \"type\": \"string\"\n          },\n          \"name\": {\n            \"description\": \"The flow's\",\n            \"type\": \"string\"\n          },\n          \"notRunReason\": {\n            \"description\": \"If it is NOT_RUN\",\n            \"type\": \"string\"\n          },\n          \"recordings\": {\n            \"description\": \"Videos\",\n            \"items\": {\n              \"type\": \"string\"\n            },\n            \"type\": \"array\"\n          },\n          \"retries\": {\n            \"description\": \"Those allowed\",\n            \"type\": \"integer\"\n          },\n          \"seconds\": {\n            \"description\": \"All its tries\",\n            \"type\": \"number\"\n          },\n          \"status\": {\n            \"description\": \"One of the outcomes\",\n            \"enum\": [\n              \"PASS\",\n              \"FAIL\",\n              \"INTERRUPTED\",\n              \"NOT_RUN\"\n            ]\n          },\n          \"timesFailed\": {\n            \"description\": \"Of those\",\n            \"type\": \"integer\"\n          },\n          \"tries\": {\n            \"description\": \"Those it took\",\n            \"type\": \"integer\"\n          },\n          \"warnings\": {\n            \"description\": \"What may explain this, or the next flow\",\n            \"items\": {\n              \"type\": \"string\"\n            },\n            \"type\": \"array\"\n          }\n        },\n        \"required\": [\n          \"name\",\n          \"file\",\n          \"status\",\n          \"cause\",\n          \"seconds\",\n          \"tries\",\n          \"timesFailed\",\n          \"retries\",\n          \"notRunReason\",\n          \"warnings\",\n          \"recordings\",\n          \"failures\"\n        ],\n        \"type\": \"object\"\n      },\n      \"type\": \"array\"\n    },\n    \"interrupted\": {\n      \"description\": \"\",\n      \"type\": \"integer\"\n    },\n    \"notRun\": {\n      \"description\": \"\",\n      \"type\": \"integer\"\n    },\n    \"ok\": {\n      \"description\": \"Nothing failed, was interrupted or didn't run: as the exit code\",\n      \"type\": \"boolean\"\n    },\n    \"order\": {\n      \"description\": \"--shuffle=\u003cseed\u003e, if shuffled\",\n      \"type\": \"string\"\n    },\n    \"passed\": {\n      \"description\": \"Also those that failed first\",\n      \"type\": \"integer\"\n    },\n    \"retried\": {\n      \"description\": \"Flows tried more than once, whether they passed or not\",\n      \"type\": \"integer\"\n    },\n    \"run\": {\n      \"description\": \"The suite's name, or what was run\",\n      \"type\": \"string\"\n    },\n    \"seconds\": {\n      \"description\": \"All of it\",\n      \"type\": \"number\"\n    },\n    \"stopped\": {\n      \"description\": \"Why, if it stopped before the end\",\n      \"type\": \"string\"\n    },\n    \"total\": {\n      \"description\": \"Flows\",\n      \"type\": \"integer\"\n    },\n    \"version\": {\n      \"description\": \"Changes if the meaning of a key does\",\n      \"type\": \"integer\"\n    }\n  },\n  \"required\": [\n    \"version\",\n    \"run\",\n    \"device\",\n    \"began\",\n    \"seconds\",\n    \"order\",\n    \"stopped\",\n    \"ok\",\n    \"total\",\n    \"passed\",\n    \"failed\",\n    \"interrupted\",\n    \"notRun\",\n    \"retried\",\n    \"flows\"\n  ],\n  \"title\": \"Results of rayo\",\n  \"type\": \"object\"\n}\n",
  "schema/suite.json": "{\n  \"$schema\": \"http://json-schema.org/draft-07/schema#\",\n  \"additionalProperties\": false,\n  \"properties\": {\n    \"app\": {\n      \"additionalProperties\": false,\n      \"description\": \"Installed before the flows\",\n      \"properties\": {\n        \"apk\": {\n          \"description\": \"What to install, if the device has another. rayo doesn't build it (required)\",\n          \"type\": [\n            \"string\",\n            \"number\",\n            \"boolean\"\n          ]\n        },\n        \"id\": {\n          \"description\": \"The app's (required)\",\n          \"type\": [\n            \"string\",\n            \"number\",\n            \"boolean\"\n          ]\n        }\n      },\n      \"required\": [\n        \"id\",\n        \"apk\"\n      ],\n      \"type\": \"object\"\n    },\n    \"device\": {\n      \"additionalProperties\": false,\n      \"description\": \"How it is while the flows run\",\n      \"properties\": {\n        \"darkMode\": {\n          \"description\": \"The dark theme\",\n          \"type\": \"boolean\"\n        },\n        \"dpi\": {\n          \"description\": \"Its density. With size, it says how many dp there are: 1080 pixels at 420 dpi are 411 dp\",\n          \"maximum\": 1000,\n          \"minimum\": 72,\n          \"type\": \"integer\"\n        },\n        \"fontScale\": {\n          \"description\": \"Of the text; 1 is normal\",\n          \"maximum\": 3,\n          \"minimum\": 0.5,\n          \"type\": \"number\"\n        },\n        \"size\": {\n          \"description\": \"Of the screen\",\n          \"pattern\": \"^[1-9]\\\\d{2,3}x[1-9]\\\\d{2,3}$\",\n          \"type\": \"string\"\n        }\n      },\n      \"type\": \"object\"\n    },\n    \"env\": {\n      \"additionalProperties\": {\n        \"type\": [\n          \"string\",\n          \"number\",\n          \"boolean\"\n        ]\n      },\n      \"description\": \"Variables for every flow, as -e, which wins; they may use ${...}, such as ${rayo.file}\",\n      \"type\": \"object\"\n    },\n    \"flows\": {\n      \"description\": \"Files, run in this order; or mappings with file, retries, mock, network and unmocked (required)\",\n      \"items\": {\n        \"anyOf\": [\n          {\n            \"type\": \"string\"\n          },\n          {\n            \"additionalProperties\": false,\n            \"properties\": {\n              \"file\": {\n                \"description\": \"The flow (required)\",\n                \"type\": [\n                  \"string\",\n                  \"number\",\n                  \"boolean\"\n                ]\n              },\n              \"mock\": {\n                \"anyOf\": [\n                  {\n                    \"type\": \"string\"\n                  },\n                  {\n                    \"items\": {\n                      \"type\": \"string\"\n                    },\n                    \"type\": \"array\"\n                  }\n                ],\n                \"description\": \"Who answers as the backend, as a flow's mock, relative to the suite; the flow's own mock wins\"\n              },\n              \"network\": {\n                \"description\": \"How the app's calls reach the mock, as a flow's network; the flow's own and --network win (default: the suite's)\",\n                \"enum\": [\n                  \"interceptor\",\n                  \"proxy\"\n                ]\n              },\n              \"retries\": {\n                \"description\": \"How many times it is tried again if it fails (default: the suite's)\",\n                \"minimum\": 0,\n                \"type\": \"integer\"\n              },\n              \"unmocked\": {\n                \"additionalProperties\": false,\n                \"description\": \"Calls without a mock that don't fail the flow (default: the suite's; its calls are added to them)\",\n                \"properties\": {\n                  \"body\": {\n                    \"description\": \"Of the answer (default: not mocked:  and the call)\",\n                    \"type\": [\n                      \"string\",\n                      \"number\",\n                      \"boolean\"\n                    ]\n                  },\n                  \"ignore\": {\n                    \"description\": \"Calls that no mock answers but that don't fail the flow: each graphql:Operation, host/path, host or /path, as --ignore-unmocked, or a mapping with host, path and graphql (required)\",\n                    \"items\": {\n                      \"anyOf\": [\n                        {\n                          \"type\": \"string\"\n                        },\n                        {\n                          \"additionalProperties\": false,\n                          \"properties\": {\n                            \"graphql\": {\n                              \"anyOf\": [\n                                {\n                                  \"type\": \"string\"\n                                },\n                                {\n                                  \"items\": {\n                                    \"type\": \"string\"\n                                  },\n                                  \"type\": \"array\"\n                                }\n                              ],\n                              \"description\": \"GraphQL operations (default: any call)\"\n                            },\n                            \"host\": {\n                              \"description\": \"Of the call, or of its subdomains (default: any)\",\n                              \"type\": [\n                                \"string\",\n                                \"number\",\n                                \"boolean\"\n                              ]\n                            },\n                            \"path\": {\n                              \"description\": \"Of the call, without the host; * is any text but /, ** any (default: any)\",\n                              \"type\": [\n                                \"string\",\n                                \"number\",\n                                \"boolean\"\n                              ]\n                            }\n                          },\n                          \"type\": \"object\"\n                        }\n                      ]\n                    },\n                    \"type\": \"array\"\n                  },\n                  \"status\": {\n                    \"description\": \"What those calls are answered with (default: 404)\",\n                    \"maximum\": 599,\n                    \"minimum\": 100,\n                    \"type\": \"integer\"\n                  }\n                },\n                \"required\": [\n                  \"ignore\"\n                ],\n                \"type\": \"object\"\n              }\n            },\n            \"required\": [\n              \"file\"\n            ],\n            \"type\": \"object\"\n          }\n        ]\n      },\n      \"minItems\": 1,\n      \"type\": \"array\"\n    },\n    \"name\": {\n      \"description\": \"Shown in the results (default: the file)\",\n      \"type\": [\n        \"string\",\n        \"number\",\n        \"boolean\"\n      ]\n    },\n    \"network\": {\n      \"description\": \"How the app's calls reach the mock, as a flow's network; the flow's own and --network win (default: each flow's, else interceptor)\",\n      \"enum\": [\n        \"interceptor\",\n        \"proxy\"\n      ]\n    },\n    \"output\": {\n      \"description\": \"As --output, which wins (default: .rayo where rayo runs)\",\n      \"type\": [\n        \"string\",\n        \"number\",\n        \"boolean\"\n      ]\n    },\n    \"record\": {\n      \"description\": \"A video of each flow that doesn't say (default: false)\",\n      \"type\": \"boolean\"\n    },\n    \"recordFolder\": {\n      \"description\": \"Where the videos go; needs record: true (default: each flow's, in output)\",\n      \"type\": [\n        \"string\",\n        \"number\",\n        \"boolean\"\n      ]\n    },\n    \"results\": {\n      \"anyOf\": [\n        {\n          \"pattern\": \"\\\\.(txt|json|xml)$\",\n          \"type\": \"string\"\n        },\n        {\n          \"items\": {\n            \"pattern\": \"\\\\.(txt|json|xml)$\",\n            \"type\": \"string\"\n          },\n          \"type\": \"array\"\n        }\n      ],\n      \"description\": \"As --results, which wins over all of them: .txt, .json or .xml (JUnit)\"\n    },\n    \"retries\": {\n      \"description\": \"How many times a flow that fails is tried again (default: 1)\",\n      \"minimum\": 0,\n      \"type\": \"integer\"\n    },\n    \"timeoutScale\": {\n      \"description\": \"Multiplies the default waits; --timeout-scale wins (default: 1)\",\n      \"maximum\": 10,\n      \"minimum\": 0.1,\n      \"type\": \"number\"\n    },\n    \"unmocked\": {\n      \"additionalProperties\": false,\n      \"description\": \"Calls without a mock that don't fail the flow (default: none: each fails the flow)\",\n      \"properties\": {\n        \"body\": {\n          \"description\": \"Of the answer (default: not mocked:  and the call)\",\n          \"type\": [\n            \"string\",\n            \"number\",\n            \"boolean\"\n          ]\n        },\n        \"ignore\": {\n          \"description\": \"Calls that no mock answers but that don't fail the flow: each graphql:Operation, host/path, host or /path, as --ignore-unmocked, or a mapping with host, path and graphql (required)\",\n          \"items\": {\n            \"anyOf\": [\n              {\n                \"type\": \"string\"\n              },\n              {\n                \"additionalProperties\": false,\n                \"properties\": {\n                  \"graphql\": {\n                    \"anyOf\": [\n                      {\n                        \"type\": \"string\"\n                      },\n                      {\n                        \"items\": {\n                          \"type\": \"string\"\n                        },\n                        \"type\": \"array\"\n                      }\n                    ],\n                    \"description\": \"GraphQL operations (default: any call)\"\n                  },\n                  \"host\": {\n                    \"description\": \"Of the call, or of its subdomains (default: any)\",\n                    \"type\": [\n                      \"string\",\n                      \"number\",\n                      \"boolean\"\n                    ]\n                  },\n                  \"path\": {\n                    \"description\": \"Of the call, without the host; * is any text but /, ** any (default: any)\",\n                    \"type\": [\n                      \"string\",\n                      \"number\",\n                      \"boolean\"\n                    ]\n                  }\n                },\n                \"type\": \"object\"\n              }\n            ]\n          },\n          \"type\": \"array\"\n        },\n        \"status\": {\n          \"description\": \"What those calls are answered with (default: 404)\",\n          \"maximum\": 599,\n          \"minimum\": 100,\n          \"type\": \"integer\"\n        }\n      },\n      \"required\": [\n        \"ignore\"\n      ],\n      \"type\": \"object\"\n    }\n  },\n  \"required\": [\n    \"flows\"\n  ],\n  \"title\": \"A rayo suite, *.rayo.yaml\",\n  \"type\": \"object\"\n}\n"
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
     "about": "Keeps an element's text in rayo.copiedText",
     "example": "- copyTextFrom: { id: order_number }\n- tapOn: { id: search }\n- pasteText",
     "tested": false,
     "alone": false,
     "element": true,
     "keys": []
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
     "about": "Moves scenarios to a state, such as { ride: docked }",
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
       "about": "As long as these hold",
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
     "about": "Scrolls until the element is on screen",
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
    "about": "when several are connected",
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
    "name": "output",
    "short": "",
    "takes": "folder",
    "about": "where failures and screenshots go (default .rayo)",
    "commands": [
     "test",
     "live",
     "do"
    ]
   },
   {
    "name": "env",
    "short": "e",
    "takes": "KEY=VALUE",
    "about": "a variable for the flows",
    "commands": [
     "test",
     "live"
    ]
   },
   {
    "name": "timeout-scale",
    "short": "",
    "takes": "factor",
    "about": "multiply the default waits, not a step's own timeout (default 1)",
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
     "test"
    ]
   },
   {
    "name": "include-tags",
    "short": "",
    "takes": "a,b",
    "about": "only flows with one of these tags",
    "commands": [
     "test"
    ]
   },
   {
    "name": "exclude-tags",
    "short": "",
    "takes": "a,b",
    "about": "not flows with one of these tags",
    "commands": [
     "test"
    ]
   },
   {
    "name": "rerun-failed",
    "short": "",
    "takes": "",
    "about": "only those that haven't passed since they last failed",
    "commands": [
     "test"
    ]
   },
   {
    "name": "retries",
    "short": "",
    "takes": "n",
    "about": "try again what fails, whatever the suite says (default 1)",
    "commands": [
     "test"
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
    "name": "continuous",
    "short": "c",
    "takes": "",
    "about": "run again whenever a file changes",
    "commands": [
     "test"
    ]
   },
   {
    "name": "verbose",
    "short": "v",
    "takes": "",
    "about": "every step (always, for a single flow)",
    "commands": [
     "test"
    ]
   },
   {
    "name": "results",
    "short": "",
    "takes": "file",
    "about": "write the results: .txt, .json or .xml (JUnit); may be repeated",
    "commands": [
     "test"
    ]
   },
   {
    "name": "timings",
    "short": "",
    "takes": "",
    "about": "each step's time, each runFlow's total, the slowest steps and flows, and a note while a step hangs",
    "commands": [
     "test"
    ]
   },
   {
    "name": "failures-only",
    "short": "",
    "takes": "",
    "about": "leave the flows that passed out of the results",
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
    "about": "capture into folder/.update, and say what changed from the mocks in folder; rayo mocks apply takes it",
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
    "about": "interceptor, the app sends them (default, or the flow's); proxy, the device's proxy does",
    "commands": [
     "test",
     "live"
    ]
   },
   {
    "name": "ignore-unmocked",
    "short": "",
    "takes": "a,b",
    "about": "calls without a mock that don't fail the flow, answered 404: graphql:Operation, host/path, host or /path (* any text but /, ** any)",
    "commands": [
     "test",
     "live"
    ]
   },
   {
    "name": "hosts",
    "short": "",
    "takes": "a,b",
    "about": "with a proxy, the hosts to open, and their subdomains (default: capturing, all but Google's, Firebase's, analytics and crash reports; else those of the mocks)",
    "commands": [
     "test",
     "live"
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
    "takes": "",
    "about": "also what is on screen afterwards",
    "commands": [
     "do"
    ]
   },
   {
    "name": "json",
    "short": "",
    "takes": "",
    "about": "for a program",
    "commands": [
     "do"
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
    "about": "Files, run in this order; or mappings with file, retries, mock, network and unmocked (required)",
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
    "about": "Variables for every flow, as -e, which wins; they may use ${...}, such as ${rayo.file}",
    "required": false
   },
   {
    "name": "timeoutScale",
    "takes": "0.1 to 10",
    "about": "Multiplies the default waits; --timeout-scale wins (default: 1)",
    "required": false
   },
   {
    "name": "network",
    "takes": "interceptor or proxy",
    "about": "How the app's calls reach the mock, as a flow's network; the flow's own and --network win (default: each flow's, else interceptor)",
    "required": false
   },
   {
    "name": "unmocked",
    "takes": "a mapping",
    "about": "Calls without a mock that don't fail the flow (default: none: each fails the flow)",
    "required": false
   },
   {
    "name": "output",
    "takes": "folder",
    "about": "As --output, which wins (default: .rayo where rayo runs)",
    "required": false
   },
   {
    "name": "results",
    "takes": "a file or a list",
    "about": "As --results, which wins over all of them: .txt, .json or .xml (JUnit)",
    "required": false
   },
   {
    "name": "record",
    "takes": "true or false",
    "about": "A video of each flow that doesn't say (default: false)",
    "required": false
   },
   {
    "name": "recordFolder",
    "takes": "folder",
    "about": "Where the videos go; needs record: true (default: each flow's, in output)",
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
    "about": "How the app's calls reach the mock, as a flow's network; the flow's own and --network win (default: the suite's)",
    "required": false
   },
   {
    "name": "unmocked",
    "takes": "a mapping",
    "about": "Calls without a mock that don't fail the flow (default: the suite's; its calls are added to them)",
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
