package execution

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestBuildIdempotencyKeySHA_Deterministic(t *testing.T) {
	id := IdempotencyIdentity{
		ToolID:         "tool-A",
		Generation:     1,
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		Source:         capability.InvocationSourceModel,
		CallerKey:      "caller-1",
	}
	a := BuildIdempotencyKeySHA(id)
	b := BuildIdempotencyKeySHA(id)
	if a != b {
		t.Fatalf("key not deterministic: %q != %q", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("expected sha256 hex (64 chars), got %d", len(a))
	}
}

func TestBuildIdempotencyKeySHA_Sensitivity(t *testing.T) {
	base := IdempotencyIdentity{
		ToolID: "tool-A", Generation: 1, UserID: "u", CharacterID: "c", ConversationID: "cv", Source: capability.InvocationSourceModel, CallerKey: "k",
	}
	keyBase := BuildIdempotencyKeySHA(base)

	cases := []nameDiff{
		{"tool", func(i *IdempotencyIdentity) { i.ToolID = "tool-B" }},
		{"generation", func(i *IdempotencyIdentity) { i.Generation = 2 }},
		{"user", func(i *IdempotencyIdentity) { i.UserID = "u2" }},
		{"character", func(i *IdempotencyIdentity) { i.CharacterID = "c2" }},
		{"conversation", func(i *IdempotencyIdentity) { i.ConversationID = "cv2" }},
		{"source", func(i *IdempotencyIdentity) { i.Source = capability.InvocationSourceWorkflow }},
		{"caller", func(i *IdempotencyIdentity) { i.CallerKey = "k2" }},
	}
	for _, c := range cases {
		modified := base
		c.apply(&modified)
		key := BuildIdempotencyKeySHA(modified)
		if key == keyBase {
			t.Fatalf("key not sensitive to %s", c.name)
		}
	}
}

type nameDiff struct {
	name string
	fn   func(*IdempotencyIdentity)
}

func (d nameDiff) apply(i *IdempotencyIdentity) { d.fn(i) }

func TestCanonicalInputHash_SortedKeys(t *testing.T) {
	a := json.RawMessage(`{"b":1,"a":2,"c":{"y":1,"x":2}}`)
	b := json.RawMessage(`{"c":{"x":2,"y":1},"b":1,"a":2}`)
	va, errA := CanonicalInputHash(a)
	vb, errB := CanonicalInputHash(b)
	if errA != nil || errB != nil {
		t.Fatalf("unexpected errors %v %v", errA, errB)
	}
	if va != vb {
		t.Fatalf("canonical mismatch:\n a=%s\n b=%s", va, vb)
	}
}

func TestCanonicalInputHash_DifferentValue(t *testing.T) {
	a, _ := CanonicalInputHash(json.RawMessage(`{"k":"v1"}`))
	b, _ := CanonicalInputHash(json.RawMessage(`{"k":"v2"}`))
	if a == b {
		t.Fatalf("different values produced identical hash")
	}
}

func TestCanonicalInputHash_NumberPreserved(t *testing.T) {
	a, _ := CanonicalInputHash(json.RawMessage(`{"n":1.0}`))
	b, _ := CanonicalInputHash(json.RawMessage(`{"n":1}`))
	if a == b {
		t.Fatalf("canonical must preserve literal number representation, same hash=%s", a)
	}
	c, _ := CanonicalInputHash(json.RawMessage(`{"n":1.5}`))
	d, _ := CanonicalInputHash(json.RawMessage(`{"n":1.5}`))
	if c != d {
		t.Fatalf("same literal must produce same hash, got %s vs %s", c, d)
	}
}

func TestCanonicalInputHash_Empty(t *testing.T) {
	if _, err := CanonicalInputHash(json.RawMessage(``)); err == nil {
		t.Fatalf("expected error for empty input")
	}
}

func TestBuildRequestFingerprintSHA_Deterministic(t *testing.T) {
	in := json.RawMessage(`{"op":"create","target":"x"}`)
	v := capability.ToolVersion{SchemaVersion: 2, Revision: "3"}
	fp1 := BuildRequestFingerprintSHA(in, v, 1)
	fp2 := BuildRequestFingerprintSHA(in, v, 1)
	if fp1 != fp2 {
		t.Fatalf("fingerprint not deterministic: %q != %q", fp1, fp2)
	}
	if len(fp1) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(fp1))
	}
}

func TestBuildRequestFingerprintSHA_Sensitive(t *testing.T) {
	in := json.RawMessage(`{"op":"create"}`)
	base := BuildRequestFingerprintSHA(in, capability.ToolVersion{SchemaVersion: 1, Revision: "1"}, 0)
	in2 := BuildRequestFingerprintSHA(json.RawMessage(`{"op":"update"}`), capability.ToolVersion{SchemaVersion: 1, Revision: "1"}, 0)
	ver2 := BuildRequestFingerprintSHA(in, capability.ToolVersion{SchemaVersion: 2, Revision: "1"}, 0)
	gen2 := BuildRequestFingerprintSHA(in, capability.ToolVersion{SchemaVersion: 1, Revision: "1"}, 1)
	if in2 == base || ver2 == base || gen2 == base {
		t.Fatalf("fingerprint not sensitive to inputs")
	}
}
