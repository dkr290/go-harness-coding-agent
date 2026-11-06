package harness

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/microsoft/agent-framework-go/agent"
	"github.com/microsoft/agent-framework-go/message"
	"github.com/microsoft/agent-framework-go/tool"
	"github.com/openai/openai-go/v3"

	"github.com/dkr290/go-harness-coding-agent/agent-v2/config"
	"github.com/dkr290/go-harness-coding-agent/agent-v2/templates"
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

		reply, err := runTurn(a, session, userInput)
		if err != nil {
			fmt.Fprintf(os.Stderr, "model error: %v\n", err)
			continue
		}

		reply = strings.TrimSpace(reply)
		if reply == "" {
			fmt.Fprintln(os.Stderr, "model error: empty response")
			continue
		}

		fmt.Printf("\nagent > %s\n\n", reply)
	}
}

// runTurn runs a single user turn: the agent's toolautocall middleware loops
// on tool calls until the model answers with plain text or the step budget
// (configured in main.go via ToolAutoCall.MaximumIterationsPerRequest) is
// exhausted.
//
// When the budget is hit, the middleware's last iteration runs without tool
// declarations, so the model is forced to end with plain text. If it still
// produced no text (it tried to keep calling tools anyway), the harness
// injects the budget-hit message into the session and makes one final,
// tool-less run so the model summarizes what it did and control returns to
// the user instead of the loop running indefinitely.
func runTurn(a *agent.Agent, session *agent.Session, userInput string) (string, error) {
	resp, err := a.RunText(
		context.Background(),
		userInput,
		agent.WithSession(session),
	).Collect()
	if err != nil {
		return "", err
	}

	// Plain text: the loop finished within budget. The framework guarantees
	// text on the budget-capped final iteration (no tools are offered), so
	// an empty response here means the run ended on attempted tool calls.
	if strings.TrimSpace(resp.String()) != "" {
		return resp.String(), nil
	}

	// Budget exhausted without a summary: inject the budget-hit message and
	// force one final run with tools disabled so the model must answer in
	// plain text and hand control back to the user.
	budget := config.StepBudget
	fmt.Printf("  [budget] step budget of %d tool-call rounds reached, forcing summary\n", budget)
	finalResp, err := a.Run(
		context.Background(),
		[]*message.Message{message.NewText(fmt.Sprintf(templates.BudgetHit, budget))},
		agent.WithSession(session),
		agent.WithToolMode(tool.ToolModeNone),
	).Collect()
	if err != nil {
		return "", err
	}
	return finalResp.String(), nil
}


