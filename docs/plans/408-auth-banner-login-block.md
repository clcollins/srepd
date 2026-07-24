# 408: Flashing OCM auth banner and login block

## Problem

When srepd starts with expired OCM tokens, it binds port 9998 via the OCM
SDK's `auth.InitiateAuthCode()` for browser-based OAuth2 PKCE authentication.
When the user presses `l` to login while this auth server is still running,
srepd launches `ocm-container` which also calls `InitiateAuthCode()` on
port 9998, causing a fatal `bind: address already in use` crash.

Users also don't notice the browser auth prompt when it opens — there's no
visible indication in srepd that they need to switch to their browser.

Investigation confirmed:
- Port 9998 is hardcoded in the upstream OCM SDK (`RedirectPort = "9998"`)
- `ocm backplane login` does NOT trigger port 9998 (uses existing tokens)
- Only `ocm-container` auto-triggers `InitiateAuthCode()` on every launch
- Regular SDK token refresh does NOT bind port 9998 — only browser auth does
- The existing `ocmAuthPending` field already tracks exactly this state

## Approach

Two coordinated features:

1. **Color-cycling auth banner** in the bottom status bar — when
   `ocmAuthPending` is true, show `>>> Please complete OCM browser auth <<<`
   with a background color that cycles through 6 red/crimson shades every
   500ms via a `tea.Tick` loop. Uses the same pattern as the typewriter
   watcher animation.

2. **Login block** at all 5 login entry points — prevents launching any
   cluster login while auth is pending, with a flash notification explaining
   why. Applies universally regardless of `cluster_login_command` since
   srepd's own auth server is the source of the conflict.

## Changes

### Model (`model.go`)
- Added `authBannerPhase int` field near `ocmAuthPending` — cycles 0-5 for
  the color animation.

### Commands (`commands.go`)
- Added `authBannerTickMsg` type and `authBannerFlashInterval` (500ms) constant.
- Added `startAuthBannerTick()` helper method.
- Modified `ocmHandoffCmd()` to batch the tick start when setting
  `ocmAuthPending = true`.

### Update loop (`tui.go`)
- Added `authBannerTickMsg` handler — cycles phase when auth is pending,
  self-terminates when auth completes.
- Added tick start in `Init()` when `ocmAuthPending` is true at startup.
- Added `authBannerPhase = 0` reset in `OCMClientReadyMsg` handler.
- Added auth guard in `loginMsg` and `rosaBoundaryLoginMsg` handlers.

### Views (`views.go`)
- Added `authBannerColors` slice (6 red/crimson shades).
- Modified `renderBottomStatus()` — auth banner takes priority over the
  update banner when `ocmAuthPending` is true.

### Key handlers (`msgHandlers.go`)
- Added auth guard in table-view and incident-view login key handlers.

### Chords (`chords.go`)
- Added auth guard in `chordRosaBoundaryLogin()`.

### Tests (`auth_banner_test.go`)
New test file with 10 test functions covering:
- Tick phase cycling and wrapping
- Tick self-termination when auth completes
- `OCMClientReadyMsg` clearing banner phase
- Banner rendering present/absent
- Login blocked at all 5 entry points

## Verification

- `make test-all` passes (fmt, vet, lint, unit tests, race detection)
- Golden snapshots unaffected (test models don't set `ocmAuthPending`)
- Visual validation confirmed with test binary
