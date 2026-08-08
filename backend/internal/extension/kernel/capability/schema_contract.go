package capability

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type ToolSchemaRole string

const (
	ToolSchemaInput  ToolSchemaRole = "input"
	ToolSchemaOutput ToolSchemaRole = "output"
)

type SchemaContractError struct {
	ToolID string
	Role   ToolSchemaRole
	Kind   string
	Cause  error
}

func (e *SchemaContractError) Error() string {
	if e == nil {
		return "tool schema validation failed"
	}

	return fmt.Sprintf(
		"tool %s %s schema %s",
		e.ToolID,
		e.Role,
		e.Kind,
	)
}

func (e *SchemaContractError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Cause
}

type JSONSchemaCache struct {
	mu      sync.RWMutex
	entries map[string]*jsonschema.Schema
}

func NewJSONSchemaCache() *JSONSchemaCache {
	return &JSONSchemaCache{
		entries: make(map[string]*jsonschema.Schema),
	}
}

func schemaCacheKey(raw json.RawMessage) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func rejectExternalSchemaRefs(value any) error {
	switch current := value.(type) {
	case map[string]any:
		for _, key := range []string{
			"$ref",
			"$dynamicRef",
			"$recursiveRef",
		} {
			rawRef, exists := current[key]
			if !exists {
				continue
			}

			ref, ok := rawRef.(string)
			if !ok {
				return fmt.Errorf(
					"%s must be a string",
					key,
				)
			}

			if ref != "" && !strings.HasPrefix(ref, "#") {
				return fmt.Errorf(
					"external schema reference is not allowed",
				)
			}
		}

		for _, child := range current {
			if err := rejectExternalSchemaRefs(child); err != nil {
				return err
			}
		}

	case []any:
		for _, child := range current {
			if err := rejectExternalSchemaRefs(child); err != nil {
				return err
			}
		}
	}

	return nil
}

func compileJSONSchema(raw json.RawMessage) (*jsonschema.Schema, error) {
	raw = bytes.TrimSpace(raw)

	if len(raw) == 0 {
		return nil, nil
	}

	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid schema json: %w", err)
	}

	if err := rejectExternalSchemaRefs(document); err != nil {
		return nil, err
	}

	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)

	key := "urn:amitia:tool-schema:" + schemaCacheKey(raw)

	if err := compiler.AddResource(key, document); err != nil {
		return nil, err
	}

	compiled, err := compiler.Compile(key)
	if err != nil {
		return nil, err
	}

	return compiled, nil
}

func (c *JSONSchemaCache) Compile(raw json.RawMessage) (*jsonschema.Schema, error) {
	raw = bytes.TrimSpace(raw)

	if len(raw) == 0 {
		return nil, nil
	}

	cacheKey := schemaCacheKey(raw)

	c.mu.RLock()
	if cached, ok := c.entries[cacheKey]; ok {
		c.mu.RUnlock()
		return cached, nil
	}
	c.mu.RUnlock()

	compiled, err := compileJSONSchema(raw)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.entries[cacheKey] = compiled
	c.mu.Unlock()

	return compiled, nil
}

func decodeSchemaInstance(raw json.RawMessage) (any, error) {
	raw = bytes.TrimSpace(raw)

	if len(raw) == 0 {
		return nil, fmt.Errorf("empty json instance")
	}

	return jsonschema.UnmarshalJSON(bytes.NewReader(raw))
}

func (c *JSONSchemaCache) Validate(schemaRaw json.RawMessage, valueRaw json.RawMessage) error {
	compiled, err := c.Compile(schemaRaw)
	if err != nil {
		return err
	}

	if compiled == nil {
		return nil
	}

	value, err := decodeSchemaInstance(valueRaw)
	if err != nil {
		return err
	}

	return compiled.Validate(value)
}

func (c *JSONSchemaCache) ValidateToolDefinition(def ToolDefinition) error {
	if len(bytes.TrimSpace(def.InputSchema)) > 0 {
		if err := c.validateInputSchema(def); err != nil {
			return err
		}
	} else if def.ModelName != "" {
		return &SchemaContractError{
			ToolID: def.ID,
			Role:   ToolSchemaInput,
			Kind:   "missing_input_schema",
		}
	}

	if len(bytes.TrimSpace(def.OutputSchema)) > 0 {
		if err := c.validateOutputSchema(def); err != nil {
			return err
		}
	}

	return nil
}

func (c *JSONSchemaCache) validateInputSchema(def ToolDefinition) error {
	compiled, err := c.Compile(def.InputSchema)
	if err != nil {
		return &SchemaContractError{
			ToolID: def.ID,
			Role:   ToolSchemaInput,
			Kind:   "invalid_schema",
			Cause:  err,
		}
	}

	if compiled == nil {
		return nil
	}

	if def.ModelName != "" {
		if !inputSchemaIsObjectContract(def.InputSchema) {
			return &SchemaContractError{
				ToolID: def.ID,
				Role:   ToolSchemaInput,
				Kind:   "must_be_object_contract",
			}
		}
	}

	return nil
}

func (c *JSONSchemaCache) validateOutputSchema(def ToolDefinition) error {
	_, err := c.Compile(def.OutputSchema)
	if err != nil {
		return &SchemaContractError{
			ToolID: def.ID,
			Role:   ToolSchemaOutput,
			Kind:   "invalid_schema",
			Cause:  err,
		}
	}

	return nil
}

func inputSchemaIsObjectContract(raw json.RawMessage) bool {
	var root struct {
		Type any `json:"type"`
	}

	if err := json.Unmarshal(raw, &root); err != nil {
		return false
	}

	switch v := root.Type.(type) {
	case string:
		return v == "object"
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s == "object" {
				return true
			}
		}
	}

	return false
}
