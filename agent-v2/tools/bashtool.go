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
			Name: "bash",
			Description: "Execute a shell command inside the workspace via sh -c. " +
				"Commands may be chained with &&, ||, ; or |. Every command in the chain " +
				"must be permitted: denied commands are always rejected, and unless all " +
				"commands are allowed, each command must be in the allowlist. " +
				"Optional timeout (in seconds); defaults to 30s.",
		}, bashCommand),
	}
}

type bashCommandArgs struct {
	Command string `json:"command" jsonschema:"The shell command line to execute; may chain commands with &&, ||, ; or |"`
	Timeout int    `json:"timeout,omitempty" jsonschema:"Timeout in seconds; defaults to 30"`
}

// bashCommand executes a permitted command line inside the workspace with a
// timeout.
func bashCommand(ctx context.Context, args bashCommandArgs) (string, error) {
	// Validate command is non-empty.
	if strings.TrimSpace(args.Command) == "" {
		return "", fmt.Errorf("command is required")
	}

	// Validate every command in the chain against the deny/allow lists.
	if err := validateCommand(args.Command); err != nil {
		return "", err
	}

	// Apply timeout.
	timeout := args.Timeout
	if timeout <= 0 {
		timeout = defaultBashTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", args.Command)
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

// validateCommand splits a command line on the chain separators &&, ||, ;
// and |, then validates the binary of every segment. All segments must pass
// for the command to run.
func validateCommand(command string) error {
	for _, segment := range splitChain(command) {
		bin := firstToken(segment)
		if bin == "" {
			continue
		}
		if err := validateBinary(bin); err != nil {
			return err
		}
	}
	return nil
}

// validateBinary checks one binary against the deny and allow lists. The
// deny list always wins: a binary in both lists is rejected. When
// ALLOW_ALL_COMMANDS is set or the allowlist is empty, every non-denied
// binary is permitted.
func validateBinary(bin string) error {
	if slices.Contains(config.BashDenylist, bin) {
		return fmt.Errorf("command denied: %s", bin)
	}
	if config.BashAllowAll || len(config.BashAllowlist) == 0 {
		return nil
	}
	if !slices.Contains(config.BashAllowlist, bin) {
		return fmt.Errorf("command not allowed: %s", bin)
	}
	return nil
}

// splitChain splits a command line on the shell chain separators &&, ||, ;
// and |. Separators inside single or double quotes are ignored.
func splitChain(command string) []string {
	var segments []string
	var current strings.Builder
	var quote rune // 0, '\'' or '"'

	runes := []rune(command)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if quote != 0 {
			current.WriteRune(c)
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
			current.WriteRune(c)
		case ';':
			segments = append(segments, current.String())
			current.Reset()
		case '&', '|':
			if i+1 < len(runes) && runes[i+1] == c {
				// && or ||
				segments = append(segments, current.String())
				current.Reset()
				i++
			} else if c == '|' {
				segments = append(segments, current.String())
				current.Reset()
			} else {
				// A lone & (background) is kept as-is.
				current.WriteRune(c)
			}
		default:
			current.WriteRune(c)
		}
	}
	segments = append(segments, current.String())
	return segments
}

// firstToken returns the first whitespace-separated token of a chain
// segment — the binary name to validate.
func firstToken(segment string) string {
	fields := strings.Fields(segment)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
