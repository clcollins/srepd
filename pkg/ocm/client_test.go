package ocm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ocmconfig "github.com/openshift-online/ocm-common/pkg/ocm/config"
)

func TestCheckTokens_NoConfigFile(t *testing.T) {
	t.Run("returns not armed when no config exists", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("OCM_CONFIG", filepath.Join(dir, "nonexistent.json"))

		cfg, armed, err := CheckTokens()

		assert.NoError(t, err)
		assert.False(t, armed)
		assert.NotNil(t, cfg, "should return a config even when unarmed")
	})
}

func TestCheckTokens_EmptyConfig(t *testing.T) {
	t.Run("returns not armed when config has no tokens", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "ocm.json")
		require.NoError(t, os.WriteFile(cfgPath, []byte("{}"), 0600))
		t.Setenv("OCM_CONFIG", cfgPath)

		cfg, armed, err := CheckTokens()

		assert.NoError(t, err)
		assert.False(t, armed)
		assert.NotNil(t, cfg)
		assert.Equal(t, productionURL, cfg.URL, "should set production URL")
		assert.Equal(t, clientID, cfg.ClientID, "should set default client ID")
	})
}

func TestCheckTokens_SetsDefaults(t *testing.T) {
	t.Run("sets URL, ClientID, TokenURL, and Scopes when empty", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "ocm.json")
		require.NoError(t, os.WriteFile(cfgPath, []byte("{}"), 0600))
		t.Setenv("OCM_CONFIG", cfgPath)

		cfg, _, err := CheckTokens()

		assert.NoError(t, err)
		assert.Equal(t, productionURL, cfg.URL)
		assert.Equal(t, clientID, cfg.ClientID)
		assert.NotEmpty(t, cfg.TokenURL)
		assert.Contains(t, cfg.Scopes, "openid")
	})
}

func TestCheckTokens_PreservesExistingClientID(t *testing.T) {
	t.Run("preserves non-empty ClientID from config", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "ocm.json")
		data, _ := json.Marshal(ocmconfig.Config{ClientID: "custom-client"})
		require.NoError(t, os.WriteFile(cfgPath, data, 0600))
		t.Setenv("OCM_CONFIG", cfgPath)

		cfg, _, err := CheckTokens()

		assert.NoError(t, err)
		assert.Equal(t, "custom-client", cfg.ClientID)
	})
}

func TestApplyAuthToken_RefreshToken(t *testing.T) {
	t.Run("offline token is set as RefreshToken", func(t *testing.T) {
		cfg := &ocmconfig.Config{}
		// "Offline" typ claim token — craft a minimal unencrypted JWT
		// Header: {"alg":"none","typ":"JWT"}
		// Payload: {"typ":"Offline","exp":9999999999}
		token := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJ0eXAiOiJPZmZsaW5lIiwiZXhwIjo5OTk5OTk5OTk5fQ."

		ApplyAuthToken(cfg, token)

		assert.Equal(t, token, cfg.RefreshToken)
		assert.Empty(t, cfg.AccessToken)
	})
}

func TestApplyAuthToken_AccessToken(t *testing.T) {
	t.Run("bearer token is set as AccessToken", func(t *testing.T) {
		cfg := &ocmconfig.Config{}
		// "Bearer" typ claim token
		// Header: {"alg":"none","typ":"JWT"}
		// Payload: {"typ":"Bearer","exp":9999999999}
		token := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJ0eXAiOiJCZWFyZXIiLCJleHAiOjk5OTk5OTk5OTl9."

		ApplyAuthToken(cfg, token)

		assert.Equal(t, token, cfg.AccessToken)
		assert.Empty(t, cfg.RefreshToken)
	})
}

func TestApplyAuthToken_UnparseableToken(t *testing.T) {
	t.Run("unparseable token is set as AccessToken", func(t *testing.T) {
		cfg := &ocmconfig.Config{}
		token := "not-a-valid-jwt"

		ApplyAuthToken(cfg, token)

		assert.Equal(t, token, cfg.AccessToken)
	})
}

func TestNewClientFromConfig_NilConfig(t *testing.T) {
	t.Run("returns error for nil config", func(t *testing.T) {
		_, err := NewClientFromConfig(nil, "test-version")

		assert.Error(t, err)
	})
}
