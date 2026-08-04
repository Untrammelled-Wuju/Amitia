//go:build legacy_migration

package package_legacy_migration

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/u-ai/backend/internal/extension/kernel"
)

type fakeLegacyMigrationSource struct {
	candidates []LegacyPackageCandidate
	signers    []LegacySignerCandidate

	err error
}

func (f *fakeLegacyMigrationSource) BackfillOwnership(
	ctx context.Context,
) error {
	return f.err
}

func (f *fakeLegacyMigrationSource) ListCandidates(
	ctx context.Context,
) ([]LegacyPackageCandidate, error) {
	if f.err != nil {
		return nil, f.err
	}

	return f.candidates, nil
}

func (f *fakeLegacyMigrationSource) ListSigners(
	ctx context.Context,
) ([]LegacySignerCandidate, error) {
	if f.err != nil {
		return nil, f.err
	}

	return f.signers, nil
}

type fakeMigrationTarget struct {
	acquireErr  error
	completeErr error

	presetCheckpoint *kernel.LegacyMigrationCheckpoint

	exists        bool
	existsErr     error
	artifactID    string
	artifactIDErr error
	preview       kernel.InstallPreview
	previewErr    error
	installResult kernel.KernelInstallResult
	installErr    error
	verifyResult  kernel.LegacyMigrationVerification
	verifyErr     error

	AcquireCalls  int
	UpdateCalls   int
	PreviewCalls  int
	InstallCalls  int
	VerifyCalls   int
	CompleteCalls int
	ReleaseCalls  int

	state      string
	lastUpdate kernel.LegacyMigrationCheckpointUpdate
}

func (f *fakeMigrationTarget) Acquire(
	ctx context.Context,
	extensionID string,
	sourceHash string,
	ownerID string,
	ttl time.Duration,
) (
	kernel.LegacyMigrationCheckpoint,
	error,
) {
	f.AcquireCalls++

	if f.acquireErr != nil {
		return kernel.LegacyMigrationCheckpoint{}, f.acquireErr
	}

	checkpoint := kernel.LegacyMigrationCheckpoint{
		MigrationID:    "mig-" + extensionID,
		ExtensionID:    extensionID,
		SourceHash:     sourceHash,
		State:          kernel.LegacyMigrationStateDetected,
		CurrentStep:    "detected",
		LeaseOwner:     ownerID,
		FencingToken:   1,
		LeaseExpiresAt: time.Now().Add(ttl).Format(time.RFC3339Nano),
	}

	if f.presetCheckpoint != nil &&
		f.presetCheckpoint.ExtensionID == extensionID {
		checkpoint = *f.presetCheckpoint
		checkpoint.SourceHash = sourceHash
		checkpoint.LeaseOwner = ownerID
		checkpoint.FencingToken++
		checkpoint.LeaseExpiresAt =
			time.Now().Add(ttl).
				Format(time.RFC3339Nano)
	}

	f.state = checkpoint.State

	return checkpoint, nil
}

func (f *fakeMigrationTarget) Update(
	ctx context.Context,
	checkpoint kernel.LegacyMigrationCheckpoint,
	update kernel.LegacyMigrationCheckpointUpdate,
) error {
	f.UpdateCalls++
	f.lastUpdate = update
	f.state = update.State

	return nil
}

func (f *fakeMigrationTarget) Complete(
	ctx context.Context,
	checkpoint kernel.LegacyMigrationCheckpoint,
	verification kernel.LegacyMigrationVerification,
) error {
	f.CompleteCalls++

	if f.completeErr != nil {
		return f.completeErr
	}

	f.state = kernel.LegacyMigrationStateCompleted
	f.lastUpdate = kernel.LegacyMigrationCheckpointUpdate{
		State: kernel.LegacyMigrationStateCompleted,
	}

	return nil
}

func (f *fakeMigrationTarget) Release(
	ctx context.Context,
	checkpoint kernel.LegacyMigrationCheckpoint,
) error {
	f.ReleaseCalls++
	f.state = checkpoint.State

	return nil
}

func (f *fakeMigrationTarget) Status(
	ctx context.Context,
	extensionID string,
) (
	kernel.LegacyMigrationCheckpoint,
	bool,
	error,
) {
	return kernel.LegacyMigrationCheckpoint{}, false, nil
}

func (f *fakeMigrationTarget) List(
	ctx context.Context,
) ([]kernel.LegacyMigrationCheckpoint, error) {
	return nil, nil
}

func (f *fakeMigrationTarget) ExtensionExists(
	ctx context.Context,
	extensionID string,
) (bool, error) {
	return f.exists, f.existsErr
}

