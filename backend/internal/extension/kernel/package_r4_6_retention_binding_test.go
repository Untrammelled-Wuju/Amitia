package kernel

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
)

func validR46RollbackPoint() PackageRollbackPoint {
	now := time.Now().UTC()
	retentionUntil := now.Add(30 * 24 * time.Hour).Format(time.RFC3339Nano)
	point := PackageRollbackPoint{
		RollbackPointID:            "rollback-point-r46",
		ExtensionID:                "com.example/r46",
		SourceVersion:              "1.0.0",
		SourceGeneration:           1,
		SourceVersionID:            "version-r46",
		SourceGenerationID:         "generation-r46",
		SnapshotID:                 "snapshot-r46",
		ArtifactID:                 "artifact-r46",
		ConfigSnapshotID:           "config-snapshot-r46",
		DefinitionSnapshotJSON:     `{"id":"com.example/r46"}`,
		ModuleSnapshotJSON:         `[]`,
		ContributionSnapshotJSON:   `[]`,
		PermissionSnapshotJSON:     `{}`,
		ScopeSnapshotJSON:          `[]`,
		ConfigSnapshotJSON:         `{}`,
		SecretRefsJSON:             `[]`,
		ResourceSnapshotJSON:       `{"entries":[]}`,
		MigrationStateSnapshotJSON: `{"mode":"none"}`,
		UserDataMigrationStateJSON: `{"tables":[]}`,
		RetentionState:             "active",
		RetentionUntil:             retentionUntil,
		ExpiresAt:                  retentionUntil,
		InstalledPath:              "/tmp/r46",
		CreatedAt:                  now.Format(time.RFC3339Nano),
	}
	hash, _ := computePackageSnapshotHash(point)
	point.SnapshotHash = hash
	return point
}

func r46VersionRecord() PackageVersionRecord {
	return PackageVersionRecord{
		VersionID:         "version-r46",
		ExtensionID:       "com.example/r46",
		Version:           "1.0.0",
		ArtifactID:        "artifact-r46",
		VersionState:      "current",
		InstalledPath:     "/tmp/r46",
		InstalledTreeHash: "tree-hash-r46",
		GenerationID:      "generation-r46",
	}
}

func r46Artifact() PackageArtifact {
	return PackageArtifact{
		ArtifactID:     "artifact-r46",
		ExtensionID:    "com.example/r46",
		Version:        "1.0.0",
		RetentionState: "active",
	}
}

func setupR46Runtime(t *testing.T) (*Runtime, *Container, context.Context) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	container, err := NewContainerBuilder().WithDBPath(filepath.Join(root, "kernel.db")).WithExtensionRoot(filepath.Join(root, "extensions")).Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Close() })
	runtime, err := NewRuntime(filepath.Join(root, "extensions"))
	if err != nil {
		t.Fatal(err)
	}
	runtime.SetContainer(container)
	return runtime, container, ctx
}

func putR46RollbackPoint(t *testing.T, ctx context.Context, container *Container, point PackageRollbackPoint) {
	t.Helper()
	if err := container.PackageRepository.PutRollbackPoint(ctx, point); err != nil {
		t.Fatalf("put rollback point: %v", err)
	}
}

func putR46Artifact(t *testing.T, ctx context.Context, container *Container, artifact PackageArtifact) {
	t.Helper()
	if err := container.PackageRepository.PutArtifact(ctx, artifact); err != nil {
		t.Fatalf("put artifact: %v", err)
	}
}

func putR46VersionRecord(t *testing.T, ctx context.Context, container *Container, record PackageVersionRecord) {
	t.Helper()
	record.ManifestHash = "manifest-hash-" + record.VersionID
	record.ContentTreeHash = "content-tree-hash-" + record.VersionID
	record.ArchiveHash = "archive-hash-" + record.VersionID
	if record.InstallOperationID == "" {
		record.InstallOperationID = "install-op-r46"
	}
	if err := container.PackageRepository.PutPackageVersion(ctx, record); err != nil {
		t.Fatalf("put version record: %v", err)
	}
}

func TestR46SnapshotHashBindsRollbackPointID(t *testing.T) {
	base := validR46RollbackPoint()
	baseHash, err := computePackageSnapshotHash(base)
	if err != nil {
		t.Fatal(err)
	}
	changed := base
	changed.RollbackPointID = "rollback-point-tampered"
	changedHash, err := computePackageSnapshotHash(changed)
	if err != nil {
		t.Fatal(err)
	}
	if baseHash == changedHash {
		t.Fatal("RollbackPointID drift must change SnapshotHash")
	}
}

