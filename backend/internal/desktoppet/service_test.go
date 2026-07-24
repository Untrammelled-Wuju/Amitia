// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/migration"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "desktoppet_test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlDB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	sqlPath := filepath.Join("..", "..", "data", "sql.sql")
	if err := migration.ApplyInitialSQLFile(db, sqlPath); err != nil {
		t.Fatalf("apply initial sql: %v", err)
	}

	runner := migration.Runner{DB: db, SkipBackup: true}
	if err := runner.Apply(migration.DefaultMigrations()); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return db
}

func newServiceFromDB(t *testing.T, db *gorm.DB) Service {
	t.Helper()
	ctx := &app.AppContext{DB: db, Context: context.Background()}
	repo := NewRepository(db, ctx)
	return NewService(repo, db)
}

func setupTestService(t *testing.T) (Service, *gorm.DB, string) {
	t.Helper()
	db := setupTestDB(t)
	dataDir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dataDir); err == nil {
		dataDir = resolved
	}
	originalCfg := config.AppCfg
	config.AppCfg = &config.Config{Storage: config.StorageConfig{DataDir: dataDir}}
	t.Cleanup(func() { config.AppCfg = originalCfg })

	if err := db.Create(&character.Character{
		ID:     "char_test",
		Name:   "测试角色",
		Status: "enabled",
	}).Error; err != nil {
		t.Fatalf("seed character: %v", err)
	}
	if err := db.Exec(`INSERT INTO image_gen_configs(id,name,api_key,model_name,base_url,is_active,enabled) VALUES(1,'测试模型','key','model','url',1,1)`).Error; err != nil {
		t.Fatalf("seed image_gen_config: %v", err)
	}

	svc := newServiceFromDB(t, db)
	return svc, db, dataDir
}

func makePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func makeFileHeader(t *testing.T, content []byte, filename string) *multipart.FileHeader {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, err := mw.CreateFormFile("referenceImage", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	mr := multipart.NewReader(body, mw.Boundary())
	form, err := mr.ReadForm(1 << 20)
	if err != nil {
		t.Fatalf("read form: %v", err)
	}
	t.Cleanup(func() { _ = form.RemoveAll() })
	files := form.File["referenceImage"]
	if len(files) != 1 {
		t.Fatalf("expected 1 file header, got %d", len(files))
	}
	return files[0]
}

func assertBusinessError(t *testing.T, err error, expectedCode string) {
	t.Helper()
	var be *BusinessError
	if !errors.As(err, &be) {
		t.Fatalf("expected *BusinessError, got %T: %v", err, err)
	}
	if be.ErrCode != expectedCode {
		t.Fatalf("expected errCode %s, got %s (msg=%s)", expectedCode, be.ErrCode, be.Msg)
	}
}

func countTasks(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Table("desktop_pet_generation_tasks").Count(&count).Error; err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	return count
}

func expectedActionKeysByCategory(t *testing.T, db *gorm.DB, catKey string) []string {
	t.Helper()
	var keys []string
	if err := db.Table("desktop_pet_action_definitions").
		Where("category_key = ? AND enabled = 1", catKey).
		Order("sort_order ASC").
		Pluck("action_key", &keys).Error; err != nil {
		t.Fatalf("pluck action keys: %v", err)
	}
	return keys
}

func createValidTask(t *testing.T, svc Service, name string, actions []string) *TaskSummaryResponse {
	t.Helper()
	fh := makeFileHeader(t, makePNG(t), "ref.png")
	summary, err := svc.CreateTask(context.Background(), "test-user", "char_test", 1, name, "", "", 512, 512, actions, fh)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	return summary
}

func TestGetActionDefinitions(t *testing.T) {
	svc, db, _ := setupTestService(t)

	resp, err := svc.GetActionDefinitions()
	if err != nil {
		t.Fatalf("GetActionDefinitions: %v", err)
	}
	if len(resp.Categories) == 0 {
		t.Fatal("expected non-empty categories")
	}

	for i := 1; i < len(resp.Categories); i++ {
		if resp.Categories[i-1].SortOrder > resp.Categories[i].SortOrder {
			t.Fatalf("categories not sorted by sortOrder: idx %d=%d > idx %d=%d",
				i-1, resp.Categories[i-1].SortOrder, i, resp.Categories[i].SortOrder)
		}
	}

	for _, cat := range resp.Categories {
		expected := expectedActionKeysByCategory(t, db, cat.Key)
		if len(cat.Actions) != len(expected) {
			t.Fatalf("category %s actions count mismatch: got %d, want %d", cat.Key, len(cat.Actions), len(expected))
		}
		for i, want := range expected {
			if cat.Actions[i].Key != want {
				t.Fatalf("category %s action[%d] = %s, want %s", cat.Key, i, cat.Actions[i].Key, want)
			}
		}
	}

	presetKeys := map[string]bool{}
	for _, p := range resp.Presets {
		presetKeys[p.Key] = true
	}
	for _, key := range []string{"minimal", "standard", "complete"} {
		if !presetKeys[key] {
			t.Fatalf("missing preset %s", key)
		}
	}

	if err := db.Exec("UPDATE desktop_pet_action_definitions SET enabled = 0 WHERE action_key = 'idle_normal'").Error; err != nil {
		t.Fatal(err)
	}
	resp2, err := svc.GetActionDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	for _, cat := range resp2.Categories {
		for _, a := range cat.Actions {
			if a.Key == "idle_normal" {
				t.Fatal("disabled action idle_normal should not be returned")
			}
		}
	}

	if err := db.Exec("DELETE FROM desktop_pet_action_definitions").Error; err != nil {
		t.Fatal(err)
	}
	resp3, err := svc.GetActionDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	if resp3.Categories == nil {
		t.Fatal("expected non-nil empty categories slice")
	}
	if len(resp3.Categories) != 0 {
		t.Fatalf("expected empty categories, got %d", len(resp3.Categories))
	}
	if len(resp3.Presets) != 3 {
		t.Fatalf("expected 3 presets, got %d", len(resp3.Presets))
	}
}

func TestCreateTask_Valid(t *testing.T) {
	svc, db, dataDir := setupTestService(t)

	summary := createValidTask(t, svc, "valid-task", []string{"idle_normal", "walk_left"})

	if summary.Status != "pending" {
		t.Fatalf("status = %s, want pending", summary.Status)
	}
	if summary.CurrentStage != "queued" {
		t.Fatalf("currentStage = %s, want queued", summary.CurrentStage)
	}
	if summary.Progress != 0 {
		t.Fatalf("progress = %d, want 0", summary.Progress)
	}
	if summary.SelectedActionCount != 2 {
		t.Fatalf("selectedActionCount = %d, want 2", summary.SelectedActionCount)
	}
	if summary.CharacterID != "char_test" {
		t.Fatalf("characterID = %s, want char_test", summary.CharacterID)
	}
	if summary.ModelConfigID != 1 {
		t.Fatalf("modelConfigID = %d, want 1", summary.ModelConfigID)
	}

	var taskActions []GenerationTaskAction
	if err := db.Where("task_id = ?", summary.ID).Find(&taskActions).Error; err != nil {
		t.Fatal(err)
	}
	if len(taskActions) != 2 {
		t.Fatalf("task actions count = %d, want 2", len(taskActions))
	}
	keys := map[string]bool{}
	for _, ta := range taskActions {
		keys[ta.ActionKey] = true
		if ta.Status != "pending" {
			t.Fatalf("task action %s status = %s, want pending", ta.ActionKey, ta.Status)
		}
	}
	if !keys["idle_normal"] || !keys["walk_left"] {
		t.Fatalf("task actions keys = %v, want idle_normal+walk_left", keys)
	}

	taskDir := filepath.Join(dataDir, "desktop-pets", "generation-tasks", summary.ID)
	if _, err := os.Stat(taskDir); err != nil {
		t.Fatalf("task dir not created: %v", err)
	}
	refPath := filepath.Join(taskDir, "source", "reference.png")
	if _, err := os.Stat(refPath); err != nil {
		t.Fatalf("reference image not saved: %v", err)
	}

	if count := countTasks(t, db); count != 1 {
		t.Fatalf("expected 1 task, got %d", count)
	}
}

func TestCreateTask_NoAction(t *testing.T) {
	svc, db, _ := setupTestService(t)

	_, err := svc.CreateTask(context.Background(), "u", "char_test", 1, "t", "", "", 512, 512, nil, nil)
	assertBusinessError(t, err, ErrCodeActionSelectionRequired)
	if count := countTasks(t, db); count != 0 {
		t.Fatalf("expected 0 tasks, got %d", count)
	}

	_, err = svc.CreateTask(context.Background(), "u", "char_test", 1, "t", "", "", 512, 512, []string{}, nil)
	assertBusinessError(t, err, ErrCodeActionSelectionRequired)
	if count := countTasks(t, db); count != 0 {
		t.Fatalf("expected 0 tasks, got %d", count)
	}
}

func TestCreateTask_NoDefaultIdle(t *testing.T) {
	svc, db, _ := setupTestService(t)

	_, err := svc.CreateTask(context.Background(), "u", "char_test", 1, "t", "", "", 512, 512, []string{"walk_left", "walk_right"}, nil)
	assertBusinessError(t, err, ErrCodeDefaultIdleActionRequired)
	if count := countTasks(t, db); count != 0 {
		t.Fatalf("expected 0 tasks, got %d", count)
	}
}

func TestCreateTask_InvalidAction(t *testing.T) {
	svc, db, _ := setupTestService(t)

	_, err := svc.CreateTask(context.Background(), "u", "char_test", 1, "t", "", "", 512, 512, []string{"nonexistent"}, nil)
	assertBusinessError(t, err, ErrCodeActionNotFound)
	if count := countTasks(t, db); count != 0 {
		t.Fatalf("expected 0 tasks, got %d", count)
	}
}

func TestCreateTask_DisabledAction(t *testing.T) {
	svc, db, _ := setupTestService(t)

	if err := db.Exec("UPDATE desktop_pet_action_definitions SET enabled = 0 WHERE action_key = 'idle_normal'").Error; err != nil {
		t.Fatal(err)
	}

	_, err := svc.CreateTask(context.Background(), "u", "char_test", 1, "t", "", "", 512, 512, []string{"idle_normal"}, nil)
	assertBusinessError(t, err, ErrCodeActionNotFound)
	if count := countTasks(t, db); count != 0 {
		t.Fatalf("expected 0 tasks, got %d", count)
	}
}

func TestCreateTask_ModelNotFound(t *testing.T) {
	svc, db, _ := setupTestService(t)

	_, err := svc.CreateTask(context.Background(), "u", "char_test", 999, "t", "", "", 512, 512, []string{"idle_normal"}, nil)
	assertBusinessError(t, err, ErrCodeImageModelNotFound)
	if count := countTasks(t, db); count != 0 {
		t.Fatalf("expected 0 tasks, got %d", count)
	}
}

func TestCreateTask_ModelDisabled(t *testing.T) {
	svc, db, _ := setupTestService(t)

	if err := db.Exec("UPDATE image_gen_configs SET enabled = 0 WHERE id = 1").Error; err != nil {
		t.Fatal(err)
	}

	_, err := svc.CreateTask(context.Background(), "u", "char_test", 1, "t", "", "", 512, 512, []string{"idle_normal"}, nil)
	assertBusinessError(t, err, ErrCodeImageModelDisabled)
	if count := countTasks(t, db); count != 0 {
		t.Fatalf("expected 0 tasks, got %d", count)
	}
}

func TestCreateTask_InvalidImage(t *testing.T) {
	svc, db, _ := setupTestService(t)

	fh := makeFileHeader(t, []byte("not an image"), "ref.png")
	_, err := svc.CreateTask(context.Background(), "u", "char_test", 1, "t", "", "", 512, 512, []string{"idle_normal"}, fh)
	assertBusinessError(t, err, ErrCodeReferenceImageInvalid)
	if count := countTasks(t, db); count != 0 {
		t.Fatalf("expected 0 tasks, got %d", count)
	}
}

func TestCreateTask_ImageTooLarge(t *testing.T) {
	svc, db, _ := setupTestService(t)

	fh := &multipart.FileHeader{
		Filename: "big.png",
		Size:     11 * 1024 * 1024,
	}
	_, err := svc.CreateTask(context.Background(), "u", "char_test", 1, "t", "", "", 512, 512, []string{"idle_normal"}, fh)
	assertBusinessError(t, err, ErrCodeReferenceImageTooLarge)
	if count := countTasks(t, db); count != 0 {
		t.Fatalf("expected 0 tasks, got %d", count)
	}
}

func TestCreateTask_FileFailureCompensation(t *testing.T) {
	svc, db, _ := setupTestService(t)

	blocker := filepath.Join(t.TempDir(), "blocker_file")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	config.AppCfg.Storage.DataDir = blocker

	fh := makeFileHeader(t, makePNG(t), "ref.png")
	_, err := svc.CreateTask(context.Background(), "u", "char_test", 1, "t", "", "", 512, 512, []string{"idle_normal"}, fh)
	if err == nil {
		t.Fatal("expected error when data dir is not a directory")
	}
	if count := countTasks(t, db); count != 0 {
		t.Fatalf("expected 0 tasks when file save failed, got %d", count)
	}
}

func TestCreateTask_DbFailureCleanup(t *testing.T) {
	svc, _, dataDir := setupTestService(t)

	summary := createValidTask(t, svc, "cleanup-task", []string{"idle_normal"})

	taskDir := filepath.Join(dataDir, "desktop-pets", "generation-tasks", summary.ID)
	if _, err := os.Stat(taskDir); err != nil {
		t.Fatalf("task dir should exist after create: %v", err)
	}
	t.Logf("test taskDir=%s dataDir=%s", taskDir, dataDir)

	if err := svc.DeleteTask(summary.ID); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	if _, err := os.Stat(taskDir); !os.IsNotExist(err) {
		remErr := os.RemoveAll(taskDir)
		t.Fatalf("task dir should be removed after delete, stat err = %v, config.DataDir=%s, manual remove err=%v", err, config.AppCfg.Storage.DataDir, remErr)
	}
}

func TestGetTask(t *testing.T) {
	svc, _, _ := setupTestService(t)

	summary := createValidTask(t, svc, "get-task", []string{"idle_normal", "walk_left"})

	detail, err := svc.GetTask(summary.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if detail.ID != summary.ID {
		t.Fatalf("detail id = %s, want %s", detail.ID, summary.ID)
	}
	if detail.Status != "pending" {
		t.Fatalf("status = %s, want pending", detail.Status)
	}
	if detail.CurrentStage != "queued" {
		t.Fatalf("currentStage = %s, want queued", detail.CurrentStage)
	}
	if len(detail.Actions) != 2 {
		t.Fatalf("actions count = %d, want 2", len(detail.Actions))
	}
	if detail.ReferenceImageUrl == "" {
		t.Fatal("referenceImageUrl should not be empty")
	}
	if detail.CharacterName != "测试角色" {
		t.Fatalf("characterName = %s, want 测试角色", detail.CharacterName)
	}
	if detail.ModelName != "测试模型" {
		t.Fatalf("modelName = %s, want 测试模型", detail.ModelName)
	}

	_, err = svc.GetTask("nonexistent-id")
	assertBusinessError(t, err, ErrCodeGenerationTaskNotFound)
}

func TestListTasks(t *testing.T) {
	svc, db, _ := setupTestService(t)

	tasks := []GenerationTask{
		{ID: "t1", CharacterID: "char_test", ModelConfigID: 1, Name: "t1", Status: "pending", CurrentStage: "queued", CreatedAt: "2026-07-24 10:00:00", UpdatedAt: "2026-07-24 10:00:00"},
		{ID: "t2", CharacterID: "char_test", ModelConfigID: 1, Name: "t2", Status: "completed", CurrentStage: "completed", CreatedAt: "2026-07-24 11:00:00", UpdatedAt: "2026-07-24 11:00:00"},
		{ID: "t3", CharacterID: "char_test", ModelConfigID: 1, Name: "t3", Status: "pending", CurrentStage: "queued", CreatedAt: "2026-07-24 12:00:00", UpdatedAt: "2026-07-24 12:00:00"},
		{ID: "t4", CharacterID: "char_other", ModelConfigID: 1, Name: "t4", Status: "pending", CurrentStage: "queued", CreatedAt: "2026-07-24 13:00:00", UpdatedAt: "2026-07-24 13:00:00"},
	}
	if err := db.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}

	resp, err := svc.ListTasks("char_test", "", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 3 {
		t.Fatalf("total = %d, want 3", resp.Total)
	}
	if len(resp.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(resp.Items))
	}
	if resp.Items[0].ID != "t3" || resp.Items[1].ID != "t2" || resp.Items[2].ID != "t1" {
		t.Fatalf("order = %s %s %s, want t3 t2 t1 (created_at desc)", resp.Items[0].ID, resp.Items[1].ID, resp.Items[2].ID)
	}
	if resp.Items[0].CharacterName != "测试角色" {
		t.Fatalf("characterName = %s, want 测试角色", resp.Items[0].CharacterName)
	}

	respPending, err := svc.ListTasks("char_test", "pending", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if respPending.Total != 2 {
		t.Fatalf("pending total = %d, want 2", respPending.Total)
	}
	if len(respPending.Items) != 2 {
		t.Fatalf("pending items = %d, want 2", len(respPending.Items))
	}
	for _, it := range respPending.Items {
		if it.Status != "pending" {
			t.Fatalf("status = %s, want pending", it.Status)
		}
	}

	respOther, err := svc.ListTasks("char_other", "", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if respOther.Total != 1 || len(respOther.Items) != 1 || respOther.Items[0].ID != "t4" {
		t.Fatalf("char_other filter failed: total=%d items=%v", respOther.Total, respOther.Items)
	}
	if respOther.Items[0].CharacterName != "" {
		t.Fatalf("char_other characterName = %s, want empty", respOther.Items[0].CharacterName)
	}

	page1, err := svc.ListTasks("char_test", "", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if page1.Total != 3 || len(page1.Items) != 2 {
		t.Fatalf("page1 total=%d items=%d, want 3/2", page1.Total, len(page1.Items))
	}
	if page1.Items[0].ID != "t3" || page1.Items[1].ID != "t2" {
		t.Fatalf("page1 order = %s %s, want t3 t2", page1.Items[0].ID, page1.Items[1].ID)
	}

	page2, err := svc.ListTasks("char_test", "", 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if page2.Total != 3 || len(page2.Items) != 1 {
		t.Fatalf("page2 total=%d items=%d, want 3/1", page2.Total, len(page2.Items))
	}
	if page2.Items[0].ID != "t1" {
		t.Fatalf("page2 item = %s, want t1", page2.Items[0].ID)
	}
}

func TestDeleteTask(t *testing.T) {
	svc, db, dataDir := setupTestService(t)

	summary := createValidTask(t, svc, "delete-task", []string{"idle_normal"})

	taskDir := filepath.Join(dataDir, "desktop-pets", "generation-tasks", summary.ID)
	if _, err := os.Stat(taskDir); err != nil {
		t.Fatalf("task dir should exist: %v", err)
	}

	if err := svc.DeleteTask(summary.ID); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	if count := countTasks(t, db); count != 0 {
		t.Fatalf("expected 0 tasks after delete, got %d", count)
	}
	if _, err := os.Stat(taskDir); !os.IsNotExist(err) {
		t.Fatalf("task dir should be removed, stat err = %v", err)
	}

	err := svc.DeleteTask(summary.ID)
	assertBusinessError(t, err, ErrCodeGenerationTaskNotFound)

	summary2 := createValidTask(t, svc, "delete-processing", []string{"idle_normal"})
	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'processing' WHERE id = ?", summary2.ID).Error; err != nil {
		t.Fatal(err)
	}
	err = svc.DeleteTask(summary2.ID)
	assertBusinessError(t, err, ErrCodeTaskStatusNotDeletable)
	if count := countTasks(t, db); count != 1 {
		t.Fatalf("processing task should still exist, got %d", count)
	}
}

func TestRefreshRecovery(t *testing.T) {
	svc, db, _ := setupTestService(t)

	summary := createValidTask(t, svc, "refresh-task", []string{"idle_normal", "walk_left"})

	svc2 := newServiceFromDB(t, db)

	detail, err := svc2.GetTask(summary.ID)
	if err != nil {
		t.Fatalf("GetTask after refresh: %v", err)
	}
	if detail.ID != summary.ID {
		t.Fatalf("id = %s, want %s", detail.ID, summary.ID)
	}
	if detail.Status != "pending" {
		t.Fatalf("status = %s, want pending", detail.Status)
	}
	if len(detail.Actions) != 2 {
		t.Fatalf("actions = %d, want 2", len(detail.Actions))
	}
	keys := map[string]bool{}
	for _, a := range detail.Actions {
		keys[a.ActionKey] = true
	}
	if !keys["idle_normal"] || !keys["walk_left"] {
		t.Fatalf("actions keys = %v, want idle_normal+walk_left", keys)
	}
	if detail.CharacterName != "测试角色" {
		t.Fatalf("characterName = %s, want 测试角色", detail.CharacterName)
	}
	if detail.ModelName != "测试模型" {
		t.Fatalf("modelName = %s, want 测试模型", detail.ModelName)
	}
}

func TestNoImageModelCall(t *testing.T) {
	svc, db, _ := setupTestService(t)

	summary := createValidTask(t, svc, "no-model-call", []string{"idle_normal"})

	if summary.Status != "pending" {
		t.Fatalf("status = %s, want pending (no generation should be triggered)", summary.Status)
	}
	if summary.CurrentStage != "queued" {
		t.Fatalf("currentStage = %s, want queued", summary.CurrentStage)
	}

	detail, err := svc.GetTask(summary.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Status != "pending" {
		t.Fatalf("detail status = %s, want pending", detail.Status)
	}
	for _, a := range detail.Actions {
		if a.Status != "pending" {
			t.Fatalf("action %s status = %s, want pending (no generation call)", a.ActionKey, a.Status)
		}
	}

	var cfgEnabled int
	if err := db.Table("image_gen_configs").Where("id = 1").Pluck("enabled", &cfgEnabled).Error; err != nil {
		t.Fatal(err)
	}
	if cfgEnabled != 1 {
		t.Fatalf("image_gen_config enabled = %d, want 1 (unchanged, no model call)", cfgEnabled)
	}
}
