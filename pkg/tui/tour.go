package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	pkgconfig "github.com/clcollins/srepd/pkg/config"
	"github.com/spf13/viper"
)

// The guided tour (#324): an in-app walkthrough of srepd's features,
// launched via :tour or suggested once after the first config save. It
// renders the incident table for context with an explanation panel beneath —
// keys are described, never executed, so it is safe with live incidents.

type tourStep struct {
	Title string
	Body  string
}

func tourSteps() []tourStep {
	return []tourStep{
		{
			Title: "The incident table",
			Body: "This is your incident list: ID, cluster/service, and title.\n" +
				"● marks incidents assigned to you; the view auto-refreshes.\n" +
				"Press t to toggle between your incidents and the whole team's.",
		},
		{
			Title: "Navigation",
			Body: "Move with j/k or the arrow keys. Press enter on an incident\n" +
				"to open it, and esc to come back to this table.",
		},
		{
			Title: "The incident viewer tabs",
			Body: "An open incident has tabs — Details, Alerts, Notes, Cluster,\n" +
				"Service Logs, LS History, Reports, and PD History — switch with\n" +
				"tab/shift+tab or number keys. Cluster data is enriched from OCM.",
		},
		{
			Title: "Key actions",
			Body: "a — acknowledge the selected incident.\n" +
				"ctrl+s — silence it (reassigns to your silent escalation policy).\n" +
				"ctrl+e — re-escalate to the next level.\n" +
				"n — add a note. l — log into the incident's cluster.",
		},
		{
			Title: "Command mode",
			Body: "Press : to enter command mode. :agent <question> asks the AI\n" +
				"agent for read-only incident investigation; :watcher <question>\n" +
				"queries the ambient LLM watcher.",
		},
		{
			Title: "Chord commands",
			Body: "Less-common actions live behind a chord prefix (default ctrl+x).\n" +
				"Press ctrl+x then ? to see every chord — for example ctrl+x s\n" +
				"for bulk silence.",
		},
		{
			Title: "Flags",
			Body: "Mark incidents matching patterns with :flag (e.g. a cluster ID\n" +
				"or org name) — matches get a marker in the table. :flags lists\n" +
				"conditions; :unflag removes them.",
		},
		{
			Title: "The watcher",
			Body: "Press w to toggle the AI watcher pane: ambient analysis of\n" +
				"incoming incidents and pattern detection across your queue,\n" +
				"streamed below the table.",
		},
		{
			Title: "That's the tour!",
			Body: "Press h any time for the full help view, and re-run this tour\n" +
				"with :tour. Happy on-calling!",
		},
	}
}

// isTourCommand reports whether the command input starts the tour.
func isTourCommand(input string) bool {
	return strings.TrimSpace(input) == ":tour"
}

// startTour enters tour mode and persists tour_seen so the one-time
// post-setup suggestion never repeats.
func (m model) startTour() (model, tea.Cmd) {
	m.tourMode = true
	m.tourStep = 0
	m.table.Blur()
	return m, markTourSeenCmd()
}

// switchTourFocusMode handles keys while the tour is active: esc/q/ctrl+c
// exits, shift+tab goes back, anything else advances; advancing past the
// last step completes the tour.
func switchTourFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "esc", "q", "ctrl+c":
		m.tourMode = false
		m.table.Focus()
		m.setStatus("tour ended — re-run any time with :tour")
		return m, nil
	case "shift+tab", "left":
		if m.tourStep > 0 {
			m.tourStep--
		}
		return m, nil
	default:
		m.tourStep++
		if m.tourStep >= len(tourSteps()) {
			m.tourMode = false
			m.table.Focus()
			m.setStatus("tour complete — press h for help any time")
		}
		return m, nil
	}
}

// renderTourPanel renders the current tour step in the app's pane language.
func (m model) renderTourPanel() string {
	steps := tourSteps()
	if m.tourStep < 0 || m.tourStep >= len(steps) {
		return ""
	}
	step := steps[m.tourStep]

	title := m.styles.Main.Bold(true).Render(step.Title)
	progress := m.styles.Muted.Render(fmt.Sprintf("%d/%d", m.tourStep+1, len(steps)))
	help := m.styles.Muted.Render("any key: next • shift+tab: back • esc: exit")

	content := fmt.Sprintf("%s  %s\n\n%s\n\n%s", title, progress, step.Body, help)
	return m.styles.FormContainer.Render(content)
}

// markTourSeenCmd persists tour_seen: true so the post-setup tour
// suggestion is shown at most once.
func markTourSeenCmd() tea.Cmd {
	return func() tea.Msg {
		viper.Set("tour_seen", true)

		home, err := os.UserHomeDir()
		if err != nil {
			log.Debug("markTourSeen", "error", err)
			return nil
		}
		configFile := filepath.Join(home, pkgconfig.CfgFileDir, pkgconfig.CfgFileName)
		data, err := os.ReadFile(configFile)
		if err != nil {
			log.Debug("markTourSeen", "error", err)
			return nil
		}
		updated, err := pkgconfig.UpsertScalarInConfig(data, "tour_seen", "true")
		if err != nil {
			log.Debug("markTourSeen", "error", err)
			return nil
		}
		// 0600: the config file holds the PagerDuty token.
		if err := os.WriteFile(configFile, updated, 0600); err != nil {
			log.Debug("markTourSeen", "error", err)
		}
		return nil
	}
}
