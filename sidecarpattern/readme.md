# Sidecar Pattern Demo (Go)

A minimal, dependency-free demo of the **sidecar pattern** using two
independent Go programs that communicate only through a shared file.

## What is the sidecar pattern?

The sidecar pattern is a way of splitting a service's responsibilities
into two (or more) separate processes that are deployed and run
*together*, sharing the same lifecycle and the same local resources
(disk, network namespace), but that don't share code and don't call
each other directly.

The name comes from a motorcycle sidecar: it isn't part of the main
bike, but it travels everywhere the bike goes, attached to it. In
software, the "main container" does the actual job it exists for,
and the "sidecar container" rides alongside it, adding some
supporting capability — without the main container needing to know
or care that the sidecar exists.

It's one of the core patterns behind Kubernetes Pods (where a Pod is
explicitly designed to run a main container plus one or more sidecar
containers that share a network and can share mounted volumes) and
service-mesh proxies like Envoy/Istio, which run as a sidecar next to
every service instance.

## Why use it?

- **Separation of concerns.** The main app focuses purely on its job
  (in this demo: making HTTP requests). Logging, monitoring, log
  shipping, proxying, config reloading, etc. live in the sidecar
  instead of being bolted onto the main app's codebase.
- **Language/runtime independence.** The main app and the sidecar
  don't need to be written in the same language, since they only
  communicate through a shared file, socket, or network port — never
  through function calls. You could rewrite this demo's sidecar in
  Python or Rust and the main app would never know.
- **Independent deployment and scaling.** You can update, restart, or
  redeploy the sidecar (e.g. to change how logs are shipped) without
  touching or redeploying the main app, and vice versa.
- **Reusability.** The exact same sidecar (a log watcher, a metrics
  exporter, a TLS-terminating proxy) can be attached to many different
  main applications with zero changes.
- **Fault isolation.** If the sidecar crashes or misbehaves, the main
  application keeps running and doing its job — they're separate
  processes, not separate threads in one program.

The trade-off is added operational complexity: now you have two
processes to run, monitor, and keep in sync (as you just saw — if
they don't agree on *where* the shared file lives, the pattern breaks
silently).

## How this demo maps to the real pattern

| Real-world sidecar setup | This demo |
|---|---|
| Main container (does the real work) | `app/main.go` |
| Sidecar container (supporting capability) | `sidecar/main.go` |
| Shared volume mounted into both containers | `logshred.log`, a plain file both processes read/write via relative path |
| Pod (main + sidecar deployed together) | Both binaries run from the same working directory on your machine |

In Kubernetes this would literally be one Pod spec with two container
definitions and one shared `emptyDir` volume mounted at the same path
in both containers — the mechanics are identical to what's happening
here with a shared local file.

## The code

### `app/main.go` — the main container

Its only job: send an HTTP request to a target URL at a random time,
then record that it happened.

- Opens `logshred.log` once, in append mode, and keeps writing to it
  for the process's whole lifetime.
- Loops forever: sleeps a random 1–5 seconds (`time.Sleep` with
  `rand.Intn`), fires an HTTP GET with a 5-second timeout, then writes
  one line to the log file with the exact timestamp the request was
  sent, the random delay used, the target URL, and the resulting
  status (or the error, if the request failed).
- Calls `f.Sync()` after every write. This forces the OS to flush the
  write to disk immediately rather than buffering it, which matters
  here because the sidecar is reading that same file from a
  completely separate process — without the sync, the sidecar could
  see a stale or incomplete view of the file for longer than expected.

### `sidecar/main.go` — the sidecar container

Its only job: notice new lines in `logshred.log` the moment they
appear, and print out when each request was made.

- Keeps track of an `offset` — how many bytes of the file it has
  already read.
- Every 200ms, it reopens the file, checks its current size with
  `Stat()`, and if the size has grown past `offset`, it seeks to
  `offset` and reads only the *new* bytes with a `bufio.Scanner` —
  this is the same trick behind `tail -f`.
- `printRequestTime` splits each line on `" | "`, takes the first
  field (the timestamp), and parses it with `time.Parse` using
  `time.RFC3339Nano` — the exact layout the main app used to format
  it. It then reprints the time in a friendlier `HH:MM:SS.mmm` form
  alongside the full line.
- If the file shrinks (e.g. you deleted and recreated it), it resets
  `offset` to 0 so it doesn't miss anything.
- Because polling can otherwise look like silent failure, it also
  prints a heartbeat every 3 seconds — either "file not found yet" or
  "no new data" — so you can always tell it's alive and exactly what
  path it's watching.

This polling approach uses only the standard library (`os`, `bufio`,
`time`) — no external file-watcher dependency — which keeps the demo
copy-paste-runnable with nothing to `go get`.

## Run it

Open two terminals **in the same folder**:

```bash
# terminal 1 — the main app
go run ./app

# terminal 2 — the sidecar
go run ./sidecar
```

Both will print the absolute path of `logshred.log` they're using —
confirm those two lines match before anything else. Then watch
terminal 2 print each request's timestamp within ~200ms of terminal 1
writing it.

To reset, delete `sidecar.log` and restart both.