func (f *fakeMigrationTarget) InstallationArtifactID(
	ctx context.Context,
	extensionID string,
) (string, error) {
	return f.artifactID, f.artifactIDErr
}

func (f *fakeMigrationTarget) Preview(
	ctx context.Context,
	request kernel.PackagePreviewRequest,
	reader io.Reader,
) (kernel.InstallPreview, error) {
	f.PreviewCalls++

	return f.preview, f.previewErr
}

func (f *fakeMigrationTarget) Install(
	ctx context.Context,
	request kernel.PackageInstallRequest,
) (kernel.KernelInstallResult, error) {
	f.InstallCalls++

	return f.installResult, f.installErr
}

func (f *fakeMigrationTarget) Verify(
	ctx context.Context,
	extensionID string,
	artifactID string,
	operationID string,
	sourceHash string,
) (
	kernel.LegacyMigrationVerification,
	error,
) {
	f.VerifyCalls++

	if f.verifyErr != nil {
		return kernel.LegacyMigrationVerification{}, f.verifyErr
	}

	result := f.verifyResult
	result.ExtensionID = extensionID
	result.ArtifactID = artifactID
	result.OperationID = operationID
	result.SourceHash = sourceHash

	return result, nil
}

func (f *fakeMigrationTarget) PutSigner(
	ctx context.Context,
	record kernel.PackagePublisherKeyRecord,
) error {
	return nil
}

const fakeLegacyExtensionID = "com.example/legacy"

func newMigrationServiceTestFixture(
	t *testing.T,
	target *fakeMigrationTarget,
) *MigrationService {
	t.Helper()

	source := &fakeLegacyMigrationSource{
		candidates: []LegacyPackageCandidate{
			{
				ExtensionID: fakeLegacyExtensionID,
				Version:     "1.0.0",
				PackageBlob: []byte("amitiax-package-blob"),
				UserID:      "user-1",
				ScopeType:   "global",
			},
		},
	}

	service, err := NewMigrationService(source, target)
	require.NoError(t, err)

	return service
}

func defaultFakePreview() kernel.InstallPreview {
	return kernel.InstallPreview{
		SessionID:             "session-1",
		ArtifactID:            "artifact-1",
		ExtensionID:           fakeLegacyExtensionID,
		Version:               "1.0.0",
		Installable:           true,
		RequiredConfirmations: nil,
		ArchiveHash:           "sha256:archive",
		ManifestHash:          "sha256:manifest",
		ArtifactHash:          "sha256:artifact",
		ContentTreeHash:       "sha256:content-tree",
	}
}

func defaultFakeVerifyResult() kernel.LegacyMigrationVerification {
	return kernel.LegacyMigrationVerification{
		InstalledVersion: "1.0.0",
		PackageID:        "artifact-1",
		FinalGatePassed:  true,
		VerificationHash: "sha256:verification",
	}
}

func defaultFakeInstallResult() kernel.KernelInstallResult {
	return kernel.KernelInstallResult{
		ExtensionID: fakeLegacyExtensionID,
		OperationID: "operation-1",
	}
}

func TestMigrationServiceFreshCandidateCompletes(t *testing.T) {
	target := &fakeMigrationTarget{
		preview:       defaultFakePreview(),
		installResult: defaultFakeInstallResult(),
		verifyResult:  defaultFakeVerifyResult(),
	}

	service := newMigrationServiceTestFixture(t, target)

	err := service.MigrateOne(
		context.Background(),
		LegacyPackageCandidate{
			ExtensionID: fakeLegacyExtensionID,
			Version:     "1.0.0",
			PackageBlob: []byte("amitiax-package-blob"),
			UserID:      "user-1",
			ScopeType:   "global",
		},
	)
	require.NoError(t, err)

	require.Equal(t, 1, target.AcquireCalls)
	require.Equal(t, 1, target.PreviewCalls)
	require.Equal(t, 1, target.InstallCalls)
	require.Equal(t, 1, target.VerifyCalls)
	require.Equal(t, 1, target.CompleteCalls)
	require.Equal(t, 0, target.ReleaseCalls)
	require.Equal(t, kernel.LegacyMigrationStateCompleted, target.state)
}

