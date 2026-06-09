package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/charmbracelet/log"
)

type ollamaProvider struct {
	endpoint   string
	model      string
	httpClient *http.Client
}

func newOllamaProvider(cfg Config) (*ollamaProvider, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}

	model := cfg.Model
	if model == "" {
		model = "llama3.1:8b"
	}

	return &ollamaProvider{
		endpoint:   strings.TrimRight(endpoint, "/"),
		model:      model,
		httpClient: &http.Client{},
	}, nil
}

func (p *ollamaProvider) Name() string {
	return "ollama"
}

type ollamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []ollamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
}

type ollamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatResponse struct {
	Message ollamaChatMessage `json:"message"`
	Done    bool              `json:"done"`
}

func (p *ollamaProvider) Query(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	log.Debug("ollama.Query", "endpoint", p.endpoint, "model", p.model)
	messages := buildOllamaMessages(systemPrompt, userPrompt)

	body, err := json.Marshal(ollamaChatRequest{
		Model:    p.model,
		Messages: messages,
		Stream:   false,
	})
	if err != nil {
		return "", fmt.Errorf("ollama: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ollama: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama: request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama: server returned %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("ollama: decode response: %w", err)
	}

	return chatResp.Message.Content, nil
}

func (p *ollamaProvider) StreamQuery(ctx context.Context, systemPrompt string, userPrompt string, ch chan<- string) error {
	defer close(ch)
	log.Debug("ollama.StreamQuery", "endpoint", p.endpoint, "model", p.model)

	messages := buildOllamaMessages(systemPrompt, userPrompt)

	body, err := json.Marshal(ollamaChatRequest{
		Model:    p.model,
		Messages: messages,
		Stream:   true,
	})
	if err != nil {
		return fmt.Errorf("ollama: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ollama: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama: request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama: server returned %d: %s", resp.StatusCode, string(respBody))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var chunk ollamaChatResponse
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}

		if chunk.Message.Content != "" {
			ch <- chunk.Message.Content
		}

		if chunk.Done {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("ollama: stream read: %w", err)
	}

	return nil
}

func (p *ollamaProvider) Healthy(ctx context.Context) error {
	log.Debug("ollama.Healthy", "endpoint", p.endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("ollama: create health request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama: health check failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama: health check returned %d", resp.StatusCode)
	}

	return nil
}

func buildOllamaMessages(systemPrompt, userPrompt string) []ollamaChatMessage {
	var messages []ollamaChatMessage
	if systemPrompt != "" {
		messages = append(messages, ollamaChatMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, ollamaChatMessage{Role: "user", Content: userPrompt})
	return messages
}
