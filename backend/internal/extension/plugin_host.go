// Deprecated: Legacy extension architecture.
// Do not add new capabilities. This implementation is retained only for
// compatibility, maintenance, testing, and migration to Extension Kernel.

package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	applog "github.com/u-ai/backend/log"
)

type pluginCallDepthKey struct{}

type pluginHost struct {
	manager          *PluginManager
	pluginID         string
	version          string
	mu               sync.Mutex
	registeredSkills map[string]bool
}

func (h *pluginHost) PluginID() string { return h.pluginID }

func (h *pluginHost) RegisterSkill(ctx context.Context, definition SkillDefinition, handler SkillHandler) error {
	if err := h.authorize(ctx); err != nil {
		return err
	}
	entry, err := h.manager.entry(h.pluginID)
	if err != nil {
		return err
	}
	declared := false
	for _, skillID := range entry.registered.Manifest.RegisteredSkills {
		if skillID == definition.ID {
			declared = true
			break
		}
	}
	if !declared || !strings.HasPrefix(definition.ID, pluginSkillPrefix(h.pluginID)) {
		return NewExtensionError(ErrPluginActionNotAllowed, "Plugin skill is outside its declared namespace", definition.ID, false, nil)
	}
	definition.Enabled = false
	if err := h.manager.skills.Register(ctx, definition, handler); err != nil {
		return err
	}
	h.mu.Lock()
	if h.registeredSkills == nil {
		h.registeredSkills = map[string]bool{}
	}
	h.registeredSkills[definition.ID] = true
	h.mu.Unlock()
	return nil
}

func (h *pluginHost) CallSkill(ctx context.Context, request ExecuteSkillRequest) (SkillResult, error) {
	if err := h.authorize(ctx); err != nil {
		return SkillResult{}, err
	}
	entry, err := h.manager.entry(h.pluginID)
	if err != nil {
		return SkillResult{}, err
	}
	entry.mu.RLock()
	enabled := entry.enabled && entry.lifecycle == PluginEnabled
	entry.mu.RUnlock()
	if !enabled {
		return SkillResult{}, NewExtensionError(ErrPluginDisabled, "Plugin is disabled", h.pluginID, false, nil)
	}
	depth, _ := ctx.Value(pluginCallDepthKey{}).(int)
	if depth >= 4 {
		return SkillResult{}, NewExtensionError(ErrPluginActionNotAllowed, "Plugin skill call depth exceeded", h.pluginID, false, nil)
	}
	if err := h.manager.repository.ValidateConversationScope(ctx, request.Scope); err != nil {
		return SkillResult{}, err
	}
	target, err := h.manager.skills.Get(ctx, request.SkillID)
	if err != nil {
		return SkillResult{}, err
	}
	for _, capability := range target.Definition.Capabilities {
		if err := h.requireCapability(ctx, capability, request.Scope); err != nil {
			return SkillResult{}, err
		}
	}
	request.Scope.CausationID = h.pluginID
	result, err := h.manager.executor.Execute(context.WithValue(ctx, pluginCallDepthKey{}, depth+1), request)
	_ = h.manager.repository.AuditPlugin(ctx, h.pluginID, "plugin.skill.call", PluginStateScope{Type: ScopeCharacter, ID: request.Scope.CharacterID}, request.Scope.TraceID, map[string]any{"skillId": request.SkillID, "status": result.Status})
	return result, err
}

func (h *pluginHost) ReadSnapshot(ctx context.Context, scope ExecutionScope) (ExtensionSnapshot, error) {
	if err := h.requireCapability(ctx, "runtime.character.read", scope); err != nil {
		return ExtensionSnapshot{}, err
	}
	if err := h.manager.repository.ValidateConversationScope(ctx, scope); err != nil {
		return ExtensionSnapshot{}, err
	}
	return ExtensionSnapshot{SchemaVersion: "1.0.0", User: SnapshotUser{ID: scope.UserID}, Character: SnapshotEntity{ID: scope.CharacterID}, Conversation: SnapshotEntity{ID: scope.ConversationID}, Channel: SnapshotChannel{Name: scope.Channel}, CapturedAt: time.Now().UTC()}, nil
}

