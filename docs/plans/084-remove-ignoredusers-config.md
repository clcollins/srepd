# 084: Remove ignoredusers from config template and help

Follow-up to #276 (083-replace-ignoredusers).

## Problem

After #276 added auto-discovery of ignored users from silent escalation
policies, the `config --create` template, optional keys help text, and
README config table still referenced `ignoredusers`.

## Changes

- Remove `ignoredusers` from example config in `cmd/config.go`
- Remove `ignoredusers` from `optionalKeys` help map in `cmd/config.go`
- Remove `ignoredusers` row from README config table

The key is still accepted at runtime via the deprecation path in #276.
