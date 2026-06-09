package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFlagCommand_ClusterID(t *testing.T) {
	t.Run("parses /flag cluster <id>", func(t *testing.T) {
		cmd, err := parseFlagCommand("/flag cluster 1q2w3e4rfakeidtest9o0p1a2s3d4f5g")

		require.NoError(t, err)
		require.NotNil(t, cmd)
		assert.Equal(t, flagCmdAdd, cmd.action)
		assert.Equal(t, FlagClusterID, cmd.condition.Type)
		assert.Equal(t, "1q2w3e4rfakeidtest9o0p1a2s3d4f5g", cmd.condition.Pattern)
		assert.Contains(t, cmd.condition.Label, "cluster")
	})
}

func TestParseFlagCommand_OrgName(t *testing.T) {
	t.Run("parses /flag org <pattern>", func(t *testing.T) {
		cmd, err := parseFlagCommand("/flag org ^Acme*")

		require.NoError(t, err)
		require.NotNil(t, cmd)
		assert.Equal(t, flagCmdAdd, cmd.action)
		assert.Equal(t, FlagOrgName, cmd.condition.Type)
		assert.Equal(t, "^Acme*", cmd.condition.Pattern)
		assert.Contains(t, cmd.condition.Label, "org")
	})
}

func TestParseFlagCommand_OrgNameMultiWord(t *testing.T) {
	t.Run("parses org pattern with spaces", func(t *testing.T) {
		cmd, err := parseFlagCommand("/flag org Red Hat Inc")

		require.NoError(t, err)
		assert.Equal(t, FlagOrgName, cmd.condition.Type)
		assert.Equal(t, "Red Hat Inc", cmd.condition.Pattern)
	})
}

func TestParseFlagCommand_List(t *testing.T) {
	t.Run("parses /flags as list command", func(t *testing.T) {
		cmd, err := parseFlagCommand("/flags")

		require.NoError(t, err)
		require.NotNil(t, cmd)
		assert.Equal(t, flagCmdList, cmd.action)
	})
}

func TestParseFlagCommand_Unflag(t *testing.T) {
	t.Run("parses /unflag <id>", func(t *testing.T) {
		cmd, err := parseFlagCommand("/unflag 3")

		require.NoError(t, err)
		require.NotNil(t, cmd)
		assert.Equal(t, flagCmdRemove, cmd.action)
		assert.Equal(t, 3, cmd.targetID)
	})
}

func TestParseFlagCommand_UnflagAll(t *testing.T) {
	t.Run("parses /unflag all", func(t *testing.T) {
		cmd, err := parseFlagCommand("/unflag all")

		require.NoError(t, err)
		require.NotNil(t, cmd)
		assert.Equal(t, flagCmdClearAll, cmd.action)
	})
}

func TestParseFlagCommand_FlagsSave(t *testing.T) {
	t.Run("parses /flags save", func(t *testing.T) {
		cmd, err := parseFlagCommand("/flags save")

		require.NoError(t, err)
		require.NotNil(t, cmd)
		assert.Equal(t, flagCmdSave, cmd.action)
		assert.Empty(t, cmd.path)
	})
}

func TestParseFlagCommand_FlagsSavePath(t *testing.T) {
	t.Run("parses /flags save <path>", func(t *testing.T) {
		cmd, err := parseFlagCommand("/flags save /tmp/myflags.json")

		require.NoError(t, err)
		require.NotNil(t, cmd)
		assert.Equal(t, flagCmdSave, cmd.action)
		assert.Equal(t, "/tmp/myflags.json", cmd.path)
	})
}

func TestParseFlagCommand_FlagsLoad(t *testing.T) {
	t.Run("parses /flags load", func(t *testing.T) {
		cmd, err := parseFlagCommand("/flags load")

		require.NoError(t, err)
		require.NotNil(t, cmd)
		assert.Equal(t, flagCmdLoad, cmd.action)
	})
}