func (h *pluginHost) ReadConfig(ctx context.Context, scope PluginStateScope) (json.RawMessage, error) {
	if err := h.authorize(ctx); err != nil {
		return nil, err
	}
	entry, err := h.manager.entry(h.pluginID)
	if err != nil {
		return nil, err
	}
	global, err := h.manager.repository.GetConfig(ctx, h.pluginID, PermissionScope{Type: ScopeGlobal}, entry.registered.Manifest.DefaultConfig)
	if err != nil {
		return nil, err
	}
	if scope.Type == ScopeGlobal {
		return append(json.RawMessage(nil), global...), nil
	}
	if err := validatePluginStateScope(scope); err != nil {
		return nil, err
	}
	return h.manager.repository.GetConfig(ctx, h.pluginID, PermissionScope{Type: scope.Type, ID: scope.ID}, global)
}

func (h *pluginHost) ReadState(ctx context.Context, scope PluginStateScope) (PluginState, error) {
	if err := h.requireCapability(ctx, "storage.own.read", executionFromPluginScope(scope)); err != nil {
		return PluginState{}, err
	}
	entry, err := h.manager.entry(h.pluginID)
	if err != nil {
		return PluginState{}, err
	}
	return h.manager.repository.ReadPluginState(ctx, h.pluginID, scope, entry.registered.Manifest.State.Default, entry.registered.Manifest.State.SchemaVersion)
}

func (h *pluginHost) WriteState(ctx context.Context, request WritePluginStateRequest) (PluginState, error) {
	if err := h.requireCapability(ctx, "storage.own.write", executionFromPluginScope(request.Scope)); err != nil {
		return PluginState{}, err
	}
	entry, err := h.manager.entry(h.pluginID)
	if err != nil {
		return PluginState{}, err
	}
	if len(entry.registered.Manifest.State.Schema) > 0 {
		if err := h.manager.validator.Validate(h.pluginID+"-plugin-state-write", entry.registered.Manifest.State.Schema, request.Data); err != nil {
			return PluginState{}, NewExtensionError(ErrPluginStateInvalid, "Plugin state schema validation failed", err.Error(), false, err)
		}
	}
	state, err := h.manager.repository.CompareAndSwapPluginState(ctx, h.pluginID, entry.registered.Manifest.State.SchemaVersion, request)
	if err == nil {
		_ = h.manager.repository.AuditPlugin(ctx, h.pluginID, "plugin.state.write", request.Scope, "", map[string]any{"revision": state.Revision})
	}
	return state, err
}

func (h *pluginHost) EmitEvent(ctx context.Context, event ExtensionEvent) error {
	if err := h.requireCapability(ctx, "event.own.emit", ExecutionScope{Trigger: TriggerSystemEvent, TraceID: event.TraceID}); err != nil {
		return err
	}
	expectedSource := "amitia://extensions/" + h.pluginID
	if event.Source == "" {
		event.Source = expectedSource
	}
	if event.Source != expectedSource || !strings.HasPrefix(event.Type, h.pluginID+".") {
		return NewExtensionError(ErrPluginEventInvalid, "Plugin event identity is invalid", event.Type, false, nil)
	}
	if strings.Contains(strings.ToLower(string(event.Data)), "authorization") || hasSecretJSON(event.Data) {
		return NewExtensionError(ErrPluginEventInvalid, "Plugin event contains sensitive data", event.Type, false, nil)
	}
	event.Depth++
	return h.manager.persistEvent(ctx, event)
}

func (h *pluginHost) RegisterSchedule(ctx context.Context, definition PluginScheduleDefinition) error {
	if err := h.requireCapability(ctx, "scheduler.own.manage", executionFromPluginScope(definition.Scope)); err != nil {
		return err
	}
	if !skillIDPattern.MatchString(definition.ScheduleID) || len(definition.Payload) > 32768 || hasSecretJSON(definition.Payload) {
		return NewExtensionError(ErrPluginScheduleInvalid, "Plugin schedule is invalid", definition.ScheduleID, false, nil)
	}
	if err := validatePluginStateScope(definition.Scope); err != nil {
		return err
	}
	switch definition.Type {
	case "once":
		runAt, err := time.Parse(time.RFC3339, definition.Expression)
		if err != nil || !runAt.After(time.Now()) {
			return NewExtensionError(ErrPluginScheduleInvalid, "Plugin one-time schedule is invalid", definition.Expression, false, err)
		}
		definition.NextRunAt = runAt.UTC().Format(time.RFC3339Nano)
	case "interval":
		duration, err := time.ParseDuration(definition.Expression)
		if err != nil || duration < time.Second || duration > 365*24*time.Hour {
			return NewExtensionError(ErrPluginScheduleInvalid, "Plugin interval schedule is invalid", definition.Expression, false, err)
		}
		definition.NextRunAt = time.Now().UTC().Add(duration).Format(time.RFC3339Nano)
	default:
		return NewExtensionError(ErrPluginScheduleInvalid, "Unsupported plugin schedule type", definition.Type, false, nil)
	}
	if definition.Timezone == "" {
		definition.Timezone = "UTC"
	}
	definition.Enabled = true
	return h.manager.repository.UpsertPluginSchedule(ctx, h.pluginID, definition)
}

