package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"bytes"
	"path/filepath"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/ai"
	"github.com/clcollins/srepd/pkg/alert"
	pkgconfig "github.com/clcollins/srepd/pkg/config"
	"github.com/clcollins/srepd/pkg/launcher"
	"github.com/clcollins/srepd/pkg/ocm"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	watcherQueryTimeout     = 60 * time.Second
	watcherSynthesisTimeout = 30 * time.Second
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
		// Route through pd.GetIncident, which applies the default API timeout,
		// instead of calling the raw client with context.Background().
		i, err := pd.GetIncident(p.Client, id)
		return gotIncidentMsg{i, err}
	}
}

// gotIncidentAlertsMsg is a message that contains the fetched incident alerts
type gotIncidentAlertsMsg struct {
	incidentID string
	alerts     []pagerduty.IncidentAlert
	err        error
}

// getIncidentAlerts returns a command that fetches the alerts for the given incident
func getIncidentAlerts(p *pd.Config, id string) tea.Cmd {
	return func() tea.Msg {
		if id == "" {
			return setStatusMsg{nilIncidentMsg}
		}
		a, err := pd.GetAlerts(p.Client, id, pagerduty.ListIncidentAlertsOptions{})
		return gotIncidentAlertsMsg{incidentID: id, alerts: a, err: err}
	}
}

// gotIncidentNotesMsg is a message that contains the fetched incident notes
type gotIncidentNotesMsg struct {
	incidentID string
	notes      []pagerduty.IncidentNote
	err        error
}

// getIncidentNotes returns a command that fetches the notes for the given incident
func getIncidentNotes(p *pd.Config, id string) tea.Cmd {
	return func() tea.Msg {
		if id == "" {
			return setStatusMsg{nilIncidentMsg}
		}
		n, err := pd.GetNotes(p.Client, id)
		return gotIncidentNotesMsg{incidentID: id, notes: n, err: err}
	}
}

// updateIncidentListMsg is a message that triggers the fetching of the incident list
type updateIncidentListMsg string

// updatedIncidentListMsg is a message that contains the fetched incident list
type updatedIncidentListMsg struct {
	incidents []pagerduty.Incident
	err       error
}

// maxUserIDsInQuery is the maximum number of user IDs to include in a single
// PagerDuty API query. PagerDuty rejects request URIs over ~4096 bytes with
// HTTP 414 (URI Too Long); 100 user_ids[] params plus the team, status, and
// pagination params stay well under that limit.
const maxUserIDsInQuery = 100

// updateIncidentList returns a command that fetches the incident list from the PagerDuty API.
// It queries per-team, splitting large member lists into chunks of at most
// maxUserIDsInQuery user IDs per query to avoid HTTP 414 (URI Too Long),
// and deduplicates the merged results.
func updateIncidentList(p *pd.Config) tea.Cmd {
	return func() tea.Msg {
		if p == nil {
			return updatedIncidentListMsg{}
		}

		ignoredIDs := ignoredUserIDs(p.IgnoredUsers)
		seen := make(map[string]bool)
		var allIncidents []pagerduty.Incident

		for _, team := range p.Teams {
			memberIDs := filterUserIDs(p.TeamMembersByTeam[team.ID], ignoredIDs)

			chunks := chunkStrings(memberIDs, maxUserIDsInQuery)
			if len(chunks) == 0 {
				// No members to filter by: query the team alone, matching
				// the API behavior when user_ids[] is omitted
				chunks = [][]string{nil}
			}

			for _, chunk := range chunks {
				opts := pd.NewListIncidentOptsFromDefaults()
				opts.TeamIDs = []string{team.ID}
				opts.UserIDs = chunk

				incidents, err := pd.GetIncidents(p.Client, opts)
				if err != nil {
					return updatedIncidentListMsg{err: err}
				}

				for _, inc := range incidents {
					if !seen[inc.ID] {
						seen[inc.ID] = true
						allIncidents = append(allIncidents, inc)
					}
				}
			}
		}

		return updatedIncidentListMsg{incidents: allIncidents}
	}
}

func ignoredUserIDs(users []*pagerduty.User) []string {
	var ids []string
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	return ids
}

func filterUserIDs(memberIDs []string, ignored []string) []string {
	var filtered []string
	for _, id := range memberIDs {
		if !slices.Contains(ignored, id) {
			filtered = append(filtered, id)
		}
	}
	return filtered
}

func chunkStrings(items []string, size int) [][]string {
	var chunks [][]string
	for len(items) > size {
		chunks = append(chunks, items[:size])
		items = items[size:]
	}
	if len(items) > 0 {
		chunks = append(chunks, items)
	}
	return chunks
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

type aiHealthCheckMsg struct {
	healthy bool
	err     error
}

type watcherResponseMsg struct {
	response string
	err      error
}

func watcherQueryCmd(provider ai.Provider, systemPrompt string, userPrompt string, incidentContext string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), watcherQueryTimeout)
		defer cancel()

		fullPrompt := userPrompt
		if incidentContext != "" {
			fullPrompt = fmt.Sprintf("%s\n\nContext:\n%s", userPrompt, incidentContext)
		}

		log.Debug("watcher.query", "provider", provider.Name(), "prompt", userPrompt)

		response, err := provider.Query(ctx, systemPrompt, fullPrompt)
		if err != nil {
			log.Warn("watcher.query", "error", err)
			return watcherResponseMsg{err: err}
		}

		return watcherResponseMsg{response: response}
	}
}

func aiHealthCheckCmd(provider ai.Provider) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		checker, ok := provider.(ai.HealthChecker)
		if !ok {
			log.Debug("ai.HealthCheck", "msg", "provider does not support health checks", "provider", provider.Name())
			return aiHealthCheckMsg{healthy: true}
		}

		err := checker.Healthy(ctx)
		if err != nil {
			log.Warn("ai.HealthCheck", "provider", provider.Name(), "status", "unhealthy", "error", err)
			return aiHealthCheckMsg{healthy: false, err: err}
		}

		log.Debug("ai.HealthCheck", "provider", provider.Name(), "status", "healthy")
		return aiHealthCheckMsg{healthy: true}
	}
}

