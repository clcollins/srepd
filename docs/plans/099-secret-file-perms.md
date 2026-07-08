# Plan 099: Enforce owner-only permissions on token-bearing config files

**Branch:** `srepd/secret-file-perms`

## Problem

The srepd config file (`~/.config/srepd/srepd.yaml`) contains the plaintext
PagerDuty API token, but every write path created it world-readable (`0644`), along
with its `~` backup, and the config directory was `0755`. On a shared host any local
user could read the token.

## Solution

Write all token-bearing files `0600` and the config directory `0700`.

### Key subtlety: `os.WriteFile` mode is ignored on existing files

`os.WriteFile(path, data, 0600)` only applies the mode when **creating** a file — on
an existing file it truncates and keeps the old permissions. Since the config file
usually already exists (it is read before every rewrite), simply changing the mode
argument would **not** secure an already-`0644` file. The fix therefore performs an
explicit `Chmod` after each write.

- Added `Chmod(name string, mode os.FileMode) error` to the `ConfigFS` interface and
  its `realFS` implementation (`pkg/tui/commands.go`).
- Added a `writeSecretFile(fs, name, data)` helper in `pkg/config/config.go` that
  writes `0600` and then `Chmod`s to `0600`. All nine config/backup write sites in
  `WriteConfig`, `WriteConfigTeams`, `WriteConfigKey`, `WriteConfigMap` now use it.
- `pkg/tui/commands.go`:
  - `writeTeamsToConfigCmd` (raw `os.WriteFile` path): `0600` + explicit `os.Chmod`
    for both the config file and backup.
  - `writeConfigCmd`: config dir `MkdirAll` `0755`→`0700`.
- `cmd/root.go` auto-migration `os.WriteFile(configFile, ...)`: `0644`→`0600`.

## Files Modified

- `pkg/config/config.go` — `ConfigFS.Chmod`, `secretFileMode`, `writeSecretFile`,
  nine write sites.
- `pkg/config/config_test.go` — `mockFS.Chmod` + perm-recording fields; five 0600 tests.
- `pkg/tui/commands.go` — `realFS.Chmod`, teams-write chmod, dir `0700`.
- `pkg/tui/config_mode_test.go` — `tuiMockFS.Chmod` + perm recording; 0700/0600 test.
- `cmd/root.go` — auto-migration write `0600`.

## Tests (TDD)

Written first, seen red (perms were `0644`/`0755`), then green:
- `TestWriteConfig_NewFile_Uses0600`, `TestWriteConfig_ExistingFileAndBackup_Use0600`,
  `TestWriteConfigTeams_Use0600`, `TestWriteConfigKey_Use0600`,
  `TestWriteConfigMap_Use0600` — assert the effective `Chmod` mode is `0600` for both
  the config file and its backup.
- `TestWriteConfigCmd_UsesOwnerOnlyPerms` — config dir created `0700`, config file `0600`.

The mocks record the `Chmod` mode, so the tests assert the *effective* permission
(the real enforcement), not just the `WriteFile` argument.

## Deferred (noted, not in scope)

`writeTeamsToConfigCmd` calls `os.UserHomeDir`/`os.ReadFile`/`os.WriteFile` directly
rather than through the `ConfigFS` seam, so it has no isolated unit test (only the
perm change is applied here). Refactoring it to `ConfigFS` for testability is a
separate cleanup.

## Verification

`make test-all` green. Manual: after saving config,
`stat ~/.config/srepd/srepd.yaml` shows `-rw-------` and the dir `drwx------`;
re-saving an already-`0644` file tightens it to `0600` (verified the Chmod path).

## Lessons Learned

- `os.WriteFile`/`os.OpenFile` apply their mode only on file **creation**. Securing an
  existing secret file requires an explicit `os.Chmod` — a mode-argument change alone
  is a silent no-op on the common (already-exists) path.
- Test the *effective* permission (post-Chmod), not the WriteFile argument, or the
  test passes while the real file stays world-readable.