func TestParseFlagCommand_FlagsLoadPath(t *testing.T) {
	t.Run("parses /flags load <path>", func(t *testing.T) {
		cmd, err := parseFlagCommand("/flags load /tmp/myflags.json")

		require.NoError(t, err)
		require.NotNil(t, cmd)
		assert.Equal(t, flagCmdLoad, cmd.action)
		assert.Equal(t, "/tmp/myflags.json", cmd.path)
	})
}

func TestParseFlagCommand_InvalidType(t *testing.T) {
	t.Run("returns error for unknown flag type", func(t *testing.T) {
		_, err := parseFlagCommand("/flag badtype value")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown flag type")
	})
}

func TestParseFlagCommand_MissingValue(t *testing.T) {
	t.Run("returns error for /flag cluster with no value", func(t *testing.T) {
		_, err := parseFlagCommand("/flag cluster")

		assert.Error(t, err)
	})
}

func TestParseFlagCommand_MissingOrgValue(t *testing.T) {
	t.Run("returns error for /flag org with no value", func(t *testing.T) {
		_, err := parseFlagCommand("/flag org")

		assert.Error(t, err)
	})
}

func TestParseFlagCommand_FlagNoArgs(t *testing.T) {
	t.Run("returns error for bare /flag", func(t *testing.T) {
		_, err := parseFlagCommand("/flag")

		assert.Error(t, err)
	})
}

func TestParseFlagCommand_UnflagInvalidID(t *testing.T) {
	t.Run("returns error for /unflag with non-numeric id", func(t *testing.T) {
		_, err := parseFlagCommand("/unflag abc")

		assert.Error(t, err)
	})
}

func TestParseFlagCommand_UnflagNoArgs(t *testing.T) {
	t.Run("returns error for bare /unflag", func(t *testing.T) {
		_, err := parseFlagCommand("/unflag")

		assert.Error(t, err)
	})
}

func TestSaveFlagsCmd_WritesJSON(t *testing.T) {
	t.Run("saves conditions to JSON file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "flags.json")
		conditions := []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "cluster1", Label: "test flag", CreatedAt: time.Now()},
			{ID: 2, Type: FlagOrgName, Pattern: "^Acme*", Label: "org flag", CreatedAt: time.Now()},
		}

		cmd := saveFlagsCmd(conditions, path)
		msg := cmd()

		saved, ok := msg.(flagsSavedMsg)
		require.True(t, ok)
		assert.NoError(t, saved.err)

		data, err := os.ReadFile(path)
		require.NoError(t, err)

		var loaded []FlagCondition
		require.NoError(t, json.Unmarshal(data, &loaded))
		assert.Len(t, loaded, 2)
		assert.Equal(t, "cluster1", loaded[0].Pattern)
		assert.Equal(t, "^Acme*", loaded[1].Pattern)
	})
}

func TestSaveFlagsCmd_CreatesDirectory(t *testing.T) {
	t.Run("creates parent directory if needed", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "subdir", "nested", "flags.json")

		cmd := saveFlagsCmd([]FlagCondition{{ID: 1}}, path)
		msg := cmd()

		saved := msg.(flagsSavedMsg)
		assert.NoError(t, saved.err)
		assert.FileExists(t, path)
	})
}

func TestSaveFlagsCmd_InvalidPath(t *testing.T) {
	t.Run("returns error for invalid path", func(t *testing.T) {
		cmd := saveFlagsCmd([]FlagCondition{{ID: 1}}, "/dev/null/impossible/flags.json")
		msg := cmd()

		saved := msg.(flagsSavedMsg)
		assert.Error(t, saved.err)
	})
}

func TestLoadFlagsCmd_ReadsJSON(t *testing.T) {
	t.Run("loads conditions from JSON file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "flags.json")
		conditions := []FlagCondition{
			{ID: 1, Type: FlagClusterID, Pattern: "cluster1", Label: "test"},
			{ID: 2, Type: FlagOrgName, Pattern: "Acme", Label: "org"},
		}
		data, _ := json.MarshalIndent(conditions, "", "  ")
		require.NoError(t, os.WriteFile(path, data, 0644))

		cmd := loadFlagsCmd(path)
		msg := cmd()

		loaded, ok := msg.(flagsLoadedMsg)
		require.True(t, ok)
		assert.NoError(t, loaded.err)
		assert.Len(t, loaded.conditions, 2)
		assert.Equal(t, "cluster1", loaded.conditions[0].Pattern)
	})
}

