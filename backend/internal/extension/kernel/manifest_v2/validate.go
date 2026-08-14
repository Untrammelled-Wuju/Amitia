package manifest_v2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

func schemaValueEqual(a, b any) bool {
	if af, ok := a.(float64); ok {
		if bf, ok2 := b.(float64); ok2 {
			return af == bf
		}
	}
	return a == b
}

func checkSchemaType(value any, t string) bool {
	switch t {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "integer":
		f, ok := value.(float64)
		return ok && f == float64(int64(f))
	case "number":
		_, ok := value.(float64)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "null":
		return value == nil
	}
	return true
}

func validateSchemaValue(value any, schema map[string]any, path string, report *ValidationReport) {
	if value == nil {
		return
	}
	if t, ok := schema["type"].(string); ok {
		if !checkSchemaType(value, t) {
			report.AddError(path, "schema_type", fmt.Sprintf("expected type %s, got %T", t, value))
			return
		}
	}
	if c, ok := schema["const"]; ok {
		if !schemaValueEqual(value, c) {
			report.AddError(path, "schema_const", fmt.Sprintf("expected const %v, got %v", c, value))
		}
	}
	if enum, ok := schema["enum"].([]any); ok {
		found := false
		for _, e := range enum {
			if schemaValueEqual(value, e) {
				found = true
				break
			}
		}
		if !found {
			report.AddError(path, "schema_enum", fmt.Sprintf("value %v not in enum %v", value, enum))
		}
	}
	if pattern, ok := schema["pattern"].(string); ok {
		if s, ok2 := value.(string); ok2 {
			if re, err := regexp.Compile(pattern); err == nil {
				if !re.MatchString(s) {
					report.AddError(path, "schema_pattern", fmt.Sprintf("value %q does not match pattern %s", s, pattern))
				}
			}
		}
	}
	if minLen, ok := schema["minLength"].(float64); ok {
		if s, ok2 := value.(string); ok2 {
			if len(s) < int(minLen) {
				report.AddError(path, "schema_minLength", fmt.Sprintf("string length %d less than minLength %d", len(s), int(minLen)))
			}
		}
	}
	if maxLen, ok := schema["maxLength"].(float64); ok {
		if s, ok2 := value.(string); ok2 {
			if len(s) > int(maxLen) {
				report.AddError(path, "schema_maxLength", fmt.Sprintf("string length %d greater than maxLength %d", len(s), int(maxLen)))
			}
		}
	}
	if minimum, ok := schema["minimum"].(float64); ok {
		if n, ok2 := value.(float64); ok2 {
			if n < minimum {
				report.AddError(path, "schema_minimum", fmt.Sprintf("value %v less than minimum %v", n, minimum))
			}
		}
	}
	if maximum, ok := schema["maximum"].(float64); ok {
		if n, ok2 := value.(float64); ok2 {
			if n > maximum {
				report.AddError(path, "schema_maximum", fmt.Sprintf("value %v greater than maximum %v", n, maximum))
			}
		}
	}
	switch t := schema["type"].(string); t {
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			return
		}
		if required, ok := schema["required"].([]any); ok {
			for _, r := range required {
				if rs, ok2 := r.(string); ok2 {
					if _, exists := obj[rs]; !exists {
						report.AddError(path+"."+rs, "schema_required", fmt.Sprintf("required field %s missing", rs))
					}
				}
			}
		}
		if props, ok := schema["properties"].(map[string]any); ok {
			for key, val := range obj {
				if ps, ok2 := props[key].(map[string]any); ok2 {
					validateSchemaValue(val, ps, path+"."+key, report)
				}
			}
		}
		if ap, ok := schema["additionalProperties"]; ok {
			if ap == false {
				if props, ok2 := schema["properties"].(map[string]any); ok2 {
					for key := range obj {
						if _, known := props[key]; !known {
							report.AddError(path+"."+key, "schema_additional_property",
								fmt.Sprintf("additional property %q not allowed", key))
						}
					}
				}
			} else if apSchema, ok2 := ap.(map[string]any); ok2 {
				if props, ok3 := schema["properties"].(map[string]any); ok3 {
					for key, val := range obj {
						if _, known := props[key]; !known {
							validateSchemaValue(val, apSchema, path+"."+key, report)
						}
					}
				}
			}
		}
		if minProps, ok := schema["minProperties"].(float64); ok {
			if len(obj) < int(minProps) {
				report.AddError(path, "schema_minProperties",
					fmt.Sprintf("object has %d properties, minimum is %d", len(obj), int(minProps)))
			}
		}
		if maxProps, ok := schema["maxProperties"].(float64); ok {
			if len(obj) > int(maxProps) {
				report.AddError(path, "schema_maxProperties",
					fmt.Sprintf("object has %d properties, maximum is %d", len(obj), int(maxProps)))
			}
		}
	case "array":
		arr, ok := value.([]any)
		if !ok {
			return
		}
		if minItems, ok := schema["minItems"].(float64); ok {
			if len(arr) < int(minItems) {
				report.AddError(path, "schema_minItems", fmt.Sprintf("array length %d less than minItems %d", len(arr), int(minItems)))
			}
		}
		if schemaFalse, ok := schema["uniqueItems"].(bool); ok && schemaFalse {
			seen := make(map[string]bool)
			for i, item := range arr {
				var key string
				switch v := item.(type) {
				case string:
					key = "s:" + v
				case float64:
					key = "n:" + fmt.Sprintf("%v", v)
				case bool:
					key = "b:" + fmt.Sprintf("%v", v)
				default:
					if data, err := json.Marshal(item); err == nil {
						key = "j:" + string(data)
					} else {
						key = fmt.Sprintf("i:%d", i)
					}
				}
				if seen[key] {
					report.AddError(fmt.Sprintf("%s[%d]", path, i), "schema_uniqueItems",
						fmt.Sprintf("duplicate array item at index %d", i))
				}
				seen[key] = true
			}
		}
		if items, ok := schema["items"].(map[string]any); ok {
			for i, item := range arr {
				validateSchemaValue(item, items, fmt.Sprintf("%s[%d]", path, i), report)
			}
		}
	}
}

