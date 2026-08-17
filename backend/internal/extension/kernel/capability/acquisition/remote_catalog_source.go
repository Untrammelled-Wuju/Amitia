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

// RemoteCatalogEntry represents a single entry fetched from the remote catalog API.
type RemoteCatalogEntry struct {
	ExtensionID         string   `json:"extensionId"`
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	Version             string   `json:"version"`
	PackageURI          string   `json:"packageUri,omitempty"`
	Hash                string   `json:"hash,omitempty"`
	SizeBytes           int64    `json:"sizeBytes,omitempty"`
	ProvidedCapabilities []string `json:"providedCapabilities,omitempty"`
	Trust               string   `json:"trust,omitempty"`
	Source              string   `json:"source,omitempty"`
}

// RemoteCatalogResponse represents the expected response from the remote catalog API.
type RemoteCatalogResponse struct {
	Entries []RemoteCatalogEntry `json:"entries"`
}

// RemoteCatalogSource fetches capability candidates from a remote catalog HTTP API.
type RemoteCatalogSource struct {
	client     *http.Client
	apiURL string
}

// NewRemoteCatalogSource creates a RemoteCatalogSource with the given API URL.
func NewRemoteCatalogSource(apiURL string) *RemoteCatalogSource {
	return &RemoteCatalogSource{
		client: &http.Client{Timeout: 15 * time.Second},
		apiURL: apiURL,
	}
}

func (s *RemoteCatalogSource) ID() string {
	return "remote_catalog"
}

func (s *RemoteCatalogSource) Kind() CandidateKind {
	return CandidateExtensionPackage
}

func (s *RemoteCatalogSource) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	if s.apiURL == "" {
		return nil, nil
	}

	resp, err := s.fetchCatalog(ctx)
	if err != nil {
		return nil, fmt.Errorf("remote catalog fetch: %w", err)
	}

	var entries []RemoteCatalogEntry
	if err := json.Unmarshal(resp, &entries); err != nil {
		var wrapped RemoteCatalogResponse
		if err := json.Unmarshal(resp, &wrapped); err != nil {
			return nil, fmt.Errorf("remote catalog parse: %w", err)
		}
		entries = wrapped.Entries
	}

	capID := string(request.CapabilityID)
	var candidates []CapabilityCandidate
	for _, entry := range entries {
		if !s.matchesRequest(entry, capID) {
			continue
		}
		candidates = append(candidates, s.toCandidate(entry))
	}

	if len(candidates) == 0 {
		return nil, nil
	}
	return candidates, nil
}

func (s *RemoteCatalogSource) fetchCatalog(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remote catalog: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (s *RemoteCatalogSource) matchesRequest(entry RemoteCatalogEntry, capID string) bool {
	if capID == "" {
		return true
	}
	if containsString(entry.Name, capID) || containsString(entry.Description, capID) || containsString(entry.ExtensionID, capID) {
		return true
	}
	for _, c := range entry.ProvidedCapabilities {
		if c == capID {
			return true
		}
	}
	return false
}

func (s *RemoteCatalogSource) toCandidate(entry RemoteCatalogEntry) CapabilityCandidate {
	var caps []capability.CapabilityID
	for _, c := range entry.ProvidedCapabilities {
		if c != "" {
			caps = append(caps, capability.CapabilityID(c))
		}
	}
	trustLevel := TrustLevel(entry.Trust)
	if trustLevel == "" {
		trustLevel = TrustUnverified
	}
	install := CandidateInstallDescriptor{
		Method: InstallExtension,
	}
	if entry.PackageURI != "" {
		install.ExtensionPackage = &ExtensionInstallDescriptor{
			PackageURI: entry.PackageURI,
			Hash:       entry.Hash,
		}
	}
	return CapabilityCandidate{
		ID:           entry.ExtensionID,
		Kind:         CandidateExtensionPackage,
		Name:         entry.Name,
		Description:  entry.Description,
		Version:      entry.Version,
		Capabilities: caps,
		Install:      install,
		Trust:        CandidateTrust{Level: trustLevel},
		Metadata: map[string]any{
			"extensionId": entry.ExtensionID,
			"source":      entry.Source,
		},
	}
}
