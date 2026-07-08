# Plan 104: Fix asyncWriter data race and send-on-closed-channel panic

**Branch:** `srepd/asyncwriter-race`

## Problem

`cmd/asyncwriter.go` had two concurrency defects on the `closed` flag, which is read
in `Write` (from the logging goroutine) and written in `Close` (from shutdown):

1. **Data race** on the plain `bool closed`.
2. **`panic: send on closed channel`** (TOCTOU): `Write` passes the `if aw.closed`
   check, then `Close` runs fully — sets `closed`, `close(aw.out)` — then `Write`
   reaches `case aw.out <- msg` and panics.

An `atomic.Bool` would fix (1) but **not** (2): the check and the send are separate
operations on separate objects, so an atomic flag cannot make them one transaction.

## Solution

Add a `sync.Mutex` that guards the `closed`-check **and** the channel send in `Write`,
and the `closed`-set **and** `close(aw.out)` in `Close`. Because they share the lock,
a `Write` can never send on a channel `Close` has already closed. `dropped` stays on
`atomic` (it was already race-free).

`Close` releases the lock before waiting on `<-aw.done` (the drain goroutine), since
`Write`'s send is non-blocking and the goroutine's completion does not depend on the
lock; a concurrent `Write` then sees `closed==true` and returns `os.ErrClosed`.

## Files Modified

- `cmd/asyncwriter.go` — `sync.Mutex`; locked `Write`/`Close`.
- `cmd/asyncwriter_test.go` — `TestAsyncWriter_ConcurrentWriteClose` + `syncBuffer`.

## Tests (TDD)

Written first, run under `-race`, seen **fail** with both a data-race report and
`panic: send on closed channel` (confirming atomic-alone would be insufficient), then
green after the mutex:
- `TestAsyncWriter_ConcurrentWriteClose` — 50 iterations of 8 concurrent writers
  racing a concurrent `Close`; asserts no panic and post-`Close` writes return
  `os.ErrClosed`.

## Verification

`make test-all` green; `go test -race ./cmd/...` green.

## Lessons Learned

- `atomic.Bool` fixes a flag race but not a check-then-act TOCTOU across two objects
  (flag + channel). When a "closed" check gates a channel send, both must be under the
  same lock, or the send can hit a closed channel and panic.
- Reproduce concurrency bugs with a stress loop under `-race` *before* fixing — it
  proved the panic was real and that the atomic-only approach would not have sufficed.
