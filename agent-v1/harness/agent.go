// Package harness
package harness

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"

	"github.com/dkr290/go-harness-coding-agent/config"
	"github.com/dkr290/go-harness-coding-agent/templates"
	"github.com/dkr290/go-harness-coding-agent/tools"
)

// Run runs the agent's conversation loop until the user quits.
func Run(client openai.Client) {
	// the conversation history. This is the entire memory of the agent.
	// Every turn, we append to it and send the whole thing to the model
	var messages []openai.ChatCompletionMessageParamUnion
	fmt.Println("Agent is ready. Type 'quit' or 'exit' to leave")

	scanner := bufio.NewScanner(os.Stdin)

	// The system message is added once, at the start of the conversation.
	messages = append(messages, openai.SystemMessage(templates.SystemPrompt))

	// The registered tool schemas are sent with every request so the model
	// knows it can call readFile, writeFile, listFiles, makeDir, deleteFile.
	toolParams, err := toolParams()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tool schema error: %v\n", err)
		return
	}

	for {
		// 1. Get user input
		fmt.Print("you > ")
		if !scanner.Scan() {
			break
		}
		userInput := strings.TrimSpace(scanner.Text())

		// 2. Allow the user to leave cleanly
		if userInput == "quit" || userInput == "exit" {
			fmt.Println("Goodbye.")
			break
		}

		// 3. Skip empty lines without making a model call
		if userInput == "" {
			continue
		}

		// 4. Append the user's message to the history
		messages = append(messages, openai.UserMessage(userInput))

		// 5. Call the model, running any requested tools until the model
		//    answers with plain text
		reply, err := runTurn(client, &messages, toolParams)
		if err != nil {
			fmt.Fprintf(os.Stderr, "model error: %v\n", err)
			continue
		}
		if reply == "" {
			fmt.Fprintln(os.Stderr, "model error: empty response")
			continue
		}

		// 6. Show the user
		fmt.Printf("\nagent > %s\n\n", reply)
	}
}

// runTurn calls the model and processes tool calls: each requested tool is
// dispatched to the registry and its result appended as a tool message, then
// the model is called again. It returns once the model replies with plain
// text. The conversation history is updated in place via messages.
func runTurn(
	client openai.Client,
	messages *[]openai.ChatCompletionMessageParamUnion,
	toolParams []openai.ChatCompletionToolUnionParam,
) (string, error) {
	for {
		params := openai.ChatCompletionNewParams{
			Model:    config.Model,
			Messages: *messages,
			Tools:    toolParams,
		}
		if !config.Thinking {
			params.ReasoningEffort = shared.ReasoningEffortNone
		}
		resp, err := client.Chat.Completions.New(context.Background(), params)
		if err != nil {
			return "", err
		}
		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("no choices in response")
		}

		msg := resp.Choices[0].Message
		// Append the assistant message verbatim — ToParam carries the
		// tool_calls the tool results below refer back to.
		*messages = append(*messages, msg.ToParam())

		// No tool calls: the model's text is the final answer for this turn.
		if len(msg.ToolCalls) == 0 {
			return msg.Content, nil
		}

		// Run every requested tool and append each result as a tool message.
		for _, call := range msg.ToolCalls {
			fn := call.AsFunction()
			var args map[string]any
			if err := json.Unmarshal([]byte(fn.Function.Arguments), &args); err != nil {
				args = map[string]any{}
			}
			result := tools.Registry.Dispatch(fn.Function.Name, args)
			fmt.Printf("  [tool] %s(%s) -> %s\n", fn.Function.Name, fn.Function.Arguments, result)
			*messages = append(*messages, openai.ToolMessage(result, fn.ID))
		}
	}
}

// toolParams converts the registry's OpenAI-shaped schemas
// ({"type": "function", "function": {...}}) into the SDK's param types.
func toolParams() ([]openai.ChatCompletionToolUnionParam, error) {
	schemas := tools.Registry.GetSchemas()
	raw, err := json.Marshal(schemas)
	if err != nil {
		return nil, fmt.Errorf("encode tool schemas: %w", err)
	}
	var params []openai.ChatCompletionToolUnionParam
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, fmt.Errorf("decode tool schemas: %w", err)
	}
	return params, nil
}
