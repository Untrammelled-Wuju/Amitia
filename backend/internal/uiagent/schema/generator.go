package schema

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/schema_ui"
)

const (
	DraftStateDraft     SchemaDraftState = "draft"
	DraftStatePreview   SchemaDraftState = "preview"
	DraftStatePublished SchemaDraftState = "published"
)

type SchemaDraftState string

// SchemaUIDocument is the canonical schema UI document from schema_ui package.
type SchemaUIDocument = schema_ui.SchemaUIDocument

// SchemaUINode is the canonical schema UI node from schema_ui package.
type SchemaUINode = schema_ui.SchemaUINode

// SchemaUIGenerator generates SchemaUIDocuments from natural language descriptions.
type SchemaUIGenerator struct {
	catalog *ComponentCatalog
}

// NewSchemaUIGenerator creates a new generator backed by the given catalog.
func NewSchemaUIGenerator(catalog *ComponentCatalog) *SchemaUIGenerator {
	return &SchemaUIGenerator{catalog: catalog}
}

// Generate produces a valid SchemaUIDocument from a description.
func (g *SchemaUIGenerator) Generate(
	description string,
	availableComps []SchemaComponentType,
) (*SchemaUIDocument, error) {
	desc := strings.ToLower(strings.TrimSpace(description))
	if desc == "" {
		return nil, fmt.Errorf("description must not be empty")
	}

	allowed := make(map[SchemaComponentType]bool)
	for _, c := range availableComps {
		allowed[c] = true
	}

	title := extractTitle(description)

	var children []SchemaUINode

	sections := analyzeDescription(desc)
	for _, sec := range sections {
		sectionChildren := []SchemaUINode{}
		for _, comp := range sec.components {
			compType := comp
			if !allowed[comp] {
				compType = CompText
			}
			if _, ok := g.catalog.Get(compType); !ok {
				compType = CompText
			}
			node := buildNode(compType, g.catalog)
			node.ID = generateNodeID(string(compType))
			sectionChildren = append(sectionChildren, node)
		}

		sectionProps, _ := json.Marshal(map[string]any{"title": sec.title})
		children = append(children, SchemaUINode{
			Type:     schema_ui.NodeSection,
			ID:       generateNodeID("section"),
			Props:    sectionProps,
			Children: sectionChildren,
		})
	}

	if len(children) == 0 {
		textProps, _ := json.Marshal(map[string]any{"text": description})
		children = []SchemaUINode{
			{
				Type:   schema_ui.NodeText,
				ID:     generateNodeID("text"),
				Props:  textProps,
			},
		}
	}

	doc := &SchemaUIDocument{
		SchemaVersion: schema_ui.SchemaUIVersion,
		Type:          "document",
		Title:         title,
		Layout: map[string]any{
			"title":     title,
			"generator": "natural-language-schema-ui",
		},
		Children: children,
	}

	return doc, nil
}

// sectionDesc describes an inferred section from the natural language input.
type sectionDesc struct {
	title      string
	components []SchemaComponentType
}

// analyzeDescription parses a lowercase description into sections with component types.
func analyzeDescription(desc string) []sectionDesc {
	var sections []sectionDesc

	// Heuristic keyword-based parsing.
	if containsAny(desc, "form", "input", "submit", "field") {
		components := []SchemaComponentType{CompField, CompSelect, CompSwitch, CompButton}
		sections = append(sections, sectionDesc{title: "Form", components: components})
	}

	if containsAny(desc, "list", "items", "collection") {
		sections = append(sections, sectionDesc{title: "List", components: []SchemaComponentType{CompList}})
	}

	if containsAny(desc, "table", "grid", "rows", "columns", "data") {
		sections = append(sections, sectionDesc{title: "Data", components: []SchemaComponentType{CompTable}})
	}

	if containsAny(desc, "dashboard", "stats", "metrics", "progress", "status") {
		components := []SchemaComponentType{CompCard, CompProgress, CompBadge}
		sections = append(sections, sectionDesc{title: "Dashboard", components: components})
	}

	if containsAny(desc, "profile", "user", "avatar", "image", "info") {
		components := []SchemaComponentType{CompImage, CompText, CompMarkdown}
		sections = append(sections, sectionDesc{title: "Profile", components: components})
	}

	if containsAny(desc, "navigation", "navigate", "link", "button") {
		components := []SchemaComponentType{CompButton, CompTabs}
		sections = append(sections, sectionDesc{title: "Navigation", components: components})
	}

	if containsAny(desc, "setting", "settings", "toggle", "switch", "preference") {
		components := []SchemaComponentType{CompSwitch, CompSlider, CompField}
		sections = append(sections, sectionDesc{title: "Settings", components: components})
	}

	return sections
}

// buildNode constructs a SchemaUINode for a given component type using catalog metadata.
func buildNode(compType SchemaComponentType, catalog *ComponentCatalog) SchemaUINode {
	schema, ok := catalog.Get(compType)
	if !ok {
		return SchemaUINode{
			Type: schema_ui.NodeType(string(compType)),
		}
	}

	props := map[string]any{}
	for _, p := range schema.Properties {
		if p.Default != nil {
			props[p.Name] = p.Default
		}
	}

	for _, p := range schema.Properties {
		if p.Required {
			if _, exists := props[p.Name]; !exists {
				props[p.Name] = placeholderValueForProperty(p)
			}
		}
	}

	propsJSON, _ := json.Marshal(props)

	return SchemaUINode{
		Type:  schema_ui.NodeType(string(compType)),
		Props: propsJSON,
	}
}

// placeholderValueForProperty returns a placeholder value based on property type.
func placeholderValueForProperty(p PropertySchema) any {
	switch p.Type {
	case "string":
		return "placeholder"
	case "number":
		return 0
	case "boolean":
		return false
	case "array":
		return []any{}
	case "object":
		return map[string]any{}
	default:
		return "placeholder"
	}
}

// --- Utility functions ---

// extractTitle derives a title from the description.
func extractTitle(desc string) string {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return "Untitled"
	}
	// Take the first sentence (up to first period or 60 chars).
	end := strings.IndexAny(desc, ".\n")
	if end == -1 || end > 60 {
		end = 60
		if len(desc) < end {
			end = len(desc)
		}
	}
	title := strings.TrimSpace(desc[:end])
	if title == "" {
		return "Untitled"
	}
	// Capitalize first letter.
	return strings.ToUpper(title[:1]) + title[1:]
}

// generateNodeID creates a simple unique-enough node ID.
var nodeIDCounter int

func generateNodeID(prefix string) string {
	nodeIDCounter++
	return fmt.Sprintf("%s_%d", prefix, nodeIDCounter)
}

// generateSchemaID creates a simple schema ID.
var schemaIDCounter int

func generateSchemaID() string {
	schemaIDCounter++
	return fmt.Sprintf("schema_%d", schemaIDCounter)
}

// containsAny checks if the description contains any of the given keywords.
func containsAny(desc string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(desc, kw) {
			return true
		}
	}
	return false
}

// componentTypesToStrings converts a slice of SchemaComponentType to strings.
func componentTypesToStrings(types []SchemaComponentType) []string {
	result := make([]string, len(types))
	for i, t := range types {
		result[i] = string(t)
	}
	return result
}
