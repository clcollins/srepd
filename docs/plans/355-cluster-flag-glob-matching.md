# 355: Enable glob matching for cluster flags

## Problem

`:flag cluster lk3-dev` did nothing when the cluster's display name was
`lk3-dev-use2-a.vbah.p1.openshiftapps.com`. The `matchClusterID` function
only did exact equality checks against the raw cluster ID, internal OCM ID,
and external ID. It did not check the cluster name or display name, and it
did not support partial/glob matching.

SREs commonly use partial cluster names or substrings from the PD service
name (e.g., `osd-lk3-dev`) to identify clusters, so the flag system needs
to support this.

## Fix

Changed `matchClusterID` to:
1. Use `matchGlob` instead of exact equality for all fields (raw ID,
   internal ID, external ID)
2. Also match against `info.Name` and `info.DisplayName` from the cluster
   cache

Since `matchGlob` treats a plain pattern as a contains match, `:flag cluster
lk3-dev` now matches any cluster whose name, display name, or ID contains
`"lk3-dev"`. Glob patterns (`*`, `^`, `$`) also work.

Updated the flag label from "cluster ID matches" to "cluster matches" to
reflect the broader matching.

## Files changed

| File | Change |
|------|--------|
| `pkg/tui/flags.go` | Rewrite `matchClusterID` to use `matchGlob` and check Name/DisplayName |
| `pkg/tui/flag_commands.go` | Update label text |
| `pkg/tui/flags_test.go` | Add 4 new test cases, update label references |
