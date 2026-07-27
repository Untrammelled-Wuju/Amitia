package wasm_runtime

import (
	"encoding/json"
	"testing"
)

func TestEncodeResultDescriptor(t *testing.T) {
	val := EncodeResultDescriptor(0x1000, 0x200)
	expected := uint64(0x1000)<<32 | uint64(0x200)
	if val != expected {
		t.Fatalf("expected 0x%x, got 0x%x", expected, val)
	}
	val = EncodeResultDescriptor(0, 0)
	if val != 0 {
		t.Fatalf("expected 0, got 0x%x", val)
	}
	val = EncodeResultDescriptor(0xFFFFFFFF, 0xFFFFFFFF)
	if val != 0xFFFFFFFFFFFFFFFF {
		t.Fatalf("expected 0xFFFFFFFFFFFFFFFF, got 0x%x", val)
	}
}

func TestDecodeResultDescriptor(t *testing.T) {
	ptr, length, err := DecodeResultDescriptor(EncodeResultDescriptor(0x1000, 0x200))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ptr != 0x1000 {
		t.Fatalf("expected ptr 0x1000, got 0x%x", ptr)
	}
	if length != 0x200 {
		t.Fatalf("expected length 0x200, got 0x%x", length)
	}
	ptr, length, err = DecodeResultDescriptor(0)
	if err != nil {
		t.Fatalf("decode zero: %v", err)
	}
	if ptr != 0 || length != 0 {
		t.Fatalf("expected 0,0 got 0x%x,0x%x", ptr, length)
	}
}

