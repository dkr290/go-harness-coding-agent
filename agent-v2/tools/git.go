package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/microsoft/agent-framework-go/tool"
	"github.com/microsoft/agent-framework-go/tool/functool"
)

// GitTools returns the git tools, ready to hand to the agent's Config.Tools list.
func GitTools() []tool.Tool {
	return []tool.Tool{
		functool.MustNew(functool.Config{
			Name:        "gitStatus",
			Description: "Show the current git status of the workspace",
		}, gitStatus),
	}
}

// gitStatus returns the output of `git status` inside the workspace.
func gitStatus(_ context.Context, _ struct{}) (string, error) {
	return runGit("status")
}

// runGit executes a git command inside the workspace directory.
// It captures both stdout and stderr, returns the exit code and output on
// non-zero exit, and returns "no output" if stdout is empty on success.
// This is an internal utility — not registered as a tool.
func runGit(args string) (string, error) {
	argsList := strings.Fields(args)
	if len(argsList) == 0 {
		return "", fmt.Errorf("no git arguments provided")
	}

	cmd := exec.Command("git", argsList...)
	cmd.Dir = Workspace().root

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

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
		return "", fmt.Errorf("error executing git: %w", err)
	}

	output := stdout.String()
	if output == "" {
		return "no output", nil
	}
	return output, nil
}

// ensureGitRepo initializes a git repository in the workspace if one does not
// already exist. It runs:
//   - git init
//   - git config user.name agent
//   - git config user.email agent@harness.local
//   - git symbolic-ref HEAD refs/heads/main
//
// This is idempotent: if .git already exists, it is a no-op.
func ensureGitRepo(root string) error {
	// Check if a git repo already exists.
	if exec.Command("git", "-C", root, "rev-parse", "--git-dir").Run() == nil {
		return nil
	}

	cmds := [][]string{
		{"init"},
		{"config", "user.name", "agent"},
		{"config", "user.email", "agent@harness.local"},
		{"symbolic-ref", "HEAD", "refs/heads/main"},
	}

	for _, args := range cmds {
		cmd := exec.Command("git", args...)
		cmd.Dir = root

		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git %s: %w\nstdout: %s\nstderr: %s",
				strings.Join(args, " "), err, stdout.String(), stderr.String())
		}
	}

	return nil
}
