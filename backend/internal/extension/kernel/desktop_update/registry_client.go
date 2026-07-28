package desktop_update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultRegistryURL = "https://registry.amitia.dev"

type RegistryClient struct {
	baseURL string
	client  *http.Client
}

func NewRegistryClient(baseURL string) *RegistryClient {
	if baseURL == "" {
		baseURL = DefaultRegistryURL
	}
	return &RegistryClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type registryExtensionResponse struct {
	ExtensionID        string   `json:"extensionId"`
	Version            string   `json:"version"`
	ManifestVersion    int      `json:"manifestVersion"`
	PackageURL         string   `json:"packageUrl"`
	PackageSHA512      string   `json:"packageSha512"`
	PackageSHA256      string   `json:"packageSha256"`
	PackageSize        int64    `json:"packageSize"`
	PublisherID        string   `json:"publisherId"`
	PublisherKeyID     string   `json:"publisherKeyId,omitempty"`
	Signature          string   `json:"signature,omitempty"`
	MinimumHostVersion string   `json:"minimumHostVersion,omitempty"`
	MaximumHostVersion string   `json:"maximumHostVersion,omitempty"`
	SupportedPlatforms []string `json:"supportedPlatforms,omitempty"`
	SupportedArch      []string `json:"supportedArch,omitempty"`
	PublishedAt        string   `json:"publishedAt,omitempty"`
	ReleaseChannel     string   `json:"releaseChannel,omitempty"`
}

func (c *RegistryClient) QueryExtension(ctx context.Context, extensionID string) (*ExtensionUpdateMetadata, error) {
	if extensionID == "" {
		return nil, fmt.Errorf("%w: extension id required", ErrInvalidMetadata)
	}

	endpoint := fmt.Sprintf("%s/api/v1/extensions/%s/latest", c.baseURL, url.PathEscape(extensionID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("desktop_update: create registry request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("desktop_update: registry request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("desktop_update: registry returned status %d", resp.StatusCode)
	}

	var regResp registryExtensionResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return nil, fmt.Errorf("desktop_update: decode registry response: %w", err)
	}

	meta := &ExtensionUpdateMetadata{
		ExtensionID:        regResp.ExtensionID,
		Version:            regResp.Version,
		ManifestVersion:    regResp.ManifestVersion,
		PackageURL:         regResp.PackageURL,
		PackageSHA512:      regResp.PackageSHA512,
		PackageSHA256:      regResp.PackageSHA256,
		PackageSize:        regResp.PackageSize,
		PublisherID:        regResp.PublisherID,
		PublisherKeyID:     regResp.PublisherKeyID,
		Signature:          regResp.Signature,
		MinimumHostVersion: regResp.MinimumHostVersion,
		MaximumHostVersion: regResp.MaximumHostVersion,
		SupportedPlatforms: regResp.SupportedPlatforms,
		SupportedArch:      regResp.SupportedArch,
		ReleaseChannel:     regResp.ReleaseChannel,
	}

	if meta.ExtensionID == "" {
		meta.ExtensionID = extensionID
	}
	if meta.ManifestVersion == 0 {
		meta.ManifestVersion = 1
	}
	if regResp.PublishedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, regResp.PublishedAt); err == nil {
			meta.PublishedAt = parsed
		}
	}
	if meta.PublishedAt.IsZero() {
		meta.PublishedAt = time.Now().UTC()
	}

	return meta, nil
}
