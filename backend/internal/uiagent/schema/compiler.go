package schema

import (
	"context"
	"errors"
)

type SchemaCompiler interface {
	Compile(ctx context.Context, doc *SchemaUIDocument) (*CompiledSchema, error)
}

type CompiledSchema struct {
	WidgetTree    interface{} `json:"widgetTree"`
	ChannelDefs   interface{} `json:"channelDefs,omitempty"`
	ObjectSchemas interface{} `json:"objectSchemas,omitempty"`
	DataSources   interface{} `json:"dataSources,omitempty"`
	WidgetCount   int         `json:"widgetCount"`
	BinderCount   int         `json:"binderCount"`
}

type schemaCompiler struct{}

func NewSchemaCompiler() SchemaCompiler {
	return &schemaCompiler{}
}

func (c *schemaCompiler) Compile(ctx context.Context, doc *SchemaUIDocument) (*CompiledSchema, error) {
	if doc == nil {
		return nil, ErrDocumentRequired
	}
	return &CompiledSchema{WidgetCount: countNodes(&doc.Root)}, nil
}

func countNodes(node *SchemaNode) int {
	if node == nil {
		return 0
	}
	count := 1
	for i := range node.Children {
		count += countNodes(&node.Children[i])
	}
	return count
}

var ErrDocumentRequired = errors.New("schema compiler: document is required")
