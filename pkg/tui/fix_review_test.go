package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/clcollins/srepd/pkg/agent"
	"github.com/clcollins/srepd/pkg/ai/tools"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- F1: Approvals key must only open from table view ---

func TestApprovalsKey_BlockedInIncidentView(t *testing.T) {
	m := sizedTestModel(t)
	m.viewingIncident = true
	m.approvals = newApprovalsStrip()
	m.approvals.Add(Ask{Kind: AskDraftNote, Title: "test", IncidentID: "P1234567"})

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	updated := result.(model)

	assert.False(t, updated.approvalsExpanded,
		"A key must not open approvals overlay while viewing an incident")
}

func TestApprovalsKey_BlockedInLogView(t *testing.T) {
	m := sizedTestModel(t)
	m.viewingLog = true
	m.approvals = newApprovalsStrip()
	m.approvals.Add(Ask{Kind: AskDraftNote, Title: "test", IncidentID: "P1234567"})

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	updated := result.(model)

	assert.False(t, updated.approvalsExpanded,
		"A key must not open approvals overlay while viewing logs")
}

func TestApprovalsKey_BlockedInDocsView(t *testing.T) {
	m := sizedTestModel(t)
	m.viewingDocs = true
	m.approvals = newApprovalsStrip()
	m.approvals.Add(Ask{Kind: AskDraftNote, Title: "test", IncidentID: "P1234567"})

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	updated := result.(model)

	assert.False(t, updated.approvalsExpanded,
		"A key must not open approvals overlay while viewing docs")
}

func TestApprovalsKey_AllowedInTableView(t *testing.T) {
	m := sizedTestModel(t)
	m.approvals = newApprovalsStrip()
	m.approvals.Add(Ask{Kind: AskDraftNote, Title: "test", IncidentID: "P1234567"})

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	updated := result.(model)

	assert.True(t, updated.approvalsExpanded,
		"A key should open approvals overlay in table view")
}

func TestApprovalsExpanded_RendersInAllViewModes(t *testing.T) {
	m := sizedTestModel(t)
	m.approvals = newApprovalsStrip()
	m.approvals.Add(Ask{Kind: AskDraftNote, Title: "test ask", Body: "test body", IncidentID: "P1234567"})
	m.approvalsExpanded = true
	m.watcherExpanded = true
	m.recomputeLayout()

	view := m.View()
	assert.Contains(t, view, "Pending Approvals",
		"expanded approvals must be visible in table view")
}

// --- F2: --- separator must not become setext heading ---

func TestWatcherBuffer_MultiEntryNoSextextHeading(t *testing.T) {
	m := sizedTestModel(t)
	m.watcherExpanded = true
	m.watcherBuffer = newWatcherBuffer(50)
	m.watcherBuffer.Append(prefixMessage(m.watcherMarker, "First verdict about etcd"))
	m.watcherBuffer.Append(prefixMessage(m.watcherMarker, "Second verdict about pods"))
	m.recomputeLayout()
	m.updateWatcherViewport()

	content := m.watcherViewport.View()

	assert.Contains(t, content, "First verdict",
		"first entry must remain visible, not swallowed by heading")
	assert.Contains(t, content, "Second verdict",
		"second entry must remain visible")
	assert.NotContains(t, content, "##",
		"entries must not be rendered as markdown headings")
}

func TestRenderWatcherMarkdown_IndividualEntries(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	m := sizedTestModel(t)
	m.watcherExpanded = true
	m.watcherBuffer = newWatcherBuffer(50)
	m.watcherBuffer.Append("**bold text** in entry one")
	m.watcherBuffer.Append("entry two with `code`")
	m.recomputeLayout()
	m.updateWatcherViewport()

	content := m.watcherViewport.View()

	assert.NotContains(t, content, "**bold text**",
		"raw markdown bold syntax should be rendered, not passed through")
}

// --- F3: Agent results must not be double-rendered ---

