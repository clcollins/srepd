# Fix dev mode fixture display differences

## Context

Dev mode fixtures are missing fields that real PagerDuty API
responses include, causing the incident viewer tabs to render
differently in dev mode vs production.

## Changes

- Add html_url to fixtureAlert struct and convertFixtureAlert
- Add html_url to all alert entries in alerts.json fixtures
- Add incident reference to fixture alerts where missing

## Verification

- make test-all passes
- Dev mode incident viewer renders alerts with clickable links
