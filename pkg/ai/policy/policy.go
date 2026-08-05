package policy

import "encoding/json"

// Class categorizes a tool's risk level (OpenWorker's taxonomy).
type Class int

const (
	ClassRead       Class = iota // Read-only data access
	ClassWriteLocal              // Writes to local state (notes, ack)
	ClassExec                    // Executes a command
	ClassExternal                // Reaches external systems (URL fetch)
)

// Mode controls the policy engine's behavior.
type Mode int

const (
	ModePlan        Mode = iota // Read-only: everything else denied
	ModeInteractive             // Reads allowed; others require approval
	ModeAuto                    // Allow per allowlists
	ModeCustom                  // User-defined via AutoAllowTools
)

// Decision is the outcome of a policy check.
type Decision int

const (
	Allow Decision = iota
	Deny
	Ask
)

// Config holds policy engine configuration.
type Config struct {
	Mode                   Mode
	AutoAllowTools         []string
	AllowedCommandPrefixes []string
}

// Decide evaluates a tool call against the policy configuration.
// Pure — no I/O.
func Decide(cfg Config, toolName string, class Class, _ json.RawMessage) Decision {
	if toolName == "" {
		return Deny
	}

	switch cfg.Mode {
	case ModePlan:
		if class == ClassRead {
			return Allow
		}
		return Deny

	case ModeInteractive:
		if class == ClassRead {
			return Allow
		}
		return Ask

	case ModeAuto:
		if class == ClassRead {
			return Allow
		}
		if isInList(toolName, cfg.AutoAllowTools) {
			return Allow
		}
		return Ask

	// ModeCustom is intentionally identical to ModeAuto in this phase.
	// Plan 415 will differentiate it with AllowedCommandPrefixes support.
	case ModeCustom:
		if class == ClassRead {
			return Allow
		}
		if isInList(toolName, cfg.AutoAllowTools) {
			return Allow
		}
		return Ask

	default:
		return Deny
	}
}

func isInList(name string, list []string) bool {
	for _, item := range list {
		if item == name {
			return true
		}
	}
	return false
}