func TestR46SnapshotHashBindsSourceVersionID(t *testing.T) {
	base := validR46RollbackPoint()
	baseHash, err := computePackageSnapshotHash(base)
	if err != nil {
		t.Fatal(err)
	}
	changed := base
	changed.SourceVersionID = "version-tampered"
	changedHash, err := computePackageSnapshotHash(changed)
	if err != nil {
		t.Fatal(err)
	}
	if baseHash == changedHash {
		t.Fatal("SourceVersionID drift must change SnapshotHash")
	}
}

func TestR46SnapshotHashBindsSourceGenerationID(t *testing.T) {
	base := validR46RollbackPoint()
	baseHash, err := computePackageSnapshotHash(base)
	if err != nil {
		t.Fatal(err)
	}
	changed := base
	changed.SourceGenerationID = "generation-tampered"
	changedHash, err := computePackageSnapshotHash(changed)
	if err != nil {
		t.Fatal(err)
	}
	if baseHash == changedHash {
		t.Fatal("SourceGenerationID drift must change SnapshotHash")
	}
}

func TestR46SnapshotHashBindsSnapshotID(t *testing.T) {
	base := validR46RollbackPoint()
	baseHash, err := computePackageSnapshotHash(base)
	if err != nil {
		t.Fatal(err)
	}
	changed := base
	changed.SnapshotID = "snapshot-tampered"
	changedHash, err := computePackageSnapshotHash(changed)
	if err != nil {
		t.Fatal(err)
	}
	if baseHash == changedHash {
		t.Fatal("SnapshotID drift must change SnapshotHash")
	}
}

func TestR46SnapshotHashBindsConfigSnapshotID(t *testing.T) {
	base := validR46RollbackPoint()
	baseHash, err := computePackageSnapshotHash(base)
	if err != nil {
		t.Fatal(err)
	}
	changed := base
	changed.ConfigSnapshotID = "config-snapshot-tampered"
	changedHash, err := computePackageSnapshotHash(changed)
	if err != nil {
		t.Fatal(err)
	}
	if baseHash == changedHash {
		t.Fatal("ConfigSnapshotID drift must change SnapshotHash")
	}
}

func TestR46SnapshotRejectsMissingSourceVersionID(t *testing.T) {
	point := validR46RollbackPoint()
	point.SourceVersionID = ""
	if err := validatePackageSnapshot(point); err == nil {
		t.Fatal("expected error for missing SourceVersionID")
	}
}

func TestR46SnapshotRejectsMissingSourceGenerationID(t *testing.T) {
	point := validR46RollbackPoint()
	point.SourceGenerationID = ""
	if err := validatePackageSnapshot(point); err == nil {
		t.Fatal("expected error for missing SourceGenerationID")
	}
}

func TestR46SnapshotRejectsMissingSnapshotID(t *testing.T) {
	point := validR46RollbackPoint()
	point.SnapshotID = ""
	if err := validatePackageSnapshot(point); err == nil {
		t.Fatal("expected error for missing SnapshotID")
	}
}

func TestR46SnapshotRejectsMissingConfigSnapshotID(t *testing.T) {
	point := validR46RollbackPoint()
	point.ConfigSnapshotID = ""
	if err := validatePackageSnapshot(point); err == nil {
		t.Fatal("expected error for missing ConfigSnapshotID")
	}
}

func TestR46SnapshotRejectsHashTampering(t *testing.T) {
	point := validR46RollbackPoint()
	point.SnapshotHash = "tampered-hash"
	if err := validatePackageSnapshot(point); err == nil {
		t.Fatal("expected error for hash tampering")
	}
}

func TestR46CreateRollbackPointPersistsVersionID(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	extensionID := "com.example/r46-create"
	versionID := "version-r46-create"
	artifactID := "artifact-r46-create"

	if err := container.ContributionRepository.PutContribution(ctx, domain.ContributionDefinition{
		ID:          "contrib-r46-create",
		ModuleID:    "module-r46-create",
		ExtensionID: domain.ExtensionID(extensionID),
		Kind:        domain.ContributionKindTool,
		Name:        domain.LocalizedText{Default: "R46 Create Tool"},
		Version:     "1.0.0",
		Definition:  map[string]any{"toolId": "r46-create-tool"},
	}); err != nil {
		t.Fatalf("put contribution: %v", err)
	}

	putR46Artifact(t, ctx, container, PackageArtifact{
		ArtifactID: artifactID, ExtensionID: extensionID, Version: "1.0.0", RetentionState: "active",
	})
	putR46VersionRecord(t, ctx, container, PackageVersionRecord{
		VersionID: versionID, ExtensionID: extensionID, Version: "1.0.0", ArtifactID: artifactID,
		VersionState: "current", GenerationID: "generation-r46-create",
	})

	installation := createTestInstallation(t, extensionID, "1.0.0", artifactID)
	installation.Metadata["generationId"] = "generation-r46-create"
	installation.Metadata["installedTreeHash"] = "tree-hash-r46-create"

	definition := &domain.ExtensionDefinition{
		ID:      domain.ExtensionID(extensionID),
		Version: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0},
	}

	point, err := runtime.createPackageRollbackPoint(ctx, "source-op-r46", "active", *installation, definition, nil, nil)
	if err != nil {
		t.Fatalf("create rollback point: %v", err)
	}

	if point.SourceVersionID != versionID {
		t.Fatalf("expected SourceVersionID=%s, got %s", versionID, point.SourceVersionID)
	}
}

