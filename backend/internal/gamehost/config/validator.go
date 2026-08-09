package config

import (
	"encoding/json"
	"fmt"
	"math"
)

type SecretProviderRegistry interface {
	KnownProviders() []string
}

type Validator struct {
	providerRegistry SecretProviderRegistry
	knownProviders   map[string]bool
}

func NewValidator(providerRegistry SecretProviderRegistry) *Validator {
	known := make(map[string]bool)
	if providerRegistry != nil {
		for _, p := range providerRegistry.KnownProviders() {
			known[p] = true
		}
	}
	return &Validator{
		providerRegistry: providerRegistry,
		knownProviders:   known,
	}
}

func (v *Validator) ValidateValue(key string, raw json.RawMessage, field ConfigField, scope ConfigScope) []ValidationError {
	if len(raw) == 0 || string(raw) == "null" {
		if field.Required {
			return []ValidationError{{
				Key:     key,
				Code:    ErrRequiredField,
				Message: "required field is missing",
			}}
		}
		return nil
	}

	var value any
	if err := json.Unmarshal(raw, &value); nil != err {
		return []ValidationError{{
			Key:     key,
			Code:    ErrInvalidJSON,
			Message: "invalid JSON value: " + err.Error(),
		}}
	}

	var errors []ValidationError

	switch field.Type {
	case ConfigTypeString:
		s, ok := value.(string)
		if !ok {
			errors = append(errors, ValidationError{
				Key:     key,
				Code:    ErrInvalidType,
				Message: "expected string",
			})
			break
		}
		if field.MinLength != nil && len(s) < *field.MinLength {
			errors = append(errors, ValidationError{
				Key:     key,
				Code:    ErrStringLength,
				Message: fmt.Sprintf("string shorter than minimum %d", *field.MinLength),
			})
		}
		if field.MaxLength != nil && len(s) > *field.MaxLength {
			errors = append(errors, ValidationError{
				Key:     key,
				Code:    ErrStringLength,
				Message: fmt.Sprintf("string longer than maximum %d", *field.MaxLength),
			})
		}
	case ConfigTypeInteger:
		num, ok := toInteger(value)
		if !ok {
			errors = append(errors, ValidationError{
				Key:     key,
				Code:    ErrInvalidType,
				Message: "expected integer",
			})
			break
		}
		if field.Minimum != nil && float64(num) < *field.Minimum {
			errors = append(errors, ValidationError{
				Key:     key,
				Code:    ErrNumberOutOfRange,
				Message: fmt.Sprintf("integer below minimum %v", *field.Minimum),
			})
		}
		if field.Maximum != nil && float64(num) > *field.Maximum {
			errors = append(errors, ValidationError{
				Key:     key,
				Code:    ErrNumberOutOfRange,
				Message: fmt.Sprintf("integer above maximum %v", *field.Maximum),
			})
		}
	case ConfigTypeNumber:
		num, ok := toNumber(value)
		if !ok {
			errors = append(errors, ValidationError{
				Key:     key,
				Code:    ErrInvalidType,
				Message: "expected number",
			})
			break
		}
		if field.Minimum != nil && num < *field.Minimum {
			errors = append(errors, ValidationError{
				Key:     key,
				Code:    ErrNumberOutOfRange,
				Message: fmt.Sprintf("number below minimum %v", *field.Minimum),
			})
		}
		if field.Maximum != nil && num > *field.Maximum {
			errors = append(errors, ValidationError{
				Key:     key,
				Code:    ErrNumberOutOfRange,
				Message: fmt.Sprintf("number above maximum %v", *field.Maximum),
			})
		}
	case ConfigTypeBoolean:
		if _, ok := value.(bool); !ok {
			errors = append(errors, ValidationError{
				Key:     key,
				Code:    ErrInvalidType,
				Message: "expected boolean",
			})
		}
	case ConfigTypeObject:
		if _, ok := value.(map[string]any); !ok {
			errors = append(errors, ValidationError{
				Key:     key,
				Code:    ErrInvalidType,
				Message: "expected object",
			})
		}
	case ConfigTypeArray:
		if _, ok := value.([]any); !ok {
			errors = append(errors, ValidationError{
				Key:     key,
				Code:    ErrInvalidType,
				Message: "expected array",
			})
		}
	default:
		errors = append(errors, ValidationError{
			Key:     key,
			Code:    ErrInvalidType,
			Message: "unknown field type: " + string(field.Type),
		})
		return errors
	}

	if len(errors) > 0 {
		return errors
	}

	if len(field.Enum) > 0 {
		inEnum := false
		for _, e := range field.Enum {
			if jsonEqual(raw, e) {
				inEnum = true
				break
			}
		}
		if !inEnum {
			return []ValidationError{{
				Key:     key,
				Code:    ErrValueNotInEnum,
				Message: "value not in enum",
			}}
		}
	}

	return nil
}

func (v *Validator) ValidateSecretRef(key string, ref *SecretRef, field ConfigField) []ValidationError {
	if !field.Secret {
		if ref != nil {
			return []ValidationError{{
				Key:     key,
				Code:    ErrSecretRefInvalid,
				Message: "secretRef provided for non-secret field",
			}}
		}
		return nil
	}

	if ref == nil {
		return nil
	}

	if ref.Provider == "" || ref.Key == "" {
		return []ValidationError{{
			Key:     key,
			Code:    ErrSecretRefInvalid,
			Message: "secretRef missing provider or key",
		}}
	}

	if len(v.knownProviders) > 0 && !v.knownProviders[ref.Provider] {
		return []ValidationError{{
			Key:     key,
			Code:    ErrProviderUnknown,
			Message: "unknown secret provider: " + ref.Provider,
		}}
	}

	return nil
}

func (v *Validator) ValidateSchema(schema *ConfigSchema) []ValidationError {
	seen := make(map[string]bool)
	var errors []ValidationError
	for _, f := range schema.Fields {
		if f.Key == "" {
			errors = append(errors, ValidationError{
				Key:     "",
				Code:    RequiredFieldMissing,
				Message: "empty field key",
			})
			continue
		}
		if seen[f.Key] {
			errors = append(errors, ValidationError{
				Key:     f.Key,
				Code:    RequiredFieldMissing,
				Message: "duplicate field key: " + f.Key,
			})
		}
		seen[f.Key] = true
		switch f.Type {
		case ConfigTypeString, ConfigTypeInteger, ConfigTypeNumber, ConfigTypeBoolean, ConfigTypeObject, ConfigTypeArray:
		default:
			errors = append(errors, ValidationError{
				Key:     f.Key,
				Code:    ErrInvalidType,
				Message: "invalid field type: " + string(f.Type),
			})
		}
	}
	return errors
}

const (
	RequiredFieldMissing = "REQUIRED_FIELD_MISSING"
)

func jsonEqual(a, b json.RawMessage) bool {
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	ab, _ := json.Marshal(av)
	bb, _ := json.Marshal(bv)
	return string(ab) == string(bb)
}

func toInteger(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		if n == math.Trunc(n) {
			return int64(n), true
		}
		return 0, false
	case int64:
		return n, true
	case int:
		return int64(n), true
	}
	return 0, false
}

func toNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	}
	return 0, false
}
