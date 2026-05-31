# Restructure log levels: demote noise to DEBUG, keep meaningful SRE actions as INFO

## Context

Current INFO level logs are noisy status updates (setStatus UI messages,
cache operations, data fetching counts) that are not useful for compliance
or operational review. INFO should be reserved for meaningful SRE actions:
acknowledging, silencing, re-escalating, logging into clusters, adding notes,
and querying Claude. Status updates and internal machinery belong at DEBUG.

Related: PR #179 (logging cleanup), PR #197 (journal identifier).

## Plan

1. Demote `setStatus` in `model.go` from `log.Info` to `log.Debug` -- this
   single change eliminates the majority of noise since all UI status
   messages flow through `setStatus()`.

2. Add new INFO-level logs for meaningful SRE actions that currently lack them:
   - Incident acknowledged (in `tui.go` acknowledgedIncidentsMsg handler)
   - Incident re-escalated (in `tui.go` reEscalatedIncidentsMsg handler)
   - Incident silenced (in `tui.go` silenceSelectedIncidentMsg handler)
   - Cluster login initiated (in `tui.go` loginMsg handler)
   - Cluster login finished (in `tui.go` loginFinishedMsg handler)
   - Note added to incident (in `tui.go` addedIncidentNoteMsg handler)
   - Claude query initiated (in `claude.go` handleClaudePrompt)
   - Claude response received (in `claude.go` handleClaudeResponse)
   - Incident opened in browser (in `tui.go` openBrowserMsg handler)
   - SOP opened in browser (in `tui.go` openSOPMsg handler)
   - Incident resolved notification (in `tui.go` updatedIncidentListMsg handler)

3. Demote launcher toolbox detection from INFO to DEBUG in `launcher.go` --
   this is a startup diagnostic message, not an SRE action.

4. Keep existing INFO logs in `cmd/root.go` unchanged -- they are startup/
   config messages that are appropriate at INFO level.

5. Keep the existing `silenceIncidents()` INFO log in `commands.go` -- it
   logs a meaningful action (silence requested) with structured fields.

## Files

- `pkg/tui/model.go` -- demote `setStatus` log from INFO to DEBUG
- `pkg/tui/tui.go` -- add INFO logs for ack, re-escalate, silence, login, note, resolve, browser, SOP
- `pkg/tui/claude.go` -- add INFO logs for claude query and response
- `pkg/launcher/launcher.go` -- demote toolbox detection from INFO to DEBUG
- `pkg/tui/model_test.go` -- add tests verifying setStatus uses DEBUG
- `pkg/tui/tui_test.go` -- add tests verifying INFO logs for SRE actions

## Verification

- `make test` passes
- `make lint` passes
- `grep -n 'log.Info' pkg/tui/model.go` returns zero results (all demoted)
- `grep -n 'log.Info' pkg/tui/tui.go` shows only meaningful SRE action logs
- `grep -n 'log.Info' pkg/launcher/launcher.go` returns zero results
- `grep -n 'log.Info' pkg/tui/commands.go` shows only silenceIncidents action log
