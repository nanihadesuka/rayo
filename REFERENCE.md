# rayo reference

Every command and feature. To begin, see the [README](README.md).

🧪 not tested on a device yet. `rayo help <command>` shows what one takes.

The tables between `generated` marks come from the code, as do `rayo help` and the JSON Schemas
(`rayo schema flow`, `suite` or `results`; also in [schema/](schema)). `go test ./test -update` writes them.

## Index

| Topic | |
|---|---|
| [Options](#options) | As `rayo help` prints them |
| [Flow file](#flow-file) | [Header](#header) · [Common options](#common-options) · [Values](#values) |
| [Selectors](#selectors) | [Conditions](#conditions) · [Relations](#relations) |
| [Apps](#apps) | [launchApp](#launchapp) · [stopApp](#stopapp-killapp) · [killApp](#stopapp-killapp) · [clearState](#clearstate) · [setPermissions](#setpermissions) · [openLink](#openlink) |
| [Touch](#touch) | [tapOn](#tapon) · [doubleTapOn](#doubletapon) · [longPressOn](#longpresson) · [swipe](#swipe) · [scroll](#scroll) · [scrollUntilVisible](#scrolluntilvisible) |
| [Text and keys](#text-and-keys) | [type](#type) · [inputText](#inputtext) · [inputRandom…](#inputrandom) · [eraseText](#erasetext) · [copyTextFrom](#copytextfrom-pastetext) · [pasteText](#copytextfrom-pastetext) · [hideKeyboard](#hidekeyboard) · [pressKey](#presskey) · [back](#back) |
| [Checks and waits](#checks-and-waits) | [assertVisible](#assertvisible) · [assertNotVisible](#assertnotvisible) · [extendedWaitUntil](#extendedwaituntil) · [assertTrue](#asserttrue) · [waitForAnimationToEnd](#waitforanimationtoend) |
| [Control](#control) | [runFlow](#runflow) · [repeat](#repeat) · [retry](#retry) · [when](#when) |
| [Scripts](#scripts) | [Expressions](#expressions) · [runScript](#runscript) · [evalScript](#evalscript) · [JavaScript API](#javascript-api) |
| [Device](#device) | [setLocation](#setlocation) · [travel](#travel) · [setAirplaneMode](#setairplanemode) · [takeScreenshot](#takescreenshot) · [startRecording](#startrecording-stoprecording) · [stopRecording](#startrecording-stoprecording) · [camera](#camera) · [shell](#shell) |
| [Server](#server) | [http](#http) |
| [Mock](#mock) | [Files](#mock-files) · [mock](#mock-step) · [mockState](#mockstate) · [network](#network) · [waitForRequest](#waitforrequest) · [Capture](#capture) |
| [Running several](#running-several) | [Suites](#suites) · [Clean state](#clean-state) · [Retries](#retries) · [Causes](#causes) · [Outcomes](#outcomes) · [Results](#results) |
| [Live sessions](#live-sessions) | [Commands](#live-commands) · [From other programs](#from-other-programs) |
| [Features](#features) | [Schemas](#schemas) · [Variables](#variables) · [Tags](#tags) · [Continuous mode](#continuous-mode) · [Failures](#failures) · [Timings](#timings) · [Profile](#profile) · [Report](#report) · [Environment](#environment) |
| [Default times](#default-times) · [Limitations](#limitations) | |

---

## Options

As `rayo help` prints them. An option of another command is an error, never ignored.

```
rayo - runs flows on Android, fast

  rayo test [options] <flow, folder or suite>...   run flows; a suite is a *.rayo.yaml
  rayo check <flow, folder or suite>...            check flows without a device
  rayo live [options] <flow>                       write a flow as it runs, command by command
  rayo do [options] <commands>                     send commands to the live session; rayo do .help
  rayo hierarchy [options]                         what is on screen, and how to refer to it
  rayo devices                                     connected devices
  rayo help <command>                              what a command takes, of a flow or of rayo; also flow, suite and json
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
  --timings                  each step's time, each runFlow's total, the slowest steps and flows, and a note while a step hangs
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

---

## Flow file

Two parts separated by `---`: a header and a list of commands.

```yaml
appId: com.example.app
---
- launchApp
- tapOn: Log in
```

### Header

<!-- generated: header -->
| Key | Takes | |
|---|---|---|
| `appId` | package | The app's |
| `name` | text | Shown in the output (default: the file's, or the folder's for `flow.yaml`) |
| `tags` | a list | To choose flows, see [Tags](#tags) |
| `env` | values | Variables for it, see [Variables](#variables) |
| `mock` | a path or a list | Who answers as the backend, see [Mock](#mock) |
| `record` | `true`, `false` or a file | A video of the flow, see [Recording a whole flow](#recording-a-whole-flow) (default: the suite's) |
| `onFlowStart` | commands | Run first |
| `onFlowComplete` | commands | Run last, also after a failure |
<!-- end -->

### Common options

Accepted by every command that takes a mapping.

<!-- generated: options -->
| Key | Takes | |
|---|---|---|
| `label` | text | What to show instead of the command |
| `optional` | true/false | A failure doesn't fail the flow |
| `timeout` | time | How long it may take |
<!-- end -->

### Values

| Value | Written as |
|---|---|
| Time | `500` (milliseconds) or `10s` |
| Point | `35%, 30%` of the screen, or `540, 1200` pixels |
| Location | `41.3785, 2.1765` (latitude, longitude) |
| Short form | `- tapOn: Log in` is `- tapOn: { text: Log in }` |

---

## Selectors

How a command refers to an element. All the conditions given must hold. Run `rayo hierarchy` to
see what is on screen.

### Conditions

<!-- generated: selector -->
| Key | Takes | |
|---|---|---|
| `id` | text | The resource id, whole or after :id/; in Compose, a testTag with testTagsAsResourceId. A regular expression of the whole |
| `text` | text | The text, description or hint. A regular expression of the whole |
| `index` | number | The n-th match, from 0, top to bottom and left to right; negative counts from the end |
| `enabled` | true/false | Whether it is enabled |
| `checked` | true/false | Whether it is checked |
| `focused` | true/false | Whether it has the focus |
| `selected` | true/false | Whether it is selected |
| `below` | selector | Entirely below another |
| `above` | selector | Entirely above another |
| `leftOf` | selector | Entirely to the left of another |
| `rightOf` | selector | Entirely to the right of another |
| `containsChild` | selector | The direct parent of another |
| `childOf` | selector | Anywhere inside another |
| `containsDescendants` | selectors | Around all of these |
<!-- end -->

`id` and `text` are regular expressions that must match the **whole** value, ignoring case. An
exact value always matches too, so `Price (€)` needs no escaping.

With several matches and no `index`, the first in drawing order of the topmost window is used. If one match contains
another, the inner one wins.

### Relations

🧪 Each takes another selector.

```yaml
- tapOn: { text: Delete, below: { id: row_title, text: Invoice 7 } }
```

---

## Apps

### launchApp

Opens the app and waits until Android has it in front.

<!-- generated: keys launchApp -->
| Key | Takes | |
|---|---|---|
| `appId` | text | The app; by default the header's. Short form |
| `clearState` | true/false | Erase its data first (false) |
| `stopApp` | true/false | Close it first (true); false brings it back as it was |
| `permissions` | permissions | Name: allow, deny or unset. With clearState, all are allowed if not said |
| `arguments` | values | Intent extras; booleans, integers and decimals keep their type |
<!-- end -->

Permission names: `all`, `location`, `camera`, `notifications`, `contacts`, `microphone`,
`bluetooth`, `storage`, `phone`, `calendar`, `sms`, `medialibrary`, or Android's own, such as
`READ_CONTACTS`. Only `all` has been tested.

```yaml
- launchApp: { clearState: true, permissions: { all: allow }, arguments: { mock: true } }
```

### stopApp, killApp

🧪 Both close the app. Short form: the package.

<!-- generated: keys stopApp -->
| Key | Takes | |
|---|---|---|
| `appId` | text | The app; by default the header's. Short form |
<!-- end -->

### clearState

🧪 Erases the app's data. Short form: the package.

<!-- generated: keys clearState -->
| Key | Takes | |
|---|---|---|
| `appId` | text | The app; by default the header's. Short form |
<!-- end -->

### setPermissions

Changes permissions without opening the app, in the order given. Takes `permissions`, as in
[launchApp](#launchapp), and `appId`. Fails if the app has no such permission. Android closes a
running app when it loses one.

<!-- generated: keys setPermissions -->
| Key | Takes | |
|---|---|---|
| `appId` | text | The app; by default the header's |
| `permissions` | permissions | Name: allow, deny or unset. With clearState, all are allowed if not said |
<!-- end -->

```yaml
- setPermissions: { permissions: { all: deny, camera: allow } }
```

### openLink

🧪 Opens a link in whichever app handles it.

<!-- generated: keys openLink -->
| Key | Takes | |
|---|---|---|
| `link` | text | What to open. Short form |
<!-- end -->

```yaml
- openLink: https://example.com/item/7
```

---

## Touch

### tapOn

Waits until the element is on screen, enabled and not moving, then taps its centre.

<!-- generated: keys tapOn -->
| Key | Takes | |
|---|---|---|
| `point` | point | Alone, of the screen; with a selector, of the element in % |
| `repeat` | number | Taps (1) |
| `delay` | time | Between taps (100) |
<!-- end -->

```yaml
- tapOn: { id: save }
- tapOn: { point: "35%, 30%" }
```

### doubleTapOn

🧪 Like `tapOn`, twice. `delay` is 100 ms by default.

<!-- generated: keys doubleTapOn -->
| Key | Takes | |
|---|---|---|
| `point` | point | Alone, of the screen; with a selector, of the element in % |
| `delay` | time | Between taps (100) |
<!-- end -->

### longPressOn

🧪 Like `tapOn`, held for a second.

<!-- generated: keys longPressOn -->
| Key | Takes | |
|---|---|---|
| `point` | point | Alone, of the screen; with a selector, of the element in % |
<!-- end -->

### swipe

🧪

<!-- generated: keys swipe -->
| Key | Takes | |
|---|---|---|
| `direction` | UP/DOWN/LEFT/RIGHT | Where the finger or the search goes (DOWN when searching) |
| `start` | point | Where the finger goes down, instead of direction |
| `end` | point | Where it goes up |
| `duration` | time | Of the swipe (250) |
| `from` | selector | Start on this element; with direction |
<!-- end -->

### scroll

🧪 Scrolls down once.

### scrollUntilVisible

🧪 Scrolls until the element is on screen.

<!-- generated: keys scrollUntilVisible -->
| Key | Takes | |
|---|---|---|
| `element` | selector | What to look for |
| `direction` | UP/DOWN/LEFT/RIGHT | Where the finger or the search goes (DOWN when searching) |
<!-- end -->

---

## Text and keys

### type

Replaces the text of a field at once, without focus or keyboard. The quickest way to fill a
form; `pressKey: Enter` won't follow it, as the field isn't focused.

<!-- generated: keys type -->
| Key | Takes | |
|---|---|---|
| `value` | text | The new text |
<!-- end -->

```yaml
- type: { id: email, value: me@example.com }
```

### inputText

Adds text to the focused field, waiting for one to be focused.

<!-- generated: keys inputText -->
| Key | Takes | |
|---|---|---|
| `text` | text | The text to add. Short form |
<!-- end -->

```yaml
- tapOn: { id: email }
- inputText: me@example.com
```

### inputRandom…

🧪 Like `inputText`, with something random.

| Command | |
|---|---|
| `inputRandomEmail` | |
| `inputRandomPersonName` | |
| `inputRandomNumber` | `length`, 8 by default |
| `inputRandomText` | `length`, 8 by default |

### eraseText

🧪 Deletes characters from the end of the focused field: 50, or the number given.

<!-- generated: keys eraseText -->
| Key | Takes | |
|---|---|---|
| `charactersToErase` | number | How many (50). Short form |
<!-- end -->

### copyTextFrom, pasteText

🧪 `copyTextFrom` keeps an element's text, or its description if it has none, in
`rayo.copiedText`. `pasteText` adds it to the focused field.

### hideKeyboard

🧪 Closes the keyboard if it is open, and waits until it has. Does nothing when none is open.

### pressKey

`Home`, `Back`, `Enter`, `Tab`, `Backspace`, `Delete`, `Escape`, `Space`, `Menu`, `Search`,
`Recents`, `Power`, `Lock`, `Volume Up`, `Volume Down`, `Remote Dpad …`, `Remote Media …`, or an
Android key code. Only `Home` and `Enter` have been tested.

<!-- generated: keys pressKey -->
| Key | Takes | |
|---|---|---|
| `key` | key | A name, such as Enter, or an Android key code. Short form |
<!-- end -->

`Home` also waits until the app has really stopped, so that opening it again counts.

### back

`pressKey: Back`.

---

## Checks and waits

Each returns as soon as it holds; none needs a fixed pause.

### assertVisible

🧪 The element is on screen, or appears in time.

### assertNotVisible

🧪 The element isn't on screen, or goes away in time.

### extendedWaitUntil

The same two, usually with a longer `timeout`.

<!-- generated: keys extendedWaitUntil -->
| Key | Takes | |
|---|---|---|
| `visible` | selector | On screen |
| `notVisible` | selector | Not on screen |
<!-- end -->

```yaml
- extendedWaitUntil: { visible: { id: home }, timeout: 20s }
- extendedWaitUntil: { notVisible: Loading.* }
```

### assertTrue

🧪 An [expression](#expressions) is true.

<!-- generated: keys assertTrue -->
| Key | Takes | |
|---|---|---|
| `condition` | JavaScript | An expression. Short form |
<!-- end -->

```yaml
- assertTrue: ${output.total == 3}
```

### waitForAnimationToEnd

🧪 Waits until nothing on screen has changed for `quiet` (500 ms). Goes on when the `timeout` is
over, without failing.

It can't know about what hasn't begun: a dialog that comes a second later may still be missing. To
wait for something late, wait for it by name, as `extendedWaitUntil` or a `when` with a longer
`timeout`.

<!-- generated: keys waitForAnimationToEnd -->
| Key | Takes | |
|---|---|---|
| `quiet` | time | How long nothing may change (500ms) |
<!-- end -->

```yaml
- waitForAnimationToEnd: { quiet: 1s, timeout: 5s }
```

---

## Control

### runFlow

<!-- generated: keys runFlow -->
| Key | Takes | |
|---|---|---|
| `file` | text | A path relative to this flow. Short form |
| `env` | values | Variables for it |
| `when` | conditions | Only if these hold |
| `commands` | commands | What to run |
<!-- end -->

```yaml
- runFlow: ../shared/login.yaml
```

### repeat

🧪 Runs `commands` again, up to `times`, while [`while`](#when) holds. One of the two is required.

<!-- generated: keys repeat -->
| Key | Takes | |
|---|---|---|
| `times` | number | At most |
| `while` | conditions | As long as these hold |
| `commands` | commands | What to run |
<!-- end -->

### retry

🧪 Runs `commands` or a `file`, again if it fails, up to `retries` (1) more times.

<!-- generated: keys retry -->
| Key | Takes | |
|---|---|---|
| `retries` | number | How many more times (1) |
| `commands` | commands | What to run |
| `file` | text | A path relative to this flow |
| `env` | values | Variables for it |
<!-- end -->

### when

🧪 A condition for `runFlow`, `runScript` and, as `while`, `repeat`. All must hold. `visible` and
`notVisible` wait up to `timeout` (0.7 s) for it; `timeout: 0` looks once, for a guard that
usually doesn't hold.

```yaml
- runFlow:
    when: { visible: "Allow notifications", timeout: 3s }
    commands: [ { tapOn: "Not now" } ]
```

<!-- generated: when -->
| Key | Takes | |
|---|---|---|
| `visible` | selector | On screen |
| `notVisible` | selector | Not on screen |
| `platform` | Android/iOS/Web | Android |
| `true` | JavaScript | An expression that holds |
| `timeout` | time | How long visible and notVisible may take (700ms); 0 looks once |
<!-- end -->

---

## Scripts

### Expressions

`${…}` is JavaScript, anywhere in a value. Something undefined is an error, not the word
"undefined".

```yaml
- inputText: ${output.user.name}
```

### runScript

Runs a file. `env` gives it variables. What it declares stays inside: share through `output`.

<!-- generated: keys runScript -->
| Key | Takes | |
|---|---|---|
| `file` | text | A path relative to this flow. Short form |
| `env` | values | Variables for it |
| `when` | conditions | Only if these hold |
<!-- end -->

```yaml
- runScript: { file: create_user.js, env: { NAME: Ana } }
```

### evalScript

🧪 Runs a line.

<!-- generated: keys evalScript -->
| Key | Takes | |
|---|---|---|
| `script` | JavaScript | A line of it. Short form |
<!-- end -->

```yaml
- evalScript: ${output.count = 1}
```

### JavaScript API

| | |
|---|---|
| `output` | An object shared by the whole flow |
| `http.get/post/put/delete(url, options)`, `http.request(url, options)` | `options`: `headers`, `body`, `method`. Returns `ok`, `status`, `body` |
| `json(text)` | Parses |
| `console.log(…)` | Prints |
| `rayo.platform`, `rayo.copiedText` | |

Only `http.put` has been tested. A script may take up to 60 seconds. There is no `require`, `fetch` or `async`.

---

## Device

### setLocation

Stays at a place, sending a fix every second like a real GPS, until the flow ends. Needs
Android 12.

<!-- generated: keys setLocation -->
| Key | Takes | |
|---|---|---|
| `latitude` | number | In degrees |
| `longitude` | number | In degrees |
<!-- end -->

```yaml
- setLocation: { latitude: 41.3785, longitude: 2.1765 }
```

### travel

🧪 Moves along `points` at `speed` metres per second (7 by default), then stays at the last.

<!-- generated: keys travel -->
| Key | Takes | |
|---|---|---|
| `points` | locations | A list of 'latitude,longitude' |
| `speed` | number | Metres per second (7) |
<!-- end -->

```yaml
- travel: { points: ["41.3785,2.1765", "41.3805,2.1748"], speed: 6 }
```

### setAirplaneMode

🧪 `enabled` or `disabled`. The end of the flow puts it back as it was.

<!-- generated: keys setAirplaneMode -->
| Key | Takes | |
|---|---|---|
| `value` | enabled/disabled/true/false | enabled or disabled. Short form |
<!-- end -->

### takeScreenshot

🧪 Saves `<output>/<flow>/<name>.png`.

<!-- generated: keys takeScreenshot -->
| Key | Takes | |
|---|---|---|
| `path` | text | The file, in the debug output. Short form |
<!-- end -->

### startRecording, stopRecording

Records the screen as an mp4. The recording ends with `stopRecording` or with the flow, also if it
fails. Without a path it has the flow's name.

<!-- generated: keys startRecording -->
| Key | Takes | |
|---|---|---|
| `path` | text | The file, in the debug output; by default the flow's name. Short form |
<!-- end -->

```yaml
- startRecording                      # <output>/<flow>/<flow>.mp4
- startRecording: login               # <output>/<flow>/login.mp4
- startRecording: { path: login }     # the same
- stopRecording
```

#### Recording a whole flow

`record` in its header; or in a [suite](#suites), for those of its flows that don't say:

| Where | | Saved as |
|---|---|---|
| Flow header | `record: true` | `<output>/<flow>/<flow>.mp4` |
| | `record: name`, or an absolute path | `<output>/<flow>/name.mp4`, or that path |
| | `record: false` | Not recorded, whatever the suite says |
| Suite | `record: true` | `<output>/<flow>/<flow>.mp4` |
| | and `recordFolder: videos`, relative to the suite | `videos/<flow>.mp4` |

Each try of a flow has its own, such as `<flow>_retry1.mp4`. One recording at a time: with one in
the header, another needs `stopRecording` first. At most 30 minutes, or 3 before Android 11 or so;
the rest isn't recorded. Without sound, and small on purpose: 480 pixels on the short side, 0.6 Mbps,
which is under 5 MB a minute. The device encodes it as it records.

### camera

Hangs a picture on the wall of the emulator's virtual room and walks up to it, such as a QR to
scan. The walk starts 3 seconds later, while the flow goes on, so use it early. `none` puts the
room back, and so does the end of the flow.

<!-- generated: keys camera -->
| Key | Takes | |
|---|---|---|
| `image` | text | What it sees, a path relative to this flow; or none. Short form |
<!-- end -->

```yaml
- camera: device/qr.png
```

The emulator's back camera must be set to *VirtualScene*. On a real device, or if the emulator
doesn't answer, the step says so and passes.

### shell

Runs a command on the device as the shell user. Fails if its exit code isn't 0.

<!-- generated: keys shell -->
| Key | Takes | |
|---|---|---|
| `command` | text | Run as the shell user. Short form |
<!-- end -->

```yaml
- shell: am broadcast -a com.example.REFRESH -p com.example.app
```

---

## Server

### http

🧪 Calls a server from the computer, not the device.

<!-- generated: keys http -->
| Key | Takes | |
|---|---|---|
| `method` | text | Such as POST (GET) |
| `url` | text | What to call, from the computer. Short form |
| `json` | values | The body |
| `expect` | number | The status (any below 300) |
<!-- end -->

```yaml
- http: { method: PUT, url: "http://127.0.0.1:8080/state", json: { state: docked } }
```

---

## Mock

rayo answers as the backend while a flow runs. The app must send its calls to
`http://127.0.0.1:8080` on the device (another port with `--mock-port`); rayo sets up
`adb reverse` and removes it. HTTP only. Every flow starts afresh, and **fails if the app calls
something without a mock**.

```yaml
appId: com.example.app
mock:
  - ../shared/mock          # the normal answers
  - overrides.json          # what differs here
---
- tapOn: { id: pay }
- waitForRequest: { method: POST, path: ".*/payments" }
- mockState: { order: shipped }
- network: offline
```

An included flow's mocks are added when it runs. How mocks work, with examples:
[Mocking](MOCKING.md).

### Mock files

A folder with `mappings/` and `__files/`, or one JSON file of mappings, in WireMock's format.
Only the following is supported; anything else is an error.

| Part | |
|---|---|
| Mapping | `id`, `name`, `priority`, `scenarioName`, `requiredScenarioState`, `newScenarioState`, `request`, `response` |
| `request` | `method`, one of `url` `urlPattern` `urlPath` `urlPathPattern`, `queryParameters`, `headers`, `bodyPatterns` |
| Matchers | `equalTo`, `contains`, `matches`, `absent`, `matchesJsonPath` (`$.a`, `$[0]`, `$['a']`, `$[*]`, and at the end `$[?(@.a == 1)]`) |
| `response` | `status`, `headers`, one of `body` `jsonBody` `bodyFileName`, `fixedDelayMilliseconds` |
| In a body | `{{now}}`, `{{now offset='-8 minutes'}}`, `{{request.path}}` `.url` `.body` `.query.x` `.headers.x`, `{{regexExtract request.query.x '(.+)' 'name'}}` then `{{name.0}}` |

The lowest `priority` wins (5 by default), then the last one added. Scenarios begin in `Started`.
Regular expressions are Go's and match the whole value, except in `regexExtract`.

### mock step

Adds a path, or a mapping written here. Its priority is 0 by default, so it wins over the files.

<!-- generated: keys mock -->
| Key | Takes | |
|---|---|---|
| `file` | text | A path relative to this flow. Short form |
| `id` | text | A name for it |
| `name` | text | A name for it |
| `priority` | number | The lowest wins (0) |
| `scenarioName` | text | The scenario it belongs to |
| `requiredScenarioState` | text | Only in this state |
| `newScenarioState` | text | Moves to this state |
| `request` | values | Which calls |
| `response` | values | What to answer |
<!-- end -->

```yaml
- mock: overrides.json
- mock: { request: { method: POST, urlPathPattern: ".*/payments" }, response: { status: 503 } }
```

### mockState

Moves scenarios to a state. A scenario or state that no mock mentions is an error.

### network

| Form | |
|---|---|
| `network: offline` | Connections close without an answer. Android still believes it is connected: see [setAirplaneMode](#setairplanemode) |
| `network: { delay: 2s }` | Every answer takes longer |
| `network: { fail: 503 }` | Every answer is this status |
| `network: online` | Back to normal |

<!-- generated: keys network -->
| Key | Takes | |
|---|---|---|
| `state` | offline/online | online or offline. Short form |
| `delay` | time | Every answer takes longer |
| `fail` | 400 to 599 | Every answer is this status, 400 to 599 |
| `path` | pattern | Only these calls, a regular expression of the whole path |
<!-- end -->

`path`, a regular expression, limits it to some calls. Each `network` replaces the one before.

### waitForRequest

<!-- generated: keys waitForRequest -->
| Key | Takes | |
|---|---|---|
| `method` | text | Such as POST (GET) |
| `path` | pattern | Only these calls, a regular expression of the whole path. Short form |
| `body` | text | A text in the body |
<!-- end -->

Waits until the app calls `path`, a regular expression of the whole path. Optional: `method`, and
`body`, a text in it. Calls already made in this flow count, but each is matched only once.

### Capture

Makes mocks from a real backend. Run the flow once with

```
rayo test --capture mock -e PASSWORD=... flow.yaml
```

The app needs nothing new: it sends its calls to rayo as with a mock, and rayo passes them on and
keeps the answers. **The calls are real, and do what they do.** One flow, once, into a new folder.

| The app calls | Passed on to |
|---|---|
| `http://127.0.0.1:8080/api.example.com/path` | `https://api.example.com/path` |
| `http://127.0.0.1:8080/path`, with `--capture-from https://api.example.com` | `https://api.example.com/path` |

While capturing, the flow's mocks are left out and `mock`, `mockState` and `network` do nothing, so
a flow that has them can be captured again. `waitForRequest` works. What was captured is also kept
if the flow fails or is interrupted.

In the folder:

| | |
|---|---|
| `mappings/capture.json`, `__files/` | The mocks: give the folder to the flow's `mock` |
| `capture.txt` | What to do next, as it was printed |

How it becomes mocks:

- A call always answered the same is always answered so.
- Otherwise there are states, of a scenario called `capture`: `Started`, `1`, `2`... A call that
  isn't a GET moves to the next, if a later answer changed after it. Of several it may be, the one
  made the fewest times is taken.
- If an answer changed **by itself**, the flow has to say when: `capture.txt` has the `mockState` to
  add, and before which step.
- A GET is told apart by its whole address. Other calls by address and order, not by body.

Left out, as far as it can be recognised: the values of `-e`; emails and JWTs anywhere; and what is
under a name such as `access_token`, `password`, `email`, `phone` or `firstName`, also in the address.
In an address, anything goes in its place. What the app sends isn't kept, nor headers other
than the answer's `Content-Type`. **Names that aren't in that list are kept, so read the files before
committing them.** A failure's screenshot and device log are not touched.

---

## Running several

### Suites

A file whose name ends in `.rayo.yaml` lists the flows that run together. It is the place to say
what each is for. Paths are relative to it.

```yaml
name: Rides
app:
  id: com.example.app
  apk: ../app/build/outputs/apk/debug/app-debug.apk
output: .results
results: .results/results.txt
flows:
  # Books a bike and rides it.
  - tests/book/flow.yaml
  # The same without booking.
  - tests/scan/flow.yaml
```

<!-- generated: suite -->
| Key | Takes | |
|---|---|---|
| `name` | text | Shown in the results (default: the file) |
| `flows` | a list | Files, run in this order; or mappings with file and retries, see [below](#a-flow-of-a-suite) (required) |
| `retries` | a number from 0 | How many times a flow that fails is tried again, see [Retries](#retries) (default: 1) |
| `app` | a mapping | Installed before the flows, see [below](#the-app-of-a-suite) |
| `device` | a mapping | How it is while the flows run, see [Device setup](#device-setup) |
| `output` | folder | As `--output`, which wins (default: `.rayo` where rayo runs) |
| `results` | a file or a list | As `--results`, which wins over all of them: `.txt`, `.json` or `.xml` (JUnit) |
| `record` | `true` or `false` | A video of each flow that doesn't say, see [Recording a whole flow](#recording-a-whole-flow) (default: `false`) |
| `recordFolder` | folder | Where the videos go; needs `record: true` (default: each flow's, in `output`) |
<!-- end -->

#### A flow of a suite

<!-- generated: suite flow -->
| Key | Takes | |
|---|---|---|
| `file` | file | The flow (required) |
| `retries` | a number from 0 | How many times it is tried again if it fails (default: the suite's) |
<!-- end -->

#### The app of a suite

<!-- generated: suite app -->
| Key | Takes | |
|---|---|---|
| `id` | package | The app's (required) |
| `apk` | file | What to install, if the device has another. rayo doesn't build it (required) |
<!-- end -->

#### What goes where

Mocks, shared steps and permissions belong in each flow, so that it can be read and run alone. The
device is chosen as always: use `--emulator` if the flows must never run on a real one.

`rayo check` takes suites too. When a folder is given, its suites aren't run.

### Device setup

```yaml
device:
  size: 1080x2400
  dpi: 420
  fontScale: 1.3
  darkMode: true
```

<!-- generated: device -->
| Key | Takes | |
|---|---|---|
| `size` | pixels, such as `1080x2400` | Of the screen |
| `dpi` | 72 to 1000 | Its density. With `size`, it says how many dp there are: 1080 pixels at 420 dpi are 411 dp |
| `fontScale` | 0.5 to 3 | Of the text; 1 is normal |
| `darkMode` | `true` or `false` | The dark theme |
<!-- end -->

What isn't said stays as it is. It is only for a suite: all its flows run on the same device, and
one flow alone runs on the device as it is. To run the same flows on two, write two suites.

It is put back at the end, also after Ctrl-C or if rayo is killed, as the device itself does it. If
the device is restarted meanwhile, use `wm size reset` and `wm density reset`. It works on real
devices too. The size doesn't change the hardware: a notch stays where it is.

### Clean state

After each flow, whether it passed or not, rayo undoes what it changed: the mock and `network`, the
`camera`, a recording, the location and airplane mode. If it can't, the flow has a `warning`, as the next may be
affected. The app's data and permissions stay: begin with `launchApp: { clearState: true }`. What a
`shell` or `http` step changed stays too.

`--shuffle` finds flows that depend on others. The order is printed as `--shuffle=<seed>`, which
repeats it.

### Retries

A flow that fails can be run again, whole, after the [clean state](#clean-state). The suite says how
many times, for all its flows and for each:

```yaml
retries: 1                          # all, if they don't say
flows:
  - tests/book/flow.yaml            # 1
  - file: tests/scan/flow.yaml      # 3
    retries: 3
  - { file: tests/pay/flow.yaml, retries: 0 }   # never
```

`--retries <n>` wins over both. **Without any of them it is 1**, also for flows without a suite;
`--retries 0` turns it off. With `-c` and no suite it is 0. It is retried at once, before the next flow. `NOT_RUN` and `INTERRUPTED` aren't, nor when capturing.

If it passes at last it is a `PASS`, as any other, with a note of where each try failed, its
[cause](#causes) and the folder with the rest. Each has its own video too.

```
[2/3] PASS scan  31.0 s  (failed once, then passed)
  try 1: tests/scan/flow.yaml:12: tapOn scan: not on screen: scan  (test)  .results/scan
[3/3] FAIL pay  27.2 s  (failed 3 times)
  try: 1
  ...

TOTAL 3: 2 passed, 1 failed, 2 retried  78.1 s
```

| | |
|---|---|
| `retried` | Flows tried more than once, whether they passed or not |
| JSON | `retried` of the run; of a flow, `tries`, `timesFailed`, `retries` allowed, and a failure for each failed try |
| JUnit | One that passed at last passes, with its failed tries in `system-out` |
| `--failures-only` | Keeps those that failed at first |

To run only some steps again, use [retry](#retry).

### Causes

Not every failure is the test's. Each has a `cause`:

| Cause | When |
|---|---|
| `app` | The app crashed or hung during the flow, or isn't installed |
| `device` | It doesn't answer, is starting, restarted, its interface hung, or the connection was lost |
| `test` | Anything else. It is only `app` or `device` if there is proof of it, so something slow is `test` |

The run stops, so as not to waste time on flows that would fail the same, when:

| | |
|---|---|
| The device can't be used: it doesn't answer, is starting, or Android isn't running | At once, always |
| 2 failures in a row are `device` | Unless `--keep-going` |
| 3 different flows in a row fail with `app`, and the same crash | Unless `--keep-going` |

For `device`, each try counts. A failure that is the test's, or a pass, starts the count again. The rest are `NOT_RUN`, and the results say why the run stopped:

```
[3/9] FAIL pay  4.1 s
  why: not on screen: home
  cause: app, the app crashed: java.lang.IllegalStateException: oops
  ...
[4/9] NOT_RUN scan  (app: 3 flows in a row, the app crashed: ...)

STOPPED app: 3 flows in a row, the app crashed: java.lang.IllegalStateException: oops
TOTAL 9: 0 passed, 3 failed, 6 not run, of those failed 3 app  14.0 s
```

JSON has `stopped` for the run, `cause` for a flow that failed, and `cause` and `because` for each failure.

### Outcomes

Every flow asked for has one; none is left out of the count.

| | | Exit code |
|---|---|---|
| `PASS` | Also if it failed first, and passed when [retried](#retries) | 0 |
| `FAIL` | | 1 |
| `INTERRUPTED` | Ctrl-C while it ran | 130 |
| `NOT_RUN` | With the reason: interrupted, `--fail-fast`, or the run [stopped](#causes) | 1 |

If the connection is lost, the flow fails and the next goes on with a new one. `--rerun-failed` runs what is in `<output>/failed.json`: what
failed, was interrupted or didn't run, until it passes.

### Results

With several flows, one line each, and for a failure all there is to know. A single flow, or `-v`,
shows every step.

```
[1/3] PASS book  18.2 s
[2/3] FAIL scan  9.1 s
  why: not on screen: finished_close
  step: tapOn finished_close
  at: shared/flows/dock.yaml:8
  from: tests/scan/flow.yaml:23 runFlow ../../shared/flows/dock.yaml
  before:
    extendedWaitUntil finished
  calls:
    12:00:01.250 GET /trips -> 200
  screen:
    ...
  saved: .results/scan
[3/3] NOT_RUN long  (--fail-fast, after scan)

PASS book  18.2 s
FAIL scan  9.1 s
NOT_RUN long  (--fail-fast, after scan)

TOTAL 3: 1 passed, 1 failed, 1 not run  27.4 s
```

The list at the end is only there if something didn't pass. Under a failure:

| Key | |
|---|---|
| `try` | Which, if there were several |
| `why` | The error |
| `cause` | `test`, `app` or `device`, see [Causes](#causes) |
| `step`, `at` | The step that failed, and where it is written |
| `from` | The step that ran that flow, and so on outwards |
| `crash` | The app crashed or hung during the flow: what the device log says |
| `before` | The last 3 steps that passed |
| `calls` | The last 5 to the [mock](#mock) |
| `screen` | The elements on it, at most 40 lines |
| `saved` | The folder with the [rest](#failures) |
| `recording` | A video; of flows that passed, only in the file |
| `warning` | Something that may explain this, or the next flow |

`--results <file>` writes it, as the file's name says. It may be repeated, for several formats at once:

```
rayo test --results out.txt --results junit.xml all.rayo.yaml
```

| File | |
|---|---|
| `.txt` | As above for every flow, with `run`, `device` and `began` at the top and each flow's `file` |
| `.json` | For a program |
| `.xml` | JUnit, for a CI, see [Report](#report) |

`--failures-only` leaves out the flows that passed, but counts them; not from JUnit.

In a `.txt`, a line that begins with an outcome is a flow: `grep -E '^(FAIL|NOT_RUN)' results.txt`. What
follows is about it: `  key: value`, or `  key:` and lines with four spaces. `TOTAL` is the last line.

In a `.json`, every key is always there: what there is none of is `""`, `0` or `[]`.

```json
{
  "version": 1,
  "run": "Rides",
  "device": "emulator-5554",
  "began": "2026-11-22T21:11:18Z",
  "seconds": 27.402,
  "order": "",
  "stopped": "",
  "ok": false,
  "total": 3, "passed": 1, "failed": 1, "interrupted": 0, "notRun": 1, "retried": 0,
  "flows": [
    {
      "name": "scan",
      "file": "tests/scan/flow.yaml",
      "status": "FAIL",
      "cause": "test",
      "seconds": 9.114,
      "tries": 1,
      "timesFailed": 1,
      "retries": 0,
      "notRunReason": "",
      "warnings": [],
      "recordings": [],
      "failures": [
        {
          "try": 1,
          "why": "not on screen: finished_close",
          "cause": "test",
          "because": "",
          "step": "tapOn finished_close",
          "file": "shared/flows/dock.yaml",
          "line": 8,
          "from": [{ "step": "runFlow ../../shared/flows/dock.yaml", "file": "tests/scan/flow.yaml", "line": 23 }],
          "before": ["extendedWaitUntil finished"],
          "calls": [{ "at": "2026-11-22T21:11:40.250Z", "method": "GET", "uri": "/trips", "outcome": "200", "mocked": true, "body": "" }],
          "screen": ["..."],
          "crash": "",
          "saved": ".results/scan"
        }
      ]
    }
  ]
}
```

The run:

<!-- generated: results -->
| Key | | |
|---|---|---|
| `version` | number | Changes if the meaning of a key does |
| `run` | text | The suite's name, or what was run |
| `device` | text | Its serial |
| `began` | text | In UTC |
| `seconds` | number | All of it |
| `order` | text | `--shuffle=<seed>`, if shuffled |
| `stopped` | text | Why, if it stopped before the end, see [Causes](#causes) |
| `ok` | true or false | Nothing failed, was interrupted or didn't run: as the exit code |
| `total` | number | Flows |
| `passed` | number | Also those that failed first |
| `failed` | number |  |
| `interrupted` | number |  |
| `notRun` | number |  |
| `retried` | number | Flows tried more than once, whether they passed or not |
| `flows` | list | See below |
<!-- end -->

Each of `flows`:

<!-- generated: results flow -->
| Key | | |
|---|---|---|
| `name` | text | The flow's |
| `file` | text | The flow's |
| `status` | text | One of the [outcomes](#outcomes) |
| `cause` | text | Of a flow that failed, or empty, see [Causes](#causes) |
| `seconds` | number | All its tries |
| `tries` | number | Those it took |
| `timesFailed` | number | Of those |
| `retries` | number | Those allowed |
| `notRunReason` | text | If it is `NOT_RUN` |
| `warnings` | list | What may explain this, or the next flow |
| `recordings` | list | Videos |
| `failures` | list | One for each try that failed, also of a flow that passed at last |
<!-- end -->

Each of `failures`:

<!-- generated: results failure -->
| Key | | |
|---|---|---|
| `try` | number | From 1 |
| `why` | text | The error |
| `cause` | text | `test`, `app` or `device`, see [Causes](#causes) |
| `because` | text | What shows that it isn't the test |
| `step` | text | The step that failed |
| `file` | text | Where it is written |
| `line` | number |  |
| `from` | list | The step that ran that flow, and so on outwards |
| `before` | list | The last 3 steps that passed |
| `calls` | list | The last 5 to the mock |
| `screen` | list | The elements on it |
| `crash` | text | The app crashed or hung during the flow: what the device log says |
| `saved` | text | The folder with the rest, see [Failures](#failures) |
<!-- end -->

```
jq -r '.flows[] | select(.status == "FAIL") | "\(.name): \(.failures[-1].why)"' results.json
(Get-Content results.json -Raw | ConvertFrom-Json).flows | Where-Object status -eq FAIL
```

---

## Live sessions

Write a test by running it, a command at a time. Good for other programs too: they see what each
step did before choosing the next.

```
$ rayo live --emulator --app com.example.app --mock ../mock tests/new/flow.yaml
live: type commands, or send them with rayo do; .help
- launchApp: { clearState: true }
  ✓ launchApp clearState  812 ms
- tapOn: { id: nothing }
  ✗ tapOn nothing  15012 ms
error: tests/new/flow.yaml:1: tapOn nothing: not on screen: nothing
screen:
  ...
- tapOn: { id: login }
  ✓ tapOn login  95 ms
.save
saved 2 steps in tests/new/flow.yaml
```

If the flow is there already, its steps run first (`--no-run` to skip them) and `.save` adds to its
end, leaving the rest as it is. A new one needs `--app`, and `--mock` takes paths relative to it.
The mock, the variables and `output` last for the whole session. When it ends, all is
[put back](#clean-state).

### Live commands

| | |
|---|---|
| Any command, or a list of them | Runs them up to the first that fails. Those that pass are kept |
| `.screen` | What is on screen, and how to refer to it |
| `.steps` | What is kept and not saved yet |
| `.undo` | Forgets the last one. **The device stays as it is** |
| `.save` | Adds them to the file |
| `.help`, `.quit` | `.quit` doesn't save, but prints what is lost |

Typed, a command is a line; one that ends in `:` goes on until an empty line. The leading `- ` may
be left out.

### From other programs

`rayo do` sends commands to the live session, from the same folder or with the same `--output`.
Each call answers when the steps have run; its exit code is 1 if one failed.

```
rayo live --serve --emulator tests/new/flow.yaml &     # until .quit, or an hour unused
rayo do .screen
rayo do --screen '- tapOn: { id: login }'
rayo do --json - < steps.yaml
rayo do .save
rayo do .quit
```

| Option | |
|---|---|
| `--screen` | Also what is on screen afterwards. After a failure it always comes |
| `--json` | `ok`, `output`, `error`, `failure` (as in the [results](#results), or `null`), `warnings`, `calls` to the mock meanwhile, `screen`, `unsaved` steps |
| `-` | Commands from standard input |

Calls the app makes without a mock are warnings here: the session goes on. A run of the saved
flow would fail.

Only this computer can use a session, with a token kept in `<output>/live.json`.

---

## Features

### Schemas

For editors to complete and check what is written, and for programs to know what is valid. At the
top of a file, for those that understand it (such as VS Code with the YAML extension):

```yaml
# yaml-language-server: $schema=../../rayo/schema/flow.json
```

They know the keys and values. What depends on other files, or on several keys at once, only
`rayo check` tells.

### Variables

Used as `${NAME}`. The first that has it wins:

1. `env` of the `runFlow` or `runScript` running now
2. `-e NAME=value`
3. What is set where the file is included
4. `env` in the file's header: a default, for what nothing above sets

Header values may be expressions, which see the variables above; one not set is `undefined`:

```yaml
env:
  CITY: ${ CITY || "Springfield" }
```

### Tags

🧪 `--include-tags smoke,login` runs flows with any of them; `--exclude-tags slow` leaves flows out.

### Continuous mode

🧪 `rayo test -c` runs again when a flow, or a file it includes, is saved.

### Failures

Saved in `<output>/<flow>/`, or `<flow>_2` for a second one with that name; the tries after the
first in `<flow>_retry1` and so on. Those of an earlier run are removed when the flow runs again.

| File | |
|---|---|
| `failure.jpg` | The screen, small: 480 pixels on the short side |
| `screen.txt` | The elements on it, as `rayo hierarchy` shows them |
| `logcat.txt` | The end of the device log |
| `requests.txt` | Every call the app made to the [mock](#mock), with its body; `requests.json`, a list like `calls`, if the results are also written to a `.json`. The last five are printed |

| Exit code | |
|---|---|
| 0 | Everything passed |
| 1 | A flow failed or didn't run, or an error |
| 2 | No command given |
| 130 | Interrupted with Ctrl-C; what was set up on the device is undone |

### Timings

`rayo test --timings` shows where a run is slow or stuck, without the internals of `--profile`:

- every step with its time, and `◂` with the total of each `runFlow`, `repeat` and `retry`
- `… still on <step> after 10 s` every 5 s while a step runs, so a hang shows where it is
- after each flow, its 5 slowest steps; at the end, the 10 slowest flows

### Profile

`--profile` adds to each step where its time went, in milliseconds:

| | |
|---|---|
| `adb+host` | The computer and adb |
| `read` | Reading the screen; long when the app's main thread is busy |
| `cache` | Looking in what Android remembers of the screen |
| `check` | Reading one element again, to confirm it |
| `wait` | Waiting for the screen to change |
| `still` | Waiting to see that an element doesn't move |
| `inject` | Touches and keys, until the app takes them |
| `shell` | Commands on the device |

### Report

JUnit: `--results report.xml`. What didn't run is `skipped`, what was interrupted is an `error`,
and a flow that failed first has its failed tries in `system-out`.

### Environment

| Variable | |
|---|---|
| `ANDROID_SERIAL` | The device, without `--device` |
| `ANDROID_HOME`, `ANDROID_SDK_ROOT` | Where to find adb if it isn't in the `PATH`; the usual places are tried too |
| `ANDROID_ADB_SERVER_PORT` | 5037 by default |

---

## Default times

| | |
|---|---|
| Finding an element, `extendedWaitUntil`, `launchApp`, `waitForRequest` | 15 s |
| `assertNotVisible` | 5 s |
| `scrollUntilVisible` | 20 s |
| With `optional: true` | 3 s, except the two above |
| `when` and `while` | 0.7 s each for `visible` and `notVisible`, or their `timeout` |

---

## Limitations

- Not supported: `addMedia`,
  `setOrientation`, sharding across devices, a
  `config.yaml` (see [suites](#suites)), and selectors by `css`, `width`, `height` or `traits`. An unknown command is an
  error, never ignored.
- `setLocation` and `travel` need Android 12 or newer.
- WebViews show only what they expose to accessibility.
- What a dialog, sheet or other window covers is not on screen: only what is on top can be found
  or tapped. Close the dialog first, for example with a `when` guard.
- Only one tool can automate a device at a time: not while another runs.
- What is marked 🧪 is **written but not tested on a
  device yet**, and so are relative selectors and tags.
