// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worker

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/internal/desktoppet/specs"
	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/internal/migration"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

const testProviderName = "seedream"

func setupWorkerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "worker_test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlDB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := migration.ApplyBaseline(db); err != nil {
		t.Fatalf("apply baseline: %v", err)
	}
	runner := migration.Runner{DB: db, SkipBackup: true}
	if err := runner.Apply(migration.DefaultMigrations()); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return db
}

func setupWorkerTestEnv(t *testing.T) (*gorm.DB, desktoppet.Repository, *imageprovider.Registry, string) {
	t.Helper()
	db := setupWorkerTestDB(t)
	dataDir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dataDir); err == nil {
		dataDir = resolved
	}
	originalCfg := config.AppCfg
	config.AppCfg = &config.Config{Storage: config.StorageConfig{DataDir: dataDir}}
	t.Cleanup(func() { config.AppCfg = originalCfg })

	ctx := &app.AppContext{DB: db, Context: context.Background()}
	repo := desktoppet.NewRepository(db, ctx)
	registry := imageprovider.NewRegistry()
	return db, repo, registry, dataDir
}

func seedWorkerCharacter(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`INSERT INTO characters(id,name,status) VALUES('char-w','worker-char','enabled')`).Error; err != nil {
		if !strings.Contains(err.Error(), "Duplicate column") && !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("seed character: %v", err)
		}
	}
}

func seedSimpleModelConfig(t *testing.T, db *gorm.DB, id int, apiKey string, enabled int) {
	t.Helper()
	if err := db.Exec(`INSERT INTO image_gen_configs(id,name,api_key,model_name,base_url,is_active,enabled) VALUES(?, 'test-model', ?, 'doubao-seedream-5-0', 'https://ark.cn-beijing.volces.com/api/v3', 1, ?)`, id, apiKey, enabled).Error; err != nil {
		t.Fatalf("seed image_gen_config: %v", err)
	}
}

func makeWorkerPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for x := 0; x < 4; x++ {
		for y := 0; y < 4; y++ {
			img.Set(x, y, color.RGBA{R: 100, G: uint8(x * 30), B: uint8(y * 30), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func writeWorkerReferenceImage(t *testing.T, dataDir, taskID string) string {
	t.Helper()
	rel := filepath.ToSlash(filepath.Join("desktop-pets", "generation-tasks", taskID, "source", "reference.png"))
	abs := filepath.Join(dataDir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, makeWorkerPNG(t), 0644); err != nil {
		t.Fatalf("write reference: %v", err)
	}
	return rel
}

func newQueuedTask(db *gorm.DB, taskID, modelKey string, outputW, outputH int) *desktoppet.GenerationTask {
	now := time.Now().Format("2006-01-02 15:04:05")
	task := &desktoppet.GenerationTask{
		ID:            taskID,
		CharacterID:   "char-w",
		ModelConfigID: 1,
		Name:          "worker-task",
		Status:        "queued",
		CurrentStage:  "queued",
		OutputWidth:   outputW,
		OutputHeight:  outputH,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_ = modelKey
	return task
}

func insertTask(t *testing.T, db *gorm.DB, taskID string, modelConfigID int, sourceRel string) *desktoppet.GenerationTask {
	t.Helper()
	now := time.Now().Format("2006-01-02 15:04:05")
	task := &desktoppet.GenerationTask{
		ID:              taskID,
		CharacterID:     "char-w",
		ModelConfigID:   modelConfigID,
		Name:            "task-" + taskID,
		SourceImagePath: sourceRel,
		OutputWidth:     512,
		OutputHeight:    512,
		Status:          "queued",
		CurrentStage:    "queued",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("insert task: %v", err)
	}
	return task
}

func insertAction(t *testing.T, db *gorm.DB, taskID, actionKey, actionID string, sortOrder int) *desktoppet.GenerationTaskAction {
	t.Helper()
	now := time.Now().Format("2006-01-02 15:04:05")
	spec, ok := specs.GetSpec(actionKey)
	if !ok {
		t.Fatalf("spec not found: %s", actionKey)
	}
	a := &desktoppet.GenerationTaskAction{
		ID:                        actionID,
		TaskID:                    taskID,
		ActionDefinitionID:        1,
		ActionKey:                 actionKey,
		ActionNameSnapshot:        actionKey,
		ActionDescriptionSnapshot: spec.MotionDescription,
		CategoryKeySnapshot:       "idle",
		CategoryNameSnapshot:      "待机",
		DefinitionVersion:         spec.Version,
		SupportsDefaultIdle:       1,
		SortOrder:                 sortOrder,
		FrameCount:                spec.FrameCount,
		EstimatedGenerationCount:  spec.FrameCount,
		Status:                    "queued",
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	if err := db.Create(a).Error; err != nil {
		t.Fatalf("insert action: %v", err)
	}
	return a
}

func assertActionStatus(t *testing.T, db *gorm.DB, actionID, expected string) {
	t.Helper()
	var a desktoppet.GenerationTaskAction
	if err := db.Where("id = ?", actionID).First(&a).Error; err != nil {
		t.Fatalf("query action: %v", err)
	}
	if a.Status != expected {
		t.Fatalf("action %s status = %q, want %q", actionID, a.Status, expected)
	}
}

func assertTaskStatus(t *testing.T, db *gorm.DB, taskID, expected string) {
	t.Helper()
	var task desktoppet.GenerationTask
	if err := db.Where("id = ?", taskID).First(&task).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if task.Status != expected {
		t.Fatalf("task %s status = %q, want %q", taskID, task.Status, expected)
	}
}

func setTaskProcessing(t *testing.T, db *gorm.DB, task *desktoppet.GenerationTask, executionID string) {
	t.Helper()
	now := time.Now().Format("2006-01-02 15:04:05")
	if err := db.Model(&desktoppet.GenerationTask{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
		"status":       "processing",
		"execution_id": executionID,
		"updated_at":   now,
	}).Error; err != nil {
		t.Fatalf("set task processing: %v", err)
	}
	task.Status = "processing"
	task.ExecutionID = executionID
}

type mockProvider struct {
	mu              sync.Mutex
	submitCalls     int32
	capabilityCalls int32
	validateCalls   int32
	capabilities    imageprovider.ImageGenerationCapabilities
	validateErr     error
	submission      *imageprovider.ImageGenerationSubmission
	submitErr       error
	submitErrs      []error
	submissions     []*imageprovider.ImageGenerationSubmission
	capabilitiesErr error
	recordedReqs    []imageprovider.ImageGenerationRequest
}

func (m *mockProvider) ValidateConfig(ctx context.Context, cfg imageprovider.ImageModelConfig) error {
	atomic.AddInt32(&m.validateCalls, 1)
	return m.validateErr
}

func (m *mockProvider) Capabilities(ctx context.Context, cfg imageprovider.ImageModelConfig) (imageprovider.ImageGenerationCapabilities, error) {
	atomic.AddInt32(&m.capabilityCalls, 1)
	if m.capabilitiesErr != nil {
		return imageprovider.ImageGenerationCapabilities{}, m.capabilitiesErr
	}
	if m.capabilities == (imageprovider.ImageGenerationCapabilities{}) {
		return imageprovider.ImageGenerationCapabilities{
			SupportsReferenceImage: true,
			SupportsMultipleImages: false,
			SupportsNegativePrompt: true,
			SupportsSeed:           true,
			SupportsAsyncOperation: false,
			SupportsCancellation:   false,
			MaxReferenceImages:     1,
			MaxOutputImages:        1,
		}, nil
	}
	return m.capabilities, nil
}

func (m *mockProvider) Submit(ctx context.Context, cfg imageprovider.ImageModelConfig, req imageprovider.ImageGenerationRequest) (*imageprovider.ImageGenerationSubmission, error) {
	idx := atomic.AddInt32(&m.submitCalls, 1) - 1
	m.mu.Lock()
	m.recordedReqs = append(m.recordedReqs, req)
	m.mu.Unlock()

	if int(idx) < len(m.submitErrs) && m.submitErrs[idx] != nil {
		return nil, m.submitErrs[idx]
	}
	if int(idx) < len(m.submissions) {
		return m.submissions[idx], nil
	}
	if m.submitErr != nil {
		return nil, m.submitErr
	}
	return m.submission, nil
}

func (m *mockProvider) Query(ctx context.Context, cfg imageprovider.ImageModelConfig, operationID string) (*imageprovider.ImageGenerationResult, error) {
	return nil, errors.New("mock does not support query")
}

func (m *mockProvider) Cancel(ctx context.Context, cfg imageprovider.ImageModelConfig, operationID string) error {
	return errors.New("mock does not support cancel")
}

func (m *mockProvider) submitCallCount() int { return int(atomic.LoadInt32(&m.submitCalls)) }

func successfulSubmission(pngBytes []byte) *imageprovider.ImageGenerationSubmission {
	return &imageprovider.ImageGenerationSubmission{
		Status:      "succeeded",
		OperationID: "op-mock",
		RequestID:   "req-mock",
		Result: &imageprovider.ImageGenerationResult{
			Images: []imageprovider.GeneratedImage{
				{Bytes: pngBytes, MimeType: "image/png", Width: 4, Height: 4},
			},
			Provider:  testProviderName,
			Model:     "doubao-seedream-5-0",
			RequestID: "req-mock",
			Status:    "succeeded",
		},
	}
}

func emptySubmission() *imageprovider.ImageGenerationSubmission {
	return &imageprovider.ImageGenerationSubmission{
		Status:      "succeeded",
		OperationID: "op-mock",
		RequestID:   "req-mock",
		Result: &imageprovider.ImageGenerationResult{
			Images:    nil,
			Provider:  testProviderName,
			RequestID: "req-mock",
			Status:    "succeeded",
		},
	}
}

type codedErr struct {
	code string
	err  error
}

func (c *codedErr) Error() string     { return c.err.Error() }
func (c *codedErr) ErrorCode() string { return c.code }
func (c *codedErr) Unwrap() error     { return c.err }

func errWithCode(code, msg string) error {
	return &codedErr{code: code, err: errors.New(msg)}
}

func TestClaimTask_IdempotentSecondCallFails(t *testing.T) {
	db, repo, _, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-claim-1"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	insertTask(t, db, taskID, 1, sourceRel)

	claimed1, err := repo.ClaimTask(taskID, "worker-A", "exec-A", time.Now().Add(5*time.Minute).Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("first ClaimTask: %v", err)
	}
	if !claimed1 {
		t.Fatal("first ClaimTask should succeed")
	}

	claimed2, err := repo.ClaimTask(taskID, "worker-B", "exec-B", time.Now().Add(5*time.Minute).Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("second ClaimTask error: %v", err)
	}
	if claimed2 {
		t.Fatal("second ClaimTask should fail (already claimed)")
	}

	var task desktoppet.GenerationTask
	if err := db.Where("id = ?", taskID).First(&task).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if task.WorkerID != "worker-A" {
		t.Fatalf("worker_id = %q, want worker-A (first claim wins)", task.WorkerID)
	}
	if task.Status != "processing" {
		t.Fatalf("task status = %q, want processing", task.Status)
	}
}

func TestRunAction_SpecMissingFailsAction(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-spec-missing"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)

	now := time.Now().Format("2006-01-02 15:04:05")
	action := &desktoppet.GenerationTaskAction{
		ID:                  "act-1",
		TaskID:              taskID,
		ActionKey:           "nonexistent_action_key",
		SortOrder:           1,
		FrameCount:          1,
		Status:              "queued",
		SupportsDefaultIdle: 1,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := db.Create(action).Error; err != nil {
		t.Fatalf("create action: %v", err)
	}

	w := NewWorker(db, repo, registry)
	status := w.runAction(context.Background(), task, action)
	if status != "failed" {
		t.Fatalf("status = %q, want failed", status)
	}
	assertActionStatus(t, db, action.ID, "failed")
	var stored desktoppet.GenerationTaskAction
	if err := db.Where("id = ?", action.ID).First(&stored).Error; err != nil {
		t.Fatalf("query action: %v", err)
	}
	if stored.ErrorCode != desktoppet.ErrCodeActionNotFound {
		t.Fatalf("error code = %q, want %q", stored.ErrorCode, desktoppet.ErrCodeActionNotFound)
	}
}

func TestRunAction_ModelConfigMissingFailsAction(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)

	taskID := "task-model-missing"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 999, sourceRel)
	action := insertAction(t, db, taskID, "idle_normal", "act-1", 1)

	w := NewWorker(db, repo, registry)
	status := w.runAction(context.Background(), task, action)
	if status != "failed" {
		t.Fatalf("status = %q, want failed", status)
	}
	var stored desktoppet.GenerationTaskAction
	if err := db.Where("id = ?", action.ID).First(&stored).Error; err != nil {
		t.Fatalf("query action: %v", err)
	}
	if stored.ErrorCode != desktoppet.ErrCodeImageModelNotFound {
		t.Fatalf("error code = %q, want %q", stored.ErrorCode, desktoppet.ErrCodeImageModelNotFound)
	}
}

func TestRunAction_ModelDisabledFailsAction(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 0)

	taskID := "task-model-disabled"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)
	action := insertAction(t, db, taskID, "idle_normal", "act-1", 1)

	w := NewWorker(db, repo, registry)
	status := w.runAction(context.Background(), task, action)
	if status != "failed" {
		t.Fatalf("status = %q, want failed", status)
	}
	var stored desktoppet.GenerationTaskAction
	if err := db.Where("id = ?", action.ID).First(&stored).Error; err != nil {
		t.Fatalf("query action: %v", err)
	}
	if stored.ErrorCode != desktoppet.ErrCodeImageModelDisabled {
		t.Fatalf("error code = %q, want %q", stored.ErrorCode, desktoppet.ErrCodeImageModelDisabled)
	}
}

func TestRunAction_ApiKeyMissingFailsAction(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "", 1)

	taskID := "task-apikey-missing"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)
	action := insertAction(t, db, taskID, "idle_normal", "act-1", 1)

	w := NewWorker(db, repo, registry)
	status := w.runAction(context.Background(), task, action)
	if status != "failed" {
		t.Fatalf("status = %q, want failed", status)
	}
	var stored desktoppet.GenerationTaskAction
	if err := db.Where("id = ?", action.ID).First(&stored).Error; err != nil {
		t.Fatalf("query action: %v", err)
	}
	if stored.ErrorCode != desktoppet.ErrCodeImageModelCredentialMissing {
		t.Fatalf("error code = %q, want %q", stored.ErrorCode, desktoppet.ErrCodeImageModelCredentialMissing)
	}
}

func TestRunAction_ProviderNotRegisteredFailsAction(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-no-provider"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)
	action := insertAction(t, db, taskID, "idle_normal", "act-1", 1)

	w := NewWorker(db, repo, registry)
	status := w.runAction(context.Background(), task, action)
	if status != "failed" {
		t.Fatalf("status = %q, want failed", status)
	}
	var stored desktoppet.GenerationTaskAction
	if err := db.Where("id = ?", action.ID).First(&stored).Error; err != nil {
		t.Fatalf("query action: %v", err)
	}
	if stored.ErrorCode != desktoppet.ErrCodeImageModelUnavailable {
		t.Fatalf("error code = %q, want %q", stored.ErrorCode, desktoppet.ErrCodeImageModelUnavailable)
	}
}

func TestRunAction_CapabilitiesUnsupportedFailsAction(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	mp := &mockProvider{
		capabilities: imageprovider.ImageGenerationCapabilities{
			SupportsReferenceImage: false,
			SupportsMultipleImages: false,
			MaxReferenceImages:     0,
			MaxOutputImages:        1,
		},
	}
	registry.Register(testProviderName, mp)

	taskID := "task-cap-unsupported"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)
	action := insertAction(t, db, taskID, "idle_normal", "act-1", 1)

	w := NewWorker(db, repo, registry)
	status := w.runAction(context.Background(), task, action)
	if status != "failed" {
		t.Fatalf("status = %q, want failed", status)
	}
	var stored desktoppet.GenerationTaskAction
	if err := db.Where("id = ?", action.ID).First(&stored).Error; err != nil {
		t.Fatalf("query action: %v", err)
	}
	if stored.ErrorCode != desktoppet.ErrCodeImageModelCapabilityUnsupported {
		t.Fatalf("error code = %q, want %q", stored.ErrorCode, desktoppet.ErrCodeImageModelCapabilityUnsupported)
	}
}

