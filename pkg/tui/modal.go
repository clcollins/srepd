package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type ModalVariant int

const (
	ModalWarning ModalVariant = iota
	ModalError
	ModalInfo
)

type Modal struct {
	Title   string
	Body    string
	Hint    string
	Variant ModalVariant
}

func renderModal(width, height int, styles Styles, theme Theme, m Modal) string {
	if width < 1 || height < 1 {
		return ""
	}

	var borderColor, textColor lipgloss.AdaptiveColor
	switch m.Variant {
	case ModalWarning:
		borderColor = theme.Warning
		textColor = theme.Highlight
	case ModalError:
		borderColor = theme.Error
		textColor = theme.Highlight
	default:
		borderColor = theme.Border
		textColor = theme.Text
	}

	maxBoxWidth := width - 4
	if maxBoxWidth < 10 {
		maxBoxWidth = 10
	}

	contentWidth := maxBoxWidth - 6

	var sections []string

	if m.Title != "" {
		titleStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor).
			Width(contentWidth).
			Align(lipgloss.Center)
		sections = append(sections, titleStyle.Render(m.Title))
	}

	if m.Body != "" {
		bodyStyle := lipgloss.NewStyle().
			Foreground(textColor).
			Width(contentWidth)
		sections = append(sections, bodyStyle.Render(m.Body))
	}

	if m.Hint != "" {
		hintStyle := lipgloss.NewStyle().
			Foreground(theme.Muted).
			Width(contentWidth).
			Align(lipgloss.Center)
		sections = append(sections, hintStyle.Render(m.Hint))
	}

	content := strings.Join(sections, "\n\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Foreground(textColor).
		Padding(1, 2).
		Width(maxBoxWidth).
		MaxHeight(height - 2).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
