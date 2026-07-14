package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestRenderModal_ContainsBodyAndHint(t *testing.T) {
	theme := DefaultTheme()
	styles := BuildStyles(theme)

	tests := []struct {
		name string
		m    Modal
	}{
		{
			name: "warning modal contains body and hint",
			m: Modal{
				Title:   "Confirm",
				Body:    "Silence P1234567? [y/n]",
				Hint:    "y: confirm  n/esc: cancel",
				Variant: ModalWarning,
			},
		},
		{
			name: "error modal contains body and hint",
			m: Modal{
				Title:   "Error",
				Body:    "something went wrong",
				Hint:    "press esc to dismiss",
				Variant: ModalError,
			},
		},
		{
			name: "info modal contains body and hint",
			m: Modal{
				Title:   "Notice",
				Body:    "operation complete",
				Hint:    "press any key",
				Variant: ModalInfo,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderModal(120, 40, styles, theme, tt.m)
			assert.Contains(t, result, tt.m.Body, "modal should contain the body text")
			assert.Contains(t, result, tt.m.Hint, "modal should contain the hint text")
		})
	}
}

func TestRenderModal_FitsWithinDimensions(t *testing.T) {
	theme := DefaultTheme()
	styles := BuildStyles(theme)

	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"standard terminal", 120, 40},
		{"narrow terminal", 40, 20},
		{"very narrow terminal", 30, 15},
		{"short terminal", 80, 10},
	}

	m := Modal{
		Title:   "Confirm",
		Body:    "Silence P1234567? [y/n]",
		Hint:    "y: confirm  n/esc: cancel",
		Variant: ModalWarning,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderModal(tt.width, tt.height, styles, theme, m)
			lines := strings.Split(result, "\n")

			assert.LessOrEqual(t, len(lines), tt.height,
				"modal should not exceed terminal height")

			for i, line := range lines {
				lineWidth := lipgloss.Width(line)
				assert.LessOrEqual(t, lineWidth, tt.width,
					"line %d should not exceed terminal width (got %d, max %d)", i, lineWidth, tt.width)
			}
		})
	}
}

func TestRenderModal_EmptyBody(t *testing.T) {
	theme := DefaultTheme()
	styles := BuildStyles(theme)

	m := Modal{
		Title:   "Notice",
		Body:    "",
		Hint:    "press esc",
		Variant: ModalInfo,
	}

	assert.NotPanics(t, func() {
		result := renderModal(80, 24, styles, theme, m)
		assert.Contains(t, result, "press esc", "hint should still render with empty body")
	})
}

func TestRenderModal_LongBodyWraps(t *testing.T) {
	theme := DefaultTheme()
	styles := BuildStyles(theme)

	longBody := "This is a very long body text that should be wrapped inside the modal box without exceeding the terminal width"
	m := Modal{
		Title:   "Confirm",
		Body:    longBody,
		Hint:    "y/n",
		Variant: ModalWarning,
	}

	result := renderModal(50, 20, styles, theme, m)
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		assert.LessOrEqual(t, lineWidth, 50,
			"line %d should not exceed terminal width (got %d)", i, lineWidth)
	}
	assert.Contains(t, result, "y/n", "hint should be visible even with long body")
}
