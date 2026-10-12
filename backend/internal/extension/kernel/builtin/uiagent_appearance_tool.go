package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type UIAppearanceCommandRunner interface {
	ExecuteAppearanceCommand(ctx context.Context, action string, expectedRevision int64, changes map[string]interface{}) (map[string]interface{}, error)
}

const UIAgentAppearanceToolName = "uiagent.appearance"

var uiAppearanceRunner UIAppearanceCommandRunner
var appearanceHexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

const (
	appearanceMaxTopLevelProperties = 32
	appearanceMaxTargets            = 128
	appearanceMaxStyleDepth         = 8
	appearanceMaxStyleKeys          = 512
	appearanceMaxStyleNodes         = 2048
	appearanceMaxStringLength       = 4096
	appearanceMaxTargetLength       = 256
)

func SetUIAppearanceRunner(runner UIAppearanceCommandRunner) {
	uiAppearanceRunner = runner
}

func buildUIAgentAppearanceTool() tool.Tool {
	return tool.Tool{
		Type: "function",
		Function: tool.Function{
			Name:        UIAgentAppearanceToolName,
			Description: "Inspect, update, undo or reset the originating device's app UI styles. Only the authenticated device that initiated the user request can be modified. This is not limited to theme presets, accent colors or global borders: apply accepts precise style targets for any page, surface, component, message, dialog or instance, any supported state and any CSS/Flutter-mappable property. For a request such as making only the chat input black, inspect page=chat and target=chat.composer.input, then apply targets=[{scope:{page:\"chat\"},target:\"chat.composer.input\",states:[\"default\"],properties:{backgroundColor:\"#000000\",foregroundColor:\"#ffffff\"}}]. Omit platform in scope; the runtime binds it to the originating device automatically. Reset accepts precise scope, target, state, property, source or transaction filters and restores matching defaults without changing source or routes.",
			Parameters: tool.Parameters{
				Type: "object",
				Properties: map[string]tool.Property{
					"action":           {Type: "string", Enum: []string{"inspect", "apply", "reset", "undo"}},
					"expectedRevision": {Type: "integer", Description: "Optional revision returned by inspect. A stale revision is rejected."},
					"changes": {
						Type:        "object",
						Description: "Apply accepts schemaVersion 1 plus legacy global fields or targets, or one scope/target/properties rule. Each target supports id, source, scope {platform,page,surface}, target, states, properties/set. Reset accepts scope, target, states, properties, transactionId and source filters, optionally under reset. Inspect accepts scope, target, limit and includeComputed. Property names are open-ended CSS/Flutter-mappable values; values support string, number, boolean, array and object.",
					},
					"targets":         {Type: "array", Description: "Precise style target rules for apply."},
					"scope":           {Type: "object", Description: "Optional platform, page or surface scope."},
					"target":          {Type: "string", Description: "Stable page, surface, component or instance target id."},
					"states":          {Type: "array", Description: "Optional UI states such as default, hover, focus, pressed, disabled, dark or light."},
					"properties":      {Type: "object", Description: "Style properties for a single target rule."},
					"set":             {Type: "object", Description: "Alias for properties in a single target rule."},
					"reset":           {Type: "object", Description: "Precise reset filters for reset."},
					"source":          {Type: "string", Enum: []string{"ai", "user", "all"}, Description: "Optional rule source filter for reset."},
					"transactionId":   {Type: "string", Description: "Optional transaction filter for reset."},
					"limit":           {Type: "integer", Description: "Optional target limit for inspect."},
					"includeComputed": {Type: "boolean", Description: "Include computed property values in inspect results."},
				},
				Required: []string{"action"},
			},
		},
	}
}

func UIAgentAppearanceToolDefinition() capability.ToolDefinition {
	modelTool := buildUIAgentAppearanceTool()
	inputSchema, _ := json.Marshal(modelTool.Function.Parameters)
	return capability.ToolDefinition{
		ID:             "builtin.uiagent.appearance",
		ModelName:      UIAgentAppearanceToolName,
		Source:         capability.ToolSourceBuiltin,
		Name:           "UI Appearance",
		Description:    modelTool.Function.Description,
		InputSchema:    inputSchema,
		OutputSchema:   json.RawMessage(`{"type":"object","additionalProperties":true}`),
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      30000,
		Enabled:        true,
		Compatible:     true,
		ToolVersion: capability.ToolVersion{
			SchemaVersion: 1,
			Revision:      "ui-style-v1",
		},
		ModelExposure: capability.ModelExposureRule{
			ExposedByDefault: true,
			Categories:       []string{"ui", "style", "appearance"},
			Priority:         78,
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          30 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       false,
			ApprovalRequired: false,
			AllowBackground:  false,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 256 * 1024,
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: capability.RuntimeTypeBuiltin,
			RuntimeID:   "builtin",
			HandlerName: UIAgentAppearanceToolName,
		},
	}
}

