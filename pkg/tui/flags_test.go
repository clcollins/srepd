package tui

import (
	"testing"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/clcollins/srepd/pkg/ocm"
	"github.com/stretchr/testify/assert"
)

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		value   string
		want    bool
	}{
		// Plain string (contains)
		{"contains match", "Widget", "Acme Widget Corp", true},
		{"contains no match", "Gadget", "Acme Widget Corp", false},
		{"contains case insensitive", "widget", "Acme Widget Corp", true},
		{"contains empty pattern", "", "Acme Widget Corp", true},
		{"contains empty value", "Widget", "", false},

		// ^STRING (hasPrefix)
		{"prefix match", "^Acme", "Acme Widget Corp", true},
		{"prefix no match", "^Widget", "Acme Widget Corp", false},
		{"prefix case insensitive", "^acme", "Acme Widget Corp", true},

		// STRING$ (hasSuffix)
		{"suffix match", "Corp$", "Acme Widget Corp", true},
		{"suffix no match", "Widget$", "Acme Widget Corp", false},
		{"suffix case insensitive", "corp$", "Acme Widget Corp", true},

		// STRING* (hasPrefix, trailing wildcard)
		{"prefix wildcard match", "Acme*", "Acme Widget Corp", true},
		{"prefix wildcard no match", "Widget*", "Acme Widget Corp", false},
		{"prefix wildcard case insensitive", "acme*", "Acme Widget Corp", true},

		// ^STRING* (hasPrefix, both markers)
		{"caret prefix wildcard match", "^Acme*", "Acme Widget Corp", true},
		{"caret prefix wildcard no match", "^Widget*", "Acme Widget Corp", false},

		// *STRING$ (hasSuffix, leading wildcard)
		{"star suffix match", "*Corp$", "Acme Widget Corp", true},
		{"star suffix no match", "*Widget$", "Acme Widget Corp", false},
		{"star suffix case insensitive", "*corp$", "Acme Widget Corp", true},

		// Edge cases
		{"exact match plain", "Acme Widget Corp", "Acme Widget Corp", true},
		{"pattern equals value with prefix", "^Acme Widget Corp", "Acme Widget Corp", true},
		{"pattern equals value with suffix", "Acme Widget Corp$", "Acme Widget Corp", true},
		{"single char pattern", "A", "Acme", true},
		{"single char no match", "Z", "Acme", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchGlob(tt.pattern, tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvaluateFlags_NoConditions(t *testing.T) {
	t.Run("returns nil when no conditions", func(t *testing.T) {
		result := evaluateFlags(
			[]string{"INC001"},
			nil,
			map[string][]string{"INC001": {"cluster1"}},
			map[string]*ocm.ClusterInfo{},
		)
		assert.Nil(t, result)
	})
}

func TestEvaluateFlags_ClusterIDDirect(t *testing.T) {
	t.Run("matches by raw cluster ID from alerts", func(t *testing.T) {
		conditions := []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "1q2w3e4rfakeidtest9o0p1a2s3d4f5g", CreatedAt: time.Now()},
		}
		result := evaluateFlags(
			[]string{"INC001"},
			conditions,
			map[string][]string{"INC001": {"1q2w3e4rfakeidtest9o0p1a2s3d4f5g"}},
			map[string]*ocm.ClusterInfo{},
		)
		assert.Equal(t, []int{1}, result["INC001"])
	})
}

func TestEvaluateFlags_ClusterIDExternal(t *testing.T) {
	t.Run("matches by external ID via clusterCache", func(t *testing.T) {
		conditions := []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "00000000-fake-uuid-test-999999999999", CreatedAt: time.Now()},
		}
		result := evaluateFlags(
			[]string{"INC001"},
			conditions,
			map[string][]string{"INC001": {"1q2w3e4rfakeidtest9o0p1a2s3d4f5g"}},
			map[string]*ocm.ClusterInfo{
				"1q2w3e4rfakeidtest9o0p1a2s3d4f5g": {
					ID:         "internal-id-fake",
					ExternalID: "00000000-fake-uuid-test-999999999999",
				},
			},
		)
		assert.Equal(t, []int{1}, result["INC001"])
	})
}

