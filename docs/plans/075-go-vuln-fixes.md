# Fix Go vulnerability scan failures

## Context

govulncheck reports vulnerabilities in golang.org/x/net (indirect
dependency) affecting textproto, x509, and http2 code paths.
Go 1.26.3 is already the latest — fix is updating x/net.

## Changes

- Update golang.org/x/net from v0.39.0 to latest
- Run go mod tidy
- Verify govulncheck passes

## Verification

- govulncheck ./... reports no vulnerabilities
- make test-all passes
