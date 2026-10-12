package builtin

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type captureAppearanceRunner struct {
	action           string
	expectedRevision int64
	changes          map[string]interface{}
}

func (r *captureAppearanceRunner) ExecuteAppearanceCommand(_ context.Context, action string, expectedRevision int64, changes map[string]interface{}) (map[string]interface{}, error) {
	r.action = action
	r.expectedRevision = expectedRevision
	r.changes = changes
	return map[string]interface{}{"ok": true}, nil
}

func TestValidateAppearanceChangesAcceptsStylePatchV1(t *testing.T) {
	changes := map[string]interface{}{
		"schemaVersion": 1,
		"themeMode":     "dark",
		"accentColor":   "#52B788",
		"palette": map[string]interface{}{
			"backgroundPrimary": "#111214",
			"surfacePrimary":    "#191A1C",
		},
		"typography": map[string]interface{}{
			"bodySize": 15,
		},
		"layout": map[string]interface{}{
			"spacing": map[string]interface{}{"page": 18},
			"radius":  map[string]interface{}{"md": 14},
		},
		"components": map[string]interface{}{
			"controlHeight": 46,
		},
		"componentVariants": map[string]interface{}{
			"button": map[string]interface{}{
				"fontWeight": 600,
			},
		},
		"background": map[string]interface{}{
			"mode":          "gradient",
			"gradientStart": "#111214",
			"gradientEnd":   "#1A2635",
			"opacity":       0.8,
			"blurEnabled":   true,
			"blurRadius":    12,
		},
	}
	if err := validateAppearanceChanges(changes); err != nil {
		t.Fatalf("valid style patch rejected: %v", err)
	}
}

func TestValidateAppearanceChangesRejectsUnknownAndUnsafeValues(t *testing.T) {
	cases := []map[string]interface{}{
		{"unknown": true},
		{"palette": map[string]interface{}{"backgroundPrimary": "red"}},
		{"background": map[string]interface{}{"mode": "script"}},
		{"background": map[string]interface{}{"opacity": 2}},
		{"typography": map[string]interface{}{"bodySize": "large"}},
		{"componentVariants": map[string]interface{}{"button": map[string]interface{}{"bad key": true}}},
	}
	for index, changes := range cases {
		if err := validateAppearanceChanges(changes); err == nil {
			t.Fatalf("case %d should be rejected", index)
		}
	}
}

func TestUIAgentAppearancePassesExpectedRevision(t *testing.T) {
	previous := uiAppearanceRunner
	runner := &captureAppearanceRunner{}
	SetUIAppearanceRunner(runner)
	t.Cleanup(func() { SetUIAppearanceRunner(previous) })
	result := uiAppearanceToolHandler(
		context.Background(),
		tool.ToolExecutionContext{Context: context.Background(), SpaceID: "space-a"},
		map[string]interface{}{
			"action":           "apply",
			"expectedRevision": 7,
			"changes": map[string]interface{}{
				"themeMode": "dark",
			},
		},
	)
	if result.Status != tool.ToolStatusSuccess {
		t.Fatalf("unexpected result: %#v", result)
	}
	if runner.action != "apply" || runner.expectedRevision != 7 {
		t.Fatalf("unexpected command: action=%s revision=%d", runner.action, runner.expectedRevision)
	}
}

func TestRegisterUIAgentAppearanceToolExposesRegisteredHandler(t *testing.T) {
	registry := capability.NewToolRegistry()
	if err := RegisterUIAgentAppearanceTool(context.Background(), registry); err != nil {
		t.Fatalf("register appearance tool: %v", err)
	}
	definition, ok := registry.GetByModelName(context.Background(), UIAgentAppearanceToolName)
	if !ok {
		t.Fatalf("expected %s in model tool registry", UIAgentAppearanceToolName)
	}
	if definition.Runtime.RuntimeType != capability.RuntimeTypeBuiltin {
		t.Fatalf("unexpected runtime type: %s", definition.Runtime.RuntimeType)
	}
	if definition.Runtime.HandlerName != UIAgentAppearanceToolName {
		t.Fatalf("unexpected handler name: %s", definition.Runtime.HandlerName)
	}
	if !tool.HasHandler(definition.Runtime.HandlerName) {
		t.Fatalf("registered handler %s is unavailable", definition.Runtime.HandlerName)
	}
	if !definition.Enabled || !definition.ModelExposure.ExposedByDefault {
		t.Fatalf("appearance tool must be enabled and model-exposed")
	}
}

