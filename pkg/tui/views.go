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
	assignedStringWidth = len("Showing assigned to User") + (horizontalPadding * 2 * 2) + (borderWidth * 2 * 2) + 10
	tableContainerStyle = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(gray)
	tableStyle          = table.Styles{
		Selected: lipgloss.NewStyle().Bold(true).Foreground(paleYellow).Background(neonPurple),
		Header:   lipgloss.NewStyle().Bold(false).Padding(0, 1).BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color(gray)).BorderBottom(true),
		Cell:     lipgloss.NewStyle().Padding(0, 1),
	}
	helpStyle           = lipgloss.NewStyle().Foreground(lilac)
	incidentViewerStyle = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color(gray)).Padding(2)
	errorStyle          = lipgloss.NewStyle().
				Bold(true).
				Width(64).
				Foreground(lipgloss.AdaptiveColor{Light: "#E11C9C", Dark: "#FF62DA"}).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#E11C9C", Dark: "#FF62DA"}).
				Padding(1, 3, 1, 3)
)

func (m model) View() string {
	helpView := helpStyle.Render(m.help.View(defaultKeyMap))

	switch {
	case m.err != nil:
		log.Debug("View", "error", m.err)
		errHelpView := helpStyle.Render(help.New().View(errorViewKeyMap))
		return (errorStyle.Render(dot+"ERROR"+dot+"\n\n"+m.err.Error()) + "\n" + errHelpView)

	case m.viewingIncident:
		log.Debug("viewingIncident")
		return mainStyle.Render(m.renderHeader() + "\n" + m.incidentViewer.View() + "\n" + helpView)

	default:
		tableView := tableContainerStyle.Render(m.table.View())
		if m.input.Focused() {
			log.Debug("viewingTable and input")
			return mainStyle.Render(m.renderHeader() + "\n" + tableView + "\n" + m.input.View() + "\n" + helpView)
		}
		log.Debug("viewingTable")
		return mainStyle.Render(m.renderHeader() + "\n" + tableView + "\n" + helpView)
	}
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
			statusStyle.Width(windowSize.Width-assignedStringWidth).Render(statusArea(m.status)),
			assigneeStyle.Render(assigneeArea(assignedTo)),
		),
	)

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
