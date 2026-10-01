package kernel

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestAndroidNativeVirtualDisplayToolsAreModelReady(t *testing.T) {
	definitions := collectAndroidNativeToolDefinitions()
	byID := make(map[string]capability.ToolDefinition, len(definitions))
	for _, definition := range definitions {
		byID[definition.ID] = definition
	}

	expected := map[string]string{
		"android.virtual_display.status":  "android_virtual_display_status",
		"android.virtual_display.create":  "android_virtual_display_create",
		"android.virtual_display.launch":  "android_virtual_display_launch",
		"android.virtual_display.capture": "android_virtual_display_capture",
		"android.virtual_display.tap":     "android_virtual_display_tap",
		"android.virtual_display.swipe":   "android_virtual_display_swipe",
		"android.virtual_display.key":     "android_virtual_display_key",
		"android.virtual_display.text":    "android_virtual_display_text",
		"android.virtual_display.release": "android_virtual_display_release",
	}
	cache := capability.NewJSONSchemaCache()
	for id, modelName := range expected {
		definition, ok := byID[id]
		if !ok {
			t.Fatalf("missing virtual display tool: %s", id)
		}
		if definition.ModelName != modelName {
			t.Fatalf("virtual display tool model name mismatch: %s: %s", id, definition.ModelName)
		}
		if !definition.Enabled {
			t.Fatalf("virtual display tool is disabled: %s", id)
		}
		if !definition.ModelExposure.ExposedByDefault {
			t.Fatalf("virtual display tool is not exposed by default: %s", id)
		}
		if err := cache.ValidateToolDefinition(definition); err != nil {
			t.Fatalf("virtual display tool schema invalid: %s: %v", id, err)
		}
	}
}