func RegisterUIAgentAppearanceTool(ctx context.Context, registry *capability.ToolRegistry) error {
	if registry == nil {
		return fmt.Errorf("tool registry is nil")
	}
	if _, exists := registry.GetByModelName(ctx, UIAgentAppearanceToolName); exists {
		return nil
	}
	return registry.Register(ctx, UIAgentAppearanceToolDefinition())
}

func init() {
	tool.Register(buildUIAgentAppearanceTool(), uiAppearanceToolHandler)
}

func validateAppearanceChanges(changes map[string]interface{}) error {
	if len(changes) == 0 {
		return fmt.Errorf("apply needs at least one appearance property")
	}
	if len(changes) > appearanceMaxTopLevelProperties {
		return fmt.Errorf("appearance patch has too many top-level properties")
	}
	hasLegacy := false
	hasTargets := false
	hasTargetField := false
	hasPropertiesField := false
	hasSetField := false
	for key, value := range changes {
		if isAppearanceLegacyKey(key) {
			hasLegacy = true
		}
		switch key {
		case "schemaVersion":
			n, ok := appearanceNumber(value)
			if !ok || n != 1 {
				return fmt.Errorf("schemaVersion must be 1")
			}
		case "prediction", "expectedRevision", "commandId", "deviceId", "platform":
			return fmt.Errorf("runtime property %s is not allowed in changes", key)
		case "targets":
			if err := validateAppearanceTargets(value); err != nil {
				return err
			}
			hasTargets = true
		case "target":
			if err := validateAppearanceTarget(value, "target"); err != nil {
				return err
			}
			hasTargetField = true
		case "scope":
			if err := validateAppearanceScope(value, "scope"); err != nil {
				return err
			}
		case "states":
			if err := validateAppearanceStates(value, "states"); err != nil {
				return err
			}
		case "properties":
			if err := validateAppearanceStyleProperties(value, "properties"); err != nil {
				return err
			}
			hasPropertiesField = true
		case "set":
			if err := validateAppearanceStyleProperties(value, "set"); err != nil {
				return err
			}
			hasSetField = true
		case "source":
			if err := validateAppearanceRuleSource(value, "source"); err != nil {
				return err
			}
		case "reset":
			return fmt.Errorf("reset filters are not allowed for apply")
		case "preset", "themeMode":
			if s, ok := value.(string); !ok || (s != "system" && s != "light" && s != "dark") {
				return fmt.Errorf("invalid %s", key)
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
					if _, ok := v.(bool); !ok {
						return fmt.Errorf("customPalette.enabled must be boolean")
					}
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
		case "palette":
			if err := validateAppearancePalette(value); err != nil {
				return err
			}
		case "typography":
			if err := validateAppearanceTypography(value); err != nil {
				return err
			}
		case "layout", "icons", "components":
			if err := validateAppearanceNumericObject(value, key, 3, 80); err != nil {
				return err
			}
		case "componentVariants":
			if err := validateAppearanceObject(value, key, 3, 128); err != nil {
				return err
			}
		case "background":
			if err := validateAppearanceBackground(value); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown appearance property %s", key)
		}
	}
	if hasTargets {
		if hasTargetField || hasPropertiesField || hasSetField || changes["scope"] != nil || changes["states"] != nil {
			return fmt.Errorf("targets cannot be mixed with a single target rule")
		}
		return nil
	}
	if hasTargetField || hasPropertiesField || hasSetField || changes["scope"] != nil || changes["states"] != nil {
		if !hasTargetField {
			return fmt.Errorf("single target rule requires target")
		}
		if !hasPropertiesField && !hasSetField {
			return fmt.Errorf("single target rule requires properties or set")
		}
		return nil
	}
	if hasLegacy {
		return nil
	}
	return fmt.Errorf("apply needs at least one appearance property or target rule")
}

func isAppearanceLegacyKey(key string) bool {
	switch key {
	case "themeMode", "accentColor", "fontScale", "cornerStyle", "dynamicEffect", "reduceAnimation",
		"customPalette", "palette", "typography", "layout", "icons", "components", "componentVariants", "background":
		return true
	default:
		return false
	}
}

func validateAppearanceTargets(value interface{}) error {
	items, ok := appearanceList(value)
	if !ok || len(items) == 0 {
		return fmt.Errorf("targets must be a nonempty array")
	}
	if len(items) > appearanceMaxTargets {
		return fmt.Errorf("targets has too many entries")
	}
	for index, item := range items {
		if err := validateAppearanceTargetRule(item, index); err != nil {
			return err
		}
	}
	return nil
}

func validateAppearanceTargetRule(value interface{}, index int) error {
	fields, ok := value.(map[string]interface{})
	if !ok || len(fields) == 0 {
		return fmt.Errorf("targets[%d] must be an object", index)
	}
	hasProperties := false
	for key, raw := range fields {
		switch key {
		case "id":
			if err := validateAppearanceIdentifier(raw, "targets["+fmt.Sprint(index)+"].id"); err != nil {
				return err
			}
		case "source":
			if err := validateAppearanceRuleSource(raw, "targets["+fmt.Sprint(index)+"].source"); err != nil {
				return err
			}
		case "scope":
			if err := validateAppearanceScope(raw, "targets["+fmt.Sprint(index)+"].scope"); err != nil {
				return err
			}
		case "target":
			if err := validateAppearanceTarget(raw, "targets["+fmt.Sprint(index)+"].target"); err != nil {
				return err
			}
		case "states":
			if err := validateAppearanceStates(raw, "targets["+fmt.Sprint(index)+"].states"); err != nil {
				return err
			}
		case "properties":
			if err := validateAppearanceStyleProperties(raw, "targets["+fmt.Sprint(index)+"].properties"); err != nil {
				return err
			}
			hasProperties = true
		case "set":
			if err := validateAppearanceStyleProperties(raw, "targets["+fmt.Sprint(index)+"].set"); err != nil {
				return err
			}
			hasProperties = true
		default:
			return fmt.Errorf("targets[%d] has unknown field %s", index, key)
		}
	}
	if _, exists := fields["target"]; !exists {
		return fmt.Errorf("targets[%d].target is required", index)
	}
	if !hasProperties {
		return fmt.Errorf("targets[%d] requires properties or set", index)
	}
	return nil
}

func validateAppearanceTarget(value interface{}, name string) error {
	text, ok := value.(string)
	if !ok || !appearanceText(text, appearanceMaxTargetLength, false) {
		return fmt.Errorf("%s must be a nonempty safe string", name)
	}
	return nil
}

func validateAppearanceIdentifier(value interface{}, name string) error {
	text, ok := value.(string)
	if !ok || !appearanceText(text, 128, false) || !validAppearanceStyleKey(text) {
		return fmt.Errorf("%s must be a valid identifier", name)
	}
	return nil
}

func validateAppearanceScope(value interface{}, name string) error {
	fields, ok := value.(map[string]interface{})
	if !ok || len(fields) == 0 {
		return fmt.Errorf("%s must be a nonempty object", name)
	}
	for key, raw := range fields {
		switch key {
		case "platform":
			text, ok := raw.(string)
			if !ok || !validAppearancePlatform(text) {
				return fmt.Errorf("%s.platform must be desktop, mobile or web", name)
			}
		case "page", "surface":
			if err := validateAppearanceTarget(raw, name+"."+key); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s has unknown field %s", name, key)
		}
	}
	return nil
}

func validAppearancePlatform(value string) bool {
	switch value {
	case "desktop", "mobile", "web":
		return true
	default:
		return false
	}
}

func validateAppearanceStates(value interface{}, name string) error {
	items, ok := appearanceList(value)
	if !ok || len(items) == 0 || len(items) > 16 {
		return fmt.Errorf("%s must be a nonempty string array", name)
	}
	for _, item := range items {
		state, ok := item.(string)
		if !ok || !validAppearanceState(state) {
			return fmt.Errorf("%s contains an invalid state", name)
		}
	}
	return nil
}

func validAppearanceState(value string) bool {
	switch value {
	case "default", "hover", "focus", "focus-visible", "active", "pressed", "selected", "checked",
		"disabled", "loading", "empty", "error", "success", "warning", "dark", "light", "visited":
		return true
	default:
		return false
	}
}

func validateAppearanceRuleSource(value interface{}, name string) error {
	text, ok := value.(string)
	if !ok || (text != "ai" && text != "user") {
		return fmt.Errorf("%s must be ai or user", name)
	}
	return nil
}

func validateAppearanceStyleProperties(value interface{}, name string) error {
	fields, ok := value.(map[string]interface{})
	if !ok || len(fields) == 0 {
		return fmt.Errorf("%s must be a nonempty object", name)
	}
	keys := 0
	nodes := 0
	for key, raw := range fields {
		if !validAppearanceStyleKey(key) {
			return fmt.Errorf("%s has invalid property %s", name, key)
		}
		keys++
		nodes++
		if keys > appearanceMaxStyleKeys || nodes > appearanceMaxStyleNodes {
			return fmt.Errorf("%s has too many properties", name)
		}
		if err := validateAppearanceStyleValue(raw, name+"."+key, 2, &keys, &nodes); err != nil {
			return err
		}
	}
	return nil
}

func validateAppearanceStyleValue(value interface{}, path string, depth int, keys *int, nodes *int) error {
	if depth > appearanceMaxStyleDepth {
		return fmt.Errorf("%s exceeds maximum depth", path)
	}
	(*nodes)++
	if *nodes > appearanceMaxStyleNodes {
		return fmt.Errorf("%s has too many values", path)
	}
	switch item := value.(type) {
	case map[string]interface{}:
		for key, nested := range item {
			if !validAppearanceStyleKey(key) {
				return fmt.Errorf("%s has invalid property %s", path, key)
			}
			(*keys)++
			if *keys > appearanceMaxStyleKeys || *nodes > appearanceMaxStyleNodes {
				return fmt.Errorf("%s has too many properties", path)
			}
			if err := validateAppearanceStyleValue(nested, path+"."+key, depth+1, keys, nodes); err != nil {
				return err
			}
		}
	case string:
		if err := validateAppearanceStyleString(item, path); err != nil {
			return err
		}
	case bool:
	case float32:
		if _, ok := appearanceNumber(item); !ok || math.Abs(float64(item)) > 1e12 {
			return fmt.Errorf("%s number is invalid", path)
		}
	case float64:
		if _, ok := appearanceNumber(item); !ok || math.Abs(item) > 1e12 {
			return fmt.Errorf("%s number is invalid", path)
		}
	case json.Number:
		if _, ok := appearanceNumber(item); !ok {
			return fmt.Errorf("%s number is invalid", path)
		}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		if _, ok := appearanceNumber(item); !ok {
			return fmt.Errorf("%s number is invalid", path)
		}
	default:
		if items, ok := appearanceList(value); ok {
			for index, nested := range items {
				if err := validateAppearanceStyleValue(nested, fmt.Sprintf("%s[%d]", path, index), depth+1, keys, nodes); err != nil {
					return err
				}
			}
			return nil
		}
		return fmt.Errorf("%s contains an unsupported value", path)
	}
	return nil
}

func validateAppearanceStyleString(value string, path string) error {
	if !appearanceText(value, appearanceMaxStringLength, true) {
		return fmt.Errorf("%s string value is invalid", path)
	}
	normalized := strings.ToLower(strings.ReplaceAll(value, " ", ""))
	for _, forbidden := range []string{"javascript:", "expression", "url("} {
		if strings.Contains(normalized, forbidden) {
			return fmt.Errorf("%s contains a forbidden value", path)
		}
	}
	return nil
}

func validAppearanceStyleKey(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, ch := range value {
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch >= '0' && ch <= '9':
		case ch == '_' || ch == '-' || ch == '.' || ch == '$' || ch == ':':
		default:
			return false
		}
	}
	return true
}

