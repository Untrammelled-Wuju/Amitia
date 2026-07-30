package kernel

import (
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/manifest_v2"
	"github.com/u-ai/backend/internal/extension/kernel/package_security"
)

type PreviewCategory string

const (
	PreviewOK                 PreviewCategory = "ok"
	PreviewNotInstallable     PreviewCategory = "not_installable"
	PreviewPartialUnsupported PreviewCategory = "partial_unsupported"
	PreviewMissingDependency  PreviewCategory = "missing_dependency"
	PreviewNeedsPermission    PreviewCategory = "needs_permission"
	PreviewNeedsScope         PreviewCategory = "needs_scope"
)

type PreviewIssue struct {
	Category PreviewCategory `json:"category"`
	Code     string          `json:"code"`
	Message  string          `json:"message"`
	Path     string          `json:"path,omitempty"`
}

type PreviewModule struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Runtime   string `json:"runtime,omitempty"`
	Supported bool   `json:"supported"`
}

type PreviewDependency struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Version  string `json:"version,omitempty"`
	Optional bool   `json:"optional"`
	Reason   string `json:"reason,omitempty"`
	Missing  bool   `json:"missing"`
}

type PreviewPermission struct {
	Name     string `json:"name"`
	Reason   string `json:"reason,omitempty"`
	Required bool   `json:"required"`
	Scope    string `json:"scope,omitempty"`
}

type PreviewScope struct {
	ContributionID string   `json:"contributionId"`
	Scopes         []string `json:"scopes"`
}

type InstallPreview struct {
	SessionID             string                                  `json:"sessionId"`
	ArtifactID            string                                  `json:"artifactId"`
	ExtensionID           string                                  `json:"extensionId"`
	Name                  string                                  `json:"name"`
	Version               string                                  `json:"version"`
	Publisher             string                                  `json:"publisher"`
	Installable           bool                                    `json:"installable"`
	Category              PreviewCategory                         `json:"category"`
	ArchiveHash           string                                  `json:"archiveHash"`
	ManifestHash          string                                  `json:"manifestHash"`
	ArtifactHash          string                                  `json:"artifactHash"`
	ContentTreeHash       string                                  `json:"contentTreeHash"`
	SignatureStatus       string                                  `json:"signatureStatus"`
	TrustDecision         string                                  `json:"trustDecision"`
	SignerKeyID           string                                  `json:"signerKeyId,omitempty"`
	RequiredConfirmations []string                                `json:"requiredConfirmations"`
	RiskFlags             []string                                `json:"riskFlags"`
	ExpiresAt             time.Time                               `json:"expiresAt"`
	SecurityPassed        bool                                    `json:"securityPassed"`
	SecurityReport        *package_security.PackageSecurityReport `json:"securityReport,omitempty"`
	Manifest              manifest_v2.Manifest                    `json:"manifest"`
	ValidationReport      manifest_v2.ValidationReport            `json:"validationReport"`
	Issues                []PreviewIssue                          `json:"issues"`
	Modules               []PreviewModule                         `json:"modules"`
	MissingDependencies   []PreviewDependency                     `json:"missingDependencies"`
	RequiredPermissions   []PreviewPermission                     `json:"requiredPermissions"`
	RequiredScopes        []PreviewScope                          `json:"requiredScopes"`
}

type PackagePreviewRequest struct {
	UserID    string
	ScopeType string
	ScopeID   string
	FileName  string
}

type PackageInstallRequest struct {
	SessionID           string          `json:"sessionId"`
	UserID              string          `json:"-"`
	ScopeType           string          `json:"scopeType"`
	ScopeID             string          `json:"scopeId"`
	Confirmations       map[string]bool `json:"confirmations"`
	ExpectedExtensionID string          `json:"-"`
}

type KernelInstallResult struct {
	OperationID     string    `json:"operationId"`
	TraceID         string    `json:"traceId"`
	Operation       string    `json:"operation"`
	ExtensionID     string    `json:"extensionId"`
	Version         string    `json:"version"`
	InstallationID  string    `json:"installationId"`
	PackageHash     string    `json:"packageHash"`
	ContentTreeHash string    `json:"contentTreeHash"`
	ArtifactPath    string    `json:"artifactPath"`
	InstallPath     string    `json:"installPath"`
	DefinitionHash  string    `json:"definitionHash"`
	InstalledAt     time.Time `json:"installedAt"`
}
