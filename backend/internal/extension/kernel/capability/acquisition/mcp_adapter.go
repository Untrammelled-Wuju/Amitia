package acquisition

import (
	"context"

	"github.com/u-ai/backend/internal/extension/mcpruntime"
)

// mcpRepositoryAdapter provides the installation view expected by MCPPackageSource.
type mcpRepositoryAdapter struct {
	repo *mcp.Repository
}

// NewMCPRepositoryAdapter creates an adapter for the MCP repository.
func NewMCPRepositoryAdapter(repo *mcp.Repository) *mcpRepositoryAdapter {
	return &mcpRepositoryAdapter{repo: repo}
}

// ListInstallations returns all MCP servers as a slice of generic interfaces.
func (a *mcpRepositoryAdapter) ListInstallations() ([]interface{}, error) {
	if a.repo == nil {
		return nil, nil
	}
	servers, err := a.repo.ListServers(context.Background())
	if err != nil {
		return nil, err
	}
	result := make([]interface{}, 0, len(servers))
	for i := range servers {
		result = append(result, &mcpServerAdapter{server: &servers[i]})
	}
	return result, nil
}

// mcpServerAdapter wraps mcp.Server to provide the interface expected
// by MCPPackageSource.buildCandidate.
type mcpServerAdapter struct {
	server *mcp.Server
}

func (a *mcpServerAdapter) GetBindingID() string {
	return a.server.ID
}

func (a *mcpServerAdapter) GetServerName() string {
	return a.server.Name
}

func (a *mcpServerAdapter) GetInstallState() string {
	return a.server.Status
}

func (a *mcpServerAdapter) GetProvidedCapabilities() []string {
	return []string{"mcp.server." + a.server.Name}
}