func TestEvaluateFlags_ClusterIDInternalViaCache(t *testing.T) {
	t.Run("matches by internal OCM ID via clusterCache", func(t *testing.T) {
		conditions := []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "internal-id-fake", CreatedAt: time.Now()},
		}
		result := evaluateFlags(
			[]string{"INC001"},
			conditions,
			map[string][]string{"INC001": {"1q2w3e4rfakeidtest9o0p1a2s3d4f5g"}},
			map[string]*ocm.ClusterInfo{
				"1q2w3e4rfakeidtest9o0p1a2s3d4f5g": {
					ID:         "internal-id-fake",
					ExternalID: "00000000-fake-uuid-test-999999999999",
				},
			},
		)
		assert.Equal(t, []int{1}, result["INC001"])
	})
}

func TestEvaluateFlags_ClusterIDNoMatch(t *testing.T) {
	t.Run("no match returns empty for that incident", func(t *testing.T) {
		conditions := []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "nonexistent-cluster", CreatedAt: time.Now()},
		}
		result := evaluateFlags(
			[]string{"INC001"},
			conditions,
			map[string][]string{"INC001": {"1q2w3e4rfakeidtest9o0p1a2s3d4f5g"}},
			map[string]*ocm.ClusterInfo{},
		)
		assert.Empty(t, result["INC001"])
	})
}

func TestEvaluateFlags_OrgNameContains(t *testing.T) {
	t.Run("matches org name by contains", func(t *testing.T) {
		conditions := []FlagCondition{
			{ID: 1, Type: FlagOrgName, Pattern: "Aeronautical", CreatedAt: time.Now()},
		}
		result := evaluateFlags(
			[]string{"INC001"},
			conditions,
			map[string][]string{"INC001": {"cluster1"}},
			map[string]*ocm.ClusterInfo{
				"cluster1": {ID: "cluster1", Organization: "Fake Aeronautical Ltd"},
			},
		)
		assert.Equal(t, []int{1}, result["INC001"])
	})
}

func TestEvaluateFlags_OrgNamePrefix(t *testing.T) {
	t.Run("matches org name by prefix glob", func(t *testing.T) {
		conditions := []FlagCondition{
			{ID: 1, Type: FlagOrgName, Pattern: "^Fake*", CreatedAt: time.Now()},
		}
		result := evaluateFlags(
			[]string{"INC001"},
			conditions,
			map[string][]string{"INC001": {"cluster1"}},
			map[string]*ocm.ClusterInfo{
				"cluster1": {ID: "cluster1", Organization: "Fake Aeronautical Ltd"},
			},
		)
		assert.Equal(t, []int{1}, result["INC001"])
	})
}

func TestEvaluateFlags_OrgNameNoMatch(t *testing.T) {
	t.Run("no match when org doesn't match pattern", func(t *testing.T) {
		conditions := []FlagCondition{
			{ID: 1, Type: FlagOrgName, Pattern: "Nonexistent Corp", CreatedAt: time.Now()},
		}
		result := evaluateFlags(
			[]string{"INC001"},
			conditions,
			map[string][]string{"INC001": {"cluster1"}},
			map[string]*ocm.ClusterInfo{
				"cluster1": {ID: "cluster1", Organization: "Fake Aeronautical Ltd"},
			},
		)
		assert.Empty(t, result["INC001"])
	})
}

func TestEvaluateFlags_NoOCMData(t *testing.T) {
	t.Run("gracefully returns no matches without OCM data", func(t *testing.T) {
		conditions := []FlagCondition{
			{ID: 1, Type: FlagOrgName, Pattern: "Anything", CreatedAt: time.Now()},
		}
		result := evaluateFlags(
			[]string{"INC001"},
			conditions,
			map[string][]string{"INC001": {"cluster1"}},
			map[string]*ocm.ClusterInfo{},
		)
		assert.Empty(t, result["INC001"])
	})
}

