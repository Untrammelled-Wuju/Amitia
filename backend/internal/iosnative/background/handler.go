package background

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/nativebridge"
)

type BackgroundHandler struct {
	bridge  nativebridge.Bridge
	taskRT  TaskRuntimePort
}

func NewBackgroundHandler(bridge nativebridge.Bridge) *BackgroundHandler {
	return &BackgroundHandler{bridge: bridge}
}

func (h *BackgroundHandler) SetTaskRuntimePort(port TaskRuntimePort) {
	h.taskRT = port
}

func (h *BackgroundHandler) HasTaskRuntimePort() bool {
	return h.taskRT != nil
}

func (h *BackgroundHandler) Execute(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationTaskRegister:
		return h.handleTaskRegister(ctx, request)
	case OperationTaskSubmit:
		return h.handleTaskSubmit(ctx, request)
	case OperationTaskCancel:
		return h.handleTaskCancel(ctx, request)
	case OperationTaskCancelAll:
		return h.handleTaskCancelAll(ctx, request)
	case OperationTaskGetPending:
		return h.handleTaskGetPending(ctx, request)
	case OperationTaskProgress:
		return h.handleTaskProgress(ctx, request)
	case OperationTaskExpire:
		return h.handleTaskExpire(ctx, request)
	case OperationTaskComplete:
		return h.handleTaskComplete(ctx, request)
	case OperationTaskReconcile:
		return h.handleTaskReconcile(ctx, request)
	case OperationRuntimeReadiness:
		return h.handleRuntimeReadiness(ctx, request)
	case OperationRuntimeEnsure:
		return h.handleRuntimeEnsure(ctx, request)
	case OperationCheckpointGet:
		return h.handleCheckpointGet(ctx, request)
	case OperationCheckpointSet:
		return h.handleCheckpointSet(ctx, request)
	case OperationCheckpointClear:
		return h.handleCheckpointClear(ctx, request)
	case OperationBindingGet:
		return h.handleBindingGet(ctx, request)
	case OperationFilePickImport:
		return h.handleFilePickImport(ctx, request)
	case OperationFilePickDirectory:
		return h.handleFilePickDirectory(ctx, request)
	case OperationFileMountReauthorize:
		return h.handleFileMountReauthorize(ctx, request)
	case OperationFileAccessStat:
		return h.handleFileAccessStat(ctx, request)
	case OperationFileAccessList:
		return h.handleFileAccessList(ctx, request)
	case OperationFileAccessRead:
		return h.handleFileAccessRead(ctx, request)
	case OperationFileAccessWrite:
		return h.handleFileAccessWrite(ctx, request)
	case OperationFileAccessMkdir:
		return h.handleFileAccessMkdir(ctx, request)
	case OperationFileAccessRename:
		return h.handleFileAccessRename(ctx, request)
	case OperationFileAccessMove:
		return h.handleFileAccessMove(ctx, request)
	case OperationFileAccessCopy:
		return h.handleFileAccessCopy(ctx, request)
	case OperationFileAccessDelete:
		return h.handleFileAccessDelete(ctx, request)
	case OperationFileExport:
		return h.handleFileExport(ctx, request)
	case OperationFileMountGet:
		return h.handleFileMountGet(ctx, request)
	case OperationFileMountList:
		return h.handleFileMountList(ctx, request)
	case OperationFileMountRemove:
		return h.handleFileMountRemove(ctx, request)
	case OperationFileGetCapabilities:
		return h.handleFileGetCapabilities(ctx, request)
	default:
		return NewBackgroundError(request, nativebridge.ErrOperationNotSupported, fmt.Sprintf("unsupported operation: %s", request.Operation))
	}
}

