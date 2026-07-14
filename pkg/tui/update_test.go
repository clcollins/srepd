package tui

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/launcher"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
)

// captureLog redirects the global logger to a buffer for the duration of the
// test and restores the previous output and level afterwards. Tests using it
// must not run in parallel — the logger is a shared global.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	origLevel := log.GetLevel()
	log.SetOutput(&buf)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		log.SetLevel(origLevel)
	})
	return &buf
}

func TestUpdatedIncidentListCacheInvalidation(t *testing.T) {
	newListModel := func() model {
		m := createTestModel()
		m.config = &pd.Config{
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U123"}},
		}
		return m
	}

	t.Run("removes cache entries for incidents no longer in the list", func(t *testing.T) {
		m := newListModel()
		m.incidentCache["GONE"] = &cachedIncidentData{
			incident: &pagerduty.Incident{APIObject: pagerduty.APIObject{ID: "GONE"}},
		}

		result, _ := m.Update(updatedIncidentListMsg{
			incidents: []pagerduty.Incident{{APIObject: pagerduty.APIObject{ID: "Q123"}}},
		})
		m = result.(model)

		assert.NotContains(t, m.incidentCache, "GONE")
	})

	t.Run("invalidates cache entries whose incident changed", func(t *testing.T) {
		m := newListModel()
		m.incidentCache["Q123"] = &cachedIncidentData{
			incident: &pagerduty.Incident{
				APIObject:          pagerduty.APIObject{ID: "Q123"},
				LastStatusChangeAt: "2024-01-01T00:00:00Z",
			},
		}

		result, _ := m.Update(updatedIncidentListMsg{
			incidents: []pagerduty.Incident{{
				APIObject:          pagerduty.APIObject{ID: "Q123"},
				LastStatusChangeAt: "2024-06-01T00:00:00Z",
			}},
		})
		m = result.(model)

		assert.NotContains(t, m.incidentCache, "Q123",
			"a changed LastStatusChangeAt must invalidate the cached entry")
	})

	t.Run("keeps cache entries for unchanged incidents", func(t *testing.T) {
		m := newListModel()
		m.incidentCache["Q123"] = &cachedIncidentData{
			incident: &pagerduty.Incident{
				APIObject:          pagerduty.APIObject{ID: "Q123"},
				LastStatusChangeAt: "2024-01-01T00:00:00Z",
			},
		}

		result, _ := m.Update(updatedIncidentListMsg{
			incidents: []pagerduty.Incident{{
				APIObject:          pagerduty.APIObject{ID: "Q123"},
				LastStatusChangeAt: "2024-01-01T00:00:00Z",
			}},
		})
		m = result.(model)

		assert.Contains(t, m.incidentCache, "Q123")
	})

	t.Run("keeps partial cache entries without incident details", func(t *testing.T) {
		m := newListModel()
		// An alerts/notes-only entry has no incident and cannot be compared
		m.incidentCache["Q123"] = &cachedIncidentData{notesLoaded: true}

		result, _ := m.Update(updatedIncidentListMsg{
			incidents: []pagerduty.Incident{{APIObject: pagerduty.APIObject{ID: "Q123"}}},
		})
		m = result.(model)

		assert.Contains(t, m.incidentCache, "Q123")
	})

	t.Run("prunes enrichment dispatch records for departed incidents", func(t *testing.T) {
		m := newListModel()
		m.enrichDispatchedAt = map[string]time.Time{
			"GONE": time.Now(),
			"Q123": time.Now(),
		}

		result, _ := m.Update(updatedIncidentListMsg{
			incidents: []pagerduty.Incident{{APIObject: pagerduty.APIObject{ID: "Q123"}}},
		})
		m = result.(model)

		assert.NotContains(t, m.enrichDispatchedAt, "GONE")
		assert.Contains(t, m.enrichDispatchedAt, "Q123")
	})
}

