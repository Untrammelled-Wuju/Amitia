package uitree

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type mockUIAccessibilitySource struct {
	statusFunc    func(ctx context.Context) SourceStatus
	snapshotFunc  func(ctx context.Context, request SnapshotRequest) (RawSnapshot, error)
}

func (m *mockUIAccessibilitySource) Type() SourceType {
	return SourceTypeAccessibility
}

func (m *mockUIAccessibilitySource) Status(ctx context.Context) SourceStatus {
	if m.statusFunc != nil {
		return m.statusFunc(ctx)
	}
	return SourceStatus{Type: SourceTypeAccessibility, Available: true}
}

func (m *mockUIAccessibilitySource) Snapshot(ctx context.Context, request SnapshotRequest) (RawSnapshot, error) {
	if m.snapshotFunc != nil {
		return m.snapshotFunc(ctx, request)
	}
	return RawSnapshot{
		Source:     SourceTypeAccessibility,
		Generation: 1,
		CapturedAt: 1000,
		RawNodes: []map[string]any{
			{
				"nodeId":    "node_1",
				"text":      "Hello",
				"clickable": true,
				"bounds":    map[string]any{"left": 0, "top": 0, "right": 100, "bottom": 50},
			},
		},
	}, nil
}

type mockUIRootSource struct {
	statusFunc   func(ctx context.Context) SourceStatus
	snapshotFunc func(ctx context.Context, request SnapshotRequest) (RawSnapshot, error)
}

func (m *mockUIRootSource) Type() SourceType {
	return SourceTypeRoot
}

func (m *mockUIRootSource) Status(ctx context.Context) SourceStatus {
	if m.statusFunc != nil {
		return m.statusFunc(ctx)
	}
	return SourceStatus{Type: SourceTypeRoot, Available: false}
}

func (m *mockUIRootSource) Snapshot(ctx context.Context, request SnapshotRequest) (RawSnapshot, error) {
	if m.snapshotFunc != nil {
		return m.snapshotFunc(ctx, request)
	}
	return RawSnapshot{}, &Error{Code: UI_TREE_ROOT_UNAVAILABLE, Message: "root unavailable"}
}

type mockUIADBSource struct {
	statusFunc   func(ctx context.Context) SourceStatus
	snapshotFunc func(ctx context.Context, request SnapshotRequest) (RawSnapshot, error)
}

func (m *mockUIADBSource) Type() SourceType {
	return SourceTypeADB
}

func (m *mockUIADBSource) Status(ctx context.Context) SourceStatus {
	if m.statusFunc != nil {
		return m.statusFunc(ctx)
	}
	return SourceStatus{Type: SourceTypeADB, Available: false}
}

func (m *mockUIADBSource) Snapshot(ctx context.Context, request SnapshotRequest) (RawSnapshot, error) {
	if m.snapshotFunc != nil {
		return m.snapshotFunc(ctx, request)
	}
	return RawSnapshot{}, &Error{Code: UI_TREE_ADB_UNAVAILABLE, Message: "adb unavailable"}
}

func TestHandler_UnknownOperation(t *testing.T) {
	service := NewService(SourceSet{}, DefaultPolicy())
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       "ui_tree.unknown",
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != "OPERATION_NOT_SUPPORTED" {
		t.Fatalf("expected OPERATION_NOT_SUPPORTED, got %+v", resp.Error)
	}
}

func TestHandler_Status_NilService(t *testing.T) {
	handler := NewHandler(nil)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationStatus,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.DomainCode != UI_TREE_UNAVAILABLE {
		t.Fatalf("expected UI_TREE_UNAVAILABLE, got %+v", resp.Error)
	}
}

func TestHandler_Status_Success(t *testing.T) {
	sources := SourceSet{
		Accessibility: &mockUIAccessibilitySource{},
		Root:          &mockUIRootSource{},
		ADB:           &mockUIADBSource{},
	}
	service := NewService(sources, DefaultPolicy())
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationStatus,
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s", resp.Status)
	}
	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}
	if resp.Result["available"] != true {
		t.Fatalf("expected available true, got %v", resp.Result["available"])
	}
}

