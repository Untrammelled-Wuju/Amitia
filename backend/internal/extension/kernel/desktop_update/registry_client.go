package desktop_update

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultRegistryURL = "https://registry.amitia.dev"

type ReleaseIndex struct {
	SchemaVersion   int      `json:"schemaVersion"`
	GeneratedAt     string   `json:"generatedAt"`
	ExtensionID     string   `json:"extensionId"`
	PublisherID     string   `json:"publisherId"`
	PublisherKeyID  string   `json:"publisherKeyId"`
	Channel         string   `json:"channel"`
	Version         string   `json:"version"`
	DownloadURL     string   `json:"downloadURL"`
	SHA256          string   `json:"sha256"`
	SignatureURL    string   `json:"signatureURL"`
	MinHostVersion  string   `json:"minHostVersion"`
	Platforms       []string `json:"platforms"`
	Architectures   []string `json:"architectures,omitempty"`
	PackageSize     int64    `json:"packageSize"`
	ManifestVersion int      `json:"manifestVersion,omitempty"`
	Signature       string   `json:"signature"`
}

type RegistryClient struct {
	baseURL   string
	client    *http.Client
	publicKey ed25519.PublicKey
}

func NewRegistryClient(baseURL string) *RegistryClient {
	if baseURL == "" {
		baseURL = DefaultRegistryURL
	}
	return &RegistryClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *RegistryClient) SetPublicKeyBase64(encoded string) error {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return fmt.Errorf("%w: invalid registry public key", ErrIndexSignatureInvalid)
	}
	c.publicKey = ed25519.PublicKey(key)
	return nil
}

func (c *RegistryClient) QueryExtension(ctx context.Context, extensionID string) (*ExtensionUpdateMetadata, error) {
	if extensionID == "" {
		return nil, &UpdateError{Code: ErrorCodeIndexInvalid, Err: fmt.Errorf("%w: extension id required", ErrIndexInvalid)}
	}
	endpoint := fmt.Sprintf("%s/api/v1/extensions/%s/latest", c.baseURL, url.PathEscape(extensionID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &UpdateError{Code: ErrorCodeNetwork, Err: fmt.Errorf("%w: %v", ErrRegistryNetwork, err)}
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, &UpdateError{Code: ErrorCodeNetwork, Err: fmt.Errorf("%w: %v", ErrRegistryNetwork, err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &UpdateError{Code: ErrorCodeNetwork, Err: fmt.Errorf("%w: registry returned status %d", ErrRegistryNetwork, resp.StatusCode)}
	}
	var index ReleaseIndex
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return nil, &UpdateError{Code: ErrorCodeIndexInvalid, Err: fmt.Errorf("%w: %v", ErrIndexInvalid, err)}
	}
	if err := validateReleaseIndex(index, extensionID); err != nil {
		return nil, &UpdateError{Code: ErrorCodeIndexInvalid, Err: err}
	}
	if err := c.verifyReleaseIndex(index); err != nil {
		return nil, &UpdateError{Code: ErrorCodeIndexSignatureInvalid, Err: err}
	}
	if index.ManifestVersion == 0 {
		index.ManifestVersion = 1
	}
	publishedAt, _ := time.Parse(time.RFC3339, index.GeneratedAt)
	return &ExtensionUpdateMetadata{
		ExtensionID: index.ExtensionID, Version: index.Version,
		ManifestVersion: index.ManifestVersion, PackageURL: index.DownloadURL,
		PackageSHA256: index.SHA256, PackageSize: index.PackageSize,
		PublisherID: index.PublisherID, PublisherKeyID: index.PublisherKeyID,
		Signature: index.SignatureURL, MinimumHostVersion: index.MinHostVersion,
		SupportedPlatforms: index.Platforms, SupportedArch: index.Architectures,
		PublishedAt: publishedAt, ReleaseChannel: index.Channel, SignedIndex: true,
	}, nil
}

func validateReleaseIndex(index ReleaseIndex, extensionID string) error {
	return validateReleaseIndexFields(index, extensionID, true)
}

func validateReleaseIndexFields(index ReleaseIndex, extensionID string, requireSignature bool) error {
	if index.SchemaVersion != 1 || index.ExtensionID != extensionID || index.PublisherID == "" ||
		index.PublisherKeyID == "" || index.Version == "" || index.DownloadURL == "" ||
		len(index.SHA256) != 64 || index.SignatureURL == "" || index.GeneratedAt == "" ||
		(requireSignature && index.Signature == "") {
		return ErrIndexInvalid
	}
	if _, err := time.Parse(time.RFC3339, index.GeneratedAt); err != nil {
		return fmt.Errorf("%w: generatedAt", ErrIndexInvalid)
	}
	if index.ManifestVersion == 0 {
		index.ManifestVersion = 1
	}
	return nil
}

func SignReleaseIndex(index ReleaseIndex, privateKey ed25519.PrivateKey) ([]byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("%w: invalid signing key", ErrIndexSignatureInvalid)
	}
	index.Signature = ""
	if err := validateReleaseIndexFields(index, index.ExtensionID, false); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(index)
	if err != nil {
		return nil, err
	}
	index.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload))
	return json.MarshalIndent(index, "", "  ")
}

func (c *RegistryClient) verifyReleaseIndex(index ReleaseIndex) error {
	if len(c.publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("%w: trusted registry key unavailable", ErrIndexSignatureInvalid)
	}
	signature, err := base64.StdEncoding.DecodeString(index.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return ErrIndexSignatureInvalid
	}
	index.Signature = ""
	payload, err := json.Marshal(index)
	if err != nil || !ed25519.Verify(c.publicKey, payload, signature) {
		return ErrIndexSignatureInvalid
	}
	return nil
}
