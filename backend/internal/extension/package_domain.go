package extension

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

type packageImportSessionRecord struct {
	ID          string `gorm:"column:id;primaryKey"`
	SpaceID     string `gorm:"column:space_id"`
	ScopeType   string `gorm:"column:scope_type"`
	ScopeID     string `gorm:"column:scope_id"`
	Format      string `gorm:"column:format"`
	PackageHash string `gorm:"column:package_hash"`
	Status      string `gorm:"column:status"`
	PreviewJSON string `gorm:"column:preview_json"`
	PackageBlob []byte `gorm:"column:package_blob"`
	FileName    string `gorm:"column:file_name"`
	ExpiresAt   string `gorm:"column:expires_at"`
	ConsumedAt  string `gorm:"column:consumed_at"`
	CreatedAt   string `gorm:"column:created_at"`
	UpdatedAt   string `gorm:"column:updated_at"`
}

func (packageImportSessionRecord) TableName() string { return "extension_package_import_sessions" }
