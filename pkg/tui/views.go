package tui

import (
	"bytes"
	"fmt"
	"html/template"
	"slices"
	"strings"
	"time"

	"charm.land/glamour/v2"
	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/alert"
	"github.com/clcollins/srepd/pkg/ocm"
)

const (
	dot       = "•"
	upArrow   = "↑"
	downArrow = "↓"

	dotWidth = 1
	idWidth  = 16
)

var (
	windowSize tea.WindowSizeMsg
)

func (m model) View() string {
	var s strings.Builder

	// errHelpView := helpStyle.Render(help.New().View(errorViewKeyMap))
	errHelpView := help.New().View(errorViewKeyMap)

	s.WriteString(m.renderHeader())

	switch {
	case m.err != nil:
		log.Debug("View", "error", m.err)

		s.WriteString(dot)
		s.WriteString("ERROR")
		s.WriteString(dot)
		s.WriteString("\n\n")
		s.WriteString(m.err.Error())
		s.WriteString("\n")
		s.WriteString(errHelpView)

		return m.styles.Error.Render(s.String())

	case m.viewingLog:
		s.WriteString(m.styles.TableContainer.Render(m.logViewer.View()))

	case m.configMode:
		s.WriteString(m.configForm.View())

	case m.configModeRequested:
		s.WriteString("  Loading configuration...")

	case m.bulkSilenceMode:
		s.WriteString(m.bulkSilenceForm.View())

	case m.teamSelectMode:
		s.WriteString(m.teamSelectForm.View())

	case m.clusterSelectMode:
		s.WriteString("  " + m.clusterSelectPrompt + "\n")
		s.WriteString(m.styles.TableContainer.Render(m.clusterSelectTable.View()))

	case m.mergeMode:
		fmt.Fprintf(&s, "  Select incident to merge %s into (Enter=select, Esc=cancel, t=toggle team):\n", m.mergeSourceIncident.ID)
		s.WriteString(m.styles.TableContainer.Render(m.mergeTable.View()))

	case m.viewingIncident:
		tabBar := m.renderTabBar()
		s.WriteString(tabBar)
		s.WriteString("\n")
		tabWindowBorders := m.styles.TabWindow.GetHorizontalBorderSize()
		tabWindowWidth := windowSize.Width - m.styles.Main.GetHorizontalBorderSize() - m.styles.Main.GetHorizontalMargins() - tabWindowBorders
		s.WriteString(m.styles.TabWindow.Width(tabWindowWidth).Render(m.incidentViewer.View()))

	default:
		s.WriteString(m.styles.TableContainer.Render(m.table.View()))
		s.WriteString("\n")
		s.WriteString(m.renderFooter())
		s.WriteString("\n")
		s.WriteString(m.renderWatcherPane())
		if m.input.Focused() {
			s.WriteString(m.input.View())
		} else {
			s.WriteString("")
		}
	}

	// Choose the appropriate keymap based on focus mode — skip srepd help in config mode
	// (the huh form renders its own help internally)
	var helpView string
	if m.configMode || m.configModeRequested {
		helpView = ""
	} else {
		var helpKeyMap help.KeyMap
		if m.chordHelpActive {
			helpKeyMap = chordKeymap{prefix: m.chordPrefix}
		} else if m.input.Focused() {
			helpKeyMap = inputModeKeyMap
		} else {
			helpKeyMap = defaultKeyMap
		}
		helpView = m.styles.Padded.Width(windowSize.Width).Render(m.help.View(helpKeyMap))
	}

	// Calculate how many newlines needed to push help and bottom status to terminal bottom
	// Count lines in the rendered output so far
	contentLines := strings.Count(s.String(), "\n") + 1 // +1 because first line doesn't have \n

	// Count help lines
	helpLines := strings.Count(helpView, "\n") + 1

	// Calculate how many lines we need to add to reach the bottom
	// -1 for the bottom status line itself, -helpLines for the help text
	linesToBottom := windowSize.Height - contentLines - helpLines - 1

	if linesToBottom > 0 {
		for i := 0; i < linesToBottom; i++ {
			s.WriteString("\n")
		}
	}

	// Add help one line above bottom status
	s.WriteString(helpView)
	s.WriteString("\n")

	// Add bottom status line at terminal bottom
	s.WriteString(m.renderBottomStatus())

	return m.styles.Main.Render(s.String())
}

