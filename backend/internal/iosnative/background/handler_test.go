package background

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/nativebridge"
)

type mockBackgroundBridge struct {
	response nativebridge.Response
	err      error
	calls    []nativebridge.Request
	delay    time.Duration
}

func (m *mockBackgroundBridge) Execute(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error) {
	m.calls = append(m.calls, req)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nativebridge.Response{}, ctx.Err()
		}
	}
	return m.response, m.err
}

func (m *mockBackgroundBridge) Health(context.Context) nativebridge.Health {
	return ""
}

func newMockBackgroundBridge(resp nativebridge.Response, err error) *mockBackgroundBridge {
	return &mockBackgroundBridge{response: resp, err: err}
}

func baseBackgroundRequest(operation string) nativebridge.Request {
	return nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Platform:        "ios",
		Operation:       operation,
		Payload:         map[string]any{},
	}
}

func TestNewBackgroundHandler(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	if h == nil {
		t.Fatal("NewBackgroundHandler returned nil")
	}
}

func TestHandler_Execute_UnknownOperation(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest("background.unknown")
	resp := h.Execute(context.Background(), req)
	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil {
		t.Fatal("expected error object")
	}
	if resp.Error.Code != nativebridge.ErrOperationNotSupported {
		t.Errorf("expected ErrOperationNotSupported, got %s", resp.Error.Code)
	}
}

func TestHandler_Status(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{"supported": true},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if len(bridge.calls) != 1 {
		t.Errorf("expected 1 bridge call, got %d", len(bridge.calls))
	}
}

func TestHandler_TaskRegister_InvalidClass(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationTaskRegister)
	req.Payload["systemClass"] = "invalid_class"
	req.Payload["identifier"] = "com.amitia.refresh"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrBackgroundIdentifierInvalid {
		t.Errorf("expected ErrBackgroundIdentifierInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_TaskRegister_MissingIdentifier(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationTaskRegister)
	req.Payload["systemClass"] = string(BackgroundClassRefresh)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_TaskRegister_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{"success": true},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationTaskRegister)
	req.Payload["systemClass"] = string(BackgroundClassRefresh)
	req.Payload["identifier"] = "com.amitia.refresh"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if bridge.calls[0].Payload["identifier"] != "com.amitia.refresh" {
		t.Error("expected identifier in payload")
	}
}

func TestHandler_TaskSubmit_InvalidClass(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationTaskSubmit)
	req.Payload["systemClass"] = "invalid"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_TaskSubmit_ContinuedNotUserInitiated(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationTaskSubmit)
	req.Payload["systemClass"] = string(BackgroundClassContinued)
	req.Payload["identifierClass"] = "continued_export"
	req.Payload["taskRunId"] = "task-001"
	req.Payload["initiator"] = string(InitiatorScheduler)
	req.Payload["title"] = "Export"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrBackgroundNotUserInitiated {
		t.Errorf("expected ErrBackgroundNotUserInitiated, got %s", resp.Error.Code)
	}
}

