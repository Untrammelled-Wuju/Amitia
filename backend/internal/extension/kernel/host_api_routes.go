package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
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

type ExtensionStateStore interface {
	Get(ctx context.Context, extensionID, moduleID, key string) (json.RawMessage, int64, bool, error)
	Set(ctx context.Context, extensionID, moduleID, key string, value json.RawMessage) (int64, error)
	CAS(ctx context.Context, extensionID, moduleID, key string, expectedVersion int64, newValue json.RawMessage) (bool, int64, error)
	Delete(ctx context.Context, extensionID, moduleID, key string) error
	List(ctx context.Context, extensionID, moduleID string) ([]sqlite.ExtensionStateEntry, error)
}

type CharacterReader interface {
	ReadCharacter(ctx context.Context, characterID string) (json.RawMessage, bool, error)
}

type ConversationReader interface {
	ReadConversation(ctx context.Context, conversationID string, limit int, offset int) ([]json.RawMessage, bool, error)
}

type MemoryQueryService interface {
	Query(ctx context.Context, extensionID string, query string, limit int) ([]json.RawMessage, error)
}

type HostAPIRouteDeps struct {
	StateStore          ExtensionStateStore
	CharacterReader     CharacterReader
	ConversationReader  ConversationReader
	MemoryQueryService  MemoryQueryService
	EventService        *event.Service
	ScheduleService     *schedule.ScheduleService
	ToolFacade          *ToolFacade
	ExecutionKernel     *execution.ExecutionPipeline
	ToolRegistry        *capability.ToolRegistry
	OperationRepository sqlite.OperationRepository
	ExtensionRoot       string
}

type resourceHandle struct {
	handleID string
	realPath string
	file     *os.File
	readOnly bool
	extensionID string
	moduleID    string
}

type resourceHandleTable struct {
	mu      sync.Mutex
	handles map[string]*resourceHandle
}

func newResourceHandleTable() *resourceHandleTable {
	return &resourceHandleTable{
		handles: make(map[string]*resourceHandle),
	}
}

func (t *resourceHandleTable) put(h *resourceHandle) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handles[h.handleID] = h
}

func (t *resourceHandleTable) get(handleID string) (*resourceHandle, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	h, ok := t.handles[handleID]
	return h, ok
}

func (t *resourceHandleTable) remove(handleID string) (*resourceHandle, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	h, ok := t.handles[handleID]
	if ok {
		delete(t.handles, handleID)
	}
	return h, ok
}

func (t *resourceHandleTable) closeAll(extensionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id, h := range t.handles {
		if h.extensionID == extensionID {
			if h.file != nil {
				_ = h.file.Close()
			}
			delete(t.handles, id)
		}
	}
}

const maxToolExecutionDepth = 8

