package extension

import "encoding/json"

func kernelSignatureStatus(status string) PackageSignatureStatus {
	switch status {
	case "valid":
		return PackageSignatureTrusted
	case "unsigned":
		return PackageSignatureUnsigned
	case "unknown_key", "legacy_signature":
		return PackageSignatureUntrusted
	default:
		return PackageSignatureInvalid
	}
}

type parsedExtensionPackage struct {
	Format       PackageFormat
	Source       string
	Files        map[string][]byte
	Raw          []byte
	PackageHash  string
	Manifest     Manifest
	ManifestRaw  json.RawMessage
	Signature    PackageSignatureView
	AgentSkill   *parsedAgentSkill
	Workflow     *WorkflowDefinition
	WorkflowRaw  json.RawMessage
	Schemas      map[string]json.RawMessage
	Tests        json.RawMessage
	Warnings     []string
	FileViews    []PackageFileView
	SignedDigest string
}

type packageVersionRecord struct {
	ID                  string `gorm:"column:id;primaryKey"`
	ExtensionID         string `gorm:"column:extension_id"`
	Version             string `gorm:"column:version"`
	ManifestJSON        string `gorm:"column:manifest_json"`
	Checksum            string `gorm:"column:checksum"`
	ArtifactID          string `gorm:"column:artifact_id"`
	ArtifactHash        string `gorm:"column:artifact_hash"`
	PackageHash         string `gorm:"column:package_hash"`
	Source              string `gorm:"column:source"`
	SignatureStatus     string `gorm:"column:signature_status"`
	SignerFingerprint   string `gorm:"column:signer_fingerprint"`
	CompatibilityStatus string `gorm:"column:compatibility_status"`
	CapabilitiesJSON    string `gorm:"column:capabilities_json"`
	InstalledBy         string `gorm:"column:installed_by"`
	ValidationStatus    string `gorm:"column:validation_status"`
	TestStatus          string `gorm:"column:test_status"`
	ArtifactStatus      string `gorm:"column:artifact_status"`
	ActivationStatus    string `gorm:"column:activation_status"`
	OperationID         string `gorm:"column:operation_id"`
	FailureCode         string `gorm:"column:failure_code"`
	ArchivedAt          string `gorm:"column:archived_at"`
	PackageBlob         []byte `gorm:"column:package_blob"`
	CreatedAt           string `gorm:"column:created_at"`
}

type packageArtifactRecord struct {
	ID                   string `gorm:"column:id;primaryKey"`
	ArtifactID           string `gorm:"column:artifact_id"`
	ExtensionID          string `gorm:"column:extension_id"`
	ExtensionVersion     string `gorm:"column:extension_version"`
	Source               string `gorm:"column:source"`
	SessionID            string `gorm:"column:session_id"`
	Revision             int64  `gorm:"column:revision"`
	ManifestJSON         string `gorm:"column:manifest_json"`
	WorkflowJSON         string `gorm:"column:workflow_json"`
	SchemasJSON          string `gorm:"column:schemas_json"`
	CompiledWorkflowJSON string `gorm:"column:compiled_workflow_json"`
	ReadmeText           string `gorm:"column:readme_text"`
	Checksum             string `gorm:"column:checksum"`
	SizeBytes            int64  `gorm:"column:size_bytes"`
	CreatedAt            string `gorm:"column:created_at"`
	ArchivedAt           string `gorm:"column:archived_at"`
	ArtifactKind         string `gorm:"column:artifact_kind"`
	ContentBlob          []byte `gorm:"column:content_blob"`
	ResourceIndexJSON    string `gorm:"column:resource_index_json"`
	ArtifactStatus       string `gorm:"column:artifact_status"`
	OperationID          string `gorm:"column:operation_id"`
}