func TestHandler_TaskSubmit_ContinuedMissingTaskRunID(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationTaskSubmit)
	req.Payload["systemClass"] = string(BackgroundClassContinued)
	req.Payload["identifierClass"] = "continued_export"
	req.Payload["initiator"] = string(InitiatorUser)
	req.Payload["title"] = "Export"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrBackgroundTaskBindingInvalid {
		t.Errorf("expected ErrBackgroundTaskBindingInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_TaskSubmit_ContinuedSuccess(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{"submitted": true},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationTaskSubmit)
	req.Payload["systemClass"] = string(BackgroundClassContinued)
	req.Payload["identifierClass"] = "continued_export"
	req.Payload["taskRunId"] = "task-001"
	req.Payload["initiator"] = string(InitiatorUser)
	req.Payload["strategy"] = string(ContinuedStrategyQueueIfNeeded)
	req.Payload["title"] = "Exporting backup"
	req.Payload["subtitle"] = "Preparing data"
	req.Payload["networkRequired"] = true
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["title"] != "Exporting backup" {
		t.Error("expected title")
	}
	if sent["strategy"] != string(ContinuedStrategyQueueIfNeeded) {
		t.Error("expected strategy")
	}
}

func TestHandler_TaskSubmit_RefreshMissingIDs(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationTaskSubmit)
	req.Payload["systemClass"] = string(BackgroundClassRefresh)
	req.Payload["identifierClass"] = "maintenance"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_TaskSubmit_RefreshWithTaskDefinitionID(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationTaskSubmit)
	req.Payload["systemClass"] = string(BackgroundClassRefresh)
	req.Payload["identifierClass"] = "maintenance"
	req.Payload["taskDefinitionID"] = "def-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_TaskCancel_InvalidClass(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationTaskCancel)
	req.Payload["systemClass"] = "invalid"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_TaskCancel_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationTaskCancel)
	req.Payload["systemClass"] = string(BackgroundClassRefresh)
	req.Payload["RequestId"] = "req-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_TaskCancelAll_InvalidClass(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationTaskCancelAll)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_TaskCancelAll_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationTaskCancelAll)
	req.Payload["systemClass"] = string(BackgroundClassProcessing)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_TaskGetPending(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{"pending": []any{}},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationTaskGetPending)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_TaskProgress_Invalid(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationTaskProgress)
	req.Payload["taskRunId"] = "task-001"
	req.Payload["totalUnits"] = float64(100)
	req.Payload["completedUnits"] = float64(150)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrBackgroundProgressInvalid {
		t.Errorf("expected ErrBackgroundProgressInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_TaskProgress_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationTaskProgress)
	req.Payload["taskRunId"] = "task-001"
	req.Payload["identifierClass"] = "export"
	req.Payload["totalUnits"] = float64(100)
	req.Payload["completedUnits"] = float64(45)
	req.Payload["phase"] = "exporting"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_TaskExpire_MissingTaskRunID(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationTaskExpire)
	req.Payload["systemClass"] = string(BackgroundClassRefresh)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_TaskExpire_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationTaskExpire)
	req.Payload["systemClass"] = string(BackgroundClassRefresh)
	req.Payload["taskRunId"] = "task-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_TaskComplete_MissingTaskRunID(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationTaskComplete)
	req.Payload["systemClass"] = string(BackgroundClassRefresh)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_TaskComplete_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationTaskComplete)
	req.Payload["systemClass"] = string(BackgroundClassProcessing)
	req.Payload["taskRunId"] = "task-001"
	req.Payload["success"] = true
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_TaskReconcile(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{"reconciled": true},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationTaskReconcile)
	req.Payload["taskRunId"] = "task-001"
	req.Payload["stagingFiles"] = []any{"file1.tmp", "file2.tmp"}
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_RuntimeReadiness(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{"ready": true},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationRuntimeReadiness)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_RuntimeEnsure(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationRuntimeEnsure)
	req.Payload["taskRunId"] = "task-001"
	req.Payload["timeoutMs"] = float64(10000)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["timeoutMs"] != 10000 {
		t.Error("expected timeoutMs")
	}
}

