package config

import "strings"

// ConfigHealth classifies whether a present config file is usable, fixable by
// the interactive wizard, or structurally broken beyond what the wizard can
// repair.
type ConfigHealth int

const (
	// HealthOK — required values are present and non-placeholder; start normally.
	HealthOK ConfigHealth = iota
	// HealthNeedsWizard — required values are missing or placeholders (e.g. a
	// config copied from the README example); route the user into the wizard.
	HealthNeedsWizard
	// HealthInvalid — the file is structurally malformed; the wizard cannot fix
	// it without risking data loss, so startup must abort with guidance.
	HealthInvalid
)

// HasPlaceholderToken reports whether token is empty/whitespace or an
// angle-bracket placeholder such as the README's "<PagerDuty API token>".
func HasPlaceholderToken(token string) bool {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return true
	}
	return isAnglePlaceholder(trimmed)
}

// ClassifyConfigHealth decides how startup should proceed when a config file
// exists. token and teams must come from viper's Get accessors (which resolve
// env vars and defaults), while settings is viper's AllSettings map, used only
// for structural checks. The returned reason is human-readable and suitable
// for logs and the wizard banner; it is empty for HealthOK.
func ClassifyConfigHealth(token string, teams []string, settings map[string]any) (ConfigHealth, string) {
	// Structural problems first: the wizard's merge path edits the YAML AST in
	// place, so a malformed document must be fixed by hand instead.
	if raw, ok := settings["service_escalation_policies"]; ok {
		if _, ok := raw.(map[string]any); !ok {
			return HealthInvalid, "'service_escalation_policies' is not a valid map"
		}
	}

	if HasPlaceholderToken(token) {
		if strings.TrimSpace(token) == "" {
			return HealthNeedsWizard, "no PagerDuty API token configured"
		}
		return HealthNeedsWizard, "the configured PagerDuty API token is a placeholder value"
	}

	if HasPlaceholderTeams(teams) {
		return HealthNeedsWizard, "no PagerDuty teams configured"
	}

	return HealthOK, ""
}

// isAnglePlaceholder reports whether s (already trimmed) is a template value
// like "<PagerDuty API token>" or "<team ID>".
func isAnglePlaceholder(s string) bool {
	return strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">")
}
