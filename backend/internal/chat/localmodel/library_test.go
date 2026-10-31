package localmodel

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestLibraryImportsDeduplicatesAndProtectsReferencedModel(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "models.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE model_artifacts (
		id TEXT PRIMARY KEY,
		owner_space_id TEXT NOT NULL DEFAULT '',
		name TEXT NOT NULL DEFAULT '',
		filename TEXT NOT NULL DEFAULT '',
		format TEXT NOT NULL DEFAULT '',
		size_bytes INTEGER NOT NULL DEFAULT 0,
		sha256 TEXT NOT NULL DEFAULT '',
		resource_uri TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT ''
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX idx_model_artifacts_owner_sha256 ON model_artifacts(owner_space_id,sha256)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE model_artifact_references (
		artifact_id TEXT NOT NULL DEFAULT '',
		model_config_id INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (artifact_id, model_config_id)
	)`).Error; err != nil {
		t.Fatal(err)
	}
	payload := append([]byte("GGUF"), bytes.Repeat([]byte{0}, 4096)...)
	library := NewLibrary(db, t.TempDir())
	first, err := library.Import(context.Background(), "space-a", "tiny", "tiny.gguf", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	second, err := library.Import(context.Background(), "space-a", "tiny-copy", "tiny-copy.gguf", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || first.ResourceURI != second.ResourceURI {
		t.Fatalf("duplicate model was not reused: %#v %#v", first, second)
	}
	if ArtifactIDFromURI(first.ResourceURI) != first.ID {
		t.Fatalf("artifact URI mismatch: %s", first.ResourceURI)
	}
	if err := db.Exec(`INSERT INTO model_artifact_references(artifact_id,model_config_id,created_at) VALUES(?,?,?)`, first.ID, 1, "now").Error; err != nil {
		t.Fatal(err)
	}
	if err := library.Delete(context.Background(), "space-a", first.ID); err == nil {
		t.Fatal("referenced model was deleted")
	}
	if err := db.Exec(`DELETE FROM model_artifact_references WHERE artifact_id=?`, first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := library.Delete(context.Background(), "space-a", first.ID); err != nil {
		t.Fatal(err)
	}
}

func TestReferencedArtifactIDsReadsProviderConfig(t *testing.T) {
	raw := `{"resourceUri":"amitia://data/models/model-a/tiny.gguf","mmprojUri":"amitia://data/models/model-b/mmproj.gguf","nested":{"modelResourceUri":"amitia://data/models/model-c/x.mnn"}}`
	ids := ReferencedArtifactIDs(raw)
	if len(ids) != 3 {
		t.Fatalf("unexpected references: %#v", ids)
	}
}
