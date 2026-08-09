package config

import (
	"encoding/json"
	"testing"
)

type fakeSecretRegistry struct {
	providers []string
}

func (r fakeSecretRegistry) KnownProviders() []string {
	return r.providers
}

func rawString(s string) json.RawMessage {
	return json.RawMessage(`"` + s + `"`)
}

func TestValidator_StringType(t *testing.T) {
	v := NewValidator(fakeSecretRegistry{providers: []string{"vault"}})

	cases := []struct {
		name    string
		raw     json.RawMessage
		field   ConfigField
		wantErr bool
	}{
		{"valid string", rawString("hello"), ConfigField{Type: ConfigTypeString}, false},
		{"number for string", json.RawMessage("123"), ConfigField{Type: ConfigTypeString}, true},
		{"bool for string", json.RawMessage("true"), ConfigField{Type: ConfigTypeString}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := v.ValidateValue("k", tc.raw, tc.field, ConfigScopePlugin)
			if tc.wantErr && len(errs) == 0 {
				t.Error("expected error, got none")
			}
			if !tc.wantErr && len(errs) > 0 {
				t.Errorf("unexpected errors: %v", errs)
			}
		})
	}
}

func TestValidator_IntegerType(t *testing.T) {
	v := NewValidator(fakeSecretRegistry{providers: []string{"vault"}})

	cases := []struct {
		name    string
		raw     json.RawMessage
		field   ConfigField
		wantErr bool
	}{
		{"valid integer", json.RawMessage("42"), ConfigField{Type: ConfigTypeInteger}, false},
		{"negative integer", json.RawMessage("-10"), ConfigField{Type: ConfigTypeInteger}, false},
		{"float for integer", json.RawMessage("3.14"), ConfigField{Type: ConfigTypeInteger}, true},
		{"string for integer", rawString("abc"), ConfigField{Type: ConfigTypeInteger}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := v.ValidateValue("num", tc.raw, tc.field, ConfigScopePlugin)
			if tc.wantErr && len(errs) == 0 {
				t.Error("expected error, got none")
			}
			if !tc.wantErr && len(errs) > 0 {
				t.Errorf("unexpected errors: %v", errs)
			}
		})
	}
}

func TestValidator_NumberRange(t *testing.T) {
	v := NewValidator(fakeSecretRegistry{providers: []string{"vault"}})

	min := 0.0
	max := 100.0
	field := ConfigField{
		Type:    ConfigTypeNumber,
		Minimum: &min,
		Maximum: &max,
	}

	cases := []struct {
		name    string
		raw     json.RawMessage
		wantErr bool
	}{
		{"in range", json.RawMessage("50"), false},
		{"at min", json.RawMessage("0"), false},
		{"at max", json.RawMessage("100"), false},
		{"below min", json.RawMessage("-1"), true},
		{"above max", json.RawMessage("101"), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := v.ValidateValue("range", tc.raw, field, ConfigScopePlugin)
			if tc.wantErr && len(errs) == 0 {
				t.Error("expected error, got none")
			}
			if !tc.wantErr && len(errs) > 0 {
				t.Errorf("unexpected errors: %v", errs)
			}
		})
	}
}

func TestValidator_IntegerRange(t *testing.T) {
	v := NewValidator(fakeSecretRegistry{providers: []string{"vault"}})

	min := 1.0
	max := 10.0
	field := ConfigField{
		Type:    ConfigTypeInteger,
		Minimum: &min,
		Maximum: &max,
	}

	errs := v.ValidateValue("port", json.RawMessage("5"), field, ConfigScopePlugin)
	if len(errs) > 0 {
		t.Errorf("unexpected error for valid integer: %v", errs)
	}

	errs = v.ValidateValue("port", json.RawMessage("0"), field, ConfigScopePlugin)
	if len(errs) == 0 {
		t.Error("expected error for integer below minimum")
	}

	errs = v.ValidateValue("port", json.RawMessage("11"), field, ConfigScopePlugin)
	if len(errs) == 0 {
		t.Error("expected error for integer above maximum")
	}
}