func appearanceText(value string, maxLength int, allowEmpty bool) bool {
	if (!allowEmpty && strings.TrimSpace(value) == "") || len(value) > maxLength {
		return false
	}
	for _, ch := range value {
		if unicode.IsControl(ch) {
			return false
		}
	}
	return true
}

func appearanceList(value interface{}) ([]interface{}, bool) {
	if value == nil {
		return nil, false
	}
	reflected := reflect.ValueOf(value)
	if reflected.Kind() != reflect.Array && reflected.Kind() != reflect.Slice {
		return nil, false
	}
	result := make([]interface{}, 0, reflected.Len())
	for index := 0; index < reflected.Len(); index++ {
		result = append(result, reflected.Index(index).Interface())
	}
	return result, true
}

func validateAppearanceResetChanges(changes map[string]interface{}) error {
	if len(changes) > appearanceMaxTopLevelProperties {
		return fmt.Errorf("reset filter has too many top-level properties")
	}
	for key, value := range changes {
		switch key {
		case "schemaVersion":
			n, ok := appearanceNumber(value)
			if !ok || n != 1 {
				return fmt.Errorf("schemaVersion must be 1")
			}
		case "reset":
			if err := validateAppearanceResetFilter(value, "reset"); err != nil {
				return err
			}
		case "scope":
			if err := validateAppearanceScope(value, "scope"); err != nil {
				return err
			}
		case "target":
			if err := validateAppearanceTarget(value, "target"); err != nil {
				return err
			}
		case "states":
			if err := validateAppearanceStates(value, "states"); err != nil {
				return err
			}
		case "properties":
			if err := validateAppearancePropertyFilter(value, "properties"); err != nil {
				return err
			}
		case "transactionId":
			if err := validateAppearanceIdentifier(value, "transactionId"); err != nil {
				return err
			}
		case "source":
			if err := validateAppearanceResetSource(value, "source"); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown reset filter %s", key)
		}
	}
	return nil
}

