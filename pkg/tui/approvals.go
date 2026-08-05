package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	pkgrand "github.com/clcollins/srepd/pkg/rand"
)

// AskKind categorizes the type of user approval request.
type AskKind int

const (
	AskDraftNote AskKind = iota
	AskSuggestedCommand
	AskEscalationSuggestion
)

// Ask represents a pending approval item from the AI watcher.
type Ask struct {
	ID        string
	Kind      AskKind
	Title     string
	Body      string
	Action    func() tea.Cmd
	CreatedAt time.Time
}

// approvalsStrip manages the list of pending asks.
type approvalsStrip struct {
	asks     []Ask
	selected int
}

func newApprovalsStrip() *approvalsStrip {
	return &approvalsStrip{}
}

func (a *approvalsStrip) Add(ask Ask) {
	if ask.ID == "" {
		ask.ID = pkgrand.ID("ASK-")
	}
	if ask.CreatedAt.IsZero() {
		ask.CreatedAt = time.Now()
	}
	a.asks = append(a.asks, ask)
}

func (a *approvalsStrip) Count() int {
	return len(a.asks)
}

func (a *approvalsStrip) Accept(idx int) tea.Cmd {
	if idx < 0 || idx >= len(a.asks) {
		return nil
	}
	ask := a.asks[idx]
	a.remove(idx)
	if ask.Action != nil {
		return ask.Action()
	}
	return nil
}

func (a *approvalsStrip) Dismiss(idx int) {
	if idx >= 0 && idx < len(a.asks) {
		a.remove(idx)
	}
}

func (a *approvalsStrip) remove(idx int) {
	a.asks = append(a.asks[:idx], a.asks[idx+1:]...)
	if a.selected >= len(a.asks) && a.selected > 0 {
		a.selected = len(a.asks) - 1
	}
}

func (a *approvalsStrip) MoveUp() {
	if a.selected > 0 {
		a.selected--
	}
}

func (a *approvalsStrip) MoveDown() {
	if a.selected < len(a.asks)-1 {
		a.selected++
	}
}

func (a *approvalsStrip) Selected() int {
	return a.selected
}

func (a *approvalsStrip) Render(width int) string {
	if len(a.asks) == 0 {
		return ""
	}
	noun := "asks"
	if len(a.asks) == 1 {
		noun = "ask"
	}
	text := fmt.Sprintf(" ⚑ %d %s — press A ", len(a.asks), noun)
	style := lipgloss.NewStyle().
		Background(lipgloss.Color("3")).
		Foreground(lipgloss.Color("0")).
		Bold(true).
		Width(width)
	return style.Render(text)
}

func (a *approvalsStrip) RenderExpanded(width int) string {
	if len(a.asks) == 0 {
		return ""
	}

	var lines []string
	header := lipgloss.NewStyle().
		Bold(true).
		Underline(true).
		Width(width).
		Render(" Pending Approvals ")
	lines = append(lines, header)

	for i, ask := range a.asks {
		prefix := "  "
		if i == a.selected {
			prefix = "> "
		}
		kindLabel := askKindLabel(ask.Kind)
		line := fmt.Sprintf("%s[%s] %s", prefix, kindLabel, ask.Title)
		lines = append(lines, line)
	}

	footer := " [Enter] Accept  [d] Dismiss  [Esc] Close "
	lines = append(lines, footer)

	result := ""
	for _, l := range lines {
		result += l + "\n"
	}
	return result
}

// inferAskKind determines the AskKind from a verdict's action text.
func inferAskKind(action string) AskKind {
	lower := strings.ToLower(action)
	switch {
	case strings.Contains(lower, "escalat") || strings.Contains(lower, "re-escalat"):
		return AskEscalationSuggestion
	case strings.Contains(lower, "command") || strings.Contains(lower, "oc ") || strings.Contains(lower, "kubectl"):
		return AskSuggestedCommand
	default:
		return AskDraftNote
	}
}

func askKindLabel(k AskKind) string {
	switch k {
	case AskDraftNote:
		return "Note"
	case AskSuggestedCommand:
		return "Command"
	case AskEscalationSuggestion:
		return "Escalation"
	default:
		return "Unknown"
	}
}
