package extension

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

func (s *ExtensionService) ListPlugins(_ context.Context, page, pageSize int) (PluginPage, error) {
	if err := s.requirePluginManager(); err != nil {
		return PluginPage{}, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	s.plugins.mu.RLock()
	entries := s.plugins.sortedEntries()
	s.plugins.mu.RUnlock()
	total := len(entries)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	items := make([]PluginView, 0, end-start)
	for _, entry := range entries[start:end] {
		items = append(items, pluginViewFromEntry(entry))
	}
	return PluginPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *ExtensionService) GetPlugin(ctx context.Context, scope ExecutionScope, pluginID string) (PluginDetailView, error) {
	entry, err := s.pluginEntry(pluginID)
	if err != nil {
		return PluginDetailView{}, err
	}
	permissions, err := s.repository.ListGrants(ctx, pluginID)
	if err != nil {
		return PluginDetailView{}, err
	}
	config, err := s.GetPluginConfig(ctx, scope, pluginID)
	if err != nil {
		return PluginDetailView{}, err
	}
	states, err := s.GetPluginStates(ctx, scope, pluginID)
	if err != nil {
		return PluginDetailView{}, err
	}
	schedules, err := s.GetPluginSchedules(ctx, scope, pluginID)
	if err != nil {
		return PluginDetailView{}, err
	}
	runs, err := s.repository.ListPluginRuns(ctx, pluginID, scope.CharacterID, 20)
	if err != nil {
		return PluginDetailView{}, err
	}
	events, _, err := s.repository.ListPluginEvents(ctx, pluginID, scope.CharacterID, "", 1, 20)
	if err != nil {
		return PluginDetailView{}, err
	}
	return PluginDetailView{PluginView: pluginViewFromEntry(entry), Permissions: filterPluginGrants(permissions, scope), Config: config, States: states, Schedules: schedules, RecentRuns: runs, RecentEvents: events}, nil
}

func (s *ExtensionService) EnablePlugin(ctx context.Context, scope ExecutionScope, pluginID string) error {
	return s.plugins.Enable(ctx, pluginID, pluginScopeFromExecution(scope))
}

func (s *ExtensionService) DisablePlugin(ctx context.Context, scope ExecutionScope, pluginID string) error {
	return s.plugins.Disable(ctx, pluginID, pluginScopeFromExecution(scope))
}

func (s *ExtensionService) ReloadPlugin(ctx context.Context, pluginID string) error {
	return s.plugins.Reload(ctx, pluginID)
}

func (s *ExtensionService) GetPluginConfig(ctx context.Context, scope ExecutionScope, pluginID string) (json.RawMessage, error) {
	entry, err := s.pluginEntry(pluginID)
	if err != nil {
		return nil, err
	}
	global, err := s.repository.GetConfig(ctx, pluginID, PermissionScope{Type: ScopeGlobal}, entry.registered.Manifest.DefaultConfig)
	if err != nil {
		return nil, err
	}
	if scope.CharacterID != "" {
		global, err = s.repository.GetConfig(ctx, pluginID, PermissionScope{Type: ScopeCharacter, ID: scope.CharacterID}, global)
		if err != nil {
			return nil, err
		}
	}
	return redactJSON(global), nil
}

func (s *ExtensionService) UpdatePluginConfig(ctx context.Context, scope ExecutionScope, pluginID string, incoming json.RawMessage) error {
	entry, err := s.pluginEntry(pluginID)
	if err != nil {
		return err
	}
	permissionScope := pluginConfigScope(scope)
	stored, err := s.repository.GetConfig(ctx, pluginID, permissionScope, entry.registered.Manifest.DefaultConfig)
	if err != nil {
		return err
	}
	var storedValue, incomingValue any
	if json.Unmarshal(stored, &storedValue) != nil || json.Unmarshal(incoming, &incomingValue) != nil {
		return NewExtensionError(ErrPluginConfigInvalid, "Plugin config JSON is invalid", pluginID, false, nil)
	}
	merged, err := json.Marshal(restoreRedactedValue(storedValue, incomingValue))
	if err != nil {
		return err
	}
	if len(merged) > 131072 {
		return NewExtensionError(ErrPluginConfigInvalid, "Plugin config is too large", pluginID, false, nil)
	}
	if len(entry.registered.Manifest.ConfigSchema) > 0 {
		if err := s.validator.Validate(pluginID+"-plugin-config-update", entry.registered.Manifest.ConfigSchema, merged); err != nil {
			return NewExtensionError(ErrPluginConfigInvalid, "Plugin config schema validation failed", err.Error(), false, err)
		}
	}
	if err := s.repository.UpdateConfig(ctx, pluginID, permissionScope, merged); err != nil {
		return err
	}
	entry.mu.RLock()
	wasEnabled := entry.enabled
	entry.mu.RUnlock()
	if err := s.plugins.Reload(ctx, pluginID); err != nil {
		_ = s.repository.UpdateConfig(ctx, pluginID, permissionScope, stored)
		if recoveryErr := s.plugins.Reload(ctx, pluginID); recoveryErr == nil && wasEnabled {
			_ = s.plugins.Enable(ctx, pluginID, pluginScopeFromExecution(scope))
		}
		return err
	}
	return s.repository.AuditPlugin(ctx, pluginID, "plugin.config.updated", PluginStateScope{Type: permissionScope.Type, ID: permissionScope.ID}, scope.TraceID, map[string]any{"version": "updated"})
}

func (s *ExtensionService) ResetPluginConfig(ctx context.Context, scope ExecutionScope, pluginID string) error {
	entry, err := s.pluginEntry(pluginID)
	if err != nil {
		return err
	}
	permissionScope := pluginConfigScope(scope)
	stored, err := s.repository.GetConfig(ctx, pluginID, permissionScope, entry.registered.Manifest.DefaultConfig)
	if err != nil {
		return err
	}
	entry.mu.RLock()
	wasEnabled := entry.enabled
	entry.mu.RUnlock()
	if err := s.repository.ResetConfig(ctx, pluginID, permissionScope); err != nil {
		return err
	}
	if err := s.plugins.Reload(ctx, pluginID); err != nil {
		_ = s.repository.UpdateConfig(ctx, pluginID, permissionScope, stored)
		if recoveryErr := s.plugins.Reload(ctx, pluginID); recoveryErr == nil && wasEnabled {
			_ = s.plugins.Enable(ctx, pluginID, pluginScopeFromExecution(scope))
		}
		return err
	}
	return nil
}

func (s *ExtensionService) GetPluginPermissions(ctx context.Context, scope ExecutionScope, pluginID string) ([]PermissionGrantView, error) {
	if _, err := s.pluginEntry(pluginID); err != nil {
		return nil, err
	}
	items, err := s.repository.ListGrants(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	return filterPluginGrants(items, scope), nil
}

func (s *ExtensionService) UpdatePluginPermissions(ctx context.Context, scope ExecutionScope, pluginID string, grants []PermissionGrantInput) error {
	entry, err := s.pluginEntry(pluginID)
	if err != nil {
		return err
	}
	declared := map[string]bool{}
	for _, capability := range entry.registered.Manifest.Capabilities {
		declared[capability] = true
	}
	for _, grant := range grants {
		if !declared[grant.Capability] {
			return NewExtensionError(ErrSkillPermissionDenied, "Plugin capability is not declared", grant.Capability, false, nil)
		}
		if err := validateGrantRoleIsolation(grant, scope); err != nil {
			return err
		}
	}
	if err := s.repository.ReplaceGrants(ctx, pluginID, grants); err != nil {
		return err
	}
	return s.repository.AuditPlugin(ctx, pluginID, "plugin.permissions.updated", pluginScopeFromExecution(scope), scope.TraceID, map[string]any{"count": len(grants)})
}

func (s *ExtensionService) GetPluginHealth(_ context.Context, pluginID string) (PluginHealth, error) {
	entry, err := s.pluginEntry(pluginID)
	if err != nil {
		return PluginHealth{}, err
	}
	entry.mu.RLock()
	view := PluginHealth{PluginID: pluginID, Lifecycle: entry.lifecycle, Health: entry.health, Compatible: entry.registered.Compatible, LastErrorCode: entry.lastError, LastErrorAt: entry.lastErrorAt, Circuits: map[PluginHook]CircuitView{}}
	for hook, circuit := range entry.circuits {
		view.Circuits[hook] = circuit.View(time.Now())
	}
	entry.mu.RUnlock()
	return view, nil
}

func (s *ExtensionService) ResetPluginCircuit(ctx context.Context, pluginID string) error {
	return s.plugins.ResetCircuit(ctx, pluginID)
}

func (s *ExtensionService) GetPluginStates(ctx context.Context, scope ExecutionScope, pluginID string) ([]PluginState, error) {
	if _, err := s.pluginEntry(pluginID); err != nil {
		return nil, err
	}
	return s.repository.ListPluginStates(ctx, pluginID, scope.CharacterID)
}

func (s *ExtensionService) GetPluginSurface(_ context.Context, pluginID string) (json.RawMessage, error) {
	entry, err := s.pluginEntry(pluginID)
	if err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), entry.registered.Manifest.Surface...), nil
}

