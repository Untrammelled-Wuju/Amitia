//go:build legacy_migration

package kernel

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

const (
	LegacyMigrationStateDetected       = "detected"
	LegacyMigrationStatePreviewed      = "previewed"
	LegacyMigrationStateMigrating      = "migrating"
	LegacyMigrationStateVerifying      = "verifying"
	LegacyMigrationStateCompleted      = "completed"
	LegacyMigrationStateManualRequired = "manual_required"
	LegacyMigrationStateBlocked        = "blocked"
)

var (
	ErrLegacyMigrationLeaseHeld =
		errors.New("kernel: legacy migration lease held")

	ErrLegacyMigrationAlreadyCompleted =
		errors.New("kernel: legacy migration already completed")

	ErrLegacyMigrationFenced =
		errors.New("kernel: legacy migration fenced")
)

type LegacyMigrationCheckpoint struct {
	MigrationID string
	ExtensionID string

	SourceHash  string
	PreviewHash string
	ArtifactID  string
	OperationID string

	State       string
	CurrentStep string

	LeaseOwner    string
	FencingToken  int64
	LeaseExpiresAt string

	VerificationHash string
	LastError        string

	CreatedAt   string
	UpdatedAt   string
	CompletedAt string
}

type LegacyMigrationCheckpointUpdate struct {
	State       string
	CurrentStep string

	PreviewHash string
	ArtifactID  string
	OperationID string

	LastError string
}

type LegacyMigrationVerification struct {
	ExtensionID string `json:"extensionId"`
	ArtifactID  string `json:"artifactId"`
	OperationID string `json:"operationId"`
	SourceHash  string `json:"sourceHash"`

	InstalledVersion string `json:"installedVersion"`
	PackageID        string `json:"packageId"`
	FinalGatePassed  bool   `json:"finalGatePassed"`

	VerificationHash string `json:"verificationHash"`
}

type KernelLegacyMigrationTarget struct {
	runtime *Runtime
	db      *sql.DB
}

func NewKernelLegacyMigrationTarget(
	runtime *Runtime,
) (*KernelLegacyMigrationTarget, error) {
	if runtime == nil ||
		runtime.Container() == nil ||
		runtime.Container().Store == nil ||
		runtime.Container().PackageRepository == nil ||
		runtime.Container().InstallationRepository == nil ||
		runtime.Container().PackageTrustRepository == nil {
		return nil,
			fmt.Errorf(
				"kernel: legacy migration target dependencies unavailable",
			)
	}

	return &KernelLegacyMigrationTarget{
		runtime: runtime,
		db:      runtime.Container().Store.DB(),
	}, nil
}

func isSupportedLegacyMigrationState(
	state string,
) bool {
	switch state {
	case LegacyMigrationStateDetected,
		LegacyMigrationStatePreviewed,
		LegacyMigrationStateMigrating,
		LegacyMigrationStateVerifying,
		LegacyMigrationStateCompleted,
		LegacyMigrationStateManualRequired,
		LegacyMigrationStateBlocked:
		return true
	default:
		return false
	}
}

func scanLegacyMigrationCheckpoint(
	row interface {
		Scan(dest ...any) error
	},
) (
	LegacyMigrationCheckpoint,
	error,
) {
	var checkpoint LegacyMigrationCheckpoint

	err := row.Scan(
		&checkpoint.MigrationID,
		&checkpoint.ExtensionID,
		&checkpoint.SourceHash,
		&checkpoint.PreviewHash,
		&checkpoint.ArtifactID,
		&checkpoint.OperationID,
		&checkpoint.State,
		&checkpoint.CurrentStep,
		&checkpoint.LeaseOwner,
		&checkpoint.FencingToken,
		&checkpoint.LeaseExpiresAt,
		&checkpoint.VerificationHash,
		&checkpoint.LastError,
		&checkpoint.CreatedAt,
		&checkpoint.UpdatedAt,
		&checkpoint.CompletedAt,
	)

	return checkpoint, err
}

const legacyMigrationCheckpointColumns = `
	migration_id,
	extension_id,
	source_hash,
	preview_hash,
	artifact_id,
	operation_id,
	state,
	current_step,
	lease_owner,
	fencing_token,
	lease_expires_at,
	verification_hash,
	last_error,
	created_at,
	updated_at,
	completed_at
`

