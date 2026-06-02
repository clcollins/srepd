# Skip plan-doc and README checks for dependabot PRs

## Context

Dependabot dependency bump PRs fail CI because they don't include
plan documents or README updates. These checks are meant for human
code changes, not automated dependency bumps.

## Changes

- Add `&& github.actor != 'dependabot[bot]'` to plan-doc and
  readme-check job conditions in go-ci.yml
