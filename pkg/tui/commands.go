package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/launcher"
	"github.com/clcollins/srepd/pkg/pd"
)

// This file contains the commands that are used in the Bubble Tea update function.
// These commands are functions that return a tea.Cmd, which performs I/O with
// another system, such as the PagerDuty API, the filesystem, or the user's terminal,
// and shouldn't perform any actual processing themselves.
//
// The returned tea.Cmd is executed by the Bubble Tea runtime, and the result of the
// command is sent back to the update function as a tea.Msg.
//
// See getIncident() as a good example of this pattern.

const (
	gettingUserStatus      = "getting user info..."
	loadingIncidentsStatus = "loading incidents..."
)

type execErr struct {
	Err        error
	ExitErr    *exec.ExitError
	ExecStdErr string
}

func (ee *execErr) Error() string {
	// Podman will return errors with the same formatting as this program
	// so strip out `Error: ` prefix and `\n` suffixes, since ocm-container
	// will just put them back
	s := ee.ExecStdErr
	s = strings.TrimPrefix(s, "Error: ")
	s = strings.TrimPrefix(s, "error: ")
	s = strings.TrimSuffix(s, "\n")
	return s
}

func (ee *execErr) Code() int {
	return ee.ExitErr.ExitCode()
}

// TODO: Deprecate incoming message
// getIncidentMsg is a message that triggers the fetching of an incident
type getIncidentMsg string

// gotIncidentMsg is a message that contains the fetched incident
type gotIncidentMsg struct {
	incident *pagerduty.Incident
	err      error
}

// getIncident returns a command that fetches the incident with the given ID
// or returns a setStatusMsg with nilIncidentMsg if the provided ID is empty
func getIncident(p *pd.Config, id string) tea.Cmd {
	return func() tea.Msg {
		if id == "" {
			return setStatusMsg{nilIncidentMsg}
		}
		ctx := context.Background()
		i, err := p.Client.GetIncidentWithContext(ctx, id)
		return gotIncidentMsg{i, err}
	}
}

// gotIncidentAlertsMsg is a message that contains the fetched incident alerts
type gotIncidentAlertsMsg struct {
	alerts []pagerduty.IncidentAlert
	err    error
}

// getIncidentAlerts returns a command that fetches the alerts for the given incident
func getIncidentAlerts(p *pd.Config, id string) tea.Cmd {
	return func() tea.Msg {
		if id == "" {
			return setStatusMsg{nilIncidentMsg}
		}
		a, err := pd.GetAlerts(p.Client, id, pagerduty.ListIncidentAlertsOptions{})
		return gotIncidentAlertsMsg{a, err}
	}
}

// gotIncidentNotesMsg is a message that contains the fetched incident notes
type gotIncidentNotesMsg struct {
	notes []pagerduty.IncidentNote
	err   error
}

// getIncidentNotes returns a command that fetches the notes for the given incident
func getIncidentNotes(p *pd.Config, id string) tea.Cmd {
	return func() tea.Msg {
		if id == "" {
			return setStatusMsg{nilIncidentMsg}
		}
		n, err := pd.GetNotes(p.Client, id)
		return gotIncidentNotesMsg{n, err}
	}
}

// updateIncidentListMsg is a message that triggers the fetching of the incident list
type updateIncidentListMsg string

// updatedIncidentListMsg is a message that contains the fetched incident list
type updatedIncidentListMsg struct {
	incidents []pagerduty.Incident
	err       error
}

// updateIncidentList returns a command that fetches the incident list from the PagerDuty API
func updateIncidentList(p *pd.Config) tea.Cmd {
	return func() tea.Msg {
		opts := newListIncidentOptsFromConfig(p)
		i, err := pd.GetIncidents(p.Client, opts)
		return updatedIncidentListMsg{i, err}
	}
}

