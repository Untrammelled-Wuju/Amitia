package capability

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestB18ValidSchemaPasses(t *testing.T) {
	cache := NewJSONSchemaCache()

	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string"}
		}
	}`)

	err := cache.Validate(schema, json.RawMessage(`{"query": "test"}`))
	if err != nil {
		t.Fatalf("expected valid input, got: %v", err)
	}
}

func TestB18InvalidJSONSchemaRejected(t *testing.T) {
	cache := NewJSONSchemaCache()

	schema := json.RawMessage(`{"type":`)

	_, err := cache.Compile(schema)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestB18InvalidJSONSchemaTypeRejected(t *testing.T) {
	cache := NewJSONSchemaCache()

	schema := json.RawMessage(`{"type": 123}`)

	_, err := cache.Compile(schema)
	if err == nil {
		t.Fatal("expected error for invalid schema type")
	}
}

func TestB18InternalRefAllowed(t *testing.T) {
	cache := NewJSONSchemaCache()

	schema := json.RawMessage(`{
		"$defs": {
			"name": {"type": "string"}
		},
		"properties": {
			"name": {"$ref": "#/$defs/name"}
		}
	}`)

	err := cache.Validate(schema, json.RawMessage(`{"name": "test"}`))
	if err != nil {
		t.Fatalf("expected internal ref to be allowed, got: %v", err)
	}
}

func TestB18RemoteRefRejected(t *testing.T) {
	err := rejectExternalSchemaRefs(map[string]any{
		"$ref": "https://example.com/schema.json",
	})
	if err == nil {
		t.Fatal("expected remote $ref to be rejected")
	}
}

func TestB18FileRefRejected(t *testing.T) {
	err := rejectExternalSchemaRefs(map[string]any{
		"$ref": "file:///etc/passwd",
	})
	if err == nil {
		t.Fatal("expected file $ref to be rejected")
	}
}

func TestB18RelativeExternalRefRejected(t *testing.T) {
	err := rejectExternalSchemaRefs(map[string]any{
		"$ref": "./other-schema.json",
	})
	if err == nil {
		t.Fatal("expected relative external $ref to be rejected")
	}
}

func TestB18ModelToolRequiresInputSchema(t *testing.T) {
	cache := NewJSONSchemaCache()

	def := ToolDefinition{
		ID:        "test/tool",
		ModelName: "test_tool",
		Source:    ToolSourceBuiltin,
		Name:      "Test Tool",
	}

	err := cache.ValidateToolDefinition(def)
	if err == nil {
		t.Fatal("expected error: model tool must have input schema")
	}

	var schemaErr *SchemaContractError
	if err != nil {
		if e, ok := err.(*SchemaContractError); ok {
			schemaErr = e
		}
	}

	if schemaErr == nil {
		t.Fatal("expected SchemaContractError for missing input schema")
	}

	if schemaErr.Kind != "missing_input_schema" {
		t.Fatalf("expected kind 'missing_input_schema', got '%s'", schemaErr.Kind)
	}
}

func TestB18InternalToolAllowsEmptyInputSchema(t *testing.T) {
	cache := NewJSONSchemaCache()

	def := ToolDefinition{
		ID:       "test/internal",
		Source:   ToolSourceInternal,
		Name:     "Internal Tool",
		Internal: true,
	}

	err := cache.ValidateToolDefinition(def)
	if err != nil {
		t.Fatalf("expected no error for internal tool with empty schema: %v", err)
	}
}

func TestB18InvalidOutputSchemaRejected(t *testing.T) {
	cache := NewJSONSchemaCache()

	def := ToolDefinition{
		ID:        "test/tool",
		ModelName: "test_tool",
		Source:    ToolSourceBuiltin,
		Name:      "Test Tool",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {"q": {"type": "string"}}
		}`),
		OutputSchema: json.RawMessage(`{"type":}`),
	}

	err := cache.ValidateToolDefinition(def)
	if err == nil {
		t.Fatal("expected error for invalid output schema")
	}

	var schemaErr *SchemaContractError
	if e, ok := err.(*SchemaContractError); ok {
		schemaErr = e
	}
	if schemaErr == nil || schemaErr.Role != ToolSchemaOutput {
		t.Fatalf("expected SchemaContractError with role=output, got: %v", err)
	}
}

func TestB18ModelToolInputSchemaMustBeObjectContract(t *testing.T) {
	cache := NewJSONSchemaCache()

	def := ToolDefinition{
		ID:        "test/tool",
		ModelName: "test_tool",
		Source:    ToolSourceBuiltin,
		Name:      "Test Tool",
		InputSchema: json.RawMessage(`{
			"type": "string"
		}`),
	}

	err := cache.ValidateToolDefinition(def)
	if err == nil {
		t.Fatal("expected error: model tool input schema must be object contract")
	}

	var schemaErr *SchemaContractError
	if e, ok := err.(*SchemaContractError); ok {
		schemaErr = e
	}
	if schemaErr == nil || schemaErr.Kind != "must_be_object_contract" {
		t.Fatalf("expected 'must_be_object_contract' error, got: %v", err)
	}
}