func TestRunAction_ValidateConfigFailureFailsAction(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	mp := &mockProvider{validateErr: errors.New("config invalid")}
	registry.Register(testProviderName, mp)

	taskID := "task-validate-fail"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)
	action := insertAction(t, db, taskID, "idle_normal", "act-1", 1)

	w := NewWorker(db, repo, registry)
	status := w.runAction(context.Background(), task, action)
	if status != "failed" {
		t.Fatalf("status = %q, want failed", status)
	}
	var stored desktoppet.GenerationTaskAction
	if err := db.Where("id = ?", action.ID).First(&stored).Error; err != nil {
		t.Fatalf("query action: %v", err)
	}
	if stored.ErrorCode != desktoppet.ErrCodeImageGenerationRequestInvalid {
		t.Fatalf("error code = %q, want %q", stored.ErrorCode, desktoppet.ErrCodeImageGenerationRequestInvalid)
	}
}

func TestRunAction_FullSuccessWritesFramesAndImage(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	pngBytes := makeWorkerPNG(t)
	mp := &mockProvider{submission: successfulSubmission(pngBytes)}
	registry.Register(testProviderName, mp)

	taskID := "task-full-success"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)
	action := insertAction(t, db, taskID, "idle_blink", "act-1", 1)

	w := NewWorker(db, repo, registry)
	status := w.runAction(context.Background(), task, action)
	if status != "succeeded" {
		t.Fatalf("status = %q, want succeeded", status)
	}

	assertActionStatus(t, db, action.ID, "succeeded")

	var frames []desktoppet.GenerationFrame
	if err := db.Where("task_action_id = ?", action.ID).Order("frame_index ASC").Find(&frames).Error; err != nil {
		t.Fatalf("query frames: %v", err)
	}
	spec, _ := specs.GetSpec("idle_blink")
	if len(frames) != spec.FrameCount {
		t.Fatalf("frame count = %d, want %d", len(frames), spec.FrameCount)
	}
	for _, f := range frames {
		if f.Status != "succeeded" {
			t.Fatalf("frame %d status = %q, want succeeded", f.FrameIndex, f.Status)
		}
		if f.ResultImagePath == "" {
			t.Fatalf("frame %d missing result image path", f.FrameIndex)
		}
		abs := filepath.Join(dataDir, f.ResultImagePath)
		if _, err := os.Stat(abs); err != nil {
			t.Fatalf("frame %d image file missing: %v", f.FrameIndex, err)
		}
		if f.ResultHash == "" {
			t.Fatalf("frame %d missing hash", f.FrameIndex)
		}
	}

	if mp.submitCallCount() != spec.FrameCount {
		t.Fatalf("submit call count = %d, want %d (one per frame)", mp.submitCallCount(), spec.FrameCount)
	}
}

