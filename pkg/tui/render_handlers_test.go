package tui

import (
	"errors"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/docs"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// logFileContentMsg
// ---------------------------------------------------------------------------

func TestLogFileContentMsg_SetsViewingLog(t *testing.T) {
	t.Run("sets logViewer content and viewingLog to true", func(t *testing.T) {
		m := createTestModel()
		m.viewingLog = false

		result, cmd := m.Update(logFileContentMsg("line 1\nline 2\nline 3"))
		m = result.(model)

		assert.True(t, m.viewingLog, "viewingLog should be true after receiving log content")
		assert.Nil(t, cmd, "no command should be returned")
	})
}

func TestLogFileContentMsg_EmptyContent(t *testing.T) {
	t.Run("handles empty content", func(t *testing.T) {
		m := createTestModel()
		m.viewingLog = false

		result, cmd := m.Update(logFileContentMsg(""))
		m = result.(model)

		assert.True(t, m.viewingLog, "viewingLog should be true even with empty content")
		assert.Nil(t, cmd, "no command should be returned")
	})
}

// ---------------------------------------------------------------------------
// renderDocsMsg
// ---------------------------------------------------------------------------

func TestRenderDocsMsg_EmptyDocsPages(t *testing.T) {
	t.Run("empty docsPages sets viewingDocs to false", func(t *testing.T) {
		m := createTestModel()
		m.docsPages = []docs.Doc{} // empty
		m.viewingDocs = true       // was viewing

		result, cmd := m.Update(renderDocsMsg("render"))
		m = result.(model)

		assert.False(t, m.viewingDocs, "viewingDocs should be false when docsPages is empty")
		assert.Nil(t, cmd, "no command should be returned for empty docs")
	})
}

func TestRenderDocsMsg_NonEmptyDocsPages(t *testing.T) {
	t.Run("non-empty docsPages returns renderDocsContent command", func(t *testing.T) {
		m := createTestModel()
		m.docsPages = docs.EmbeddedDocs() // use real embedded docs
		m.viewingDocs = false

		result, cmd := m.Update(renderDocsMsg("render"))
		_ = result.(model)

		assert.NotNil(t, cmd, "should return a renderDocsContent command when docs are available")
	})
}

// ---------------------------------------------------------------------------
// renderedDocsMsg
// ---------------------------------------------------------------------------

func TestRenderedDocsMsg_Error(t *testing.T) {
	t.Run("error: returns errMsg", func(t *testing.T) {
		m := createTestModel()

		docErr := errors.New("markdown render failed")
		result, cmd := m.Update(renderedDocsMsg{content: "", err: docErr})
		_ = result.(model)

		assert.NotNil(t, cmd, "should return a command wrapping errMsg")

		msg := cmd()
		em, ok := msg.(errMsg)
		assert.True(t, ok, "returned command should produce errMsg")
		assert.Equal(t, docErr, em.error)
	})
}

func TestRenderedDocsMsg_HappyPath(t *testing.T) {
	t.Run("sets docsViewer content and viewingDocs to true", func(t *testing.T) {
		m := createTestModel()
		m.viewingDocs = false

		result, cmd := m.Update(renderedDocsMsg{content: "# Welcome\n\nHello docs", err: nil})
		m = result.(model)

		assert.True(t, m.viewingDocs, "viewingDocs should be true after receiving rendered docs")
		assert.Nil(t, cmd, "no command should be returned on success")
	})
}

// ---------------------------------------------------------------------------
// renderIncidentMsg
// ---------------------------------------------------------------------------

func TestRenderIncidentMsg_NilSelectedIncident(t *testing.T) {
	t.Run("guard: sets status and viewingIncident to false", func(t *testing.T) {
		m := createTestModel()
		m.selectedIncident = nil
		m.viewingIncident = true

		result, cmd := m.Update(renderIncidentMsg("render"))
		m = result.(model)

		assert.False(t, m.viewingIncident, "viewingIncident should be false when no incident selected")
		assert.Contains(t, m.status, "failed render incidents")
		assert.Contains(t, m.status, "no incidents provided")
		assert.Nil(t, cmd, "no command should be returned")
	})
}

func TestRenderIncidentMsg_HappyPath(t *testing.T) {
	t.Run("returns renderIncident command when selectedIncident is set", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.config.Client = &pd.MockPagerDutyClient{}

		result, cmd := m.Update(renderIncidentMsg("render"))
		_ = result.(model)

		assert.NotNil(t, cmd, "should return a renderIncident command")
	})
}

// ---------------------------------------------------------------------------
// renderedIncidentMsg
// ---------------------------------------------------------------------------

func TestRenderedIncidentMsg_Error(t *testing.T) {
	t.Run("error: returns errMsg", func(t *testing.T) {
		m := createTestModel()

		renderErr := errors.New("template error")
		result, cmd := m.Update(renderedIncidentMsg{content: "", err: renderErr})
		_ = result.(model)

		assert.NotNil(t, cmd, "should return a command wrapping errMsg")

		msg := cmd()
		em, ok := msg.(errMsg)
		assert.True(t, ok, "returned command should produce errMsg")
		assert.Equal(t, renderErr, em.error)
	})
}

func TestRenderedIncidentMsg_HappyWithSelectedIncident(t *testing.T) {
	t.Run("sets incidentViewer content and viewingIncident to true", func(t *testing.T) {
		m := createTestModel()
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "P888"},
		}
		m.viewingIncident = false

		result, cmd := m.Update(renderedIncidentMsg{content: "## Incident P888\nDetails here", err: nil})
		m = result.(model)

		assert.True(t, m.viewingIncident, "viewingIncident should be true after successful render")
		assert.Nil(t, cmd, "no command should be returned on success")
	})
}

