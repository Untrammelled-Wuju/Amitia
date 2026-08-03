package kernel

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
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
	InstalledPath             string                                  `json:"installedPath,omitempty"`
	InstalledTreeHash         string                                  `json:"installedTreeHash,omitempty"`
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
	ArtifactPolicyRetainArtifact    ArtifactPolicy = "retainArtifact"
	ArtifactPolicyDeleteArtifact    ArtifactPolicy = "deleteArtifact"
	ArtifactPolicyRetainForRollback ArtifactPolicy = "retainForRollback"
	ArtifactPolicyRetainForExport   ArtifactPolicy = "retainForExport"
)

type RollbackSnapshotRequirement struct {
	Required                   bool   `json:"required"`
	Reason                     string `json:"reason,omitempty"`
	ConfigChanged              bool   `json:"configChanged"`
	ResourcesChanged           bool   `json:"resourcesChanged"`
	UserDataChanged            bool   `json:"userDataChanged"`
	MigrationPlanPresent       bool   `json:"migrationPlanPresent"`
	MigrationDefinitionPresent bool   `json:"migrationDefinitionPresent"`
	MigrationOperationPresent  bool   `json:"migrationOperationPresent"`
	MigrationStateUnverified   bool   `json:"migrationStateUnverified,omitempty"`
	ManifestNoDataChange       bool   `json:"manifestNoDataChange,omitempty"`
	NoDataChange               bool   `json:"noDataChange"`
	RequirementHash            string `json:"requirementHash,omitempty"`
	PreviewHash                string `json:"previewHash,omitempty"`
}

type RollbackSnapshotRequirementInput struct {
	Manifest               manifest_v2.Manifest
	ManifestNoDataChange   bool
	ConfigBeforeHash       string
	ConfigAfterHash        string
	ResourceBeforeTreeHash string
	ResourceAfterTreeHash  string
	ResourceSetDiff        ResourceSetDiff
	UserDataBeforeHash     string
	UserDataAfterHash      string
	MigrationPlan          *migration.ReversiblePreflight
	MigrationDefinitions   []migration.MigrationDefinition
	MigrationOperations    []migration.MigrationOperation
}

type ResourceSetDiff struct {
	Added   []string
	Removed []string
	Changed []string
}

