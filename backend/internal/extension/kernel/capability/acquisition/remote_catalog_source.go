package acquisition

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/timeoutpolicy"
)

type RemoteCatalogEntry struct {
	ExtensionID          string                 `json:"extensionId"`
	Name                 string                 `json:"name"`
	Description          string                 `json:"description"`
	Version              string                 `json:"version"`
	PackageURI           string                 `json:"packageUri,omitempty"`
	Hash                 string                 `json:"hash,omitempty"`
	SizeBytes            int64                  `json:"sizeBytes,omitempty"`
	ProvidedCapabilities []string               `json:"providedCapabilities,omitempty"`
	Trust                string                 `json:"trust,omitempty"`
	Source               string                 `json:"source,omitempty"`
	Publisher            RemoteCatalogPublisher `json:"publisher,omitempty"`
	Package              RemoteCatalogPackage   `json:"package,omitempty"`
	Signature            RemoteCatalogSignature `json:"signature,omitempty"`
	Categories           []string               `json:"categories,omitempty"`
	Permissions          []string               `json:"permissions,omitempty"`
	ReleaseNotes         string                 `json:"releaseNotes,omitempty"`
	PublishedAt          string                 `json:"publishedAt,omitempty"`
}

type RemoteCatalogPublisher struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName,omitempty"`
	TrustLevel  string `json:"trustLevel,omitempty"`
	Homepage    string `json:"homepage,omitempty"`
}

type RemoteCatalogPackage struct {
	URI       string `json:"uri"`
	SHA256    string `json:"sha256"`
	FileName  string `json:"fileName,omitempty"`
	SizeBytes int64  `json:"sizeBytes,omitempty"`
	MediaType string `json:"mediaType,omitempty"`
}

type RemoteCatalogSignature struct {
	Status    string `json:"status,omitempty"`
	Algorithm string `json:"algorithm,omitempty"`
	KeyID     string `json:"keyId,omitempty"`
	Value     string `json:"value,omitempty"`
	SignedAt  string `json:"signedAt,omitempty"`
}

type RemoteCatalogSourceRef struct {
	ID          string `json:"id"`
	Type        string `json:"type,omitempty"`
	URL         string `json:"url,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	TrustLevel  string `json:"trustLevel,omitempty"`
}

type RemoteCatalogResponse struct {
	SchemaVersion string                 `json:"schemaVersion,omitempty"`
	ID            string                 `json:"id,omitempty"`
	Name          string                 `json:"name,omitempty"`
	Source        RemoteCatalogSourceRef `json:"source,omitempty"`
	Entries       []RemoteCatalogEntry   `json:"entries"`
}

type RemoteCatalogSource struct {
	client *http.Client
	apiURL string
	id     string
}

func NewRemoteCatalogSource(apiURL string) *RemoteCatalogSource {
	sum := sha256.Sum256([]byte(apiURL))
	return NewRemoteCatalogSourceWithID(fmt.Sprintf("remote_catalog_%x", sum[:6]), apiURL)
}

func NewRemoteCatalogSourceWithID(id, apiURL string) *RemoteCatalogSource {
	if strings.TrimSpace(id) == "" {
		id = "remote_catalog"
	}
	return &RemoteCatalogSource{
		client: timeoutpolicy.Client(&http.Client{Timeout: 15 * time.Second}),
		apiURL: apiURL,
		id:     id,
	}
}

func (s *RemoteCatalogSource) ID() string {
	return s.id
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
	catalog := RemoteCatalogResponse{}
	if err := json.Unmarshal(resp, &catalog); err != nil {
		var entries []RemoteCatalogEntry
		if entriesErr := json.Unmarshal(resp, &entries); entriesErr != nil {
			return nil, fmt.Errorf("remote catalog parse: %w", err)
		}
		catalog.Entries = entries
	}
	capID := string(request.CapabilityID)
	var candidates []CapabilityCandidate
	for _, entry := range catalog.Entries {
		if !s.matchesRequest(entry, capID) {
			continue
		}
		candidates = append(candidates, s.toCandidate(entry, catalog))
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
	for _, item := range entry.ProvidedCapabilities {
		if item == capID {
			return true
		}
	}
	return false
}

func (s *RemoteCatalogSource) toCandidate(entry RemoteCatalogEntry, catalog RemoteCatalogResponse) CapabilityCandidate {
	caps := make([]capability.CapabilityID, 0, len(entry.ProvidedCapabilities))
	for _, item := range entry.ProvidedCapabilities {
		if item != "" {
			caps = append(caps, capability.CapabilityID(item))
		}
	}
	packageURI := strings.TrimSpace(entry.Package.URI)
	if packageURI == "" {
		packageURI = strings.TrimSpace(entry.PackageURI)
	}
	packageHash := strings.TrimSpace(entry.Package.SHA256)
	if packageHash == "" {
		packageHash = strings.TrimSpace(entry.Hash)
	}
	fileName := strings.TrimSpace(entry.Package.FileName)
	sizeBytes := entry.Package.SizeBytes
	if sizeBytes == 0 {
		sizeBytes = entry.SizeBytes
	}
	sourceID := strings.TrimSpace(entry.Source)
	if sourceID == "" {
		sourceID = strings.TrimSpace(catalog.Source.ID)
	}
	if sourceID == "" {
		sourceID = s.ID()
	}
	publisherID := strings.TrimSpace(entry.Publisher.ID)
	if publisherID == "" {
		publisherID = sourceID
	}
	install := CandidateInstallDescriptor{Method: InstallExtension}
	if packageURI != "" {
		install.ExtensionPackage = &ExtensionInstallDescriptor{PackageURI: packageURI, Hash: packageHash}
	}
	lowerURI := strings.ToLower(packageURI)
	sourceVerified := strings.HasPrefix(lowerURI, "https://") || strings.HasPrefix(lowerURI, "file://")
	return CapabilityCandidate{
		ID:           sourceID + ":" + entry.ExtensionID + "@" + entry.Version,
		ExtensionID:  entry.ExtensionID,
		PackageName:  entry.ExtensionID,
		Kind:         CandidateExtensionPackage,
		Name:         entry.Name,
		Description:  entry.Description,
		Version:      entry.Version,
		Capabilities: caps,
		Source: CandidateSource{
			Registry: sourceID, URI: packageURI, Verified: sourceVerified, Publisher: publisherID,
		},
		Install: install,
		Trust: CandidateTrust{
			Level: normalizeRemoteCatalogTrust(entry.Trust), SourceVerified: sourceVerified,
		},
		Metadata: map[string]any{
			"extensionId": entry.ExtensionID, "source": sourceID, "sourceType": catalog.Source.Type,
			"sourceURL": catalog.Source.URL, "catalogId": catalog.ID, "schemaVersion": catalog.SchemaVersion,
			"packageFile": fileName, "packageSize": sizeBytes, "publisher": entry.Publisher,
			"signature": entry.Signature, "categories": entry.Categories, "permissions": entry.Permissions,
			"releaseNotes": entry.ReleaseNotes, "publishedAt": entry.PublishedAt,
		},
	}
}

func normalizeRemoteCatalogTrust(value string) TrustLevel {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(TrustBlocked):
		return TrustBlocked
	case string(TrustBuiltin):
		return TrustBuiltin
	default:
		return TrustUnverified
	}
}
