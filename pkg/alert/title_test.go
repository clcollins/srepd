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

func TestExtractAlertName(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected string
	}{
		{
			name:     "osd_hive format",
			title:    "ClusterOperatorDown CRITICAL (1)",
			expected: "ClusterOperatorDown",
		},
		{
			name:     "osd_hive with hyphen",
			title:    "console-ErrorBudgetBurn CRITICAL (1)",
			expected: "console-ErrorBudgetBurn",
		},
		{
			name:     "osd_hive with bracket prefixes",
			title:    "[SL Sent] [OHSS-54318] CannotRetrieveUpdatesSRE CRITICAL (1)",
			expected: "CannotRetrieveUpdatesSRE",
		},
		{
			name:     "appsre FIRING format",
			title:    "[FIRING:1] ClusterProvisioningDelay - production hivep04ew2 uhc-production-xxx hive-controllers hive (edf-mas ProvisionFailed metrics production)",
			expected: "ClusterProvisioningDelay",
		},
		{
			name:     "rhobs_hcp format",
			title:    "[HCP] [RHOBS] (Critical) ClusterOperatorDown for HCP: a4ba96fe-ac69-4573-a78b-17d38eeaab99",
			expected: "ClusterOperatorDown",
		},
		{
			name:     "rhobs_hcp with spaces in alert name",
			title:    "[HCP] [RHOBS] (Critical) API SLO Burn for HCP: e1bb2c27-3a39-4611-9963-860f18babc1a",
			expected: "API SLO Burn",
		},
		{
			name:     "rhobs_infra format",
			title:    "[HCP] [RHOBS Infra] (Warning) RMOAPIRequestErrorRate",
			expected: "RMOAPIRequestErrorRate",
		},
		{
			name:     "deadmanssnitch format",
			title:    "testcluster.5uqt.p1.openshiftapps.com has gone missing",
			expected: "ClusterGoneMissing",
		},
		{
			name:     "cee escalation format",
			title:    "OHSS-54116: Access request tracker for cluster '2m89uoc2k3hg86u969bss1q0godb07ek'",
			expected: "OHSS-54116",
		},
		{
			name:     "empty string",
			title:    "",
			expected: "",
		},
		{
			name:     "plain alert name only",
			title:    "SomeAlertName",
			expected: "SomeAlertName",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractAlertName(tt.title)
			assert.Equal(t, tt.expected, result)
		})
	}
}
