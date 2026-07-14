package tui

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTableWithRows(incidentIDs []string) table.Model {
	t := table.New(table.WithFocused(true))
	var rows []table.Row
	for _, id := range incidentIDs {
		rows = append(rows, table.Row{"A", id, "Title", "Service"})
	}
	t.SetColumns([]table.Column{
		{Title: "•", Width: 2},
		{Title: "ID", Width: 15},
		{Title: "Summary", Width: 40},
		{Title: "Service", Width: 30},
	})
	t.SetRows(rows)
	return t
}

// completeEntry returns a fully-loaded cache entry fetched at the given time.
func completeEntry(lastFetched time.Time) *cachedIncidentData {
	return &cachedIncidentData{
		dataLoaded:   true,
		notesLoaded:  true,
		alertsLoaded: true,
		lastFetched:  lastFetched,
	}
}

func TestNeedsEnrichment(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		entry    *cachedIncidentData
		expected bool
	}{
		{"nil entry needs enrichment", nil, true},
		{"missing details needs enrichment", &cachedIncidentData{notesLoaded: true, alertsLoaded: true, lastFetched: now}, true},
		{"missing notes needs enrichment", &cachedIncidentData{dataLoaded: true, alertsLoaded: true, lastFetched: now}, true},
		{"missing alerts needs enrichment", &cachedIncidentData{dataLoaded: true, notesLoaded: true, lastFetched: now}, true},
		{"complete and fresh does not need enrichment", completeEntry(now), false},
		{"complete but stale needs enrichment", completeEntry(now.Add(-incidentCacheTTL - time.Minute)), true},
		{"complete with zero lastFetched needs enrichment", completeEntry(time.Time{}), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, needsEnrichment(tt.entry, now))
		})
	}
}

func TestPickNextEnrichment_AllCachedAndFresh(t *testing.T) {
	now := time.Now()
	m := model{
		table:  makeTableWithRows([]string{"INC1", "INC2", "INC3"}),
		config: &pd.Config{Client: &pd.MockPagerDutyClient{}},
		incidentCache: map[string]*cachedIncidentData{
			"INC1": completeEntry(now),
			"INC2": completeEntry(now),
			"INC3": completeEntry(now),
		},
	}

	cmd := pickNextEnrichment(&m)
	assert.Nil(t, cmd, "should return nil when all incidents are cached and fresh")
}

func TestPickNextEnrichment_PicksStaleIncident(t *testing.T) {
	m := model{
		table:  makeTableWithRows([]string{"INC1"}),
		config: &pd.Config{Client: &pd.MockPagerDutyClient{}},
		incidentCache: map[string]*cachedIncidentData{
			"INC1": completeEntry(time.Now().Add(-incidentCacheTTL - time.Minute)),
		},
	}

	cmd := pickNextEnrichment(&m)
	require.NotNil(t, cmd, "a stale incident should be re-enriched")
	assert.Equal(t, "INC1", string(cmd().(getIncidentMsg)))
}

func TestPickNextEnrichment_RespectsDispatchCooldown(t *testing.T) {
	stale := completeEntry(time.Now().Add(-incidentCacheTTL - time.Minute))

	t.Run("recently dispatched incident is skipped", func(t *testing.T) {
		m := model{
			table:         makeTableWithRows([]string{"INC1", "INC2"}),
			config:        &pd.Config{Client: &pd.MockPagerDutyClient{}},
			incidentCache: map[string]*cachedIncidentData{"INC1": stale},
			enrichDispatchedAt: map[string]time.Time{
				"INC1": time.Now(),
			},
		}

		cmd := pickNextEnrichment(&m)
		require.NotNil(t, cmd)
		assert.Equal(t, "INC2", string(cmd().(getIncidentMsg)),
			"a stale incident dispatched within the cooldown must not be re-dispatched")
	})

	t.Run("incident past the cooldown is eligible again", func(t *testing.T) {
		m := model{
			table:         makeTableWithRows([]string{"INC1"}),
			config:        &pd.Config{Client: &pd.MockPagerDutyClient{}},
			incidentCache: map[string]*cachedIncidentData{"INC1": stale},
			enrichDispatchedAt: map[string]time.Time{
				"INC1": time.Now().Add(-enrichDispatchCooldown - time.Second),
			},
		}

		cmd := pickNextEnrichment(&m)
		require.NotNil(t, cmd)
		assert.Equal(t, "INC1", string(cmd().(getIncidentMsg)))
	})
}

