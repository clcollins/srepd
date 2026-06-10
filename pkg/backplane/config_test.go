package backplane

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg, err := LoadConfig()
	assert.Nil(t, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backplane config not found")
}

func TestLoadConfig_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfgDir := filepath.Join(tmpDir, configDir)
	require.NoError(t, os.MkdirAll(cfgDir, 0o755))

	proxyURL := "http://proxy.example.com:8080"
	data, _ := json.Marshal(Config{
		URL:      "https://backplane.example.com",
		ProxyURL: &proxyURL,
		Govcloud: false,
	})
	require.NoError(t, os.WriteFile(filepath.Join(cfgDir, configFile), data, 0o644))

	cfg, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "https://backplane.example.com", cfg.URL)
	assert.Equal(t, "http://proxy.example.com:8080", *cfg.ProxyURL)
	assert.False(t, cfg.Govcloud)
}

func TestLoadConfig_MalformedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfgDir := filepath.Join(tmpDir, configDir)
	require.NoError(t, os.MkdirAll(cfgDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cfgDir, configFile), []byte("{invalid}"), 0o644))

	cfg, err := LoadConfig()
	assert.Nil(t, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid backplane config")
}

func TestLoadConfig_EmptyURL(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfgDir := filepath.Join(tmpDir, configDir)
	require.NoError(t, os.MkdirAll(cfgDir, 0o755))

	data, _ := json.Marshal(Config{URL: ""})
	require.NoError(t, os.WriteFile(filepath.Join(cfgDir, configFile), data, 0o644))

	cfg, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "", cfg.URL)
}
