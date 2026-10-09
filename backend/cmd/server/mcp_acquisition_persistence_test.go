package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/mcp"
	"github.com/u-ai/backend/internal/migration"
	"gorm.io/gorm"
)

func TestAcquisitionDiscoveryMCPConfigurationPersistsEncrypted(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(directory, "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := migration.ApplyBaseline(db); err != nil {
		t.Fatal(err)
	}
	repository := mcp.NewRepository(db)
	persistence, err := newMCPAcquisitionPersistence(repository, directory)
	if err != nil {
		t.Fatal(err)
	}
	id, err := persistence.SaveMCPConfiguration(ctx, "test-mcp", "stdio", "npx", []string{"-y", "@example/server@1.2.3"}, map[string]string{"TEST_TOKEN": "isolated-test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	if err := persistence.MarkMCPReady(ctx, id); err != nil {
		t.Fatal(err)
	}
	restored, err := newMCPAcquisitionPersistence(repository, directory)
	if err != nil {
		t.Fatal(err)
	}
	server, err := repository.GetServer(ctx, id)
	if err != nil || server.Status != "ready" || server.AuthType != "stdio_env" || server.Name != "test-mcp" {
		t.Fatalf("server=%+v err=%v", server, err)
	}
	references, err := repository.CredentialReferences(ctx, id)
	if err != nil || len(references) != 1 || references[0] == "" {
		t.Fatalf("references=%+v err=%v", references, err)
	}
	secret, err := restored.secrets.Get(ctx, references[0])
	if err != nil || !bytes.Contains(secret, []byte("isolated-test-secret")) {
		t.Fatal("encrypted environment did not survive reopening")
	}
	raw, err := os.ReadFile(filepath.Join(directory, "mcp", "secrets.json"))
	if err != nil || bytes.Contains(raw, []byte("isolated-test-secret")) {
		t.Fatal("plaintext credential found in secret file")
	}
	if _, err := restored.SaveMCPConfiguration(ctx, "duplicate", "stdio", "npx", []string{"-y", "@example/server@1.2.3"}, nil); err == nil {
		t.Fatal("duplicate identity was accepted")
	}
	if err := restored.RemoveMCPConfiguration(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetServer(ctx, id); err == nil {
		t.Fatal("configuration survived rollback")
	}
	if _, err := restored.secrets.Get(ctx, references[0]); err == nil {
		t.Fatal("credential survived rollback")
	}
}