func (h *BackgroundHandler) bridgeCall(ctx context.Context, request nativebridge.Request, operation string, payload map[string]any) nativebridge.Response {
	if h.bridge == nil {
		return NewBackgroundError(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}
	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       operation,
			Payload:         payload,
		})
		if err != nil {
			done <- NewBackgroundError(request, ErrOutcomeUnknown, err.Error())
			return
		}
		done <- resp
	}()
	select {
	case <-ctx.Done():
		return NewBackgroundError(request, ErrTimeout, operation+" cancelled")
	case resp := <-done:
		return resp
	}
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func getInt64(m map[string]any, key string, defaultVal int64) int64 {
	switch v := m[key].(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	}
	return defaultVal
}

func getInt(m map[string]any, key string, defaultVal int) int {
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return defaultVal
}

func (h *BackgroundHandler) handleStatus(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationStatus, map[string]any{})
}

func (h *BackgroundHandler) handleTaskRegister(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	systemClass := BackgroundSystemClass(getString(request.Payload, "systemClass"))
	identifier := getString(request.Payload, "identifier")
	if !IsValidSystemClass(systemClass) {
		return NewBackgroundError(request, ErrBackgroundIdentifierInvalid, "invalid system class")
	}
	if err := ValidateIdentifier(identifier); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewBackgroundError(request, code, msg)
	}
	return h.bridgeCall(ctx, request, OperationTaskRegister, map[string]any{
		"systemClass": string(systemClass),
		"identifier":  identifier,
	})
}

func (h *BackgroundHandler) handleTaskSubmit(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	req := BackgroundSubmissionRequest{
		SystemClass:           BackgroundSystemClass(getString(request.Payload, "systemClass")),
		TaskRunID:             getString(request.Payload, "taskRunId"),
		TaskDefinitionID:      getString(request.Payload, "taskDefinitionID"),
		Reason:                getString(request.Payload, "reason"),
		Strategy:              ContinuedTaskStrategy(getString(request.Payload, "strategy")),
		Initiator:             TaskInitiator(getString(request.Payload, "initiator")),
		NetworkRequired:       getBool(request.Payload, "networkRequired"),
		ExternalPowerRequired: getBool(request.Payload, "externalPowerRequired"),
		GPURequired:           getBool(request.Payload, "gpuRequired"),
		Title:                 getString(request.Payload, "title"),
		Subtitle:              getString(request.Payload, "subtitle"),
	}
	if err := ValidateSubmission(req); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewBackgroundError(request, code, msg)
	}
	payload := map[string]any{
		"systemClass":           string(req.SystemClass),
		"networkRequired":       req.NetworkRequired,
		"externalPowerRequired": req.ExternalPowerRequired,
		"gpuRequired":           req.GPURequired,
	}
	if req.TaskRunID != "" {
		payload["taskRunId"] = req.TaskRunID
	}
	if req.TaskDefinitionID != "" {
		payload["taskDefinitionId"] = req.TaskDefinitionID
	}
	if req.Reason != "" {
		payload["reason"] = req.Reason
	}
	if req.Strategy != "" {
		payload["strategy"] = string(req.Strategy)
	}
	if req.Initiator != "" {
		payload["initiator"] = req.Initiator
	}
	if req.Title != "" {
		payload["title"] = req.Title
	}
	if req.Subtitle != "" {
		payload["subtitle"] = req.Subtitle
	}
	return h.bridgeCall(ctx, request, OperationTaskSubmit, payload)
}

func (h *BackgroundHandler) handleTaskCancel(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	systemClass := BackgroundSystemClass(getString(request.Payload, "systemClass"))
	if !IsValidSystemClass(systemClass) {
		return NewBackgroundError(request, ErrBackgroundIdentifierInvalid, "invalid system class")
	}
	payload := map[string]any{
		"systemClass": string(systemClass),
	}
	if reqID := getString(request.Payload, "RequestId"); reqID != "" {
		payload["RequestId"] = reqID
	}
	return h.bridgeCall(ctx, request, OperationTaskCancel, payload)
}

