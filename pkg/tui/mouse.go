package tui

import tea "github.com/charmbracelet/bubbletea"

// mouseWatcherStartY returns the Y coordinate where the watcher pane begins.
// This is computed from the current layout so it stays correct after window resizes.
// The -4 adjustment accounts for style padding that the layout constants don't capture.
func (m model) mouseWatcherStartY() int {
	containerVOverhead := m.styles.TableContainer.GetVerticalBorderSize() +
		m.styles.TableContainer.GetVerticalPadding() +
		m.styles.TableContainer.GetVerticalMargins()

	return layoutHeaderLines + containerVOverhead + m.layout.TableHeight + layoutFooterLines + layoutFooterNewline - 4
}

// handleMouseMsg routes mouse events to the correct component based on the
// current view mode and the mouse Y position within the window.
//
// For non-table views (incident viewer, log viewer, and any future views),
// mouse events are forwarded unconditionally to the active viewport via the
// focus-mode dispatch so new views get mouse scroll for free.
//
// For the main table view, Y coordinates determine whether the event targets
// the incident table or the watcher pane.
func (m model) handleMouseMsg(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseButtonWheelUp && msg.Button != tea.MouseButtonWheelDown {
		return m, nil
	}

	switch {
	case m.viewingIncident:
		m.incidentViewer, _ = m.incidentViewer.Update(msg)
		return m, nil

	case m.viewingLog:
		m.logViewer, _ = m.logViewer.Update(msg)
		return m, nil

	case m.configMode, m.bulkSilenceMode, m.teamSelectMode, m.clusterSelectMode, m.mergeMode:
		return m, nil

	default:
		return m.handleMouseScrollMainView(msg)
	}
}

// handleMouseScrollMainView routes wheel events on the main table+watcher screen.
func (m model) handleMouseScrollMainView(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.watcherExpanded && msg.Y >= m.mouseWatcherStartY() {
		var cmd tea.Cmd
		m.watcherViewport, cmd = m.watcherViewport.Update(msg)
		return m, cmd
	}

	switch msg.Button {
	case tea.MouseButtonWheelDown:
		m.table.MoveDown(1)
	case tea.MouseButtonWheelUp:
		m.table.MoveUp(1)
	}
	cmd := m.syncSelectedIncidentToHighlightedRow()
	return m, cmd
}
