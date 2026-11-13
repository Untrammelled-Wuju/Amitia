package chat

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/system/dataportability"
)

type chatBackupWriter struct {
	components map[string]*bytes.Buffer
}

func (w *chatBackupWriter) CreateComponent(id, _ string, _ dataportability.ComponentKind) (io.WriteCloser, error) {
	w.components[id] = &bytes.Buffer{}
	return chatNopWriteCloser{Writer: w.components[id]}, nil
}

func (*chatBackupWriter) WriteJSON(string, any) error { return nil }

type chatNopWriteCloser struct{ io.Writer }

func (chatNopWriteCloser) Close() error { return nil }

type chatBackupReader struct {
	components map[string][]byte
}

func (r *chatBackupReader) ReadComponent(id string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(r.components[id])), nil
}

func (*chatBackupReader) ReadJSON(string, any) error { return nil }

func (r *chatBackupReader) ListComponents() []string {
	ids := make([]string, 0, len(r.components))
	for id := range r.components {
		ids = append(ids, id)
	}
	return ids
}

func openChatKernelDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(coordination.ResourceSchema); err != nil {
		t.Fatal(err)
	}
	return db
}

func insertChatRecord(t *testing.T, db *sql.DB, ownerID string, kind string, resourceID string, roleID string, revision int64, body string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,source_id,revision,deleted,body,updated_at) VALUES(?,?,?,?,'',?,0,?,?)`, ownerID, kind, resourceID, roleID, revision, body, "2026-02-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
}

func seededChatKernel(t *testing.T) *sql.DB {
	t.Helper()
	db := openChatKernelDB(t)
	insertChatRecord(t, db, "device-a", "conversation", "conv-1", "role-a", 2, `{"id":"conv-1","conversationId":"conv-1","characterId":"role-a","title":"第一段对话","channel":"web"}`)
	insertChatRecord(t, db, "device-a", "conversation", "conv-2", "role-a", 1, `{"id":"conv-2","conversationId":"conv-2","characterId":"role-a","title":"第二段对话","channel":"desktop"}`)
	insertChatRecord(t, db, "device-a", "conversation", "conv-3", "role-b", 1, `{"id":"conv-3","conversationId":"conv-3","characterId":"role-b","title":"别的角色","channel":"web"}`)
	insertChatRecord(t, db, "device-a", "message", "req-1/user", "role-a", 1, `{"id":"req-1/user","conversationId":"conv-1","characterId":"role-a","role":"user","content":"你好"}`)
	insertChatRecord(t, db, "device-a", "message", "req-1/assistant", "role-a", 1, `{"id":"req-1/assistant","conversationId":"conv-1","characterId":"role-a","role":"assistant","content":"在"}`)
	insertChatRecord(t, db, "device-a", "message", "req-2/user", "role-b", 1, `{"id":"req-2/user","conversationId":"conv-3","characterId":"role-b","role":"user","content":"别的会话"}`)
	return db
}

func readChatLines(t *testing.T, payload []byte) []ownedChatRecordV1 {
	t.Helper()
	records := make([]ownedChatRecordV1, 0)
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var record ownedChatRecordV1
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return records
}

func TestOwnedChatBackupExportsConversationsAndMessagesFromKernel(t *testing.T) {
	db := seededChatKernel(t)
	contributor := NewChatBackupContributor(db)
	writer := &chatBackupWriter{components: map[string]*bytes.Buffer{}}
	if err := contributor.Export(t.Context(), dataportability.BackupRequest{Scope: dataportability.ScopeAll}, writer); err != nil {
		t.Fatal(err)
	}
	conversations := readChatLines(t, writer.components[ComponentIDChatConversations].Bytes())
	messages := readChatLines(t, writer.components[ComponentIDChatMessages].Bytes())
	if len(conversations) != 3 || len(messages) != 3 {
		t.Fatalf("unexpected export size conversations=%d messages=%d", len(conversations), len(messages))
	}
	for _, record := range conversations {
		if record.ID == "conv-1" && !strings.Contains(string(record.Body), "第一段对话") {
			t.Fatalf("conversation body lost: %s", string(record.Body))
		}
	}
	plans, err := contributor.Plan(t.Context(), dataportability.BackupRequest{Scope: dataportability.ScopeAll})
	if err != nil {
		t.Fatal(err)
	}
	for _, plan := range plans {
		switch plan.ID {
		case ComponentIDChatConversations:
			if plan.ItemCount != 3 {
				t.Fatalf("conversation plan count=%d", plan.ItemCount)
			}
		case ComponentIDChatMessages:
			if plan.ItemCount != 3 {
				t.Fatalf("message plan count=%d", plan.ItemCount)
			}
		}
	}
}

func TestOwnedChatBackupFiltersByCharacterScope(t *testing.T) {
	db := seededChatKernel(t)
	contributor := NewChatBackupContributor(db)
	writer := &chatBackupWriter{components: map[string]*bytes.Buffer{}}
	if err := contributor.Export(t.Context(), dataportability.BackupRequest{Scope: dataportability.ScopeCharacter, CharacterID: "role-a"}, writer); err != nil {
		t.Fatal(err)
	}
	if conversations := readChatLines(t, writer.components[ComponentIDChatConversations].Bytes()); len(conversations) != 2 {
		t.Fatalf("character scope conversations=%d", len(conversations))
	}
	if messages := readChatLines(t, writer.components[ComponentIDChatMessages].Bytes()); len(messages) != 2 {
		t.Fatalf("character scope messages=%d", len(messages))
	}
}

func TestOwnedChatBackupRestoreRoundTrip(t *testing.T) {
	db := seededChatKernel(t)
	contributor := NewChatBackupContributor(db)
	writer := &chatBackupWriter{components: map[string]*bytes.Buffer{}}
	if err := contributor.Export(t.Context(), dataportability.BackupRequest{Scope: dataportability.ScopeAll}, writer); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM kernel_device_owned_resources WHERE kind IN ('conversation','message')`); err != nil {
		t.Fatal(err)
	}
	reader := &chatBackupReader{components: map[string][]byte{
		ComponentIDChatConversations: writer.components[ComponentIDChatConversations].Bytes(),
		ComponentIDChatMessages:      writer.components[ComponentIDChatMessages].Bytes(),
	}}
	identity := dataportability.NewImportIdentityMap()
	if err := contributor.RestoreChats(context.Background(), reader, dataportability.RestoreOptions{OperationID: "restore", CharacterPolicy: dataportability.CollisionReplace, IdentityMap: identity}); err != nil {
		t.Fatal(err)
	}
	var conversations, messages int
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_device_owned_resources WHERE kind='conversation' AND deleted=0`).Scan(&conversations); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_device_owned_resources WHERE kind='message' AND deleted=0`).Scan(&messages); err != nil {
		t.Fatal(err)
	}
	if conversations != 3 || messages != 3 {
		t.Fatalf("restored conversations=%d messages=%d", conversations, messages)
	}
	var body string
	if err := db.QueryRow(`SELECT CAST(body AS TEXT) FROM kernel_device_owned_resources WHERE kind='message' AND resource_id='req-1/user'`).Scan(&body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `"conversationId":"conv-1"`) {
		t.Fatalf("message conversation link lost: %s", body)
	}
}

