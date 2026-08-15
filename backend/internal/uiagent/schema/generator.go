package schema

import (
	"fmt"
	"strings"
)

// SchemaDraftState represents the lifecycle state of a schema document.
type SchemaDraftState string

const (
	DraftStateDraft     SchemaDraftState = "draft"
	DraftStatePreview   SchemaDraftState = "preview"
	DraftStatePublished SchemaDraftState = "published"
)

// SchemaUIDocument is the top-level representation of a generated schema UI.
type SchemaUIDocument struct {
	Version     string            `json:"version"`
	SchemaID    string            `json:"schemaId"`
	Revision    int               `json:"revision"`
	State       SchemaDraftState `json:"state"`
	Title       string            `json:"title"`
	Root        SchemaNode        `json:"root"`
	ExtensionID string            `json:"extensionId,omitempty"`
	Metadata    map[string]any    `json:"metadata,omitempty"`
}

// SchemaNode is a single node in the schema tree.
type SchemaNode struct {
	Type       string         `json:"type"`
	ID         string         `json:"id,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
	Children   []SchemaNode   `json:"children,omitempty"`
	Bindings   []DataBinding  `json:"bindings,omitempty"`
	Actions    []SchemaAction `json:"actions,omitempty"`
}

// DataBinding connects a node property to a data source.
type DataBinding struct {
	Property   string `json:"property"`
	Source     string `json:"source"`
	DataSource string `json:"dataSource,omitempty"`
}

// SchemaAction defines an action triggered by user interaction.
type SchemaAction struct {
	Trigger string         `json:"trigger"`
	Type    string         `json:"type"`
	Target  string         `json:"target,omitempty"`
	Payload map[string]any `json:"payload,omitempty"`
}

// SchemaUIGenerator generates SchemaUIDocuments from natural language descriptions.
type SchemaUIGenerator struct {
	catalog *ComponentCatalog
}

// NewSchemaUIGenerator creates a new generator backed by the given catalog.
func NewSchemaUIGenerator(catalog *ComponentCatalog) *SchemaUIGenerator {
	return &SchemaUIGenerator{catalog: catalog}
}

// Generate produces a valid SchemaUIDocument from a description.
// The description is parsed to determine the layout and components needed.
func (g *SchemaUIGenerator) Generate(
	description string,
	availableComps []SchemaComponentType,
) (*SchemaUIDocument, error) {
	desc := strings.ToLower(strings.TrimSpace(description))
	if desc == "" {
		return nil, fmt.Errorf("description must not be empty")
	}

	// Build the set of allowed component types.
	allowed := make(map[SchemaComponentType]bool)
	for _, c := range availableComps {
		allowed[c] = true
	}

	// Determine the title from the description (first sentence or first 60 chars).
	title := extractTitle(description)

	// Build the root page node.
	rootNode := SchemaNode{
		Type:       string(CompPage),
		ID:         "root",
		Properties: map[string]any{"title": title},
		Children:   []SchemaNode{},
	}

	// Analyze the description to determine required sections and components.
	sections := analyzeDescription(desc)

	for _, sec := range sections {
		sectionNode := SchemaNode{
			Type:       string(CompSection),
			ID:         generateNodeID("section"),
			Properties: map[string]any{"title": sec.title},
			Children:   []SchemaNode{},
		}

		for _, comp := range sec.components {
			// Fall back to text if the component is not in the allowed list.
			compType := comp
			if !allowed[comp] {
				compType = CompText
			}

			 // Skip if the catalog doesn't know this type.
			if _, ok := g.catalog.Get(compType); !ok {
				compType = CompText
			}

			node := buildNode(compType, g.catalog)
			node.ID = generateNodeID(string(compType))
			sectionNode.Children = append(sectionNode.Children, node)
		}

		rootNode.Children = append(rootNode.Children, sectionNode)
	}

	// If no sections were inferred, create a default one with text.
	if len(rootNode.Children) == 0 {
		sectionNode := SchemaNode{
			Type:       string(CompSection),
			ID:         generateNodeID("section"),
			Properties: map[string]any{"title": title},
			Children: []SchemaNode{
				{
					Type:       string(CompText),
					ID:         generateNodeID("text"),
					Properties: map[string]any{"content": description},
				},
			},
		}
		rootNode.Children = append(rootNode.Children, sectionNode)
	}

	doc := &SchemaUIDocument{
		Version:  "1.0",
		SchemaID: generateSchemaID(),
		Revision: 1,
		State:    DraftStateDraft,
		Title:    title,
		Root:     rootNode,
		Metadata: map[string]any{
			"generator":  "natural-language-schema-ui",
			"generated":  true,
			"components": componentTypesToStrings(availableComps),
		},
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

// buildNode constructs a SchemaNode for a given component type using catalog metadata.
func buildNode(compType SchemaComponentType, catalog *ComponentCatalog) SchemaNode {
	schema, ok := catalog.Get(compType)
	if !ok {
		return SchemaNode{
			Type:       string(compType),
			Properties: map[string]any{},
		}
	}

	props := map[string]any{}
	// Apply default values from the property schema.
	for _, p := range schema.Properties {
		if p.Default != nil {
			props[p.Name] = p.Default
		}
	}

	// Provide reasonable placeholder values for required properties.
	for _, p := range schema.Properties {
		if p.Required {
			if _, exists := props[p.Name]; !exists {
				props[p.Name] = placeholderForProperty(p)
			}
		}
	}

	return SchemaNode{
		Type:       string(compType),
		Properties: props,
	}
}

// placeholderForProperty returns a placeholder value based on property type.
func placeholderForProperty(p PropertySchema) any {
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
