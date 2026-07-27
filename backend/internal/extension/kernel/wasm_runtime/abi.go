package wasm_runtime

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	ABIVersion1  = "amitia_wasm_abi_v1"
	ABICurrent   = ABIVersion1

	ExportAmitiaAlloc   = "amitia_alloc"
	ExportAmitiaDealloc = "amitia_dealloc"
	ExportAmitiaInvoke  = "amitia_invoke"

	HostModuleAmitia = "amitia"
)

var RequiredExports = []string{
	ExportAmitiaAlloc,
	ExportAmitiaDealloc,
	ExportAmitiaInvoke,
}

var AllowedHostModules = map[string]bool{
	HostModuleAmitia: true,
}

type ABIDescriptor struct {
	Version           string
	AllocExport       string
	DeallocExport     string
	InvokeExport      string
	ResultEncoding    string
	InputEncoding     string
	MaxInputBytes     int64
	MaxOutputBytes    int64
}

func DefaultABIDescriptor() ABIDescriptor {
	return ABIDescriptor{
		Version:        ABICurrent,
		AllocExport:    ExportAmitiaAlloc,
		DeallocExport:  ExportAmitiaDealloc,
		InvokeExport:   ExportAmitiaInvoke,
		ResultEncoding: "i64_packed",
		InputEncoding:  "utf8_json",
		MaxInputBytes:  1 * 1024 * 1024,
		MaxOutputBytes: 1 * 1024 * 1024,
	}
}

func EncodeResultDescriptor(ptr uint32, length uint32) uint64 {
	return uint64(ptr)<<32 | uint64(length)
}

func DecodeResultDescriptor(val uint64) (ptr uint32, length uint32, err error) {
	ptr = uint32(val >> 32)
	length = uint32(val & 0xFFFFFFFF)
	if ptr == 0 && length > 0 {
		return 0, 0, errors.New("wasm_abi: null pointer with non-zero length")
	}
	return ptr, length, nil
}

func ValidateABIExports(exports []string) error {
	exportMap := make(map[string]bool, len(exports))
	for _, e := range exports {
		exportMap[e] = true
	}
	for _, required := range RequiredExports {
		if !exportMap[required] {
			return NewWASMError(ErrCodeABIMismatch, fmt.Sprintf("missing required export: %s", required), nil)
		}
	}
	return nil
}

func ValidateImports(imports []string, allowed []HostImportName) error {
	allowedSet := make(map[string]bool)
	for _, a := range allowed {
		allowedSet[string(a)] = true
	}
	for _, imp := range imports {
		if !IsAllowedImport(imp, allowedSet) {
			return NewWASMError(ErrCodeImportNotAllowed, fmt.Sprintf("import not allowed: %s", imp), nil)
		}
	}
	return nil
}

func IsAllowedImport(importStr string, allowedSet map[string]bool) bool {
	if importStr == "" {
		return true
	}
	parts := splitImport(importStr)
	if len(parts) != 2 {
		return false
	}
	moduleName := parts[0]
	if !AllowedHostModules[moduleName] {
		return false
	}
	fullName := moduleName + "." + parts[1]
	return allowedSet[fullName]
}

func splitImport(s string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

func CanonicalizeInput(input json.RawMessage) ([]byte, error) {
	if len(input) == 0 {
		return []byte("{}"), nil
	}
	var v interface{}
	if err := json.Unmarshal(input, &v); err != nil {
		return nil, fmt.Errorf("wasm_abi: input is not valid JSON: %w", err)
	}
	canonical, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("wasm_abi: canonicalize input: %w", err)
	}
	return canonical, nil
}

func ValidateOutput(output []byte, maxBytes int64) error {
	if int64(len(output)) > maxBytes {
		return NewWASMError(ErrCodeOutputInvalid, fmt.Sprintf("output exceeds max bytes: %d > %d", len(output), maxBytes), nil)
	}
	if len(output) == 0 {
		return errors.New("wasm_abi: empty output")
	}
	var v interface{}
	if err := json.Unmarshal(output, &v); err != nil {
		return NewWASMError(ErrCodeOutputInvalid, fmt.Sprintf("output is not valid JSON: %v", err), nil)
	}
	return nil
}

func WriteU32LE(buf []byte, offset int, val uint32) error {
	if offset+4 > len(buf) {
		return errors.New("wasm_abi: buffer too small for u32")
	}
	binary.LittleEndian.PutUint32(buf[offset:], val)
	return nil
}

func ReadU32LE(buf []byte, offset int) (uint32, error) {
	if offset+4 > len(buf) {
		return 0, errors.New("wasm_abi: buffer too small for u32")
	}
	return binary.LittleEndian.Uint32(buf[offset:]), nil
}
