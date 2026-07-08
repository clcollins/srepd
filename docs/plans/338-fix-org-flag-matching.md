# 338: Fix org flag matching and add OrganizationID

## Problem

The `:flag org <pattern>` command never matched any incidents because
`clusterFromResponse` stored `cluster.Subscription().ID()` (a subscription
UUID) into `info.Organization`. The flag system tried to glob-match a
customer name like `"Camunda*"` against that UUID, which never matched.

Additionally, the Cluster tab displayed this subscription UUID as the
"Organization" field, which was not useful.

## Approach

1. **Add `OrganizationID` field** to `ClusterInfo` to store the raw org ID
   separately from the human-readable name.

2. **Fix org resolution in `GetCluster`** — remove the incorrect
   `cluster.Subscription().ID()` assignment from `clusterFromResponse`.
   Instead, extract the org ID from the accounts management subscription
   response (which was already being queried), then fetch the org name via
   `AccountsMgmt().V1().Organizations().Organization(orgID).Get()`. This is
   best-effort: if the lookup fails, fields stay empty.

3. **Match flags against both fields** — update `matchOrgName` to try
   matching the pattern against both `Organization` (name) and
   `OrganizationID`. This means `:flag org Camunda*` matches the org name
   while `:flag org 1a2b3c4d` matches the org ID directly.

4. **Display both fields** in the Cluster tab.

## Key decisions

- Org name resolution is best-effort and non-blocking to avoid slowing
  cluster enrichment for a secondary API call.
- The `:flag org` command transparently matches either field — no separate
  `:flag orgid` command needed.
- `clusterFromResponse` remains a pure mapping function; org resolution
  happens in `GetCluster` where we have the subscription context.

## Files changed

| File | Change |
|------|--------|
| `pkg/ocm/ocm.go` | Add `OrganizationID` field |
| `pkg/ocm/client.go` | Fix org resolution, add `enrichOrganization` helper |
| `pkg/tui/flags.go` | Match against both Organization and OrganizationID |
| `pkg/tui/views.go` | Display Organization ID in Cluster tab |
| `pkg/ocm/fixtures.go` | Add OrganizationID to fixture struct |
| `pkg/ocm/client_test.go` | Update clusterFromResponse tests |
| `pkg/ocm/ocm_test.go` | Add OrganizationID to test fixture |
| `pkg/tui/flags_test.go` | Add test cases for org ID matching |
| `testdata/fixtures/clusters.json` | Add organization_id to fixture data |

## Testing

- All existing tests pass
- Three new flag test cases: org ID match, org ID exact match without name,
  org name match when ID doesn't match
- `make test-all` passes (fmt, vet, lint, test, race)
