# A full example

A shop: log in, buy something, the bank refuses the card, and no connection. These are all its files, in
[examples/shop](examples/shop). The app is made up; the files are real, and rayo checks them.

## The folders

```
shop/
├─ all.suite.yaml               what runs together, and how
├─ steps/login.yaml            steps that several tests use
├─ tests/
│   ├─ browse/flow.yaml
│   ├─ pay/flow.yaml
│   ├─ declined/flow.yaml      and declined.json, an answer only for it
│   └─ offline/flow.yaml
└─ mock/
    ├─ mappings/shop.json      which call gets which answer
    └─ __files/products.json   a long answer
```

## The suite

<!-- file: examples/shop/all.suite.yaml -->

## Steps that are shared

Each test begins by running it, so each begins clean and can be run alone.

<!-- file: examples/shop/steps/login.yaml -->

## The tests

The simplest one:

<!-- file: examples/shop/tests/browse/flow.yaml -->

This one checks what the app sent, and changes what the backend says:

<!-- file: examples/shop/tests/pay/flow.yaml -->

This one adds an answer that wins over the normal one:

<!-- file: examples/shop/tests/declined/flow.yaml -->
<!-- file: examples/shop/tests/declined/declined.json -->

And this one cuts the connection:

<!-- file: examples/shop/tests/offline/flow.yaml -->

## The backend

<!-- file: examples/shop/mock/mappings/shop.json -->
<!-- file: examples/shop/mock/__files/products.json -->

## Run it

```
rayo check all.suite.yaml                 # mistakes, without a device
rayo test --emulator all.suite.yaml       # all of it
rayo test --only pay all.suite.yaml       # one
```

## What comes out

How it would look, if `pay` failed once and the app had a bug in `offline`:

```
[1/4] PASS browse  9.8 s
[2/4] PASS pay  31.4 s  (failed once, then passed)
  try 1: tests/pay/flow.yaml:13: tapOn refresh: not on screen: refresh  (test)  results/pay_failure.jpg
[3/4] PASS declined  11.2 s
[4/4] FAIL offline  41.9 s  (failed 2 times)
  try: 1
  why: not on screen: You are offline
  cause: app, the app crashed: java.net.ConnectException: unexpected end of stream
  step: assertVisible You are offline
  at: tests/offline/flow.yaml:7
  crash: FATAL EXCEPTION: main
    Process: com.example.shop, PID: 4711
    java.net.ConnectException: unexpected end of stream
  before:
    extendedWaitUntil products
    network offline
    tapOn refresh
  saved: results/offline_failure.jpg
  recording: results/videos/offline.mp4
  ...

TOTAL 4: 3 passed, 1 failed, of those failed 1 app, 2 retried  94.3 s
```

And in `results/`:

```
results/
├─ results.txt                 the same, with the details of every try
├─ junit.xml                   for a CI
├─ failed.json                 what --rerun-failed runs
├─ videos/                     pay.mp4, offline.mp4, offline_retry1.mp4: of the tries that failed
├─ offline_failure.jpg         of each failed try: the screen
├─ offline_screen.txt          the elements on it
├─ offline_logcat.txt          the device log
├─ offline_requests.txt        the calls to the backend
└─ offline_retry1_failure.jpg  ... and so on for the second try
```
