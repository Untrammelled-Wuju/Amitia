package event

import (
	"encoding/json"
	"errors"
	"sort"
	"testing"
	"time"
)

func TestEventTypeID_ReservedNamespace(t *testing.T) {
	reserved := []EventTypeID{
		"system.foo",
		"security.bar",
		"permission.scope.changed",
		"scope.granted",
		"secret.rotated",
	}
	for _, id := range reserved {
		if !id.IsReservedNamespace() {
			t.Errorf("expected %q to be reserved namespace", id)
		}
		if !id.IsHostNamespace() {
			t.Errorf("expected reserved %q to be host namespace", id)
		}
	}

	nonReserved := []EventTypeID{
		"extension.abc.created",
		"user.login",
		"custom.event",
		"foo",
		"",
	}
	for _, id := range nonReserved {
		if id.IsReservedNamespace() {
			t.Errorf("expected %q not to be reserved namespace", id)
		}
	}
}

func TestEventTypeID_ExtensionNamespace(t *testing.T) {
	id := EventTypeID("extension.abc.user.created")
	if !id.IsExtensionNamespace("abc") {
		t.Errorf("expected %q to be extension namespace of abc", id)
	}
	if id.IsExtensionNamespace("xyz") {
		t.Errorf("expected %q not to be extension namespace of xyz", id)
	}
	if id.IsHostNamespace() {
		t.Errorf("expected extension namespace not to be host namespace")
	}

	prefixCollision := EventTypeID("extension.abcd.user.created")
	if prefixCollision.IsExtensionNamespace("abc") {
		t.Errorf("expected %q not to match abc namespace (prefix collision)", prefixCollision)
	}

	host := EventTypeID("system.foo")
	if host.IsExtensionNamespace("abc") {
		t.Errorf("expected system namespace not to be extension namespace")
	}
	if !host.IsHostNamespace() {
		t.Errorf("expected system namespace to be host namespace")
	}

	edge := EventTypeID("extension.abc.")
	if !edge.IsExtensionNamespace("abc") {
		t.Errorf("expected extension.abc. to match abc namespace")
	}
}

