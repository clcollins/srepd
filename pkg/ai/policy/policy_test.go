package policy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecide_Exhaustive(t *testing.T) {
	noInput := json.RawMessage(`{}`)

	tests := []struct {
		name     string
		cfg      Config
		toolName string
		class    Class
		want     Decision
	}{
		// ModePlan: ClassRead → Allow, everything else → Deny
		{"Plan/ClassRead", Config{Mode: ModePlan}, "get_incident", ClassRead, Allow},
		{"Plan/ClassWriteLocal", Config{Mode: ModePlan}, "add_note", ClassWriteLocal, Deny},
		{"Plan/ClassExec", Config{Mode: ModePlan}, "run_cmd", ClassExec, Deny},
		{"Plan/ClassExternal", Config{Mode: ModePlan}, "fetch_url", ClassExternal, Deny},

		// ModeInteractive: ClassRead → Allow, everything else → Ask
		{"Interactive/ClassRead", Config{Mode: ModeInteractive}, "get_incident", ClassRead, Allow},
		{"Interactive/ClassWriteLocal", Config{Mode: ModeInteractive}, "add_note", ClassWriteLocal, Ask},
		{"Interactive/ClassExec", Config{Mode: ModeInteractive}, "run_cmd", ClassExec, Ask},
		{"Interactive/ClassExternal", Config{Mode: ModeInteractive}, "fetch_url", ClassExternal, Ask},

		// ModeAuto: ClassRead → Allow, allowlisted → Allow, others → Ask
		{"Auto/ClassRead", Config{Mode: ModeAuto}, "get_incident", ClassRead, Allow},
		{"Auto/ClassWriteLocal/NotListed", Config{Mode: ModeAuto}, "add_note", ClassWriteLocal, Ask},
		{"Auto/ClassWriteLocal/Listed", Config{Mode: ModeAuto, AutoAllowTools: []string{"add_note"}}, "add_note", ClassWriteLocal, Allow},
		{"Auto/ClassExec/NotListed", Config{Mode: ModeAuto}, "run_cmd", ClassExec, Ask},
		{"Auto/ClassExec/Listed", Config{Mode: ModeAuto, AutoAllowTools: []string{"run_cmd"}}, "run_cmd", ClassExec, Allow},
		{"Auto/AllowlistMiss", Config{Mode: ModeAuto, AutoAllowTools: []string{"other_tool"}}, "add_note", ClassWriteLocal, Ask},

		// ModeCustom: same as ModeAuto with AutoAllowTools
		{"Custom/ClassRead", Config{Mode: ModeCustom}, "get_incident", ClassRead, Allow},
		{"Custom/NotListed", Config{Mode: ModeCustom}, "add_note", ClassWriteLocal, Ask},
		{"Custom/Listed", Config{Mode: ModeCustom, AutoAllowTools: []string{"add_note"}}, "add_note", ClassWriteLocal, Allow},

		// Edge cases
		{"EmptyToolName", Config{Mode: ModeInteractive}, "", ClassRead, Deny},
		{"UnknownMode", Config{Mode: Mode(99)}, "get_incident", ClassRead, Deny},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide(tt.cfg, tt.toolName, tt.class, noInput)
			assert.Equal(t, tt.want, got,
				"Decide(%+v, %q, class=%d) = %d, want %d",
				tt.cfg.Mode, tt.toolName, tt.class, got, tt.want)
		})
	}
}

func TestDecide_AllowlistMultipleTools(t *testing.T) {
	cfg := Config{
		Mode:           ModeAuto,
		AutoAllowTools: []string{"add_note", "run_cmd"},
	}
	assert.Equal(t, Allow, Decide(cfg, "add_note", ClassWriteLocal, nil))
	assert.Equal(t, Allow, Decide(cfg, "run_cmd", ClassExec, nil))
	assert.Equal(t, Ask, Decide(cfg, "fetch_url", ClassExternal, nil))
}

func TestDecide_ClassReadAllowedInEveryMode(t *testing.T) {
	modes := []Mode{ModePlan, ModeInteractive, ModeAuto, ModeCustom}
	for _, mode := range modes {
		cfg := Config{Mode: mode}
		got := Decide(cfg, "get_incident", ClassRead, nil)
		assert.Equal(t, Allow, got, "ClassRead must be Allow in mode %d", mode)
	}
}