func TestR46CreateRollbackPointPersistsGenerationID(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	extensionID := "com.example/r46-gen"
	generationID := "generation-r46-gen"
	artifactID := "artifact-r46-gen"

	if err := container.ContributionRepository.PutContribution(ctx, domain.ContributionDefinition{
		ID:          "contrib-r46-gen",
		ModuleID:    "module-r46-gen",
		ExtensionID: domain.ExtensionID(extensionID),
		Kind:        domain.ContributionKindTool,
		Name:        domain.LocalizedText{Default: "R46 Gen Tool"},
		Version:     "1.0.0",
		Definition:  map[string]any{"toolId": "r46-gen-tool"},
	}); err != nil {
		t.Fatalf("put contribution: %v", err)
	}

	putR46Artifact(t, ctx, container, PackageArtifact{
		ArtifactID: artifactID, ExtensionID: extensionID, Version: "1.0.0", RetentionState: "active",
	})
	putR46VersionRecord(t, ctx, container, PackageVersionRecord{
		VersionID: "version-r46-gen", ExtensionID: extensionID, Version: "1.0.0", ArtifactID: artifactID,
		VersionState: "current", GenerationID: generationID,
	})

	installation := createTestInstallation(t, extensionID, "1.0.0", artifactID)
	installation.Metadata["generationId"] = generationID
	installation.Metadata["installedTreeHash"] = "tree-hash-r46-gen"

	definition := &domain.ExtensionDefinition{
		ID:      domain.ExtensionID(extensionID),
		Version: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0},
	}

	point, err := runtime.createPackageRollbackPoint(ctx, "source-op-r46", "active", *installation, definition, nil, nil)
	if err != nil {
		t.Fatalf("create rollback point: %v", err)
	}

	if point.SourceGenerationID != generationID {
		t.Fatalf("expected SourceGenerationID=%s, got %s", generationID, point.SourceGenerationID)
	}
}

func TestR46CreateRollbackPointPersistsSnapshotID(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	extensionID := "com.example/r46-snap"
	artifactID := "artifact-r46-snap"

	if err := container.ContributionRepository.PutContribution(ctx, domain.ContributionDefinition{
		ID:          "contrib-r46-snap",
		ModuleID:    "module-r46-snap",
		ExtensionID: domain.ExtensionID(extensionID),
		Kind:        domain.ContributionKindTool,
		Name:        domain.LocalizedText{Default: "R46 Snap Tool"},
		Version:     "1.0.0",
		Definition:  map[string]any{"toolId": "r46-snap-tool"},
	}); err != nil {
		t.Fatalf("put contribution: %v", err)
	}

	putR46Artifact(t, ctx, container, PackageArtifact{
		ArtifactID: artifactID, ExtensionID: extensionID, Version: "1.0.0", RetentionState: "active",
	})
	putR46VersionRecord(t, ctx, container, PackageVersionRecord{
		VersionID: "version-r46-snap", ExtensionID: extensionID, Version: "1.0.0", ArtifactID: artifactID,
		VersionState: "current", GenerationID: "generation-r46-snap",
	})

	installation := createTestInstallation(t, extensionID, "1.0.0", artifactID)
	installation.Metadata["generationId"] = "generation-r46-snap"
	installation.Metadata["installedTreeHash"] = "tree-hash-r46-snap"

	definition := &domain.ExtensionDefinition{
		ID:      domain.ExtensionID(extensionID),
		Version: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0},
	}

	point, err := runtime.createPackageRollbackPoint(ctx, "source-op-r46", "active", *installation, definition, nil, nil)
	if err != nil {
		t.Fatalf("create rollback point: %v", err)
	}

	if point.SnapshotID == "" {
		t.Fatal("expected non-empty SnapshotID")
	}
}