type watcherSynthesisMsg struct {
	observation string
	response    string
	err         error
}

func watcherSynthesizeCmd(provider ai.Provider, systemPrompt string, observation string, incidentSummary string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), watcherSynthesisTimeout)
		defer cancel()

		synthesisPrefix := "A pattern detector identified the following observation. " +
			"Provide a brief (1-3 sentence) analysis of what this pattern might indicate " +
			"and any suggested investigation steps. Be concise."
		userPrompt := fmt.Sprintf("%s\n\nObservation: %s\n\nCurrent incidents:\n%s", synthesisPrefix, observation, incidentSummary)

		log.Debug("watcher.synthesize", "provider", provider.Name(), "observation", observation)

		response, err := provider.Query(ctx, systemPrompt, userPrompt)
		if err != nil {
			log.Warn("watcher.synthesize", "error", err)
			return watcherSynthesisMsg{observation: observation, err: err}
		}

		return watcherSynthesisMsg{observation: observation, response: response}
	}
}

// clearFlashMsg is sent after a flash notification's display duration has elapsed.
// The status is only cleared if it still matches the original flash message,
// preventing newer messages from being prematurely dismissed.
type clearFlashMsg struct {
	message string
}

type PollIncidentsMsg struct{}

type lazyEnrichMsg struct{}

func pickNextEnrichment(m *model) tea.Cmd {
	if m.config == nil {
		return nil
	}

	rows := m.table.Rows()
	if len(rows) == 0 {
		return nil
	}

	cursor := m.table.Cursor()
	n := len(rows)

	for offset := 0; offset < n; offset++ {
		var indices []int
		if offset == 0 {
			indices = []int{cursor}
		} else {
			above := cursor - offset
			below := cursor + offset
			if below < n {
				indices = append(indices, below)
			}
			if above >= 0 {
				indices = append(indices, above)
			}
		}

		for _, idx := range indices {
			if idx < 0 || idx >= n {
				continue
			}
			row := rows[idx]
			if len(row) < 2 {
				continue
			}
			incidentID := row[1]
			if _, exists := m.incidentCache[incidentID]; exists {
				continue
			}
			return func() tea.Msg { return getIncidentMsg(incidentID) }
		}
	}

	return nil
}

// logFileContentMsg is a message containing the contents of the debug log file.
type logFileContentMsg string

// readLogFile returns a command that reads the log file at the given path,
// filtered to lines from the current session (on or after since).
func readLogFile(path string, since time.Time) tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(path)
		if err != nil {
			return logFileContentMsg(fmt.Sprintf("No log file found at %s", path))
		}
		if since.IsZero() {
			return logFileContentMsg(string(data))
		}
		lines := strings.Split(string(data), "\n")
		sinceStr := since.Format("2006/01/02 15:04:05")
		startIdx := len(lines)
		for i, line := range lines {
			if len(line) >= 19 && line[:19] >= sinceStr {
				startIdx = i
				break
			}
		}
		return logFileContentMsg(strings.Join(lines[startIdx:], "\n"))
	}
}

func readJournalLog(since time.Time) tea.Cmd {
	return func() tea.Msg {
		sinceStr := since.Format("2006-01-02 15:04:05")
		cmd := exec.Command("journalctl", "_COMM=srepd", "--since", sinceStr, "--no-pager")
		out, err := cmd.Output()
		if err != nil {
			return logFileContentMsg(fmt.Sprintf("Failed to read journal: %v", err))
		}
		if len(out) == 0 {
			return logFileContentMsg("No journal entries found for this session")
		}
		return logFileContentMsg(string(out))
	}
}

type renderIncidentMsg string

type renderedIncidentMsg struct {
	content string
	err     error
}

type renderDocsMsg string

type renderedDocsMsg struct {
	content string
	err     error
}

func renderDocsContent(m *model) tea.Cmd {
	return func() tea.Msg {
		content := m.renderDocsTabContent()

		rendered, err := renderIncidentMarkdown(m, content)
		if err != nil {
			return renderedDocsMsg{content, err}
		}

		return renderedDocsMsg{rendered, nil}
	}
}

func renderIncident(m *model) tea.Cmd {
	return func() tea.Msg {
		t, err := m.renderTabContent()
		if err != nil {
			return errMsg{err}
		}

		content, err := renderIncidentMarkdown(m, t)
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
		"tui.ShouldBeAcknowledged()",
		"assigned", assigned,
		"acknowledged", acknowledged,
		"autoAcknowledge", autoAcknowledge,
		"userIsOnCall", userIsOnCall,
		"doIt", doIt,
	)
	return doIt
}

// ShouldBeAcknowledgedCached checks if an incident should be auto-acknowledged using a cached userIsOnCall result.
// This avoids repeated calls to UserIsOnCall() when checking multiple incidents in the same update cycle.
func ShouldBeAcknowledgedCached(i pagerduty.Incident, id string, userIsOnCall bool) bool {
	return AssignedToUser(i, id) && !AcknowledgedByUser(i, id) && userIsOnCall
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
		log.Debug("tui.UserIsOnCall()", "error", err)
		return false
	}

	for _, o := range onCalls {
		log.Debug("tui.UserIsOnCall()", "on-call", o)

		start, err := time.Parse(timeLayout, o.Start)
		if err != nil {
			log.Debug("tui.UserIsOnCall()", "msg", "error parsing on-call start time", "error", err)
			return false
		}
		end, err := time.Parse(timeLayout, o.End)
		if err != nil {
			log.Debug("tui.UserIsOnCall()", "msg", "error parsing on-call end time", "error", err)
			return false
		}

		if start.Before(time.Now()) && end.After(time.Now()) {
			return true
		}
	}

	return false
}

