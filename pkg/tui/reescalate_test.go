package tui

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestResolveReescalateLevel(t *testing.T) {
	t.Run("defaults to 2 when unset", func(t *testing.T) {
		viper.Reset()
		assert.Equal(t, uint(reEscalateDefaultPolicyLevel), resolveReescalateLevel())
		assert.Equal(t, uint(2), resolveReescalateLevel())
	})

	t.Run("uses the configured level", func(t *testing.T) {
		viper.Reset()
		viper.Set("reescalate_level", 3)
		defer viper.Reset()
		assert.Equal(t, uint(3), resolveReescalateLevel())
	})

	t.Run("falls back to default on zero or negative", func(t *testing.T) {
		viper.Reset()
		viper.Set("reescalate_level", 0)
		assert.Equal(t, uint(2), resolveReescalateLevel())

		viper.Set("reescalate_level", -1)
		assert.Equal(t, uint(2), resolveReescalateLevel())
		viper.Reset()
	})
}
