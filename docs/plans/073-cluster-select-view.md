# Refactor cluster selection to scrollable table view

## Context

When logging into a multi-cluster incident, the cluster selection
prompt in the status bar gets overwritten by async API responses,
leaving the keyboard locked with no visible instructions.

## Changes

- Replace status bar prompt with scrollable table view (same
  pattern as merge mode)
- Add clusterSelectTable and clusterSelectPrompt to model
- Enter selects, Escape cancels, Up/Down navigates
- Remove digit-key (1-9) handling and handleClusterSelectInput
- Remove cluster select rendering from renderHeader()

## Verification

- make test-all passes
- Multi-cluster incident login shows scrollable cluster list
- Async status updates don't interfere with cluster selection
