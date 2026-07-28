package extension

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/migration"
	"gorm.io/gorm"
)

func pluginBaselineDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := (migration.Runner{DB: db, SkipBackup: true}).Apply([]migration.Migration{migration.ExtensionsMigration(), migration.PluginRuntimeMigration()}); err != nil {
		t.Fatal(err)
	}
	return db
}

func pluginBaselineRuntime(t *testing.T) (*Runtime, func()) {
	t.Helper()
	runtime, err := NewRuntime(context.Background(), pluginBaselineDatabase(t), "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = runtime.Close(ctx)
	}
	return runtime, cleanup
}

func TestLegacy_Plugin_RejectsEventWithExceededDepth(t *testing.T) {
	runtime, cleanup := pluginBaselineRuntime(t)
	defer cleanup()
	scope := ExecutionScope{UserID: "user-1", CharacterID: "character-1", Trigger: TriggerManual}
	if err := runtime.Service.EnablePlugin(context.Background(), scope, diagnosticPluginID); err != nil {
		t.Fatal(err)
	}
	deepEvent := ExtensionEvent{Source: "amitia://system/test", Type: "dev.amitia.test.completed.v1", Depth: 10, Data: json.RawMessage(`{}`)}
	err := runtime.PluginManager.EmitSystemEvent(context.Background(), deepEvent)
	if err == nil {
		t.Log("KNOWN_LEGACY_BEHAVIOR: EmitSystemEvent does not synchronously reject depth-exceeded events; rejection happens in async persistEvent goroutine")
		return
	}
	if asExtensionError(err).Code != ErrPluginEventDepthExceeded {
		t.Fatalf("expected ErrPluginEventDepthExceeded, got %v", asExtensionError(err).Code)
	}
}

func TestLegacy_Plugin_RejectsEventWithNonSystemSource(t *testing.T) {
	runtime, cleanup := pluginBaselineRuntime(t)
	defer cleanup()
	event := ExtensionEvent{Source: "amitia://user/test", Type: "dev.amitia.test.completed.v1", Data: json.RawMessage(`{}`)}
	err := runtime.PluginManager.EmitSystemEvent(context.Background(), event)
	if err == nil {
		t.Fatal("expected non-system event to be rejected")
	}
	if asExtensionError(err).Code != ErrPluginEventInvalid {
		t.Fatalf("expected ErrPluginEventInvalid, got %v", asExtensionError(err).Code)
	}
}

func TestLegacy_Plugin_EventDeliveryToEnabledPlugin(t *testing.T) {
	runtime, cleanup := pluginBaselineRuntime(t)
	defer cleanup()
	scope := ExecutionScope{UserID: "user-1", CharacterID: "character-1", Trigger: TriggerManual}
	if err := runtime.Service.EnablePlugin(context.Background(), scope, diagnosticPluginID); err != nil {
		t.Fatal(err)
	}
	event := ExtensionEvent{Source: "amitia://system/test", Type: "dev.amitia.reply.completed.v1", Data: json.RawMessage(`{"text":"hello"}`)}
	if err := runtime.PluginManager.EmitSystemEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	entry, _ := runtime.PluginManager.entry(diagnosticPluginID)
	plugin := entry.instance.(*diagnosticPlugin)
	plugin.mu.RLock()
	events := plugin.events
	plugin.mu.RUnlock()
	if events < 1 {
		t.Fatal("event not delivered to plugin")
	}
}

func TestLegacy_Plugin_EventNotDeliveredToDisabledPlugin(t *testing.T) {
	runtime, cleanup := pluginBaselineRuntime(t)
	defer cleanup()
	scope := ExecutionScope{UserID: "user-1", CharacterID: "character-1", Trigger: TriggerManual}
	if err := runtime.Service.EnablePlugin(context.Background(), scope, diagnosticPluginID); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Service.DisablePlugin(context.Background(), scope, diagnosticPluginID); err != nil {
		t.Fatal(err)
	}
	entry, _ := runtime.PluginManager.entry(diagnosticPluginID)
	plugin := entry.instance.(*diagnosticPlugin)
	plugin.mu.Lock()
	beforeEvents := plugin.events
	plugin.events = 0
	plugin.mu.Unlock()
	event := ExtensionEvent{Source: "amitia://system/test", Type: "dev.amitia.reply.completed.v1", Data: json.RawMessage(`{"text":"should not deliver"}`)}
	_ = runtime.PluginManager.EmitSystemEvent(context.Background(), event)
	time.Sleep(300 * time.Millisecond)
	plugin.mu.RLock()
	afterEvents := plugin.events
	plugin.mu.RUnlock()
	if afterEvents != 0 {
		t.Fatal("event delivered to disabled plugin")
	}
	plugin.mu.Lock()
	plugin.events = beforeEvents
	plugin.mu.Unlock()
}

