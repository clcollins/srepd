package tui

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"strings"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	dot        = "•"
	upArrow    = "↑"
	downArrow  = "↓"
	gray       = lipgloss.Color("240")
	paleYellow = lipgloss.Color("229")
	neonPurple = lipgloss.Color("57")
	lilac      = lipgloss.Color("105")
)

var (
	windowSize          tea.WindowSizeMsg
	horizontalPadding   = 1
	borderWidth         = 1
	mainStyle           = lipgloss.NewStyle().Margin(0, 0).Padding(0, horizontalPadding)
	assigneeStyle       = mainStyle.Copy()
	statusStyle         = mainStyle.Copy()
	assignedStringWidth = len("Assigned to User") + (horizontalPadding * 2 * 2) + (borderWidth * 2 * 2) + 10
	tableContainerStyle = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(gray)
	tableStyle          = table.Styles{
		Selected: lipgloss.NewStyle().Bold(true).Foreground(paleYellow).Background(neonPurple),
		Header:   lipgloss.NewStyle().Bold(false).Padding(0, 1).BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color(gray)).BorderBottom(true),
		Cell:     lipgloss.NewStyle().Padding(0, 1),
	}
	helpStyle           = lipgloss.NewStyle().Foreground(lilac)
	incidentViewerStyle = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color(gray)).Padding(2)
)

func (m model) renderHeader() string {
	var s strings.Builder
	var assignedTo string

	assignedTo = "User"

	if m.teamMode {
		assignedTo = "Team"
	}

	s.WriteString(
		lipgloss.JoinHorizontal(
			0.2,
			statusStyle.Width(windowSize.Width-assignedStringWidth).Render(statusArea(m.status)),
			assigneeStyle.Render(assigneeArea(assignedTo)),
		),
	)

	return s.String()
}

func assigneeArea(s string) string {
	var fstring = "Assigned to " + s
	fstring = strings.TrimSuffix(fstring, "\n")

	return fstring
}

func statusArea(s string) string {
	var fstring = "> %s"
	fstring = strings.TrimSuffix(fstring, "\n")

	return fmt.Sprintf(fstring, s)
}

// TODO: Is this needed
// func truncateStringToWidth(s string, width int) string {
// 	if len(s) > width {
// 		s = s[:width-1] + "…"
// 	}
//
// 	return s
// }

func (m model) template() string {
	debug("template")
	template, err := template.New("incident").Funcs(funcMap).Parse(incidentTemplate)
	if err != nil {
		log.Fatal(err)
	}

	o := new(bytes.Buffer)
	summary := summarize(m.selectedIncident, m.selectedIncidentAlerts, m.selectedIncidentNotes)
	err = template.Execute(o, summary)
	if err != nil {
		log.Fatal(err)
	}

	return o.String()
}

func summarize(i *pagerduty.Incident, a []pagerduty.IncidentAlert, n []pagerduty.IncidentNote) incidentSummary {
	debug("summarize")
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
	debug(fmt.Sprintf("summarizeNotes: %v", len(n)))
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
	HTMLURL  string
	Service  string
	Created  string
	Status   string
	Incident string
	Details  map[string]interface{}
}

func summarizeAlerts(a []pagerduty.IncidentAlert) []alertSummary {
	debug(fmt.Sprintf("summarizeAlerts: %v", len(a)))
	var s []alertSummary

	for _, alt := range a {
		s = append(s, alertSummary{
			ID:       alt.ID,
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
}

func summarizeIncident(i *pagerduty.Incident) incidentSummary {
	debug(fmt.Sprintf("summarizeIncident: %+v", i))
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
		s.Acknowledged = append(s.Acknowledged, ack.Acknowledger.Summary)
	}

	return s
}

var funcMap = template.FuncMap{
	"Truncate": func(s string) string {
		return fmt.Sprintf("%s ...", s[:5])
	},
	"ToUpper": strings.ToUpper,
}

const incidentTemplate = `
# {{ .ID }}

{{ if .Priority }}PRIORITY {{ .Priority }} - {{ end }}{{ .Title }}

{{ .HTMLURL }}

## Summary

* Service: {{ .Service }}
* Status: {{ .Status }}
* Priority: {{ .Priority }}
* Urgency: {{ .Urgency }}
* Created: {{ .Created }}

## Responders and Escalation

{{ if not .Acknowledged -}}
Assigned to:{{ range $assignee := .Assigned }}
+ {{ $assignee }}
{{ end -}}
{{ else -}}
Acknowledged by:{{ range $ack := .Acknowledged }}
* {{ $ack }}
{{ end -}}
{{ end -}}

## Notes

{{ range $note := .Notes }}
> {{ $note.Content }}

- {{ $note.User }} @ {{ $note.Created }}
{{ end }}

## Alerts ({{ len .Alerts }})

{{ range $alert := .Alerts }}
### {{ $alert.ID }}
* Service: {{ $alert.Service }}
* Status: {{ $alert.Status }}
* Created: {{ $alert.Created }}
* Link: {{ $alert.HTMLURL }}
{{ range $key, $value := $alert.Details }}
* {{ $key }}: {{ $value }} 
{{ end }}

{{ end }}
`