func TestR46CreateRollbackPointRejectsVersionArtifactMismatch(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	extensionID := "com.example/r46-mismatch1"
	artifactID := "artifact-r46-mismatch1"
	differentArtifact := "artifact-different"

	putR46Artifact(t, ctx, container, PackageArtifact{
		ArtifactID: artifactID, ExtensionID: extensionID, Version: "1.0.0", RetentionState: "active",
	})
	putR46VersionRecord(t, ctx, container, PackageVersionRecord{
		VersionID: "version-r46-mismatch1", ExtensionID: extensionID, Version: "1.0.0", ArtifactID: differentArtifact,
		VersionState: "current", GenerationID: "generation-r46-mismatch1",
	})

	installation := createTestInstallation(t, extensionID, "1.0.0", artifactID)
	installation.Metadata["generationId"] = "generation-r46-mismatch1"
	installation.Metadata["installedTreeHash"] = "tree-hash-r46-mismatch1"

	definition := &domain.ExtensionDefinition{
		ID:      domain.ExtensionID(extensionID),
		Version: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0},
	}

	_, err := runtime.createPackageRollbackPoint(ctx, "source-op-r46", "active", *installation, definition, nil, nil)
	if err == nil {
		t.Fatal("expected error for version artifact mismatch")
	}
}

func TestR46CreateRollbackPointRejectsVersionGenerationMismatch(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	extensionID := "com.example/r46-mismatch2"
	artifactID := "artifact-r46-mismatch2"
	generationID := "generation-r46-mismatch2"
	differentGeneration := "generation-different"

	putR46Artifact(t, ctx, container, PackageArtifact{
		ArtifactID: artifactID, ExtensionID: extensionID, Version: "1.0.0", RetentionState: "active",
	})
	putR46VersionRecord(t, ctx, container, PackageVersionRecord{
		VersionID: "version-r46-mismatch2", ExtensionID: extensionID, Version: "1.0.0", ArtifactID: artifactID,
		VersionState: "current", GenerationID: differentGeneration,
	})

	installation := createTestInstallation(t, extensionID, "1.0.0", artifactID)
	installation.Metadata["generationId"] = generationID
	installation.Metadata["installedTreeHash"] = "tree-hash-r46-mismatch2"

	definition := &domain.ExtensionDefinition{
		ID:      domain.ExtensionID(extensionID),
		Version: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0},
	}

	_, err := runtime.createPackageRollbackPoint(ctx, "source-op-r46", "active", *installation, definition, nil, nil)
	if err == nil {
		t.Fatal("expected error for version generation mismatch")
	}
}

func TestR46CreateRollbackPointRejectsArtifactVersionMismatch(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	extensionID := "com.example/r46-mismatch3"
	artifactID := "artifact-r46-mismatch3"

	putR46Artifact(t, ctx, container, PackageArtifact{
		ArtifactID: artifactID, ExtensionID: extensionID, Version: "2.0.0", RetentionState: "active",
	})
	putR46VersionRecord(t, ctx, container, PackageVersionRecord{
		VersionID: "version-r46-mismatch3", ExtensionID: extensionID, Version: "1.0.0", ArtifactID: artifactID,
		VersionState: "current", GenerationID: "generation-r46-mismatch3",
	})

	installation := createTestInstallation(t, extensionID, "1.0.0", artifactID)
	installation.Metadata["generationId"] = "generation-r46-mismatch3"
	installation.Metadata["installedTreeHash"] = "tree-hash-r46-mismatch3"

	definition := &domain.ExtensionDefinition{
		ID:      domain.ExtensionID(extensionID),
		Version: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0},
	}

	_, err := runtime.createPackageRollbackPoint(ctx, "source-op-r46", "active", *installation, definition, nil, nil)
	if err == nil {
		t.Fatal("expected error for artifact version mismatch")
	}
}

func TestR46RetentionBindingPassesForExactIdentity(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	point := validR46RollbackPoint()
	putR46Artifact(t, ctx, container, r46Artifact())
	putR46RollbackPoint(t, ctx, container, point)
	putR46VersionRecord(t, ctx, container, r46VersionRecord())

	binding, err := runtime.verifyRollbackRetentionBinding(ctx, "com.example/r46", "1.0.0", "artifact-r46")
	if err != nil {
		t.Fatalf("expected binding to pass, got: %v", err)
	}
	if binding.RollbackPoint.RollbackPointID != "rollback-point-r46" {
		t.Fatal("wrong rollback point id")
	}
	if binding.VersionRecord.VersionID != "version-r46" {
		t.Fatal("wrong version id")
	}
	if binding.Reference.ReferenceOwnerID != "rollback-point-r46" {
		t.Fatal("wrong reference owner id")
	}
}

func TestR46RetentionBindingRejectsSourceVersionIDMismatch(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	point := validR46RollbackPoint()
	putR46Artifact(t, ctx, container, r46Artifact())
	putR46RollbackPoint(t, ctx, container, point)
	putR46VersionRecord(t, ctx, container, PackageVersionRecord{
		VersionID: "version-different", ExtensionID: "com.example/r46", Version: "1.0.0",
		ArtifactID: "artifact-r46", VersionState: "current", GenerationID: "generation-r46",
	})

	_, err := runtime.verifyRollbackRetentionBinding(ctx, "com.example/r46", "1.0.0", "artifact-r46")
	if err == nil {
		t.Fatal("expected rejection for version id mismatch")
	}
}