func validateAppearanceResetFilter(value interface{}, name string) error {
	fields, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("%s must be an object", name)
	}
	for key, raw := range fields {
		switch key {
		case "scope":
			if err := validateAppearanceScope(raw, name+".scope"); err != nil {
				return err
			}
		case "target":
			if err := validateAppearanceTarget(raw, name+".target"); err != nil {
				return err
			}
		case "states":
			if err := validateAppearanceStates(raw, name+".states"); err != nil {
				return err
			}
		case "properties":
			if err := validateAppearancePropertyFilter(raw, name+".properties"); err != nil {
				return err
			}
		case "transactionId":
			if err := validateAppearanceIdentifier(raw, name+".transactionId"); err != nil {
				return err
			}
		case "source":
			if err := validateAppearanceResetSource(raw, name+".source"); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s has unknown field %s", name, key)
		}
	}
	return nil
}

func validateAppearancePropertyFilter(value interface{}, name string) error {
	items, ok := appearanceList(value)
	if !ok || len(items) == 0 || len(items) > appearanceMaxStyleKeys {
		return fmt.Errorf("%s must be a nonempty string array", name)
	}
	for _, item := range items {
		property, ok := item.(string)
		if !ok || !validAppearanceStyleKey(property) {
			return fmt.Errorf("%s contains an invalid property", name)
		}
	}
	return nil
}

