package config

import (
	"strings"
)

// SalvageConfigValues does a best-effort line-by-line extraction of scalar
// key: value pairs from a config file that viper couldn't parse (e.g.
// malformed YAML). This lets the wizard pre-fill values — especially the
// token — that the user should never have to re-enter when they're right
// there in the file. It is deliberately simple: no YAML parsing (that
// already failed), just looking for lines that match "key: value" at the
// top indent level.
func SalvageConfigValues(data []byte) map[string]string {
	result := make(map[string]string)

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}

		idx := strings.Index(line, ":")
		if idx < 1 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		if key == "" || strings.HasPrefix(key, "#") {
			continue
		}

		value := strings.TrimSpace(line[idx+1:])

		// Strip inline comments (only when preceded by whitespace)
		if ci := strings.Index(value, " #"); ci >= 0 {
			value = strings.TrimSpace(value[:ci])
		}

		// Strip surrounding quotes
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		if value == "" {
			continue
		}

		// Skip map/sequence indicators — we only want scalars
		if value == "|" || value == ">" || strings.HasPrefix(value, "-") || strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") {
			continue
		}

		result[key] = value
	}

	return result
}