func TestRenderedIncidentMsg_HappyWithoutSelectedIncident(t *testing.T) {
	t.Run("discards render silently when selectedIncident is nil (late arrival)", func(t *testing.T) {
		m := createTestModel()
		m.selectedIncident = nil
		m.viewingIncident = false

		result, cmd := m.Update(renderedIncidentMsg{content: "stale content", err: nil})
		m = result.(model)

		assert.False(t, m.viewingIncident,
			"viewingIncident should remain false when incident was closed before render arrived")
		assert.Nil(t, cmd, "no command should be returned for discarded render")
	})
}

func TestRenderedIncidentMsg_FirstRenderGoesToTop(t *testing.T) {
	t.Run("first render triggers GotoTop, progressive update does not", func(t *testing.T) {
		m := createTestModel()
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "P111"},
		}
		m.viewingIncident = false

		// First render
		result, _ := m.Update(renderedIncidentMsg{content: "initial render", err: nil})
		m = result.(model)
		assert.True(t, m.viewingIncident, "viewingIncident should be true after first render")

		// Second render (progressive update) -- viewingIncident is already true
		result, _ = m.Update(renderedIncidentMsg{content: "updated render", err: nil})
		m = result.(model)
		assert.True(t, m.viewingIncident, "viewingIncident should still be true after progressive update")
	})
}

// ---------------------------------------------------------------------------
// renderedIncidentMsg table-driven summary
// ---------------------------------------------------------------------------

func TestRenderedIncidentMsg_TableDriven(t *testing.T) {
	tests := []struct {
		name             string
		content          string
		err              error
		selectedIncident *pagerduty.Incident
		initialViewing   bool
		expectViewing    bool
		expectCmd        bool
	}{
		{
			name:           "error returns errMsg",
			content:        "",
			err:            errors.New("render failed"),
			expectViewing:  false,
			expectCmd:      true,
			initialViewing: false,
		},
		{
			name:    "happy path with incident",
			content: "rendered content",
			err:     nil,
			selectedIncident: &pagerduty.Incident{
				APIObject: pagerduty.APIObject{ID: "PRENDER"},
			},
			initialViewing: false,
			expectViewing:  true,
			expectCmd:      false,
		},
		{
			name:             "discards when incident is nil (late arrival)",
			content:          "stale",
			err:              nil,
			selectedIncident: nil,
			initialViewing:   false,
			expectViewing:    false,
			expectCmd:        false,
		},
		{
			name:    "progressive update preserves viewing state",
			content: "updated",
			err:     nil,
			selectedIncident: &pagerduty.Incident{
				APIObject: pagerduty.APIObject{ID: "PPROG"},
			},
			initialViewing: true,
			expectViewing:  true,
			expectCmd:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.selectedIncident = tt.selectedIncident
			m.viewingIncident = tt.initialViewing

			result, cmd := m.Update(renderedIncidentMsg{content: tt.content, err: tt.err})
			m = result.(model)

			assert.Equal(t, tt.expectViewing, m.viewingIncident, "viewingIncident mismatch")

			if tt.expectCmd {
				assert.NotNil(t, cmd, "expected a command")
			} else {
				assert.Nil(t, cmd, "expected no command")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Focus mode overlap fix: ctrl+h from incident view
// ---------------------------------------------------------------------------

func TestCtrlH_FromIncidentView_ClearsViewingIncident(t *testing.T) {
	m := createTestModelWithSelectedIncident()
	m.config.Client = &pd.MockPagerDutyClient{}
	m.viewingIncident = true

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlH})
	m = result.(model)

	assert.True(t, m.viewingDocs, "viewingDocs should be true")
	assert.False(t, m.viewingIncident, "viewingIncident should be cleared")
	assert.True(t, m.docsReturnToIncident, "docsReturnToIncident should be set")
	assert.NotNil(t, cmd, "should return renderDocsMsg command")
}

func TestEscape_FromDocs_ReturnsToIncidentView(t *testing.T) {
	m := createTestModelWithSelectedIncident()
	m.config.Client = &pd.MockPagerDutyClient{}
	m.viewingDocs = true
	m.docsReturnToIncident = true

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(model)

	assert.False(t, m.viewingDocs, "viewingDocs should be cleared")
	assert.True(t, m.viewingIncident, "viewingIncident should be restored")
	assert.False(t, m.docsReturnToIncident, "docsReturnToIncident should be cleared")
}

func TestEscape_FromDocs_ReturnsToTable(t *testing.T) {
	m := createTestModelWithSelectedIncident()
	m.config.Client = &pd.MockPagerDutyClient{}
	m.viewingDocs = true
	m.docsReturnToIncident = false

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(model)

	assert.False(t, m.viewingDocs, "viewingDocs should be cleared")
	assert.False(t, m.viewingIncident, "viewingIncident should remain false")
}

func TestCtrlH_FromTable_DoesNotSetReturnFlag(t *testing.T) {
	m := createTestModelWithSelectedIncident()
	m.config.Client = &pd.MockPagerDutyClient{}
	m.viewingIncident = false

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlH})
	m = result.(model)

	assert.True(t, m.viewingDocs, "viewingDocs should be true")
	assert.False(t, m.docsReturnToIncident, "docsReturnToIncident should be false from table")
}
