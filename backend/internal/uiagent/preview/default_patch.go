package preview

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/schema_ui"
	"github.com/u-ai/backend/internal/uiagent/schema"
)

type defaultPatchGenerator struct {
	validator schema.SchemaValidator
	catalog   *schema.ComponentCatalog
}

func NewDefaultPatchGenerator(v schema.SchemaValidator) PatchGenerator {
	return &defaultPatchGenerator{validator: v}
}

func NewDefaultPatchGeneratorWithCatalog(v schema.SchemaValidator, catalog *schema.ComponentCatalog) PatchGenerator {
	return &defaultPatchGenerator{validator: v, catalog: catalog}
}

var (
	errMissingPropRE  = regexp.MustCompile(`node "(.+?)" missing required property: "(.+?)"`)
	errEmptyTargetRE  = regexp.MustCompile(`action in node "(.+?)" has empty target`)
	errBadChildRE     = regexp.MustCompile(`node "(.+?)" does not allow child type "(.+?)"`)
	errBadActionRE    = regexp.MustCompile(`component "(.+?)" does not support action target "(.+?)"`)
	errDangerPropRE   = regexp.MustCompile(`property "(.+?)" in node "(.+?)" contains potentially dangerous content`)
	errDangerBindRE   = regexp.MustCompile(`binding source in node "(.+?)" contains potentially dangerous content`)
	errDangerActionRE = regexp.MustCompile(`action target in node "(.+?)" contains potentially dangerous content`)
)

func (g *defaultPatchGenerator) GeneratePatch(ctx context.Context, obs *ObservationResult) (*Patch, error) {
	if obs == nil || len(obs.Errors) == 0 {
		return nil, nil
	}
	if g.validator == nil {
		return nil, fmt.Errorf("patch generator: validator not configured")
	}

	patch := &Patch{
		SessionID: obs.SessionID,
	}

	for _, err := range obs.Errors {
		if m := errMissingPropRE.FindStringSubmatch(err); m != nil {
			patch.Fixes = append(patch.Fixes, Fix{
				Path:    "add_property",
				Type:    "missing_property",
				Content: m[1] + "|" + m[2],
			})
			patch.TargetPaths = append(patch.TargetPaths, m[1])
			continue
		}
		if m := errEmptyTargetRE.FindStringSubmatch(err); m != nil {
			patch.Fixes = append(patch.Fixes, Fix{
				Path:    "set_action_target",
				Type:    "empty_action_target",
				Content: m[1],
			})
			patch.TargetPaths = append(patch.TargetPaths, m[1])
			continue
		}
		if m := errBadChildRE.FindStringSubmatch(err); m != nil {
			patch.Fixes = append(patch.Fixes, Fix{
				Path:    "remove_child",
				Type:    "invalid_child_type",
				Content: m[1] + "|" + m[2],
			})
			patch.TargetPaths = append(patch.TargetPaths, m[1])
			continue
		}
		if m := errBadActionRE.FindStringSubmatch(err); m != nil {
			patch.Fixes = append(patch.Fixes, Fix{
				Path:    "fix_action_target",
				Type:    "unsupported_action",
				Content: m[1] + "|" + m[2],
			})
			patch.TargetPaths = append(patch.TargetPaths, m[1])
			continue
		}
		if m := errDangerPropRE.FindStringSubmatch(err); m != nil {
			patch.Fixes = append(patch.Fixes, Fix{
				Path:    "sanitize_property",
				Type:    "dangerous_property",
				Content: m[2] + "|" + m[1],
			})
			patch.TargetPaths = append(patch.TargetPaths, m[2])
			continue
		}
		if m := errDangerBindRE.FindStringSubmatch(err); m != nil {
			patch.Fixes = append(patch.Fixes, Fix{
				Path:    "sanitize_binding",
				Type:    "dangerous_binding",
				Content: m[1],
			})
			patch.TargetPaths = append(patch.TargetPaths, m[1])
			continue
		}
		if m := errDangerActionRE.FindStringSubmatch(err); m != nil {
			patch.Fixes = append(patch.Fixes, Fix{
				Path:    "sanitize_action",
				Type:    "dangerous_action",
				Content: m[1],
			})
			patch.TargetPaths = append(patch.TargetPaths, m[1])
			continue
		}
	}

	if len(patch.Fixes) == 0 {
		return nil, nil
	}
	return patch, nil
}

