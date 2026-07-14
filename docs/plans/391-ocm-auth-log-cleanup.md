# 391: Route OCM auth messages to structured log

Branch: `srepd/ocm-auth-log-cleanup`

## What

The async OCM authentication goroutine in `cmd/root.go` used
`fmt.Fprintln(os.Stderr, ...)` for two status messages ("tokens expired"
and "authentication successful"). These fire after the TUI alt-screen is
active, so they're either invisible or flash briefly — providing no value
to the user while cluttering stderr.

## Approach

Replace the two `fmt.Fprintln(os.Stderr, ...)` calls with `log.Info()`
so they route to the systemd journal (or log file on macOS) alongside all
other OCM lifecycle messages. The TUI already surfaces OCM auth status
via `OCMClientReadyMsg`.

## Files

- `cmd/root.go` — two lines changed in the `ocmAuthPending` goroutine