func TestHandler_Snapshot_NilService(t *testing.T) {
	handler := NewHandler(nil)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationSnapshot,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.DomainCode != UI_TREE_UNAVAILABLE {
		t.Fatalf("expected UI_TREE_UNAVAILABLE, got %+v", resp.Error)
	}
}

func TestHandler_Snapshot_Success(t *testing.T) {
	sources := SourceSet{
		Accessibility: &mockUIAccessibilitySource{},
	}
	service := NewService(sources, DefaultPolicy())
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationSnapshot,
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s: %+v", resp.Status, resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}
	if resp.Result["source"] != "accessibility" {
		t.Fatalf("expected accessibility source, got %v", resp.Result["source"])
	}
}

func TestHandler_Snapshot_NoSourceAvailable(t *testing.T) {
	sources := SourceSet{
		Accessibility: &mockUIAccessibilitySource{
			statusFunc: func(ctx context.Context) SourceStatus {
				return SourceStatus{Type: SourceTypeAccessibility, Available: false, Reason: "not connected"}
			},
		},
	}
	service := NewService(sources, DefaultPolicy())
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationSnapshot,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.DomainCode != UI_TREE_UNAVAILABLE {
		t.Fatalf("expected UI_TREE_UNAVAILABLE, got %+v", resp.Error)
	}
}

func TestHandler_Find_NilService(t *testing.T) {
	handler := NewHandler(nil)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationFind,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.DomainCode != UI_TREE_UNAVAILABLE {
		t.Fatalf("expected UI_TREE_UNAVAILABLE, got %+v", resp.Error)
	}
}

func TestHandler_Find_Success(t *testing.T) {
	sources := SourceSet{
		Accessibility: &mockUIAccessibilitySource{},
	}
	service := NewService(sources, DefaultPolicy())

	snapshotResp, err := service.Snapshot(context.Background(), SnapshotRequest{})
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if snapshotResp.SnapshotID == "" {
		t.Fatal("expected snapshot ID")
	}

	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationFind,
		Payload: map[string]any{
			"text": "Hello",
		},
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s: %+v", resp.Status, resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}
	if resp.Result["count"] == nil {
		t.Fatal("expected count in result")
	}
}

func TestHandler_Get_NilService(t *testing.T) {
	handler := NewHandler(nil)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationGet,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.DomainCode != UI_TREE_UNAVAILABLE {
		t.Fatalf("expected UI_TREE_UNAVAILABLE, got %+v", resp.Error)
	}
}

func TestHandler_Get_MissingSnapshotID(t *testing.T) {
	sources := SourceSet{
		Accessibility: &mockUIAccessibilitySource{},
	}
	service := NewService(sources, DefaultPolicy())
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationGet,
		Payload: map[string]any{
			"nodeId": "node_1",
		},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.DomainCode != UI_TREE_INVALID_REQUEST {
		t.Fatalf("expected UI_TREE_INVALID_REQUEST, got %+v", resp.Error)
	}
}

func TestHandler_Get_Success(t *testing.T) {
	sources := SourceSet{
		Accessibility: &mockUIAccessibilitySource{},
	}
	service := NewService(sources, DefaultPolicy())

	snapshot, err := service.Snapshot(context.Background(), SnapshotRequest{})
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}

	var nodeID string
	if len(snapshot.Nodes) > 0 {
		nodeID = snapshot.Nodes[0].NodeID
	} else {
		t.Fatal("expected nodes in snapshot")
	}

	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationGet,
		Payload: map[string]any{
			"snapshotId": snapshot.SnapshotID,
			"nodeId":     nodeID,
		},
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s: %+v", resp.Status, resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestHandler_Get_StaleSnapshot(t *testing.T) {
	sources := SourceSet{
		Accessibility: &mockUIAccessibilitySource{},
	}
	service := NewService(sources, DefaultPolicy())
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationGet,
		Payload: map[string]any{
			"snapshotId": "uis_nonexistent",
			"nodeId":     "node_1",
		},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.DomainCode != UI_TREE_SNAPSHOT_NOT_FOUND {
		t.Fatalf("expected UI_TREE_SNAPSHOT_NOT_FOUND, got %+v", resp.Error)
	}
}
