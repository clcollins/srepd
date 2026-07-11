package tui

import (
	"strings"
	"testing"

	pkgconfig "github.com/clcollins/srepd/pkg/config"
	"github.com/stretchr/testify/assert"
)

// The wizard opens with a welcome step: what srepd is, where to get a token,
// how to bail out — and, when routed here by a broken config (OB-1), why.
func TestWelcomeDescription_NewUser(t *testing.T) {
	desc := welcomeDescription(true, "")

	assert.Contains(t, desc, "PagerDuty")
	assert.Contains(t, desc, "User Settings", "must include the token acquisition path")
	assert.Contains(t, desc, "ctrl+c", "must tell the user how to quit")
	assert.NotContains(t, desc, "You're here because")
}

func TestWelcomeDescription_WithReason(t *testing.T) {
	desc := welcomeDescription(false, "the configured PagerDuty API token is a placeholder value")

	assert.Contains(t, desc, "You're here because")
	assert.Contains(t, desc, "placeholder value")
}

func TestWelcomeDescription_ExistingConfig(t *testing.T) {
	desc := welcomeDescription(false, "")
	assert.Contains(t, desc, "existing", "reconfiguration runs should acknowledge the existing config")
}

// Step breadcrumbs: "Title · 2/6" on always-visible milestones; keep-confirm
// variants share their picker's number.
func TestStepTitle(t *testing.T) {
	assert.Equal(t, "Select your PagerDuty teams · 2/6", stepTitle("Select your PagerDuty teams", 2))
	assert.Equal(t, "PagerDuty API token · 1/6", stepTitle("PagerDuty API token", 1))
}

// The summary renders from structured rows so it can be styled; content and
// changed markers must survive the styling.
func TestRenderConfigSummary(t *testing.T) {
	rows := []pkgconfig.SummaryRow{
		{Label: "Token", Value: "u+ab********", Changed: true},
		{Label: "Teams", Value: "SRE Platform (PTEAM1)", Changed: false},
	}
	styles := BuildStyles(DefaultTheme())

	out := renderConfigSummary(rows, styles, "")

	assert.Contains(t, out, "Token")
	assert.Contains(t, out, "u+ab********")
	assert.Contains(t, out, "Teams")
	assert.Contains(t, out, "changed", "changed rows must be marked")
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	assert.Len(t, lines, 2)
}

func TestRenderConfigSummary_PresetLine(t *testing.T) {
	out := renderConfigSummary(nil, BuildStyles(DefaultTheme()), "https://team/preset.yaml")
	assert.Contains(t, out, "Preset applied")
	assert.Contains(t, out, "https://team/preset.yaml")
}
