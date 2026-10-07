package config

import (
	"os"
	"strings"
)

const (
	DefaultModel   = "gpt-4o-mini"
	DefaultBaseURL = "https://api.openai.com/v1"
)

var (
	APIKey   = envOrDefault("OPENAI_API_KEY", "")
	Model    = envOrDefault("OPENAI_MODEL", DefaultModel)
	BaseURL  = envOrDefault("OPENAI_BASE_URL", DefaultBaseURL)
	Thinking = envBool("OPENAI_THINKING", true)
)

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
}
