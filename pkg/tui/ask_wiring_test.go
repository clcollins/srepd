package tui

import (
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/clcollins/srepd/pkg/ai/tools"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAskFromVerdict_DraftNote_ActionCallsPDAddNote(t *testing.T) {
	mock := &pd.MockPagerDutyClient{}
	m := createTestModel()
	m.config = &pd.Config{Client: mock}
	m.selectedIncident = &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC-TEST-001"},
	}

	verdict := tools.Verdict{
		Tier:    tools.TierActionable,
		Summary: "Cluster shows elevated error rate",
		Action:  "The cluster error rate is above threshold — posting investigation note",
	}

	ask := m.buildAskFromVerdict(verdict, nil)

	assert.Equal(t, AskDraftNote, ask.Kind)
	require.NotNil(t, ask.Action, "DraftNote Action must not be nil")

	cmd := ask.Action()
	require.NotNil(t, cmd, "DraftNote Action() must return a non-nil Cmd")

	msg := cmd()
	noteMsg, ok := msg.(addedIncidentNoteMsg)
	require.True(t, ok, "DraftNote Action must produce addedIncidentNoteMsg, got %T", msg)
	require.NoError(t, noteMsg.err)
	require.NotNil(t, noteMsg.note)
	assert.Equal(t, verdict.Action, noteMsg.note.Content, "note content must match verdict action")
	assert.Equal(t, 1, mock.CallCounts["CreateIncidentNoteWithContext"],
		"PD mock CreateIncidentNoteWithContext must be called exactly once")
}

func TestBuildAskFromVerdict_SuggestedCommand_CopiesNotExecutes(t *testing.T) {
	m := createTestModel()

	cmdText := "oc get nodes -o wide"
	verdict := tools.Verdict{
		Tier:    tools.TierActionable,
		Summary: "Suggest checking node status",
		Action:  cmdText,
	}

	ask := m.buildAskFromVerdict(verdict, nil)

	assert.Equal(t, AskSuggestedCommand, ask.Kind)
	require.NotNil(t, ask.Action, "SuggestedCommand Action must not be nil")

	cmd := ask.Action()
	require.NotNil(t, cmd, "SuggestedCommand Action() must return a non-nil Cmd")

	msg := cmd()
	statusMsg, ok := msg.(setStatusMsg)
	require.True(t, ok, "SuggestedCommand Action must produce setStatusMsg, got %T", msg)
	assert.Contains(t, statusMsg.string, cmdText,
		"status message must contain the command text")
	assert.Contains(t, statusMsg.string, "copy to terminal",
		"status message must indicate clipboard/copy, not execution")
}

func TestBuildAskFromVerdict_EscalationSuggestion_ReEscalates(t *testing.T) {
	m := createTestModel()
	incident := pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC-ESCALATE-001"},
		Title:     "Critical alert needs re-escalation",
	}
	m.selectedIncident = &incident

	verdict := tools.Verdict{
		Tier:    tools.TierActionable,
		Summary: "Incident should be re-escalated",
		Action:  "Re-escalate this incident to the on-call team",
	}

	ask := m.buildAskFromVerdict(verdict, nil)

	assert.Equal(t, AskEscalationSuggestion, ask.Kind)
	require.NotNil(t, ask.Action, "EscalationSuggestion Action must not be nil")

	cmd := ask.Action()
	require.NotNil(t, cmd, "EscalationSuggestion Action() must return a non-nil Cmd")

	msg := cmd()
	reescMsg, ok := msg.(unAcknowledgeIncidentsMsg)
	require.True(t, ok, "EscalationSuggestion Action must produce unAcknowledgeIncidentsMsg, got %T", msg)
	require.Len(t, reescMsg.incidents, 1)
	assert.Equal(t, "INC-ESCALATE-001", reescMsg.incidents[0].ID,
		"re-escalation must target the selected incident")
}

func TestBuildAskFromVerdict_UnknownText_FallbackHasAction(t *testing.T) {
	mock := &pd.MockPagerDutyClient{}
	m := createTestModel()
	m.config = &pd.Config{Client: mock}
	m.selectedIncident = &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC-FALLBACK-001"},
	}

	verdict := tools.Verdict{
		Tier:    tools.TierActionable,
		Summary: "Generic observation",
		Action:  "Something happened that does not match any known pattern",
	}

	ask := m.buildAskFromVerdict(verdict, nil)

	assert.Equal(t, AskDraftNote, ask.Kind,
		"inferAskKind fallback must return AskDraftNote, not zero-value")
	require.NotNil(t, ask.Action,
		"fallback ask must have a non-nil Action — a strip entry with nil Action does nothing on Accept")

	cmd := ask.Action()
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(addedIncidentNoteMsg)
	assert.True(t, ok, "fallback Action must produce addedIncidentNoteMsg, got %T", msg)
}

func TestInferAskKind_Fallback_IsSensibleDefault(t *testing.T) {
	tests := []struct {
		name     string
		action   string
		expected AskKind
	}{
		{"escalation keyword", "Re-escalate this incident", AskEscalationSuggestion},
		{"command keyword", "Run this command: oc get pods", AskSuggestedCommand},
		{"kubectl keyword", "kubectl get nodes", AskSuggestedCommand},
		{"generic text", "High error rate observed in the cluster", AskDraftNote},
		{"empty string", "", AskDraftNote},
		{"no keywords", "Investigation findings: memory pressure detected", AskDraftNote},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kind := inferAskKind(tc.action)
			assert.Equal(t, tc.expected, kind)
		})
	}
}

