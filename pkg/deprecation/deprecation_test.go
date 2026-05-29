package deprecation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeprecated_KnownKey_Shell(t *testing.T) {
	assert.True(t, Deprecated("shell"), "expected 'shell' to be deprecated")
}

func TestDeprecated_KnownKey_Silentuser(t *testing.T) {
	assert.True(t, Deprecated("silentuser"), "expected 'silentuser' to be deprecated")
}

func TestDeprecated_UnknownKey(t *testing.T) {
	assert.False(t, Deprecated("token"), "expected 'token' to not be deprecated")
}

func TestDeprecated_EmptyString(t *testing.T) {
	assert.False(t, Deprecated(""), "expected empty string to not be deprecated")
}
