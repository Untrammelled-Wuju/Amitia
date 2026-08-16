package event

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

type mockPermissionChecker struct {
	granted bool
	mu      sync.Mutex
}

func (m *mockPermissionChecker) CheckSubscriptionPermission(_ context.Context, _ EventSubscriptionDefinition) (bool, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.granted {
		return true, "", nil
	}
	return false, "permission_denied", nil
}

func (m *mockPermissionChecker) setGranted(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.granted = v
}

type mockScopeChecker struct {
	valid bool
	mu    sync.Mutex
}

func (m *mockScopeChecker) CheckSubscriptionScope(_ context.Context, _ EventSubscriptionDefinition, _ EventEnvelope) (bool, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.valid {
		return true, "", nil
	}
	return false, "delivery_rejected_scope", nil
}

func (m *mockScopeChecker) setValid(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.valid = v
}

type mockDependencyChecker struct {
	ready bool
}

func (m *mockDependencyChecker) CheckSubscriptionDependencies(_ context.Context, _ EventSubscriptionDefinition) (bool, string, error) {
	if m.ready {
		return true, "", nil
	}
	return false, "dependency_missing", nil
}

type mockRuntimeChecker struct {
	available bool
}

func (m *mockRuntimeChecker) CheckSubscriptionRuntime(_ context.Context, _ EventSubscriptionDefinition) (bool, string, error) {
	if m.available {
		return true, "", nil
	}
	return false, "blocked_runtime", nil
}

type mockCircuitLookup struct{}

func (m *mockCircuitLookup) LookupCircuitState(_ string) CircuitState {
	return CircuitClosed
}

type mockDeliveryHandler struct {
	mu         sync.Mutex
	callCount  int
	failUntil  int
	failReason string
}

func (m *mockDeliveryHandler) handle(_ context.Context, _ Delivery, _ EventEnvelope, _ *ResolvedSubscription) error {
	m.mu.Lock()
	m.callCount++
	fail := m.callCount <= m.failUntil
	reason := m.failReason
	m.mu.Unlock()
	if !fail {
		return nil
	}
	return mockErrorFromReason(reason)
}

func mockErrorFromReason(reason string) error {
	switch reason {
	case "transient_error", "timeout":
		return ErrTimeout
	case "runtime_error":
		return ErrRuntimeCrashed
	case "runtime_unavailable":
		return ErrRuntimeUnavailable
	case "temporary_dependency_unavailable":
		return ErrTemporaryDependencyUnavailable
	case "temporary_host_error":
		return ErrTemporaryHostError
	case "rate_limited":
		return ErrRateLimited
	case "permanent_error", "invalid_result":
		return ErrInvalidResult
	case "protocol_error":
		return ErrProtocolError
	default:
		return fmt.Errorf("%s", reason)
	}
}

func (m *mockDeliveryHandler) reset(failUntil int, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount = 0
	m.failUntil = failUntil
	m.failReason = reason
}

func (m *mockDeliveryHandler) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func setupTestService(t *testing.T, permGranted, scopeValid, depReady, runtimeAvailable bool) (*Service, *mockPermissionChecker, *mockScopeChecker, *mockDeliveryHandler, func()) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("event_test_%d.db", time.Now().UnixNano()))
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := migrateTestDB(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	permChecker := &mockPermissionChecker{granted: permGranted}
	scopeChecker := &mockScopeChecker{valid: scopeValid}
	depChecker := &mockDependencyChecker{ready: depReady}
	runtimeChecker := &mockRuntimeChecker{available: runtimeAvailable}
	resolver := NewDefaultEffectiveResolver(permChecker, scopeChecker, depChecker, runtimeChecker, &mockCircuitLookup{}, nil)

	cfg := DefaultServiceConfig().WithDB(db)
	cfg.Dispatcher.PollInterval = 50 * time.Millisecond
	cfg.Dispatcher.BatchSize = 10
	cfg.Dispatcher.GlobalConcurrency = 4
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	if err := svc.SetEffectiveResolver(resolver); err != nil {
		t.Fatalf("set effective resolver: %v", err)
	}

	handler := &mockDeliveryHandler{}
	svc.SetDeliveryHandler(handler.handle)

	if err := svc.RegisterDefaultEventTypes(context.Background()); err != nil {
		t.Fatalf("register default event types: %v", err)
	}
	if err := svc.RegisterEventType(context.Background(), testEventTypeDefinition()); err != nil {
		t.Fatalf("register test event type: %v", err)
	}

	cleanup := func() {
		svc.Stop()
		_ = db.Close()
		_ = os.Remove(dbPath)
	}
	return svc, permChecker, scopeChecker, handler, cleanup
}

