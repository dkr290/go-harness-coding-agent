package config

import (
	"os"
	"strconv"
	"strings"
)

const (
	DefaultModel      = "gpt-4o-mini"
	DefaultBaseURL    = "https://api.openai.com/v1"
	DefaultStepBudget = 25
)

var (
	APIKey        = envOrDefault("OPENAI_API_KEY", "")
	Model         = envOrDefault("OPENAI_MODEL", DefaultModel)
	BaseURL       = envOrDefault("OPENAI_BASE_URL", DefaultBaseURL)
	Thinking      = envBool("OPENAI_THINKING", true)
	BashAllowlist = getBashAllowlist()
	BashDenylist  = getBashDenylist()
	BashAllowAll  = envBool("ALLOW_ALL_COMMANDS", false)
	StepBudget    = envInt("STEP_BUDGET", DefaultStepBudget)
)

// getListEnv parses a comma-separated environment variable into a trimmed,
// non-empty list. It returns an empty slice when the variable is unset.
func getListEnv(key string) []string {
	v := os.Getenv(key)
	list := make([]string, 0)
	if v == "" {
		return list
	}
	for _, cmd := range strings.Split(v, ",") {
		cmd = strings.TrimSpace(cmd)
		if cmd != "" {
			list = append(list, cmd)
		}
	}
	return list
}

func getBashAllowlist() []string {
	if list := getListEnv("BASH_ALLOWLIST"); len(list) > 0 {
		return list
	}
	// Default allowlist.
	return []string{
		// Shell basics
		"cat", "echo", "ls", "pwd", "cd",
		"mkdir", "rm", "cp", "mv",
		"chmod", "chown",
		// Text processing
		"grep", "sed", "awk", "cut",
		"sort", "head", "tail", "wc",
		"uniq", "diff", "tr", "xxd",
		// File utilities
		"find", "file", "stat", "tree",
		"tar", "zip", "unzip",
		// Git
		"git",
		// System info
		"uname", "whoami", "hostname", "uptime",
		"df", "free", "ps", "top",
		// Networking
		"curl", "wget", "ping", "nslookup", "dig",
		// Programming
		"go", "python", "python3", "node",
		"npm", "yarn", "pip",
		"gcc", "make", "cmake",
		// Package managers
		"apt-get", "apt", "apk",
	}
}

// getBashDenylist returns the user-provided deny list, or the default deny
// list when BASH_DENYLIST is unset. Denied commands are always rejected,
// even if they also appear in the allowlist.
func getBashDenylist() []string {
	if list := getListEnv("BASH_DENYLIST"); len(list) > 0 {
		return list
	}
	// Default denylist.
	return []string{"rm", "sudo", "dd", "mkfs"}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 1 {
		return fallback
	}
	return n
}
