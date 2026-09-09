// Playground for learning how tools/registry.go works.
//
// Run it:  go run ./playground
// Then tweak the struct tags / types below and re-run to watch the
// generated JSON schema and dispatch behavior change.
//
// Each printed section below says which lines of tools/registry.go
// produced what you see.


package main

import (
	"encoding/json"
	"fmt"

	"github.com/dkr290/go-harness-coding-agent/tools"
)

// ---------------------------------------------------------------------------
// STEP A: Define a typed args struct + a plain Go function.
// This is ALL a tool author writes. Everything else (name, schema, argument
// binding) is derived by reflection in tools/registry.go.
//
// Try changing things here and re-running:
//   - remove `omitempty` from MaxLines  -> watch it appear in "required"
//   - rename the json tag `path` -> `file_path` -> watch the schema change
//   - rename the function readFile -> loadFile -> watch "name" change
// ---------------------------------------------------------------------------
type readFileArgs struct {
	Path     string `json:"path" jsonschema:"description=Path to the file to read"`
	MaxLines int    `json:"max_lines,omitempty" jsonschema:"description=Read at most this many lines"`
	Shout    bool   `json:"shout,omitempty" jsonschema:"description=UPPERCASE the result"`
}

func readFile(args readFileArgs) (string, error) {
	out := fmt.Sprintf("pretending to read %q (max_lines=%d)", args.Path, args.MaxLines)
	if args.Shout {
		out = "PRETENDING TO READ " + fmt.Sprintf("%q", args.Path)
	}
	return out, nil
}

func main() {
	// ---------------------------------------------------------------------
	// STEP B: RegisterToolTo(registry, description, fn)
	// Behind this ONE call, registry.go does (lines 117-150):
	//   1. line 121: reflect.TypeOf(t).Kind() -> panics unless T is a struct
	//   2. line 126: funcName(fn) -> runtime.FuncForPC + reflect.ValueOf(fn)
	//                 .Pointer() to recover the function's name "readFile"
	//   3. line 130: schemaFor[T]() -> invopop/jsonschema walks the struct
	//                 fields with reflect and builds the JSON schema
	//   4. line 138: stores a wrapper closure that will remarshal args into
	//                 readFileArgs before calling readFile
	// ---------------------------------------------------------------------
	r := tools.NewToolRegistry()
	tools.RegisterToolTo(r, "Read the contents of a file", readFile)

	// ---------------------------------------------------------------------
	// PRINT 1: The schema the LLM would see (GetSchemas, lines 41-64).
	// Everything under "parameters" came out of schemaFor's reflection:
	//   - Go type  string/int/bool   -> "type": "string"/"integer"/"boolean"
	//   - json:"path"                -> property name "path"
	//   - jsonschema:"description="  -> "description" per property
	//   - no omitempty on Path       -> "required": ["path"]
	// Also note "name": "readFile" — nobody typed that string; funcName
	// recovered it from the compiled binary's symbol table.
	// ---------------------------------------------------------------------
	fmt.Println("=== 1. GetSchemas() — output of schemaFor + funcName reflection ===")
	raw, _ := json.MarshalIndent(r.GetSchemas(), "", "  ")
	fmt.Println(string(raw))

	// ---------------------------------------------------------------------
	// PRINT 2: A successful Dispatch (lines 67-81).
	// The wrapper closure (line 141) calls remarshal: the untyped
	// map[string]any below is json.Marshal'ed, then json.Unmarshal'ed into
	// a typed readFileArgs, and readFile is called with it.
	// ---------------------------------------------------------------------
	fmt.Println("\n=== 2. Dispatch with valid arguments (remarshal: map -> JSON -> typed struct) ===")
	fmt.Println(r.Dispatch("readFile", map[string]any{
		"path":      "/etc/hostname",
		"max_lines": 10,
	}))
	fmt.Println(r.Dispatch("readFile", map[string]any{
		"path":  "/etc/hostname",
		"shout": true,
	}))

	// ---------------------------------------------------------------------
	// PRINT 3: Deliberately bad inputs — watch where errors come from.
	//   - unknown tool name  -> caught at line 69-72 (map lookup miss)
	//   - wrong JSON type    -> remarshal's json.Unmarshal fails (line 143)
	// Note: a MISSING "path" does NOT error — encoding/json just leaves the
	// field at its zero value. "required" is only enforced by the LLM API,
	// not by the registry.
	// ---------------------------------------------------------------------
	fmt.Println("\n=== 3. Error paths ===")
	fmt.Println("unknown tool :", r.Dispatch("nope", map[string]any{}))
	fmt.Println("wrong type   :", r.Dispatch("readFile", map[string]any{"path": 42}))
	fmt.Println("missing path :", r.Dispatch("readFile", map[string]any{"max_lines": 3}))
}
