//go:build legacy_migration

package kernel

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newLegacyMigrationTargetTestFixture(t *testing.T) (*KernelLegacyMigrationTarget, *sql.DB) {
	t.Helper()

	ctx := context.Background()
	root := t.TempDir()

	container, err := NewContainerBuilder().
		WithDBPath(filepath.Join(root, "kernel.db")).
		WithExtensionRoot(filepath.Join(root, "extensions")).
		Build(ctx)
	require.NoError(t, err)

	t.Cleanup(func() { _ = container.Close() })

	runtime, err := NewRuntime(filepath.Join(root, "extensions"))
	require.NoError(t, err)

	runtime.SetContainer(container)

	target, err := NewKernelLegacyMigrationTarget(runtime)
	require.NoError(t, err)

	return target, container.Store.DB()
}

func requireLegacyMigrationCheckpointSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('extension_package_legacy_migration_checkpoints')`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 17, count)

	columnTypes := map[string]string{}
	rows, err := db.Query(`SELECT name, type FROM pragma_table_info('extension_package_legacy_migration_checkpoints')`)
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		var name, typ string
		require.NoError(t, rows.Scan(&name, &typ))
		columnTypes[name] = typ
	}
	require.NoError(t, rows.Err())

	for _, name := range []string{
		"migration_id",
		"extension_id",
		"source_hash",
		"preview_hash",
		"preview_session_id",
		"artifact_id",
		"operation_id",
		"state",
		"current_step",
		"lease_owner",
		"fencing_token",
		"lease_expires_at",
		"verification_hash",
		"last_error",
		"created_at",
		"updated_at",
		"completed_at",
	} {
		_, exists := columnTypes[name]
		require.True(t, exists, "checkpoint column missing: %s", name)
	}
	require.Equal(t, "TEXT", columnTypes["preview_session_id"])

	var uniqueIndexCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM pragma_index_list('extension_package_legacy_migration_checkpoints') WHERE "unique" = 1 AND origin = 'u'`).Scan(&uniqueIndexCount)
	require.NoError(t, err)
	require.Equal(t, 1, uniqueIndexCount)

	var primaryKeyColumn string
	err = db.QueryRow(`SELECT name FROM pragma_table_info('extension_package_legacy_migration_checkpoints') WHERE pk <> 0 ORDER BY pk LIMIT 1`).Scan(&primaryKeyColumn)
	require.NoError(t, err)
	require.Equal(t, "migration_id", primaryKeyColumn)
}

func TestLegacyMigrationCheckpointTableCreated(t *testing.T) {
	target, db := newLegacyMigrationTargetTestFixture(t)

	require.NotNil(t, target)

	requireLegacyMigrationCheckpointSchema(t, db)
}

func TestLegacyMigrationAcquirePersistsCheckpoint(t *testing.T) {
	target, db := newLegacyMigrationTargetTestFixture(t)

	ctx := context.Background()

	checkpoint, err := target.Acquire(ctx, "com.example/legacy", "sha256:abc", "worker-1", 5*time.Minute)
	require.NoError(t, err)

	require.NotEmpty(t, checkpoint.MigrationID)
	require.Equal(t, "com.example/legacy", checkpoint.ExtensionID)
	require.Equal(t, "sha256:abc", checkpoint.SourceHash)
	require.Equal(t, LegacyMigrationStateDetected, checkpoint.State)
	require.Equal(t, "worker-1", checkpoint.LeaseOwner)
	require.Equal(t, int64(1), checkpoint.FencingToken)
	require.NotEmpty(t, checkpoint.LeaseExpiresAt)

	var persistedMigrationID string
	err = db.QueryRow(`SELECT migration_id FROM extension_package_legacy_migration_checkpoints WHERE extension_id = ?`, "com.example/legacy").Scan(&persistedMigrationID)
	require.NoError(t, err)
	require.Equal(t, checkpoint.MigrationID, persistedMigrationID)
}

func TestLegacyMigrationAcquireRejectsSourceHashChange(t *testing.T) {
	target, _ := newLegacyMigrationTargetTestFixture(t)

	ctx := context.Background()

	checkpoint, err := target.Acquire(ctx, "com.example/legacy", "sha256:abc", "worker-1", 5*time.Minute)
	require.NoError(t, err)

	require.NoError(t, target.Release(ctx, checkpoint))

	_, err = target.Acquire(ctx, "com.example/legacy", "sha256:changed", "worker-1", 5*time.Minute)
	require.Error(t, err)
	require.Contains(t, err.Error(), "source changed")
}

