package alert

import (
	"regexp"
	"strings"
)

// bracketPrefixPattern matches a leading "[...]" followed by optional whitespace.
var bracketPrefixPattern = regexp.MustCompile(`^\[([^\]]*)\]\s*`)

// osdHiveSuffix matches " SEVERITY (N)" at the end of osd_hive titles
var osdHiveSuffix = regexp.MustCompile(`\s+(CRITICAL|WARNING|Critical|Warning)\s+\(\d+\)$`)

// firingPrefix matches "[FIRING:N] " at the start
var firingPrefix = regexp.MustCompile(`^\[FIRING:\d+\]\s+`)

// rhobsHCPPattern matches "(Severity) AlertName for HCP: uuid"
var rhobsHCPPattern = regexp.MustCompile(`^\((?:Critical|Warning|Soaking)\)\s+(.+?)\s+for\s+HCP:\s+`)

// rhobsInfraPattern matches "(Severity) AlertName" (no "for HCP:")
var rhobsInfraPattern = regexp.MustCompile(`^\((?:Critical|Warning|Soaking)\)\s+(.+)$`)

// dmsPattern matches "cluster-url has gone missing"
var dmsPattern = regexp.MustCompile(`\shas\s+gone\s+missing$`)

// ceePattern matches "OHSS-NNNNN: description"
var ceePattern = regexp.MustCompile(`^(OHSS-\d+):`)

// appsreTrailing matches " - label dump (parenthetical)" after alert name
var appsreTrailing = regexp.MustCompile(`\s+-\s+.+$`)

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

// ExtractAlertName extracts just the alert name from an incident title,
// stripping bracket prefixes, severity suffixes, FIRING prefixes,
// "for HCP:" suffixes, and other per-type noise.
func ExtractAlertName(title string) string {
	if title == "" {
		return ""
	}

	// Check for FIRING prefix before stripping brackets (appsre type)
	if m := firingPrefix.FindString(title); m != "" {
		cleaned := strings.TrimPrefix(title, m)
		cleaned = appsreTrailing.ReplaceAllString(cleaned, "")
		return strings.TrimSpace(cleaned)
	}

	cleaned, _ := StripBracketPrefixes(title)

	// Dead Man's Snitch: "cluster has gone missing"
	if dmsPattern.MatchString(cleaned) {
		return "ClusterGoneMissing"
	}

	// CEE escalation: "OHSS-NNNNN: description"
	if m := ceePattern.FindStringSubmatch(cleaned); m != nil {
		return m[1]
	}

	// RHOBS HCP: "(Severity) AlertName for HCP: uuid"
	if m := rhobsHCPPattern.FindStringSubmatch(cleaned); m != nil {
		return strings.TrimSpace(m[1])
	}

	// RHOBS Infra: "(Severity) AlertName"
	if m := rhobsInfraPattern.FindStringSubmatch(cleaned); m != nil {
		return strings.TrimSpace(m[1])
	}

	// OSD Hive: "AlertName SEVERITY (N)"
	cleaned = osdHiveSuffix.ReplaceAllString(cleaned, "")

	return strings.TrimSpace(cleaned)
}
