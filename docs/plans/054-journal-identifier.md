# Add srepd journal identifier and priority mapping

## Context

The journalWriter sends all log messages without a SYSLOG_IDENTIFIER,
making srepd logs impossible to filter with `journalctl -t srepd`.
Also uses hard-coded PriInfo for all messages regardless of level.

## Plan

1. Pass `SYSLOG_IDENTIFIER=srepd` in journal.Send() vars map
2. Map log levels to journal priorities (ERROR→PriErr, WARN→PriWarning, DEBUG→PriDebug)

## Files Modified

- `cmd/root.go` — journalWriter with identifier and priority mapping

## Verification

- `journalctl -t srepd` shows only srepd logs
- Error messages show as priority err in journal