func TestLegacyMigrationLeaseFencing(t *testing.T) {
	target, _ := newLegacyMigrationTargetTestFixture(t)

	ctx := context.Background()

	checkpointA, err := target.Acquire(ctx, "com.example/legacy", "sha256:abc", "worker-a", 5*time.Minute)
	require.NoError(t, err)

	_, err = target.Acquire(ctx, "com.example/legacy", "sha256:abc", "worker-b", 5*time.Minute)
	require.ErrorIs(t, err, ErrLegacyMigrationLeaseHeld)

	require.NoError(t, target.Release(ctx, checkpointA))

	checkpointB, err := target.Acquire(ctx, "com.example/legacy", "sha256:abc", "worker-b", 5*time.Minute)
	require.NoError(t, err)
	require.Equal(t, "worker-b", checkpointB.LeaseOwner)
	require.Greater(t, checkpointB.FencingToken, checkpointA.FencingToken)

	_, err = target.Acquire(ctx, "com.example/legacy", "sha256:abc", "worker-a", 5*time.Minute)
	require.ErrorIs(t, err, ErrLegacyMigrationLeaseHeld)
}

func TestLegacyMigrationVerifyRejectsEmptyOperationID(t *testing.T) {
	target, _ := newLegacyMigrationTargetTestFixture(t)

	_, err := target.Verify(
		context.Background(),
		"com.example/legacy",
		"artifact-id",
		"",
		"sha256:abc",
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "identity incomplete")
}

func moveCheckpointToVerifying(t *testing.T, target *KernelLegacyMigrationTarget, checkpoint LegacyMigrationCheckpoint, artifactID, operationID string) LegacyMigrationCheckpoint {
	t.Helper()

	update := LegacyMigrationCheckpointUpdate{
		State:       LegacyMigrationStateVerifying,
		CurrentStep: "verify_kernel_install",
		ArtifactID:  artifactID,
		OperationID: operationID,
	}

	require.NoError(t, target.Update(context.Background(), checkpoint, update))

	checkpoint.State = update.State
	checkpoint.CurrentStep = update.CurrentStep
	checkpoint.ArtifactID = update.ArtifactID
	checkpoint.OperationID = update.OperationID

	return checkpoint
}

func TestLegacyMigrationCompleteRejectsFinalGateFalse(t *testing.T) {
	target, _ := newLegacyMigrationTargetTestFixture(t)

	ctx := context.Background()

	checkpoint, err := target.Acquire(ctx, "com.example/legacy", "sha256:abc", "worker-1", 5*time.Minute)
	require.NoError(t, err)

	checkpoint = moveCheckpointToVerifying(t, target, checkpoint, "artifact-id", "operation-id")

	verification := LegacyMigrationVerification{
		ExtensionID:      checkpoint.ExtensionID,
		ArtifactID:       "artifact-id",
		OperationID:      "operation-id",
		SourceHash:       "sha256:abc",
		InstalledVersion: "1.0.0",
		PackageID:        "artifact-id",
		FinalGatePassed:  false,
	}
	verification.VerificationHash = hashLegacyMigrationVerification(verification)

	err = target.Complete(ctx, checkpoint, verification)
	require.Error(t, err)

	status, found, statusErr := target.Status(ctx, checkpoint.ExtensionID)
	require.NoError(t, statusErr)
	require.True(t, found)
	require.Equal(t, LegacyMigrationStateVerifying, status.State)
}

func TestLegacyMigrationCompleteRejectsVerificationHashMismatch(t *testing.T) {
	target, _ := newLegacyMigrationTargetTestFixture(t)

	ctx := context.Background()

	checkpoint, err := target.Acquire(ctx, "com.example/legacy", "sha256:abc", "worker-1", 5*time.Minute)
	require.NoError(t, err)

	checkpoint = moveCheckpointToVerifying(t, target, checkpoint, "artifact-id", "operation-id")

	verification := LegacyMigrationVerification{
		ExtensionID:      checkpoint.ExtensionID,
		ArtifactID:       "artifact-id",
		OperationID:      "operation-id",
		SourceHash:       "sha256:abc",
		InstalledVersion: "1.0.0",
		PackageID:        "artifact-id",
		FinalGatePassed:  true,
	}
	verification.VerificationHash = "sha256:forged"

	err = target.Complete(ctx, checkpoint, verification)
	require.Error(t, err)
	require.Contains(t, err.Error(), "hash mismatch")

	status, found, statusErr := target.Status(ctx, checkpoint.ExtensionID)
	require.NoError(t, statusErr)
	require.True(t, found)
	require.Equal(t, LegacyMigrationStateVerifying, status.State)
}

