//go:build legacy_migration

package package_legacy_migration

import (
	"context"
	"io"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
)

type LegacyPackageCandidate struct {
	ExtensionID string `gorm:"column:extension_id"`
	Version     string `gorm:"column:version"`
	PackageBlob []byte `gorm:"column:package_blob"`

	UserID    string `gorm:"column:user_id"`
	ScopeType string `gorm:"column:scope_type"`
	ScopeID   string `gorm:"column:scope_id"`
}

type LegacySignerCandidate struct {
	Fingerprint string `gorm:"column:fingerprint"`
	CreatedAt   string `gorm:"column:created_at"`
}

type MigrationReport struct {
	Total             int
	Completed         int
	PendingManual     int
	Blocked           int
	PendingExtensions []string
}

type KernelMigrationTarget interface {
	Acquire(
		ctx context.Context,
		extensionID string,
		sourceHash string,
		ownerID string,
		ttl time.Duration,
	) (
		kernel.LegacyMigrationCheckpoint,
		error,
	)

	Update(
		ctx context.Context,
		checkpoint kernel.LegacyMigrationCheckpoint,
		update kernel.LegacyMigrationCheckpointUpdate,
	) error

	Complete(
		ctx context.Context,
		checkpoint kernel.LegacyMigrationCheckpoint,
		verification kernel.LegacyMigrationVerification,
	) error

	Release(
		ctx context.Context,
		checkpoint kernel.LegacyMigrationCheckpoint,
	) error

	Status(
		ctx context.Context,
		extensionID string,
	) (
		kernel.LegacyMigrationCheckpoint,
		bool,
		error,
	)

	List(
		ctx context.Context,
	) ([]kernel.LegacyMigrationCheckpoint, error)

	ExtensionExists(
		ctx context.Context,
		extensionID string,
	) (bool, error)

	InstallationArtifactID(
		ctx context.Context,
		extensionID string,
	) (string, error)

	Preview(
		ctx context.Context,
		request kernel.PackagePreviewRequest,
		reader io.Reader,
	) (kernel.InstallPreview, error)

	Install(
		ctx context.Context,
		request kernel.PackageInstallRequest,
	) (kernel.KernelInstallResult, error)

	Verify(
		ctx context.Context,
		extensionID string,
		artifactID string,
		operationID string,
		sourceHash string,
	) (
		kernel.LegacyMigrationVerification,
		error,
	)

	PutSigner(
		ctx context.Context,
		record kernel.PackagePublisherKeyRecord,
	) error
}
