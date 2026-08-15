package schema

import (
	"fmt"
	"regexp"
	"strings"
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

// allowedActions is the whitelist of permitted action types.
var allowedActions = map[string]bool{
	"navigate":           true,
	"invoke_tool":        true,
	"invoke_capability":  true,
	"open_resource":      true,
	"set_state":          true,
	"submit_form":        true,
}

// interactiveComponentTypes identifies component types that require accessibility labels.
var interactiveComponentTypes = map[string]bool{
	string(CompButton):  true,
	string(CompField):   true,
	string(CompSelect):  true,
	string(CompSwitch):  true,
	string(CompSlider):  true,
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

	// Validate the root node recursively.
	validateNode(&result, v.catalog, doc.Root, true, "")

	return result
}

// validateNode recursively validates a schema node.
func validateNode(result *ValidationResult, catalog *ComponentCatalog, node SchemaNode, isRoot bool, parentType string) {
	nodeType := node.Type

	// Rule 1: Node type must exist in catalog.
	schema, ok := catalog.Get(SchemaComponentType(nodeType))
	if !ok {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("unknown component type: %q", nodeType))
		return
	}

	// Rule 2: Required properties must exist.
	for _, reqProp := range schema.RequiredProps {
		if node.Properties == nil {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("node %q missing required property: %q", nodeType, reqProp))
			continue
		}
		if _, exists := node.Properties[reqProp]; !exists {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("node %q missing required property: %q", nodeType, reqProp))
		}
	}

	// Rule 3: If this is a root node, it must be a page type.
	if isRoot && nodeType != string(CompPage) {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("root node must be %q, got %q", CompPage, nodeType))
	}

	// Rule 3 (children): Children types must be in parent's AllowedChildren.
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

	// Rule 4: Actions must be in the whitelist and component must support them.
	if len(node.Actions) > 0 {
		// Build set of actions this component supports.
		supportedActions := make(map[string]bool)
		for _, a := range schema.Actions {
			supportedActions[a] = true
		}

		for _, action := range node.Actions {
			if !allowedActions[action.Type] {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("action type %q is not allowed (node: %q)", action.Type, nodeType))
			}
			if len(supportedActions) > 0 && !supportedActions[action.Type] {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("component %q does not support action %q", nodeType, action.Type))
			}
		}
	}

	// Rule 5: Scan all string properties for dangerous patterns.
	if node.Properties != nil {
		for key, val := range node.Properties {
			if strVal, ok := val.(string); ok {
				if containsDangerousContent(strVal) {
					result.Valid = false
					result.Errors = append(result.Errors, fmt.Sprintf("property %q in node %q contains potentially dangerous content", key, nodeType))
				}
			}
		}
	}

	// Also scan bindings and actions for dangerous content.
	for _, binding := range node.Bindings {
		if containsDangerousContent(binding.Source) {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("binding source in node %q contains potentially dangerous content", nodeType))
		}
	}
	for _, action := range node.Actions {
		if action.Target != "" && containsDangerousContent(action.Target) {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("action target in node %q contains potentially dangerous content", nodeType))
		}
		if action.Payload != nil {
			for k, v := range action.Payload {
				if strVal, ok := v.(string); ok {
					if containsDangerousContent(strVal) {
						result.Valid = false
						result.Errors = append(result.Errors, fmt.Sprintf("action payload key %q in node %q contains potentially dangerous content", k, nodeType))
					}
				}
			}
		}
	}

	// Rule 6: Interactive elements need a label for accessibility.
	if interactiveComponentTypes[nodeType] {
		hasLabel := false
		if node.Properties != nil {
			if label, exists := node.Properties["label"]; exists {
				if strLabel, ok := label.(string); ok && strings.TrimSpace(strLabel) != "" {
					hasLabel = true
				}
			}
		}
		if !hasLabel {
			result.Warnings = append(result.Warnings, fmt.Sprintf("interactive node %q should have a non-empty label property for accessibility", nodeType))
		}
	}

	// Recurse into children.
	for i := range node.Children {
		validateNode(result, catalog, node.Children[i], false, nodeType)
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
