package tui

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/clcollins/srepd/pkg/tui/style"
)

const (
	dot       = "•"
	upArrow   = "↑"
	downArrow = "↓"
)

var (
	windowSize tea.WindowSizeMsg
)

func (m model) View() string {
	debug("View")
	helpView := style.Help.Render(m.help.View(defaultKeyMap))

	switch {
	case m.err != nil:
		debug("error")
		errHelpView := style.Help.Render(help.New().View(errorViewKeyMap))
		return (style.Error.Render(dot+"ERROR"+dot+"\n\n"+m.err.Error()) + "\n" + errHelpView)

	case m.viewingIncident:
		debug("viewingIncident")
		return style.Main.Render(m.renderHeader() + "\n" + m.incidentViewer.View() + "\n" + helpView)

	default:
		tableView := style.TableContainer.Render(m.table.View())
		if m.input.Focused() {
			debug("viewingTable and input")
			return style.Main.Render(m.renderHeader() + "\n" + tableView + "\n" + m.input.View() + "\n" + helpView)
		}
		debug("viewingTable")
		return style.Main.Render(m.renderHeader() + "\n" + tableView + "\n" + helpView)
	}
}

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
			style.Status.Width(windowSize.Width-style.AssignedStringWidth).Render(statusArea(m.status)),
			style.Assignee.Render(assigneeArea(assignedTo)),
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

func (m model) template() (string, error) {
	debug("template")
	template, err := template.New("incident-full.tmpl").Funcs(funcMap).ParseFiles("pkg/tui/views/incident-full.tmpl")
	if err != nil {
		// TODO: Figure out how to handle this with a proper errMsg
		fmt.Printf("Could not render template: %+v", err)
		return "", err
	}

	o := new(bytes.Buffer)
	summary := summarize(m.selectedIncident, m.selectedIncidentAlerts, m.selectedIncidentNotes)
	err = template.Execute(o, summary)
	if err != nil {
		fmt.Printf("Could not render template: %+v", err)
		// TODO: Figure out how to handle this with a proper errMsg
		return "", err
	}

	return o.String(), nil
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
	debug(fmt.Sprintf("summarizeAlerts: %v", len(a)))
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

func renderIncidentMarkdown(content string) (string, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(windowSize.Width),
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
