package task_runtime

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	amitialog "log"
)

func retryTaskRunID(origin string) string {
	if origin == "" {
		return ""
	}
	namespace := [16]byte{0x6b, 0xa7, 0xb8, 0x12, 0x9d, 0xad, 0x11, 0xd1, 0x80, 0xb4, 0x00, 0xc0, 0x4f, 0xd4, 0x30, 0xc8}
	h := sha1.New()
	if _, err := h.Write(namespace[:]); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	if _, err := h.Write([]byte("amitia/task-runtime/retry/v1\x00" + origin)); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	value := h.Sum(nil)
	value[6] = value[6]&0x0f | 0x50
	value[8] = value[8]&0x3f | 0x80
	encoded := hex.EncodeToString(value[:16])
	return "tr-" + encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:]
}

func matchingRetryRun(origin, existing *TaskRun) bool {
	return origin != nil && existing != nil && origin.TaskRunID != "" &&
		existing.TaskRunID == retryTaskRunID(origin.TaskRunID) &&
		existing.OperationID == origin.OperationID &&
		existing.InvocationID == origin.InvocationID &&
		existing.TaskDefinitionID == origin.TaskDefinitionID &&
		existing.DefinitionFingerprint == origin.DefinitionFingerprint &&
		existing.ScopeSnapshotID == origin.ScopeSnapshotID &&
		existing.PermissionSnapshotID == origin.PermissionSnapshotID &&
		existing.DependencySnapshotID == origin.DependencySnapshotID &&
		existing.ExtensionID == origin.ExtensionID &&
		existing.ModuleID == origin.ModuleID &&
		existing.InputHash == origin.InputHash &&
		existing.Attempt >= origin.Attempt+1 && existing.Attempt <= origin.MaxAttempts &&
		existing.MaxAttempts == origin.MaxAttempts &&
		existing.Generation >= origin.Generation+1
}

func (s *TaskRuntimeService) existingRetry(ctx context.Context, origin *TaskRun) (*TaskRun, error) {
	candidate, err := s.store.GetTaskRun(ctx, retryTaskRunID(origin.TaskRunID))
	if err != nil {
		if IsTaskErrorCode(err, ErrTaskNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if candidate == nil {
		return nil, nil
	}
	if candidate.TaskRunID != retryTaskRunID(origin.TaskRunID) {
		return nil, NewTaskError(ErrTaskScopeDenied, "重试查询返回了其他任务")
	}
	if !matchingRetryRun(origin, candidate) {
		return nil, NewTaskError(ErrTaskScopeDenied, "重试任务身份与原任务不一致")
	}
	return candidate, nil
}
