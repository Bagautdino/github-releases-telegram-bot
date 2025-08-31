package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	GithubToken       string
	TelegramToken     string
	DefaultChatID     int64
	IntervalMinutes   int
	TimeZone          string
	AdvisorEnabled    bool
	OpenRouterAPIKey  string
	OpenRouterModel   string
	AllowedUserIDs    []int64
	MaxChangelogChars int
	MaxBullets        int
}

func Load() (*Config, error) {
	cfg := &Config{
		GithubToken:       mustGetEnv("GITHUB_TOKEN"),
		TelegramToken:     mustGetEnv("TELEGRAM_BOT_TOKEN"),
		DefaultChatID:     parseInt64(getEnv("DEFAULT_CHAT_ID", "0")),
		IntervalMinutes:   parseInt(getEnv("POLL_INTERVAL_MINUTES", "10")),
		TimeZone:          getEnv("TIMEZONE", "Europe/Amsterdam"),
		AdvisorEnabled:    getEnv("ADVISOR_ENABLED", "0") == "1",
		OpenRouterAPIKey:  getEnv("OPENROUTER_API_KEY", ""),
		OpenRouterModel:   getEnv("OPENROUTER_MODEL", "openrouter/anthropic/claude-3-haiku"),
		AllowedUserIDs:    parseUserIDs(getEnv("ALLOWED_USER_IDS", "")),
		MaxChangelogChars: parseInt(getEnv("MAX_CHANGELOG_CHARS", "2500")),
		MaxBullets:        parseInt(getEnv("MAX_BULLETS", "8")),
	}

	return cfg, nil
}

func mustGetEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic("Required environment variable " + key + " is not set")
	}
	return value
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

func parseInt64(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return i
}

func parseUserIDs(s string) []int64 {
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	var ids []int64
	for _, part := range parts {
		if id := parseInt64(strings.TrimSpace(part)); id != 0 {
			ids = append(ids, id)
		}
	}
	return ids
}
