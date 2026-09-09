// Package tools registry: stores tools, generates schemas, dispatches calls.
package tools

import (
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strings"

	"github.com/invopop/jsonschema"
)

// Tool is a registered tool — wraps a Go function with its metadata.
type Tool struct {
	Name        string
	Description string
	Function    func(arguments map[string]any) (any, error)
	Schema      map[string]any
}

// ToolRegistry holds tools, exposes their schemas, dispatches incoming calls.
type ToolRegistry struct {
	// Tools indexed by name for O(1) dispatch lookup.
	tools map[string]Tool
}

// NewToolRegistry returns an empty registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
// Last-write-wins if the same name is registered twice.
func (r *ToolRegistry) Register(tool Tool) {
	r.tools[tool.Name] = tool
}

// GetSchemas returns the list of tool schemas in the OpenAI tools= format.
func (r *ToolRegistry) GetSchemas() []map[string]any {
	// Sort by name so the output is deterministic — Go maps iterate in random
	// order, while the model and any tests prefer a stable tools list.
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	// Wrap each tool's schema in OpenAI's required {"type": "function"} envelope.
	schemas := make([]map[string]any, 0, len(names))
	for _, name := range names {
		t := r.tools[name]
		schemas = append(schemas, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Schema,
			},
		})
	}
	return schemas
}

// Dispatch calls the named tool with the given arguments. Returns its string output.
func (r *ToolRegistry) Dispatch(name string, arguments map[string]any) string {
	// Reject unknown tool names — return an error the model can read and recover from.
	t, ok := r.tools[name]
	if !ok {
		return fmt.Sprintf("error: unknown tool '%s'", name)
	}

	// Run the tool, catching any error so a buggy call doesn't crash the loop.
	// The model sees the error string and decides what to do next.
	result, err := t.Function(arguments)
	if err != nil {
		return fmt.Sprintf("error: %T: %v", err, err)
	}
	return fmt.Sprintf("%v", result)
}

// Registry is the single global registry the rest of the harness imports.
var Registry = NewToolRegistry()

// toolReflector is configured to emit the flat object schemas OpenAI's tools
// format expects: no $id (Anonymous), the top-level struct inlined instead of
// a $ref (ExpandedStruct), and no $defs section (DoNotReference).
// AllowAdditionalProperties is set explicitly (as in OpenAI's structured
// outputs example) — it is the library default for structs, but spelling it
// out documents intent and is required if we ever enable strict tool calling.
var toolReflector = &jsonschema.Reflector{
	Anonymous:                 true,
	ExpandedStruct:            true,
	DoNotReference:            true,
	AllowAdditionalProperties: false,
}

// RegisterTool registers fn as a tool in the given registry — the Go
// equivalent of Python's @tool decorator. Real tools register into the global
// Registry from init(); tests and other isolated setups pass their own
// registry instead.
//
// Go has no decorators and cannot read doc comments at runtime, so the
// description is passed explicitly and registration happens from init():
//
//	type readFileArgs struct {
//		Path string `json:"path" jsonschema:"description=Path to the file to read"`
//	}
//
//	func readFile(args readFileArgs) (string, error) { ... }
//
//	func init() { RegisterTool(Registry, "Read the contents of a file", readFile) }
//
// fn must have the signature func(T) (R, error), where T is a struct type
// describing the tool's parameters. The tool's name is the function's name.
func RegisterTool[T any, R any](r *ToolRegistry, description string, fn func(T) (R, error)) {
	// Validate that T is a struct type — schema generation and argument binding
	// require a struct with json tags.
	var t T
	if reflect.TypeOf(t).Kind() != reflect.Struct {
		panic(fmt.Sprintf("tools: RegisterTool requires a struct type for parameters, got %T", t))
	}

	// Step 1: extract metadata from the function itself.
	name := funcName(fn)

	// Step 2: build a JSON schema for the function's parameters via reflection.
	// This plays the role pydantic's TypeAdapter plays in the Python version.
	schema, err := schemaFor[T]()
	if err != nil {
		panic(fmt.Sprintf("tools: cannot generate schema for %s: %v", name, err))
	}

	// Step 3: register the tool in the registry.
	// The wrapper decodes the argument map into T before calling fn, so fn
	// itself works with a plain typed struct.
	r.Register(Tool{
		Name:        name,
		Description: description,
		Function: func(arguments map[string]any) (any, error) {
			var args T
			if err := remarshal(arguments, &args); err != nil {
				return nil, err
			}
			return fn(args)
		},
		Schema: schema,
	})
}

// funcName returns the short name of fn, e.g. "readFile" — the Go stand-in
// for Python's func.__name__.
func funcName(fn any) string {
	full := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	// Strip the package path: "module/pkg.readFile" -> "readFile".
	if i := strings.LastIndex(full, "."); i >= 0 {
		return full[i+1:]
	}
	return full
}

// remarshal converts a decoded argument map into the tool's typed parameter
// struct by round-tripping through JSON — the stand-in for Python's **kwargs
// call, where the runtime binds a dict to typed parameters.
func remarshal(arguments map[string]any, target any) error {
	raw, err := json.Marshal(arguments)
	if err != nil {
		return fmt.Errorf("tools: cannot encode arguments: %w", err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("tools: cannot decode arguments: %w", err)
	}
	return nil
}

// schemaFor builds an OpenAI-compatible JSON schema from the struct type T
// using the invopop/jsonschema library — the Go stand-in for pydantic's
// TypeAdapter in the Python version.
//
// Parameter names come from `json` tags, per-parameter descriptions from
// `jsonschema:"description=..."` tags, and a field is required unless its
// `json` tag marks it with `omitempty`.
func schemaFor[T any]() (map[string]any, error) {
	var v T
	schema := toolReflector.Reflect(&v)
	// Drop the "$schema" key — OpenAI's tools format doesn't expect it.
	schema.Version = ""

	// Round-trip through JSON to hand back the plain map Tool.Schema stores.
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("tools: cannot marshal schema for %T: %w", v, err)
	}

	// Decode into map[string]json.RawMessage (as in OpenAI's structured
	// outputs example): values stay as raw JSON bytes so integer constraints
	// are not rounded through float64 before the SDK serializes the request.
	var rawSchema map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawSchema); err != nil {
		return nil, fmt.Errorf("tools: cannot decode schema for %T: %w", v, err)
	}
	out := make(map[string]any, len(rawSchema))
	for key, value := range rawSchema {
		out[key] = value
	}
	return out, nil
}
