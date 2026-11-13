package tools

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/dkr290/go-harness-coding-agent/agent-v2/config"
)

// withBashConfig overrides the global bash allow/deny configuration for the
// duration of a test and restores it afterwards.
func withBashConfig(t *testing.T, allow, deny []string, allowAll bool) {
	t.Helper()
	origAllow, origDeny, origAllowAll := config.BashAllowlist, config.BashDenylist, config.BashAllowAll
	config.BashAllowlist, config.BashDenylist, config.BashAllowAll = allow, deny, allowAll
	t.Cleanup(func() {
		config.BashAllowlist, config.BashDenylist, config.BashAllowAll = origAllow, origDeny, origAllowAll
	})
}

func TestSplitChain(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"single", "ls -la", []string{"ls -la"}},
		{"and", "go build && go test", []string{"go build ", " go test"}},
		{"or", "foo || bar", []string{"foo ", " bar"}},
		{"semicolon", "a; b", []string{"a", " b"}},
		{"pipe", "cat f | grep x", []string{"cat f ", " grep x"}},
		{"mixed", "a && b || c ; d | e", []string{"a ", " b ", " c ", " d ", " e"}},
		{"quoted semicolon", `echo "a;b"`, []string{`echo "a;b"`}},
		{"quoted pipe", `echo 'a|b' && ls`, []string{`echo 'a|b' `, " ls"}},
		{"quoted and", `echo "a&&b"`, []string{`echo "a&&b"`}},
		{"lone ampersand kept", "sleep 1 &", []string{"sleep 1 &"}},
		{"empty segments", "a &&  && b", []string{"a ", "  ", " b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := splitChain(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("splitChain(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

func TestFirstToken(t *testing.T) {
	if got := firstToken("  go build ./..."); got != "go" {
		t.Errorf("firstToken = %q, want %q", got, "go")
	}
	if got := firstToken("   "); got != "" {
		t.Errorf("firstToken of blank = %q, want empty", got)
	}
}

func TestValidateBinary(t *testing.T) {
	allow := []string{"ls", "echo", "both"}
	deny := []string{"rm", "both"}

	t.Run("allowed command passes", func(t *testing.T) {
		withBashConfig(t, allow, deny, false)
		if err := validateBinary("ls"); err != nil {
			t.Errorf("validateBinary(ls) = %v, want nil", err)
		}
	})

	t.Run("unlisted command rejected", func(t *testing.T) {
		withBashConfig(t, allow, deny, false)
		err := validateBinary("curl")
		if err == nil || !strings.Contains(err.Error(), "command not allowed: curl") {
			t.Errorf("validateBinary(curl) = %v, want not-allowed error", err)
		}
	})

	t.Run("denied command rejected", func(t *testing.T) {
		withBashConfig(t, allow, deny, false)
		err := validateBinary("rm")
		if err == nil || !strings.Contains(err.Error(), "command denied: rm") {
			t.Errorf("validateBinary(rm) = %v, want denied error", err)
		}
	})

	t.Run("deny wins on conflict", func(t *testing.T) {
		withBashConfig(t, allow, deny, false)
		err := validateBinary("both")
		if err == nil || !strings.Contains(err.Error(), "command denied: both") {
			t.Errorf("validateBinary(both) = %v, want denied error", err)
		}
	})

	t.Run("deny wins on conflict even with allow-all", func(t *testing.T) {
		withBashConfig(t, allow, deny, true)
		if err := validateBinary("both"); err == nil {
			t.Error("validateBinary(both) with allow-all = nil, want denied error")
		}
	})

	t.Run("allow-all bypasses allowlist", func(t *testing.T) {
		withBashConfig(t, allow, deny, true)
		if err := validateBinary("anything"); err != nil {
			t.Errorf("validateBinary(anything) with allow-all = %v, want nil", err)
		}
	})

	t.Run("empty allowlist permits everything not denied", func(t *testing.T) {
		withBashConfig(t, nil, deny, false)
		if err := validateBinary("anything"); err != nil {
			t.Errorf("validateBinary(anything) with empty allowlist = %v, want nil", err)
		}
		if err := validateBinary("rm"); err == nil {
			t.Error("validateBinary(rm) with empty allowlist = nil, want denied error")
		}
	})

	t.Run("empty denylist denies nothing", func(t *testing.T) {
		withBashConfig(t, allow, nil, true)
		if err := validateBinary("rm"); err != nil {
			t.Errorf("validateBinary(rm) with empty denylist = %v, want nil", err)
		}
	})
}

func TestValidateCommandChains(t *testing.T) {
	withBashConfig(t, []string{"ls", "echo", "go"}, []string{"rm"}, false)

	if err := validateCommand("go build ./... && go test ./..."); err != nil {
		t.Errorf("validateCommand(valid chain) = %v, want nil", err)
	}
	if err := validateCommand(`echo "a;b" | grep a`); err == nil ||
		!strings.Contains(err.Error(), "command not allowed: grep") {
		t.Errorf("validateCommand(pipe to unlisted) = %v, want not-allowed error for grep", err)
	}
	err := validateCommand("ls && rm -rf x")
	if err == nil || !strings.Contains(err.Error(), "command denied: rm") {
		t.Errorf("validateCommand(chain with denied) = %v, want denied error for rm", err)
	}
	if err := validateCommand(`echo "rm -rf /"`); err != nil {
		t.Errorf("validateCommand(quoted separator text) = %v, want nil", err)
	}
}

// TestBashCommandEndToEnd runs permitted and rejected commands through the
// real tool handler against the real workspace.
func TestBashCommandEndToEnd(t *testing.T) {
	withBashConfig(t, []string{"echo"}, []string{"ls"}, false)
	ctx := context.Background()

	out, err := bashCommand(ctx, bashCommandArgs{Command: "echo hello && echo world"})
	if err != nil {
		t.Fatalf("bashCommand(echo chain): %v", err)
	}
	if !strings.Contains(out, "hello") || !strings.Contains(out, "world") {
		t.Errorf("unexpected output: %q", out)
	}

	if _, err := bashCommand(ctx, bashCommandArgs{Command: "echo hi && ls"}); err == nil ||
		!strings.Contains(err.Error(), "command denied: ls") {
		t.Errorf("expected denied error for chained ls, got %v", err)
	}

	if _, err := bashCommand(ctx, bashCommandArgs{Command: "  "}); err == nil {
		t.Error("expected error for empty command")
	}
}
