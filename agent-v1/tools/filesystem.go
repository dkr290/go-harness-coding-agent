package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// workspace is the directory all file tools are scoped to — "agent/workspace"
// next to the running executable, not inside any package directory.


type workspace struct {
	root string
}

func init() {
	RegisterTool(Registry, "Read the contents of a file inside the workspace", readFile)
	RegisterTool(
		Registry,
		"Write content to a file inside the workspace, creating or overwriting it",
		writeFile,
	)
	RegisterTool(
		Registry,
		"List the entries of a directory inside the workspace, sorted, with a trailing / on directories",
		listFiles,
	)
	RegisterTool(
		Registry,
		"Create a directory inside the workspace, including any missing parent directories (existing directories are a no-op)",
		makeDir,
	)
	RegisterTool(
		Registry,
		"Delete a file from the workspace (does not delete directories)",
		deleteFile,
	)
}

// Workspace is the single static workspace the file tools operate in,
// rooted at "/workspace" under the app root.
var Workspace = mustWorkspace("./workspace")


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

type readFileArgs struct {
	Path string `json:"path" jsonschema:"description=Path to the file to read; relative to the workspace"`
}

// readFile reads the contents of a file inside the workspace.
func readFile(args readFileArgs) (string, error) {
	rel, err := Workspace.resolve(args.Path)
	if err != nil {
		return "", err
	}

	// os.OpenRoot confines the open to the workspace root: even if a
	// symlink inside the workspace points outside, the open fails.
	root, err := os.OpenRoot(Workspace.root)
	if err != nil {
		return "", fmt.Errorf("open workspace: %w", err)
	}
	defer root.Close()

	data, err := root.ReadFile(rel)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(data), nil
}

type writeFileArgs struct {
	Path    string `json:"path"    jsonschema:"description=Path to the file to write; relative to the workspace"`
	Content string `json:"content" jsonschema:"description=Full content to write to the file"`
}

// writeFile creates or overwrites a file inside the workspace and returns a
// confirmation with the byte count written.
func writeFile(args writeFileArgs) (string, error) {
	rel, err := Workspace.resolve(args.Path)
	if err != nil {
		return "", err
	}

	// os.OpenRoot confines the open to the workspace root: even if a
	// symlink inside the workspace points outside, the open fails.
	root, err := os.OpenRoot(Workspace.root)
	if err != nil {
		return "", fmt.Errorf("open workspace: %w", err)
	}
	defer root.Close()

	// Create parent directories — os.Root.WriteFile does not.
	if dir := filepath.Dir(rel); dir != "." {
		if err := root.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("create directories: %w", err)
		}
	}

	if err := root.WriteFile(rel, []byte(args.Content), 0o644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(args.Content), rel), nil
}

type listFilesArgs struct {
	Path string `json:"path" jsonschema:"description=Directory to list; relative to the workspace (use \".\" for the workspace root)"`
}

// listFiles lists the entries of a directory inside the workspace, sorted by
// name, with a trailing "/" on directories so the model can tell them apart
// from files.
func listFiles(args listFilesArgs) (string, error) {
	rel, err := Workspace.resolve(args.Path)
	if err != nil {
		return "", err
	}

	// os.OpenRoot confines the open to the workspace root: even if a
	// symlink inside the workspace points outside, the open fails.
	root, err := os.OpenRoot(Workspace.root)
	if err != nil {
		return "", fmt.Errorf("open workspace: %w", err)
	}
	defer root.Close()

	fileInfo, err := root.Stat(rel)
	if err != nil {
		return "", fmt.Errorf("stat path: %w", err)
	}
	if !fileInfo.IsDir() {
		return "", fmt.Errorf("not a directory: %s", rel)
	}

	dir, err := root.Open(rel)
	if err != nil {
		return "", fmt.Errorf("open directory: %w", err)
	}
	defer dir.Close()

	entries, err := dir.ReadDir(-1)
	if err != nil {
		return "", fmt.Errorf("read directory: %w", err)
	}

	// Sort by name for deterministic output the model can rely on.
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}
	sort.Strings(names)

	return strings.Join(names, "\n"), nil
}

type makeDirArgs struct {
	Path string `json:"path" jsonschema:"description=Directory to create; relative to the workspace (parents are created as needed)"`
}

// makeDir creates a directory inside the workspace, including any missing
// parent directories. Recreating an existing directory is a no-op.
func makeDir(args makeDirArgs) (string, error) {
	rel, err := Workspace.resolve(args.Path)
	if err != nil {
		return "", err
	}

	// os.OpenRoot confines the open to the workspace root: even if a
	// symlink inside the workspace points outside, the open fails.
	root, err := os.OpenRoot(Workspace.root)
	if err != nil {
		return "", fmt.Errorf("open workspace: %w", err)
	}
	defer root.Close()

	// MkdirAll creates the whole chain and succeeds if the directory
	// already exists — but it errors if the path is an existing file, so
	// give that case a clearer message.
	if err := root.MkdirAll(rel, 0o755); err != nil {
		if info, statErr := root.Stat(rel); statErr == nil && !info.IsDir() {
			return "", fmt.Errorf("not a directory: %s", rel)
		}
		return "", fmt.Errorf("create directory: %w", err)
	}
	return fmt.Sprintf("created directory %s", rel), nil
}

type deleteFileArgs struct {
	Path string `json:"path" jsonschema:"description=Path to the file to delete; relative to the workspace"`
}

// deleteFile deletes a file from the workspace. It refuses to delete
// directories — only files.
func deleteFile(args deleteFileArgs) (string, error) {
	rel, err := Workspace.resolve(args.Path)
	if err != nil {
		return "", err
	}

	// os.OpenRoot confines the open to the workspace root: even if a
	// symlink inside the workspace points outside, the open fails.
	root, err := os.OpenRoot(Workspace.root)
	if err != nil {
		return "", fmt.Errorf("open workspace: %w", err)
	}
	defer root.Close()

	// Stat follows the final symlink but stays confined to the workspace:
	// a symlink to a directory (inside or pointing outside) is refused,
	// and a symlink to an outside file errors as an escape rather than
	// deleting the target.
	info, err := root.Stat(rel)
	if err != nil {
		return "", fmt.Errorf("stat path: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("is a directory, not a file: %s", rel)
	}

	// Remove deletes the link itself for symlinks, and never recurses into
	// directories — a directory can never be deleted through this tool.
	if err := root.Remove(rel); err != nil {
		return "", fmt.Errorf("delete file: %w", err)
	}
	return fmt.Sprintf("deleted %s", rel), nil
}
