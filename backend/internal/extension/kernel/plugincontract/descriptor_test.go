package plugincontract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func descriptorSchemaPayload(index int) []byte {
	properties := make([]string, 0, 8)
	for field := 0; field < 8; field++ {
		properties = append(properties, fmt.Sprintf(`"field%d":{"type":"string","description":"field %d of catalog item %d with a realistically verbose description used to model real provider schemas","minLength":1,"maxLength":512}`, field, field, index))
	}
	return []byte(fmt.Sprintf(`{"type":"object","properties":{%s},"required":["field0","field1"],"additionalProperties":false}`, strings.Join(properties, ",")))
}

func newDescriptor(index int) Descriptor {
	return Descriptor{
		ID:      fmt.Sprintf("builtin/catalog/item-%d", index),
		Summary: fmt.Sprintf("catalog item %d", index),
		Tags:    []string{"btag", "Atag", "btag"},
		Scopes:  []ServiceScope{ServiceScopeGlobal, ServiceScopeConversation},
		Permissions: []DescriptorPermission{
			{Capability: "memory.read", Risk: "low"},
			{Capability: "memory.write", Risk: "medium"},
		},
		Placement: ServicePlacementCore,
	}
}

func TestSchemaStoreIsContentAddressed(t *testing.T) {
	store := NewSchemaStore()
	first, err := store.Put([]byte(`{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"integer"}}}`))
	if err != nil {
		t.Fatalf("put first: %v", err)
	}
	second, err := store.Put([]byte("{\n  \"properties\": {\"b\": {\"type\": \"integer\"}, \"a\": {\"type\": \"string\"}},\n  \"type\": \"object\"\n}"))
	if err != nil {
		t.Fatalf("put second: %v", err)
	}
	if first != second {
		t.Fatalf("same schema with different key order must share a ref: %s vs %s", first, second)
	}
	if store.Count() != 1 {
		t.Fatalf("store must deduplicate, got %d entries", store.Count())
	}
	resolved, err := store.Resolve(first)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	sum := sha256.Sum256(resolved)
	if hex.EncodeToString(sum[:]) != first.Hash {
		t.Fatalf("resolved bytes must hash back to the ref")
	}
	if err := first.Validate(); err != nil {
		t.Fatalf("ref must validate: %v", err)
	}
	parsed, err := ParseSchemaRef(first.String())
	if err != nil {
		t.Fatalf("parse ref: %v", err)
	}
	if parsed != first {
		t.Fatalf("ref round trip mismatch: %v vs %v", parsed, first)
	}
}

func TestSchemaStoreRejectsBadInput(t *testing.T) {
	store := NewSchemaStore()
	if _, err := store.Put([]byte(`{"type":`)); err == nil {
		t.Fatalf("broken schema must be rejected")
	}
	missing := SchemaRef{MediaType: DefaultSchemaMediaType, Hash: strings.Repeat("a", 64)}
	if _, err := store.Resolve(missing); !errors.Is(err, ErrUnknownSchemaRef) {
		t.Fatalf("unknown ref must report ErrUnknownSchemaRef, got %v", err)
	}
	if _, err := store.Resolve(SchemaRef{MediaType: DefaultSchemaMediaType, Hash: "short"}); err == nil {
		t.Fatalf("malformed ref must be rejected")
	}
	if _, err := ParseSchemaRef("no-hash-separator"); err == nil {
		t.Fatalf("ref without # must be rejected")
	}
}

