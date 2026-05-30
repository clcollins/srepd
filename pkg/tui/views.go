package tui

import (
	"bytes"
	"fmt"
	"html/template"
	"slices"
	"strings"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/alert"
)

const (
	dot       = "•"
	upArrow   = "↑"
	downArrow = "↓"

	dotWidth = 1
	idWidth  = 16
)

var (
	white          = lipgloss.AdaptiveColor{Dark: "#ffffff", Light: "#ffffff"}
	lightBlue      = lipgloss.AdaptiveColor{Dark: "#778da9", Light: "#778da9"}
	blue           = lipgloss.AdaptiveColor{Dark: "#415a77", Light: "#415a77"}
	backgroundBlue = lipgloss.AdaptiveColor{Dark: "#0d1b2a", Light: "#0d1b2a"}
	backgroundRed  = lipgloss.AdaptiveColor{Dark: "#a4133c", Light: "#a4133c"}

	// For future
	// gray           = lipgloss.AdaptiveColor{Dark: "#e0e1dd", Light: "#e0e1dd"}
	// darkBlue       = lipgloss.AdaptiveColor{Dark: "#1b263b", Light: "#1b263b"}
)

type pallet struct {
	text       lipgloss.AdaptiveColor
	background lipgloss.AdaptiveColor
	border     lipgloss.AdaptiveColor
}

type colorModel struct {
	normal   pallet
	notice   pallet
	warning  pallet
	selected pallet
	err      pallet
}

var srepdPallet = colorModel{
	normal: pallet{
		text:       lightBlue,
		background: lipgloss.AdaptiveColor{},
		border:     blue,
	},
	notice: pallet{
		text:       white,
		background: lipgloss.AdaptiveColor{},
		border:     lipgloss.AdaptiveColor{},
	},
	warning: pallet{
		text:       white,
		background: backgroundRed,
		border:     lipgloss.AdaptiveColor{},
	},
	selected: pallet{
		text:       white,
		background: blue,
		border:     blue,
	},
	err: pallet{
		text:       white,
		background: backgroundBlue,
		border:     blue,
	},
}

var (
	windowSize tea.WindowSizeMsg

	mainStyle = lipgloss.NewStyle().
			Width(windowSize.Width).
			Height(windowSize.Height).
			Margin(0, 0).
			Padding(0, 0).
			Foreground(srepdPallet.normal.text).
			Background(srepdPallet.normal.background).
			BorderForeground(srepdPallet.normal.border).
			BorderBackground(srepdPallet.normal.background)

	assignedStringWidth = len("Showing assigned to User") + 2

	paddedStyle = mainStyle.Padding(0, 2, 0, 1)

	mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#5C5C5C"})

	//lint:ignore U1000 - future proofing
	warningStyle = lipgloss.NewStyle().Foreground(srepdPallet.warning.text).Background(srepdPallet.warning.background)

	tableContainerStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), true)
	tableCellStyle      = lipgloss.NewStyle().Padding(0, 1)
	tableHeaderStyle    = lipgloss.NewStyle().Padding(0, 1).Border(lipgloss.RoundedBorder(), false, false, true).Foreground(srepdPallet.notice.text).Background(srepdPallet.notice.background)
	tableSelectedStyle  = lipgloss.NewStyle().Border(lipgloss.HiddenBorder(), false).Background(srepdPallet.selected.background).Foreground(srepdPallet.selected.text).Bold(true)

	tableStyle = table.Styles{
		Cell:     tableCellStyle,
		Selected: tableSelectedStyle,
		Header:   tableHeaderStyle,
	}

	// Action log table styles - header with border like main table, no selection highlight
	actionLogTableStyle = table.Styles{
		Cell:     tableCellStyle,
		Selected: tableCellStyle,   // Same as cell style = no highlight
		Header:   tableHeaderStyle, // Same header style as main table (with border)
	}

	// Action log container style - border like main table
	actionLogContainerStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), true)

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Width(64).
			Border(lipgloss.RoundedBorder()).
			Foreground(srepdPallet.err.text).
			Background(srepdPallet.err.background).
			BorderForeground(srepdPallet.err.border).
			Padding(1, 3, 1, 3)
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

		return errorStyle.Render(s.String())

	case m.viewingLog:
		s.WriteString(tableContainerStyle.Render(m.logViewer.View()))

	case m.viewingIncident:
		s.WriteString(tableContainerStyle.Render(m.incidentViewer.View()))

	default:
		s.WriteString(tableContainerStyle.Render(m.table.View()))
		s.WriteString("\n")
		// Render refresh status line immediately below main table
		s.WriteString(m.renderFooter())
		s.WriteString("\n")
		// Input field always reserves a line (empty if not focused)
		if m.input.Focused() {
			s.WriteString(m.input.View())
		} else {
			s.WriteString("") // Preserve empty line when input not focused
		}
		// Only render action log if it's toggled on
		if m.showActionLog {
			s.WriteString("\n")
			// Render action log table with border like main table
			s.WriteString(actionLogContainerStyle.Render(m.actionLogTable.View()))
		}
	}

	// Choose the appropriate keymap based on focus mode
	var helpKeyMap help.KeyMap
	if m.input.Focused() {
		helpKeyMap = inputModeKeyMap
	} else {
		helpKeyMap = defaultKeyMap
	}

	// Render help separately so we can count its lines
	helpView := paddedStyle.Width(windowSize.Width).Render(m.help.View(helpKeyMap))

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

	return mainStyle.Render(s.String())
}

