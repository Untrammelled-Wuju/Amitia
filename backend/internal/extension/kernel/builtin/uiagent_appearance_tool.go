package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/u-ai/backend/internal/agent/tool"
)

type UIAppearanceCommandRunner interface {
	ExecuteAppearanceCommand(ctx context.Context, action string, changes map[string]interface{}) (map[string]interface{}, error)
}

var uiAppearanceRunner UIAppearanceCommandRunner
var appearanceHexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func SetUIAppearanceRunner(runner UIAppearanceCommandRunner) {
	uiAppearanceRunner = runner
}

func init() {
	tool.Register(tool.Tool{
		Type: "function",
		Function: tool.Function{
			Name: "uiagent.appearance",
			Description: "Inspect, update, undo or restore the originating device's app appearance. Only the authenticated device that initiated the user request can be modified. Administrators cannot target other devices. Applying, undoing or resetting needs confirmation on that device. Does not edit source or routes.",
			Parameters: tool.Parameters{
				Type: "object",
				Properties: map[string]tool.Property{
					"action": {Type: "string", Enum: []string{"inspect", "apply", "reset", "undo"}},
					"changes": {Type: "object", Description: "For apply: optional preset (system/light/dark), accentColor (#RRGGBB), fontScale (0.8..1.4), cornerStyle (0..2), dynamicEffect, reduceAnimation, customPalette {enabled, primary, secondary, text, textMode}. Unknown properties are rejected."},
				},
				Required: []string{"action"},
			},
		},
	}, uiAppearanceToolHandler)
}

func validateAppearanceChanges(changes map[string]interface{}) error {
	if len(changes) == 0 {
		return fmt.Errorf("apply needs at least one appearance property")
	}
	for key, value := range changes {
		switch key {
		case "preset":
			if s, ok := value.(string); !ok || (s != "system" && s != "light" && s != "dark") {
				return fmt.Errorf("invalid preset")
			}
		case "accentColor":
			if s, ok := value.(string); !ok || !appearanceHexColor.MatchString(s) {
				return fmt.Errorf("accentColor must be #RRGGBB")
			}
		case "fontScale":
			n, ok := appearanceNumber(value)
			if !ok || n < .8 || n > 1.4 {
				return fmt.Errorf("fontScale must be 0.8..1.4")
			}
		case "cornerStyle":
			n, ok := appearanceNumber(value)
			if !ok || n < 0 || n > 2 || math.Trunc(n) != n {
				return fmt.Errorf("cornerStyle must be an integer 0..2")
			}
		case "dynamicEffect", "reduceAnimation":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("%s must be boolean", key)
			}
		case "customPalette":
			fields, ok := value.(map[string]interface{})
			if !ok || len(fields) == 0 {
				return fmt.Errorf("customPalette must be a nonempty object")
			}
			for k, v := range fields {
				switch k {
				case "enabled":
					if _, ok := v.(bool); !ok { return fmt.Errorf("customPalette.enabled must be boolean") }
				case "primary", "secondary", "text":
					if s, ok := v.(string); !ok || !appearanceHexColor.MatchString(s) {
						return fmt.Errorf("customPalette.%s must be #RRGGBB", k)
					}
				case "textMode":
					s, ok := v.(string)
					if !ok || (s != "auto" && s != "dark" && s != "light" && s != "custom") {
						return fmt.Errorf("invalid customPalette.textMode")
					}
				default:
					return fmt.Errorf("unknown customPalette property %s", k)
				}
			}
		default:
			return fmt.Errorf("unknown appearance property %s", key)
		}
	}
	return nil
}

func appearanceNumber(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, !math.IsNaN(n) && !math.IsInf(n, 0)
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil && !math.IsInf(f, 0) && !math.IsNaN(f)
	default:
		return 0, false
	}
}

func uiAppearanceToolHandler(ctx context.Context, execCtx tool.ToolExecutionContext, args map[string]interface{}) tool.ToolCallResult {
	if uiAppearanceRunner == nil {
		return tool.ErrorResult("ui_appearance_unavailable", "UI appearance runtime is not configured")
	}
	action, _ := args["action"].(string)
	if strings.TrimSpace(execCtx.SpaceID) == "" {
		return tool.ErrorResult("ui_appearance_scope_missing", "Space identity is required")
	}
	switch action {
	case "inspect", "apply", "reset", "undo":
	default:
		return tool.ErrorResult("invalid_input", "unsupported appearance action")
	}
	if _, exists := args["platform"]; exists { return tool.ErrorResult("invalid_input", "cross-device targeting is forbidden") }
	if _, exists := args["deviceId"]; exists { return tool.ErrorResult("invalid_input", "cross-device targeting is forbidden") }
	changes, _ := args["changes"].(map[string]interface{})
	if action == "apply" {
		if err := validateAppearanceChanges(changes); err != nil {
			return tool.ErrorResult("invalid_input", err.Error())
		}
	} else if len(changes) != 0 {
		return tool.ErrorResult("invalid_input", "changes only allowed for apply")
	}
	if execCtx.Context == nil { return tool.ErrorResult("ui_appearance_origin_missing", "trusted originating device session is unavailable") }
	result, err := uiAppearanceRunner.ExecuteAppearanceCommand(execCtx.Context, action, changes)
	if err != nil {
		return tool.ErrorResult("ui_appearance_failed", err.Error())
	}
	raw, _ := json.Marshal(result)
	return tool.ToolCallResult{
		Status: tool.ToolStatusSuccess,
		Content: string(raw),
		VisibleText: fmt.Sprintf("Appearance %s confirmed on the originating device", action),
		Confidence: 1,
	}
}
