package package_distribution

import (
	"encoding/hex"
	"fmt"
	"time"
)

type ExtensionUpdateMetadata struct {
	ExtensionID        string             `json:"extensionId"`
	Version            string             `json:"version"`
	ManifestVersion    int                `json:"manifestVersion"`
	PackageURL         string             `json:"packageUrl"`
	PackageSHA256      string             `json:"packageSha256"`
	PackageSHA512      string             `json:"packageSha512,omitempty"`
	PackageSize        int64              `json:"packageSize"`
	PublisherID        string             `json:"publisherId"`
	PublisherKeyID     string             `json:"publisherKeyId"`
	Signature          string             `json:"signature,omitempty"`
	MinimumHostVersion string             `json:"minimumHostVersion,omitempty"`
	MaximumHostVersion string             `json:"maximumHostVersion,omitempty"`
	SupportedPlatforms []string           `json:"supportedPlatforms,omitempty"`
	SupportedArch      []string           `json:"supportedArch,omitempty"`
	PublishedAt        time.Time          `json:"publishedAt"`
	ReleaseChannel     string             `json:"releaseChannel,omitempty"`
	Migration          *MigrationMetadata `json:"migration,omitempty"`
	RollbackPolicy     string             `json:"rollbackPolicy,omitempty"`
	SignedIndex        bool               `json:"signedIndex"`
}

type MigrationMetadata struct {
	HasMigration               bool `json:"hasMigration"`
	IsReversible               bool `json:"isReversible"`
	RequiresManualConfirmation bool `json:"requiresManualConfirmation"`
	RollbackNotSupported       bool `json:"rollbackNotSupported"`
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
	if m.PackageSHA256 != "" {
		if decoded, err := hex.DecodeString(m.PackageSHA256); err != nil || len(decoded) != 32 {
			return fmt.Errorf("%w: package sha256 must be 64 hexadecimal characters", ErrInvalidMetadata)
		}
	}
	if m.PackageSHA512 != "" {
		if decoded, err := hex.DecodeString(m.PackageSHA512); err != nil || len(decoded) != 64 {
			return fmt.Errorf("%w: package sha512 must be 128 hexadecimal characters", ErrInvalidMetadata)
		}
	}
	if m.PublisherID == "" {
		return fmt.Errorf("%w: publisher id required", ErrInvalidMetadata)
	}
	if m.ManifestVersion <= 0 {
		return fmt.Errorf("%w: manifest version required", ErrInvalidMetadata)
	}
	if m.PackageSize <= 0 && !m.SignedIndex {
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
