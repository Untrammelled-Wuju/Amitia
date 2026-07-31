package kernel

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/manifest_v2"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
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
	SessionID                 string                                  `json:"sessionId"`
	ArtifactID                string                                  `json:"artifactId"`
	ExtensionID               string                                  `json:"extensionId"`
	Name                      string                                  `json:"name"`
	Version                   string                                  `json:"version"`
	Publisher                 string                                  `json:"publisher"`
	Installable               bool                                    `json:"installable"`
	Category                  PreviewCategory                         `json:"category"`
	ArchiveHash               string                                  `json:"archiveHash"`
	ManifestHash              string                                  `json:"manifestHash"`
	ArtifactHash              string                                  `json:"artifactHash"`
	ContentTreeHash           string                                  `json:"contentTreeHash"`
	SignatureStatus           string                                  `json:"signatureStatus"`
	TrustDecision             string                                  `json:"trustDecision"`
	DevOnly                   bool                                    `json:"devOnly"`
	DeveloperSessionID        string                                  `json:"developerSessionId,omitempty"`
	SignerKeyID               string                                  `json:"signerKeyId,omitempty"`
	RequiredConfirmations     []string                                `json:"requiredConfirmations"`
	RiskFlags                 []string                                `json:"riskFlags"`
	ExpiresAt                 time.Time                               `json:"expiresAt"`
	SecurityPassed            bool                                    `json:"securityPassed"`
	SecurityReport            *package_security.PackageSecurityReport `json:"securityReport,omitempty"`
	Manifest                  manifest_v2.Manifest                    `json:"manifest"`
	ValidationReport          manifest_v2.ValidationReport            `json:"validationReport"`
	Issues                    []PreviewIssue                          `json:"issues"`
	Modules                   []PreviewModule                         `json:"modules"`
	MissingDependencies       []PreviewDependency                     `json:"missingDependencies"`
	RequiredPermissions       []PreviewPermission                     `json:"requiredPermissions"`
	RequiredScopes            []PreviewScope                          `json:"requiredScopes"`
	MigrationPreview          *migration.ReversiblePreflight          `json:"migrationPreview,omitempty"`
	MigrationPlanHash         string                                  `json:"migrationPlanHash,omitempty"`
	MigrationSnapshotRequired bool                                    `json:"migrationSnapshotRequired"`
	MigrationManualRequired   bool                                    `json:"migrationManualRequired"`
	MigrationIrreversible     bool                                    `json:"migrationIrreversible"`
}

type PackagePreviewRequest struct {
	UserID             string
	ScopeType          string
	ScopeID            string
	FileName           string
	AllowUnsignedDev   bool
	DeveloperSessionID string
}

type PackageInstallRequest struct {
	SessionID           string          `json:"sessionId"`
	UserID              string          `json:"-"`
	ScopeType           string          `json:"scopeType"`
	ScopeID             string          `json:"scopeId"`
	Confirmations       map[string]bool `json:"confirmations"`
	ConfirmationToken   string          `json:"confirmationToken"`
	ExpectedExtensionID string          `json:"expectedExtensionId,omitempty"`
	IdempotencyKey      string          `json:"idempotencyKey,omitempty"`
}

type PackagePreviewConfirmationRequest struct {
	SessionID     string          `json:"sessionId"`
	UserID        string          `json:"-"`
	ScopeType     string          `json:"scopeType"`
	ScopeID       string          `json:"scopeId"`
	Confirmations map[string]bool `json:"confirmations"`
}

type PackagePreviewConfirmation struct {
	ConfirmationToken string    `json:"confirmationToken"`
	ExpiresAt         time.Time `json:"expiresAt"`
}

type ArtifactPolicy string

const (
	ArtifactPolicyRetainArtifact  ArtifactPolicy = "retainArtifact"
	ArtifactPolicyDeleteArtifact  ArtifactPolicy = "deleteArtifact"
	ArtifactPolicyRetainForRollback ArtifactPolicy = "retainForRollback"
	ArtifactPolicyRetainForExport ArtifactPolicy = "retainForExport"
)

