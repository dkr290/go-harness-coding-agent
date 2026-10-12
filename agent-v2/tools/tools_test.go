package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microsoft/agent-framework-go/tool"

	"github.com/dkr290/go-harness-coding-agent/agent-v2/templates"
)

// findTool returns the FuncTool with the given name, or fails.
func findTool(t *testing.T, name string) tool.FuncTool {
	t.Helper()
	for _, tl := range FilesystemTools() {
		if ft, ok := tl.(tool.FuncTool); ok && ft.Name() == name {
			return ft
		}
	}
	t.Fatalf("tool %q not found", name)
	return nil
}

// TestToolSchemas verifies every tool exposes a name, description and input
// schema — the framework generates these from the handler's argument struct.
func TestToolSchemas(t *testing.T) {
	tls := FilesystemTools()
	if len(tls) != 5 {
		t.Fatalf("expected 5 filesystem tools, got %d", len(tls))
	}
	for _, tl := range tls {
		if tl.Name() == "" {
			t.Error("tool with empty name")
		}
		if tl.Description() == "" {
			t.Errorf("tool %q has empty description", tl.Name())
		}
		st, ok := tl.(tool.SchemaTool)
		if !ok {
			t.Fatalf("tool %q does not expose a schema", tl.Name())
		}
		if st.Schema() == nil {
			t.Errorf("tool %q has nil schema", tl.Name())
		}
	}
}

// TestWriteThenReadFile exercises argument binding and dispatch end-to-end
// through the framework's Call interface, using the real workspace.
func TestWriteThenReadFile(t *testing.T) {
	ctx := context.Background()

	out, err := findTool(t, "writeFile").Call(ctx, `{"path": "test/hello.txt", "content": "hi gopher"}`)
	if err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	if !strings.Contains(out.(string), "wrote 9 bytes") {
		t.Errorf("unexpected writeFile output: %v", out)
	}

	out, err = findTool(t, "readFile").Call(ctx, `{"path": "test/hello.txt"}`)
	if err != nil {
		t.Fatalf("readFile: %v", err)
	}
	if out.(string) != "hi gopher" {
		t.Errorf("readFile = %q, want %q", out, "hi gopher")
	}

	if _, err := findTool(t, "deleteFile").Call(ctx, `{"path": "test/hello.txt"}`); err != nil {
		t.Fatalf("deleteFile: %v", err)
	}
}

// TestAgentsMDSeeded verifies a fresh workspace gets AGENTS.md seeded from the
// embedded template.
func TestAgentsMDSeeded(t *testing.T) {
	root := t.TempDir()
	if _, err := newWorkspace(root); err != nil {
		t.Fatalf("newWorkspace: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read seeded AGENTS.md: %v", err)
	}
	if string(got) != templates.AgentsMD {
		t.Errorf("seeded AGENTS.md does not match the embedded template")
	}
}

// TestAgentsMDNotOverwritten verifies an existing AGENTS.md is preserved, so
// the agent's memory survives across sessions.
func TestAgentsMDNotOverwritten(t *testing.T) {
	root := t.TempDir()
	custom := "# My memory\n\nDo not overwrite me."
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(custom), 0o644); err != nil {
		t.Fatalf("write custom AGENTS.md: %v", err)
	}
	if _, err := newWorkspace(root); err != nil {
		t.Fatalf("newWorkspace: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if string(got) != custom {
		t.Errorf("AGENTS.md was overwritten, got: %q", got)
	}
}

// TestToolErrors verifies tool errors surface as Go errors the framework can
// relay back to the model.
func TestToolErrors(t *testing.T) {
	ctx := context.Background()

	if _, err := findTool(t, "readFile").Call(ctx, `{"path": "../escape.txt"}`); err == nil ||
		!strings.Contains(err.Error(), "escapes workspace") {
		t.Errorf("expected escape error, got %v", err)
	}
	if _, err := findTool(t, "readFile").Call(ctx, `{"path": "does/not/exist.txt"}`); err == nil {
		t.Error("expected error for missing file")
	}
	if _, err := findTool(t, "deleteFile").Call(ctx, `{"path": "."}`); err == nil ||
		!strings.Contains(err.Error(), "is a directory") {
		t.Errorf("expected directory refusal, got %v", err)
	}
}
