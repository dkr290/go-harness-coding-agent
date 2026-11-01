package tools

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/microsoft/agent-framework-go/tool"
	"github.com/microsoft/agent-framework-go/tool/functool"

	"github.com/dkr290/go-harness-coding-agent/agent-v2/config"
)

// Default timeout for bash commands in seconds.
const defaultBashTimeout = 30

// BashTool returns the bash shell tool, ready to hand to the agent's
// Config.Tools list.
func BashTool() []tool.Tool {
	return []tool.Tool{
		functool.MustNew(functool.Config{
			Name:        "bash",
			Description: "Execute an allowed command inside the workspace. The command must be in the allowlist. Optional arguments and a timeout (in seconds) can be provided. Defaults to 30s timeout.",
		}, bashCommand),
	}
}

type bashCommandArgs struct {
	Command string   `json:"command" jsonschema:"The binary name to execute; must be in the allowlist"`
	Args    []string `json:"args,omitempty" jsonschema:"Arguments to pass to the command"`
	Timeout int      `json:"timeout,omitempty" jsonschema:"Timeout in seconds; defaults to 30"`
}

// bashCommand executes an allowed command inside the workspace with a timeout.
func bashCommand(ctx context.Context, args bashCommandArgs) (string, error) {
	// Validate command is non-empty.
	if args.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	// Validate command is in the allowlist.
	if !isAllowed(args.Command) {
		return "", fmt.Errorf("command not allowed: %s", args.Command)
	}

	// Apply timeout.
	timeout := args.Timeout
	if timeout <= 0 {
		timeout = defaultBashTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, args.Command, args.Args...)
	cmd.Dir = Workspace().root

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	// Context timeout: return a structured message so the model can retry.
	if err != nil && ctx.Err() == context.DeadlineExceeded {
		return fmt.Sprintf("command timed out after %ds", timeout), nil
	}

	// Non-zero exit: return exit code and output to the model.
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			output := stdout.String()
			if output == "" {
				output = "no output"
			}
			return fmt.Sprintf("exit code: %d\nstdout: %s\nstderr: %s",
				exitErr.ExitCode(), output, stderr.String()), nil
		}
		return "", fmt.Errorf("error executing command: %s, %w", args.Command, err)
	}

	output := stdout.String()
	if output == "" {
		return "no output", nil
	}
	return output, nil
}

// isAllowed checks whether the given command name is in the allowlist.

func isAllowed(command string) bool {
	return slices.Contains(config.BashAllowlist, command)
}
