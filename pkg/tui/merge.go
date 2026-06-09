package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/pd"
)

type mergeIncidentMsg struct{}

type mergedIncidentMsg struct {
	sourceID string
	targetID string
	err      error
}

func mergeIncidents(p *pd.Config, sourceID, targetID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := p.Client.MergeIncidentsWithContext(ctx,
			p.CurrentUser.Email, targetID,
			[]pagerduty.MergeIncidentsOptions{{ID: sourceID, Type: "incident_reference"}})
		return mergedIncidentMsg{sourceID, targetID, err}
	}
}

func filterMergeCandidates(incidents []pagerduty.Incident, excludeID string) []pagerduty.Incident {
	var result []pagerduty.Incident
	for _, inc := range incidents {
		if inc.ID != excludeID {
			result = append(result, inc)
		}
	}
	return result
}

func (m *model) rebuildMergeTable() {
	candidates := filterMergeCandidates(m.incidentList, m.mergeSourceIncident.ID)

	var rows []table.Row
	for _, i := range candidates {
		state := stateShorthand(i, m.config.CurrentUser.ID)
		if m.mergeTeamMode || AssignedToUser(i, m.config.CurrentUser.ID) {
			rows = append(rows, table.Row{state, i.ID, i.Title, i.Service.Summary})
		}
	}

	m.mergeTable.SetStyles(m.styles.Table)
	m.mergeTable.SetColumns(m.table.Columns())
	m.mergeTable.SetRows(rows)
	m.mergeTable.SetHeight(m.table.Height())
	m.mergeTable.Focus()
}

func switchMergeFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Quit):
			return m, tea.Quit

		case key.Matches(msg, defaultKeyMap.Back):
			m.mergeMode = false
			m.mergeSourceIncident = nil
			m.table.Focus()
			return m, nil

		case key.Matches(msg, defaultKeyMap.Team):
			m.mergeTeamMode = !m.mergeTeamMode
			m.rebuildMergeTable()
			return m, nil

		case key.Matches(msg, defaultKeyMap.Enter):
			selectedRow := m.mergeTable.SelectedRow()
			if len(selectedRow) < 2 {
				m.setStatus("no incident selected")
				return m, nil
			}
			targetID := selectedRow[1]
			sourceID := m.mergeSourceIncident.ID
			m.pendingConfirmation = &confirmActionState{
				prompt: fmt.Sprintf("Merge %s into %s? [y/n]", sourceID, targetID),
				action: func() tea.Msg {
					return mergeIncidentMsg{}
				},
			}
			m.mergeTargetID = targetID
			return m, nil

		default:
			var cmd tea.Cmd
			m.mergeTable, cmd = m.mergeTable.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}
