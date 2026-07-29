package extension

import (
	"encoding/json"
	"time"
)

type PackageFormat string
type PackageOperation string
type PackageSignatureStatus string
type PackageConflictStatus string

const (
	PackageFormatAmitiax        PackageFormat          = "amitiax"
	PackageFormatAgentSkillsZIP PackageFormat          = "agentskills-zip"
	PackageFormatAgentSkillsDir PackageFormat          = "agentskills-directory"
	PackageFormatUnknown        PackageFormat          = "unknown"
	PackageOperationInstall     PackageOperation       = "install"
	PackageOperationUpgrade     PackageOperation       = "upgrade"
	PackageOperationRollback    PackageOperation       = "rollback"
	PackageOperationUninstall   PackageOperation       = "uninstall"
	PackageOperationExport      PackageOperation       = "export"
	PackageSignatureUnsigned    PackageSignatureStatus = "unsigned"
	PackageSignatureUntrusted   PackageSignatureStatus = "valid-untrusted"
	PackageSignatureTrusted     PackageSignatureStatus = "valid-trusted"
	PackageSignatureInvalid     PackageSignatureStatus = "invalid"
	PackageConflictNew          PackageConflictStatus  = "new"
	PackageConflictSame         PackageConflictStatus  = "same-version-same-content"
	PackageConflictDifferent    PackageConflictStatus  = "same-version-different-content"
	PackageConflictUpgrade      PackageConflictStatus  = "upgrade"
	PackageConflictDowngrade    PackageConflictStatus  = "downgrade"
	PackageConflictID           PackageConflictStatus  = "id-conflict"
	PackageConflictName         PackageConflictStatus  = "name-conflict"
)

const (
	ErrPackageFormatUnsupported            = "PACKAGE_FORMAT_UNSUPPORTED"
	ErrPackageInvalidArchive               = "PACKAGE_INVALID_ARCHIVE"
	ErrPackagePathTraversal                = "PACKAGE_PATH_TRAVERSAL"
	ErrPackageArchiveLimit                 = "PACKAGE_ARCHIVE_LIMIT_EXCEEDED"
	ErrPackageManifestMissing              = "PACKAGE_MANIFEST_MISSING"
	ErrPackageManifestInvalid              = "PACKAGE_MANIFEST_INVALID"
	ErrPackageEntryUnsupported             = "PACKAGE_ENTRY_UNSUPPORTED"
	ErrPackageChecksumMissing              = "PACKAGE_CHECKSUM_MISSING"
	ErrPackageChecksumInvalid              = "PACKAGE_CHECKSUM_INVALID"
	ErrPackageChecksumMismatch             = "PACKAGE_CHECKSUM_MISMATCH"
	ErrPackageUnlistedFile                 = "PACKAGE_UNLISTED_FILE"
	ErrPackageMissingFile                  = "PACKAGE_MISSING_FILE"
	ErrPackageSignatureInvalid             = "PACKAGE_SIGNATURE_INVALID"
	ErrPackageEngineIncompatible           = "PACKAGE_ENGINE_INCOMPATIBLE"
	ErrPackageCapabilityMismatch           = "PACKAGE_CAPABILITY_MISMATCH"
	ErrPackageHighRiskConfirmationRequired = "PACKAGE_HIGH_RISK_CONFIRMATION_REQUIRED"
	ErrPackageSecretDetected               = "PACKAGE_SECRET_DETECTED"
	ErrPackageIDConflict                   = "PACKAGE_ID_CONFLICT"
	ErrPackageNameConflict                 = "PACKAGE_NAME_CONFLICT"
	ErrPackageVersionConflict              = "PACKAGE_VERSION_CONFLICT"
	ErrPackageSameVersionDifferentContent  = "PACKAGE_SAME_VERSION_DIFFERENT_CONTENT"
	ErrPackageDependencyMissing            = "PACKAGE_DEPENDENCY_MISSING"
	ErrPackageDependencyInUse              = "PACKAGE_DEPENDENCY_IN_USE"
	ErrPackageTestRequired                 = "PACKAGE_TEST_REQUIRED"
	ErrPackageTestFailed                   = "PACKAGE_TEST_FAILED"
	ErrPackageInstallFailed                = "PACKAGE_INSTALL_FAILED"
	ErrPackageUpgradeFailed                = "PACKAGE_UPGRADE_FAILED"
	ErrPackageRollbackFailed               = "PACKAGE_ROLLBACK_FAILED"
	ErrPackageUninstallFailed              = "PACKAGE_UNINSTALL_FAILED"
	ErrPackageConfigMigrationRequired      = "PACKAGE_CONFIG_MIGRATION_REQUIRED"
	ErrPackageConfigMigrationFailed        = "PACKAGE_CONFIG_MIGRATION_FAILED"
	ErrPackageImportSessionExpired         = "PACKAGE_IMPORT_SESSION_EXPIRED"
	ErrPackageImportSessionConsumed        = "PACKAGE_IMPORT_SESSION_CONSUMED"
	ErrPackageOperationInProgress          = "PACKAGE_OPERATION_IN_PROGRESS"
	ErrPackageArtifactInvalid              = "PACKAGE_ARTIFACT_INVALID"
	ErrPackageExportNotAllowed             = "PACKAGE_EXPORT_NOT_ALLOWED"
)

