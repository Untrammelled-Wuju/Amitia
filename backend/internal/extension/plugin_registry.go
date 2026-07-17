package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type RegisteredPlugin struct {
	Manifest            PluginManifest
	Factory             PluginFactory
	RawManifest         json.RawMessage
	NormalizedManifest  json.RawMessage
	Compatible          bool
	CompatibilityReason string
}

type PluginFilter struct {
	Enabled *bool
	Hook    PluginHook
}

type PluginRegistry struct {
	mu            sync.RWMutex
	items         map[string]RegisteredPlugin
	validator     *SchemaValidator
	engineVersion string
}

func NewPluginRegistry(engineVersion string, validator *SchemaValidator) *PluginRegistry {
	if !semverPattern.MatchString(engineVersion) {
		engineVersion = "1.0.0"
	}
	return &PluginRegistry{items: map[string]RegisteredPlugin{}, validator: validator, engineVersion: engineVersion}
}

func (r *PluginRegistry) Register(_ context.Context, plugin Plugin, factory PluginFactory) error {
	if plugin == nil || factory == nil {
		return NewExtensionError(ErrPluginManifestInvalid, "Plugin factory is required", "", false, nil)
	}
	manifest := plugin.Manifest()
	raw, err := json.Marshal(manifest)
	if err != nil {
		return NewExtensionError(ErrPluginManifestInvalid, "Plugin manifest cannot be encoded", "", false, err)
	}
	if err := r.validate(plugin, manifest, raw); err != nil {
		return err
	}
	normalized, err := normalizeRawJSON(raw)
	if err != nil {
		return NewExtensionError(ErrPluginManifestInvalid, "Plugin manifest cannot be normalized", manifest.Metadata.ID, false, err)
	}
	compatible, reason := r.compatibility(manifest)
	registered := RegisteredPlugin{Manifest: clonePluginManifest(manifest), Factory: factory, RawManifest: append(json.RawMessage(nil), raw...), NormalizedManifest: normalized, Compatible: compatible, CompatibilityReason: reason}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[manifest.Metadata.ID]; exists {
		return NewExtensionError(ErrPluginManifestInvalid, "Duplicate plugin ID", manifest.Metadata.ID, false, nil)
	}
	r.items[manifest.Metadata.ID] = registered
	return nil
}

func (r *PluginRegistry) Get(_ context.Context, pluginID string) (RegisteredPlugin, error) {
	r.mu.RLock()
	item, ok := r.items[pluginID]
	r.mu.RUnlock()
	if !ok {
		return RegisteredPlugin{}, NewExtensionError(ErrPluginNotFound, "Plugin not found", pluginID, false, nil)
	}
	return cloneRegisteredPlugin(item), nil
}

func (r *PluginRegistry) List(_ context.Context, filter PluginFilter) ([]RegisteredPlugin, error) {
	r.mu.RLock()
	items := make([]RegisteredPlugin, 0, len(r.items))
	for _, item := range r.items {
		if filter.Enabled != nil && item.Manifest.Enabled != *filter.Enabled {
			continue
		}
		if filter.Hook != "" && !hasPluginHook(item.Manifest.Hooks, filter.Hook) {
			continue
		}
		items = append(items, cloneRegisteredPlugin(item))
	}
	r.mu.RUnlock()
	sort.Slice(items, func(i, j int) bool { return items[i].Manifest.Metadata.ID < items[j].Manifest.Metadata.ID })
	return items, nil
}

func (r *PluginRegistry) Unregister(_ context.Context, pluginID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[pluginID]; !ok {
		return NewExtensionError(ErrPluginNotFound, "Plugin not found", pluginID, false, nil)
	}
	delete(r.items, pluginID)
	return nil
}