func TestValidator_StringLength(t *testing.T) {
	v := NewValidator(fakeSecretRegistry{providers: []string{"vault"}})

	minLen := 3
	maxLen := 6
	field := ConfigField{
		Type:      ConfigTypeString,
		MinLength: &minLen,
		MaxLength: &maxLen,
	}

	cases := []struct {
		name    string
		raw     json.RawMessage
		wantErr bool
	}{
		{"in range", rawString("abc"), false},
		{"exact max", rawString("abcdef"), false},
		{"too short", rawString("ab"), true},
		{"too long", rawString("abcdefg"), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := v.ValidateValue("s", tc.raw, field, ConfigScopePlugin)
			if tc.wantErr && len(errs) == 0 {
				t.Error("expected error, got none")
			}
			if !tc.wantErr && len(errs) > 0 {
				t.Errorf("unexpected errors: %v", errs)
			}
		})
	}
}

func TestValidator_EnumConstraint(t *testing.T) {
	v := NewValidator(fakeSecretRegistry{providers: []string{"vault"}})

	field := ConfigField{
		Type:  ConfigTypeString,
		Enum:  []json.RawMessage{rawString("a"), rawString("b"), rawString("c")},
	}

	errs := v.ValidateValue("mode", rawString("b"), field, ConfigScopePlugin)
	if len(errs) > 0 {
		t.Errorf("unexpected error for valid enum value: %v", errs)
	}

	errs = v.ValidateValue("mode", rawString("d"), field, ConfigScopePlugin)
	if len(errs) == 0 {
		t.Error("expected error for value not in enum")
	}
}

func TestValidator_RequiredField(t *testing.T) {
	v := NewValidator(fakeSecretRegistry{providers: []string{"vault"}})

	field := ConfigField{Type: ConfigTypeString, Required: true}

	cases := []struct {
		name    string
		raw     json.RawMessage
		wantErr bool
	}{
		{"missing value", nil, true},
		{"empty bytes", json.RawMessage{}, true},
		{"null value", json.RawMessage("null"), true},
		{"valid value", rawString("hello"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := v.ValidateValue("req", tc.raw, field, ConfigScopePlugin)
			if tc.wantErr && len(errs) == 0 {
				t.Error("expected error, got none")
			}
			if !tc.wantErr && len(errs) > 0 {
				t.Errorf("unexpected errors: %v", errs)
			}
		})
	}
}

func TestValidator_BooleanType(t *testing.T) {
	v := NewValidator(fakeSecretRegistry{providers: []string{"vault"}})

	field := ConfigField{Type: ConfigTypeBoolean}

	if errs := v.ValidateValue("flag", json.RawMessage("true"), field, ConfigScopePlugin); len(errs) > 0 {
		t.Errorf("unexpected error for valid boolean: %v", errs)
	}

	if errs := v.ValidateValue("flag", rawString("not-bool"), field, ConfigScopePlugin); len(errs) == 0 {
		t.Error("expected error for non-boolean value")
	}
}

func TestValidator_SecretRef_NonSecretField(t *testing.T) {
	v := NewValidator(fakeSecretRegistry{providers: []string{"vault"}})

	field := ConfigField{Type: ConfigTypeString, Secret: false}
	ref := &SecretRef{Provider: "vault", Key: "my-secret"}

	errs := v.ValidateSecretRef("apikey", ref, field)
	if len(errs) == 0 {
		t.Error("expected error when secretRef is provided for non-secret field")
	}
}

func TestValidator_SecretRef_Valid(t *testing.T) {
	v := NewValidator(fakeSecretRegistry{providers: []string{"vault"}})

	field := ConfigField{Type: ConfigTypeString, Secret: true}
	ref := &SecretRef{Provider: "vault", Key: "my-secret"}

	errs := v.ValidateSecretRef("apikey", ref, field)
	if len(errs) > 0 {
		t.Errorf("unexpected error for valid secret ref: %v", errs)
	}
}

