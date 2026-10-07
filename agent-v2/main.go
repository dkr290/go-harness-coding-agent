package main

import (
	"github.com/microsoft/agent-framework-go/agent"
	"github.com/microsoft/agent-framework-go/provider/openaiprovider"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/dkr290/go-harness-coding-agent/agent-v2/config"
	"github.com/dkr290/go-harness-coding-agent/agent-v2/harness"
	"github.com/dkr290/go-harness-coding-agent/agent-v2/templates"
	"github.com/dkr290/go-harness-coding-agent/agent-v2/tools"
)

func main() {
	client := openai.NewClient(
		option.WithAPIKey(config.APIKey),
		option.WithBaseURL(config.BaseURL),
	)
	a := openaiprovider.NewChatCompletionsAgent(
		client,
		openaiprovider.AgentConfig{
			Model:        config.Model,
			Instructions: templates.SystemPrompt,
			Config: agent.Config{
				Name:  "CodingAgent",
				Tools: append(tools.FilesystemTools(),tools.GitTools()...),
			},
		},
	)
	harness.Run(client, a)
}
