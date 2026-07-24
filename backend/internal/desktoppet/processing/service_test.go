// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func setupServiceEnv(t *testing.T) (*service, *gorm.DB, Repository, string) {
	t.Helper()
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)
	dataDir := t.TempDir()
	ctx := &app.AppContext{DB: db, Context: context.Background()}
	svc := NewService(repo, db, ctx, dataDir).(*service)
	return svc, db, repo, dataDir
}

func seedFullGenerationTask(t *testing.T, db *gorm.DB, dataDir, taskID, userID, actionKey string, supportsIdle int) desktoppet.GenerationTaskAction {
	t.Helper()
	seedValidatorTask(t, db, taskID, userID, "succeeded")
	action := desktoppet.GenerationTaskAction{
		ID:                    "gta-" + taskID,
		TaskID:                taskID,
		ActionKey:             actionKey,
		ActionNameSnapshot:    actionKey,
		Status:                "succeeded",
		SupportsDefaultIdle:   supportsIdle,
		CurrentAttempt:        1,
		FrameCount:            2,
		GenerationSpecVersion: "v1",
		SortOrder:             1,
	}
	if err := db.Create(&action).Error; err != nil {
		t.Fatalf("create action: %v", err)
	}

	rel1 := "desktop-pets/generation-tasks/" + taskID + "/generated/" + actionKey + "/attempt-1/raw/frame-0001.png"
	rel2 := "desktop-pets/generation-tasks/" + taskID + "/generated/" + actionKey + "/attempt-1/raw/frame-0002.png"
	abs1 := writeValidatorPNG(t, dataDir, rel1, 64, 64)
	abs2 := writeValidatorPNG(t, dataDir, rel2, 64, 64)
	hash1 := fileSHA256(t, abs1)
	hash2 := fileSHA256(t, abs2)

	seedValidatorFrame(t, db, "gf-"+taskID+"-1", taskID, action.ID, rel1, hash1, 0, 1, "succeeded")
	seedValidatorFrame(t, db, "gf-"+taskID+"-2", taskID, action.ID, rel2, hash2, 1, 1, "succeeded")

	return action
}

func seedProcessingTaskRow(t *testing.T, db *gorm.DB, taskID, genTaskID string, version int, status string) *ProcessingTask {
	t.Helper()
	task := &ProcessingTask{
		ID:                taskID,
		GenerationTaskID:  genTaskID,
		ProcessingVersion: version,
		Status:            status,
		CurrentStage:      status,
		OutputWidth:       512,
		OutputHeight:      512,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create processing task %s: %v", taskID, err)
	}
	return task
}

func seedProcessingActionRow(t *testing.T, db *gorm.DB, actionID, processingTaskID, genActionID, actionKey, status string, excluded int) *ProcessingAction {
	t.Helper()
	action := &ProcessingAction{
		ID:                     actionID,
		ProcessingTaskID:       processingTaskID,
		GenerationTaskActionID: genActionID,
		ActionKey:              actionKey,
		ActionNameSnapshot:     actionKey,
		SourceAttemptNumber:    1,
		Status:                 status,
		Excluded:               excluded,
	}
	if err := db.Create(action).Error; err != nil {
		t.Fatalf("create processing action %s: %v", actionID, err)
	}
	return action
}

func assertProcessingErrorCode(t *testing.T, err error, wantCode string) {
	t.Helper()
	if err == nil {
		t.Fatalf("期望错误但得到 nil")
	}
	var pe *ProcessingError
	if !errors.As(err, &pe) {
		t.Fatalf("期望 *ProcessingError，实际类型 %T: %v", err, err)
	}
	if pe.Code != wantCode {
		t.Fatalf("Code = %s, 期望 %s", pe.Code, wantCode)
	}
}

