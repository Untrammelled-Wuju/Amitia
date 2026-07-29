// Deprecated: Legacy extension architecture.
// Do not add new capabilities. This implementation is retained only for
// compatibility, maintenance, testing, and migration to Extension Kernel.

package skill

import (
	"context"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/mcp"
)

type ToolFacadeSyncer interface {
	SyncMCPTools(ctx context.Context, serverID string, tools []mcp.ToolDefinition) error
	UnregisterMCPTools(ctx context.Context, serverID string) error
}

type DuplicateRecorder interface {
	RecordDuplicate(ctx context.Context, toolID, serverID, owner string, generation int64) error
	ResolveByToolID(ctx context.Context, toolID string) error
}

type Runtime struct {
	repository        *mcp.Repository
	extensions        *extension.Runtime
	toolFacadeSyncer  ToolFacadeSyncer
	duplicateRecorder DuplicateRecorder
}

type Option func(*Runtime)

func WithToolFacadeSyncer(syncer ToolFacadeSyncer) Option {
	return func(r *Runtime) {
		r.toolFacadeSyncer = syncer
	}
}

func WithDuplicateRecorder(recorder DuplicateRecorder) Option {
	return func(r *Runtime) {
		r.duplicateRecorder = recorder
	}
}

func New(repository *mcp.Repository, extensions *extension.Runtime, opts ...Option) *Runtime {
	r := &Runtime{repository: repository, extensions: extensions}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *Runtime) SetToolFacadeSyncer(syncer ToolFacadeSyncer) {
	r.toolFacadeSyncer = syncer
}

func (r *Runtime) SetDuplicateRecorder(recorder DuplicateRecorder) {
	r.duplicateRecorder = recorder
}

func (r *Runtime) RegisterAll(ctx context.Context) error {
	servers, err := r.repository.ListServers(ctx)
	if err != nil {
		return err
	}
	for _, server := range servers {
		if err := r.RegisterServer(ctx, server.ID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) RegisterServer(ctx context.Context, serverID string) error {
	if _, err := r.repository.GetServer(ctx, serverID); err != nil {
		return err
	}
	tools, err := r.repository.ListTools(ctx, serverID, false)
	if err != nil {
		return err
	}
	registered, err := r.extensions.Registry.List(ctx, extension.SkillFilter{Source: extension.SkillSourceMCP, IncludeInternal: true})
	if err != nil {
		return err
	}
	for _, current := range registered {
		if strings.HasPrefix(current.Definition.ID, "mcp."+skillSegment(serverID)+".") {
			kernel.GlobalLegacyCallCounter().IncDuplicateMCPToolRegistration()
			if r.duplicateRecorder != nil {
				_ = r.duplicateRecorder.RecordDuplicate(ctx, current.Definition.ID, serverID, current.Definition.Entry.Name, 0)
			}
			if err := r.extensions.Registry.Unregister(ctx, current.Definition.ID); err != nil {
				return fmt.Errorf("failed to unregister legacy MCP tool %s: %w", current.Definition.ID, err)
			}
			if r.duplicateRecorder != nil {
				_ = r.duplicateRecorder.ResolveByToolID(ctx, current.Definition.ID)
			}
		}
	}
	if r.toolFacadeSyncer == nil {
		return fmt.Errorf("toolFacadeSyncer is not configured")
	}
	return r.toolFacadeSyncer.SyncMCPTools(ctx, serverID, tools)
}

func (r *Runtime) UnregisterServer(ctx context.Context, serverID string) error {
	registered, err := r.extensions.Registry.List(ctx, extension.SkillFilter{Source: extension.SkillSourceMCP, IncludeInternal: true})
	if err != nil {
		return err
	}
	prefix := "mcp." + skillSegment(serverID) + "."
	for _, current := range registered {
		if strings.HasPrefix(current.Definition.ID, prefix) {
			if err := r.extensions.Registry.Unregister(ctx, current.Definition.ID); err != nil {
				return fmt.Errorf("failed to unregister legacy MCP tool %s: %w", current.Definition.ID, err)
			}
		}
	}
	if r.toolFacadeSyncer != nil {
		return r.toolFacadeSyncer.UnregisterMCPTools(ctx, serverID)
	}
	return nil
}

func normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var result strings.Builder
	for _, item := range value {
		if item >= 'a' && item <= 'z' || item >= '0' && item <= '9' {
			result.WriteRune(item)
		} else if result.Len() > 0 && !strings.HasSuffix(result.String(), "_") {
			result.WriteByte('_')
		}
	}
	return strings.Trim(result.String(), "_")
}
func skillSegment(value string) string { return strings.ReplaceAll(normalize(value), "_", "-") }
