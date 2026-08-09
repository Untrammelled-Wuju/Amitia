package config

import (
	"context"
	"sort"
	"time"
)

type ConfigStore interface {
	LoadPluginConfig(ctx context.Context, pluginID string) (*ConfigBlob, error)
	SavePluginConfig(ctx context.Context, pluginID string, entries []ConfigEntry) error
	LoadRuntimeConfig(ctx context.Context, runtimeID string) (*ConfigBlob, error)
	SaveRuntimeConfig(ctx context.Context, runtimeID string, entries []ConfigEntry) error
	LoadServiceConfig(ctx context.Context, runtimeID, serviceID string) (*ConfigBlob, error)
	SaveServiceConfig(ctx context.Context, runtimeID, serviceID string, entries []ConfigEntry) error
}

type ConfigBlob struct {
	Scope   ConfigScope   `json:"scope"`
	Entries []ConfigEntry `json:"entries"`
}

type Resolver struct {
	store            ConfigStore
	schema           *ConfigSchema
	providerRegistry SecretProviderRegistry
}

func NewResolver(store ConfigStore, schema *ConfigSchema, registry SecretProviderRegistry) *Resolver {
	return &Resolver{
		store:            store,
		schema:           schema,
		providerRegistry: registry,
	}
}

func (r *Resolver) Resolve(ctx context.Context, pluginID, runtimeID, serviceID string) (*ScopedConfig, []ValidationError) {
	if r.schema == nil {
		return nil, nil
	}

	blobs := r.loadAllBlobs(ctx, pluginID, runtimeID, serviceID)
	resolved := r.mergeBlobs(blobs)

	appliedKeys := make(map[string]bool)
	for k := range resolved {
		appliedKeys[k] = true
	}

	r.applySchemaDefaults(resolved, appliedKeys)

	entryList := sortEntries(resolved)

	allErrors := r.validateAll(entryList)

	revision := ComputeRevision(entryList)

	cfg := &ScopedConfig{
		PluginID:   pluginID,
		RuntimeID:  runtimeID,
		ServiceID:  serviceID,
		Entries:    entryList,
		Revision:   revision,
		CompiledAt: time.Now().UTC(),
	}

	return cfg, allErrors
}

func (r *Resolver) loadAllBlobs(ctx context.Context, pluginID, runtimeID, serviceID string) []ConfigBlob {
	var blobs []ConfigBlob
	if r.store == nil {
		return blobs
	}

	if serviceID != "" && runtimeID != "" {
		blob, err := r.store.LoadServiceConfig(ctx, runtimeID, serviceID)
		if err == nil && blob != nil {
			blobs = append(blobs, *blob)
		}
	}

	if runtimeID != "" {
		blob, err := r.store.LoadRuntimeConfig(ctx, runtimeID)
		if err == nil && blob != nil {
			blobs = append(blobs, *blob)
		}
	}

	if pluginID != "" {
		blob, err := r.store.LoadPluginConfig(ctx, pluginID)
		if err == nil && blob != nil {
			blobs = append(blobs, *blob)
		}
	}

	sort.Slice(blobs, func(i, j int) bool {
		return ScopePriority(blobs[i].Scope) > ScopePriority(blobs[j].Scope)
	})

	return blobs
}

func (r *Resolver) mergeBlobs(blobs []ConfigBlob) map[string]ConfigEntry {
	resolved := make(map[string]ConfigEntry)
	appliedKeys := make(map[string]bool)

	for _, blob := range blobs {
		for _, e := range blob.Entries {
			if appliedKeys[e.Key] {
				continue
			}
			entry := e
			entry.Scope = blob.Scope
			resolved[e.Key] = entry
			appliedKeys[e.Key] = true
		}
	}

	return resolved
}

func (r *Resolver) applySchemaDefaults(resolved map[string]ConfigEntry, appliedKeys map[string]bool) {
	if r.schema == nil {
		return
	}

	for _, field := range r.schema.Fields {
		if appliedKeys[field.Key] {
			continue
		}
		if len(field.Default) == 0 || string(field.Default) == "null" {
			continue
		}
		entry := ConfigEntry{
			Key:   field.Key,
			Value: field.Default,
			Scope: ConfigScopePlugin,
		}
		resolved[field.Key] = entry
		appliedKeys[field.Key] = true
	}
}

func (r *Resolver) validateAll(entryList []ConfigEntry) []ValidationError {
	var allErrors ValidationErrorList

	if r.schema == nil {
		return allErrors
	}

	validator := NewValidator(r.providerRegistry)

	for _, entry := range entryList {
		field, ok := r.schema.Field(entry.Key)
		if !ok {
			continue
		}

		if entry.SecretRef != nil {
			if errs := validator.ValidateSecretRef(entry.Key, entry.SecretRef, field); len(errs) > 0 {
				allErrors = append(allErrors, errs...)
			}
		} else if errs := validator.ValidateValue(entry.Key, entry.Value, field, entry.Scope); len(errs) > 0 {
			allErrors = append(allErrors, errs...)
		}
	}

	return allErrors
}

func sortEntries(resolved map[string]ConfigEntry) []ConfigEntry {
	entryList := make([]ConfigEntry, 0, len(resolved))
	for _, e := range resolved {
		entryList = append(entryList, e)
	}

	sort.SliceStable(entryList, func(i, j int) bool {
		return entryList[i].Key < entryList[j].Key
	})

	return entryList
}