type RollbackSnapshotRequirement struct {
	Required                 bool   `json:"required"`
	Reason                   string `json:"reason,omitempty"`
	ConfigChanged            bool   `json:"configChanged"`
	ResourcesChanged         bool   `json:"resourcesChanged"`
	UserDataChanged          bool   `json:"userDataChanged"`
	MigrationRequired        bool   `json:"migrationRequired"`
	MigrationStateUnverified bool   `json:"migrationStateUnverified,omitempty"`
	NoDataChange             bool   `json:"noDataChange"`
	RequirementHash          string `json:"requirementHash,omitempty"`
	PreviewHash              string `json:"previewHash,omitempty"`
}

type RemoveArtifactStepResult struct {
	ArtifactID     string          `json:"artifactId"`
	ArtifactPolicy ArtifactPolicy  `json:"artifactPolicy"`
	Deleted        bool            `json:"deleted"`
	RemainingRefs  int             `json:"remainingRefs"`
	DeletedAt      time.Time       `json:"deletedAt"`
}

type packageConfirmationClaims struct {
	SessionID               string          `json:"sessionId"`
	ArtifactID              string          `json:"artifactId"`
	ArchiveHash             string          `json:"archiveHash"`
	ManifestHash            string          `json:"manifestHash"`
	ContentTreeHash         string          `json:"contentTreeHash"`
	UserID                  string          `json:"userId"`
	ScopeType               string          `json:"scopeType"`
	ScopeID                 string          `json:"scopeId"`
	PolicyVersion           string          `json:"policyVersion"`
	SecurityPolicyHash      string          `json:"securityPolicyHash,omitempty"`
	KeyID                   string          `json:"kid,omitempty"`
	DeveloperSessionID      string          `json:"developerSessionId,omitempty"`
	MigrationPlanHash       string          `json:"migrationPlanHash,omitempty"`
	ArtifactPolicy          ArtifactPolicy  `json:"artifactPolicy,omitempty"`
	PreviewHash             string          `json:"previewHash,omitempty"`
	CurrentVersionID        string          `json:"currentVersionId,omitempty"`
	CurrentGenerationID     string          `json:"currentGenerationId,omitempty"`
	SnapshotRequirementHash string          `json:"snapshotRequirementHash,omitempty"`
	Confirmations           map[string]bool `json:"confirmations"`
	ExpiresAt               int64           `json:"expiresAt"`
}

var packageConfirmationKey = func() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	return key
}()

var packageConfirmationKeyStore *PackageConfirmationKeyStore

func SetPackageConfirmationKeyStore(store *PackageConfirmationKeyStore) {
	packageConfirmationKeyStore = store
}

func signPackageConfirmation(claims packageConfirmationClaims) (string, error) {
	if packageConfirmationKeyStore != nil && packageConfirmationKeyStore.HasActiveKey() {
		return packageConfirmationKeyStore.signConfirmation(claims)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, packageConfirmationKey)
	_, _ = mac.Write([]byte(encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encoded + "." + signature, nil
}

func verifyPackageConfirmation(token string) (packageConfirmationClaims, error) {
	if packageConfirmationKeyStore != nil && packageConfirmationKeyStore.HasActiveKey() {
		return packageConfirmationKeyStore.verifyConfirmation(token)
	}
	var claims packageConfirmationClaims
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return claims, fmt.Errorf("kernel: confirmation token invalid")
	}
	verifyParts := parts[:2]
	if len(parts) == 3 {
		verifyParts = parts[:2]
	}
	mac := hmac.New(sha256.New, packageConfirmationKey)
	_, _ = mac.Write([]byte(verifyParts[0]))
	provided, err := base64.RawURLEncoding.DecodeString(verifyParts[1])
	if err != nil || !hmac.Equal(mac.Sum(nil), provided) {
		return claims, fmt.Errorf("kernel: confirmation token invalid")
	}
	payload, err := base64.RawURLEncoding.DecodeString(verifyParts[0])
	if err != nil || json.Unmarshal(payload, &claims) != nil {
		return claims, fmt.Errorf("kernel: confirmation token invalid")
	}
	if claims.ExpiresAt <= time.Now().UTC().Unix() {
		return claims, fmt.Errorf("kernel: confirmation token expired")
	}
	return claims, nil
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
