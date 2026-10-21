package worldbook

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/system/dataportability"
)

type memoryBackupWriter struct {
	components map[string]*bytes.Buffer
}

func (w *memoryBackupWriter) CreateComponent(id, _ string, _ dataportability.ComponentKind) (io.WriteCloser, error) {
	w.components[id] = &bytes.Buffer{}
	return nopWriteCloser{Writer: w.components[id]}, nil
}

func (*memoryBackupWriter) WriteJSON(string, any) error { return nil }

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

type memoryBackupReader struct {
	components map[string][]byte
}

func (r *memoryBackupReader) ReadComponent(id string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(r.components[id])), nil
}

func (*memoryBackupReader) ReadJSON(string, any) error { return nil }

func (*memoryBackupReader) ListComponents() []string { return nil }

func TestOwnedWorldbookBackupRestoresOwnerRoleRevisionAndBody(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(coordination.ResourceSchema); err != nil {
		t.Fatal(err)
	}
	rule := `{"id":"rule","scope":"global","matchType":"keyword","matchPattern":"联盟","matchScope":"full_context","injectContent":"全局规则","priority":1,"enabled":true}`
	if _, err := db.Exec(`INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,deleted,body,updated_at) VALUES('device-a','worldbook','rule',?,3,0,?,?)`, coordination.GlobalWorldbookRole, rule, "2026-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	contributor := NewWorldbookBackupContributor(db)
	writer := &memoryBackupWriter{components: map[string]*bytes.Buffer{}}
	if err := contributor.Export(t.Context(), dataportability.BackupRequest{Scope: dataportability.ScopeAll}, writer); err != nil {
		t.Fatal(err)
	}
	if writer.components[ComponentIDWorldbookRecords].Len() == 0 {
		t.Fatal("worldbook backup component is empty")
	}
	if _, err := db.Exec(`DELETE FROM kernel_device_owned_resources WHERE kind='worldbook'`); err != nil {
		t.Fatal(err)
	}
	reader := &memoryBackupReader{components: map[string][]byte{ComponentIDWorldbookRecords: writer.components[ComponentIDWorldbookRecords].Bytes()}}
	if err := contributor.RestoreWorldbook(context.Background(), reader, dataportability.RestoreOptions{OperationID: "restore", CharacterPolicy: dataportability.CollisionReplace}); err != nil {
		t.Fatal(err)
	}
	var owner, role, body string
	var revision int64
	if err := db.QueryRow(`SELECT owner_id,role_id,revision,CAST(body AS TEXT) FROM kernel_device_owned_resources WHERE kind='worldbook' AND resource_id='rule'`).Scan(&owner, &role, &revision, &body); err != nil {
		t.Fatal(err)
	}
	if owner != "device-a" || role != coordination.GlobalWorldbookRole || revision != 3 || body != rule {
		t.Fatalf("restored worldbook changed: owner=%s role=%s revision=%d body=%s", owner, role, revision, body)
	}
}
