# Plan 103: cmd/ correctness — home-dir errors and allowlist token masking

**Branch:** `srepd/cmd-correctness`

## Problem

1. **Ignored `os.UserHomeDir()` errors** — `cmd/root.go` `PreRun` and `Run` used
   `home, _ := os.UserHomeDir()`. On failure `home` is `""`, so `configFile` became a
   relative `.config/srepd/srepd.yaml` resolved against CWD, silently mis-detecting
   whether the config exists.
2. **Substring token masking** — `validateConfig` masked a key's value under `--debug`
   only when the key name contained the literal substring `"token"`. Any secret-bearing
   key not named `*token*` (`api_key`, `secret`, `password`, a future OCM/webhook key)
   was logged in cleartext.

## Solution

1. Added `resolveConfigFilePath(homeDir func() (string, error)) (string, error)` that
   surfaces the home-dir error. Both `PreRun` and `Run` call it and `log.Fatal` on
   error instead of proceeding with a bogus path. `homeDir` is injectable for tests;
   production passes `os.UserHomeDir`.
2. Replaced the substring check with `maskConfigValue(key, value)` backed by an
   **allowlist** (`safeToLogConfigKeys`) of known non-secret keys. Anything not on the
   allowlist is masked — secret-by-default.

## Files Modified

- `cmd/root.go` — `resolveConfigFilePath`; both closures use it and handle the error.
- `cmd/config.go` — `safeToLogConfigKeys`, `maskConfigValue`; `validateConfig` uses it.
- `cmd/root_test.go` — `TestResolveConfigFilePath`.
- `cmd/config_test.go` — `TestMaskConfigValue`.

## Tests (TDD)

Written first, seen red, then green:
- `TestResolveConfigFilePath` — joins on success; surfaces the home-dir error.
- `TestMaskConfigValue` — masks `token`/`api_key`/`secret`/`password`/unknown keys;
  reveals allowlisted keys (`teams`, `editor`, `terminal`).

## Verification

`make test-all` green (fmt, vet, lint, test, race, test-fixtures).

## Lessons Learned

- Ignoring `os.UserHomeDir()`'s error turns a missing HOME into a wrong-but-plausible
  relative path — surface it.
- Secret masking must be an allowlist (safe-by-name), not a denylist substring; a
  `token`-substring check silently leaks every other secret-bearing key.