func TestServiceCreateProcessingTask_Normal(t *testing.T) {
	svc, db, _, dataDir := setupServiceEnv(t)

	seedFullGenerationTask(t, db, dataDir, "gt-create-normal", "user-1", "idle_normal", 1)

	req := &CreateProcessingTaskRequest{
		GenerationTaskID:           "gt-create-normal",
		UserID:                     "user-1",
		OutputWidth:                512,
		OutputHeight:               512,
		TargetCharacterHeightRatio: 0.8,
		AnchorMode:                 "feet_center",
		BackgroundMode:             "remove_background",
		OutputFormat:               "png",
		DefaultFPS:                 10,
	}

	task, err := svc.CreateProcessingTask(req)
	if err != nil {
		t.Fatalf("CreateProcessingTask 失败: %v", err)
	}
	if task == nil {
		t.Fatal("返回的 task 为空")
	}
	if task.Status != "queued" {
		t.Fatalf("Status = %s, 期望 queued", task.Status)
	}
	if task.CurrentStage != "queued" {
		t.Fatalf("CurrentStage = %s, 期望 queued", task.CurrentStage)
	}
	if task.Progress != 0 {
		t.Fatalf("Progress = %d, 期望 0", task.Progress)
	}
	if task.ProcessingVersion != 1 {
		t.Fatalf("ProcessingVersion = %d, 期望 1", task.ProcessingVersion)
	}
	if task.GenerationTaskID != "gt-create-normal" {
		t.Fatalf("GenerationTaskID = %s, 期望 gt-create-normal", task.GenerationTaskID)
	}

	actions, err := svc.repo.ListProcessingActions(task.ID)
	if err != nil {
		t.Fatalf("ListProcessingActions 失败: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("动作数 = %d, 期望 1", len(actions))
	}
	if actions[0].Status != "pending" {
		t.Fatalf("动作 Status = %s, 期望 pending", actions[0].Status)
	}
	if actions[0].ActionKey != "idle_normal" {
		t.Fatalf("动作 ActionKey = %s, 期望 idle_normal", actions[0].ActionKey)
	}
}

func TestServiceCreateProcessingTask_TaskNotFound(t *testing.T) {
	svc, _, _, _ := setupServiceEnv(t)

	req := &CreateProcessingTaskRequest{
		GenerationTaskID: "gt-nonexistent",
		UserID:           "user-1",
	}

	_, err := svc.CreateProcessingTask(req)
	assertProcessingErrorCode(t, err, ErrCodeGenerationTaskNotReady)
}

func TestServiceCreateProcessingTask_NoSuccessfulActions(t *testing.T) {
	svc, db, _, _ := setupServiceEnv(t)

	seedValidatorTask(t, db, "gt-no-success", "user-1", "succeeded")
	seedValidatorAction(t, db, "gta-no-success", "gt-no-success", "idle_normal", "failed", 1, 2, "v1")

	req := &CreateProcessingTaskRequest{
		GenerationTaskID: "gt-no-success",
		UserID:           "user-1",
	}

	_, err := svc.CreateProcessingTask(req)
	assertProcessingErrorCode(t, err, ErrCodeNoSuccessfulActions)
}

func TestServiceCreateProcessingTask_TaskGenerating(t *testing.T) {
	svc, db, _, _ := setupServiceEnv(t)

	seedValidatorTask(t, db, "gt-generating", "user-1", "generating")

	req := &CreateProcessingTaskRequest{
		GenerationTaskID: "gt-generating",
		UserID:           "user-1",
	}

	_, err := svc.CreateProcessingTask(req)
	assertProcessingErrorCode(t, err, ErrCodeGenerationTaskNotReady)
}

func TestServiceGetProcessingTask(t *testing.T) {
	svc, db, _, _ := setupServiceEnv(t)

	seedProcessingTaskRow(t, db, "pt-get", "gt-get", 1, "processing")
	seedProcessingActionRow(t, db, "pa-get-1", "pt-get", "gta-get-1", "idle_normal", "succeeded", 0)
	seedProcessingActionRow(t, db, "pa-get-2", "pt-get", "gta-get-2", "wave", "failed", 0)

	resp, err := svc.GetProcessingTask("pt-get")
	if err != nil {
		t.Fatalf("GetProcessingTask 失败: %v", err)
	}
	if resp == nil {
		t.Fatal("返回的 resp 为空")
	}
	if resp.ProcessingTask == nil {
		t.Fatal("ProcessingTask 为空")
	}
	if resp.ProcessingTask.ID != "pt-get" {
		t.Fatalf("ProcessingTask.ID = %s, 期望 pt-get", resp.ProcessingTask.ID)
	}
	if len(resp.Actions) != 2 {
		t.Fatalf("Actions 数 = %d, 期望 2", len(resp.Actions))
	}
	if resp.QualitySummary.TotalActions != 2 {
		t.Fatalf("TotalActions = %d, 期望 2", resp.QualitySummary.TotalActions)
	}
	if resp.QualitySummary.SucceededActions != 1 {
		t.Fatalf("SucceededActions = %d, 期望 1", resp.QualitySummary.SucceededActions)
	}
	if resp.QualitySummary.FailedActions != 1 {
		t.Fatalf("FailedActions = %d, 期望 1", resp.QualitySummary.FailedActions)
	}
}

func TestServiceCancelProcessingTask(t *testing.T) {
	svc, db, _, _ := setupServiceEnv(t)

	seedProcessingTaskRow(t, db, "pt-cancel", "gt-cancel", 1, "queued")
	seedProcessingActionRow(t, db, "pa-cancel-1", "pt-cancel", "gta-cancel-1", "idle_normal", "pending", 0)
	seedProcessingActionRow(t, db, "pa-cancel-2", "pt-cancel", "gta-cancel-2", "wave", "succeeded", 0)

	if err := svc.CancelProcessingTask("pt-cancel"); err != nil {
		t.Fatalf("CancelProcessingTask 失败: %v", err)
	}

	task, err := svc.repo.GetProcessingTask("pt-cancel")
	if err != nil {
		t.Fatalf("GetProcessingTask 失败: %v", err)
	}
	if task.Status != "cancelled" {
		t.Fatalf("Status = %s, 期望 cancelled", task.Status)
	}
	if task.CancelRequestedAt == "" {
		t.Fatal("CancelRequestedAt 应该被设置")
	}

	actions, err := svc.repo.ListProcessingActions("pt-cancel")
	if err != nil {
		t.Fatalf("ListProcessingActions 失败: %v", err)
	}
	for _, a := range actions {
		if a.ActionKey == "idle_normal" && a.Status != "cancelled" {
			t.Fatalf("pending 动作应被取消, 实际 %s", a.Status)
		}
		if a.ActionKey == "wave" && a.Status != "succeeded" {
			t.Fatalf("已完成动作应保留, 实际 %s", a.Status)
		}
	}
}

func TestServiceCancelProcessingTask_StateConflict(t *testing.T) {
	svc, db, _, _ := setupServiceEnv(t)

	seedProcessingTaskRow(t, db, "pt-cancel-conflict", "gt-conflict", 1, "succeeded")

	err := svc.CancelProcessingTask("pt-cancel-conflict")
	assertProcessingErrorCode(t, err, ErrCodeProcessingTaskStateConflict)
}

func TestServiceRetryProcessingAction(t *testing.T) {
	svc, db, _, _ := setupServiceEnv(t)

	seedProcessingTaskRow(t, db, "pt-retry", "gt-retry", 1, "failed")
	seedProcessingActionRow(t, db, "pa-retry-1", "pt-retry", "gta-retry-1", "idle_normal", "failed", 0)

	originalActions, err := svc.repo.ListProcessingActions("pt-retry")
	if err != nil {
		t.Fatalf("ListProcessingActions 失败: %v", err)
	}
	if len(originalActions) != 1 {
		t.Fatalf("初始动作数 = %d, 期望 1", len(originalActions))
	}

	if err := svc.RetryProcessingAction("pt-retry", "idle_normal"); err != nil {
		t.Fatalf("RetryProcessingAction 失败: %v", err)
	}

	task, err := svc.repo.GetProcessingTask("pt-retry")
	if err != nil {
		t.Fatalf("GetProcessingTask 失败: %v", err)
	}
	if task.Status != "queued" {
		t.Fatalf("Status = %s, 期望 queued", task.Status)
	}

	actions, err := svc.repo.ListProcessingActions("pt-retry")
	if err != nil {
		t.Fatalf("ListProcessingActions 失败: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("重试后动作数 = %d, 期望 2 (保留历史)", len(actions))
	}

	var newAction *ProcessingAction
	for i := range actions {
		if actions[i].Status == "pending" {
			newAction = &actions[i]
			break
		}
	}
	if newAction == nil {
		t.Fatal("未找到 pending 状态的新动作")
	}
	if newAction.ActionKey != "idle_normal" {
		t.Fatalf("新动作 ActionKey = %s, 期望 idle_normal", newAction.ActionKey)
	}
	if newAction.Progress != 0 {
		t.Fatalf("新动作 Progress = %d, 期望 0", newAction.Progress)
	}
}

func TestServiceRetryProcessingAction_NotRetryable(t *testing.T) {
	svc, db, _, _ := setupServiceEnv(t)

	seedProcessingTaskRow(t, db, "pt-retry-pending", "gt-retry-p", 1, "queued")
	seedProcessingActionRow(t, db, "pa-retry-p", "pt-retry-pending", "gta-retry-p", "idle_normal", "pending", 0)

	err := svc.RetryProcessingAction("pt-retry-pending", "idle_normal")
	assertProcessingErrorCode(t, err, ErrCodeProcessingActionNotRetryable)
}

func TestServiceCreatePackage_Normal(t *testing.T) {
	svc, db, _, dataDir := setupServiceEnv(t)

	taskID := "gt-pkg-svc-normal"
	action := seedFullGenerationTask(t, db, dataDir, taskID, "user-1", "idle_normal", 1)

	if err := db.Model(&desktoppet.GenerationTask{}).Where("id = ?", taskID).Update("character_id", "char-svc-1").Error; err != nil {
		t.Fatalf("update character_id: %v", err)
	}

	seedProcessingTaskRow(t, db, "pt-pkg-svc", taskID, 1, "succeeded")
	seedProcessingActionRow(t, db, "pa-pkg-svc", "pt-pkg-svc", action.ID, "idle_normal", "succeeded", 0)

	setupPackagerProcessedAction(t, dataDir, taskID, 1, "idle_normal", 2)
	setupPackagePreviewFile(t, dataDir, taskID, 1)

	req := &CreatePackageRequest{
		ProcessingTaskID: "pt-pkg-svc",
		UserID:           "user-1",
	}

	resp, err := svc.CreatePackage(req)
	if err != nil {
		t.Fatalf("CreatePackage 失败: %v", err)
	}
	if resp == nil {
		t.Fatal("返回的 resp 为空")
	}
	if resp.PackageID == "" {
		t.Fatal("PackageID 为空")
	}
	if resp.PackageHash == "" {
		t.Fatal("PackageHash 为空")
	}
	if resp.Status != "ready" {
		t.Fatalf("Status = %s, 期望 ready", resp.Status)
	}
}

func TestServiceCreatePackage_DefaultActionMissing(t *testing.T) {
	svc, db, _, dataDir := setupServiceEnv(t)

	taskID := "gt-pkg-svc-no-idle"
	action := seedFullGenerationTask(t, db, dataDir, taskID, "user-1", "wave", 0)

	seedProcessingTaskRow(t, db, "pt-pkg-no-idle", taskID, 1, "succeeded")
	seedProcessingActionRow(t, db, "pa-pkg-no-idle", "pt-pkg-no-idle", action.ID, "wave", "succeeded", 0)

	req := &CreatePackageRequest{
		ProcessingTaskID: "pt-pkg-no-idle",
		UserID:           "user-1",
	}

	_, err := svc.CreatePackage(req)
	assertProcessingErrorCode(t, err, ErrCodeDefaultIdleActionUnavailable)
}

func TestServiceCreatePackage_ExcludeDefaultIdleFailed(t *testing.T) {
	svc, db, _, dataDir := setupServiceEnv(t)

	taskID := "gt-pkg-svc-excluded"
	idleAction := seedFullGenerationTask(t, db, dataDir, taskID, "user-1", "idle_normal", 1)

	waveAction := desktoppet.GenerationTaskAction{
		ID:                    "gta-" + taskID + "-wave",
		TaskID:                taskID,
		ActionKey:             "wave",
		ActionNameSnapshot:    "wave",
		Status:                "succeeded",
		SupportsDefaultIdle:   0,
		CurrentAttempt:        1,
		FrameCount:            2,
		GenerationSpecVersion: "v1",
		SortOrder:             2,
	}
	if err := db.Create(&waveAction).Error; err != nil {
		t.Fatalf("create wave action: %v", err)
	}

	rel1 := "desktop-pets/generation-tasks/" + taskID + "/generated/wave/attempt-1/raw/frame-0001.png"
	rel2 := "desktop-pets/generation-tasks/" + taskID + "/generated/wave/attempt-1/raw/frame-0002.png"
	abs1 := writeValidatorPNG(t, dataDir, rel1, 64, 64)
	abs2 := writeValidatorPNG(t, dataDir, rel2, 64, 64)
	seedValidatorFrame(t, db, "gf-"+taskID+"-w1", taskID, waveAction.ID, rel1, fileSHA256(t, abs1), 0, 1, "succeeded")
	seedValidatorFrame(t, db, "gf-"+taskID+"-w2", taskID, waveAction.ID, rel2, fileSHA256(t, abs2), 1, 1, "succeeded")

	seedProcessingTaskRow(t, db, "pt-pkg-excluded", taskID, 1, "succeeded")
	seedProcessingActionRow(t, db, "pa-pkg-excluded-idle", "pt-pkg-excluded", idleAction.ID, "idle_normal", "succeeded", 1)
	seedProcessingActionRow(t, db, "pa-pkg-excluded-wave", "pt-pkg-excluded", waveAction.ID, "wave", "succeeded", 0)

	req := &CreatePackageRequest{
		ProcessingTaskID: "pt-pkg-excluded",
		UserID:           "user-1",
	}

	_, err := svc.CreatePackage(req)
	assertProcessingErrorCode(t, err, ErrCodeDefaultIdleActionUnavailable)
}

func TestServiceSwitchAttempt(t *testing.T) {
	svc, db, _, _ := setupServiceEnv(t)

	taskID := "gt-switch"
	seedValidatorTask(t, db, taskID, "user-1", "succeeded")

	genAction := desktoppet.GenerationTaskAction{
		ID:                    "gta-switch",
		TaskID:                taskID,
		ActionKey:             "idle_normal",
		ActionNameSnapshot:    "idle_normal",
		Status:                "succeeded",
		SupportsDefaultIdle:   1,
		CurrentAttempt:        1,
		AttemptNumber:         2,
		FrameCount:            2,
		GenerationSpecVersion: "v1",
		SortOrder:             1,
	}
	if err := db.Create(&genAction).Error; err != nil {
		t.Fatalf("create gen action: %v", err)
	}

	seedProcessingTaskRow(t, db, "pt-switch", taskID, 1, "succeeded")
	pa := seedProcessingActionRow(t, db, "pa-switch", "pt-switch", genAction.ID, "idle_normal", "succeeded", 0)

	if pa.SourceAttemptNumber != 1 {
		t.Fatalf("初始 SourceAttemptNumber = %d, 期望 1", pa.SourceAttemptNumber)
	}

	if err := svc.SwitchAttempt("pt-switch", "idle_normal", 2); err != nil {
		t.Fatalf("SwitchAttempt 失败: %v", err)
	}

	actions, err := svc.repo.ListProcessingActions("pt-switch")
	if err != nil {
		t.Fatalf("ListProcessingActions 失败: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("动作数 = %d, 期望 1", len(actions))
	}
	if actions[0].SourceAttemptNumber != 2 {
		t.Fatalf("SourceAttemptNumber = %d, 期望 2", actions[0].SourceAttemptNumber)
	}
	if actions[0].Status != "pending" {
		t.Fatalf("Status = %s, 期望 pending", actions[0].Status)
	}

	task, err := svc.repo.GetProcessingTask("pt-switch")
	if err != nil {
		t.Fatalf("GetProcessingTask 失败: %v", err)
	}
	if task.Status != "queued" {
		t.Fatalf("Status = %s, 期望 queued", task.Status)
	}
}

func TestServiceSwitchAttempt_InvalidAttempt(t *testing.T) {
	svc, db, _, _ := setupServiceEnv(t)

	taskID := "gt-switch-invalid"
	seedValidatorTask(t, db, taskID, "user-1", "succeeded")

	genAction := desktoppet.GenerationTaskAction{
		ID:                    "gta-switch-invalid",
		TaskID:                taskID,
		ActionKey:             "idle_normal",
		ActionNameSnapshot:    "idle_normal",
		Status:                "succeeded",
		SupportsDefaultIdle:   1,
		CurrentAttempt:        1,
		AttemptNumber:         2,
		FrameCount:            2,
		GenerationSpecVersion: "v1",
		SortOrder:             1,
	}
	if err := db.Create(&genAction).Error; err != nil {
		t.Fatalf("create gen action: %v", err)
	}

	seedProcessingTaskRow(t, db, "pt-switch-invalid", taskID, 1, "succeeded")
	seedProcessingActionRow(t, db, "pa-switch-invalid", "pt-switch-invalid", genAction.ID, "idle_normal", "succeeded", 0)

	err := svc.SwitchAttempt("pt-switch-invalid", "idle_normal", 3)
	assertProcessingErrorCode(t, err, ErrCodeProcessingInvalidAttempt)
}

func TestServiceExcludeAction_ExcludeUniqueDefaultIdleFailed(t *testing.T) {
	svc, db, _, _ := setupServiceEnv(t)

	taskID := "gt-exclude-unique"
	seedValidatorTask(t, db, taskID, "user-1", "succeeded")

	genAction := desktoppet.GenerationTaskAction{
		ID:                    "gta-exclude-unique",
		TaskID:                taskID,
		ActionKey:             "idle_normal",
		ActionNameSnapshot:    "idle_normal",
		Status:                "succeeded",
		SupportsDefaultIdle:   1,
		CurrentAttempt:        1,
		FrameCount:            2,
		GenerationSpecVersion: "v1",
		SortOrder:             1,
	}
	if err := db.Create(&genAction).Error; err != nil {
		t.Fatalf("create gen action: %v", err)
	}

	seedProcessingTaskRow(t, db, "pt-exclude-unique", taskID, 1, "succeeded")
	seedProcessingActionRow(t, db, "pa-exclude-unique", "pt-exclude-unique", genAction.ID, "idle_normal", "succeeded", 0)

	err := svc.ExcludeAction("pt-exclude-unique", "idle_normal")
	assertProcessingErrorCode(t, err, ErrCodeProcessingExcludedDefault)
}

func TestServiceExcludeAction_Normal(t *testing.T) {
	svc, db, _, _ := setupServiceEnv(t)

	taskID := "gt-exclude-normal"
	seedValidatorTask(t, db, taskID, "user-1", "succeeded")

	idleAction := desktoppet.GenerationTaskAction{
		ID:                    "gta-exclude-idle",
		TaskID:                taskID,
		ActionKey:             "idle_normal",
		ActionNameSnapshot:    "idle_normal",
		Status:                "succeeded",
		SupportsDefaultIdle:   1,
		CurrentAttempt:        1,
		FrameCount:            2,
		GenerationSpecVersion: "v1",
		SortOrder:             1,
	}
	if err := db.Create(&idleAction).Error; err != nil {
		t.Fatalf("create idle action: %v", err)
	}

	waveAction := desktoppet.GenerationTaskAction{
		ID:                    "gta-exclude-wave",
		TaskID:                taskID,
		ActionKey:             "wave",
		ActionNameSnapshot:    "wave",
		Status:                "succeeded",
		SupportsDefaultIdle:   0,
		CurrentAttempt:        1,
		FrameCount:            2,
		GenerationSpecVersion: "v1",
		SortOrder:             2,
	}
	if err := db.Create(&waveAction).Error; err != nil {
		t.Fatalf("create wave action: %v", err)
	}

	seedProcessingTaskRow(t, db, "pt-exclude-normal", taskID, 1, "succeeded")
	seedProcessingActionRow(t, db, "pa-exclude-idle", "pt-exclude-normal", idleAction.ID, "idle_normal", "succeeded", 0)
	paWave := seedProcessingActionRow(t, db, "pa-exclude-wave", "pt-exclude-normal", waveAction.ID, "wave", "succeeded", 0)

	if err := svc.ExcludeAction("pt-exclude-normal", "wave"); err != nil {
		t.Fatalf("ExcludeAction 失败: %v", err)
	}

	var stored ProcessingAction
	if err := db.Where("id = ?", paWave.ID).First(&stored).Error; err != nil {
		t.Fatalf("query action: %v", err)
	}
	if stored.Excluded != 1 {
		t.Fatalf("Excluded = %d, 期望 1", stored.Excluded)
	}
}

func TestServiceGetProcessingTask_NotFound(t *testing.T) {
	svc, _, _, _ := setupServiceEnv(t)

	_, err := svc.GetProcessingTask("nonexistent")
	assertProcessingErrorCode(t, err, ErrCodeProcessingTaskNotFound)
}

func TestServiceRetryProcessingAction_ActionNotFound(t *testing.T) {
	svc, db, _, _ := setupServiceEnv(t)

	seedProcessingTaskRow(t, db, "pt-retry-no-action", "gt-no-action", 1, "failed")

	err := svc.RetryProcessingAction("pt-retry-no-action", "nonexistent")
	assertProcessingErrorCode(t, err, ErrCodeProcessingActionNotFound)
}

func TestServiceCreateProcessingTask_AlreadyRunning(t *testing.T) {
	svc, db, _, dataDir := setupServiceEnv(t)

	seedFullGenerationTask(t, db, dataDir, "gt-already-running", "user-1", "idle_normal", 1)
	seedProcessingTaskRow(t, db, "pt-existing-running", "gt-already-running", 1, "processing")

	req := &CreateProcessingTaskRequest{
		GenerationTaskID: "gt-already-running",
		UserID:           "user-1",
	}

	_, err := svc.CreateProcessingTask(req)
	assertProcessingErrorCode(t, err, ErrCodeProcessingTaskAlreadyRunning)
}

func TestServiceCreateProcessingTask_IncrementVersion(t *testing.T) {
	svc, db, _, dataDir := setupServiceEnv(t)

	seedFullGenerationTask(t, db, dataDir, "gt-increment", "user-1", "idle_normal", 1)
	seedProcessingTaskRow(t, db, "pt-v1", "gt-increment", 1, "succeeded")

	req := &CreateProcessingTaskRequest{
		GenerationTaskID: "gt-increment",
		UserID:           "user-1",
	}

	task, err := svc.CreateProcessingTask(req)
	if err != nil {
		t.Fatalf("CreateProcessingTask 失败: %v", err)
	}
	if task.ProcessingVersion != 2 {
		t.Fatalf("ProcessingVersion = %d, 期望 2", task.ProcessingVersion)
	}
}
