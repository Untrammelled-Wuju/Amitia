package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

type PrepareHostPluginPackageRequest struct {
	SpaceID     string
	ExtensionID string
	Version     string
	PackageURI  string
	SHA256      string
	FileName    string
	Source      PackageSourceProvenance
}

type HostPluginPackageManagerPort interface {
	Prepare(context.Context, PrepareHostPluginPackageRequest) (InstallPreview, error)
	Install(context.Context, InstallHostPluginPackageRequest) (KernelInstallResult, error)
	Uninstall(context.Context, UninstallHostPluginPackageRequest) (PackageOperationRecord, error)
	List(context.Context) (HostPluginInventory, error)
}

type InstallHostPluginPackageRequest struct {
	SpaceID          string
	ExtensionID      string
	Version          string
	PreviewSessionID string
	Confirmations    map[string]bool
	IdempotencyKey   string
}

type UninstallHostPluginPackageRequest struct {
	SpaceID        string
	ExtensionID    string
	IdempotencyKey string
}

type HostPluginInstallation struct {
	ExtensionID       string `json:"extensionId"`
	Version           string `json:"version"`
	InstallationState string `json:"installationState"`
	Enabled           bool   `json:"enabled"`
	SystemManaged     bool   `json:"systemManaged"`
}

type HostPluginInventory struct {
	Plugins []HostPluginInstallation `json:"plugins"`
}

type HostPluginPackageManager struct {
	runtime  *Runtime
	profile  runtimeprofile.Profile
	client   *http.Client
	maxBytes int64
}

func NewHostPluginPackageManager(runtime *Runtime, profile runtimeprofile.Profile) *HostPluginPackageManager {
	return &HostPluginPackageManager{
		runtime: runtime,
		profile: profile,
		client: &http.Client{
			Timeout: 30 * time.Minute,
			CheckRedirect: func(request *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("plugin package redirect limit exceeded")
				}
				if request.URL == nil || request.URL.User != nil {
					return errors.New("plugin package redirect must not contain user information")
				}
				if profile == runtimeprofile.ProfileCloudCore && request.URL.Scheme != "https" {
					return errors.New("统筹中心插件包重定向必须使用 HTTPS")
				}
				if len(via) > 0 && via[len(via)-1].URL.Host != request.URL.Host {
					request.Header.Del("Authorization")
					request.Header.Del("Cookie")
				}
				return nil
			},
		},
		maxBytes: 4 << 30,
	}
}