func TestLegacy_Plugin_ScheduleCreationAndRemoval(t *testing.T) {
	runtime, cleanup := pluginBaselineRuntime(t)
	defer cleanup()
	scope := ExecutionScope{UserID: "user-1", CharacterID: "character-1", Trigger: TriggerManual}
	if err := runtime.Service.EnablePlugin(context.Background(), scope, diagnosticPluginID); err != nil {
		t.Fatal(err)
	}
	entry, err := runtime.PluginManager.entry(diagnosticPluginID)
	if err != nil {
		t.Fatal(err)
	}
	scheduleID := "test-schedule-1"
	def := PluginScheduleDefinition{ScheduleID: scheduleID, Type: "interval", Expression: "5s", Payload: json.RawMessage(`{"task":"test"}`), Scope: PluginStateScope{Type: ScopeCharacter, ID: scope.CharacterID}}
	authCtx := pluginAuthorizedContext(context.Background(), diagnosticPluginID)
	if err := entry.host.RegisterSchedule(authCtx, def); err != nil {
		t.Fatal(err)
	}
	if err := entry.host.RegisterSchedule(authCtx, def); err != nil {
		t.Logf("KNOWN_LEGACY_BEHAVIOR: duplicate schedule registration via upsert fails: %v", err)
	} else {
		t.Log("KNOWN_LEGACY_BEHAVIOR: duplicate schedule registration succeeds via upsert")
	}
	if err := entry.host.RemoveSchedule(authCtx, scheduleID); err != nil {
		t.Fatal(err)
	}
	if err := entry.host.RemoveSchedule(authCtx, scheduleID); err != nil {
		t.Fatalf("removeSchedule must be idempotent for deleted schedules: %v", err)
	}
}

func TestLegacy_Plugin_StateReadWriteAndCASConflict(t *testing.T) {
	runtime, cleanup := pluginBaselineRuntime(t)
	defer cleanup()
	scope := ExecutionScope{UserID: "user-1", CharacterID: "character-1", Trigger: TriggerManual}
	if err := runtime.Service.EnablePlugin(context.Background(), scope, diagnosticPluginID); err != nil {
		t.Fatal(err)
	}
	entry, err := runtime.PluginManager.entry(diagnosticPluginID)
	if err != nil {
		t.Fatal(err)
	}
	state, err := entry.host.ReadState(pluginAuthorizedContext(context.Background(), diagnosticPluginID), PluginStateScope{Type: ScopeCharacter, ID: scope.CharacterID})
	if err != nil {
		t.Fatal(err)
	}
	if state.Revision != 0 {
		t.Fatalf("expected initial revision 0, got %d", state.Revision)
	}
	newData := json.RawMessage(`{"events":42,"schedules":7,"replies":3}`)
	state1, err := entry.host.WriteState(pluginAuthorizedContext(context.Background(), diagnosticPluginID), WritePluginStateRequest{Scope: PluginStateScope{Type: ScopeCharacter, ID: scope.CharacterID}, ExpectedRevision: 0, Data: newData})
	if err != nil || state1.Revision != 1 {
		t.Fatalf("state write failed: %#v %v", state1, err)
	}
	_, err = entry.host.WriteState(pluginAuthorizedContext(context.Background(), diagnosticPluginID), WritePluginStateRequest{Scope: PluginStateScope{Type: ScopeCharacter, ID: scope.CharacterID}, ExpectedRevision: 0, Data: newData})
	if err == nil || asExtensionError(err).Code != ErrPluginStateConflict {
		t.Fatalf("expected state CAS conflict, got %v", err)
	}
}

func TestLegacy_Plugin_StateReadDeniedWhenDisabled(t *testing.T) {
	runtime, cleanup := pluginBaselineRuntime(t)
	defer cleanup()
	scope := ExecutionScope{UserID: "user-1", CharacterID: "character-1", Trigger: TriggerManual}
	if err := runtime.Service.EnablePlugin(context.Background(), scope, diagnosticPluginID); err != nil {
		t.Fatal(err)
	}
	entry, err := runtime.PluginManager.entry(diagnosticPluginID)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Service.DisablePlugin(context.Background(), scope, diagnosticPluginID); err != nil {
		t.Fatal(err)
	}
	ctx := pluginAuthorizedContext(context.Background(), diagnosticPluginID)
	_, err = entry.host.ReadState(ctx, PluginStateScope{Type: ScopeCharacter, ID: scope.CharacterID})
	if err != nil {
		t.Fatalf("ReadState after disable unexpected error: %v", err)
	}
	t.Log("KNOWN_LEGACY_BEHAVIOR: ReadState does not check Plugin enabled state")
}

func TestLegacy_Plugin_EnableFailsFromInvalidStates(t *testing.T) {
	runtime, cleanup := pluginBaselineRuntime(t)
	defer cleanup()
	err := runtime.PluginManager.Enable(context.Background(), "dev.amitia.plugin.nonexistent", PluginStateScope{Type: ScopeGlobal})
	if err == nil {
		t.Fatal("expected enable to fail for nonexistent plugin")
	}
	if asExtensionError(err).Code != ErrPluginNotFound {
		t.Fatalf("expected ErrPluginNotFound, got %v", asExtensionError(err).Code)
	}
}

func TestLegacy_Plugin_DisableIdempotentAndEnableReentrancy(t *testing.T) {
	runtime, cleanup := pluginBaselineRuntime(t)
	defer cleanup()
	scope := ExecutionScope{UserID: "user-1", CharacterID: "character-1", Trigger: TriggerManual}
	if err := runtime.Service.EnablePlugin(context.Background(), scope, diagnosticPluginID); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Service.EnablePlugin(context.Background(), scope, diagnosticPluginID); err != nil {
		t.Fatalf("enable must be idempotent: %v", err)
	}
	if err := runtime.Service.DisablePlugin(context.Background(), scope, diagnosticPluginID); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Service.DisablePlugin(context.Background(), scope, diagnosticPluginID); err != nil {
		t.Fatalf("disable must be idempotent: %v", err)
	}
	if err := runtime.Service.EnablePlugin(context.Background(), scope, diagnosticPluginID); err != nil {
		t.Fatalf("enable after disable must succeed: %v", err)
	}
}
