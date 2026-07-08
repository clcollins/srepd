package tui

import (
	"testing"

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

func TestPickNextEnrichment_AllCached(t *testing.T) {
	m := model{
		table: makeTableWithRows([]string{"INC1", "INC2", "INC3"}),
		incidentCache: map[string]*cachedIncidentData{
			"INC1": {dataLoaded: true, notesLoaded: true, alertsLoaded: true},
			"INC2": {dataLoaded: true, notesLoaded: true, alertsLoaded: true},
			"INC3": {dataLoaded: true, notesLoaded: true, alertsLoaded: true},
		},
	}

	cmd := pickNextEnrichment(&m)
	assert.Nil(t, cmd, "should return nil when all incidents are cached")
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
		"INC2": {dataLoaded: true, notesLoaded: true, alertsLoaded: true},
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

	cache["INC3"] = &cachedIncidentData{dataLoaded: true, notesLoaded: true, alertsLoaded: true}
	cmd = pickNextEnrichment(&m)
	require.NotNil(t, cmd)
	msg = cmd()
	enrichMsg = msg.(getIncidentMsg)
	assert.Equal(t, "INC1", string(enrichMsg), "should enrich highlight-1 next")
}

func TestPickNextEnrichment_SkipsPartialCache(t *testing.T) {
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
	assert.Equal(t, "INC2", string(enrichMsg), "should skip partially cached incident (fetch already in progress)")
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
