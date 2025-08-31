package advisor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
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
			Timeout: 20 * time.Second, // Увеличили timeout с 10 до 20 секунд
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
		MaxTokens: 350, // Увеличили с 200 до 350 для более полных ответов
		Messages: []Message{
			{
				Role:    "system",
				Content: "Ты опытный DevOps инженер. Анализируешь релизы для разработчиков. Отвечай СТРОГО в формате:\n\n🔧 КЛЮЧЕВЫЕ ИЗМЕНЕНИЯ:\n• [конкретное изменение]\n• [конкретное изменение]\n\n⚠️ ВАЖНО:\n• [что важно знать при обновлении]\n\nОтвечай кратко, максимум 3-4 пункта. БЕЗ заголовков, БЕЗ нумерации, БЕЗ лишнего текста. Только практическая польза для инженеров.",
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
	
	return fmt.Sprintf(`Релиз: %s %s

Изменения:
%s

Проанализируй что важно для DevOps/разработчиков. Следуй СТРОГО формату из system prompt.`, repo, tag, bulletsText)
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

	// Format and limit response length for Telegram
	content = formatLLMResponse(content)

	return content, nil
}

// formatLLMResponse cleans up and formats LLM response for better readability
func formatLLMResponse(content string) string {
	if content == "" {
		return ""
	}
	
	// Clean up common formatting issues
	content = strings.ReplaceAll(content, "**", "") // Remove markdown bold
	content = strings.ReplaceAll(content, "*", "")  // Remove markdown italic
	content = strings.ReplaceAll(content, "#", "")  // Remove markdown headers
	
	// Fix common issues with numbered lists
	content = regexp.MustCompile(`(?m)^\s*(\d+)\.\s*`).ReplaceAllString(content, "$1. ")
	
	// Clean up extra whitespace
	content = regexp.MustCompile(`\s+`).ReplaceAllString(content, " ")
	content = regexp.MustCompile(`\n\s*\n`).ReplaceAllString(content, "\n")
	
	// Remove common LLM artifacts and clean up formatting
	content = strings.ReplaceAll(content, "Анализ релиз-нот", "")
	content = strings.ReplaceAll(content, "Анализ релиза", "")
	content = regexp.MustCompile(`(?i)^##\s*`).ReplaceAllString(content, "")
	
	// Clean up emoji formatting and ensure proper spacing
	content = regexp.MustCompile(`🔧\s*`).ReplaceAllString(content, "🔧 ")
	content = regexp.MustCompile(`⚠️\s*`).ReplaceAllString(content, "⚠️ ")
	content = regexp.MustCompile(`•\s*`).ReplaceAllString(content, "• ")
	
	// Remove duplicate spaces around structured elements
	content = regexp.MustCompile(`\s*🔧\s*`).ReplaceAllString(content, "\n🔧 ")
	content = regexp.MustCompile(`\s*⚠️\s*`).ReplaceAllString(content, "\n\n⚠️ ")
	content = regexp.MustCompile(`\s*•\s*`).ReplaceAllString(content, "\n• ")
	
	// Trim and ensure proper structure
	content = strings.TrimSpace(content)
	
	// Smart truncation - try to cut at sentence boundary
	maxLength := 600 // Увеличили с 400 до 600
	if len(content) > maxLength {
		// Try to cut at sentence end
		sentences := strings.Split(content, ". ")
		truncated := ""
		
		for _, sentence := range sentences {
			test := truncated + sentence + ". "
			if len(test) > maxLength-10 { // Leave some margin
				break
			}
			truncated = test
		}
		
		if truncated != "" {
			content = strings.TrimSpace(truncated)
			if !strings.HasSuffix(content, ".") {
				content += "."
			}
		} else {
			// Fallback: cut at word boundary
			words := strings.Fields(content)
			truncated = ""
			for _, word := range words {
				test := truncated + " " + word
				if len(test) > maxLength-10 {
					break
				}
				truncated = test
			}
			content = strings.TrimSpace(truncated) + "…"
		}
	}
	
	return content
}