func (m model) renderFooter() string {
	left := refreshArea(m.autoRefresh, m.autoAcknowledge, m.showLowUrgency)

	if !m.watcherExpanded {
		return m.styles.Padded.Render(left)
	}

	right := m.renderWatcherStatus()
	renderedRight := m.styles.Muted.Render(right)
	rightWidth := lipgloss.Width(renderedRight)
	leftWidth := windowSize.Width - rightWidth - m.styles.Padded.GetHorizontalPadding() - m.styles.Padded.GetHorizontalBorderSize()

	return lipgloss.JoinHorizontal(
		0.2,
		m.styles.Padded.Width(leftWidth).Render(left),
		renderedRight,
	)
}

func (m model) renderHeader() string {
	var s strings.Builder
	var assignedTo string

	assignedTo = "You"

	if m.teamMode {
		assignedTo = "Team"
	}

	var statusContent string
	if m.pendingConfirmation != nil {
		statusContent = m.styles.Warning.Render(m.pendingConfirmation.prompt)
	} else {
		statusContent = statusArea(m.status, m.apiInProgress, m.spinner.View(), m.theme.Text)
	}

	leftWidth := windowSize.Width * 4 / 6
	rightWidth := windowSize.Width - leftWidth

	s.WriteString(
		lipgloss.JoinHorizontal(
			0.2,
			m.styles.Padded.Width(leftWidth).Render(statusContent),
			m.styles.Padded.Width(rightWidth).Align(lipgloss.Right).Render(assigneeArea(assignedTo)),
		),
	)

	s.WriteString("\n")
	return s.String()
}

func (m model) renderBottomStatus() string {
	var s strings.Builder
	var selectedID string

	if m.selectedIncident != nil {
		selectedID = m.selectedIncident.ID
	}

	if m.updateAvailable {
		versionDisplay := updateString(Version, m.updateVersion)

		updateNotice := fmt.Sprintf("An update is available: %s", m.updateVersion)
		updateStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(m.theme.Highlight).
			Background(m.theme.Selected).
			Padding(0, 1).
			Align(lipgloss.Center)

		sideWidth := windowSize.Width / 6
		centerWidth := windowSize.Width - (sideWidth * 2)

		s.WriteString(
			lipgloss.JoinHorizontal(
				0.2,
				m.styles.Muted.Width(sideWidth).Padding(0, 0, 0, 1).Render(selectedID),
				updateStyle.Width(centerWidth).Render(updateNotice),
				m.styles.Muted.Width(sideWidth).Padding(0, 1, 0, 0).Align(lipgloss.Right).Render(versionDisplay),
			),
		)
	} else {
		versionDisplay := versionString()
		rightCol := m.styles.Padded.Render(versionDisplay)
		rightWidth := lipgloss.Width(rightCol)

		s.WriteString(
			lipgloss.JoinHorizontal(
				0.2,
				m.styles.Muted.Width(windowSize.Width-rightWidth-m.styles.Padded.GetHorizontalPadding()-m.styles.Padded.GetHorizontalBorderSize()).Padding(0, 2, 0, 1).Render(selectedID),
				rightCol,
			),
		)
	}

	return s.String()
}

func assigneeArea(s string) string {
	var fstring = "Showing assigned to " + s
	fstring = strings.TrimSuffix(fstring, "\n")

	return fstring
}

func statusArea(s string, showSpinner bool, spinnerView string, textColor lipgloss.AdaptiveColor) string {
	if showSpinner {
		statusStyle := lipgloss.NewStyle().Foreground(textColor)
		return fmt.Sprintf("%s %s", spinnerView, statusStyle.Render(s))
	}

	var prefix = ">"
	var fstring = "%s %s"
	fstring = strings.TrimSuffix(fstring, "\n")

	return fmt.Sprintf(fstring, prefix, s)
}

func refreshArea(autoRefresh bool, autoAck bool, showLowUrgency bool) string {
	var fstring = "Watching for updates... "

	if autoRefresh && autoAck {
		fstring = fstring + " [auto-acknowledge]"
	} else if !autoRefresh {
		fstring = fstring + " [PAUSED]"
	}

	if !showLowUrgency {
		fstring = fstring + " [high urgency only]"
	}

	fstring = strings.TrimSuffix(fstring, "\n")
	return fstring
}

