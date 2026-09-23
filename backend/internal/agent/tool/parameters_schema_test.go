package tool

import (
	"encoding/json"
	"testing"
)

func TestB18ParseParametersSchemaPreservesComplexFields(t *testing.T) {
	complexSchema := json.RawMessage(`{
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
						"key": {
							"type": "string"
						}
					}
				}
			}
		}
	}`)

	params, err := ParseParametersSchema(complexSchema)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	encoded, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if decoded["type"] != "object" {
		t.Fatalf("expected type 'object', got %v", decoded["type"])
	}

	if ap, ok := decoded["additionalProperties"]; !ok || ap != false {
		t.Fatalf("expected additionalProperties=false, got %v", ap)
	}

	properties, ok := decoded["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties to be a map")
	}

	query, ok := properties["query"].(map[string]any)
	if !ok {
		t.Fatal("expected query property")
	}
	if ml, ok := query["minLength"]; !ok || ml != float64(1) {
		t.Fatalf("expected minLength=1, got %v", ml)
	}

	mode, ok := properties["mode"].(map[string]any)
	if !ok {
		t.Fatal("expected mode property")
	}
	if enum, ok := mode["enum"].([]any); !ok || len(enum) != 2 {
		t.Fatalf("expected enum with 2 items, got %v", mode["enum"])
	}
}

func TestB18ParseParametersSchemaRejectsEmpty(t *testing.T) {
	_, err := ParseParametersSchema(json.RawMessage{})
	if err == nil {
		t.Fatal("expected error for empty schema")
	}
}

func TestB18ParseParametersSchemaRejectsInvalidJSON(t *testing.T) {
	_, err := ParseParametersSchema(json.RawMessage(`{"type":`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseParametersSchemaSupportsUnionPropertyTypes(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"required": ["namespace", "key"],
		"properties": {
			"namespace": {
				"type": "string",
				"enum": ["system", "secure", "global"]
			},
			"key": {
				"type": "string"
			},
			"value": {
				"type": ["string", "number", "boolean", "null"]
			}
		},
		"additionalProperties": false
	}`)

	params, err := ParseParametersSchema(schema)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	encoded, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	properties := decoded["properties"].(map[string]any)
	value := properties["value"].(map[string]any)
	types, ok := value["type"].([]any)
	if !ok || len(types) != 4 {
		t.Fatalf("expected union type to be preserved, got %v", value["type"])
	}
	if params.Properties["value"].Type != "string" {
		t.Fatalf("expected first non-null type to be decoded, got %q", params.Properties["value"].Type)
	}
}

func TestB18MarshalJSONFallsBackToTypedFields(t *testing.T) {
	params := Parameters{
		Type: "object",
		Properties: map[string]Property{
			"name": {Type: "string", Description: "The name"},
		},
		Required: []string{"name"},
	}

	encoded, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if decoded["type"] != "object" {
		t.Fatalf("expected type 'object', got %v", decoded["type"])
	}

	if req, ok := decoded["required"].([]any); !ok || len(req) != 1 || req[0] != "name" {
		t.Fatalf("expected required=['name'], got %v", decoded["required"])
	}
}