func TestR46RetentionBindingRejectsVersionLabelMismatch(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	point := validR46RollbackPoint()
	putR46Artifact(t, ctx, container, r46Artifact())
	putR46RollbackPoint(t, ctx, container, point)
	putR46VersionRecord(t, ctx, container, PackageVersionRecord{
		VersionID: "version-r46", ExtensionID: "com.example/r46", Version: "2.0.0",
		ArtifactID: "artifact-r46", VersionState: "current", GenerationID: "generation-r46",
	})

	_, err := runtime.verifyRollbackRetentionBinding(ctx, "com.example/r46", "1.0.0", "artifact-r46")
	if err == nil {
		t.Fatal("expected rejection for version label mismatch")
	}
}

func TestR46RetentionBindingRejectsVersionArtifactMismatch(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	point := validR46RollbackPoint()
	putR46Artifact(t, ctx, container, r46Artifact())
	putR46RollbackPoint(t, ctx, container, point)
	putR46VersionRecord(t, ctx, container, PackageVersionRecord{
		VersionID: "version-r46", ExtensionID: "com.example/r46", Version: "1.0.0",
		ArtifactID: "artifact-different", VersionState: "current", GenerationID: "generation-r46",
	})

	_, err := runtime.verifyRollbackRetentionBinding(ctx, "com.example/r46", "1.0.0", "artifact-r46")
	if err == nil {
		t.Fatal("expected rejection for version artifact mismatch")
	}
}

func TestR46RetentionBindingRejectsVersionGenerationMismatch(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	point := validR46RollbackPoint()
	putR46Artifact(t, ctx, container, r46Artifact())
	putR46RollbackPoint(t, ctx, container, point)
	putR46VersionRecord(t, ctx, container, PackageVersionRecord{
		VersionID: "version-r46", ExtensionID: "com.example/r46", Version: "1.0.0",
		ArtifactID: "artifact-r46", VersionState: "current", GenerationID: "generation-different",
	})

	_, err := runtime.verifyRollbackRetentionBinding(ctx, "com.example/r46", "1.0.0", "artifact-r46")
	if err == nil {
		t.Fatal("expected rejection for version generation mismatch")
	}
}