func TestDescriptorValidation(t *testing.T) {
	store := NewSchemaStore()
	ref, err := store.Put(descriptorSchemaPayload(1))
	if err != nil {
		t.Fatalf("put schema: %v", err)
	}
	valid := newDescriptor(1)
	valid.InputRef = &ref
	valid.OutputRef = &ref
	if err := valid.Validate(); err != nil {
		t.Fatalf("baseline descriptor must validate: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*Descriptor)
	}{
		{"uppercase id", func(d *Descriptor) { d.ID = "Builtin/Catalog/Item" }},
		{"two segment id", func(d *Descriptor) { d.ID = "builtin/item" }},
		{"empty summary", func(d *Descriptor) { d.Summary = "  " }},
		{"multiline summary", func(d *Descriptor) { d.Summary = "line1\nline2" }},
		{"unknown placement", func(d *Descriptor) { d.Placement = ServicePlacement("edge") }},
		{"unknown scope", func(d *Descriptor) { d.Scopes = []ServiceScope{"galaxy"} }},
		{"uppercase tag is normalized but empty tag rejects", func(d *Descriptor) { d.Tags = []string{""} }},
		{"tag with slash", func(d *Descriptor) { d.Tags = []string{"a/b"} }},
		{"empty permission capability", func(d *Descriptor) { d.Permissions = []DescriptorPermission{{Risk: "low"}} }},
		{"unknown permission risk", func(d *Descriptor) {
			d.Permissions = []DescriptorPermission{{Capability: "memory.read", Risk: "extreme"}}
		}},
		{"undefined input ref shape", func(d *Descriptor) {
			unknown := SchemaRef{MediaType: DefaultSchemaMediaType, Hash: "not-a-hash"}
			d.InputRef = &unknown
		}},
	}
	for _, item := range cases {
		candidate := valid
		item.mutate(&candidate)
		if err := candidate.Validate(); err == nil {
			t.Fatalf("%s must be rejected", item.name)
		} else if !errors.Is(err, ErrInvalidDescriptor) {
			t.Fatalf("%s: error must wrap ErrInvalidDescriptor, got %v", item.name, err)
		}
	}
}

func TestDescriptorIndexRejectsUnresolvableRefs(t *testing.T) {
	index := NewDescriptorIndex()
	unknown := SchemaRef{MediaType: DefaultSchemaMediaType, Hash: strings.Repeat("b", 64)}
	descriptor := newDescriptor(1)
	descriptor.InputRef = &unknown
	if _, err := index.Add(descriptor); !errors.Is(err, ErrUnknownSchemaRef) {
		t.Fatalf("index must reject refs it cannot resolve, got %v", err)
	}
	if _, err := index.Add(newDescriptor(2)); err != nil {
		t.Fatalf("descriptor without refs must be accepted: %v", err)
	}
}

func TestDescriptorPermissionShapeCarriesNoValues(t *testing.T) {
	typ := reflect.TypeOf(DescriptorPermission{})
	for index := 0; index < typ.NumField(); index++ {
		name := strings.ToLower(typ.Field(index).Name)
		if name == "description" || name == "value" || name == "secret" {
			t.Fatalf("descriptor permission must not expose field %s", typ.Field(index).Name)
		}
	}
	encoded, err := json.Marshal(newDescriptor(1))
	if err != nil {
		t.Fatalf("marshal descriptor: %v", err)
	}
	var decoded struct {
		Permissions []map[string]json.RawMessage `json:"permissions"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal descriptor: %v", err)
	}
	for _, permission := range decoded.Permissions {
		if len(permission) > 2 {
			t.Fatalf("permission must only carry capability/risk, got %d keys", len(permission))
		}
		if _, exists := permission["description"]; exists {
			t.Fatalf("permission must not carry description")
		}
	}
}

func TestDescriptorSnapshotExcludesSchemaAndRestoresByRef(t *testing.T) {
	index := NewDescriptorIndex()
	const total = 1000
	fullSchemaBytes := 0
	originals := make(map[string][]byte, total)
	for i := 0; i < total; i++ {
		payload := descriptorSchemaPayload(i)
		fullSchemaBytes += len(payload)
		ref, err := index.PutSchema(payload)
		if err != nil {
			t.Fatalf("put schema %d: %v", i, err)
		}
		descriptor := newDescriptor(i)
		descriptor.InputRef = &ref
		if _, err := index.Add(descriptor); err != nil {
			t.Fatalf("add descriptor %d: %v", i, err)
		}
		originals[descriptor.ID] = payload
	}
	if index.Len() != total {
		t.Fatalf("expected %d descriptors, got %d", total, index.Len())
	}
	if index.Schemas().Count() != total {
		t.Fatalf("expected %d distinct schemas, got %d", total, index.Schemas().Count())
	}
	snapshot, err := index.MarshalSnapshot()
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if strings.Contains(string(snapshot), `"properties"`) {
		t.Fatalf("snapshot must not embed full schemas")
	}
	if strings.Contains(string(snapshot), `"required"`) {
		t.Fatalf("snapshot must not embed schema keywords")
	}
	if len(snapshot) > 640*1024 {
		t.Fatalf("snapshot of %d descriptors is %d bytes, above the 640 KiB index budget", total, len(snapshot))
	}
	t.Logf("snapshot=%d bytes inline=%d bytes entries=%d schemas=%d", len(snapshot), fullSchemaBytes, index.Len(), index.Schemas().Count())
	if len(snapshot)*2 > fullSchemaBytes {
		t.Fatalf("snapshot (%d bytes) must stay well below the inline-schema footprint (%d bytes)", len(snapshot), fullSchemaBytes)
	}
	for _, descriptor := range index.Snapshot() {
		if descriptor.InputRef == nil {
			t.Fatalf("descriptor %s lost its schema ref", descriptor.ID)
		}
		resolved, err := index.Resolve(*descriptor.InputRef)
		if err != nil {
			t.Fatalf("resolve %s: %v", descriptor.ID, err)
		}
		sum := sha256.Sum256(resolved)
		if hex.EncodeToString(sum[:]) != descriptor.InputRef.Hash {
			t.Fatalf("descriptor %s ref hash mismatch", descriptor.ID)
		}
		canonical, err := CanonicalizeJSONValue(originals[descriptor.ID])
		if err != nil {
			t.Fatalf("canonicalize original %s: %v", descriptor.ID, err)
		}
		if string(canonical) != string(resolved) {
			t.Fatalf("descriptor %s schema not restored exactly", descriptor.ID)
		}
	}
}

func TestDescriptorIndexDeduplicatesAndRejectsDuplicates(t *testing.T) {
	index := NewDescriptorIndex()
	shared, err := index.PutSchema(descriptorSchemaPayload(7))
	if err != nil {
		t.Fatalf("put shared schema: %v", err)
	}
	for i := 0; i < 50; i++ {
		descriptor := newDescriptor(i)
		descriptor.InputRef = &shared
		if _, err := index.Add(descriptor); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
	}
	if index.Schemas().Count() != 1 {
		t.Fatalf("identical schemas must be stored once, got %d", index.Schemas().Count())
	}
	duplicate := newDescriptor(0)
	duplicate.InputRef = &shared
	if _, err := index.Add(duplicate); err == nil {
		t.Fatalf("duplicate descriptor id must be rejected")
	}
	replaced, err := index.Replace(duplicate)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if replaced.ID != duplicate.ID {
		t.Fatalf("replace returned wrong id %s", replaced.ID)
	}
	if !index.Remove(duplicate.ID) {
		t.Fatalf("remove must report success")
	}
	if index.Remove(duplicate.ID) {
		t.Fatalf("second remove must report false")
	}
	if index.Len() != 49 {
		t.Fatalf("expected 49 descriptors, got %d", index.Len())
	}
	if _, exists := index.Get(duplicate.ID); exists {
		t.Fatalf("removed descriptor must not be readable")
	}
}
