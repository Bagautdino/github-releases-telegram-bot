package advisor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client provides LLM advisory functionality via OpenRouter
type Client struct {
	apiKey  string
	model   string
	http    *http.Client
	baseURL string
}

// New creates a new OpenRouter client
func New(apiKey, model string) *Client {
	return &Client{
		apiKey:  apiKey,
		model:   model,
		baseURL: "https://openrouter.ai/api/v1",
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Request represents an OpenRouter API request
type Request struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Response represents an OpenRouter API response
type Response struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// Advise generates advice about a GitHub release
func (c *Client) Advise(ctx context.Context, repo, tag string, bullets []string) (string, error) {
	// Skip if client is not configured
	if c == nil || c.apiKey == "" || c.model == "" {
		return "", nil
	}
	
	// Skip if API key is "disabled" for testing
	if c.apiKey == "disabled" {
		return "", nil
	}

	prompt := c.buildPrompt(repo, tag, bullets)

	req := Request{
		Model:     c.model,
		MaxTokens: 200,
		Messages: []Message{
			{
				Role:    "system",
				Content: "Ты опытный технический эксперт. Анализируешь релизы ПО для разработчиков. Отвечай ТОЛЬКО на русском языке, максимально сухо и конкретно. Никакого маркетинга - только реальная практическая польза для инженеров.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	return c.makeRequest(ctx, req)
}

// buildPrompt creates a prompt for the LLM
func (c *Client) buildPrompt(repo, tag string, bullets []string) string {
	bulletsText := strings.Join(bullets, "; ")
	
	return fmt.Sprintf(`Проанализируй релиз %s %s. Напиши на русском языке краткий анализ (максимум 120 слов):

Изменения: %s

Напиши только практическую пользу:
1. Что конкретно улучшилось для разработчиков
2. Стоит ли обновляться и почему
3. Есть ли проблемы при обновлении

Пиши сухо и технично, как эксперт для инженеров. Без маркетинга и воды.`, repo, tag, bulletsText)
}

// makeRequest sends a request to OpenRouter API
func (c *Client) makeRequest(ctx context.Context, req Request) (string, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("HTTP-Referer", "https://github.com/yourorg/tg-release-bot")
	httpReq.Header.Set("X-Title", "TG Release Bot")
	httpReq.Header.Set("User-Agent", "TG-Release-Bot/1.0")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Читаем тело ответа для более детальной ошибки
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openrouter returned status %d: %s", resp.StatusCode, string(body))
	}

	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Error != nil {
		return "", fmt.Errorf("openrouter error: %s", response.Error.Message)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	content := strings.TrimSpace(response.Choices[0].Message.Content)

	// Limit response length for Telegram
	if len(content) > 400 {
		content = content[:400] + "…"
	}

	return content, nil
}
