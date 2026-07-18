package host

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/mcp"
)

type ConfiguredRoots struct {
	repository *mcp.Repository
}

func NewConfiguredRoots(repository *mcp.Repository) *ConfiguredRoots {
	return &ConfiguredRoots{repository: repository}
}

func (p *ConfiguredRoots) Roots(ctx context.Context, serverID string) ([]Root, error) {
	enabled, configuration, err := p.repository.ServerCapabilityEnabled(ctx, serverID, "roots")
	if err != nil || !enabled {
		return []Root{}, err
	}
	var value struct {
		Roots []Root `json:"roots"`
	}
	if json.Unmarshal(configuration, &value) != nil {
		return []Root{}, nil
	}
	if len(value.Roots) > 20 {
		value.Roots = value.Roots[:20]
	}
	return value.Roots, nil
}