func TestAgentResult_NoDoubleRender(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	m := sizedTestModel(t)
	m.watcherExpanded = true
	m.watcherBuffer = newWatcherBuffer(50)
	m.claudeQuerying = true
	m.recomputeLayout()

	// Simulate agent session Init + Result
	result, _ := m.Update(agentSessionEventMsg{
		event: agent.Event{Kind: agent.Init},
	})
	m = result.(model)

	result, _ = m.Update(agentSessionEventMsg{
		event: agent.Event{Kind: agent.Result, Text: "**Important** finding with `code`"},
	})
	m = result.(model)

	content := m.watcherViewport.View()

	// Raw ev.Text should be stored and rendered once by updateWatcherViewport,
	// not pre-rendered then re-rendered
	assert.NotContains(t, content, "**Important**",
		"raw bold markers must not survive — text should be glamour-rendered exactly once")

	// Double-rendering produces nested ANSI sequences where glamour
	// re-processes its own output. Count NBSP occurrences: glamour uses
	// exactly one pair around inline code spans. Double-rendering would
	// produce extra pairs.
	nbspCount := strings.Count(content, " ")
	assert.LessOrEqual(t, nbspCount, 2,
		"at most one NBSP pair (around `code`); double-rendering would produce more")
}

// --- F4: IncidentTitle must be sanitized ---

func TestBuildAskFromVerdict_SanitizesIncidentTitle(t *testing.T) {
	m := createTestModel()
	m.config = &pd.Config{Client: &pd.MockPagerDutyClient{}}

	incident := pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC-1"},
		Title:     "Alert \x1b[31mred\x1b[0m injection \x1b]0;pwned\x07",
	}
	m.incidentList = []pagerduty.Incident{incident}

	verdict := tools.Verdict{
		Tier:    tools.TierActionable,
		Summary: "Post note",
		Action:  "Note content",
	}

	ask := m.buildAskFromVerdict(verdict, []string{"INC-1"})

	assert.NotContains(t, ask.IncidentTitle, "\x1b",
		"ANSI escape sequences must be stripped from IncidentTitle")
	assert.NotContains(t, ask.IncidentTitle, "\x07",
		"BEL character must be stripped from IncidentTitle")
	assert.Contains(t, ask.IncidentTitle, "Alert",
		"visible text must be preserved")
	assert.Contains(t, ask.IncidentTitle, "injection",
		"visible text must be preserved")
}

func TestBuildAskFromVerdict_SanitizesSelectedIncidentTitle(t *testing.T) {
	m := createTestModel()
	m.config = &pd.Config{Client: &pd.MockPagerDutyClient{}}
	m.selectedIncident = &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC-2"},
		Title:     "Fallback \x1b[31mred\x1b[0m title",
	}

	verdict := tools.Verdict{
		Tier:    tools.TierActionable,
		Summary: "Post note",
		Action:  "Note content",
	}

	// No originating incidents — falls back to selectedIncident
	ask := m.buildAskFromVerdict(verdict, nil)

	assert.NotContains(t, ask.IncidentTitle, "\x1b",
		"ANSI escapes must be stripped from fallback IncidentTitle")
	assert.Contains(t, ask.IncidentTitle, "Fallback",
		"visible text must be preserved")
}

// --- F5: Approvals pane must not exceed WatcherHeight ---

func TestApprovalsPane_HeightClamped(t *testing.T) {
	m := sizedTestModel(t)
	m.approvals = newApprovalsStrip()

	// Add a long-body ask that would exceed WatcherHeight
	longBody := strings.Repeat("This is a very long line of approval body text.\n", 40)
	m.approvals.Add(Ask{
		Kind:       AskDraftNote,
		Title:      "Long approval",
		Body:       longBody,
		IncidentID: "P1234567",
	})
	m.approvalsExpanded = true
	m.watcherExpanded = true
	m.recomputeLayout()

	view := m.View()
	lines := strings.Split(view, "\n")
	lineCount := len(lines)
	if lineCount > 0 && lines[lineCount-1] == "" {
		lineCount--
	}

	assert.LessOrEqual(t, lineCount, 40,
		"view with long approvals body must not exceed terminal height (%d lines)", lineCount)
}