func TestValidateAppearanceChangesAcceptsTargetRules(t *testing.T) {
	changes := map[string]interface{}{
		"schemaVersion": 1,
		"targets": []interface{}{
			map[string]interface{}{
				"id":     "rule-chat-input",
				"source": "ai",
				"scope": map[string]interface{}{
					"platform": "desktop",
					"page":     "/chat",
					"surface":  "composer",
				},
				"target": "chat.composer.input",
				"states": []interface{}{"default", "focus"},
				"properties": map[string]interface{}{
					"backgroundColor": "#000000",
					"foregroundColor": "#FFFFFF",
					"borderRadius":    16,
					"padding": map[string]interface{}{
						"x": 12,
						"y": 10,
					},
					"shadow": []interface{}{
						map[string]interface{}{
							"x":     0,
							"y":     2,
							"blur":  8,
							"color": "#00000066",
						},
					},
					"customVendorProperty": "allowed",
					"enabled":              true,
				},
			},
		},
	}
	if err := validateAppearanceChanges(changes); err != nil {
		t.Fatalf("valid target rule rejected: %v", err)
	}
}

func TestValidateAppearanceChangesAcceptsSingleTargetRule(t *testing.T) {
	changes := map[string]interface{}{
		"scope": map[string]interface{}{
			"platform": "mobile",
			"page":     "/chat",
		},
		"target": "chat.composer.send",
		"states": []string{"default", "pressed"},
		"set": map[string]interface{}{
			"backgroundColor": "#FF0000",
			"borderRadius":    999,
			"elevation":       4,
		},
	}
	if err := validateAppearanceChanges(changes); err != nil {
		t.Fatalf("valid single target rule rejected: %v", err)
	}
}

func TestValidateAppearanceChangesRejectsInvalidTargetRules(t *testing.T) {
	validScope := map[string]interface{}{"platform": "desktop", "page": "/chat"}
	cases := []map[string]interface{}{
		{
			"targets": []interface{}{
				map[string]interface{}{"target": "chat.composer.input"},
			},
		},
		{
			"targets": []interface{}{
				map[string]interface{}{"target": "", "properties": map[string]interface{}{"color": "#fff"}},
			},
		},
		{
			"targets": []interface{}{
				map[string]interface{}{
					"scope":      map[string]interface{}{"platform": "watch"},
					"target":     "chat.composer.input",
					"properties": map[string]interface{}{"color": "#fff"},
				},
			},
		},
		{
			"targets": []interface{}{
				map[string]interface{}{
					"target":     "chat.composer.input",
					"states":     []interface{}{"invented"},
					"properties": map[string]interface{}{"color": "#fff"},
				},
			},
		},
		{
			"scope":  validScope,
			"target": "chat.composer.input",
			"set": map[string]interface{}{
				"background": "url(https://example.com/a.png)",
			},
		},
		{
			"scope":  validScope,
			"target": "chat.composer.input",
			"set": map[string]interface{}{
				"background": "javascript:alert(1)",
			},
		},
		{
			"scope":  validScope,
			"target": "chat.composer.input",
			"set": map[string]interface{}{
				"filter": "expression(alert(1))",
			},
		},
		{
			"scope":  validScope,
			"target": "chat.composer.input",
			"set": map[string]interface{}{
				"content": "line\nbreak",
			},
		},
	}
	for index, changes := range cases {
		if err := validateAppearanceChanges(changes); err == nil {
			t.Fatalf("case %d should be rejected", index)
		}
	}
}