func (h *BackgroundHandler) handleTaskCancelAll(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	systemClass := BackgroundSystemClass(getString(request.Payload, "systemClass"))
	if !IsValidSystemClass(systemClass) {
		return NewBackgroundError(request, ErrBackgroundIdentifierInvalid, "invalid system class")
	}
	return h.bridgeCall(ctx, request, OperationTaskCancelAll, map[string]any{
		"systemClass": string(systemClass),
	})
}

func (h *BackgroundHandler) handleTaskGetPending(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationTaskGetPending, map[string]any{
		"systemClass": getString(request.Payload, "systemClass"),
	})
}

func (h *BackgroundHandler) handleTaskProgress(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	progress := BackgroundTaskProgress{
		TaskRunID:      getString(request.Payload, "taskRunId"),
		TotalUnits:     getInt64(request.Payload, "totalUnits", 0),
		CompletedUnits: getInt64(request.Payload, "completedUnits", 0),
		Phase:          getString(request.Payload, "phase"),
	}
	if err := ValidateProgress(progress); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewBackgroundError(request, code, msg)
	}
	if h.taskRT != nil {
		if err := h.taskRT.ReportProgress(ctx, progress.TaskRunID, progress.TotalUnits, progress.CompletedUnits, progress.Phase); err != nil {
			return NewBackgroundError(request, ErrTaskRuntimeError, err.Error())
		}
	}
	return h.bridgeCall(ctx, request, OperationTaskProgress, map[string]any{
		"taskRunId":      progress.TaskRunID,
		"totalUnits":     progress.TotalUnits,
		"completedUnits": progress.CompletedUnits,
		"phase":          progress.Phase,
	})
}

func (h *BackgroundHandler) handleTaskExpire(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	systemClass := BackgroundSystemClass(getString(request.Payload, "systemClass"))
	taskRunID := getString(request.Payload, "taskRunId")
	if !IsValidSystemClass(systemClass) {
		return NewBackgroundError(request, ErrBackgroundIdentifierInvalid, "invalid system class")
	}
	if err := ValidateTaskRunID(taskRunID); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewBackgroundError(request, code, msg)
	}
	if h.taskRT != nil {
		if err := h.taskRT.SignalExpiration(ctx, taskRunID, "execution_window_expiring"); err != nil {
			return NewBackgroundError(request, ErrTaskRuntimeError, err.Error())
		}
	}
	return h.bridgeCall(ctx, request, OperationTaskExpire, map[string]any{
		"systemClass": string(systemClass),
		"taskRunId":   taskRunID,
	})
}

func (h *BackgroundHandler) handleTaskComplete(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	systemClass := BackgroundSystemClass(getString(request.Payload, "systemClass"))
	taskRunID := getString(request.Payload, "taskRunId")
	if !IsValidSystemClass(systemClass) {
		return NewBackgroundError(request, ErrBackgroundIdentifierInvalid, "invalid system class")
	}
	if err := ValidateTaskRunID(taskRunID); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewBackgroundError(request, code, msg)
	}
	success := getBool(request.Payload, "success")
	var errCode, errMsg string
	if !success {
		errCode = getString(request.Payload, "errorCode")
		errMsg = getString(request.Payload, "errorMessage")
	}
	if h.taskRT != nil {
		if err := h.taskRT.CompleteRun(ctx, taskRunID, success, errCode, errMsg); err != nil {
			return NewBackgroundError(request, ErrTaskRuntimeError, err.Error())
		}
	}
	return h.bridgeCall(ctx, request, OperationTaskComplete, map[string]any{
		"systemClass": string(systemClass),
		"taskRunId":   taskRunID,
		"success":     success,
	})
}

func (h *BackgroundHandler) handleTaskReconcile(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationTaskReconcile, map[string]any{
		"taskRunId":    getString(request.Payload, "taskRunId"),
		"stagingFiles": getStringSlice(request.Payload, "stagingFiles"),
	})
}

