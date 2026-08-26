package manifest_v2

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

func TestSupportedRuntimeTypesIncludeExternalService(t *testing.T) {
	for _, moduleType := range []string{"builtin", "javascript", "data_only", "wasm", "native", "service"} {
		if !IsSupportedModuleType(moduleType) {
			t.Fatalf("module type %q unexpectedly unsupported", moduleType)
		}
	}
	for _, runtimeType := range []string{"javascript", "mcp", "workflow", "static", "wasm", "service"} {
		if !IsSupportedRuntimeType(runtimeType) {
			t.Fatalf("runtime type %q unexpectedly unsupported", runtimeType)
		}
	}
}

func TestSupportedRuntimeTypesRejectNonCanonicalAliases(t *testing.T) {
	for _, moduleType := range []string{" Service ", "SERVICE", "Native"} {
		if IsSupportedModuleType(moduleType) {
			t.Fatalf("non-canonical module type %q was accepted", moduleType)
		}
	}
	for _, runtimeType := range []string{" Service ", "SERVICE", "JavaScript"} {
		if IsSupportedRuntimeType(runtimeType) {
			t.Fatalf("non-canonical runtime type %q was accepted", runtimeType)
		}
	}
}

func TestRuntimeSupportTablesMatchManifestSchema(t *testing.T) {
	var schema struct {
		Properties struct {
			Modules struct {
				Items struct {
					Properties struct {
						Type struct {
							Enum []string `json:"enum"`
						} `json:"type"`
						Runtime struct {
							Properties struct {
								Type struct {
									Enum []string `json:"enum"`
								} `json:"type"`
							} `json:"properties"`
						} `json:"runtime"`
					} `json:"properties"`
				} `json:"items"`
			} `json:"modules"`
		} `json:"properties"`
	}
	if err := json.Unmarshal([]byte(manifestV2Schema), &schema); err != nil {
		t.Fatalf("decode manifest schema: %v", err)
	}

	moduleEnums := append([]string(nil), schema.Properties.Modules.Items.Properties.Type.Enum...)
	runtimeEnums := append([]string(nil), schema.Properties.Modules.Items.Properties.Runtime.Properties.Type.Enum...)
	moduleTypes := mapKeys(supportedModuleTypes)
	runtimeTypes := mapKeys(supportedRuntimeTypes)

	sort.Strings(moduleEnums)
	sort.Strings(runtimeEnums)
	if !reflect.DeepEqual(moduleTypes, moduleEnums) {
		t.Fatalf("module support/schema drift: support=%v schema=%v", moduleTypes, moduleEnums)
	}
	if !reflect.DeepEqual(runtimeTypes, runtimeEnums) {
		t.Fatalf("runtime support/schema drift: support=%v schema=%v", runtimeTypes, runtimeEnums)
	}

	for moduleType := range executableModuleTypes {
		if !IsSupportedModuleType(moduleType) {
			t.Fatalf("executable module type %q is not a supported module type", moduleType)
		}
	}
	if IsExecutableModuleType("data_only") {
		t.Fatal("data_only must never be treated as executable")
	}
}

func mapKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	sort.Strings(keys)
	return keys
}