func TestEventTypeDefinition_Validate(t *testing.T) {
	valid := EventTypeDefinition{
		EventTypeID:      "system.test",
		Version:          1,
		MaxPayloadBytes:  1024,
		MaxMetadataBytes: 256,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid definition, got %v", err)
	}

	cases := []struct {
		name    string
		modify  func(d *EventTypeDefinition)
		wantErr bool
	}{
		{"empty type id", func(d *EventTypeDefinition) { d.EventTypeID = "" }, true},
		{"zero version", func(d *EventTypeDefinition) { d.Version = 0 }, true},
		{"negative version", func(d *EventTypeDefinition) { d.Version = -1 }, true},
		{"zero max payload", func(d *EventTypeDefinition) { d.MaxPayloadBytes = 0 }, true},
		{"zero max metadata", func(d *EventTypeDefinition) { d.MaxMetadataBytes = 0 }, true},
		{"higher version", func(d *EventTypeDefinition) { d.Version = 2 }, false},
		{"larger payload", func(d *EventTypeDefinition) { d.MaxPayloadBytes = 4096 }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := valid
			c.modify(&d)
			err := d.Validate()
			if c.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestEventTypeDefinition_Hash(t *testing.T) {
	base := EventTypeDefinition{
		EventTypeID:      "extension.abc.created",
		Version:          1,
		MaxPayloadBytes:  1024,
		MaxMetadataBytes: 256,
		RiskLevel:        RiskLevelLow,
	}
	h1 := base.Hash()
	h2 := base.Hash()
	if h1 != h2 {
		t.Errorf("expected same hash for identical definition")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char sha256 hex, got %d", len(h1))
	}

	modified := base
	modified.Version = 2
	if modified.Hash() == h1 {
		t.Errorf("expected different hash after version change")
	}

	modified2 := base
	modified2.RiskLevel = RiskLevelHigh
	if modified2.Hash() == h1 {
		t.Errorf("expected different hash after risk level change")
	}

	modified3 := base
	modified3.MaxPayloadBytes = 2048
	if modified3.Hash() == h1 {
		t.Errorf("expected different hash after payload bytes change")
	}

	modified4 := base
	modified4.ProducerPolicy = EventProducerPolicy{MaxPayloadBytes: 512}
	if modified4.Hash() == h1 {
		t.Errorf("expected different hash after producer policy change")
	}

	empty := EventTypeDefinition{}
	if empty.Hash() == "" {
		t.Errorf("expected non-empty hash for empty definition")
	}
}

func TestEventEnvelope_Validate(t *testing.T) {
	def := EventTypeDefinition{
		EventTypeID:      "system.test",
		Version:          1,
		MaxPayloadBytes:  1024,
		MaxMetadataBytes: 256,
	}
	payload := json.RawMessage(`{"hello":"world"}`)
	env := NewEventEnvelope(def.EventTypeID, 1, "producer-1", "host", payload)
	if err := env.Validate(def, 8); err != nil {
		t.Fatalf("expected valid envelope, got %v", err)
	}
	if env.EventID == "" {
		t.Fatalf("expected non-empty event id")
	}
	if env.PayloadHash == "" {
		t.Fatalf("expected non-empty payload hash")
	}
	if env.IdempotencyKey == "" {
		t.Fatalf("expected non-empty idempotency key")
	}

	t.Run("type mismatch", func(t *testing.T) {
		e := env
		e.EventTypeID = "system.other"
		if err := e.Validate(def, 8); err == nil {
			t.Errorf("expected type mismatch error")
		}
	})

	t.Run("empty event id", func(t *testing.T) {
		e := env
		e.EventID = ""
		if err := e.Validate(def, 8); err == nil {
			t.Errorf("expected empty event id error")
		}
	})

	t.Run("empty event type id", func(t *testing.T) {
		e := env
		e.EventTypeID = ""
		if err := e.Validate(def, 8); err == nil {
			t.Errorf("expected empty event type id error")
		}
	})

	t.Run("zero version", func(t *testing.T) {
		e := env
		e.EventVersion = 0
		if err := e.Validate(def, 8); err == nil {
			t.Errorf("expected zero version error")
		}
	})

	t.Run("empty producer", func(t *testing.T) {
		e := env
		e.ProducerID = ""
		if err := e.Validate(def, 8); err == nil {
			t.Errorf("expected empty producer error")
		}
	})

	t.Run("payload too large", func(t *testing.T) {
		e := env
		e.Payload = make(json.RawMessage, def.MaxPayloadBytes+1)
		if err := e.Validate(def, 8); err == nil {
			t.Errorf("expected payload too large error")
		}
	})

	t.Run("metadata too large", func(t *testing.T) {
		e := env
		e.Metadata = make(json.RawMessage, def.MaxMetadataBytes+1)
		if err := e.Validate(def, 8); err == nil {
			t.Errorf("expected metadata too large error")
		}
	})

	t.Run("depth exceeded", func(t *testing.T) {
		e := env
		e.Depth = 9
		if err := e.Validate(def, 8); err == nil {
			t.Errorf("expected depth exceeded error")
		}
	})

	t.Run("empty payload hash", func(t *testing.T) {
		e := env
		e.PayloadHash = ""
		if err := e.Validate(def, 8); err == nil {
			t.Errorf("expected empty payload hash error")
		}
	})

	t.Run("aggregate builder", func(t *testing.T) {
		v := int64(3)
		e := env.WithAggregate("order", "ord-1", &v)
		if err := e.Validate(def, 8); err != nil {
			t.Errorf("expected valid aggregate envelope, got %v", err)
		}
		if e.AggregateVersionOrZero() != 3 {
			t.Errorf("expected aggregate version 3, got %d", e.AggregateVersionOrZero())
		}
		if e.PartitionKey != "order:ord-1" {
			t.Errorf("expected partition key order:ord-1, got %s", e.PartitionKey)
		}
	})
}

func TestCompiledFilter_AllowedFieldsList(t *testing.T) {
	allowed := []string{"type", "source", "priority"}
	def := EventFilterDefinition{
		Root: FilterNode{
			Operator: FilterOpEquals,
			Field:    "type",
			Value:    json.RawMessage(`"order"`),
		},
	}
	cf, err := CompileFilter(def, allowed)
	if err != nil {
		t.Fatalf("expected compile success, got %v", err)
	}

	got := cf.AllowedFieldsList()
	sort.Strings(got)
	expected := append([]string(nil), allowed...)
	sort.Strings(expected)
	if len(got) != len(expected) {
		t.Fatalf("expected %d fields, got %d", len(expected), len(got))
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Errorf("expected %q, got %q", expected[i], got[i])
		}
	}

	if cf.Match(map[string]any{"type": "order"}) != true {
		t.Errorf("expected match for type=order")
	}
	if cf.Match(map[string]any{"type": "refund"}) != false {
		t.Errorf("expected no match for type=refund")
	}
	if cf.Match(map[string]any{}) != false {
		t.Errorf("expected no match for empty fields")
	}

	if _, err := CompileFilter(def, nil); err == nil {
		t.Errorf("expected error for empty allowed fields")
	}

	invalidDef := EventFilterDefinition{
		Root: FilterNode{
			Operator: FilterOpEquals,
			Field:    "forbidden",
			Value:    json.RawMessage(`"x"`),
		},
	}
	if _, err := CompileFilter(invalidDef, allowed); err == nil {
		t.Errorf("expected error for disallowed field")
	}

	andDef := EventFilterDefinition{
		Root: FilterNode{
			Operator: FilterOpAnd,
			Children: []FilterNode{
				{Operator: FilterOpEquals, Field: "type", Value: json.RawMessage(`"order"`)},
				{Operator: FilterOpNumericGT, Field: "priority", Value: json.RawMessage(`5`)},
			},
		},
	}
	cf2, err := CompileFilter(andDef, allowed)
	if err != nil {
		t.Fatalf("expected and compile success, got %v", err)
	}
	if !cf2.Match(map[string]any{"type": "order", "priority": float64(10)}) {
		t.Errorf("expected and match")
	}
	if cf2.Match(map[string]any{"type": "order", "priority": float64(1)}) {
		t.Errorf("expected no and match for low priority")
	}
}

func TestRetryPolicy_NextBackoff(t *testing.T) {
	policy := RetryPolicy{
		MaxAttempts:    5,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     1000 * time.Millisecond,
		Multiplier:     2.0,
		Jitter:         0,
	}

	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{-1, 100 * time.Millisecond},
		{0, 100 * time.Millisecond},
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 800 * time.Millisecond},
		{5, 1000 * time.Millisecond},
		{10, 1000 * time.Millisecond},
	}
	for _, c := range cases {
		got := policy.ComputeBackoff(c.attempt)
		if got != c.want {
			t.Errorf("attempt %d: expected %v, got %v", c.attempt, c.want, got)
		}
	}

	noCap := RetryPolicy{
		MaxAttempts:    5,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     1 * time.Hour,
		Multiplier:     2.0,
		Jitter:         0,
	}
	if got := noCap.ComputeBackoff(5); got != 160*time.Millisecond {
		t.Errorf("expected 160ms without cap, got %v", got)
	}

	defaultPolicy := DefaultRetryPolicy()
	if !defaultPolicy.ShouldRetry(1, "timeout") {
		t.Errorf("expected retryable for timeout")
	}
	if defaultPolicy.ShouldRetry(defaultPolicy.MaxAttempts, "timeout") {
		t.Errorf("expected no retry at max attempts")
	}
	if defaultPolicy.ShouldRetry(1, "permission_denied") {
		t.Errorf("expected no retry for non-retryable permission_denied")
	}
	if defaultPolicy.ShouldRetry(1, "event_loop_detected") {
		t.Errorf("expected no retry for non-retryable event_loop_detected")
	}
	if !defaultPolicy.IsRetryable("runtime_unavailable") {
		t.Errorf("expected runtime_unavailable retryable")
	}

	sched := NewBackoffSchedule(policy)
	if sched.MaxAttempts() != 5 {
		t.Errorf("expected max attempts 5, got %d", sched.MaxAttempts())
	}
	if sched.Next(1) != 100*time.Millisecond {
		t.Errorf("expected next backoff 100ms, got %v", sched.Next(1))
	}
	if sched.Next(3) != 400*time.Millisecond {
		t.Errorf("expected next backoff 400ms, got %v", sched.Next(3))
	}

	merged := policy.MergeWith(RetryPolicy{MaxAttempts: 3})
	if merged.MaxAttempts != 3 {
		t.Errorf("expected merged max attempts 3, got %d", merged.MaxAttempts)
	}
	merged2 := policy.MergeWith(RetryPolicy{MaxAttempts: 100})
	if merged2.MaxAttempts != policy.MaxAttempts {
		t.Errorf("expected merged max attempts capped to %d, got %d", policy.MaxAttempts, merged2.MaxAttempts)
	}
}

