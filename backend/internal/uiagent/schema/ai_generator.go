package schema

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/schema_ui"
)

// LLMSchemaCallFunc invokes an LLM to generate a schema from a description.
// It receives a JSON prompt and returns JSON output.
type LLMSchemaCallFunc func(ctx interface{}, promptJSON []byte) ([]byte, error)

// AISchemaGenerator produces SchemaUIDocuments using an LLM.
// Heuristic fallback is prohibited in production paths.
type AISchemaGenerator struct {
	catalog       *ComponentCatalog
	llmCall       LLMSchemaCallFunc
	allowHeuristic bool
}

// NewAISchemaGenerator creates an AI-backed generator for production use.
// Heuristic fallback is disabled; if LLM is unavailable, Generate returns an error.
func NewAISchemaGenerator(catalog *ComponentCatalog, llmCall LLMSchemaCallFunc) *AISchemaGenerator {
	return &AISchemaGenerator{catalog: catalog, llmCall: llmCall, allowHeuristic: false}
}

// NewAISchemaGeneratorWithHeuristic creates a generator that permits heuristic fallback.
// Intended for development and tests only.
func NewAISchemaGeneratorWithHeuristic(catalog *ComponentCatalog, llmCall LLMSchemaCallFunc) *AISchemaGenerator {
	return &AISchemaGenerator{catalog: catalog, llmCall: llmCall, allowHeuristic: true}
}

// SetLLMCallFunc sets the LLM call function after construction.
func (g *AISchemaGenerator) SetLLMCallFunc(llmCall LLMSchemaCallFunc) {
	g.llmCall = llmCall
}

// HasLLMCallFunc returns true if a LLM call function is configured.
func (g *AISchemaGenerator) HasLLMCallFunc() bool {
	return g.llmCall != nil
}

// Generate produces a SchemaUIDocument from a description.
func (g *AISchemaGenerator) Generate(
	description string,
	availableComps []SchemaComponentType,
) (*SchemaUIDocument, error) {
	desc := strings.TrimSpace(description)
	if desc == "" {
		return nil, fmt.Errorf("description must not be empty")
	}

	if g.llmCall != nil {
		doc, err := g.generateWithLLM(desc, availableComps)
		if err == nil {
			return doc, nil
		}
	}

	if g.allowHeuristic {
		return g.generateHeuristic(desc, availableComps)
	}

	return nil, fmt.Errorf("schema generation requires an available LLM provider")
}

func (g *AISchemaGenerator) generateWithLLM(description string, availableComps []SchemaComponentType) (*SchemaUIDocument, error) {
	prompt := buildAIPrompt(description, availableComps)
	promptBytes, err := json.Marshal(prompt)
	if err != nil {
		return nil, fmt.Errorf("marshal prompt: %w", err)
	}

	outputBytes, err := g.llmCall(nil, promptBytes)
	if err != nil {
		return nil, fmt.Errorf("llm call: %w", err)
	}

	var doc SchemaUIDocument
	if err := json.Unmarshal(outputBytes, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal llm output: %w", err)
	}

	if len(doc.Children) == 0 {
		return nil, fmt.Errorf("llm output has no children")
	}
	if doc.SchemaVersion == "" {
		doc.SchemaVersion = schema_ui.SchemaUIVersion
	}
	if doc.Type == "" {
		doc.Type = "document"
	}
	if doc.Title == "" {
		doc.Title = extractTitle(description)
	}
	return &doc, nil
}

func (g *AISchemaGenerator) generateHeuristic(description string, availableComps []SchemaComponentType) (*SchemaUIDocument, error) {
	gen := NewSchemaUIGenerator(g.catalog)
	return gen.Generate(description, availableComps)
}

type AIPrompt struct {
	Task          string   `json:"task"`
	Description   string   `json:"description"`
	AvailableComp []string `json:"availableComponents"`
	Instructions  string   `json:"instructions"`
}

func buildAIPrompt(description string, availableComps []SchemaComponentType) AIPrompt {
	comps := make([]string, len(availableComps))
	for i, c := range availableComps {
		comps[i] = string(c)
	}
	if len(comps) == 0 {
		comps = []string{"page", "section", "text", "button", "field", "card", "list", "table"}
	}

	return AIPrompt{
		Task:          "generate_schema_ui",
		Description:   description,
		AvailableComp: comps,
		Instructions:  "Generate a SchemaUIDocument in JSON format with children nodes. Each node must have 'type' and optionally 'id', 'props', 'children', 'bindings', 'actions'. The document must have schemaVersion, type, title, and children fields.",
	}
}
