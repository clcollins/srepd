package tui

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/clcollins/srepd/pkg/pd"
)

type updateIncidentListMsg string
type updatedIncidentListMsg struct {
	incidents []pagerduty.Incident
	err       error
}

func updateIncidentList(p *pd.Config) tea.Cmd {
	return func() tea.Msg {
		opts := pd.NewListIncidentOptsFromDefaults(p)
		i, err := pd.GetIncidents(p, opts)
		return updatedIncidentListMsg{i, err}
	}
}

type getIncidentMsg string
type gotIncidentMsg struct {
	incident *pagerduty.Incident
	err      error
}

func getIncident(p *pd.Config, id string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		i, err := p.Client.GetIncidentWithContext(ctx, id)
		return gotIncidentMsg{i, err}
	}
}

type gotIncidentAlertsMsg struct {
	alerts []pagerduty.IncidentAlert
	err    error
}

func getIncidentAlerts(p *pd.Config, id string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		a, err := pd.GetAlerts(ctx, p, id)
		return gotIncidentAlertsMsg{a, err}
	}
}

type gotIncidentNotesMsg struct {
	notes []pagerduty.IncidentNote
	err   error
}

func getIncidentNotes(p *pd.Config, id string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		n, err := pd.GetNotes(ctx, p, id)
		return gotIncidentNotesMsg{n, err}
	}
}

type getCurrentUserMsg string
type gotCurrentUserMsg struct {
	user *pagerduty.User
	err  error
}

func getCurrentUser(pdConfig *pd.Config) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		u, err := pdConfig.Client.GetCurrentUserWithContext(ctx, pagerduty.GetCurrentUserOptions{})
		return gotCurrentUserMsg{u, err}
	}
}

func AssignedToAnyUsers(i pagerduty.Incident, ids []string) bool {
	for _, a := range i.Assignments {
		for _, id := range ids {
			if a.Assignee.ID == id {
				return true
			}
		}
	}
	return false
}

func AssignedToUser(i pagerduty.Incident, id string) bool {
	for _, a := range i.Assignments {
		if a.Assignee.ID == id {
			return true
		}
	}
	return false
}

type editorFinishedMsg struct {
	err  error
	file *os.File
}

var defaultEditor = "/usr/bin/vim"

func openEditorCmd(editor string) tea.Cmd {
	file, err := os.CreateTemp(os.TempDir(), "")
	if err != nil {
		return func() tea.Msg {
			return errMsg{error: err}
		}
	}
	c := exec.Command(editor, file.Name())
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err, file}
	})
}

func newIncidentViewer(content string) viewport.Model {

	vp := viewport.New(windowSize.Width, windowSize.Height-5)
	vp.MouseWheelEnabled = true
	vp.Style = lipgloss.NewStyle().
		Width(windowSize.Width - 10).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		PaddingRight(2)
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(windowSize.Width),
	)
	if err != nil {
		log.Fatal(err)
	}

	str, err := renderer.Render(content)
	if err != nil {
		log.Fatal(err)
	}

	vp.SetContent(str)
	return vp
}

type acknowledgeIncidentsMsg struct {
	incidents []pagerduty.Incident
}
type acknowledgedIncidentsMsg struct {
	incidents []pagerduty.Incident
	err       error
}
type waitForSelectedIncidentsThenAcknowledgeMsg string

func acknowledgeIncidents(pdConfig *pd.Config, email string, i []pagerduty.Incident) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		a, err := pd.AcknowledgeIncident(ctx, pdConfig, email, i)
		return acknowledgedIncidentsMsg{a, err}
	}
}

type reassignIncidentsMsg struct {
	incidents []pagerduty.Incident
	users     []*pagerduty.User
}
type reassignedIncidentsMsg []pagerduty.Incident

// reassignIncident accepts a context, pdConfig, currentUser email, incident ID, and a []pagerduty.User to assign
// and returns a "reassignedIncidentMsg" tea.Msg with the incident ID as a string
func reassignIncidents(pdConfig *pd.Config, email string, i []pagerduty.Incident, u []*pagerduty.User) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		r, err := pd.ReassignIncident(ctx, pdConfig, email, i, u)
		if err != nil {
			return errMsg{err}
		}
		return reassignedIncidentsMsg(r)
	}
}

type silenceIncidentsMsg []pagerduty.Incident
type waitForSelectedIncidentsThenSilenceMsg string

var errSilenceIncidentInvalidArgs = errors.New("silenceIncidents: invalid arguments")

// Silence incidents accepts a []pagerduty.Incident, and []pagerduty.User to assign
func silenceIncidents(i []pagerduty.Incident, u []*pagerduty.User) tea.Cmd {
	return func() tea.Msg {
		if len(i) == 0 || len(u) == 0 {
			return errMsg{errSilenceIncidentInvalidArgs}
		}
		return reassignIncidentsMsg{i, u}
	}
}

type clearSelectedIncidentsMsg string

type addIncidentNoteMsg string
type addedIncidentNoteMsg struct {
	note *pagerduty.IncidentNote
	err  error
}

func addNoteToIncident(pdConfig *pd.Config, id string, user *pagerduty.User, content *os.File) tea.Cmd {
	return func() tea.Msg {
		defer content.Close()

		ctx := context.Background()

		bytes, err := os.ReadFile(content.Name())
		if err != nil {
			return errMsg{err}
		}
		content := string(bytes[:])

		n, err := pd.AddNoteToIncident(ctx, pdConfig, id, user, content)
		return addedIncidentNoteMsg{n, err}
	}
}
