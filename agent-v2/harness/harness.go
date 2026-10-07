package harness

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/microsoft/agent-framework-go/agent"
	"github.com/openai/openai-go/v3"
)



func Run(client openai.Client, a *agent.Agent) {


	// A session gives the agent conversation memory across turns.
	session, err := a.CreateSession(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create session: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Agent is ready. Type 'quit' or 'exit' to leave")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("you > ")
		if !scanner.Scan() {
			break
		}
		userInput := strings.TrimSpace(scanner.Text())

		if userInput == "quit" || userInput == "exit" {
			fmt.Println("Goodbye.")
			break
		}
		if userInput == "" {
			continue
		}

		resp, err := a.RunText(
			context.Background(),
			userInput,
			agent.WithSession(session),
		).Collect()
		if err != nil {
			fmt.Fprintf(os.Stderr, "model error: %v\n", err)
			continue
		}

		reply := strings.TrimSpace(resp.String())
		if reply == "" {
			fmt.Fprintln(os.Stderr, "model error: empty response")
			continue
		}

		fmt.Printf("\nagent > %s\n\n", reply)
	}
}


