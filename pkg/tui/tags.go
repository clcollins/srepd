package tui

import (
	"strings"

	"github.com/clcollins/srepd/pkg/pd"

	tea "github.com/charmbracelet/bubbletea"
)

const tagInputPrompt = "enter tags (comma-sep) > "

func ParseTags(input string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return []string{}
	}

	if strings.Contains(input, ",") {
		return parseCommaSeparated(input)
	}

	if isSingleBracketedValue(input) {
		tag := stripBrackets(input)
		if tag == "" {
			return []string{}
		}
		return []string{tag}
	}

	if isBracketDelimited(input) {
		return parseBracketDelimited(input)
	}

	tag := strings.TrimSpace(input)
	if tag == "" {
		return []string{}
	}
	return []string{tag}
}

func FormatTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	var b strings.Builder
	for _, tag := range tags {
		b.WriteByte('[')
		b.WriteString(tag)
		b.WriteByte(']')
	}
	return b.String()
}

func PrependTags(formattedTags string, existingTitle string) string {
	if formattedTags == "" {
		return existingTitle
	}

	newTags := parseBracketDelimited(formattedTags)
	existingTags := ExtractExistingTags(existingTitle)

	existingSet := make(map[string]struct{}, len(existingTags))
	for _, t := range existingTags {
		existingSet[strings.ToLower(t)] = struct{}{}
	}

	var novel []string
	for _, t := range newTags {
		if _, dup := existingSet[strings.ToLower(t)]; !dup {
			novel = append(novel, t)
		}
	}

	if len(novel) == 0 {
		return existingTitle
	}

	prefix := FormatTags(novel)
	if existingTitle == "" {
		return prefix
	}
	if existingTitle[0] == '[' {
		return prefix + existingTitle
	}
	return prefix + " " + existingTitle
}

func ExtractExistingTags(title string) []string {
	var tags []string
	i := 0
	for i < len(title) {
		for i < len(title) && title[i] == ' ' {
			i++
		}
		if i >= len(title) || title[i] != '[' {
			break
		}
		close := strings.IndexByte(title[i+1:], ']')
		if close < 0 {
			break
		}
		tag := title[i+1 : i+1+close]
		tags = append(tags, tag)
		i = i + 1 + close + 1
	}
	if len(tags) == 0 {
		return []string{}
	}
	return tags
}

func parseCommaSeparated(input string) []string {
	parts := strings.Split(input, ",")
	var tags []string
	for _, p := range parts {
		tag := stripBrackets(strings.TrimSpace(p))
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	if tags == nil {
		return []string{}
	}
	return tags
}

func isSingleBracketedValue(input string) bool {
	if len(input) < 2 || input[0] != '[' {
		return false
	}
	last := strings.LastIndexByte(input, ']')
	if last < 0 || last != len(input)-1 {
		return false
	}
	inner := input[1:last]
	return !strings.Contains(inner, "] [") && !strings.Contains(inner, "][")
}

func isBracketDelimited(input string) bool {
	return strings.Contains(input, "[") && strings.Contains(input, "]")
}

func parseBracketDelimited(input string) []string {
	var tags []string
	i := 0
	for i < len(input) {
		open := strings.IndexByte(input[i:], '[')
		if open < 0 {
			break
		}
		open += i
		close := strings.IndexByte(input[open+1:], ']')
		if close < 0 {
			break
		}
		tag := strings.TrimSpace(input[open+1 : open+1+close])
		if tag != "" {
			tags = append(tags, tag)
		}
		i = open + 1 + close + 1
	}
	if tags == nil {
		return []string{}
	}
	return tags
}

func stripBrackets(s string) string {
	for len(s) >= 2 && s[0] == '[' && s[len(s)-1] == ']' {
		inner := strings.TrimSpace(s[1 : len(s)-1])
		if inner == s {
			break
		}
		s = inner
	}
	return s
}

type updatedIncidentTitleMsg struct {
	incidentID string
	newTitle   string
	err        error
}

func updateIncidentTitle(config *pd.Config, incidentID string, newTitle string) tea.Cmd {
	return func() tea.Msg {
		_, err := pd.UpdateIncidentTitle(config.Client, incidentID, newTitle, config.CurrentUser)
		return updatedIncidentTitleMsg{
			incidentID: incidentID,
			newTitle:   newTitle,
			err:        err,
		}
	}
}
