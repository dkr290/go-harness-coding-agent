package config

import "os"

const (
	DefaultModel   = "qwen3.8-27b-heretic-abliterated-uncensored"
	DefaultBaseURL = "http://localai:8080/v1"
)

var (
	APIKey  = envOrDefault("OPENAI_API_KEY", "")
	Model   = envOrDefault("OPENAI_MODEL", DefaultModel)
	BaseURL = envOrDefault("OPENAI_BASE_URL", DefaultBaseURL)
)

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