func validateAppearanceResetSource(value interface{}, name string) error {
	text, ok := value.(string)
	if !ok || (text != "ai" && text != "user" && text != "all") {
		return fmt.Errorf("%s must be ai, user or all", name)
	}
	return nil
}

func validateAppearanceInspectChanges(changes map[string]interface{}) error {
	if len(changes) > appearanceMaxTopLevelProperties {
		return fmt.Errorf("inspect filter has too many top-level properties")
	}
	for key, value := range changes {
		switch key {
		case "schemaVersion":
			n, ok := appearanceNumber(value)
			if !ok || n != 1 {
				return fmt.Errorf("schemaVersion must be 1")
			}
		case "scope":
			if err := validateAppearanceScope(value, "scope"); err != nil {
				return err
			}
		case "target":
			if err := validateAppearanceTarget(value, "target"); err != nil {
				return err
			}
		case "limit":
			n, ok := appearanceNumber(value)
			if !ok || n < 1 || n > 512 || math.Trunc(n) != n {
				return fmt.Errorf("limit must be an integer 1..512")
			}
		case "includeComputed":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("includeComputed must be boolean")
			}
		default:
			return fmt.Errorf("unknown inspect filter %s", key)
		}
	}
	return nil
}

func validateAppearanceTypography(value interface{}) error {
	if err := validateAppearanceObject(value, "typography", 2, 64); err != nil {
		return err
	}
	fields, _ := value.(map[string]interface{})
	for key, raw := range fields {
		if key == "fontFamily" {
			if _, ok := raw.(string); !ok {
				return fmt.Errorf("typography.fontFamily must be a string")
			}
			continue
		}
		if _, ok := appearanceNumber(raw); !ok {
			return fmt.Errorf("typography.%s must be a number", key)
		}
	}
	return nil
}

