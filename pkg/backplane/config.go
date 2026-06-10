package backplane

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
)

const (
	configDir  = ".config/backplane"
	configFile = "config.json"
)

// Config holds the backplane configuration loaded from config.json.
type Config struct {
	URL      string  `json:"url"`
	ProxyURL *string `json:"proxy-url"`
	Govcloud bool    `json:"govcloud"`
}

// LoadConfig reads the backplane configuration from ~/.config/backplane/config.json.
func LoadConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("unable to determine home directory: %w", err)
	}

	path := filepath.Join(home, configDir, configFile)
	log.Debug("backplane.LoadConfig", "path", path)

	data, err := os.ReadFile(path)
	if err != nil {
		log.Debug("backplane.LoadConfig", "msg", "config file not found", "path", path)
		return nil, fmt.Errorf("backplane config not found at %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Warn("backplane.LoadConfig", "msg", "invalid JSON in config file", "path", path, "error", err)
		return nil, fmt.Errorf("invalid backplane config at %s: %w", path, err)
	}

	hasProxy := cfg.ProxyURL != nil && *cfg.ProxyURL != ""
	if cfg.URL == "" {
		log.Debug("backplane.LoadConfig", "msg", "config file has no url field, will resolve from OCM", "path", path)
	}
	log.Info("backplane.LoadConfig", "path", path, "url_set", cfg.URL != "", "has_proxy", hasProxy, "govcloud", cfg.Govcloud)
	return &cfg, nil
}
