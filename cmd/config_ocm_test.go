package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// OB-6 regression guard: config-wizard mode must never trigger OCM auth. A
// brand-new user setting up srepd should not see "OCM tokens expired —
// opening browser" or OCM warnings before they have even saved a token.
// launchTUIWithConfig lives in config.go; the OCM setup must stay out of it.
func TestConfigMode_NoOCMAuth(t *testing.T) {
	src, err := os.ReadFile("config.go")
	assert.NoError(t, err)

	content := string(src)
	for _, forbidden := range []string{
		"ocm.CheckTokens",
		"ocm.AuthenticateAsync",
		"ocm.NewClientFromConfig",
		"ocm.ApplyAuthToken",
		"OCMClientReadyMsg",
	} {
		assert.NotContains(t, content, forbidden,
			"config mode must not perform OCM auth (%s found in cmd/config.go)", forbidden)
	}
}

// The OCM startup logic must live in exactly one place: the setupOCM helper
// used by the normal launch path.
func TestSetupOCM_SingleCallSite(t *testing.T) {
	src, err := os.ReadFile("root.go")
	assert.NoError(t, err)

	content := string(src)
	assert.Contains(t, content, "func setupOCM(", "setupOCM helper must exist in root.go")
	assert.Equal(t, 1, strings.Count(content, "ocm.CheckTokens"),
		"ocm.CheckTokens must be called only inside setupOCM")
}