func (h *BackgroundHandler) handleRuntimeReadiness(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationRuntimeReadiness, map[string]any{
		"taskDefinitionId": getString(request.Payload, "taskDefinitionId"),
		"taskRunId":        getString(request.Payload, "taskRunId"),
	})
}

func (h *BackgroundHandler) handleRuntimeEnsure(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	payload := map[string]any{
		"taskDefinitionId": getString(request.Payload, "taskDefinitionId"),
		"taskRunId":        getString(request.Payload, "taskRunId"),
	}
	if timeoutMs := getInt(request.Payload, "timeoutMs", 0); timeoutMs > 0 {
		payload["timeoutMs"] = timeoutMs
	}
	return h.bridgeCall(ctx, request, OperationRuntimeEnsure, payload)
}

func (h *BackgroundHandler) handleCheckpointGet(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	taskRunID := getString(request.Payload, "taskRunId")
	if err := ValidateTaskRunID(taskRunID); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewBackgroundError(request, code, msg)
	}
	if h.taskRT != nil {
		cp, err := h.taskRT.GetCheckpoint(ctx, taskRunID)
		if err != nil {
			return NewBackgroundError(request, ErrTaskRuntimeError, err.Error())
		}
		result := map[string]any{
			"taskRunId": taskRunID,
			"lastUnit":  cp.LastUnit,
			"phase":     cp.Phase,
		}
		if cp.Data != nil {
			result["data"] = cp.Data
		}
		return nativebridge.Response{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Status:          "success",
			Result:          result,
		}
	}
	return h.bridgeCall(ctx, request, OperationCheckpointGet, map[string]any{
		"taskRunId": taskRunID,
	})
}

func (h *BackgroundHandler) handleCheckpointSet(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	req := BackgroundCheckpointSetRequest{
		TaskRunID:  getString(request.Payload, "taskRunId"),
		Generation: getInt64(request.Payload, "generation", 0),
		LastUnit:   getInt64(request.Payload, "lastUnit", 0),
		Phase:      getString(request.Payload, "phase"),
	}
	if data, ok := request.Payload["checkpointData"].(map[string]any); ok {
		req.CheckpointData = data
	}
	if err := ValidateCheckpoint(req); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewBackgroundError(request, code, msg)
	}
	if h.taskRT != nil {
		cp := CheckpointData{
			LastUnit: req.LastUnit,
			Phase:    req.Phase,
			Data:     req.CheckpointData,
		}
		if err := h.taskRT.SetCheckpoint(ctx, req.TaskRunID, cp); err != nil {
			return NewBackgroundError(request, ErrTaskRuntimeError, err.Error())
		}
		return nativebridge.Response{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Status:          "success",
		}
	}
	payload := map[string]any{
		"taskRunId":  req.TaskRunID,
		"generation": req.Generation,
		"lastUnit":   req.LastUnit,
		"phase":      req.Phase,
	}
	if len(req.CheckpointData) > 0 {
		payload["checkpointData"] = req.CheckpointData
	}
	return h.bridgeCall(ctx, request, OperationCheckpointSet, payload)
}

func (h *BackgroundHandler) handleCheckpointClear(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	taskRunID := getString(request.Payload, "taskRunId")
	if err := ValidateTaskRunID(taskRunID); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewBackgroundError(request, code, msg)
	}
	if h.taskRT != nil {
		if err := h.taskRT.ClearCheckpoint(ctx, taskRunID); err != nil {
			return NewBackgroundError(request, ErrTaskRuntimeError, err.Error())
		}
		return nativebridge.Response{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Status:          "success",
		}
	}
	return h.bridgeCall(ctx, request, OperationCheckpointClear, map[string]any{
		"taskRunId": taskRunID,
	})
}

func (h *BackgroundHandler) handleBindingGet(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	taskRunID := getString(request.Payload, "taskRunId")
	if err := ValidateTaskRunID(taskRunID); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewBackgroundError(request, code, msg)
	}
	return h.bridgeCall(ctx, request, OperationBindingGet, map[string]any{
		"taskRunId": taskRunID,
	})
}