func TestGotIncidentNotesMsg_SetsLastFetched(t *testing.T) {
	t.Run("new cache entry records fetch time", func(t *testing.T) {
		m := createTestModel()

		result, _ := m.Update(gotIncidentNotesMsg{incidentID: "Q123", notes: []pagerduty.IncidentNote{}})
		m = result.(model)

		assert.False(t, m.incidentCache["Q123"].lastFetched.IsZero(),
			"a notes-first cache entry must record when its data was fetched")
	})

	t.Run("existing cache entry fetch time advances", func(t *testing.T) {
		m := createTestModel()
		stale := time.Now().Add(-time.Hour)
		m.incidentCache["Q123"] = &cachedIncidentData{lastFetched: stale}

		result, _ := m.Update(gotIncidentNotesMsg{incidentID: "Q123", notes: []pagerduty.IncidentNote{}})
		m = result.(model)

		assert.True(t, m.incidentCache["Q123"].lastFetched.After(stale),
			"refreshing notes must advance the entry's fetch time")
	})
}

func TestGotIncidentAlertsMsg_SetsLastFetched(t *testing.T) {
	t.Run("new cache entry records fetch time", func(t *testing.T) {
		m := createTestModel()
		m.config = &pd.Config{CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}}}

		result, _ := m.Update(gotIncidentAlertsMsg{incidentID: "Q123", alerts: []pagerduty.IncidentAlert{}})
		m = result.(model)

		assert.False(t, m.incidentCache["Q123"].lastFetched.IsZero(),
			"an alerts-first cache entry must record when its data was fetched")
	})

	t.Run("existing cache entry fetch time advances", func(t *testing.T) {
		m := createTestModel()
		m.config = &pd.Config{CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}}}
		stale := time.Now().Add(-time.Hour)
		m.incidentCache["Q123"] = &cachedIncidentData{lastFetched: stale}

		result, _ := m.Update(gotIncidentAlertsMsg{incidentID: "Q123", alerts: []pagerduty.IncidentAlert{}})
		m = result.(model)

		assert.True(t, m.incidentCache["Q123"].lastFetched.After(stale),
			"refreshing alerts must advance the entry's fetch time")
	})
}

func TestUpdateDebugPreamble_LogsAtDebugLevel(t *testing.T) {
	buf := captureLog(t)
	log.SetLevel(log.DebugLevel)

	m := createTestModel()
	m.Update(updateIncidentListMsg("test"))

	assert.Contains(t, buf.String(), "Update", "message flow should be logged at debug level")
}

func TestUpdateDebugPreamble_SilentAtInfoLevel(t *testing.T) {
	buf := captureLog(t)
	log.SetLevel(log.InfoLevel)

	m := createTestModel()
	m.Update(updateIncidentListMsg("test"))

	assert.NotContains(t, buf.String(), "Update", "no per-message debug output above debug level")
}

func TestUpdateDebugPreamble_SkipsFrequentMessages(t *testing.T) {
	buf := captureLog(t)
	log.SetLevel(log.DebugLevel)

	m := createTestModel()
	m.Update(TickMsg{})

	assert.NotContains(t, buf.String(), "Update", "high-frequency messages must not be logged")
}

func createTestTarGz(t *testing.T, filename string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name: filename,
		Mode: 0755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

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

func TestAssetName(t *testing.T) {
	tests := []struct {
		name         string
		goos         string
		goarch       string
		expectedName string
	}{
		{
			name:         "linux amd64",
			goos:         "linux",
			goarch:       "amd64",
			expectedName: "srepd_Linux_x86_64.tar.gz",
		},
		{
			name:         "linux arm64",
			goos:         "linux",
			goarch:       "arm64",
			expectedName: "srepd_Linux_arm64.tar.gz",
		},
		{
			name:         "darwin amd64",
			goos:         "darwin",
			goarch:       "amd64",
			expectedName: "srepd_Darwin_x86_64.tar.gz",
		},
		{
			name:         "darwin arm64",
			goos:         "darwin",
			goarch:       "arm64",
			expectedName: "srepd_Darwin_arm64.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := releaseAssetName(tt.goos, tt.goarch)
			assert.Equal(t, tt.expectedName, result)
		})
	}
}