func TestPickNextEnrichment_RecordsDispatch(t *testing.T) {
	m := model{
		table:         makeTableWithRows([]string{"INC1"}),
		config:        &pd.Config{Client: &pd.MockPagerDutyClient{}},
		incidentCache: make(map[string]*cachedIncidentData),
	}

	cmd := pickNextEnrichment(&m)
	require.NotNil(t, cmd)
	assert.WithinDuration(t, time.Now(), m.enrichDispatchedAt["INC1"], time.Second,
		"a dispatch must be recorded for the picked incident")

	// A permanently failing fetch never writes the cache; the recorded
	// dispatch must prevent an immediate re-pick storm
	assert.Nil(t, pickNextEnrichment(&m), "the same incident must not be re-picked within the cooldown")
}

func TestPickNextEnrichment_HighlightedFirst(t *testing.T) {
	tbl := makeTableWithRows([]string{"INC1", "INC2", "INC3"})
	tbl.SetCursor(1)

	m := model{
		table:         tbl,
		incidentCache: make(map[string]*cachedIncidentData),
		config:        &pd.Config{Client: &pd.MockPagerDutyClient{}},
	}

	cmd := pickNextEnrichment(&m)
	require.NotNil(t, cmd, "should return a command")

	msg := cmd()
	enrichMsg, ok := msg.(getIncidentMsg)
	require.True(t, ok, "should return getIncidentMsg")
	assert.Equal(t, "INC2", string(enrichMsg), "should enrich the highlighted incident first")
}

func TestPickNextEnrichment_SpiralOrder(t *testing.T) {
	tbl := makeTableWithRows([]string{"INC0", "INC1", "INC2", "INC3", "INC4"})
	tbl.SetCursor(2)

	cache := map[string]*cachedIncidentData{
		"INC2": completeEntry(time.Now()),
	}

	m := model{
		table:         tbl,
		incidentCache: cache,
		config:        &pd.Config{Client: &pd.MockPagerDutyClient{}},
	}

	cmd := pickNextEnrichment(&m)
	require.NotNil(t, cmd)
	msg := cmd()
	enrichMsg := msg.(getIncidentMsg)
	assert.Equal(t, "INC3", string(enrichMsg), "should enrich highlight+1 after highlight is cached")

	cache["INC3"] = completeEntry(time.Now())
	cmd = pickNextEnrichment(&m)
	require.NotNil(t, cmd)
	msg = cmd()
	enrichMsg = msg.(getIncidentMsg)
	assert.Equal(t, "INC1", string(enrichMsg), "should enrich highlight-1 next")
}

func TestPickNextEnrichment_PicksIncompleteCache(t *testing.T) {
	// A partial entry used to mean "fetch already in progress" and was
	// skipped forever if the missing fetches failed; incomplete entries are
	// now eligible, with the dispatch cooldown preventing re-pick storms
	tbl := makeTableWithRows([]string{"INC1", "INC2"})
	tbl.SetCursor(0)

	m := model{
		table: tbl,
		incidentCache: map[string]*cachedIncidentData{
			"INC1": {dataLoaded: true, notesLoaded: false, alertsLoaded: false},
		},
		config: &pd.Config{Client: &pd.MockPagerDutyClient{}},
	}

	cmd := pickNextEnrichment(&m)
	require.NotNil(t, cmd)
	msg := cmd()
	enrichMsg := msg.(getIncidentMsg)
	assert.Equal(t, "INC1", string(enrichMsg), "an incomplete cache entry should be re-enriched")
}

func TestPickNextEnrichment_EmptyTable(t *testing.T) {
	m := model{
		table:         makeTableWithRows(nil),
		incidentCache: make(map[string]*cachedIncidentData),
	}

	cmd := pickNextEnrichment(&m)
	assert.Nil(t, cmd, "should return nil for empty table")
}

func TestPickNextEnrichment_NoConfig(t *testing.T) {
	m := model{
		table:         makeTableWithRows([]string{"INC1"}),
		incidentCache: make(map[string]*cachedIncidentData),
		config:        nil,
	}

	cmd := pickNextEnrichment(&m)
	assert.Nil(t, cmd, "should return nil when config is nil")
}

func TestLazyEnrichMsg_TypeExists(t *testing.T) {
	_ = lazyEnrichMsg{}
}