type RemoveArtifactStepResult struct {
	ArtifactID         string         `json:"artifactId"`
	ExtensionID        string         `json:"extensionId,omitempty"`
	ArtifactPolicy     ArtifactPolicy `json:"artifactPolicy"`
	Deleted            bool           `json:"deleted"`
	Retained           bool           `json:"retained,omitempty"`
	RetentionState     string         `json:"retentionState,omitempty"`
	RemainingRefs      int            `json:"remainingRefs"`
	DeletedAt          time.Time      `json:"deletedAt"`
	EvidenceHashBefore string         `json:"evidenceHashBefore,omitempty"`
	EvidenceHashAfter  string         `json:"evidenceHashAfter,omitempty"`
	EvidenceHash       string         `json:"evidenceHash,omitempty"`
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
	ExtensionID             string          `json:"extensionId"`
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
	InstalledPath           string          `json:"installedPath,omitempty"`
	InstalledTreeHash       string          `json:"installedTreeHash,omitempty"`
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

type ExecutePackageUninstallRequest struct {
	ExtensionID       string `json:"extensionId"`
	UserID            string `json:"userId"`
	ScopeType         string `json:"scopeType"`
	ScopeID           string `json:"scopeId"`
	ConfirmationToken string `json:"confirmationToken"`
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

const PackageConfirmationClaimsSchemaVersion = 1

type PackageOperationType string

const (
	PackageOperationTypeInstall   PackageOperationType = "install"
	PackageOperationTypeUpdate    PackageOperationType = "update"
	PackageOperationTypeRollback  PackageOperationType = "rollback"
	PackageOperationTypeUninstall PackageOperationType = "uninstall"
)

type PackageConfirmationClaims struct {
	SchemaVersion           int             `json:"schemaVersion"`
	OperationType           string          `json:"operationType"`
	ExtensionID             string          `json:"extensionId"`
	ArtifactID              string          `json:"artifactId"`
	ArtifactPolicy          ArtifactPolicy  `json:"artifactPolicy,omitempty"`
	PolicyReason            string          `json:"policyReason,omitempty"`
	PreviewSessionID        string          `json:"previewSessionId,omitempty"`
	PreviewHash             string          `json:"previewHash,omitempty"`
	CurrentVersionID        string          `json:"currentVersionId,omitempty"`
	CurrentGenerationID     string          `json:"currentGenerationId,omitempty"`
	SecurityPolicyHash      string          `json:"securityPolicyHash,omitempty"`
	SnapshotRequirementHash string          `json:"snapshotRequirementHash,omitempty"`
	PolicyVersion           string          `json:"policyVersion"`
	UserID                  string          `json:"userId"`
	ScopeType               string          `json:"scopeType"`
	ScopeID                 string          `json:"scopeId"`
	ConfirmedItems          []string        `json:"confirmedItems"`
	Confirmations           map[string]bool `json:"confirmations"`
	IssuedAt                int64           `json:"issuedAt"`
	ExpiresAt               int64           `json:"expiresAt"`
	Nonce                   string          `json:"nonce"`
	ArchiveHash             string          `json:"archiveHash,omitempty"`
	ManifestHash            string          `json:"manifestHash,omitempty"`
	ContentTreeHash         string          `json:"contentTreeHash,omitempty"`
	KeyID                   string          `json:"kid,omitempty"`
	DeveloperSessionID      string          `json:"developerSessionId,omitempty"`
	MigrationPlanHash       string          `json:"migrationPlanHash,omitempty"`
	InstalledPath           string          `json:"installedPath,omitempty"`
	InstalledTreeHash       string          `json:"installedTreeHash,omitempty"`
	SourceVersionID         string          `json:"sourceVersionId,omitempty"`
	SourceGenerationID      string          `json:"sourceGenerationId,omitempty"`
	TargetVersionID         string          `json:"targetVersionId,omitempty"`
	TargetGenerationID      string          `json:"targetGenerationId,omitempty"`
	RollbackPointID         string          `json:"rollbackPointId,omitempty"`
	CurrentVersion          string          `json:"currentVersion,omitempty"`
	TargetVersion           string          `json:"targetVersion,omitempty"`
	SourceGeneration        int64           `json:"sourceGeneration,omitempty"`
	TargetGeneration        int64           `json:"targetGeneration,omitempty"`
}

type PackageRollbackConfirmationClaims struct {
	SchemaVersion           int             `json:"schemaVersion"`
	OperationType           string          `json:"operationType"`
	PolicyVersion           string          `json:"policyVersion"`
	ExtensionID             string          `json:"extensionId"`
	ArtifactID              string          `json:"artifactId"`
	SourceVersionID         string          `json:"sourceVersionId,omitempty"`
	SourceGenerationID      string          `json:"sourceGenerationId,omitempty"`
	TargetVersionID         string          `json:"targetVersionId,omitempty"`
	TargetGenerationID      string          `json:"targetGenerationId,omitempty"`
	RollbackPointID         string          `json:"rollbackPointId"`
	PreviewSessionID        string          `json:"previewSessionId"`
	PreviewHash             string          `json:"previewHash"`
	SecurityPolicyHash      string          `json:"securityPolicyHash"`
	SnapshotRequirementHash string          `json:"snapshotRequirementHash"`
	UserID                  string          `json:"userId"`
	ScopeType               string          `json:"scopeType"`
	ScopeID                 string          `json:"scopeId"`
	ConfirmedItems          []string        `json:"confirmedItems"`
	Confirmations           map[string]bool `json:"confirmations"`
	IssuedAt                int64           `json:"issuedAt"`
	ExpiresAt               int64           `json:"expiresAt"`
	Nonce                   string          `json:"nonce"`
}

const PackageRollbackConfirmationClaimsSchemaVersion = 1

func signPackageRollbackConfirmation(claims PackageRollbackConfirmationClaims) (string, error) {
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

func verifyPackageRollbackConfirmation(token string) (PackageRollbackConfirmationClaims, error) {
	var claims PackageRollbackConfirmationClaims
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return claims, fmt.Errorf("kernel: rollback confirmation token invalid")
	}
	mac := hmac.New(sha256.New, packageConfirmationKey)
	_, _ = mac.Write([]byte(parts[0]))
	provided, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(mac.Sum(nil), provided) {
		return claims, fmt.Errorf("kernel: rollback confirmation token invalid")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || json.Unmarshal(payload, &claims) != nil {
		return claims, fmt.Errorf("kernel: rollback confirmation token invalid")
	}
	if claims.ExpiresAt <= time.Now().UTC().Unix() {
		return claims, fmt.Errorf("kernel: rollback confirmation token expired")
	}
	if claims.OperationType != "rollback" {
		return claims, fmt.Errorf("kernel: rollback confirmation token operation type mismatch")
	}
	return claims, nil
}

func (c PackageConfirmationClaims) ExpiresAtTime() time.Time {
	return time.Unix(c.ExpiresAt, 0).UTC()
}

func (c PackageConfirmationClaims) IssuedAtTime() time.Time {
	return time.Unix(c.IssuedAt, 0).UTC()
}

func confirmedItemsFromMap(m map[string]bool) []string {
	items := make([]string, 0, len(m))
	for k, v := range m {
		if v {
			items = append(items, k)
		}
	}
	return normalizeConfirmedItems(items)
}

func confirmedItemsToMap(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, item := range items {
		m[item] = true
	}
	return m
}

func normalizeConfirmedItems(items []string) []string {
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func validateConfirmedItemsConsistency(items []string, m map[string]bool) bool {
	if len(items) != len(m) {
		return false
	}
	for _, item := range items {
		if !m[item] {
			return false
		}
	}
	return true
}

func validateRequiredConfirmations(confirmed []string, required []string) error {
	if len(required) == 0 {
		return nil
	}
	if len(confirmed) == 0 {
		return NewPackageError(PackageErrCodeConfirmationRequired, 403, ErrPackageConfirmationRequired)
	}
	confirmedSet := make(map[string]bool, len(confirmed))
	for _, item := range confirmed {
		confirmedSet[item] = true
	}
	missing := make([]string, 0)
	for _, req := range required {
		if !confirmedSet[req] {
			missing = append(missing, req)
		}
	}
	if len(missing) > 0 {
		return NewPackageError(PackageErrCodeConfirmationItemsMissing, 403,
			fmt.Errorf("%w: %s", ErrPackageConfirmationItemsMissing, strings.Join(missing, ", ")))
	}
	return nil
}

func parseAndValidateOperationConfirmationClaims(operation PackageOperationRecord, expectedPolicyVersion string) (PackageConfirmationClaims, error) {
	var claims PackageConfirmationClaims
	if operation.ConfirmationClaimsJSON == "" || operation.ConfirmationClaimsJSON == "{}" {
		return claims, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, ErrPackageConfirmationClaimsInvalid)
	}
	if err := json.Unmarshal([]byte(operation.ConfirmationClaimsJSON), &claims); err != nil {
		return claims, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, fmt.Errorf("%w: %v", ErrPackageConfirmationClaimsInvalid, err))
	}
	if claims.SchemaVersion != PackageConfirmationClaimsSchemaVersion {
		return claims, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, fmt.Errorf("%w: unsupported claims schemaVersion %d, expected %d", ErrPackageConfirmationClaimsInvalid, claims.SchemaVersion, PackageConfirmationClaimsSchemaVersion))
	}
	if claims.OperationType != operation.OperationType {
		return claims, NewPackageError(PackageErrCodeConfirmationOperationMismatch, 403, ErrPackageConfirmationOperationMismatch)
	}
	if claims.OperationType != string(PackageOperationTypeInstall) && claims.OperationType != string(PackageOperationTypeUpdate) && claims.OperationType != string(PackageOperationTypeRollback) && claims.OperationType != string(PackageOperationTypeUninstall) {
		return claims, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, fmt.Errorf("%w: invalid operationType %s", ErrPackageConfirmationClaimsInvalid, claims.OperationType))
	}
	if claims.ExtensionID != operation.ExtensionID {
		return claims, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, fmt.Errorf("%w: extensionId mismatch", ErrPackageConfirmationClaimsInvalid))
	}
	if claims.ArtifactID != operation.ArtifactID {
		return claims, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, fmt.Errorf("%w: artifactId mismatch", ErrPackageConfirmationClaimsInvalid))
	}
	if claims.UserID != operation.UserID {
		return claims, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, fmt.Errorf("%w: userId mismatch", ErrPackageConfirmationClaimsInvalid))
	}
	if claims.ScopeType != operation.ScopeType {
		return claims, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, fmt.Errorf("%w: scopeType mismatch", ErrPackageConfirmationClaimsInvalid))
	}
	if claims.ScopeID != operation.ScopeID {
		return claims, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, fmt.Errorf("%w: scopeId mismatch", ErrPackageConfirmationClaimsInvalid))
	}
	if claims.PolicyVersion != expectedPolicyVersion {
		return claims, NewPackageError(PackageErrCodeConfirmationPolicyVersionStale, 403, ErrPackageConfirmationPolicyVersionStale)
	}
	if claims.PreviewSessionID != operation.PreviewSessionID {
		return claims, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, fmt.Errorf("%w: previewSessionId mismatch", ErrPackageConfirmationClaimsInvalid))
	}
	if claims.SnapshotRequirementHash != operation.SnapshotRequirementHash {
		return claims, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, ErrPackageSnapshotRequirementHashMismatch)
	}
	if claims.PreviewHash == "" {
		return claims, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, fmt.Errorf("%w: previewHash required", ErrPackageConfirmationClaimsInvalid))
	}
	if claims.SecurityPolicyHash == "" {
		return claims, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, fmt.Errorf("%w: securityPolicyHash required", ErrPackageConfirmationClaimsInvalid))
	}
	if claims.IssuedAt == 0 {
		return claims, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, fmt.Errorf("%w: issuedAt required", ErrPackageConfirmationClaimsInvalid))
	}
	if claims.ExpiresAt == 0 {
		return claims, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, fmt.Errorf("%w: expiresAt required", ErrPackageConfirmationClaimsInvalid))
	}
	if claims.Nonce == "" {
		return claims, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, fmt.Errorf("%w: nonce required", ErrPackageConfirmationClaimsInvalid))
	}
	if !validateConfirmedItemsConsistency(claims.ConfirmedItems, claims.Confirmations) {
		return claims, NewPackageError(PackageErrCodeConfirmationItemsMismatch, 403, ErrPackageConfirmationItemsMismatch)
	}
	return claims, nil
}