// noAcknowledgeMsg is a no-op result from checkOnCallAndAcknowledge indicating
// that no incidents should be auto-acknowledged (user not on-call, on-call check
// failed, or no candidate matched). It is deliberately distinct from
// acknowledgeIncidentsMsg: emitting acknowledgeIncidentsMsg{incidents: nil} would
// fall back to acknowledging the currently selected incident (see the
// acknowledgeIncidentsMsg handler), which must never happen from the background
// auto-ack sweep.
type noAcknowledgeMsg struct{}

// checkOnCallAndAcknowledge runs the on-call check OFF the Bubble Tea Update loop
// (so a slow PagerDuty request never freezes the UI), then filters the candidate
// incidents by the FRESH on-call result and emits an acknowledgeIncidentsMsg for
// those that should be auto-acknowledged.
//
// On-call status is NEVER cached — it is checked live on every refresh. A cached
// value would keep auto-acknowledging incidents after the user's shift ends if they
// leave SREPD running; checking live means auto-ack stops within one refresh cycle
// of going off-call. For the same reason, the re-escalate/reassign paths must also
// perform a live check and must never read a cached on-call value.
func checkOnCallAndAcknowledge(p *pd.Config, id string, candidates []pagerduty.Incident) tea.Cmd {
	return func() tea.Msg {
		if !UserIsOnCall(p, id) {
			return noAcknowledgeMsg{}
		}

		var toAck []pagerduty.Incident
		for _, i := range candidates {
			if ShouldBeAcknowledgedCached(i, id, true) {
				toAck = append(toAck, i)
			}
		}

		if len(toAck) == 0 {
			return noAcknowledgeMsg{}
		}

		return acknowledgeIncidentsMsg{incidents: toAck}
	}
}

// TODO: Can we use a single function and struct to handle
// the openEditorCmd, login and openBrowserCmd commands?

type browserFinishedMsg struct {
	err error
}
type openBrowserMsg string
type openSOPMsg string

func openBrowserCmd(browser []string, url string) tea.Cmd {
	log.Debug("tui.openBrowserCmd()", "msg", "opening browser", "browser", browser, "url", url)

	var args []string
	args = append(args, browser[1:]...)
	args = append(args, url)

	c := exec.Command(browser[0], args...)
	log.Debug("tui.openBrowserCmd()", "command", c.String())

	err := c.Start()
	if err != nil {
		log.Debug("tui.openBrowserCmd()", "error", err)
		return func() tea.Msg {
			return browserFinishedMsg{err}
		}
	}

	// Browser opened successfully - don't wait for it or check stderr
	// Browsers write warnings/info to stderr that aren't actual errors

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
		log.Debug("tui.openEditorCmd()", "error", err)
		return func() tea.Msg {
			return errMsg{error: err}
		}
	}

	if len(initialMsg) > 0 {
		for _, msg := range initialMsg {
			_, err = file.WriteString(msg)
			if err != nil {
				log.Debug("tui.openEditorCmd()", "error", err)
				return func() tea.Msg {
					return errMsg{error: err}
				}
			}
		}
	}

	args = append(args, editor[1:]...)
	args = append(args, file.Name())

	c := exec.Command(editor[0], args...)

	log.Debug("tui.openEditorCmd()", "command", c)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			log.Debug("tui.openEditorCmd()", "error", err)
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

// clusterSelectedMsg is sent when the user picks a cluster from the
// multi-cluster selection prompt. The string value is the cluster_id.
type clusterSelectedMsg string

type rosaBoundaryLoginMsg string

type rosaBoundaryClusterSelectedMsg string

type loginFinishedMsg struct {
	err error
}

// buildPagerDutyEnvVars constructs a slice of "-e", "KEY=VALUE" pairs for passing
// PagerDuty incident context to ocm-container as individual environment variables.
// Only alerts whose cluster_id matches clusterID contribute to ALERT_NAMES and
// ALERT_LINKS. This replaces the previous approach of base64-encoding a JSON blob,
// which could exceed ARG_MAX with many alerts.
// sanitizeEnvValue removes or escapes characters that could cause issues
// in environment variable values passed via -e flags to terminals or containers.
func sanitizeEnvValue(s string) string {
	r := strings.NewReplacer(
		"\n", " ",
		"\r", "",
		"\t", " ",
		"'", "",
		"\"", "",
		"`", "",
		"$", "",
		"\\", "",
	)
	return r.Replace(s)
}

func buildPagerDutyEnvVars(incident *pagerduty.Incident, alerts []pagerduty.IncidentAlert, notes []pagerduty.IncidentNote, clusterID string) []string {
	var envFlags []string

	// Incident-level variables
	if incident != nil {
		envFlags = append(envFlags, "-e", fmt.Sprintf("PAGERDUTY_INCIDENT_ID=%s", incident.ID))
		envFlags = append(envFlags, "-e", fmt.Sprintf("PAGERDUTY_INCIDENT_TITLE=%s", sanitizeEnvValue(incident.Title)))
		envFlags = append(envFlags, "-e", fmt.Sprintf("PAGERDUTY_INCIDENT_URL=%s", incident.HTMLURL))
		envFlags = append(envFlags, "-e", fmt.Sprintf("PAGERDUTY_INCIDENT_SERVICE=%s", sanitizeEnvValue(incident.Service.Summary)))
		envFlags = append(envFlags, "-e", fmt.Sprintf("PAGERDUTY_INCIDENT_URGENCY=%s", incident.Urgency))
		envFlags = append(envFlags, "-e", fmt.Sprintf("PAGERDUTY_INCIDENT_STATUS=%s", incident.Status))
		envFlags = append(envFlags, "-e", fmt.Sprintf("REASON=%s", incident.HTMLURL))
	}

	// Cluster ID
	if clusterID != "" {
		envFlags = append(envFlags, "-e", fmt.Sprintf("PAGERDUTY_CLUSTER_ID=%s", clusterID))
	}

	// Filter alerts to those matching the selected cluster and collect names/links
	var matchingNames []string
	var matchingLinks []string
	for _, alert := range alerts {
		alertCluster := getDetailFieldFromAlert("cluster_id", alert)
		if clusterID != "" && alertCluster != clusterID {
			continue
		}

		name := getDetailFieldFromAlert("alert_name", alert)
		if name != "" {
			matchingNames = append(matchingNames, sanitizeEnvValue(name))
		}

		// Check both SOP link fields in priority order, same as getSOPLink
		for _, field := range sopLinkFields {
			link := getDetailFieldFromAlert(field, alert)
			if link != "" {
				matchingLinks = append(matchingLinks, link)
				break
			}
		}
	}

	envFlags = append(envFlags, "-e", fmt.Sprintf("PAGERDUTY_ALERT_COUNT=%s", strconv.Itoa(len(matchingNames))))
	envFlags = append(envFlags, "-e", fmt.Sprintf("PAGERDUTY_ALERT_NAMES=%s", strings.Join(matchingNames, ",")))
	envFlags = append(envFlags, "-e", fmt.Sprintf("PAGERDUTY_ALERT_LINKS=%s", strings.Join(matchingLinks, ",")))

	// Notes metadata
	notesExist := len(notes) > 0
	envFlags = append(envFlags, "-e", fmt.Sprintf("PAGERDUTY_NOTES_EXIST=%s", strconv.FormatBool(notesExist)))
	envFlags = append(envFlags, "-e", fmt.Sprintf("PAGERDUTY_NOTE_COUNT=%s", strconv.Itoa(len(notes))))

	// Claude Code detection
	if launcher.HasClaudeCode() {
		envFlags = append(envFlags, "-e", "PAGERDUTY_CLAUDE_AVAILABLE=true")
	}

	return envFlags
}

