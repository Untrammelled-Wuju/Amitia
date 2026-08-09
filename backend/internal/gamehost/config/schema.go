package config

import (
	"encoding/json"
)

type ConfigFieldType string

const (
	ConfigTypeString  ConfigFieldType = "string"
	ConfigTypeInteger ConfigFieldType = "integer"
	ConfigTypeNumber  ConfigFieldType = "number"
	ConfigTypeBoolean ConfigFieldType = "boolean"
	ConfigTypeObject  ConfigFieldType = "object"
	ConfigTypeArray   ConfigFieldType = "array"
)

type ConfigScope string

const (
	ConfigScopePlugin  ConfigScope = "plugin"
	ConfigScopeRuntime ConfigScope = "runtime"
	ConfigScopeService ConfigScope = "service"
)

type ConfigField struct {
	Key          string           `json:"key"`
	Type         ConfigFieldType  `json:"type"`
	Required     bool             `json:"required"`
	Secret       bool             `json:"secret"`
	Default      json.RawMessage  `json:"default,omitempty"`
	Description  string           `json:"description,omitempty"`
	Enum         []json.RawMessage `json:"enum,omitempty"`
	Minimum      *float64         `json:"minimum,omitempty"`
	Maximum      *float64         `json:"maximum,omitempty"`
	MinLength    *int             `json:"minLength,omitempty"`
	MaxLength    *int             `json:"maxLength,omitempty"`
	Metadata     map[string]json.RawMessage `json:"metadata,omitempty"`
}

type ConfigSchema struct {
	SchemaVersion int           `json:"schemaVersion"`
	Fields        []ConfigField `json:"fields"`
	fieldIndex    map[string]int
}

func NewSchema(fields []ConfigField) *ConfigSchema {
	idx := make(map[string]int, len(fields))
	for i, f := range fields {
		idx[f.Key] = i
	}
	return &ConfigSchema{
		SchemaVersion: 1,
		Fields:        fields,
		fieldIndex:    idx,
	}
}

func (s *ConfigSchema) Field(key string) (ConfigField, bool) {
	if i, ok := s.fieldIndex[key]; ok {
		return s.Fields[i], true
	}
	return ConfigField{}, false
}

func (s *ConfigSchema) HasField(key string) bool {
	_, ok := s.fieldIndex[key]
	return ok
}

func (s *ConfigSchema) Clone() *ConfigSchema {
	fields := make([]ConfigField, len(s.Fields))
	copy(fields, s.Fields)
	return NewSchema(fields)
}

func ScopePriority(scope ConfigScope) int {
	switch scope {
	case ConfigScopePlugin:
		return 0
	case ConfigScopeRuntime:
		return 1
	case ConfigScopeService:
		return 2
	default:
		return -1
	}
}
