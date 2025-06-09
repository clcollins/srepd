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
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
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

	incidentViewerStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), true)

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

	case m.viewingIncident:
		s.WriteString(m.incidentViewer.View())

	default:
		s.WriteString(tableContainerStyle.Render(m.table.View()))
	}

	if m.input.Focused() {

		s.WriteString("\n")
		s.WriteString(m.input.View())
	}

	s.WriteString("\n")
	s.WriteString(m.renderFooter())
	s.WriteString("\n")
	s.WriteString(paddedStyle.Width(windowSize.Width).Render(m.help.View(defaultKeyMap)))

	return mainStyle.Render(s.String())
}

func (m model) renderFooter() string {
	var s strings.Builder
	s.WriteString(
		lipgloss.JoinHorizontal(
			0.2,
			paddedStyle.Render(refreshArea(m.autoRefresh, m.autoAcknowledge)),
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

	s.WriteString(
		lipgloss.JoinHorizontal(
			0.2,

			paddedStyle.Width(windowSize.Width-assignedStringWidth-paddedStyle.GetHorizontalPadding()-paddedStyle.GetHorizontalBorderSize()).Render(statusArea(m.status)),

			paddedStyle.Render(assigneeArea(assignedTo)),
		),
	)

	s.WriteString("\n")
	return s.String()
}

func assigneeArea(s string) string {
	var fstring = "Showing assigned to " + s
	fstring = strings.TrimSuffix(fstring, "\n")

	return fstring
}

func statusArea(s string) string {
	var fstring = "> %s"
	fstring = strings.TrimSuffix(fstring, "\n")

	return fmt.Sprintf(fstring, s)
}

func refreshArea(autoRefresh bool, autoAck bool) string {
	var fstring = "Watching for updates... "

	if autoRefresh && autoAck {
		fstring = fstring + " [auto-acknowledge]"
	} else if !autoRefresh {
		fstring = fstring + " [PAUSED]"
	}

	fstring = strings.TrimSuffix(fstring, "\n")
	return fstring
}

func (m model) template() (string, error) {
	template, err := template.New("incident").Funcs(funcMap).Parse(incidentTemplate)
	if err != nil {
		// TODO: Figure out how to handle this with a proper errMsg
		return "", err
	}

	o := new(bytes.Buffer)
	summary := summarize(m.selectedIncident, m.selectedIncidentAlerts, m.selectedIncidentNotes)
	err = template.Execute(o, summary)
	if err != nil {
		// TODO: Figure out how to handle this with a proper errMsg
		return "", err
	}

	return o.String(), nil
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
	ID       string
	Name     string
	Link     string
	HTMLURL  string
	Service  string
	Created  string
	Status   string
	Incident string
	Details  map[string]interface{}
	Cluster  string
}

func summarizeAlerts(a []pagerduty.IncidentAlert) []alertSummary {
	var s []alertSummary

	for _, alt := range a {

		// Our alerts are not standardized enough
		// CPD, for example, does not have "alert_name"
		name := getDetailFieldFromAlert("alert_name", alt)
		cluster := getDetailFieldFromAlert("cluster_id", alt)
		link := getDetailFieldFromAlert("link", alt)

		s = append(s, alertSummary{
			ID:       alt.ID,
			Name:     name,
			Link:     link,
			Cluster:  cluster,
			HTMLURL:  alt.HTMLURL,
			Service:  alt.Service.Summary,
			Created:  alt.CreatedAt,
			Status:   alt.Status,
			Incident: alt.Incident.ID,
			Details:  alt.Body["details"].(map[string]interface{}),
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
}

const incidentTemplate = `
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

## Notes

{{ if .Notes }}
{{ range $note := .Notes }}
> {{ $note.Content }}
    -- {{ $note.User }} @ {{ $note.Created }}
{{ end }}
{{ else }}
_none_
{{ end }}

## Alerts ({{ len .Alerts }})

{{ range $alert := .Alerts }}
{{ $alert.ID }} - {{ $alert.Status }}
{{ if $alert.Name }}
_{{- $alert.Name }}_
{{- end }}
{{ $alert.HTMLURL }}

* Cluster: {{ $alert.Cluster }}
* Service: {{ $alert.Service }}
* Created: {{ $alert.Created }}
* SOP: {{ $alert.Link }}

Details :

{{ range $key, $value := $alert.Details }}
* {{ $key }}: {{ $value }} 
{{ end }}

{{ end }}
`

func renderIncidentMarkdown(content string) (string, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(0),
	)
	if err != nil {
		return "", err
	}
	str, err := renderer.Render(content)
	if err != nil {
		return str, err
	}

	return str, nil
}

const noteTemplate = `

# Please enter the note message content above. Lines starting
# with '#' will be ignored and an empty message aborts the note.
#
# Incident: {{ .ID }}
# Summary: {{ .Title }}
# Service: {{ .Service }}
#
`