// newListIncidentOptsFromConfig returns a ListIncidentsOptions struct
// with the UserIDs and TeamIDs fields populated from the given Config
func newListIncidentOptsFromConfig(p *pd.Config) pagerduty.ListIncidentsOptions {
	var opts = pagerduty.ListIncidentsOptions{}

	// If the Config is nil, return the default options
	if p == nil {
		return opts
	}

	// Convert the list of *pagerduty.User to a slice of user IDs
	if p.IgnoredUsers == nil {
		p.IgnoredUsers = []*pagerduty.User{}
	}

	ignoredUserIDs := func(u []*pagerduty.User) []string {
		var l []string
		for _, i := range u {
			l = append(l, i.ID)
		}
		return l
	}(p.IgnoredUsers)

	// If the UserID from p.TeamMemberIDs is not in the ignoredUserIDs slice, add it to the opts.UserIDs slice
	if p.TeamsMemberIDs == nil {
		p.TeamsMemberIDs = []string{}
	}

	opts.UserIDs = func(a []string, i []string) []string {
		var l []string
		for _, u := range a {
			if !slices.Contains(i, u) {
				l = append(l, u)
			}
		}
		return l
	}(p.TeamsMemberIDs, ignoredUserIDs)

	// Convert the list of *pagerduty.Team to a slice of team IDs
	if p.Teams == nil {
		p.Teams = []*pagerduty.Team{}
	}

	opts.TeamIDs = func(t []*pagerduty.Team) []string {
		var l []string
		for _, x := range t {
			l = append(l, x.ID)
		}
		return l
	}(p.Teams)

	return opts
}

// HOUSEKEEPING: The above are commands that have complete unit tests and incoming
// and outgoing tea.Msg types, ordered alphabetically. Below are commands that need to
// be refactored to have unit tests and incoming and outgoing tea.Msg types, ordered

type errMsg struct{ error }
type setStatusMsg struct{ string }
type waitForSelectedIncidentThenDoMsg struct {
	action tea.Cmd
	msg    tea.Msg
}

type TickMsg struct {
}

type PollIncidentsMsg struct{}

type renderIncidentMsg string

type renderedIncidentMsg struct {
	content string
	err     error
}

func renderIncident(m *model) tea.Cmd {
	return func() tea.Msg {
		t, err := m.template()
		if err != nil {
			return errMsg{err}
		}

		content, err := renderIncidentMarkdown(t)
		if err != nil {
			return errMsg{err}
		}

		return renderedIncidentMsg{content, err}
	}
}

// AssignedToAnyUsers returns true if the incident is assigned to any of the given users
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

// ShouldBeAcknowledged returns true if the incident is assigned to the given user,
// the user has not acknowledged the incident yet, and autoAcknowledge is enabled
func ShouldBeAcknowledged(p *pd.Config, i pagerduty.Incident, id string, autoAcknowledge bool) bool {
	assigned := AssignedToUser(i, id)
	acknowledged := AcknowledgedByUser(i, id)
	userIsOnCall := UserIsOnCall(p, id)
	doIt := assigned && !acknowledged && autoAcknowledge && userIsOnCall
	log.Debug(
		"commands.ShouldBeAcknowledged",
		"assigned", assigned,
		"acknowledged", acknowledged,
		"autoAcknowledge", autoAcknowledge,
		"userIsOnCall", userIsOnCall,
		"doIt", doIt,
	)
	return AssignedToUser(i, id) && !AcknowledgedByUser(i, id) && autoAcknowledge
}

// AssignedToUser returns true if the incident is assigned to the given user
func AssignedToUser(i pagerduty.Incident, id string) bool {
	for _, a := range i.Assignments {
		if a.Assignee.ID == id {
			return true
		}
	}
	return false
}

// AcknowledgedByUser returns true if the incident has been acknowledged by the given user
func AcknowledgedByUser(i pagerduty.Incident, id string) bool {
	for _, a := range i.Acknowledgements {
		if a.Acknowledger.ID == id {
			return true
		}
	}
	return false
}