func setupDefaultHostAPIRoutes(gateway *host_api.DefaultGateway, deps HostAPIRouteDeps) error {
	handleTable := newResourceHandleTable()

	type routeDef struct {
		method          host_api.Method
		riskLevel       host_api.RiskLevel
		sideEffectLevel host_api.SideEffectLevel
		timeout         time.Duration
		handler         host_api.Handler
	}

	routes := []routeDef{
		{
			method:          host_api.MethodStateGet,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					Key string `json:"key"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.StateStore == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "state store not configured"},
					}, nil
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				modID := string(req.RuntimeIdentity.ModuleID)
				value, version, found, err := deps.StateStore.Get(ctx, extID, modID, p.Key)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"key":     p.Key,
					"value":   json.RawMessage(value),
					"found":   found,
					"version": version,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodStateCAS,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					Key            string          `json:"key"`
					ExpectedVersion int64          `json:"expectedVersion"`
					NewValue       json.RawMessage `json:"newValue"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.StateStore == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "state store not configured"},
					}, nil
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				modID := string(req.RuntimeIdentity.ModuleID)
				swapped, newVersion, err := deps.StateStore.CAS(ctx, extID, modID, p.Key, p.ExpectedVersion, p.NewValue)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"swapped":   swapped,
					"version":   newVersion,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodStateDelete,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					Key string `json:"key"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.StateStore == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "state store not configured"},
					}, nil
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				modID := string(req.RuntimeIdentity.ModuleID)
				if err := deps.StateStore.Delete(ctx, extID, modID, p.Key); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{"ok": true})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodStateList,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.StateStore == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "state store not configured"},
					}, nil
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				modID := string(req.RuntimeIdentity.ModuleID)
				entries, err := deps.StateStore.List(ctx, extID, modID)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				keys := make([]map[string]any, 0, len(entries))
				for _, e := range entries {
					keys = append(keys, map[string]any{
						"key":     e.Key,
						"value":   json.RawMessage(e.Value),
						"version": e.Version,
					})
				}
				output, _ := json.Marshal(map[string]any{
					"keys": keys,
					"total": len(keys),
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodResourceOpen,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					Path string `json:"path"`
					Mode string `json:"mode"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.ExtensionRoot == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "extension root not configured"},
					}, nil
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				extDir := filepath.Join(deps.ExtensionRoot, extID)
				realPath := filepath.Join(extDir, p.Path)
				if !isPathSafe(extDir, realPath) {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "path escapes extension directory"},
					}, nil
				}
				var f *os.File
				var err error
				readOnly := p.Mode == "r" || p.Mode == "read"
				if readOnly {
					f, err = os.Open(realPath)
				} else {
					f, err = os.OpenFile(realPath, os.O_RDWR|os.O_CREATE, 0644)
				}
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: err.Error()},
					}, nil
				}
				handleID := fmt.Sprintf("res-%s", uuid.NewString())
				h := &resourceHandle{
					handleID: handleID,
					realPath: realPath,
					file:     f,
					readOnly: readOnly,
					extensionID: extID,
					moduleID:    string(req.RuntimeIdentity.ModuleID),
				}
				handleTable.put(h)
				output, _ := json.Marshal(map[string]any{
					"handleId": handleID,
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
			method:          host_api.MethodResourceRead,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         10 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					HandleID string `json:"handleId"`
					Length   int    `json:"length"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				h, ok := handleTable.get(p.HandleID)
				if !ok {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: "handle not found"},
					}, nil
				}
				if h.extensionID != string(req.RuntimeIdentity.ExtensionID) {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: "handle does not belong to caller"},
					}, nil
				}
				if p.Length <= 0 || p.Length > 1024*1024 {
					p.Length = 4096
				}
				buf := make([]byte, p.Length)
				n, err := h.file.Read(buf)
				eof := false
				if err != nil {
					if errors.Is(err, io.EOF) {
						eof = true
					} else {
						return host_api.CallResult{
							Status: host_api.StatusFailed,
							Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
						}, nil
					}
				}
				if n < p.Length {
					eof = true
				}
				output, _ := json.Marshal(map[string]any{
					"data": string(buf[:n]),
					"eof":  eof,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodResourceWrite,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         10 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					HandleID string `json:"handleId"`
					Data     string `json:"data"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				h, ok := handleTable.get(p.HandleID)
				if !ok {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: "handle not found"},
					}, nil
				}
				if h.extensionID != string(req.RuntimeIdentity.ExtensionID) {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: "handle does not belong to caller"},
					}, nil
				}
				if h.readOnly {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: "handle is read-only"},
					}, nil
				}
				n, err := h.file.Write([]byte(p.Data))
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"written": n,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodResourceClose,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectNone,
			timeout:         3 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					HandleID string `json:"handleId"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				h, ok := handleTable.remove(p.HandleID)
				if !ok {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: "handle not found"},
					}, nil
				}
				if h.extensionID != string(req.RuntimeIdentity.ExtensionID) {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: "handle does not belong to caller"},
					}, nil
				}
				if h.file != nil {
					_ = h.file.Close()
				}
				output, _ := json.Marshal(map[string]any{"ok": true})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodResourceStat,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         3 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					Path string `json:"path"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.ExtensionRoot == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "extension root not configured"},
					}, nil
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				extDir := filepath.Join(deps.ExtensionRoot, extID)
				realPath := filepath.Join(extDir, p.Path)
				if !isPathSafe(extDir, realPath) {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "path escapes extension directory"},
					}, nil
				}
				info, err := os.Stat(realPath)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"path":  p.Path,
					"size":  info.Size(),
					"isDir": info.IsDir(),
					"modTime": info.ModTime().UTC().Format(time.RFC3339),
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodCharacterRead,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					CharacterID string `json:"characterId"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.CharacterReader == nil {
					output, _ := json.Marshal(map[string]any{
						"characterId": p.CharacterID,
						"available":   false,
					})
					return host_api.CallResult{
						Status: host_api.StatusSuccess,
						Output: output,
					}, nil
				}
				data, available, err := deps.CharacterReader.ReadCharacter(ctx, p.CharacterID)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"characterId": p.CharacterID,
					"available":   available,
					"data":        json.RawMessage(data),
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodConversationRead,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					ConversationID string `json:"conversationId"`
					Limit          int    `json:"limit"`
					Offset         int    `json:"offset"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.ConversationReader == nil {
					output, _ := json.Marshal(map[string]any{
						"messages": []any{},
						"hasMore":  false,
					})
					return host_api.CallResult{
						Status: host_api.StatusSuccess,
						Output: output,
					}, nil
				}
				if p.Limit <= 0 || p.Limit > 100 {
					p.Limit = 50
				}
				msgs, hasMore, err := deps.ConversationReader.ReadConversation(ctx, p.ConversationID, p.Limit, p.Offset)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"messages": msgs,
					"hasMore":  hasMore,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodMemoryQuery,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         10 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					Query string `json:"query"`
					Limit int    `json:"limit"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.MemoryQueryService == nil {
					output, _ := json.Marshal(map[string]any{
						"results": []any{},
						"total":   0,
					})
					return host_api.CallResult{
						Status: host_api.StatusSuccess,
						Output: output,
					}, nil
				}
				if p.Limit <= 0 || p.Limit > 50 {
					p.Limit = 10
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				results, err := deps.MemoryQueryService.Query(ctx, extID, p.Query, p.Limit)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"results": results,
					"total":   len(results),
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodEventEmit,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
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
				if deps.EventService == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "event service not configured"},
					}, nil
				}
				if p.EventType == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "eventType is required"},
					}, nil
				}
				opts := event.PublishOptions{
					ProducerID:    string(req.RuntimeIdentity.ExtensionID),
					ProducerType:  "extension",
					AggregateType: p.EventType,
					AggregateID:   req.CallID,
					TraceID:       req.TraceID,
				}
				pubResult, err := deps.EventService.Publish(ctx, event.EventTypeID(p.EventType), 1, p.Payload, opts)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"ok":       pubResult.Accepted,
					"eventType": p.EventType,
					"eventId":  pubResult.EventID,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodEventSubscribe,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectNone,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					EventType string `json:"eventType"`
					Entry     string `json:"entry"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.EventService == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "event service not configured"},
					}, nil
				}
				if p.EventType == "" || p.Entry == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "eventType and entry are required"},
					}, nil
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				modID := string(req.RuntimeIdentity.ModuleID)
				subID := fmt.Sprintf("sub-%s", uuid.NewString())
				def := event.EventSubscriptionDefinition{
					ContributionID: subID,
					ExtensionID:    extID,
					ModuleID:       modID,
					EventTypeID:    event.EventTypeID(p.EventType),
					Entry:          p.Entry,
					Enabled:        true,
					Generation:     req.RuntimeIdentity.Generation,
					CreatedAt:      time.Now().UTC(),
					UpdatedAt:      time.Now().UTC(),
				}
				if err := def.Validate(); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if err := deps.EventService.RegisterSubscription(ctx, def); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"ok":             true,
					"subscriptionId": subID,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodEventUnsubscribe,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectNone,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					SubscriptionID string `json:"subscriptionId"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.EventService == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "event service not configured"},
					}, nil
				}
				sub, ok := deps.EventService.GetSubscription(ctx, p.SubscriptionID)
				if !ok {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: "subscription not found"},
					}, nil
				}
				if sub.Definition.ExtensionID != string(req.RuntimeIdentity.ExtensionID) {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: "subscription does not belong to caller"},
					}, nil
				}
				if err := deps.EventService.UnregisterSubscription(ctx, p.SubscriptionID); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{"ok": true})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodScheduleCreate,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         10 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var def schedule.ScheduleContributionDefinition
				if err := json.Unmarshal(req.Input, &def); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.ScheduleService == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "schedule service not configured"},
					}, nil
				}
				def.ExtensionID = string(req.RuntimeIdentity.ExtensionID)
				def.ModuleID = string(req.RuntimeIdentity.ModuleID)
				if def.ContributionID == "" {
					def.ContributionID = fmt.Sprintf("sched-contrib-%s", uuid.NewString())
				}
				if err := deps.ScheduleService.InstallDefinition(ctx, &def); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"ok":         true,
					"scheduleId": def.ScheduleID,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodScheduleCancel,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					ScheduleID string `json:"scheduleId"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.ScheduleService == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "schedule service not configured"},
					}, nil
				}
				scheduleDef, _, err := deps.ScheduleService.GetSchedule(ctx, p.ScheduleID)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: err.Error()},
					}, nil
				}
				if scheduleDef == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: "schedule not found"},
					}, nil
				}
				if scheduleDef.ExtensionID != string(req.RuntimeIdentity.ExtensionID) {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: "schedule does not belong to caller"},
					}, nil
				}
				if err := deps.ScheduleService.Uninstall(ctx, p.ScheduleID); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{"ok": true})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodScheduleList,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.ScheduleService == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "schedule service not configured"},
					}, nil
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				defs, err := deps.ScheduleService.ListSchedules(ctx, extID)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"schedules": defs,
					"total":     len(defs),
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodToolExecute,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectExternal,
			timeout:         30 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					ToolID string          `json:"toolId"`
					Input  json.RawMessage `json:"input"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.ExecutionKernel == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "execution kernel not configured"},
					}, nil
				}
				if p.ToolID == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "toolId is required"},
					}, nil
				}
				invocation := capability.ToolInvocationContext{
					InvocationID: req.InvocationID,
					ParentID:     req.ParentID,
					ExtensionID:  string(req.RuntimeIdentity.ExtensionID),
					ModuleID:     string(req.RuntimeIdentity.ModuleID),
					Source:       capability.InvocationSourcePlugin,
					TraceID:      req.TraceID,
				}
				if invocation.InvocationID == "" {
					invocation.InvocationID = fmt.Sprintf("hostapi-%s", uuid.NewString())
				}
				execReq := execution.ToolExecutionRequest{
					ToolID:     capability.CapabilityID(p.ToolID),
					Input:      p.Input,
					Invocation: invocation,
				}
				result := deps.ExecutionKernel.Execute(ctx, execReq)
				output, _ := json.Marshal(map[string]any{
					"invocationId": result.InvocationID,
					"status":       string(result.Status),
					"content":      result.Content,
					"structured":   json.RawMessage(result.Structured),
					"error":       result.Error,
				})
				if result.Status != capability.ToolResultStatusSuccess {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Output: output,
						Error: &host_api.Error{
							Code:    host_api.ErrorCodeInternal,
							Message: fmt.Sprintf("tool execution failed: %s", result.Status),
						},
					}, nil
				}
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodUINotify,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectNone,
			timeout:         3 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				return host_api.CallResult{
					Status: host_api.StatusFailed,
					Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "ui host not available"},
				}, nil
			},
		},
		{
			method:          host_api.MethodUIDialog,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectNone,
			timeout:         30 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				return host_api.CallResult{
					Status: host_api.StatusFailed,
					Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "ui host not available"},
				}, nil
			},
		},
		{
			method:          host_api.MethodUINavigate,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectNone,
			timeout:         3 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				return host_api.CallResult{
					Status: host_api.StatusFailed,
					Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "ui host not available"},
				}, nil
			},
		},
	}

	for _, rd := range routes {
		perm := host_api.RoutePermissionForMethod(rd.method)
		if perm == nil {
			return fmt.Errorf("host_api: no permission mapping for method %s, refusing to register", rd.method)
		}
		route := host_api.Route{
			Method:          rd.method,
			Version:         1,
			Permission:      perm,
			ScopePolicy:     host_api.RouteScopeForMethod(rd.method),
			RiskLevel:       rd.riskLevel,
			SideEffectLevel: rd.sideEffectLevel,
			Timeout:         rd.timeout,
			Handler:         rd.handler,
		}
		if err := gateway.RegisterRoute(route); err != nil {
			return fmt.Errorf("host_api: register route %s v%d: %w", route.Method, route.Version, err)
		}
	}

	return nil
}

func isPathSafe(baseDir, targetPath string) bool {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return false
	}
	if strings.HasPrefix(rel, "..") {
		return false
	}
	return true
}