func TestValidateAppearanceResetChangesAcceptsFilters(t *testing.T) {
	changes := map[string]interface{}{
		"schemaVersion": 1,
		"reset": map[string]interface{}{
			"scope": map[string]interface{}{
				"platform": "desktop",
				"page":     "/chat",
			},
			"target":        "chat.composer.input",
			"states":        []interface{}{"default", "focus"},
			"properties":    []string{"backgroundColor", "foregroundColor"},
			"transactionId": "tx-123",
			"source":        "ai",
		},
	}
	if err := validateAppearanceResetChanges(changes); err != nil {
		t.Fatalf("valid reset filters rejected: %v", err)
	}
	if err := validateAppearanceResetChanges(map[string]interface{}{"reset": map[string]interface{}{}}); err != nil {
		t.Fatalf("empty reset filter should reset all: %v", err)
	}
}

func TestValidateAppearanceResetChangesRejectsInvalidFilters(t *testing.T) {
	cases := []map[string]interface{}{
		{"properties": map[string]interface{}{"backgroundColor": "#000000"}},
		{"reset": map[string]interface{}{"properties": []interface{}{"bad key"}}},
		{"reset": map[string]interface{}{"source": "system"}},
		{"reset": map[string]interface{}{"unknown": true}},
	}
	for index, changes := range cases {
		if err := validateAppearanceResetChanges(changes); err == nil {
			t.Fatalf("case %d should be rejected", index)
		}
	}
}

func TestValidateAppearanceInspectChangesAcceptsFilters(t *testing.T) {
	changes := map[string]interface{}{
		"scope": map[string]interface{}{
			"platform": "mobile",
			"page":     "/chat",
		},
		"target":          "chat.composer.input",
		"limit":           25,
		"includeComputed": true,
	}
	if err := validateAppearanceInspectChanges(changes); err != nil {
		t.Fatalf("valid inspect filters rejected: %v", err)
	}
	changes["limit"] = 0
	if err := validateAppearanceInspectChanges(changes); err == nil {
		t.Fatal("invalid inspect limit should be rejected")
	}
}

func TestUIAgentAppearancePassesResetAndInspectFilters(t *testing.T) {
	previous := uiAppearanceRunner
	runner := &captureAppearanceRunner{}
	SetUIAppearanceRunner(runner)
	t.Cleanup(func() { SetUIAppearanceRunner(previous) })

	result := uiAppearanceToolHandler(
		context.Background(),
		tool.ToolExecutionContext{Context: context.Background(), SpaceID: "space-a"},
		map[string]interface{}{
			"action": "reset",
			"target": "chat.composer.input",
			"reset": map[string]interface{}{
				"properties": []string{"backgroundColor"},
				"source":     "ai",
			},
		},
	)
	if result.Status != tool.ToolStatusSuccess {
		t.Fatalf("reset result: %#v", result)
	}
	if runner.action != "reset" || runner.changes["target"] != "chat.composer.input" {
		t.Fatalf("reset filters not forwarded: action=%s changes=%#v", runner.action, runner.changes)
	}

	result = uiAppearanceToolHandler(
		context.Background(),
		tool.ToolExecutionContext{Context: context.Background(), SpaceID: "space-a"},
		map[string]interface{}{
			"action":          "inspect",
			"target":          "chat.composer.input",
			"limit":           20,
			"includeComputed": true,
		},
	)
	if result.Status != tool.ToolStatusSuccess {
		t.Fatalf("inspect result: %#v", result)
	}
	if runner.action != "inspect" || runner.changes["limit"] != 20 || runner.changes["includeComputed"] != true {
		t.Fatalf("inspect filters not forwarded: action=%s changes=%#v", runner.action, runner.changes)
	}

	result = uiAppearanceToolHandler(
		context.Background(),
		tool.ToolExecutionContext{Context: context.Background(), SpaceID: "space-a"},
		map[string]interface{}{
			"action":  "undo",
			"changes": map[string]interface{}{"target": "chat.composer.input"},
		},
	)
	if result.Status != tool.ToolStatusFailed || result.ErrorCode != "invalid_input" {
		t.Fatalf("undo with changes should fail: %#v", result)
	}
}
