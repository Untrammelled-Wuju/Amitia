package character

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/character/card"
	"github.com/u-ai/backend/pkg/app"
)

type testCardWorldbookBridge struct {
	entries  []card.CharacterBookEntry
	exported *card.CharacterBook
	calls    int
	fail     bool
	verify   func() error
}

func (b *testCardWorldbookBridge) ImportCardWorldbook(_ context.Context, _, _, _, _ string, entries []card.CharacterBookEntry) (int, error) {
	b.calls++
	b.entries = append([]card.CharacterBookEntry(nil), entries...)
	if b.verify != nil {
		if err := b.verify(); err != nil {
			return 0, err
		}
	}
	if b.fail {
		return 0, errors.New("test worldbook failure")
	}
	return len(entries), nil
}

func (b *testCardWorldbookBridge) ExportCardWorldbook(_ context.Context, _, _ string) (*card.CharacterBook, error) {
	return b.exported, nil
}

func TestCardImportPersistsWorldbookSessionAndRetriesWithoutDuplicateCharacter(t *testing.T) {
	db := newTestDB(t)
	ctx := app.NewAppContext(db, nil)
	bridge := &testCardWorldbookBridge{fail: true}
	bridge.verify = func() error {
		gate := runtimeSource(db)
		if !gate.TryRLock() {
			return errors.New("role source write lock still held")
		}
		gate.RUnlock()
		return nil
	}
	svc := NewServiceWithCardWorldbook(NewRepository(ctx), ctx, bridge)
	data := testCHARX(t)
	importID := "card-import-worldbook-test"

	if _, err := svc.(*service).ImportCardForSpaceContext(context.Background(), data, "character.charx", true, importID, "device-space"); err == nil {
		t.Fatal("expected first worldbook import to fail")
	}
	var failed CardImportOperation
	if err := db.Where("space_id=? AND id=?", "device-space", importID).First(&failed).Error; err != nil {
		t.Fatal(err)
	}
	if failed.Status != "failed" || failed.WorldbookExpected != 2 || failed.LastError == "" {
		t.Fatalf("unexpected failed import session: %+v", failed)
	}

	bridge.fail = false
	result, err := svc.(*service).ImportCardForSpaceContext(context.Background(), data, "character.charx", true, importID, "device-space")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ready" || result.WorldbookImportedCount != 2 || result.LorebookEntryCount != 2 || len(bridge.entries) != 2 {
		t.Fatalf("unexpected import result: %+v entries=%d", result, len(bridge.entries))
	}
	var count int64
	if err := db.Model(&Character{}).Where("space_id=?", "device-space").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("characters=%d err=%v", count, err)
	}
	replayed, err := svc.(*service).ImportCardForSpaceContext(context.Background(), data, "character.charx", true, importID, "device-space")
	if err != nil || replayed.CharacterID != result.CharacterID || replayed.Status != "ready" {
		t.Fatalf("replay result=%+v err=%v", replayed, err)
	}
	if bridge.calls != 2 {
		t.Fatalf("worldbook bridge calls=%d", bridge.calls)
	}
}

func TestCardExportReadsWorldbookFromSingleSource(t *testing.T) {
	db := newTestDB(t)
	ctx := app.NewAppContext(db, nil)
	bridge := &testCardWorldbookBridge{exported: &card.CharacterBook{Entries: []card.CharacterBookEntry{{
		Name: "港口", Keys: []string{"港口", "码头"}, Content: "港口在夜间封闭", Enabled: true, InsertionOrder: 3,
	}}}}
	svc := NewServiceWithCardWorldbook(NewRepository(ctx), ctx, bridge)
	data := testCHARX(t)
	result, err := svc.(*service).ImportCardForSpaceContext(context.Background(), data, "character.charx", true, "card-export-worldbook-test", "device-space")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	_, exported, err := svc.(*service).ExportCardForSpaceContext(context.Background(), result.CharacterID, "v3_charx", "device-space")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(exported), int64(len(exported)))
	if err != nil {
		t.Fatal(err)
	}
	var cardJSON []byte
	for _, file := range reader.File {
		if file.Name != "card.json" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(rc); err != nil {
			rc.Close()
			t.Fatal(err)
		}
		rc.Close()
		cardJSON = buf.Bytes()
	}
	var payload struct {
		CharacterBook struct {
			Entries []card.CharacterBookEntry `json:"entries"`
		} `json:"character_book"`
	}
	if err := json.Unmarshal(cardJSON, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.CharacterBook.Entries) != 1 || payload.CharacterBook.Entries[0].Name != "港口" || len(payload.CharacterBook.Entries[0].Keys) != 2 {
		t.Fatalf("unexpected exported character book: %+v", payload.CharacterBook)
	}
}

func testCHARX(t *testing.T) []byte {
	t.Helper()
	cardJSON, err := json.Marshal(map[string]any{
		"spec": "chara_card_v3", "spec_version": "3.0",
		"name": "测试角色", "description": "角色说明",
		"character_book": map[string]any{"entries": []map[string]any{
			{"id": 7, "name": "晨星", "keys": []string{"晨星", "星图"}, "content": "晨星计划只对探索队公开", "enabled": true, "insertion_order": 10},
			{"name": "雨天", "keys": []string{"雨"}, "secondary_keys": []string{"港口"}, "content": "雨天港口关闭", "enabled": true, "selective": true, "priority": 3},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	entry, err := writer.Create("card.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(cardJSON); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
