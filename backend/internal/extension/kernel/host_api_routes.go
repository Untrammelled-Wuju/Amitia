package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

type hostAPIAuditWriter struct {
	opRepo hostAPIOperationPutter
}

type hostAPIOperationPutter interface {
	PutOperation(ctx context.Context, op hostAPIOperationRecord) error
}

type hostAPIOperationRecord struct {
	OperationID   string
	OperationType string
	ExtensionID   string
	Status        string
	ErrorMessage  string
}

func newHostAPIAuditWriter() *hostAPIAuditWriter {
	return &hostAPIAuditWriter{}
}

func (w *hostAPIAuditWriter) RecordCall(ctx context.Context, request host_api.CallRequest, result host_api.CallResult) {
	log.Printf("[host-api-audit] call=%s method=%s ext=%s status=%s",
		request.CallID, request.Method, request.RuntimeIdentity.ExtensionID, result.Status)
}

type hostAPIPermissionChecker struct {
	broker permissionCheckerAdapter
}

type permissionCheckerAdapter interface {
	CheckHostAPIPermission(ctx context.Context, extID string, requirements []host_api.PermissionRequirement) error
}

func newHostAPIPermissionChecker() *hostAPIPermissionChecker {
	return &hostAPIPermissionChecker{}
}

func (c *hostAPIPermissionChecker) Check(ctx context.Context, identity runtime_supervisor.RuntimeIdentity, requirements []host_api.PermissionRequirement) error {
	return nil
}

type hostAPIScopeChecker struct{}

func newHostAPIScopeChecker() *hostAPIScopeChecker {
	return &hostAPIScopeChecker{}
}

func (c *hostAPIScopeChecker) Check(ctx context.Context, identity runtime_supervisor.RuntimeIdentity, scopeSnapshotID string, policy host_api.ScopePolicy) error {
	return nil
}

func setupDefaultHostAPIRoutes(gateway *host_api.DefaultGateway, eventSvc *event.Service, scheduleSvc *schedule.ScheduleService) error {
	routes := []host_api.Route{
		{
			Method:          host_api.MethodEventEmit,
			Version:         1,
			RiskLevel:       host_api.RiskLow,
			SideEffectLevel: host_api.SideEffectWrite,
			Timeout:         5000000000,
			Handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					EventType string          `json:"eventType"`
					Payload   json.RawMessage `json:"payload"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if eventSvc != nil && p.EventType != "" {
					opts := event.PublishOptions{
						ProducerID:    string(req.RuntimeIdentity.ExtensionID),
						ProducerType:  "extension",
						AggregateType: p.EventType,
						AggregateID:   req.CallID,
					}
					_, _ = eventSvc.Publish(ctx, event.EventTypeID(p.EventType), 1, p.Payload, opts)
				}
				output, _ := json.Marshal(map[string]any{"ok": true, "eventType": p.EventType})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			Method:          host_api.MethodUINotify,
			Version:         1,
			RiskLevel:       host_api.RiskLow,
			SideEffectLevel: host_api.SideEffectNone,
			Timeout:         3000000000,
			Handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				output, _ := json.Marshal(map[string]any{"ok": true})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			Method:          host_api.MethodStateGet,
			Version:         1,
			RiskLevel:       host_api.RiskLow,
			SideEffectLevel: host_api.SideEffectReadOnly,
			Timeout:         5000000000,
			Handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					Key string `json:"key"`
				}
				_ = json.Unmarshal(req.Input, &p)
				output, _ := json.Marshal(map[string]any{
					"key":   p.Key,
					"value": nil,
					"found": false,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			Method:          host_api.MethodStateCAS,
			Version:         1,
			RiskLevel:       host_api.RiskMedium,
			SideEffectLevel: host_api.SideEffectWrite,
			Timeout:         5000000000,
			Handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				output, _ := json.Marshal(map[string]any{"ok": true, "swapped": false})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			Method:          host_api.MethodScheduleCreate,
			Version:         1,
			RiskLevel:       host_api.RiskMedium,
			SideEffectLevel: host_api.SideEffectWrite,
			Timeout:         10000000000,
			Handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				output, _ := json.Marshal(map[string]any{"ok": true, "scheduleId": ""})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			Method:          host_api.MethodScheduleCancel,
			Version:         1,
			RiskLevel:       host_api.RiskMedium,
			SideEffectLevel: host_api.SideEffectWrite,
			Timeout:         5000000000,
			Handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				output, _ := json.Marshal(map[string]any{"ok": true})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			Method:          host_api.MethodCharacterRead,
			Version:         1,
			RiskLevel:       host_api.RiskLow,
			SideEffectLevel: host_api.SideEffectReadOnly,
			Timeout:         5000000000,
			Handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				output, _ := json.Marshal(map[string]any{
					"characterId": req.RuntimeIdentity.ExtensionID,
					"available":   false,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			Method:          host_api.MethodConversationRead,
			Version:         1,
			RiskLevel:       host_api.RiskLow,
			SideEffectLevel: host_api.SideEffectReadOnly,
			Timeout:         5000000000,
			Handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				output, _ := json.Marshal(map[string]any{
					"messages": []any{},
					"hasMore":  false,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			Method:          host_api.MethodMemoryQuery,
			Version:         1,
			RiskLevel:       host_api.RiskLow,
			SideEffectLevel: host_api.SideEffectReadOnly,
			Timeout:         10000000000,
			Handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				output, _ := json.Marshal(map[string]any{
					"results": []any{},
					"total":   0,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			Method:          host_api.MethodResourceOpen,
			Version:         1,
			RiskLevel:       host_api.RiskLow,
			SideEffectLevel: host_api.SideEffectReadOnly,
			Timeout:         5000000000,
			Handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					Path string `json:"path"`
					Mode string `json:"mode"`
				}
				_ = json.Unmarshal(req.Input, &p)
				output, _ := json.Marshal(map[string]any{
					"handleId": fmt.Sprintf("res-%s", req.CallID),
					"path":     p.Path,
					"mode":     p.Mode,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			Method:          host_api.MethodResourceRead,
			Version:         1,
			RiskLevel:       host_api.RiskLow,
			SideEffectLevel: host_api.SideEffectReadOnly,
			Timeout:         10000000000,
			Handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				output, _ := json.Marshal(map[string]any{
					"data":   "",
					"eof":    true,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			Method:          host_api.MethodEventSubscribe,
			Version:         1,
			RiskLevel:       host_api.RiskLow,
			SideEffectLevel: host_api.SideEffectNone,
			Timeout:         5000000000,
			Handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					EventType string `json:"eventType"`
				}
				_ = json.Unmarshal(req.Input, &p)
				output, _ := json.Marshal(map[string]any{
					"ok":        true,
					"subscriptionId": fmt.Sprintf("sub-%s", req.CallID),
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
	}

	for _, route := range routes {
		if err := gateway.RegisterRoute(route); err != nil {
			return fmt.Errorf("host_api: register route %s v%d: %w", route.Method, route.Version, err)
		}
	}

	return nil
}
