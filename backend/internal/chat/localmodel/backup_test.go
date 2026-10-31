package localmodel

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
)

type testArchiveReader struct {
	reader *dataportability.ArchiveReader
}

type testBackupWriter struct {
	*dataportability.ArchiveWriter
}

func (testBackupWriter) WriteJSON(string, interface{}) error {
	return nil
}

func (r testArchiveReader) ReadComponent(id string) (io.ReadCloser, error) {
	return r.reader.OpenComponent(fmt.Sprintf("datasets/%s.ndjson", id))
}

func (r testArchiveReader) ReadJSON(id string, value interface{}) error {
	reader, err := r.ReadComponent(id)
	if err != nil {
		return err
	}
	defer reader.Close()
	return json.NewDecoder(reader).Decode(value)
}

func (r testArchiveReader) ListComponents() []string {
	return []string{ModelAssetsComponent}
}

func newBackupTestContributor(t *testing.T) (*ArtifactBackupContributor, *gorm.DB, string) {
	t.Helper()
	dataRoot := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(dataRoot, "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&ModelArtifact{}); err != nil {
		t.Fatal(err)
	}
	return NewArtifactBackupContributor(db, dataRoot), db, dataRoot
}

func seedBackupArtifact(t *testing.T, contributor *ArtifactBackupContributor, db *gorm.DB) ModelArtifact {
	t.Helper()
	data := append([]byte("GGUF"), bytes.Repeat([]byte{1}, 2048)...)
	id := "local_model_backup_test"
	filename := "backup-test.gguf"
	dir := filepath.Join(contributor.Root, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filename), data, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	row := ModelArtifact{
		ID:           id,
		OwnerSpaceID: "space-a",
		Name:         "backup-test",
		Filename:     filename,
		Format:       "gguf",
		SizeBytes:    int64(len(data)),
		SHA256:       hex.EncodeToString(digest[:]),
		ResourceURI:  ModelArtifactURI(id, filename),
		Status:       "ready",
		CreatedAt:    "2026-10-31T00:00:00Z",
		UpdatedAt:    "2026-10-31T00:00:00Z",
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

func writeModelAssetArchive(t *testing.T, path string, rows []ModelArtifact, files map[string][]byte) {
	t.Helper()
	writer := dataportability.NewArchiveWriter(path)
	component, err := writer.CreateComponent(ModelAssetsComponent, "model.assets.v1.tar", dataportability.KindResource)
	if err != nil {
		t.Fatal(err)
	}
	archive := tar.NewWriter(component)
	metadata, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if err := archive.WriteHeader(&tar.Header{Name: "model-artifacts.json", Mode: 0o600, Size: int64(len(metadata))}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write(metadata); err != nil {
		t.Fatal(err)
	}
	for name, data := range files {
		if err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Finalize(); err != nil {
		t.Fatal(err)
	}
}

func TestArtifactBackupRoundTripRestoresFileAndMetadata(t *testing.T) {
	contributor, db, dataRoot := newBackupTestContributor(t)
	row := seedBackupArtifact(t, contributor, db)
	archivePath := filepath.Join(dataRoot, "backup.amitia-backup")
	writer := testBackupWriter{ArchiveWriter: dataportability.NewArchiveWriter(archivePath)}
	request := dataportability.BackupRequest{IncludeModelAssets: true}
	plans, err := contributor.Plan(context.Background(), request)
	if err != nil || len(plans) != 1 || plans[0].ItemCount != 1 {
		t.Fatalf("unexpected backup plan: %#v %v", plans, err)
	}
	if err := contributor.Export(context.Background(), request, writer); err != nil {
		t.Fatal(err)
	}
	if err := writer.Finalize(); err != nil {
		t.Fatal(err)
	}
	reader, err := dataportability.NewArchiveReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	adapter := testArchiveReader{reader: reader}
	preview, err := contributor.PreviewImport(context.Background(), dataportability.ImportPreviewRequest{}, adapter)
	if err != nil || len(preview) != 1 || preview[0].ItemCount != 1 {
		t.Fatalf("unexpected import preview: %#v %v", preview, err)
	}
	if err := db.Delete(&ModelArtifact{}, "id = ?", row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(contributor.Root, row.ID)); err != nil {
		t.Fatal(err)
	}
	if err := contributor.Import(context.Background(), dataportability.ImportRequest{}, adapter); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(filepath.Join(contributor.Root, row.ID, row.Filename))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(restored)
	if hex.EncodeToString(digest[:]) != row.SHA256 {
		t.Fatal("restored model content hash mismatch")
	}
	var stored ModelArtifact
	if err := db.Where("id = ?", row.ID).Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.ResourceURI != row.ResourceURI || stored.SHA256 != row.SHA256 || stored.SizeBytes != row.SizeBytes {
		t.Fatalf("restored metadata mismatch: %#v", stored)
	}
}

func TestArtifactBackupImportRejectsTamperedContent(t *testing.T) {
	contributor, db, dataRoot := newBackupTestContributor(t)
	id := "local_model_tampered"
	filename := "tampered.gguf"
	data := append([]byte("GGUF"), bytes.Repeat([]byte{2}, 2048)...)
	row := ModelArtifact{
		ID:           id,
		OwnerSpaceID: "space-a",
		Name:         "tampered",
		Filename:     filename,
		Format:       "gguf",
		SizeBytes:    int64(len(data)),
		SHA256:       "0000000000000000000000000000000000000000000000000000000000000000",
		ResourceURI:  ModelArtifactURI(id, filename),
		Status:       "ready",
	}
	archivePath := filepath.Join(dataRoot, "tampered.amitia-backup")
	writeModelAssetArchive(t, archivePath, []ModelArtifact{row}, map[string][]byte{
		"models/" + id + "/" + filename: data,
	})
	reader, err := dataportability.NewArchiveReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	err = contributor.Import(context.Background(), dataportability.ImportRequest{}, testArchiveReader{reader: reader})
	if err == nil {
		t.Fatal("tampered model content was accepted")
	}
	var count int64
	if err := db.Model(&ModelArtifact{}).Where("id = ?", id).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("tampered model metadata was persisted")
	}
}

func TestArtifactBackupImportRejectsUnsafeArtifactIdentity(t *testing.T) {
	contributor, db, dataRoot := newBackupTestContributor(t)
	row := ModelArtifact{
		ID:           "..",
		OwnerSpaceID: "space-a",
		Name:         "unsafe",
		Filename:     "unsafe.gguf",
		Format:       "gguf",
		SizeBytes:    4,
		SHA256:       "0000000000000000000000000000000000000000000000000000000000000000",
	}
	archivePath := filepath.Join(dataRoot, "unsafe.amitia-backup")
	writeModelAssetArchive(t, archivePath, []ModelArtifact{row}, nil)
	reader, err := dataportability.NewArchiveReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	err = contributor.Import(context.Background(), dataportability.ImportRequest{}, testArchiveReader{reader: reader})
	if err == nil {
		t.Fatal("unsafe artifact identity was accepted")
	}
	var count int64
	if err := db.Model(&ModelArtifact{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("unsafe artifact metadata was persisted")
	}
}
