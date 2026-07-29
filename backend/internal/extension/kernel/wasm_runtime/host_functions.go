package wasm_runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type HostFunctionSet struct {
	registry *HostImportRegistry
	logger   func(level, msg string, fields map[string]any)
	storage  HostStorageBroker
	resource HostResourceReader
	toolHub  HostToolInvoker
}

type HostStorageBroker interface {
	Get(ctx context.Context, extensionID, key string) ([]byte, error)
	CAS(ctx context.Context, extensionID, key string, oldVal, newVal []byte) (bool, error)
}

type HostResourceReader interface {
	Read(ctx context.Context, extensionID, resourcePath string) ([]byte, error)
}

type HostToolInvoker interface {
	Invoke(ctx context.Context, extensionID, toolID string, input json.RawMessage) (json.RawMessage, error)
}

type HostFunctionConfig struct {
	Logger   func(level, msg string, fields map[string]any)
	Storage  HostStorageBroker
	Resource HostResourceReader
	ToolHub  HostToolInvoker
}

func NewHostFunctionSet(cfg HostFunctionConfig) *HostFunctionSet {
	h := &HostFunctionSet{
		registry: NewHostImportRegistry(),
		logger:   cfg.Logger,
		storage:  cfg.Storage,
		resource: cfg.Resource,
		toolHub:  cfg.ToolHub,
	}
	if h.logger == nil {
		h.logger = func(level, msg string, fields map[string]any) {}
	}
	h.registerAll()
	return h
}

func (h *HostFunctionSet) Registry() *HostImportRegistry {
	return h.registry
}

func (h *HostFunctionSet) registerAll() {
	h.registry.Register(ImportLog, h.handleLog)
	h.registry.Register(ImportTime, h.handleTime)
	h.registry.Register(ImportRandom, h.handleRandom)
	h.registry.Register(ImportStorageGet, h.handleStorageGet)
	h.registry.Register(ImportStorageCAS, h.handleStorageCAS)
	h.registry.Register(ImportResourceRead, h.handleResourceRead)
	h.registry.Register(ImportArtifactWrite, h.handleArtifactWrite)
	h.registry.Register(ImportToolInvoke, h.handleToolInvoke)
	h.registry.Register(ImportResultSetError, h.handleSetError)
}

func (h *HostFunctionSet) handleLog(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
	var p struct {
		Level   string `json:"level"`
		Message string `json:"message"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &p)
	}
	if p.Level == "" {
		p.Level = "info"
	}
	h.logger(p.Level, fmt.Sprintf("wasm[%s]: %s", hctx.ModuleID, p.Message), map[string]any{
		"extension":  hctx.ExtensionID,
		"invocation": hctx.InvocationID,
	})
	return json.RawMessage(`{}`), nil
}

func (h *HostFunctionSet) handleTime(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
	now := time.Now().UTC()
	return json.Marshal(map[string]any{
		"unix":      now.Unix(),
		"unix_nano": now.UnixNano(),
		"iso8601":   now.Format(time.RFC3339Nano),
	})
}

func (h *HostFunctionSet) handleRandom(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
	var p struct {
		Min int `json:"min"`
		Max int `json:"max"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &p)
	}
	if p.Max == 0 {
		p.Max = 1000000
	}
	seed := time.Now().UnixNano()
	val := int(seed % int64(p.Max-p.Min+1))
	if val < 0 {
		val = -val
	}
	val += p.Min
	return json.Marshal(map[string]any{
		"seed":  seed,
		"value": val,
	})
}

func (h *HostFunctionSet) handleStorageGet(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
	if h.storage == nil {
		return nil, NewWASMError(ErrCodeHostCallFailed, "storage not configured", nil)
	}
	var p struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, NewWASMError(ErrCodeHostCallFailed, fmt.Sprintf("parse params: %v", err), err)
	}
	if p.Key == "" {
		return nil, NewWASMError(ErrCodeHostCallFailed, "storage key required", nil)
	}
	data, err := h.storage.Get(ctx, hctx.ExtensionID, p.Key)
	if err != nil {
		return nil, NewWASMError(ErrCodeHostCallFailed, fmt.Sprintf("storage get: %v", err), err)
	}
	if data == nil {
		return json.Marshal(map[string]any{"found": false})
	}
	return json.Marshal(map[string]any{
		"found": true,
		"data":  string(data),
	})
}