func TestEvaluateFlags_MultipleConditions(t *testing.T) {
	t.Run("returns multiple matching condition IDs", func(t *testing.T) {
		conditions := []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "cluster1", CreatedAt: time.Now()},
			{ID: 2, Type: FlagOrgName, Pattern: "Aeronautical", CreatedAt: time.Now()},
			{ID: 3, Type: FlagClusterID, Pattern: "nonexistent", CreatedAt: time.Now()},
		}
		result := evaluateFlags(
			[]string{"INC001"},
			conditions,
			map[string][]string{"INC001": {"cluster1"}},
			map[string]*ocm.ClusterInfo{
				"cluster1": {ID: "cluster1", Organization: "Fake Aeronautical Ltd"},
			},
		)
		assert.Equal(t, []int{1, 2}, result["INC001"])
	})
}

func TestEvaluateFlags_MultipleIncidents(t *testing.T) {
	t.Run("evaluates each incident independently", func(t *testing.T) {
		conditions := []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "cluster1", CreatedAt: time.Now()},
		}
		result := evaluateFlags(
			[]string{"INC001", "INC002"},
			conditions,
			map[string][]string{
				"INC001": {"cluster1"},
				"INC002": {"cluster2"},
			},
			map[string]*ocm.ClusterInfo{},
		)
		assert.Equal(t, []int{1}, result["INC001"])
		assert.Empty(t, result["INC002"])
	})
}

func TestEvaluateFlags_NoClusters(t *testing.T) {
	t.Run("incident with no clusters never matches", func(t *testing.T) {
		conditions := []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "cluster1", CreatedAt: time.Now()},
		}
		result := evaluateFlags(
			[]string{"INC001"},
			conditions,
			map[string][]string{},
			map[string]*ocm.ClusterInfo{},
		)
		assert.Empty(t, result["INC001"])
	})
}

func TestAddFlagConditionMsg(t *testing.T) {
	t.Run("adds condition and assigns ID", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = defaultFlagMarker
		m.incidentClusterMap = make(map[string][]string)
		m.clusterCache = make(map[string]*ocm.ClusterInfo)

		cond := FlagCondition{
			Type:      FlagClusterID,
			Pattern:   "cluster1",
			Label:     "cluster ID matches \"cluster1\"",
			CreatedAt: time.Now(),
		}

		result, _ := m.Update(addFlagConditionMsg{condition: cond})
		updated := result.(model)

		assert.Len(t, updated.flagConditions, 1)
		assert.Equal(t, 1, updated.flagConditions[0].ID)
		assert.Equal(t, "cluster1", updated.flagConditions[0].Pattern)
		assert.Equal(t, 1, updated.flagNextID)
	})
}

func TestAddFlagConditionMsg_IncrementsID(t *testing.T) {
	t.Run("auto-increments IDs", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = defaultFlagMarker
		m.flagNextID = 5
		m.flagConditions = []FlagCondition{
			{ID: 5, Type: FlagClusterID, Pattern: "existing"},
		}
		m.incidentClusterMap = make(map[string][]string)
		m.clusterCache = make(map[string]*ocm.ClusterInfo)

		cond := FlagCondition{Type: FlagOrgName, Pattern: "Acme", CreatedAt: time.Now()}
		result, _ := m.Update(addFlagConditionMsg{condition: cond})
		updated := result.(model)

		assert.Len(t, updated.flagConditions, 2)
		assert.Equal(t, 6, updated.flagConditions[1].ID)
		assert.Equal(t, 6, updated.flagNextID)
	})
}

func TestRemoveFlagConditionMsg(t *testing.T) {
	t.Run("removes condition by ID", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = defaultFlagMarker
		m.flagConditions = []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "cluster1"},
			{ID: 2, Type: FlagOrgName, Pattern: "Acme"},
			{ID: 3, Type: FlagClusterID, Pattern: "cluster3"},
		}
		m.incidentClusterMap = make(map[string][]string)
		m.clusterCache = make(map[string]*ocm.ClusterInfo)

		result, _ := m.Update(removeFlagConditionMsg{id: 2})
		updated := result.(model)

		assert.Len(t, updated.flagConditions, 2)
		assert.Equal(t, 1, updated.flagConditions[0].ID)
		assert.Equal(t, 3, updated.flagConditions[1].ID)
	})
}