type PackageLimits struct {
	MaxFiles            int
	MaxDepth            int
	MaxExpandedBytes    int64
	MaxFileBytes        int64
	MaxManifestBytes    int64
	MaxSchemaBytes      int64
	MaxWorkflowBytes    int64
	MaxSkillBytes       int64
	MaxCompressionRatio uint64
}

func DefaultPackageLimits() PackageLimits {
	return PackageLimits{MaxFiles: 1000, MaxDepth: 16, MaxExpandedBytes: 100 << 20, MaxFileBytes: 25 << 20, MaxManifestBytes: 256 << 10, MaxSchemaBytes: 2 << 20, MaxWorkflowBytes: 5 << 20, MaxSkillBytes: 512 << 10, MaxCompressionRatio: 100}
}

type PackageFileView struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Kind string `json:"kind"`
}

type PackageRisk struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type PackageSignatureView struct {
	Status      PackageSignatureStatus `json:"status"`
	Fingerprint string                 `json:"fingerprint,omitempty"`
	Algorithm   string                 `json:"algorithm,omitempty"`
	DisplayName string                 `json:"displayName,omitempty"`
}

type PackageChecksumView struct {
	Valid       bool   `json:"valid"`
	PackageHash string `json:"packageHash"`
}

type PackageDependencyView struct {
	ID                string `json:"id"`
	VersionConstraint string `json:"versionConstraint,omitempty"`
	Required          bool   `json:"required"`
	Installed         bool   `json:"installed"`
	Version           string `json:"version,omitempty"`
}

type PackageUninstallPreview struct {
	ExtensionID         string                  `json:"extensionId"`
	CurrentVersion      string                  `json:"currentVersion"`
	Enabled             bool                    `json:"enabled"`
	Dependents          []PackageDependencyView `json:"dependents"`
	ScheduleCount       int64                   `json:"scheduleCount"`
	Grants              []string                `json:"grants"`
	ConfigPresent       bool                    `json:"configPresent"`
	HistoricalRuns      int64                   `json:"historicalRuns"`
	ArtifactArchived    bool                    `json:"artifactArchived"`
	Cleanup             []string                `json:"cleanup"`
	Preserved           []string                `json:"preserved"`
	ReadSource          string                  `json:"readSource"`
	RuntimeImpacts      []PackageRuntimeImpact  `json:"runtimeImpacts,omitempty"`
	ContributionSummary map[string]int          `json:"contributionSummary,omitempty"`
	EventSubscriptions  []string                `json:"eventSubscriptions,omitempty"`
}

type PackageRuntimeImpact struct {
	InstanceID  string `json:"instanceId"`
	ModuleID    string `json:"moduleId"`
	RuntimeType string `json:"runtimeType"`
	Desired     string `json:"desired"`
	Actual      string `json:"actual"`
	Health      string `json:"health"`
}