func (m model) renderTabContent() (string, error) {
	var alerts []pagerduty.IncidentAlert
	var notes []pagerduty.IncidentNote

	if m.incidentAlertsLoaded {
		alerts = m.selectedIncidentAlerts
	}
	if m.incidentNotesLoaded {
		notes = m.selectedIncidentNotes
	}

	summary := summarize(m.selectedIncident, alerts, notes, m.clusterCache)
	summary.AlertsLoading = !m.incidentAlertsLoaded
	summary.NotesLoading = !m.incidentNotesLoaded

	switch m.activeTab {
	case tabDetails:
		return m.renderDetailsTab(summary)
	case tabAlerts:
		return m.renderAlertsTab(summary)
	case tabNotes:
		return m.renderNotesTab(summary)
	case tabCluster:
		return m.renderClusterTab()
	case tabServiceLogs:
		return m.renderServiceLogsTab()
	case tabLimitedSupport:
		return m.renderLimitedSupportTab()
	}
	return "", nil
}

func (m model) renderDetailsTab(summary incidentSummary) (string, error) {
	tmpl, err := template.New("details").Funcs(funcMap).Parse(detailsTabTemplate)
	if err != nil {
		return "", err
	}
	o := new(bytes.Buffer)
	err = tmpl.Execute(o, summary)
	if err != nil {
		return "", err
	}

	content := o.String()

	// Add impacted clusters section if we have cluster data
	clusterIDs := getUniqueClusters(m.selectedIncidentAlerts)
	if len(clusterIDs) > 0 {
		var clusters strings.Builder
		clusters.WriteString("\n## Impacted Clusters:\n\n")
		for _, id := range clusterIDs {
			if info, ok := m.clusterCache[id]; ok {
				displayName := info.DisplayName
				if displayName == "" {
					displayName = info.Name
				}
				fmt.Fprintf(&clusters, "* %s (%s)\n", displayName, id)
			} else {
				fmt.Fprintf(&clusters, "* %s\n", id)
			}
		}
		content += clusters.String()
	}

	content += m.renderFlagConditionsSection()

	return content, nil
}

func (m model) renderAlertsTab(summary incidentSummary) (string, error) {
	if summary.AlertsLoading {
		return "\n_Loading alerts..._\n", nil
	}
	if len(summary.Alerts) == 0 {
		return "\n_No alerts_\n", nil
	}

	var content strings.Builder
	for i, a := range summary.Alerts {
		tmpl, err := template.New("alert").Funcs(funcMap).Parse(alertTabTemplate)
		if err != nil {
			return "", err
		}
		data := struct {
			Alert alertSummary
			Index int
			Total int
		}{Alert: a, Index: i, Total: len(summary.Alerts)}
		o := new(bytes.Buffer)
		err = tmpl.Execute(o, data)
		if err != nil {
			return "", err
		}
		content.WriteString(o.String())
		if i < len(summary.Alerts)-1 {
			content.WriteString("\n---\n")
		}
	}
	return content.String(), nil
}

func (m model) renderNotesTab(summary incidentSummary) (string, error) {
	if summary.NotesLoading {
		return "\n_Loading notes..._\n", nil
	}
	if len(summary.Notes) == 0 {
		return "\n_No notes_\n", nil
	}

	sorted := make([]noteSummary, len(summary.Notes))
	copy(sorted, summary.Notes)
	slices.SortFunc(sorted, func(a, b noteSummary) int {
		return strings.Compare(b.Created, a.Created)
	})

	var content strings.Builder
	for i, n := range sorted {
		tmpl, err := template.New("note").Funcs(funcMap).Parse(noteTabTemplate)
		if err != nil {
			return "", err
		}
		data := struct {
			Note  noteSummary
			Index int
			Total int
		}{Note: n, Index: i, Total: len(summary.Notes)}
		o := new(bytes.Buffer)
		err = tmpl.Execute(o, data)
		if err != nil {
			return "", err
		}
		content.WriteString(o.String())
		if i < len(summary.Notes)-1 {
			content.WriteString("\n---\n")
		}
	}
	return content.String(), nil
}

