package tools

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Tier classifies the importance of a watcher investigation result.
type Tier int

const (
	TierSilent     Tier = iota // Transcript only
	TierNoteworthy             // Pane entry
	TierActionable             // Pane entry + Ask
)

// Verdict is the structured output from a watcher investigation.
type Verdict struct {
	Tier    Tier
	Summary string
	Action  string
}

type verdictJSON struct {
	Tier    string `json:"tier"`
	Summary string `json:"summary"`
	Action  string `json:"action"`
}

var fencedJSONRe = regexp.MustCompile("(?s)```(?:json)?\\s*\\n(\\{[^`]*\\})\\s*\\n```")

// ParseWatcherVerdict extracts a verdict from the model's response.
// It looks for a fenced JSON block containing tier/summary/action.
// Malformed or missing blocks default to TierNoteworthy.
func ParseWatcherVerdict(output []byte) (Verdict, error) {
	matches := fencedJSONRe.FindSubmatch(output)
	if matches == nil {
		return Verdict{Tier: TierNoteworthy, Summary: string(output)}, nil
	}

	var raw verdictJSON
	if err := json.Unmarshal(matches[1], &raw); err != nil {
		return Verdict{Tier: TierNoteworthy, Summary: string(output)}, nil
	}

	tier := parseTier(raw.Tier)
	return Verdict{
		Tier:    tier,
		Summary: raw.Summary,
		Action:  raw.Action,
	}, nil
}

func parseTier(s string) Tier {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "silent":
		return TierSilent
	case "actionable":
		return TierActionable
	default:
		return TierNoteworthy
	}
}