func TestSelfUpdate_WithMockServer(t *testing.T) {
	t.Run("downloads and extracts binary from tar.gz", func(t *testing.T) {
		tarData := createTestTarGz(t, "srepd", []byte("#!/bin/sh\necho updated"))

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(tarData)
		}))
		defer server.Close()

		tmpBinary := filepath.Join(t.TempDir(), "srepd")
		err := os.WriteFile(tmpBinary, []byte("old binary"), 0755)
		assert.NoError(t, err)

		err = selfUpdate(server.URL, tmpBinary)
		assert.NoError(t, err)

		updated, err := os.ReadFile(tmpBinary)
		assert.NoError(t, err)
		assert.Equal(t, "#!/bin/sh\necho updated", string(updated))
	})

	t.Run("returns error on download failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		tmpBinary := filepath.Join(t.TempDir(), "srepd")
		err := os.WriteFile(tmpBinary, []byte("old"), 0755)
		assert.NoError(t, err)

		err = selfUpdate(server.URL+"/notfound", tmpBinary)
		assert.Error(t, err)

		content, _ := os.ReadFile(tmpBinary)
		assert.Equal(t, "old", string(content), "original binary should be untouched on error")
	})
}

func TestFindAssetURL(t *testing.T) {
	tests := []struct {
		name       string
		assets     []releaseAsset
		targetName string
		expectURL  string
		expectOK   bool
	}{
		{
			name: "finds matching asset",
			assets: []releaseAsset{
				{Name: "srepd_Linux_x86_64.tar.gz", DownloadURL: "https://example.com/linux"},
				{Name: "srepd_Darwin_arm64.tar.gz", DownloadURL: "https://example.com/darwin"},
			},
			targetName: "srepd_Linux_x86_64.tar.gz",
			expectURL:  "https://example.com/linux",
			expectOK:   true,
		},
		{
			name: "returns false when no match",
			assets: []releaseAsset{
				{Name: "srepd_Darwin_arm64.tar.gz", DownloadURL: "https://example.com/darwin"},
			},
			targetName: "srepd_Linux_x86_64.tar.gz",
			expectURL:  "",
			expectOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, ok := findAssetURL(tt.assets, tt.targetName)
			assert.Equal(t, tt.expectOK, ok)
			assert.Equal(t, tt.expectURL, url)
		})
	}
}

func TestDevModeFieldSet(t *testing.T) {
	t.Run("InitialModelWithConfig sets devMode to true", func(t *testing.T) {
		config := &pd.Config{
			Client: &pd.MockPagerDutyClient{},
		}

		teaModel, _ := InitialModelWithConfig(config, []string{"vi"}, launcher.ClusterLauncher{}, launcher.ClusterLauncher{}, false, nil, nil, "", nil)
		m := teaModel.(model)

		assert.True(t, m.devMode, "devMode should be true for InitialModelWithConfig")
	})
}

func TestEnterBulkSilenceMsg_BuildsForm(t *testing.T) {
	t.Run("enterBulkSilenceMsg creates multi-select form", func(t *testing.T) {
		m := createTestModel()
		m.config = &pd.Config{
			Client:      &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}},
		}
		m.incidentList = []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "Q123"}, Title: "Alert 1", Service: pagerduty.APIObject{Summary: "SvcA"}},
			{APIObject: pagerduty.APIObject{ID: "Q456"}, Title: "Alert 2", Service: pagerduty.APIObject{Summary: "SvcB"}},
		}

		result, cmd := m.Update(enterBulkSilenceMsg{})
		updated := result.(model)

		assert.True(t, updated.bulkSilenceMode, "should enter bulk silence mode")
		assert.NotNil(t, updated.bulkSilenceForm, "form should be created")
		assert.NotNil(t, cmd, "should return init command for form")
	})

	t.Run("enterBulkSilenceMsg with no incidents shows status", func(t *testing.T) {
		m := createTestModel()
		m.config = &pd.Config{
			Client:      &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}},
		}
		m.incidentList = nil

		result, _ := m.Update(enterBulkSilenceMsg{})
		updated := result.(model)

		assert.False(t, updated.bulkSilenceMode, "should not enter bulk silence mode")
		assert.Contains(t, updated.status, "no incidents")
	})
}

