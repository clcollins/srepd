package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

type openaiCompatProvider struct {
	endpoint       string
	model          string
	apiKey         string
	httpClient     *http.Client
	requestTimeout time.Duration
}

func newOpenAICompatProvider(cfg Config, apiKey string) (*openaiCompatProvider, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("openai: endpoint is required")
	}

	return &openaiCompatProvider{
		endpoint:       strings.TrimRight(cfg.Endpoint, "/"),
		model:          cfg.Model,
		apiKey:         apiKey,
		httpClient:     &http.Client{},
		requestTimeout: defaultRequestTimeout,
	}, nil
}

func (p *openaiCompatProvider) Name() string {
	return "openai"
}

type openaiChatRequest struct {
	Model    string              `json:"model"`
	Messages []openaiChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
}

type openaiChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiChatResponse struct {
	Choices []openaiChoice `json:"choices"`
}

type openaiChoice struct {
	Message openaiChatMessage `json:"message"`
	Delta   openaiChatMessage `json:"delta"`
}

func (p *openaiCompatProvider) Query(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	log.Debug("openai.Query", "endpoint", p.endpoint, "model", p.model)
	ctx, cancel := ensureTimeout(ctx, p.requestTimeout)
	defer cancel()
	messages := buildOpenAIMessages(systemPrompt, userPrompt)

	body, err := json.Marshal(openaiChatRequest{
		Model:    p.model,
		Messages: messages,
		Stream:   false,
	})
	if err != nil {
		return "", fmt.Errorf("openai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("openai: create request: %w", err)
	}
	p.setHeaders(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai: request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		// Do NOT include the response body: a proxy/gateway may echo the
		// Authorization header back in its error body, which would leak the API
		// token into logs. Status code only (matches the Healthy method).
		return "", fmt.Errorf("openai: server returned %d", resp.StatusCode)
	}

	var chatResp openaiChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("openai: decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", nil
	}

	return chatResp.Choices[0].Message.Content, nil
}

func (p *openaiCompatProvider) StreamQuery(ctx context.Context, systemPrompt string, userPrompt string, ch chan<- string) error {
	defer close(ch)
	log.Debug("openai.StreamQuery", "endpoint", p.endpoint, "model", p.model)

	messages := buildOpenAIMessages(systemPrompt, userPrompt)

	body, err := json.Marshal(openaiChatRequest{
		Model:    p.model,
		Messages: messages,
		Stream:   true,
	})
	if err != nil {
		return fmt.Errorf("openai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("openai: create request: %w", err)
	}
	p.setHeaders(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("openai: request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		// Status code only — see the Query method: the body may echo the token.
		return fmt.Errorf("openai: server returned %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk openaiChatResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			ch <- chunk.Choices[0].Delta.Content
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("openai: stream read: %w", err)
	}

	return nil
}

func (p *openaiCompatProvider) Healthy(ctx context.Context) error {
	log.Debug("openai.Healthy", "endpoint", p.endpoint)
	ctx, cancel := ensureTimeout(ctx, p.requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint+"/v1/models", nil)
	if err != nil {
		return fmt.Errorf("openai: create health request: %w", err)
	}
	p.setHeaders(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("openai: health check failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("openai: health check returned %d", resp.StatusCode)
	}

	return nil
}

func (p *openaiCompatProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
}

func buildOpenAIMessages(systemPrompt, userPrompt string) []openaiChatMessage {
	var messages []openaiChatMessage
	if systemPrompt != "" {
		messages = append(messages, openaiChatMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, openaiChatMessage{Role: "user", Content: userPrompt})
	return messages
}
