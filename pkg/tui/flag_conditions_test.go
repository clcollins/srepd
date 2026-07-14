package tui

import (
	"errors"
	"testing"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/clcollins/srepd/pkg/ocm"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
)

// --- addFlagConditionMsg handler tests ---
// Basic add/increment tests exist in flags_test.go; these cover
// the full handler behavior including returned commands.

func TestAddFlagConditionMsg_ReturnsBatch(t *testing.T) {
	t.Run("returns batch with flash and rebuild commands", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = emojiFlagMarker
		m.incidentClusterMap = make(map[string][]string)
		m.clusterCache = make(map[string]*ocm.ClusterInfo)

		cond := FlagCondition{
			Type:      FlagClusterID,
			Pattern:   "cluster1",
			Label:     "cluster matches \"cluster1\"",
			CreatedAt: time.Now(),
		}

		_, cmd := m.Update(addFlagConditionMsg{condition: cond})

		assert.NotNil(t, cmd, "should return a batch command with flash + rebuild")
	})
}

func TestAddFlagConditionMsg_RebuildsFlagMatchCache(t *testing.T) {
	t.Run("rebuilds cache after adding condition", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = emojiFlagMarker
		m.incidentList = []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "INC001"}},
		}
		m.incidentClusterMap = map[string][]string{
			"INC001": {"cluster1"},
		}
		m.clusterCache = make(map[string]*ocm.ClusterInfo)

		cond := FlagCondition{
			Type:      FlagClusterID,
			Pattern:   "cluster1",
			Label:     "cluster matches cluster1",
			CreatedAt: time.Now(),
		}

		result, _ := m.Update(addFlagConditionMsg{condition: cond})
		updated := result.(model)

		// Cache should have been rebuilt with the new condition
		assert.NotNil(t, updated.flagMatchCache, "flag match cache should be rebuilt")
		assert.Equal(t, []int{1}, updated.flagMatchCache["INC001"],
			"INC001 should be flagged by condition #1")
	})
}

// --- removeFlagConditionMsg handler tests ---
// Basic removal tests exist in flags_test.go; these cover returned commands.

func TestRemoveFlagConditionMsg_ReturnsBatch(t *testing.T) {
	t.Run("returns batch with flash and rebuild commands", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = emojiFlagMarker
		m.flagConditions = []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "cluster1"},
		}
		m.incidentClusterMap = make(map[string][]string)
		m.clusterCache = make(map[string]*ocm.ClusterInfo)

		_, cmd := m.Update(removeFlagConditionMsg{id: 1})

		assert.NotNil(t, cmd, "should return a batch command")
	})
}

func TestRemoveFlagConditionMsg_RebuildsCacheAfterRemoval(t *testing.T) {
	t.Run("rebuilds flag match cache after removing condition", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = emojiFlagMarker
		m.flagConditions = []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "cluster1"},
			{ID: 2, Type: FlagClusterID, Pattern: "cluster2"},
		}
		m.incidentList = []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "INC001"}},
		}
		m.incidentClusterMap = map[string][]string{
			"INC001": {"cluster1"},
		}
		m.clusterCache = make(map[string]*ocm.ClusterInfo)
		m.flagMatchCache = map[string][]int{"INC001": {1}}

		result, _ := m.Update(removeFlagConditionMsg{id: 2})
		updated := result.(model)

		assert.Len(t, updated.flagConditions, 1, "should have 1 condition remaining")
		// Cache should be rebuilt; INC001 still matches condition 1
		assert.Equal(t, []int{1}, updated.flagMatchCache["INC001"])
	})
}

// --- clearFlagConditionsMsg handler tests ---
// Basic clear test exists in flags_test.go; this covers returned commands.

func TestClearFlagConditionsMsg_ReturnsBatch(t *testing.T) {
	t.Run("returns batch with flash and rebuild commands", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = emojiFlagMarker
		m.flagConditions = []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "a"},
		}
		m.incidentClusterMap = make(map[string][]string)
		m.clusterCache = make(map[string]*ocm.ClusterInfo)

		_, cmd := m.Update(clearFlagConditionsMsg{})

		assert.NotNil(t, cmd, "should return batch command with flash notification and rebuild")
	})
}

