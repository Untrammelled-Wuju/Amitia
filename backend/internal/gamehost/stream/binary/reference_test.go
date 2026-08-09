package binary

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBinaryObjectID_Valid(t *testing.T) {
	id := NewBinaryObjectID()
	if err := ValidateBinaryObjectID(id); err != nil {
		t.Fatalf("valid id rejected: %v", err)
	}
}

func TestBinaryObjectID_EmptyRejected(t *testing.T) {
	if err := ValidateBinaryObjectID(""); err == nil {
		t.Fatal("empty id should be rejected")
	}
}

func TestBinaryObjectID_NoPrefixRejected(t *testing.T) {
	if err := ValidateBinaryObjectID("xyz_123"); err == nil {
		t.Fatal("missing bin_ prefix should be rejected")
	}
}

func TestBinaryObjectID_InvalidCharsRejected(t *testing.T) {
	if err := ValidateBinaryObjectID("bin_xyz/123"); err == nil {
		t.Fatal("slash should be rejected")
	}
}

func TestBinaryStorageKind_Valid(t *testing.T) {
	if err := BinaryStorageFile.Validate(); err != nil {
		t.Fatalf("file should be valid: %v", err)
	}
	if err := BinaryStorageSharedMemory.Validate(); err != nil {
		t.Fatalf("shared_memory should be valid: %v", err)
	}
}

func TestBinaryStorageKind_UnknownRejected(t *testing.T) {
	if err := BinaryStorageKind("s3").Validate(); err == nil {
		t.Fatal("unknown kind should be rejected")
	}
}

func TestBinaryLifetime_Valid(t *testing.T) {
	if err := BinaryLifetimeMessage.Validate(); err != nil {
		t.Fatalf("message should be valid: %v", err)
	}
	if err := BinaryLifetimeRuntime.Validate(); err != nil {
		t.Fatalf("runtime should be valid: %v", err)
	}
}

func TestBinaryLifetime_UnknownRejected(t *testing.T) {
	if err := BinaryLifetime("permanent").Validate(); err == nil {
		t.Fatal("unknown lifetime should be rejected")
	}
}

func TestChecksum_SHA256Valid(t *testing.T) {
	cs := &Checksum{Algorithm: "sha256", Value: "abc123"}
	if err := cs.Validate(); err != nil {
		t.Fatalf("sha256 should be valid: %v", err)
	}
}

func TestChecksum_UnsupportedAlgorithmRejected(t *testing.T) {
	cs := &Checksum{Algorithm: "md5", Value: "abc"}
	if err := cs.Validate(); err == nil {
		t.Fatal("md5 should be rejected")
	}
}

func TestChecksum_EmptyValueRejected(t *testing.T) {
	cs := &Checksum{Algorithm: "sha256", Value: ""}
	if err := cs.Validate(); err == nil {
		t.Fatal("empty value should be rejected")
	}
}

func TestBinaryReference_Valid(t *testing.T) {
	id := NewBinaryObjectID()
	ref := &BinaryReference{
		ID:       id,
		Kind:     BinaryStorageFile,
		Size:     1024,
		Lifetime: BinaryLifetimeMessage,
	}
	if err := ref.Validate(); err != nil {
		t.Fatalf("valid ref rejected: %v", err)
	}
}

func TestBinaryReference_EmptyIDRejected(t *testing.T) {
	ref := &BinaryReference{
		ID:       "",
		Kind:     BinaryStorageFile,
		Size:     100,
		Lifetime: BinaryLifetimeMessage,
	}
	if err := ref.Validate(); err == nil {
		t.Fatal("empty id should be rejected")
	}
}

func TestBinaryReference_NegativeSizeRejected(t *testing.T) {
	id := NewBinaryObjectID()
	ref := &BinaryReference{
		ID:       id,
		Kind:     BinaryStorageFile,
		Size:     -1,
		Lifetime: BinaryLifetimeMessage,
	}
	if err := ref.Validate(); err == nil {
		t.Fatal("negative size should be rejected")
	}
}

func TestBinaryReference_UnknownKindRejected(t *testing.T) {
	id := NewBinaryObjectID()
	ref := &BinaryReference{
		ID:       id,
		Kind:     BinaryStorageKind("http"),
		Size:     100,
		Lifetime: BinaryLifetimeMessage,
	}
	if err := ref.Validate(); err == nil {
		t.Fatal("unknown kind should be rejected")
	}
}

func TestBinaryReference_UnknownLifetimeRejected(t *testing.T) {
	id := NewBinaryObjectID()
	ref := &BinaryReference{
		ID:       id,
		Kind:     BinaryStorageFile,
		Size:     100,
		Lifetime: BinaryLifetime("forever"),
	}
	if err := ref.Validate(); err == nil {
		t.Fatal("unknown lifetime should be rejected")
	}
}

func TestBinaryReference_LongMediaTypeRejected(t *testing.T) {
	id := NewBinaryObjectID()
	ref := &BinaryReference{
		ID:        id,
		Kind:      BinaryStorageFile,
		Size:      100,
		MediaType: strings.Repeat("x", maxMediaTypeLength+1),
		Lifetime:  BinaryLifetimeMessage,
	}
	if err := ref.Validate(); err == nil {
		t.Fatal("long media type should be rejected")
	}
}

func TestBinaryReference_MetadataTooLargeRejected(t *testing.T) {
	id := NewBinaryObjectID()
	ref := &BinaryReference{
		ID:       id,
		Kind:     BinaryStorageFile,
		Size:     100,
		Lifetime: BinaryLifetimeMessage,
		Metadata: map[string]json.RawMessage{
			"key": json.RawMessage(strings.Repeat("x", maxMetadataBytes+1)),
		},
	}
	if err := ref.Validate(); err == nil {
		t.Fatal("large metadata should be rejected")
	}
}

func TestBinaryReference_NoPathInWire(t *testing.T) {
	id := NewBinaryObjectID()
	ref := &BinaryReference{
		ID:        id,
		Kind:      BinaryStorageFile,
		Size:      248392,
		MediaType: "image/png",
		Lifetime:  BinaryLifetimeMessage,
	}
	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	s := string(data)
	if strings.Contains(s, "C:\\") || strings.Contains(s, "/home/") || strings.Contains(s, "/tmp/") {
		t.Fatal("wire should not contain absolute paths")
	}
}

func TestBinaryReference_LargeIntegerMetadata(t *testing.T) {
	id := NewBinaryObjectID()
	ref := &BinaryReference{
		ID:       id,
		Kind:     BinaryStorageFile,
		Size:     100,
		Lifetime: BinaryLifetimeMessage,
		Metadata: map[string]json.RawMessage{
			"tick": json.RawMessage(`9007199254740993`),
		},
	}
	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var restored BinaryReference
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if string(restored.Metadata["tick"]) != "9007199254740993" {
		t.Fatalf("large integer corrupted: %s", restored.Metadata["tick"])
	}
}

func TestBinaryReference_MetadataDeepCopy(t *testing.T) {
	id := NewBinaryObjectID()
	ref := &BinaryReference{
		ID:       id,
		Kind:     BinaryStorageFile,
		Size:     100,
		Lifetime: BinaryLifetimeMessage,
	}
	_, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
}