type PackageImportPreview struct {
	SessionID               string                   `json:"sessionId"`
	Format                  PackageFormat            `json:"format"`
	SkillType               string                   `json:"skillType"`
	ID                      string                   `json:"id"`
	Name                    string                   `json:"name"`
	Version                 string                   `json:"version"`
	Description             string                   `json:"description"`
	License                 string                   `json:"license"`
	Source                  string                   `json:"source"`
	ScopeType               string                   `json:"scopeType"`
	ScopeID                 string                   `json:"scopeId"`
	PackageHash             string                   `json:"packageHash"`
	Checksum                PackageChecksumView      `json:"checksum"`
	Signature               PackageSignatureView     `json:"signature"`
	Compatible              bool                     `json:"compatible"`
	Compatibility           string                   `json:"compatibility"`
	Capabilities            []string                 `json:"capabilities"`
	HighRisk                []string                 `json:"highRiskCapabilities"`
	CapabilityConfirmations []string                 `json:"capabilityConfirmations"`
	Triggers                []SkillTrigger           `json:"triggers"`
	Dependencies            []PackageDependencyView  `json:"dependencies"`
	AgentSkill              *AgentSkillImportPreview `json:"agentSkill,omitempty"`
	WorkflowSteps           []string                 `json:"workflowSteps,omitempty"`
	Scripts                 int                      `json:"scripts"`
	ScriptsRequired         bool                     `json:"scriptsRequired"`
	References              int                      `json:"references"`
	Assets                  int                      `json:"assets"`
	Files                   []PackageFileView        `json:"files"`
	TotalSize               int64                    `json:"totalSize"`
	FileCount               int                      `json:"fileCount"`
	TestStatus              string                   `json:"testStatus"`
	TestReport              *PackageDryRunReport     `json:"testReport,omitempty"`
	Risks                   []PackageRisk            `json:"risks"`
	Warnings                []string                 `json:"warnings"`
	Errors                  []string                 `json:"errors"`
	Conflict                PackageConflictStatus    `json:"conflict"`
	AvailableActions        []string                 `json:"availableActions"`
	CurrentVersion          string                   `json:"currentVersion,omitempty"`
	RollbackVersion         string                   `json:"rollbackVersion,omitempty"`
	UpgradeDiff             *PackageVersionDiff      `json:"upgradeDiff,omitempty"`
	ExpiresAt               time.Time                `json:"expiresAt"`
}

type PackageDryRunReport struct {
	Status       string                    `json:"status"`
	CaseCount    int                       `json:"caseCount"`
	PassedCount  int                       `json:"passedCount"`
	FailedCount  int                       `json:"failedCount"`
	DurationMS   int64                     `json:"durationMs"`
	Cases        []PackageDryRunCaseReport `json:"cases"`
	Capabilities []string                  `json:"capabilities"`
	SideEffects  []SideEffectRecord        `json:"sideEffects"`
}

type PackageDryRunCaseReport struct {
	ID         string               `json:"id"`
	Name       string               `json:"name"`
	Mode       string               `json:"mode"`
	Status     string               `json:"status"`
	DurationMS int64                `json:"durationMs"`
	Steps      []WorkflowStepResult `json:"steps"`
	Assertions []AssertionResult    `json:"assertions"`
	Output     json.RawMessage      `json:"output,omitempty"`
	Error      *ExtensionError      `json:"error,omitempty"`
}

type PreviewPackageImportRequest struct {
	UserID      string
	ScopeType   string
	ScopeID     string
	FileName    string
	Raw         []byte
	Directory   map[string][]byte
	RootName    string
	OperationID string
}

type InstallPackageRequest struct {
	SessionID              string   `json:"sessionId"`
	UserID                 string   `json:"-"`
	ScopeType              string   `json:"scopeType"`
	ScopeID                string   `json:"scopeId"`
	ConfirmUnsigned        bool     `json:"confirmUnsigned"`
	ConfirmedCapabilities  []string `json:"confirmedCapabilities"`
	ConfirmScripts         bool     `json:"confirmScripts"`
	ConfirmVersionChange   bool     `json:"confirmVersionChange"`
	ConfirmSignerChange    bool     `json:"confirmSignerChange"`
	ConfirmConfigMigration bool     `json:"confirmConfigMigration"`
	ExpectedExtensionID    string   `json:"-"`
}

type ExportPackageRequest struct {
	UserID      string `json:"-"`
	ExtensionID string `json:"-"`
	Version     string `json:"version,omitempty"`
	Format      string `json:"format"`
	ScopeType   string `json:"scopeType"`
	ScopeID     string `json:"scopeId"`
}