func TestRunAction_PartialSuccessWhenSomeFramesFail(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	pngBytes := makeWorkerPNG(t)
	successSub := successfulSubmission(pngBytes)
	emptySub := emptySubmission()

	spec, _ := specs.GetSpec("idle_blink")
	errs := make([]error, spec.FrameCount)
	subs := make([]*imageprovider.ImageGenerationSubmission, spec.FrameCount)
	for i := 0; i < spec.FrameCount; i++ {
		if i == 0 {
			subs[i] = successSub
		} else {
			subs[i] = emptySub
			errs[i] = nil
		}
	}
	_ = errs

	calls := int32(0)
	mp := &mockProvider{}
	mp.mu.Lock()
	mp.recordedReqs = nil
	mp.mu.Unlock()
	mp.submission = nil
	mp.submitErr = nil
	mp.submitErrs = nil

	dynamicProvider := &dynamicMockProvider{
		subs:  subs,
		calls: &calls,
	}
	registry.Register(testProviderName, dynamicProvider)

	taskID := "task-partial"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)
	action := insertAction(t, db, taskID, "idle_blink", "act-1", 1)

	w := NewWorker(db, repo, registry)
	status := w.runAction(context.Background(), task, action)
	if status != "partially_succeeded" {
		t.Fatalf("status = %q, want partially_succeeded", status)
	}
	assertActionStatus(t, db, action.ID, "partially_succeeded")
}

type dynamicMockProvider struct {
	subs  []*imageprovider.ImageGenerationSubmission
	calls *int32
	mu    sync.Mutex
}

func (d *dynamicMockProvider) ValidateConfig(ctx context.Context, cfg imageprovider.ImageModelConfig) error {
	return nil
}

func (d *dynamicMockProvider) Capabilities(ctx context.Context, cfg imageprovider.ImageModelConfig) (imageprovider.ImageGenerationCapabilities, error) {
	return imageprovider.ImageGenerationCapabilities{
		SupportsReferenceImage: true,
		SupportsMultipleImages: false,
		MaxReferenceImages:     1,
		MaxOutputImages:        1,
		SupportsNegativePrompt: true,
		SupportsSeed:           true,
	}, nil
}

func (d *dynamicMockProvider) Submit(ctx context.Context, cfg imageprovider.ImageModelConfig, req imageprovider.ImageGenerationRequest) (*imageprovider.ImageGenerationSubmission, error) {
	idx := int(atomic.AddInt32(d.calls, 1)) - 1
	d.mu.Lock()
	defer d.mu.Unlock()
	if idx >= len(d.subs) {
		return nil, nil
	}
	return d.subs[idx], nil
}

func (d *dynamicMockProvider) Query(ctx context.Context, cfg imageprovider.ImageModelConfig, operationID string) (*imageprovider.ImageGenerationResult, error) {
	return nil, errors.New("not supported")
}

func (d *dynamicMockProvider) Cancel(ctx context.Context, cfg imageprovider.ImageModelConfig, operationID string) error {
	return errors.New("not supported")
}

type cutoffMockProvider struct {
	cutoff  int32
	calls   int32
	success *imageprovider.ImageGenerationSubmission
	failErr error
}

func (c *cutoffMockProvider) ValidateConfig(ctx context.Context, cfg imageprovider.ImageModelConfig) error {
	return nil
}

func (c *cutoffMockProvider) Capabilities(ctx context.Context, cfg imageprovider.ImageModelConfig) (imageprovider.ImageGenerationCapabilities, error) {
	return imageprovider.ImageGenerationCapabilities{
		SupportsReferenceImage: true,
		SupportsMultipleImages: false,
		MaxReferenceImages:     1,
		MaxOutputImages:        1,
		SupportsNegativePrompt: true,
		SupportsSeed:           true,
	}, nil
}

