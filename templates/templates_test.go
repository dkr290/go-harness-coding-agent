package templates

import (
	"strings"
	"testing"
)

func TestSystemPromptIsEmbedded(t *testing.T) {
	if !strings.Contains(SystemPrompt, "coding assistant") {
		t.Errorf("SystemPrompt missing expected content, got: %q", SystemPrompt)
	}
}

func TestGet(t *testing.T) {
	got, err := Get("system.prompt.txt")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != SystemPrompt {
		t.Error("Get and SystemPrompt must return the same content")
	}
}

func TestGetMissing(t *testing.T) {
	if _, err := Get("does-not-exist.txt"); err == nil {
		t.Error("expected an error for a missing template, got nil")
	}
}

func TestListContainsSystemPrompt(t *testing.T) {
	names, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, n := range names {
		if n == "system.prompt.txt" {
			return
		}
	}
	t.Errorf("List() = %v, want it to contain system.prompt.txt", names)
}