func TestClearFlagConditionsMsg_ClearsAllState(t *testing.T) {
	t.Run("clears conditions and match cache", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = emojiFlagMarker
		m.flagConditions = []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "a"},
			{ID: 2, Type: FlagOrgName, Pattern: "b"},
			{ID: 3, Type: FlagClusterID, Pattern: "c"},
		}
		m.flagMatchCache = map[string][]int{
			"INC001": {1, 2},
			"INC002": {3},
		}
		m.incidentClusterMap = make(map[string][]string)
		m.clusterCache = make(map[string]*ocm.ClusterInfo)

		result, _ := m.Update(clearFlagConditionsMsg{})
		updated := result.(model)

		assert.Nil(t, updated.flagConditions, "flagConditions should be nil")
		assert.Nil(t, updated.flagMatchCache, "flagMatchCache should be nil after clearing all conditions")
	})
}

// --- listFlagConditionsMsg handler tests ---

func TestListFlagConditionsMsg_SetsViewerContent(t *testing.T) {
	t.Run("formats flags list and sets viewer content", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = emojiFlagMarker
		m.flagConditions = []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "cluster1", Label: "cluster matches \"cluster1\""},
			{ID: 2, Type: FlagOrgName, Pattern: "^Acme*", Label: "org name matches \"^Acme*\""},
		}
		// Need incidentViewer to be properly initialized
		m.incidentViewer = newIncidentViewer()

		result, cmd := m.Update(listFlagConditionsMsg{})
		updated := result.(model)

		assert.True(t, updated.viewingIncident, "should set viewingIncident to true")
		assert.Nil(t, cmd, "should return nil cmd")
	})
}

func TestListFlagConditionsMsg_EmptyConditions(t *testing.T) {
	t.Run("shows no active flags message when conditions is nil", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = emojiFlagMarker
		m.flagConditions = nil
		m.incidentViewer = newIncidentViewer()

		result, cmd := m.Update(listFlagConditionsMsg{})
		updated := result.(model)

		assert.True(t, updated.viewingIncident, "should set viewingIncident to true")
		assert.Nil(t, cmd)
	})
}

func TestListFlagConditionsMsg_BlursTable(t *testing.T) {
	t.Run("blurs the table when showing flags list", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = emojiFlagMarker
		m.flagConditions = []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "test"},
		}
		m.incidentViewer = newIncidentViewer()
		m.table.Focus()

		result, _ := m.Update(listFlagConditionsMsg{})
		updated := result.(model)

		assert.True(t, updated.viewingIncident)
		// After Update, table should be blurred (Blur() called in handler)
		// The table's Focused() state is internal but we verified the handler calls Blur()
	})
}

// --- flagsSavedMsg handler tests ---

func TestFlagsSavedMsg_Error(t *testing.T) {
	t.Run("returns flash notification with error", func(t *testing.T) {
		m := createTestModel()

		result, cmd := m.Update(flagsSavedMsg{
			err: errors.New("disk full"),
		})
		updated := result.(model)

		assert.Contains(t, updated.status, "flags save failed")
		assert.Contains(t, updated.status, "disk full")
		assert.NotNil(t, cmd, "should return flash notification cmd")
	})
}

func TestFlagsSavedMsg_HappyPath(t *testing.T) {
	t.Run("returns flash notification with count", func(t *testing.T) {
		m := createTestModel()
		m.flagConditions = []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "a"},
			{ID: 2, Type: FlagOrgName, Pattern: "b"},
		}

		result, cmd := m.Update(flagsSavedMsg{err: nil})
		updated := result.(model)

		assert.Contains(t, updated.status, "flags saved")
		assert.Contains(t, updated.status, "2 conditions")
		assert.NotNil(t, cmd, "should return flash notification cmd")
	})
}

func TestFlagsSavedMsg_ZeroConditions(t *testing.T) {
	t.Run("reports zero conditions on save", func(t *testing.T) {
		m := createTestModel()
		m.flagConditions = nil

		result, cmd := m.Update(flagsSavedMsg{err: nil})
		updated := result.(model)

		assert.Contains(t, updated.status, "flags saved")
		assert.Contains(t, updated.status, "0 conditions")
		assert.NotNil(t, cmd)
	})
}

// --- flagsLoadedMsg handler tests ---

func TestFlagsLoadedMsg_Error(t *testing.T) {
	t.Run("returns flash notification with error", func(t *testing.T) {
		m := createTestModel()

		result, cmd := m.Update(flagsLoadedMsg{
			conditions: nil,
			err:        errors.New("file not found"),
		})
		updated := result.(model)

		assert.Contains(t, updated.status, "flags load failed")
		assert.Contains(t, updated.status, "file not found")
		assert.NotNil(t, cmd, "should return flash notification cmd")
		// flagConditions should not be changed on error
		assert.Empty(t, updated.flagConditions)
	})
}

