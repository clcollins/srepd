package tui

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
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

type releaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

type githubRelease struct {
	TagName string         `json:"tag_name"`
	HTMLURL string         `json:"html_url"`
	Assets  []releaseAsset `json:"assets"`
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

func releaseAssetName(goos, goarch string) string {
	os := strings.ToUpper(goos[:1]) + goos[1:]
	arch := goarch
	if goarch == "amd64" {
		arch = "x86_64"
	}
	return fmt.Sprintf("srepd_%s_%s.tar.gz", os, arch)
}

func findAssetURL(assets []releaseAsset, targetName string) (string, bool) {
	for _, a := range assets {
		if a.Name == targetName {
			return a.DownloadURL, true
		}
	}
	return "", false
}

func selfUpdate(assetURL string, binaryPath string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(assetURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip open failed: %w", err)
	}
	defer gr.Close() //nolint:errcheck

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("srepd binary not found in archive")
		}
		if err != nil {
			return fmt.Errorf("tar read failed: %w", err)
		}
		if hdr.Name == "srepd" {
			tmpPath := binaryPath + ".tmp"
			f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return fmt.Errorf("create temp file failed: %w", err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()          //nolint:errcheck
				os.Remove(tmpPath) //nolint:errcheck
				return fmt.Errorf("write failed: %w", err)
			}
			if err := f.Close(); err != nil {
				os.Remove(tmpPath) //nolint:errcheck
				return fmt.Errorf("close failed: %w", err)
			}
			if err := os.Rename(tmpPath, binaryPath); err != nil {
				os.Remove(tmpPath) //nolint:errcheck
				return fmt.Errorf("replace binary failed: %w", err)
			}
			return nil
		}
	}
}

// RunSelfUpdate checks for the latest release and updates the binary in place.
// This is called from the --update CLI flag before the TUI starts.
func RunSelfUpdate() error {
	log.Info("Checking for updates...")

	client := &http.Client{Timeout: updateCheckTimeout}
	resp, err := client.Get(githubReleasesURL)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse release info: %w", err)
	}

	if !isNewerVersion(Version, release.TagName) {
		log.Info("Already up to date", "version", Version)
		return nil
	}

	assetName := releaseAssetName(runtime.GOOS, runtime.GOARCH)
	assetURL, ok := findAssetURL(release.Assets, assetName)
	if !ok {
		return fmt.Errorf("no release asset found for %s/%s (expected %s)", runtime.GOOS, runtime.GOARCH, assetName)
	}

	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine binary path: %w", err)
	}

	log.Info("Downloading update", "version", release.TagName, "asset", assetName)
	if err := selfUpdate(assetURL, binaryPath); err != nil {
		return err
	}

	log.Info("Updated successfully", "version", release.TagName)
	fmt.Printf("Updated srepd to %s\n", release.TagName)
	return nil
}
