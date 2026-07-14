# 374: Supply-chain hardening — SHA-pinned actions, dependency rules

Prompted by: "Operation Muck and Load" (fake Go DNS scanner
`github.com/kaleidora/dnsub-scanning-tool` typosquatting `dnsub`; 700+
malicious versions across 222 GitHub lure repos; Socket research).
Branch: `srepd/supply-chain-hardening`

## Audit result (context)

srepd is clean: no IOC matches in go.mod/go.sum, the onboarding PR stack
(#363–#374) added zero dependencies, all direct deps are org-backed
well-known modules, go.sum + the Go checksum DB give tamper-evidence, and
CI runs govulncheck. govulncheck does NOT catch typosquat malware, though —
that class is only defended at dependency-add time, which motivates the
rules below.

## Changes

1. **GitHub Actions pinned to full commit SHAs** (the same style as the
   already-pinned codecov action), so a hijacked or force-moved tag cannot
   swap code into CI:
   - `actions/checkout@v6` → `df4cb1c069e1874edd31b4311f1884172cec0e10 # v6` (×12)
   - `actions/cache@v5` → `caa296126883cff596d87d8935842f9db880ef25 # v5` (×3)
   - `actions/setup-go@v6` → `924ae3a1cded613372ab5595356fb5720e22ba16 # v6`
     (inside the local `setup-go-from-mod` composite action)
   SHAs resolved from the official repos via `gh api repos/<owner>/<repo>/commits/<tag>`.
2. **dependabot `github-actions` ecosystem** (weekly) added so the SHA pins
   are maintained automatically (dependabot bumps the SHA and keeps the
   `# vN` comment).
3. **AGENTS.md "Dependencies (supply-chain rules)" section**: new go.mod
   modules require explicit human provenance review (exact-path typosquat
   check, owner/history/version-volume scrutiny); AI agents must call out
   dependency additions prominently in PR descriptions; prefer stdlib or
   existing deps; keep the SHA-pin style for any new action.

## Verification

- `grep -r "uses:" .github/` shows only SHA-pinned third-party actions and
  the local composite action.
- Full local CI suite green (no Go code changed; suite run regardless).
- Remote CI green on the PR — the workflow parses and the pinned actions
  resolve.
