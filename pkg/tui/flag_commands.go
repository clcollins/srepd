package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
)

type flagCmdAction int

const (
	flagCmdAdd flagCmdAction = iota
	flagCmdRemove
	flagCmdClearAll
	flagCmdList
	flagCmdSave
	flagCmdLoad
)

type parsedFlagCommand struct {
	action    flagCmdAction
	condition FlagCondition
	targetID  int
	path      string
}

type addFlagConditionMsg struct{ condition FlagCondition }
type removeFlagConditionMsg struct{ id int }
type clearFlagConditionsMsg struct{}
type listFlagConditionsMsg struct{}
type flagsSavedMsg struct{ err error }
type flagsLoadedMsg struct {
	conditions []FlagCondition
	err        error
}

func isFlagCommand(input string) bool {
	trimmed := strings.TrimSpace(input)
	return trimmed == ":flags" ||
		strings.HasPrefix(trimmed, ":flags ") ||
		strings.HasPrefix(trimmed, ":flag ") ||
		trimmed == ":flag" ||
		strings.HasPrefix(trimmed, ":unflag ") ||
		trimmed == ":unflag"
}

func parseFlagCommand(input string) (*parsedFlagCommand, error) {
	trimmed := strings.TrimSpace(input)
	parts := strings.Fields(trimmed)

	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	switch parts[0] {
	case ":flags":
		return parseFlagsSubcommand(parts[1:])
	case ":flag":
		return parseFlagAdd(parts[1:])
	case ":unflag":
		return parseUnflag(parts[1:])
	default:
		return nil, fmt.Errorf("unknown command: %s", parts[0])
	}
}

func parseFlagsSubcommand(args []string) (*parsedFlagCommand, error) {
	if len(args) == 0 {
		return &parsedFlagCommand{action: flagCmdList}, nil
	}

	switch args[0] {
	case "save":
		cmd := &parsedFlagCommand{action: flagCmdSave}
		if len(args) > 1 {
			cmd.path = strings.Join(args[1:], " ")
		}
		return cmd, nil
	case "load":
		cmd := &parsedFlagCommand{action: flagCmdLoad}
		if len(args) > 1 {
			cmd.path = strings.Join(args[1:], " ")
		}
		return cmd, nil
	default:
		return nil, fmt.Errorf("unknown /flags subcommand: %s (use: save, load)", args[0])
	}
}

func parseFlagAdd(args []string) (*parsedFlagCommand, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("usage: /flag <cluster|org> <value>")
	}

	flagType := args[0]
	value := strings.Join(args[1:], " ")

	switch flagType {
	case "cluster":
		if value == "" {
			return nil, fmt.Errorf("usage: /flag cluster <cluster-id>")
		}
		return &parsedFlagCommand{
			action: flagCmdAdd,
			condition: FlagCondition{
				Type:    FlagClusterID,
				Pattern: value,
				Label:   fmt.Sprintf("cluster ID matches %q", value),
			},
		}, nil
	case "org":
		if value == "" {
			return nil, fmt.Errorf("usage: /flag org <pattern>")
		}
		return &parsedFlagCommand{
			action: flagCmdAdd,
			condition: FlagCondition{
				Type:    FlagOrgName,
				Pattern: value,
				Label:   fmt.Sprintf("org name matches %q", value),
			},
		}, nil
	default:
		return nil, fmt.Errorf("unknown flag type %q (use: cluster, org)", flagType)
	}
}

func parseUnflag(args []string) (*parsedFlagCommand, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("usage: /unflag <id|all>")
	}

	if args[0] == "all" {
		return &parsedFlagCommand{action: flagCmdClearAll}, nil
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("invalid flag ID %q: must be a number or 'all'", args[0])
	}

	return &parsedFlagCommand{action: flagCmdRemove, targetID: id}, nil
}

func defaultFlagsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "flags.json"
	}
	return filepath.Join(home, ".config", "srepd", "flags.json")
}

func saveFlagsCmd(conditions []FlagCondition, path string) tea.Cmd {
	return func() tea.Msg {
		if path == "" {
			path = defaultFlagsPath()
		}
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return flagsSavedMsg{err: fmt.Errorf("create directory: %w", err)}
		}
		data, err := json.MarshalIndent(conditions, "", "  ")
		if err != nil {
			return flagsSavedMsg{err: fmt.Errorf("marshal flags: %w", err)}
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return flagsSavedMsg{err: fmt.Errorf("write flags: %w", err)}
		}
		log.Info("flags saved", "path", path, "count", len(conditions))
		return flagsSavedMsg{}
	}
}

func loadFlagsCmd(path string) tea.Cmd {
	return func() tea.Msg {
		if path == "" {
			path = defaultFlagsPath()
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return flagsLoadedMsg{err: fmt.Errorf("read flags: %w", err)}
		}
		var conditions []FlagCondition
		if err := json.Unmarshal(data, &conditions); err != nil {
			return flagsLoadedMsg{err: fmt.Errorf("parse flags: %w", err)}
		}
		log.Info("flags loaded", "path", path, "count", len(conditions))
		return flagsLoadedMsg{conditions: conditions}
	}
}

func (m model) dispatchFlagCommand(input string) tea.Cmd {
	parsed, err := parseFlagCommand(input)
	if err != nil {
		return m.flashNotification(err.Error())
	}

	switch parsed.action {
	case flagCmdAdd:
		cond := parsed.condition
		cond.CreatedAt = time.Now()
		return func() tea.Msg { return addFlagConditionMsg{condition: cond} }
	case flagCmdRemove:
		id := parsed.targetID
		return func() tea.Msg { return removeFlagConditionMsg{id: id} }
	case flagCmdClearAll:
		return func() tea.Msg { return clearFlagConditionsMsg{} }
	case flagCmdList:
		return func() tea.Msg { return listFlagConditionsMsg{} }
	case flagCmdSave:
		return saveFlagsCmd(m.flagConditions, parsed.path)
	case flagCmdLoad:
		return loadFlagsCmd(parsed.path)
	default:
		return nil
	}
}
