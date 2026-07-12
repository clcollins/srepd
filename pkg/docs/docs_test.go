package docs

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
)

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "standard title",
			content:  "# My Document\n\nSome content.",
			expected: "My Document",
		},
		{
			name:     "title with leading whitespace",
			content:  "#   Spaced Title  \n\nContent.",
			expected: "Spaced Title",
		},
		{
			name:     "no title",
			content:  "Just some text without a heading.",
			expected: "",
		},
		{
			name:     "multiple titles uses first",
			content:  "# First Title\n\n# Second Title\n",
			expected: "First Title",
		},
		{
			name:     "h2 not matched",
			content:  "## Not A Title\n\n# Real Title\n",
			expected: "Real Title",
		},
		{
			name:     "empty content",
			content:  "",
			expected: "",
		},
		{
			name:     "title is only line",
			content:  "# Solo",
			expected: "Solo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ExtractTitle(tt.content))
		})
	}
}

func TestTruncateTitle(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		maxLen   int
		expected string
	}{
		{
			name:     "short title no truncation",
			title:    "Hi",
			maxLen:   8,
			expected: "Hi",
		},
		{
			name:     "exactly max length",
			title:    "12345678",
			maxLen:   8,
			expected: "12345678",
		},
		{
			name:     "longer than max",
			title:    "Configuration Reference",
			maxLen:   8,
			expected: "Configur...",
		},
		{
			name:     "empty title",
			title:    "",
			maxLen:   8,
			expected: "",
		},
		{
			name:     "one over max",
			title:    "123456789",
			maxLen:   8,
			expected: "12345678...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, TruncateTitle(tt.title, tt.maxLen))
		})
	}
}

func TestBuildDocList(t *testing.T) {
	docsFS := fstest.MapFS{
		"colors.md":         {Data: []byte("# Color Palettes\n\nColors info.")},
		"ai-agents.md":      {Data: []byte("# AI Agents\n\nAgent info.")},
		"configuration.md":  {Data: []byte("# Configuration Reference\n\nConfig info.")},
		"quickstart.md":     {Data: []byte("# Quickstart\n\nDuplicate that should be excluded.")},
		"subdir/ignored.md": {Data: []byte("# Ignored\n\nShould not appear.")},
		"not-markdown.txt":  {Data: []byte("Not markdown")},
	}
	readme := "# SREPD\n\nA PagerDuty TUI."
	quickstart := "# Quickstart\n\nKey bindings."

	docs, err := BuildDocList(quickstart, readme, docsFS)
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(docs), 5, "should include quickstart + readme + 3 docs files")

	assert.Equal(t, "Quickstart", docs[0].Title, "first doc should be Quickstart")
	assert.Contains(t, docs[0].Content, "Key bindings")

	assert.Equal(t, "SREPD", docs[1].Title, "second doc should be README")
	assert.Contains(t, docs[1].Content, "PagerDuty TUI")

	remainingTitles := make([]string, 0)
	for _, d := range docs[2:] {
		remainingTitles = append(remainingTitles, d.Title)
	}
	assert.Equal(t, []string{"AI Agents", "Color Palettes", "Configuration Reference"}, remainingTitles,
		"remaining docs should be sorted alphabetically by title")

	for _, d := range docs {
		assert.NotEqual(t, "Ignored", d.Title, "should not include files from subdirectories")
	}
}

func TestBuildDocListEmptyFS(t *testing.T) {
	docsFS := fstest.MapFS{}
	readme := "# SREPD\n\nReadme content."
	quickstart := "# Quickstart\n\nQuickstart content."

	docs, err := BuildDocList(quickstart, readme, docsFS)
	assert.NoError(t, err)
	assert.Len(t, docs, 2, "should have quickstart and readme only")
	assert.Equal(t, "Quickstart", docs[0].Title)
	assert.Equal(t, "SREPD", docs[1].Title)
}

func TestBuildDocListNoTitle(t *testing.T) {
	docsFS := fstest.MapFS{
		"untitled.md": {Data: []byte("No heading here, just content.")},
	}
	readme := "# SREPD\n\nReadme."
	quickstart := "# Quickstart\n\nQuickstart."

	docs, err := BuildDocList(quickstart, readme, docsFS)
	assert.NoError(t, err)
	assert.Len(t, docs, 3)
	assert.Equal(t, "untitled.md", docs[2].Title, "untitled docs should use filename as title")
}
