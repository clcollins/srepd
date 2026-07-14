# 383: Onboarding docs and headers (OB-2 + OB-7)

Branch: `srepd/onboarding-docs-v2`

## Changes

- README: Getting Started section (two steps: create token, run srepd),
  link to presets doc; YAML example demoted to "Advanced: manual
  configuration" with config generate and placeholder warning; terminal
  default corrected to `gnome-terminal --`; dead flag_marker row removed;
  agent_cli_command documents `""` = disabled.
- configCmd long description: token acquisition path + discovery summary.
- Copyright headers fixed in cmd/config.go and pkg/config/config.go.