type PackageUninstallPreviewIdentity struct {
	ExtensionID             string
	ArtifactID              string
	ArtifactPolicy          ArtifactPolicy
	PolicyReason            string
	CurrentVersionID        string
	CurrentGenerationID     string
	CurrentVersion          string
	InstalledPath           string
	InstalledTreeHash       string
	DependentsHash          string
	SecurityPolicyHash      string
	SnapshotRequirementHash string
	UserID                  string
	ScopeType               string
	ScopeID                 string
	PolicyVersion           string
	PreviewHash             string
}

type ArtifactPolicyStepResult struct {
	ArtifactID      string         `json:"artifactId"`
	ExtensionID     string         `json:"extensionId"`
	ArtifactPolicy  ArtifactPolicy `json:"artifactPolicy"`
	Deleted         bool           `json:"deleted"`
	Retained        bool           `json:"retained"`
	RetentionState  string         `json:"retentionState"`
	DeletedAt       *time.Time     `json:"deletedAt,omitempty"`
	RemainingRefs   int64          `json:"remainingRefs"`
	ReferenceHash   string         `json:"referenceHash,omitempty"`
	BeforeStateHash string         `json:"beforeStateHash,omitempty"`
	AfterStateHash  string         `json:"afterStateHash,omitempty"`
	EvidenceHash    string         `json:"evidenceHash,omitempty"`
}

