package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Sender handles Telegram message sending
type Sender struct {
	bot *tgbotapi.BotAPI
}

// NewSender creates a new Telegram sender
func NewSender(token string) (*Sender, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	return &Sender{bot: bot}, nil
}

// SendHTML sends an HTML message to a chat, splitting if necessary
func (s *Sender) SendHTML(ctx context.Context, chatID int64, html string) error {
	chunks := chunkHTML(html, 4000)

	for _, chunk := range chunks {
		msg := tgbotapi.NewMessage(chatID, chunk)
		msg.ParseMode = "HTML"
		msg.DisableWebPagePreview = true

		// Retry logic for sending messages
		var lastErr error
		for attempt := 0; attempt < 3; attempt++ {
			_, err := s.bot.Send(msg)
			if err == nil {
				break
			}

			lastErr = err
			
			// Check if error is permanent (don't retry these)
			if isPermanentError(err) {
				return fmt.Errorf("permanent telegram error: %w", err)
			}

			// Exponential backoff for retryable errors
			if attempt < 2 {
				backoff := time.Duration(500*(attempt+1)) * time.Millisecond
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoff):
				}
			}
		}

		if lastErr != nil {
			return fmt.Errorf("failed to send message after retries: %w", lastErr)
		}

		// Small delay between chunks to avoid rate limiting
		if len(chunks) > 1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}

// chunkHTML splits HTML text into chunks that fit Telegram's message size limit
func chunkHTML(text string, maxSize int) []string {
	if len(text) <= maxSize {
		return []string{text}
	}

	var chunks []string
	remaining := text

	for len(remaining) > maxSize {
		// Try to find a good breaking point (newline)
		breakPoint := findBreakPoint(remaining, maxSize)
		if breakPoint == -1 {
			breakPoint = maxSize
		}

		chunks = append(chunks, remaining[:breakPoint])
		remaining = remaining[breakPoint:]

		// Trim leading whitespace from remaining text
		for len(remaining) > 0 && (remaining[0] == '\n' || remaining[0] == ' ') {
			remaining = remaining[1:]
		}
	}

	if len(remaining) > 0 {
		chunks = append(chunks, remaining)
	}

	return chunks
}

// findBreakPoint finds the best place to break text within the size limit
func findBreakPoint(text string, maxSize int) int {
	if len(text) <= maxSize {
		return len(text)
	}

	// Look for newline within the limit
	lastNewline := -1
	for i := 0; i < maxSize && i < len(text); i++ {
		if text[i] == '\n' {
			lastNewline = i
		}
	}

	if lastNewline > maxSize/2 { // Only use if it's not too early
		return lastNewline + 1 // Include the newline
	}

	// Look for space within the limit
	lastSpace := -1
	for i := 0; i < maxSize && i < len(text); i++ {
		if text[i] == ' ' {
			lastSpace = i
		}
	}

	if lastSpace > maxSize/2 { // Only use if it's not too early
		return lastSpace + 1 // Include the space
	}

	return -1 // No good break point found
}

// isPermanentError checks if a Telegram API error is permanent and shouldn't be retried
func isPermanentError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := strings.ToLower(err.Error())
	
	// These errors indicate permanent issues that won't be fixed by retrying
	permanentErrors := []string{
		"chat not found",
		"bot was blocked by the user",
		"user is deactivated",
		"text must be encoded in utf-8",
		"message is too long",
		"bad request: can't parse entities",
		"forbidden",
	}
	
	for _, permErr := range permanentErrors {
		if strings.Contains(errStr, permErr) {
			return true
		}
	}
	
	return false
}
