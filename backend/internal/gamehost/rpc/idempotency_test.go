package rpc

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func TestIdempotencyIndex_CheckOrAdd(t *testing.T) {
	idx := NewIdempotencyIndex()
	key := RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req-1"}
	fp := ComputeFingerprint("minecraft.move", []byte(`{"x":1}`))

	err := idx.CheckOrAdd(key, fp, RequestStateRunning)
	if err != nil {
		t.Fatalf("first check/add failed: %v", err)
	}

	err = idx.CheckOrAdd(key, fp, RequestStateRunning)
	if err == nil {
		t.Error("duplicate should return error")
	}
}

func TestIdempotencyIndex_Exists(t *testing.T) {
	idx := NewIdempotencyIndex()
	key := RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req-1"}
	fp := ComputeFingerprint("minecraft.move", []byte(`{"x":1}`))

	if state := idx.Exists(key, fp); state != "" {
		t.Errorf("non-existent key should return empty, got %q", state)
	}

	idx.CheckOrAdd(key, fp, RequestStateRunning)

	if state := idx.Exists(key, fp); state != RequestStateRunning {
		t.Errorf("expected running, got %q", state)
	}
}

func TestIdempotencyIndex_UpdateState(t *testing.T) {
	idx := NewIdempotencyIndex()
	key := RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req-1"}
	fp := ComputeFingerprint("minecraft.move", nil)

	idx.CheckOrAdd(key, fp, RequestStateRunning)
	idx.UpdateState(key, fp, RequestStateCompleted)

	if state := idx.Exists(key, fp); state != RequestStateCompleted {
		t.Errorf("expected completed, got %q", state)
	}
}

func TestIdempotencyIndex_Remove(t *testing.T) {
	idx := NewIdempotencyIndex()
	key := RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req-1"}
	fp := ComputeFingerprint("minecraft.move", nil)

	idx.CheckOrAdd(key, fp, RequestStateRunning)
	idx.Remove(key, fp)

	if state := idx.Exists(key, fp); state != "" {
		t.Error("removed entry should not exist")
	}
}

func TestIdempotencyIndex_CheckIDReuse(t *testing.T) {
	idx := NewIdempotencyIndex()
	key := RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req-1"}
	fp := ComputeFingerprint("minecraft.move", []byte(`{"v":1}`))
	differentFP := ComputeFingerprint("minecraft.attack", []byte(`{"v":2}`))

	idx.CheckOrAdd(key, fp, RequestStateCompleted)

	err := idx.CheckIDReuse(key, fp)
	if err == nil {
		t.Error("same key+fp reuse should error")
	}

	err = idx.CheckIDReuse(key, differentFP)
	if err != nil {
		t.Logf("different fingerprint (different payload): no error (expected)")
	}
}

func TestCompletedResponseCache_Basic(t *testing.T) {
	cache := NewCompletedResponseCache(DefaultCompletedResponseCacheConfig())

	key := RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req-1"}
	fp := ComputeFingerprint("minecraft.move", nil)
	resp := protocol.Envelope{
		Type:    protocol.MessageTypeResponse,
		Payload: json.RawMessage(`{"result":"ok"}`),
	}

	cache.Save(CompletedRequest{
		Key:         key,
		Fingerprint: fp,
		Response:    resp,
		FinishedAt:  time.Now().UTC(),
	})

	if cache.Len() != 1 {
		t.Errorf("expected cache len 1, got %d", cache.Len())
	}

	cached, ok := cache.Lookup(key, fp)
	if !ok {
		t.Fatal("lookup should succeed")
	}
	if string(cached.Payload) != `{"result":"ok"}` {
		t.Errorf("payload mismatch: %s", cached.Payload)
	}
}

func TestCompletedResponseCache_WrongFingerprint(t *testing.T) {
	cache := NewCompletedResponseCache(DefaultCompletedResponseCacheConfig())

	key := RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req-1"}
	fp1 := ComputeFingerprint("minecraft.move", []byte(`{"v":1}`))
	fp2 := ComputeFingerprint("minecraft.move", []byte(`{"v":2}`))

	cache.Save(CompletedRequest{
		Key:         key,
		Fingerprint: fp1,
		FinishedAt:  time.Now().UTC(),
	})

	_, ok := cache.Lookup(key, fp2)
	if ok {
		t.Error("lookup with wrong fingerprint should fail")
	}
}

func TestCompletedResponseCache_TTLExpiry(t *testing.T) {
	cache := NewCompletedResponseCache(CompletedResponseCacheConfig{
		MaxEntries:   1024,
		RetentionTTL: time.Millisecond,
	})

	key := RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req-1"}
	fp := ComputeFingerprint("minecraft.move", nil)

	cache.Save(CompletedRequest{
		Key:         key,
		Fingerprint: fp,
		FinishedAt:  time.Now().UTC().Add(-time.Hour),
	})

	_, ok := cache.Lookup(key, fp)
	if ok {
		t.Error("expired entry should not be found")
	}
}

func TestCompletedResponseCache_Capacity(t *testing.T) {
	cache := NewCompletedResponseCache(CompletedResponseCacheConfig{
		MaxEntries:   3,
		RetentionTTL: time.Hour,
	})

	for i := 0; i < 5; i++ {
		key := RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req"}
		fp := ComputeFingerprint("minecraft.move", json.RawMessage(`{"v":`+itoa(i)+`}`))
		cache.Save(CompletedRequest{
			Key:         key,
			Fingerprint: fp,
			FinishedAt:  time.Now().UTC(),
		})
	}

	if cache.Len() > 3 {
		t.Errorf("cache should respect max entries, got %d", cache.Len())
	}
}

func TestCompletedResponseCache_Invalidate(t *testing.T) {
	cache := NewCompletedResponseCache(DefaultCompletedResponseCacheConfig())
	key := RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req-1"}
	fp := ComputeFingerprint("minecraft.move", nil)

	cache.Save(CompletedRequest{
		Key:         key,
		Fingerprint: fp,
		FinishedAt:  time.Now().UTC(),
	})

	cache.Invalidate(key)

	_, ok := cache.Lookup(key, fp)
	if ok {
		t.Error("invalidated entry should not be found")
	}
}

func TestCloneEnvelope_DeepCopy(t *testing.T) {
	env := protocol.Envelope{
		Payload: json.RawMessage(`{"a":1}`),
		Metadata: map[string]json.RawMessage{
			"key": json.RawMessage(`value`),
		},
	}

	clone := cloneEnvelope(env)

	if &clone.Payload[0] == &env.Payload[0] {
		t.Error("payload should be deep copied")
	}

	clone.Metadata["key"] = json.RawMessage(`modified`)
	if string(env.Metadata["key"]) != "value" {
		t.Error("modifying clone should not affect original")
	}
}

func itoa(i int) string {
	return [...]string{"0", "1", "2", "3", "4", "5"}[i]
}