func (c *cutoffMockProvider) Submit(ctx context.Context, cfg imageprovider.ImageModelConfig, req imageprovider.ImageGenerationRequest) (*imageprovider.ImageGenerationSubmission, error) {
	idx := atomic.AddInt32(&c.calls, 1)
	if idx <= c.cutoff {
		return nil, c.failErr
	}
	return c.success, nil
}

func (c *cutoffMockProvider) Query(ctx context.Context, cfg imageprovider.ImageModelConfig, operationID string) (*imageprovider.ImageGenerationResult, error) {
	return nil, errors.New("not supported")
}

func (c *cutoffMockProvider) Cancel(ctx context.Context, cfg imageprovider.ImageModelConfig, operationID string) error {
	return errors.New("not supported")
}

func (c *cutoffMockProvider) submitCallCount() int { return int(atomic.LoadInt32(&c.calls)) }

func TestRunAction_AllFramesFailReturnsFailed(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	mp := &mockProvider{submission: emptySubmission()}
	registry.Register(testProviderName, mp)

	taskID := "task-all-fail"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)
	action := insertAction(t, db, taskID, "idle_blink", "act-1", 1)

	w := NewWorker(db, repo, registry)
	status := w.runAction(context.Background(), task, action)
	if status != "failed" {
		t.Fatalf("status = %q, want failed", status)
	}
	assertActionStatus(t, db, action.ID, "failed")
}

func TestRunAction_TransientErrorRetriedWithinMaxRetries(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	pngBytes := makeWorkerPNG(t)
	success := successfulSubmission(pngBytes)
	emptySub := emptySubmission()

	spec, _ := specs.GetSpec("idle_blink")
	subs := make([]*imageprovider.ImageGenerationSubmission, spec.FrameCount)
	for i := range subs {
		if i == 0 {
			subs[i] = emptySub
		} else {
			subs[i] = success
		}
	}

	calls := int32(0)
	dyn := &dynamicMockProvider{subs: subs, calls: &calls}
	registry.Register(testProviderName, dyn)

	taskID := "task-retry-transient"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)
	action := insertAction(t, db, taskID, "idle_blink", "act-1", 1)

	w := NewWorker(db, repo, registry)
	status := w.runAction(context.Background(), task, action)
	if status != "partially_succeeded" {
		t.Fatalf("status = %q, want partially_succeeded", status)
	}

	var frames []desktoppet.GenerationFrame
	if err := db.Where("task_action_id = ?", action.ID).Order("frame_index ASC").Find(&frames).Error; err != nil {
		t.Fatalf("query frames: %v", err)
	}
	if frames[0].Status != "failed" {
		t.Fatalf("frame 0 status = %q, want failed (empty result, no retry)", frames[0].Status)
	}
	if frames[1].Status != "succeeded" {
		t.Fatalf("frame 1 status = %q, want succeeded", frames[1].Status)
	}
}

func TestSubmitWithRetry_AuthErrorNotRetried(t *testing.T) {
	db, repo, registry, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	mp := &mockProvider{
		submitErrs: []error{
			errWithCode(desktoppet.ErrCodeImageGenerationAuthFailed, "auth failed"),
			errWithCode(desktoppet.ErrCodeImageGenerationAuthFailed, "auth failed 2"),
			errWithCode(desktoppet.ErrCodeImageGenerationAuthFailed, "auth failed 3"),
		},
	}
	registry.Register(testProviderName, mp)

	w := NewWorker(db, repo, registry)
	frame := &desktoppet.GenerationFrame{ID: uuid.New().String(), FrameIndex: 0}
	cfg := imageprovider.ImageModelConfig{ApiKey: "k", ModelName: "m", BaseUrl: "https://x.com"}
	req := imageprovider.ImageGenerationRequest{Prompt: "p", OutputCount: 1}

	result, code, _ := w.submitWithRetry(context.Background(), mp, cfg, req, frame)
	if result != nil {
		t.Fatal("expected nil result on auth error")
	}
	if code != desktoppet.ErrCodeImageGenerationAuthFailed {
		t.Fatalf("error code = %q, want %q", code, desktoppet.ErrCodeImageGenerationAuthFailed)
	}
	if calls := mp.submitCallCount(); calls != 1 {
		t.Fatalf("submit call count = %d, want 1 (auth error should not retry)", calls)
	}
}

func TestSubmitWithRetry_TransientErrorRetriedThenSucceeds(t *testing.T) {
	db, repo, registry, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	pngBytes := makeWorkerPNG(t)
	success := successfulSubmission(pngBytes)
	mp := &mockProvider{
		submission: success,
		submitErrs: []error{
			errWithCode(desktoppet.ErrCodeImageGenerationRateLimited, "rate limited"),
			nil,
		},
	}
	registry.Register(testProviderName, mp)

	w := NewWorker(db, repo, registry)
	frame := &desktoppet.GenerationFrame{ID: uuid.New().String(), FrameIndex: 0}
	cfg := imageprovider.ImageModelConfig{ApiKey: "k", ModelName: "m", BaseUrl: "https://x.com"}
	req := imageprovider.ImageGenerationRequest{Prompt: "p", OutputCount: 1}

	result, code, _ := w.submitWithRetry(context.Background(), mp, cfg, req, frame)
	if result == nil {
		t.Fatalf("expected result after retry, code=%q", code)
	}
	if calls := mp.submitCallCount(); calls != 2 {
		t.Fatalf("submit call count = %d, want 2 (1 fail + 1 success)", calls)
	}
}

