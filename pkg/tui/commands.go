package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/clcollins/srepd/pkg/pd"
)

const (
	gettingUserStatus      = "getting user info..."
	loadingIncidentsStatus = "loading incidents..."
)

type waitForSelectedIncidentThenDoMsg struct {
	action string
	msg    tea.Msg
}

//lint:ignore U1000 - future proofing
type waitForSelectedIncidentThenRenderMsg string

func renderIncidentMarkdown(content string) (string, error) {
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

	return str, nil
}

//lint:ignore U1000 - future proofing
func updateSelectedIncident(p *pd.Config, id string) tea.Cmd {
	return tea.Sequence(
		getIncident(p, id),
		getIncidentAlerts(p, id),
		getIncidentNotes(p, id),
	)
}

type updateIncidentListMsg string
type updatedIncidentListMsg struct {
	incidents []pagerduty.Incident
	err       error
}

func updateIncidentList(p *pd.Config) tea.Cmd {
	return func() tea.Msg {
		opts := pd.NewListIncidentOptsFromDefaults()
		opts.TeamIDs = getTeamsAsStrings(p)
		i, err := pd.GetIncidents(p.Client, opts)
		return updatedIncidentListMsg{i, err}
	}
}

type renderIncidentMsg string

type renderedIncidentMsg struct {
	content string
	err     error
}