func (m model) renderFooter() string {
	var s strings.Builder
	s.WriteString(
		lipgloss.JoinHorizontal(
			0.2,
			paddedStyle.Render(refreshArea(m.autoRefresh, m.autoAcknowledge, m.showLowUrgency)),
		),
	)

	return s.String()
}

func (m model) renderHeader() string {
	var s strings.Builder
	var assignedTo string

	assignedTo = "You"

	if m.teamMode {
		assignedTo = "Team"
	}

	// When a cluster selection or confirmation prompt is active, show it in the
	// status area instead of the normal status text, using the warning style to
	// draw attention
	var statusContent string
	if m.clusterSelectMode {
		statusContent = warningStyle.Render(m.status)
	} else if m.pendingConfirmation != nil {
		statusContent = warningStyle.Render(m.pendingConfirmation.prompt)
	} else {
		statusContent = statusArea(m.status, m.apiInProgress, m.spinner.View())
	}

	s.WriteString(
		lipgloss.JoinHorizontal(
			0.2,

			paddedStyle.Width(windowSize.Width-assignedStringWidth-paddedStyle.GetHorizontalPadding()-paddedStyle.GetHorizontalBorderSize()).Render(statusContent),

			paddedStyle.Render(assigneeArea(assignedTo)),
		),
	)

	s.WriteString("\n")
	return s.String()
}

func (m model) renderBottomStatus() string {
	var s strings.Builder
	var selectedID string

	// Show selected incident (always synced to highlighted row)
	if m.selectedIncident != nil {
		selectedID = m.selectedIncident.ID
	} else {
		selectedID = ""
	}

	versionWidth := len(GitSHA) + 2

	s.WriteString(
		lipgloss.JoinHorizontal(
			0.2,
			mutedStyle.Width(windowSize.Width-versionWidth-paddedStyle.GetHorizontalPadding()-paddedStyle.GetHorizontalBorderSize()).Padding(0, 2, 0, 1).Render(selectedID),
			mutedStyle.Padding(0, 2, 0, 1).Render(GitSHA),
		),
	)

	return s.String()
}

func assigneeArea(s string) string {
	var fstring = "Showing assigned to " + s
	fstring = strings.TrimSuffix(fstring, "\n")

	return fstring
}

