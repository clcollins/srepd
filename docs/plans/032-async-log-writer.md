# 032 - Async Log Writer Drop Mitigation

## Context

The `asyncWriter` in `main.go` silently drops log messages when its
internal channel buffer is full (the non-blocking `select`/`default`
path). Users have no indication that messages are being lost, making
it harder to diagnose issues in high-volume logging scenarios.

Additionally, the `newAsyncWriter` constructor accepted `*os.File`
instead of `io.Writer`, which made the component unnecessarily
coupled to file I/O and impossible to unit test without touching the
filesystem.

Prior plans consulted: 004 (panic handling), 005 (unsafe type
assertions) -- both emphasize observable failure modes over silent
behavior.

## Plan / Solution

1. **Add an atomic drop counter** (`dropped uint64`) to `asyncWriter`
   using `sync/atomic` for lock-free concurrent access.

2. **Increment the counter** in the `Write` method's `default` case
   when the buffer is full and a message must be dropped.

3. **Periodic drop reporting**: The background consumer goroutine
   checks the counter after every successful write. When the
   cumulative count crosses a multiple of 100, a synthetic
   `[asyncWriter] dropped N log messages due to full buffer` entry is
   written to the underlying writer.

4. **Final flush on Close**: If there are unreported drops (count not
   a clean multiple of 100) when the channel is drained, a final
   report with the total is written before the goroutine exits.

5. **Increase default buffer** from 1,000 to 5,000 to reduce drop
   frequency under normal operation.

6. **Refactor `newAsyncWriter` to accept `io.Writer`** instead of
   `*os.File`, enabling testability with `bytes.Buffer` and custom
   writers.

7. **Add a `Dropped() uint64` method** for programmatic access to the
   drop count (used by tests, potentially useful for future TUI
   status display).

## Files Modified

| File | Change |
|------|--------|
| `main.go` | Refactored `asyncWriter`: added `dropped` counter, `Dropped()` method, periodic/final drop reporting, `io.Writer` parameter, 5000 buffer size |
| `main_test.go` | New file with 6 test cases covering normal writes, buffer-full drops, close semantics, counter accuracy, final reports, and input-copy safety |
| `docs/plans/032-async-log-writer.md` | This plan document |

## Verification

- `make test` -- all tests pass including 6 new asyncWriter tests
- `make vet` -- no issues
- `make fmt-check` -- no formatting issues
- `golangci-lint run` -- zero issues
- Manual review: drop counter uses `sync/atomic` for thread safety,
  `io.Writer` interface enables dependency injection for tests