func TestHandler_CheckpointGet_MissingTaskRunID(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationCheckpointGet)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_CheckpointGet_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{"generation": 1},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationCheckpointGet)
	req.Payload["taskRunId"] = "task-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_CheckpointSet_Invalid(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationCheckpointSet)
	req.Payload["taskRunId"] = "task-001"
	req.Payload["generation"] = float64(-1)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_CheckpointSet_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationCheckpointSet)
	req.Payload["taskRunId"] = "task-001"
	req.Payload["generation"] = float64(2)
	req.Payload["lastUnit"] = float64(50)
	req.Payload["phase"] = "exporting"
	req.Payload["checkpointData"] = map[string]any{"key": "value"}
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_CheckpointClear_MissingTaskRunID(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationCheckpointClear)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_CheckpointClear_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationCheckpointClear)
	req.Payload["taskRunId"] = "task-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_BindingGet_MissingTaskRunID(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationBindingGet)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_BindingGet_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{"binding": map[string]any{}},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationBindingGet)
	req.Payload["taskRunId"] = "task-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_FilePickImport(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{"resourceUri": "amitia://temp/file1"},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationFilePickImport)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_FilePickDirectory(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{"rootUri": "amitia://workspace/@mount1"},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationFilePickDirectory)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_FileMountReauthorize_MissingMountID(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationFileMountReauthorize)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrFileGrantInvalid {
		t.Errorf("expected ErrFileGrantInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_FileMountReauthorize_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{"reauthorized": true},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationFileMountReauthorize)
	req.Payload["mountId"] = "mount-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_FileAccessStat_InvalidPath(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationFileAccessStat)
	req.Payload["mountId"] = "mount-001"
	req.Payload["relativePath"] = "../escape"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrFilePathInvalid {
		t.Errorf("expected ErrFilePathInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_FileAccessStat_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{"name": "file.txt"},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationFileAccessStat)
	req.Payload["mountId"] = "mount-001"
	req.Payload["relativePath"] = "docs/file.txt"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_FileAccessList_InvalidMountID(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationFileAccessList)
	req.Payload["relativePath"] = "docs"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_FileAccessList_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{"entries": []any{}},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationFileAccessList)
	req.Payload["mountId"] = "mount-001"
	req.Payload["relativePath"] = "docs"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_FileAccessRead_MissingMountID(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationFileAccessRead)
	req.Payload["relativePath"] = "file.txt"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_FileAccessRead_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{"content": "data"},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationFileAccessRead)
	req.Payload["mountId"] = "mount-001"
	req.Payload["relativePath"] = "docs/file.txt"
	req.Payload["offset"] = float64(0)
	req.Payload["length"] = float64(1024)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_FileAccessWrite_TooLarge(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationFileAccessWrite)
	req.Payload["mountId"] = "mount-001"
	req.Payload["relativePath"] = "docs/file.txt"
	req.Payload["content"] = make([]byte, MaxContentBytes+1)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrFileSizeLimitExceeded {
		t.Errorf("expected ErrFileSizeLimitExceeded, got %s", resp.Error.Code)
	}
}

func TestHandler_FileAccessWrite_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationFileAccessWrite)
	req.Payload["mountId"] = "mount-001"
	req.Payload["relativePath"] = "docs/file.txt"
	req.Payload["content"] = []byte("hello world")
	req.Payload["atomic"] = true
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_FileAccessMkdir_InvalidPath(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationFileAccessMkdir)
	req.Payload["mountId"] = "mount-001"
	req.Payload["relativePath"] = "/absolute"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_FileAccessMkdir_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationFileAccessMkdir)
	req.Payload["mountId"] = "mount-001"
	req.Payload["relativePath"] = "newdir"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_FileAccessRename_InvalidNewName(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationFileAccessRename)
	req.Payload["mountId"] = "mount-001"
	req.Payload["relativePath"] = "docs/old.txt"
	req.Payload["newName"] = "new/path.txt"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrFilePathInvalid {
		t.Errorf("expected ErrFilePathInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_FileAccessRename_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationFileAccessRename)
	req.Payload["mountId"] = "mount-001"
	req.Payload["relativePath"] = "docs/old.txt"
	req.Payload["newName"] = "new.txt"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_FileAccessMove_Traversal(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationFileAccessMove)
	req.Payload["mountId"] = "mount-001"
	req.Payload["relativePath"] = "docs/file.txt"
	req.Payload["newRelativePath"] = "../../escape"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_FileAccessMove_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationFileAccessMove)
	req.Payload["mountId"] = "mount-001"
	req.Payload["relativePath"] = "docs/file.txt"
	req.Payload["newRelativePath"] = "archive/file.txt"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_FileAccessCopy_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationFileAccessCopy)
	req.Payload["mountId"] = "mount-001"
	req.Payload["relativePath"] = "docs/file.txt"
	req.Payload["newRelativePath"] = "backup/file.txt"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_FileAccessDelete_MissingMountID(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationFileAccessDelete)
	req.Payload["relativePath"] = "docs/file.txt"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_FileAccessDelete_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationFileAccessDelete)
	req.Payload["mountId"] = "mount-001"
	req.Payload["relativePath"] = "docs/file.txt"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_FileExport_MissingResourceURI(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationFileExport)
	req.Payload["mountId"] = "mount-001"
	req.Payload["relativePath"] = "output/result.txt"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrFileExportFailed {
		t.Errorf("expected ErrFileExportFailed, got %s", resp.Error.Code)
	}
}

