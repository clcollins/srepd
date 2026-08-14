# Plan 421: Bridge Release v1.6.5 — openshift-online/srepd → openshift-online/srepd Migration

## Problem

srepd is moving to `github.com/openshift-online/srepd`. Because this is a
fresh commit to a new repo (not a GitHub repository transfer), GitHub sets up
no redirects of any kind. Every srepd binary in the field (v1.6.4 and earlier)
has this URL compiled in:

```
https://api.github.com/repos/openshift-online/srepd/releases/latest
```

(`githubReleasesURL` constant, `pkg/tui/update.go`). Without a bridge, existing
users will never learn about releases in the new repo: the startup update banner
fails silently (debug-level log only) and `srepd update` reports "already up to
date" forever.

## Approach

Publish one final release in the old repo (`openshift-online/srepd`) whose only
functional change is pointing `githubReleasesURL` at `openshift-online/srepd`.
Old binaries see v1.6.5 as an update, self-update to it from this repo's release
assets, and from then on check `openshift-online/srepd` — where they'll find
v1.7.0+ and complete the migration.

## Changes

**Functional (the only real change):**
- `pkg/tui/update.go`: `githubReleasesURL` → `openshift-online/srepd`

**Cosmetic (consistency):**
- `pkg/tui/update.go`: dev-mode fake release URL → `openshift-online/srepd`
- `cmd/update.go`: help text URL → `openshift-online/srepd`

**Documentation:**
- `README.md`: prominent `[!IMPORTANT]` migration notice at the very top

**Explicitly NOT changed:**
- `go.mod` module path or any import paths — repo keeps `github.com/openshift-online/srepd`
- `.goreleaser.yaml` — `release.github.owner` stays `clcollins` so the release
  publishes to this repo where old binaries are looking
- `pkg/tui/update_test.go` — fixture URLs are self-contained httptest strings
  that don't reference the constant; tests pass unchanged

## Ordering / Prerequisites

- **`openshift-online/srepd` must rename its `go.mod` module path** from
  `github.com/openshift-online/srepd` to `github.com/openshift-online/srepd` (and
  update all internal import paths) before publishing v1.7.0. Without this,
  `go install github.com/openshift-online/srepd@latest` (advertised in the
  bridge README notice) fails with a module path mismatch error for new users.
- openshift-online/srepd must have a published release (v1.7.0) before or
  promptly after v1.6.5 ships. Until then, the startup banner fails silently
  (harmless) but `srepd update` prints a 404 error for anyone who runs it.
- Version must be lower than the new repo's first release (v1.6.5 < v1.7.0)
  so `isNewerVersion` immediately offers the next hop.

## Release Procedure

1. PR and merge to `main` (CI is disabled; run `make test-all` locally first)
2. `git tag v1.6.5 && git push origin v1.6.5`
3. `make release` (goreleaser; reads GITHUB_TOKEN from
   `~/.config/goreleaser/goreleaser_token`)
4. After confirming migration works end-to-end, **archive** the repository
   (Settings → Archive). Never delete — archived repos keep serving the
   releases API and asset downloads, so stragglers who run `srepd update`
   months later still get bridged.

## Verification

- `curl -s https://api.github.com/repos/openshift-online/srepd/releases/latest | jq .tag_name` → `"v1.6.5"`
- Download a v1.6.4 binary, run `srepd update` → updates to v1.6.5
- Run the v1.6.5 binary's `srepd update` → reaches openshift-online/srepd
- `strings dist/.../srepd | grep releases/latest` → shows only the openshift-online URL
