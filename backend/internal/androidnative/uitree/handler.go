package uitree

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Execute(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationSnapshot:
		return h.handleSnapshot(ctx, request)
	case OperationFind:
		return h.handleFind(ctx, request)
	case OperationGet:
		return h.handleGet(ctx, request)
	default:
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    "OPERATION_NOT_SUPPORTED",
				Message: "unsupported UI Tree operation: " + request.Operation,
			},
		}
	}
}

func (h *Handler) handleStatus(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:       "PROVIDER_UNAVAILABLE",
				Message:    "UI Tree service not initialized",
				DomainCode: UI_TREE_UNAVAILABLE,
			},
		}
	}

	status := h.service.Status(ctx)

	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result: map[string]any{
			"available":          status.Available,
			"preferredSource":    status.PreferredSource,
			"availableSources":   status.AvailableSources,
			"accessibilityConnected": status.AccessibilityReady,
			"rootAvailable":      status.RootAvailable,
			"adbAvailable":       status.ADBAvailable,
		},
	}
}

func (h *Handler) handleSnapshot(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:       "PROVIDER_UNAVAILABLE",
				Message:    "UI Tree service not initialized",
				DomainCode: UI_TREE_UNAVAILABLE,
			},
		}
	}

	var req SnapshotRequest
	if request.Payload != nil {
		payloadBytes, err := json.Marshal(request.Payload)
		if err != nil {
			return h.errorResponse(request, UI_TREE_INVALID_REQUEST, "failed to marshal payload")
		}
		if err := json.Unmarshal(payloadBytes, &req); err != nil {
			return h.errorResponse(request, UI_TREE_INVALID_REQUEST, "invalid snapshot payload")
		}
	}

	snapshot, err := h.service.Snapshot(ctx, req)
	if err != nil {
		if treeErr, ok := err.(*Error); ok {
			return h.errorResponseWithCode(request, treeErr.Code, treeErr.Message)
		}
		return h.errorResponse(request, UI_TREE_SNAPSHOT_FAILED, err.Error())
	}

	result := h.snapshotToResult(snapshot)

	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result:          result,
	}
}

func (h *Handler) handleFind(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:       "PROVIDER_UNAVAILABLE",
				Message:    "UI Tree service not initialized",
				DomainCode: UI_TREE_UNAVAILABLE,
			},
		}
	}

	var req FindRequest
	if request.Payload != nil {
		payloadBytes, err := json.Marshal(request.Payload)
		if err != nil {
			return h.errorResponse(request, UI_TREE_INVALID_REQUEST, "failed to marshal payload")
		}
		if err := json.Unmarshal(payloadBytes, &req); err != nil {
			return h.errorResponse(request, UI_TREE_INVALID_REQUEST, "invalid find payload")
		}
	}

	result, err := h.service.Find(ctx, req)
	if err != nil {
		if treeErr, ok := err.(*Error); ok {
			return h.errorResponseWithCode(request, treeErr.Code, treeErr.Message)
		}
		return h.errorResponse(request, UI_TREE_SNAPSHOT_FAILED, err.Error())
	}

	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result: map[string]any{
			"snapshotId":  result.SnapshotID,
			"nodeIds":     result.NodeIDs,
			"count":       result.Count,
		},
	}
}

func (h *Handler) handleGet(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:       "PROVIDER_UNAVAILABLE",
				Message:    "UI Tree service not initialized",
				DomainCode: UI_TREE_UNAVAILABLE,
			},
		}
	}

	var req GetRequest
	if request.Payload != nil {
		payloadBytes, err := json.Marshal(request.Payload)
		if err != nil {
			return h.errorResponse(request, UI_TREE_INVALID_REQUEST, "failed to marshal payload")
		}
		if err := json.Unmarshal(payloadBytes, &req); err != nil {
			return h.errorResponse(request, UI_TREE_INVALID_REQUEST, "invalid get payload")
		}
	}

	if req.SnapshotID == "" {
		return h.errorResponse(request, UI_TREE_INVALID_REQUEST, "snapshotId is required")
	}
	if req.NodeID == "" {
		return h.errorResponse(request, UI_TREE_INVALID_REQUEST, "nodeId is required")
	}

	result, err := h.service.Get(ctx, req)
	if err != nil {
		if treeErr, ok := err.(*Error); ok {
			return h.errorResponseWithCode(request, treeErr.Code, treeErr.Message)
		}
		return h.errorResponse(request, UI_NODE_NOT_FOUND, err.Error())
	}

	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result: map[string]any{
			"snapshotId":  result.SnapshotID,
			"generation":  result.Generation,
			"source":      result.Source,
			"node":        nodeToMap(result.Node),
		},
	}
}