func (t *KernelLegacyMigrationTarget) Acquire(
	ctx context.Context,
	extensionID string,
	sourceHash string,
	ownerID string,
	ttl time.Duration,
) (
	LegacyMigrationCheckpoint,
	error,
) {
	if strings.TrimSpace(extensionID) == "" ||
		strings.TrimSpace(sourceHash) == "" ||
		strings.TrimSpace(ownerID) == "" {
		return LegacyMigrationCheckpoint{},
			fmt.Errorf(
				"kernel: legacy migration acquisition identity incomplete",
			)
	}

	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return LegacyMigrationCheckpoint{}, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	nowText := now.Format(time.RFC3339Nano)

	_, err = tx.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO
		extension_package_legacy_migration_checkpoints (
			migration_id,
			extension_id,
			source_hash,
			state,
			current_step,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"legacy-migration-"+uuid.NewString(),
		extensionID,
		sourceHash,
		LegacyMigrationStateDetected,
		"detected",
		nowText,
		nowText,
	)
	if err != nil {
		return LegacyMigrationCheckpoint{}, err
	}

	checkpoint, err :=
		scanLegacyMigrationCheckpoint(
			tx.QueryRowContext(
				ctx,
				`SELECT `+
					legacyMigrationCheckpointColumns+
					`
				 FROM extension_package_legacy_migration_checkpoints
				 WHERE extension_id=?`,
				extensionID,
			),
		)
	if err != nil {
		return LegacyMigrationCheckpoint{}, err
	}

	if checkpoint.State ==
		LegacyMigrationStateCompleted {
		return checkpoint,
			ErrLegacyMigrationAlreadyCompleted
	}

	if checkpoint.SourceHash != sourceHash {
		return LegacyMigrationCheckpoint{},
			fmt.Errorf(
				"kernel: legacy migration source changed for %s",
				extensionID,
			)
	}

	if checkpoint.LeaseOwner != "" &&
		checkpoint.LeaseOwner != ownerID {
		expiresAt, parseErr :=
			time.Parse(
				time.RFC3339Nano,
				checkpoint.LeaseExpiresAt,
			)

		if parseErr != nil {
			return LegacyMigrationCheckpoint{},
				fmt.Errorf(
					"kernel: invalid legacy migration lease expiration: %w",
					parseErr,
				)
		}

		if expiresAt.After(now) {
			return LegacyMigrationCheckpoint{},
				ErrLegacyMigrationLeaseHeld
		}
	}

	nextFencingToken :=
		checkpoint.FencingToken + 1

	leaseExpiresAt :=
		now.Add(ttl).
			Format(time.RFC3339Nano)

	result, err := tx.ExecContext(
		ctx,
		`UPDATE extension_package_legacy_migration_checkpoints
		 SET
			lease_owner=?,
			fencing_token=?,
			lease_expires_at=?,
			updated_at=?
		 WHERE extension_id=?
		   AND fencing_token=?
		   AND state<>?`,
		ownerID,
		nextFencingToken,
		leaseExpiresAt,
		nowText,
		extensionID,
		checkpoint.FencingToken,
		LegacyMigrationStateCompleted,
	)
	if err != nil {
		return LegacyMigrationCheckpoint{}, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return LegacyMigrationCheckpoint{}, err
	}

	if rows != 1 {
		return LegacyMigrationCheckpoint{},
			ErrLegacyMigrationFenced
	}

	checkpoint, err =
		scanLegacyMigrationCheckpoint(
			tx.QueryRowContext(
				ctx,
				`SELECT `+
					legacyMigrationCheckpointColumns+
					`
				 FROM extension_package_legacy_migration_checkpoints
				 WHERE extension_id=?`,
				extensionID,
			),
		)
	if err != nil {
		return LegacyMigrationCheckpoint{}, err
	}

	if err := tx.Commit(); err != nil {
		return LegacyMigrationCheckpoint{}, err
	}

	return checkpoint, nil
}

