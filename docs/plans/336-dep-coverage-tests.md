# Plan: Add test coverage for Dependabot dependency touch points (#336)

**Status:** Complete
**PR:** #336

## Problem

Three open Dependabot PRs update dependencies (glamour/v2, ocm-sdk-go,
ocm-common) but the functions that directly use these dependencies lack
unit test coverage. Without tests exercising these touch points, a
breaking change in a dependency bump could go undetected.

## Solution

Add targeted unit tests for the functions that directly consume these
dependencies:

### glamour/v2 — `buildGlamourStyle`

`pkg/tui/theme.go:buildGlamourStyle` converts a Theme into a glamour
`ansi.StyleConfig`. It was the only glamour-consuming function without
tests. New tests verify all color mappings, bold flags, and background
clearing.

### ocm-sdk-go — `clusterFromResponse`

`pkg/ocm/client.go:clusterFromResponse` converts an OCM SDK
`cmv1.Cluster` into a local `ClusterInfo`. Tests use the SDK's builder
API (`cmv1.NewCluster()`) to construct realistic cluster objects and
verify field mapping, display name logic (DomainPrefix + DNS), and
nil-safety for optional fields (Region, Hypershift, CCS, Subscription).

### ocm-common

Already well-tested via `CheckTokens`, `ApplyAuthToken`, and config
loading tests. The `clusterFromResponse` tests indirectly exercise the
types that flow through the ocm-common connection builder.