func (m model) renderClusterTab() (string, error) {
	if len(m.clusterCache) == 0 {
		if m.ocmClient == nil {
			if m.ocmAuthPending {
				return "\n_OCM authenticating — complete login in browser..._\n", nil
			}
			return "\n_OCM not connected_\n", nil
		}
		return "\n_Loading cluster info..._\n", nil
	}

	var content strings.Builder
	clusters := m.sortedClusterIDs()
	for i, id := range clusters {
		info := m.clusterCache[id]
		fmt.Fprintf(&content, "### Cluster %d/%d\n\n", i+1, len(clusters))
		fmt.Fprintf(&content, "* Name: %s\n", info.Name)
		fmt.Fprintf(&content, "* Display Name: %s\n", info.DisplayName)
		fmt.Fprintf(&content, "* ID: %s\n", info.ID)
		fmt.Fprintf(&content, "* External ID: %s\n", info.ExternalID)
		fmt.Fprintf(&content, "* State: %s\n", info.State)
		fmt.Fprintf(&content, "* Region: %s\n", info.Region)
		fmt.Fprintf(&content, "* Cloud Provider: %s\n", info.CloudProvider)
		fmt.Fprintf(&content, "* Version: %s\n", info.Version)
		fmt.Fprintf(&content, "* Hypershift: %v\n", info.Hypershift)
		fmt.Fprintf(&content, "* CCS: %v\n", info.CCS)
		fmt.Fprintf(&content, "* Organization: %s\n", info.Organization)
		if i < len(clusters)-1 {
			content.WriteString("\n---\n")
		}
	}
	return content.String(), nil
}

func (m model) renderServiceLogsTab() (string, error) {
	if len(m.serviceLogCache) == 0 {
		if m.ocmClient == nil {
			if m.ocmAuthPending {
				return "\n_OCM authenticating — complete login in browser..._\n", nil
			}
			return "\n_OCM not connected_\n", nil
		}
		return "\n_Loading service logs..._\n", nil
	}

	var allLogs []ocm.ServiceLog
	for _, id := range m.sortedClusterIDs() {
		allLogs = append(allLogs, m.serviceLogCache[id]...)
	}
	slices.SortFunc(allLogs, func(a, b ocm.ServiceLog) int {
		return strings.Compare(b.Timestamp, a.Timestamp)
	})

	total := len(allLogs)
	var content strings.Builder
	for i, l := range allLogs {
		fmt.Fprintf(&content, "### Service Log %d/%d\n\n", i+1, total)
		fmt.Fprintf(&content, "* Severity: %s\n", l.Severity)
		fmt.Fprintf(&content, "* Service: %s\n", l.ServiceName)
		fmt.Fprintf(&content, "* Timestamp: %s\n", l.Timestamp)
		fmt.Fprintf(&content, "* Summary: %s\n", l.Summary)
		if l.Description != "" {
			fmt.Fprintf(&content, "\n> %s\n", l.Description)
		}
		if i < total-1 {
			content.WriteString("\n---\n")
		}
	}
	return content.String(), nil
}

func (m model) renderLimitedSupportTab() (string, error) {
	if len(m.limitedSupportCache) == 0 {
		if m.ocmClient == nil {
			if m.ocmAuthPending {
				return "\n_OCM authenticating — complete login in browser..._\n", nil
			}
			return "\n_OCM not connected_\n", nil
		}
		return "\n_No limited support history_\n", nil
	}

	var allReasons []ocm.LimitedSupportReason
	for _, id := range m.sortedClusterIDs() {
		allReasons = append(allReasons, m.limitedSupportCache[id]...)
	}

	if len(allReasons) == 0 {
		return "\n_No limited support history_\n", nil
	}

	slices.SortFunc(allReasons, func(a, b ocm.LimitedSupportReason) int {
		return strings.Compare(b.CreatedAt, a.CreatedAt)
	})

	total := len(allReasons)
	var content strings.Builder
	for i, r := range allReasons {
		fmt.Fprintf(&content, "### Limited Support %d/%d\n\n", i+1, total)
		fmt.Fprintf(&content, "* Summary: %s\n", r.Summary)
		fmt.Fprintf(&content, "* Detection: %s\n", r.DetectionType)
		fmt.Fprintf(&content, "* Created: %s\n", r.CreatedAt)
		if r.Details != "" {
			fmt.Fprintf(&content, "\n> %s\n", r.Details)
		}
		if i < total-1 {
			content.WriteString("\n---\n")
		}
	}
	return content.String(), nil
}

