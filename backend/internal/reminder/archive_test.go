package reminder

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/system/dataportability"
)

type taskBackupMemory map[string][]byte
type taskBackupWriter struct {
	bytes.Buffer
	close func([]byte)
}

func (w *taskBackupWriter) Close() error { w.close(append([]byte(nil), w.Bytes()...)); return nil }
func (m taskBackupMemory) CreateComponent(id, _ string, _ dataportability.ComponentKind) (io.WriteCloser, error) {
	return &taskBackupWriter{close: func(raw []byte) { m[id] = raw }}, nil
}
func (m taskBackupMemory) WriteJSON(id string, value interface{}) error {
	raw, err := json.Marshal(value)
	m[id] = raw
	return err
}
func (m taskBackupMemory) ReadComponent(id string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(m[id])), nil
}
func (m taskBackupMemory) ReadJSON(id string, value interface{}) error {
	return json.Unmarshal(m[id], value)
}
func (m taskBackupMemory) ListComponents() []string {
	keys := []string{}
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func TestTaskBackupSignedRoundtripPreservesReadOnlyHistoryAndPausesRestoredTask(t *testing.T) {
	source, scheduler, clock, _ := newReminderTestService(t)
	caller := Caller{SpaceID: "space", DeviceID: "device"}
	item, err := source.Create(t.Context(), caller, newReminderRequest(), "manual")
	if err != nil {
		t.Fatal(err)
	}
	trigger, err := source.TriggerNow(t.Context(), caller, item.ID, item.Generation)
	if err != nil {
		t.Fatal(err)
	}
	clock.Advance(time.Second)
	if _, err := scheduler.GetExecutor().Execute(t.Context(), trigger); err != nil {
		t.Fatal(err)
	}
	_, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	current := source
	contributor := NewTaskBackupContributor(func(context.Context) (*Service, Caller, error) { return current, caller, nil }, signer)
	data := taskBackupMemory{}
	if err := contributor.Export(t.Context(), dataportability.BackupRequest{Scope: dataportability.ScopeAll}, data); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data[TaskBackupComponent]), `"binding"`) || strings.Contains(string(data[TaskBackupComponent]), "permissionSnapshot") {
		t.Fatal("backup exported execution authority")
	}
	target, targetScheduler, _, _ := newReminderTestService(t)
	current = target
	if err := contributor.Import(t.Context(), dataportability.ImportRequest{CharacterPolicy: dataportability.CollisionReplace}, data); err != nil {
		t.Fatal(err)
	}
	restored, err := target.Find(t.Context(), caller, item.ID)
	if err != nil || !restored.Paused || restored.Enabled != 0 || restored.OwnerID != item.OwnerID {
		t.Fatalf("restored task activated or moved: %+v %v", restored, err)
	}
	history, total, err := target.History(t.Context(), caller, "", item.ID, 1, 20, "")
	if err != nil || total != 1 || len(history) != 1 || !history[0].Archived {
		t.Fatalf("history lost: %+v %d %v", history, total, err)
	}
	triggers, err := targetScheduler.GetAllTriggers(t.Context(), item.ID)
	if err != nil || len(triggers) != 0 {
		t.Fatalf("archive replayed triggers: %d %v", len(triggers), err)
	}
	if _, err := target.TriggerNow(t.Context(), caller, item.ID, restored.Generation); err == nil {
		t.Fatal("paused restored task executed")
	}
}

func TestTaskBackupRejectsTamperedProvenanceAndHistoricalFormat(t *testing.T) {
	service, _, _, _ := newReminderTestService(t)
	caller := Caller{SpaceID: "space", DeviceID: "device"}
	if _, err := service.Create(t.Context(), caller, newReminderRequest(), "manual"); err != nil {
		t.Fatal(err)
	}
	_, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	contributor := NewTaskBackupContributor(func(context.Context) (*Service, Caller, error) { return service, caller, nil }, signer)
	data := taskBackupMemory{}
	if err := contributor.Export(t.Context(), dataportability.BackupRequest{Scope: dataportability.ScopeAll}, data); err != nil {
		t.Fatal(err)
	}
	var signed signedTaskArchive
	if err := json.Unmarshal(data[TaskBackupComponent], &signed); err != nil {
		t.Fatal(err)
	}
	signed.Archive.Tasks[0].Source = "chat"
	data[TaskBackupComponent], _ = json.Marshal(signed)
	if _, err := contributor.PreviewImport(t.Context(), dataportability.ImportPreviewRequest{}, data); err == nil {
		t.Fatal("modified task provenance accepted")
	}
	legacy := taskBackupMemory{"reminder.legacy": []byte(`[]`)}
	if err := contributor.Import(t.Context(), dataportability.ImportRequest{}, legacy); err == nil {
		t.Fatal("legacy backup format accepted")
	}
}

func TestTaskArchiveRechecksOwnerAndPluginBeforeRestoring(t *testing.T) {
	service, _, _, _ := newReminderTestService(t)
	caller := Caller{SpaceID: "space", DeviceID: "device"}
	pluginCaller := caller
	pluginCaller.PluginID, pluginCaller.ModuleID = "plugin", "module"
	if _, err := service.Create(t.Context(), pluginCaller, newReminderRequest(), "plugin"); err != nil {
		t.Fatal(err)
	}
	archive, err := service.ExportTaskArchive(t.Context(), caller, "")
	if err != nil {
		t.Fatal(err)
	}
	target, _, _, _ := newReminderTestService(t)
	target.deps.PluginActive = func(context.Context, string, string) error { return ErrForbidden }
	if err := target.RestoreTaskArchive(t.Context(), caller, dataportability.ImportRequest{}, archive); err == nil {
		t.Fatal("disabled plugin restored executable task")
	}
	archive.OwnerID = "other-owner"
	if err := target.RestoreTaskArchive(t.Context(), caller, dataportability.ImportRequest{}, archive); err == nil {
		t.Fatal("different owner accepted")
	}
	items, err := target.List(t.Context(), caller, "")
	if err != nil || len(items) != 0 {
		t.Fatalf("failed restore partially committed: %d %v", len(items), err)
	}
}
