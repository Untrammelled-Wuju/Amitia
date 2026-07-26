package package_security

type PackageSourceType string

const (
	SourceLocalFile         PackageSourceType = "local_file"
	SourceUploadedFile      PackageSourceType = "uploaded_file"
	SourceWorkshopArtifact  PackageSourceType = "workshop_artifact"
	SourceGeneratedArtifact PackageSourceType = "generated_artifact"
	SourceMigrationArtifact PackageSourceType = "migration_artifact"
	SourceRemoteDownload    PackageSourceType = "remote_download"
	SourceSystemBundle      PackageSourceType = "system_bundle"
)

func (s PackageSourceType) IsValid() bool {
	switch s {
	case SourceLocalFile, SourceUploadedFile, SourceWorkshopArtifact,
		SourceGeneratedArtifact, SourceMigrationArtifact,
		SourceRemoteDownload, SourceSystemBundle:
		return true
	}
	return false
}

type PackageSource struct {
	SourceType   PackageSourceType `json:"source_type"`
	LocalPath    string            `json:"local_path,omitempty"`
	DisplayName  string            `json:"display_name,omitempty"`
	Origin       string            `json:"origin,omitempty"`
	ExpectedSize int64             `json:"expected_size,omitempty"`
	Metadata     map[string]any    `json:"metadata,omitempty"`
}