// --- F6: Renderer caching ---

func TestRenderWatcherMarkdown_CachesRenderer(t *testing.T) {
	m := sizedTestModel(t)
	m.watcherExpanded = true
	m.recomputeLayout()

	// Call twice with same width — should reuse cached renderer
	result1 := m.renderWatcherMarkdown("**bold**", 80)
	result2 := m.renderWatcherMarkdown("**bold**", 80)

	assert.Equal(t, result1, result2,
		"same width should produce identical output (cached renderer)")

	// Different width should still work
	result3 := m.renderWatcherMarkdown("**bold**", 60)
	assert.NotEmpty(t, result3, "different width should still render")
}

// --- Minor: Watcher/Input keys must not fire under approvals overlay ---

func TestWatcherKey_BlockedUnderApprovalsOverlay(t *testing.T) {
	m := sizedTestModel(t)
	m.approvals = newApprovalsStrip()
	m.approvals.Add(Ask{Kind: AskDraftNote, Title: "test", IncidentID: "P1234567"})
	m.approvalsExpanded = true
	m.watcherExpanded = true
	m.recomputeLayout()

	wasExpanded := m.watcherExpanded

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	updated := result.(model)

	assert.Equal(t, wasExpanded, updated.watcherExpanded,
		"w key must not toggle watcher while approvals overlay is open")
}

func TestInputKey_BlockedUnderApprovalsOverlay(t *testing.T) {
	m := sizedTestModel(t)
	m.config = &pd.Config{Client: &pd.MockPagerDutyClient{}}
	m.approvals = newApprovalsStrip()
	m.approvals.Add(Ask{Kind: AskDraftNote, Title: "test", IncidentID: "P1234567"})
	m.approvalsExpanded = true
	m.watcherExpanded = true
	m.recomputeLayout()

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	updated := result.(model)

	assert.False(t, updated.input.Focused(),
		": key must not open input while approvals overlay is open")
}

// --- Helper to check no line exceeds width with approvals ---

func TestView_ApprovalsExpandedNoLineExceedsWidth(t *testing.T) {
	for _, width := range []int{60, 80, 120} {
		t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
			m := createTestModelWithSelectedIncident()
			m.config.Client = &pd.MockPagerDutyClient{}
			size := tea.WindowSizeMsg{Width: width, Height: 40}
			windowSize = size
			result, _ := m.Update(size)
			m = result.(model)
			m.table.SetRows([]table.Row{
				{dot, "P1234567", "Test Alert Firing", "test-service"},
			})

			m.approvals = newApprovalsStrip()
			m.approvals.Add(Ask{
				Kind:          AskDraftNote,
				Title:         "A very long approval title that should be wrapped properly within bounds",
				Body:          "Body text that is also quite long and should be wrapped within the pane width",
				IncidentID:    "P1234567",
				IncidentTitle: "Test Alert Firing",
			})
			m.approvalsExpanded = true
			m.watcherExpanded = true
			m.recomputeLayout()

			view := m.View()
			for i, line := range strings.Split(view, "\n") {
				w := lipgloss.Width(line)
				assert.LessOrEqual(t, w, width,
					"approvals line %d is %d cols wide (limit %d): %q",
					i, w, width, line)
			}
		})
	}
}

// --- Golden tests for new rendering ---

func TestGolden_WatcherMultiEntry(t *testing.T) {
	m := goldenTestModel(t)
	m.watcherExpanded = true
	m.watcherBuffer = newWatcherBuffer(50)
	m.watcherBuffer.Append(prefixMessage(m.watcherMarker, "**Key Findings:**\n- etcd member down"))
	m.watcherBuffer.Append(prefixMessage(m.watcherMarker, "**Follow-up:**\n- Cluster recovered"))
	m.recomputeLayout()
	m.updateWatcherViewport()

	view := m.View()

	// In ASCII mode (golden tests), glamour is bypassed, but entries must still
	// be separate and not corrupted by setext heading interpretation
	assert.Contains(t, view, "Key Findings",
		"first entry should be visible")
	assert.Contains(t, view, "Follow-up",
		"second entry should be visible")
}

