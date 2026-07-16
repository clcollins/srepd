# 402: Fix deprecated goreleaser archive format options

## Problem

The v1.6.3 release run succeeded but ended with:

```
you are using deprecated options, check the output above for details
```

The warnings were:

```
DEPRECATED: archives.format should not be used anymore
DEPRECATED: archives.format_overrides.format should not be used anymore
```

GoReleaser v2 renamed the archive `format` key to `formats`, which takes a
list rather than a single string. The old key still works today, but it is
scheduled for removal — a future goreleaser upgrade would break `make
release` at exactly the moment we want to cut a release, which is the worst
time to discover a broken config.

## Approach

A pure rename in `.goreleaser.yaml`, in the two places goreleaser flagged:

- `archives[0].format: tar.gz` → `archives[0].formats: [tar.gz]`
- `archives[0].format_overrides[0].format: zip` → `formats: [zip]`

Nothing else changes. `name_template` is untouched, so the published asset
filenames stay byte-identical (`srepd_Linux_x86_64.tar.gz`,
`srepd_Darwin_arm64.tar.gz`, etc.). This matters: a rename that silently
altered asset names would break any download scripts pinned to those URLs.

## Verification

- `goreleaser check` — validates the config and emits no deprecation
  warnings (previously two).
- `make build` — snapshot build succeeds with the new keys.

Both were run before commit; asset naming was confirmed unchanged against
the live v1.6.3 release assets.

## Notes / lessons

- `.goreleaser.yaml` is not in `make readme-check`'s watch list, so no
  README update is required for this change.
- **Do not use `make release` to inspect config warnings.** While chasing
  these warnings, `make release` was re-run purely to capture its output —
  but that target publishes to GitHub, and it re-invoked a publish against
  the already-released v1.6.3 tag. No damage resulted (goreleaser re-uploaded
  identical assets to the existing release; `publishedAt` was unchanged), but
  it was an unrequested publish. Use `goreleaser check` for config
  validation — it parses without publishing.
