package dataportability

const (
	FormatVersion        = 1
	FormatName           = "amitia-backup"
	MaxPackageSizeBytes  = 1024 * 1024 * 1024 * 64
	MaxSingleResource    = 1024 * 1024 * 1024
	MaxTotalResources    = 1024 * 1024 * 1024 * 16
	MaxArchiveEntries    = 100000
	MaxCompressionRatio  = 100
)

type BackupProfile string
type BackupScope string
type BackupPurpose string
type ComponentKind string

const (
	ProfileFull     BackupProfile = "full"
	ProfilePortable BackupProfile = "portable"

	ScopeAll       BackupScope = "all"
	ScopeCharacter BackupScope = "character"
	ScopeSettings  BackupScope = "settings"

	PurposeMigration  BackupPurpose = "migration"
	PurposeUser       BackupPurpose = "user"
	PurposePreRestore BackupPurpose = "pre_restore"
	PurposePreImport  BackupPurpose = "pre_import"

	KindSQLite     ComponentKind = "sqlite"
	KindDataset    ComponentKind = "dataset"
	KindResource   ComponentKind = "resource"
	KindMetadata   ComponentKind = "metadata"
	KindManifest   ComponentKind = "manifest"
)

type BackupManifest struct {
	Format             string                    `json:"format"`
	FormatVersion      int                       `json:"formatVersion"`
	BackupID           string                    `json:"backupId"`
	CreatedAt          string                    `json:"createdAt"`
	AppVersion         string                    `json:"appVersion"`
	SchemaFingerprint  string                    `json:"schemaFingerprint"`
	Profile            string                    `json:"profile"`
	Scope              string                    `json:"scope"`
	CharacterID        string                    `json:"characterId,omitempty"`
	Encrypted          bool                      `json:"encrypted"`
	SourcePlatform     string                    `json:"sourcePlatform"`
	Components         []BackupComponentManifest `json:"components"`
	RequiresReindex    bool                      `json:"requiresReindex"`
	RequiresMigration  bool                      `json:"requiresMigration,omitempty"`
}

type BackupComponentManifest struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	LogicalName    string `json:"logicalName"`
	Path           string `json:"path"`
	SizeBytes      int64  `json:"sizeBytes"`
	ItemCount      int64  `json:"itemCount"`
	SHA256         string `json:"sha256"`
	Required       bool   `json:"required"`
	SourceOfTruth  bool   `json:"sourceOfTruth"`
	Rebuildable    bool   `json:"rebuildable"`
	Sensitive      bool   `json:"sensitive"`
}
