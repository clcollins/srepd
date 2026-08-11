package delta

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func intPtr(n int) *int { return &n }

func TestDiff_NoChange(t *testing.T) {
	snaps := []Snapshot{
		{ID: "P1", Title: "Alert A", Service: "svc-a", Status: "triggered", Urgency: "high"},
		{ID: "P2", Title: "Alert B", Service: "svc-b", Status: "acknowledged", Urgency: "low"},
	}
	changes := Diff(snaps, snaps)
	assert.Empty(t, changes, "identical snapshots must produce no changes")
}

func TestDiff_NewIncident(t *testing.T) {
	prev := []Snapshot{
		{ID: "P1", Title: "Alert A", Service: "svc-a", Status: "triggered", Urgency: "high"},
	}
	curr := []Snapshot{
		{ID: "P1", Title: "Alert A", Service: "svc-a", Status: "triggered", Urgency: "high"},
		{ID: "P2", Title: "New Alert", Service: "svc-b", Status: "triggered", Urgency: "high"},
	}
	changes := Diff(prev, curr)
	require.Len(t, changes, 1)
	assert.Equal(t, IncidentNew, changes[0].Kind)
	assert.Equal(t, "P2", changes[0].IncidentID)
	assert.Contains(t, changes[0].Summary, "New Alert")
}

func TestDiff_IncidentResolved(t *testing.T) {
	prev := []Snapshot{
		{ID: "P1", Title: "Alert A", Service: "svc-a", Status: "triggered", Urgency: "high"},
		{ID: "P2", Title: "Alert B", Service: "svc-b", Status: "triggered", Urgency: "high"},
	}
	curr := []Snapshot{
		{ID: "P1", Title: "Alert A", Service: "svc-a", Status: "triggered", Urgency: "high"},
	}
	changes := Diff(prev, curr)
	require.Len(t, changes, 1)
	assert.Equal(t, IncidentResolved, changes[0].Kind)
	assert.Equal(t, "P2", changes[0].IncidentID)
}

func TestDiff_StatusChange(t *testing.T) {
	prev := []Snapshot{
		{ID: "P1", Title: "Alert A", Service: "svc-a", Status: "triggered", Urgency: "high"},
	}
	curr := []Snapshot{
		{ID: "P1", Title: "Alert A", Service: "svc-a", Status: "acknowledged", Urgency: "high"},
	}
	changes := Diff(prev, curr)
	require.Len(t, changes, 1)
	assert.Equal(t, StatusChanged, changes[0].Kind)
	assert.Contains(t, changes[0].Summary, "triggered")
	assert.Contains(t, changes[0].Summary, "acknowledged")
}

func TestDiff_UrgencyChange(t *testing.T) {
	prev := []Snapshot{
		{ID: "P1", Title: "Alert A", Service: "svc-a", Status: "triggered", Urgency: "low"},
	}
	curr := []Snapshot{
		{ID: "P1", Title: "Alert A", Service: "svc-a", Status: "triggered", Urgency: "high"},
	}
	changes := Diff(prev, curr)
	require.Len(t, changes, 1)
	assert.Equal(t, UrgencyChanged, changes[0].Kind)
	assert.Contains(t, changes[0].Summary, "low")
	assert.Contains(t, changes[0].Summary, "high")
}

func TestDiff_NoteAdded(t *testing.T) {
	prev := []Snapshot{
		{ID: "P1", Title: "Alert A", Service: "svc-a", Status: "triggered", Urgency: "high", NoteCount: intPtr(2)},
	}
	curr := []Snapshot{
		{ID: "P1", Title: "Alert A", Service: "svc-a", Status: "triggered", Urgency: "high", NoteCount: intPtr(4)},
	}
	changes := Diff(prev, curr)
	require.Len(t, changes, 1)
	assert.Equal(t, NoteAdded, changes[0].Kind)
	assert.Contains(t, changes[0].Summary, "2 new note(s)")
}

func TestDiff_AlertAdded(t *testing.T) {
	prev := []Snapshot{
		{ID: "P1", Title: "Alert A", Service: "svc-a", Status: "triggered", Urgency: "high", AlertCount: intPtr(1)},
	}
	curr := []Snapshot{
		{ID: "P1", Title: "Alert A", Service: "svc-a", Status: "triggered", Urgency: "high", AlertCount: intPtr(3)},
	}
	changes := Diff(prev, curr)
	require.Len(t, changes, 1)
	assert.Equal(t, AlertAdded, changes[0].Kind)
	assert.Contains(t, changes[0].Summary, "2 new alert(s)")
}

func TestDiff_MultipleChanges(t *testing.T) {
	prev := []Snapshot{
		{ID: "P1", Title: "Alert A", Service: "svc-a", Status: "triggered", Urgency: "low", NoteCount: intPtr(1), AlertCount: intPtr(1)},
	}
	curr := []Snapshot{
		{ID: "P1", Title: "Alert A", Service: "svc-a", Status: "acknowledged", Urgency: "high", NoteCount: intPtr(3), AlertCount: intPtr(2)},
	}
	changes := Diff(prev, curr)
	assert.Len(t, changes, 4, "status + urgency + notes + alerts")
}

