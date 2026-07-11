package tui

import (
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func setupMouseTestModel(incidents []pagerduty.Incident) model {
	m := createTestModelWithTable(incidents)
	m.help = newHelp()
	windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}
	m.recomputeLayout()
	m.table.Focus()
	return m
}

func testIncidents() []pagerduty.Incident {
	return []pagerduty.Incident{
		{
			APIObject:          pagerduty.APIObject{ID: "Q001"},
			Title:              "Incident 1",
			Service:            pagerduty.APIObject{Summary: "svc-1"},
			LastStatusChangeAt: "2024-01-01T00:00:00Z",
		},
		{
			APIObject:          pagerduty.APIObject{ID: "Q002"},
			Title:              "Incident 2",
			Service:            pagerduty.APIObject{Summary: "svc-2"},
			LastStatusChangeAt: "2024-01-01T00:00:00Z",
		},
		{
			APIObject:          pagerduty.APIObject{ID: "Q003"},
			Title:              "Incident 3",
			Service:            pagerduty.APIObject{Summary: "svc-3"},
			LastStatusChangeAt: "2024-01-01T00:00:00Z",
		},
	}
}

func TestMouseScroll_TableScrollDown(t *testing.T) {
	t.Run("wheel down on table area moves cursor down", func(t *testing.T) {
		m := setupMouseTestModel(testIncidents())

		// Table cursor starts at row 0
		assert.Equal(t, 0, m.table.Cursor())

		msg := tea.MouseMsg{
			X:      10,
			Y:      5, // within table area
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonWheelDown,
		}
		result, _ := m.Update(msg)
		updated := result.(model)

		assert.Equal(t, 1, updated.table.Cursor())
	})
}

func TestMouseScroll_TableScrollUp(t *testing.T) {
	t.Run("wheel up on table area moves cursor up", func(t *testing.T) {
		m := setupMouseTestModel(testIncidents())

		// Move to row 2 first
		m.table.MoveDown(2)
		assert.Equal(t, 2, m.table.Cursor())

		msg := tea.MouseMsg{
			X:      10,
			Y:      5, // within table area
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonWheelUp,
		}
		result, _ := m.Update(msg)
		updated := result.(model)

		assert.Equal(t, 1, updated.table.Cursor())
	})
}

func TestMouseScroll_TableScrollUpAtTop(t *testing.T) {
	t.Run("wheel up at top of table stays at row 0", func(t *testing.T) {
		m := setupMouseTestModel(testIncidents())
		assert.Equal(t, 0, m.table.Cursor())

		msg := tea.MouseMsg{
			X:      10,
			Y:      5,
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonWheelUp,
		}
		result, _ := m.Update(msg)
		updated := result.(model)

		assert.Equal(t, 0, updated.table.Cursor())
	})
}

func TestMouseScroll_WatcherExpandedScrollWatcher(t *testing.T) {
	t.Run("wheel down in watcher area scrolls watcher viewport", func(t *testing.T) {
		m := setupMouseTestModel(testIncidents())
		m.watcherExpanded = true
		m.recomputeLayout()
		m.watcherViewport.Width = m.layout.WatcherWidth
		m.watcherViewport.Height = m.layout.WatcherHeight

		lines := ""
		for i := 0; i < 50; i++ {
			lines += "line\n"
		}
		m.watcherViewport.SetContent(lines)

		// Y coordinate in watcher area: after table + footer + newline
		watcherY := m.mouseWatcherStartY()

		msg := tea.MouseMsg{
			X:      10,
			Y:      watcherY + 2,
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonWheelDown,
		}
		result, _ := m.Update(msg)
		updated := result.(model)

		assert.Greater(t, updated.watcherViewport.YOffset, 0)
	})
}

func TestMouseScroll_WatcherExpandedScrollTable(t *testing.T) {
	t.Run("wheel down in table area scrolls table even when watcher expanded", func(t *testing.T) {
		m := setupMouseTestModel(testIncidents())
		m.watcherExpanded = true
		m.recomputeLayout()
		m.watcherViewport.Width = m.layout.WatcherWidth
		m.watcherViewport.Height = m.layout.WatcherHeight

		assert.Equal(t, 0, m.table.Cursor())

		msg := tea.MouseMsg{
			X:      10,
			Y:      5, // within table area
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonWheelDown,
		}
		result, _ := m.Update(msg)
		updated := result.(model)

		assert.Equal(t, 1, updated.table.Cursor())
	})
}

func TestMouseScroll_WatcherCollapsedScrollTable(t *testing.T) {
	t.Run("wheel down scrolls table when watcher collapsed", func(t *testing.T) {
		m := setupMouseTestModel(testIncidents())
		m.watcherExpanded = false
		m.recomputeLayout()

		assert.Equal(t, 0, m.table.Cursor())

		msg := tea.MouseMsg{
			X:      10,
			Y:      5,
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonWheelDown,
		}
		result, _ := m.Update(msg)
		updated := result.(model)

		assert.Equal(t, 1, updated.table.Cursor())
	})
}

