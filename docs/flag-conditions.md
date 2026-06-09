# Flag Conditions

Flag conditions let you visually mark incidents that match specific criteria.
Flagged incidents show a configurable marker prefix on their Summary in the
incident table, and a "Flag Conditions" section in the Details tab explaining
which flags matched.

## Quick Start

Press `f` to open the flag input prompt (pre-filled with `/flag `), or enter
input mode with `:` and type the command manually.

```
/flag cluster 1q2w3e4rfakeidtest9o0p1a2s3d4f5g
/flag org ^Acme*
```

## Commands

| Command | Description |
|---------|-------------|
| `/flag cluster <id>` | Flag incidents involving a specific cluster (matches OCM internal ID, external ID, or raw alert cluster ID) |
| `/flag org <pattern>` | Flag incidents involving clusters owned by an organization matching the pattern |
| `/flags` | List all active flag conditions |
| `/unflag <id>` | Remove a flag condition by its session ID |
| `/unflag all` | Clear all flag conditions |
| `/flags save [path]` | Save flag conditions to a JSON file |
| `/flags load [path]` | Load flag conditions from a JSON file |

## Condition Types

### Cluster ID (`/flag cluster <id>`)

Matches any incident whose alerts reference the given cluster. The ID is
compared against:

1. The raw `cluster_id` field from alert details
2. The OCM internal cluster ID (from `ClusterInfo.ID`)
3. The OCM external cluster ID/UUID (from `ClusterInfo.ExternalID`)

Matching is case-insensitive. If OCM enrichment has not yet completed for a
cluster, only the raw alert cluster ID is checked.

### Organization Name (`/flag org <pattern>`)

Matches incidents involving clusters whose `Organization` field (from OCM)
matches the pattern. Requires OCM enrichment to be active.

**Supported glob patterns:**

| Pattern | Meaning | Example |
|---------|---------|---------|
| `STRING` | Contains (case-insensitive) | `Acme` matches "Acme Corp" and "Big Acme Ltd" |
| `^STRING` | Starts with | `^Acme` matches "Acme Corp" but not "Big Acme" |
| `STRING$` | Ends with | `Corp$` matches "Acme Corp" but not "Corp Inc" |
| `STRING*` | Starts with (trailing wildcard) | `Acme*` same as `^Acme` |
| `^STRING*` | Starts with (both markers) | `^Acme*` same as `^Acme` |
| `*STRING$` | Ends with (leading wildcard) | `*Corp$` same as `Corp$` |

## Display

### Table View

Flagged incidents have the flag marker prepended to their Summary column:

```
  • | P1234567 | 🚩 ClusterMonitoringError...  | mycluster.example.org
  A | P7654321 | KubePersistentVolumeFilling... | other-cluster.example.org
```

### Details Tab

When viewing a flagged incident, the Details tab includes a "Flag Conditions"
section at the bottom:

```
## Flag Conditions

* 🚩 #1: cluster ID matches "1q2w3e4rfakeidtest9o0p1a2s3d4f5g"
* 🚩 #2: org name matches "^Acme*"
```

## Configuration

### `flag_marker` (optional)

The prefix marker shown on flagged incidents. Default: `🚩 ` (flag emoji +
space).

For terminals that don't render emoji well, set to `|►` (no trailing space
needed):

```yaml
flag_marker: "|►"
```

Add this to your `~/.config/srepd/srepd.yaml`.

## Persistence

Flag conditions are **session-only by default** — they are lost when srepd
exits. To persist them:

```
/flags save              # saves to ~/.config/srepd/flags.json
/flags save /tmp/my.json # saves to a custom path
/flags load              # loads from ~/.config/srepd/flags.json
/flags load /tmp/my.json # loads from a custom path
```

### Save File Format (`flags.json`)

The save file is a JSON array of flag condition objects:

```json
[
  {
    "id": 1,
    "type": 0,
    "pattern": "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
    "label": "cluster ID matches \"1q2w3e4rfakeidtest9o0p1a2s3d4f5g\"",
    "created_at": "2026-06-08T16:30:00Z"
  },
  {
    "id": 2,
    "type": 1,
    "pattern": "^Acme*",
    "label": "org name matches \"^Acme*\"",
    "created_at": "2026-06-08T16:31:00Z"
  }
]
```

**Field descriptions:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | Session-assigned unique ID. When loaded, IDs are preserved and the next auto-increment starts after the highest loaded ID. |
| `type` | integer | Condition type enum: `0` = FlagClusterID, `1` = FlagOrgName. Future types (SUPPORTEX, OHSS) will use higher values. |
| `pattern` | string | The match pattern. For cluster IDs, this is the literal ID. For org names, this is the glob pattern including any `^`, `$`, `*` markers. |
| `label` | string | Human-readable description shown in the UI. |
| `created_at` | string | ISO 8601 timestamp of when the condition was created. |

**Default path:** `~/.config/srepd/flags.json`

The directory is created automatically if it doesn't exist. The file is
overwritten on each save (not appended).

## Behavior with OCM

Flag matching depends on cluster enrichment data from OCM:

- **Cluster ID flags** work partially without OCM — the raw `cluster_id` from
  alert details is always checked. OCM internal/external ID matching requires
  enrichment.
- **Organization flags** require OCM enrichment — they silently produce no
  matches when OCM is disconnected or cluster data hasn't arrived yet.
- When OCM connects or enrichment data arrives, the flag match cache is
  automatically rebuilt, and previously-unflagged incidents may become flagged.

## Future Extensions

The following condition types are planned but not yet implemented:

- **SUPPORTEX** — flag clusters with open support exceptions
- **OHSS** — flag clusters with open OHSS Jira cards

These will require OCM API integration for the corresponding endpoints.
