# Mocking

rayo answers as the backend. A mock is a folder: `mappings/` says **when** to answer, and `__files/`
holds **what** to answer. The names are fixed.

```
mock/
├─ mappings/                 the rules: if the app calls this, answer that
│   ├─ products.json
│   └─ order.json
└─ __files/                  the bodies, as the backend would send them
    ├─ products/all.json
    └─ order/shipped.json
```

A flow names its mocks, in order, in its header:

```yaml
mock: [../shared/mock, mock/overrides.json]
```

## Two ways to connect the app

rayo can only answer calls that reach it. There are two ways to send them there:

- **Interceptor** (the default). The app itself sends its calls to rayo instead of the server. You add
  [a few lines](#in-your-app) to the app, only in its debug build. It sees the calls made through the
  app's own HTTP client.
- **Proxy** (`--network proxy`). rayo sets the phone's network proxy while the test runs, so every call goes
  through rayo, also those of SDKs in the app. [No code](#proxy): the debug build only has to trust rayo's
  certificate, from `rayo ca export`.

Which one:

- You can change the app, and all its calls go through one client: **interceptor**. Simplest, nothing on the phone changes.
- You can't change the app's code, or an SDK (payments, maps, chat…) makes calls of its own: **proxy**.
- The app pins certificates: **interceptor**, or turn pinning off in the debug build.

Both read and write the same mock folders, so you can capture one way and replay the other.
The [table](#proxy) below compares them in detail.

## In your app

rayo can't reach inside an app: the app has to send its calls to `http://127.0.0.1:8080` on the device.
With OkHttp, an interceptor does it. It can be **only in the debug build** (`src/debug`), and only on
when a test says so, so nothing of it reaches your users.

```kotlin
/** https://host/path becomes http://127.0.0.1:8080/host/path, when a test asked for it. */
object MockRedirect : Interceptor {
  @Volatile var on = false

  override fun intercept(chain: Interceptor.Chain): Response {
    val request = chain.request()
    if (!on) return chain.proceed(request)
    val url = request.url.newBuilder()
      .scheme("http").host("127.0.0.1").port(8080)
      .encodedPath("/${request.url.host}${request.url.encodedPath}")
      .build()
    return chain.proceed(request.newBuilder().url(url).build())
  }
}
```

Add it to the client with `addInterceptor(MockRedirect)`, and turn it on before the first call, from
what `launchApp: { arguments: { mock: true } }` gives:

```kotlin
MockRedirect.on = intent.getBooleanExtra("mock", false)
```

Android refuses calls that aren't HTTPS, so allow that one, also in `src/debug`, with
`android:networkSecurityConfig="@xml/network_security_config"`:

```xml
<network-security-config>
  <domain-config cleartextTrafficPermitted="true">
    <domain includeSubdomains="false">127.0.0.1</domain>
  </domain-config>
</network-security-config>
```

The host goes in the path so that several can be mocked, and so that a capture knows whom to call:
a mapping for `https://api.example.com/products` has the path `/api.example.com/products`. An app
with one backend may leave it out, as the examples here do; to capture it, use `--capture-from`.

## Proxy

The interceptor only sees the calls made through the app's own client. A third-party SDK that brings its
own client, such as a payment SDK that polls a URL from an answer, still calls the real server: a capture
misses those calls without saying so, and a mock run sends them live. With `--network proxy`, or
`network: proxy` in a flow's header, the device's HTTP proxy sends every call to rayo instead.

```
rayo ca new                                           # once per project: .rayo/ca, committed
rayo ca export app/src/debug/res/raw/rayo_ca.pem      # once, for the app to trust it
rayo test --capture mock/ --network proxy flow.yaml   # capture
rayo test --network proxy flow.yaml                   # mock
```

| | `interceptor` (default) | `proxy` |
|---|---|---|
| What it sees | Calls made through the app's own client | Every HTTP(S) call on the device: the app, SDKs; WebViews of `--hosts` |
| In the app | The interceptor, and plain HTTP allowed to `127.0.0.1` | No code: the debug build trusts rayo's certificate (CA) |
| On the device | `adb reverse`, so `127.0.0.1:8080` on the phone reaches rayo | The same, and the phone's proxy set while the flow runs, then put back |
| Misses | Calls of an SDK with its own client | Hosts the app pins certificates for, or doesn't trust rayo's CA for: the flow fails at once |

Both write and read the same folders, with the host in the path, so a capture made one way replays the
other way. An app that still rewrites its URLs works behind the proxy too.

- **The CA** (`ca.pem` and `ca-key.pem`) is, in this order: the folder of `--ca`, or of `ca:` in the
  suite; else `.rayo/ca` next to the suite or the flow, or in a folder above it up to the repository's
  root; else that of the computer, in `~/.rayo`, made the first time. `rayo ca new [<folder>]` makes one
  for the project, in `.rayo/ca` if not said: committed with the flows and the mocks, a fresh checkout
  or CI signs as the app expects, with no setup. Its key signs only for test builds that trust it.
  The run says which CA the proxy signed with. `rayo ca export` writes the certificate and the `network_security_config` that trusts it in
  `debug-overrides` only. A release build ignores it.
- **Hosts.** The proxy decrypts only some hosts and tunnels the rest untouched: those of `--hosts`, and
  their subdomains; else, capturing, all but Google's, Firebase's, analytics and crash reports; else
  those the mocks have in their paths. Then a call to a host of the mocks that none fits fails as
  `not mocked`, and a call to another host goes to the real one. The hosts left alone are listed after the flow.
- **Browsers.** A Custom Tab or the browser, such as for a login page, trusts only the system's CAs,
  and would show "Your connection is not private". The proxy reads the start of each TLS connection
  first: Chrome and what is made from it (WebViews, Cronet) send GREASE values, Android's own TLS
  doesn't. Such a connection is passed untouched, and its hosts listed after the flow, unless the host
  is in `--hosts`: name it there to capture or mock a WebView's calls.
- **Pinning.** A host the proxy opens whose TLS the app refuses fails the flow at the step it happens,
  as `pinned or untrusted CA: <host> rejected rayo's CA`: its calls would reach the real backend,
  neither captured nor mocked, so they are refused, never sent on. A mocked run never reaches the real
  backend of a host it mocks. The app needs rayo's CA in its debug `network_security_config`, from `rayo ca
  export`; a build that pins certificates needs the pins off in debug. A host meant to reach the real
  one is left out of `--hosts`, and passes untouched.
- **Plain `http://`** passes untouched, and is listed.
- **Clean-up.** The proxy is set by a shell on the device that puts the one before back when the flow
  ends, and also when the connection closes, however rayo stopped: a device isn't left with a proxy
  with nothing behind it.

## What happens on a call

1. The app calls `http://127.0.0.1:8080/...` on the device, which reaches rayo.
2. rayo looks for a mapping that fits: method, address, query, headers, body and state.

   ```json
   {
     "request": { "method": "GET", "urlPath": "/products" },
     "response": { "status": 200, "headers": { "Content-Type": "application/json" }, "bodyFileName": "products/all.json" }
   }
   ```

   If several fit, the lowest `priority` wins (5 by default), and then the last one loaded. So
   `overrides.json` wins over `shared/mock`, as it comes after.
3. It answers with `__files/products/all.json`, with what is between `{{ }}` filled in, such as
   `{{now offset='-8 minutes'}}`.
4. If none fits, the app gets a 404 and **the flow fails**.

## State

The same call can be answered differently as things advance. A mapping may belong to a scenario,
which begins in `Started`:

```json
{ "mappings": [
  { "scenarioName": "order", "requiredScenarioState": "Started", "newScenarioState": "paid",
    "request": { "method": "POST", "urlPath": "/payments" },
    "response": { "status": 200 } },
  { "scenarioName": "order", "requiredScenarioState": "shipped",
    "request": { "method": "GET", "urlPath": "/order" },
    "response": { "status": 200, "bodyFileName": "order/shipped.json" } },
  { "priority": 10,
    "request": { "method": "GET", "urlPath": "/order" },
    "response": { "status": 200, "jsonBody": { "state": "waiting" } } }
] }
```

Here paying moves `order` to `paid`. What the backend does by itself, the flow says:
`- mockState: { order: shipped }`. The last mapping is for any other state.

## Why two folders

- **Reuse**: several mappings can answer with the same body.
- **Readable**: the logic isn't buried in long bodies.
- **Easy to replace**: a body is a real answer, to paste without touching the rules.

It isn't required: an answer can be in its mapping, with `body` or `jsonBody`. A few lines go there;
what is long or shared goes in `__files/`.

## From the real backend

`rayo test --capture` writes such a folder from a real backend, through the interceptor or the proxy.
REST and GraphQL calls go in the same folder. A REST call is matched by its method, address, query and
body, as WireMock's recorder does, so calls to one endpoint with other bodies get their own answers. A
GraphQL call is matched by its operation and variables (and, if one
name is sent with several query documents, by its shape), so a `Wallet` query is never answered with a
`Geocode` answer, whatever order the app makes them in. See [GraphQL](REFERENCE.md#graphql).

When the backend changes, `rayo test --capture-update mock` captures again beside the mocks and says,
mapping by mapping, what changed; `rayo mocks apply mock` takes all of it, or `--only` some. With
`--check` it fails instead, for CI. See [Updating mocks](REFERENCE.md#updating-mocks).

## Keeping mocks up to date

A capture is made once. When the app or the backend changes after it, don't capture again from
nothing: these are the ways to bring the mocks up to date, keeping what still holds and the mappings
written by hand.

**Knowing that they are out of date.** A flow run on its mocks fails when the app calls what no
mapping answers, and says which calls, and the nearest mapping:

```
the app called what isn't mocked:
    GET /api/users/me/badges
```

A mapping that is no longer called doesn't fail anything: an update shows it as `gone`.

**The app makes a call more, or one less, or the backend answers otherwise.** Update from the real
backend, only the flows that changed, then take what you want:

```sh
rayo test --capture-update mock --only checkout flows/   # captures into mock/.update, and says what changed
rayo mocks apply mock                                    # takes the changed and the new
rayo mocks apply --only "GET /api/users/me/badges" mock  # or only these
rayo mocks apply --prune mock                            # also takes out those gone
```

The update makes its mocks as the capture made them (with the settings kept in
`capture.calls.json`), so a mapping that didn't change is the same, and isn't touched. Look at what
is printed, or at `mock/.update/changes.txt`, before applying.

**A new flow, on mocks that are there.** Go on with the capture, as if it had been made in one go:

```sh
rayo test --capture mock --capture-append --only newFlow flows/
```

**In CI.** Find out, without changing anything, whether the backend still answers as the mocks say:

```sh
rayo test --capture-update mock --check flows/
```

Each of these calls the real backend, for the flows it runs: it has to be reachable, with the
accounts and data the flows need. `--only` keeps that to the flows that changed. Commit the mocks
after `rayo mocks apply`, and leave `mock/.update/` out (add it to `.gitignore`).

**Tidy.** `rayo mocks check mock` says which mappings never answer, as a later one matches the very
same calls, and which bodies in `__files` no mapping uses: often leftovers of edits by hand.

All that a mapping can say, and
the commands `mock`, `mockState`, `network`, `waitForRequest` and `assertRequest`, are in the
[reference](REFERENCE.md#mock).

## Coming from WireMock or mitmproxy

A mocks folder of WireMock's, `mappings/` and `__files/`, is one of rayo's: give it to a flow's `mock`
as it is. rayo's captures load in WireMock too: they use only its matchers. What rayo can't do, such as
webhooks or proxying to a real backend, is an error when the mocks load, not a mock that does less.
An app that calls one backend, with `--capture-from`, has paths without the host, as WireMock's; through
the proxy, a mapping without the host answers too.

| WireMock recording | rayo |
|---|---|
| `POST /__admin/recordings/start`, then `snapshot` | `rayo test --capture mock flow.yaml` |
| `requestBodyPattern: auto` (`equalToJson` with `ignoreArrayOrder` and `ignoreExtraElements`, `equalToXml`, `equalTo`) | The default; `--rest off` for none |
| `repeatsAsScenarios: true` | Always: the same call answered otherwise gets states, of the scenario `capture` |
| `captureHeaders` | `--capture-headers Accept-Language,X-Api-Version` |
| `extractBodyCriteria` | Always: every body goes in `__files/` |
| `persist`, `--root-dir` | The folder of `--capture` |
| Near misses | Said when a call isn't mocked: the nearest mapping, and what of it didn't fit |
| `--global-response-templating` | Mappings without `transformers`; with `"transformers": ["response-template"]`, as WireMock's local mode |

| mitmproxy server replay | rayo |
|---|---|
| `mitmdump -w flows` | `rayo test --capture mock --network proxy flow.yaml`, or `rayo mocks import flows.har mock` with `mitmdump -nr flows --set hardump=flows.har` |
| `mitmdump --server-replay flows` | `rayo test flow.yaml`, with `mock: mock` in its header |
| `server_replay_ignore_params` | `--ignore-params a,b` |
| `server_replay_ignore_payload_params` | `--ignore-body a,b`, also for JSON and GraphQL variables |
| `server_replay_ignore_content` | `--rest off` |
| `server_replay_ignore_host` | `--ignore-host` |
| Answers given once each, in order (`server_replay_reuse` off) | States: a call made three times is answered as the three times were; after that, as the last |
| `server_replay_kill_extra` | Always: a call that isn't mocked fails the flow |
| `server_replay_refresh` | `{{now offset='...'}}` in a body |

Dates and times are matched by their form, not their value, so a call made with the time of the run
still fits; `--timestamps exact` to match them as they were. All these settings are kept with the
capture, and used again by `--capture-append` and `--capture-update`.