func TestLoopGuard_Check(t *testing.T) {
	guard := NewLoopGuard(4, 10)
	if guard.MaxDepth() != 4 {
		t.Errorf("expected max depth 4, got %d", guard.MaxDepth())
	}

	if err := guard.Enter("chain-1", "system.test", "contrib-1", 2, "trace-1"); err != nil {
		t.Fatalf("expected enter success, got %v", err)
	}
	if guard.ChainDepth("chain-1") != 2 {
		t.Errorf("expected depth 2, got %d", guard.ChainDepth("chain-1"))
	}

	if err := guard.Enter("chain-1", "system.test", "contrib-1", 5, "trace-1"); !errors.Is(err, ErrEventDepthExceeded) {
		t.Errorf("expected ErrEventDepthExceeded, got %v", err)
	}
	if guard.ChainDepth("chain-1") != 2 {
		t.Errorf("expected depth unchanged after rejected enter, got %d", guard.ChainDepth("chain-1"))
	}

	guard.Exit("chain-1")
	if guard.ChainDepth("chain-1") != 0 {
		t.Errorf("expected depth 0 after exit, got %d", guard.ChainDepth("chain-1"))
	}

	seen := make(map[string]bool)
	if guard.CheckDuplicate("chain-1", "idem-1", seen) {
		t.Errorf("expected first check not duplicate")
	}
	if !guard.CheckDuplicate("chain-1", "idem-1", seen) {
		t.Errorf("expected second check duplicate")
	}
	if guard.CheckDuplicate("chain-2", "idem-1", seen) {
		t.Errorf("expected different chain not duplicate")
	}
	if guard.CheckDuplicate("chain-1", "", seen) {
		t.Errorf("expected empty idempotency not duplicate")
	}

	guard2 := NewLoopGuard(4, 10)
	chainKey := "chain-loop"
	for i := 0; i < guard2.maxPerWindow; i++ {
		if err := guard2.Enter(chainKey, "system.repeat", "contrib-repeat", 1, "trace-2"); err != nil {
			t.Fatalf("unexpected error on enter %d: %v", i, err)
		}
	}
	if err := guard2.Enter(chainKey, "system.repeat", "contrib-repeat", 1, "trace-2"); !errors.Is(err, ErrEventLoopDetected) {
		t.Errorf("expected ErrEventLoopDetected, got %v", err)
	}

	guard2.Reset(chainKey)
	if err := guard2.Enter(chainKey, "system.repeat", "contrib-repeat", 1, "trace-2"); err != nil {
		t.Errorf("expected enter success after reset, got %v", err)
	}

	defaultGuard := NewLoopGuard(0, 0)
	if defaultGuard.MaxDepth() != 8 {
		t.Errorf("expected default max depth 8, got %d", defaultGuard.MaxDepth())
	}

	other := NewLoopGuard(2, 0)
	if err := other.Enter("c", "system.x", "", 2, "t"); err != nil {
		t.Fatalf("expected enter at max depth ok, got %v", err)
	}
	if err := other.Enter("c", "system.x", "", 3, "t"); !errors.Is(err, ErrEventDepthExceeded) {
		t.Errorf("expected ErrEventDepthExceeded above max, got %v", err)
	}
}