type ExportedPackage struct {
	ExportID        string    `json:"exportId"`
	FileName        string    `json:"fileName"`
	MIME            string    `json:"mime"`
	Size            int64     `json:"size"`
	Hash            string    `json:"hash"`
	Version         string    `json:"version"`
	Format          string    `json:"format"`
	TestsIncluded   bool      `json:"testsIncluded"`
	ReadmeIncluded  bool      `json:"readmeIncluded"`
	SBOMIncluded    bool      `json:"sbomIncluded"`
	ScriptsIncluded bool      `json:"scriptsIncluded"`
	SecretScan      string    `json:"secretScan"`
	SignatureStatus string    `json:"signatureStatus"`
	ExpiresAt       time.Time `json:"expiresAt"`
	Content         []byte    `json:"-"`
}

type PackageOperationResult struct {
	OperationID string           `json:"operationId"`
	TraceID     string           `json:"traceId"`
	Operation   PackageOperation `json:"operation"`
	ExtensionID string           `json:"extensionId"`
	Version     string           `json:"version"`
	Enabled     bool             `json:"enabled"`
	Status      string           `json:"status"`
}

type PackageOperationView struct {
	ID                string           `json:"id"`
	Operation         PackageOperation `json:"operation"`
	ExtensionID       string           `json:"extensionId"`
	PreviousVersion   string           `json:"previousVersion,omitempty"`
	TargetVersion     string           `json:"targetVersion,omitempty"`
	Source            string           `json:"source"`
	PackageHash       string           `json:"packageHash"`
	SignatureStatus   string           `json:"signatureStatus"`
	SignerFingerprint string           `json:"signerFingerprint,omitempty"`
	ScopeType         string           `json:"scopeType"`
	ScopeID           string           `json:"scopeId"`
	Status            string           `json:"status"`
	ErrorCode         string           `json:"errorCode,omitempty"`
	TraceID           string           `json:"traceId"`
	CreatedAt         string           `json:"createdAt"`
	CompletedAt       string           `json:"completedAt,omitempty"`
}

type PackageVersionView struct {
	Version             string          `json:"version"`
	Manifest            json.RawMessage `json:"manifest"`
	ArtifactID          string          `json:"artifactId"`
	ArtifactHash        string          `json:"artifactHash"`
	PackageHash         string          `json:"packageHash"`
	Source              string          `json:"source"`
	SignatureStatus     string          `json:"signatureStatus"`
	SignerFingerprint   string          `json:"signerFingerprint,omitempty"`
	CompatibilityStatus string          `json:"compatibilityStatus"`
	Capabilities        []string        `json:"capabilities"`
	InstalledAt         string          `json:"installedAt"`
	InstalledBy         string          `json:"installedBy"`
	Active              bool            `json:"active"`
	ValidationStatus    string          `json:"validationStatus"`
	TestStatus          string          `json:"testStatus"`
	ArtifactStatus      string          `json:"artifactStatus"`
	ActivationStatus    string          `json:"activationStatus"`
	OperationID         string          `json:"operationId,omitempty"`
	FailureCode         string          `json:"failureCode,omitempty"`
	Archived            bool            `json:"archived"`
}

type PackageVersionDiff struct {
	ExtensionID  string                 `json:"extensionId"`
	FromVersion  string                 `json:"fromVersion"`
	ToVersion    string                 `json:"toVersion"`
	Manifest     map[string]interface{} `json:"manifest"`
	Schemas      map[string]interface{} `json:"schemas"`
	Workflow     map[string]interface{} `json:"workflow"`
	Instructions map[string]interface{} `json:"instructions"`
	Capabilities map[string][]string    `json:"capabilities"`
	Signature    map[string]string      `json:"signature"`
	Scripts      map[string][]string    `json:"scripts"`
	Dependencies map[string][]string    `json:"dependencies"`
	Trust        map[string]string      `json:"trust"`
	Risks        []PackageRisk          `json:"risks"`
}

type PackageSignerView struct {
	Fingerprint string `json:"fingerprint"`
	Algorithm   string `json:"algorithm"`
	DisplayName string `json:"displayName"`
	Trusted     bool   `json:"trusted"`
	TrustedAt   string `json:"trustedAt,omitempty"`
	RevokedAt   string `json:"revokedAt,omitempty"`
}
