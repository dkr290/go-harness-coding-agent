package main

import (
	"fmt"
	"os"

	"github.com/microsoft/agent-framework-go/agent"
	"github.com/microsoft/agent-framework-go/agent/harness/toolautocall"
	"github.com/microsoft/agent-framework-go/provider/openaiprovider"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/dkr290/go-harness-coding-agent/agent-v2/config"
	"github.com/dkr290/go-harness-coding-agent/agent-v2/harness"
	"github.com/dkr290/go-harness-coding-agent/agent-v2/templates"
	"github.com/dkr290/go-harness-coding-agent/agent-v2/tools"
)

func main() {
	// Initialize the workspace (creates directory and git repo if needed)
	// so that git is ready before any tool is invoked.
	if err := tools.InitializeWorkspace(); err != nil {
		fmt.Fprintf(os.Stderr, "cannot initialize workspace: %v\n", err)
		os.Exit(1)
	}

	client := openai.NewClient(
		option.WithAPIKey(config.APIKey),
		option.WithBaseURL(config.BaseURL),
	)
	a := openaiprovider.NewChatCompletionsAgent(
		client,
		openaiprovider.AgentConfig{
			Model:        config.Model,
			Instructions: templates.SystemPrompt + templates.AgentsMD,
			// Step budget: cap the tool-call loop at StepBudget-1 rounds per
			// user turn so the harness's budget-hit run (which forces a
			// tool-less summary) is the StepBudget-th and final model call.
			// The framework strips tool declarations on the last iteration,
			// so the model cannot exceed the budget even if it tries.
			ToolAutoCall: &toolautocall.Config{
				MaximumIterationsPerRequest: new(config.StepBudget - 1),
			},
			Config: agent.Config{
				Name:  "CodingAgent",
				Tools: append(append(tools.FilesystemTools(), tools.GitTools()...), tools.BashTool()...),
			},
		},
	)
	harness.Run(client, a)
}
