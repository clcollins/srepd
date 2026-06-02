package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
)

const (
	githubReleasesURL  = "https://api.github.com/repos/clcollins/srepd/releases/latest"
	updateCheckTimeout = 10 * time.Second
)

type updateAvailableMsg struct {
	current    string
	latest     string
	releaseURL string
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func versionString() string {
	return fmt.Sprintf("%s - %s", Version, GitSHA)
}

func updateString(current, latest string) string {
	return fmt.Sprintf("Update: %s → %s", current, latest)
}

func checkForUpdate(devMode bool, apiURL string) tea.Cmd {
	return func() tea.Msg {
		if devMode {
			return updateAvailableMsg{
				current:    Version,
				latest:     "v99.0.0",
				releaseURL: "https://github.com/clcollins/srepd/releases/tag/v99.0.0",
			}
		}

		url := apiURL
		if url == "" {
			url = githubReleasesURL
		}

		client := &http.Client{Timeout: updateCheckTimeout}
		resp, err := client.Get(url)
		if err != nil {
			log.Debug("checkForUpdate", "error", err)
			return nil
		}
		defer resp.Body.Close() //nolint:errcheck

		if resp.StatusCode != http.StatusOK {
			log.Debug("checkForUpdate", "status", resp.StatusCode)
			return nil
		}

		var release githubRelease
		if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
			log.Debug("checkForUpdate", "error", err)
			return nil
		}

		if release.TagName == "" {
			return nil
		}

		if !isNewerVersion(Version, release.TagName) {
			return nil
		}

		return updateAvailableMsg{
			current:    Version,
			latest:     release.TagName,
			releaseURL: release.HTMLURL,
		}
	}
}

func isNewerVersion(current, latest string) bool {
	if current == "dev" {
		return true
	}

	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")

	currentParts := strings.Split(current, ".")
	latestParts := strings.Split(latest, ".")

	for i := 0; i < len(latestParts) && i < len(currentParts); i++ {
		var c, l int
		_, _ = fmt.Sscanf(currentParts[i], "%d", &c)
		_, _ = fmt.Sscanf(latestParts[i], "%d", &l)
		if l > c {
			return true
		}
		if l < c {
			return false
		}
	}

	return false
}