func TestHandler_FileExport_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationFileExport)
	req.Payload["mountId"] = "mount-001"
	req.Payload["relativePath"] = "output/result.txt"
	req.Payload["resourceUri"] = "amitia://temp/result"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_FileMountGet(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{"mount": map[string]any{}},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationFileMountGet)
	req.Payload["mountId"] = "mount-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_FileMountList(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{"mounts": []any{}},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationFileMountList)
	req.Payload["limit"] = float64(10)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_FileMountRemove_MissingMountID(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationFileMountRemove)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_FileMountRemove_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationFileMountRemove)
	req.Payload["mountId"] = "mount-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_FileGetCapabilities_MissingMountID(t *testing.T) {
	h := NewBackgroundHandler(newMockBackgroundBridge(nativebridge.Response{}, nil))
	req := baseBackgroundRequest(OperationFileGetCapabilities)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_FileGetCapabilities_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-bg-001",
		Status:          "ok",
		Result:          map[string]any{"atomicWrite": true},
	}
	bridge := newMockBackgroundBridge(expected, nil)
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationFileGetCapabilities)
	req.Payload["mountId"] = "mount-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_ContextCancel(t *testing.T) {
	bridge := &mockBackgroundBridge{
		delay: 5 * time.Second,
	}
	h := NewBackgroundHandler(bridge)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := baseBackgroundRequest(OperationStatus)
	resp := h.Execute(ctx, req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrTimeout {
		t.Errorf("expected ErrTimeout, got %s", resp.Error.Code)
	}
}