func (t *KernelLegacyMigrationTarget) Update(
	ctx context.Context,
	checkpoint LegacyMigrationCheckpoint,
	update LegacyMigrationCheckpointUpdate,
) error {
	if !isSupportedLegacyMigrationState(
		update.State,
	) {
		return fmt.Errorf(
			"kernel: unsupported legacy migration state %s",
			update.State,
		)
	}

	if strings.TrimSpace(
		checkpoint.LeaseOwner,
	) == "" ||
		checkpoint.FencingToken <= 0 {
		return fmt.Errorf(
			"kernel: legacy migration write guard incomplete",
		)
	}

	result, err := t.db.ExecContext(
		ctx,
		`UPDATE extension_package_legacy_migration_checkpoints
		 SET
			state=?,
			current_step=?,
			preview_hash=CASE
				WHEN ?='' THEN preview_hash
				ELSE ?
			END,
			artifact_id=CASE
				WHEN ?='' THEN artifact_id
				ELSE ?
			END,
			operation_id=CASE
				WHEN ?='' THEN operation_id
				ELSE ?
			END,
			last_error=?,
			updated_at=?
		 WHERE extension_id=?
		   AND lease_owner=?
		   AND fencing_token=?
		   AND state<>?`,
		update.State,
		update.CurrentStep,
		update.PreviewHash,
		update.PreviewHash,
		update.ArtifactID,
		update.ArtifactID,
		update.OperationID,
		update.OperationID,
		update.LastError,
		time.Now().UTC().
			Format(time.RFC3339Nano),
		checkpoint.ExtensionID,
		checkpoint.LeaseOwner,
		checkpoint.FencingToken,
		LegacyMigrationStateCompleted,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows != 1 {
		return ErrLegacyMigrationFenced
	}

	return nil
}

func (t *KernelLegacyMigrationTarget) Complete(
	ctx context.Context,
	checkpoint LegacyMigrationCheckpoint,
	verification LegacyMigrationVerification,
) error {
	if strings.TrimSpace(
		verification.VerificationHash,
	) == "" {
		return fmt.Errorf(
			"kernel: legacy migration verification hash missing",
		)
	}

	now :=
		time.Now().UTC().
			Format(time.RFC3339Nano)

	result, err := t.db.ExecContext(
		ctx,
		`UPDATE extension_package_legacy_migration_checkpoints
		 SET
			state=?,
			current_step='completed',
			artifact_id=?,
			operation_id=?,
			verification_hash=?,
			last_error='',
			lease_owner='',
			lease_expires_at='',
			completed_at=?,
			updated_at=?
		 WHERE extension_id=?
		   AND lease_owner=?
		   AND fencing_token=?
		   AND state=?`,
		LegacyMigrationStateCompleted,
		verification.ArtifactID,
		verification.OperationID,
		verification.VerificationHash,
		now,
		now,
		checkpoint.ExtensionID,
		checkpoint.LeaseOwner,
		checkpoint.FencingToken,
		LegacyMigrationStateVerifying,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows != 1 {
		return ErrLegacyMigrationFenced
	}

	_, err = t.db.ExecContext(
		ctx,
		`INSERT INTO extension_package_legacy_migrations (
			extension_id,
			migration_status,
			attempt_count,
			last_error,
			legacy_path,
			artifact_id,
			updated_at
		) VALUES (?, ?, 1, '', ?, ?, ?)
		ON CONFLICT(extension_id) DO UPDATE SET
			migration_status=excluded.migration_status,
			attempt_count=
				extension_package_legacy_migrations.attempt_count+1,
			last_error='',
			legacy_path=excluded.legacy_path,
			artifact_id=excluded.artifact_id,
			updated_at=excluded.updated_at`,
		checkpoint.ExtensionID,
		LegacyMigrationStateCompleted,
		checkpoint.SourceHash,
		verification.ArtifactID,
		now,
	)

	return err
}

func (t *KernelLegacyMigrationTarget) Release(
	ctx context.Context,
	checkpoint LegacyMigrationCheckpoint,
) error {
	result, err := t.db.ExecContext(
		ctx,
		`UPDATE extension_package_legacy_migration_checkpoints
		 SET
			lease_owner='',
			lease_expires_at='',
			updated_at=?
		 WHERE extension_id=?
		   AND lease_owner=?
		   AND fencing_token=?`,
		time.Now().UTC().
			Format(time.RFC3339Nano),
		checkpoint.ExtensionID,
		checkpoint.LeaseOwner,
		checkpoint.FencingToken,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrLegacyMigrationFenced
	}

	return nil
}

func (t *KernelLegacyMigrationTarget) Status(
	ctx context.Context,
	extensionID string,
) (
	LegacyMigrationCheckpoint,
	bool,
	error,
) {
	checkpoint, err :=
		scanLegacyMigrationCheckpoint(
			t.db.QueryRowContext(
				ctx,
				`SELECT `+
					legacyMigrationCheckpointColumns+
					`
				 FROM extension_package_legacy_migration_checkpoints
				 WHERE extension_id=?`,
				extensionID,
			),
		)

	if errors.Is(err, sql.ErrNoRows) {
		return LegacyMigrationCheckpoint{},
			false,
			nil
	}

	if err != nil {
		return LegacyMigrationCheckpoint{},
			false,
			err
	}

	return checkpoint, true, nil
}

func (t *KernelLegacyMigrationTarget) List(
	ctx context.Context,
) ([]LegacyMigrationCheckpoint, error) {
	rows, err := t.db.QueryContext(
		ctx,
		`SELECT `+
			legacyMigrationCheckpointColumns+
			`
		 FROM extension_package_legacy_migration_checkpoints
		 ORDER BY extension_id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result :=
		make(
			[]LegacyMigrationCheckpoint,
			0,
			32,
		)

	for rows.Next() {
		checkpoint, err :=
			scanLegacyMigrationCheckpoint(
				rows,
			)
		if err != nil {
			return nil, err
		}

		result = append(
			result,
			checkpoint,
		)
	}

	return result, rows.Err()
}

func (t *KernelLegacyMigrationTarget) ExtensionExists(
	ctx context.Context,
	extensionID string,
) (bool, error) {
	_, err :=
		t.runtime.Container().
			InstallationRepository.
			GetInstallation(
				ctx,
				domain.ExtensionID(
					extensionID,
				),
			)

	if err == nil {
		return true, nil
	}

	if errors.Is(
		err,
		domain.ErrInvalidExtensionID,
	) {
		return false, nil
	}

	return false, err
}

func (t *KernelLegacyMigrationTarget) InstallationArtifactID(
	ctx context.Context,
	extensionID string,
) (string, error) {
	installation, err :=
		t.runtime.Container().
			InstallationRepository.
			GetInstallation(
				ctx,
				domain.ExtensionID(
					extensionID,
				),
			)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(
		installation.PackageID,
	) == "" {
		return "",
			fmt.Errorf(
				"kernel: installation package id missing",
			)
	}

	return installation.PackageID, nil
}

func (t *KernelLegacyMigrationTarget) Preview(
	ctx context.Context,
	request PackagePreviewRequest,
	reader io.Reader,
) (InstallPreview, error) {
	return t.runtime.PreviewPackage(
		ctx,
		request,
		reader,
	)
}

func (t *KernelLegacyMigrationTarget) Install(
	ctx context.Context,
	request PackageInstallRequest,
) (KernelInstallResult, error) {
	return t.runtime.ExecutePackageInstall(
		ctx,
		request,
	)
}

func hashLegacyMigrationVerification(
	verification LegacyMigrationVerification,
) string {
	copyValue := verification
	copyValue.VerificationHash = ""

	raw, err := json.Marshal(copyValue)
	if err != nil {
		panic(err)
	}

	sum := sha256.Sum256(raw)

	return "sha256:" +
		hex.EncodeToString(sum[:])
}

func (t *KernelLegacyMigrationTarget) Verify(
	ctx context.Context,
	extensionID string,
	artifactID string,
	operationID string,
	sourceHash string,
) (
	LegacyMigrationVerification,
	error,
) {
	installation, err :=
		t.runtime.Container().
			InstallationRepository.
			GetInstallation(
				ctx,
				domain.ExtensionID(
					extensionID,
				),
			)
	if err != nil {
		return LegacyMigrationVerification{},
			err
	}

	if installation.PackageID != artifactID {
		return LegacyMigrationVerification{},
			fmt.Errorf(
				"kernel: migrated installation artifact mismatch: %s != %s",
				installation.PackageID,
				artifactID,
			)
	}

	finalGatePassed := false

	if strings.TrimSpace(
		operationID,
	) != "" {
		gate, err :=
			t.runtime.VerifyPackageFinalGate(
				ctx,
				operationID,
			)
		if err != nil {
			return LegacyMigrationVerification{},
				err
			}

		if !gate.Passed {
			return LegacyMigrationVerification{},
				fmt.Errorf(
					"kernel: migrated operation final gate failed",
				)
		}

		if gate.ExtensionID != extensionID {
			return LegacyMigrationVerification{},
				fmt.Errorf(
					"kernel: migrated operation extension mismatch",
				)
		}

		finalGatePassed = true
	}

	verification :=
		LegacyMigrationVerification{
			ExtensionID:
				extensionID,
			ArtifactID:
				artifactID,
			OperationID:
				operationID,
			SourceHash:
				sourceHash,
			InstalledVersion:
				installation.
					InstalledVersion.
					String(),
			PackageID:
				installation.PackageID,
			FinalGatePassed:
				finalGatePassed,
		}

	verification.VerificationHash =
		hashLegacyMigrationVerification(
			verification,
		)

	return verification, nil
}

func (t *KernelLegacyMigrationTarget) PutSigner(
	ctx context.Context,
	record PackagePublisherKeyRecord,
) error {
	return t.runtime.Container().
		PackageTrustRepository.
		Put(
			ctx,
			record,
		)
}
