package tui

import (
	"encoding/json"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/ai/tools"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdate_InvestigationMsg_ActionableVerdictAddsAsk(t *testing.T) {
	m := createTestModel()
	m.approvals = newApprovalsStrip()
	m.help = newHelp()
	windowSize = tea.WindowSizeMsg{Width: 120, Height: 40}

	mock := &pd.MockPagerDutyClient{}
	m.config = &pd.Config{Client: mock}
	m.selectedIncident = &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC-M1-001"},
	}

	msg := investigationMsg{
		observation: "elevated error rate on cluster",
		verdict: tools.Verdict{
			Tier:    tools.TierActionable,
			Summary: "Cluster error rate above threshold",
			Action:  "Post investigation note about error rate spike",
		},
	}

	result, cmd := m.Update(msg)
	updated := result.(model)

	require.Equal(t, 1, updated.approvals.Count(),
		"actionable verdict with non-empty Action must add exactly one ask to the strip")
	ask := updated.approvals.asks[0]
	assert.Equal(t, "Cluster error rate above threshold", ask.Title)
	assert.Equal(t, AskDraftNote, ask.Kind)
	assert.NotNil(t, cmd, "handler must return the typewriter tick cmd")
}

func TestUpdate_InvestigationMsg_SilentVerdictDoesNotAddAsk(t *testing.T) {
	m := createTestModel()
	m.approvals = newApprovalsStrip()
	m.help = newHelp()
	windowSize = tea.WindowSizeMsg{Width: 120, Height: 40}

	msg := investigationMsg{
		observation: "routine check passed",
		verdict: tools.Verdict{
			Tier:    tools.TierSilent,
			Summary: "All clear",
			Action:  "",
		},
	}

	result, _ := m.Update(msg)
	updated := result.(model)

	assert.Equal(t, 0, updated.approvals.Count(),
		"silent verdict must not add an ask to the strip")
}

func TestUpdate_InvestigationMsg_NoteworthyVerdictDoesNotAddAsk(t *testing.T) {
	m := createTestModel()
	m.approvals = newApprovalsStrip()
	m.help = newHelp()
	windowSize = tea.WindowSizeMsg{Width: 120, Height: 40}

	msg := investigationMsg{
		observation: "something interesting found",
		verdict: tools.Verdict{
			Tier:    tools.TierNoteworthy,
			Summary: "Interesting pattern detected",
			Action:  "",
		},
	}

	result, _ := m.Update(msg)
	updated := result.(model)

	assert.Equal(t, 0, updated.approvals.Count(),
		"noteworthy verdict must not add an ask to the strip")
}

func TestUpdate_ApprovalsEnter_ReturnsCmdThatPostsNote(t *testing.T) {
	m := createTestModel()
	m.approvals = newApprovalsStrip()
	m.help = newHelp()
	windowSize = tea.WindowSizeMsg{Width: 120, Height: 40}

	mock := &pd.MockPagerDutyClient{}
	m.config = &pd.Config{Client: mock}
	m.selectedIncident = &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC-M2-001"},
	}

	noteContent := "Investigation: elevated error rate on cluster"
	m.approvals.Add(Ask{
		Kind:  AskDraftNote,
		Title: "Post investigation note",
		Body:  noteContent,
		Action: func() tea.Cmd {
			return m.postAINoteCmd(noteContent)
		},
	})

	m.approvalsExpanded = true
	m.table.Focus()

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.Update(enterMsg)
	updated := result.(model)

	assert.Equal(t, 0, updated.approvals.Count(),
		"accepted ask must be removed from the strip")

	require.NotNil(t, cmd,
		"Enter on an ask must return the action's tea.Cmd, not nil")

	produced := cmd()
	noteMsg, ok := produced.(addedIncidentNoteMsg)
	require.True(t, ok, "cmd must produce addedIncidentNoteMsg, got %T", produced)
	require.NoError(t, noteMsg.err)
	require.NotNil(t, noteMsg.note)
	assert.Equal(t, noteContent, noteMsg.note.Content,
		"the posted note content must match the verdict action")
	assert.Equal(t, 1, mock.CallCounts["CreateIncidentNoteWithContext"],
		"PD mock CreateIncidentNoteWithContext must be called exactly once")
}

func TestUpdate_ApprovalsDismiss_DoesNotReturnActionCmd(t *testing.T) {
	m := createTestModel()
	m.approvals = newApprovalsStrip()
	m.help = newHelp()
	windowSize = tea.WindowSizeMsg{Width: 120, Height: 40}

	actionCalled := false
	m.approvals.Add(Ask{
		Kind:  AskDraftNote,
		Title: "Post investigation note",
		Body:  "some note",
		Action: func() tea.Cmd {
			actionCalled = true
			return func() tea.Msg { return setStatusMsg{"should not happen"} }
		},
	})

	m.approvalsExpanded = true
	m.table.Focus()

	dismissMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
	result, cmd := m.Update(dismissMsg)
	updated := result.(model)

	assert.Equal(t, 0, updated.approvals.Count(),
		"dismissed ask must be removed from the strip")
	assert.Nil(t, cmd,
		"Dismiss must not return an action cmd")
	assert.False(t, actionCalled,
		"Dismiss must not invoke the ask's Action")
}

func TestUpdate_InvestigationMsg_ToolAsksAreNotSurfaced(t *testing.T) {
	m := createTestModel()
	m.approvals = newApprovalsStrip()
	m.help = newHelp()
	windowSize = tea.WindowSizeMsg{Width: 120, Height: 40}

	mock := &pd.MockPagerDutyClient{}
	m.config = &pd.Config{Client: mock}
	m.selectedIncident = &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC-F2-001"},
	}

	msg := investigationMsg{
		observation: "investigation with tool asks (deferred to plan 415)",
		verdict: tools.Verdict{
			Tier:    tools.TierActionable,
			Summary: "Action plus tool asks present",
			Action:  "Post note about findings",
		},
		toolAsks: []toolAsk{
			{toolName: "bash", input: json.RawMessage(`{"cmd":"ls"}`)},
			{toolName: "read_file", input: json.RawMessage(`{"path":"/etc/hosts"}`)},
		},
	}

	result, cmd := m.Update(msg)
	updated := result.(model)

	require.Equal(t, 1, updated.approvals.Count(),
		"only the verdict Action ask must be surfaced; tool asks are deferred (plan 415)")
	assert.Equal(t, AskDraftNote, updated.approvals.asks[0].Kind,
		"the sole ask must be the verdict action, not a tool permission")
	assert.NotNil(t, cmd, "handler must return the typewriter tick cmd")
}