func TestBuildAskFromVerdict_DraftNote_TargetsOriginalIncident(t *testing.T) {
	mock := &pd.MockPagerDutyClient{}
	m := createTestModel()
	m.config = &pd.Config{Client: mock}

	incidentA := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC-A"},
		Title:     "Incident A",
	}
	incidentB := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC-B"},
		Title:     "Incident B",
	}

	m.selectedIncident = incidentA

	verdict := tools.Verdict{
		Tier:    tools.TierActionable,
		Summary: "Post note",
		Action:  "Note content for incident A",
	}
	ask := m.buildAskFromVerdict(verdict, nil)

	assert.Equal(t, "INC-A", ask.IncidentID,
		"Ask must snapshot the incident ID at creation time")
	assert.Equal(t, "Incident A", ask.IncidentTitle,
		"Ask must snapshot the incident title at creation time")

	// Simulate user browsing to a different incident
	m.selectedIncident = incidentB

	cmd := ask.Action()
	require.NotNil(t, cmd)
	msg := cmd()

	noteMsg, ok := msg.(addedIncidentNoteMsg)
	require.True(t, ok, "expected addedIncidentNoteMsg, got %T", msg)
	require.NoError(t, noteMsg.err)

	assert.Equal(t, "INC-A", noteMsg.incidentID,
		"note must target incident A (snapshotted), not B (live selection)")
}

func TestBuildAskFromVerdict_Escalation_TargetsOriginalIncident(t *testing.T) {
	m := createTestModel()

	incidentA := pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC-A"},
		Title:     "Incident A",
	}
	incidentB := pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC-B"},
		Title:     "Incident B",
	}

	m.selectedIncident = &incidentA

	verdict := tools.Verdict{
		Tier:    tools.TierActionable,
		Summary: "Re-escalate",
		Action:  "Re-escalate this incident",
	}
	ask := m.buildAskFromVerdict(verdict, nil)

	assert.Equal(t, "INC-A", ask.IncidentID)

	// Simulate user browsing to a different incident
	m.selectedIncident = &incidentB

	cmd := ask.Action()
	require.NotNil(t, cmd)
	msg := cmd()

	reescMsg, ok := msg.(unAcknowledgeIncidentsMsg)
	require.True(t, ok, "expected unAcknowledgeIncidentsMsg, got %T", msg)
	require.Len(t, reescMsg.incidents, 1)
	assert.Equal(t, "INC-A", reescMsg.incidents[0].ID,
		"escalation must target incident A (snapshotted), not B (live selection)")
}

func TestBuildAskFromVerdict_NilSelectedIncident_NoAction(t *testing.T) {
	m := createTestModel()
	m.selectedIncident = nil

	verdict := tools.Verdict{
		Tier:    tools.TierActionable,
		Summary: "Re-escalate",
		Action:  "Re-escalate this incident",
	}
	ask := m.buildAskFromVerdict(verdict, nil)

	assert.Empty(t, ask.IncidentID, "no incident selected means empty IncidentID")
	require.NotNil(t, ask.Action)

	cmd := ask.Action()
	require.NotNil(t, cmd)
	msg := cmd()

	statusMsg, ok := msg.(setStatusMsg)
	require.True(t, ok, "nil incident must produce setStatusMsg, got %T", msg)
	assert.Contains(t, statusMsg.string, "no incident")
}

func TestBuildAskFromVerdict_SanitizesControlSequences(t *testing.T) {
	m := createTestModel()
	m.selectedIncident = &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC-SANITIZE-001"},
	}

	verdict := tools.Verdict{
		Tier:    tools.TierActionable,
		Summary: "Clean\x1b]52;c;SGVsbG8=\x07title\x1b[31m",
		Action:  "Injected\x1b[2Jaction\x07text",
	}

	ask := m.buildAskFromVerdict(verdict, nil)

	assert.Equal(t, "Cleantitle", ask.Title,
		"Ask.Title must have control sequences stripped")
	assert.Equal(t, "Injectedactiontext", ask.Body,
		"Ask.Body must have control sequences stripped")
}

func TestBuildAskFromVerdict_UnhandledKind_FallbackAction(t *testing.T) {
	mock := &pd.MockPagerDutyClient{}
	m := createTestModel()
	m.config = &pd.Config{Client: mock}
	m.selectedIncident = &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC-UNHANDLED-001"},
	}

	verdict := tools.Verdict{
		Tier:    tools.TierActionable,
		Summary: "Tool permission request",
		Action:  "Permission to run write_note tool",
	}

	ask := m.buildAskFromVerdict(verdict, nil)

	require.NotNil(t, ask.Action,
		"even if the AskKind switch has no matching case, Action must not be nil")

	cmd := ask.Action()
	require.NotNil(t, cmd, "fallback Action() must return a non-nil Cmd")
	msg := cmd()
	assert.NotNil(t, msg, "fallback Action must produce a message")
}