// commandContainsOCMContainer returns true if any element of the command
// slice contains "ocm-container", indicating this is an ocm-container flow
// where -e flags should be passed directly as container arguments.
func commandContainsOCMContainer(command []string) bool {
	for _, arg := range command {
		if strings.Contains(arg, "ocm-container") {
			return true
		}
	}
	return false
}

// insertOCMContainerEnvFlags inserts -e env flags into a command at the
// correct position for ocm-container: after the ocm-container command itself
// but before its other arguments (like --cluster-id).
func insertOCMContainerEnvFlags(command []string, envFlags []string) []string {
	if len(envFlags) == 0 {
		return command
	}

	// Find the ocm-container argument in the command slice
	ocmIdx := -1
	for i, arg := range command {
		if strings.Contains(arg, "ocm-container") {
			ocmIdx = i
			break
		}
	}

	log.Debug("tui.insertOCMContainerEnvFlags()", "ocmIdx", ocmIdx, "commandLen", len(command))

	if ocmIdx < 0 {
		// No ocm-container found; return command unchanged
		return command
	}

	// Insert env flags right after the ocm-container argument
	insertIdx := ocmIdx + 1
	result := make([]string, 0, len(command)+len(envFlags))
	result = append(result, command[:insertIdx]...)
	result = append(result, envFlags...)
	result = append(result, command[insertIdx:]...)

	return result
}

// extractEnvVarPairs extracts "KEY=VALUE" strings from a slice of
// ["-e", "KEY=VALUE", "-e", "KEY2=VALUE2", ...] pairs. It skips the "-e"
// elements and returns only the environment variable assignments, suitable
// for setting on exec.Cmd.Env.
func extractEnvVarPairs(envFlags []string) []string {
	var pairs []string
	for i := 0; i < len(envFlags)-1; i += 2 {
		if envFlags[i] == "-e" {
			pairs = append(pairs, envFlags[i+1])
		}
	}
	return pairs
}

func login(vars map[string]string, l launcher.ClusterLauncher, incident *pagerduty.Incident, alerts []pagerduty.IncidentAlert, notes []pagerduty.IncidentNote) tea.Cmd {
	// The first element of Terminal is the command to be executed, followed by args, in order
	// This handles if folks use, eg: flatpak run <some package> as a terminal.
	command := l.BuildLoginCommand(vars)

	// Build individual PAGERDUTY_* environment variables filtered to the selected cluster
	clusterID := vars["%%CLUSTER_ID%%"]
	envFlags := buildPagerDutyEnvVars(incident, alerts, notes, clusterID)

	// Determine the correct env var passing mechanism based on the command flow:
	// 1. ocm-container flow: use -e flags (they become podman -e flags)
	// 2. Non-ocm-container in toolbox: use --env= flags on flatpak-spawn
	// 3. Non-ocm-container, not in toolbox: set on exec.Cmd.Env

	var finalCommand []string
	var processEnvVars []string // Only used for the exec.Cmd.Env case

	if commandContainsOCMContainer(command) {
		// ocm-container flow: insert -e flags into the command for ocm-container
		finalCommand = insertOCMContainerEnvFlags(command, envFlags)
		log.Debug("tui.login(): ocm-container flow", "finalCommand", finalCommand)
	} else if l.IsToolbox() {
		// Non-ocm-container in toolbox: use --env= flags on flatpak-spawn
		// so that the host process launched via flatpak-spawn inherits them
		toolboxFlags := l.ToolboxEnvFlags(envFlags)
		finalCommand = launcher.InsertToolboxEnvFlags(command, toolboxFlags)
		log.Debug("tui.login(): toolbox non-ocm-container flow", "toolboxFlags", toolboxFlags, "finalCommand", finalCommand)
	} else {
		// Non-ocm-container, not in toolbox: set env vars on the process directly
		finalCommand = command
		processEnvVars = extractEnvVarPairs(envFlags)
		log.Debug("tui.login(): direct process env flow", "processEnvVars", processEnvVars, "finalCommand", finalCommand)
	}

	c := exec.Command(finalCommand[0], finalCommand[1:]...)

	// For the non-ocm-container, non-toolbox case, set env vars on the process
	if len(processEnvVars) > 0 {
		c.Env = append(os.Environ(), processEnvVars...)
	}

	log.Debug("tui.login(): original command", "command", command)
	log.Debug("tui.login(): env flags", "envFlags", envFlags)
	log.Debug("tui.login(): final command", "finalCommand", finalCommand)
	log.Debug("tui.login()", "command", c.String())

	startCmdErr := c.Start()
	if startCmdErr != nil {
		log.Error("tui.login()", "error", startCmdErr)
		return func() tea.Msg {
			return loginFinishedMsg{startCmdErr}
		}
	}

	// Reap the child process in background to avoid zombies.
	// Don't block — return immediately so srepd stays responsive.
	go func() {
		err := c.Wait()
		if err != nil {
			log.Debug("tui.login(): terminal process exited", "error", err)
		}
	}()

	return func() tea.Msg {
		return loginFinishedMsg{}
	}
}