func TestSubmitWithRetry_TransientErrorExhaustsRetries(t *testing.T) {
	db, repo, registry, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	mp := &mockProvider{
		submitErrs: []error{
			errWithCode(desktoppet.ErrCodeImageGenerationTimeout, "timeout 1"),
			errWithCode(desktoppet.ErrCodeImageGenerationTimeout, "timeout 2"),
			errWithCode(desktoppet.ErrCodeImageGenerationTimeout, "timeout 3"),
		},
	}
	registry.Register(testProviderName, mp)

	w := NewWorker(db, repo, registry)
	frame := &desktoppet.GenerationFrame{ID: uuid.New().String(), FrameIndex: 0}
	cfg := imageprovider.ImageModelConfig{ApiKey: "k", ModelName: "m", BaseUrl: "https://x.com"}
	req := imageprovider.ImageGenerationRequest{Prompt: "p", OutputCount: 1}

	result, code, _ := w.submitWithRetry(context.Background(), mp, cfg, req, frame)
	if result != nil {
		t.Fatal("expected nil result after retries exhausted")
	}
	if code != desktoppet.ErrCodeImageGenerationTimeout {
		t.Fatalf("error code = %q, want %q", code, desktoppet.ErrCodeImageGenerationTimeout)
	}
	if calls := mp.submitCallCount(); calls != maxFrameRetries+1 {
		t.Fatalf("submit call count = %d, want %d (1 + %d retries)", calls, maxFrameRetries+1, maxFrameRetries)
	}
}

func TestSubmitWithRetry_NilSubmissionRetried(t *testing.T) {
	db, repo, registry, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	pngBytes := makeWorkerPNG(t)
	success := successfulSubmission(pngBytes)
	mp := &mockProvider{
		submissions: []*imageprovider.ImageGenerationSubmission{nil, success},
	}
	registry.Register(testProviderName, mp)

	w := NewWorker(db, repo, registry)
	frame := &desktoppet.GenerationFrame{ID: uuid.New().String(), FrameIndex: 0}
	cfg := imageprovider.ImageModelConfig{ApiKey: "k", ModelName: "m", BaseUrl: "https://x.com"}
	req := imageprovider.ImageGenerationRequest{Prompt: "p", OutputCount: 1}

	result, code, _ := w.submitWithRetry(context.Background(), mp, cfg, req, frame)
	if result == nil {
		t.Fatalf("expected result, code=%q", code)
	}
	if calls := mp.submitCallCount(); calls != 2 {
		t.Fatalf("submit call count = %d, want 2 (first nil triggers retry)", calls)
	}
}

func TestSubmitWithRetry_NonSuccessStatusWithTransientCodeRetried(t *testing.T) {
	db, repo, registry, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	pngBytes := makeWorkerPNG(t)
	failedFirst := &imageprovider.ImageGenerationSubmission{
		Status: "succeeded",
		Result: &imageprovider.ImageGenerationResult{
			Status:       "failed",
			ErrorCode:    desktoppet.ErrCodeImageGenerationRateLimited,
			ErrorMessage: "rate limited",
		},
	}
	successSecond := successfulSubmission(pngBytes)
	mp := &mockProvider{
		submission: successSecond,
		submitErrs: []error{
			nil,
			nil,
		},
	}
	mp.submission = nil

	dyn := &dynamicMockProvider{
		subs:  []*imageprovider.ImageGenerationSubmission{failedFirst, successSecond},
		calls: new(int32),
	}
	registry.Register(testProviderName, dyn)

	w := NewWorker(db, repo, registry)
	frame := &desktoppet.GenerationFrame{ID: uuid.New().String(), FrameIndex: 0}
	cfg := imageprovider.ImageModelConfig{ApiKey: "k", ModelName: "m", BaseUrl: "https://x.com"}
	req := imageprovider.ImageGenerationRequest{Prompt: "p", OutputCount: 1}

	result, code, _ := w.submitWithRetry(context.Background(), dyn, cfg, req, frame)
	if result == nil {
		t.Fatalf("expected result after retry, code=%q", code)
	}
	if calls := atomic.LoadInt32(dyn.calls); calls != 2 {
		t.Fatalf("submit call count = %d, want 2", calls)
	}
}

func TestIsTransientError_Codes(t *testing.T) {
	transient := []string{
		desktoppet.ErrCodeImageGenerationTimeout,
		desktoppet.ErrCodeImageGenerationRateLimited,
		desktoppet.ErrCodeImageGenerationProviderRejected,
		desktoppet.ErrCodeImageResultDownloadFailed,
	}
	for _, code := range transient {
		if !isTransientError(code) {
			t.Fatalf("isTransientError(%q) = false, want true", code)
		}
	}
	nonTransient := []string{
		desktoppet.ErrCodeImageGenerationAuthFailed,
		desktoppet.ErrCodeImageGenerationRequestInvalid,
		desktoppet.ErrCodeImageModelCapabilityUnsupported,
		desktoppet.ErrCodeImageModelCredentialMissing,
		"",
		"UNKNOWN_CODE",
	}
	for _, code := range nonTransient {
		if isTransientError(code) {
			t.Fatalf("isTransientError(%q) = true, want false", code)
		}
	}
}

func TestIsNonRetriableError_Codes(t *testing.T) {
	nonRetriable := []string{
		desktoppet.ErrCodeImageGenerationAuthFailed,
		desktoppet.ErrCodeImageGenerationRequestInvalid,
		desktoppet.ErrCodeImageModelCapabilityUnsupported,
		desktoppet.ErrCodeImageModelCredentialMissing,
	}
	for _, code := range nonRetriable {
		if !isNonRetriableError(code) {
			t.Fatalf("isNonRetriableError(%q) = false, want true", code)
		}
	}
	retriable := []string{
		desktoppet.ErrCodeImageGenerationTimeout,
		desktoppet.ErrCodeImageGenerationRateLimited,
		desktoppet.ErrCodeImageGenerationProviderRejected,
		"",
	}
	for _, code := range retriable {
		if isNonRetriableError(code) {
			t.Fatalf("isNonRetriableError(%q) = true, want false", code)
		}
	}
}