// UserIsOnCall returns true if the current time is between any of the current user's pagerduty.OnCalls in the next six hours
func UserIsOnCall(p *pd.Config, id string) bool {
	var timeLayout = "2006-01-02T15:04:05Z"
	opts := pagerduty.ListOnCallOptions{
		UserIDs: []string{id},
		Since:   time.Now().String(),
		Until:   time.Now().Add(time.Hour * 6).String(),
	}

	onCalls, err := pd.GetUserOnCalls(p.Client, id, opts)
	if err != nil {
		log.Debug("commands.UserIsOnCall", "error", err)
		return false
	}

	for _, o := range onCalls {
		log.Debug("commands.UserIsOnCall", "on-call", o)

		start, err := time.Parse(timeLayout, o.Start)
		if err != nil {
			log.Debug("commands.UserIsOnCall", "error parsing on-call start time", err)
			return false
		}
		end, err := time.Parse(timeLayout, o.End)
		if err != nil {
			log.Debug("commands.UserIsOnCall", "error parsing on-call end time", err)
			return false
		}

		if start.Before(time.Now()) && end.After(time.Now()) {
			return true
		}
	}

	return false
}

// TODO: Can we use a single function and struct to handle
// the openEditorCmd, login and openBrowserCmd commands?

type browserFinishedMsg struct {
	err error
}
type openBrowserMsg string

func openBrowserCmd(browser []string, url string) tea.Cmd {
	log.Debug("tui.openBrowserCmd(): opening browser")
	log.Debug(fmt.Sprintf("tui.openBrowserCmd(): %v %v", browser, url))

	var args []string
	args = append(args, browser[1:]...)
	args = append(args, url)

	c := exec.Command(browser[0], args...)
	log.Debug(fmt.Sprintf("tui.openBrowserCmd(): %v", c.String()))
	stderr, pipeErr := c.StderrPipe()
	if pipeErr != nil {
		log.Debug(fmt.Sprintf("tui.openBrowserCmd(): %v", pipeErr.Error()))
		return func() tea.Msg {
			return browserFinishedMsg{err: pipeErr}
		}
	}

	err := c.Start()
	if err != nil {
		log.Debug(fmt.Sprintf("tui.openBrowserCmd(): %v", err.Error()))
		return func() tea.Msg {
			return browserFinishedMsg{err}
		}
	}

	out, err := io.ReadAll(stderr)
	if err != nil {
		log.Debug(fmt.Sprintf("tui.openBrowserCmd(): %v", err.Error()))
		return func() tea.Msg {
			return browserFinishedMsg{err}
		}
	}

	if len(out) > 0 {
		log.Debug(fmt.Sprintf("tui.openBrowserCmd(): error: %s", out))
		return func() tea.Msg {
			return browserFinishedMsg{fmt.Errorf("%s", out)}
		}
	}

	return func() tea.Msg {
		return browserFinishedMsg{}
	}
}

type editorFinishedMsg struct {
	err  error
	file *os.File
}

func openEditorCmd(editor []string, initialMsg ...string) tea.Cmd {
	log.Debug("tui.openEditorCmd(): opening editor")
	var args []string

	file, err := os.CreateTemp(os.TempDir(), "")
	if err != nil {
		log.Debug(fmt.Sprintf("tui.openEditorCmd(): error: %v", err))
		return func() tea.Msg {
			return errMsg{error: err}
		}
	}

	if len(initialMsg) > 0 {
		for _, msg := range initialMsg {
			_, err = file.WriteString(msg)
			if err != nil {
				log.Debug(fmt.Sprintf("tui.openEditorCmd(): error: %v", err))
				return func() tea.Msg {
					return errMsg{error: err}
				}
			}
		}
	}

	args = append(args, editor[1:]...)
	args = append(args, file.Name())

	c := exec.Command(editor[0], args...)

	log.Debug(fmt.Sprintf("%+v", c))
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			log.Debug(fmt.Sprintf("tui.openEditorCmd(): error: %v", err))
			return errMsg{error: err}
		}
		return editorFinishedMsg{err, file}
	})
}

