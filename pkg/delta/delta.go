package delta

import (
	"fmt"
	"strings"
	"time"
)

// ChangeKind classifies a state transition between consecutive polls.
type ChangeKind int

const (
	IncidentNew      ChangeKind = iota // first sighting — no prior state
	IncidentResolved                   // was present, now absent
	StatusChanged
	UrgencyChanged
	Escalated
	NoteAdded
	AlertAdded
)

func (k ChangeKind) String() string {
	switch k {
	case IncidentNew:
		return "new"
	case IncidentResolved:
		return "resolved"
	case StatusChanged:
		return "status_changed"
	case UrgencyChanged:
		return "urgency_changed"
	case Escalated:
		return "escalated"
	case NoteAdded:
		return "note_added"
	case AlertAdded:
		return "alert_added"
	default:
		return "unknown"
	}
}

// Change represents a single state transition for an incident.
type Change struct {
	Kind       ChangeKind
	IncidentID string
	Summary    string
}

// Snapshot captures the fingerprint-relevant fields of an incident at a point
// in time. Pure value type — no I/O.
type Snapshot struct {
	ID              string
	Title           string
	Service         string
	ClusterID       string
	Status          string
	Urgency         string
	NoteCount       int
	AlertCount      int
	EscalationLevel int
}

// Diff computes changes between prev and curr snapshots. Pure function: values
// in, values out, no I/O. First-sighting semantics: a snapshot in curr with no
// match in prev produces IncidentNew. A snapshot in prev with no match in curr
// produces IncidentResolved.
func Diff(prev, curr []Snapshot) []Change {
	prevMap := make(map[string]Snapshot, len(prev))
	for _, s := range prev {
		prevMap[s.ID] = s
	}

	currMap := make(map[string]Snapshot, len(curr))
	for _, s := range curr {
		currMap[s.ID] = s
	}

	var changes []Change

	// Detect new and changed incidents (iterate curr in order for determinism)
	for _, c := range curr {
		p, existed := prevMap[c.ID]
		if !existed {
			changes = append(changes, Change{
				Kind:       IncidentNew,
				IncidentID: c.ID,
				Summary:    fmt.Sprintf("New incident: %s (%s)", c.Title, c.Service),
			})
			continue
		}
		if p.Status != c.Status {
			changes = append(changes, Change{
				Kind:       StatusChanged,
				IncidentID: c.ID,
				Summary:    fmt.Sprintf("Status changed: %s → %s", p.Status, c.Status),
			})
		}
		if p.Urgency != c.Urgency {
			changes = append(changes, Change{
				Kind:       UrgencyChanged,
				IncidentID: c.ID,
				Summary:    fmt.Sprintf("Urgency changed: %s → %s", p.Urgency, c.Urgency),
			})
		}
		if c.EscalationLevel > p.EscalationLevel {
			changes = append(changes, Change{
				Kind:       Escalated,
				IncidentID: c.ID,
				Summary:    fmt.Sprintf("Escalated: level %d → %d", p.EscalationLevel, c.EscalationLevel),
			})
		}
		if c.NoteCount > p.NoteCount {
			added := c.NoteCount - p.NoteCount
			changes = append(changes, Change{
				Kind:       NoteAdded,
				IncidentID: c.ID,
				Summary:    fmt.Sprintf("%d new note(s)", added),
			})
		}
		if c.AlertCount > p.AlertCount {
			added := c.AlertCount - p.AlertCount
			changes = append(changes, Change{
				Kind:       AlertAdded,
				IncidentID: c.ID,
				Summary:    fmt.Sprintf("%d new alert(s)", added),
			})
		}
	}

	// Detect resolved incidents (iterate prev in order for determinism)
	for _, p := range prev {
		if _, exists := currMap[p.ID]; !exists {
			changes = append(changes, Change{
				Kind:       IncidentResolved,
				IncidentID: p.ID,
				Summary:    fmt.Sprintf("Resolved: %s", p.Title),
			})
		}
	}

	return changes
}

// Narrate formats changes into a compact narrative block for the LLM.
// Pure function: changes + reference time in, string out.
func Narrate(changes []Change, now time.Time) string {
	if len(changes) == 0 {
		return ""
	}

	var lines []string
	for _, c := range changes {
		lines = append(lines, fmt.Sprintf("- [%s] %s: %s", c.IncidentID, c.Kind, c.Summary))
	}

	const maxLines = 20
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, fmt.Sprintf("... and %d more changes", len(changes)-maxLines))
	}

	return fmt.Sprintf("Changes since last check:\n%s", strings.Join(lines, "\n"))
}