func statusArea(s string, showSpinner bool, spinnerView string) string {
	if showSpinner {
		// Apply normal text color to the status text to prevent spinner color bleed
		statusStyle := lipgloss.NewStyle().Foreground(srepdPallet.normal.text)
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

func (m model) template() (string, error) {
	// Progressive rendering: show what we have, mark missing parts as loading
	var alerts []pagerduty.IncidentAlert
	var notes []pagerduty.IncidentNote

	if m.incidentAlertsLoaded {
		alerts = m.selectedIncidentAlerts
	} else {
		alerts = nil
	}

	if m.incidentNotesLoaded {
		notes = m.selectedIncidentNotes
	} else {
		notes = nil
	}

	summary := summarize(m.selectedIncident, alerts, notes)
	summary.AlertsLoading = !m.incidentAlertsLoaded
	summary.NotesLoading = !m.incidentNotesLoaded

	// Render tab header
	alertCount := len(alerts)
	noteCount := len(notes)
	tabHeader := renderTabHeader(m.activeTab, alertCount, noteCount, summary.AlertsLoading, summary.NotesLoading)

	var content string

	switch m.activeTab {
	case tabAlerts:
		if summary.AlertsLoading {
			content = tabHeader + "\n\n_Loading alerts..._\n"
		} else if len(summary.Alerts) == 0 {
			content = tabHeader + "\n\n_No alerts_\n"
		} else {
			idx := m.activeAlertIdx
			if idx >= len(summary.Alerts) {
				idx = len(summary.Alerts) - 1
			}
			data := singleAlertTabData{
				Alert: summary.Alerts[idx],
				Index: idx,
				Total: len(summary.Alerts),
			}
			tmpl, err := template.New("alert").Funcs(funcMap).Parse(alertTabTemplate)
			if err != nil {
				return "", err
			}
			o := new(bytes.Buffer)
			err = tmpl.Execute(o, data)
			if err != nil {
				return "", err
			}
			content = tabHeader + "\n" + o.String()
		}

	case tabNotes:
		if summary.NotesLoading {
			content = tabHeader + "\n\n_Loading notes..._\n"
		} else if len(summary.Notes) == 0 {
			content = tabHeader + "\n\n_No notes_\n"
		} else {
			idx := m.activeNoteIdx
			if idx >= len(summary.Notes) {
				idx = len(summary.Notes) - 1
			}
			data := singleNoteTabData{
				Note:  summary.Notes[idx],
				Index: idx,
				Total: len(summary.Notes),
			}
			tmpl, err := template.New("note").Funcs(funcMap).Parse(noteTabTemplate)
			if err != nil {
				return "", err
			}
			o := new(bytes.Buffer)
			err = tmpl.Execute(o, data)
			if err != nil {
				return "", err
			}
			content = tabHeader + "\n" + o.String()
		}

	default: // tabDetails
		tmpl, err := template.New("details").Funcs(funcMap).Parse(detailsTabTemplate)
		if err != nil {
			return "", err
		}
		o := new(bytes.Buffer)
		err = tmpl.Execute(o, summary)
		if err != nil {
			return "", err
		}
		content = tabHeader + "\n" + o.String()
	}

	return content, nil
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

func summarize(i *pagerduty.Incident, a []pagerduty.IncidentAlert, n []pagerduty.IncidentNote) incidentSummary {
	summary := summarizeIncident(i)
	summary.Alerts = summarizeAlerts(a)
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
	ID        string
	Name      string
	Link      string
	HTMLURL   string
	Service   string
	Created   string
	Status    string
	Incident  string
	Cluster   string
	Severity  string
	Tags      []string
	AlertType string
}

func summarizeAlerts(a []pagerduty.IncidentAlert) []alertSummary {
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

		s = append(s, alertSummary{
			ID:        alt.ID,
			Name:      name,
			Link:      link,
			Cluster:   cluster,
			HTMLURL:   alt.HTMLURL,
			Service:   alt.Service.Summary,
			Created:   alt.CreatedAt,
			Status:    alt.Status,
			Incident:  alt.Incident.ID,
			Severity:  normalized.Severity,
			Tags:      normalized.Tags,
			AlertType: normalized.AlertType,
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
}

const incidentTemplate = `
{{- if .Priority -}}
# {{ .Priority }} **{{ .ID }}** - {{ .Status }}
{{- else -}}
# **{{ .ID }}** - {{ .Status }}
{{- end }}

{{ ToLink .Title .HTMLURL }}

* Service: {{ .Service }}
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

## Notes

{{ if .NotesLoading }}
_Loading notes..._
{{ else if .Notes }}
{{ range $note := .Notes }}
  > {{ $note.Content }}
  -- {{ $note.User }} @ {{ $note.Created }}
{{ end }}
{{ else }}
_none_
{{ end }}

## Alerts{{ if not .AlertsLoading }} ({{ len .Alerts }}){{ end }}

{{ if .AlertsLoading }}
_Loading alerts..._
{{ else }}
{{ $alertLen := len .Alerts }}{{ range $i, $alert := .Alerts }}
### {{ if $alert.Name }}{{ $alert.Name }}{{ else }}{{ $alert.ID }}{{ end }} ({{ $alert.Status }}){{ if $alert.Severity }} [{{ $alert.Severity }}]{{ end }}{{ if $alert.AlertType }} ({{ $alert.AlertType }}){{ end }}

  * Cluster: {{ $alert.Cluster }}
  * SOP: {{ if $alert.Link }}{{ ToLink "SOP" $alert.Link }}{{ else }}_none_{{ end }}
  * Alert: {{ ToLink $alert.ID $alert.HTMLURL }}
  * Service: {{ $alert.Service }}
  * Created: {{ $alert.Created }}

{{ if not (Last $i $alertLen) }}---{{ end }}

{{ end }}
{{ end }}
`

func renderIncidentMarkdown(m *model, content string) (string, error) {
	// If no renderer available, return plain content
	if m.markdownRenderer == nil {
		return content, nil
	}

	// Reuse the cached renderer - it was created with a reasonable default width
	// and glamour's word wrapping will handle variations reasonably well
	str, err := m.markdownRenderer.Render(content)
	if err != nil {
		return str, err
	}

	return str, nil
}

// Tab constants
const (
	tabDetails = 0
	tabAlerts  = 1
	tabNotes   = 2
	tabCount   = 3
)

// singleAlertTabData is the data passed to the alertTabTemplate
type singleAlertTabData struct {
	Alert alertSummary
	Index int // 0-based
	Total int
}

// singleNoteTabData is the data passed to the noteTabTemplate
type singleNoteTabData struct {
	Note  noteSummary
	Index int // 0-based
	Total int
}

// renderTabHeader renders the tab bar with counts and highlights the active tab.
// Example: " [Details]  Alerts (5)  Notes (3) "
func renderTabHeader(activeTab int, alertCount int, noteCount int, alertsLoading bool, notesLoading bool) string {
	tabs := make([]string, tabCount)
	tabs[tabDetails] = "Details"

	if alertsLoading {
		tabs[tabAlerts] = "Alerts (...)"
	} else {
		tabs[tabAlerts] = fmt.Sprintf("Alerts (%d)", alertCount)
	}

	if notesLoading {
		tabs[tabNotes] = "Notes (...)"
	} else {
		tabs[tabNotes] = fmt.Sprintf("Notes (%d)", noteCount)
	}

	var parts []string
	for i, tab := range tabs {
		if i == activeTab {
			parts = append(parts, fmt.Sprintf("[%s]", tab))
		} else {
			parts = append(parts, fmt.Sprintf(" %s ", tab))
		}
	}

	return strings.Join(parts, " ")
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

// alertTabTemplate renders a single alert with navigation header
const alertTabTemplate = `
### Alert {{ add1 .Index }}/{{ .Total }}

### {{ if .Alert.Name }}{{ .Alert.Name }}{{ else }}{{ .Alert.ID }}{{ end }} ({{ .Alert.Status }}){{ if .Alert.Severity }} [{{ .Alert.Severity }}]{{ end }}{{ if .Alert.AlertType }} ({{ .Alert.AlertType }}){{ end }}

* Cluster: {{ .Alert.Cluster }}
* SOP: {{ if .Alert.Link }}{{ ToLink "SOP" .Alert.Link }}{{ else }}_none_{{ end }}
* Alert: {{ ToLink .Alert.ID .Alert.HTMLURL }}
* Service: {{ .Alert.Service }}
* Created: {{ .Alert.Created }}
`

// noteTabTemplate renders a single note with navigation header
const noteTabTemplate = `
### Note {{ add1 .Index }}/{{ .Total }}

> {{ .Note.Content }}

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
