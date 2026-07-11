package tui

import (
	"testing"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestParseTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "comma-separated bare words",
			input:    "foo, bar, baz",
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:     "comma-separated with brackets",
			input:    "[foo],[bar],[baz]",
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:     "single tag with colon",
			input:    "firing:1",
			expected: []string{"firing:1"},
		},
		{
			name:     "single tag with space",
			input:    "SL Sent",
			expected: []string{"SL Sent"},
		},
		{
			name:     "bracket-delimited no commas",
			input:    "[FOO] [Bar] [baz]",
			expected: []string{"FOO", "Bar", "baz"},
		},
		{
			name:     "bare words no commas no brackets",
			input:    "foo bar baz",
			expected: []string{"foo bar baz"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "all-empty tokens from commas",
			input:    "  ,  , ",
			expected: []string{},
		},
		{
			name:     "single bracketed tag",
			input:    "[already-bracketed]",
			expected: []string{"already-bracketed"},
		},
		{
			name:     "mixed bracketed and bare with commas",
			input:    "[HCP], RHOBS, [SL Sent]",
			expected: []string{"HCP", "RHOBS", "SL Sent"},
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: []string{},
		},
		{
			name:     "single comma",
			input:    ",",
			expected: []string{},
		},
		{
			name:     "brackets with extra whitespace",
			input:    "  [  HCP  ]  ,  [  RHOBS  ]  ",
			expected: []string{"HCP", "RHOBS"},
		},
		{
			name:     "bracket-delimited with leading bracket",
			input:    "[FIRING:1] [HCP]",
			expected: []string{"FIRING:1", "HCP"},
		},
		{
			name:     "single bracket pair with spaces inside",
			input:    "[SL Sent]",
			expected: []string{"SL Sent"},
		},
		{
			name:     "nested brackets stripped to content",
			input:    "[[nested]]",
			expected: []string{"nested"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseTags(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		expected string
	}{
		{
			name:     "multiple tags",
			tags:     []string{"foo", "bar"},
			expected: "[foo][bar]",
		},
		{
			name:     "empty slice",
			tags:     []string{},
			expected: "",
		},
		{
			name:     "nil slice",
			tags:     nil,
			expected: "",
		},
		{
			name:     "single tag",
			tags:     []string{"single"},
			expected: "[single]",
		},
		{
			name:     "tag with spaces",
			tags:     []string{"SL Sent"},
			expected: "[SL Sent]",
		},
		{
			name:     "tag with colon",
			tags:     []string{"firing:1"},
			expected: "[firing:1]",
		},
		{
			name:     "three tags",
			tags:     []string{"HCP", "RHOBS", "SL Sent"},
			expected: "[HCP][RHOBS][SL Sent]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTags(tt.tags)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPrependTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     string
		title    string
		expected string
	}{
		{
			name:     "prepend to bare title",
			tags:     "[HCP][RHOBS]",
			title:    "ClusterOperatorDown CRITICAL (1)",
			expected: "[HCP][RHOBS] ClusterOperatorDown CRITICAL (1)",
		},
		{
			name:     "prepend to title with existing tags",
			tags:     "[SL Sent]",
			title:    "[HCP] ClusterOperatorDown CRITICAL (1)",
			expected: "[SL Sent][HCP] ClusterOperatorDown CRITICAL (1)",
		},
		{
			name:     "empty tags returns original title",
			tags:     "",
			title:    "ClusterOperatorDown CRITICAL (1)",
			expected: "ClusterOperatorDown CRITICAL (1)",
		},
		{
			name:     "duplicate tag not added",
			tags:     "[HCP]",
			title:    "[HCP] ClusterOperatorDown CRITICAL (1)",
			expected: "[HCP] ClusterOperatorDown CRITICAL (1)",
		},
		{
			name:     "duplicate among multiple — only new ones added",
			tags:     "[HCP][RHOBS][SL Sent]",
			title:    "[HCP][RHOBS] ClusterOperatorDown CRITICAL (1)",
			expected: "[SL Sent][HCP][RHOBS] ClusterOperatorDown CRITICAL (1)",
		},
		{
			name:     "case-insensitive duplicate detection",
			tags:     "[hcp]",
			title:    "[HCP] ClusterOperatorDown CRITICAL (1)",
			expected: "[HCP] ClusterOperatorDown CRITICAL (1)",
		},
		{
			name:     "empty title with tags",
			tags:     "[HCP]",
			title:    "",
			expected: "[HCP]",
		},
		{
			name:     "all tags are duplicates",
			tags:     "[HCP][RHOBS]",
			title:    "[HCP][RHOBS] SomeAlert",
			expected: "[HCP][RHOBS] SomeAlert",
		},
		{
			name:     "tags with spaces between existing brackets",
			tags:     "[SL Sent]",
			title:    "[HCP] [RHOBS] SomeAlert",
			expected: "[SL Sent][HCP] [RHOBS] SomeAlert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PrependTags(tt.tags, tt.title)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractExistingTags(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected []string
	}{
		{
			name:     "no tags",
			title:    "ClusterOperatorDown CRITICAL (1)",
			expected: []string{},
		},
		{
			name:     "single tag",
			title:    "[HCP] ClusterOperatorDown",
			expected: []string{"HCP"},
		},
		{
			name:     "multiple tags no spaces",
			title:    "[HCP][RHOBS] ClusterOperatorDown",
			expected: []string{"HCP", "RHOBS"},
		},
		{
			name:     "multiple tags with spaces",
			title:    "[HCP] [RHOBS] ClusterOperatorDown",
			expected: []string{"HCP", "RHOBS"},
		},
		{
			name:     "tag with spaces inside",
			title:    "[SL Sent] ClusterOperatorDown",
			expected: []string{"SL Sent"},
		},
		{
			name:     "mixed tags",
			title:    "[SL Sent][OHSS-54318] CannotRetrieveUpdatesSRE",
			expected: []string{"SL Sent", "OHSS-54318"},
		},
		{
			name:     "FIRING tag",
			title:    "[FIRING:1] ClusterProvisioningDelay",
			expected: []string{"FIRING:1"},
		},
		{
			name:     "empty string",
			title:    "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractExistingTags(tt.title)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// tagKeyMsg returns a tea.KeyMsg for ctrl+t.
func tagKeyMsg() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyCtrlT}
}

func createTestModelWithIncidentRows(incidents []pagerduty.Incident) model {
	m := createTestModel()
	m.input = newTextInput()
	m.incidentList = incidents

	cols := []table.Column{
		{Title: "Status", Width: 2},
		{Title: "ID", Width: 10},
		{Title: "Summary", Width: 30},
		{Title: "Service", Width: 20},
	}

	var rows []table.Row
	for _, inc := range incidents {
		rows = append(rows, table.Row{".", inc.ID, inc.Title, inc.Service.Summary})
	}

	m.table = table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
	)

	return m
}

func TestTagKey_OpensInputWithPrompt(t *testing.T) {
	incidents := []pagerduty.Incident{
		{
			APIObject:          pagerduty.APIObject{ID: "Q111"},
			Title:              "ClusterOperatorDown CRITICAL (1)",
			Service:            pagerduty.APIObject{ID: "SVC1", Summary: "test-service"},
			LastStatusChangeAt: time.Now().Format(time.RFC3339),
		},
	}
	m := createTestModelWithIncidentRows(incidents)

	result, _ := m.Update(tagKeyMsg())
	updatedModel := result.(model)

	assert.True(t, updatedModel.input.Focused(), "input should be focused after ctrl+t")
	assert.True(t, updatedModel.tagInputActive, "tagInputActive should be set")
	assert.Equal(t, tagInputPrompt, updatedModel.input.Value(),
		"input should be pre-populated with tag prompt")
}

func TestTagKey_NoIncidentHighlighted(t *testing.T) {
	m := createTestModel()
	m.table.Focus()

	result, _ := m.Update(tagKeyMsg())
	updatedModel := result.(model)

	assert.False(t, updatedModel.input.Focused(), "input should not be focused")
	assert.False(t, updatedModel.tagInputActive, "tagInputActive should not be set")
	assert.Contains(t, updatedModel.status, "no incident highlighted")
}

func TestTagInput_EnterProcessesTags(t *testing.T) {
	incidents := []pagerduty.Incident{
		{
			APIObject:          pagerduty.APIObject{ID: "Q111"},
			Title:              "ClusterOperatorDown CRITICAL (1)",
			Service:            pagerduty.APIObject{ID: "SVC1", Summary: "test-service"},
			LastStatusChangeAt: time.Now().Format(time.RFC3339),
		},
	}
	m := createTestModelWithIncidentRows(incidents)
	m.selectedIncident = &incidents[0]
	m.tagInputActive = true
	m.input.Focus()
	m.input.SetValue(tagInputPrompt + "HCP, RHOBS")

	result, cmd := m.Update(enterKeyMsg())
	updatedModel := result.(model)

	assert.False(t, updatedModel.input.Focused(), "input should be blurred after Enter")
	assert.False(t, updatedModel.tagInputActive, "tagInputActive should be cleared")
	assert.NotNil(t, cmd, "should return a command to update the title")
}

func TestTagInput_EmptyTagsAfterPrompt(t *testing.T) {
	incidents := []pagerduty.Incident{
		{
			APIObject:          pagerduty.APIObject{ID: "Q111"},
			Title:              "ClusterOperatorDown CRITICAL (1)",
			Service:            pagerduty.APIObject{ID: "SVC1", Summary: "test-service"},
			LastStatusChangeAt: time.Now().Format(time.RFC3339),
		},
	}
	m := createTestModelWithIncidentRows(incidents)
	m.selectedIncident = &incidents[0]
	m.tagInputActive = true
	m.input.Focus()
	m.input.SetValue(tagInputPrompt)

	result, cmd := m.Update(enterKeyMsg())
	updatedModel := result.(model)

	assert.False(t, updatedModel.input.Focused(), "input should be blurred")
	assert.False(t, updatedModel.tagInputActive, "tagInputActive should be cleared")
	assert.Nil(t, cmd, "no command when tags are empty")
	assert.Contains(t, updatedModel.status, "no tags")
}

func TestTagInput_EscapeCancels(t *testing.T) {
	m := createTestModel()
	m.input = newTextInput()
	m.tagInputActive = true
	m.input.Focus()
	m.input.SetValue(tagInputPrompt + "HCP")

	escMsg := tea.KeyMsg{Type: tea.KeyEscape}
	result, _ := m.Update(escMsg)
	updatedModel := result.(model)

	assert.False(t, updatedModel.input.Focused(), "input should be blurred after Esc")
	assert.False(t, updatedModel.tagInputActive, "tagInputActive should be cleared on Esc")
}

func TestUpdatedIncidentTitleMsg_UpdatesIncidentList(t *testing.T) {
	incidents := []pagerduty.Incident{
		{
			APIObject:          pagerduty.APIObject{ID: "Q111"},
			Title:              "ClusterOperatorDown CRITICAL (1)",
			Service:            pagerduty.APIObject{ID: "SVC1", Summary: "test-service"},
			LastStatusChangeAt: time.Now().Format(time.RFC3339),
		},
		{
			APIObject:          pagerduty.APIObject{ID: "Q222"},
			Title:              "SomeOtherAlert",
			Service:            pagerduty.APIObject{ID: "SVC2", Summary: "test-service-2"},
			LastStatusChangeAt: time.Now().Format(time.RFC3339),
		},
	}
	m := createTestModelWithIncidentRows(incidents)

	msg := updatedIncidentTitleMsg{
		incidentID: "Q111",
		newTitle:   "[HCP] ClusterOperatorDown CRITICAL (1)",
	}

	result, _ := m.Update(msg)
	updatedModel := result.(model)

	assert.Equal(t, "[HCP] ClusterOperatorDown CRITICAL (1)", updatedModel.incidentList[0].Title)
	assert.Equal(t, "SomeOtherAlert", updatedModel.incidentList[1].Title)
}

func TestUpdatedIncidentTitleMsg_Error(t *testing.T) {
	m := createTestModel()

	msg := updatedIncidentTitleMsg{
		incidentID: "Q111",
		newTitle:   "[HCP] SomeAlert",
		err:        assert.AnError,
	}

	result, _ := m.Update(msg)
	updatedModel := result.(model)

	assert.Contains(t, updatedModel.status, "tag update failed")
}
