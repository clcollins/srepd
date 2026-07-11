package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BuildSummaryRows exposes the summary as structured rows so the TUI can
// style them; BuildSummary keeps rendering the same plain text from the same
// rows (existing tests pin that behavior).
func TestBuildSummaryRows(t *testing.T) {
	existing := ExistingConfig{Token: "oldtoken", Teams: []string{"T1"}}
	final := ResolvedValues{
		Token:        "newtoken1234",
		Teams:        []string{"T1", "T2"},
		SilentPolicy: "PSIL1",
	}
	changes := ConfigChanges{TokenChanged: true, TeamsChanged: true, SilentChanged: true}
	teamNames := map[string]string{"T1": "Alpha", "T2": "Beta"}
	policyNames := map[string]string{"PSIL1": "Silent Test"}

	rows := BuildSummaryRows(existing, final, changes, teamNames, policyNames)

	assert.GreaterOrEqual(t, len(rows), 3)
	assert.Equal(t, "Token", rows[0].Label)
	assert.True(t, rows[0].Changed)
	assert.Contains(t, rows[0].Value, "****", "token must be masked")

	assert.Equal(t, "Teams", rows[1].Label)
	assert.Contains(t, rows[1].Value, "Alpha (T1)")
	assert.True(t, rows[1].Changed)

	var silentRow *SummaryRow
	for i := range rows {
		if rows[i].Label == "Silent policy" {
			silentRow = &rows[i]
		}
	}
	assert.NotNil(t, silentRow)
	assert.Contains(t, silentRow.Value, "Silent Test (PSIL1)")
}

func TestBuildSummaryRows_UnchangedMarks(t *testing.T) {
	existing := ExistingConfig{Token: "sametoken12", Teams: []string{"T1"}}
	final := ResolvedValues{Token: "sametoken12", Teams: []string{"T1"}}

	rows := BuildSummaryRows(existing, final, ConfigChanges{}, nil, nil)

	assert.False(t, rows[0].Changed)
	assert.False(t, rows[1].Changed)
}
