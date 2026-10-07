package tools

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// decodeRaw unmarshals a schema value into a plain map. Since schemaFor
// stores values as json.RawMessage (to preserve raw JSON), tests decode them
// before asserting on their contents.
func decodeRaw(t *testing.T, v any) map[string]any {
	t.Helper()
	raw, ok := v.(json.RawMessage)
	if !ok {
		t.Fatalf("expected json.RawMessage, got %T: %v", v, v)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("cannot decode schema value: %v", err)
	}
	return out
}

type greetArgs struct {
	Name  string `json:"name" jsonschema:"description=Who to greet"`
	Shout bool   `json:"shout,omitempty" jsonschema:"description=Greet in uppercase"`
}

func greet(args greetArgs) (string, error) {
	g := "hello, " + args.Name
	if args.Shout {
		g = strings.ToUpper(g)
	}
	return g, nil
}

func fail(_ struct{}) (string, error) {
	return "", errors.New("boom")
}

func TestRegisterToolAndDispatch(t *testing.T) {
	r := NewToolRegistry()
	RegisterTool(r, "Greet someone", greet)

	if got := r.Dispatch("greet", map[string]any{"name": "gopher"}); got != "hello, gopher" {
		t.Errorf("dispatch = %q, want %q", got, "hello, gopher")
	}
	if got := r.Dispatch("greet", map[string]any{"name": "gopher", "shout": true}); got != "HELLO, GOPHER" {
		t.Errorf("dispatch = %q, want %q", got, "HELLO, GOPHER")
	}
}

func TestDispatchErrors(t *testing.T) {
	r := NewToolRegistry()
	RegisterTool(r, "Always fails", fail)

	if got := r.Dispatch("missing", nil); !strings.Contains(got, "unknown tool 'missing'") {
		t.Errorf("unknown tool: got %q", got)
	}
	if got := r.Dispatch("fail", map[string]any{}); !strings.Contains(got, "boom") {
		t.Errorf("failing tool: got %q", got)
	}
}

func TestGetSchemas(t *testing.T) {
	r := NewToolRegistry()
	RegisterTool(r, "Greet someone", greet)

	schemas := r.GetSchemas()
	if len(schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(schemas))
	}
	if schemas[0]["type"] != "function" {
		t.Errorf("missing function envelope: %v", schemas[0])
	}

	fn, ok := schemas[0]["function"].(map[string]any)
	if !ok {
		t.Fatalf("missing function payload: %v", schemas[0])
	}
	if fn["name"] != "greet" || fn["description"] != "Greet someone" {
		t.Errorf("wrong metadata: %v", fn)
	}

	params, ok := fn["parameters"].(map[string]any)
	if !ok {
		t.Fatalf("missing parameters: %v", fn)
	}
	// Decoding the properties RawMessage turns nested values into plain maps.
	props := decodeRaw(t, params["properties"])
	if props["name"].(map[string]any)["type"] != "string" {
		t.Errorf("name should be a string: %v", props)
	}
	if props["shout"].(map[string]any)["type"] != "boolean" {
		t.Errorf("shout should be a boolean: %v", props)
	}

	// required is stored as json.RawMessage too; decode it into a slice.
	var required []string
	if err := json.Unmarshal(params["required"].(json.RawMessage), &required); err != nil {
		t.Fatalf("cannot decode required: %v", err)
	}
	if len(required) != 1 || required[0] != "name" {
		t.Errorf("required should be [name] (omitempty marks shout optional): %v", required)
	}
}

func TestRegisterToolRequiresStruct(t *testing.T) {
	// Non-struct type should panic at registration time.
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when registering non-struct type")
		}
	}()

	RegisterTool(NewToolRegistry(), "bad", func(s string) (string, error) { return s, nil })
}
