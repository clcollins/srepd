# How SREPD Discovers Teams, Policies, and Which Incidents to Show

## The Problem

When an SRE opens srepd, they want to see incidents they need to act on.
They don't want to see bot-triaged noise, silent test alerts, or
incidents parked on placeholder users. Today, getting this right requires
manually configuring team IDs, escalation policy IDs, and ignored user
IDs — all looked up by hand from the PagerDuty web UI.

This document explains how srepd can figure all of that out automatically
from just a PagerDuty API token and the user's team membership.

## How PagerDuty Organizes Things

PagerDuty has a hierarchy that matters for understanding incident routing:

```
You (a PagerDuty user)
  └── belong to Teams (e.g., "Platform SRE")
        └── which own Services (one per alert source — each cluster gets one)
              └── each Service has one Escalation Policy
                    └── which has ordered Rules (levels 1, 2, 3, ...)
                          └── each Rule targets either:
                                • A User (user_reference) — a specific person or bot
                                • A Schedule (schedule_reference) — an on-call rotation
```

When an alert fires on a Service, PagerDuty creates an incident and
routes it through that Service's Escalation Policy, starting at level 1.
If nobody acknowledges it, PagerDuty escalates to level 2, then 3, etc.

## Two Kinds of Escalation Policies

This is the key insight that makes auto-discovery work.

### Real Policies (incidents eventually reach humans)

Real escalation policies have **on-call schedules** as targets. When an
incident reaches a schedule target, PagerDuty pages whoever is currently
on-call for that rotation.

Example: **OpenShift Escalation Policy**
```
Level 1: Nobody SREP (user)      ← bot placeholder, catches new alerts
Level 2: Weekday Primary (schedule) ← real human gets paged
Level 3: Weekday Secondary (schedule)
Level 4: Weekend Oncall (schedule)
Level 5: Management (schedule)
```

Level 1 is a bot user. CAD (the cluster automation tool) picks up the
alert, does triage, and if it needs human attention, re-escalates to
level 2. If nobody responds, PagerDuty automatically walks up the levels.

### Silent Policies (incidents never reach humans)

Silent escalation policies have **only user targets** — no schedules at
all. They exist to "park" alerts that are non-actionable, like Dead Man's
Snitch heartbeats or synthetic monitoring noise.

Example: **Silent Test - Non-Actionable**
```
Level 1: Nobody SREP (user)   ← bot
Level 2: Silent Test (user)   ← another bot
```

No schedules anywhere. These incidents will never page a real person.
They exist for tracking and compliance, not for SRE action.

## The Classification Heuristic

To classify a policy automatically:

```
If ANY target across ANY rule is a schedule_reference → REAL policy
If ALL targets are user_reference (no schedules)      → SILENT policy
```

This was validated against real Platform SRE data and correctly classified
all five policies found in production.

## What SREPD Discovers Automatically

Starting from just the user's teams:

### Step 1: Find your services

For each team you selected, fetch all PagerDuty Services associated with
that team. Each service has exactly one escalation policy.

### Step 2: Collect unique policies

Many services share the same escalation policy (e.g., most clusters use
the same "OpenShift Escalation Policy"). Collect the unique set.

### Step 3: Classify each policy

Fetch the full details of each unique policy and apply the heuristic:
does it have schedules (REAL) or only users (SILENT)?

### Step 4: Pick the defaults

- **DEFAULT**: The REAL policy associated with the most services. This is
  almost always "OpenShift Escalation Policy" — the one with on-call
  schedules that pages actual SREs.

- **SILENT_DEFAULT**: The SILENT policy used for general non-actionable
  alerts. Used when silencing incidents (re-routing them to the bot policy
  so they stop paging).

### Step 5: Map service overrides

Any service whose escalation policy differs from DEFAULT gets a specific
mapping. For example, Dead Man's Snitch alerts come from a specific PD
service and route to "DMS Silent Test" — a silent policy. This mapping
tells srepd how to re-escalate DMS incidents differently from regular ones.

### Step 6: Extract ignored users

The union of all user_reference targets from SILENT policies gives us the
list of bot/placeholder users. These are the users whose assigned
incidents should be hidden in the individual view.

## What This Replaces in the Config

### Before (manual config)

```yaml
teams:
  - PASPK4G  # looked up from PD web UI

service_escalation_policies:
  DEFAULT: PA4586M         # looked up from PD web UI
  SILENT_DEFAULT: PCGXUDY  # looked up from PD web UI
  P5LAB5Y: PVBANNN        # looked up from PD web UI

ignoredusers:
  - P53J4TK  # looked up from PD web UI
  - P8QS6CC  # looked up from PD web UI
```

### After (auto-discovered)

```yaml
teams:
  - PASPK4G  # Platform SRE (selected interactively)

# service_escalation_policies and ignoredusers are auto-discovered
# from the PD API based on team membership. You can still set them
# manually to override auto-discovery.
```

## Escalation Level Filtering

Even with auto-discovery, the DEFAULT policy routes new incidents to a
bot user at level 1 before a human at level 2+. By default, srepd shows
only incidents at escalation level 2 or higher — these are the ones that
have been triaged and need human attention.

A keyboard shortcut (`ctrl+x e`) cycles through the minimum escalation
level:

```
Level 2+ (default) → Level 3+ → Level 4+ → All levels → Level 2+
```

This lets you focus on what needs your attention right now (level 2+) or
broaden the view to see everything including bot-triaged incidents (all
levels). The current filter level shows in the status bar.

## Backward Compatibility

If you have an existing config with `service_escalation_policies` and/or
`ignoredusers`, they will continue to work. Auto-discovery only kicks in
when those keys are absent. When `ignoredusers` is present, srepd logs a
deprecation notice suggesting you remove it.
