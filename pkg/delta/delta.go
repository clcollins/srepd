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
	NoteAdded
	AlertAdded
	IncidentUpdated // title or service changed
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
	case NoteAdded:
		return "note_added"
	case AlertAdded:
		return "alert_added"
	case IncidentUpdated:
		return "incident_updated"
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
//
// NoteCount and AlertCount use *int to distinguish "unknown/not yet loaded"
// (nil) from "loaded and genuinely zero" (&0). Diff skips note/alert
// comparisons when the previous value is nil, preventing false-change bursts
// when the lazy enrichment cache loads between polls.
type Snapshot struct {
	ID         string
	Title      string
	Service    string
	Status     string
	Urgency    string
	NoteCount  *int
	AlertCount *int
}

// SnapshotFromFields constructs a Snapshot from individual fields, avoiding a
// dependency on any PagerDuty type in this package.
func SnapshotFromFields(id, title, service, status, urgency string, noteCount, alertCount *int) Snapshot {
	return Snapshot{
		ID:         id,
		Title:      title,
		Service:    service,
		Status:     status,
		Urgency:    urgency,
		NoteCount:  noteCount,
		AlertCount: alertCount,
	}
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
		if p.Title != c.Title {
			changes = append(changes, Change{
				Kind:       IncidentUpdated,
				IncidentID: c.ID,
				Summary:    fmt.Sprintf("Title changed: %s → %s", p.Title, c.Title),
			})
		}
		if p.Service != c.Service {
			changes = append(changes, Change{
				Kind:       IncidentUpdated,
				IncidentID: c.ID,
				Summary:    fmt.Sprintf("Service changed: %s → %s", p.Service, c.Service),
			})
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
		if p.NoteCount != nil && c.NoteCount != nil && *c.NoteCount > *p.NoteCount {
			added := *c.NoteCount - *p.NoteCount
			changes = append(changes, Change{
				Kind:       NoteAdded,
				IncidentID: c.ID,
				Summary:    fmt.Sprintf("%d new note(s)", added),
			})
		}
		if p.AlertCount != nil && c.AlertCount != nil && *c.AlertCount > *p.AlertCount {
			added := *c.AlertCount - *p.AlertCount
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
