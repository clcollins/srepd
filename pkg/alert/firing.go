package alert

import (
	"regexp"
	"strings"
)

// alertmanagerKVPattern matches " - key = value" lines from Alertmanager firing format.
var alertmanagerKVPattern = regexp.MustCompile(`^\s*-\s+(\S+)\s+=\s+(.+)$`)

// rhobsKVPattern matches "    key: value" lines from RHOBS YAML-like firing format.
var rhobsKVPattern = regexp.MustCompile(`^\s+-?\s*(\S+):\s+(.+)$`)

// ParseFiring auto-detects the firing field format and extracts all key-value pairs
// into a flat map. Handles two formats:
//   - Alertmanager format: starts with "Labels:" and uses " - key = value" lines
//   - RHOBS YAML format: uses "  - key: value" or "    key: value" lines
//
// Returns an empty (non-nil) map for empty or unrecognized input.
func ParseFiring(firing string) map[string]string {
	result := make(map[string]string)
	if strings.TrimSpace(firing) == "" {
		return result
	}

	lines := strings.Split(firing, "\n")

	// Detect format by looking at the content
	isAlertmanager := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Labels:" {
			isAlertmanager = true
			break
		}
	}

	if isAlertmanager {
		parseAlertmanagerFiring(lines, result)
	} else {
		parseRHOBSFiring(lines, result)
	}

	return result
}

// parseAlertmanagerFiring parses the Alertmanager " - key = value" format.
// Handles both Labels and Annotations sections.
func parseAlertmanagerFiring(lines []string, result map[string]string) {
	for _, line := range lines {
		// Skip section headers and Source: line
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "Labels:" || trimmed == "Annotations:" || strings.HasPrefix(trimmed, "Source:") {
			continue
		}

		matches := alertmanagerKVPattern.FindStringSubmatch(line)
		if matches != nil {
			key := strings.TrimSpace(matches[1])
			value := strings.TrimSpace(matches[2])
			result[key] = value
		}
	}
}

// parseRHOBSFiring parses the RHOBS YAML-like "key: value" format.
func parseRHOBSFiring(lines []string, result map[string]string) {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		matches := rhobsKVPattern.FindStringSubmatch(line)
		if matches != nil {
			key := strings.TrimSpace(matches[1])
			value := strings.TrimSpace(matches[2])
			result[key] = value
		}
	}
}
