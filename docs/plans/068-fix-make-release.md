# Fix make release target

## Context

goreleaser builds 4 platform binaries in parallel, which exhausts
process limits in the Fedora Toolbox container. Also, there was no
way to pass custom release notes to the release target.

## Changes

- Add --parallelism 1 to goreleaser release command
- Add optional RELEASE_NOTES variable for custom changelog

## Verification

- make release builds sequentially without resource exhaustion
- make release RELEASE_NOTES=/path/to/notes.md passes notes through