type parseTemplateForNoteMsg string
type parsedTemplateForNoteMsg struct {
	content string
	err     error
}

func parseTemplateForNote(p *pagerduty.Incident) tea.Cmd {
	return func() tea.Msg {
		content, err := addNoteTemplate(p.HTMLURL, p.Title, p.Service.Summary)
		return parsedTemplateForNoteMsg{content, err}
	}
}

type loginMsg string
type loginFinishedMsg struct {
	err error
}

func login(vars map[string]string, launcher launcher.ClusterLauncher) tea.Cmd {
	// The first element of Terminal is the command to be executed, followed by args, in order
	// This handles if folks use, eg: flatpak run <some package> as a terminal.
	command := launcher.BuildLoginCommand(vars)
	c := exec.Command(command[0], command[1:]...)

	log.Debug(fmt.Sprintf("tui.login(): %v", c.String()))

	var stdOut io.ReadCloser
	var stdOutPipeErr error
	stdOut, stdOutPipeErr = c.StdoutPipe()
	if stdOutPipeErr != nil {
		log.Warn("tui.login():", "stdOutPipeErr", stdOutPipeErr.Error())
		return func() tea.Msg {
			return loginFinishedMsg{stdOutPipeErr}
		}
	}

	var stdErr io.ReadCloser
	var stdErrPipeErr error
	stdErr, stdErrPipeErr = c.StderrPipe()
	if stdErrPipeErr != nil {
		log.Warn("tui.login():", "stdErrErr", stdErrPipeErr.Error())
		return func() tea.Msg {
			return loginFinishedMsg{stdErrPipeErr}
		}
	}

	startCmdErr := c.Start()
	if startCmdErr != nil {
		log.Warn("tui.login():", "startCmdErr", startCmdErr.Error())
		return func() tea.Msg {
			return loginFinishedMsg{startCmdErr}
		}
	}

	var out []byte
	var stdOutReadErr error
	out, stdOutReadErr = io.ReadAll(stdOut)
	if stdOutReadErr != nil {
		log.Warn("tui.login():", "stdOutReadErr", stdOutReadErr.Error())
		return func() tea.Msg {
			return loginFinishedMsg{stdOutReadErr}
		}
	}

	var errOut []byte
	var stdErrReadErr error
	errOut, stdErrReadErr = io.ReadAll(stdErr)
	if stdErrReadErr != nil {
		log.Warn("tui.login():", "stdErrReadErr", stdErrReadErr.Error())
		return func() tea.Msg {
			return loginFinishedMsg{stdErrReadErr}
		}
	}

	var err error
	if len(errOut) > 0 {
		err = errors.New(string(errOut))
	}

	processErr := c.Wait()

	if processErr != nil {
		if exitError, ok := processErr.(*exec.ExitError); ok {
			execExitErr := &execErr{
				Err:        processErr,
				ExitErr:    exitError,
				ExecStdErr: string(errOut),
			}
			log.Warn("tui.login():", "processErr", execExitErr)
			return func() tea.Msg {
				return loginFinishedMsg{execExitErr}
			}
		}
		log.Warn("tui.login():", "processErr", processErr.Error())
		return func() tea.Msg {
			return loginFinishedMsg{processErr}
		}
	}

	if err != nil {
		log.Warn("tui.login():", "execStdErr", err.Error())
		return func() tea.Msg {
			// Do not return the execStdErr as an error
			return loginFinishedMsg{}
		}
	}

	var stdOutAsErr error
	if len(out) > 0 {
		stdOutAsErr = errors.New(string(out))
		log.Warn("tui.login():", "stdOutAsErr", stdOutAsErr.Error())
		return func() tea.Msg {
			return loginFinishedMsg{stdOutAsErr}
		}
	}

	return func() tea.Msg {
		return loginFinishedMsg{}
	}
}

