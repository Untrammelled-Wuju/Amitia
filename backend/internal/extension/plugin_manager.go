package extension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	applog "github.com/u-ai/backend/log"
)

type pluginRuntimeEntry struct {
	mu          sync.RWMutex
	registered  RegisteredPlugin
	instance    Plugin
	host        *pluginHost
	lifecycle   PluginLifecycleStatus
	health      string
	enabled     bool
	lastError   string
	lastErrorAt string
	semaphore   chan struct{}
	circuits    map[PluginHook]*pluginCircuit
}

type afterReplyInvocation struct {
	snapshot ExtensionSnapshot
	reply    ReplyView
}

type pluginIdentityContextKey struct{}

type PluginManager struct {
	registry     *PluginRegistry
	skills       *Registry
	executor     *Executor
	permissions  *DefaultPermissionEvaluator
	repository   *Repository
	validator    *SchemaValidator
	mu           sync.RWMutex
	entries      map[string]*pluginRuntimeEntry
	accepting    bool
	ctx          context.Context
	cancel       context.CancelFunc
	afterReplyQ  chan afterReplyInvocation
	eventWake    chan struct{}
	eventIngress chan ExtensionEvent
	wg           sync.WaitGroup
}

func NewPluginManager(registry *PluginRegistry, skills *Registry, executor *Executor, permissions *DefaultPermissionEvaluator, repository *Repository, validator *SchemaValidator) *PluginManager {
	return &PluginManager{registry: registry, skills: skills, executor: executor, permissions: permissions, repository: repository, validator: validator, entries: map[string]*pluginRuntimeEntry{}, afterReplyQ: make(chan afterReplyInvocation, 128), eventWake: make(chan struct{}, 1), eventIngress: make(chan ExtensionEvent, 128)}
}

