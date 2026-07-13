# 367: Teams UX — greeting, smart preselection, advanced gate

Issue: #353/#324 (onboarding overhaul — token-first wizard phase)
Branch: `srepd/teams-ux-greeting`

## Problem

Three friction points in the wizard's team step and beyond:
1. After pasting a token, the user got no confirmation of *who* PagerDuty
   thinks they are — just a team list.
2. A new user on exactly one team (the common SRE case) still had to
   manually select it; #324 asks the wizard to steer toward one team.
3. `custom_service_escalation_policies` — a "have to know to know" team
   policy — was prompted at every new user, even though most should never
   touch it.

## Approach

- **Greeting**: `pd.GetCurrentUserWithTeams` returns the full user (one API
  call for identity + teams; `GetCurrentUserTeams` now delegates to it).
  `fetchTeamOptions` returns the user's name; the OptionsFunc stores
  `FetchedUserName`/`FetchedTeamCount` on `configFormState`, and the team
  step's `DescriptionFunc` — keyed on `&FetchedUserName` so it re-fires
  right after the fetch completes — renders `teamGreeting(name, count)`:
  "Hi Chris — found 2 teams on your PagerDuty profile. …most users need
  exactly one." Neutral copy until a user resolves.
- **Preselection**: `teamPreselection(existingTeamSet, teams)` — existing
  config always wins; else a single fetched team is preselected; multiple
  teams preselect nothing (deliberate choice, guided by copy). Kept
  MultiSelect (per user decision) — no migration story for multi-team
  configs, no second code path.
- **Advanced gate**: new "Configure advanced options?" confirm (default No),
  shown only to users with no existing custom mappings; the custom-mappings
  input hides unless it's opened (or the user already has mappings, where
  the keep/edit flow is unchanged). Copy points at team onboarding docs /
  the upcoming preset mechanism.
- Mock user now carries `Name: "Mock User"`.

Full group reorder (welcome first, environment group) lands with PRs 7/10.

## Tests (TDD — written first)

`pkg/tui/config_teams_ux_test.go`: `teamGreeting` (plural/singular/fallback),
`fetchTeamOptions` returns the user name, `teamPreselection` table (existing
wins / single auto / nil set / multiple none). Updated PR-366 tests to the
3-value `fetchTeamOptions` signature.
