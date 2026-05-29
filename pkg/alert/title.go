package alert

import (
	"regexp"
	"strings"
)

// bracketPrefixPattern matches a leading "[...]" followed by optional whitespace.
var bracketPrefixPattern = regexp.MustCompile(`^\[([^\]]*)\]\s*`)

// StripBracketPrefixes repeatedly removes leading [...] patterns from a title string.
// Returns the cleaned title and the list of extracted tag strings (trimmed of whitespace).
// SREs manually prepend tags like [SL Sent], [OHSS-52020], etc. to incident titles.
func StripBracketPrefixes(title string) (cleaned string, tags []string) {
	cleaned = title
	for {
		loc := bracketPrefixPattern.FindStringSubmatchIndex(cleaned)
		if loc == nil {
			break
		}
		// Extract the tag content (group 1) and trim whitespace
		tag := strings.TrimSpace(cleaned[loc[2]:loc[3]])
		tags = append(tags, tag)
		// Remove the matched prefix
		cleaned = cleaned[loc[1]:]
	}
	return cleaned, tags
}
