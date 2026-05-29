package alert

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripBracketPrefixes_SLSent(t *testing.T) {
	cleaned, tags := StripBracketPrefixes("[SL Sent] ClusterOperatorDown CRITICAL (1)")
	assert.Equal(t, "ClusterOperatorDown CRITICAL (1)", cleaned)
	assert.Equal(t, []string{"SL Sent"}, tags)
}

func TestStripBracketPrefixes_Multiple(t *testing.T) {
	// Per spec: "Repeatedly remove leading [...] patterns until the first non-bracket character"
	// So ALL leading brackets get stripped, including [HCP] and [RHOBS]
	cleaned, tags := StripBracketPrefixes("[SL Sent] [OHSS-52020 ] [HCP] [RHOBS] (Critical) ClusterOperatorDown for HCP: abc-123")
	assert.Equal(t, "(Critical) ClusterOperatorDown for HCP: abc-123", cleaned)
	assert.Equal(t, []string{"SL Sent", "OHSS-52020", "HCP", "RHOBS"}, tags)
}

func TestStripBracketPrefixes_None(t *testing.T) {
	cleaned, tags := StripBracketPrefixes("ClusterOperatorDown CRITICAL (1)")
	assert.Equal(t, "ClusterOperatorDown CRITICAL (1)", cleaned)
	assert.Empty(t, tags)
}

func TestStripBracketPrefixes_VariantCasing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantTag  string
		wantRest string
	}{
		{
			name:     "SL_Sent underscore variant",
			input:    "[SL_Sent] AlertName CRITICAL (1)",
			wantTag:  "SL_Sent",
			wantRest: "AlertName CRITICAL (1)",
		},
		{
			name:     "SL SENT uppercase",
			input:    "[SL SENT] AlertName CRITICAL (1)",
			wantTag:  "SL SENT",
			wantRest: "AlertName CRITICAL (1)",
		},
		{
			name:     "SL sent lowercase",
			input:    "[SL sent] AlertName CRITICAL (1)",
			wantTag:  "SL sent",
			wantRest: "AlertName CRITICAL (1)",
		},
		{
			name:     "Proactive Case with colon",
			input:    "[Proactive Case: OHSS-54318] AlertName CRITICAL (1)",
			wantTag:  "Proactive Case: OHSS-54318",
			wantRest: "AlertName CRITICAL (1)",
		},
		{
			name:     "Jira ticket reference",
			input:    "[SREPHOA-60] AlertName CRITICAL (1)",
			wantTag:  "SREPHOA-60",
			wantRest: "AlertName CRITICAL (1)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned, tags := StripBracketPrefixes(tt.input)
			assert.Equal(t, tt.wantRest, cleaned)
			assert.Contains(t, tags, tt.wantTag)
		})
	}
}

func TestStripBracketPrefixes_TrailingSpaceInBracket(t *testing.T) {
	// Real-world case: "[OHSS-52020 ]" has trailing space inside bracket
	cleaned, tags := StripBracketPrefixes("[OHSS-52020 ] AlertName CRITICAL (1)")
	assert.Equal(t, "AlertName CRITICAL (1)", cleaned)
	assert.Equal(t, []string{"OHSS-52020"}, tags)
}

func TestStripBracketPrefixes_EmptyString(t *testing.T) {
	cleaned, tags := StripBracketPrefixes("")
	assert.Equal(t, "", cleaned)
	assert.Empty(t, tags)
}
