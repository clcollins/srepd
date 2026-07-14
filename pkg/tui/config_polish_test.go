package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWelcomeDescription_NewUser(t *testing.T) {
	desc := welcomeDescription(true, "")

	assert.Contains(t, desc, "PagerDuty")
	assert.Contains(t, desc, "User Settings")
	assert.Contains(t, desc, "ctrl+c")
	assert.NotContains(t, desc, "You're here because")
}

func TestWelcomeDescription_WithReason(t *testing.T) {
	desc := welcomeDescription(false, "the configured PagerDuty API token is a placeholder value")

	assert.Contains(t, desc, "You're here because")
	assert.Contains(t, desc, "placeholder value")
}

func TestWelcomeDescription_ExistingConfig(t *testing.T) {
	desc := welcomeDescription(false, "")
	assert.Contains(t, desc, "existing")
}

func TestStepTitle(t *testing.T) {
	assert.Equal(t, "Select your PagerDuty teams · 2/6", stepTitle("Select your PagerDuty teams", 2))
	assert.Equal(t, "PagerDuty API token · 1/6", stepTitle("PagerDuty API token", 1))
}
