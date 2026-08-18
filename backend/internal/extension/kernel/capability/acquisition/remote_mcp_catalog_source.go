package acquisition

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

// RemoteMCPCatalogEntry is a catalog-owned install descriptor for an MCP
// server that is not necessarily present on the local machine yet.
type RemoteMCPCatalogEntry struct {
	ID                   string            `json:"id"`
	ServerName           string            `json:"serverName"`
	Name                 string            `json:"name,omitempty"`
	Description          string            `json:"description,omitempty"`
	Version              string            `json:"version,omitempty"`
	Transport            string            `json:"transport"`
	Command              string            `json:"command,omitempty"`
	Args                 []string          `json:"args,omitempty"`
	Env                  map[string]string `json:"env,omitempty"`
	Registry             string            `json:"registry,omitempty"`
	ProvidedCapabilities []string          `json:"providedCapabilities"`
	Trust                string            `json:"trust,omitempty"`
}

type RemoteMCPCatalogSource struct {
	client *http.Client
	apiURL string
}

func NewRemoteMCPCatalogSource(apiURL string) *RemoteMCPCatalogSource {
	return &RemoteMCPCatalogSource{client: &http.Client{Timeout: 15 * time.Second}, apiURL: apiURL}
}

func (s *RemoteMCPCatalogSource) ID() string          { return "remote_mcp_catalog" }
func (s *RemoteMCPCatalogSource) Kind() CandidateKind { return CandidateMCP }

func (s *RemoteMCPCatalogSource) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	if s.apiURL == "" {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remote MCP catalog fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remote MCP catalog: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("remote MCP catalog read: %w", err)
	}
	var entries []RemoteMCPCatalogEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		var wrapped struct {
			Entries []RemoteMCPCatalogEntry `json:"entries"`
		}
		if err2 := json.Unmarshal(body, &wrapped); err2 != nil {
			return nil, fmt.Errorf("remote MCP catalog parse: %w", err)
		}
		entries = wrapped.Entries
	}

	wanted := string(request.CapabilityID)
	candidates := make([]CapabilityCandidate, 0)
	for _, entry := range entries {
		if entry.ServerName == "" {
			continue
		}
		caps := make([]capability.CapabilityID, 0, len(entry.ProvidedCapabilities))
		matched := wanted == ""
		for _, raw := range entry.ProvidedCapabilities {
			if raw == "" {
				continue
			}
			caps = append(caps, capability.CapabilityID(raw))
			if raw == wanted {
				matched = true
			}
		}
		if !matched || len(caps) == 0 {
			continue
		}
		trustLevel := TrustLevel(entry.Trust)
		if trustLevel == "" {
			trustLevel = TrustUnverified
		}
		id := entry.ID
		if id == "" {
			id = "mcp:" + entry.ServerName
		}
		name := entry.Name
		if name == "" {
			name = entry.ServerName
		}
		candidates = append(candidates, CapabilityCandidate{
			ID: id, Kind: CandidateMCP, Name: name, Description: entry.Description, Version: entry.Version,
			Capabilities: caps,
			Source:       CandidateSource{Registry: entry.Registry, URI: s.apiURL},
			Install: CandidateInstallDescriptor{Method: InstallMCP, MCP: &MCPInstallDescriptor{
				ServerName: entry.ServerName, Transport: entry.Transport, Command: entry.Command,
				Args: entry.Args, Env: entry.Env, Registry: entry.Registry,
			}},
			Trust: CandidateTrust{Level: trustLevel, SourceVerified: trustLevel == TrustVerified || trustLevel == TrustTrusted || trustLevel == TrustBuiltin},
		})
	}
	return candidates, nil
}
