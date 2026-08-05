package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseWatcherVerdict_ValidTiers(t *testing.T) {
	tests := []struct {
		name     string
		tier     string
		wantTier Tier
	}{
		{"silent", "silent", TierSilent},
		{"noteworthy", "noteworthy", TierNoteworthy},
		{"actionable", "actionable", TierActionable},
		{"Silent_uppercase", "Silent", TierSilent},
		{"ACTIONABLE_allcaps", "ACTIONABLE", TierActionable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []byte("Some analysis.\n```json\n{\"tier\": \"" + tt.tier + "\", \"summary\": \"test summary\", \"action\": \"do something\"}\n```\n")
			v, err := ParseWatcherVerdict(input)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTier, v.Tier)
			assert.Equal(t, "test summary", v.Summary)
			assert.Equal(t, "do something", v.Action)
		})
	}
}

func TestParseWatcherVerdict_MissingBlock(t *testing.T) {
	v, err := ParseWatcherVerdict([]byte("Just plain text analysis with no JSON block"))
	require.NoError(t, err)
	assert.Equal(t, TierNoteworthy, v.Tier)
	assert.Contains(t, v.Summary, "Just plain text")
}

func TestParseWatcherVerdict_MalformedJSON(t *testing.T) {
	input := []byte("Analysis.\n```json\n{invalid json}\n```\n")
	v, err := ParseWatcherVerdict(input)
	require.NoError(t, err)
	assert.Equal(t, TierNoteworthy, v.Tier)
}

func TestParseWatcherVerdict_ExtraKeysIgnored(t *testing.T) {
	input := []byte("Analysis.\n```json\n{\"tier\": \"actionable\", \"summary\": \"s\", \"action\": \"a\", \"extra\": \"ignored\"}\n```\n")
	v, err := ParseWatcherVerdict(input)
	require.NoError(t, err)
	assert.Equal(t, TierActionable, v.Tier)
	assert.Equal(t, "s", v.Summary)
}

func TestParseWatcherVerdict_EmptyInput(t *testing.T) {
	v, err := ParseWatcherVerdict([]byte(""))
	require.NoError(t, err)
	assert.Equal(t, TierNoteworthy, v.Tier)
}

func TestParseWatcherVerdict_UnknownTier(t *testing.T) {
	input := []byte("```json\n{\"tier\": \"unknown_value\", \"summary\": \"s\"}\n```\n")
	v, err := ParseWatcherVerdict(input)
	require.NoError(t, err)
	assert.Equal(t, TierNoteworthy, v.Tier)
}
