package config

import (
	"os"
	"strings"
)

const (
	DefaultModel   = "gpt-4o-mini"
	DefaultBaseURL = "https://api.openai.com/v1"
)

var (
	APIKey   = envOrDefault("OPENAI_API_KEY", "")
	Model    = envOrDefault("OPENAI_MODEL", DefaultModel)
	BaseURL  = envOrDefault("OPENAI_BASE_URL", DefaultBaseURL)
	Thinking = envBool("OPENAI_THINKING", true)
	BashAllowlist = getBashAllowlist()
)

func getBashAllowlist() []string {
	v := os.Getenv("BASH_ALLOWLIST")
	if v != "" {
		// User-provided comma-separated list.
		list := make([]string, 0)
		for _, cmd := range strings.Split(v, ",") {
			cmd = strings.TrimSpace(cmd)
			if cmd != "" {
				list = append(list, cmd)
			}
		}
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
