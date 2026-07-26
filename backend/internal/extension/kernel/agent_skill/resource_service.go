package agent_skill

import (
	"context"
	"fmt"
)

type ResourceService struct {
	catalog *AgentSkillCatalog
	resolver ResourcePathResolver
	maxReadBytes int64
}

type ResourcePathResolver func(ctx context.Context, extensionID, resourcePath string) ([]byte, error)

func NewResourceService(catalog *AgentSkillCatalog, resolver ResourcePathResolver) *ResourceService {
	return &ResourceService{
		catalog:      catalog,
		resolver:     resolver,
		maxReadBytes: 512 * 1024,
	}
}

func (s *ResourceService) ListResources(ctx context.Context, extensionID string) ([]SkillResourceDescriptor, error) {
	def, ok := s.catalog.Get(extensionID)
	if !ok {
		return nil, fmt.Errorf("agent skill %s not found", extensionID)
	}
	if !def.Enabled {
		return nil, fmt.Errorf("agent skill %s is disabled", extensionID)
	}
	return def.Resources, nil
}

func (s *ResourceService) ReadResource(ctx context.Context, extensionID, resourcePath string) ([]byte, error) {
	def, ok := s.catalog.Get(extensionID)
	if !ok {
		return nil, fmt.Errorf("agent skill %s not found", extensionID)
	}
	if !def.Enabled {
		return nil, fmt.Errorf("agent skill %s is disabled", extensionID)
	}

	var resource *SkillResourceDescriptor
	for _, r := range def.Resources {
		if r.Path == resourcePath {
			resource = &r
			break
		}
	}
	if resource == nil {
		return nil, fmt.Errorf("resource %s not found in skill %s", resourcePath, extensionID)
	}

	if !resource.TextReadable {
		return nil, fmt.Errorf("resource %s is not text readable", resourcePath)
	}

	if resource.Size > s.maxReadBytes {
		return nil, fmt.Errorf("resource %s exceeds max read size", resourcePath)
	}

	if s.resolver == nil {
		return nil, fmt.Errorf("resource resolver not configured")
	}

	return s.resolver(ctx, extensionID, resourcePath)
}
