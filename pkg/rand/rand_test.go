package rand

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringWithCharset_CorrectLength(t *testing.T) {
	lengths := []int{1, 5, 10, 50, 100}
	for _, length := range lengths {
		result := StringWithCharset(length, "abc")
		assert.Equal(t, length, len(result), "expected length %d, got %d", length, len(result))
	}
}

func TestStringWithCharset_OnlyCharsetChars(t *testing.T) {
	customCharset := "XYZ"
	result := StringWithCharset(100, customCharset)
	for _, ch := range result {
		assert.True(t, strings.ContainsRune(customCharset, ch),
			"character %q is not in charset %q", ch, customCharset)
	}
}

func TestStringWithCharset_ZeroLength(t *testing.T) {
	result := StringWithCharset(0, "abc")
	assert.Equal(t, "", result, "expected empty string for zero length")
}

func TestString_CorrectLength(t *testing.T) {
	lengths := []int{1, 5, 10, 50}
	for _, length := range lengths {
		result := String(length)
		assert.Equal(t, length, len(result), "expected length %d, got %d", length, len(result))
	}
}

func TestString_DefaultCharset(t *testing.T) {
	defaultCharset := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := String(200)
	for _, ch := range result {
		assert.True(t, strings.ContainsRune(defaultCharset, ch),
			"character %q is not in the default charset %q", ch, defaultCharset)
	}
}

func TestID_HasPrefix(t *testing.T) {
	prefix := "TEST-"
	result := ID(prefix)
	assert.True(t, strings.HasPrefix(result, prefix),
		"expected ID %q to start with prefix %q", result, prefix)
}

func TestID_CorrectTotalLength(t *testing.T) {
	prefix := "PRE"
	result := ID(prefix)
	expectedLength := len(prefix) + 13
	assert.Equal(t, expectedLength, len(result),
		"expected total length %d (prefix %d + 13), got %d", expectedLength, len(prefix), len(result))
}
