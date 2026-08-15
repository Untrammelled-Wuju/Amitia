package schema

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/schema_ui"
)

// ValidationResult contains the outcome of a schema validation.
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// SchemaValidator validates a SchemaUIDocument.
type SchemaValidator interface {
	Validate(doc *SchemaUIDocument) ValidationResult
}

// defaultSchemaValidator implements SchemaValidator using a ComponentCatalog.
type defaultSchemaValidator struct {
	catalog *ComponentCatalog
}

// NewSchemaValidator creates a validator backed by the given catalog.
func NewSchemaValidator(catalog *ComponentCatalog) SchemaValidator {
	return &defaultSchemaValidator{catalog: catalog}
}

// interactiveComponentTypes identifies component types that require accessibility labels.
var interactiveComponentTypes = map[string]bool{
	string(CompButton): true,
	string(CompField):  true,
	string(CompSelect): true,
	string(CompSwitch): true,
	string(CompSlider): true,
}

// dangerousPatterns matches strings that may indicate injection of executable code.
var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)<\s*script`),
	regexp.MustCompile(`(?i)javascript\s*:`),
	regexp.MustCompile(`(?i)on\w+\s*=`),
	regexp.MustCompile(`(?i)\beval\s*\(`),
	regexp.MustCompile(`(?i)\bFunction\s*\(`),
	regexp.MustCompile(`(?i)\bexec\s*\(`),
	regexp.MustCompile(`(?i)\bshell\s*\(`),
	regexp.MustCompile(`(?i)\bproc_open`),
	regexp.MustCompile(`(?i)\bsystem\s*\(`),
	regexp.MustCompile(`(?i)\bbacktick\s*`),
	regexp.MustCompile(`(?i)\$\{.*\}`),
	regexp.MustCompile(`(?i)<%.*%>`),
	regexp.MustCompile(`(?i)\{\{.*\}\}`),
}

// Validate checks the document against the catalog and security rules.
func (v *defaultSchemaValidator) Validate(doc *SchemaUIDocument) ValidationResult {
	result := ValidationResult{Valid: true}

	if doc == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "document is nil")
		return result
	}

	for i := range doc.Children {
		validateSchemaNode(&result, v.catalog, &doc.Children[i], "")
	}

	return result
}

func validateSchemaNode(result *ValidationResult, catalog *ComponentCatalog, node *schema_ui.SchemaUINode, parentType string) {
	if node == nil {
		return
	}
	nodeType := string(node.Type)

	schema, ok := catalog.Get(SchemaComponentType(nodeType))
	if !ok {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("unknown component type: %q", nodeType))
		return
	}

	var props map[string]any
	if len(node.Props) > 0 {
		_ = json.Unmarshal(node.Props, &props)
	}

	for _, reqProp := range schema.RequiredProps {
		if props == nil {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("node %q missing required property: %q", nodeType, reqProp))
			continue
		}
		if _, exists := props[reqProp]; !exists {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("node %q missing required property: %q", nodeType, reqProp))
		}
	}

	if len(node.Children) > 0 && len(schema.AllowedChildren) > 0 {
		allowedChildSet := make(map[SchemaComponentType]bool)
		for _, ac := range schema.AllowedChildren {
			allowedChildSet[ac] = true
		}
		for _, child := range node.Children {
			if !allowedChildSet[SchemaComponentType(child.Type)] {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("node %q does not allow child type %q", nodeType, child.Type))
			}
		}
	}

	if len(node.Actions) > 0 {
		supportedActions := make(map[string]bool)
		for _, a := range schema.Actions {
			supportedActions[a] = true
		}

		for _, action := range node.Actions {
			if action.Target == "" {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("action in node %q has empty target", nodeType))
			}
			if len(supportedActions) > 0 && !supportedActions[action.Target] {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("component %q does not support action target %q", nodeType, action.Target))
			}
		}
	}

	if props != nil {
		for key, val := range props {
			if strVal, ok := val.(string); ok {
				if containsDangerousContent(strVal) {
					result.Valid = false
					result.Errors = append(result.Errors, fmt.Sprintf("property %q in node %q contains potentially dangerous content", key, nodeType))
				}
			}
		}
	}

	for _, binding := range node.Bindings {
		if containsDangerousContent(string(binding.Source)) {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("binding source in node %q contains potentially dangerous content", nodeType))
		}
	}
	for _, action := range node.Actions {
		if action.Target != "" && containsDangerousContent(action.Target) {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("action target in node %q contains potentially dangerous content", nodeType))
		}
	}

	if interactiveComponentTypes[nodeType] {
		hasLabel := false
		if props != nil {
			if label, exists := props["label"]; exists {
				if strLabel, ok := label.(string); ok && strings.TrimSpace(strLabel) != "" {
					hasLabel = true
				}
			}
		}
		if !hasLabel {
			result.Warnings = append(result.Warnings, fmt.Sprintf("interactive node %q should have a non-empty label property for accessibility", nodeType))
		}
	}

	for i := range node.Children {
		validateSchemaNode(result, catalog, &node.Children[i], nodeType)
	}
}

// containsDangerousContent checks if a string contains patterns that could indicate code injection.
func containsDangerousContent(s string) bool {
	for _, re := range dangerousPatterns {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}
