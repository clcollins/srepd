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
// annotation so the cursor starts on the best candidate, then the rest,
// then the skip and manual-entry escapes. No option value may equal the
// empty string: huh's Select moves the cursor (and scrolls the viewport)
// to the option matching the current bound value when async options
// arrive, and the bound value starts empty — an empty-valued option would
// yank the viewport away from the top (the "picker only shows Skip" bug).
func TestBuildPolicyOptions_SilentFirstEscapesLast(t *testing.T) {
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
	assert.Contains(t, opts[0].Key, "possible candidate", "silent candidates must be annotated")
	assert.Equal(t, "PREAL1", opts[2].Value)
	assert.NotContains(t, opts[2].Key, "possible candidate")
	assert.Equal(t, policyChoiceSkip, opts[4].Value)
	assert.Equal(t, policyChoiceManual, opts[5].Value)

	for _, o := range opts {
		assert.NotEmpty(t, o.Value,
			"no option value may be empty or the initial cursor jumps off the top")
	}
}

// Within the silent candidates, policies with "silent" in the name lead:
// they are the strongest signal of intent, so the cursor starts on the
// likeliest pick even when the API returns them in another order.
func TestBuildPolicyOptions_SilentNamePriority(t *testing.T) {
	opts := buildPolicyOptions([]pagerduty.EscalationPolicy{
		silentPolicy("PNULL1", "Null Route"),
		silentPolicy("PSIL1", "Team Silent Test"),
		silentPolicy("PNONACT1", "Non-Actionable"),
		silentPolicy("PSIL2", "SILENT stage"),
		realPolicy("PREAL1", "Primary On-Call"),
	})

	// silent-named candidates, then other candidates, then real, then escapes
	assert.Equal(t, "PSIL1", opts[0].Value, `names containing "silent" lead (case-insensitive)`)
	assert.Equal(t, "PSIL2", opts[1].Value)
	assert.Equal(t, "PNULL1", opts[2].Value)
	assert.Equal(t, "PNONACT1", opts[3].Value)
	assert.Equal(t, "PREAL1", opts[4].Value)
	assert.Equal(t, policyChoiceSkip, opts[5].Value)
}

func TestBuildPolicyOptions_SentinelsAlwaysPresent(t *testing.T) {
	opts := buildPolicyOptions(nil)

	assert.Len(t, opts, 2)
	assert.Contains(t, opts[0].Key, "Skip")
	assert.Equal(t, policyChoiceSkip, opts[0].Value)
	assert.Contains(t, opts[1].Key, "manually")
	assert.Equal(t, policyChoiceManual, opts[1].Value)
}

// The silent policy conventionally lives on a companion team (e.g.
// "<team> - Non-actionable") that the user belongs to but does not select
// for incident filtering. Fetching policies for only the selected teams
// missed it, so the picker fetches across ALL of the user's teams.
func TestFetchPolicyOptions_FetchesAllUserTeams(t *testing.T) {
	mock := &pd.MockPagerDutyClient{
		ListEscalationPoliciesResponses: []pagerduty.ListEscalationPoliciesResponse{
			{EscalationPolicies: []pagerduty.EscalationPolicy{
				silentPolicy("PSILENT1", "Silent Test"),
				realPolicy("PREAL1", "Primary"),
			}},
		},
	}
	factory := func(string) pd.PagerDutyClient { return mock }

	opts := fetchPolicyOptions(factory, "u+token", "")

	assert.Len(t, opts, 4, "two policies + skip + manual")
	assert.Equal(t, "PSILENT1", opts[0].Value)
	// The mock user belongs to TEAM_001 and TEAM_002; both must be queried
	// even though a wizard selection would typically cover only one.
	if assert.Len(t, mock.RecordedListEscalationPoliciesOpts, 1) {
		assert.ElementsMatch(t, []string{"TEAM_001", "TEAM_002"},
			mock.RecordedListEscalationPoliciesOpts[0].TeamIDs)
	}
}

func TestFetchPolicyOptions_NoTokenStillOffersEscapes(t *testing.T) {
	opts := fetchPolicyOptions(mockFactoryOK(), "", "")

	// Without a token there is nothing to fetch; the user must still be
	// able to skip or enter manually.
	assert.Len(t, opts, 2)
	assert.Equal(t, policyChoiceSkip, opts[0].Value)
	assert.Equal(t, policyChoiceManual, opts[1].Value)
}

func TestFetchPolicyOptions_ErrorClassifiedAndEscapesKept(t *testing.T) {
	mock := &pd.MockPagerDutyClient{ListEscalationPoliciesErr: pagerduty.APIError{StatusCode: 429}}
	factory := func(string) pd.PagerDutyClient { return mock }

	opts := fetchPolicyOptions(factory, "u+token", "")

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
	assert.Equal(t, "", resolveSilentPolicyChoice("", "ignored"),
		"untouched picker (empty choice) means no policy")
	assert.Equal(t, "PMANUAL9", resolveSilentPolicyChoice(policyChoiceManual, "  PMANUAL9  "),
		"manual choice uses the trimmed free-text input")
	assert.Equal(t, "", resolveSilentPolicyChoice(policyChoiceManual, "   "),
		"manual with blank input means no policy")
}
