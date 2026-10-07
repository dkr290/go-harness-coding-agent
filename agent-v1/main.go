package main

import (
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/dkr290/go-harness-coding-agent/config"
	"github.com/dkr290/go-harness-coding-agent/harness"
)

func main() {
	client := openai.NewClient(
		option.WithAPIKey(config.APIKey),
		option.WithBaseURL(config.BaseURL),
	)

	harness.Run(client)
}