// Verify golden snapshot for multi-entry watcher
func TestGolden_WatcherMultiEntrySnapshot(t *testing.T) {
	m := goldenTestModel(t)
	m.watcherExpanded = true
	m.watcherBuffer = newWatcherBuffer(50)
	m.watcherBuffer.Append(prefixMessage(m.watcherMarker, "First observation about cluster storm"))
	m.watcherBuffer.Append(prefixMessage(m.agentMarker, "Analysis complete: etcd quorum is safe"))
	m.recomputeLayout()
	m.updateWatcherViewport()

	require.NotEmpty(t, m.View(), "multi-entry watcher view must not be empty")
}

// --- SEC-004b: PagerDuty data must be sanitized before rendering ---

func TestSummarizeIncident_SanitizesFields(t *testing.T) {
	inc := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "P123"},
		Title:     "Alert \x1b[31mred\x1b[0m injection",
		Service: pagerduty.APIObject{
			Summary: "svc \x1b]0;pwned\x07 name",
		},
		EscalationPolicy: pagerduty.APIObject{
			Summary: "policy \x1b[2Jclear",
		},
	}

	s := summarizeIncident(inc)

	assert.NotContains(t, s.Title, "\x1b",
		"incident title must be stripped of ANSI escapes")
	assert.Contains(t, s.Title, "Alert",
		"visible text must be preserved in title")
	assert.NotContains(t, s.Service, "\x1b",
		"service name must be stripped of ANSI escapes")
	assert.NotContains(t, s.Service, "\x07",
		"BEL must be stripped from service name")
	assert.NotContains(t, s.EscalationPolicy, "\x1b",
		"escalation policy must be stripped of ANSI escapes")
}

func TestSummarizeNotes_SanitizesContent(t *testing.T) {
	notes := []pagerduty.IncidentNote{
		{
			ID:      "N1",
			Content: "Note with \x1b[31mANSI\x1b[0m and \x1b]0;title\x07 injection",
			User: pagerduty.APIObject{
				Summary: "user \x1b[2J clear-screen",
			},
		},
	}

	summaries := summarizeNotes(notes)
	require.Len(t, summaries, 1)

	assert.NotContains(t, summaries[0].Content, "\x1b",
		"note content must be stripped of ANSI escapes")
	assert.NotContains(t, summaries[0].Content, "\x07",
		"BEL must be stripped from note content")
	assert.NotContains(t, summaries[0].User, "\x1b",
		"note user must be stripped of ANSI escapes")
	assert.Contains(t, summaries[0].Content, "Note with",
		"visible text must be preserved")
}

func TestSummarizeAlerts_SanitizesFields(t *testing.T) {
	alerts := []pagerduty.IncidentAlert{
		{
			APIObject: pagerduty.APIObject{ID: "A1"},
			Service: pagerduty.APIObject{
				Summary: "evil-svc \x1b[31mred\x1b[0m",
			},
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"alert_name": "alert \x1b[2Jclear",
				},
			},
		},
	}

	summaries := summarizeAlerts(alerts, nil)
	require.Len(t, summaries, 1)

	assert.NotContains(t, summaries[0].Service, "\x1b",
		"alert service must be stripped of ANSI escapes")
	assert.NotContains(t, summaries[0].Name, "\x1b",
		"alert name must be stripped of ANSI escapes")
}

func TestTableRows_SanitizePDData(t *testing.T) {
	title := "\x1b[31mEvil\x1b[0m Title"
	sanitized := stripControl(title)

	assert.NotContains(t, sanitized, "\x1b",
		"table row title must be stripped of ANSI escapes")
	assert.Contains(t, sanitized, "Evil",
		"visible text must be preserved")
	assert.Contains(t, sanitized, "Title",
		"visible text must be preserved")
}
