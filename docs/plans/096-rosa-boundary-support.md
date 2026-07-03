# 096: rosa-boundary Cluster Login Support

## Problem

SREPD currently supports a single cluster login method via `cluster_login_command`
(default: `ocm backplane login`). rosa-boundary is a new CLI for launching ephemeral
SRE investigation containers on AWS Fargate via ECS Exec, and SREs need to use it
alongside the existing login flow.

## Approach

Add rosa-boundary as a second, parallel login method accessible via the chord
`ctrl+x b`. The rosa-boundary command is standardized across all SREs, so the
default value is written into the config file on first setup.

- New config key `rosa_boundary_command` with default
  `rosa-boundary start-task --cluster-id %%CLUSTER_ID%% --connect`
- New chord `ctrl+x b` triggers rosa-boundary login
- Second `ClusterLauncher` stored on the model, validated at startup
- Simplified login function that skips PagerDuty env var injection
  (Fargate containers have their own environment)
- Multi-cluster incidents reuse the existing cluster selector UI

## Files Changed

- `pkg/config/config.go` - New config key in DefaultOptionalKeys, OptionalKeys,
  BuildFullConfig()
- `pkg/tui/model.go` - rosaBoundaryLauncher and rosaBoundaryClusterSelect fields;
  updated InitialModel/InitialModelWithConfig signatures
- `pkg/tui/chords.go` - Registered chord "b" with chordRosaBoundaryLogin handler
- `pkg/tui/commands.go` - rosaBoundaryLoginMsg, rosaBoundaryClusterSelectedMsg types;
  rosaBoundaryLogin() function
- `pkg/tui/tui.go` - Update handlers for both new message types
- `pkg/tui/msgHandlers.go` - Cluster select dispatch checks rosaBoundaryClusterSelect
- `cmd/root.go` - Creates second launcher, passes to InitialModel
- `cmd/config.go` - Passes zero-value launcher for config mode
- `README.md` - Documents new config key and chord

## Testing

- Config: TestDefaultOptionalKeys_RosaBoundaryCommand, updated BuildFullConfig and
  EndToEnd tests
- Chords: TestChordRosaBoundaryLogin_Registered, _LauncherDisabled, _LauncherEnabled
- All existing tests updated for new InitialModel signature

## Post-Mortem / Lessons Learned

(To be filled after merge)