func TestBulkSilenceConfirmedMsg_PerServicePolicy(t *testing.T) {
	t.Run("bulkSilenceConfirmedMsg uses per-service policy lookup", func(t *testing.T) {
		m := createTestModel()
		m.config = &pd.Config{
			Client:      &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}},
			EscalationPolicies: map[string]*pagerduty.EscalationPolicy{
				"SILENT_DEFAULT": {APIObject: pagerduty.APIObject{ID: "POL_DEFAULT"}, Name: "Default Silent"},
				"SVC_CUSTOM":     {APIObject: pagerduty.APIObject{ID: "POL_CUSTOM"}, Name: "Custom Silent"},
			},
		}

		incidents := []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "Q123"}, Service: pagerduty.APIObject{ID: "SVC_CUSTOM", Summary: "Custom Svc"}},
			{APIObject: pagerduty.APIObject{ID: "Q456"}, Service: pagerduty.APIObject{ID: "SVC_OTHER", Summary: "Other Svc"}},
		}

		result, cmd := m.Update(bulkSilenceConfirmedMsg{incidents: incidents})
		updated := result.(model)

		assert.NotNil(t, cmd, "should return batch command")
		assert.Contains(t, updated.status, "Silenced")
	})

	t.Run("bulkSilenceConfirmedMsg with empty incidents shows status", func(t *testing.T) {
		m := createTestModel()
		m.config = &pd.Config{
			Client:      &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}},
		}

		result, _ := m.Update(bulkSilenceConfirmedMsg{incidents: nil})
		updated := result.(model)

		assert.Contains(t, updated.status, "no incidents")
	})
}

func TestSilenceIncidentsMsg_PerServicePolicy(t *testing.T) {
	t.Run("silenceIncidentsMsg uses per-service policy lookup", func(t *testing.T) {
		m := createTestModel()
		m.config = &pd.Config{
			Client:      &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}},
			EscalationPolicies: map[string]*pagerduty.EscalationPolicy{
				"SILENT_DEFAULT": {APIObject: pagerduty.APIObject{ID: "POL_DEFAULT"}, Name: "Default Silent"},
				"SVC_CUSTOM":     {APIObject: pagerduty.APIObject{ID: "POL_CUSTOM"}, Name: "Custom Silent"},
			},
		}

		incidents := []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "Q123"}, Service: pagerduty.APIObject{ID: "SVC_CUSTOM"}},
		}

		result, cmd := m.Update(silenceIncidentsMsg{incidents: incidents})
		updated := result.(model)

		assert.NotNil(t, cmd, "should return batch command")
		assert.Contains(t, updated.status, "Silenced")
	})

	t.Run("silenceIncidentsMsg with nil incidents shows error", func(t *testing.T) {
		m := createTestModel()
		m.config = &pd.Config{
			Client:      &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}},
		}

		result, _ := m.Update(silenceIncidentsMsg{incidents: nil})
		updated := result.(model)

		assert.Contains(t, updated.status, "failed silencing")
	})

	t.Run("silenceIncidentsMsg does not silence the selected incident twice", func(t *testing.T) {
		m := createTestModel()
		m.config = &pd.Config{
			Client:      &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}},
			EscalationPolicies: map[string]*pagerduty.EscalationPolicy{
				"SILENT_DEFAULT": {APIObject: pagerduty.APIObject{ID: "POL_DEFAULT"}, Name: "Default Silent"},
			},
		}

		incidents := []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "Q123"}, Service: pagerduty.APIObject{ID: "SVC1"}},
		}
		m.selectedIncident = &incidents[0]

		result, _ := m.Update(silenceIncidentsMsg{incidents: incidents})
		updated := result.(model)

		assert.Equal(t, 1, strings.Count(updated.status, "Q123"),
			"an incident already in the message must be silenced only once")
	})

	t.Run("silenceIncidentsMsg still includes selected incident when absent", func(t *testing.T) {
		m := createTestModel()
		m.config = &pd.Config{
			Client:      &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}},
			EscalationPolicies: map[string]*pagerduty.EscalationPolicy{
				"SILENT_DEFAULT": {APIObject: pagerduty.APIObject{ID: "POL_DEFAULT"}, Name: "Default Silent"},
			},
		}

		incidents := []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "Q123"}, Service: pagerduty.APIObject{ID: "SVC1"}},
		}
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "Q456"},
			Service:   pagerduty.APIObject{ID: "SVC1"},
		}

		result, _ := m.Update(silenceIncidentsMsg{incidents: incidents})
		updated := result.(model)

		assert.Contains(t, updated.status, "Q123")
		assert.Contains(t, updated.status, "Q456",
			"a selected incident not in the message should still be silenced")
	})
}