func TestHandler_BridgeError(t *testing.T) {
	bridge := newMockBackgroundBridge(nativebridge.Response{}, errors.New("bridge failed"))
	h := NewBackgroundHandler(bridge)

	req := baseBackgroundRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_BridgeUnavailable(t *testing.T) {
	h := NewBackgroundHandler(nil)
	req := baseBackgroundRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrNativeBridgeUnavailable {
		t.Errorf("expected ErrNativeBridgeUnavailable, got %s", resp.Error.Code)
	}
}

func TestValidateIdentifier(t *testing.T) {
	if err := ValidateIdentifier("com.amitia.refresh"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateIdentifier(""); err == nil {
		t.Error("expected error for empty identifier")
	}
	if err := ValidateIdentifier("id with spaces"); err == nil {
		t.Error("expected error for identifier with spaces")
	}
	longID := make([]byte, MaxIdentifierLength+1)
	if err := ValidateIdentifier(string(longID)); err == nil {
		t.Error("expected error for too long identifier")
	}
}

func TestValidateTaskRunID(t *testing.T) {
	if err := ValidateTaskRunID("task-001"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateTaskRunID(""); err == nil {
		t.Error("expected error for empty taskRunId")
	}
}

func TestValidateRelativePath(t *testing.T) {
	validPaths := []string{"docs/file.txt", "folder/sub/file.md", "."}
	for _, p := range validPaths {
		if err := ValidateRelativePath(p); err != nil {
			t.Errorf("unexpected error for %q: %v", p, err)
		}
	}
	invalidPaths := []string{"../escape", "/absolute", "folder/../file", ""}
	for _, p := range invalidPaths {
		if err := ValidateRelativePath(p); err == nil {
			t.Errorf("expected error for %q", p)
		}
	}
}

func TestValidateSubmission(t *testing.T) {
	err := ValidateSubmission(BackgroundSubmissionRequest{
		SystemClass:      BackgroundClassContinued,
		IdentifierClass:  "export",
		TaskRunID:        "task-001",
		Initiator:        InitiatorUser,
		Title:            "Export",
		Strategy:         ContinuedStrategyQueueIfNeeded,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateSubmission(BackgroundSubmissionRequest{
		SystemClass:      BackgroundClassContinued,
		IdentifierClass:  "export",
		Initiator:        InitiatorScheduler,
	})
	if err == nil {
		t.Error("expected error for scheduler-initiated continued task")
	}

	longTitle := make([]byte, MaxTitleLength+1)
	err = ValidateSubmission(BackgroundSubmissionRequest{
		SystemClass:      BackgroundClassContinued,
		IdentifierClass:  "export",
		TaskRunID:        "task-001",
		Initiator:        InitiatorUser,
		Title:            string(longTitle),
	})
	if err == nil {
		t.Error("expected error for too long title")
	}
}

func TestValidateProgress(t *testing.T) {
	if err := ValidateProgress(BackgroundTaskProgress{
		TaskRunID: "task-001", TotalUnits: 100, CompletedUnits: 50,
	}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateProgress(BackgroundTaskProgress{
		TaskRunID: "task-001", TotalUnits: 100, CompletedUnits: 150,
	}); err == nil {
		t.Error("expected error when completed > total")
	}
	if err := ValidateProgress(BackgroundTaskProgress{
		TotalUnits: 100, CompletedUnits: 50,
	}); err == nil {
		t.Error("expected error for missing taskRunId")
	}
}

func TestValidateCheckpoint(t *testing.T) {
	if err := ValidateCheckpoint(BackgroundCheckpointSetRequest{
		TaskRunID: "task-001", Generation: 1,
	}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateCheckpoint(BackgroundCheckpointSetRequest{
		Generation: 1,
	}); err == nil {
		t.Error("expected error for missing taskRunId")
	}
}

func TestIsValidSystemClass(t *testing.T) {
	for _, c := range AllowedSystemClasses {
		if !IsValidSystemClass(c) {
			t.Errorf("expected %s to be valid", c)
		}
	}
	if IsValidSystemClass("invalid_class") {
		t.Error("expected invalid class")
	}
}

func TestIsValidContinuedStrategy(t *testing.T) {
	for _, s := range AllowedContinuedStrategies {
		if !IsValidContinuedStrategy(s) {
			t.Errorf("expected %s to be valid", s)
		}
	}
	if IsValidContinuedStrategy("invalid_strategy") {
		t.Error("expected invalid strategy")
	}
}

func TestIsValidContinuedInitiator(t *testing.T) {
	for _, i := range AllowedContinuedInitiators {
		if !IsValidContinuedInitiator(i) {
			t.Errorf("expected %s to be valid", i)
		}
	}
	if IsValidContinuedInitiator(InitiatorScheduler) {
		t.Error("expected scheduler to be invalid for continued")
	}
}

func TestClampLimit(t *testing.T) {
	if ClampLimit(0) != DefaultListLimit {
		t.Errorf("expected default %d", DefaultListLimit)
	}
	if ClampLimit(-1) != DefaultListLimit {
		t.Errorf("expected default %d", DefaultListLimit)
	}
	if ClampLimit(50) != 50 {
		t.Error("expected 50")
	}
	if ClampLimit(MaxListLimit+1) != MaxListLimit {
		t.Errorf("expected max %d", MaxListLimit)
	}
}

func TestMapCodeToMessage(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{ErrBackgroundUnavailable, "background tasks are not available on this device"},
		{ErrBackgroundContinuedUnavailable, "continued processing is not available on this device"},
		{ErrBackgroundNotUserInitiated, "continued processing requires user initiation"},
		{ErrFilePickerUIRequired, "document picker must be shown in the foreground"},
		{ErrFileSecurityScopeFailed, "failed to access security-scoped resource"},
		{ErrFilePathInvalid, "invalid file path"},
		{ErrFileSizeLimitExceeded, "file size exceeds limit"},
		{"UNKNOWN_CODE", "UNKNOWN_CODE"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result := MapCodeToMessage(tt.code)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