func TestDecodeResultDescriptor_NullPtrNonZeroLength(t *testing.T) {
	val := EncodeResultDescriptor(0, 100)
	_, _, err := DecodeResultDescriptor(val)
	if err == nil {
		t.Fatalf("expected error for null ptr with non-zero length")
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	testCases := []struct {
		ptr    uint32
		length uint32
	}{
		{0, 0},
		{1, 1},
		{0x1000, 0x200},
		{0x7FFFFFFF, 0x7FFFFFFF},
	}
	for _, tc := range testCases {
		encoded := EncodeResultDescriptor(tc.ptr, tc.length)
		if tc.ptr == 0 && tc.length > 0 {
			continue
		}
		ptr, length, err := DecodeResultDescriptor(encoded)
		if err != nil {
			t.Fatalf("decode(%d,%d): %v", tc.ptr, tc.length, err)
		}
		if ptr != tc.ptr {
			t.Fatalf("ptr mismatch: expected %d, got %d", tc.ptr, ptr)
		}
		if length != tc.length {
			t.Fatalf("length mismatch: expected %d, got %d", tc.length, length)
		}
	}
}

func TestValidateABIExports(t *testing.T) {
	exports := []string{
		ExportAmitiaAlloc,
		ExportAmitiaDealloc,
		ExportAmitiaInvoke,
	}
	if err := ValidateABIExports(exports); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
	exports = append(exports, "extra_export")
	if err := ValidateABIExports(exports); err != nil {
		t.Fatalf("extra exports should still be valid: %v", err)
	}
}

func TestValidateABIExports_MissingRequired(t *testing.T) {
	if err := ValidateABIExports([]string{}); err == nil {
		t.Fatalf("expected error for empty exports")
	}
	err := ValidateABIExports([]string{ExportAmitiaAlloc, ExportAmitiaDealloc})
	if err == nil {
		t.Fatalf("expected error for missing invoke")
	}
	werr, ok := err.(*WASMError)
	if !ok {
		t.Fatalf("expected WASMError, got %T", err)
	}
	if werr.Code != ErrCodeABIMismatch {
		t.Fatalf("expected %s, got %s", ErrCodeABIMismatch, werr.Code)
	}
	if err := ValidateABIExports([]string{ExportAmitiaAlloc, ExportAmitiaInvoke}); err == nil {
		t.Fatalf("expected error for missing dealloc")
	}
	if err := ValidateABIExports([]string{ExportAmitiaDealloc, ExportAmitiaInvoke}); err == nil {
		t.Fatalf("expected error for missing alloc")
	}
}

func TestValidateImports(t *testing.T) {
	allowed := []HostImportName{ImportLog, ImportTime}
	allowedSet := make(map[string]bool)
	for _, a := range allowed {
		allowedSet[string(a)] = true
	}
	if !IsAllowedImport("", allowedSet) {
		t.Fatalf("empty import should be allowed")
	}
	if !IsAllowedImport("amitia.log", allowedSet) {
		t.Fatalf("amitia.log should be allowed")
	}
	if IsAllowedImport("amitia.storage_get", allowedSet) {
		t.Fatalf("amitia.storage_get should not be allowed")
	}
	if IsAllowedImport("unknown.module", allowedSet) {
		t.Fatalf("unknown module should not be allowed")
	}
}

func TestValidateImportsWithAllowedList(t *testing.T) {
	allowed := []HostImportName{ImportLog, ImportTime}
	if err := ValidateImports([]string{"amitia.log"}, allowed); err != nil {
		t.Fatalf("amitia.log should be allowed: %v", err)
	}
	if err := ValidateImports([]string{"amitia.storage_get"}, allowed); err == nil {
		t.Fatalf("amitia.storage_get should not be allowed")
	}
}

func TestCanonicalizeInput(t *testing.T) {
	result, err := CanonicalizeInput(nil)
	if err != nil {
		t.Fatalf("nil input: %v", err)
	}
	if string(result) != "{}" {
		t.Fatalf("expected {}, got %s", string(result))
	}
	result, err = CanonicalizeInput(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("empty object: %v", err)
	}
	if string(result) != "{}" {
		t.Fatalf("expected {}, got %s", string(result))
	}
	result, err = CanonicalizeInput(json.RawMessage(`{"b":1,"a":2}`))
	if err != nil {
		t.Fatalf("unsorted: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(result, &obj); err != nil {
		t.Fatalf("result not valid json: %v", err)
	}
	if obj["a"].(float64) != 2 || obj["b"].(float64) != 1 {
		t.Fatalf("canonicalized content mismatch: %s", string(result))
	}
}

func TestCanonicalizeInput_Invalid(t *testing.T) {
	_, err := CanonicalizeInput(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatalf("expected error for invalid json")
	}
}

func TestValidateOutput(t *testing.T) {
	if err := ValidateOutput([]byte(`{"ok":true}`), 1024); err != nil {
		t.Fatalf("valid output: %v", err)
	}
	if err := ValidateOutput([]byte(`{}`), 1024); err != nil {
		t.Fatalf("empty object: %v", err)
	}
}

func TestValidateOutput_Empty(t *testing.T) {
	if err := ValidateOutput([]byte{}, 1024); err == nil {
		t.Fatalf("expected error for empty output")
	}
}

func TestValidateOutput_InvalidJSON(t *testing.T) {
	err := ValidateOutput([]byte(`{invalid`), 1024)
	if err == nil {
		t.Fatalf("expected error for invalid json")
	}
	werr, ok := err.(*WASMError)
	if !ok {
		t.Fatalf("expected WASMError, got %T", err)
	}
	if werr.Code != ErrCodeOutputInvalid {
		t.Fatalf("expected %s, got %s", ErrCodeOutputInvalid, werr.Code)
	}
}

func TestValidateOutput_TooLarge(t *testing.T) {
	large := make([]byte, 100)
	for i := range large {
		large[i] = 'a'
	}
	large[0] = '"'
	large[99] = '"'
	err := ValidateOutput(large, 50)
	if err == nil {
		t.Fatalf("expected error for too large output")
	}
	werr, ok := err.(*WASMError)
	if !ok {
		t.Fatalf("expected WASMError, got %T", err)
	}
	if werr.Code != ErrCodeOutputInvalid {
		t.Fatalf("expected %s, got %s", ErrCodeOutputInvalid, werr.Code)
	}
}

func TestDefaultABIDescriptor(t *testing.T) {
	desc := DefaultABIDescriptor()
	if desc.Version != ABICurrent {
		t.Fatalf("expected %s, got %s", ABICurrent, desc.Version)
	}
	if desc.AllocExport != ExportAmitiaAlloc {
		t.Fatalf("expected %s, got %s", ExportAmitiaAlloc, desc.AllocExport)
	}
	if desc.DeallocExport != ExportAmitiaDealloc {
		t.Fatalf("expected %s, got %s", ExportAmitiaDealloc, desc.DeallocExport)
	}
	if desc.InvokeExport != ExportAmitiaInvoke {
		t.Fatalf("expected %s, got %s", ExportAmitiaInvoke, desc.InvokeExport)
	}
	if desc.MaxInputBytes <= 0 {
		t.Fatalf("MaxInputBytes should be > 0")
	}
	if desc.MaxOutputBytes <= 0 {
		t.Fatalf("MaxOutputBytes should be > 0")
	}
}

func TestWriteReadU32LE(t *testing.T) {
	buf := make([]byte, 8)
	if err := WriteU32LE(buf, 0, 0x12345678); err != nil {
		t.Fatalf("write: %v", err)
	}
	val, err := ReadU32LE(buf, 0)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if val != 0x12345678 {
		t.Fatalf("expected 0x12345678, got 0x%x", val)
	}
}

func TestWriteU32LE_BufferTooSmall(t *testing.T) {
	buf := make([]byte, 2)
	if err := WriteU32LE(buf, 0, 1); err == nil {
		t.Fatalf("expected error for small buffer")
	}
}

func TestReadU32LE_BufferTooSmall(t *testing.T) {
	buf := make([]byte, 2)
	if _, err := ReadU32LE(buf, 0); err == nil {
		t.Fatalf("expected error for small buffer")
	}
}