type defaultApplier struct {
	sessions SessionManager
	catalog  *schema.ComponentCatalog
}

func NewDefaultApplier(mgr SessionManager) Applier {
	return &defaultApplier{sessions: mgr}
}

func NewDefaultApplierWithCatalog(mgr SessionManager, catalog *schema.ComponentCatalog) Applier {
	return &defaultApplier{sessions: mgr, catalog: catalog}
}

func (a *defaultApplier) ApplyPatch(ctx context.Context, sessionID string, patch *Patch) (string, error) {
	if patch == nil {
		return "", fmt.Errorf("applier: nil patch")
	}
	if a.sessions == nil {
		return "", fmt.Errorf("applier: session manager not configured")
	}

	session, err := a.sessions.Get(sessionID)
	if err != nil {
		return "", fmt.Errorf("applier: get session: %w", err)
	}
	if session.Schema == nil {
		return "", fmt.Errorf("applier: session schema is nil")
	}

	applied := 0
	for _, fix := range patch.Fixes {
		if err := a.applySchemaFix(session.Schema, fix); err != nil {
			continue
		}
		applied++
	}

	if applied == 0 {
		return "", fmt.Errorf("applier: no fixes applied")
	}

	return fmt.Sprintf("tx_%s_%d", sessionID, applied), nil
}

func (a *defaultApplier) applySchemaFix(doc *schema.SchemaUIDocument, fix Fix) error {
	if doc == nil {
		return fmt.Errorf("nil document")
	}
	switch fix.Type {
	case "missing_property":
		parts := strings.SplitN(fix.Content, "|", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid missing_property content")
		}
		return a.fixMissingProperty(doc, parts[0], parts[1])
	case "empty_action_target":
		return a.fixEmptyActionTarget(doc, fix.Content)
	case "invalid_child_type":
		parts := strings.SplitN(fix.Content, "|", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid invalid_child_type content")
		}
		return a.fixInvalidChild(doc, parts[0], parts[1])
	case "unsupported_action":
		parts := strings.SplitN(fix.Content, "|", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid unsupported_action content")
		}
		return a.fixUnsupportedAction(doc, parts[0], parts[1])
	case "dangerous_property":
		parts := strings.SplitN(fix.Content, "|", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid dangerous_property content")
		}
		return a.fixDangerousProperty(doc, parts[0], parts[1])
	case "dangerous_binding":
		return a.fixDangerousBinding(doc, fix.Content)
	case "dangerous_action":
		return a.fixDangerousAction(doc, fix.Content)
	default:
		return fmt.Errorf("unknown fix type: %s", fix.Type)
	}
}

func (a *defaultApplier) fixMissingProperty(doc *schema.SchemaUIDocument, nodeType, propName string) error {
	nodes := a.findNodesByType(doc, nodeType)
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes of type %s found", nodeType)
	}
	for _, node := range nodes {
		var props map[string]any
		if len(node.Props) > 0 {
			_ = json.Unmarshal(node.Props, &props)
		}
		if props == nil {
			props = make(map[string]any)
		}
		if _, exists := props[propName]; !exists {
			props[propName] = a.defaultValueForProp(propName)
			newProps, err := json.Marshal(props)
			if err != nil {
				return err
			}
			node.Props = newProps
		}
	}
	return nil
}

func (a *defaultApplier) fixEmptyActionTarget(doc *schema.SchemaUIDocument, nodeType string) error {
	nodes := a.findNodesByType(doc, nodeType)
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes of type %s found", nodeType)
	}
	defaultTarget := "submit"
	if a.catalog != nil {
		cs, ok := a.catalog.Get(schema.SchemaComponentType(nodeType))
		if ok && len(cs.Actions) > 0 {
			defaultTarget = cs.Actions[0]
		}
	}
	for _, node := range nodes {
		for i := range node.Actions {
			if node.Actions[i].Target == "" {
				node.Actions[i].Target = defaultTarget
			}
		}
	}
	return nil
}