func TestRemoveFlagConditionMsg_NotFound(t *testing.T) {
	t.Run("no-op for unknown ID", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = defaultFlagMarker
		m.flagConditions = []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "cluster1"},
		}
		m.incidentClusterMap = make(map[string][]string)
		m.clusterCache = make(map[string]*ocm.ClusterInfo)

		result, _ := m.Update(removeFlagConditionMsg{id: 99})
		updated := result.(model)

		assert.Len(t, updated.flagConditions, 1)
	})
}

func TestClearFlagConditionsMsg(t *testing.T) {
	t.Run("removes all conditions", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = defaultFlagMarker
		m.flagConditions = []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "a"},
			{ID: 2, Type: FlagOrgName, Pattern: "b"},
		}
		m.flagMatchCache = map[string][]int{"INC001": {1}}
		m.incidentClusterMap = make(map[string][]string)
		m.clusterCache = make(map[string]*ocm.ClusterInfo)

		result, _ := m.Update(clearFlagConditionsMsg{})
		updated := result.(model)

		assert.Empty(t, updated.flagConditions)
		assert.Nil(t, updated.flagMatchCache)
	})
}

func TestRebuildFlagMatchCache(t *testing.T) {
	t.Run("rebuilds cache from current state", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = defaultFlagMarker
		m.flagConditions = []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "cluster1"},
		}
		m.incidentList = []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "INC001"}},
		}
		m.incidentClusterMap = map[string][]string{
			"INC001": {"cluster1"},
		}
		m.clusterCache = make(map[string]*ocm.ClusterInfo)

		m.rebuildFlagMatchCache()

		assert.Equal(t, []int{1}, m.flagMatchCache["INC001"])
	})
}

func TestRebuildFlagMatchCache_NoConditions(t *testing.T) {
	t.Run("clears cache when no conditions", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = defaultFlagMarker
		m.flagConditions = nil
		m.flagMatchCache = map[string][]int{"INC001": {1}}

		m.rebuildFlagMatchCache()

		assert.Nil(t, m.flagMatchCache)
	})
}

func TestRenderFlagConditionsSection(t *testing.T) {
	t.Run("renders flag section for flagged incident", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = defaultFlagMarker
		m.flagConditions = []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "cluster1", Label: "cluster ID matches \"cluster1\""},
			{ID: 2, Type: FlagOrgName, Pattern: "Acme", Label: "org name matches \"Acme\""},
		}
		m.flagMatchCache = map[string][]int{"INC001": {1, 2}}
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "INC001"},
		}

		content := m.renderFlagConditionsSection()
		assert.Contains(t, content, "Flag Conditions")
		assert.Contains(t, content, "cluster ID matches")
		assert.Contains(t, content, "org name matches")
	})
}

func TestRenderFlagConditionsSection_NoFlags(t *testing.T) {
	t.Run("returns empty when incident not flagged", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = defaultFlagMarker
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "INC001"},
		}

		content := m.renderFlagConditionsSection()
		assert.Empty(t, content)
	})
}

func TestRenderFlagConditionsSection_NoIncident(t *testing.T) {
	t.Run("returns empty when no incident selected", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = defaultFlagMarker

		content := m.renderFlagConditionsSection()
		assert.Empty(t, content)
	})
}

func TestFormatFlagsList(t *testing.T) {
	t.Run("formats list of active flags", func(t *testing.T) {
		conditions := []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "cluster1", Label: "cluster ID matches \"cluster1\""},
			{ID: 2, Type: FlagOrgName, Pattern: "^Acme*", Label: "org name matches \"^Acme*\""},
		}

		content := formatFlagsList(conditions)
		assert.Contains(t, content, "#1")
		assert.Contains(t, content, "#2")
		assert.Contains(t, content, "cluster ID matches")
		assert.Contains(t, content, "org name matches")
	})
}

func TestFormatFlagsList_Empty(t *testing.T) {
	t.Run("shows no active flags message", func(t *testing.T) {
		content := formatFlagsList(nil)
		assert.Contains(t, content, "No active flag conditions")
	})
}
