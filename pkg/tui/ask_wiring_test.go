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

	ask := m.buildAskFromVerdict(verdict)

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

	ask := m.buildAskFromVerdict(verdict)

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

	ask := m.buildAskFromVerdict(verdict)

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

	ask := m.buildAskFromVerdict(verdict)

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

	ask := m.buildAskFromVerdict(verdict)

	require.NotNil(t, ask.Action,
		"even if the AskKind switch has no matching case, Action must not be nil")

	cmd := ask.Action()
	require.NotNil(t, cmd, "fallback Action() must return a non-nil Cmd")
	msg := cmd()
	assert.NotNil(t, msg, "fallback Action must produce a message")
}