func (h *BackgroundHandler) handleFilePickImport(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationFilePickImport, map[string]any{})
}

func (h *BackgroundHandler) handleFilePickDirectory(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationFilePickDirectory, map[string]any{})
}

func (h *BackgroundHandler) handleFileMountReauthorize(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	mountID := getString(request.Payload, "mountId")
	if mountID == "" {
		return NewBackgroundError(request, ErrFileGrantInvalid, "mountId is required")
	}
	return h.bridgeCall(ctx, request, OperationFileMountReauthorize, map[string]any{
		"mountId": mountID,
	})
}

func (h *BackgroundHandler) handleFileAccessStat(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	req := IOSFileAccessRequest{
		MountID:      getString(request.Payload, "mountId"),
		RelativePath: getString(request.Payload, "relativePath"),
	}
	if err := ValidateStatRequest(req); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewBackgroundError(request, code, msg)
	}
	return h.bridgeCall(ctx, request, OperationFileAccessStat, map[string]any{
		"mountId":      req.MountID,
		"relativePath": req.RelativePath,
	})
}

func (h *BackgroundHandler) handleFileAccessList(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	req := IOSFileAccessRequest{
		MountID:      getString(request.Payload, "mountId"),
		RelativePath: getString(request.Payload, "relativePath"),
	}
	if err := ValidateListRequest(req); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewBackgroundError(request, code, msg)
	}
	return h.bridgeCall(ctx, request, OperationFileAccessList, map[string]any{
		"mountId":      req.MountID,
		"relativePath": req.RelativePath,
	})
}

func (h *BackgroundHandler) handleFileAccessRead(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	req := IOSFileReadRequest{
		MountID:      getString(request.Payload, "mountId"),
		RelativePath: getString(request.Payload, "relativePath"),
		Offset:       getInt64(request.Payload, "offset", 0),
		Length:       getInt64(request.Payload, "length", 0),
	}
	if err := ValidateReadRequest(req); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewBackgroundError(request, code, msg)
	}
	payload := map[string]any{
		"mountId":      req.MountID,
		"relativePath": req.RelativePath,
	}
	if req.Offset > 0 {
		payload["offset"] = req.Offset
	}
	if req.Length > 0 {
		payload["length"] = req.Length
	}
	return h.bridgeCall(ctx, request, OperationFileAccessRead, payload)
}

func (h *BackgroundHandler) handleFileAccessWrite(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	req := IOSFileWriteRequest{
		MountID:      getString(request.Payload, "mountId"),
		RelativePath: getString(request.Payload, "relativePath"),
		Atomic:       getBool(request.Payload, "atomic"),
	}
	if data, ok := request.Payload["contentBase64"].(string); ok {
		req.ContentBase64 = data
	}
	if err := ValidateWriteRequest(req); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewBackgroundError(request, code, msg)
	}
	return h.bridgeCall(ctx, request, OperationFileAccessWrite, map[string]any{
		"mountId":        req.MountID,
		"relativePath":   req.RelativePath,
		"contentBase64":  req.ContentBase64,
		"atomic":         req.Atomic,
	})
}

func (h *BackgroundHandler) handleFileAccessMkdir(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	req := IOSFileMkdirRequest{
		MountID:      getString(request.Payload, "mountId"),
		RelativePath: getString(request.Payload, "relativePath"),
	}
	if err := ValidateMkdirRequest(req); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewBackgroundError(request, code, msg)
	}
	return h.bridgeCall(ctx, request, OperationFileAccessMkdir, map[string]any{
		"mountId":      req.MountID,
		"relativePath": req.RelativePath,
	})
}