func TestFinalizeTask_AllSucceededReturnsSucceeded(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-finalize-ok"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)
	setTaskProcessing(t, db, task, "exec-finalize-ok")

	results := []actionResult{
		{actionID: "a1", status: "succeeded"},
		{actionID: "a2", status: "succeeded"},
	}
	w := NewWorker(db, repo, registry)
	w.finalizeTask(context.Background(), task, results)
	assertTaskStatus(t, db, taskID, "succeeded")

	var stored desktoppet.GenerationTask
	if err := db.Where("id = ?", taskID).First(&stored).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if stored.Progress != 100 {
		t.Fatalf("progress = %d, want 100", stored.Progress)
	}
	if stored.CurrentStage != "completed" {
		t.Fatalf("current_stage = %q, want completed", stored.CurrentStage)
	}
}

func TestFinalizeTask_AllFailedReturnsFailed(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-finalize-failed"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)
	setTaskProcessing(t, db, task, "exec-finalize-failed")

	results := []actionResult{
		{actionID: "a1", status: "failed"},
		{actionID: "a2", status: "failed"},
	}
	w := NewWorker(db, repo, registry)
	w.finalizeTask(context.Background(), task, results)
	assertTaskStatus(t, db, taskID, "failed")

	var stored desktoppet.GenerationTask
	if err := db.Where("id = ?", taskID).First(&stored).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if stored.ErrorCode != desktoppet.ErrCodeGenerationWorkerError {
		t.Fatalf("error code = %q, want %q", stored.ErrorCode, desktoppet.ErrCodeGenerationWorkerError)
	}
}

func TestFinalizeTask_PartialSuccessReturnsPartiallySucceeded(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-finalize-partial"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)
	setTaskProcessing(t, db, task, "exec-finalize-partial")

	results := []actionResult{
		{actionID: "a1", status: "succeeded"},
		{actionID: "a2", status: "failed"},
		{actionID: "a3", status: "partially_succeeded"},
	}
	w := NewWorker(db, repo, registry)
	w.finalizeTask(context.Background(), task, results)
	assertTaskStatus(t, db, taskID, "partially_succeeded")
}

func TestFinalizeTask_AllSkippedReturnsFailed(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-finalize-skipped"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)
	setTaskProcessing(t, db, task, "exec-finalize-skipped")

	results := []actionResult{
		{actionID: "a1", status: "skipped"},
		{actionID: "a2", status: "skipped"},
	}
	w := NewWorker(db, repo, registry)
	w.finalizeTask(context.Background(), task, results)
	assertTaskStatus(t, db, taskID, "failed")
}

func TestFinalizeTask_CancelledWithSuccessReturnsPartiallySucceeded(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-finalize-cancelled-ok"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)
	now := time.Now().Format("2006-01-02 15:04:05")
	if err := db.Model(&desktoppet.GenerationTask{}).Where("id = ?", taskID).Update("cancel_requested_at", now).Error; err != nil {
		t.Fatalf("set cancel: %v", err)
	}
	setTaskProcessing(t, db, task, "exec-finalize-cancelled-ok")

	results := []actionResult{
		{actionID: "a1", status: "succeeded"},
		{actionID: "a2", status: "skipped"},
	}
	w := NewWorker(db, repo, registry)
	w.finalizeTask(context.Background(), task, results)
	assertTaskStatus(t, db, taskID, "partially_succeeded")
}

func TestFinalizeTask_CancelledAllFailedReturnsCancelled(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-finalize-cancelled-fail"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)
	now := time.Now().Format("2006-01-02 15:04:05")
	if err := db.Model(&desktoppet.GenerationTask{}).Where("id = ?", taskID).Update("cancel_requested_at", now).Error; err != nil {
		t.Fatalf("set cancel: %v", err)
	}
	setTaskProcessing(t, db, task, "exec-finalize-cancelled-fail")

	results := []actionResult{
		{actionID: "a1", status: "failed"},
		{actionID: "a2", status: "skipped"},
	}
	w := NewWorker(db, repo, registry)
	w.finalizeTask(context.Background(), task, results)
	assertTaskStatus(t, db, taskID, "cancelled")

	var stored desktoppet.GenerationTask
	if err := db.Where("id = ?", taskID).First(&stored).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if stored.ErrorCode != "" || stored.ErrorMessage != "" {
		t.Fatalf("cancelled task should have empty error, got code=%q msg=%q", stored.ErrorCode, stored.ErrorMessage)
	}
}

func TestRunActions_ActionIsolationOneFailureDoesNotBlockOthers(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	pngBytes := makeWorkerPNG(t)
	spec1, _ := specs.GetSpec("idle_blink")
	spec2, _ := specs.GetSpec("idle_normal")
	_ = spec2

	mp := &cutoffMockProvider{
		cutoff:  int32(spec1.FrameCount),
		success: successfulSubmission(pngBytes),
		failErr: errWithCode(desktoppet.ErrCodeImageGenerationAuthFailed, "auth failed for action1"),
	}
	registry.Register(testProviderName, mp)

	taskID := "task-isolation"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)
	action1 := insertAction(t, db, taskID, "idle_blink", "act-1", 1)
	action2 := insertAction(t, db, taskID, "idle_normal", "act-2", 2)

	w := NewWorker(db, repo, registry)
	results := w.runActions(context.Background(), task)
	if len(results) != 2 {
		t.Fatalf("results count = %d, want 2", len(results))
	}
	if results[0].status != "failed" {
		t.Fatalf("action1 status = %q, want failed", results[0].status)
	}
	if results[1].status != "succeeded" {
		t.Fatalf("action2 status = %q, want succeeded (isolation)", results[1].status)
	}
	assertActionStatus(t, db, action1.ID, "failed")
	assertActionStatus(t, db, action2.ID, "succeeded")

	totalCalls := mp.submitCallCount()
	expectedCalls := spec1.FrameCount + spec2.FrameCount
	if totalCalls != expectedCalls {
		t.Fatalf("total submit calls = %d, want %d (no retry on auth error, one call per frame)", totalCalls, expectedCalls)
	}
}

