package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F2 tests: exactly one marker per --- block

func TestPrefixMessage_OneMarkerPerBlock(t *testing.T) {
	t.Run("single line gets one marker", func(t *testing.T) {
		result := prefixMessage("📡 ", "hello world")
		assert.Equal(t, "📡 hello world", result)
	})

	t.Run("multi-line block gets exactly one marker at start", func(t *testing.T) {
		text := "line one\nline two\nline three"
		result := prefixMessage("📡 ", text)

		lines := strings.Split(result, "\n")
		markerCount := 0
		for _, line := range lines {
			if strings.HasPrefix(line, "📡 ") {
				markerCount++
			}
		}
		assert.Equal(t, 1, markerCount,
			"multi-line block must have exactly ONE marker, got %d in: %q", markerCount, result)
		assert.True(t, strings.HasPrefix(result, "📡 "),
			"marker must be at the start of the block")
	})

	t.Run("agent marker multi-line gets one marker", func(t *testing.T) {
		text := "Hi! I'm watching.\n\nStill the same situation.\nWaiting on AMS."
		result := prefixMessage("🤖 ", text)

		lines := strings.Split(result, "\n")
		markerCount := 0
		for _, line := range lines {
			if strings.HasPrefix(line, "🤖 ") {
				markerCount++
			}
		}
		assert.Equal(t, 1, markerCount,
			"agent block must have exactly ONE marker, got %d in: %q", markerCount, result)
	})

	t.Run("markdown content gets one marker", func(t *testing.T) {
		text := "**Key Findings:**\n- `etcdMembersDownSRE` is critical\n- With ≤3 etcd members, quorum threatened"
		result := prefixMessage("📡 ", text)

		markerCount := strings.Count(result, "📡 ")
		assert.Equal(t, 1, markerCount,
			"markdown block must have exactly ONE marker, got %d", markerCount)
	})

	t.Run("empty string returns empty", func(t *testing.T) {
		result := prefixMessage("📡 ", "")
		assert.Equal(t, "", result)
	})

	t.Run("no-emoji markers also get one per block", func(t *testing.T) {
		text := "first line\nsecond line\nthird line"
		result := prefixMessage("☻ ", text)

		markerCount := strings.Count(result, "☻ ")
		assert.Equal(t, 1, markerCount)
	})
}

func TestWatcherBuffer_OneMarkerPerBlock_Integration(t *testing.T) {
	buf := newWatcherBuffer(50)

	buf.Append(prefixMessage("📡 ", "First verdict\nwith details"))
	buf.Append(prefixMessage("🤖 ", "Agent response\n\nWith blank lines\nand more text"))
	buf.Append(prefixMessage("📡 ", "Second verdict"))

	content := buf.Content()
	blocks := strings.Split(content, "\n---\n")
	require.Len(t, blocks, 3)

	watcherCount := strings.Count(blocks[0], "📡 ")
	assert.Equal(t, 1, watcherCount, "first block must have exactly one watcher marker")

	agentCount := strings.Count(blocks[1], "🤖 ")
	assert.Equal(t, 1, agentCount, "second block must have exactly one agent marker")

	watcherCount2 := strings.Count(blocks[2], "📡 ")
	assert.Equal(t, 1, watcherCount2, "third block must have exactly one watcher marker")
}

func TestWatcherBuffer_StreamingSetLast_OneMarker(t *testing.T) {
	buf := newWatcherBuffer(50)

	// Simulate streaming: Append empty, then SetLast with growing partial
	buf.Append(prefixMessage("🤖 ", ""))
	buf.SetLast(prefixMessage("🤖 ", "Hello"))
	buf.SetLast(prefixMessage("🤖 ", "Hello world"))
	buf.SetLast(prefixMessage("🤖 ", "Hello world\nSecond line"))
	buf.SetLast(prefixMessage("🤖 ", "Hello world\nSecond line\nThird line"))

	content := buf.Content()
	markerCount := strings.Count(content, "🤖 ")
	assert.Equal(t, 1, markerCount,
		"streaming block must have exactly ONE marker after multiple SetLast calls, got %d in:\n%s",
		markerCount, content)
}

// F1 tests: approvals render in watcher pane slot

func TestView_ApprovalsExpandedRendersInWatcherSlot(t *testing.T) {
	m := sizedTestModel(t)
	m.approvals = newApprovalsStrip()
	m.approvals.Add(Ask{
		Kind:          AskDraftNote,
		Title:         "OCM Agent notification failure on test-cluster.example.com",
		Body:          "OCM Agent has been unable to post ServiceLog notifications for ≥60 minutes",
		IncidentID:    "P1234567",
		IncidentTitle: "Test Alert Firing",
	})
	m.approvalsExpanded = true
	m.watcherExpanded = true
	m.recomputeLayout()

	view := m.View()

	// Approvals should be visible in the content area (watcher slot)
	assert.Contains(t, view, "Pending Approvals", "approvals header must be visible")
	assert.Contains(t, view, "OCM Agent notification failure", "ask title must be visible")

	// Body must be rendered
	assert.Contains(t, view, "OCM Agent has been unable to post", "ask body must be rendered")

	// Action target line must show incident info
	assert.Contains(t, view, "P1234567", "incident ID must be shown per ask")
}