func TestB18SchemaContractErrorDoesNotLeakInput(t *testing.T) {
	cache := NewJSONSchemaCache()

	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"count": {"type": "integer"}
		}
	}`)

	input := json.RawMessage(`{"count": "SUPER_SECRET_VALUE"}`)

	err := cache.Validate(schema, input)
	if err == nil {
		t.Fatal("expected validation error")
	}

	errStr := err.Error()
	if strings.Contains(errStr, "SUPER_SECRET_VALUE") {
		t.Fatalf("error should not leak input values: %s", errStr)
	}
}

func TestB18RegistryRejectsInvalidInputSchema(t *testing.T) {
	reg := NewToolRegistry()

	def := ToolDefinition{
		ID:          "test/invalid",
		ModelName:   "invalid_tool",
		Source:      ToolSourceBuiltin,
		Name:        "Invalid Tool",
		InputSchema: json.RawMessage(`{"type": 123}`),
	}

	err := reg.Register(context.Background(), def)
	if err == nil {
		t.Fatal("expected registry to reject tool with invalid input schema")
	}

	if reg.Count() != 0 {
		t.Fatalf("expected registry to remain empty, got %d tools", reg.Count())
	}
}

func TestB18RegistryRejectsModelToolWithoutInputSchema(t *testing.T) {
	reg := NewToolRegistry()

	def := ToolDefinition{
		ID:        "test/no-schema",
		ModelName: "no_schema_tool",
		Source:    ToolSourceBuiltin,
		Name:      "No Schema Tool",
	}

	err := reg.Register(context.Background(), def)
	if err == nil {
		t.Fatal("expected registry to reject model tool without input schema")
	}

	if reg.Count() != 0 {
		t.Fatalf("expected registry to remain empty, got %d tools", reg.Count())
	}
}

func TestB18BatchRegisterAtomicOnInvalidSchema(t *testing.T) {
	reg := NewToolRegistry()

	validDef := ToolDefinition{
		ID:        "batch/valid",
		ModelName: "valid_tool",
		Source:    ToolSourceBuiltin,
		Name:      "Valid",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {"q": {"type": "string"}}
		}`),
	}

	invalidDef := ToolDefinition{
		ID:          "batch/invalid",
		ModelName:   "invalid_tool",
		Source:      ToolSourceBuiltin,
		Name:        "Invalid",
		InputSchema: json.RawMessage(`{"type": 123}`),
	}

	err := reg.BatchRegister(context.Background(), []ToolDefinition{validDef, invalidDef})
	if err == nil {
		t.Fatal("expected batch register to fail on invalid schema")
	}

	if reg.Count() != 0 {
		t.Fatalf("expected registry to remain empty after failed batch, got %d tools", reg.Count())
	}
}

func TestB18RegistryRejectsInputSchemaWithExternalRef(t *testing.T) {
	reg := NewToolRegistry()

	def := ToolDefinition{
		ID:        "test/external-ref",
		ModelName: "external_ref_tool",
		Source:    ToolSourceBuiltin,
		Name:      "External Ref Tool",
		InputSchema: json.RawMessage(`{
			"$ref": "https://evil.example/schema.json"
		}`),
	}

	err := reg.Register(context.Background(), def)
	if err == nil {
		t.Fatal("expected registry to reject tool with external $ref in input schema")
	}

	if reg.Count() != 0 {
		t.Fatalf("expected registry to remain empty, got %d tools", reg.Count())
	}
}

func TestB18RegistryAcceptsValidComplexSchema(t *testing.T) {
	reg := NewToolRegistry()

	def := ToolDefinition{
		ID:        "test/complex",
		ModelName: "complex_tool",
		Source:    ToolSourceBuiltin,
		Name:      "Complex Tool",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"additionalProperties": false,
			"required": ["query", "mode"],
			"properties": {
				"query": {
					"type": "string",
					"minLength": 1
				},
				"mode": {
					"type": "string",
					"enum": ["fast", "deep"]
				},
				"filters": {
					"type": "array",
					"items": {
						"type": "object",
						"required": ["key"],
						"properties": {
							"key": {"type": "string"}
						}
					}
				}
			}
		}`),
	}

	err := reg.Register(context.Background(), def)
	if err != nil {
		t.Fatalf("expected valid complex schema to be accepted, got: %v", err)
	}

	if reg.Count() != 1 {
		t.Fatalf("expected 1 tool registered, got %d", reg.Count())
	}
}

func TestB18BatchReplaceAtomicOnInvalidSchema(t *testing.T) {
	reg := NewToolRegistry()

	existing := ToolDefinition{
		ID:          "batch-replace/existing",
		ExtensionID: "com.test",
		ModelName:   "existing_tool",
		Source:      ToolSourcePlugin,
		Name:        "Existing",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {"q": {"type": "string"}}
		}`),
	}

	err := reg.Register(context.Background(), existing)
	if err != nil {
		t.Fatalf("unexpected error registering existing: %v", err)
	}

	invalidBatch := ToolDefinition{
		ID:          "batch-replace/existing",
		ExtensionID: "com.test",
		ModelName:   "updated_tool",
		Source:      ToolSourcePlugin,
		Name:        "Updated",
		InputSchema: json.RawMessage(`{"type": 123}`),
	}

	err = reg.BatchReplace(context.Background(), []ToolDefinition{invalidBatch})
	if err == nil {
		t.Fatal("expected batch replace to fail on invalid schema")
	}

	retrieved, ok := reg.Get(context.Background(), "batch-replace/existing")
	if !ok {
		t.Fatal("expected existing tool to remain after failed batch replace")
	}
	if retrieved.Name != "Existing" {
		t.Fatalf("expected original tool to remain, got Name=%s", retrieved.Name)
	}
}