func (h *BackgroundHandler) handleFileAccessRename(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	req := IOSFileRenameRequest{
		MountID:      getString(request.Payload, "mountId"),
		RelativePath: getString(request.Payload, "relativePath"),
		NewName:      getString(request.Payload, "newName"),
	}
	if err := ValidateRenameRequest(req); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewBackgroundError(request, code, msg)
	}
	return h.bridgeCall(ctx, request, OperationFileAccessRename, map[string]any{
		"mountId":      req.MountID,
		"relativePath": req.RelativePath,
		"newName":      req.NewName,
	})
}

func (h *BackgroundHandler) handleFileAccessMove(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	req := IOSFileMoveRequest{
		MountID:         getString(request.Payload, "mountId"),
		RelativePath:    getString(request.Payload, "relativePath"),
		NewRelativePath: getString(request.Payload, "newRelativePath"),
	}
	if err := ValidateMoveRequest(req); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewBackgroundError(request, code, msg)
	}
	return h.bridgeCall(ctx, request, OperationFileAccessMove, map[string]any{
		"mountId":         req.MountID,
		"relativePath":    req.RelativePath,
		"newRelativePath": req.NewRelativePath,
	})
}

func (h *BackgroundHandler) handleFileAccessCopy(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	req := IOSFileCopyRequest{
		MountID:         getString(request.Payload, "mountId"),
		RelativePath:    getString(request.Payload, "relativePath"),
		NewRelativePath: getString(request.Payload, "newRelativePath"),
	}
	if err := ValidateCopyRequest(req); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewBackgroundError(request, code, msg)
	}
	return h.bridgeCall(ctx, request, OperationFileAccessCopy, map[string]any{
		"mountId":         req.MountID,
		"relativePath":    req.RelativePath,
		"newRelativePath": req.NewRelativePath,
	})
}

func (h *BackgroundHandler) handleFileAccessDelete(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	req := IOSFileDeleteRequest{
		MountID:      getString(request.Payload, "mountId"),
		RelativePath: getString(request.Payload, "relativePath"),
	}
	if err := ValidateDeleteRequest(req); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewBackgroundError(request, code, msg)
	}
	return h.bridgeCall(ctx, request, OperationFileAccessDelete, map[string]any{
		"mountId":      req.MountID,
		"relativePath": req.RelativePath,
	})
}

func (h *BackgroundHandler) handleFileExport(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	req := IOSFileExportRequest{
		MountID:      getString(request.Payload, "mountId"),
		RelativePath: getString(request.Payload, "relativePath"),
		ResourceURI:  getString(request.Payload, "resourceUri"),
	}
	if err := ValidateExportRequest(req); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewBackgroundError(request, code, msg)
	}
	return h.bridgeCall(ctx, request, OperationFileExport, map[string]any{
		"mountId":      req.MountID,
		"relativePath": req.RelativePath,
		"resourceUri":  req.ResourceURI,
	})
}

func (h *BackgroundHandler) handleFileMountGet(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationFileMountGet, map[string]any{
		"mountId": getString(request.Payload, "mountId"),
	})
}

func (h *BackgroundHandler) handleFileMountList(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	limit := getInt(request.Payload, "limit", DefaultListLimit)
	limit = ClampLimit(limit)
	return h.bridgeCall(ctx, request, OperationFileMountList, map[string]any{
		"limit":  limit,
		"offset": getInt(request.Payload, "offset", 0),
	})
}

func (h *BackgroundHandler) handleFileMountRemove(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	mountID := getString(request.Payload, "mountId")
	if mountID == "" {
		return NewBackgroundError(request, ErrFileGrantInvalid, "mountId is required")
	}
	return h.bridgeCall(ctx, request, OperationFileMountRemove, map[string]any{
		"mountId": mountID,
	})
}

func (h *BackgroundHandler) handleFileGetCapabilities(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	mountID := getString(request.Payload, "mountId")
	if mountID == "" {
		return NewBackgroundError(request, ErrFileGrantInvalid, "mountId is required")
	}
	return h.bridgeCall(ctx, request, OperationFileGetCapabilities, map[string]any{
		"mountId": mountID,
	})
}
