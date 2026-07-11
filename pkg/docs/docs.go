package docs

import (
	"embed"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
)

var embeddedReadme string
var embeddedQuickstart string
var embeddedDocsFS embed.FS
var embeddedDocsFSSet bool

func SetEmbeddedContent(readme, quickstart string, docsFS embed.FS) {
	embeddedReadme = readme
	embeddedQuickstart = quickstart
	embeddedDocsFS = docsFS
	embeddedDocsFSSet = true
}

func EmbeddedDocs() []Doc {
	if !embeddedDocsFSSet {
		return []Doc{
			{Title: "Quickstart", Content: "# Quickstart\n\nNo embedded docs available."},
			{Title: "SREPD", Content: "# SREPD\n\nNo embedded docs available."},
		}
	}
	subFS, err := fs.Sub(embeddedDocsFS, "docs")
	if err != nil {
		return []Doc{
			{Title: "Quickstart", Content: embeddedQuickstart},
			{Title: "SREPD", Content: embeddedReadme},
		}
	}
	result, _ := BuildDocList(embeddedQuickstart, embeddedReadme, subFS)
	return result
}

type Doc struct {
	Title   string
	Content string
}

func ExtractTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
		}
	}
	return ""
}

func TruncateTitle(title string, maxLen int) string {
	if title == "" {
		return ""
	}
	runes := []rune(title)
	if len(runes) <= maxLen {
		return title
	}
	return string(runes[:maxLen]) + "..."
}

func BuildDocList(quickstartContent, readmeContent string, docsFS fs.FS) ([]Doc, error) {
	var docs []Doc

	quickstartTitle := ExtractTitle(quickstartContent)
	if quickstartTitle == "" {
		quickstartTitle = "Quickstart"
	}
	docs = append(docs, Doc{Title: quickstartTitle, Content: quickstartContent})

	readmeTitle := ExtractTitle(readmeContent)
	if readmeTitle == "" {
		readmeTitle = "README"
	}
	docs = append(docs, Doc{Title: readmeTitle, Content: readmeContent})

	entries, err := fs.ReadDir(docsFS, ".")
	if err != nil {
		return docs, nil
	}

	var remaining []Doc
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		if entry.Name() == "quickstart.md" {
			continue
		}
		data, err := fs.ReadFile(docsFS, entry.Name())
		if err != nil {
			continue
		}
		content := string(data)
		title := ExtractTitle(content)
		if title == "" {
			title = entry.Name()
		}
		remaining = append(remaining, Doc{Title: title, Content: content})
	}

	slices.SortFunc(remaining, func(a, b Doc) int {
		return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
	})

	docs = append(docs, remaining...)
	return docs, nil
}
