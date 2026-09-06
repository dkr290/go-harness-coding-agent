// Package harness
package harness

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	openai "github.com/openai/openai-go/v3"

	"github.com/dkr290/go-harness-coding-agent/config"
)

// Run runs the agent's conversation loop until the user quits.
func Run(client openai.Client) {
	// the conversation history. This is the entire memory of the agent.
	// Every turn, we append to it and send the whole thing to the model
	var messages []openai.ChatCompletionMessageParamUnion
	fmt.Println("Agent is ready. Type 'quit' or 'exit' to leave")

	scanner := bufio.NewScanner(os.Stdin)

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

		// 5. Call the model with the full conversation so far
		resp, err := client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
			Model:    config.Model,
			Messages: messages,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "model error: %v\n", err)
			continue
		}
		if len(resp.Choices) == 0 {
			fmt.Fprintln(os.Stderr, "model error: no choices in response")
			continue
		}

		// 6. Extract the assistant's reply and append it to the history
		assistantMessage := resp.Choices[0].Message.Content
		messages = append(messages, openai.AssistantMessage(assistantMessage))

		// 7. Show the user
		fmt.Printf("\nagent > %s\n\n", assistantMessage)
	}
}
