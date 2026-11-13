package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/desktoppet/installation/coordinator"
	"github.com/u-ai/backend/internal/desktoppet/integration"
	"github.com/u-ai/backend/internal/desktoppet/release"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeOwnershipReader struct {
	identities map[string]*release.PetIdentityData
	releases   map[string]*release.ReleaseData
	err        error
}

func (f *fakeOwnershipReader) GetPetIdentity(petID string) (*release.PetIdentityData, error) {
	if f.err != nil {
		return nil, f.err
	}
	if identity, ok := f.identities[petID]; ok {
		return identity, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeOwnershipReader) GetRelease(releaseID string) (*release.ReleaseData, error) {
	if f.err != nil {
		return nil, f.err
	}
	if data, ok := f.releases[releaseID]; ok {
		return data, nil
	}
	return nil, gorm.ErrRecordNotFound
}

type fakeInstallationUninstaller struct {
	requests []coordinator.UninstallRequest
	err      error
}

func (f *fakeInstallationUninstaller) Uninstall(_ context.Context, req coordinator.UninstallRequest) (*coordinator.UninstallResult, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return &coordinator.UninstallResult{Status: "failed", ErrorMessage: f.err.Error()}, f.err
	}
	return &coordinator.UninstallResult{OperationID: "op-" + req.InstallationID, Status: "waiting_runtime_ack"}, nil
}

func newCleanupTestRepository(t *testing.T, installations []installation.Installation) installation.Repository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "petx-cleanup.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&installation.Installation{}); err != nil {
		t.Fatalf("migrate installation schema: %v", err)
	}
	for _, ins := range installations {
		row := ins
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("create installation %s: %v", ins.ID, err)
		}
	}
	return installation.NewRepository(db, nil)
}

func TestPetxInstallationCleanupUninstallsOwnedInstallations(t *testing.T) {
	repo := newCleanupTestRepository(t, []installation.Installation{
		{ID: "ins-owned", SpaceID: "space-one", DeviceID: "device-one", PetID: "pet-owned", CurrentReleaseID: "rel-owned"},
		{ID: "ins-other", SpaceID: "space-one", DeviceID: "device-one", PetID: "pet-other", CurrentReleaseID: "rel-other"},
		{ID: "ins-plain", SpaceID: "space-one", DeviceID: "device-one", PetID: "pet-plain", CurrentReleaseID: "rel-plain"},
		{ID: "ins-other-space", SpaceID: "space-two", DeviceID: "device-two", PetID: "pet-owned", CurrentReleaseID: "rel-owned"},
	})
	reader := &fakeOwnershipReader{
		identities: map[string]*release.PetIdentityData{
			"pet-owned": {ID: "pet-owned", UpstreamPetID: "com.amitia/desktop-pet"},
			"pet-other": {ID: "pet-other", UpstreamPetID: "com.amitia/other-plugin"},
			"pet-plain": {ID: "pet-plain"},
		},
		releases: map[string]*release.ReleaseData{
			"rel-owned": {ID: "rel-owned", SourceType: petxPluginReleaseSourceType, SourceProcessingTask: "com.amitia/desktop-pet"},
			"rel-other": {ID: "rel-other", SourceType: petxPluginReleaseSourceType, SourceProcessingTask: "com.amitia/other-plugin"},
			"rel-plain": {ID: "rel-plain", SourceType: "local_asset"},
		},
	}
	uninstaller := &fakeInstallationUninstaller{}
	cleanup := newPetxInstallationCleanup(repo, reader, uninstaller, "space-two")

	if err := cleanup.CleanupOwnedInstallations(context.Background(), integration.OwnedInstallationCleanupRequest{ExtensionID: "com.amitia/desktop-pet", SpaceID: "space-one"}); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if len(uninstaller.requests) != 1 {
		t.Fatalf("uninstall requests=%d want=1", len(uninstaller.requests))
	}
	req := uninstaller.requests[0]
	if req.InstallationID != "ins-owned" || req.DeviceCtx.DeviceID != "device-one" || req.DeviceCtx.SpaceID != "space-one" {
		t.Fatalf("uninstall request=%+v", req)
	}
	if req.IdempotencyKey != "plugin-uninstall:com.amitia/desktop-pet:ins-owned" {
		t.Fatalf("idempotency key=%q", req.IdempotencyKey)
	}
	if req.Metadata["reason"] != "plugin_uninstalled" {
		t.Fatalf("metadata=%v", req.Metadata)
	}
}

func TestPetxInstallationCleanupPreservesUninstallError(t *testing.T) {
	repo := newCleanupTestRepository(t, []installation.Installation{
		{ID: "ins-owned", SpaceID: "space-one", DeviceID: "device-one", PetID: "pet-owned", CurrentReleaseID: "rel-owned"},
	})
	reader := &fakeOwnershipReader{
		identities: map[string]*release.PetIdentityData{
			"pet-owned": {ID: "pet-owned", UpstreamPetID: "com.amitia/desktop-pet"},
		},
		releases: map[string]*release.ReleaseData{
			"rel-owned": {ID: "rel-owned", SourceType: petxPluginReleaseSourceType, SourceProcessingTask: "com.amitia/desktop-pet"},
		},
	}
	uninstallErr := errors.New("create durable command: durable command idempotency conflict: key=desired:device-one:12")
	cleanup := newPetxInstallationCleanup(repo, reader, &fakeInstallationUninstaller{err: uninstallErr}, "space-one")

	err := cleanup.CleanupOwnedInstallations(context.Background(), integration.OwnedInstallationCleanupRequest{ExtensionID: "com.amitia/desktop-pet"})
	if err == nil {
		t.Fatal("expected uninstall error")
	}
	if !errors.Is(err, uninstallErr) {
		t.Fatalf("original error chain lost: %v", err)
	}
}

func TestPetxInstallationCleanupRejectsMissingOwnershipEvidence(t *testing.T) {
	repo := newCleanupTestRepository(t, []installation.Installation{
		{ID: "ins-owned", SpaceID: "space-one", DeviceID: "device-one", PetID: "pet-owned", CurrentReleaseID: "rel-missing"},
	})
	reader := &fakeOwnershipReader{
		identities: map[string]*release.PetIdentityData{
			"pet-owned": {ID: "pet-owned", UpstreamPetID: "com.amitia/desktop-pet"},
		},
		releases: map[string]*release.ReleaseData{},
	}
	uninstaller := &fakeInstallationUninstaller{}
	cleanup := newPetxInstallationCleanup(repo, reader, uninstaller, "space-one")

	err := cleanup.CleanupOwnedInstallations(context.Background(), integration.OwnedInstallationCleanupRequest{ExtensionID: "com.amitia/desktop-pet"})
	if err == nil {
		t.Fatal("expected ownership evidence error")
	}
	if !strings.Contains(err.Error(), "rel-missing") || !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("error must keep release id and cause: %v", err)
	}
	if len(uninstaller.requests) != 0 {
		t.Fatalf("uninstall must not run without ownership evidence, requests=%d", len(uninstaller.requests))
	}
}
