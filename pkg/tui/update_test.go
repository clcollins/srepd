package tui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/clcollins/srepd/pkg/launcher"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
)

func TestVersionDisplay_Default(t *testing.T) {
	t.Run("shows version and git SHA in bottom status", func(t *testing.T) {
		result := versionString()
		assert.Contains(t, result, Version)
		assert.Contains(t, result, GitSHA)
	})
}

func TestVersionDisplay_WithUpdate(t *testing.T) {
	t.Run("shows update notification when update available", func(t *testing.T) {
		result := updateString("v1.0.0", "v1.1.0")
		assert.Contains(t, result, "v1.0.0")
		assert.Contains(t, result, "v1.1.0")
		assert.Contains(t, result, "Update")
	})
}

func TestCheckForUpdate_DevMode(t *testing.T) {
	t.Run("dev mode returns fake update without API call", func(t *testing.T) {
		cmd := checkForUpdate(true, "")
		msg := cmd()

		updateMsg, ok := msg.(updateAvailableMsg)
		assert.True(t, ok, "should return updateAvailableMsg")
		assert.Equal(t, "v99.0.0", updateMsg.latest)
		assert.NotEmpty(t, updateMsg.current)
	})
}

func TestCheckForUpdate_WithMockServer(t *testing.T) {
	t.Run("detects newer version from GitHub API", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintln(w, `{"tag_name": "v2.0.0", "html_url": "https://github.com/clcollins/srepd/releases/tag/v2.0.0"}`)
		}))
		defer server.Close()

		cmd := checkForUpdate(false, server.URL)
		msg := cmd()

		updateMsg, ok := msg.(updateAvailableMsg)
		assert.True(t, ok, "should return updateAvailableMsg")
		assert.Equal(t, "v2.0.0", updateMsg.latest)
		assert.Equal(t, "https://github.com/clcollins/srepd/releases/tag/v2.0.0", updateMsg.releaseURL)
	})

	t.Run("returns nil when already on latest", func(t *testing.T) {
		origVersion := Version
		Version = "v1.5.0"
		defer func() { Version = origVersion }()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintln(w, `{"tag_name": "v1.5.0", "html_url": "https://github.com/clcollins/srepd/releases/tag/v1.5.0"}`)
		}))
		defer server.Close()

		cmd := checkForUpdate(false, server.URL)
		msg := cmd()

		assert.Nil(t, msg, "should return nil when no update available")
	})

	t.Run("returns nil on API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		cmd := checkForUpdate(false, server.URL)
		msg := cmd()

		assert.Nil(t, msg, "should return nil on API error")
	})

	t.Run("returns nil on invalid JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprintln(w, "not json")
		}))
		defer server.Close()

		cmd := checkForUpdate(false, server.URL)
		msg := cmd()

		assert.Nil(t, msg, "should return nil on invalid JSON")
	})
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		latest   string
		expected bool
	}{
		{
			name:     "newer patch version",
			current:  "v1.0.0",
			latest:   "v1.0.1",
			expected: true,
		},
		{
			name:     "newer minor version",
			current:  "v1.0.0",
			latest:   "v1.1.0",
			expected: true,
		},
		{
			name:     "newer major version",
			current:  "v1.0.0",
			latest:   "v2.0.0",
			expected: true,
		},
		{
			name:     "same version",
			current:  "v1.0.0",
			latest:   "v1.0.0",
			expected: false,
		},
		{
			name:     "older version",
			current:  "v1.1.0",
			latest:   "v1.0.0",
			expected: false,
		},
		{
			name:     "dev is always outdated",
			current:  "dev",
			latest:   "v1.0.0",
			expected: true,
		},
		{
			name:     "handles missing v prefix",
			current:  "1.0.0",
			latest:   "v1.1.0",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNewerVersion(tt.current, tt.latest)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUpdateAvailableMsg_HandledInModel(t *testing.T) {
	t.Run("updateAvailableMsg sets model fields", func(t *testing.T) {
		m := createTestModel()

		msg := updateAvailableMsg{
			current:    "v1.0.0",
			latest:     "v1.1.0",
			releaseURL: "https://github.com/clcollins/srepd/releases/tag/v1.1.0",
		}

		result, _ := m.Update(msg)
		updated := result.(model)

		assert.True(t, updated.updateAvailable)
		assert.Equal(t, "v1.1.0", updated.updateVersion)
		assert.Equal(t, "https://github.com/clcollins/srepd/releases/tag/v1.1.0", updated.updateReleaseURL)
	})
}

func TestDevModeFieldSet(t *testing.T) {
	t.Run("InitialModelWithConfig sets devMode to true", func(t *testing.T) {
		config := &pd.Config{
			Client: &pd.MockPagerDutyClient{},
		}

		teaModel, _ := InitialModelWithConfig(config, []string{"vi"}, launcher.ClusterLauncher{}, false)
		m := teaModel.(model)

		assert.True(t, m.devMode, "devMode should be true for InitialModelWithConfig")
	})
}