func TestView_ApprovalsExpandedWrapsLongTitles(t *testing.T) {
	m := sizedTestModel(t)
	m.approvals = newApprovalsStrip()
	longTitle := "OCM Agent has been unable to post ServiceLog notifications for ≥60 minutes on cluster rhos-ota.ynna.p1.example.com which is very long"
	m.approvals.Add(Ask{
		Kind:          AskDraftNote,
		Title:         longTitle,
		Body:          "Detailed body text",
		IncidentID:    "P1234567",
		IncidentTitle: "Test Alert",
	})
	m.approvalsExpanded = true
	m.watcherExpanded = true
	m.recomputeLayout()

	view := m.View()

	// No line should exceed terminal width
	for i, line := range strings.Split(view, "\n") {
		w := lipgloss.Width(line)
		assert.LessOrEqual(t, w, 120,
			"line %d exceeds terminal width: %d cols: %q", i, w, line)
	}
}

func TestView_ApprovalsCollapsedShowsBadge(t *testing.T) {
	m := sizedTestModel(t)
	m.approvals = newApprovalsStrip()
	m.approvals.Add(Ask{Kind: AskDraftNote, Title: "Test"})
	m.approvalsExpanded = false

	view := m.View()

	assert.Contains(t, view, "⚑", "collapsed badge must show flag marker")
	assert.Contains(t, view, "1 ask", "collapsed badge must show count")
	assert.Contains(t, view, "press A", "collapsed badge must show key hint")
}

func TestView_ApprovalsRestoredWatcherStateOnClose(t *testing.T) {
	m := sizedTestModel(t)
	m.approvals = newApprovalsStrip()
	m.approvals.Add(Ask{Kind: AskDraftNote, Title: "Test"})
	m.watcherExpanded = true
	m.approvalsExpanded = true
	m.watcherWasExpanded = true

	// Simulate closing approvals via Escape
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := result.(model)

	assert.False(t, m2.approvalsExpanded, "approvals must close on Escape")
	assert.True(t, m2.watcherExpanded, "watcher must be restored to prior expanded state")
}

// F3 tests: markdown rendering in watcher pane

func TestUpdateWatcherViewport_RendersMarkdown(t *testing.T) {
	// Glamour skips rendering in Ascii mode (golden test safety guard),
	// so temporarily enable TrueColor to exercise the glamour path.
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := sizedTestModel(t)
	m.watcherExpanded = true
	m.watcherBuffer = newWatcherBuffer(50)
	m.recomputeLayout()

	m.watcherBuffer.Append(prefixMessage("📡 ", "**Key Findings:**\n- `etcdMembersDownSRE` is critical\n- Quorum threatened"))
	m.updateWatcherViewport()

	content := m.watcherViewport.View()

	assert.NotContains(t, content, "**Key Findings:**",
		"raw bold markdown must not appear after glamour rendering")
}

func TestRenderApprovalsExpanded_ShowsBody(t *testing.T) {
	strip := newApprovalsStrip()
	strip.Add(Ask{
		Kind:          AskDraftNote,
		Title:         "Post note to incident",
		Body:          "The cluster etcd members are failing health checks",
		IncidentID:    "P1234567",
		IncidentTitle: "etcdMembersDownSRE on test-cluster",
	})

	rendered := strip.RenderExpanded(80)

	assert.Contains(t, rendered, "Pending Approvals", "header must be present")
	assert.Contains(t, rendered, "Post note to incident", "title must be present")
	assert.Contains(t, rendered, "etcd members are failing", "body must be rendered")
	assert.Contains(t, rendered, "P1234567", "incident ID must be shown")
}

func TestRenderApprovalsExpanded_ActionTargetLine(t *testing.T) {
	strip := newApprovalsStrip()
	strip.Add(Ask{
		Kind:          AskDraftNote,
		Title:         "Investigation findings",
		Body:          "Cluster shows elevated error rate",
		IncidentID:    "P9999999",
		IncidentTitle: "CPU High on prod-cluster",
	})

	rendered := strip.RenderExpanded(100)

	assert.Contains(t, rendered, "P9999999", "incident ID must appear in action target line")
	assert.Contains(t, rendered, "Note", "ask kind must appear")
}
