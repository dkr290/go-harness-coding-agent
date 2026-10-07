// Package templates provides access to the embedded prompt templates.
//
// Every *.txt file in this directory is embedded into the binary at compile
// time automatically. However, to use a template from code (e.g. from the
// harness package), you MUST add a package-level variable for it below —
// dropping the file in is not enough on its own.
//
// How to add a new template, step by step:
//
//  1. Drop the file in this directory, e.g. file.1.txt
//  2. Add a variable in the var block below:  var File1 = MustGet("file.1.txt")
//  3. Reference it from code as:  templates.File1
//
// The variable panics at startup if the file is missing, so a wrong or
// forgotten file name fails fast instead of silently misbehaving later.
package templates

import (
	"embed"
	"fmt"
)

// files holds every *.txt template in this directory.
//
//go:embed *.txt
var files embed.FS

// Template variables — one per embedded file the program uses.
//
// REQUIRED STEP FOR EVERY NEW TEMPLATE: after dropping a new *.txt file in
// this directory, add a variable for it in this block, e.g.:
//
//	File1 = MustGet("file.1.txt")
//
// and then reference it from code as templates.File1. A missing file panics
// at startup, so typos and forgotten files fail fast.
var (
	// SystemPrompt is the agent's system prompt (system.prompt.txt).
	SystemPrompt = MustGet("system.prompt.txt")

	// Add new templates below, one per file:
	// File1 = MustGet("file.1.txt")
)

// Get returns the contents of the embedded template with the given name,
// e.g. Get("system.prompt.txt").
func Get(name string) (string, error) {
	b, err := files.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("templates: cannot load %q: %w", name, err)
	}
	return string(b), nil
}

// MustGet is like Get but panics when the template is missing. It is meant
// for package-level variables, where a missing template is a programming
// error that should fail fast at startup.
func MustGet(name string) string {
	s, err := Get(name)
	if err != nil {
		panic(err)
	}
	return s
}

// List returns the names of all embedded templates.
func List() ([]string, error) {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("templates: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