func TestLegacyMigrationCompleteIsAtomic(t *testing.T) {
	target, db := newLegacyMigrationTargetTestFixture(t)

	ctx := context.Background()

	checkpoint, err := target.Acquire(ctx, "com.example/legacy", "sha256:abc", "worker-1", 5*time.Minute)
	require.NoError(t, err)

	checkpoint = moveCheckpointToVerifying(t, target, checkpoint, "artifact-id", "operation-id")

	verification := LegacyMigrationVerification{
		ExtensionID:      checkpoint.ExtensionID,
		ArtifactID:       "artifact-id",
		OperationID:      "operation-id",
		SourceHash:       "sha256:abc",
		InstalledVersion: "1.0.0",
		PackageID:        "artifact-id",
		FinalGatePassed:  true,
	}
	verification.VerificationHash = hashLegacyMigrationVerification(verification)

	defer func() {
		_, _ = db.Exec(`DROP TRIGGER IF EXISTS fail_legacy_migration_summary`)
	}()

	_, err = db.Exec(`
CREATE TRIGGER fail_legacy_migration_summary
BEFORE INSERT ON extension_package_legacy_migrations
BEGIN
SELECT RAISE(ABORT, 'forced summary failure');
END;
`)
	require.NoError(t, err)

	err = target.Complete(ctx, checkpoint, verification)
	require.Error(t, err)

	var state string
	stateErr := db.QueryRow(`SELECT state FROM extension_package_legacy_migration_checkpoints WHERE extension_id=?`, checkpoint.ExtensionID).Scan(&state)
	require.NoError(t, stateErr)
	require.Equal(t, LegacyMigrationStateVerifying, state)

	status, found, statusErr := target.Status(ctx, checkpoint.ExtensionID)
	require.NoError(t, statusErr)
	require.True(t, found)
	require.Equal(t, LegacyMigrationStateVerifying, status.State)
}

func TestLegacyMigrationCompletePersistsSummaryAndClearsLease(t *testing.T) {
	target, db := newLegacyMigrationTargetTestFixture(t)

	ctx := context.Background()

	checkpoint, err := target.Acquire(ctx, "com.example/legacy", "sha256:abc", "worker-1", 5*time.Minute)
	require.NoError(t, err)

	checkpoint = moveCheckpointToVerifying(t, target, checkpoint, "artifact-id", "operation-id")

	verification := LegacyMigrationVerification{
		ExtensionID:      checkpoint.ExtensionID,
		ArtifactID:       "artifact-id",
		OperationID:      "operation-id",
		SourceHash:       "sha256:abc",
		InstalledVersion: "1.0.0",
		PackageID:        "artifact-id",
		FinalGatePassed:  true,
	}
	verification.VerificationHash = hashLegacyMigrationVerification(verification)

	require.NoError(t, target.Complete(ctx, checkpoint, verification))

	status, found, statusErr := target.Status(ctx, checkpoint.ExtensionID)
	require.NoError(t, statusErr)
	require.True(t, found)
	require.Equal(t, LegacyMigrationStateCompleted, status.State)
	require.Equal(t, "artifact-id", status.ArtifactID)
	require.Equal(t, "operation-id", status.OperationID)
	require.Equal(t, verification.VerificationHash, status.VerificationHash)
	require.Empty(t, status.LeaseOwner)
	require.Empty(t, status.LeaseExpiresAt)
	require.NotEmpty(t, status.CompletedAt)

	var migrationStatus string
	migrationErr := db.QueryRow(`SELECT migration_status FROM extension_package_legacy_migrations WHERE extension_id=?`, checkpoint.ExtensionID).Scan(&migrationStatus)
	require.NoError(t, migrationErr)
	require.Equal(t, LegacyMigrationStateCompleted, migrationStatus)

	_, err = target.Acquire(ctx, checkpoint.ExtensionID, checkpoint.SourceHash, "worker-1", 5*time.Minute)
	require.ErrorIs(t, err, ErrLegacyMigrationAlreadyCompleted)
}

func TestLegacyMigrationCompleteRejectsNonVerifyingState(t *testing.T) {
	target, _ := newLegacyMigrationTargetTestFixture(t)

	ctx := context.Background()

	checkpoint, err := target.Acquire(ctx, "com.example/legacy", "sha256:abc", "worker-1", 5*time.Minute)
	require.NoError(t, err)

	verification := LegacyMigrationVerification{
		ExtensionID:      checkpoint.ExtensionID,
		ArtifactID:       "artifact-id",
		OperationID:      "operation-id",
		SourceHash:       "sha256:abc",
		InstalledVersion: "1.0.0",
		PackageID:        "artifact-id",
		FinalGatePassed:  true,
	}
	verification.VerificationHash = hashLegacyMigrationVerification(verification)

	err = target.Complete(ctx, checkpoint, verification)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be verifying")
}