func renderIncident(m *model) tea.Cmd {
	return func() tea.Msg {
		content, err := renderIncidentMarkdown(m.template())
		if err != nil {
			m.setStatus(err.Error())
			log.Fatal(err)
		}

		return renderedIncidentMsg{content, err}
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
	debug("getIncidentAlerts")
	return func() tea.Msg {
		a, err := pd.GetAlerts(p.Client, id, pagerduty.ListIncidentAlertsOptions{})
		return gotIncidentAlertsMsg{a, err}
	}
}

type gotIncidentNotesMsg struct {
	notes []pagerduty.IncidentNote
	err   error
}

func getIncidentNotes(p *pd.Config, id string) tea.Cmd {
	debug("getIncidentNotes")
	return func() tea.Msg {
		n, err := pd.GetNotes(p.Client, id)
		return gotIncidentNotesMsg{n, err}
	}
}

type getCurrentUserMsg string
type gotCurrentUserMsg struct {
	user *pagerduty.User
	err  error
}

func getCurrentUser(p *pd.Config) tea.Cmd {
	return func() tea.Msg {
		u, err := p.Client.GetCurrentUserWithContext(
			context.Background(),
			pagerduty.GetCurrentUserOptions{},
		)
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

func openEditorCmd(editor []string) tea.Cmd {
	var args []string

	file, err := os.CreateTemp(os.TempDir(), "")
	if err != nil {
		return func() tea.Msg {
			return errMsg{error: err}
		}
	}

	args = append(args, editor[1:]...)
	args = append(args, file.Name())

	c := exec.Command(editor[0], args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err, file}
	})
}

type loginMsg string
type loginFinishedMsg error

type ClusterLauncher struct {
	Terminal            []string
	Shell               []string
	ClusterLoginCommand []string
}

func login(cluster string, launcher ClusterLauncher) tea.Cmd {
	debug("login")

	// Check if we have the necessary info to try to login
	errs := []error{}
	if launcher.Terminal == nil {
		debug("Terminal is not set")
		errs = append(errs, errors.New("terminal is not set"))
	}
	if launcher.Shell == nil {
		debug("Shell is not set")
		errs = append(errs, errors.New("shell is not set"))
	}
	if launcher.ClusterLoginCommand == nil {
		debug("ClusterLoginCommand is not set")
		errs = append(errs, errors.New("ClusterLoginCommand is not set"))
	}

	if len(errs) > 0 {
		err := fmt.Errorf("login error: %v", errs)
		debug(err.Error())
		return func() tea.Msg {
			return loginFinishedMsg(err)
		}
	}

	var args []string
	args = append(args, launcher.Terminal[1:]...)
	args = append(args, "--") // Terminal separator
	args = append(args, launcher.Shell...)
	args = append(args, launcher.ClusterLoginCommand...)
	args = append(args, cluster)

	// The first element of Terminal is the command to be executed, followed by args, in order
	// This handles if folks use, eg: flatpak run <some package> as a terminal.
	c := exec.Command(launcher.Terminal[0], args...)

	debug(c.String())
	// TODO: What do we do with the stdout?  Anything?
	_, pipeErr := c.StdoutPipe()
	if pipeErr != nil {
		debug(pipeErr.Error())
		return func() tea.Msg {
			return loginFinishedMsg(pipeErr)
		}
	}
	stderr, pipeErr := c.StderrPipe()
	if pipeErr != nil {
		debug(pipeErr.Error())
		return func() tea.Msg {
			return loginFinishedMsg(pipeErr)
		}
	}

	err := c.Start()
	if err != nil {
		debug(err.Error())
		return func() tea.Msg {
			return loginFinishedMsg(err)
		}
	}

	out, err := io.ReadAll(stderr)
	if err != nil {
		debug(err.Error())
		return func() tea.Msg {
			return loginFinishedMsg(err)
		}
	}

	if len(out) > 0 {
		debug(fmt.Sprintf("login error: %s", out))
		return func() tea.Msg {
			return loginFinishedMsg(fmt.Errorf("%s", out))
		}
	}

	return func() tea.Msg {
		return loginFinishedMsg(nil)
	}
}

type clearSelectedIncidentsMsg string

type acknowledgeIncidentsMsg struct {
	incidents []*pagerduty.Incident
}
type acknowledgedIncidentsMsg struct {
	incidents []pagerduty.Incident
	err       error
}

type waitForSelectedIncidentsThenAcknowledgeMsg string

func acknowledgeIncidents(p *pd.Config, incidents []*pagerduty.Incident) tea.Cmd {
	return func() tea.Msg {
		u, err := p.Client.GetCurrentUserWithContext(context.Background(), pagerduty.GetCurrentUserOptions{})
		if err != nil {
			return errMsg{err}
		}
		a, err := pd.AcknowledgeIncident(p.Client, incidents, u)
		if err != nil {
			return errMsg{err}
		}
		return acknowledgedIncidentsMsg{a, err}
	}
}

type reassignIncidentsMsg struct {
	incidents []*pagerduty.Incident
	users     []*pagerduty.User
}
type reassignedIncidentsMsg []pagerduty.Incident

func reassignIncidents(p *pd.Config, i []*pagerduty.Incident, users []*pagerduty.User) tea.Cmd {
	return func() tea.Msg {
		u, err := p.Client.GetCurrentUserWithContext(context.Background(), pagerduty.GetCurrentUserOptions{})
		if err != nil {
			return errMsg{err}
		}
		r, err := pd.ReassignIncidents(p.Client, i, u, users)
		if err != nil {
			return errMsg{err}
		}
		return reassignedIncidentsMsg(r)
	}
}

type silenceIncidentsMsg struct {
	incidents []*pagerduty.Incident
}
type waitForSelectedIncidentsThenSilenceMsg string

var errSilenceIncidentInvalidArgs = errors.New("silenceIncidents: invalid arguments")

func silenceIncidents(i []*pagerduty.Incident, u []*pagerduty.User) tea.Cmd {
	// SilenceIncidents doesn't have it's own "silencedIncidentsMessage"
	// because it's really just a reassignment
	log.Printf("silence requested for incident(s) %v; reassigning to %v", i, u)
	return func() tea.Msg {
		if len(i) == 0 || len(u) == 0 {
			return errMsg{errSilenceIncidentInvalidArgs}
		}
		return reassignIncidentsMsg{i, u}
	}
}

//lint:ignore U1000 - future proofing
type addIncidentNoteMsg string
type waitForSelectedIncidentsThenAnnotateMsg string
type addedIncidentNoteMsg struct {
	note *pagerduty.IncidentNote
	err  error
}

func addNoteToIncident(p *pd.Config, incident *pagerduty.Incident, content *os.File) tea.Cmd {
	return func() tea.Msg {
		defer content.Close()

		bytes, err := os.ReadFile(content.Name())
		if err != nil {
			return errMsg{err}
		}
		content := string(bytes[:])

		u, err := p.Client.GetCurrentUserWithContext(context.Background(), pagerduty.GetCurrentUserOptions{})
		if err != nil {
			return errMsg{err}
		}

		n, err := pd.PostNote(p.Client, incident.ID, u, content)
		return addedIncidentNoteMsg{n, err}
	}
}

func getTeamsAsStrings(p *pd.Config) []string {
	var teams []string
	for _, t := range p.Teams {
		teams = append(teams, t.ID)
	}
	return teams
}

func getDetailFieldFromAlert(f string, a pagerduty.IncidentAlert) string {
	if a.Body["details"] != nil {

		if a.Body["details"].(map[string]interface{})[f] != nil {
			return a.Body["details"].(map[string]interface{})[f].(string)
		}
		debug(fmt.Sprintf("alert body \"details\" does not contain field %s", f))
		return ""
	}
	debug("alert body \"details\" is nil")
	return ""
}