func (r *PluginRegistry) validate(plugin Plugin, manifest PluginManifest, raw json.RawMessage) error {
	if manifest.Kind != "Plugin" || manifest.Entry.Kind != "builtin" {
		return NewExtensionError(ErrPluginManifestInvalid, "Only builtin Plugin entries are allowed", manifest.Metadata.ID, false, nil)
	}
	if !skillIDPattern.MatchString(manifest.Metadata.ID) || !strings.Contains(manifest.Metadata.ID, ".plugin.") {
		return NewExtensionError(ErrPluginManifestInvalid, "Invalid plugin ID", manifest.Metadata.ID, false, nil)
	}
	if !semverPattern.MatchString(manifest.Metadata.Version) || !semverPattern.MatchString(manifest.Compatibility.EngineMin) || (manifest.Compatibility.EngineMaxExclusive != "" && !semverPattern.MatchString(manifest.Compatibility.EngineMaxExclusive)) {
		return NewExtensionError(ErrPluginManifestInvalid, "Invalid plugin version", manifest.Metadata.Version, false, nil)
	}
	if strings.TrimSpace(manifest.Metadata.Name) == "" || strings.TrimSpace(manifest.Metadata.Description) == "" || strings.TrimSpace(manifest.Metadata.Author) == "" || strings.TrimSpace(manifest.Metadata.License) == "" || strings.TrimSpace(manifest.Entry.Name) == "" {
		return NewExtensionError(ErrPluginManifestInvalid, "Plugin metadata is incomplete", manifest.Metadata.ID, false, nil)
	}
	if manifest.Execution.HookTimeoutMS < 1 || manifest.Execution.HookTimeoutMS > 5000 || manifest.Execution.MaxConcurrency < 1 || manifest.Execution.MaxConcurrency > 16 || manifest.Execution.FailureThreshold < 1 || manifest.Execution.FailureThreshold > 20 || manifest.Execution.CircuitOpenMS < 100 || manifest.Execution.CircuitOpenMS > 3600000 {
		return NewExtensionError(ErrPluginManifestInvalid, "Plugin execution limits are invalid", manifest.Metadata.ID, false, nil)
	}
	if !semverPattern.MatchString(manifest.State.SchemaVersion) {
		return NewExtensionError(ErrPluginManifestInvalid, "Invalid plugin state version", manifest.State.SchemaVersion, false, nil)
	}
	seenHooks := map[PluginHook]bool{}
	for _, hook := range manifest.Hooks {
		if !validPluginHook(hook) || seenHooks[hook] {
			return NewExtensionError(ErrPluginManifestInvalid, "Invalid plugin hook", string(hook), false, nil)
		}
		seenHooks[hook] = true
		if !implementsPluginHook(plugin, hook) {
			return NewExtensionError(ErrPluginManifestInvalid, "Declared plugin hook is not implemented", string(hook), false, nil)
		}
	}
	for _, capability := range manifest.Capabilities {
		if _, ok := Capability(capability); !ok {
			return NewExtensionError(ErrPluginManifestInvalid, "Unknown capability", capability, false, nil)
		}
	}
	for _, subscription := range manifest.Subscriptions {
		if !validEventType(subscription) {
			return NewExtensionError(ErrPluginManifestInvalid, "Invalid event subscription", subscription, false, nil)
		}
	}
	for _, skillID := range manifest.RegisteredSkills {
		if !skillIDPattern.MatchString(skillID) || !strings.HasPrefix(skillID, pluginSkillPrefix(manifest.Metadata.ID)) {
			return NewExtensionError(ErrPluginManifestInvalid, "Plugin skill is outside its namespace", skillID, false, nil)
		}
	}
	if r.validator == nil {
		return NewExtensionError(ErrPluginManifestInvalid, "Schema validator is unavailable", manifest.Metadata.ID, false, nil)
	}
	if err := r.validator.ValidateManifest(raw); err != nil {
		return NewExtensionError(ErrPluginManifestInvalid, "Invalid plugin manifest", err.Error(), false, err)
	}
	if len(manifest.ConfigSchema) > 0 {
		if err := r.validator.ValidateSchema(manifest.Metadata.ID+"-plugin-config", manifest.ConfigSchema); err != nil {
			return NewExtensionError(ErrPluginManifestInvalid, "Invalid plugin config schema", err.Error(), false, err)
		}
		if err := r.validator.Validate(manifest.Metadata.ID+"-plugin-default-config", manifest.ConfigSchema, normalizeJSON(manifest.DefaultConfig)); err != nil {
			return NewExtensionError(ErrPluginManifestInvalid, "Invalid plugin default config", err.Error(), false, err)
		}
		var defaultConfig any
		if json.Unmarshal(normalizeJSON(manifest.DefaultConfig), &defaultConfig) == nil && hasPlaintextSecret(defaultConfig) {
			return NewExtensionError(ErrPluginManifestInvalid, "Secret defaults are not allowed in plugin manifests", manifest.Metadata.ID, false, nil)
		}
	}
	if len(manifest.State.Schema) > 0 {
		if err := r.validator.ValidateSchema(manifest.Metadata.ID+"-plugin-state", manifest.State.Schema); err != nil {
			return NewExtensionError(ErrPluginManifestInvalid, "Invalid plugin state schema", err.Error(), false, err)
		}
		if err := r.validator.Validate(manifest.Metadata.ID+"-plugin-default-state", manifest.State.Schema, normalizeJSON(manifest.State.Default)); err != nil {
			return NewExtensionError(ErrPluginManifestInvalid, "Invalid plugin default state", err.Error(), false, err)
		}
	}
	if len(manifest.Surface) > 0 {
		if err := validateSurface(manifest, manifest.Surface); err != nil {
			return err
		}
	}
	return nil
}

