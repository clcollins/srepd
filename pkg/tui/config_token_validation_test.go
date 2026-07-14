package tui

import (
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
)

func mockFactoryWithErr(err error) func(string) pd.PagerDutyClient {
	return func(string) pd.PagerDutyClient {
		return &pd.MockPagerDutyClient{GetCurrentUserErr: err}
	}
}

func mockFactoryOK() func(string) pd.PagerDutyClient {
	return func(string) pd.PagerDutyClient { return &pd.MockPagerDutyClient{} }
}

func TestValidateTokenInput_EmptyWithExistingToken(t *testing.T) {
	err := validateTokenInput(mockFactoryOK(), "", "existing-token")
	assert.NoError(t, err, "blank input keeps the existing token")
}

func TestValidateTokenInput_EmptyWithoutExistingToken(t *testing.T) {
	err := validateTokenInput(mockFactoryOK(), "  ", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestValidateTokenInput_ValidToken(t *testing.T) {
	err := validateTokenInput(mockFactoryOK(), "u+goodtoken", "")
	assert.NoError(t, err)
}

// OB-3: a 401 must be reported as a token problem with acquisition help, not
// a raw API dump.
func TestValidateTokenInput_Classifies401(t *testing.T) {
	err := validateTokenInput(mockFactoryWithErr(pagerduty.APIError{StatusCode: 401}), "u+badtoken", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired token")
}

func TestFetchTeamOptions_NoTokenPrompts(t *testing.T) {
	opts, teams, _ := fetchTeamOptions(mockFactoryOK(), "", "", nil)
	assert.Nil(t, teams)
	assert.Len(t, opts, 1)
	assert.Contains(t, opts[0].Key, "enter a token first")
}

func TestFetchTeamOptions_Success(t *testing.T) {
	opts, teams, _ := fetchTeamOptions(mockFactoryOK(), "u+goodtoken", "", map[string]bool{"TEAM_002": true})
	assert.Len(t, teams, 2, "mock returns two teams")
	assert.Len(t, opts, 2)
	assert.Contains(t, opts[0].Key, "Mock Team Alpha")
}

// OB-3: the error option must carry the classified message, and its value
// must be empty so validation can reject it.
func TestFetchTeamOptions_ClassifiedErrorOption(t *testing.T) {
	opts, teams, userName := fetchTeamOptions(mockFactoryWithErr(pagerduty.APIError{StatusCode: 429}), "u+token", "", nil)
	assert.Nil(t, teams)
	assert.Empty(t, userName)
	assert.Len(t, opts, 1)
	assert.Contains(t, opts[0].Key, "rate limited")
	assert.Equal(t, "", opts[0].Value)
}

func TestValidateTeamValues(t *testing.T) {
	assert.Error(t, validateTeamValues(nil), "no teams selected")
	assert.Error(t, validateTeamValues([]string{""}),
		"selecting the error/prompt placeholder must be rejected")
	assert.NoError(t, validateTeamValues([]string{"TEAM_001"}))
}

func TestValidateTeamValues_ErrorMentionsRecovery(t *testing.T) {
	err := validateTeamValues([]string{""})
	assert.Contains(t, err.Error(), "shift+tab", "error must tell the user how to go back and fix the token")
}
