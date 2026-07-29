// Fake claude binary for integration tests. Speaks the verified stream-json
// contract on stdout and records argv/stdin to FAKECLAUDE_LOG (JSONL).
// Enforces session-ID semantics via FAKECLAUDE_STATE dir, reproducing the
// verified behaviour of claude 2.1.220 (captured 2026-07-28):
//
//	--session-id <fresh>     → exit 0, normal NDJSON
//	--session-id <used>      → exit 1, stderr error, NO result line
//	--resume <known>         → exit 0, normal NDJSON, memory retained
//	--resume <unknown>       → scripted failure (UNVERIFIED against real CLI)
//
// Scenarios scripted via FAKECLAUDE_SCRIPT (JSON file).
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type script struct {
	Turns           [][]json.RawMessage `json:"turns"`
	CrashBeforeInit bool                `json:"crash_before_init"`
	RejectResume    bool                `json:"reject_resume"`
}

type logEntry struct {
	Type string   `json:"type"`
	Args []string `json:"args,omitempty"`
	Line string   `json:"line,omitempty"`
}

func main() {
	logPath := os.Getenv("FAKECLAUDE_LOG")
	stateDir := os.Getenv("FAKECLAUDE_STATE")
	scriptPath := os.Getenv("FAKECLAUDE_SCRIPT")

	args := os.Args[1:]

	var sessionID, resumeID string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--session-id":
			if i+1 < len(args) {
				i++
				sessionID = args[i]
			}
		case "--resume":
			if i+1 < len(args) {
				i++
				resumeID = args[i]
			}
		}
	}

	writeLog(logPath, logEntry{Type: "argv", Args: args})

	var s script
	if scriptPath != "" {
		if data, err := os.ReadFile(scriptPath); err == nil {
			_ = json.Unmarshal(data, &s)
		}
	}

	if s.CrashBeforeInit {
		os.Exit(1)
	}

	id := sessionID
	if sessionID != "" && stateDir != "" {
		statePath := filepath.Join(stateDir, sessionID)
		if _, err := os.Stat(statePath); err == nil {
			// Verified: duplicate session-id → exit 1, stderr, NO result line
			fmt.Fprintf(os.Stderr, "Error: Session ID %s is already in use.\n", sessionID)
			os.Exit(1)
		}
		_ = os.MkdirAll(stateDir, 0755)
		_ = os.WriteFile(statePath, []byte("used"), 0644)
	}

	if resumeID != "" {
		if s.RejectResume {
			fmt.Fprintf(os.Stderr, "Error: Session %s resume rejected by script.\n", resumeID)
			os.Exit(1)
		}
		id = resumeID
		if stateDir != "" {
			statePath := filepath.Join(stateDir, resumeID)
			if _, err := os.Stat(statePath); err != nil {
				// UNVERIFIED: unknown resume-id behaviour is scripted, not
				// verified against the real Claude CLI.
				fmt.Fprintf(os.Stderr, "Error: Session %s not found.\n", resumeID)
				os.Exit(1)
			}
		}
	}

	initLine, _ := json.Marshal(map[string]interface{}{
		"type": "system", "subtype": "init", "session_id": id,
	})
	fmt.Println(string(initLine))

	scanner := bufio.NewScanner(os.Stdin)
	turnIdx := 0
	for scanner.Scan() {
		line := scanner.Text()
		writeLog(logPath, logEntry{Type: "stdin", Line: line})

		if turnIdx < len(s.Turns) {
			for _, event := range s.Turns[turnIdx] {
				var m map[string]interface{}
				if err := json.Unmarshal(event, &m); err == nil {
					if _, ok := m["session_id"]; !ok {
						m["session_id"] = id
					}
					data, _ := json.Marshal(m)
					fmt.Println(string(data))
				}
			}
		} else {
			text := fmt.Sprintf("response %d", turnIdx)
			resp, _ := json.Marshal(map[string]interface{}{
				"type": "assistant",
				"message": map[string]interface{}{
					"role": "assistant",
					"content": []map[string]interface{}{
						{"type": "text", "text": text},
					},
				},
			})
			fmt.Println(string(resp))
			result, _ := json.Marshal(map[string]interface{}{
				"type": "result", "subtype": "success",
				"session_id": id, "result": text,
				"is_error": false,
			})
			fmt.Println(string(result))
		}
		turnIdx++
	}
}

func writeLog(path string, entry logEntry) {
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	data, _ := json.Marshal(entry)
	_, _ = f.Write(data)
	_, _ = f.Write([]byte("\n"))
}