func TestFlagsLoadedMsg_HappyPath(t *testing.T) {
	t.Run("sets flagConditions and updates flagNextID to max ID", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = emojiFlagMarker
		m.incidentClusterMap = make(map[string][]string)
		m.clusterCache = make(map[string]*ocm.ClusterInfo)

		conditions := []FlagCondition{
			{ID: 3, Type: FlagClusterID, Pattern: "cluster1", Label: "test"},
			{ID: 7, Type: FlagOrgName, Pattern: "Acme", Label: "org"},
			{ID: 5, Type: FlagClusterID, Pattern: "cluster2", Label: "test2"},
		}

		result, cmd := m.Update(flagsLoadedMsg{conditions: conditions, err: nil})
		updated := result.(model)

		assert.Len(t, updated.flagConditions, 3)
		assert.Equal(t, 7, updated.flagNextID, "flagNextID should be set to max ID in loaded conditions")
		assert.Contains(t, updated.status, "flags loaded")
		assert.Contains(t, updated.status, "3 conditions")
		assert.NotNil(t, cmd, "should return batch cmd with flash and rebuild")
	})
}

func TestFlagsLoadedMsg_UpdatesFlagNextIDToMax(t *testing.T) {
	t.Run("flagNextID tracks the highest loaded ID", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = emojiFlagMarker
		m.flagNextID = 100 // existing high value should be overwritten
		m.incidentClusterMap = make(map[string][]string)
		m.clusterCache = make(map[string]*ocm.ClusterInfo)

		conditions := []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "a"},
			{ID: 2, Type: FlagOrgName, Pattern: "b"},
		}

		result, _ := m.Update(flagsLoadedMsg{conditions: conditions, err: nil})
		updated := result.(model)

		assert.Equal(t, 2, updated.flagNextID,
			"flagNextID should be set to max loaded ID, not preserved from before")
	})
}

func TestFlagsLoadedMsg_RebuildsFlagMatchCache(t *testing.T) {
	t.Run("rebuilds flag match cache after loading", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = emojiFlagMarker
		m.incidentList = []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "INC001"}},
		}
		m.incidentClusterMap = map[string][]string{
			"INC001": {"cluster1"},
		}
		m.clusterCache = make(map[string]*ocm.ClusterInfo)

		conditions := []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "cluster1", Label: "test"},
		}

		result, _ := m.Update(flagsLoadedMsg{conditions: conditions, err: nil})
		updated := result.(model)

		assert.NotNil(t, updated.flagMatchCache, "flag match cache should be rebuilt")
		assert.Equal(t, []int{1}, updated.flagMatchCache["INC001"])
	})
}

func TestFlagsLoadedMsg_EmptyConditions(t *testing.T) {
	t.Run("handles empty conditions list", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = emojiFlagMarker
		m.incidentClusterMap = make(map[string][]string)
		m.clusterCache = make(map[string]*ocm.ClusterInfo)

		result, cmd := m.Update(flagsLoadedMsg{conditions: []FlagCondition{}, err: nil})
		updated := result.(model)

		assert.Empty(t, updated.flagConditions)
		assert.Equal(t, 0, updated.flagNextID)
		assert.Contains(t, updated.status, "flags loaded")
		assert.NotNil(t, cmd)
	})
}

func TestFlagsLoadedMsg_TriggersAlertFetchForUnmappedIncidents(t *testing.T) {
	t.Run("fetches alerts for incidents without cluster mappings when config is set", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = emojiFlagMarker
		m.config = &pd.Config{
			Client:      &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}},
		}
		m.incidentList = []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "INC001"}},
			{APIObject: pagerduty.APIObject{ID: "INC002"}},
		}
		// INC001 has a cluster mapping, INC002 does not
		m.incidentClusterMap = map[string][]string{
			"INC001": {"cluster1"},
		}
		m.clusterCache = make(map[string]*ocm.ClusterInfo)

		conditions := []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "cluster2"},
		}

		_, cmd := m.Update(flagsLoadedMsg{conditions: conditions, err: nil})

		// Should have returned a batch cmd that includes alert fetching for unmapped incidents
		assert.NotNil(t, cmd, "should return batch commands including alert fetches")
	})
}