type PackageFinalGateResultV2 struct {
	OperationID                 string                    `json:"operationId"`
	OperationType               string                    `json:"operationType"`
	ExtensionID                 string                    `json:"extensionId"`
	ClaimsVerified              bool                      `json:"claimsVerified"`
	PolicyVersionVerified       bool                      `json:"policyVersionVerified"`
	ConfirmedItemsVerified      bool                      `json:"confirmedItemsVerified"`
	PreviewIdentityVerified     bool                      `json:"previewIdentityVerified"`
	ArtifactPolicyVerified      bool                      `json:"artifactPolicyVerified"`
	SnapshotRequirementVerified bool                      `json:"snapshotRequirementVerified"`
	SnapshotVerified            bool                      `json:"snapshotVerified"`
	SnapshotExemptionVerified   bool                      `json:"snapshotExemptionVerified"`
	VersionStateVerified        bool                      `json:"versionStateVerified"`
	GenerationStateVerified     bool                      `json:"generationStateVerified"`
	InstallationStateVerified   bool                      `json:"installationStateVerified"`
	EvidenceHash                string                    `json:"evidenceHash,omitempty"`
	Checks                      []PackageFinalGateCheck   `json:"checks"`
	Findings                    []PackageFinalGateFinding `json:"findings,omitempty"`
	Passed                      bool                      `json:"passed"`
	VerifiedAt                  string                    `json:"verifiedAt"`
}

type PackageFinalGateCheckV2 struct {
	Name              string `json:"name"`
	Passed            bool   `json:"passed"`
	ErrorCode         string `json:"errorCode,omitempty"`
	ExpectedHash      string `json:"expectedHash,omitempty"`
	ActualHash        string `json:"actualHash,omitempty"`
	EvidenceReference string `json:"evidenceReference,omitempty"`
}