func TestDiff_NilNoteCountSkipsComparison(t *testing.T) {
	prev := []Snapshot{
		{ID: "P1", Title: "A", Service: "svc", Status: "triggered", Urgency: "high", NoteCount: nil},
	}
	curr := []Snapshot{
		{ID: "P1", Title: "A", Service: "svc", Status: "triggered", Urgency: "high", NoteCount: intPtr(5)},
	}
	changes := Diff(prev, curr)
	for _, c := range changes {
		assert.NotEqual(t, NoteAdded, c.Kind, "nil→known must not produce NoteAdded")
	}
}

func TestDiff_NilAlertCountSkipsComparison(t *testing.T) {
	prev := []Snapshot{
		{ID: "P1", Title: "A", Service: "svc", Status: "triggered", Urgency: "high", AlertCount: nil},
	}
	curr := []Snapshot{
		{ID: "P1", Title: "A", Service: "svc", Status: "triggered", Urgency: "high", AlertCount: intPtr(3)},
	}
	changes := Diff(prev, curr)
	for _, c := range changes {
		assert.NotEqual(t, AlertAdded, c.Kind, "nil→known must not produce AlertAdded")
	}
}

func TestDiff_ZeroToNonZeroNoteCountDetected(t *testing.T) {
	prev := []Snapshot{
		{ID: "P1", Title: "A", Service: "svc", Status: "triggered", Urgency: "high", NoteCount: intPtr(0)},
	}
	curr := []Snapshot{
		{ID: "P1", Title: "A", Service: "svc", Status: "triggered", Urgency: "high", NoteCount: intPtr(1)},
	}
	changes := Diff(prev, curr)
	require.Len(t, changes, 1)
	assert.Equal(t, NoteAdded, changes[0].Kind, "genuine 0→1 must be detected")
}

func TestDiff_FirstSighting(t *testing.T) {
	curr := []Snapshot{
		{ID: "P1", Title: "Alert A", Service: "svc-a", Status: "triggered", Urgency: "high"},
		{ID: "P2", Title: "Alert B", Service: "svc-b", Status: "triggered", Urgency: "low"},
	}
	changes := Diff(nil, curr)
	assert.Len(t, changes, 2, "every incident on first sighting must produce IncidentNew")
	for _, c := range changes {
		assert.Equal(t, IncidentNew, c.Kind)
	}
}

func TestDiff_ReorderingOnly(t *testing.T) {
	prev := []Snapshot{
		{ID: "P1", Title: "A", Service: "svc", Status: "triggered", Urgency: "high"},
		{ID: "P2", Title: "B", Service: "svc", Status: "triggered", Urgency: "high"},
	}
	curr := []Snapshot{
		{ID: "P2", Title: "B", Service: "svc", Status: "triggered", Urgency: "high"},
		{ID: "P1", Title: "A", Service: "svc", Status: "triggered", Urgency: "high"},
	}
	changes := Diff(prev, curr)
	assert.Empty(t, changes, "reordering without field changes must produce no changes")
}

func TestDiff_EmptyBoth(t *testing.T) {
	changes := Diff(nil, nil)
	assert.Empty(t, changes)
}

func TestNarrate_Empty(t *testing.T) {
	result := Narrate(nil, time.Now())
	assert.Equal(t, "", result)
}

func TestNarrate_SingleChange(t *testing.T) {
	changes := []Change{
		{Kind: IncidentNew, IncidentID: "P1", Summary: "New incident: Alert A (svc-a)"},
	}
	result := Narrate(changes, time.Now())
	assert.Contains(t, result, "Changes since last check")
	assert.Contains(t, result, "P1")
	assert.Contains(t, result, "new")
	assert.Contains(t, result, "Alert A")
}

func TestNarrate_MultipleChanges(t *testing.T) {
	changes := []Change{
		{Kind: IncidentNew, IncidentID: "P1", Summary: "New incident"},
		{Kind: StatusChanged, IncidentID: "P2", Summary: "Status changed"},
	}
	result := Narrate(changes, time.Now())
	assert.Contains(t, result, "P1")
	assert.Contains(t, result, "P2")
}

func TestNarrate_CapsAtMaxLines(t *testing.T) {
	var changes []Change
	for i := 0; i < 25; i++ {
		changes = append(changes, Change{
			Kind:       IncidentNew,
			IncidentID: "P" + string(rune('A'+i)),
			Summary:    "New incident",
		})
	}
	result := Narrate(changes, time.Now())
	assert.Contains(t, result, "... and 5 more changes")
}

func TestChangeKind_String(t *testing.T) {
	assert.Equal(t, "new", IncidentNew.String())
	assert.Equal(t, "resolved", IncidentResolved.String())
	assert.Equal(t, "status_changed", StatusChanged.String())
	assert.Equal(t, "urgency_changed", UrgencyChanged.String())
	assert.Equal(t, "note_added", NoteAdded.String())
	assert.Equal(t, "alert_added", AlertAdded.String())
	assert.Equal(t, "unknown", ChangeKind(99).String())
}
