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
		functool.MustNew(functool.Config{
			Name:        "gitDiff",
			Description: "Show unstaged changes in the workspace",
		}, gitDiff),
		functool.MustNew(functool.Config{
			Name:        "gitLog",
			Description: "Show recent commit history. Defaults to the last 10 commits",
		}, gitLog),
		functool.MustNew(functool.Config{
			Name:        "gitCommit",
			Description: "Stage all current changes and commit them with the given message",
		}, gitCommit),
		functool.MustNew(functool.Config{
			Name:        "gitCheckout",
			Description: "Switch to a commit or branch. Used for both rollback and branch switching",
		}, gitCheckout),
		functool.MustNew(functool.Config{
			Name:        "gitBranch",
			Description: "List branches when called with no name, or create a new branch when a name is given. Does NOT switch to the new branch; call gitCheckout to switch",
		}, gitBranch),
	}
}

// gitStatus returns the output of `git status` inside the workspace.
func gitStatus(_ context.Context, _ struct{}) (string, error) {
	return runGit("status")
}

func gitDiff(_ context.Context, _ struct{}) (string, error) {
	return runGit("diff")
}

type gitLogArgs struct {
	Limit int `json:"limit" jsonschema:"Maximum number of commits to show"`
}

func gitLog(_ context.Context, args gitLogArgs) (string, error) {
	if args.Limit <= 0 {
		args.Limit = 10
	}
	return runGit("log", fmt.Sprintf("--max-count=%d", args.Limit), "--oneline")
}

type gitCommitArgs struct {
	Message string `json:"message" jsonschema:"The commit message"`
}

func gitCommit(_ context.Context, args gitCommitArgs) (string, error) {
	if _, err := runGit("add", "-A"); err != nil {
		return "", fmt.Errorf("git add: %w", err)
	}
	return runGit("commit", "-m", args.Message)
}

type gitCheckoutArgs struct {
	Ref string `json:"ref" jsonschema:"The commit SHA or branch name to check out"`
}

func gitCheckout(_ context.Context, args gitCheckoutArgs) (string, error) {
	return runGit("checkout", args.Ref)
}

type gitBranchArgs struct {
	Name string `json:"name,omitempty" jsonschema:"Name of the new branch to create; omit to list existing branches"`
}

func gitBranch(_ context.Context, args gitBranchArgs) (string, error) {
	if args.Name == "" {
		return runGit("branch")
	}
	return runGit("branch", args.Name)
}


// runGit executes a git command inside the workspace directory.
// It captures both stdout and stderr, returns the exit code and output on
// non-zero exit, and returns "no output" if stdout is empty on success.
// This is an internal utility — not registered as a tool.
func runGit(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("no git arguments provided")
	}

	cmd := exec.Command("git", args...)
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