func TestMouseScroll_IncidentViewForwardsToViewport(t *testing.T) {
	t.Run("mouse scroll in incident view forwards to incidentViewer", func(t *testing.T) {
		m := setupMouseTestModel(testIncidents())
		m.viewingIncident = true

		lines := ""
		for i := 0; i < 100; i++ {
			lines += "line\n"
		}
		m.incidentViewer.Width = m.layout.IncidentViewerWidth
		m.incidentViewer.Height = m.layout.IncidentViewerHeight
		m.incidentViewer.SetContent(lines)

		msg := tea.MouseMsg{
			X:      10,
			Y:      10,
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonWheelDown,
		}
		result, _ := m.Update(msg)
		updated := result.(model)

		assert.Greater(t, updated.incidentViewer.YOffset, 0)
	})
}

func TestMouseScroll_LogViewForwardsToViewport(t *testing.T) {
	t.Run("mouse scroll in log view forwards to logViewer", func(t *testing.T) {
		m := setupMouseTestModel(testIncidents())
		m.viewingLog = true

		lines := ""
		for i := 0; i < 100; i++ {
			lines += "line\n"
		}
		m.logViewer.Width = m.layout.IncidentViewerWidth
		m.logViewer.Height = m.layout.IncidentViewerHeight
		m.logViewer.SetContent(lines)

		msg := tea.MouseMsg{
			X:      10,
			Y:      10,
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonWheelDown,
		}
		result, _ := m.Update(msg)
		updated := result.(model)

		assert.Greater(t, updated.logViewer.YOffset, 0)
	})
}

func TestMouseScroll_NonWheelEventsAreNoOps(t *testing.T) {
	t.Run("non-wheel mouse events return nil cmd on main view", func(t *testing.T) {
		m := setupMouseTestModel(testIncidents())

		msg := tea.MouseMsg{
			X:      10,
			Y:      5,
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonLeft,
		}
		result, cmd := m.Update(msg)
		updated := result.(model)

		assert.Equal(t, 0, updated.table.Cursor())
		assert.Nil(t, cmd)
	})
}

func TestMouseScroll_AfterWindowResize(t *testing.T) {
	t.Run("scroll routing adapts after window resize", func(t *testing.T) {
		m := setupMouseTestModel(testIncidents())
		m.watcherExpanded = true
		m.recomputeLayout()
		m.watcherViewport.Width = m.layout.WatcherWidth
		m.watcherViewport.Height = m.layout.WatcherHeight

		lines := ""
		for i := 0; i < 50; i++ {
			lines += "line\n"
		}
		m.watcherViewport.SetContent(lines)

		// Record watcher start Y before resize
		watcherYBefore := m.mouseWatcherStartY()

		// Resize the window
		windowSize = tea.WindowSizeMsg{Width: 80, Height: 40}
		m.recomputeLayout()
		m.watcherViewport.Width = m.layout.WatcherWidth
		m.watcherViewport.Height = m.layout.WatcherHeight

		watcherYAfter := m.mouseWatcherStartY()

		// The boundary should have changed with the resize
		assert.NotEqual(t, watcherYBefore, watcherYAfter)

		// Scrolling in the new table area should still move the table
		assert.Equal(t, 0, m.table.Cursor())
		msg := tea.MouseMsg{
			X:      10,
			Y:      5,
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonWheelDown,
		}
		result, _ := m.Update(msg)
		updated := result.(model)
		assert.Equal(t, 1, updated.table.Cursor())
	})
}

func TestMouseScroll_SyncsSelectedIncident(t *testing.T) {
	t.Run("scrolling table via mouse syncs selectedIncident", func(t *testing.T) {
		m := setupMouseTestModel(testIncidents())

		// Sync initial selection
		m.syncSelectedIncidentToHighlightedRow()
		assert.Equal(t, "Q001", m.selectedIncident.ID)

		msg := tea.MouseMsg{
			X:      10,
			Y:      5,
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonWheelDown,
		}
		result, cmd := m.Update(msg)
		updated := result.(model)

		assert.Equal(t, 1, updated.table.Cursor())
		// Mouse scroll should trigger sync (returns a cmd for pre-fetch)
		assert.NotNil(t, cmd)
	})
}

func TestMouseWatcherStartY(t *testing.T) {
	t.Run("returns correct boundary between table and watcher", func(t *testing.T) {
		m := setupMouseTestModel(testIncidents())
		m.watcherExpanded = true
		m.recomputeLayout()

		containerVOverhead := m.styles.TableContainer.GetVerticalBorderSize() +
			m.styles.TableContainer.GetVerticalPadding() +
			m.styles.TableContainer.GetVerticalMargins()

		// Header (2) + table container overhead + table height + footer (1) + newline (1) - 4 adjustment
		expected := layoutHeaderLines + containerVOverhead + m.layout.TableHeight + layoutFooterLines + layoutFooterNewline - 4
		assert.Equal(t, expected, m.mouseWatcherStartY())
	})
}
