// Deprecated: Legacy extension architecture.
// Do not add new capabilities. This implementation is retained only for
// compatibility, maintenance, testing, and migration to Extension Kernel.

package extension

import (
	"embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schema/manifest.schema.json
var schemaFS embed.FS

type SchemaValidator struct {
	manifest *jsonschema.Schema
	mu       sync.RWMutex
	compiled map[string]*jsonschema.Schema
}

func NewSchemaValidator() (*SchemaValidator, error) {
	raw, err := schemaFS.ReadFile("schema/manifest.schema.json")
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	var manifestSchema interface{}
	if err := json.Unmarshal(raw, &manifestSchema); err != nil {
		return nil, err
	}
	if err := compiler.AddResource("manifest.schema.json", manifestSchema); err != nil {
		return nil, err
	}
	manifest, err := compiler.Compile("manifest.schema.json")
	if err != nil {
		return nil, err
	}
	return &SchemaValidator{manifest: manifest, compiled: map[string]*jsonschema.Schema{}}, nil
}

func (v *SchemaValidator) ValidateManifest(raw json.RawMessage) error {
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("manifest json: %w", err)
	}
	if err := v.manifest.Validate(value); err != nil {
		return fmt.Errorf("manifest schema: %w", err)
	}
	return nil
}

func (v *SchemaValidator) ValidateSchema(name string, raw json.RawMessage) error {
	_, err := v.compile(name, raw)
	return err
}

func (v *SchemaValidator) Validate(name string, schemaRaw, valueRaw json.RawMessage) error {
	schema, err := v.compile(name, schemaRaw)
	if err != nil {
		return err
	}
	var value interface{}
	if err := json.Unmarshal(valueRaw, &value); err != nil {
		return fmt.Errorf("$ json invalid: %w", err)
	}
	if err := schema.Validate(value); err != nil {
		return err
	}
	return nil
}

func (v *SchemaValidator) compile(name string, raw json.RawMessage) (*jsonschema.Schema, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("schema is required")
	}
	key := name + "\x00" + string(raw)
	v.mu.RLock()
	compiled := v.compiled[key]
	v.mu.RUnlock()
	if compiled != nil {
		return compiled, nil
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	resource := name + ".schema.json"
	var schemaValue interface{}
	if err := json.Unmarshal(raw, &schemaValue); err != nil {
		return nil, err
	}
	if err := compiler.AddResource(resource, schemaValue); err != nil {
		return nil, err
	}
	compiled, err := compiler.Compile(resource)
	if err != nil {
		return nil, err
	}
	v.mu.Lock()
	v.compiled[key] = compiled
	v.mu.Unlock()
	return compiled, nil
}