func ValidateRawWithSchema(data []byte) *ValidationReport {
	report := &ValidationReport{}
	var raw any
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := rejectDuplicateJSONKeys(decoder); err != nil {
		report.AddError("", "duplicate_key", fmt.Sprintf("duplicate JSON key: %v", err))
		return report
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		report.AddError("", "schema_parse", fmt.Sprintf("failed to parse manifest: %v", err))
		return report
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(manifestV2Schema), &schema); err != nil {
		report.AddError("", "schema_parse", fmt.Sprintf("failed to parse schema: %v", err))
		return report
	}
	validateSchemaValue(raw, schema, "", report)
	return report
}

func ParseValidated(data []byte) (Manifest, ValidationReport, error) {
	schemaReport := ValidateRawWithSchema(data)
	combined := ValidationReport{}
	combined.MergePtr(schemaReport)

	m, err := Parse(data)
	if err != nil {
		return Manifest{}, combined, err
	}

	normalized, normReport := m.NormalizeCompatibility()
	combined.Merge(normReport)

	valReport := normalized.Validate()
	combined.Merge(valReport)

	return normalized, combined, nil
}

func (m Manifest) ValidateWithSchema() *ValidationReport {
	data, err := json.Marshal(m)
	if err != nil {
		report := &ValidationReport{}
		report.AddError("", "schema_marshal", fmt.Sprintf("failed to marshal: %v", err))
		return report
	}
	return ValidateRawWithSchema(data)
}

func ValidateFile(filePath string) (*Manifest, *ValidationReport) {
	report := &ValidationReport{}
	data, err := os.ReadFile(filePath)
	if err != nil {
		report.AddErrorWithLocation("", "file_read", fmt.Sprintf("failed to read file: %v", err), filePath, 0, 0)
		return nil, report
	}
	manifest, parsedReport, parseErr := ParseValidated(data)
	if parseErr != nil {
		report.Merge(parsedReport)
		report.AttachFile(filePath)
		return nil, report
	}
	report.Merge(parsedReport)
	report.AttachFile(filePath)
	return &manifest, report
}

func offsetToLineColumn(data []byte, offset int64) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > int64(len(data)) {
		offset = int64(len(data))
	}
	line := 1
	col := 1
	for i := int64(0); i < offset; i++ {
		if data[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}
