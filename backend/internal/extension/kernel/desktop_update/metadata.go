package desktop_update

import (
	"fmt"
	"time"
)

type ExtensionUpdateMetadata struct {
	ExtensionID        string
	Version            string
	ManifestVersion    int
	PackageURL         string
	PackageSHA256      string
	PackageSHA512      string
	PackageSize        int64
	PublisherID        string
	PublisherKeyID     string
	Signature          string
	MinimumHostVersion string
	MaximumHostVersion string
	SupportedPlatforms []string
	SupportedArch      []string
	PublishedAt        time.Time
	ReleaseChannel     string
	Migration          *MigrationMetadata
	RollbackPolicy     string
}

type MigrationMetadata struct {
	HasMigration              bool
	IsReversible              bool
	RequiresManualConfirmation bool
	RollbackNotSupported      bool
}

func (m *ExtensionUpdateMetadata) Validate() error {
	if m.ExtensionID == "" {
		return fmt.Errorf("%w: extension id required", ErrInvalidMetadata)
	}
	if m.Version == "" {
		return fmt.Errorf("%w: version required", ErrInvalidMetadata)
	}
	if _, err := ParseVersion(m.Version); err != nil {
		return fmt.Errorf("%w: invalid version %s", ErrInvalidMetadata, m.Version)
	}
	if m.PackageURL == "" {
		return fmt.Errorf("%w: package url required", ErrInvalidMetadata)
	}
	if m.PackageSHA256 == "" && m.PackageSHA512 == "" {
		return fmt.Errorf("%w: package sha256 or sha512 required", ErrInvalidMetadata)
	}
	if m.PublisherID == "" {
		return fmt.Errorf("%w: publisher id required", ErrInvalidMetadata)
	}
	if m.ManifestVersion <= 0 {
		return fmt.Errorf("%w: manifest version required", ErrInvalidMetadata)
	}
	if m.PackageSize <= 0 {
		return fmt.Errorf("%w: package size must be positive", ErrInvalidMetadata)
	}
	switch m.ReleaseChannel {
	case "", "stable", "beta", "nightly":
	default:
		return fmt.Errorf("%w: invalid release channel %s", ErrInvalidMetadata, m.ReleaseChannel)
	}
	return nil
}

func (m *ExtensionUpdateMetadata) HasMigration() bool {
	return m.Migration != nil && m.Migration.HasMigration
}

func (m *ExtensionUpdateMetadata) IsMigrationReversible() bool {
	if m.Migration == nil {
		return true
	}
	if !m.Migration.HasMigration {
		return true
	}
	return m.Migration.IsReversible
}

func (m *ExtensionUpdateMetadata) RequiresConfirmation() bool {
	if m.Migration != nil && m.Migration.RequiresManualConfirmation {
		return true
	}
	if m.RollbackPolicy == "none" {
		return true
	}
	return false
}

func (m *ExtensionUpdateMetadata) SupportsPlatform(platform string) bool {
	if len(m.SupportedPlatforms) == 0 {
		return true
	}
	for _, p := range m.SupportedPlatforms {
		if p == platform {
			return true
		}
	}
	return false
}

func (m *ExtensionUpdateMetadata) SupportsArch(arch string) bool {
	if len(m.SupportedArch) == 0 {
		return true
	}
	for _, a := range m.SupportedArch {
		if a == arch {
			return true
		}
	}
	return false
}