func (h *Handler) snapshotToResult(snapshot UITreeSnapshot) map[string]any {
	windows := make([]map[string]any, 0, len(snapshot.Windows))
	for _, w := range snapshot.Windows {
		windows = append(windows, map[string]any{
			"windowId":    w.WindowID,
			"type":        string(w.Type),
			"packageName": w.PackageName,
			"title":       w.Title,
			"active":      w.Active,
			"focused":     w.Focused,
			"displayId":   w.DisplayID,
			"bounds":      rectToMap(w.Bounds),
			"rootNodeId":  w.RootNodeID,
		})
	}

	nodes := make([]map[string]any, 0, len(snapshot.Nodes))
	for _, n := range snapshot.Nodes {
		nodes = append(nodes, nodeToMap(n))
	}

	return map[string]any{
		"snapshotId":     snapshot.SnapshotID,
		"generation":     snapshot.Generation,
		"source":         snapshot.Source,
		"capturedAt":     snapshot.CapturedAt,
		"activeWindowId": snapshot.ActiveWindowID,
		"windows":        windows,
		"nodes":          nodes,
		"nodeCount":      snapshot.NodeCount,
		"truncated":      snapshot.Truncated,
		"capability": map[string]any{
			"available":           snapshot.Capability.Available,
			"source":              snapshot.Capability.Source,
			"multiWindow":         snapshot.Capability.MultiWindow,
			"stableNodeReference": snapshot.Capability.StableNodeReference,
			"canReadText":         snapshot.Capability.CanReadText,
			"canReadActions":      snapshot.Capability.CanReadActions,
			"supportsFind":        snapshot.Capability.SupportsFind,
			"degraded":            snapshot.Capability.Degraded,
		},
	}
}

func nodeToMap(node UINode) map[string]any {
	return map[string]any{
		"nodeId":             node.NodeID,
		"parentId":           node.ParentID,
		"windowId":           node.WindowID,
		"childIds":           node.ChildIDs,
		"className":          node.ClassName,
		"packageName":        node.PackageName,
		"text":               node.Text,
		"contentDescription": node.ContentDescription,
		"resourceId":         node.ResourceID,
		"role":               node.Role,
		"bounds":             rectToMap(node.Bounds),
		"visibleToUser":      node.VisibleToUser,
		"enabled":            node.Enabled,
		"focusable":          node.Focusable,
		"focused":            node.Focused,
		"selected":           node.Selected,
		"checked":            node.Checked,
		"checkable":          node.Checkable,
		"clickable":          node.Clickable,
		"longClickable":      node.LongClickable,
		"scrollable":         node.Scrollable,
		"editable":           node.Editable,
		"password":           node.Password,
		"actions":            node.Actions,
		"depth":              node.Depth,
	}
}

func rectToMap(r Rect) map[string]any {
	return map[string]any{
		"left":   r.Left,
		"top":    r.Top,
		"right":  r.Right,
		"bottom": r.Bottom,
	}
}

func (h *Handler) errorResponse(request capability.AndroidBridgeRequest, domainCode, message string) capability.AndroidBridgeResponse {
	return h.errorResponseWithCode(request, domainCode, message)
}

func (h *Handler) errorResponseWithCode(request capability.AndroidBridgeRequest, domainCode, message string) capability.AndroidBridgeResponse {
	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "error",
		Error: &capability.AndroidError{
			Code:       MapUITreeErrorToCanonical(domainCode),
			Message:    message,
			DomainCode: domainCode,
		},
	}
}

var _ androidnative.Handler = (*Handler)(nil)