func (m *HostPluginPackageManager) Prepare(ctx context.Context, request PrepareHostPluginPackageRequest) (InstallPreview, error) {
	if m == nil || m.runtime == nil {
		return InstallPreview{}, errors.New("plugin package manager unavailable")
	}
	extensionID := strings.TrimSpace(request.ExtensionID)
	version := strings.TrimSpace(request.Version)
	packageURI := strings.TrimSpace(request.PackageURI)
	expectedHash := strings.ToLower(strings.TrimSpace(request.SHA256))
	if extensionID == "" || version == "" || packageURI == "" {
		return InstallPreview{}, errors.New("extensionId, version and packageUri are required")
	}
	if len(expectedHash) != sha256.Size*2 {
		return InstallPreview{}, errors.New("sha256 must be a 64-character hexadecimal digest")
	}
	if _, err := hex.DecodeString(expectedHash); err != nil {
		return InstallPreview{}, fmt.Errorf("sha256 must be valid hexadecimal: %w", err)
	}
	reader, fileName, resolvedURL, err := m.openPackage(ctx, packageURI, request.FileName)
	if err != nil {
		return InstallPreview{}, err
	}
	defer reader.Close()
	file, err := os.CreateTemp("", "amitia-plugin-*.amitiax")
	if err != nil {
		return InstallPreview{}, fmt.Errorf("create plugin package download: %w", err)
	}
	path := file.Name()
	defer os.Remove(path)
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(reader, m.maxBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		return InstallPreview{}, fmt.Errorf("download plugin package: %w", copyErr)
	}
	if closeErr != nil {
		return InstallPreview{}, fmt.Errorf("close plugin package download: %w", closeErr)
	}
	if size > m.maxBytes {
		return InstallPreview{}, fmt.Errorf("plugin package exceeds download limit: %d", size)
	}
	actualHash := hex.EncodeToString(hash.Sum(nil))
	if actualHash != expectedHash {
		return InstallPreview{}, fmt.Errorf("plugin package SHA-256 mismatch: expected=%s actual=%s", expectedHash, actualHash)
	}
	source, err := os.Open(path)
	if err != nil {
		return InstallPreview{}, fmt.Errorf("open downloaded plugin package: %w", err)
	}
	defer source.Close()
	provenance := request.Source
	if strings.TrimSpace(provenance.URI) == "" {
		provenance.URI = packageURI
	}
	provenance.URI = sanitizePluginSourceURL(provenance.URI)
	provenance.ResolvedURL = sanitizePluginSourceURL(resolvedURL)
	preview, err := m.runtime.PreviewPackage(ctx, PackagePreviewRequest{
		SpaceID:            request.SpaceID,
		ScopeType:          "global",
		FileName:           fileName,
		AllowUnsignedLocal: m.profile != runtimeprofile.ProfileCloudCore,
		Source:             provenance,
	}, source)
	if err != nil {
		return InstallPreview{}, fmt.Errorf("preview plugin package: %w", err)
	}
	if preview.ExtensionID != extensionID {
		return InstallPreview{}, fmt.Errorf("plugin package identity mismatch: expected=%s actual=%s", extensionID, preview.ExtensionID)
	}
	if preview.Version != version {
		return InstallPreview{}, fmt.Errorf("plugin package version mismatch: expected=%s actual=%s", version, preview.Version)
	}
	return preview, nil
}

func sanitizePluginSourceURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.User = nil
	return parsed.String()
}

func (m *HostPluginPackageManager) Install(ctx context.Context, request InstallHostPluginPackageRequest) (KernelInstallResult, error) {
	if m == nil || m.runtime == nil {
		return KernelInstallResult{}, errors.New("plugin package manager unavailable")
	}
	sessionID := strings.TrimSpace(request.PreviewSessionID)
	if sessionID == "" {
		return KernelInstallResult{}, errors.New("previewSessionId is required")
	}
	container := m.runtime.Container()
	if container == nil || container.PackageRepository == nil {
		return KernelInstallResult{}, errors.New("plugin package repository unavailable")
	}
	session, err := container.PackageRepository.GetPreview(ctx, sessionID, request.SpaceID, "global", "")
	if err != nil {
		return KernelInstallResult{}, fmt.Errorf("load plugin package preview: %w", err)
	}
	if request.ExtensionID != "" && session.ExtensionID != request.ExtensionID {
		return KernelInstallResult{}, fmt.Errorf("plugin package preview identity mismatch: expected=%s actual=%s", request.ExtensionID, session.ExtensionID)
	}
	if request.Version != "" && session.Version != request.Version {
		return KernelInstallResult{}, fmt.Errorf("plugin package preview version mismatch: expected=%s actual=%s", request.Version, session.Version)
	}
	var preview InstallPreview
	if err := json.Unmarshal([]byte(session.PreviewResultJSON), &preview); err != nil {
		return KernelInstallResult{}, fmt.Errorf("decode plugin package preview: %w", err)
	}
	if !preview.Installable {
		messages := make([]string, 0, len(preview.Issues))
		for _, issue := range preview.Issues {
			messages = append(messages, issue.Message)
		}
		return KernelInstallResult{}, fmt.Errorf("plugin package preview is not installable: %s", strings.Join(messages, "; "))
	}
	confirmation, err := m.runtime.ConfirmPackagePreview(ctx, PackagePreviewConfirmationRequest{
		SessionID: sessionID, SpaceID: request.SpaceID, ScopeType: "global", Confirmations: request.Confirmations,
	})
	if err != nil {
		return KernelInstallResult{}, fmt.Errorf("confirm plugin package preview: %w", err)
	}
	_, installedErr := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(session.ExtensionID))
	if installedErr != nil && !errors.Is(installedErr, domain.ErrInvalidExtensionID) {
		return KernelInstallResult{}, fmt.Errorf("read installed plugin state: %w", installedErr)
	}
	idempotencyKey := strings.TrimSpace(request.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = "host-plugin-install:" + request.SpaceID + ":" + sessionID
	}
	installRequest := PackageInstallRequest{
		SessionID: sessionID, SpaceID: request.SpaceID, ScopeType: "global",
		Confirmations: request.Confirmations, ConfirmationToken: confirmation.ConfirmationToken,
		ExpectedExtensionID: request.ExtensionID, IdempotencyKey: idempotencyKey,
	}
	var result KernelInstallResult
	if installedErr == nil {
		result, err = m.runtime.ExecutePackageUpdate(ctx, installRequest)
	} else {
		result, err = m.runtime.ExecutePackageInstall(ctx, installRequest)
	}
	if err != nil {
		return KernelInstallResult{}, fmt.Errorf("install plugin package: %w", err)
	}
	if installedErr != nil {
		if err := m.runtime.Enable(ctx, session.ExtensionID); err != nil {
			return KernelInstallResult{}, fmt.Errorf("enable plugin package: %w", err)
		}
	}
	return result, nil
}

