package tui

import (
	"math"
	"reflect"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
)

// setStatusMsgHandler is the message handler for the setStatusMsg message
func (m model) setStatusMsgHandler(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.setStatus(msg.(setStatusMsg).Status())
	return m, nil
}

// errMsgHandler is the message handler for the errMsg message
func (m model) errMsgHandler(msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Error("errMsgHandler", "tea.errMsg", msg)
	m.setStatus(msg.(errMsg).Error())
	m.err = msg.(errMsg)
	return m, nil
}

// windowSizeMsgHandler is the message handler for the windowSizeMsg message
// and resizes the tui according to the new terminal window size
func (m model) windowSizeMsgHandler(msg tea.Msg) (tea.Model, tea.Cmd) {
	windowSize = msg.(tea.WindowSizeMsg)
	rowCount := func(m model) int {
		// This func sets the table height to a reasonable size
		// in the case there are no incidents
		if len(m.table.Rows()) > 0 {
			return len(m.table.Rows())
		}
		return 10
	}(m)

	verticalMargins := mainStyle.GetVerticalMargins()
	horizontalMargins := mainStyle.GetHorizontalMargins()
	verticalPadding := mainStyle.GetVerticalPadding()
	horizontalPadding := mainStyle.GetHorizontalPadding()
	verticalBorders := mainStyle.GetVerticalBorderSize()
	horizontalBorders := mainStyle.GetHorizontalBorderSize()

	tableVerticalMargins := tableContainerStyle.GetVerticalMargins()
	tableHorizontalMargins := tableContainerStyle.GetHorizontalMargins()
	tableVerticalPadding := tableContainerStyle.GetVerticalPadding()
	tableHorizontalPadding := tableContainerStyle.GetHorizontalPadding()
	tableVerticalBorders := tableContainerStyle.GetVerticalBorderSize()
	tableHorizontalBorders := tableContainerStyle.GetHorizontalBorderSize()

	cellVerticalPadding := tableCellStyle.GetVerticalPadding() * rowCount    // Number of rows
	cellHorizontalPadding := tableCellStyle.GetHorizontalPadding() * 4       // Four columns
	cellVerticalMargins := tableCellStyle.GetVerticalMargins() * rowCount    // Number of rows
	cellHorizontalMargins := tableCellStyle.GetHorizontalMargins() * 4       // Four columns
	cellVerticalBorders := tableCellStyle.GetVerticalBorderSize() * rowCount // Number of rows
	cellHorizontalBorders := tableCellStyle.GetHorizontalBorderSize() * 4    // Four columns

	estimatedExtraLinesFromComponents := 7 // TODO: figure out how to calculate this

	horizontalScratchWidth := horizontalMargins + horizontalPadding + horizontalBorders
	verticalScratchWidth := verticalMargins + verticalPadding + verticalBorders

	incidentHorizontalScratchWidth := incidentViewerStyle.GetHorizontalMargins() + incidentViewerStyle.GetHorizontalPadding() + incidentViewerStyle.GetHorizontalBorderSize()
	incidentVerticalScratchWidth := incidentViewerStyle.GetVerticalMargins() + incidentViewerStyle.GetVerticalPadding() + incidentViewerStyle.GetVerticalBorderSize()

	tableHorizontalScratchWidth := tableHorizontalMargins + tableHorizontalPadding + tableHorizontalBorders + cellHorizontalPadding + cellHorizontalMargins + cellHorizontalBorders
	tableVerticalScratchWidth := tableVerticalMargins + tableVerticalPadding + tableVerticalBorders + cellVerticalPadding + cellVerticalMargins + cellVerticalBorders

	tableWidth := windowSize.Width - horizontalScratchWidth - tableHorizontalScratchWidth
	tableHeight := windowSize.Height - verticalScratchWidth - tableVerticalScratchWidth - rowCount - estimatedExtraLinesFromComponents

	m.table.SetHeight(tableHeight)

	// converting to floats, rounding up and converting back to int handles layout issues arising from odd numbers
	columnWidth := int(math.Ceil(float64(tableWidth-idWidth-dotWidth) / float64(2)))

	log.Debug("tui.windowSizeMsgHandler",
		"window_width", windowSize.Width,
		"window_height", windowSize.Height,
		"table_horizontal_scratch_width", tableHorizontalScratchWidth,
		"table_vertical_scratch_width", tableVerticalScratchWidth,
		"incident_horizontal_scratch_width", incidentHorizontalScratchWidth,
		"incident_vertical_scratch_width", incidentVerticalScratchWidth,
		"horizontal_scratch_width", horizontalScratchWidth,
		"vertical_scratch_width", verticalScratchWidth,
		"table_width", tableWidth,
		"table_height", tableHeight,
		"column_width", columnWidth,
	)

	m.table.SetColumns([]table.Column{
		{Title: dot, Width: dotWidth},
		{Title: "ID", Width: idWidth - dotWidth},
		{Title: "Summary", Width: columnWidth},
		{Title: "Service", Width: columnWidth},
	})

	m.incidentViewer.Width = windowSize.Width - horizontalScratchWidth - incidentHorizontalScratchWidth
	m.incidentViewer.Height = windowSize.Height - verticalScratchWidth - incidentVerticalScratchWidth

	m.help.Width = windowSize.Width - horizontalScratchWidth

	return m, nil
}