func migrateTestDB(db *sql.DB) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS extension_event_types (
			event_type_id TEXT NOT NULL,
			version INTEGER NOT NULL,
			description TEXT,
			payload_schema_json TEXT,
			metadata_schema_json TEXT,
			producer_policy_json TEXT NOT NULL,
			subscriber_policy_json TEXT NOT NULL,
			delivery_policy_json TEXT NOT NULL,
			ordering_policy TEXT NOT NULL,
			retention_policy_json TEXT NOT NULL,
			sensitive_fields_json TEXT,
			projection_rules_json TEXT,
			max_payload_bytes INTEGER NOT NULL,
			max_metadata_bytes INTEGER NOT NULL,
			risk_level TEXT NOT NULL DEFAULT 'low',
			definition_hash TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			PRIMARY KEY (event_type_id, version)
		)`,
		`CREATE TABLE IF NOT EXISTS extension_event_outbox (
			outbox_id TEXT PRIMARY KEY,
			event_id TEXT NOT NULL UNIQUE,
			event_type_id TEXT NOT NULL,
			event_version INTEGER NOT NULL,
			producer_id TEXT NOT NULL,
			producer_type TEXT NOT NULL,
			producer_generation INTEGER NOT NULL DEFAULT 0,
			event_domain TEXT NOT NULL DEFAULT '',
			causation_id TEXT NOT NULL DEFAULT '',
			aggregate_type TEXT,
			aggregate_id TEXT,
			aggregate_version INTEGER,
			partition_key TEXT,
			ordering_key TEXT,
			idempotency_key TEXT NOT NULL UNIQUE,
			scope_snapshot_id TEXT,
			permission_snapshot_id TEXT,
			trace_id TEXT,
			operation_id TEXT,
			parent_event_id TEXT,
			depth INTEGER NOT NULL DEFAULT 0,
			occurred_at DATETIME NOT NULL,
			published_at DATETIME,
			payload_json TEXT NOT NULL,
			metadata_json TEXT,
			payload_hash TEXT NOT NULL,
			definition_hash TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			available_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			error_code TEXT,
			error_message TEXT,
			lease_owner TEXT,
			lease_expires_at DATETIME,
			dispatched_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS extension_event_subscriptions (
			contribution_id TEXT PRIMARY KEY,
			extension_id TEXT NOT NULL,
			module_id TEXT NOT NULL,
			event_type_id TEXT NOT NULL,
			event_version_range TEXT,
			entry TEXT NOT NULL,
			filter_json TEXT,
			projection_json TEXT,
			delivery_policy_json TEXT,
			retry_policy_json TEXT,
			ordering_requirement TEXT NOT NULL DEFAULT 'none',
			timeout_ms INTEGER NOT NULL DEFAULT 5000,
			max_in_flight INTEGER NOT NULL DEFAULT 4,
			permission_requirements_json TEXT,
			scope_rule_json TEXT,
			dependency_requirements_json TEXT,
			runtime_binding_json TEXT,
			definition_hash TEXT NOT NULL,
			generation INTEGER NOT NULL DEFAULT 1,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			UNIQUE(extension_id, contribution_id)
		)`,
		`CREATE TABLE IF NOT EXISTS extension_event_deliveries (
			delivery_id TEXT PRIMARY KEY,
			event_id TEXT NOT NULL,
			subscription_id TEXT NOT NULL,
			extension_id TEXT NOT NULL,
			module_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			partition_key TEXT,
			ordering_key TEXT,
			sequence INTEGER NOT NULL DEFAULT 0,
			attempt INTEGER NOT NULL DEFAULT 0,
			max_attempts INTEGER NOT NULL DEFAULT 5,
			available_at DATETIME NOT NULL,
			lease_owner TEXT,
			lease_expires_at DATETIME,
			runtime_instance_id TEXT,
			scope_snapshot_id TEXT,
			permission_snapshot_id TEXT,
			projected_payload_hash TEXT,
			subscription_generation INTEGER NOT NULL DEFAULT 0,
			target_generation INTEGER NOT NULL DEFAULT 0,
			producer_generation INTEGER NOT NULL DEFAULT 0,
			started_at DATETIME,
			finished_at DATETIME,
			error_code TEXT,
			error_message TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS extension_event_dead_letters (
			dead_letter_id TEXT PRIMARY KEY,
			event_id TEXT NOT NULL,
			delivery_id TEXT NOT NULL,
			subscription_id TEXT NOT NULL,
			extension_id TEXT NOT NULL,
			module_id TEXT NOT NULL,
			event_type_id TEXT NOT NULL,
			event_version INTEGER NOT NULL,
			reason TEXT NOT NULL,
			error_code TEXT,
			error_message TEXT,
			attempts INTEGER NOT NULL DEFAULT 0,
			partition_key TEXT,
			ordering_key TEXT,
			payload_hash TEXT,
			projected_payload_hash TEXT,
			definition_hash TEXT,
			scope_snapshot_id TEXT,
			permission_snapshot_id TEXT,
			runtime_instance_id TEXT,
			trace_id TEXT,
			operation_id TEXT,
			origin_event_json TEXT,
			subscription_snapshot_json TEXT,
			created_at DATETIME NOT NULL,
			replay_count INTEGER NOT NULL DEFAULT 0,
			last_replay_at DATETIME,
			status TEXT NOT NULL DEFAULT 'pending',
			updated_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS extension_event_audit (
			audit_id INTEGER PRIMARY KEY AUTOINCREMENT,
			operation_id TEXT,
			invocation_id TEXT,
			event_id TEXT,
			delivery_id TEXT,
			action TEXT NOT NULL,
			actor TEXT,
			extension_id TEXT,
			timestamp DATETIME NOT NULL,
			payload_hash TEXT,
			error_code TEXT,
			success INTEGER NOT NULL DEFAULT 0,
			detail_json TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS extension_event_invocations (
			invocation_id TEXT PRIMARY KEY,
			operation_id TEXT,
			event_id TEXT NOT NULL,
			delivery_id TEXT,
			subscription_id TEXT,
			attempt INTEGER NOT NULL DEFAULT 0,
			runtime_instance_id TEXT,
			scope_snapshot_id TEXT,
			permission_snapshot_id TEXT,
			trace_id TEXT,
			filter_result TEXT,
			projection_result TEXT,
			ordering_result TEXT,
			permission_result TEXT,
			scope_result TEXT,
			status TEXT NOT NULL,
			started_at DATETIME NOT NULL,
			finished_at DATETIME,
			error_code TEXT,
			error_message TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS extension_event_side_effects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			invocation_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			target TEXT NOT NULL,
			hash TEXT,
			occurred_at DATETIME NOT NULL
		)`,
	}
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

func makeTestSubscriptionDef(contributionID, extensionID, eventTypeID string) EventSubscriptionDefinition {
	return EventSubscriptionDefinition{
		ContributionID:    contributionID,
		ExtensionID:       extensionID,
		ModuleID:          "main",
		EventTypeID:       EventTypeID(eventTypeID),
		EventVersionRange: "^1",
		Entry:             "onEvent",
		Enabled:           true,
		Generation:        1,
		Timeout:           5 * time.Second,
		MaxInFlight:       4,
		DeliveryPolicy: SubscriptionDeliveryPolicy{
			Timeout:     5 * time.Second,
			MaxInFlight: 4,
		},
		RetryPolicy: RetryPolicy{
			MaxAttempts:    3,
			InitialBackoff: 100 * time.Millisecond,
			MaxBackoff:     1 * time.Second,
			Multiplier:     2,
		},
	}
}

func testEventTypeDefinition() EventTypeDefinition {
	maxPayload := int64(256 * 1024)
	maxMeta := int64(32 * 1024)
	return EventTypeDefinition{
		EventTypeID:      EventTypeID("system.test"),
		Version:          1,
		Description:      "Test event type for E2E",
		MaxPayloadBytes:  maxPayload,
		MaxMetadataBytes: maxMeta,
		RiskLevel:        RiskLevelLow,
		ProducerPolicy: EventProducerPolicy{
			AllowedProducers:   []string{"host", "system", "test"},
			MaxPayloadBytes:    maxPayload,
			MaxMetadataBytes:   maxMeta,
			RateLimitPerSecond: 100,
		},
		SubscriberPolicy: EventSubscriberPolicy{
			AllowThirdParty:     true,
			MaxSubscribers:      64,
			RequiredPermissions: []string{"event.subscribe"},
		},
		DeliveryPolicy: EventDeliveryPolicy{
			Timeout:           5 * time.Second,
			MaxAttempts:       5,
			InitialBackoff:    100 * time.Millisecond,
			MaxBackoff:        1 * time.Second,
			BackoffMultiplier: 2,
			JitterFactor:      0.2,
			MaxInFlight:       4,
		},
		OrderingPolicy: OrderingNone,
		RetentionPolicy: EventRetentionPolicy{
			MaxAge:                24 * time.Hour,
			MaxDeliveryCount:      5,
			DeleteAfterSuccess:    true,
			DeleteAfterDeadLetter: false,
			ArchiveDeadLetters:    true,
		},
	}
}

func publishTestEvent(t *testing.T, svc *Service, typeID EventTypeID) string {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{"msg": "hello"})
	result, err := svc.Publish(context.Background(), typeID, 1, payload, PublishOptions{
		ProducerID:   "test-producer",
		ProducerType: "host",
	})
	if err != nil {
		t.Fatalf("publish event: %v", err)
	}
	return result.EventID
}

func waitForDeliveryStatus(svc *Service, eventID string, status DeliveryStatus, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		deliveries, err := svc.ListDeliveriesByEvent(context.Background(), eventID)
		if err == nil && len(deliveries) > 0 {
			for _, d := range deliveries {
				if d.Status == status {
					return true
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func waitForDeliveryCount(svc *Service, eventID string, count int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		deliveries, err := svc.ListDeliveriesByEvent(context.Background(), eventID)
		if err == nil && len(deliveries) >= count {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func TestEventE2E_NoPermission_NoDelivery(t *testing.T) {
	svc, _, _, _, cleanup := setupTestService(t, false, true, true, true)
	defer cleanup()

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start service: %v", err)
	}

	def := makeTestSubscriptionDef("contrib-no-perm", "com.test/noperm", "system.test")
	def.PermissionRequirements = []PermissionRequirement{{PermissionID: "test.permission", Scope: permission.PermissionScope{Type: permission.ScopeExtension}}}
	if err := svc.RegisterSubscription(context.Background(), def); err != nil {
		t.Fatalf("register subscription: %v", err)
	}

	eventID := publishTestEvent(t, svc, "system.test")

	if waitForDeliveryCount(svc, eventID, 1, 3*time.Second) {
		deliveries, _ := svc.ListDeliveriesByEvent(context.Background(), eventID)
		for _, d := range deliveries {
			if d.Status == DeliveryStatusSucceeded {
				t.Errorf("expected no successful delivery without permission, got status %s", d.Status)
			}
		}
	}
}

func TestEventE2E_WrongScope_NoDelivery(t *testing.T) {
	svc, _, _, _, cleanup := setupTestService(t, true, false, true, true)
	defer cleanup()

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start service: %v", err)
	}

	def := makeTestSubscriptionDef("contrib-wrong-scope", "com.test/wrongscope", "system.test")
	def.ScopeRule = ScopeRule{RequiredScope: "character", CharacterBinding: true}
	if err := svc.RegisterSubscription(context.Background(), def); err != nil {
		t.Fatalf("register subscription: %v", err)
	}

	eventID := publishTestEvent(t, svc, "system.test")

	if waitForDeliveryCount(svc, eventID, 1, 3*time.Second) {
		deliveries, _ := svc.ListDeliveriesByEvent(context.Background(), eventID)
		for _, d := range deliveries {
			if d.Status == DeliveryStatusSucceeded {
				t.Errorf("expected no successful delivery with wrong scope, got status %s", d.Status)
			}
		}
	}
}

func TestEventE2E_RuntimeNotReady_NoDelivery(t *testing.T) {
	svc, _, _, _, cleanup := setupTestService(t, true, true, true, false)
	defer cleanup()

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start service: %v", err)
	}

	def := makeTestSubscriptionDef("contrib-no-runtime", "com.test/noruntime", "system.test")
	def.RuntimeBinding = RuntimeBinding{Entry: "onEvent", Timeout: 5 * time.Second, MaxInFlight: 4}
	if err := svc.RegisterSubscription(context.Background(), def); err != nil {
		t.Fatalf("register subscription: %v", err)
	}

	eventID := publishTestEvent(t, svc, "system.test")

	if waitForDeliveryCount(svc, eventID, 1, 3*time.Second) {
		deliveries, _ := svc.ListDeliveriesByEvent(context.Background(), eventID)
		for _, d := range deliveries {
			if d.Status == DeliveryStatusSucceeded {
				t.Errorf("expected no successful delivery with runtime not ready, got status %s", d.Status)
			}
		}
	}
}

func TestEventE2E_AllConditionsMet_DeliverySucceeds(t *testing.T) {
	svc, _, _, handler, cleanup := setupTestService(t, true, true, true, true)
	defer cleanup()
	handler.reset(0, "")

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start service: %v", err)
	}

	def := makeTestSubscriptionDef("contrib-success", "com.test/success", "system.test")
	if err := svc.RegisterSubscription(context.Background(), def); err != nil {
		t.Fatalf("register subscription: %v", err)
	}

	eventID := publishTestEvent(t, svc, "system.test")

	if !waitForDeliveryStatus(svc, eventID, DeliveryStatusSucceeded, 5*time.Second) {
		deliveries, _ := svc.ListDeliveriesByEvent(context.Background(), eventID)
		for _, d := range deliveries {
			t.Logf("delivery %s status=%s code=%s msg=%s", d.DeliveryID, d.Status, d.ErrorCode, d.ErrorMessage)
		}
		t.Fatalf("expected delivery to succeed")
	}

	if handler.count() == 0 {
		t.Errorf("expected handler to be called at least once")
	}
}

func TestEventE2E_Retry(t *testing.T) {
	svc, _, _, handler, cleanup := setupTestService(t, true, true, true, true)
	defer cleanup()
	handler.reset(2, "transient_error")

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start service: %v", err)
	}

	def := makeTestSubscriptionDef("contrib-retry", "com.test/retry", "system.test")
	def.RetryPolicy = RetryPolicy{
		MaxAttempts:    5,
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     200 * time.Millisecond,
		Multiplier:     2,
	}
	if err := svc.RegisterSubscription(context.Background(), def); err != nil {
		t.Fatalf("register subscription: %v", err)
	}

	eventID := publishTestEvent(t, svc, "system.test")

	if !waitForDeliveryStatus(svc, eventID, DeliveryStatusSucceeded, 10*time.Second) {
		deliveries, _ := svc.ListDeliveriesByEvent(context.Background(), eventID)
		for _, d := range deliveries {
			t.Logf("delivery %s status=%s attempt=%d code=%s", d.DeliveryID, d.Status, d.Attempt, d.ErrorCode)
		}
		t.Fatalf("expected delivery to eventually succeed after retry")
	}

	if handler.count() < 3 {
		t.Errorf("expected at least 3 handler calls (2 fail + 1 success), got %d", handler.count())
	}
}

func TestEventE2E_DeadLetter(t *testing.T) {
	svc, _, _, handler, cleanup := setupTestService(t, true, true, true, true)
	defer cleanup()
	handler.reset(999, "permanent_error")

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start service: %v", err)
	}

	def := makeTestSubscriptionDef("contrib-deadletter", "com.test/deadletter", "system.test")
	def.RetryPolicy = RetryPolicy{
		MaxAttempts:    2,
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		Multiplier:     1,
	}
	if err := svc.RegisterSubscription(context.Background(), def); err != nil {
		t.Fatalf("register subscription: %v", err)
	}

	eventID := publishTestEvent(t, svc, "system.test")

	if !waitForDeliveryStatus(svc, eventID, DeliveryStatusDeadLetter, 10*time.Second) {
		deliveries, _ := svc.ListDeliveriesByEvent(context.Background(), eventID)
		for _, d := range deliveries {
			t.Logf("delivery %s status=%s attempt=%d", d.DeliveryID, d.Status, d.Attempt)
		}
		t.Fatalf("expected delivery to end up in dead letter")
	}

	deadLetters, err := svc.ListDeadLetters(context.Background(), DeadLetterFilter{}, 10, 0)
	if err != nil {
		t.Fatalf("list dead letters: %v", err)
	}
	if len(deadLetters) == 0 {
		t.Errorf("expected at least 1 dead letter record")
	}
}

func TestEventE2E_GenerationUpdate_OldDeliveryCancelled(t *testing.T) {
	svc, _, _, _, cleanup := setupTestService(t, true, true, true, true)
	defer cleanup()

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start service: %v", err)
	}

	def := makeTestSubscriptionDef("contrib-gen-fence", "com.test/genfence", "system.test")
	def.Generation = 1
	if err := svc.RegisterSubscription(context.Background(), def); err != nil {
		t.Fatalf("register subscription: %v", err)
	}

	delivery := Delivery{
		DeliveryID:             newDeliveryID(),
		EventID:                "evt-old-gen",
		SubscriptionID:         "contrib-gen-fence",
		ExtensionID:            "com.test/genfence",
		ModuleID:               "main",
		Status:                 DeliveryStatusPending,
		MaxAttempts:            3,
		AvailableAt:            time.Now().UTC(),
		SubscriptionGeneration: 1,
		TargetGeneration:       1,
		CreatedAt:              time.Now().UTC(),
		UpdatedAt:              time.Now().UTC(),
	}
	if err := svc.deliveryStore.CreateDelivery(context.Background(), delivery); err != nil {
		t.Fatalf("create delivery: %v", err)
	}

	if err := svc.UpdateExtensionGeneration(context.Background(), "com.test/genfence", 2, []EventSubscriptionDefinition{def}); err != nil {
		t.Fatalf("update generation: %v", err)
	}

	updated, err := svc.GetDelivery(context.Background(), delivery.DeliveryID)
	if err != nil {
		t.Fatalf("get delivery: %v", err)
	}
	if updated.Status != DeliveryStatusCancelled {
		t.Errorf("expected cancel_stale_subscription, got status %s (code=%s)", updated.Status, updated.ErrorCode)
	}
	if updated.ErrorCode != "cancel_stale_subscription" {
		t.Errorf("expected error code cancel_stale_subscription, got %s", updated.ErrorCode)
	}
}

func TestEventE2E_Restart_SubscriptionRecovered(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("event_restart_%d.db", time.Now().UnixNano()))
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := migrateTestDB(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	permChecker := &mockPermissionChecker{granted: true}
	scopeChecker := &mockScopeChecker{valid: true}
	depChecker := &mockDependencyChecker{ready: true}
	runtimeChecker := &mockRuntimeChecker{available: true}
	resolver := NewDefaultEffectiveResolver(permChecker, scopeChecker, depChecker, runtimeChecker, &mockCircuitLookup{}, nil)

	cfg := DefaultServiceConfig().WithDB(db)
	cfg.Dispatcher.PollInterval = 50 * time.Millisecond
	svc1, err := NewService(cfg)
	if err != nil {
		t.Fatalf("create service 1: %v", err)
	}
	if err := svc1.SetEffectiveResolver(resolver); err != nil {
		t.Fatalf("set effective resolver: %v", err)
	}
	handler := &mockDeliveryHandler{}
	svc1.SetDeliveryHandler(handler.handle)
	if err := svc1.RegisterDefaultEventTypes(context.Background()); err != nil {
		t.Fatalf("register event types: %v", err)
	}
	if err := svc1.RegisterEventType(context.Background(), testEventTypeDefinition()); err != nil {
		t.Fatalf("register test event type: %v", err)
	}

	def := makeTestSubscriptionDef("contrib-restart", "com.test/restart", "system.test")
	if err := svc1.RegisterSubscription(context.Background(), def); err != nil {
		t.Fatalf("register subscription: %v", err)
	}
	if err := svc1.Start(context.Background()); err != nil {
		t.Fatalf("start service 1: %v", err)
	}
	svc1.Stop()

	svc2, err := NewService(cfg)
	if err != nil {
		t.Fatalf("create service 2: %v", err)
	}
	if err := svc2.SetEffectiveResolver(resolver); err != nil {
		t.Fatalf("set effective resolver svc2: %v", err)
	}
	svc2.SetDeliveryHandler(handler.handle)
	if err := svc2.RegisterDefaultEventTypes(context.Background()); err != nil {
		t.Fatalf("register event types svc2: %v", err)
	}
	if err := svc2.RegisterEventType(context.Background(), testEventTypeDefinition()); err != nil {
		t.Fatalf("register test event type svc2: %v", err)
	}
	if err := svc2.Start(context.Background()); err != nil {
		t.Fatalf("start service 2: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	sub, ok := svc2.GetSubscription(context.Background(), "contrib-restart")
	if !ok {
		t.Fatalf("expected subscription to be recovered after restart")
	}
	if sub.Definition.ExtensionID != "com.test/restart" {
		t.Errorf("expected extension id com.test/restart, got %s", sub.Definition.ExtensionID)
	}

	svc2.Stop()
	_ = db.Close()
	_ = os.Remove(dbPath)
}

func TestEventE2E_Uninstall_CleansSubscriptionsAndDeliveries(t *testing.T) {
	svc, _, _, _, cleanup := setupTestService(t, true, true, true, true)
	defer cleanup()

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start service: %v", err)
	}

	def := makeTestSubscriptionDef("contrib-uninstall", "com.test/uninstall", "system.test")
	if err := svc.RegisterSubscription(context.Background(), def); err != nil {
		t.Fatalf("register subscription: %v", err)
	}

	eventID := publishTestEvent(t, svc, "system.test")
	time.Sleep(500 * time.Millisecond)

	if err := svc.RemoveSubscriptionsByExtension(context.Background(), "com.test/uninstall"); err != nil {
		t.Fatalf("remove subscriptions: %v", err)
	}

	_, ok := svc.GetSubscription(context.Background(), "contrib-uninstall")
	if ok {
		t.Errorf("expected subscription to be removed after uninstall")
	}

	n, err := svc.CancelDeliveriesByExtension(context.Background(), "com.test/uninstall", "extension_uninstalled")
	if err != nil {
		t.Fatalf("cancel deliveries: %v", err)
	}
	_ = n

	deliveries, err := svc.ListDeliveriesByEvent(context.Background(), eventID)
	if err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	for _, d := range deliveries {
		if d.Status == DeliveryStatusPending || d.Status == DeliveryStatusLeased {
			t.Errorf("expected no pending deliveries after cleanup, got %s", d.Status)
		}
	}
}

func TestEventE2E_Replay(t *testing.T) {
	svc, _, _, handler, cleanup := setupTestService(t, true, true, true, true)
	defer cleanup()
	handler.reset(999, "permanent_error")

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start service: %v", err)
	}

	def := makeTestSubscriptionDef("contrib-replay", "com.test/replay", "system.test")
	def.RetryPolicy = RetryPolicy{
		MaxAttempts:    1,
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		Multiplier:     1,
	}
	if err := svc.RegisterSubscription(context.Background(), def); err != nil {
		t.Fatalf("register subscription: %v", err)
	}

	eventID := publishTestEvent(t, svc, "system.test")

	if !waitForDeliveryStatus(svc, eventID, DeliveryStatusDeadLetter, 10*time.Second) {
		t.Fatalf("expected delivery to end up in dead letter")
	}

	deadLetters, err := svc.ListDeadLetters(context.Background(), DeadLetterFilter{}, 10, 0)
	if err != nil || len(deadLetters) == 0 {
		t.Fatalf("expected dead letter records")
	}

	handler.reset(0, "")

	err = svc.ReplayDeadLetter(context.Background(), ReplayRequest{
		DeadLetterID: deadLetters[0].DeadLetterID,
		Strategy:     ReplaySameSubscription,
	})
	if err != nil {
		t.Fatalf("replay dead letter: %v", err)
	}

	if !waitForDeliveryStatus(svc, eventID, DeliveryStatusSucceeded, 5*time.Second) {
		deliveries, _ := svc.ListDeliveriesByEvent(context.Background(), eventID)
		hasSucceeded := false
		for _, d := range deliveries {
			if d.Status == DeliveryStatusSucceeded {
				hasSucceeded = true
			}
		}
		if !hasSucceeded {
			t.Errorf("expected replayed delivery to succeed")
		}
	}
}

func TestEventE2E_SubscriptionPersisted(t *testing.T) {
	svc, _, _, _, cleanup := setupTestService(t, true, true, true, true)
	defer cleanup()

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start service: %v", err)
	}

	def := makeTestSubscriptionDef("contrib-persist", "com.test/persist", "system.test")
	if err := svc.RegisterSubscription(context.Background(), def); err != nil {
		t.Fatalf("register subscription: %v", err)
	}

	repoDef, err := svc.subscriptionRepo.Get(context.Background(), "contrib-persist")
	if err != nil {
		t.Fatalf("get from repository: %v", err)
	}
	if repoDef.ExtensionID != "com.test/persist" {
		t.Errorf("expected extension id com.test/persist in repository, got %s", repoDef.ExtensionID)
	}
	if repoDef.Generation != 1 {
		t.Errorf("expected generation 1, got %d", repoDef.Generation)
	}
}

func TestEventE2E_UpdateGenerationAtomic(t *testing.T) {
	svc, _, _, _, cleanup := setupTestService(t, true, true, true, true)
	defer cleanup()

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start service: %v", err)
	}

	def1 := makeTestSubscriptionDef("contrib-atomic-1", "com.test/atomic", "system.test")
	if err := svc.RegisterSubscription(context.Background(), def1); err != nil {
		t.Fatalf("register subscription 1: %v", err)
	}

	badDef := makeTestSubscriptionDef("", "com.test/atomic", "system.test")
	err := svc.UpdateExtensionGeneration(context.Background(), "com.test/atomic", 2, []EventSubscriptionDefinition{badDef})
	if err == nil {
		t.Errorf("expected error for invalid subscription in generation update")
	}

	sub, ok := svc.GetSubscription(context.Background(), "contrib-atomic-1")
	if !ok {
		t.Fatalf("expected old subscription to still exist after failed generation update")
	}
	if sub.Definition.Generation != 1 {
		t.Errorf("expected old generation preserved (1), got %d", sub.Definition.Generation)
	}
}

func TestProjection_SensitiveFields(t *testing.T) {
	def := DefaultHostEventTypes()[0]
	projector := NewPayloadProjector(def)
	payload, _ := json.Marshal(map[string]any{
		"messageId":      "msg-1",
		"conversationId": "conv-1",
		"text":           "hello secret",
		"attachments":    []string{"a.png"},
		"context":        "sensitive context",
		"systemPrompt":   "top secret prompt",
	})

	result, err := projector.Project(payload, EventProjectionRequest{}, map[string]bool{"message.read": true})
	if err != nil {
		t.Fatalf("projection failed: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(result.Payload, &parsed); err != nil {
		t.Fatalf("unmarshal projected: %v", err)
	}
	if _, ok := parsed["messageId"]; !ok {
		t.Errorf("expected normal field messageId to be present")
	}
	if _, ok := parsed["text"]; !ok {
		t.Errorf("expected sensitive field with permission (text) to be present")
	}
	if _, ok := parsed["context"]; ok {
		t.Errorf("expected sensitive omit field (context) to be removed")
	}
	if _, ok := parsed["systemPrompt"]; ok {
		t.Errorf("expected sensitive omit field (systemPrompt) to be removed")
	}
}

func TestProjection_SensitiveFields_NoPermission(t *testing.T) {
	def := DefaultHostEventTypes()[0]
	projector := NewPayloadProjector(def)
	payload, _ := json.Marshal(map[string]any{
		"messageId": "msg-1",
		"text":      "hello secret",
	})

	result, err := projector.Project(payload, EventProjectionRequest{}, map[string]bool{})
	if err != nil {
		t.Fatalf("projection failed: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(result.Payload, &parsed); err != nil {
		t.Fatalf("unmarshal projected: %v", err)
	}
	if _, ok := parsed["messageId"]; !ok {
		t.Errorf("expected normal field messageId to be present")
	}
	textVal, ok := parsed["text"]
	if !ok {
		t.Errorf("expected sensitive field text to be present (masked)")
	} else {
		s := fmt.Sprintf("%v", textVal)
		if s == "hello secret" {
			t.Errorf("expected sensitive field text to be masked without permission, got %q", s)
		}
	}
}

func TestPublish_SchemaValidation_Rejected(t *testing.T) {
	svc, _, _, _, cleanup := setupTestService(t, true, true, true, true)
	defer cleanup()
	schemaDef := EventTypeDefinition{
		EventTypeID:      EventTypeID("system.schema.test"),
		Version:          1,
		Description:      "Test with schema",
		PayloadSchema:    json.RawMessage(`{"required":["id","name"],"properties":{"id":{"type":"string"},"name":{"type":"string"}}}`),
		MaxPayloadBytes:  1024,
		MaxMetadataBytes: 256,
		ProducerPolicy:   EventProducerPolicy{AllowedProducers: []string{"host"}},
	}
	if err := svc.RegisterEventType(context.Background(), schemaDef); err != nil {
		t.Fatalf("register schema event type: %v", err)
	}

	missingRequiredPayload, _ := json.Marshal(map[string]any{"id": "only-id"})
	_, err := svc.Publish(context.Background(), "system.schema.test", 1, missingRequiredPayload, PublishOptions{
		ProducerID:   "host",
		ProducerType: "host",
	})
	if err == nil {
		t.Errorf("expected publish to fail when required field 'name' is missing")
	}

	validPayload, _ := json.Marshal(map[string]any{"id": "1", "name": "test"})
	_, err = svc.Publish(context.Background(), "system.schema.test", 1, validPayload, PublishOptions{
		ProducerID:   "host",
		ProducerType: "host",
	})
	if err != nil {
		t.Errorf("expected publish to succeed with valid payload, got %v", err)
	}
}

func TestDeadLetter_OutcomeUnknown(t *testing.T) {
	delivery := Delivery{
		DeliveryID:     "del-unknown",
		EventID:        "evt-unknown",
		SubscriptionID: "sub-unknown",
		Status:         DeliveryStatusDeadLetter,
	}
	env := EventEnvelope{EventID: "evt-unknown"}
	subDef := EventSubscriptionDefinition{ContributionID: "sub-unknown"}
	rec := NewDeadLetterRecord(delivery, env, subDef, DeadLetterOutcomeUnknown)
	if rec.Reason != DeadLetterOutcomeUnknown {
		t.Errorf("expected reason outcome_unknown, got %q", rec.Reason)
	}
	if !IsPermanent(ErrDeadLetterOutcomeUnknown) {
		t.Errorf("expected ErrDeadLetterOutcomeUnknown to be permanent")
	}
	if ErrorCode(ErrDeadLetterOutcomeUnknown) != "outcome_unknown" {
		t.Errorf("expected error code outcome_unknown, got %q", ErrorCode(ErrDeadLetterOutcomeUnknown))
	}
}
