package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	pkgconfig "github.com/clcollins/srepd/pkg/config"
	"github.com/spf13/viper"
)

type tourStep struct {
	Title string
	Body  string
}

func tourSteps() []tourStep {
	return []tourStep{
		{
			Title: "The incident table",
			Body: "The initial screen is your incident list: ID, cluster/service,\n" +
				"and title. The view auto-refreshes. Press t to toggle between\n" +
				"your incidents and the whole team's.",
		},
		{
			Title: "Incident markers",
			Body: "'●' marks incidents assigned to you.\n" +
				"'A' marks incidents you have acknowledged, and 'a' marks\n" +
				"incidents acknowledged by someone else.",
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
				"ctrl+e — re-escalate to the next SRE at the end of your shift.\n" +
				"n — add a note. l — log into the incident's cluster.",
		},
		{
			Title: "Command mode",
			Body: "Press : to enter command mode. :agent <question> asks the AI\n" +
				"agent for read-only incident investigation; :watcher <question>\n" +
				"queries the ambient LLM watcher.",
		},
		{
			Title: "The watcher",
			Body: "Press w to toggle the AI watcher pane: ambient analysis of\n" +
				"incoming incidents and pattern detection across your queue,\n" +
				"streamed below the table.",
		},
		{
			Title: "Flags",
			Body: "Mark incidents matching patterns — matches get a marker in\n" +
				"the table so you can spot them at a glance.\n\n" +
				"  :flag cluster abc-12345    flag by cluster ID\n" +
				"  :flag org acme.*           flag by customer name (regex)\n" +
				"  :flags                     list active flag conditions\n" +
				"  :unflag 1                  remove flag condition #1",
		},
		{
			Title: "Saving flags",
			Body: "Flags are in-memory by default. To keep them across sessions:\n\n" +
				"  :flags save       save to ~/.config/srepd/flags.json\n" +
				"  :flags load       restore saved flags\n\n" +
				"You can also specify a custom path: :flags save /tmp/my-flags.json",
		},
		{
			Title: "Chord commands",
			Body: "Less-common actions live behind a chord prefix (default ctrl+x).\n" +
				"Press ctrl+x then ? to see every chord — for example ctrl+x s\n" +
				"for bulk silence.",
		},
		{
			Title: "That's the tour!",
			Body: "Press h any time for the full help view, ctrl+h for\n" +
				"keybindings and docs, and re-run this tour with :tour.\n" +
				"Happy on-calling!",
		},
	}
}

func isTourCommand(input string) bool {
	return strings.TrimSpace(input) == ":tour"
}

func (m model) startTour() (model, tea.Cmd) {
	m.tourMode = true
	m.tourStep = 0
	m.table.Blur()
	return m, markTourSeenCmd()
}

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

	panelWidth := 70
	if windowSize.Width-4 < panelWidth {
		panelWidth = windowSize.Width - 4
	}

	panel := m.styles.FormContainer.Width(panelWidth).Render(content)

	// Center horizontally
	mainH := m.styles.Main.GetHorizontalMargins() + m.styles.Main.GetHorizontalPadding()
	availW := windowSize.Width - mainH
	panelW := lipgloss.Width(panel)
	padLeft := (availW - panelW) / 2
	if padLeft < 0 {
		padLeft = 0
	}

	// Center vertically
	panelH := lipgloss.Height(panel)
	mainV := m.styles.Main.GetVerticalMargins() + m.styles.Main.GetVerticalPadding()
	availH := windowSize.Height - mainV - 3 // header + status
	padTop := (availH - panelH) / 2
	if padTop < 0 {
		padTop = 0
	}

	var sb strings.Builder
	for i := 0; i < padTop; i++ {
		sb.WriteString("\n")
	}
	for _, line := range strings.Split(panel, "\n") {
		sb.WriteString(strings.Repeat(" ", padLeft))
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}

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
		if err := os.WriteFile(configFile, updated, 0600); err != nil {
			log.Debug("markTourSeen", "error", err)
		}
		return nil
	}
}
