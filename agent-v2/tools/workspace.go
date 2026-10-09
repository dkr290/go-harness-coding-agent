// Package tools provides the workspace-scoped filesystem tools for the agent.
//
// Tools are built with the agent-framework's functool package: each tool is a
// typed func(context.Context, Args) (Result, error), and the framework takes
// care of schema generation, argument binding, and dispatch — no registry is
// needed. Tools are constructed lazily by FilesystemTools() because the
// workspace is resolved relative to the process working directory at startup,
// not at package-initialization time.
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// workspace is the directory all file tools are scoped to.
type workspace struct {
	root string
}

var (
	workspaceOnce sync.Once
	workspaceInst *workspace
)

// InitializeWorkspace creates the workspace directory and initializes a git
// repository if one does not already exist. It must be called once at startup
// before any tools are used.
func InitializeWorkspace() error {
	var err error
	workspaceOnce.Do(func() {
		ws, e := newWorkspace("./workspace")
		if e != nil {
			err = fmt.Errorf("tools: cannot initialize workspace: %w", e)
			return
		}
		workspaceInst = ws
	})
	return err
}

// Workspace is the single static workspace the file tools operate in, rooted
// at "./workspace" under the process working directory. It is resolved on
// first use (after flag parsing) rather than from init(), so tests and
// embedding binaries control their own working directory first.
func Workspace() *workspace {
	workspaceOnce.Do(func() {
		ws, e := newWorkspace("./workspace")
		if e != nil {
			panic(fmt.Sprintf("tools: cannot initialize workspace: %v", e))
		}
		workspaceInst = ws
	})
	return workspaceInst
}

// Resolve validates that the workspace is initialized and the workspace
// root exists. It is intended for eager initialization at application startup.
func (w *workspace) Resolve(p string) (string, error) {
	if w == nil {
		return "", fmt.Errorf("workspace not initialized")
	}
	if p == "" {
		return w.root, nil
	}
	return w.resolve(p)
}

func newWorkspace(root string) (*workspace, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	if err := ensureGitRepo(abs); err != nil {
		return nil, fmt.Errorf("initialize git: %w", err)
	}
	return &workspace{root: abs}, nil
}

// resolve validates a relative path; os.Root enforces containment at use time,
// including symlinks and concurrent path changes.
func (w *workspace) resolve(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	if !filepath.IsLocal(p) {
		return "", fmt.Errorf("path escapes workspace: %q", p)
	}
	return filepath.Clean(p), nil
}
