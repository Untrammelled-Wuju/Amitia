package schema

import (
	"context"
	"fmt"
)

// FlutterRenderer renders a SchemaUIDocument into a Flutter-compatible widget tree.
type FlutterRenderer struct {
	catalog *ComponentCatalog
}

// NewFlutterRenderer creates a renderer backed by the given catalog.
func NewFlutterRenderer(catalog *ComponentCatalog) *FlutterRenderer {
	return &FlutterRenderer{catalog: catalog}
}

// RenderOutput is the result of rendering a schema document.
type RenderOutput struct {
	WidgetTree   *WidgetNode `json:"widgetTree"`
	WidgetCount  int         `json:"widgetCount"`
	BinderCount  int         `json:"binderCount"`
	HasErrors    bool        `json:"hasErrors"`
	Errors       []string    `json:"errors,omitempty"`
	PreviewToken string      `json:"previewToken,omitempty"`
}

// WidgetNode represents a Flutter widget in the render tree.
type WidgetNode struct {
	Type       string                 `json:"type"`
	WidgetName string                 `json:"widgetName"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Children   []*WidgetNode          `json:"children,omitempty"`
	Binding    *WidgetBinding         `json:"binding,omitempty"`
}

// WidgetBinding represents a data binding on a widget.
type WidgetBinding struct {
	Property string `json:"property"`
	Source   string `json:"source"`
}

// Render converts a SchemaUIDocument into a Flutter widget tree.
func (r *FlutterRenderer) Render(ctx context.Context, doc *SchemaUIDocument, req RenderSchemaRequest) (*RenderOutput, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is nil")
	}
	if doc.Root.Type == "" {
		return nil, fmt.Errorf("document root is empty")
	}

	output := &RenderOutput{
		PreviewToken: req.PreviewToken,
	}

	rootWidget, errs := r.renderNode(&doc.Root)
	output.WidgetTree = rootWidget
	output.WidgetCount = countWidgetNodes(rootWidget)
	output.BinderCount = countBindings(rootWidget)

	if len(errs) > 0 {
		output.HasErrors = true
		output.Errors = errs
	}

	return output, nil
}

func (r *FlutterRenderer) renderNode(node *SchemaNode) (*WidgetNode, []string) {
	if node == nil {
		return nil, nil
	}

	widget := &WidgetNode{
		Type:       node.Type,
		WidgetName: r.mapToFlutterWidget(node.Type),
		Properties: make(map[string]interface{}),
	}

	var errors []string

	for k, v := range node.Properties {
		widget.Properties[k] = v
	}

	for _, binding := range node.Bindings {
		widget.Binding = &WidgetBinding{
			Property: binding.Property,
			Source:   binding.Source,
		}
		break
	}

	for _, child := range node.Children {
		childWidget, childErrs := r.renderNode(&child)
		if childWidget != nil {
			widget.Children = append(widget.Children, childWidget)
		}
		errors = append(errors, childErrs...)
	}

	if r.catalog != nil {
		if schema, ok := r.catalog.Get(SchemaComponentType(node.Type)); ok {
			for _, reqProp := range schema.RequiredProps {
				if _, exists := widget.Properties[reqProp]; !exists {
					errors = append(errors, fmt.Sprintf("node %q missing required property %q", node.Type, reqProp))
				}
			}
		}
	}

	return widget, errors
}

func (r *FlutterRenderer) mapToFlutterWidget(schemaType string) string {
	switch SchemaComponentType(schemaType) {
	case CompPage:
		return "Scaffold"
	case CompSection:
		return "Column"
	case CompStack:
		return "Column"
	case CompRow:
		return "Row"
	case CompGrid:
		return "GridView"
	case CompTabs:
		return "TabBarView"
	case CompCard:
		return "Card"
	case CompText:
		return "Text"
	case CompMarkdown:
		return "Markdown"
	case CompBadge:
		return "Chip"
	case CompIcon:
		return "Icon"
	case CompImage:
		return "Image"
	case CompField:
		return "TextField"
	case CompSelect:
		return "DropdownButton"
	case CompSwitch:
		return "Switch"
	case CompSlider:
		return "Slider"
	case CompButton:
		return "ElevatedButton"
	case CompList:
		return "ListView"
	case CompTable:
		return "DataTable"
	case CompProgress:
		return "LinearProgressIndicator"
	default:
		return "Container"
	}
}

func countWidgetNodes(node *WidgetNode) int {
	if node == nil {
		return 0
	}
	count := 1
	for _, child := range node.Children {
		count += countWidgetNodes(child)
	}
	return count
}

func countBindings(node *WidgetNode) int {
	if node == nil {
		return 0
	}
	count := 0
	if node.Binding != nil {
		count = 1
	}
	for _, child := range node.Children {
		count += countBindings(child)
	}
	return count
}