func TestMigrationServiceResumesVerifyingCheckpoint(t *testing.T) {
	target := &fakeMigrationTarget{
		verifyResult: defaultFakeVerifyResult(),
		presetCheckpoint: &kernel.LegacyMigrationCheckpoint{
			ExtensionID:    fakeLegacyExtensionID,
			SourceHash:     hashBytes([]byte("amitiax-package-blob")),
			PreviewHash:    "sha256:preview",
			ArtifactID:     "artifact-1",
			OperationID:    "operation-1",
			State:          kernel.LegacyMigrationStateVerifying,
			CurrentStep:    "verify_kernel_install",
			LeaseOwner:     "worker-a",
			FencingToken:   1,
			LeaseExpiresAt: time.Now().Add(5 * time.Minute).Format(time.RFC3339Nano),
		},
	}

	service := newMigrationServiceTestFixture(t, target)

	err := service.MigrateOne(
		context.Background(),
		LegacyPackageCandidate{
			ExtensionID: fakeLegacyExtensionID,
			Version:     "1.0.0",
			PackageBlob: []byte("amitiax-package-blob"),
			UserID:      "user-1",
			ScopeType:   "global",
		},
	)
	require.NoError(t, err)

	require.Equal(t, 0, target.PreviewCalls)
	require.Equal(t, 0, target.InstallCalls)
	require.Equal(t, 1, target.VerifyCalls)
	require.Equal(t, 1, target.CompleteCalls)
	require.Equal(t, 0, target.ReleaseCalls)
	require.Equal(t, kernel.LegacyMigrationStateCompleted, target.state)
}

func TestMigrationServiceResumesMigratingWithOperation(t *testing.T) {
	target := &fakeMigrationTarget{
		verifyResult: defaultFakeVerifyResult(),
		presetCheckpoint: &kernel.LegacyMigrationCheckpoint{
			ExtensionID:      fakeLegacyExtensionID,
			SourceHash:       hashBytes([]byte("amitiax-package-blob")),
			PreviewHash:      "sha256:preview",
			PreviewSessionID: "session-1",
			ArtifactID:       "artifact-1",
			OperationID:      "operation-1",
			State:            kernel.LegacyMigrationStateMigrating,
			CurrentStep:      "kernel_install",
			LeaseOwner:       "worker-a",
			FencingToken:     1,
			LeaseExpiresAt:   time.Now().Add(5 * time.Minute).Format(time.RFC3339Nano),
		},
	}

	service := newMigrationServiceTestFixture(t, target)

	err := service.MigrateOne(
		context.Background(),
		LegacyPackageCandidate{
			ExtensionID: fakeLegacyExtensionID,
			Version:     "1.0.0",
			PackageBlob: []byte("amitiax-package-blob"),
			UserID:      "user-1",
			ScopeType:   "global",
		},
	)
	require.NoError(t, err)

	require.Equal(t, 0, target.PreviewCalls)
	require.Equal(t, 0, target.InstallCalls)
	require.Equal(t, 1, target.VerifyCalls)
	require.Equal(t, 1, target.CompleteCalls)
	require.Equal(t, 0, target.ReleaseCalls)
	require.Equal(t, kernel.LegacyMigrationStateCompleted, target.state)
}

func TestMigrationServiceRepreviewsPreviewedCheckpoint(t *testing.T) {
	preview := defaultFakePreview()

	target := &fakeMigrationTarget{
		preview:       preview,
		installResult: defaultFakeInstallResult(),
		verifyResult:  defaultFakeVerifyResult(),
		presetCheckpoint: &kernel.LegacyMigrationCheckpoint{
			ExtensionID:      fakeLegacyExtensionID,
			SourceHash:       hashBytes([]byte("amitiax-package-blob")),
			PreviewHash:      hashPreview(preview),
			PreviewSessionID: preview.SessionID,
			ArtifactID:       preview.ArtifactID,
			State:            kernel.LegacyMigrationStatePreviewed,
			CurrentStep:      "previewed",
			LeaseOwner:       "worker-a",
			FencingToken:     1,
			LeaseExpiresAt:   time.Now().Add(5 * time.Minute).Format(time.RFC3339Nano),
		},
	}

	service := newMigrationServiceTestFixture(t, target)

	err := service.MigrateOne(
		context.Background(),
		LegacyPackageCandidate{
			ExtensionID: fakeLegacyExtensionID,
			Version:     "1.0.0",
			PackageBlob: []byte("amitiax-package-blob"),
			UserID:      "user-1",
			ScopeType:   "global",
		},
	)
	require.NoError(t, err)

	require.Equal(t, 1, target.PreviewCalls)
	require.Equal(t, 1, target.InstallCalls)
	require.Equal(t, 1, target.VerifyCalls)
	require.Equal(t, 1, target.CompleteCalls)
	require.Equal(t, 0, target.ReleaseCalls)
	require.Equal(t, kernel.LegacyMigrationStateCompleted, target.state)
}

