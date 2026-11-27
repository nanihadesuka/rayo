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
| [Device](#device) | [setLocation](#setlocation) · [travel](#travel) · [setAirplaneMode](#setairplanemode) · [setOrientation](#setorientation) · [takeScreenshot](#takescreenshot) · [startRecording](#startrecording-stoprecording) · [stopRecording](#startrecording-stoprecording) · [camera](#camera) · [shell](#shell) |
| [Server](#server) | [http](#http) |
| [Mock](#mock) | [Files](#mock-files) · [mock](#mock-step) · [mockState](#mockstate) · [network](#network) · [waitForRequest](#waitforrequest) · [assertRequest](#assertrequest) · [Capture](#capture) |
| [Running several](#running-several) | [Suites](#suites) · [Clean state](#clean-state) · [Retries](#retries) · [Blame](#blame) · [Outcomes](#outcomes) · [Results](#results) · [For a program](#for-a-program) · [Exit codes](#exit-codes) · [Errors](#errors) · [Events](#events) |
| [Live sessions](#live-sessions) | [Commands](#live-commands) · [From other programs](#from-other-programs) |
| [Features](#features) | [Schemas](#schemas) · [Variables](#variables) · [Tags](#tags) · [Continuous mode](#continuous-mode) · [Resume](#resume) · [Failures](#failures) · [Timings](#timings) · [Profile](#profile) · [Report](#report) · [Environment](#environment) |
| [Default times](#default-times) · [Limitations](#limitations) | |

---

## Options

As `rayo help` prints them. An option of another command is an error, never ignored.

```
rayo - runs flows on Android, fast

New here, or driving rayo from a program? Start with: rayo help agents

  rayo test [options] <flow, folder or suite>...   run flows; a suite is a *.suite.yaml
  rayo check <flow, folder or suite>...            check flows without a device, and say which would run
  rayo explain [<run folder, .zip or suite>] [<flow>]  why the flows of the last run failed, in a few lines
  rayo status [options] <flow, folder or suite>... whether each flow passed the last time it ran, in any run
  rayo live [options] <flow>                       write a flow as it runs, command by command
  rayo live [options] <suite> <flow>               the same, with the suite's env, output and CA
  rayo do [options] <commands>                     send commands to the live session; rayo do .help
  rayo hierarchy [options]                         what is on screen, and how to refer to it
  rayo devices                                     connected devices
  rayo mocks apply [options] <folder>              take the changes of rayo test --capture-update into the mocks
  rayo mocks import [options] <file.har> <folder>  write the calls of a HAR file, such as mitmproxy's, as mocks
  rayo mocks check <folder>                        mappings that never answer, and bodies no mapping uses
  rayo ca new [<folder>]                           a CA for the project, in .rayo/ca, to commit with the flows
  rayo ca export <file>                            the certificate the proxy signs with, for the app's debug build
  rayo help <command>                              what a command takes, of a flow or of rayo; or <command> --help
  rayo help <topic>                                flow, suite, json, all (everything on one page), or a topic below
  rayo help search <word>...                       the lines of all the help that have the words
  rayo schema <name>                               a JSON Schema: flow, suite, results, or of --format json
  rayo version

Topics, rayo help <topic>
  agents                     start here: the fast, cheap way to use rayo
  write-a-flow               write a flow
  debug-a-flow               debug a failing flow
  resume                     go on from where a flow failed, after a fix, without running it all again
  hold                       fix failing flows as they fail: hold the run at a failure, fix the files, run again from there, go on
  record-mocks               record mocks from the real backend
  connect-mocks              connect the app to the mocks: interceptor or proxy
  run-in-ci                  run in a CI
  recipes                    short answers: video, logcat, reset, log in, deeplink, what a run leaves
  limits                     limitations and gotchas
  exit-codes                 exit codes

Options of test, live and hierarchy
  --device <serial>          when several are connected; else ANDROID_SERIAL, else the only one
  --emulator                 never a real device, even if it is the only one

Options of test and check: which flows
  --changed-only             only the flows of which a file changed: their own, one they run, or a mock; as --since HEAD
  --since <git ref>          only the flows of which a file changed since then, also not committed

Options of hierarchy and do
  --find <text>              of the screen, only the elements most like it, instead of all

Options of test, live, do, explain and status
  --output <folder>          where failures and screenshots go (default .rayo) (suite: output.folder)

Options of test and live
  --timeout-scale <factor>   multiply the default waits, not a step's own timeout, from 0.1 to 10 (default 1) (suite: timeoutScale)
  --mock-port <port>         where the app finds the mock on the device (default 8080)
  --animations               leave the device's animations on
  --profile                  where the time of each step went

Options of test and check: which flows of them
  --only <a,b>               only the flows with these names
  --include-tags <a,b>       only flows with one of these tags
  --exclude-tags <a,b>       not flows with one of these tags
  --rerun-failed             only those that haven't passed since they last failed
  --min-flows <n>            fail (exit 2) if fewer flows than this are chosen, as a typo in a tag or a name would; of all the parts with --shard, and not with --rerun-failed or --resume (default: the suite's minFlows, else none) (suite: minFlows)
  --shard <i/n>              only part i of n, for n machines that run the same flows: each flow is in one part, by its path, whatever the order; it writes to .rayo/shard-i-of-n unless --output or the suite says

Options of test, check and live
  -e, --env <KEY=VALUE>      a variable for the flows; over the suite's env, which is over a flow's own (suite: env)

Options of test and check: as the suite says, for one run
  --retries <n>              try again what fails, every flow, also one the suite gives its own retries (default 1, or the suite's) (suite: retries)

Options of test: how they run
  --fail-fast                stop at the first failure
  --flaky-fails-build        exit 4 if every flow passed but some only on a retry (default: exit 0) (suite: flakyFailsBuild)
  --keep-going               don't stop when the device or the app fails over and over
  --shuffle[=seed]           in a random order, or in that of an earlier run
  --screens-before <n>       keep the screen after each of the last n steps that passed, and save them with a failure as screens-before.txt: how the app got there; each costs a read of the screen (default 0) (suite: output.screensBefore)
  -c, --continuous           run again whenever a file changes
  --on-fail <what>           go-on (default): retry it as allowed, then the next flow; hold: keep the flow where it failed, the app as it is, until rayo do says how to go on, so that it is fixed in its files and run again from there; see rayo help hold
  --hold-timeout <duration>  with --on-fail hold, leave a flow failed if no command comes for this long, such as 30m (default 5m)

Options of test: from where, after a fix; see rayo help resume
  --resume                   each flow that failed last time, from the step that failed, with the app as it is and the mocks' scenarios as they were then
  --from <step>              begin one flow at a step: file:line as rayo explain's at: says, a line of its own file, or a text of one step such as pickDate; the app is as it is, onFlowStart doesn't run
  --until <step>             stop one flow after a step, given as --from takes it, leaving the app there; onFlowComplete doesn't run
  --mock-state <a=s,b=t>     with --resume or --from, the scenarios' states to begin with, over those of the last run

Options of test and check: what is written, as the suite's output
  --video                    a video of each flow whose header doesn't say record, as output: { video: true } (suite: output.video)
  --keep <when>              the files of every try (all, default), only of the tries that failed, or none (suite: output.keep)
  --clean <when>             remove the files of earlier runs: none (default), keeps them; auto, a whole run begins with none, a partial one removes a flow's as it starts; all, every run begins with none (suite: output.clean)
  --artifacts <a,b>          the kinds of file written: video, screenshot, logcat, screen, requests (default all) (suite: output.artifacts)
  --name-template <template> how the files are named in the output folder (default {flow}_{attempt}_{kind}.{ext}) (suite: output.nameTemplate)

Options of test: what comes out
  --ignore-lock              run even if another rayo seems to write to the output folder, as when its running.json was left by one that is gone
  --detail <how much>        printed: brief, the verdict of a failure; normal (default), also why, where, the nearest elements and the calls, and every step of a single flow; full, also the screen, every step, and whole files (suite: detail)
  --failure-window <seconds> a failure's log and calls begin this long before the failing step (default 10) (suite: failureWindow)
  --results <file>           write the results: .txt, .json, .xml (JUnit) or .md; may be repeated (suite: results)
  --results-template <file>  write the results with that extension as this file says, with {suite}, {passed}, {failed}, {flaky} and the like, and {flows}...{end} (suite: resultsTemplate)
  --show <a,b>               the flows the results list: failed, flaky, slow or all (default all) (suite: show)
  --slowest <n>              how many flows the slow list has (default 5) (suite: slowest)
  --timings                  while it runs: each step's time, each runFlow's total, the slowest steps and flows, and a note while a step hangs

Options of test and live: mocks from the real backend
  --capture <folder>         call the real backend, and write what it answers as mocks; in live, on .quit, without the calls of steps that failed
  --capture-update <folder>  capture into folder/.update, and say what changed from the mocks in folder; rayo mocks apply takes it. Given a suite instead, the folder is the flow's mock
  --check                    with --capture-update, fail if the mocks differ from the backend, and change nothing
  --capture-append           go on with the capture already in the folder, as if in one go
  --redact-value <name=value> what a private field of a body becomes, such as phone=+4915100000000 (default one of the same form: a date stays a date); again for more (suite: capture.redactValue)

Options of test, live and mocks: how a capture or an import becomes mocks
  --capture-from <url>       the backend, if calls don't begin with /host/ (suite: capture.from)
  --graphql <match>          tell GraphQL calls apart by operation+variables (default), by operation, or off, as any call (suite: capture.graphql)
  --graphql-path <regex>     paths of GraphQL calls, besides those whose body has query or operationName (default .*/graphql) (suite: capture.graphqlPath)
  --rest <match>             match other calls by their body too (body, default), or by method and address alone (off) (suite: capture.rest)
  --ignore-params <a,b>      names in the query not to match, such as session or depart_at (suite: capture.ignoreParams)
  --ignore-body <a,b>        names, or paths such as trip.departAt, in JSON bodies and GraphQL variables, and names in forms, not to match (suite: capture.ignoreBody)
  --timestamps <match>       match dates and times in the query and bodies by their form (format, default), or exactly (suite: capture.timestamps)
  --capture-headers <a,b>    headers of the request to match too, such as Accept-Language (suite: capture.captureHeaders)
  --ignore-host              match calls whose path begins with a host, whatever the host (suite: capture.ignoreHost)
  --ignore-path-ids          match UUIDs and ULIDs in paths by their form, as they are new on each run (suite: capture.ignorePathIds)
  --stateless <a,b>          paths such as api.shop.com/quotes/** whose calls only read, whatever the method: they don't change the state (* any text but /, ** any) (suite: capture.stateless)

Options of test, live and ca: how calls reach the mock
  --network <way>            interceptor, the app sends them, with code in its debug build (default, or the flow's); proxy, the device's proxy does, no code; see rayo help connect-mocks (suite: network)
  --ignore-unmocked <a,b>    calls without a mock that don't fail the flow, answered 404: graphql:Operation, host/path, host or /path (* any text but /, ** any) (suite: unmocked.ignore)
  --ca <folder>              with a proxy, the CA it signs with, its ca.pem and ca-key.pem (default .rayo/ca next to the suite or above it, else ~/.rayo); see rayo ca (suite: ca)
  --hosts <a,b>              with a proxy, the hosts to open, and their subdomains (default: capturing, all but Google's, Firebase's, analytics and crash reports; else those of the mocks) (suite: capture.hosts)

Options of live
  --app <package>            of a flow that isn't there yet
  --mock <a,b>               its mocks, relative to it
  --no-run                   don't begin by running the steps it has
  --serve                    only take commands from rayo do, until .quit
  --idle-timeout <duration>  end the session if no command comes for this long, such as 1h; steps not saved are printed (default 5m)

Options of explain
  --latest                   only the flows of the newest run in failed.json, not those still there from earlier ones

Options of mocks
  --only <a,b>               the changes whose name has one of these, such as graphql:Wallet or GET /profile (default all)
  --prune                    also take out the mocks that weren't called

Options of do
  --screen[=changes]         also what is on screen afterwards; --screen=changes, only the lines that appeared (+) or went (-) since the screen the session last gave
  --ignore-screen <regex>    with --screen, leave out the lines that match, such as a clock's; says how many
  --timeout <duration>       how long to wait for the answer, such as 30s (default as long as it takes); the session goes on
  --discard                  with .quit, end even if steps were not saved

Options of test, check, explain, status, devices, hierarchy, do and mocks
  --format <how>             human (default); json, for a program, see rayo schema; ndjson, the same on one line, and for test an event a line as it runs (suite: format)
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
| `mock` | a path or a list | Who answers as the backend, see [Mock](#mock); `${...}` too, where an empty path loads nothing |
| `network` | `interceptor` or `proxy` | How the app's calls reach the mock, see [Proxy](#proxy); `${...}` too, such as `${NETWORK}` from `-e`, where empty is the default (default: `interceptor`, or `--network`) |
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
`READ_CONTACTS`. Only `all` has been tested. On an Android older than the permission, `notifications` and
`medialibrary` before 13, `bluetooth` before 12, it is skipped with a note such as `notifications: not
asked before Android 13, ignored`.

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
| `fling` | true/false | Lift the finger still moving, so the list goes on by itself (true); false stops it where the finger does |
<!-- end -->

### scroll

🧪 Scrolls down once.

### scrollUntilVisible

🧪 Scrolls until the element is on screen. If the screen stops changing, the end of the list, it fails
then, saying so, instead of waiting out the timeout.

<!-- generated: keys scrollUntilVisible -->
| Key | Takes | |
|---|---|---|
| `element` | selector | What to look for |
| `direction` | UP/DOWN/LEFT/RIGHT | Where the finger or the search goes (DOWN when searching) |
| `centerElement` | true/false | Go on until it is in the middle of the list, or the list ends (false) |
| `speed` | number | 0 to 100, as Maestro: each scroll takes (100 - speed) * 10 ms (40) |
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

`output` names where else to keep it, so that the next copy doesn't replace it: `output: orderId`
keeps it in `${output.orderId}` too, for the rest of the flow and the flows it runs.

```yaml
- copyTextFrom: { id: order_number, output: orderId }
- copyTextFrom: { id: total, output: total }
- inputText: ${output.orderId}
- assertTrue: ${output.total == '5 €'}
```

<!-- generated: keys copyTextFrom -->
| Key | Takes | |
|---|---|---|
| `output` | name | Also keeps it in output.<this name>, which the next copy doesn't replace |
<!-- end -->

### hideKeyboard

🧪 Closes the keyboard if it is open with one Back, and waits until it has. Does nothing when none is
open. It never sends a second Back, which could close the window under the keyboard, such as a
Custom Tab: if the keyboard is still open at the `timeout`, it fails. To try again, use `back` or
`retry`. A keyboard is open when the input method's window is at least an eighth of the screen: with
a hardware keyboard, the small toolbar the input method may show isn't one, so nothing is sent.

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

🧪 Waits until nothing on screen has changed for `quiet` (1 s). Goes on when the `timeout` is
over, without failing.

A quiet screen isn't a finished transition: while the app waits for the network, nothing moves, so
this returns, and a dialog that comes after may still be missing. It can't know about what hasn't
begun. To wait for something late, wait for it by name, as `extendedWaitUntil`, or a `when` with a
longer `timeout`:

```yaml
- tapOn: Continue
- runFlow:
    when: { visible: Allow notifications, timeout: 5s }
    commands: [ { tapOn: Not now } ]
- assertVisible: Home
```

<!-- generated: keys waitForAnimationToEnd -->
| Key | Takes | |
|---|---|---|
| `quiet` | time | How long nothing may change (1s) |
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
| `while` | conditions | As long as these hold; without times, it fails after 1000 rounds that it still holds |
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

### setOrientation

🧪 `PORTRAIT`, `LANDSCAPE_LEFT`, `UPSIDE_DOWN` or `LANDSCAPE_RIGHT`, as Maestro names them. The screen stops
turning by itself until the end of the flow, which puts both settings back as they were. The quickest
way to see whether the app keeps what was typed and where it was when the activity is made again.

<!-- generated: keys setOrientation -->
| Key | Takes | |
|---|---|---|
| `orientation` | PORTRAIT/LANDSCAPE_LEFT/UPSIDE_DOWN/LANDSCAPE_RIGHT | How the screen is turned. Short form |
<!-- end -->

### takeScreenshot

🧪 Saves `<output>/<flow>_<name>.png`, named as [the other files](#what-is-kept).

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
- startRecording                      # <output>/<flow>.mp4
- startRecording: login               # <output>/<flow>_login.mp4
- startRecording: { path: login }     # the same
- stopRecording
```

#### Recording a whole flow

`record` in its header; or `video` in a [suite's `output`](#what-is-kept), for those of its flows that don't say:

| Where | | Saved as |
|---|---|---|
| Flow header | `record: true` | `<output>/<flow>.mp4` |
| | `record: name`, or an absolute path | `<output>/<flow>_name.mp4`, or that path |
| | `record: false` | Not recorded, whatever the suite says |
| Suite | `output: { video: true }`, or `--video` | `<output>/<flow>.mp4` |

Each try of a flow has its own, such as `<flow>_retry1.mp4`. All are kept, unless
[`keep`](#what-is-kept) says otherwise: `keep: failed` keeps only those of the tries that failed. One recording at a time: with one in
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

An included flow's mocks are added when it runs. `mock` and `network` may use `${...}`, from `-e`
or a suite's `env`, so that one suite runs both mocked and against the real backend, with no flow
around each: a path that comes out empty loads nothing, and an empty `network` is the default.

```yaml
mock: ${MOCKS ? MOCKS + '/' + rayo.file : ''}   # rayo test -e MOCKS=mocks -e NETWORK=proxy all.suite.yaml
network: ${NETWORK || ''}
```

How mocks work, with examples:
[Mocking](MOCKING.md).

### Mock files

A folder with `mappings/` and `__files/`, or one JSON file of mappings, in WireMock's format: a folder
WireMock recorded loads as it is, and WireMock loads rayo's captures. Only the following is supported;
anything else, such as webhooks or `proxyBaseUrl`, is an error rather than a mock that does less.

| Part | |
|---|---|
| Mapping | `id`, `uuid`, `name`, `priority`, `scenarioName`, `requiredScenarioState`, `newScenarioState`, `request`, `response`; `persistent`, `insertionIndex` and `metadata` are taken and do nothing |
| `request` | `method`; one of `url` `urlPattern` `urlPath` `urlPathPattern` `urlPathTemplate` (`/users/{id}`, with `pathParameters`); `queryParameters`, `headers`, `cookies`, `formParameters`, `basicAuthCredentials`, `host`, `bodyPatterns` |
| Matchers | `equalTo` (and `caseInsensitive`), `contains`, `doesNotContain`, `matches`, `doesNotMatch`, `absent`, `anything`, `equalToJson` (and `ignoreArrayOrder`, `ignoreExtraElements`), `equalToXml`, `binaryEqualTo`, `matchesJsonPath`, `and`, `or`, `not`; for a query, header, cookie or form, also `hasExactly` and `includes` |
| `equalToJson` | A JSON value, or a text of one. Keys in any order, numbers by value. Placeholders: `${json-unit.ignore}`, `${json-unit.ignore-element}` (may be absent), `${json-unit.any-string}`, `${json-unit.any-number}`, `${json-unit.any-boolean}`, `${json-unit.regex}pattern` |
| `matchesJsonPath` | An expression, true if it finds something not empty; or `{ "expression": "$.a", "equalTo": "x" }` with any matcher for what it finds. Jayway's JSONPath: `$.a`, `$['a']`, `$[0]`, `$[-1]`, `$[0,2]`, `$[1:3]`, `$.*`, `$..a`, `length()`, and filters anywhere, with `==` `!=` `<` `>` `&&` `=~ /re/` `in` `nin` `size` `empty` `subsetof` `anyof` `noneof` |
| `response` | `status`, `statusMessage`, `headers` (a value, or a list of them), one of `body` `jsonBody` `base64Body` `bodyFileName`, `fixedDelayMilliseconds`, `delayDistribution` (`lognormal`, `uniform`, `fixed`), `chunkedDribbleDelay`, `fault` (`CONNECTION_RESET_BY_PEER`, `EMPTY_RESPONSE`, `MALFORMED_RESPONSE_CHUNK`, `RANDOM_DATA_THEN_CLOSE`), `transformers`, `transformerParameters` |
| In a body | `{{now}}` (with `offset='-8 minutes'`, `format='yyyy-MM-dd'` or `epoch` or `unix`, `timezone='Europe/Madrid'`), `{{request.method}}` `.path` `.url` `.body` `.query.x` `.query.x.[1]` `.headers.x` `.cookies.x` `.pathSegments.[0]`, `{{jsonPath request.body '$.a'}}`, `{{randomValue length=8 type='ALPHANUMERIC'}}` (or `UUID`, `NUMERIC`...), `{{regexExtract request.path '[a-z]+'}}`, `{{regexExtract request.query.x '(.+)' 'name'}}` then `{{name.0}}`, `{{parameters.x}}` |

The lowest `priority` wins (5 by default), then the last one added. Scenarios begin in `Started`.
Regular expressions are Go's and match the whole value, except in `regexExtract`; those WireMock
writes mean the same in both, but Java's lookarounds aren't supported.

A body is filled in as `transformers` says. Without it, as rayo always has: every `{{ }}` is filled in,
and one that isn't known is an error. With `"transformers": ["response-template"]`, as WireMock does:
`{{x}}` is escaped for HTML and `{{{x}}}` isn't. With `"transformers": []`, it is sent as it is. A body
that isn't text, such as an image, is always sent as it is.

A call whose body is compressed (`Content-Encoding: gzip` or `deflate`) is matched by what it holds.
Through the proxy, a call is `/host/path`; a mapping without the host, as WireMock's are, answers it
too when no mapping names that host.

When no mapping answers, the flow fails and says which came nearest, and why, as WireMock's near
misses do: `POST /availability -> not mocked; nearest POST /availability {"category":"ondemand"}: body:
$.category: "prebooked", expected "ondemand"`.

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

### assertRequest

<!-- generated: keys assertRequest -->
| Key | Takes | |
|---|---|---|
| `method` | text | Such as POST (GET) |
| `path` | pattern | Only these calls, a regular expression of the whole path |
| `body` | text | A text in the body |
| `times` | number | Exactly so many calls by now; 0 for none |
<!-- end -->

Checks, without waiting, that the app has called `path` exactly `times` times so far in this flow:
`times: 0` for a call it must never make. `method` and `body` narrow it as in waitForRequest; calls
that a waitForRequest matched count too. For a call that may still be on its way, waitForRequest first.

```yaml
- tapOn: Pay
- waitForRequest: { method: POST, path: ".*/payments" }
- assertRequest: { method: POST, path: ".*/payments", times: 1 }   # not twice
- assertRequest: { path: ".*/analytics", times: 0 }
```

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

A long flow whose calls do something real (an order, a payment) is better captured in a
[live session](#live-sessions), which a failure doesn't end:

```
rayo live --serve --capture mock -e PASSWORD=... flow.yaml &
rayo do .retry          # after fixing the step that failed, in the file
rayo do .rewind 2       # forget the last 2 steps and their calls; move the app back by hand
rayo do .quit           # writes the capture
```

The calls of a step that fails, and of those rewound, are left out, so what is written is one
capture, as if the flow had run once without them. Without the session, `--capture-append` goes on
with a capture that is there already, in another run: its calls come first and the states go on
from them.

In the folder:

| | |
|---|---|
| `mappings/capture.json`, `__files/` | The mocks: give the folder to the flow's `mock` |
| `capture.txt` | What to do next, as it was printed |
| `capture.calls.json` | The calls, and what the app sent, hidden as in the mocks; and how they were made into mocks. For `--capture-append` and `--capture-update` |

How it becomes mocks, as WireMock's recorder does by default:

- A call is matched by its method, its path, its query and its body. The query is matched with
  `queryParameters`, so the order of its parameters doesn't count. The body as WireMock's
  `requestBodyPattern: auto`: JSON with `equalToJson`, `ignoreArrayOrder` and `ignoreExtraElements`;
  a form with `formParameters`; XML with `equalToXml`; other text with `equalTo`; anything else with
  `binaryEqualTo`. A multipart body, or one over 64 KB, isn't matched. `--rest off` matches by method
  and address alone. GraphQL calls are told apart [their own way](#graphql).
- Calls matched the same are the same call. One always answered the same is always answered so.
- Otherwise there are states, of a scenario called `capture`: `Started`, `1`, `2`... A call that
  isn't a read (a GET, HEAD, OPTIONS or GraphQL query) moves to the next, if a later answer changed
  after it. Of several it may be, the one made the fewest times is taken. So the same call is
  answered as often, and in the order, as during the capture: WireMock's `repeatsAsScenarios`.
- If an answer changed **by itself**, the flow has to say when: `capture.txt` has the `mockState` to
  add, and before which step.
- As `ignoreExtraElements` lets a mapping fit a call with more in it, the mappings that match more
  get a lower `priority`, from 5: a call is answered by the closest. A mapping written by hand without
  a priority, loaded after, wins over all of them.
- What changes from run to run can't be matched as it was. A date and time (ISO 8601) in the query
  or in a JSON body is matched by its form; `--timestamps exact` matches it as it was. `--ignore-params
  depart_at,session` leaves parameters of the query out; `--ignore-body nonce,trip.clientTime` leaves
  names, at any depth, or paths, out of JSON bodies, GraphQL variables and forms; `--ignore-host`
  matches the path of any host (then, through the proxy, `--hosts` says which to open);
  `--ignore-path-ids` matches a UUID or ULID in a path, such as `/executions/<uuid>/status`, by its form.
  `capture.txt` lists what was left out.
- A call that changes nothing, as GET, keeps the state; any other may change it. `--stateless
  'api.shop.com/quotes/**,*/availability'` says which paths only read, whatever the method, such as a
  POST that searches: they answer in any state, as often as the app calls them. `*` is any text but
  `/`, `**` any text, from the host when calls begin with one.
- When calls go to several hosts, `capture.txt` says which of them change the state: one that isn't
  the app's backend, such as analytics, is best left out with `--stateless 'its.host/**'`.
- A read that answers one way when the app sends `Authorization` and another when it doesn't, such
  as `GET /users/me` before and after logging in, is matched by whether the header is there, not by
  a state: `(signed in)` and `(signed out)` in its name.
- `--capture-headers Accept-Language` matches these headers of the request too.
- The headers of the answer are kept, but for those that change by themselves (`Date`, `ETag`, trace
  ids...).
- These settings are kept in `capture.calls.json`: `--capture-append` and `--capture-update` make the
  mocks the same way, unless told otherwise.

`rayo mocks import flows.har mock` makes mocks of a HAR file in the same way, as if captured: one of
mitmproxy (`mitmdump -nr flows --set hardump=flows.har`), Charles, or a browser. With `--capture-from
https://api.example.com`, only the calls to it are taken, without the host in the path; else every
call, with its host, but those to Google, Firebase, analytics and crash reports. The values of `--env`
and the suite's `env` are hidden as in a capture. Calls are taken in the order they started (those
without a time last, then as written), so the same file always makes the same mocks.

#### Updating mocks

When the backend changes, capture again beside the mocks instead of over them:

```sh
rayo test --capture-update mock --capture-from https://api.example.com flow.yaml
```

The new capture goes to `mock/.update/`, and each of its mappings is compared with the mapping of
the same name (and state) in `mock/`, including hand-written ones. What is printed, and kept in
`mock/.update/changes.txt`, is each mapping that is `changed` (the paths that differ in what it
matches, such as `request.urlPath: "/a" -> "/b"`; status; each header by name; delay, fault,
`priority`, `scenarioName` and `newScenarioState`; the paths in the body that differ, such as
`body.data.wallet.price: 5 -> 7`, or for a body that isn't JSON its first line that does, such as
`body:12: ... -> ...`), `new` or `gone`. Past 20 differences of a mapping, `... and N more`. Then
take what you want:

```sh
rayo mocks apply mock                                   # all the changes and new ones
rayo mocks apply --only graphql:Wallet,"GET /profile" mock   # those whose name has one of these
rayo mocks apply --prune mock                           # also take out those gone
```

In CI, `--check` fails the run when anything differs, and changes nothing:

```sh
rayo test --capture-update mock --check --capture-from https://api.example.com flow.yaml
```

A changed mapping is written where it was, with its body in the same file of `__files`, so hand-written
mappings in other files stay where they are. It works the same with the interceptor and the proxy.
Mocks captured before REST bodies and GraphQL variables were matched have other names: the first
update of them shows each as `gone` and `new`; `rayo mocks apply --prune` takes the new ones.

#### Checking mocks

`rayo check` already fails on a mock that doesn't load. `rayo mocks check <folder>` also says what
loads but is never used, and exits 1 if anything is:

- a mapping that never answers, as a later one of the same priority and scenario state matches the
  very same calls: the later one wins, so the earlier is dead, often a leftover of an edit;
- a file of `__files` that no mapping's `bodyFileName` names.

```
$ rayo mocks check mock
mock/mappings/profile.json never answers: mock/mappings/profile-new.json, after it, answers the same calls (GET /profile)
mock/__files/old-profile.json: no mapping uses it
```

#### GraphQL

GraphQL and REST calls are captured together, through the interceptor or the proxy. A call is taken
as GraphQL when it is a JSON `POST` whose body has `query`, `operationName` or
`extensions.persistedQuery.sha256Hash`, or one to a path matching `--graphql-path` (default
`.*/graphql`). Anything else is captured as before.

- Each GraphQL call is told apart by its **operation**, with a `bodyPatterns` entry
  `matchesJsonPath: { expression: $.operationName, equalTo: Geocode }`. Mocks therefore don't depend
  on the order of the calls: a `Wallet` query is never answered with a `Geocode` answer. Without
  `operationName`, the name in the query is matched; with neither, the hash of a persisted query, or
  else the query itself. Only WireMock's matchers are written, so WireMock reads the mocks the same.
- A **query** counts like a GET and a **mutation** like any other call: only mutations move the
  scenario on, and states whose answers don't differ are joined.
- Which operation a document runs is read past comments, strings and fragments, also when it has
  several: the one `operationName` names. That tells a query from a mutation.
- When the same operation name is sent with **different query documents** (other fields, other
  fragments), each document gets its own mappings, matched on the query whatever its spaces and
  commas. When there is only one document per name, only the name is matched, so a small edit to a
  query doesn't break the mocks.
- An automatic persisted query, sent first with its hash alone and then again with the query, gets a
  mapping for each.
- The **variables** are matched too, with `equalToJson` on `$.variables`, but for those under private
  names, those of `--ignore-body`, and dates and times, matched by their form. `--graphql operation`
  matches the operation alone; `--graphql off` takes GraphQL calls as any other.
- A batch, an array of operations, is matched by its whole body, and is a read if all of them are
  queries. A query sent with GET is matched by its address.
- Only `operationName`, the query text and, if asked for, the variables are kept from the request.
  A query with something private written into it is matched by its name only.
- When mocking, a GraphQL call that nothing matches is reported with its operation:
  `POST /graphql (GraphQL operation Unknown)`.

Left out, as far as it can be recognised: the values of `-e`; emails and JWTs anywhere; and what is
under a name such as `access_token`, `password`, `email`, `phone` or `firstName`, also in the address
and in what the app sends. In an address or a body, anything goes in its place. The value of every
cookie, and headers such as `Authorization`, are hidden. In a JSON answer, a hidden value keeps its
form, so that a typed client still reads it: a date or time becomes `2000-01-01T00:00:00Z` in the
same layout, a phone `+12025550123`, a first name `Alex`, a last name `Doe`, digits zeros, a number
`0`; what has no form, such as a token, `REDACTED`. `--redact-value phone=+4915100000000` (again for
more) says what a name becomes. An answer that isn't text is kept as it was;
a body the app sent that isn't text isn't kept if a secret is found in it. **Names that aren't in that
list are kept, so read the files before committing them.** A failure's screenshot and device log are
not touched.

---

## Running several

### Suites

A file whose name ends in `.suite.yaml` lists the flows that run together. It is the place to say
what each is for. Paths are relative to it.

```yaml
name: Rides
app:
  id: com.example.app
  apk: ../app/build/outputs/apk/debug/app-debug.apk
output: .results
results: .results/results.txt   # relative to the suite, as every path in it
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
| `flows` | a list | Files, run in this order; or mappings with file, retries, mock, network, unmocked and capture, see [below](#a-flow-of-a-suite) (required) |
| `retries` | a number from 0 | How many times a flow that fails is tried again, see [Retries](#retries) (default: 1) |
| `app` | a mapping | Installed before the flows, see [below](#the-app-of-a-suite) |
| `device` | a mapping | How it is while the flows run, see [Device setup](#device-setup) |
| `env` | values | Variables for every flow, over a flow's own `env`; `-e` overrides one; they may use `${...}`, such as `${rayo.file}`, see [Variables](#variables) |
| `timeoutScale` | 0.1 to 10 | Multiplies the default waits, see [Default times](#default-times); `--timeout-scale` overrides it (default: 1) |
| `network` | `interceptor` or `proxy` | How the app's calls reach the mock, as a flow's `network`; the flow's own and `--network` override it, see [Mocks of a suite](#mocks-of-a-suite) (default: each flow's, else `interceptor`) |
| `unmocked` | a mapping | Calls without a mock that don't fail the flow, see [Calls not worth mocking](#calls-not-worth-mocking) (default: none: each fails the flow) |
| `capture` | a mapping | How a capture calls the backend and becomes mocks, as the options of the same names, which win, see [Capturing a suite's flow](#capturing-a-suites-flow) |
| `ca` | folder | `--ca` overrides it: the CA the proxy signs with, its ca.pem and ca-key.pem, relative to the suite; `rayo ca new` makes one, see [Mocks of a suite](#mocks-of-a-suite) (default: `.rayo/ca` next to the suite or above it, up to the repository's root; else `~/.rayo`) |
| `output` | a folder, or a mapping | What a run writes, and where: a folder, or folder, keep, clean, video, artifacts and nameTemplate, see [What is kept](#what-is-kept) (default: `.rayo` where rayo runs) |
| `results` | a file or a list | `--results` replaces them all: `.txt`, `.json`, `.xml` (JUnit) or `.md`, see [Results](#results) |
| `detail` | `brief`, `normal`, `full` | `--detail` overrides it: how much is printed, see [Results](#results) (default: `normal`) |
| `failureWindow` | seconds, from 1 | `--failure-window` overrides it: a failure's log and calls begin this long before the failing step, see [Failures](#failures) (default: 10) |
| `resultsTemplate` | file | `--results-template` overrides it: the results with its extension are written as it says, see [A template of your own](#a-template-of-your-own) |
| `show` | a name or a list | `--show` overrides it: the flows the results list, of `failed`, `flaky`, `slow`, `all`, see [Results](#results) (default: `all`) |
| `slowest` | a number from 1 | `--slowest` overrides it: how many flows the slow list has (default: 5) |
| `format` | `human`, `json` or `ndjson` | What is printed; `--format` overrides it, see [For a program](#for-a-program) (default: `human`) |
| `minFlows` | a number from 1 | `--min-flows` overrides it: fewer flows chosen, by tags, names or what changed, is an error (exit `2`), not a green run of nothing (default: none) |
| `flakyFailsBuild` | `true` or `false` | `--flaky-fails-build` too: a run where every flow passed, some only on a retry, exits `4`, see [Flaky flows](#flaky-flows) (default: `false`: it exits `0`) |
<!-- end -->

#### A flow of a suite

<!-- generated: suite flow -->
| Key | Takes | |
|---|---|---|
| `file` | file | The flow (required) |
| `retries` | a number from 0 | How many times it is tried again if it fails (default: the suite's) |
| `mock` | a path or a list | Who answers as the backend, as a flow's `mock`, relative to the suite; the flow's own `mock` wins, see [Mocks of a suite](#mocks-of-a-suite) |
| `network` | `interceptor` or `proxy` | How the app's calls reach the mock, as a flow's `network`; the flow's own and `--network` override it, see [Mocks of a suite](#mocks-of-a-suite) (default: the suite's) |
| `unmocked` | a mapping | Calls without a mock that don't fail the flow, see [Calls not worth mocking](#calls-not-worth-mocking) (default: the suite's; its calls are added to them) |
| `capture` | a mapping | How a capture calls the backend and becomes mocks, as the options of the same names, which win, see [Capturing a suite's flow](#capturing-a-suites-flow) (default: the suite's; what it says wins over it) |
<!-- end -->

#### Mocks of a suite

A suite can say which mocks each flow runs on, and how the app's calls reach them, so that the same
flow files run unchanged against the real backend and on mocks, with no wrapper flows:

```yaml
name: Shop (mocked)
network: proxy                  # for every flow; --network wins
flows:
  - file: tests/checkout.yaml
    mock: mocks/checkout        # a path or a list, relative to the suite
    retries: 2
  - file: tests/login.yaml
    mock: [mocks/common, mocks/login]
  - file: tests/pinned_host.yaml
    mock: mocks/pinned_host
    network: interceptor        # this flow, the other way
```

What wins, first to last: `--network`; the flow's own `mock` and `network`; its entry in the suite;
the suite's `network`. A flow whose header has a `mock` runs on that one only, not on its entry's.
The paths are checked when the suite is read, as a flow's are.

#### Calls not worth mocking

A call that no mock answers fails the flow. Some aren't worth recording: those the app makes on its
own at startup, or one cancelled during the capture. `unmocked` lets them through, answered, in the
suite for every flow or in a flow's entry, which adds its own; `--ignore-unmocked` adds more:

```yaml
unmocked:
  ignore:
    - host: guest-api.example.com
      path: /api/v2/locations/favorites/dismiss
    - graphql: [Geocode, ReverseGeocode]
    - api.example.com/pings/**  # as --ignore-unmocked takes them
  status: 200                   # what they are answered with (default 404)
  body: "{}"
```

```sh
rayo test --ignore-unmocked 'graphql:Geocode,api.example.com/pings/**' staging.suite.yaml
```

Every other call without a mock still fails the flow. Those let through are said at the end of the
flow, as `ignored 2 unmocked calls: ...`, so that nothing is hidden. They change nothing while
capturing, as the real backend answers.

<!-- generated: suite unmocked -->
| Key | Takes | |
|---|---|---|
| `ignore` | a list | Calls that no mock answers but that don't fail the flow: each `graphql:Operation`, `host/path`, `host` or `/path`, as `--ignore-unmocked`, or a mapping with host, path and graphql, see [Calls not worth mocking](#calls-not-worth-mocking) (required) |
| `status` | 100 to 599 | What those calls are answered with (default: 404) |
| `body` | text | Of the answer (default: `not mocked: ` and the call) |
<!-- end -->

An ignored call, when a mapping:

<!-- generated: suite ignored -->
| Key | Takes | |
|---|---|---|
| `host` | host | Of the call, or of its subdomains (default: any) |
| `path` | a path | Of the call, without the host; `*` is any text but `/`, `**` any (default: any) |
| `graphql` | a name or a list | GraphQL operations (default: any call) |
<!-- end -->

#### Capturing a suite's flow

How a capture calls the backend and becomes mocks belongs to the suite, as it describes its backend.
`capture` says it for every flow, and a flow's entry over it; each key is the option of the same
name, which wins over both:

```yaml
network: proxy
capture:
  hosts: [a.example.com, b.example.com]
  ignoreParams: [_pollTs]
  ignoreBody: [nonce, deviceData]
  ignorePathIds: true
  stateless: [a.example.com/graphql]
  redactValue: { phone: "+4915100000000" }
flows:
  - file: flows/checkout.yaml
    mock: mocks/checkout
    capture:                    # what it says wins over the suite's, key by key
      ignoreBody: [nonce, deviceData, cartId]
```

Then re-recording a flow's mock needs no script: `--capture-update` with the suite, instead of a
folder, updates the flow's own mock, the last of its `mock`:

```sh
rayo test --capture-update staging.suite.yaml --only checkout
rayo mocks apply mocks/checkout
```

`capture` is read only while capturing.

<!-- generated: suite capture -->
| Key | Takes | |
|---|---|---|
| `hosts` | a name or a list | As `--hosts` |
| `from` | url | As `--capture-from` |
| `graphql` | `operation+variables`, `operation`, `off` | As `--graphql` |
| `graphqlPath` | regex | As `--graphql-path` |
| `rest` | `body`, `off` | As `--rest` |
| `ignoreParams` | a name or a list | As `--ignore-params` |
| `ignoreBody` | a name or a list | As `--ignore-body` |
| `timestamps` | `format`, `exact` | As `--timestamps` |
| `captureHeaders` | a name or a list | As `--capture-headers` |
| `ignoreHost` | `true` or `false` | As `--ignore-host` |
| `ignorePathIds` | `true` or `false` | As `--ignore-path-ids` |
| `stateless` | a path or a list | As `--stateless` |
| `redactValue` | values | As `--redact-value`: what a private field of a body becomes, such as `phone: "+4915100000000"` |
<!-- end -->

#### The app of a suite

<!-- generated: suite app -->
| Key | Takes | |
|---|---|---|
| `id` | package | The app's (required) |
| `apk` | file | What to install, if the device has another. rayo doesn't build it (required) |
<!-- end -->

#### What goes where

Shared steps and permissions belong in each flow, so that it can be read and run alone; mocks too,
unless the same flow also runs without them, see [Mocks of a suite](#mocks-of-a-suite). The
device is chosen as always: use `--emulator` if the flows must never run on a real one.

`rayo check` takes suites too. A folder is the flows directly in it; a suite runs only when its
file is given. Subfolders are left out, as they hold what flows share: sub-flows for `runFlow`,
mocks and the like; give several folders to run the flows of each. A suite in a given folder is
skipped, saying so on stderr: `skipped e2e/all.suite.yaml (a suite; name it to run it)`.

A file ending in `.rayo.yaml`, the name suites had before, is no longer read: given, it fails,
saying the rename, such as `staging.rayo.yaml -> staging.suite.yaml`; in a folder, it is skipped,
saying the same.

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

`retries: 3` means up to 4 tries: the first and 3 more. The line says `passed on retry 2` when the
third try passed, and `failed all 4 tries` when none did.

If it passes at last it is `FLAKY`, not `PASS`, with a note of where each try failed, its
[blame](#blame) and the folder with the rest. Each has its own video too.

```
[2/3] FLAKY scan  31.0 s  (passed on retry 1)
  try 1: tests/scan/flow.yaml:12: tapOn scan: not on screen: scan  (test)  .results/scan
[3/3] FAIL pay  27.2 s  (failed all 4 tries)
  try: 1
  ...

TOTAL 3: 1 passed, 1 flaky, 1 failed, 2 retried  78.1 s
```

| | |
|---|---|
| `retried` | Flows tried more than once, whether they passed or not |
| JSON | `flaky` of the run; of a flow, `status` `FLAKY`, `tries`, `timesFailed`, `retries` allowed, and a failure for each failed try |
| JUnit | See [Flaky flows](#flaky-flows) |

#### Flaky flows

A run whose flows all passed, some of them `FLAKY`, exits `0`; with `--flaky-fails-build`, or
`flakyFailsBuild: true` in the suite, it exits `4`, see [Exit codes](#exit-codes).

In JUnit, as Maven Surefire writes it, which Jenkins, GitLab and most CI read: the `testcase` passes, and
has a `flakyFailure` for each try that failed, its `message` the error and the [blame](#blame), its `type` the
cause and its `stackTrace` all there is to know, as in a `.txt`. It isn't counted in `failures`. A `failure`
has the verdict as its `message` and the cause as its `type`; an `error` (interrupted) has `interrupted`. The
`classname` of each `testcase` is its folder from the one all flows are in, with dots, so CI pages group
them; `file` is from where rayo runs, with slashes. What XML can't hold is left out, and a text over 256 KiB
keeps its beginning and its end.

```xml
<testcase name="scan" classname="scan" file="tests/scan/flow.yaml" time="31.000">
  <flakyFailure message="try 1: tests/scan/flow.yaml:12: tapOn scan: not on screen: scan (test)" type="not on screen">
    <stackTrace>  why: not on screen: scan ...</stackTrace>
  </flakyFailure>
</testcase>
```

To run only some steps again, use [retry](#retry).

### Blame

Not every failure is the test's. Each says whose fault it is, its `blame`:

| Blame | When |
|---|---|
| `app` | The app crashed or hung during the flow, or isn't installed, or an element wasn't found while the app was in front with nothing readable on it: still starting, stuck, or not readable by accessibility. That one's cause stays `not on screen` |
| `device` | It doesn't answer, is starting, restarted, its interface hung, or the connection was lost |
| `test` | Anything else. It is only `app` or `device` if there is proof of it, so something slow to appear on a screen that shows other things is `test` |

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
  blame: app, the app crashed: java.lang.IllegalStateException: oops
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
| `PASS` | At the first try | 0 |
| `PARTIAL` | It passed, but only part of it ran: it began or stopped at a step ([`--resume`, `--from`, `--until`](#resume)), or was fixed [while held](#hold). It stays in `failed.json`, and the summary says `NOT WHOLE` | 0 |
| `FLAKY` | It failed, and passed when [retried](#retries) | 0, or 4 with [`flakyFailsBuild`](#flaky-flows) |
| `FAIL` | | 1 |
| `INTERRUPTED` | Ctrl-C while it ran | 130 |
| `NOT_RUN` | With the reason: interrupted, `--fail-fast`, or the run [stopped](#blame) | 1 |

If the connection is lost, the flow fails and the next goes on with a new one. `--rerun-failed` runs what is in `<output>/failed.json`: what
failed, was interrupted or didn't run, until it passes; and what passed only in part, until it passes whole.

### Results

With several flows, one line each, and for a failure what helps to know why: first the verdict,
`<flow> failed at <step>: <cause>`, then the rest. Of `step failed`, the verdict goes on with the first line
of why and where the step is written, such as `login failed at assertTrue ${ok}: step failed: it is false (login.yaml:12)`,
so the line is enough on its own; a failure that isn't of a step says `failed before its first step` or
`failed outside a step`. A single flow shows every step. The last line says where the results went:
`results: .rayo  (rayo explain)`, when a flow didn't pass.

How much is printed is `--detail`, or `detail:` in a suite:

| | |
|---|---|
| `brief` | Only the verdict and `saved` of a failure; no steps |
| `normal` | The default: as below; every step of a single flow |
| `full` | Also `screen`, and every step of every flow; the [files](#failures) of a failure are whole |

The cause of a verdict, `cause` in JSON, is what went wrong; [`blame`](#blame) is whose fault it is, and `message` of an [error](#errors) is the text:

| | |
|---|---|
| `not on screen` | What a step looked for wasn't there; `nearest` says what was most like it |
| `timeout` | Something didn't happen in time, or stayed on screen |
| `app crashed` | The app crashed, hung or isn't installed, see [Blame](#blame) |
| `unmocked call` | The app called what no [mock](#mock) answers |
| `device error` | The device was lost, restarted or hung, see [Blame](#blame) |
| `step failed` | Anything else, such as a script or a command that failed |

```
[1/3] PASS book  18.2 s
[2/3] FAIL scan  9.1 s
  scan failed at tapOn finished_close: not on screen
  why: not on screen: finished_close
  step: tapOn finished_close
  at: shared/flows/dock.yaml:8
  from: tests/scan/flow.yaml:23 runFlow ../../shared/flows/dock.yaml
  before:
    extendedWaitUntil finished
  nearest:
    id: com.rides:id/finish_close | text: Close  [40,2100][1040,2220]
  calls:
    12:00:01.250 GET /trips -> 200
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
| | The verdict, a line without a key |
| `why` | The error |
| `blame` | `test`, `app` or `device`, and what shows it, see [Blame](#blame) |
| `step`, `at` | The step that failed, and where it is written |
| `from` | The step that ran that flow, and so on outwards |
| `crash` | The app crashed or hung during the flow: what the device log says |
| `before` | The last 3 steps that passed |
| `nearest` | Of `not on screen`, at most 5 elements on it most like what was looked for, the closest first |
| `calls` | The last 5 to the [mock](#mock), and before them those of the failure window that weren't mocked or weren't answered 2xx or 3xx |
| `screen` | With `--detail full`, the elements on it, at most 40 lines; they are all in `screen.txt` |
| `saved` | The folder with the [rest](#failures) |
| `recording` | A video; of flows that passed, only in the file |
| `warning` | Something that may explain this, or the next flow |

`--results <file>` writes it, as the file's name says. It may be repeated, for several formats at once:

```
rayo test --results out.txt --results junit.xml all.suite.yaml
```

| File | |
|---|---|
| `.txt` | As above for every flow, with `run`, `device` and `began` at the top and each flow's `file` |
| `.json` | For a program |
| `.xml` | JUnit, for a CI, see [Report](#report) |
| `.md` | Short, for a chat or a mail: the verdict and counts, then what failed, what was flaky and what was slow |

A path is as written: from where rayo runs, or in a suite from the suite. rayo sends nothing anywhere;
it writes the report, and you send it.

`--show`, or `show:` in a suite, says which flows the results list, the screen's list at the end too:
`failed` (also interrupted and not run), `flaky`, `slow`, `all` (default), or several, such as
`--show failed,flaky`. They are all counted anyway. `slow` is the slowest, as many as `--slowest <n>` says (default 5),
which is also the length of the slow list of `.md` and of `summary.slowest` in `.json`. JUnit always has every flow,
as what reads it counts them. Which files stay is [`keep`](#what-is-kept)'s.

Each flow has its attempts (`passed on retry 2`, `failed all 3 tries`), its time, when it began, and the
step that failed and why. `--timings` and `--profile` are only for the screen, while it runs.

```markdown
# Rides

**FAIL** · 3 flows: 1 passed, 1 flaky, 1 failed · 78.1 s · 2026-11-22T21:11:18Z · emulator-5554

## Failed

- **pay** 27.2 s, failed all 2 tries: `tapOn Pay` at tests/pay/flow.yaml:12: not on screen: Pay (test)

## Flaky

- **scan** 31.0 s, passed on retry 1: `tapOn scan` at tests/scan/flow.yaml:9: not on screen: scan (test)

## Slowest

- scan 31.0 s
- pay 27.2 s
- book 19.9 s
```

#### A template of your own

`--results-template <file>`, or `resultsTemplate:` in a suite, writes the results whose name ends as its own does
(`report.md` writes every `--results *.md`) as it says, instead of the format built in:

```
{verdict} {suite}: {passed} passed, {flaky} flaky, {failed} failed, {skipped} not run in {duration}, from {started}
{flows}- {status} {name} {duration} {attempts} {step} {error}
{end}
```

| Run | |
|---|---|
| `{suite}` | The suite's name, or what was run |
| `{verdict}` | `PASS`, `FAIL`, `FLAKY` or `PASS, with flaky flows` |
| `{total}`, `{passed}`, `{flaky}`, `{failed}`, `{interrupted}`, `{skipped}` | How many; `{skipped}` didn't run |
| `{duration}`, `{started}` | Such as `78.1 s`; in UTC |
| `{device}` | Its serial |
| `{flows}`...`{end}` | Written once for each flow that `show` says |

| A flow, between `{flows}` and `{end}` | |
|---|---|
| `{name}`, `{file}`, `{status}` | Its [outcome](#outcomes) |
| `{attempts}`, `{tries}` | Such as `passed on retry 2`, empty for one try; how many |
| `{duration}`, `{started}` | All its tries; when the first began |
| `{step}`, `{at}`, `{error}` | Of its last try that failed: the step, where it is written, and what went wrong |
| `{cause}`, `{blame}` | Of that try: the cause of its verdict, such as `not on screen`; `test`, `app` or `device`, see [Blame](#blame) |

In a `.txt`, a line that begins with an outcome is a flow: `grep -E '^(FAIL|NOT_RUN)' results.txt`. What
follows is about it: `  key: value`, or `  key:` and lines with four spaces. `TOTAL` is the last line.

In a `.json`, every key is always there: what there is none of is `""`, `0` or `[]`.

```json
{
  "schemaVersion": 3,
  "run": "Rides",
  "device": "emulator-5554",
  "order": "",
  "stopped": "",
  "ok": false,
  "summary": {
    "began": "2026-11-22T21:11:18Z", "seconds": 27.402,
    "total": 3, "passed": 1, "flaky": 0, "failed": 1, "interrupted": 0, "notRun": 1, "retried": 0,
    "failedFlows": ["scan", "long"], "flakyFlows": [],
    "slowest": [{ "name": "book", "seconds": 18.21 }, { "name": "scan", "seconds": 9.114 }]
  },
  "flows": [
    {
      "name": "scan",
      "file": "tests/scan/flow.yaml",
      "status": "FAIL",
      "blame": "test",
      "began": "2026-11-22T21:11:36Z",
      "seconds": 9.114,
      "attempts": "",
      "step": "tapOn finished_close",
      "at": "shared/flows/dock.yaml:8",
      "why": "not on screen: finished_close",
      "tries": 1,
      "timesFailed": 1,
      "retries": 0,
      "notRunReason": "",
      "warnings": [],
      "recordings": [],
      "failures": [
        {
          "try": 1,
          "error": {
            "file": "shared/flows/dock.yaml", "line": 8, "step": "tapOn finished_close",
            "message": "not on screen: finished_close", "expected": "", "nearest": "", "fix": ""
          },
          "verdict": "scan failed at tapOn finished_close: not on screen",
          "cause": "not on screen",
          "blame": "test",
          "because": "",
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
| `schemaVersion` | number | Changes if the meaning of a key does |
| `run` | text | The suite's name, or what was run |
| `id` | text | The run's own, as failed.json and history.jsonl give it: when it began, in UTC, then 4 hex digits |
| `device` | text | Its serial |
| `order` | text | `--shuffle=<seed>`, if shuffled |
| `shard` | mapping or null | The part of --shard i/n this run was; null if all |
| `stopped` | text | Why, if it stopped before the end, see [Blame](#blame) |
| `ok` | true or false | Nothing failed, was interrupted or didn't run, nor was flaky with flaky-fails-build: as the exit code |
| `summary` | mapping | The run at a glance, see below |
| `flows` | list | Those that show says, see below |
<!-- end -->

`summary`:

<!-- generated: results summary -->
| Key | | |
|---|---|---|
| `began` | text | In UTC |
| `seconds` | number | All of it |
| `total` | number | Flows |
| `passed` | number | At the first try |
| `flaky` | number | Failed, then passed on a retry |
| `partial` | number | Passed, but began or stopped at a step (--resume, --from, --until, or fixed while held): not a pass of the whole flow |
| `failed` | number |  |
| `interrupted` | number |  |
| `notRun` | number |  |
| `retried` | number | Flows tried more than once, whether they passed or not |
| `failedFlows` | list | The names of those that failed, were interrupted or didn't run |
| `flakyFlows` | list | The names of those that were flaky |
| `slowest` | list | The flows that took longest, as many as slowest says |
| `exit` | mapping | The exit code of the run, see [Exit codes](#exit-codes) |
<!-- end -->

Each of `flows`:

<!-- generated: results flow -->
| Key | | |
|---|---|---|
| `name` | text | The flow's |
| `file` | text | The flow's |
| `status` | text | One of the [outcomes](#outcomes) |
| `blame` | text | Of a flow that failed, whose fault it is, or empty, see [Blame](#blame) |
| `began` | text | Its first try, in UTC; empty if it didn't run |
| `seconds` | number | All its tries |
| `attempts` | text | Such as 'passed on retry 2' or 'failed all 3 tries'; empty for one try |
| `step` | text | Of its last try that failed, the step that failed |
| `at` | text | Where that step is written, file:line |
| `why` | text | What went wrong in that try |
| `tries` | number | Those it took |
| `timesFailed` | number | Of those |
| `retries` | number | Those allowed |
| `notRunReason` | text | If it is `NOT_RUN` |
| `warnings` | list | What may explain this, or the next flow |
| `recordings` | list | Videos |
| `mark` | mapping or null | What was said of it while held, with .mark; null if nothing |
| `failures` | list | One for each try that failed, also of a flow that passed at last |
<!-- end -->

Each of `failures`:

<!-- generated: results failure -->
| Key | | |
|---|---|---|
| `try` | number | From 1 |
| `verdict` | text | In a line: '<flow> failed at <step>: <cause>'; of 'step failed', then the first line of why and `(<file>:<line>)`; 'failed before its first step' or 'failed outside a step' if it isn't of a step; of an interrupted flow, '<flow> interrupted at <step> (<file:line>)' |
| `cause` | text | What went wrong, see [Results](#results) |
| `error` | mapping | What went wrong, at which step, see [Errors](#errors) |
| `blame` | text | Whose fault it is: `test`, `app` or `device`, see [Blame](#blame) |
| `because` | text | What shows that it isn't the test |
| `from` | list | The step that ran that flow, and so on outwards |
| `before` | list | The last 3 steps that passed |
| `nearest` | list | Of 'not on screen', the elements on it most like what was looked for |
| `calls` | list | The last 5 to the mock, and before them those that went wrong in the failure window |
| `screen` | list | The elements on it |
| `crash` | text | The app crashed or hung during the flow: what the device log says |
| `saved` | text | The first file of the rest, which are named alike, see [Failures](#failures) |
| `scenarios` | list | Of the mocks, the state each was in as it failed; empty without scenarios |
| `trail` | list | The scenarios as each step began, the last 2000 steps: what --resume and --from set, see [Resume](#resume) |
| `next` | list | What to do about it, a line each, by its cause and blame: the same for the same failure, for a program to follow |
<!-- end -->

```
jq -r '.flows[] | select(.status == "FAIL") | "\(.name): \(.failures[-1].verdict)"' results.json
(Get-Content results.json -Raw | ConvertFrom-Json).flows | Where-Object status -eq FAIL
```

### Explain

`rayo explain` prints, for each flow of the last run that hasn't passed since, only what tells why:
the verdict, `why`, `at`, `from`, the `nearest` elements, the `calls` and `saved`, as a failure does. It
reads `<output>/failed.json`, which keeps the last failure of each. Give it a run's folder, a `.zip` of one as
CI keeps it, or a suite, whose `output` it reads; and a flow's file or name. With none, it is `.rayo`, or else
the output folder of a suite here, or a folder down, whose `failed.json` is newest, and it says which:

```
rayo explain
rayo explain pay
rayo explain all.suite.yaml pay
rayo explain rayo-results.zip --format json
rayo explain --latest
```

`failed.json` keeps a flow until it passes, so after a few runs it has flows of more than one; each says
its `run`, and `--latest` keeps those of the newest. Each run adds a line for each flow to
`<output>/history.jsonl`, the last 5000: `history:` says how a flow's last runs went, such as
`failed 3 of its last 10 runs, at the same step each time: it passes at times, so it may be flaky`.
`next:` says what to do about the failure, by its cause and whose it is, the same each time, also in
the results as `next`.

`--find <text>`, on `rayo hierarchy` and `rayo do --screen`, prints only the elements of the screen most
like the text, the closest first, instead of all of them.

`--changed-only`, on `rayo test` and `rayo check`, keeps only the flows of which a file changed since the last
commit, also those not committed yet: the flow's own, one it runs with `runFlow`, or a file of its mocks.
`--since <git ref>` does so since that ref, such as `--since origin/main`.

`rayo check` says which flows would run, in that order, with their tags and mocks; with `--format json`, also the retries of each.
Two flows with the same name are an error, as `--only` and the output folder can't tell them apart: give
one a `name:` in its header. A name of `--only`, or a tag, that no flow has is an error that says the
nearest one and those there are; a tag no flow has is let be when another of the list is there. A key
written twice in a step, a `when` or `while` without a condition, an empty `text` or `id`, and an `id`
with spaces around it are errors too. Most mistakes of a name, a key or a word end in `did you mean ...?`;
`rayo help test` says the key of a suite that each option overrides, as `(suite: output.keep)`.

### For a program

`--format` says what is printed: `human`, the default; `json`, for a program; `ndjson`, the same on
one line. `test`, `check`, `devices`, `hierarchy`, `do` and `mocks` take it, and a suite's `format`
too, which `--format` wins over. With `json` or `ndjson`, only that goes to the standard output.

| Command | `--format json` prints | Schema |
|---|---|---|
| `test` | The [results](#results), as a `.json` of `--results` | `rayo schema results` |
| `check` | `schemaVersion`, `ok` and how many `flows` are fine | `rayo schema check` |
| `devices` | `schemaVersion` and the `devices` | `rayo schema devices` |
| `hierarchy` | `schemaVersion` and the `screen`, a line each | `rayo schema hierarchy` |
| `do` | See [From other programs](#from-other-programs) | `rayo schema do` |
| `mocks` | `schemaVersion` and the `notes` | `rayo schema mocks` |

If a command can't do what it was asked, it prints instead:

<!-- generated: errors -->
| Key | | |
|---|---|---|
| `schemaVersion` | number | Changes if the meaning of a key does |
| `ok` | true or false | false |
| `exit` | number | The exit code, see [Exit codes](#exit-codes) |
| `errors` | list | One for each, see [Errors](#errors) |
<!-- end -->

`schemaVersion` changes if the meaning of a key does; every key is always there.

### Exit codes

The same whatever `--format` says:

<!-- generated: exit codes -->
| Code | | |
|---|---|---|
| `0` | passed | Every flow passed, or the command did what it was asked |
| `1` | failed | A flow failed, was interrupted or didn't run; or a step sent with `rayo do` failed |
| `2` | configuration | Something written is wrong: an option, a flow, a suite or a mock; nothing ran |
| `3` | device | No device to run on, or it was lost or unusable, so the flows couldn't run or didn't all run |
| `4` | flaky | Every flow passed, some only on a retry, and `flaky-fails-build` is `true`; else such a run exits `0` |
| `130` | interrupted | Ctrl-C, or SIGTERM as CI sends when it cancels a job: what ran is in the results, each interrupted flow with the step it was at |
<!-- end -->

### Errors

Mistakes in what is written and flows that failed are told the same way, as data:

<!-- generated: error -->
| Key | | |
|---|---|---|
| `file` | text | Where it is written; empty if it isn't of a file |
| `line` | number | 0 if not known |
| `step` | text | Of a flow that failed, the step |
| `message` | text | What is wrong |
| `expected` | text | What it takes, if that can be said |
| `nearest` | text | Of that, the closest to what is written |
| `fix` | text | What to write instead, if that can be said |
<!-- end -->

```json
{ "file": "flows/login.yaml", "line": 3, "step": "", "message": "unknown command \"tapOm\", did you mean tapOn?",
  "expected": "", "nearest": "tapOn", "fix": "tapOn" }
```

### Events

`rayo test --format ndjson` prints a line as each thing happens, so a program needn't wait for the
end: `flowStarted`; for each try, `stepFailed` if a step failed, then `attempt`; with `--on-fail hold`,
`held` as a flow is [held](#hold), with the `session`'s folder; `flowFinished`, after all its tries; and last
`outcome`, with the results. If the run can't begin, the only line is `error`.

<!-- generated: event -->
| Key | | |
|---|---|---|
| `schemaVersion` | number | Changes if the meaning of a key does |
| `event` | text | What happened, see [Events](#events) |
| `flow` | text | Its name; empty in outcome and error |
| `file` | text | The flow's |
| `index` | number | Of the flow, from 1 |
| `of` | number | Flows to run |
| `try` | number | Of stepFailed and attempt, from 1 |
| `status` | text | Of attempt, PASS or FAIL; of flowFinished, the flow's [outcome](#outcomes) |
| `error` | mapping or null | Of stepFailed and of an attempt that failed, see [Errors](#errors) |
| `result` | mapping or null | Of flowFinished, the flow as in the [results](#results) |
| `results` | mapping or null | Of outcome, the [results](#results), as --format json |
| `exit` | number | Of outcome and error, the exit code, see [Exit codes](#exit-codes) |
| `errors` | list | Of error, as rayo prints them with --format json |
| `session` | text | Of held, the output folder of the session that holds the flow: rayo do --output <it> .help, or rayo do .help there |
<!-- end -->

```
rayo test --format ndjson all.suite.yaml | jq -c 'select(.event == "stepFailed") | .error'
```

---

### Status

`rayo status` says, for each flow, how it went the last time it ran into the output folder, whichever run
that was, from `history.jsonl`: after fixing 2 of 14 flows and running only those, whether all 14 are green
now, and the run each one's files come from (`files.json`). It takes what `rayo test` takes: flows, a folder or a suite.

```
rayo status all.suite.yaml
  PASS    login     run 20261127T101500-1a2b
  PARTIAL checkout  run 20261127T103000-3c4d  (only part of it ran: run it whole)
  NEVER   refund
1 of 3 passed the last time they ran, from 2 run(s): not green
```

`PASS` and `FLAKY` count as passed; `PARTIAL` doesn't, as only part of the flow ran; `NEVER` never ran into the
folder. It exits `0` when every flow passed, else `1`. `--format json` prints the same, see `rayo schema status`.

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
saved 2 steps in tests/new/flow.yaml:4-5
rayo check: the flow is fine
```

If the flow is there already, its steps run first (`--no-run` to skip them) and `.save` adds to its
end, leaving the rest as it is. A new one needs `--app`, and `--mock` takes paths relative to it.
`rayo live all.suite.yaml login` is the suite's flow with the suite's `env`, `output`, `timeoutScale`, `ca`
and `unmocked`; `-e KEY=VALUE` sets a variable over them, as for `rayo test`.
The mock, the variables and `output` last for the whole session. When it ends, all is
[put back](#clean-state).

### Live commands

| | |
|---|---|
| Any command, or a list of them | Runs them up to the first that fails. Those that pass are kept |
| `.screen` | What is on screen, and how to refer to it |
| `.steps` | The steps done, numbered as `.rewind` counts them, each saved (with its step and line in the file) or unsaved |
| `.undo` | Forgets the last one, and says how many are left. **The device stays as it is** |
| `.save` | Adds them to the file, says on which lines, and runs `rayo check` on it |
| `.retry` | Reads the file again and runs its steps from the one that failed, so a step can be fixed in the file and tried again |
| `.from <n>` | The same, from its n-th step |
| `.rewind [n]` | Forgets the last n steps done (1), typed or of the file, and the calls they made to a [capture](#capture). **The device stays as it is**: move the app back by hand, then `.retry` |
| `.help`, `.quit [--discard]` | With steps not saved, `.quit` is an error and the session goes on: `.save` them, or `.quit --discard` ends and prints what is lost. The end of what is typed discards |

Typed, a command is a line; one that ends in `:` goes on until an empty line. The leading `- ` may
be left out.

### From other programs

`rayo do` sends commands to the live session, from the same folder or with the same `--output`.
Each call answers when the steps have run; its exit code is 1 if one failed, see [Exit codes](#exit-codes).

```
rayo live --serve --emulator tests/new/flow.yaml &     # until .quit, or 5 minutes unused (--idle-timeout)
rayo do .screen
rayo do --screen '- tapOn: { id: login }'
rayo do --format json - < steps.yaml
rayo do .save
rayo do .quit
```

| Option | |
|---|---|
| `--screen` | Also what is on screen afterwards. After a failure it always comes |
| `--screen=changes` | Only the lines that appeared (`+`) or went (`-`) since the screen the session last gave. The first has no screen before: it is all of it, said as `(whole screen: nothing before)`, and `baseline: true` |
| `--ignore-screen <regex>` | With `--screen`, leaves out the lines that match, such as a clock's, and says how many (`ignored`) |
| `--timeout <duration>` | Waits for the answer that long at most, then fails; the session goes on with the steps |
| `--discard` | With `.quit`, ends though steps were not saved |
| `--format json` | `schemaVersion`, `ok`, `output`, `error`, `failure` (as in the [results](#results), or `null`), `warnings`, `calls` to the mock meanwhile, `screen`, `baseline`, `ignored`, `unsaved` steps; `rayo schema do` |
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
3. `env` of the suite, for all its flows
4. What is set where the file is included
5. `env` in the file's header: a default, for what nothing above sets

In `${...}` and conditions, a name that nothing sets is `undefined`, not an error, so defaults work
with `||` or `??`. Only a whole `${...}` that ends up `undefined` fails, as there is nothing to write:

```yaml
env:
  CITY: ${ CITY || "Springfield" }
```

Values of `-e`, of a suite's `env` and of a header may all use `${...}`. Besides the variables,
rayo gives `rayo.file`, the flow's file name without its extension, and `rayo.platform`. To give one of
them a name that flows from another tool expect, set it once for all of them:

```yaml
# all.suite.yaml
flows: [login.yaml, book.yaml]
env:
  MAESTRO_FILENAME: ${rayo.file}
```

or, without a suite, `rayo test -e 'MAESTRO_FILENAME=${rayo.file}' flows/`.

### Tags

🧪 `--include-tags smoke,login` runs flows with any of them; `--exclude-tags slow` leaves flows out.

### Continuous mode

🧪 `rayo test -c` runs again when a flow, or a file it includes, is saved.

### Resume

After a fix to the mocks or a flow, go on from the step that failed instead of running the flow again:
`rayo test --resume <suite>`. The files are read again; the step that failed runs first. `onFlowStart`
and the steps before it don't run, so the app must be where the failure left it; their `mock:` steps
and the mocks of the flows they `runFlow` are still loaded. The mocks' scenarios are set as they were
as that step began: each failure in `failed.json` keeps them in `trail`, and `scenarios` as it failed.
Each call says the `mapping` that answered it and what it did to its `scenario`.

`--from <file:line>` begins one flow at any step, before or after the failure, and `--until <file:line>`
stops it after one, leaving the app there, without `onFlowComplete`. `--mock-state booking=booked` sets
scenarios over those of the last run. Such a run has no retries, and a pass is `PARTIAL`, which leaves the flow in
`failed.json`: run it whole to be sure. In a live session, `.states` lists the scenarios.

A step inside a `runFlow` with `when`, or a `repeat` with `while`, can be begun from: the condition is
taken as holding to reach it, and says so. The `startRecording` steps of `onFlowStart` still run, so
that a `stopRecording` finds its recording; a `stopRecording` of a recording the flow began before the step
only says so. `--resume` leaves out a flow that failed in `onFlowStart` or `onFlowComplete`, and says
so, and goes on with the others.

### Hold

`rayo test --on-fail hold <suite>` stops at the first failure, before `onFlowComplete` and without
retries, with the device, the app and the mocks as they are, and says `HELD <flow>` with the verdict,
`why`, `at`, `from` and `nearest`. The flow is fixed in its own files, run again from where it failed,
and the run goes on: nothing is copied and no flow is written for it. `rayo do` reaches it, in the
suite's output folder by itself; typed commands work too.

| | |
|---|---|
| `.why` | The failure again |
| `.screen`, `.find <text>` | What is on screen now; or only what is most like a text |
| `.diff` | What of the screen changed since it failed: `+` appeared, `-` went |
| `.retry` | Reads the flow and the flows it runs again, and runs from the step that failed to the end of its steps. A step that moved, as lines were added above it, is found by its text, or else by its place among the file's steps |
| `.from <step>` | The same from another step: `file:line`, a text of one, or n of `.steps`; also inside a `when` or a `repeat` |
| `.rewind [n]` | The same from the n-th step before the one that failed (1). The app stays as it is: move it back first if those steps changed it |
| `.steps` | The last steps that passed, numbered as `.from` and `.rewind` count them |
| `.probe [n]` | Runs the step that failed n times (5), as it is now, and says how often it passes and how long each took: a flake, or always |
| `.states` | The mocks' scenarios and the state of each |
| Any command, or a list | Runs as it is, to move the app; it isn't added to the flow |
| `.mark <as> [why]` | `app-bug`, `test-bug` or `flaky`, kept in the results and `failed.json` as `mark` |
| `.continue` | Once its steps passed: `onFlowComplete`, then the next flows |
| `.skip`, `.abort` | Leave it failed and go on; or run no more flows |

A flow fixed while held is `PARTIAL`, not `PASS`: it stays in `failed.json` and `rayo test --rerun-failed`
runs it whole. If it checks less than when it failed, fewer asserts or more `optional` steps, its warnings
say `WEAKENED`. With no command for 5 minutes, or `--hold-timeout`, the flow is left failed and the run goes on.
`rayo do --format json` answers with `held`: the flow, `at`, and `fixed` once its steps passed.

### Failures

Saved in `<output>`, next to `failed.json`, named after the flow and the try: `<flow>_failure.jpg`,
and `<flow>_retry1_failure.jpg` for the second try; `<flow>_2_...` for a second flow with that name.
Those of an earlier run are removed when the flow runs again. `saved` is the first of them.
With `--screens-before n`, or the suite's `output.screensBefore`, the screen after each of the last n steps that
passed is in `<flow>_screens-before.txt`: how the app got there, such as a dialog that came and went.

| File | Kind | |
|---|---|---|
| `failure.jpg` | `screenshot` | The screen, small: 480 pixels on the short side |
| `screen.txt` | `screen` | The elements on it, as `rayo hierarchy` shows them |
| `logcat.txt` | `logcat` | The device log from the failure window to the failure, of the app and of crashes; whole with `--detail full` |
| `requests.txt` | `requests` | The calls the app made to the [mock](#mock) in the failure window, with their bodies; all with `--detail full`; `requests.json`, a list like `calls`, if the results are also written to a `.json` |

The failure window begins 10 s before the failing step did; `--failure-window <seconds>`, or
`failureWindow:` in a suite, says how long.

### What is kept

What a run writes, and where, is said in one place: `output` in a [suite](#suites). Each key has an
option of `rayo test` with the same name, in kebab-case, that overrides it for one run; `rayo check`
takes them too, and says what the run would write.

```yaml
output: .rayo                                  # only the folder
output: { folder: out, keep: failed, video: true }
```

| | Takes | |
|---|---|---|
| `folder`, `--output` | a folder, relative to the suite (default `.rayo` where rayo runs) | Where everything goes |
| `video`, `--video` | `true` or `false` (default `false`) | A video of each flow whose header doesn't say `record`, see [Recording a whole flow](#recording-a-whole-flow) |
| `clean`, `--clean` | `none` (default), `auto`, `all` | The files of earlier runs. `none` keeps them beside the new. `auto`: a whole run begins with none; a partial one (`--only`, tags, `--rerun-failed`, `--resume`, `--from`, `--on-fail hold`, `--shard`, `--since`) removes a flow's files only as that flow starts, so a held flow's stay until its retry and the other flows' are untouched. `all`: every run begins with none. Only the files `files.json` lists are removed, never one rayo didn't write |
| `keep`, `--keep` | `all` (default), `failed`, `none` | `all`: the files of every try. `failed`: the files of a try that passed are removed, also its videos and screenshots; a try that failed keeps its own, even if a retry then passes. `none` writes none. `failed.json` and the [results](#results) are always written |
| `artifacts`, `--artifacts` | `video`, `screenshot`, `logcat`, `screen`, `requests` (default all) | The kinds of file written; `screenshot` is also that of `takeScreenshot` |
| `nameTemplate`, `--name-template` | `{flow}`, `{attempt}`, `{kind}` and `{ext}`, and `{run}` if wanted (default `{flow}_{attempt}_{kind}.{ext}`) | How the files are named in `<output>`. `{run}` is the run's id, as in the results, such as `{run}/{flow}_{attempt}_{kind}.{ext}` for a folder each run. `{attempt}` is empty for the first try, then `retry1`..., `{kind}` is `failure`, `screen`, `logcat`, `requests`, a screenshot's name, and empty for the flow's video; the `_` or `-` around what is empty goes. A `/` makes folders, and a part of the path left without a name takes the try's |

Which run wrote which file is in `<output>/files.json`: for each flow, the id of its last run and the files
it wrote (`files`), and with `clean: none` those of its earlier runs still there (`older`). The `recordings`
of each flow are also in the `.json` [results](#results), so `rayo check` notes when `results` has no `.json`.

While a run writes to a folder, `<output>/running.json` says its id, pid and device. A second `rayo test` into the
same folder is refused with the pid of the first, unless `--ignore-lock`. A run that was killed leaves it: the next
says so. The `.json`, `.xml`, `.md` and `.txt` results are written as the run begins, each flow `INTERRUPTED`, and
again as it ends; so a killed run leaves results that say so, not those of an earlier run.

A video of each flow (`video: true` or `--video`) and a flow that records to a file of its own (`startRecording: <path>`,
also in a flow it runs) can't go together, as only one recording goes on at a time: `rayo check` and `rayo test`
refuse it before the device is touched. Give the flow `record: false`, or use `startRecording` without a path.

When the first 3 flows of a run all fail alike, at the same step with the same message, what they share, such as a login,
is broken: the run stops, `STOPPED setup: ...`, and the rest are `NOT_RUN`; `--keep-going` runs them all.

What would change nothing is an error, before the run, and `rayo check` says the same of a suite:
`video` in `artifacts` when no flow is recorded (`video: true`, `record: true` in a flow, or `startRecording`),
`artifacts` or `nameTemplate` with `keep: none`. So are `--flaky-fails-build` with `--retries 0`,
and `--hosts` when no flow goes through the proxy.

```
rayo test --keep failed --artifacts video,screenshot all.suite.yaml   # with output: { video: true } in the suite
rayo test --name-template '{flow}_{attempt}/{kind}.{ext}' all.suite.yaml   # a folder for each try: pay/failure.jpg, pay/pay.mp4, pay_retry1/...
```

| Exit code | |
|---|---|
| 0 | Everything passed |
| 1 | A flow failed or didn't run, or an error |
| 2 | No command given |
| 130 | Interrupted with Ctrl-C; what was set up on the device is undone |

### Timings

`rayo test --timings` shows where a run is slow or stuck, without the internals of `--profile`:

- before the flows, where the setup went: `setup: pick 3 ms, install and device 0 ms, agent and mocks
  900 ms; 4 adb calls` (picking the device, installing the app and the suite's device settings,
  starting the agent and the mocks, and how many times adb was asked so far)
- every step with its time, and `◂` with the total of each `runFlow`, `repeat` and `retry`
- `… still on <step> after 10 s` every 5 s while a step runs, so a hang shows where it is
- after each flow, its 5 slowest steps; at the end, the 10 slowest flows
- `default waits ×2` at the start, with a [`--timeout-scale`](#default-times) other than 1

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

`--timeout-scale <factor>`, or `timeoutScale:` in a suite, multiplies all of these: `2` for a slow CI emulator,
`0.5` for a fast local one. It takes 0.1 to 10; the option wins over the suite, and the default is 1. A step's own
`timeout:` is not scaled, and `timeout: 0` stays 0. With `--timings`, a factor other than 1 is shown at the start.

---

## Limitations

- Not supported: `addMedia`, running on several devices at once (`--shard i/n` splits the flows
  between machines instead), a
  `config.yaml` (see [suites](#suites)), and selectors by `css`, `width`, `height` or `traits`. An unknown command is an
  error, never ignored.
- `setLocation` and `travel` need Android 12 or newer.
- WebViews show only what they expose to accessibility.
- What a dialog, sheet or other window covers is not on screen: only what is on top can be found
  or tapped. Close the dialog first, for example with a `when` guard.
- Only one tool can automate a device at a time: not while another runs.
- What is marked 🧪 is **written but not tested on a
  device yet**, and so are relative selectors and tags.
