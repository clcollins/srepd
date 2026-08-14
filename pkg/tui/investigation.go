package tui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/openshift-online/srepd/pkg/ai"
	"github.com/openshift-online/srepd/pkg/ai/policy"
	"github.com/openshift-online/srepd/pkg/ai/tools"
)

// ToolRunnerFactory creates a BetaToolRunner. Implemented by *anthropic.BetaMessageService.
type ToolRunnerFactory interface {
	NewToolRunner(betaTools []anthropic.BetaTool, params anthropic.BetaToolRunnerParams, opts ...option.RequestOption) *anthropic.BetaToolRunner
}

// toolAsk captures a tool call that requires user approval during an investigation.
type toolAsk struct {
	toolName string
	input    json.RawMessage
}

// investigationMsg carries the result of a watcher tool-using investigation.
type investigationMsg struct {
	observation string
	verdict     tools.Verdict
	err         error
	toolAsks    []toolAsk
	incidentIDs []string // triggering incident IDs for scoped actions
}

// investigationConfig holds the settings for a watcher investigation.
type investigationConfig struct {
	maxToolTurns int
	timeout      time.Duration
	policyConfig policy.Config
}

func defaultInvestigationConfig() investigationConfig {
	return investigationConfig{
		maxToolTurns: 6,
		timeout:      90 * time.Second,
		policyConfig: policy.Config{Mode: policy.ModeInteractive},
	}
}

// watcherInvestigateCmd runs a bounded tool-using investigation in response
// to a detector observation. It uses the SDK's BetaToolRunner with
// NextMessage for per-turn control.
func watcherInvestigateCmd(
	runner ToolRunnerFactory,
	registry *tools.Registry,
	cfg investigationConfig,
	systemPrompt string,
	observation string,
	contextStr string,
	model string,
	onAsk func(toolName string, input json.RawMessage),
	incidentIDs []string,
) tea.Cmd {
	return func() tea.Msg {
		if runner == nil || registry == nil {
			return investigationMsg{
				observation: observation,
				err:         fmt.Errorf("tool runner or registry not configured"),
				incidentIDs: incidentIDs,
			}
		}

		if cfg.maxToolTurns <= 0 {
			cfg.maxToolTurns = 6
		}

		decide := func(toolName string, class policy.Class, input json.RawMessage) policy.Decision {
			return policy.Decide(cfg.policyConfig, toolName, class, input)
		}

		var mu sync.Mutex
		var collectedAsks []toolAsk
		seenAsks := make(map[string]struct{})
		wrappedOnAsk := func(toolName string, input json.RawMessage) {
			h := sha256.Sum256(input)
			key := toolName + ":" + hex.EncodeToString(h[:])
			mu.Lock()
			defer mu.Unlock()
			if _, dup := seenAsks[key]; dup {
				return
			}
			seenAsks[key] = struct{}{}
			collectedAsks = append(collectedAsks, toolAsk{toolName: toolName, input: input})
			if onAsk != nil {
				onAsk(toolName, input)
			}
		}

		gatedTools := registry.GatedBetaTools(decide, wrappedOnAsk)

		verdictInstruction := "\n\nAfter your investigation, end your response with a fenced JSON block:\n" +
			"```json\n{\"tier\": \"silent|noteworthy|actionable\", \"summary\": \"...\", \"action\": \"...\"}\n```\n" +
			"Use 'silent' for routine/expected patterns, 'noteworthy' for patterns worth surfacing, " +
			"'actionable' for patterns requiring user action (draft a note, suggest a command, or suggest escalation)."

		userPrompt := fmt.Sprintf("Observation: %s\n\nContext:\n%s", observation, contextStr)

		messages := []anthropic.BetaMessageParam{
			anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(userPrompt)),
		}

		if model == "" {
			return investigationMsg{
				observation: observation,
				err:         fmt.Errorf("investigation: model not configured; set llm_api.model or use a provider with a default"),
				incidentIDs: incidentIDs,
			}
		}

		toolRunner := runner.NewToolRunner(gatedTools, anthropic.BetaToolRunnerParams{
			BetaMessageNewParams: anthropic.BetaMessageNewParams{
				Model:     anthropic.Model(model),
				MaxTokens: 2048,
				System: []anthropic.BetaTextBlockParam{
					{Text: systemPrompt + verdictInstruction},
				},
				Messages: messages,
			},
			MaxIterations: cfg.maxToolTurns,
		})

		ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
		defer cancel()

		var fullText strings.Builder
		for {
			msg, err := toolRunner.NextMessage(ctx)
			if err != nil {
				log.Debug("investigation.runner", "error", err)
				log.Warn("investigation.runner", "error", ai.ClassifyProviderError(err))
				return investigationMsg{
					observation: observation,
					err:         err,
					incidentIDs: incidentIDs,
				}
			}
			if msg == nil {
				break
			}

			for _, block := range msg.Content {
				if block.Type == "text" {
					fullText.WriteString(block.AsText().Text)
				}
			}
		}

		text := fullText.String()
		verdict, _ := tools.ParseWatcherVerdict([]byte(text))

		mu.Lock()
		asks := collectedAsks
		mu.Unlock()

		return investigationMsg{
			observation: observation,
			verdict:     verdict,
			toolAsks:    asks,
			incidentIDs: incidentIDs,
		}
	}
}

// extractToolRunnerFactory returns a ToolRunnerFactory from a provider that
// supports the Anthropic Beta Messages API, or nil.
func extractToolRunnerFactory(provider interface{}) ToolRunnerFactory {
	type betaMessagesProvider interface {
		BetaMessages() *anthropic.BetaMessageService
	}
	if bmp, ok := provider.(betaMessagesProvider); ok {
		return bmp.BetaMessages()
	}
	return nil
}

// isAnthropicFamily returns true if the provider name is an Anthropic-family
// provider that supports the Tool Runner.
func isAnthropicFamily(providerName string) bool {
	switch providerName {
	case "anthropic", "anthropic-bedrock", "anthropic-vertex":
		return true
	default:
		return false
	}
}