func TestOwnedChatBackupRestoreRemapsCharacter(t *testing.T) {
	db := seededChatKernel(t)
	contributor := NewChatBackupContributor(db)
	writer := &chatBackupWriter{components: map[string]*bytes.Buffer{}}
	if err := contributor.Export(t.Context(), dataportability.BackupRequest{Scope: dataportability.ScopeAll}, writer); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM kernel_device_owned_resources WHERE kind IN ('conversation','message')`); err != nil {
		t.Fatal(err)
	}
	reader := &chatBackupReader{components: map[string][]byte{
		ComponentIDChatConversations: writer.components[ComponentIDChatConversations].Bytes(),
		ComponentIDChatMessages:      writer.components[ComponentIDChatMessages].Bytes(),
	}}
	identity := dataportability.NewImportIdentityMap()
	identity.AddCharacter("role-a", "role-new")
	if err := contributor.RestoreChats(context.Background(), reader, dataportability.RestoreOptions{OperationID: "restore", CharacterPolicy: dataportability.CollisionReplace, IdentityMap: identity}); err != nil {
		t.Fatal(err)
	}
	var role string
	if err := db.QueryRow(`SELECT role_id FROM kernel_device_owned_resources WHERE kind='conversation' AND resource_id='conv-1'`).Scan(&role); err != nil {
		t.Fatal(err)
	}
	if role != "role-new" {
		t.Fatalf("character remap failed: %s", role)
	}
	var body string
	if err := db.QueryRow(`SELECT CAST(body AS TEXT) FROM kernel_device_owned_resources WHERE kind='message' AND resource_id='req-1/user'`).Scan(&body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `"characterId":"role-new"`) {
		t.Fatalf("message character not remapped: %s", body)
	}
}

func TestOwnedChatBackupRestoreDuplicatesWithNewIdentifiers(t *testing.T) {
	db := seededChatKernel(t)
	contributor := NewChatBackupContributor(db)
	writer := &chatBackupWriter{components: map[string]*bytes.Buffer{}}
	if err := contributor.Export(t.Context(), dataportability.BackupRequest{Scope: dataportability.ScopeAll}, writer); err != nil {
		t.Fatal(err)
	}
	reader := &chatBackupReader{components: map[string][]byte{
		ComponentIDChatConversations: writer.components[ComponentIDChatConversations].Bytes(),
		ComponentIDChatMessages:      writer.components[ComponentIDChatMessages].Bytes(),
	}}
	identity := dataportability.NewImportIdentityMap()
	if err := contributor.RestoreChats(context.Background(), reader, dataportability.RestoreOptions{OperationID: "restore", CharacterPolicy: dataportability.CollisionDuplicate, IdentityMap: identity}); err != nil {
		t.Fatal(err)
	}
	var conversations, messages int
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_device_owned_resources WHERE kind='conversation' AND deleted=0`).Scan(&conversations); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_device_owned_resources WHERE kind='message' AND deleted=0`).Scan(&messages); err != nil {
		t.Fatal(err)
	}
	if conversations != 6 || messages != 6 {
		t.Fatalf("duplicate restore conversations=%d messages=%d", conversations, messages)
	}
	mapped, ok := identity.GetConversation("conv-1")
	if !ok || mapped == "conv-1" {
		t.Fatalf("conversation identity not remapped: %s", mapped)
	}
	var linked int
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_device_owned_resources WHERE kind='message' AND deleted=0 AND json_extract(body,'$.conversationId')=?`, mapped).Scan(&linked); err != nil {
		t.Fatal(err)
	}
	if linked != 2 {
		t.Fatalf("duplicated messages not linked to new conversation: %d", linked)
	}
}