func TestMigrationServiceRejectsPreviewHashDrift(t *testing.T) {
	preview := defaultFakePreview()

	target := &fakeMigrationTarget{
		preview:       preview,
		installResult: defaultFakeInstallResult(),
		verifyResult:  defaultFakeVerifyResult(),
		presetCheckpoint: &kernel.LegacyMigrationCheckpoint{
			ExtensionID:      fakeLegacyExtensionID,
			SourceHash:       hashBytes([]byte("amitiax-package-blob")),
			PreviewHash:      "sha256:stale-preview-hash",
			PreviewSessionID: "session-stale",
			ArtifactID:       "artifact-stale",
			State:            kernel.LegacyMigrationStatePreviewed,
			CurrentStep:      "previewed",
			LeaseOwner:       "worker-a",
			FencingToken:     1,
			LeaseExpiresAt:   time.Now().Add(5 * time.Minute).Format(time.RFC3339Nano),
		},
	}

	service := newMigrationServiceTestFixture(t, target)

	err := service.MigrateOne(
		context.Background(),
		LegacyPackageCandidate{
			ExtensionID: fakeLegacyExtensionID,
			Version:     "1.0.0",
			PackageBlob: []byte("amitiax-package-blob"),
			UserID:      "user-1",
			ScopeType:   "global",
		},
	)
	require.NoError(t, err)

	require.Equal(t, kernel.LegacyMigrationStateBlocked, target.state)
	require.Equal(t, "preview_hash_changed", target.lastUpdate.CurrentStep)
	require.Equal(t, 1, target.PreviewCalls)
	require.Equal(t, 0, target.InstallCalls)
	require.Equal(t, 0, target.VerifyCalls)
	require.Equal(t, 0, target.CompleteCalls)
	require.Equal(t, 1, target.ReleaseCalls)
}

func TestMigrationServiceExistingInstallWithoutOperationRequiresManual(t *testing.T) {
	target := &fakeMigrationTarget{
		exists:        true,
		artifactID:    "artifact-existing",
		preview:       defaultFakePreview(),
		installResult: defaultFakeInstallResult(),
		verifyResult:  defaultFakeVerifyResult(),
	}

	service := newMigrationServiceTestFixture(t, target)

	err := service.MigrateOne(
		context.Background(),
		LegacyPackageCandidate{
			ExtensionID: fakeLegacyExtensionID,
			Version:     "1.0.0",
			PackageBlob: []byte("amitiax-package-blob"),
			UserID:      "user-1",
			ScopeType:   "global",
		},
	)
	require.NoError(t, err)

	require.Equal(t, kernel.LegacyMigrationStateManualRequired, target.state)
	require.Equal(t, "existing_installation_without_migration_operation", target.lastUpdate.CurrentStep)
	require.Equal(t, "artifact-existing", target.lastUpdate.ArtifactID)
	require.Equal(t, 0, target.PreviewCalls)
	require.Equal(t, 0, target.InstallCalls)
	require.Equal(t, 0, target.VerifyCalls)
	require.Equal(t, 0, target.CompleteCalls)
	require.Equal(t, 1, target.ReleaseCalls)
}

func TestMigrationServiceConcurrentLeaseFencing(t *testing.T) {
	target := &fakeMigrationTarget{
		acquireErr:    kernel.ErrLegacyMigrationLeaseHeld,
		preview:       defaultFakePreview(),
		installResult: defaultFakeInstallResult(),
		verifyResult:  defaultFakeVerifyResult(),
	}

	service := newMigrationServiceTestFixture(t, target)

	err := service.MigrateOne(
		context.Background(),
		LegacyPackageCandidate{
			ExtensionID: fakeLegacyExtensionID,
			Version:     "1.0.0",
			PackageBlob: []byte("amitiax-package-blob"),
			UserID:      "user-1",
			ScopeType:   "global",
		},
	)
	require.ErrorIs(t, err, kernel.ErrLegacyMigrationLeaseHeld)

	require.Equal(t, 1, target.AcquireCalls)
	require.Equal(t, 0, target.PreviewCalls)
	require.Equal(t, 0, target.InstallCalls)
	require.Equal(t, 0, target.VerifyCalls)
	require.Equal(t, 0, target.CompleteCalls)
}

func TestMigrationServiceNeverCompletesWithoutFinalGate(t *testing.T) {
	target := &fakeMigrationTarget{
		preview:       defaultFakePreview(),
		installResult: defaultFakeInstallResult(),
		verifyResult: kernel.LegacyMigrationVerification{
			FinalGatePassed: false,
		},
	}

	service := newMigrationServiceTestFixture(t, target)

	err := service.MigrateOne(
		context.Background(),
		LegacyPackageCandidate{
			ExtensionID: fakeLegacyExtensionID,
			Version:     "1.0.0",
			PackageBlob: []byte("amitiax-package-blob"),
			UserID:      "user-1",
			ScopeType:   "global",
		},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "final gate was not passed")

	require.Equal(t, 0, target.CompleteCalls)
	require.Equal(t, 1, target.VerifyCalls)
}