func TestRunActions_SkipsAlreadyCompletedActions(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	mp := &mockProvider{submission: successfulSubmission(makeWorkerPNG(t))}
	registry.Register(testProviderName, mp)

	taskID := "task-skip-completed"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)

	now := time.Now().Format("2006-01-02 15:04:05")
	completedSpec, _ := specs.GetSpec("idle_blink")
	completedAction := &desktoppet.GenerationTaskAction{
		ID:                  "act-completed",
		TaskID:              taskID,
		ActionKey:           "idle_blink",
		SortOrder:           1,
		FrameCount:          completedSpec.FrameCount,
		Status:              "succeeded",
		SupportsDefaultIdle: 1,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := db.Create(completedAction).Error; err != nil {
		t.Fatalf("create completed action: %v", err)
	}
	pendingAction := insertAction(t, db, taskID, "idle_normal", "act-pending", 2)
	pendingSpec, _ := specs.GetSpec("idle_normal")

	w := NewWorker(db, repo, registry)
	results := w.runActions(context.Background(), task)
	if len(results) != 2 {
		t.Fatalf("results count = %d, want 2", len(results))
	}
	if results[0].status != "succeeded" {
		t.Fatalf("completed action status = %q, want succeeded (preserved)", results[0].status)
	}
	if results[1].status != "succeeded" {
		t.Fatalf("pending action status = %q, want succeeded", results[1].status)
	}
	if calls := mp.submitCallCount(); calls != pendingSpec.FrameCount {
		t.Fatalf("submit calls = %d, want %d (completed action skipped, only pending action invoked)", calls, pendingSpec.FrameCount)
	}
	_ = pendingAction
}

func TestRunActions_CancelledTaskSkipsPendingActions(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	mp := &mockProvider{submission: successfulSubmission(makeWorkerPNG(t))}
	registry.Register(testProviderName, mp)

	taskID := "task-cancel-during"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)
	now := time.Now().Format("2006-01-02 15:04:05")
	if err := db.Model(&desktoppet.GenerationTask{}).Where("id = ?", taskID).Update("cancel_requested_at", now).Error; err != nil {
		t.Fatalf("set cancel: %v", err)
	}
	insertAction(t, db, taskID, "idle_blink", "act-1", 1)

	w := NewWorker(db, repo, registry)
	results := w.runActions(context.Background(), task)
	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}
	if results[0].status != "skipped" {
		t.Fatalf("cancelled task action status = %q, want skipped", results[0].status)
	}
	if calls := mp.submitCallCount(); calls != 0 {
		t.Fatalf("cancelled task should not invoke provider, got %d calls", calls)
	}
}

func TestIsTaskCancelled_FlaggedTaskReturnsTrue(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-cancel-check"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	insertTask(t, db, taskID, 1, sourceRel)

	w := NewWorker(db, repo, registry)
	if w.isTaskCancelled(taskID) {
		t.Fatal("task without cancel flag should not be cancelled")
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	if err := db.Model(&desktoppet.GenerationTask{}).Where("id = ?", taskID).Update("cancel_requested_at", now).Error; err != nil {
		t.Fatalf("set cancel: %v", err)
	}
	if !w.isTaskCancelled(taskID) {
		t.Fatal("task with cancel flag should be cancelled")
	}
}

func TestIsTaskCancelled_NonExistentTaskReturnsFalse(t *testing.T) {
	db, repo, registry, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	w := NewWorker(db, repo, registry)
	if w.isTaskCancelled("nonexistent-task-id") {
		t.Fatal("non-existent task should not be reported as cancelled")
	}
}

func TestSerializeUsage_NilReturnsUnknown(t *testing.T) {
	if got := serializeUsage(nil); got != "unknown" {
		t.Fatalf("serializeUsage(nil) = %q, want unknown", got)
	}
}

func TestSerializeUsage_NonNilReturnsJSON(t *testing.T) {
	u := &imageprovider.GenerationUsage{PromptTokens: 5, TotalTokens: 10}
	got := serializeUsage(u)
	if !strings.Contains(got, "PromptTokens") {
		t.Fatalf("serializeUsage output missing PromptTokens field: %q", got)
	}
	if !strings.Contains(got, "5") {
		t.Fatalf("serializeUsage output missing 5: %q", got)
	}
}

func TestFramePhaseDescription_Helper(t *testing.T) {
	spec, ok := specs.GetSpec("idle_blink")
	if !ok {
		t.Fatal("spec not found")
	}
	if got := framePhaseDescription(spec, 0); got == "" {
		t.Fatal("frame 0 description should not be empty")
	}
	if got := framePhaseDescription(spec, len(spec.FramePhases)+5); got == "" {
		t.Fatal("overflow description should fall back to last phase, not empty")
	}
	last := strings.TrimSpace(spec.FramePhases[len(spec.FramePhases)-1].Description)
	if got := framePhaseDescription(spec, -1); got != last {
		t.Fatalf("frame -1 description = %q, want %q", got, last)
	}
}

func TestErrorCodeOf_CodedError(t *testing.T) {
	err := errWithCode("CUSTOM_CODE", "msg")
	if got := errorCodeOf(err); got != "CUSTOM_CODE" {
		t.Fatalf("errorCodeOf = %q, want CUSTOM_CODE", got)
	}
}

func TestErrorCodeOf_PlainErrorReturnsEmpty(t *testing.T) {
	err := errors.New("plain")
	if got := errorCodeOf(err); got != "" {
		t.Fatalf("errorCodeOf(plain) = %q, want empty", got)
	}
}