func TestLoadFlagsCmd_FileNotFound(t *testing.T) {
	t.Run("returns error for missing file", func(t *testing.T) {
		cmd := loadFlagsCmd("/nonexistent/flags.json")
		msg := cmd()

		loaded := msg.(flagsLoadedMsg)
		assert.Error(t, loaded.err)
	})
}

func TestLoadFlagsCmd_InvalidJSON(t *testing.T) {
	t.Run("returns error for malformed JSON", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "flags.json")
		require.NoError(t, os.WriteFile(path, []byte("{not valid json}"), 0644))

		cmd := loadFlagsCmd(path)
		msg := cmd()

		loaded := msg.(flagsLoadedMsg)
		assert.Error(t, loaded.err)
	})
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Run("save and load preserves all fields", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "flags.json")
		now := time.Now().Truncate(time.Second)
		original := []FlagCondition{
			{ID: 5, Type: FlagClusterID, Pattern: "abc123", Label: "cluster flag", CreatedAt: now},
			{ID: 8, Type: FlagOrgName, Pattern: "^Red Hat*", Label: "org flag", CreatedAt: now},
		}

		saveMsg := saveFlagsCmd(original, path)().(flagsSavedMsg)
		require.NoError(t, saveMsg.err)

		loadMsg := loadFlagsCmd(path)().(flagsLoadedMsg)
		require.NoError(t, loadMsg.err)
		require.Len(t, loadMsg.conditions, 2)

		assert.Equal(t, 5, loadMsg.conditions[0].ID)
		assert.Equal(t, FlagClusterID, loadMsg.conditions[0].Type)
		assert.Equal(t, "abc123", loadMsg.conditions[0].Pattern)
		assert.Equal(t, "cluster flag", loadMsg.conditions[0].Label)

		assert.Equal(t, 8, loadMsg.conditions[1].ID)
		assert.Equal(t, FlagOrgName, loadMsg.conditions[1].Type)
		assert.Equal(t, "^Red Hat*", loadMsg.conditions[1].Pattern)
	})
}

func TestDispatchFlagCommand_Add(t *testing.T) {
	t.Run("dispatches addFlagConditionMsg for /flag cluster", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = defaultFlagMarker

		cmd := m.dispatchFlagCommand("/flag cluster abc123")
		require.NotNil(t, cmd)

		msg := cmd()
		addMsg, ok := msg.(addFlagConditionMsg)
		assert.True(t, ok)
		assert.Equal(t, FlagClusterID, addMsg.condition.Type)
		assert.Equal(t, "abc123", addMsg.condition.Pattern)
	})
}

func TestDispatchFlagCommand_List(t *testing.T) {
	t.Run("dispatches listFlagConditionsMsg for /flags", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = defaultFlagMarker

		cmd := m.dispatchFlagCommand("/flags")
		require.NotNil(t, cmd)

		msg := cmd()
		_, ok := msg.(listFlagConditionsMsg)
		assert.True(t, ok)
	})
}

func TestDispatchFlagCommand_Invalid(t *testing.T) {
	t.Run("returns flash notification for invalid command", func(t *testing.T) {
		m := createTestModel()
		m.flagMarker = defaultFlagMarker

		cmd := m.dispatchFlagCommand("/flag badtype value")
		require.NotNil(t, cmd)

		// The flash notification returns a tea.Cmd that produces setStatusMsg
		// We can't easily inspect the exact message but we can verify it's not nil
		_ = cmd
	})
}

// Suppress unused import warnings
var _ = tea.Cmd(nil)

func TestIsFlagCommand(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"flag command", "/flag cluster abc", true},
		{"flags command", "/flags", true},
		{"unflag command", "/unflag 3", true},
		{"not a flag command", "hello world", false},
		{"claude prompt", "explain this code", false},
		{"partial match", "/flagged", false},
		{"flags save", "/flags save", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isFlagCommand(tt.input))
		})
	}
}