func (s *ExtensionService) GetPluginSchedules(ctx context.Context, scope ExecutionScope, pluginID string) ([]PluginScheduleDefinition, error) {
	if _, err := s.pluginEntry(pluginID); err != nil {
		return nil, err
	}
	items, err := s.repository.ListPluginSchedules(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	filtered := items[:0]
	for _, item := range items {
		if item.Scope.Type == ScopeGlobal || (item.Scope.Type == ScopeCharacter && item.Scope.ID == scope.CharacterID) {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (s *ExtensionService) SetPluginScheduleEnabled(ctx context.Context, scope ExecutionScope, pluginID, scheduleID string, enabled bool) error {
	if _, err := s.pluginEntry(pluginID); err != nil {
		return err
	}
	owner, err := s.repository.PluginScheduleScope(ctx, pluginID, scheduleID)
	if err != nil {
		return err
	}
	if owner.Type == ScopeCharacter && owner.ID != scope.CharacterID {
		return NewExtensionError(ErrSkillPermissionDenied, "Plugin schedule character scope mismatch", scheduleID, false, nil)
	}
	return s.repository.SetPluginScheduleEnabled(ctx, pluginID, scheduleID, enabled)
}

func (s *ExtensionService) GetPluginEvents(ctx context.Context, scope ExecutionScope, pluginID, status string, page, pageSize int) (PluginEventPage, error) {
	if _, err := s.pluginEntry(pluginID); err != nil {
		return PluginEventPage{}, err
	}
	items, total, err := s.repository.ListPluginEvents(ctx, pluginID, scope.CharacterID, status, page, pageSize)
	if err != nil {
		return PluginEventPage{}, err
	}
	return PluginEventPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *ExtensionService) RetryPluginEvent(ctx context.Context, scope ExecutionScope, pluginID, eventID string) error {
	entry, err := s.pluginEntry(pluginID)
	if err != nil {
		return err
	}
	entry.mu.RLock()
	enabled := entry.enabled
	entry.mu.RUnlock()
	if !enabled {
		return NewExtensionError(ErrPluginDisabled, "Plugin is disabled", pluginID, false, nil)
	}
	event, err := s.repository.PluginEvent(ctx, eventID)
	if err != nil {
		return NewExtensionError(ErrPluginEventDeadLetter, "Dead-letter event not found", eventID, false, err)
	}
	if event.Subject != "" && !strings.Contains(event.Subject, "character/"+scope.CharacterID) {
		return NewExtensionError(ErrSkillPermissionDenied, "Plugin event character scope mismatch", eventID, false, nil)
	}
	return s.repository.RetryPluginEvent(ctx, pluginID, eventID)
}

func (s *ExtensionService) ExecutePluginSurfaceAction(ctx context.Context, scope ExecutionScope, pluginID, actionID string, input json.RawMessage) (SkillResult, error) {
	entry, err := s.pluginEntry(pluginID)
	if err != nil {
		return SkillResult{}, err
	}
	skillID, err := surfaceActionSkill(entry.registered.Manifest, actionID)
	if err != nil {
		return SkillResult{}, err
	}
	scope.Trigger = TriggerManual
	return entry.host.CallSkill(pluginAuthorizedContext(ctx, pluginID), ExecuteSkillRequest{SkillID: skillID, Input: normalizeJSON(input), Scope: scope, IdempotencyKey: scope.RequestID + ":" + pluginID + ":" + actionID})
}

func (s *ExtensionService) requirePluginManager() error {
	if s.plugins == nil {
		return NewExtensionError(ErrPluginLoadFailed, "Plugin Runtime is unavailable", "", false, nil)
	}
	return nil
}

func (s *ExtensionService) pluginEntry(pluginID string) (*pluginRuntimeEntry, error) {
	if err := s.requirePluginManager(); err != nil {
		return nil, err
	}
	return s.plugins.entry(pluginID)
}

func pluginViewFromEntry(entry *pluginRuntimeEntry) PluginView {
	entry.mu.RLock()
	defer entry.mu.RUnlock()
	open := 0
	for _, circuit := range entry.circuits {
		if circuit.View(time.Now()).State != CircuitClosed {
			open++
		}
	}
	return PluginView{Manifest: clonePluginManifest(entry.registered.Manifest), Source: "builtin", Lifecycle: entry.lifecycle, Health: entry.health, Compatible: entry.registered.Compatible, Enabled: entry.enabled, CurrentCircuits: open, LastErrorCode: entry.lastError, LastErrorAt: entry.lastErrorAt}
}

func pluginScopeFromExecution(scope ExecutionScope) PluginStateScope {
	if scope.CharacterID != "" {
		return PluginStateScope{Type: ScopeCharacter, ID: scope.CharacterID}
	}
	return PluginStateScope{Type: ScopeGlobal}
}

func pluginConfigScope(scope ExecutionScope) PermissionScope {
	if scope.CharacterID != "" {
		return PermissionScope{Type: ScopeCharacter, ID: scope.CharacterID}
	}
	return PermissionScope{Type: ScopeGlobal}
}

func filterPluginGrants(items []PermissionGrantView, scope ExecutionScope) []PermissionGrantView {
	filtered := make([]PermissionGrantView, 0, len(items))
	for _, item := range items {
		visible := item.ScopeType == ScopeGlobal
		switch item.ScopeType {
		case ScopeCharacter:
			visible = item.ScopeID == scope.CharacterID
		case ScopeConversation:
			visible = item.ScopeID == scope.ConversationID
		case ScopeChannel:
			visible = item.ScopeID == scope.Channel
		case ScopeSession:
			visible = item.ScopeID == scope.SessionID
		}
		if visible {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
