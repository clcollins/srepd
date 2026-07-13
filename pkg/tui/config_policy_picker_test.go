package tui

import (
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
)

func silentPolicy(id, name string) pagerduty.EscalationPolicy {
	return pagerduty.EscalationPolicy{
		APIObject: pagerduty.APIObject{ID: id},
		Name:      name,
		EscalationRules: []pagerduty.EscalationRule{
			{Targets: []pagerduty.APIObject{{Type: "user_reference", ID: "UBOT"}}},
		},
	}
}

func realPolicy(id, name string) pagerduty.EscalationPolicy {
	return pagerduty.EscalationPolicy{
		APIObject: pagerduty.APIObject{ID: id},
		Name:      name,
		EscalationRules: []pagerduty.EscalationRule{
			{Targets: []pagerduty.APIObject{{Type: "schedule_reference", ID: "SCHED1"}}},
		},
	}
}

// OB-4: the silent-policy step becomes a fetched picker. SILENT-classified
// policies (no schedules notified) lead the list with a recommendation
// annotation; skip and manual-entry escapes are always present.
func TestBuildPolicyOptions_SilentFirstWithAnnotation(t *testing.T) {
	opts := buildPolicyOptions([]pagerduty.EscalationPolicy{
		realPolicy("PREAL1", "Primary On-Call"),
		silentPolicy("PSILENT1", "Silent Test"),
		realPolicy("PREAL2", "Secondary"),
		silentPolicy("PSILENT2", "Null Route"),
	})

	// 4 policies + skip + manual
	assert.Len(t, opts, 6)
	assert.Equal(t, "PSILENT1", opts[0].Value, "silent policies must come first")
	assert.Equal(t, "PSILENT2", opts[1].Value)
	assert.Contains(t, opts[0].Key, "Silent Test")
	assert.Contains(t, opts[0].Key, "recommended", "silent candidates must be annotated")
	assert.Equal(t, "PREAL1", opts[2].Value)
	assert.NotContains(t, opts[2].Key, "recommended")
}

func TestBuildPolicyOptions_SentinelsAlwaysPresent(t *testing.T) {
	opts := buildPolicyOptions(nil)

	assert.Len(t, opts, 2)
	assert.Contains(t, opts[0].Key, "Skip")
	assert.Equal(t, policyChoiceSkip, opts[0].Value)
	assert.Contains(t, opts[1].Key, "manually")
	assert.Equal(t, policyChoiceManual, opts[1].Value)
}

func TestFetchPolicyOptions_UsesTeamFilter(t *testing.T) {
	mock := &pd.MockPagerDutyClient{
		ListEscalationPoliciesResponses: []pagerduty.ListEscalationPoliciesResponse{
			{EscalationPolicies: []pagerduty.EscalationPolicy{
				silentPolicy("PSILENT1", "Silent Test"),
				realPolicy("PREAL1", "Primary"),
			}},
		},
	}
	factory := func(string) pd.PagerDutyClient { return mock }

	opts := fetchPolicyOptions(factory, "u+token", "", []string{"TEAM_001"})

	assert.Len(t, opts, 4, "two policies + skip + manual")
	assert.Equal(t, "PSILENT1", opts[0].Value)
}

func TestFetchPolicyOptions_NoTeamsStillOffersEscapes(t *testing.T) {
	opts := fetchPolicyOptions(mockFactoryOK(), "u+token", "", nil)

	// GetTeamEscalationPolicies with no teams returns no policies; the user
	// must still be able to skip or enter manually.
	assert.Len(t, opts, 2)
	assert.Equal(t, policyChoiceSkip, opts[0].Value)
	assert.Equal(t, policyChoiceManual, opts[1].Value)
}

func TestFetchPolicyOptions_ErrorClassifiedAndEscapesKept(t *testing.T) {
	mock := &pd.MockPagerDutyClient{ListEscalationPoliciesErr: pagerduty.APIError{StatusCode: 429}}
	factory := func(string) pd.PagerDutyClient { return mock }

	opts := fetchPolicyOptions(factory, "u+token", "", []string{"TEAM_001"})

	// The classified error is shown as a disabled-style row, and the user can
	// still skip or enter an ID manually.
	assert.Len(t, opts, 3)
	assert.Contains(t, opts[0].Key, "rate limited")
	assert.Equal(t, policyChoiceSkip, opts[0].Value, "error row must resolve to skip so selecting it is harmless")
	assert.Equal(t, policyChoiceSkip, opts[1].Value)
	assert.Equal(t, policyChoiceManual, opts[2].Value)
}

func TestResolveSilentPolicyChoice(t *testing.T) {
	assert.Equal(t, "PSILENT1", resolveSilentPolicyChoice("PSILENT1", "ignored"),
		"picker choice wins")
	assert.Equal(t, "", resolveSilentPolicyChoice(policyChoiceSkip, "ignored"),
		"skip means no policy")
	assert.Equal(t, "PMANUAL9", resolveSilentPolicyChoice(policyChoiceManual, "  PMANUAL9  "),
		"manual choice uses the trimmed free-text input")
	assert.Equal(t, "", resolveSilentPolicyChoice(policyChoiceManual, "   "),
		"manual with blank input means no policy")
}
