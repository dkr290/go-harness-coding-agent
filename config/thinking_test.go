package config

import (
	"encoding/json"
	"os"
	"testing"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

func TestThinkingEnv(t *testing.T) {
	cases := []struct {
		env  string
		want bool
	}{
		{"", true},
		{"1", true},
		{"true", true},
		{"YES", true},
		{"0", false},
		{"false", false},
		{"off", false},
		{"garbage", true},
	}
	for _, c := range cases {
		os.Setenv("OPENAI_THINKING", c.env)
		got := envBool("OPENAI_THINKING", true)
		if got != c.want {
			t.Errorf("envBool(%q) = %v, want %v", c.env, got, c.want)
		}
	}
}

func TestThinkingMarshal(t *testing.T) {
	base := openai.ChatCompletionNewParams{
		Model:    "test",
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("hi")},
	}

	// thinking enabled: no reasoning_effort sent, model uses its default
	b, err := json.Marshal(base)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if contains(string(b), "reasoning_effort") {
		t.Errorf("thinking enabled: expected reasoning_effort omitted, got %s", b)
	}

	// thinking disabled: explicit none
	base.ReasoningEffort = shared.ReasoningEffortNone
	b, err = json.Marshal(base)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !contains(string(b), `"reasoning_effort":"none"`) {
		t.Errorf("thinking disabled: expected reasoning_effort none, got %s", b)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