func (h *HostFunctionSet) handleStorageCAS(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
	if h.storage == nil {
		return nil, NewWASMError(ErrCodeHostCallFailed, "storage not configured", nil)
	}
	var p struct {
		Key    string `json:"key"`
		OldVal string `json:"old_val"`
		NewVal string `json:"new_val"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, NewWASMError(ErrCodeHostCallFailed, fmt.Sprintf("parse params: %v", err), err)
	}
	if p.Key == "" {
		return nil, NewWASMError(ErrCodeHostCallFailed, "storage key required", nil)
	}
	swapped, err := h.storage.CAS(ctx, hctx.ExtensionID, p.Key, []byte(p.OldVal), []byte(p.NewVal))
	if err != nil {
		return nil, NewWASMError(ErrCodeHostCallFailed, fmt.Sprintf("storage cas: %v", err), err)
	}
	return json.Marshal(map[string]any{
		"swapped": swapped,
	})
}

func (h *HostFunctionSet) handleResourceRead(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
	if h.resource == nil {
		return nil, NewWASMError(ErrCodeHostCallFailed, "resource reader not configured", nil)
	}
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, NewWASMError(ErrCodeHostCallFailed, fmt.Sprintf("parse params: %v", err), err)
	}
	if p.Path == "" {
		return nil, NewWASMError(ErrCodeHostCallFailed, "resource path required", nil)
	}
	if err := ValidateModulePath(p.Path); err != nil {
		return nil, err
	}
	data, err := h.resource.Read(ctx, hctx.ExtensionID, p.Path)
	if err != nil {
		return nil, NewWASMError(ErrCodeHostCallFailed, fmt.Sprintf("resource read: %v", err), err)
	}
	return json.Marshal(map[string]any{
		"data": string(data),
	})
}

func (h *HostFunctionSet) handleArtifactWrite(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
	var p struct {
		Path string `json:"path"`
		Data string `json:"data"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, NewWASMError(ErrCodeHostCallFailed, fmt.Sprintf("parse params: %v", err), err)
	}
	if p.Path == "" {
		return nil, NewWASMError(ErrCodeHostCallFailed, "artifact path required", nil)
	}
	h.logger("info", "wasm artifact write", map[string]any{
		"extension": hctx.ExtensionID,
		"path":      p.Path,
		"size":      len(p.Data),
	})
	return json.Marshal(map[string]any{
		"written": true,
		"path":    p.Path,
	})
}

func (h *HostFunctionSet) handleToolInvoke(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
	if h.toolHub == nil {
		return nil, NewWASMError(ErrCodeHostCallFailed, "tool hub not configured", nil)
	}
	var p struct {
		ToolID string          `json:"tool_id"`
		Input  json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, NewWASMError(ErrCodeHostCallFailed, fmt.Sprintf("parse params: %v", err), err)
	}
	if p.ToolID == "" {
		return nil, NewWASMError(ErrCodeHostCallFailed, "tool_id required", nil)
	}
	result, err := h.toolHub.Invoke(ctx, hctx.ExtensionID, p.ToolID, p.Input)
	if err != nil {
		return nil, NewWASMError(ErrCodeHostCallFailed, fmt.Sprintf("tool invoke: %v", err), err)
	}
	if len(result) == 0 {
		result = json.RawMessage(`{}`)
	}
	return result, nil
}

func (h *HostFunctionSet) handleSetError(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
	var p struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &p)
	}
	h.logger("error", fmt.Sprintf("wasm error[%s]: %s", p.Code, p.Message), map[string]any{
		"extension":  hctx.ExtensionID,
		"invocation": hctx.InvocationID,
		"code":       p.Code,
	})
	return json.RawMessage(`{"set":true}`), nil
}

func (h *HostFunctionSet) RegisterDefaults(registry *HostImportRegistry) {
	registry.Register(ImportLog, h.handleLog)
	registry.Register(ImportTime, h.handleTime)
}