func (m *HostPluginPackageManager) openPackage(ctx context.Context, packageURI, requestedName string) (io.ReadCloser, string, string, error) {
	parsed, err := url.Parse(packageURI)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http" && parsed.Scheme != "file") {
		return nil, "", "", errors.New("invalid plugin package URI")
	}
	fileName := strings.TrimSpace(requestedName)
	if fileName == "" {
		fileName = filepath.Base(parsed.Path)
	}
	if fileName == "" || fileName == "." || fileName == string(filepath.Separator) {
		fileName = "plugin.amitiax"
	}
	if parsed.Scheme == "file" {
		if m.profile == runtimeprofile.ProfileCloudCore {
			return nil, "", "", errors.New("统筹中心不能从本地文件安装插件包")
		}
		file, err := os.Open(filepath.FromSlash(parsed.Path))
		if err != nil {
			return nil, "", "", fmt.Errorf("open local plugin package: %w", err)
		}
		return file, fileName, parsed.String(), nil
	}
	if m.profile == runtimeprofile.ProfileCloudCore {
		if parsed.Scheme != "https" {
			return nil, "", "", errors.New("统筹中心仅允许通过 HTTPS 下载插件包")
		}
		host := parsed.Hostname()
		if ip := net.ParseIP(host); ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()) {
			return nil, "", "", errors.New("plugin package URI cannot target a private network")
		}
		addresses, resolveErr := net.DefaultResolver.LookupIPAddr(ctx, host)
		if resolveErr != nil {
			return nil, "", "", fmt.Errorf("resolve plugin package host: %w", resolveErr)
		}
		for _, address := range addresses {
			if address.IP.IsPrivate() || address.IP.IsLoopback() || address.IP.IsLinkLocalUnicast() || address.IP.IsUnspecified() {
				return nil, "", "", errors.New("plugin package host resolves to a private network")
			}
		}
	}
	client := m.client
	if m.profile == runtimeprofile.ProfileCloudCore {
		guardedClient, guardErr := privateNetworkGuardedHTTPClient(client)
		if guardErr != nil {
			return nil, "", "", fmt.Errorf("enforce plugin package network policy: %w", guardErr)
		}
		client = guardedClient
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, packageURI, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("create plugin package request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, "", "", fmt.Errorf("download plugin package: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		response.Body.Close()
		if readErr != nil {
			return nil, "", "", fmt.Errorf("plugin package download returned HTTP %d and response read failed: %w", response.StatusCode, readErr)
		}
		return nil, "", "", fmt.Errorf("plugin package download returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	resolvedURL := packageURI
	if response.Request != nil && response.Request.URL != nil {
		resolvedURL = response.Request.URL.String()
	}
	return response.Body, fileName, resolvedURL, nil
}
