package tui

import (
	"testing"

	"github.com/clcollins/srepd/pkg/docs"
	"github.com/stretchr/testify/assert"
)

func TestDocsTabPage(t *testing.T) {
	tests := []struct {
		name        string
		activeTab   int
		tabsPerPage int
		expected    int
	}{
		{"first page start", 0, 8, 0},
		{"first page end", 7, 8, 0},
		{"second page start", 8, 8, 1},
		{"second page middle", 10, 8, 1},
		{"third page", 16, 8, 2},
		{"small pages", 3, 2, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, docsTabPage(tt.activeTab, tt.tabsPerPage))
		})
	}
}

func TestDocsPageStartEnd(t *testing.T) {
	tests := []struct {
		name      string
		activeTab int
		perPage   int
		total     int
		wantStart int
		wantEnd   int
	}{
		{"first page full", 0, 8, 16, 0, 8},
		{"second page full", 8, 8, 16, 8, 16},
		{"partial last page", 8, 8, 10, 8, 10},
		{"single page", 0, 8, 5, 0, 5},
		{"exact fit", 0, 8, 8, 0, 8},
		{"small page size", 2, 3, 7, 0, 3},
		{"last small page", 6, 3, 7, 6, 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := docsPageStartEnd(tt.activeTab, tt.perPage, tt.total)
			assert.Equal(t, tt.wantStart, start, "start index")
			assert.Equal(t, tt.wantEnd, end, "end index")
		})
	}
}

func TestDocsTabLabel(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected string
	}{
		{"short title", "Colors", "Colors"},
		{"exactly 8", "12345678", "12345678"},
		{"long title", "Configuration Reference", "Configur..."},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, docsTabLabel(tt.title))
		})
	}
}

func TestDocsNextTab(t *testing.T) {
	tests := []struct {
		name     string
		current  int
		total    int
		expected int
	}{
		{"advance within page", 0, 10, 1},
		{"wrap at end", 9, 10, 0},
		{"single tab", 0, 1, 0},
		{"cross page boundary", 7, 16, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, docsNextTab(tt.current, tt.total))
		})
	}
}

func TestDocsPrevTab(t *testing.T) {
	tests := []struct {
		name     string
		current  int
		total    int
		expected int
	}{
		{"go back within page", 5, 10, 4},
		{"wrap at start", 0, 10, 9},
		{"single tab", 0, 1, 0},
		{"cross page boundary back", 8, 16, 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, docsPrevTab(tt.current, tt.total))
		})
	}
}

func TestDocsPagingIndicator(t *testing.T) {
	tests := []struct {
		name      string
		activeTab int
		perPage   int
		total     int
		expected  string
	}{
		{"single page", 0, 8, 5, ""},
		{"first of two pages", 0, 8, 16, " [1/2]"},
		{"second of two pages", 8, 8, 16, " [2/2]"},
		{"middle page", 8, 8, 24, " [2/3]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, docsPagingIndicator(tt.activeTab, tt.perPage, tt.total))
		})
	}
}

func TestClearDocsView(t *testing.T) {
	m := model{
		viewingDocs:   true,
		docsActiveTab: 5,
	}
	m.clearDocsView()
	assert.False(t, m.viewingDocs)
	assert.Equal(t, 0, m.docsActiveTab)
}

func TestBuildDocsPageLabels(t *testing.T) {
	pages := []docs.Doc{
		{Title: "Quickstart", Content: ""},
		{Title: "SREPD", Content: ""},
		{Title: "AI Agents", Content: ""},
		{Title: "Color Palettes", Content: ""},
		{Title: "Configuration Reference", Content: ""},
		{Title: "Flag Conditions", Content: ""},
		{Title: "LLM Provider Configuration", Content: ""},
		{Title: "Escalation Policies", Content: ""},
		{Title: "Extra Doc One", Content: ""},
		{Title: "Extra Doc Two", Content: ""},
	}

	labels := buildDocsPageLabels(pages, 0, 8)
	assert.Len(t, labels, 8, "should show 8 tabs on first page")
	assert.Equal(t, "Quicksta...", labels[0])
	assert.Equal(t, "SREPD", labels[1])

	labels2 := buildDocsPageLabels(pages, 8, 8)
	assert.Len(t, labels2, 2, "second page should show remaining 2 tabs")
	assert.Equal(t, "Extra Do...", labels2[0])
}