func TestValidator_SecretRef_UnknownProvider(t *testing.T) {
	v := NewValidator(fakeSecretRegistry{providers: []string{"vault"}})

	field := ConfigField{Type: ConfigTypeString, Secret: true}
	ref := &SecretRef{Provider: "unknown", Key: "my-secret"}

	errs := v.ValidateSecretRef("apikey", ref, field)
	if len(errs) == 0 {
		t.Error("expected error for unknown secret provider")
	}
}

func TestValidator_SecretRef_MissingProviderOrKey(t *testing.T) {
	v := NewValidator(fakeSecretRegistry{providers: []string{"vault"}})

	field := ConfigField{Type: ConfigTypeString, Secret: true}

	cases := []struct {
		name string
		ref  *SecretRef
	}{
		{"empty provider", &SecretRef{Provider: "", Key: "x"}},
		{"empty key", &SecretRef{Provider: "vault", Key: ""}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := v.ValidateSecretRef("apikey", tc.ref, field)
			if len(errs) == 0 {
				t.Error("expected error for incomplete secret ref")
			}
		})
	}
}

func TestValidator_NoProviderRegistry(t *testing.T) {
	v := NewValidator(nil)

	field := ConfigField{Type: ConfigTypeString, Secret: true}
	ref := &SecretRef{Provider: "any", Key: "one"}

	errs := v.ValidateSecretRef("apikey", ref, field)
	if len(errs) > 0 {
		t.Errorf("expected no error without registry, got: %v", errs)
	}
}

func TestValidator_ObjectType(t *testing.T) {
	v := NewValidator(nil)

	field := ConfigField{Type: ConfigTypeObject}

	valid := json.RawMessage(`{"a":1,"b":2}`)
	invalid := rawString("not-object")

	if errs := v.ValidateValue("map", valid, field, ConfigScopePlugin); len(errs) > 0 {
		t.Errorf("unexpected error for valid object: %v", errs)
	}

	if errs := v.ValidateValue("map", invalid, field, ConfigScopePlugin); len(errs) == 0 {
		t.Error("expected error for non-object value")
	}
}

func TestValidator_ArrayType(t *testing.T) {
	v := NewValidator(nil)

	field := ConfigField{Type: ConfigTypeArray}

	valid := json.RawMessage(`[1,2,3]`)
	invalid := json.RawMessage(`{"x":1}`)

	if errs := v.ValidateValue("arr", valid, field, ConfigScopePlugin); len(errs) > 0 {
		t.Errorf("unexpected error for valid array: %v", errs)
	}

	if errs := v.ValidateValue("arr", invalid, field, ConfigScopePlugin); len(errs) == 0 {
		t.Error("expected error for non-array value")
	}
}

func TestValidateSchema_DuplicateKeys(t *testing.T) {
	v := NewValidator(nil)

	schema := NewSchema([]ConfigField{
		{Key: "dup", Type: ConfigTypeString},
		{Key: "dup", Type: ConfigTypeInteger},
	})

	errs := v.ValidateSchema(schema)
	if len(errs) == 0 {
		t.Error("expected validation errors for duplicate field keys")
	}
}

func TestValidateSchema_InvalidFieldType(t *testing.T) {
	v := NewValidator(nil)

	schema := NewSchema([]ConfigField{
		{Key: "f", Type: ConfigFieldType("unknown")},
	})

	errs := v.ValidateSchema(schema)
	if len(errs) == 0 {
		t.Error("expected validation error for invalid field type")
	}
}

func TestValidateSchema_EmptyKey(t *testing.T) {
	v := NewValidator(nil)

	schema := NewSchema([]ConfigField{
		{Key: "", Type: ConfigTypeString},
	})

	errs := v.ValidateSchema(schema)
	if len(errs) == 0 {
		t.Error("expected validation error for empty field key")
	}
}
