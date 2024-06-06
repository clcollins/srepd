package tui

import (
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
	top, _, bottom, _ := mainStyle.GetMargin()
	eighthWindow := windowSize.Width / 8
	cellPadding := (horizontalPadding * 2) * 4
	borderEdges := 2 + 10

	m.help.Width = windowSize.Width - borderEdges

	m.table.SetColumns([]table.Column{
		{Title: dot, Width: 1},
		{Title: "ID", Width: eighthWindow + cellPadding - borderEdges},
		{Title: "Summary", Width: eighthWindow * 3},
		{Title: "Service", Width: eighthWindow * 3},
	})

	height := windowSize.Height - top - bottom - 10
	m.table.SetHeight(height)
	m.incidentViewer.Width = windowSize.Width - borderEdges
	m.incidentViewer.Height = height

	return m, nil
}

func (m model) keyMsgHandler(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key.Matches(msg.(tea.KeyMsg), defaultKeyMap.Quit) {
		return m, tea.Quit
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
				func() tea.Msg { return waitForSelectedIncidentsThenAcknowledgeMsg("wait") },
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
			m.viewingIncident = false
			m.selectedIncident = nil
			m.selectedIncidentAlerts = nil
			m.selectedIncidentNotes = nil

		case key.Matches(msg, defaultKeyMap.Ack):
			return m, func() tea.Msg { return acknowledgeIncidentsMsg{incidents: []*pagerduty.Incident{m.selectedIncident}} }

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