func (a *defaultApplier) fixInvalidChild(doc *schema.SchemaUIDocument, parentType, childType string) error {
	nodes := a.findNodesByType(doc, parentType)
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes of type %s found", parentType)
	}
	for _, node := range nodes {
		var validChildren []schema_ui.SchemaUINode
		for _, child := range node.Children {
			if string(child.Type) != childType {
				validChildren = append(validChildren, child)
			}
		}
		node.Children = validChildren
	}
	return nil
}

func (a *defaultApplier) fixUnsupportedAction(doc *schema.SchemaUIDocument, nodeType, badTarget string) error {
	nodes := a.findNodesByType(doc, nodeType)
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes of type %s found", nodeType)
	}
	replacement := "submit"
	if a.catalog != nil {
		cs, ok := a.catalog.Get(schema.SchemaComponentType(nodeType))
		if ok && len(cs.Actions) > 0 {
			replacement = cs.Actions[0]
		}
	}
	for _, node := range nodes {
		for i := range node.Actions {
			if node.Actions[i].Target == badTarget {
				node.Actions[i].Target = replacement
			}
		}
	}
	return nil
}

func (a *defaultApplier) fixDangerousProperty(doc *schema.SchemaUIDocument, nodeType, propName string) error {
	nodes := a.findNodesByType(doc, nodeType)
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes of type %s found", nodeType)
	}
	for _, node := range nodes {
		var props map[string]any
		if len(node.Props) > 0 {
			_ = json.Unmarshal(node.Props, &props)
		}
		if props != nil {
			if val, exists := props[propName]; exists {
				if strVal, ok := val.(string); ok && containsDangerous(strVal) {
					props[propName] = "[removed]"
					newProps, err := json.Marshal(props)
					if err != nil {
						return err
					}
					node.Props = newProps
				}
			}
		}
	}
	return nil
}

func (a *defaultApplier) fixDangerousBinding(doc *schema.SchemaUIDocument, nodeType string) error {
	nodes := a.findNodesByType(doc, nodeType)
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes of type %s found", nodeType)
	}
	for _, node := range nodes {
		for i := range node.Bindings {
			if containsDangerous(string(node.Bindings[i].Source)) {
				node.Bindings[i].Source = schema_ui.SourceStatic
			}
		}
	}
	return nil
}

func (a *defaultApplier) fixDangerousAction(doc *schema.SchemaUIDocument, nodeType string) error {
	nodes := a.findNodesByType(doc, nodeType)
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes of type %s found", nodeType)
	}
	for _, node := range nodes {
		for i := range node.Actions {
			if containsDangerous(node.Actions[i].Target) {
				node.Actions[i].Target = "submit"
			}
		}
	}
	return nil
}

func (a *defaultApplier) findNodesByType(doc *schema.SchemaUIDocument, nodeType string) []*schema_ui.SchemaUINode {
	var results []*schema_ui.SchemaUINode
	for i := range doc.Children {
		results = append(results, findByNodeType(&doc.Children[i], nodeType)...)
	}
	return results
}

func findByNodeType(node *schema_ui.SchemaUINode, nodeType string) []*schema_ui.SchemaUINode {
	var results []*schema_ui.SchemaUINode
	if node == nil {
		return nil
	}
	if string(node.Type) == nodeType {
		results = append(results, node)
	}
	for i := range node.Children {
		results = append(results, findByNodeType(&node.Children[i], nodeType)...)
	}
	return results
}

func (a *defaultApplier) defaultValueForProp(propName string) string {
	switch propName {
	case "label", "title", "placeholder":
		return "Text"
	case "text", "content", "description":
		return ""
	case "name", "id":
		return "field_" + propName
	default:
		return ""
	}
}

func containsDangerous(s string) bool {
	dangerous := []string{"<script", "javascript:", "onerror=", "onload=", "eval(", "Function(", "exec(", "shell(", "system("}
	lower := strings.ToLower(s)
	for _, pattern := range dangerous {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
