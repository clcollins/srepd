package tui

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

// The huh forms (config wizard, team picker, bulk silence) must render in
// SREPD's palette, not huh's stock Charm theme. SrepdHuhTheme derives every
// visible style — focused AND blurred — from the app Theme, so `colors`
// config overrides restyle the forms along with the rest of the UI.

func TestSrepdHuhTheme_FocusedUsesAppPalette(t *testing.T) {
	theme := DefaultTheme()
	ht := SrepdHuhTheme(theme)

	assert.Equal(t, theme.Highlight, ht.Focused.Title.GetForeground(),
		"focused titles must use the app highlight color")
	assert.Equal(t, theme.Muted, ht.Focused.Description.GetForeground(),
		"focused descriptions must use the app muted color")
	assert.Equal(t, theme.Border, ht.Focused.Base.GetBorderLeftForeground(),
		"focused field border must use the app border color")
	assert.Equal(t, theme.Highlight, ht.Focused.SelectedOption.GetForeground())
	assert.Equal(t, theme.Text, ht.Focused.UnselectedOption.GetForeground())
	assert.Equal(t, theme.Warning, ht.Focused.ErrorMessage.GetForeground(),
		"errors must use the app warning color")
}

func TestSrepdHuhTheme_BlurredMatchesAppPalette(t *testing.T) {
	theme := DefaultTheme()
	ht := SrepdHuhTheme(theme)

	// Blurred (inactive) fields previously kept huh's stock purple. They must
	// render in the app palette, dimmed via the muted color.
	assert.Equal(t, theme.Muted, ht.Blurred.Title.GetForeground(),
		"blurred titles must use the app muted color")
	assert.Equal(t, theme.Muted, ht.Blurred.Description.GetForeground())
	assert.Equal(t, theme.Text, ht.Blurred.TextInput.Text.GetForeground())
}

func TestSrepdHuhTheme_FollowsColorOverrides(t *testing.T) {
	custom := ThemeFromConfig(map[string]string{
		"highlight": "#ff0000",
		"muted":     "#00ff00",
	})
	ht := SrepdHuhTheme(custom)

	assert.Equal(t, lipgloss.AdaptiveColor{Dark: "#ff0000", Light: "#ff0000"},
		ht.Focused.Title.GetForeground(),
		"user color overrides must flow into the huh theme")
	assert.Equal(t, lipgloss.AdaptiveColor{Dark: "#00ff00", Light: "#00ff00"},
		ht.Blurred.Title.GetForeground())
}

// All three huh form builders must use SrepdHuhTheme; none may fall back to
// huh.ThemeCharm. Source-scan guard, same pattern as plan 086 verification.
func TestNoStockCharmThemeInForms(t *testing.T) {
	for _, file := range []string{"tui.go", "views.go", "msgHandlers.go", "commands.go"} {
		content, err := os.ReadFile(file)
		assert.NoError(t, err)
		assert.NotContains(t, string(content), "huh.ThemeCharm",
			"%s must use SrepdHuhTheme, not the stock Charm theme", file)
	}
}

func TestBuildStyles_FormContainer(t *testing.T) {
	styles := BuildStyles(DefaultTheme())

	assert.Equal(t, lipgloss.RoundedBorder(), styles.FormContainer.GetBorderStyle(),
		"forms must sit in the same rounded-border pane language as other views")
	assert.Equal(t, DefaultTheme().Border, styles.FormContainer.GetBorderTopForeground())
	assert.Positive(t, styles.FormContainer.GetHorizontalPadding(),
		"form container must inset its content")
}

func TestLayout_FormWidthCapped(t *testing.T) {
	styles := BuildStyles(DefaultTheme())

	narrow := computeLayout(tea.WindowSizeMsg{Width: 80, Height: 40}, styles, "", false)
	assert.LessOrEqual(t, narrow.FormWidth, 80-styles.FormContainer.GetHorizontalFrameSize(),
		"form width must fit inside the container on narrow terminals")

	wide := computeLayout(tea.WindowSizeMsg{Width: 300, Height: 60}, styles, "", false)
	assert.LessOrEqual(t, wide.FormWidth, layoutMaxFormWidth,
		"form width must be capped on wide terminals — full-bleed forms are unreadable")
}
