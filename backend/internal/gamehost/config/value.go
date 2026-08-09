package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"
)

type SecretRef struct {
	Provider string `json:"provider"`
	Key      string `json:"key"`
}

type ConfigEntry struct {
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
	SecretRef *SecretRef      `json:"secretRef,omitempty"`
	Scope     ConfigScope     `json:"scope"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type ScopedConfig struct {
	PluginID   string        `json:"pluginID"`
	RuntimeID  string        `json:"runtimeID,omitempty"`
	ServiceID  string        `json:"serviceID,omitempty"`
	Entries    []ConfigEntry `json:"entries"`
	Revision   string        `json:"revision"`
	CompiledAt time.Time     `json:"compiledAt"`
}

type revisionEntry struct {
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value,omitempty"`
	SecretRef *SecretRef      `json:"secretRef,omitempty"`
	Scope     ConfigScope     `json:"scope"`
}

func ComputeRevision(entries []ConfigEntry) string {
	sorted := make([]revisionEntry, 0, len(entries))
	for _, e := range entries {
		sorted = append(sorted, revisionEntry{
			Key:       e.Key,
			Value:     e.Value,
			SecretRef: e.SecretRef,
			Scope:     e.Scope,
		})
	}

	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Key != sorted[j].Key {
			return sorted[i].Key < sorted[j].Key
		}
		return sorted[i].Scope < sorted[j].Scope
	})

	h := sha256.New()
	enc := json.NewEncoder(h)
	enc.SetEscapeHTML(false)
	enc.Encode(sorted)

	sum := h.Sum(nil)
	return "crev-" + hex.EncodeToString(sum)
}

func (sc *ScopedConfig) SecretKeys() []string {
	var keys []string
	for _, e := range sc.Entries {
		if e.SecretRef != nil && e.SecretRef.Provider != "" {
			keys = append(keys, e.Key)
		}
	}
	return keys
}

func (sc *ScopedConfig) HasSecrets() bool {
	for _, e := range sc.Entries {
		if e.SecretRef != nil && e.SecretRef.Provider != "" {
			return true
		}
	}
	return false
}