func validateAppearanceNumericObject(value interface{}, name string, maxDepth int, maxKeys int) error {
	if err := validateAppearanceObject(value, name, maxDepth, maxKeys); err != nil {
		return err
	}
	var walk func(interface{}, string) error
	walk = func(raw interface{}, path string) error {
		switch item := raw.(type) {
		case map[string]interface{}:
			for key, nested := range item {
				if err := walk(nested, path+"."+key); err != nil {
					return err
				}
			}
		default:
			if _, ok := appearanceNumber(item); !ok {
				return fmt.Errorf("%s must contain numbers", path)
			}
		}
		return nil
	}
	return walk(value, name)
}

func validateAppearancePalette(value interface{}) error {
	fields, ok := value.(map[string]interface{})
	if !ok || len(fields) == 0 || len(fields) > 32 {
		return fmt.Errorf("palette must be a nonempty object")
	}
	for key, raw := range fields {
		text, ok := raw.(string)
		if !ok || !appearanceHexColor.MatchString(text) {
			return fmt.Errorf("palette.%s must be #RRGGBB", key)
		}
		if !validAppearanceKey(key) {
			return fmt.Errorf("invalid palette property %s", key)
		}
	}
	return nil
}

func validateAppearanceObject(value interface{}, name string, maxDepth int, maxKeys int) error {
	fields, ok := value.(map[string]interface{})
	if !ok || len(fields) == 0 {
		return fmt.Errorf("%s must be a nonempty object", name)
	}
	count := 0
	var walk func(interface{}, int) error
	walk = func(raw interface{}, depth int) error {
		if depth > maxDepth {
			return fmt.Errorf("%s exceeds maximum depth", name)
		}
		switch item := raw.(type) {
		case map[string]interface{}:
			for key, nested := range item {
				count++
				if count > maxKeys {
					return fmt.Errorf("%s has too many properties", name)
				}
				if !validAppearanceKey(key) {
					return fmt.Errorf("invalid %s property %s", name, key)
				}
				if err := walk(nested, depth+1); err != nil {
					return err
				}
			}
		case string:
			if len(item) > 128 || strings.ContainsAny(item, "\x00\r\n") {
				return fmt.Errorf("%s string value is invalid", name)
			}
		case bool:
		case json.Number:
			if _, ok := appearanceNumber(item); !ok {
				return fmt.Errorf("%s number is invalid", name)
			}
		case float64:
			if _, ok := appearanceNumber(item); !ok || math.Abs(item) > 100000 {
				return fmt.Errorf("%s number is invalid", name)
			}
		case int:
			if math.Abs(float64(item)) > 100000 {
				return fmt.Errorf("%s number is invalid", name)
			}
		default:
			return fmt.Errorf("%s contains an unsupported value", name)
		}
		return nil
	}
	return walk(value, 1)
}