func (m model) sortedClusterIDs() []string {
	if m.selectedIncident == nil {
		return nil
	}
	incidentClusters := m.incidentClusterMap[m.selectedIncident.ID]
	var ids []string
	for _, id := range incidentClusters {
		if _, ok := m.clusterCache[id]; ok {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

func addNoteTemplate(id string, title string, service string) (string, error) {
	template, err := template.New("note").Parse(noteTemplate)
	if err != nil {
		return "", err
	}

	o := new(bytes.Buffer)
	err = template.Execute(o, struct {
		ID      string
		Title   string
		Service string
	}{
		ID:      id,
		Title:   title,
		Service: service,
	})
	if err != nil {
		return "", err
	}

	return o.String(), nil
}

func summarize(i *pagerduty.Incident, a []pagerduty.IncidentAlert, n []pagerduty.IncidentNote, clusterCache map[string]*ocm.ClusterInfo) incidentSummary {
	summary := summarizeIncident(i)
	summary.Alerts = summarizeAlerts(a, clusterCache)
	summary.Notes = summarizeNotes(n)
	return summary
}

type noteSummary struct {
	ID      string
	User    string
	Content string
	Created string
}

func summarizeNotes(n []pagerduty.IncidentNote) []noteSummary {
	var s []noteSummary

	for _, note := range n {
		s = append(s, noteSummary{
			ID:      note.ID,
			User:    note.User.Summary,
			Content: note.Content,
			Created: note.CreatedAt,
		})
	}

	return s
}

type alertSummary struct {
	ID          string
	Name        string
	Link        string
	HTMLURL     string
	Service     string
	Created     string
	Status      string
	Incident    string
	Cluster     string
	Severity    string
	Tags        []string
	AlertType   string
	Namespace   string
	Description string
	ClusterName string
}

func summarizeAlerts(a []pagerduty.IncidentAlert, clusterCache map[string]*ocm.ClusterInfo) []alertSummary {
	var s []alertSummary

	for _, alt := range a {
		// Use the alert normalization engine to extract structured fields
		serviceSummary := alt.Service.Summary
		// For the title, we don't have access to the incident title here,
		// so we use an empty string. The incident title is available at the
		// incident level (incidentSummary.Title), not the alert level.
		normalized := alert.NormalizeAlert(serviceSummary, "", alt)

		// Fall back to raw detail extraction if normalization yielded empty fields
		name := normalized.AlertName
		if name == "" {
			name = getDetailFieldFromAlert("alert_name", alt)
		}

		cluster := normalized.ClusterID
		if cluster == "" {
			cluster = getDetailFieldFromAlert("cluster_id", alt)
		}

		link := normalized.SOPLink
		if link == "" {
			link = getDetailFieldFromAlert("link", alt)
		}

		var clusterName string
		if clusterCache != nil && cluster != "" {
			if info, ok := clusterCache[cluster]; ok {
				clusterName = info.DisplayName
				if clusterName == "" {
					clusterName = info.Name
				}
			}
		}

		s = append(s, alertSummary{
			ID:          alt.ID,
			Name:        name,
			Link:        link,
			Cluster:     cluster,
			ClusterName: clusterName,
			HTMLURL:     alt.HTMLURL,
			Service:     alt.Service.Summary,
			Created:     alt.CreatedAt,
			Status:      alt.Status,
			Incident:    alt.Incident.ID,
			Severity:    normalized.Severity,
			Tags:        normalized.Tags,
			AlertType:   normalized.AlertType,
			Namespace:   normalized.Namespace,
			Description: normalized.Description,
		})

	}

	return s
}

type incidentSummary struct {
	ID               string
	Title            string
	HTMLURL          string
	Service          string
	EscalationPolicy string
	Created          string
	Urgency          string
	Priority         string
	Status           string
	Teams            []string
	Assigned         []string
	Acknowledged     []string
	Alerts           []alertSummary
	Notes            []noteSummary
	Clusters         []string
	// Progressive rendering flags
	AlertsLoading bool
	NotesLoading  bool
}

func summarizeIncident(i *pagerduty.Incident) incidentSummary {
	var s incidentSummary

	s.ID = i.ID
	s.Title = i.Title
	s.HTMLURL = i.HTMLURL
	s.Service = i.Service.Summary
	s.EscalationPolicy = i.EscalationPolicy.Summary
	s.Created = i.CreatedAt
	s.Urgency = i.Urgency
	s.Status = i.Status

	if i.Priority != nil {
		s.Priority = i.Priority.Summary
	}

	for _, team := range i.Teams {
		s.Teams = append(s.Teams, team.Summary)
	}
	for _, asn := range i.Assignments {
		s.Assigned = append(s.Assigned, asn.Assignee.Summary)
	}

	for _, ack := range i.Acknowledgements {
		// Suppress multiple acknowledgements by the same person being shown in the summary
		if !slices.Contains(s.Acknowledged, ack.Acknowledger.Summary) {
			s.Acknowledged = append(s.Acknowledged, ack.Acknowledger.Summary)
		}
	}

	return s
}

var funcMap = template.FuncMap{
	"Truncate": func(s string) string {
		return fmt.Sprintf("%s ...", s[:5])
	},
	"ToLink": func(s, link string) string {
		return fmt.Sprintf("[%s](%s)", s, link)
	},
	"ToUpper": strings.ToUpper,
	"Last": func(i, length int) bool {
		return i == length-1
	},
	"add1": func(i int) int {
		return i + 1
	},
	"SOPName": func(url string) string {
		parts := strings.Split(url, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
		return url
	},
	"blockquote": func(s string) template.HTML {
		lines := strings.Split(s, "\n")
		for i, line := range lines {
			lines[i] = "> " + template.HTMLEscapeString(line)
		}
		return template.HTML(strings.Join(lines, "\n"))
	},
}

func renderIncidentMarkdown(m *model, content string) (string, error) {
	wrapWidth := m.incidentViewer.Width
	if wrapWidth < 40 {
		wrapWidth = 100
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(m.styles.GlamourStyle),
		glamour.WithWordWrap(wrapWidth),
	)
	if err != nil {
		return content, nil
	}

	str, err := renderer.Render(content)
	if err != nil {
		return str, err
	}

	return str, nil
}

const (
	tabDetails        = 0
	tabAlerts         = 1
	tabNotes          = 2
	tabCluster        = 3
	tabServiceLogs    = 4
	tabLimitedSupport = 5
	tabCount          = 6
)

func tabBorderWithBottom(left, middle, right string) lipgloss.Border {
	border := lipgloss.RoundedBorder()
	border.BottomLeft = left
	border.Bottom = middle
	border.BottomRight = right
	return border
}

func (m model) renderTabBar() string {
	tabLabels := make([]string, tabCount)
	tabLabels[tabDetails] = "Details"

	if !m.incidentAlertsLoaded {
		tabLabels[tabAlerts] = "Alerts (...)"
	} else {
		tabLabels[tabAlerts] = fmt.Sprintf("Alerts (%d)", len(m.selectedIncidentAlerts))
	}

	if !m.incidentNotesLoaded {
		tabLabels[tabNotes] = "Notes (...)"
	} else {
		tabLabels[tabNotes] = fmt.Sprintf("Notes (%d)", len(m.selectedIncidentNotes))
	}

	// Scope OCM tab counts to the current incident's clusters
	var incidentClusters []string
	if m.selectedIncident != nil {
		incidentClusters = m.incidentClusterMap[m.selectedIncident.ID]
	}

	clusterCount := 0
	for _, id := range incidentClusters {
		if _, ok := m.clusterCache[id]; ok {
			clusterCount++
		}
	}
	tabLabels[tabCluster] = fmt.Sprintf("Cluster (%d)", clusterCount)

	logCount := 0
	for _, id := range incidentClusters {
		logCount += len(m.serviceLogCache[id])
	}
	tabLabels[tabServiceLogs] = fmt.Sprintf("SLs (%d)", logCount)

	lsCount := 0
	for _, id := range incidentClusters {
		lsCount += len(m.limitedSupportCache[id])
	}
	tabLabels[tabLimitedSupport] = fmt.Sprintf("LS History (%d)", lsCount)

	var renderedTabs []string
	for i, label := range tabLabels {
		var style lipgloss.Style
		isFirst, isActive := i == 0, i == m.activeTab
		if isActive {
			style = m.styles.ActiveTab
		} else {
			style = m.styles.InactiveTab
		}
		border, _, _, _, _ := style.GetBorder()
		if isFirst && isActive {
			border.BottomLeft = "│"
		} else if isFirst && !isActive {
			border.BottomLeft = "├"
		}
		style = style.Border(border)
		renderedTabs = append(renderedTabs, style.Render(label))
	}

	tabRow := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
	tabWidth := lipgloss.Width(tabRow)
	remainingWidth := windowSize.Width - tabWidth - 1
	if remainingWidth > 0 {
		gapBorder := lipgloss.Border{Bottom: "─", Right: " ", BottomRight: "┐"}
		gap := lipgloss.NewStyle().
			BorderForeground(m.theme.Border).
			Border(gapBorder, false, true, true, false).
			Width(remainingWidth).
			Render(" ")
		tabRow = lipgloss.JoinHorizontal(lipgloss.Bottom, tabRow, gap)
	}

	return tabRow
}

// detailsTabTemplate renders only the incident metadata (no alerts or notes)
const detailsTabTemplate = `
{{- if .Priority -}}
# {{ .Priority }} {{ .ID }} - {{ .Status }}
{{- else -}}
# {{ .ID }} - {{ .Status }}
{{- end }}

{{ ToLink .Title .HTMLURL }}

* Service: {{ .Service }}
{{- if .Teams }}
* Team: {{ range $i, $team := .Teams }}{{ if $i }}, {{ end }}{{ $team }}{{ end }}
{{- end }}
* Urgency: {{ .Urgency }}
* Created: {{ .Created }}

{{ if not .Acknowledged -}}
Assigned to:{{ range $assignee := .Assigned }}
+ *{{ $assignee }}* *(not yet acknowledged)*
{{ end }}
{{- else -}}
{{ $length := len .Acknowledged }}
Acknowledged by: {{ range $i, $ack := .Acknowledged -}}
{{ if Last $i $length }}**{{ $ack }}**{{ else }}**{{ $ack }},**{{ end }}
{{ end }}
{{- end -}}
`

const alertTabTemplate = `
### Alert {{ add1 .Index }}/{{ .Total }}

**{{ .Alert.ID }}** ({{ .Alert.Status }}){{ if .Alert.Severity }} [{{ .Alert.Severity }}]{{ end }}{{ if .Alert.AlertType }} ({{ .Alert.AlertType }}){{ end }}

{{ if .Alert.HTMLURL }}{{ if .Alert.Name }}{{ ToLink .Alert.Name .Alert.HTMLURL }}{{ else }}{{ ToLink .Alert.ID .Alert.HTMLURL }}{{ end }}{{ else }}{{ if .Alert.Name }}{{ .Alert.Name }}{{ else }}{{ .Alert.ID }}{{ end }}{{ end }}

* Cluster: {{ if .Alert.ClusterName }}{{ .Alert.ClusterName }} ({{ .Alert.Cluster }}){{ else }}{{ .Alert.Cluster }}{{ end }}
{{ if .Alert.Namespace }}* Namespace: {{ .Alert.Namespace }}
{{ end -}}
* SOP: {{ if .Alert.Link }}{{ ToLink (SOPName .Alert.Link) .Alert.Link }}{{ else }}_none_{{ end }}
* Service: {{ .Alert.Service }}
* Created: {{ .Alert.Created }}
{{ if .Alert.Description }}
> {{ .Alert.Description }}
{{ end -}}
`

// noteTabTemplate renders a single note with navigation header (kept for backward compatibility)
const noteTabTemplate = `
### Note {{ add1 .Index }}/{{ .Total }}

{{ blockquote .Note.Content }}

-- {{ .Note.User }} @ {{ .Note.Created }}
`

const noteTemplate = `

# Please enter the note message content above. Lines starting
# with '#' will be ignored and an empty message aborts the note.
#
# Incident: {{ .ID }}
# Summary: {{ .Title }}
# Service: {{ .Service }}
#
`

func (m model) renderWatcherPane() string {
	if !m.watcherExpanded {
		return ""
	}

	return m.styles.WatcherContainer.Render(m.watcherViewport.View()) + "\n"
}

func (m model) renderWatcherStatus() string {
	parts := []string{"[AI Watcher]"}

	if m.aiProvider != nil {
		parts = append(parts, m.aiProvider.Name())
		if m.aiHealthy {
			parts = append(parts, "healthy")
		} else {
			parts = append(parts, "offline")
		}
	}

	if m.claudeQuerying || m.watcherAnalyzing {
		remaining := m.watcherQueryTimeout - time.Since(m.watcherQueryStart).Truncate(time.Second)
		if remaining < 0 {
			remaining = 0
		}
		highlight := lipgloss.NewStyle().Foreground(m.theme.Highlight)
		parts = append(parts, m.spinner.View()+" "+highlight.Render(fmt.Sprintf("analyzing... %s", remaining)))
	} else {
		parts = append(parts, "idle")
	}

	return strings.Join(parts, " | ")
}