func (r *PluginRegistry) compatibility(manifest PluginManifest) (bool, string) {
	if compareSemver(r.engineVersion, manifest.Compatibility.EngineMin) < 0 {
		return false, fmt.Sprintf("engine %s is lower than %s", r.engineVersion, manifest.Compatibility.EngineMin)
	}
	if manifest.Compatibility.EngineMaxExclusive != "" && compareSemver(r.engineVersion, manifest.Compatibility.EngineMaxExclusive) >= 0 {
		return false, fmt.Sprintf("engine %s is not lower than %s", r.engineVersion, manifest.Compatibility.EngineMaxExclusive)
	}
	return true, ""
}

func implementsPluginHook(plugin Plugin, hook PluginHook) bool {
	switch hook {
	case HookOnLoad:
		_, ok := plugin.(LoadHook)
		return ok
	case HookOnEnable:
		_, ok := plugin.(EnableHook)
		return ok
	case HookBeforePrompt:
		_, ok := plugin.(BeforePromptHook)
		return ok
	case HookAfterReply:
		_, ok := plugin.(AfterReplyHook)
		return ok
	case HookOnEvent:
		_, ok := plugin.(EventHook)
		return ok
	case HookOnSchedule:
		_, ok := plugin.(ScheduleHook)
		return ok
	case HookOnDisable:
		_, ok := plugin.(DisableHook)
		return ok
	case HookOnUnload:
		_, ok := plugin.(UnloadHook)
		return ok
	default:
		return false
	}
}

func validPluginHook(hook PluginHook) bool {
	switch hook {
	case HookOnLoad, HookOnEnable, HookBeforePrompt, HookAfterReply, HookOnEvent, HookOnSchedule, HookOnDisable, HookOnUnload:
		return true
	default:
		return false
	}
}

func hasPluginHook(hooks []PluginHook, hook PluginHook) bool {
	for _, item := range hooks {
		if item == hook {
			return true
		}
	}
	return false
}

func pluginSkillPrefix(pluginID string) string {
	return strings.Replace(pluginID, ".plugin.", ".skill.", 1) + "."
}

func validEventType(value string) bool {
	parts := strings.Split(value, ".")
	return skillIDPattern.MatchString(value) && len(parts) >= 4 && strings.HasPrefix(parts[len(parts)-1], "v")
}

func normalizeRawJSON(raw json.RawMessage) (json.RawMessage, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func cloneRegisteredPlugin(item RegisteredPlugin) RegisteredPlugin {
	item.Manifest = clonePluginManifest(item.Manifest)
	item.RawManifest = append(json.RawMessage(nil), item.RawManifest...)
	item.NormalizedManifest = append(json.RawMessage(nil), item.NormalizedManifest...)
	return item
}

func clonePluginManifest(manifest PluginManifest) PluginManifest {
	copyManifest := manifest
	copyManifest.Capabilities = append([]string(nil), manifest.Capabilities...)
	copyManifest.Hooks = append([]PluginHook(nil), manifest.Hooks...)
	copyManifest.Subscriptions = append([]string(nil), manifest.Subscriptions...)
	copyManifest.RegisteredSkills = append([]string(nil), manifest.RegisteredSkills...)
	copyManifest.ConfigSchema = append(json.RawMessage(nil), manifest.ConfigSchema...)
	copyManifest.DefaultConfig = append(json.RawMessage(nil), manifest.DefaultConfig...)
	copyManifest.State.Schema = append(json.RawMessage(nil), manifest.State.Schema...)
	copyManifest.State.Default = append(json.RawMessage(nil), manifest.State.Default...)
	copyManifest.Surface = append(json.RawMessage(nil), manifest.Surface...)
	return copyManifest
}