func (m model) keyMsgHandler(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key.Matches(msg.(tea.KeyMsg), defaultKeyMap.Quit) {
		return m, tea.Quit
	}

	if key.Matches(msg.(tea.KeyMsg), defaultKeyMap.AutoRefresh) {
		m.autoRefresh = !m.autoRefresh
		return m, nil
	}

	if key.Matches(msg.(tea.KeyMsg), defaultKeyMap.AutoAck) {
		m.autoAcknowledge = !m.autoAcknowledge
		return m, nil
	}

	// Default commands for the table view
	switch {
	case m.err != nil:
		return switchErrorFocusMode(m, msg)

	case m.viewingIncident:
		return switchIncidentFocusMode(m, msg)

	case m.input.Focused():
		return switchInputFocusMode(m, msg)

	case m.table.Focused():
		return switchTableFocusMode(m, msg)
	}

	return m, nil
}

// tableFocusMode is the main mode for the application
func switchTableFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Debug("switchTableFocusMode", reflect.TypeOf(msg), msg)
	var cmds []tea.Cmd

	// [1] is column two of the row: the incident ID
	var row table.Row
	var incidentID string
	row = m.table.SelectedRow()
	if row == nil {
		incidentID = ""
	} else {
		incidentID = m.table.SelectedRow()[1]
	}

	log.Debug("switchTableFocusMode", "incidentID", incidentID)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Help):
			m.toggleHelp()

		case key.Matches(msg, defaultKeyMap.Up):
			m.table.MoveUp(1)

		case key.Matches(msg, defaultKeyMap.Down):
			m.table.MoveDown(1)

		case key.Matches(msg, defaultKeyMap.Top):
			m.table.GotoTop()

		case key.Matches(msg, defaultKeyMap.Bottom):
			m.table.GotoBottom()

		case key.Matches(msg, defaultKeyMap.Input):
			return m, tea.Sequence(
				m.input.Focus(),
			)

		case key.Matches(msg, defaultKeyMap.Team):
			m.teamMode = !m.teamMode
			log.Debug("switchTableFocusMode", "teamMode", m.teamMode)
			cmds = append(cmds, func() tea.Msg { return updatedIncidentListMsg{m.incidentList, nil} })

		case key.Matches(msg, defaultKeyMap.Refresh):
			m.clearSelectedIncident(msg.String() + " (refresh)")
			m.setStatus(loadingIncidentsStatus)
			cmds = append(cmds, updateIncidentList(m.config))

		// In table mode, highlighted incidents are not selected yet, so they need to be retrieved
		// and then can be acted upon.  Since tea.Sequence does not wait for completion, the
		// "waitForSelectedIncidentsThen..." functions are used to wait for the selected incident
		// to be retrieved from PagerDuty
		case key.Matches(msg, defaultKeyMap.Enter):
			m.viewingIncident = true
			return m, doIfIncidentSelected(&m, func() tea.Msg {
				return waitForSelectedIncidentThenDoMsg{
					action: func() tea.Msg { return renderIncidentMsg("render") }, msg: "render",
				}
			})

		case key.Matches(msg, defaultKeyMap.Silence):
			return m, doIfIncidentSelected(&m, tea.Sequence(
				func() tea.Msg { return getIncidentMsg(incidentID) },
				func() tea.Msg {
					return waitForSelectedIncidentThenDoMsg{
						msg: "silence",
						action: func() tea.Msg {
							return silenceSelectedIncidentMsg{}
						},
					}
				},
			))

		case key.Matches(msg, defaultKeyMap.Ack):
			return m, doIfIncidentSelected(&m, tea.Sequence(
				func() tea.Msg { return getIncidentMsg(incidentID) },
				func() tea.Msg { return waitForSelectedIncidentsThenAcknowledgeMsg("Ack") },
			))

		case key.Matches(msg, defaultKeyMap.UnAck):
			return m, doIfIncidentSelected(&m, tea.Sequence(
				func() tea.Msg { return getIncidentMsg(incidentID) },
				func() tea.Msg { return waitForSelectedIncidentsThenUnAcknowledgeMsg("UnAck") },
			))

		case key.Matches(msg, defaultKeyMap.Note):
			return m, doIfIncidentSelected(&m, tea.Sequence(
				func() tea.Msg { return getIncidentMsg(incidentID) },
				func() tea.Msg {
					return waitForSelectedIncidentThenDoMsg{action: openEditorCmd(m.editor), msg: "add note"}
				},
			))

		case key.Matches(msg, defaultKeyMap.Login):
			return m, doIfIncidentSelected(&m, tea.Sequence(
				func() tea.Msg { return getIncidentMsg(incidentID) },
				func() tea.Msg {
					return waitForSelectedIncidentThenDoMsg{action: func() tea.Msg { return loginMsg("login") }, msg: "wait"}
				},
			))

		case key.Matches(msg, defaultKeyMap.Open):
			return m, doIfIncidentSelected(&m, tea.Sequence(
				func() tea.Msg { return getIncidentMsg(incidentID) },
				func() tea.Msg {
					return waitForSelectedIncidentThenDoMsg{action: func() tea.Msg { return openBrowserMsg("incident") }, msg: ""}
				},
			))

		}
	}
	return m, tea.Batch(cmds...)
}

func switchInputFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Debug("switchInputFocusMode", reflect.TypeOf(msg), msg)
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Help):
			m.toggleHelp()

		case key.Matches(msg, defaultKeyMap.Back):
			m.input.Blur()
			m.table.Focus()
			m.input.Prompt = defaultInputPrompt
			return m, nil

		case key.Matches(msg, defaultKeyMap.Enter):

		}
	}
	return m, tea.Batch(cmds...)
}

func switchIncidentFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Debug("switchIncidentFocusMode", reflect.TypeOf(msg), msg)
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Help):
			m.toggleHelp()

		// This un-sets the selected incident and returns to the table view
		case key.Matches(msg, defaultKeyMap.Back):
			m.clearSelectedIncident(msg.String() + " (back)")

		case key.Matches(msg, defaultKeyMap.Ack):
			return m, func() tea.Msg { return acknowledgeIncidentsMsg{incidents: []pagerduty.Incident{*m.selectedIncident}} }

		case key.Matches(msg, defaultKeyMap.Silence):
			return m, func() tea.Msg { return silenceSelectedIncidentMsg{} }

		case key.Matches(msg, defaultKeyMap.Note):
			cmds = append(cmds, openEditorCmd(m.editor))

		case key.Matches(msg, defaultKeyMap.Refresh):
			return m, func() tea.Msg { return getIncidentMsg(m.selectedIncident.ID) }

		case key.Matches(msg, defaultKeyMap.Login):
			return m, tea.Sequence(
				func() tea.Msg { return getIncidentMsg(m.selectedIncident.ID) },
				func() tea.Msg {
					return waitForSelectedIncidentThenDoMsg{action: func() tea.Msg { return loginMsg("login") }, msg: "wait"}
				},
			)
		}
	}

	m.incidentViewer, cmd = m.incidentViewer.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func switchErrorFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Debug("switchErrorFocusMode", reflect.TypeOf(msg), msg)
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Back):
			m.err = nil
		}
	}
	return m, nil
}