func validateAppearanceBackground(value interface{}) error {
	fields, ok := value.(map[string]interface{})
	if !ok || len(fields) == 0 {
		return fmt.Errorf("background must be a nonempty object")
	}
	for key, raw := range fields {
		switch key {
		case "mode":
			s, ok := raw.(string)
			if !ok || (s != "default" && s != "color" && s != "gradient" && s != "image" && s != "video") {
				return fmt.Errorf("invalid background.mode")
			}
		case "enabled", "blurEnabled":
			if _, ok := raw.(bool); !ok {
				return fmt.Errorf("background.%s must be boolean", key)
			}
		case "color", "gradientStart", "gradientEnd", "overlayColor":
			s, ok := raw.(string)
			if !ok || !appearanceHexColor.MatchString(s) {
				return fmt.Errorf("background.%s must be #RRGGBB", key)
			}
		case "gradientAngle":
			n, ok := appearanceNumber(raw)
			if !ok || n < 0 || n > 360 {
				return fmt.Errorf("background.gradientAngle must be 0..360")
			}
		case "assetUri":
			s, ok := raw.(string)
			if !ok || strings.TrimSpace(s) == "" || len(s) > 2048 || strings.ContainsAny(s, "\x00\r\n") {
				return fmt.Errorf("background.assetUri is invalid")
			}
		case "name":
			s, ok := raw.(string)
			if !ok || len(s) > 180 || strings.ContainsAny(s, "\x00\r\n") {
				return fmt.Errorf("background.name is invalid")
			}
		case "opacity", "overlayOpacity":
			n, ok := appearanceNumber(raw)
			if !ok || n < 0 || n > 1 {
				return fmt.Errorf("background.%s must be 0..1", key)
			}
		case "blurRadius":
			n, ok := appearanceNumber(raw)
			if !ok || n < 0 || n > 30 {
				return fmt.Errorf("background.blurRadius must be 0..30")
			}
		case "fit":
			s, ok := raw.(string)
			if !ok || (s != "cover" && s != "contain" && s != "stretch" && s != "repeat") {
				return fmt.Errorf("invalid background.fit")
			}
		default:
			return fmt.Errorf("unknown background property %s", key)
		}
	}
	return nil
}

func validAppearanceKey(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, ch := range value {
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch >= '0' && ch <= '9':
		case ch == '_' || ch == '-' || ch == '.':
		default:
			return false
		}
	}
	return true
}

func appearanceNumber(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, !math.IsNaN(n) && !math.IsInf(n, 0)
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		f := float64(n)
		return f, !math.IsNaN(f) && !math.IsInf(f, 0)
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
	if _, exists := args["platform"]; exists {
		return tool.ErrorResult("invalid_input", "cross-device targeting is forbidden")
	}
	if _, exists := args["deviceId"]; exists {
		return tool.ErrorResult("invalid_input", "cross-device targeting is forbidden")
	}
	changes, err := appearanceChangesFromArgs(args)
	if err != nil {
		return tool.ErrorResult("invalid_input", err.Error())
	}
	expectedRevision := int64(0)
	if raw, exists := args["expectedRevision"]; exists {
		n, ok := appearanceNumber(raw)
		if !ok || n < 0 || math.Trunc(n) != n {
			return tool.ErrorResult("invalid_input", "expectedRevision must be a nonnegative integer")
		}
		expectedRevision = int64(n)
	}
	switch action {
	case "apply":
		if err := validateAppearanceChanges(changes); err != nil {
			return tool.ErrorResult("invalid_input", err.Error())
		}
	case "reset":
		if err := validateAppearanceResetChanges(changes); err != nil {
			return tool.ErrorResult("invalid_input", err.Error())
		}
	case "inspect":
		if err := validateAppearanceInspectChanges(changes); err != nil {
			return tool.ErrorResult("invalid_input", err.Error())
		}
	case "undo":
		if len(changes) != 0 {
			return tool.ErrorResult("invalid_input", "undo does not accept changes")
		}
	}
	if execCtx.Context == nil {
		return tool.ErrorResult("ui_appearance_origin_missing", "trusted originating device session is unavailable")
	}
	result, err := uiAppearanceRunner.ExecuteAppearanceCommand(execCtx.Context, action, expectedRevision, changes)
	if err != nil {
		return tool.ErrorResult("ui_appearance_failed", err.Error())
	}
	raw, _ := json.Marshal(result)
	return tool.ToolCallResult{
		Status:      tool.ToolStatusSuccess,
		Content:     string(raw),
		VisibleText: fmt.Sprintf("Appearance %s confirmed on the originating device", action),
		Confidence:  1,
	}
}

func appearanceChangesFromArgs(args map[string]interface{}) (map[string]interface{}, error) {
	changes := map[string]interface{}{}
	if raw, exists := args["changes"]; exists {
		object, ok := raw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("changes must be an object")
		}
		for key, value := range object {
			changes[key] = value
		}
	}
	for _, key := range []string{
		"schemaVersion", "targets", "reset", "scope", "target", "states", "properties", "set",
		"source", "transactionId", "limit", "includeComputed",
	} {
		if value, exists := args[key]; exists {
			if _, alreadySet := changes[key]; !alreadySet {
				changes[key] = value
			}
		}
	}
	return changes, nil
}
