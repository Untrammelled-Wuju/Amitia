package uitree

import (
	"context"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/androidnative"
)

type AccessibilitySource struct {
	bridge androidnative.NativeBridge
	policy Policy
}

func NewAccessibilitySource(bridge androidnative.NativeBridge, policy Policy) *AccessibilitySource {
	return &AccessibilitySource{
		bridge: bridge,
		policy: policy,
	}
}

func (s *AccessibilitySource) Type() SourceType {
	return SourceTypeAccessibility
}

func (s *AccessibilitySource) Status(ctx context.Context) SourceStatus {
	if s.bridge == nil {
		return SourceStatus{Type: SourceTypeAccessibility, Available: false, Reason: "bridge not configured"}
	}

	bridgeReq := androidnative.NativeBridgeRequest{
		ProtocolVersion: 1,
		RequestId:       "",
		Operation:       "accessibility.status",
		Payload:         map[string]any{},
	}

	resp, err := s.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return SourceStatus{Type: SourceTypeAccessibility, Available: false, Reason: err.Error()}
	}

	if resp.Error != nil {
		return SourceStatus{Type: SourceTypeAccessibility, Available: false, Reason: resp.Error.Message}
	}

	connected, _ := resp.Result["connected"].(bool)
	canRetrieve, _ := resp.Result["canRetrieveWindowContent"].(bool)

	if !connected {
		return SourceStatus{Type: SourceTypeAccessibility, Available: false, Reason: "accessibility service not connected"}
	}

	return SourceStatus{Type: SourceTypeAccessibility, Available: canRetrieve, Reason: ""}
}

func (s *AccessibilitySource) Snapshot(ctx context.Context, request SnapshotRequest) (RawSnapshot, error) {
	if s.bridge == nil {
		return RawSnapshot{}, &Error{Code: UI_TREE_NATIVE_BRIDGE_UNAVAILABLE, Message: "bridge not configured"}
	}

	payload := map[string]any{
		"includeAllWindows": request.IncludeAllWindows,
		"includeInvisible":  request.IncludeInvisible,
	}
	if request.MaxDepth != nil {
		payload["maxDepth"] = *request.MaxDepth
	}

	bridgeReq := androidnative.NativeBridgeRequest{
		ProtocolVersion: 1,
		RequestId:       "",
		Operation:       OperationSnapshot,
		Payload:         payload,
	}

	resp, err := s.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return RawSnapshot{}, &Error{Code: UI_TREE_SNAPSHOT_FAILED, Message: err.Error()}
	}

	if resp.Error != nil {
		return RawSnapshot{}, &Error{Code: resp.Error.DomainCode, Message: resp.Error.Message}
	}

	return mapAccessibilityRawSnapshot(resp.Result), nil
}

func mapAccessibilityRawSnapshot(result map[string]any) RawSnapshot {
	raw := RawSnapshot{
		Source: SourceTypeAccessibility,
	}

	if v, ok := result["generation"].(float64); ok {
		raw.Generation = int64(v)
	}
	if v, ok := result["capturedAt"].(float64); ok {
		raw.CapturedAt = int64(v)
	} else {
		raw.CapturedAt = time.Now().UnixMilli()
	}
	if v, ok := result["truncated"].(bool); ok {
		raw.Truncated = v
	}
	if v, ok := result["multiWindow"].(bool); ok {
		raw.MultiWindow = v
	}
	raw.StableRef = true

	if windows, ok := result["windows"].([]any); ok {
		raw.RawWindows = make([]map[string]any, 0, len(windows))
		for _, w := range windows {
			if wm, ok := w.(map[string]any); ok {
				raw.RawWindows = append(raw.RawWindows, wm)
			}
		}
	}

	if nodes, ok := result["nodes"].([]any); ok {
		raw.RawNodes = make([]map[string]any, 0, len(nodes))
		for _, n := range nodes {
			if nm, ok := n.(map[string]any); ok {
				raw.RawNodes = append(raw.RawNodes, nm)
			}
		}
	}

	return raw
}

func init() {
	_ = json.Marshal
}