func TestCircuitBreaker_Transitions(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		OpenTimeout:      50 * time.Millisecond,
		HalfOpenMax:      1,
		TriggerErrors:    []string{"timeout", "runtime_error"},
	}
	cb := NewCircuitBreaker(config)

	if cb.State() != CircuitClosed {
		t.Fatalf("expected initial closed, got %s", cb.State())
	}
	if ok, state := cb.Allow(); !ok || state != CircuitClosed {
		t.Errorf("expected allow in closed, got %v %s", ok, state)
	}

	for i := 0; i < config.FailureThreshold; i++ {
		cb.RecordFailure("timeout")
	}
	if cb.State() != CircuitOpen {
		t.Errorf("expected open after %d failures, got %s", config.FailureThreshold, cb.State())
	}
	if ok, state := cb.Allow(); ok || state != CircuitOpen {
		t.Errorf("expected deny in open, got %v %s", ok, state)
	}

	time.Sleep(60 * time.Millisecond)
	if ok, state := cb.Allow(); !ok || state != CircuitHalfOpen {
		t.Errorf("expected half_open after timeout, got %v %s", ok, state)
	}

	cb.RecordSuccess()
	if cb.State() != CircuitHalfOpen {
		t.Errorf("expected still half_open after 1 success, got %s", cb.State())
	}
	cb.RecordSuccess()
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed after success threshold, got %s", cb.State())
	}

	for i := 0; i < config.FailureThreshold; i++ {
		cb.RecordFailure("runtime_error")
	}
	if cb.State() != CircuitOpen {
		t.Fatalf("expected open again, got %s", cb.State())
	}
	time.Sleep(60 * time.Millisecond)
	if ok, state := cb.Allow(); !ok || state != CircuitHalfOpen {
		t.Fatalf("expected half_open again, got %v %s", ok, state)
	}
	cb.RecordFailure("timeout")
	if cb.State() != CircuitOpen {
		t.Errorf("expected open after half_open failure, got %s", cb.State())
	}

	nonTrigger := NewCircuitBreaker(config)
	nonTrigger.RecordFailure("unknown_error")
	nonTrigger.RecordFailure("unknown_error")
	nonTrigger.RecordFailure("unknown_error")
	if nonTrigger.State() != CircuitClosed {
		t.Errorf("expected closed after non-trigger errors, got %s", nonTrigger.State())
	}

	stats := cb.Stats()
	if stats.State != CircuitOpen {
		t.Errorf("expected stats open, got %s", stats.State)
	}
	if stats.TotalFails <= 0 {
		t.Errorf("expected total fails > 0, got %d", stats.TotalFails)
	}
	if stats.LastFailCode != "timeout" {
		t.Errorf("expected last fail code timeout, got %s", stats.LastFailCode)
	}

	cb.Reset()
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed after reset, got %s", cb.State())
	}

	registry := NewCircuitBreakerRegistry(config)
	cbA := registry.GetOrCreate("subscriber-a")
	cbB := registry.GetOrCreate("subscriber-a")
	if cbA != cbB {
		t.Errorf("expected same breaker for same key")
	}
	cbC := registry.GetOrCreate("subscriber-c")
	if cbA == cbC {
		t.Errorf("expected different breaker for different key")
	}
	if _, ok := registry.Get("subscriber-a"); !ok {
		t.Errorf("expected to find registered breaker")
	}
	registry.Reset("subscriber-a")
	if cbA.State() != CircuitClosed {
		t.Errorf("expected registry reset to closed, got %s", cbA.State())
	}
	all := registry.All()
	if len(all) != 2 {
		t.Errorf("expected 2 breakers in registry, got %d", len(all))
	}
}