func rosaBoundaryLogin(vars map[string]string, l launcher.ClusterLauncher) tea.Cmd {
	// rosa-boundary is a standalone CLI that manages its own interactive session
	// via session-manager-plugin. It should be executed directly without wrapping
	// in a terminal emulator (unlike ocm-container which needs a new terminal).
	// We build the command manually from the launcher's template instead of using
	// BuildLoginCommand which adds the terminal wrapper.
	command := l.BuildRosaBoundaryCommand(vars)

	c := exec.Command(command[0], command[1:]...)

	log.Debug("tui.rosaBoundaryLogin()", "command", c.String())

	startCmdErr := c.Start()
	if startCmdErr != nil {
		log.Error("tui.rosaBoundaryLogin()", "error", startCmdErr)
		return func() tea.Msg {
			return loginFinishedMsg{startCmdErr}
		}
	}

	go func() {
		err := c.Wait()
		if err != nil {
			log.Debug("tui.rosaBoundaryLogin(): command process exited", "error", err)
		}
	}()

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

func acknowledgeIncidents(p *pd.Config, incidents []pagerduty.Incident) tea.Cmd {
	return func() tea.Msg {
		a, err := pd.AcknowledgeIncident(p.Client, incidents, p.CurrentUser, p.CurrentUser)
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
		u, err := pd.GetCurrentUser(p.Client)
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
		r, err := pd.ReEscalateIncidents(p.Client, i, p.CurrentUser, e, l)
		if err != nil {
			return errMsg{err}
		}
		return reEscalatedIncidentsMsg(r)
	}
}

func fetchEscalationPolicyAndReEscalate(p *pd.Config, incidents []pagerduty.Incident, policyID string, level uint) tea.Cmd {
	return func() tea.Msg {
		// Fetch the full escalation policy details
		policy, err := pd.GetEscalationPolicy(p.Client, policyID, pagerduty.GetEscalationPolicyOptions{})
		if err != nil {
			log.Error("tui.fetchEscalationPolicyAndReEscalate()", "msg", "failed to fetch escalation policy", "policy_id", policyID, "error", err)
			return errMsg{err}
		}

		// Now re-escalate with the fetched policy
		r, err := pd.ReEscalateIncidents(p.Client, incidents, p.CurrentUser, policy, level)
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
type enterBulkSilenceMsg struct{}
type bulkSilenceConfirmedMsg struct {
	incidents []pagerduty.Incident
}

var errSilenceIncidentInvalidArgs = errors.New("silenceIncidents: invalid arguments")

func silenceIncidents(incidents []pagerduty.Incident, policy *pagerduty.EscalationPolicy, level uint) tea.Cmd {
	// SilenceIncidents doesn't have it's own "silencedIncidentsMessage" because it's really just a re-escalation
	if policy == nil {
		log.Debug("tui.silenceIncidents()", "msg", "nil escalation policy provided")
		return func() tea.Msg {
			return errMsg{errSilenceIncidentInvalidArgs}
		}
	}

	if len(incidents) == 0 {
		log.Debug("tui.silenceIncidents()", "msg", "no incidents provided")
		return func() tea.Msg {
			return errMsg{errSilenceIncidentInvalidArgs}
		}
	}

	if policy.Name == "" || policy.ID == "" || level == 0 {
		log.Debug("tui.silenceIncidents()", "msg", "invalid arguments", "incident_count", len(incidents), "policy_name", policy.Name, "policy_id", policy.ID, "level", level)
		return func() tea.Msg {
			return errMsg{errSilenceIncidentInvalidArgs}
		}
	}

	incidentIDs := getIDsFromIncidents(incidents)

	log.Info("tui.silenceIncidents()", "msg", "silence requested", "incidents", incidentIDs, "policy_name", policy.Name, "policy_id", policy.ID, "level", level)

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

		u, err := pd.GetCurrentUser(p.Client)
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

// getDetailFieldFromAlert safely extracts a string field from alert body details.
// Returns "" if body, details, or field is missing or wrong type. Never panics.
func getDetailFieldFromAlert(f string, a pagerduty.IncidentAlert) string {
	if a.Body == nil {
		return ""
	}
	detailsRaw, ok := a.Body["details"]
	if !ok || detailsRaw == nil {
		return ""
	}
	details, ok := detailsRaw.(map[string]interface{})
	if !ok {
		return ""
	}
	fieldRaw, ok := details[f]
	if !ok || fieldRaw == nil {
		return ""
	}
	fieldStr, ok := fieldRaw.(string)
	if !ok {
		return ""
	}
	return fieldStr
}

// sopLinkFields lists the alert detail field names that may contain an SOP URL,
// checked in priority order. "link" is used by most SRE alerts as a label;
// "runbook_url" is the Prometheus/Alertmanager annotation convention.
var sopLinkFields = []string{"link", "runbook_url"}

// getSOPLink extracts the SOP link from alerts using the alert normalization engine.
// Falls back to raw detail field extraction for backward compatibility.
// Returns the URL and true if found, or "" and false if no SOP link exists.
func getSOPLink(alerts []pagerduty.IncidentAlert) (string, bool) {
	// Try normalized extraction first (handles firing text parsing for appsre, notes for DMS)
	for _, a := range alerts {
		normalized := alert.NormalizeAlert(a.Service.Summary, "", a)
		if normalized.SOPLink != "" {
			return normalized.SOPLink, true
		}
	}

	// Fallback to raw detail field extraction for backward compatibility
	for _, a := range alerts {
		for _, field := range sopLinkFields {
			link := getDetailFieldFromAlert(field, a)
			if link != "" {
				return link, true
			}
		}
	}
	return "", false
}

// getUniqueClusters extracts deduplicated cluster_ids from alerts, preserving
// the order of first appearance. Alerts without a cluster_id are skipped, as are
// values that are not well-formed cluster IDs (ocm.ValidClusterID). Cluster IDs
// originate from attacker-influenceable PagerDuty alert data and are later
// substituted into launched commands, so rejecting malformed values here prevents
// argument injection at the launcher boundary.
func getUniqueClusters(alerts []pagerduty.IncidentAlert) []string {
	seen := make(map[string]bool)
	var clusters []string
	for _, a := range alerts {
		cluster := getDetailFieldFromAlert("cluster_id", a)
		if cluster == "" {
			normalized := alert.NormalizeAlert(a.Service.Summary, "", a)
			cluster = normalized.ClusterID
		}
		if cluster == "" || seen[cluster] {
			continue
		}
		if !ocm.ValidClusterID(cluster) {
			log.Warn("skipping malformed cluster_id from alert data", "cluster_id", cluster)
			continue
		}
		seen[cluster] = true
		clusters = append(clusters, cluster)
	}
	return clusters
}

func mapClusterServices(alerts []pagerduty.IncidentAlert) map[string]string {
	result := make(map[string]string)
	for _, a := range alerts {
		cluster := getDetailFieldFromAlert("cluster_id", a)
		if cluster == "" {
			normalized := alert.NormalizeAlert(a.Service.Summary, "", a)
			cluster = normalized.ClusterID
		}
		if cluster != "" {
			if _, exists := result[cluster]; !exists {
				result[cluster] = a.Service.Summary
			}
		}
	}
	return result
}

// getEscalationPolicyKey is a helper function to determine the escalation policy key
func getEscalationPolicyKey(serviceID string, policies map[string]*pagerduty.EscalationPolicy) string {
	if _, ok := policies[serviceID]; ok {
		return serviceID
	}
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

// filterByUrgency returns a filtered copy of incidents based on urgency.
// When showLow is true, all incidents are returned unchanged.
// When showLow is false, only high-urgency incidents are returned.
func filterByUrgency(incidents []pagerduty.Incident, showLow bool) []pagerduty.Incident {
	if showLow {
		return incidents
	}

	var filtered []pagerduty.Incident
	for _, i := range incidents {
		if i.Urgency == "high" {
			filtered = append(filtered, i)
		}
	}
	return filtered
}

func getIDsFromIncidents(incidents []pagerduty.Incident) []string {
	var ids []string
	for _, i := range incidents {
		ids = append(ids, i.ID)
	}
	return ids
}

type fetchedTeamsMsg struct {
	teams []pagerduty.Team
	err   error
}

type teamsSelectedMsg struct {
	ids   []string
	names map[string]string
}

type teamsConfigUpdatedMsg struct{ err error }

func fetchUserTeams(client pd.PagerDutyClient) tea.Cmd {
	return func() tea.Msg {
		teams, err := pd.GetCurrentUserTeams(client)
		return fetchedTeamsMsg{teams: teams, err: err}
	}
}

func writeTeamsToConfigCmd(teamIDs []string, teamNames map[string]string) tea.Cmd {
	return func() tea.Msg {
		home, err := os.UserHomeDir()
		if err != nil {
			return teamsConfigUpdatedMsg{err: fmt.Errorf("failed to get home directory: %w", err)}
		}

		configFile := filepath.Join(home, ".config", "srepd", "srepd.yaml")

		data, err := os.ReadFile(configFile)
		if err != nil {
			return teamsConfigUpdatedMsg{err: fmt.Errorf("failed to read config: %w", err)}
		}

		updated, err := updateTeamsInYAML(data, teamIDs, teamNames)
		if err != nil {
			return teamsConfigUpdatedMsg{err: err}
		}

		// 0600: the config file (and its backup) contain the plaintext PagerDuty
		// token and must not be world-readable. Chmod after WriteFile because
		// WriteFile's mode is ignored when the file already exists.
		backupFile := configFile + "~"
		if err := os.WriteFile(backupFile, data, 0600); err != nil {
			return teamsConfigUpdatedMsg{err: fmt.Errorf("failed to create config backup: %w", err)}
		}
		if err := os.Chmod(backupFile, 0600); err != nil {
			return teamsConfigUpdatedMsg{err: fmt.Errorf("failed to secure config backup: %w", err)}
		}

		if err := os.WriteFile(configFile, updated, 0600); err != nil {
			return teamsConfigUpdatedMsg{err: fmt.Errorf("failed to write config: %w", err)}
		}
		if err := os.Chmod(configFile, 0600); err != nil {
			return teamsConfigUpdatedMsg{err: fmt.Errorf("failed to secure config: %w", err)}
		}

		return teamsConfigUpdatedMsg{}
	}
}

func updateTeamsInYAML(configData []byte, teamIDs []string, teamNames map[string]string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(configData, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("invalid YAML document structure")
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected YAML mapping at root")
	}

	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == "teams" {
			teamsValue := root.Content[i+1]
			teamsValue.Content = nil
			if len(teamIDs) == 0 {
				teamsValue.Kind = yaml.SequenceNode
				teamsValue.Tag = "!!seq"
				teamsValue.Style = yaml.FlowStyle
			} else {
				teamsValue.Kind = yaml.SequenceNode
				teamsValue.Tag = "!!seq"
				teamsValue.Style = 0
				for _, id := range teamIDs {
					node := &yaml.Node{
						Kind:  yaml.ScalarNode,
						Tag:   "!!str",
						Value: id,
					}
					if name, ok := teamNames[id]; ok {
						node.LineComment = name
					}
					teamsValue.Content = append(teamsValue.Content, node)
				}
			}

			var buf bytes.Buffer
			enc := yaml.NewEncoder(&buf)
			enc.SetIndent(2)
			if err := enc.Encode(&doc); err != nil {
				return nil, fmt.Errorf("failed to encode config YAML: %w", err)
			}
			if err := enc.Close(); err != nil {
				return nil, fmt.Errorf("failed to close YAML encoder: %w", err)
			}
			return buf.Bytes(), nil
		}
	}

	return nil, fmt.Errorf("'teams' key not found in config")
}

// --- Config wizard message types and commands ---

// configFormState holds form field values as a pointer struct so that
// huh form Value() bindings survive bubbletea's model copying.
type configFormState struct {
	TokenInput    string
	SelectedTeams []string
	// SilentPolicyChoice is the picker selection: a policy ID,
	// policyChoiceSkip, or policyChoiceManual (reveals the free-text input
	// bound to SilentPolicy).
	SilentPolicyChoice string
	SilentPolicy       string
	CustomInput        string
	KeepTeams          bool
	KeepSilent         bool
	KeepCustom         bool
	// AdvancedOptions gates the team-policy groups (custom service→policy
	// mappings) that most users should never see; defaults to No.
	AdvancedOptions bool
	Confirm         bool
	// Environment step (OB-5).
	TerminalChoice string
	EditorInput    string
	AgentEnabled   bool
	// AgentOffered records whether the AI section was shown at all —
	// hidden means "keep the existing agent setting".
	AgentOffered bool
	// FetchedUserName/FetchedTeamCount are set by the team OptionsFunc after
	// a successful fetch and drive the greeting DescriptionFunc.
	FetchedUserName  string
	FetchedTeamCount int
	// PresetCommandsSafe/PresetSourceTrusted are the extra safety
	// confirmations shown after "Save changes?" when a --preset seeded
	// fields that srepd executes (terminal, editor, cluster login). Both
	// default to false and both must be affirmed or the save is discarded.
	PresetCommandsSafe  bool
	PresetSourceTrusted bool
}

// configWizardReadyMsg is sent when the existing config has been resolved
// and is ready to build the config wizard form.
type configWizardReadyMsg struct {
	existing      pkgconfig.ExistingConfig
	kd            pkgconfig.KeepDefaults
	isNewFile     bool
	teamNames     map[string]string
	policyNames   map[string]string
	presetApplied pkgconfig.PresetApplied
	// wizardReason explains why the wizard auto-launched (e.g. YAML parse
	// error, missing/placeholder token). Surfaced in the token step description
	// so the user understands what happened. Empty for explicit `srepd config`.
	wizardReason string
}

// configSavedMsg is sent after the config has been written to disk.
type configSavedMsg struct{ err error }

// configCompletedMsg is sent when the config form completes and values
// have been resolved for writing.
type configCompletedMsg struct {
	final          pkgconfig.ResolvedValues
	changes        pkgconfig.ConfigChanges
	teamNames      map[string]string
	customPolicies map[string]string
	isNewFile      bool
}

// prepareConfigWizardCmd resolves existing config from Viper and checks
// if the config file exists, returning a configWizardReadyMsg.
func prepareConfigWizardCmd(m model) tea.Cmd {
	return func() tea.Msg {
		existing := pkgconfig.ResolveExistingConfig(
			viper.GetString("token"),
			viper.GetStringSlice("teams"),
			viper.GetString("default_silent_escalation_policy"),
			viper.GetStringMapString("custom_service_escalation_policies"),
			viper.GetString("custom_service_escalation_policies"),
			viper.GetStringMapString("service_escalation_policies"),
		)
		existing.Terminal = viperConfiguredString("terminal")
		existing.Editor = viperConfiguredString("editor")
		existing.ClusterLoginCommand = viperConfiguredString("cluster_login_command")
		existing.AgentCLICommand = viper.GetString("agent_cli_command")

		var presetApplied pkgconfig.PresetApplied
		if ref := viper.GetString("config_preset"); ref != "" {
			preset, presetErr := pkgconfig.LoadPreset(ref, nil)
			if presetErr != nil {
				return errMsg{fmt.Errorf("failed to load preset: %w", presetErr)}
			}
			existing, presetApplied = pkgconfig.ApplyPreset(existing, preset)
		}

		kd := pkgconfig.ResolveKeepDefaults(existing.Teams, existing.SilentPolicy, existing.CustomPolicies)

		home, _ := os.UserHomeDir()
		configFile := filepath.Join(home, pkgconfig.CfgFileDir, pkgconfig.CfgFileName)
		isNewFile := false
		if _, err := os.Stat(configFile); errors.Is(err, os.ErrNotExist) {
			isNewFile = true
		}

		teamNames := make(map[string]string)
		policyNames := make(map[string]string)
		if existing.Token != "" {
			client := pd.NewClient(existing.Token)
			teams, err := pd.GetCurrentUserTeams(client)
			if err == nil {
				for _, team := range teams {
					teamNames[team.ID] = team.Name
				}
			}

			policyIDs := make(map[string]bool)
			if existing.SilentPolicy != "" {
				policyIDs[existing.SilentPolicy] = true
			}
			for _, polID := range existing.CustomPolicies {
				policyIDs[polID] = true
			}
			for id := range policyIDs {
				pol, polErr := pd.GetEscalationPolicy(client, id, pagerduty.GetEscalationPolicyOptions{})
				if polErr == nil && pol != nil {
					policyNames[id] = pol.Name
				}
			}
		}

		return configWizardReadyMsg{
			existing:      existing,
			kd:            kd,
			isNewFile:     isNewFile,
			teamNames:     teamNames,
			policyNames:   policyNames,
			presetApplied: presetApplied,
			wizardReason:  viper.GetString("config_wizard_reason"),
		}
	}
}

// viperConfiguredString returns the value for key only when the user set it
// (config file, env var, or an explicit flag) — not when it merely carries
// an in-process default from ensureViperDefaults/validateConfig.
func viperConfiguredString(key string) string {
	if viper.InConfig(key) {
		return viper.GetString(key)
	}
	if v := os.Getenv("SREPD_" + strings.ToUpper(key)); v != "" {
		return v
	}
	return ""
}

// realFS implements pkgconfig.ConfigFS using the real filesystem.
type realFS struct{}

func (realFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (realFS) OpenFile(name string, flag int, perm os.FileMode) (io.WriteCloser, error) {
	return os.OpenFile(name, flag, perm)
}

func (realFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (realFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (realFS) Chmod(name string, mode os.FileMode) error {
	return os.Chmod(name, mode)
}

// detectGenerateEnvironment probes the current system for terminals, editor,
// agent CLI, and cluster login commands — the same detection that
// `srepd config generate` uses — so a brand-new config file includes
// commented alternatives and detected values.
func detectGenerateEnvironment() *pkgconfig.GenerateEnvironment {
	env := &pkgconfig.GenerateEnvironment{}
	detected := launcher.DetectTerminals(exec.LookPath, os.Getenv, runtime.GOOS)
	for _, dt := range detected {
		env.Terminals = append(env.Terminals, dt.Command)
	}
	if e := os.Getenv("EDITOR"); e != "" {
		env.Editor = e
	} else if v := os.Getenv("VISUAL"); v != "" {
		env.Editor = v
	}
	if _, err := exec.LookPath("claude"); err == nil {
		env.AgentCLI = pkgconfig.DefaultOptionalKeys["agent_cli_command"]
	}
	if _, err := exec.LookPath("ocm"); err == nil {
		env.ClusterLoginCmds = append(env.ClusterLoginCmds, "ocm backplane login %%CLUSTER_ID%%")
	}
	if _, err := exec.LookPath("ocm-container"); err == nil {
		env.ClusterLoginCmds = append(env.ClusterLoginCmds, "ocm-container --cluster-id %%CLUSTER_ID%%")
	}
	return env
}

// writeConfigCmd writes the config to disk using the resolved values.
func writeConfigCmd(final pkgconfig.ResolvedValues, changes pkgconfig.ConfigChanges, teamNames map[string]string, customPolicies map[string]string, isNewFile bool, fs pkgconfig.ConfigFS) tea.Cmd {
	return func() tea.Msg {
		home, err := os.UserHomeDir()
		if err != nil {
			return configSavedMsg{err: err}
		}
		configDir := filepath.Join(home, pkgconfig.CfgFileDir)
		// 0700: the config dir holds the token-bearing config file; keep it
		// owner-only.
		if err := fs.MkdirAll(configDir, 0700); err != nil {
			return configSavedMsg{err: err}
		}
		// For new files, detect the environment so the generated config
		// includes commented alternatives (detected terminals, etc.)
		// matching `srepd config generate` output.
		var env *pkgconfig.GenerateEnvironment
		if isNewFile {
			env = detectGenerateEnvironment()
		}
		if err := pkgconfig.WriteConfig(fs, home, final, changes, teamNames, customPolicies, isNewFile, env); err != nil {
			return configSavedMsg{err: err}
		}
		// Update Viper with new values
		viper.Set("token", final.Token)
		viper.Set("teams", final.Teams)
		if final.SilentPolicy != "" {
			viper.Set("default_silent_escalation_policy", final.SilentPolicy)
		}
		// Set defaults for optional keys
		for k, v := range pkgconfig.DefaultOptionalKeys {
			if viper.GetString(k) == "" {
				viper.Set(k, v)
			}
		}
		return configSavedMsg{err: nil}
	}
}

// OCMClientReadyMsg is sent when async OCM authentication completes.
// Exported because cmd/root.go sends it via p.Send().
type OCMClientReadyMsg struct {
	Client ocm.OCMClient
	Err    error
}

// connectOCMCmdIfNeeded returns a command that connects OCM after the config
// wizard exits into a live session, so cluster enrichment works exactly like
// a normal launch. OCM auth is deliberately skipped while the wizard runs
// (OB-6); this is the follow-through that keeps the first session fully
// functional. Returns nil when OCM is already connected or in dev mode. The
// connection (including browser auth for expired tokens) blocks inside the
// tea.Cmd goroutine, and the result flows through the existing
// OCMClientReadyMsg handler (client, deferred backplane, enrichment).
func (m model) connectOCMCmdIfNeeded() tea.Cmd {
	if m.ocmClient != nil || m.devMode {
		return nil
	}
	connect := m.ocmConnect
	if connect == nil {
		connect = func() (ocm.OCMClient, error) {
			client, err := ocm.Connect(Version)
			if err != nil {
				return nil, err
			}
			return client, nil
		}
	}
	return func() tea.Msg {
		client, err := connect()
		if err != nil || client == nil {
			return OCMClientReadyMsg{Err: err}
		}
		return OCMClientReadyMsg{Client: client}
	}
}

// ocmHandoffCmd wraps connectOCMCmdIfNeeded for the wizard-exit paths,
// setting ocmAuthPending so the UI reflects the in-flight connection. When
// requirePDConfig is true (discard/no-changes/abort paths), the handoff only
// happens if a usable PD config exists — a brand-new user backing out of the
// wizard must not get a browser-auth prompt on the way out.
func (m *model) ocmHandoffCmd(requirePDConfig bool) tea.Cmd {
	if requirePDConfig && (m.config == nil || m.config.Client == nil) {
		return nil
	}
	cmd := m.connectOCMCmdIfNeeded()
	if cmd != nil {
		m.ocmAuthPending = true
	}
	return cmd
}

type pdClientInitializedMsg struct {
	config *pd.Config
	err    error
}

func initPDClientCmd() tea.Cmd {
	return func() tea.Msg {
		pdConfig, err := pd.NewConfig(
			viper.GetString("token"),
			viper.GetStringSlice("teams"),
			viper.GetStringMapString("service_escalation_policies"),
			viper.GetStringSlice("ignoredusers"),
			viper.GetString("default_silent_escalation_policy"),
			viper.GetStringMapString("custom_service_escalation_policies"),
		)
		return pdClientInitializedMsg{config: pdConfig, err: err}
	}
}
