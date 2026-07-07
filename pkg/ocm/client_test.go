package ocm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ocmconfig "github.com/openshift-online/ocm-common/pkg/ocm/config"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
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

func TestClusterFromResponse(t *testing.T) {
	t.Run("basic field mapping", func(t *testing.T) {
		cluster, err := cmv1.NewCluster().
			ID("internal-id-123").
			ExternalID("ext-uuid-456").
			Name("mycluster").
			OpenshiftVersion("4.16.38").
			State(cmv1.ClusterStateReady).
			Build()
		require.NoError(t, err)

		info := clusterFromResponse(cluster)

		assert.Equal(t, "internal-id-123", info.ID)
		assert.Equal(t, "ext-uuid-456", info.ExternalID)
		assert.Equal(t, "mycluster", info.Name)
		assert.Equal(t, "mycluster", info.DisplayName)
		assert.Equal(t, "4.16.38", info.Version)
		assert.Equal(t, "ready", info.State)
	})

	t.Run("display name uses domain prefix and DNS", func(t *testing.T) {
		cluster, err := cmv1.NewCluster().
			ID("id-1").
			Name("mycluster").
			DomainPrefix("mycluster").
			DNS(cmv1.NewDNS().BaseDomain("abc1.p1.openshiftapps.com")).
			Build()
		require.NoError(t, err)

		info := clusterFromResponse(cluster)

		assert.Equal(t, "mycluster.abc1.p1.openshiftapps.com", info.DisplayName)
		assert.Equal(t, "mycluster", info.Name)
	})

	t.Run("region extraction", func(t *testing.T) {
		cluster, err := cmv1.NewCluster().
			ID("id-1").
			Name("test").
			Region(cmv1.NewCloudRegion().ID("us-east-1")).
			Build()
		require.NoError(t, err)

		info := clusterFromResponse(cluster)

		assert.Equal(t, "us-east-1", info.Region)
	})

	t.Run("hypershift flag", func(t *testing.T) {
		cluster, err := cmv1.NewCluster().
			ID("id-1").
			Name("test").
			Hypershift(cmv1.NewHypershift().Enabled(true)).
			Build()
		require.NoError(t, err)

		info := clusterFromResponse(cluster)

		assert.True(t, info.Hypershift)
	})

	t.Run("CCS flag", func(t *testing.T) {
		cluster, err := cmv1.NewCluster().
			ID("id-1").
			Name("test").
			CCS(cmv1.NewCCS().Enabled(true)).
			Build()
		require.NoError(t, err)

		info := clusterFromResponse(cluster)

		assert.True(t, info.CCS)
	})

	t.Run("nil optional fields are safe", func(t *testing.T) {
		cluster, err := cmv1.NewCluster().
			ID("id-1").
			Name("test").
			Build()
		require.NoError(t, err)

		info := clusterFromResponse(cluster)

		assert.Empty(t, info.Region)
		assert.False(t, info.Hypershift)
		assert.False(t, info.CCS)
		assert.Empty(t, info.Organization)
	})

	t.Run("cloud provider extraction", func(t *testing.T) {
		cluster, err := cmv1.NewCluster().
			ID("id-1").
			Name("test").
			CloudProvider(cmv1.NewCloudProvider().ID("aws")).
			Build()
		require.NoError(t, err)

		info := clusterFromResponse(cluster)

		assert.Equal(t, "aws", info.CloudProvider)
	})
}
