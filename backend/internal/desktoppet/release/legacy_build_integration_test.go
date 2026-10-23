package release_test

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	"github.com/u-ai/backend/internal/desktoppet/release"
	releaserepo "github.com/u-ai/backend/internal/desktoppet/release/repository"
	releasestorage "github.com/u-ai/backend/internal/desktoppet/release/storage"
	"gorm.io/gorm"
)

func TestLegacyPackageMigrationProducesRuntimeReadyManifest(t *testing.T) {
	dataDir := t.TempDir()
	packageDir := filepath.Join(dataDir, "legacy-package")
	actionDir := filepath.Join(packageDir, "actions", "idle_normal")
	frameDir := filepath.Join(actionDir, "frames")
	if err := os.MkdirAll(frameDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestPNG(t, filepath.Join(packageDir, "preview.png"))
	writeTestPNG(t, filepath.Join(frameDir, "frame-0001.png"))
	action := map[string]any{
		"key":             "idle_normal",
		"name":            "待机",
		"loopType":        "loop",
		"playbackMode":    "loop",
		"fps":             12,
		"frameDurationMs": 83,
		"frames": []map[string]any{{
			"index":      0,
			"file":       "frames/frame-0001.png",
			"durationMs": 83,
		}},
		"anchor":       map[string]any{"x": 0.5, "y": 1},
		"returnAction": "",
		"returnPolicy": "none",
	}
	writeJSON(t, filepath.Join(actionDir, "action.json"), action)
	manifestPath := filepath.Join(packageDir, "manifest.json")
	writeJSON(t, manifestPath, map[string]any{
		"schemaVersion":    1,
		"packageId":        "legacy-package",
		"name":             "迁移桌宠",
		"defaultAction":    "idle_normal",
		"preview":          "preview.png",
		"canvas":           map[string]any{"width": 512, "height": 512},
		"generationTaskId": "legacy-task",
		"actions": []map[string]any{{
			"key":    "idle_normal",
			"name":   "待机",
			"config": "actions/idle_normal/action.json",
		}},
	})

	db, err := gorm.Open(sqlite.Open(filepath.Join(dataDir, "release.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&release.ReleaseBuildSnapshot{},
		&release.ReleaseBuildOperation{},
		&release.ReleaseData{},
		&release.ReleaseFileData{},
		&release.ReleaseEventOutbox{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE desktop_pet_identities (
		id TEXT PRIMARY KEY,
		owner_space_id TEXT,
		name TEXT,
		slug TEXT,
		upstream_pet_id TEXT,
		default_action_key TEXT,
		next_release_sequence INTEGER DEFAULT 0,
		created_at TEXT,
		updated_at TEXT
	)`).Error; err != nil {
		t.Fatal(err)
	}
	repo := releaserepo.NewSQLiteRepository(db)
	storage := releasestorage.NewFileSystemStorage(dataDir)
	if err := storage.Validate(); err != nil {
		t.Fatal(err)
	}
	svc := release.NewReleaseService(repo, nil, storage, nil, nil)
	port, ok := svc.(release.LegacyPackageBuildPort)
	if !ok {
		t.Fatal("release service does not implement legacy package build port")
	}

	result, err := port.MigrateLegacyPackage(context.Background(), release.LegacyPackageBuildRequest{
		SpaceID:         "space-test",
		LegacyPackageID: "legacy-package",
		PackageName:     "迁移桌宠",
		PackageDir:      packageDir,
		ManifestPath:    manifestPath,
		Version:         1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Release == nil || result.Release.Lifecycle != string(release.ReleaseLifecycleReady) {
		t.Fatalf("release result=%+v", result)
	}
	var manifest packageformat.Manifest
	if err := json.Unmarshal([]byte(result.Release.ManifestJSON), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.License.NoticePath != "NOTICE.txt" {
		t.Fatalf("notice path=%q", manifest.License.NoticePath)
	}
	publishedDir, err := storage.PublishedDir(result.PetID, result.Release.ID)
	if err != nil {
		t.Fatal(err)
	}
	noticeData, err := os.ReadFile(filepath.Join(publishedDir, manifest.License.NoticePath))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(noticeData), "SPDX-License-Identifier: AGPL-3.0-only") {
		t.Fatalf("notice content=%q", noticeData)
	}
	files, err := repo.GetReleaseFiles(result.Release.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundNotice := false
	for _, file := range files {
		if file.Path == manifest.License.NoticePath {
			foundNotice = true
			break
		}
	}
	if !foundNotice {
		t.Fatalf("release files missing %s", manifest.License.NoticePath)
	}
}

func writeTestPNG(t *testing.T, target string) {
	t.Helper()
	imageData := image.NewRGBA(image.Rect(0, 0, 512, 512))
	imageData.Set(0, 0, color.Transparent)
	file, err := os.Create(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, imageData); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeJSON(t *testing.T, target string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
