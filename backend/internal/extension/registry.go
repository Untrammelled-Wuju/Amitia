package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var skillIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:[.-][a-z0-9]+)*$`)
var semverPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

type RegistryStateStore interface {
	ResolveEnabled(context.Context, SkillDefinition) (bool, error)
	UpsertDefinition(context.Context, SkillDefinition) error
}

type SkillRegistry interface {
	Register(context.Context, SkillDefinition, SkillHandler) error
	Unregister(context.Context, string) error
	Get(context.Context, string) (RegisteredSkill, error)
	GetByModelName(context.Context, string) (RegisteredSkill, error)
	List(context.Context, SkillFilter) ([]RegisteredSkill, error)
	Available(context.Context, ExecutionScope) ([]SkillDefinition, error)
	SetEnabled(context.Context, string, bool) error
}

type Registry struct {
	mu            sync.RWMutex
	items         map[string]RegisteredSkill
	modelNames    map[string]string
	validator     *SchemaValidator
	stateStore    RegistryStateStore
	engineVersion string
}

func NewRegistry(engineVersion string, validator *SchemaValidator, stateStore RegistryStateStore) *Registry {
	if engineVersion == "" || !semverPattern.MatchString(engineVersion) {
		engineVersion = "1.0.0"
	}
	return &Registry{items: map[string]RegisteredSkill{}, modelNames: map[string]string{}, validator: validator, stateStore: stateStore, engineVersion: engineVersion}
}

func (r *Registry) Register(ctx context.Context, definition SkillDefinition, handler SkillHandler) error {
	if handler == nil {
		return NewExtensionError(ErrSkillManifestInvalid, "Skill handler is required", definition.ID, false, nil)
	}
	if err := r.validateDefinition(definition); err != nil {
		return err
	}
	if r.stateStore != nil {
		enabled, err := r.stateStore.ResolveEnabled(ctx, definition)
		if err != nil {
			return err
		}
		definition.Enabled = enabled
	}
	definition.Compatible, definition.CompatibilityReason = r.compatibility(definition.Manifest)
	if definition.Timeout <= 0 {
		definition.Timeout = time.Duration(definition.TimeoutMS) * time.Millisecond
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[definition.ID]; exists {
		return NewExtensionError(ErrSkillDuplicateID, "Duplicate skill ID", definition.ID, false, nil)
	}
	if existing, exists := r.modelNames[definition.ModelName]; exists {
		return NewExtensionError(ErrSkillDuplicateID, "Duplicate model tool name", existing, false, nil)
	}
	r.items[definition.ID] = cloneRegistered(RegisteredSkill{Definition: definition, Handler: handler})
	r.modelNames[definition.ModelName] = definition.ID
	if r.stateStore != nil {
		if err := r.stateStore.UpsertDefinition(ctx, definition); err != nil {
			delete(r.items, definition.ID)
			delete(r.modelNames, definition.ModelName)
			return err
		}
	}
	return nil
}

func (r *Registry) Unregister(_ context.Context, skillID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[skillID]
	if !ok {
		return NewExtensionError(ErrSkillNotFound, "Skill not found", skillID, false, nil)
	}
	delete(r.modelNames, item.Definition.ModelName)
	delete(r.items, skillID)
	return nil
}

func (r *Registry) Get(_ context.Context, skillID string) (RegisteredSkill, error) {
	r.mu.RLock()
	item, ok := r.items[skillID]
	r.mu.RUnlock()
	if !ok {
		return RegisteredSkill{}, NewExtensionError(ErrSkillNotFound, "Skill not found", skillID, false, nil)
	}
	return cloneRegistered(item), nil
}

func (r *Registry) GetByModelName(ctx context.Context, name string) (RegisteredSkill, error) {
	r.mu.RLock()
	id, ok := r.modelNames[name]
	r.mu.RUnlock()
	if !ok {
		return RegisteredSkill{}, NewExtensionError(ErrSkillNotFound, "Skill not found", name, false, nil)
	}
	return r.Get(ctx, id)
}

func (r *Registry) List(_ context.Context, filter SkillFilter) ([]RegisteredSkill, error) {
	r.mu.RLock()
	items := make([]RegisteredSkill, 0, len(r.items))
	for _, item := range r.items {
		if filter.Enabled != nil && item.Definition.Enabled != *filter.Enabled {
			continue
		}
		if filter.Trigger != "" && !hasTrigger(item.Definition.Triggers, filter.Trigger) {
			continue
		}
		if filter.Source != "" && item.Definition.Source != filter.Source {
			continue
		}
		items = append(items, cloneRegistered(item))
	}
	r.mu.RUnlock()
	sort.Slice(items, func(i, j int) bool { return items[i].Definition.ID < items[j].Definition.ID })
	return items, nil
}

func (r *Registry) Available(ctx context.Context, scope ExecutionScope) ([]SkillDefinition, error) {
	items, err := r.List(ctx, SkillFilter{Trigger: scope.Trigger})
	if err != nil {
		return nil, err
	}
	result := make([]SkillDefinition, 0, len(items))
	for _, item := range items {
		if item.Definition.Enabled && item.Definition.Compatible {
			result = append(result, cloneDefinition(item.Definition))
		}
	}
	return result, nil
}

func (r *Registry) SetEnabled(ctx context.Context, skillID string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[skillID]
	if !ok {
		return NewExtensionError(ErrSkillNotFound, "Skill not found", skillID, false, nil)
	}
	if enabled && !item.Definition.Compatible {
		return NewExtensionError(ErrSkillIncompatible, "Skill is incompatible", item.Definition.CompatibilityReason, false, nil)
	}
	item.Definition.Enabled = enabled
	if r.stateStore != nil {
		if err := r.stateStore.UpsertDefinition(ctx, item.Definition); err != nil {
			return err
		}
	}
	r.items[skillID] = item
	return nil
}

func (r *Registry) validateDefinition(definition SkillDefinition) error {
	if !skillIDPattern.MatchString(definition.ID) {
		return NewExtensionError(ErrSkillManifestInvalid, "Invalid skill ID", definition.ID, false, nil)
	}
	if !semverPattern.MatchString(definition.Version) {
		return NewExtensionError(ErrSkillManifestInvalid, "Invalid skill version", definition.Version, false, nil)
	}
	if strings.TrimSpace(definition.ModelName) == "" || (definition.Entry.Kind == "workflow" && strings.TrimSpace(definition.Entry.ArtifactID) == "") || (definition.Entry.Kind != "workflow" && strings.TrimSpace(definition.Entry.Name) == "") {
		return NewExtensionError(ErrSkillManifestInvalid, "Skill entry is invalid", definition.ID, false, nil)
	}
	for _, capability := range definition.Capabilities {
		if _, ok := Capability(capability); !ok {
			return NewExtensionError(ErrSkillManifestInvalid, "Unknown capability", capability, false, nil)
		}
	}
	if r.validator == nil {
		return NewExtensionError(ErrSkillManifestInvalid, "Schema validator is unavailable", definition.ID, false, nil)
	}
	if err := r.validator.ValidateManifest(definition.Manifest); err != nil {
		return NewExtensionError(ErrSkillManifestInvalid, "Invalid skill manifest", err.Error(), false, err)
	}
	var manifest Manifest
	if err := json.Unmarshal(definition.Manifest, &manifest); err != nil {
		return NewExtensionError(ErrSkillManifestInvalid, "Invalid skill manifest", err.Error(), false, err)
	}
	if !semverPattern.MatchString(manifest.Compatibility.EngineMin) || (manifest.Compatibility.EngineMaxExclusive != "" && !semverPattern.MatchString(manifest.Compatibility.EngineMaxExclusive)) {
		return NewExtensionError(ErrSkillManifestInvalid, "Invalid engine compatibility version", definition.ID, false, nil)
	}
	if manifest.Metadata.ID != definition.ID || manifest.Metadata.Name != definition.Name || manifest.Metadata.Version != definition.Version || manifest.Metadata.Description != definition.Description || manifest.Metadata.Author != definition.Author || manifest.Metadata.License != definition.License {
		return NewExtensionError(ErrSkillManifestInvalid, "Manifest metadata does not match skill definition", definition.ID, false, nil)
	}
	if manifest.Entry != definition.Entry || !sameStrings(manifest.Capabilities, definition.Capabilities) || !sameTriggers(manifest.Triggers, definition.Triggers) {
		return NewExtensionError(ErrSkillManifestInvalid, "Manifest routing does not match skill definition", definition.ID, false, nil)
	}
	if manifest.Execution.TimeoutMS != definition.TimeoutMS || manifest.Execution.HasSideEffects != definition.HasSideEffects || manifest.Execution.Retryable != definition.Retryable || manifest.Execution.Idempotent != definition.Idempotent {
		return NewExtensionError(ErrSkillManifestInvalid, "Manifest execution policy does not match skill definition", definition.ID, false, nil)
	}
	if manifest.AllowLLM != hasTrigger(definition.Triggers, TriggerLLM) || manifest.AllowManual != hasTrigger(definition.Triggers, TriggerManual) {
		return NewExtensionError(ErrSkillManifestInvalid, "Manifest trigger flags do not match skill definition", definition.ID, false, nil)
	}
	if !sameJSON(manifest.InputSchema, definition.InputSchema) || !sameJSON(manifest.OutputSchema, definition.OutputSchema) || !sameJSON(normalizeJSON(manifest.DefaultConfig), normalizeJSON(definition.DefaultConfig)) {
		return NewExtensionError(ErrSkillManifestInvalid, "Manifest schemas do not match skill definition", definition.ID, false, nil)
	}
	if len(manifest.ConfigSchema) != 0 || len(definition.ConfigSchema) != 0 {
		if !sameJSON(normalizeJSON(manifest.ConfigSchema), normalizeJSON(definition.ConfigSchema)) {
			return NewExtensionError(ErrSkillManifestInvalid, "Manifest config schema does not match skill definition", definition.ID, false, nil)
		}
	}
	if err := r.validator.ValidateSchema(definition.ID+"-input", definition.InputSchema); err != nil {
		return NewExtensionError(ErrSkillManifestInvalid, "Invalid input schema", err.Error(), false, err)
	}
	if err := r.validator.ValidateSchema(definition.ID+"-output", definition.OutputSchema); err != nil {
		return NewExtensionError(ErrSkillManifestInvalid, "Invalid output schema", err.Error(), false, err)
	}
	if len(definition.ConfigSchema) > 0 {
		if err := r.validator.ValidateSchema(definition.ID+"-config", definition.ConfigSchema); err != nil {
			return NewExtensionError(ErrSkillManifestInvalid, "Invalid config schema", err.Error(), false, err)
		}
		if err := r.validator.Validate(definition.ID+"-default-config", definition.ConfigSchema, normalizeJSON(definition.DefaultConfig)); err != nil {
			return NewExtensionError(ErrSkillManifestInvalid, "Invalid default configuration", err.Error(), false, err)
		}
	}
	var defaultConfig interface{}
	if json.Unmarshal(normalizeJSON(definition.DefaultConfig), &defaultConfig) == nil && hasPlaintextSecret(defaultConfig) {
		return NewExtensionError(ErrSkillManifestInvalid, "Secret defaults are not allowed in manifests", definition.ID, false, nil)
	}
	return nil
}

func sameJSON(left, right json.RawMessage) bool {
	var leftValue interface{}
	var rightValue interface{}
	if json.Unmarshal(left, &leftValue) != nil || json.Unmarshal(right, &rightValue) != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func sameTriggers(left, right []SkillTrigger) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (r *Registry) compatibility(raw json.RawMessage) (bool, string) {
	var manifest Manifest
	if json.Unmarshal(raw, &manifest) != nil {
		return false, "manifest cannot be parsed"
	}
	if manifest.Compatibility.EngineMin != "" && compareSemver(r.engineVersion, manifest.Compatibility.EngineMin) < 0 {
		return false, fmt.Sprintf("engine %s is lower than %s", r.engineVersion, manifest.Compatibility.EngineMin)
	}
	if manifest.Compatibility.EngineMaxExclusive != "" && compareSemver(r.engineVersion, manifest.Compatibility.EngineMaxExclusive) >= 0 {
		return false, fmt.Sprintf("engine %s is not lower than %s", r.engineVersion, manifest.Compatibility.EngineMaxExclusive)
	}
	return true, ""
}

func compareSemver(left, right string) int {
	parse := func(value string) [3]int {
		value = strings.SplitN(value, "-", 2)[0]
		parts := strings.Split(value, ".")
		var result [3]int
		for i := 0; i < len(parts) && i < 3; i++ {
			result[i], _ = strconv.Atoi(parts[i])
		}
		return result
	}
	l := parse(left)
	r := parse(right)
	for i := 0; i < 3; i++ {
		if l[i] < r[i] {
			return -1
		}
		if l[i] > r[i] {
			return 1
		}
	}
	return 0
}

func cloneRegistered(item RegisteredSkill) RegisteredSkill {
	return RegisteredSkill{Definition: cloneDefinition(item.Definition), Handler: item.Handler}
}

func cloneDefinition(definition SkillDefinition) SkillDefinition {
	copyDef := definition
	copyDef.InputSchema = append(json.RawMessage(nil), definition.InputSchema...)
	copyDef.OutputSchema = append(json.RawMessage(nil), definition.OutputSchema...)
	copyDef.ConfigSchema = append(json.RawMessage(nil), definition.ConfigSchema...)
	copyDef.DefaultConfig = append(json.RawMessage(nil), definition.DefaultConfig...)
	copyDef.Manifest = append(json.RawMessage(nil), definition.Manifest...)
	copyDef.Capabilities = append([]string{}, definition.Capabilities...)
	copyDef.Dependencies = append([]string(nil), definition.Dependencies...)
	copyDef.Triggers = append([]SkillTrigger{}, definition.Triggers...)
	return copyDef
}