func (h *pluginHost) RemoveSchedule(ctx context.Context, scheduleID string) error {
	if err := h.requireCapability(ctx, "scheduler.own.manage", ExecutionScope{Trigger: TriggerSystemEvent}); err != nil {
		return err
	}
	return h.manager.repository.DeletePluginSchedule(ctx, h.pluginID, scheduleID)
}

func (h *pluginHost) authorize(ctx context.Context) error {
	if ctx == nil || ctx.Err() != nil || ctx.Value(pluginIdentityContextKey{}) != h.pluginID {
		return NewExtensionError(ErrSkillPermissionDenied, "Plugin Host context is invalid", h.pluginID, false, nil)
	}
	return nil
}

func (h *pluginHost) Logger() PluginLogger {
	return pluginBoundLogger{pluginID: h.pluginID, version: h.version}
}
func (h *pluginHost) Tracer() PluginTracer {
	return pluginBoundTracer{pluginID: h.pluginID, version: h.version}
}

func (h *pluginHost) requireCapability(ctx context.Context, capability string, scope ExecutionScope) error {
	if err := h.authorize(ctx); err != nil {
		return err
	}
	entry, err := h.manager.entry(h.pluginID)
	if err != nil {
		return err
	}
	declared := false
	for _, item := range entry.registered.Manifest.Capabilities {
		if item == capability {
			declared = true
			break
		}
	}
	if !declared {
		return NewExtensionError(ErrSkillPermissionDenied, "Plugin capability is not declared", capability, false, nil)
	}
	identity := ExtensionIdentity{ExtensionID: h.pluginID, SkillID: h.pluginID, Version: h.version}
	if h.manager.permissions.EvaluateExecution(ctx, identity, capability, scope) == DecisionDeny {
		return NewExtensionError(ErrSkillPermissionDenied, "Plugin capability is denied", capability, false, nil)
	}
	return nil
}

func (h *pluginHost) verifyRegisteredSkills() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	entry, err := h.manager.entry(h.pluginID)
	if err != nil {
		return err
	}
	if len(h.registeredSkills) != len(entry.registered.Manifest.RegisteredSkills) {
		return NewExtensionError(ErrPluginLoadFailed, "Plugin registered skill set does not match manifest", h.pluginID, false, nil)
	}
	for _, skillID := range entry.registered.Manifest.RegisteredSkills {
		if !h.registeredSkills[skillID] {
			return NewExtensionError(ErrPluginLoadFailed, "Plugin did not register declared skill", skillID, false, nil)
		}
	}
	return nil
}

type pluginBoundLogger struct{ pluginID, version string }

func (l pluginBoundLogger) Info(_ context.Context, message string, fields map[string]any) {
	applog.Info(message, l.fields(fields))
}
func (l pluginBoundLogger) Warn(_ context.Context, message string, fields map[string]any) {
	applog.Warn(message, l.fields(fields))
}
func (l pluginBoundLogger) Error(_ context.Context, message string, fields map[string]any) {
	applog.Error(message, l.fields(fields))
}
func (l pluginBoundLogger) fields(fields map[string]any) applog.Fields {
	copyFields := map[string]any{}
	for key, value := range fields {
		copyFields[key] = value
	}
	redactValue(copyFields)
	copyFields["plugin_id"], copyFields["plugin_version"] = l.pluginID, l.version
	return applog.Fields(copyFields)
}

type pluginBoundTracer struct{ pluginID, version string }

func (t pluginBoundTracer) Start(ctx context.Context, span string) (context.Context, func(error)) {
	started := time.Now()
	return ctx, func(err error) {
		fields := applog.Fields{"plugin_id": t.pluginID, "plugin_version": t.version, "span": span, "duration_ms": time.Since(started).Milliseconds()}
		if err != nil {
			fields["error"] = "plugin operation failed"
			applog.Warn("plugin trace completed", fields)
		} else {
			applog.Info("plugin trace completed", fields)
		}
	}
}

func hasSecretJSON(raw json.RawMessage) bool {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return true
	}
	return hasPlaintextSecret(value)
}

func (h *pluginHost) String() string { return fmt.Sprintf("PluginHost(%s@%s)", h.pluginID, h.version) }
