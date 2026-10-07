package main

import (
	"fmt"
	"os"
	"path/filepath"
)


type workspace struct {
	root string
}

func main(){

var Workspace = mustWorkspace(filepath.Join("../agent", "workspace"))


fmt.Println(Workspace.resolve("test.txt"))


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


func mustWorkspace(root string) *workspace {
	ws, err := newWorkspace(root)
	if err != nil {
		panic(fmt.Sprintf("tools: cannot initialize workspace: %v", err))
	}
	return ws
}

func newWorkspace(root string) (*workspace, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	return &workspace{root: abs}, nil
}