func (m *PluginManager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.accepting {
		m.mu.Unlock()
		return nil
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.accepting = true
	m.mu.Unlock()
	plugins, err := m.registry.List(ctx, PluginFilter{})
	if err != nil {
		return err
	}
	for _, registered := range plugins {
		if err := m.load(ctx, registered); err != nil {
			applog.Warn("official plugin load failed", applog.Fields{"plugin_id": registered.Manifest.Metadata.ID, "error_code": asExtensionError(err).Code})
		}
	}
	m.wg.Add(4)
	go m.afterReplyWorker()
	go m.eventIngressWorker()
	go m.eventWorker()
	go m.scheduleWorker()
	return nil
}

func (m *PluginManager) Stop(ctx context.Context) error {
	m.mu.Lock()
	if !m.accepting {
		m.mu.Unlock()
		return nil
	}
	m.accepting = false
	if m.cancel != nil {
		m.cancel()
	}
	entries := m.sortedEntries()
	m.mu.Unlock()
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
	for _, entry := range entries {
		entry.mu.Lock()
		if entry.lifecycle == PluginUnloaded {
			entry.mu.Unlock()
			continue
		}
		entry.lifecycle = PluginUnloading
		entry.enabled = false
		entry.mu.Unlock()
		if hook, ok := entry.instance.(UnloadHook); ok && hasPluginHook(entry.registered.Manifest.Hooks, HookOnUnload) {
			_ = m.invoke(entry, HookOnUnload, ExecutionScope{}, true, func(callCtx context.Context) error { return hook.OnUnload(callCtx) })
		}
		entry.mu.Lock()
		entry.lifecycle = PluginUnloaded
		entry.mu.Unlock()
	}
	return nil
}

func (m *PluginManager) load(ctx context.Context, registered RegisteredPlugin) error {
	pluginID := registered.Manifest.Metadata.ID
	instance := registered.Factory()
	if instance == nil {
		return NewExtensionError(ErrPluginLoadFailed, "Plugin factory returned nil", pluginID, false, nil)
	}
	enabled, err := m.repository.UpsertPlugin(ctx, registered, PluginRegistered, "unknown")
	if err != nil {
		return err
	}
	entry := &pluginRuntimeEntry{registered: registered, instance: instance, lifecycle: PluginRegistered, health: "unknown", enabled: false, semaphore: make(chan struct{}, registered.Manifest.Execution.MaxConcurrency), circuits: map[PluginHook]*pluginCircuit{}}
	for _, hook := range registered.Manifest.Hooks {
		entry.circuits[hook] = newPluginCircuit(registered.Manifest.Execution.FailureThreshold, time.Duration(registered.Manifest.Execution.CircuitOpenMS)*time.Millisecond)
	}
	host := &pluginHost{manager: m, pluginID: pluginID, version: registered.Manifest.Metadata.Version}
	entry.host = host
	m.mu.Lock()
	m.entries[pluginID] = entry
	m.mu.Unlock()
	if !registered.Compatible {
		entry.lifecycle, entry.health, entry.lastError = PluginError, "error", ErrPluginIncompatible
		_ = m.repository.UpdatePluginLifecycle(ctx, pluginID, false, PluginError, "error", ErrPluginIncompatible)
		return NewExtensionError(ErrPluginIncompatible, "Plugin is incompatible", registered.CompatibilityReason, false, nil)
	}
	if hook, ok := instance.(LoadHook); ok && hasPluginHook(registered.Manifest.Hooks, HookOnLoad) {
		if err := m.invoke(entry, HookOnLoad, ExecutionScope{}, true, func(callCtx context.Context) error { return hook.OnLoad(callCtx, host) }); err != nil {
			m.setEntryError(ctx, entry, ErrPluginLoadFailed)
			return err
		}
	}
	if err := host.verifyRegisteredSkills(); err != nil {
		m.setEntryError(ctx, entry, ErrPluginLoadFailed)
		return err
	}
	if err := m.migrateStates(ctx, entry); err != nil {
		m.setEntryError(ctx, entry, ErrPluginStateMigration)
		return err
	}
	entry.mu.Lock()
	entry.lifecycle, entry.health = PluginLoaded, "healthy"
	entry.mu.Unlock()
	if enabled {
		return m.Enable(ctx, pluginID, PluginStateScope{Type: ScopeGlobal})
	}
	entry.mu.Lock()
	entry.lifecycle, entry.health = PluginDisabled, "healthy"
	entry.mu.Unlock()
	return m.repository.UpdatePluginLifecycle(ctx, pluginID, false, PluginDisabled, "healthy", "")
}

func (m *PluginManager) Enable(ctx context.Context, pluginID string, scope PluginStateScope) error {
	entry, err := m.entry(pluginID)
	if err != nil {
		return err
	}
	entry.mu.Lock()
	if entry.enabled && entry.lifecycle == PluginEnabled {
		entry.mu.Unlock()
		return nil
	}
	if !entry.registered.Compatible {
		entry.mu.Unlock()
		return NewExtensionError(ErrPluginIncompatible, "Plugin is incompatible", pluginID, false, nil)
	}
	if entry.lifecycle != PluginLoaded && entry.lifecycle != PluginDisabled && entry.lifecycle != PluginCircuitOpen {
		current := entry.lifecycle
		entry.mu.Unlock()
		return NewExtensionError(ErrPluginEnableFailed, "Plugin cannot be enabled from current state", string(current), false, nil)
	}
	entry.mu.Unlock()
	if hook, ok := entry.instance.(EnableHook); ok && hasPluginHook(entry.registered.Manifest.Hooks, HookOnEnable) {
		if err := m.invoke(entry, HookOnEnable, executionFromPluginScope(scope), true, func(callCtx context.Context) error { return hook.OnEnable(callCtx) }); err != nil {
			_ = m.repository.UpdatePluginLifecycle(ctx, pluginID, false, PluginDisabled, "degraded", ErrPluginEnableFailed)
			return NewExtensionError(ErrPluginEnableFailed, "Plugin enable hook failed", pluginID, false, err)
		}
	}
	for _, skillID := range entry.registered.Manifest.RegisteredSkills {
		if err := m.skills.SetEnabled(ctx, skillID, true); err != nil {
			for _, rollbackID := range entry.registered.Manifest.RegisteredSkills {
				_ = m.skills.SetEnabled(ctx, rollbackID, false)
			}
			return NewExtensionError(ErrPluginEnableFailed, "Plugin skill enable failed", skillID, false, err)
		}
	}
	entry.mu.Lock()
	entry.enabled, entry.lifecycle, entry.health, entry.lastError = true, PluginEnabled, "healthy", ""
	entry.mu.Unlock()
	if err := m.repository.UpdatePluginLifecycle(ctx, pluginID, true, PluginEnabled, "healthy", ""); err != nil {
		return err
	}
	_ = m.repository.AuditPlugin(ctx, pluginID, "plugin.enabled", scope, "", map[string]any{"enabled": true})
	m.emitLifecycleEvent(ctx, entry, "dev.amitia.extension.plugin.enabled.v1")
	return nil
}

func (m *PluginManager) Disable(ctx context.Context, pluginID string, scope PluginStateScope) error {
	entry, err := m.entry(pluginID)
	if err != nil {
		return err
	}
	entry.mu.Lock()
	if !entry.enabled && entry.lifecycle == PluginDisabled {
		entry.mu.Unlock()
		return nil
	}
	if entry.lifecycle != PluginEnabled && entry.lifecycle != PluginCircuitOpen && entry.lifecycle != PluginLoaded {
		current := entry.lifecycle
		entry.mu.Unlock()
		return NewExtensionError(ErrPluginDisableFailed, "Plugin cannot be disabled from current state", string(current), false, nil)
	}
	entry.enabled = false
	entry.lifecycle = PluginDisabled
	entry.mu.Unlock()
	for _, skillID := range entry.registered.Manifest.RegisteredSkills {
		_ = m.skills.SetEnabled(ctx, skillID, false)
	}
	var hookErr error
	if hook, ok := entry.instance.(DisableHook); ok && hasPluginHook(entry.registered.Manifest.Hooks, HookOnDisable) {
		hookErr = m.invoke(entry, HookOnDisable, executionFromPluginScope(scope), true, func(callCtx context.Context) error { return hook.OnDisable(callCtx) })
	}
	health, code := "healthy", ""
	if hookErr != nil {
		health, code = "degraded", ErrPluginDisableFailed
	}
	entry.mu.Lock()
	entry.health = health
	entry.lastError = code
	entry.mu.Unlock()
	if err := m.repository.UpdatePluginLifecycle(ctx, pluginID, false, PluginDisabled, health, code); err != nil {
		return err
	}
	_ = m.repository.AuditPlugin(ctx, pluginID, "plugin.disabled", scope, "", map[string]any{"hookError": hookErr != nil})
	m.emitLifecycleEvent(ctx, entry, "dev.amitia.extension.plugin.disabled.v1")
	if hookErr != nil {
		return NewExtensionError(ErrPluginDisableFailed, "Plugin disable hook failed", pluginID, false, hookErr)
	}
	return nil
}

func (m *PluginManager) Reload(ctx context.Context, pluginID string) error {
	entry, err := m.entry(pluginID)
	if err != nil {
		return err
	}
	entry.mu.RLock()
	wasEnabled := entry.enabled
	registered := entry.registered
	entry.mu.RUnlock()
	if wasEnabled {
		if err := m.Disable(ctx, pluginID, PluginStateScope{Type: ScopeGlobal}); err != nil {
			return err
		}
	}
	if hook, ok := entry.instance.(UnloadHook); ok && hasPluginHook(registered.Manifest.Hooks, HookOnUnload) {
		_ = m.invoke(entry, HookOnUnload, ExecutionScope{}, true, func(callCtx context.Context) error { return hook.OnUnload(callCtx) })
	}
	for _, skillID := range registered.Manifest.RegisteredSkills {
		_ = m.skills.Unregister(ctx, skillID)
	}
	m.mu.Lock()
	delete(m.entries, pluginID)
	m.mu.Unlock()
	if err := m.load(ctx, registered); err != nil {
		return err
	}
	if wasEnabled {
		return m.Enable(ctx, pluginID, PluginStateScope{Type: ScopeGlobal})
	}
	return nil
}

func (m *PluginManager) ResetCircuit(ctx context.Context, pluginID string) error {
	entry, err := m.entry(pluginID)
	if err != nil {
		return err
	}
	for _, circuit := range entry.circuits {
		circuit.Reset()
	}
	_ = m.repository.AuditPlugin(ctx, pluginID, "plugin.circuit.reset", PluginStateScope{Type: ScopeGlobal}, "", map[string]any{})
	return nil
}

func (m *PluginManager) DispatchBeforePrompt(ctx context.Context, snapshot ExtensionSnapshot) []ContextContribution {
	deadlineCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	entries := m.sortedEntriesForHook(HookBeforePrompt)
	contributions := make([]ContextContribution, 0)
	for _, entry := range entries {
		if deadlineCtx.Err() != nil {
			break
		}
		hook := entry.instance.(BeforePromptHook)
		var returned []ContextContribution
		err := m.invoke(entry, HookBeforePrompt, scopeFromSnapshot(snapshot), false, func(callCtx context.Context) error {
			var callErr error
			returned, callErr = hook.BeforePrompt(callCtx, snapshot)
			return callErr
		})
		if err != nil {
			continue
		}
		for _, contribution := range returned {
			validated, ok := validateContribution(entry.registered.Manifest.Metadata.ID, contribution)
			if ok {
				contributions = append(contributions, validated)
			}
		}
	}
	sort.SliceStable(contributions, func(i, j int) bool {
		if contributions[i].Priority == contributions[j].Priority {
			return contributions[i].Source < contributions[j].Source
		}
		return contributions[i].Priority > contributions[j].Priority
	})
	used := 0
	filtered := contributions[:0]
	for _, contribution := range contributions {
		estimated := (len([]rune(contribution.Content)) + 3) / 4
		if estimated > contribution.TokenLimit {
			estimated = contribution.TokenLimit
		}
		if used+estimated > 1200 {
			continue
		}
		used += estimated
		filtered = append(filtered, contribution)
	}
	return filtered
}

func (m *PluginManager) DispatchAfterReply(snapshot ExtensionSnapshot, reply ReplyView) bool {
	m.mu.RLock()
	accepting := m.accepting
	m.mu.RUnlock()
	if !accepting {
		return false
	}
	select {
	case m.afterReplyQ <- afterReplyInvocation{snapshot: snapshot, reply: reply}:
		return true
	default:
		applog.Warn("plugin after-reply queue full", applog.Fields{"conversation_id": snapshot.Conversation.ID})
		return false
	}
}

func (m *PluginManager) EmitSystemEvent(ctx context.Context, event ExtensionEvent) error {
	if event.Source == "" {
		event.Source = "amitia://system"
	}
	if !strings.HasPrefix(event.Source, "amitia://system") {
		return NewExtensionError(ErrPluginEventInvalid, "System event source is invalid", event.Source, false, nil)
	}
	select {
	case m.eventIngress <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return NewExtensionError(ErrPluginEventInvalid, "Plugin event queue is full", event.Type, true, nil)
	}
}

func (m *PluginManager) persistEvent(ctx context.Context, event ExtensionEvent) error {
	if event.Depth > 8 {
		return NewExtensionError(ErrPluginEventDepthExceeded, "Plugin event depth exceeded", event.Type, false, nil)
	}
	if !validEventType(event.Type) {
		return NewExtensionError(ErrPluginEventInvalid, "Plugin event type is invalid", event.Type, false, nil)
	}
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.SpecVersion == "" {
		event.SpecVersion = "1.0"
	}
	if event.SpecVersion != "1.0" {
		return NewExtensionError(ErrPluginEventInvalid, "Plugin event specversion is invalid", event.SpecVersion, false, nil)
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	if event.DataContentType == "" {
		event.DataContentType = "application/json"
	}
	entries := m.sortedEntriesForEvent(event.Type)
	pluginIDs := make([]string, 0, len(entries))
	for _, entry := range entries {
		pluginIDs = append(pluginIDs, entry.registered.Manifest.Metadata.ID)
	}
	if err := m.repository.CreatePluginEvent(ctx, event, pluginIDs); err != nil {
		return err
	}
	select {
	case m.eventWake <- struct{}{}:
	default:
	}
	return nil
}

func (m *PluginManager) invoke(entry *pluginRuntimeEntry, hook PluginHook, scope ExecutionScope, allowDisabled bool, call func(context.Context) error) error {
	entry.mu.RLock()
	enabled := entry.enabled
	lifecycle := entry.lifecycle
	manifest := entry.registered.Manifest
	circuit := entry.circuits[hook]
	entry.mu.RUnlock()
	if !allowDisabled && (!enabled || lifecycle != PluginEnabled) {
		return NewExtensionError(ErrPluginDisabled, "Plugin is disabled", manifest.Metadata.ID, false, nil)
	}
	if circuit != nil && !circuit.Allow(time.Now()) {
		return NewExtensionError(ErrPluginCircuitOpen, "Plugin hook circuit is open", string(hook), true, nil)
	}
	callCtx, cancel := context.WithTimeout(context.Background(), time.Duration(manifest.Execution.HookTimeoutMS)*time.Millisecond)
	defer cancel()
	callCtx = context.WithValue(callCtx, pluginIdentityContextKey{}, manifest.Metadata.ID)
	select {
	case entry.semaphore <- struct{}{}:
	case <-callCtx.Done():
		return NewExtensionError(ErrPluginHookTimeout, "Plugin hook concurrency wait timed out", string(hook), true, callCtx.Err())
	}
	started := time.Now()
	result := make(chan error, 1)
	go func() {
		defer func() {
			<-entry.semaphore
			if recovered := recover(); recovered != nil {
				result <- fmt.Errorf("plugin panic")
			}
		}()
		result <- call(callCtx)
	}()
	var err error
	select {
	case err = <-result:
	case <-callCtx.Done():
		err = NewExtensionError(ErrPluginHookTimeout, "Plugin hook timed out", string(hook), true, callCtx.Err())
	}
	errorCode := ""
	status := "succeeded"
	circuitState := CircuitClosed
	if err != nil {
		status, errorCode = "failed", ErrPluginHookFailed
		if errors.Is(err, context.DeadlineExceeded) || asExtensionError(err).Code == ErrPluginHookTimeout {
			status, errorCode = "timed_out", ErrPluginHookTimeout
		}
		if circuit != nil {
			circuitState, _ = circuit.Failure(time.Now())
		}
		entry.mu.Lock()
		entry.health, entry.lastError, entry.lastErrorAt = "degraded", errorCode, time.Now().UTC().Format(time.RFC3339Nano)
		entry.mu.Unlock()
	} else if circuit != nil {
		circuitState, _ = circuit.Success()
	}
	run := PluginRunView{RunID: uuid.NewString(), PluginID: manifest.Metadata.ID, PluginVersion: manifest.Metadata.Version, Hook: hook, CharacterID: scope.CharacterID, ConversationID: scope.ConversationID, Channel: scope.Channel, Status: status, DurationMS: time.Since(started).Milliseconds(), ErrorCode: errorCode, TraceID: scope.TraceID, CircuitState: circuitState, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	_ = m.repository.CreatePluginRun(context.Background(), run)
	return err
}

func (m *PluginManager) afterReplyWorker() {
	defer m.wg.Done()
	for {
		select {
		case invocation := <-m.afterReplyQ:
			for _, entry := range m.sortedEntriesForHook(HookAfterReply) {
				hook := entry.instance.(AfterReplyHook)
				_ = m.invoke(entry, HookAfterReply, scopeFromSnapshot(invocation.snapshot), false, func(callCtx context.Context) error {
					return hook.AfterReply(callCtx, invocation.snapshot, invocation.reply)
				})
			}
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *PluginManager) eventWorker() {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.eventWake:
			m.processPendingEvents()
		case <-ticker.C:
			m.processPendingEvents()
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *PluginManager) eventIngressWorker() {
	defer m.wg.Done()
	for {
		select {
		case event := <-m.eventIngress:
			if err := m.persistEvent(m.ctx, event); err != nil {
				applog.Warn("plugin event persistence failed", applog.Fields{"event_type": event.Type, "error_code": asExtensionError(err).Code})
			}
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *PluginManager) processPendingEvents() {
	deliveries, err := m.repository.PendingPluginDeliveries(m.ctx, 20)
	if err != nil {
		return
	}
	for _, delivery := range deliveries {
		entry, getErr := m.entry(delivery.PluginID)
		if getErr != nil {
			continue
		}
		entry.mu.RLock()
		enabled := entry.enabled && entry.lifecycle == PluginEnabled
		entry.mu.RUnlock()
		if !enabled {
			continue
		}
		event, eventErr := m.repository.PluginEvent(m.ctx, delivery.EventID)
		if eventErr != nil {
			continue
		}
		hook, ok := entry.instance.(EventHook)
		if !ok || !hasPluginHook(entry.registered.Manifest.Hooks, HookOnEvent) {
			continue
		}
		err = m.invoke(entry, HookOnEvent, scopeFromEvent(event), false, func(callCtx context.Context) error { return hook.OnEvent(callCtx, event) })
		if err == nil {
			_ = m.repository.UpdatePluginDelivery(m.ctx, delivery, "completed", "", "", time.Time{})
			continue
		}
		if delivery.Attempts+1 >= 3 {
			_ = m.repository.UpdatePluginDelivery(m.ctx, delivery, "dead_letter", asExtensionError(err).Code, "plugin event handler failed", time.Time{})
		} else {
			delay := time.Duration(1<<delivery.Attempts) * time.Second
			_ = m.repository.UpdatePluginDelivery(m.ctx, delivery, "failed", asExtensionError(err).Code, "plugin event handler failed", time.Now().Add(delay))
		}
	}
}

func (m *PluginManager) scheduleWorker() {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.processDueSchedules()
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *PluginManager) processDueSchedules() {
	records, err := m.repository.DuePluginSchedules(m.ctx, 20)
	if err != nil {
		return
	}
	for _, record := range records {
		entry, getErr := m.entry(record.PluginID)
		if getErr != nil {
			continue
		}
		entry.mu.RLock()
		enabled := entry.enabled && entry.lifecycle == PluginEnabled
		entry.mu.RUnlock()
		if !enabled {
			continue
		}
		hook, ok := entry.instance.(ScheduleHook)
		if !ok || !hasPluginHook(entry.registered.Manifest.Hooks, HookOnSchedule) {
			continue
		}
		invocation := PluginScheduleInvocation{PluginID: record.PluginID, ScheduleID: record.ScheduleID, InvocationID: uuid.NewString(), Scope: PluginStateScope{Type: ScopeType(record.ScopeType), ID: record.ScopeID}, Payload: json.RawMessage(record.PayloadJSON), TriggeredAt: time.Now().UTC()}
		err = m.invoke(entry, HookOnSchedule, executionFromPluginScope(invocation.Scope), false, func(callCtx context.Context) error { return hook.OnSchedule(callCtx, invocation) })
		next := nextScheduleRun(record)
		status := "succeeded"
		if err != nil {
			status = "failed"
		}
		_ = m.repository.CompletePluginSchedule(m.ctx, record, status, next)
	}
}

func (m *PluginManager) entry(pluginID string) (*pluginRuntimeEntry, error) {
	m.mu.RLock()
	entry := m.entries[pluginID]
	m.mu.RUnlock()
	if entry == nil {
		return nil, NewExtensionError(ErrPluginNotFound, "Plugin not found", pluginID, false, nil)
	}
	return entry, nil
}

func (m *PluginManager) sortedEntries() []*pluginRuntimeEntry {
	entries := make([]*pluginRuntimeEntry, 0, len(m.entries))
	for _, entry := range m.entries {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].registered.Manifest.Metadata.ID < entries[j].registered.Manifest.Metadata.ID
	})
	return entries
}

func (m *PluginManager) sortedEntriesForHook(hook PluginHook) []*pluginRuntimeEntry {
	m.mu.RLock()
	entries := m.sortedEntries()
	m.mu.RUnlock()
	result := entries[:0]
	for _, entry := range entries {
		entry.mu.RLock()
		eligible := entry.enabled && entry.lifecycle == PluginEnabled && hasPluginHook(entry.registered.Manifest.Hooks, hook)
		entry.mu.RUnlock()
		if eligible {
			result = append(result, entry)
		}
	}
	return result
}

func (m *PluginManager) sortedEntriesForEvent(eventType string) []*pluginRuntimeEntry {
	entries := m.sortedEntriesForHook(HookOnEvent)
	result := entries[:0]
	for _, entry := range entries {
		for _, subscription := range entry.registered.Manifest.Subscriptions {
			if subscription == eventType {
				result = append(result, entry)
				break
			}
		}
	}
	return result
}

func (m *PluginManager) setEntryError(ctx context.Context, entry *pluginRuntimeEntry, code string) {
	entry.mu.Lock()
	entry.lifecycle, entry.health, entry.enabled, entry.lastError, entry.lastErrorAt = PluginError, "error", false, code, time.Now().UTC().Format(time.RFC3339Nano)
	entry.mu.Unlock()
	_ = m.repository.UpdatePluginLifecycle(ctx, entry.registered.Manifest.Metadata.ID, false, PluginError, "error", code)
}

func (m *PluginManager) emitLifecycleEvent(ctx context.Context, entry *pluginRuntimeEntry, eventType string) {
	raw, _ := json.Marshal(map[string]string{"pluginId": entry.registered.Manifest.Metadata.ID})
	_ = m.persistEvent(ctx, ExtensionEvent{Source: "amitia://system/extensions", Type: eventType, Data: raw})
}

func (m *PluginManager) migrateStates(ctx context.Context, entry *pluginRuntimeEntry) error {
	migrator, ok := entry.instance.(StateMigrator)
	if !ok {
		return nil
	}
	if migrator.CurrentVersion() != entry.registered.Manifest.State.SchemaVersion {
		return NewExtensionError(ErrPluginStateMigration, "Plugin state migrator version mismatch", entry.registered.Manifest.Metadata.ID, false, nil)
	}
	states, err := m.repository.AllPluginStates(ctx, entry.registered.Manifest.Metadata.ID)
	if err != nil {
		return err
	}
	for _, state := range states {
		if state.SchemaVersion == migrator.CurrentVersion() {
			continue
		}
		newVersion, data, migrateErr := migrator.Migrate(ctx, state.SchemaVersion, state.Data)
		if migrateErr != nil {
			return NewExtensionError(ErrPluginStateMigration, "Plugin state migration failed", entry.registered.Manifest.Metadata.ID, false, migrateErr)
		}
		if newVersion != migrator.CurrentVersion() {
			return NewExtensionError(ErrPluginStateMigration, "Plugin state migration returned invalid version", newVersion, false, nil)
		}
		if len(entry.registered.Manifest.State.Schema) > 0 {
			if err := m.validator.Validate(entry.registered.Manifest.Metadata.ID+"-migrated-state", entry.registered.Manifest.State.Schema, data); err != nil {
				return NewExtensionError(ErrPluginStateMigration, "Migrated plugin state is invalid", err.Error(), false, err)
			}
		}
		_, err = m.repository.CompareAndSwapPluginState(ctx, entry.registered.Manifest.Metadata.ID, newVersion, WritePluginStateRequest{Scope: PluginStateScope{Type: state.ScopeType, ID: state.ScopeID}, ExpectedRevision: state.Revision, Data: data})
		if err != nil {
			return err
		}
		_ = m.repository.AuditPlugin(ctx, entry.registered.Manifest.Metadata.ID, "plugin.state.migrated", PluginStateScope{Type: state.ScopeType, ID: state.ScopeID}, "", map[string]any{"from": state.SchemaVersion, "to": newVersion})
	}
	return nil
}

func pluginAuthorizedContext(ctx context.Context, pluginID string) context.Context {
	return context.WithValue(ctx, pluginIdentityContextKey{}, pluginID)
}

func validateContribution(pluginID string, contribution ContextContribution) (ContextContribution, bool) {
	content := strings.TrimSpace(contribution.Content)
	if content == "" || len([]rune(content)) > 4096 || contribution.TokenLimit < 1 {
		return ContextContribution{}, false
	}
	lower := strings.ToLower(content)
	for _, forbidden := range []string{"ignore previous", "ignore all", "system prompt", "developer message", "忽略之前", "忽略所有", "系统提示词", "</system", "<system"} {
		if strings.Contains(lower, forbidden) {
			return ContextContribution{}, false
		}
	}
	if contribution.ExpiresAt != nil && !contribution.ExpiresAt.After(time.Now()) {
		return ContextContribution{}, false
	}
	contribution.Source = pluginID
	contribution.Content = content
	if contribution.TokenLimit > 512 {
		contribution.TokenLimit = 512
	}
	return contribution, true
}

func executionFromPluginScope(scope PluginStateScope) ExecutionScope {
	result := ExecutionScope{Trigger: TriggerSystemEvent}
	if scope.Type == ScopeCharacter {
		result.CharacterID = scope.ID
	}
	if scope.Type == ScopeConversation {
		result.ConversationID = scope.ID
	}
	return result
}

func scopeFromSnapshot(snapshot ExtensionSnapshot) ExecutionScope {
	return ExecutionScope{UserID: snapshot.User.ID, CharacterID: snapshot.Character.ID, ConversationID: snapshot.Conversation.ID, Channel: snapshot.Channel.Name, Trigger: TriggerSystemEvent}
}

func scopeFromEvent(event ExtensionEvent) ExecutionScope {
	return ExecutionScope{TraceID: event.TraceID, CorrelationID: event.CorrelationID, CausationID: event.CausationID, Trigger: TriggerSystemEvent}
}

func nextScheduleRun(record pluginScheduleRecord) string {
	if record.ScheduleType != "interval" {
		return ""
	}
	duration, err := time.ParseDuration(record.Expression)
	if err != nil || duration < time.Second {
		return ""
	}
	return time.Now().UTC().Add(duration).Format(time.RFC3339Nano)
}