func TestR46RetentionBindingRejectsMissingSnapshotID(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	point := validR46RollbackPoint()
	point.SnapshotID = ""
	point.SnapshotHash, _ = computePackageSnapshotHash(point)
	putR46Artifact(t, ctx, container, r46Artifact())
	putR46VersionRecord(t, ctx, container, r46VersionRecord())

	if _, err := container.PackageRepository.DB().ExecContext(ctx, `INSERT INTO extension_package_rollback_points (
		rollback_point_id, extension_id, source_version, source_generation, source_version_id,
		source_generation_id, snapshot_id, artifact_id,
		definition_snapshot_json, module_snapshot_json, contribution_snapshot_json,
		permission_snapshot_json, scope_snapshot_json, config_snapshot_id, config_snapshot_json,
		secret_refs_json, resource_snapshot_json, migration_state_snapshot_json,
		user_data_migration_state_json, snapshot_hash, retention_state, retention_until,
		source_operation_id, installed_path, created_at, expires_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		point.RollbackPointID, point.ExtensionID, point.SourceVersion, point.SourceGeneration,
		point.SourceVersionID, point.SourceGenerationID, point.SnapshotID, point.ArtifactID,
		point.DefinitionSnapshotJSON, point.ModuleSnapshotJSON, point.ContributionSnapshotJSON,
		point.PermissionSnapshotJSON, point.ScopeSnapshotJSON, point.ConfigSnapshotID, point.ConfigSnapshotJSON,
		point.SecretRefsJSON, point.ResourceSnapshotJSON, point.MigrationStateSnapshotJSON,
		point.UserDataMigrationStateJSON, point.SnapshotHash, point.RetentionState, point.RetentionUntil,
		point.SourceOperationID, point.InstalledPath, point.CreatedAt, point.ExpiresAt); err != nil {
		t.Fatalf("insert rollback point: %v", err)
	}

	_, err := runtime.verifyRollbackRetentionBinding(ctx, "com.example/r46", "1.0.0", "artifact-r46")
	if err == nil {
		t.Fatal("expected rejection for missing snapshot id")
	}
}

func TestR46RetentionBindingRejectsSnapshotHashMismatch(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	point := validR46RollbackPoint()
	point.SourceGenerationID = "generation-tampered"
	putR46Artifact(t, ctx, container, r46Artifact())
	putR46VersionRecord(t, ctx, container, r46VersionRecord())

	if _, err := container.PackageRepository.DB().ExecContext(ctx, `INSERT INTO extension_package_rollback_points (
		rollback_point_id, extension_id, source_version, source_generation, source_version_id,
		source_generation_id, snapshot_id, artifact_id,
		definition_snapshot_json, module_snapshot_json, contribution_snapshot_json,
		permission_snapshot_json, scope_snapshot_json, config_snapshot_id, config_snapshot_json,
		secret_refs_json, resource_snapshot_json, migration_state_snapshot_json,
		user_data_migration_state_json, snapshot_hash, retention_state, retention_until,
		source_operation_id, installed_path, created_at, expires_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		point.RollbackPointID, point.ExtensionID, point.SourceVersion, point.SourceGeneration,
		point.SourceVersionID, point.SourceGenerationID, point.SnapshotID, point.ArtifactID,
		point.DefinitionSnapshotJSON, point.ModuleSnapshotJSON, point.ContributionSnapshotJSON,
		point.PermissionSnapshotJSON, point.ScopeSnapshotJSON, point.ConfigSnapshotID, point.ConfigSnapshotJSON,
		point.SecretRefsJSON, point.ResourceSnapshotJSON, point.MigrationStateSnapshotJSON,
		point.UserDataMigrationStateJSON, point.SnapshotHash, point.RetentionState, point.RetentionUntil,
		point.SourceOperationID, point.InstalledPath, point.CreatedAt, point.ExpiresAt); err != nil {
		t.Fatalf("insert rollback point: %v", err)
	}

	_, err := runtime.verifyRollbackRetentionBinding(ctx, "com.example/r46", "1.0.0", "artifact-r46")
	if err == nil {
		t.Fatal("expected rejection for snapshot hash mismatch")
	}
}

func TestR46RetentionBindingRejectsWrongReferenceOwner(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	point := validR46RollbackPoint()
	point.RollbackPointID = "rollback-point-wrong-owner-r46"
	point.SnapshotHash, _ = computePackageSnapshotHash(point)
	putR46Artifact(t, ctx, container, r46Artifact())
	putR46VersionRecord(t, ctx, container, r46VersionRecord())

	if _, err := container.PackageRepository.DB().ExecContext(ctx, `INSERT INTO extension_package_rollback_points (
		rollback_point_id, extension_id, source_version, source_generation, source_version_id,
		source_generation_id, snapshot_id, artifact_id,
		definition_snapshot_json, module_snapshot_json, contribution_snapshot_json,
		permission_snapshot_json, scope_snapshot_json, config_snapshot_id, config_snapshot_json,
		secret_refs_json, resource_snapshot_json, migration_state_snapshot_json,
		user_data_migration_state_json, snapshot_hash, retention_state, retention_until,
		source_operation_id, installed_path, created_at, expires_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		point.RollbackPointID, point.ExtensionID, point.SourceVersion, point.SourceGeneration,
		point.SourceVersionID, point.SourceGenerationID, point.SnapshotID, point.ArtifactID,
		point.DefinitionSnapshotJSON, point.ModuleSnapshotJSON, point.ContributionSnapshotJSON,
		point.PermissionSnapshotJSON, point.ScopeSnapshotJSON, point.ConfigSnapshotID, point.ConfigSnapshotJSON,
		point.SecretRefsJSON, point.ResourceSnapshotJSON, point.MigrationStateSnapshotJSON,
		point.UserDataMigrationStateJSON, point.SnapshotHash, point.RetentionState, point.RetentionUntil,
		point.SourceOperationID, point.InstalledPath, point.CreatedAt, point.ExpiresAt); err != nil {
		t.Fatalf("insert rollback point: %v", err)
	}

	expiresAt, _ := time.Parse(time.RFC3339Nano, point.ExpiresAt)
	if _, err := container.PackageRepository.AcquireArtifactReference(ctx, "artifact-r46", ArtifactReferenceRollbackPoint, "com.example/r46", expiresAt); err != nil {
		t.Fatalf("create wrong-owner reference: %v", err)
	}

	_, err := runtime.verifyRollbackRetentionBinding(ctx, "com.example/r46", "1.0.0", "artifact-r46")
	if err == nil {
		t.Fatal("expected rejection for wrong reference owner")
	}
}

func TestR46RetentionBindingRejectsMissingReference(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	point := validR46RollbackPoint()
	point.RollbackPointID = "rollback-point-no-reference"
	point.SnapshotHash, _ = computePackageSnapshotHash(point)
	putR46Artifact(t, ctx, container, r46Artifact())
	putR46VersionRecord(t, ctx, container, r46VersionRecord())

	if _, err := container.PackageRepository.DB().ExecContext(ctx, `INSERT INTO extension_package_rollback_points (
		rollback_point_id, extension_id, source_version, source_generation, source_version_id,
		source_generation_id, snapshot_id, artifact_id,
		definition_snapshot_json, module_snapshot_json, contribution_snapshot_json,
		permission_snapshot_json, scope_snapshot_json, config_snapshot_id, config_snapshot_json,
		secret_refs_json, resource_snapshot_json, migration_state_snapshot_json,
		user_data_migration_state_json, snapshot_hash, retention_state, retention_until,
		source_operation_id, installed_path, created_at, expires_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		point.RollbackPointID, point.ExtensionID, point.SourceVersion, point.SourceGeneration,
		point.SourceVersionID, point.SourceGenerationID, point.SnapshotID, point.ArtifactID,
		point.DefinitionSnapshotJSON, point.ModuleSnapshotJSON, point.ContributionSnapshotJSON,
		point.PermissionSnapshotJSON, point.ScopeSnapshotJSON, point.ConfigSnapshotID, point.ConfigSnapshotJSON,
		point.SecretRefsJSON, point.ResourceSnapshotJSON, point.MigrationStateSnapshotJSON,
		point.UserDataMigrationStateJSON, point.SnapshotHash, point.RetentionState, point.RetentionUntil,
		point.SourceOperationID, point.InstalledPath, point.CreatedAt, point.ExpiresAt); err != nil {
		t.Fatalf("insert rollback point without reference: %v", err)
	}

	_, err := runtime.verifyRollbackRetentionBinding(ctx, "com.example/r46", "1.0.0", "artifact-r46")
	if err == nil {
		t.Fatal("expected rejection for missing reference")
	}
}

func TestR46RetentionBindingRejectsDuplicateReference(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	point := validR46RollbackPoint()
	putR46Artifact(t, ctx, container, r46Artifact())
	putR46RollbackPoint(t, ctx, container, point)
	putR46VersionRecord(t, ctx, container, r46VersionRecord())

	pastExpiry := time.Now().UTC().Add(-1 * time.Hour)
	if _, err := container.PackageRepository.AcquireArtifactReference(ctx, "artifact-r46", ArtifactReferenceRollbackPoint, point.RollbackPointID, pastExpiry); err != nil {
		t.Fatalf("duplicate reference: %v", err)
	}

	_, err := runtime.verifyRollbackRetentionBinding(ctx, "com.example/r46", "1.0.0", "artifact-r46")
	if err == nil {
		t.Fatal("expected rejection for duplicate reference")
	}
}

func TestR46RetentionBindingRejectsReleasedReference(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	point := validR46RollbackPoint()
	putR46Artifact(t, ctx, container, r46Artifact())
	putR46RollbackPoint(t, ctx, container, point)
	putR46VersionRecord(t, ctx, container, r46VersionRecord())

	if err := container.PackageRepository.ReleaseArtifactReference(ctx, "artifact-r46", ArtifactReferenceRollbackPoint, point.RollbackPointID); err != nil {
		t.Fatalf("release reference: %v", err)
	}

	_, err := runtime.verifyRollbackRetentionBinding(ctx, "com.example/r46", "1.0.0", "artifact-r46")
	if err == nil {
		t.Fatal("expected rejection for released reference")
	}
}

func TestR46RetentionBindingRejectsExpiredReference(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	point := validR46RollbackPoint()
	point.RetentionUntil = time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339Nano)
	point.ExpiresAt = point.RetentionUntil
	point.SnapshotHash, _ = computePackageSnapshotHash(point)
	putR46Artifact(t, ctx, container, r46Artifact())
	putR46VersionRecord(t, ctx, container, r46VersionRecord())

	if _, err := container.PackageRepository.DB().ExecContext(ctx, `INSERT INTO extension_package_rollback_points (
		rollback_point_id, extension_id, source_version, source_generation, source_version_id,
		source_generation_id, snapshot_id, artifact_id,
		definition_snapshot_json, module_snapshot_json, contribution_snapshot_json,
		permission_snapshot_json, scope_snapshot_json, config_snapshot_id, config_snapshot_json,
		secret_refs_json, resource_snapshot_json, migration_state_snapshot_json,
		user_data_migration_state_json, snapshot_hash, retention_state, retention_until,
		source_operation_id, installed_path, created_at, expires_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		point.RollbackPointID, point.ExtensionID, point.SourceVersion, point.SourceGeneration,
		point.SourceVersionID, point.SourceGenerationID, point.SnapshotID, point.ArtifactID,
		point.DefinitionSnapshotJSON, point.ModuleSnapshotJSON, point.ContributionSnapshotJSON,
		point.PermissionSnapshotJSON, point.ScopeSnapshotJSON, point.ConfigSnapshotID, point.ConfigSnapshotJSON,
		point.SecretRefsJSON, point.ResourceSnapshotJSON, point.MigrationStateSnapshotJSON,
		point.UserDataMigrationStateJSON, point.SnapshotHash, point.RetentionState, point.RetentionUntil,
		point.SourceOperationID, point.InstalledPath, point.CreatedAt, point.ExpiresAt); err != nil {
		t.Fatalf("insert rollback point: %v", err)
	}

	_, err := runtime.verifyRollbackRetentionBinding(ctx, "com.example/r46", "1.0.0", "artifact-r46")
	if err == nil {
		t.Fatal("expected rejection for expired reference")
	}
}

func TestR46RetentionBindingRejectsReferenceExpirationDrift(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	point := validR46RollbackPoint()
	putR46Artifact(t, ctx, container, r46Artifact())
	putR46VersionRecord(t, ctx, container, r46VersionRecord())

	expiresAt := time.Now().UTC().Add(45 * 24 * time.Hour)
	point.ExpiresAt = point.RetentionUntil
	if _, err := container.PackageRepository.DB().ExecContext(ctx, `INSERT INTO extension_package_rollback_points (
		rollback_point_id, extension_id, source_version, source_generation, source_version_id,
		source_generation_id, snapshot_id, artifact_id,
		definition_snapshot_json, module_snapshot_json, contribution_snapshot_json,
		permission_snapshot_json, scope_snapshot_json, config_snapshot_id, config_snapshot_json,
		secret_refs_json, resource_snapshot_json, migration_state_snapshot_json,
		user_data_migration_state_json, snapshot_hash, retention_state, retention_until,
		source_operation_id, installed_path, created_at, expires_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		point.RollbackPointID, point.ExtensionID, point.SourceVersion, point.SourceGeneration,
		point.SourceVersionID, point.SourceGenerationID, point.SnapshotID, point.ArtifactID,
		point.DefinitionSnapshotJSON, point.ModuleSnapshotJSON, point.ContributionSnapshotJSON,
		point.PermissionSnapshotJSON, point.ScopeSnapshotJSON, point.ConfigSnapshotID, point.ConfigSnapshotJSON,
		point.SecretRefsJSON, point.ResourceSnapshotJSON, point.MigrationStateSnapshotJSON,
		point.UserDataMigrationStateJSON, point.SnapshotHash, point.RetentionState, point.RetentionUntil,
		point.SourceOperationID, point.InstalledPath, point.CreatedAt, point.ExpiresAt); err != nil {
		t.Fatalf("insert rollback point: %v", err)
	}
	if _, err := container.PackageRepository.AcquireArtifactReference(ctx, "artifact-r46", ArtifactReferenceRollbackPoint, point.RollbackPointID, expiresAt); err != nil {
		t.Fatalf("create reference with different expiration: %v", err)
	}

	_, err := runtime.verifyRollbackRetentionBinding(ctx, "com.example/r46", "1.0.0", "artifact-r46")
	if err == nil {
		t.Fatal("expected rejection for reference expiration drift")
	}
}

func TestR46UninstallPolicyUsesRollbackPointIDAsReferenceOwner(t *testing.T) {
	runtime, container, ctx := setupR46Runtime(t)
	point := validR46RollbackPoint()
	putR46Artifact(t, ctx, container, r46Artifact())
	putR46RollbackPoint(t, ctx, container, point)
	putR46VersionRecord(t, ctx, container, r46VersionRecord())

	policy, _, err := runtime.computeUninstallArtifactPolicy(ctx, "artifact-r46", "com.example/r46", "1.0.0")
	if err != nil {
		t.Fatalf("expected policy computation to succeed, got: %v", err)
	}
	if policy != ArtifactPolicyRetainForRollback {
		t.Fatalf("expected RetainForRollback, got %s", policy)
	}
}

func createTestInstallation(t *testing.T, extensionID, version, artifactID string) *domain.ExtensionInstallation {
	t.Helper()
	installation := &domain.ExtensionInstallation{
		InstallationID:    "installation-" + uuid.NewString(),
		ExtensionID:       domain.ExtensionID(extensionID),
		InstalledVersion:  domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0},
		PackageID:         artifactID,
		InstallationState: domain.InstallationStateInstalled,
		Generation:        1,
		Metadata: map[string]any{
			"installedPath":     "/tmp/" + extensionID,
			"artifactId":        artifactID,
			"installedTreeHash": "tree-hash-" + extensionID,
		},
	}
	return installation
}

func init() {
	_ = migration.MigrationDefinition{}
}
