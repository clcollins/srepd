# 363: No OCM auth in config-wizard mode (OB-6)

Issue: #353 (OB-6), part of the onboarding overhaul (#353, #324)
Branch: `srepd/config-mode-no-ocm`

## Problem

`launchTUIWithConfig()` — the config-wizard entry point — ran the same OCM
startup as the normal launch path: `ocm.CheckTokens()` plus an async
browser-auth goroutine. A brand-new user running `srepd config` (or hitting
the first-run wizard) saw "OCM tokens expired — opening browser for
authentication..." and OCM warnings before they had even saved a PagerDuty
token. Confusing at best, alarming at worst — and entirely unnecessary, since
config mode makes no OCM calls.

## Approach

Defer OCM, don't drop it. The wizard itself is OCM-free, and the session
that continues after the wizard connects OCM exactly like a normal launch —
a first-run user must not get a degraded (enrichment-less) srepd, or they
try it once, get underwhelmed, and never come back.

- `cmd/config.go` `launchTUIWithConfig()`: delete the OCM token-check block
  and the async browser-auth goroutine; pass `nil` ocmClient and `false`
  ocmAuthPending to `tui.InitialModel`. Config mode is now OCM-free.
- `cmd/root.go`: extract the (previously duplicated) OCM startup into a
  `setupOCM()` helper returning `(ocm.OCMClient, *ocm.Client, authPending,
  *ocmconfig.Config)`. `launchTUI()` — the normal path — is its only caller,
  with behavior unchanged.
- `pkg/ocm`: new `Connect(agentVersion)` — CheckTokens → browser auth if
  expired → NewClientFromConfig, blocking; for use inside a tea.Cmd.
- `pkg/tui`: post-wizard OCM handoff. `connectOCMCmdIfNeeded()` (nil when
  connected or dev mode; result flows through the existing
  `OCMClientReadyMsg` handler, which already wires the client, deferred
  backplane, and cluster enrichment). `ocmHandoffCmd(requirePDConfig)` sets
  `ocmAuthPending` and gates on a usable PD config. Wired into all four
  wizard-exit paths: save (`configSavedMsg`, batched with PD init), discard,
  no-changes, and abort — the last three require an existing PD config so a
  brand-new user backing out never gets a browser-auth prompt on the way out.
  Injectable `model.ocmConnect` for tests.

## Tests (TDD — written first)

`cmd/config_ocm_test.go` source-scan regression guards (pattern from plan 086
verification):
- `TestConfigMode_NoOCMAuth` — `cmd/config.go` must not reference
  `ocm.CheckTokens`, `ocm.AuthenticateAsync`, `ocm.NewClientFromConfig`,
  `ocm.ApplyAuthToken`, or `OCMClientReadyMsg`.
- `TestSetupOCM_SingleCallSite` — `setupOCM` exists in root.go and is the
  single `ocm.CheckTokens` call site.

`pkg/tui/config_ocm_handoff_test.go` (mock OCM client via injectable
`ocmConnect`):
- connect cmd produced when no client; nil when connected / dev mode; error
  surfaces as `OCMClientReadyMsg.Err`
- `configSavedMsg` success queues the connect (`ocmAuthPending` set); save
  error and already-connected cases do not
- wizard abort without a PD config performs no OCM handoff