type clearSelectedIncidentsMsg string

type acknowledgeIncidentsMsg struct {
	incidents []pagerduty.Incident
}
type unAcknowledgeIncidentsMsg struct {
	incidents []pagerduty.Incident
}
type acknowledgedIncidentsMsg struct {
	incidents []pagerduty.Incident
	err       error
}
type unAcknowledgedIncidentsMsg struct {
	incidents []pagerduty.Incident
	err       error
}

func acknowledgeIncidents(p *pd.Config, incidents []pagerduty.Incident, reEscalate bool) tea.Cmd {
	return func() tea.Msg {
		var err error

		if reEscalate {
			a, err := pd.AcknowledgeIncident(p.Client, incidents, &pagerduty.User{})
			return unAcknowledgedIncidentsMsg{a, err}
		}

		a, err := pd.AcknowledgeIncident(p.Client, incidents, p.CurrentUser)

		return acknowledgedIncidentsMsg{a, err}
	}
}

type reassignIncidentsMsg struct {
	incidents []pagerduty.Incident
	users     []*pagerduty.User
}
type reassignedIncidentsMsg []pagerduty.Incident

func reassignIncidents(p *pd.Config, i []pagerduty.Incident, users []*pagerduty.User) tea.Cmd {
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

type reEscalateIncidentsMsg struct {
	incidents []pagerduty.Incident
	policy    *pagerduty.EscalationPolicy
	level     uint
}

type reEscalatedIncidentsMsg []pagerduty.Incident

func reEscalateIncidents(p *pd.Config, i []pagerduty.Incident, e *pagerduty.EscalationPolicy, l uint) tea.Cmd {
	return func() tea.Msg {
		r, err := pd.ReEscalateIncidents(p.Client, i, e, l)
		if err != nil {
			return errMsg{err}
		}
		return reEscalatedIncidentsMsg(r)
	}
}

type silenceSelectedIncidentMsg struct{}
type silenceIncidentsMsg struct {
	incidents []pagerduty.Incident
}

var errSilenceIncidentInvalidArgs = errors.New("silenceIncidents: invalid arguments")

func silenceIncidents(incidents []pagerduty.Incident, policy *pagerduty.EscalationPolicy, level uint) tea.Cmd {
	// SilenceIncidents doesn't have it's own "silencedIncidentsMessage" because it's really just a re-escalation
	if policy == nil {
		log.Debug("silenceIncidents: nil escalation policy provided")
		return func() tea.Msg {
			return errMsg{errSilenceIncidentInvalidArgs}
		}
	}

	if len(incidents) == 0 {
		log.Debug("silenceIncidents: no incidents provided")
		return func() tea.Msg {
			return errMsg{errSilenceIncidentInvalidArgs}
		}
	}

	if policy.Name == "" || policy.ID == "" || level == 0 {
		log.Debug("silenceIncidents: invalid arguments", "incident(s) count: %d; policy: %s(%s), level: %d", len(incidents), policy.Name, policy.ID, level)
		return func() tea.Msg {
			return errMsg{errSilenceIncidentInvalidArgs}
		}
	}

	incidentIDs := getIDsFromIncidents(incidents)

	log.Printf("silence requested for incident(s) %v; reassigning to %s(%s), level %d", incidentIDs, policy.Name, policy.ID, level)

	return func() tea.Msg {
		return reEscalateIncidentsMsg{incidents, policy, level}
	}
}

//lint:ignore U1000 - future proofing
type addIncidentNoteMsg string
type addedIncidentNoteMsg struct {
	note *pagerduty.IncidentNote
	err  error
}

func addNoteToIncident(p *pd.Config, incident *pagerduty.Incident, file *os.File) tea.Cmd {
	return func() tea.Msg {
		defer file.Close() //nolint:errcheck

		bytes, err := os.ReadFile(file.Name())
		if err != nil {
			return errMsg{err}
		}

		note := removeCommentsFromBytes(bytes, "#")

		u, err := p.Client.GetCurrentUserWithContext(context.Background(), pagerduty.GetCurrentUserOptions{})
		if err != nil {
			return errMsg{err}
		}

		if note != "" {
			n, err := pd.PostNote(p.Client, incident.ID, u, note)
			return addedIncidentNoteMsg{n, err}
		}

		return addedIncidentNoteMsg{nil, errors.New(nilNoteErr)}
	}
}

// removeCommentsFromBytes removes any lines beginning with any of the provided prefixes []byte and returns a string.
func removeCommentsFromBytes(b []byte, prefixes ...string) string {
	var content strings.Builder

	lines := strings.Split(string(b[:]), "\n")

	for _, c := range lines {
		for _, a := range prefixes {
			if !strings.HasPrefix(c, a) {
				content.WriteString(c)
			}
		}
	}

	return content.String()
}

func getDetailFieldFromAlert(f string, a pagerduty.IncidentAlert) string {
	if a.Body["details"] != nil {

		if a.Body["details"].(map[string]interface{})[f] != nil {
			return a.Body["details"].(map[string]interface{})[f].(string)
		}
		log.Debug(fmt.Sprintf("tui.getDetailFieldFromAlert(): alert body \"details\" does not contain field %s", f))
		return ""
	}
	log.Debug("tui.getDetailFieldFromAlert(): alert body \"details\" is nil")
	return ""
}

// getEscalationPolicyKey is a helper function to determine the escalation policy key
func getEscalationPolicyKey(serviceID string, policies map[string]*pagerduty.EscalationPolicy) string {
	if policy, ok := policies[serviceID]; ok {
		log.Debug("Update", "getEscalationPolicyKey", "escalation policy override found for service", "service", serviceID, "policy", policy.Name)
		return serviceID
	}
	log.Debug("Update", "getEscalationPolicyKey", "no escalation policy override for service; using default", "service", serviceID, "policy", silentDefaultPolicyKey)
	return silentDefaultPolicyKey
}

// stateShorthand returns the state of the incident as a single character
// A = acknowledged by user
// a = acknowledged by someone else
// X = stale (TODO: need to figure out how to do this)
// dot = triggered
func stateShorthand(i pagerduty.Incident, id string) string {
	switch {
	case AcknowledgedByUser(i, id):
		return "A"
	case acknowledged(i.Acknowledgements):
		return "a"
	default:
		return dot
	}
}

// acknowledged returns true if the incident has been acknowledged by anyone
func acknowledged(a []pagerduty.Acknowledgement) bool {
	return len(a) > 0
}

func doIfIncidentSelected(m *model, cmd tea.Cmd) tea.Cmd {
	if m.table.SelectedRow() == nil {
		log.Debug("doIfIncidentSelected", "selectedRow", "nil")
		m.viewingIncident = false
		return func() tea.Msg { return setStatusMsg{nilIncidentMsg} }
	}
	log.Debug("doIfIncidentSelected", "selectedRow", m.table.SelectedRow())
	return tea.Sequence(
		func() tea.Msg { return getIncidentMsg(m.table.SelectedRow()[1]) },
		cmd,
	)
}

func runScheduledJobs(m *model) []tea.Cmd {
	var cmds []tea.Cmd
	for _, job := range m.scheduledJobs {
		if time.Since(job.lastRun) > job.frequency && job.jobMsg != nil {
			log.Debug("Update: TicketMsg", "scheduledJob", job.jobMsg, "frequency", job.frequency, "lastRun", job.lastRun, "running", true)
			cmds = append(cmds, job.jobMsg)
			job.lastRun = time.Now()
		}
	}
	return cmds
}

func getIDsFromIncidents(incidents []pagerduty.Incident) []string {
	var ids []string
	for _, i := range incidents {
		ids = append(ids, i.ID)
	}
	return ids
}