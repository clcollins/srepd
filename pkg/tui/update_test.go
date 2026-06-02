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
	"testing"

	"github.com/clcollins/srepd/pkg/launcher"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
)

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

		teaModel, _ := InitialModelWithConfig(config, []string{"vi"}, launcher.ClusterLauncher{}, false)
		m := teaModel.(model)

		assert.True(t, m.devMode, "devMode should be true for InitialModelWithConfig")
	})
}
