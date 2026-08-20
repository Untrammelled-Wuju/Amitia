package canonical

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/mcp"
)

// ToolSyncer translates compatibility repository discovery records into the
// single canonical ToolFacade registry. It never registers tools in the legacy
// extension registry.
type ToolSyncer struct {
	repository *mcp.Repository
	facade     *kernel.ToolFacade
}

func NewToolSyncer(repository *mcp.Repository, facade *kernel.ToolFacade) *ToolSyncer {
	return &ToolSyncer{repository: repository, facade: facade}
}

func (s *ToolSyncer) RegisterServer(ctx context.Context, serverID string) error {
	if s == nil || s.repository == nil || s.facade == nil {
		return fmt.Errorf("MCP canonical tool sync unavailable")
	}
	server, err := s.repository.GetServer(ctx, serverID)
	if err != nil {
		return err
	}
	tools, err := s.repository.ListTools(ctx, serverID, false)
	if err != nil {
		return err
	}
	descriptors := make([]capability.MCPToolDescriptor, 0, len(tools))
	for _, tool := range tools {
		if tool.Enabled != 1 {
			continue
		}
		annotations := map[string]any{}
		if strings.TrimSpace(tool.AnnotationsJSON) != "" {
			_ = json.Unmarshal([]byte(tool.AnnotationsJSON), &annotations)
		}
		inputSchema := json.RawMessage(tool.InputSchemaJSON)
		if !json.Valid(inputSchema) {
			inputSchema = json.RawMessage(`{"type":"object"}`)
		}
		outputSchema := json.RawMessage(tool.OutputSchemaJSON)
		if !json.Valid(outputSchema) {
			outputSchema = json.RawMessage(`{}`)
		}
		title := strings.TrimSpace(tool.Title)
		if title == "" {
			title = tool.RemoteName
		}
		descriptors = append(descriptors, capability.MCPToolDescriptor{
			ServerID:     serverID,
			ServerName:   firstNonEmpty(server.DisplayName, server.Name, serverID),
			Name:         tool.RemoteName,
			Title:        title,
			Description:  tool.Description,
			InputSchema:  inputSchema,
			OutputSchema: outputSchema,
			Annotations:  annotations,
			RevisionHash: tool.Hash,
		})
	}
	_, err = s.facade.SyncMCPTools(ctx, serverID, descriptors)
	return err
}

func (s *ToolSyncer) UnregisterServer(ctx context.Context, serverID string) error {
	if s == nil || s.facade == nil {
		return nil
	}
	s.facade.UnregisterMCPTools(ctx, serverID)
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "mcp"
}
