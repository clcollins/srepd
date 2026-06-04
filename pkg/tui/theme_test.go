package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestDefaultTheme(t *testing.T) {
	t.Run("returns theme with expected default colors", func(t *testing.T) {
		theme := DefaultTheme()

		assert.Equal(t, "#778da9", theme.Text.Dark)
		assert.Equal(t, "#415a77", theme.Border.Dark)
		assert.Equal(t, "#ffffff", theme.Highlight.Dark)
		assert.Equal(t, "#415a77", theme.Selected.Dark)
		assert.Equal(t, "#a4133c", theme.Warning.Dark)
		assert.Equal(t, "#0d1b2a", theme.Error.Dark)
		assert.Equal(t, "#5C5C5C", theme.Muted.Dark)
		assert.Equal(t, "#7D56F4", theme.Tab.Dark)
	})
}

func TestThemeFromConfig(t *testing.T) {
	t.Run("nil config returns default theme", func(t *testing.T) {
		theme := ThemeFromConfig(nil)
		def := DefaultTheme()

		assert.Equal(t, def.Text, theme.Text)
		assert.Equal(t, def.Border, theme.Border)
	})

	t.Run("empty config returns default theme", func(t *testing.T) {
		theme := ThemeFromConfig(map[string]string{})
		def := DefaultTheme()

		assert.Equal(t, def.Text, theme.Text)
		assert.Equal(t, def.Tab, theme.Tab)
	})

	t.Run("full config overrides all colors", func(t *testing.T) {
		cfg := map[string]string{
			"text":      "#aabbcc",
			"border":    "#112233",
			"highlight": "#ddeeff",
			"selected":  "#445566",
			"warning":   "#ff0000",
			"error":     "#330000",
			"muted":     "#999999",
			"tab":       "#cc00ff",
		}
		theme := ThemeFromConfig(cfg)

		assert.Equal(t, lipgloss.AdaptiveColor{Dark: "#aabbcc", Light: "#aabbcc"}, theme.Text)
		assert.Equal(t, lipgloss.AdaptiveColor{Dark: "#112233", Light: "#112233"}, theme.Border)
		assert.Equal(t, lipgloss.AdaptiveColor{Dark: "#ddeeff", Light: "#ddeeff"}, theme.Highlight)
		assert.Equal(t, lipgloss.AdaptiveColor{Dark: "#445566", Light: "#445566"}, theme.Selected)
		assert.Equal(t, lipgloss.AdaptiveColor{Dark: "#ff0000", Light: "#ff0000"}, theme.Warning)
		assert.Equal(t, lipgloss.AdaptiveColor{Dark: "#330000", Light: "#330000"}, theme.Error)
		assert.Equal(t, lipgloss.AdaptiveColor{Dark: "#999999", Light: "#999999"}, theme.Muted)
		assert.Equal(t, lipgloss.AdaptiveColor{Dark: "#cc00ff", Light: "#cc00ff"}, theme.Tab)
	})

	t.Run("partial config overrides only specified keys", func(t *testing.T) {
		cfg := map[string]string{
			"text": "#aabbcc",
			"tab":  "#cc00ff",
		}
		theme := ThemeFromConfig(cfg)
		def := DefaultTheme()

		assert.Equal(t, lipgloss.AdaptiveColor{Dark: "#aabbcc", Light: "#aabbcc"}, theme.Text)
		assert.Equal(t, lipgloss.AdaptiveColor{Dark: "#cc00ff", Light: "#cc00ff"}, theme.Tab)
		assert.Equal(t, def.Border, theme.Border)
		assert.Equal(t, def.Warning, theme.Warning)
	})

	t.Run("invalid hex values fall back to defaults", func(t *testing.T) {
		cfg := map[string]string{
			"text":   "not-a-color",
			"border": "xyz",
			"tab":    "#cc00ff",
		}
		theme := ThemeFromConfig(cfg)
		def := DefaultTheme()

		assert.Equal(t, def.Text, theme.Text)
		assert.Equal(t, def.Border, theme.Border)
		assert.Equal(t, lipgloss.AdaptiveColor{Dark: "#cc00ff", Light: "#cc00ff"}, theme.Tab)
	})

	t.Run("unknown keys are ignored", func(t *testing.T) {
		cfg := map[string]string{
			"text":          "#aabbcc",
			"nonexistent":   "#ffffff",
			"also_not_real": "#000000",
		}
		theme := ThemeFromConfig(cfg)

		assert.Equal(t, lipgloss.AdaptiveColor{Dark: "#aabbcc", Light: "#aabbcc"}, theme.Text)
	})
}

func TestBuildStyles(t *testing.T) {
	t.Run("returns styles derived from theme", func(t *testing.T) {
		theme := DefaultTheme()
		styles := BuildStyles(theme)

		assert.NotNil(t, styles.Main)
		assert.NotNil(t, styles.Table)
		assert.NotNil(t, styles.TableContainer)
		assert.NotNil(t, styles.Error)
		assert.NotNil(t, styles.Warning)
		assert.NotNil(t, styles.Muted)
		assert.NotNil(t, styles.Padded)
		assert.NotNil(t, styles.ActiveTab)
		assert.NotNil(t, styles.InactiveTab)
		assert.NotNil(t, styles.TabWindow)
	})
}

func TestIsValidHexColor(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"valid 6-digit hex", "#aabbcc", true},
		{"valid 3-digit hex", "#abc", true},
		{"valid uppercase", "#AABBCC", true},
		{"valid mixed case", "#AaBbCc", true},
		{"missing hash", "aabbcc", false},
		{"empty string", "", false},
		{"too short", "#ab", false},
		{"too long", "#aabbccdd", false},
		{"invalid chars", "#gghhii", false},
		{"word", "red", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidHexColor(tt.input))
		})
	}
}
