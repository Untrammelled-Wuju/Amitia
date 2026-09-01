package workflow

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type WorkflowTriggerEvent struct {
	EventID     string          `json:"eventId"`
	EventType   string          `json:"eventType"`
	Source      string          `json:"source"`
	OwnerUserID string          `json:"ownerUserId"`
	DeviceID    string          `json:"deviceId"`
	OccurredAt  time.Time       `json:"occurredAt"`
	Payload     json.RawMessage `json:"payload"`
}

type WorkflowEventMatchResult struct {
	Matched      bool
	Payload      json.RawMessage
	DedupEventID string
}

type WorkflowTriggerSecretResolver func(context.Context, string, WorkflowTriggerEvent, TriggerBinding) ([]byte, error)

type WorkflowEventMatcher interface {
	Match(context.Context, WorkflowTriggerEvent, TriggerBinding, WorkflowTriggerSecretResolver) (WorkflowEventMatchResult, error)
}

type WorkflowEventMatcherRegistry struct {
	mu       sync.RWMutex
	matchers map[string]WorkflowEventMatcher
	fallback WorkflowEventMatcher
}

func NewWorkflowEventMatcherRegistry() *WorkflowEventMatcherRegistry {
	registry := &WorkflowEventMatcherRegistry{
		matchers: make(map[string]WorkflowEventMatcher),
		fallback: passthroughWorkflowEventMatcher{},
	}
	registry.Register("device.android.intent", androidIntentWorkflowEventMatcher{})
	registry.Register("device.android.tasker", taskerWorkflowEventMatcher{})
	registry.Register("voice.wake.detected", voiceWakeWorkflowEventMatcher{})
	registry.Register("voice.asr.final", voicePhraseWorkflowEventMatcher{})
	registry.Register("device.app.foreground", newAppForegroundWorkflowEventMatcher())
	return registry
}

func (r *WorkflowEventMatcherRegistry) Register(eventType string, matcher WorkflowEventMatcher) {
	if r == nil || matcher == nil {
		return
	}
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		return
	}
	r.mu.Lock()
	r.matchers[eventType] = matcher
	r.mu.Unlock()
}

func (r *WorkflowEventMatcherRegistry) Match(ctx context.Context, event WorkflowTriggerEvent, binding TriggerBinding, resolver WorkflowTriggerSecretResolver) (WorkflowEventMatchResult, error) {
	if r == nil {
		return WorkflowEventMatchResult{Matched: true, Payload: event.Payload}, nil
	}
	eventType := canonicalWorkflowEventType(event)
	r.mu.RLock()
	matcher := r.matchers[eventType]
	fallback := r.fallback
	r.mu.RUnlock()
	if matcher == nil {
		matcher = fallback
	}
	if matcher == nil {
		return WorkflowEventMatchResult{Matched: true, Payload: event.Payload}, nil
	}
	return matcher.Match(ctx, event, binding, resolver)
}

func TriggerSecretNamespace(userID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(userID)))
	return fmt.Sprintf("workflow-trigger-%x", sum[:16])
}

func TriggerSecretRefOwnedByUser(ref, userID string) bool {
	namespace := TriggerSecretNamespace(userID)
	prefix := "secret://" + namespace + "/"
	trimmed := strings.TrimSpace(ref)
	if !strings.HasPrefix(trimmed, prefix) {
		return false
	}
	suffix := strings.TrimPrefix(trimmed, prefix)
	return suffix != "" && !strings.Contains(suffix, "/")
}

func canonicalWorkflowEventType(event WorkflowTriggerEvent) string {
	eventType := strings.TrimSpace(event.EventType)
	if event.OwnerUserID != "" {
		prefix := "user:" + event.OwnerUserID + ":"
		if strings.HasPrefix(eventType, prefix) {
			return strings.TrimPrefix(eventType, prefix)
		}
	}
	return eventType
}

type workflowEventMatchRollbacker interface {
	Rollback(WorkflowTriggerEvent, TriggerBinding)
}

func (r *WorkflowEventMatcherRegistry) Rollback(event WorkflowTriggerEvent, binding TriggerBinding) {
	if r == nil {
		return
	}
	eventType := canonicalWorkflowEventType(event)
	r.mu.RLock()
	matcher := r.matchers[eventType]
	r.mu.RUnlock()
	rollbacker, ok := matcher.(workflowEventMatchRollbacker)
	if !ok {
		return
	}
	rollbacker.Rollback(event, binding)
}

type passthroughWorkflowEventMatcher struct{}

func (passthroughWorkflowEventMatcher) Match(_ context.Context, event WorkflowTriggerEvent, _ TriggerBinding, _ WorkflowTriggerSecretResolver) (WorkflowEventMatchResult, error) {
	return WorkflowEventMatchResult{Matched: true, Payload: event.Payload}, nil
}

type androidIntentWorkflowEventMatcher struct{}

type androidIntentMatcherConfig struct {
	Actions       []string `json:"actions"`
	Categories    []string `json:"categories"`
	DataSchemes   []string `json:"dataSchemes"`
	MimeTypes     []string `json:"mimeTypes"`
	DedupWindowMS int64    `json:"dedupWindowMs"`
}

type androidIntentEventPayload struct {
	Action     string   `json:"action"`
	Categories []string `json:"categories"`
	DataScheme string   `json:"dataScheme"`
	MimeType   string   `json:"mimeType"`
}

func (androidIntentWorkflowEventMatcher) Match(_ context.Context, event WorkflowTriggerEvent, binding TriggerBinding, _ WorkflowTriggerSecretResolver) (WorkflowEventMatchResult, error) {
	var cfg androidIntentMatcherConfig
	if err := unmarshalTriggerConfig(binding.Config, &cfg); err != nil {
		return WorkflowEventMatchResult{}, err
	}
	var payload androidIntentEventPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return WorkflowEventMatchResult{}, fmt.Errorf("workflow intent event payload: %w", err)
	}
	if len(cfg.Actions) > 0 && !containsExact(cfg.Actions, payload.Action) {
		return WorkflowEventMatchResult{}, nil
	}
	if len(cfg.Categories) > 0 && !containsAll(payload.Categories, cfg.Categories) {
		return WorkflowEventMatchResult{}, nil
	}
	if len(cfg.DataSchemes) > 0 && !containsExact(cfg.DataSchemes, payload.DataScheme) {
		return WorkflowEventMatchResult{}, nil
	}
	if len(cfg.MimeTypes) > 0 && !matchesMimeTypeList(cfg.MimeTypes, payload.MimeType) {
		return WorkflowEventMatchResult{}, nil
	}
	result := WorkflowEventMatchResult{Matched: true, Payload: event.Payload}
	if cfg.DedupWindowMS > 0 {
		occurredAt := event.OccurredAt
		if occurredAt.IsZero() {
			occurredAt = time.Now().UTC()
		}
		bucket := occurredAt.UnixMilli() / cfg.DedupWindowMS
		sum := sha256.Sum256(event.Payload)
		result.DedupEventID = fmt.Sprintf("intent:%x:%d", sum[:12], bucket)
	}
	return result, nil
}

type taskerWorkflowEventMatcher struct{}

type taskerMatcherConfig struct {
	EventName        string   `json:"eventName"`
	SecretRef        string   `json:"secretRef"`
	AllowedVariables []string `json:"allowedVariables"`
}

type taskerEventPayload struct {
	EventName string         `json:"eventName"`
	Secret    string         `json:"secret"`
	Variables map[string]any `json:"variables"`
	EventID   string         `json:"eventId,omitempty"`
}

func (taskerWorkflowEventMatcher) Match(ctx context.Context, event WorkflowTriggerEvent, binding TriggerBinding, resolver WorkflowTriggerSecretResolver) (WorkflowEventMatchResult, error) {
	var cfg taskerMatcherConfig
	if err := unmarshalTriggerConfig(binding.Config, &cfg); err != nil {
		return WorkflowEventMatchResult{}, err
	}
	var payload taskerEventPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return WorkflowEventMatchResult{}, fmt.Errorf("workflow tasker event payload: %w", err)
	}
	if strings.TrimSpace(cfg.EventName) == "" || payload.EventName != cfg.EventName {
		return WorkflowEventMatchResult{}, nil
	}
	if strings.TrimSpace(cfg.SecretRef) == "" || resolver == nil {
		return WorkflowEventMatchResult{}, nil
	}
	expected, err := resolver(ctx, cfg.SecretRef, event, binding)
	if err != nil {
		return WorkflowEventMatchResult{}, err
	}
	matched := subtle.ConstantTimeCompare([]byte(payload.Secret), expected) == 1
	for i := range expected {
		expected[i] = 0
	}
	if !matched {
		return WorkflowEventMatchResult{}, nil
	}
	filtered := make(map[string]any)
	allowed := make(map[string]struct{}, len(cfg.AllowedVariables))
	for _, key := range cfg.AllowedVariables {
		allowed[strings.TrimSpace(key)] = struct{}{}
	}
	for key, value := range payload.Variables {
		if _, ok := allowed[key]; ok {
			filtered[key] = value
		}
	}
	safePayload, err := json.Marshal(map[string]any{
		"eventName": payload.EventName,
		"variables": filtered,
		"eventId":   event.EventID,
	})
	if err != nil {
		return WorkflowEventMatchResult{}, err
	}
	return WorkflowEventMatchResult{Matched: true, Payload: safePayload}, nil
}

type voiceWakeWorkflowEventMatcher struct{}

type voiceWakeMatcherConfig struct {
	Mode         string `json:"mode"`
	WakeConfigID string `json:"wakeConfigId"`
}

func (voiceWakeWorkflowEventMatcher) Match(_ context.Context, event WorkflowTriggerEvent, binding TriggerBinding, _ WorkflowTriggerSecretResolver) (WorkflowEventMatchResult, error) {
	var cfg voiceWakeMatcherConfig
	if err := unmarshalTriggerConfig(binding.Config, &cfg); err != nil {
		return WorkflowEventMatchResult{}, err
	}
	cfg.WakeConfigID = strings.TrimSpace(cfg.WakeConfigID)
	if cfg.WakeConfigID == "" {
		return WorkflowEventMatchResult{}, errors.New("workflow wake trigger wakeConfigId is required")
	}
	var payload struct {
		WakeConfigID string `json:"wakeConfigId"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return WorkflowEventMatchResult{}, fmt.Errorf("workflow wake event payload: %w", err)
	}
	if payload.WakeConfigID != cfg.WakeConfigID {
		return WorkflowEventMatchResult{}, nil
	}
	return WorkflowEventMatchResult{Matched: true, Payload: event.Payload}, nil
}

type voicePhraseWorkflowEventMatcher struct{}

type voicePhraseMatcherConfig struct {
	Mode      string   `json:"mode"`
	Phrases   []string `json:"phrases"`
	MatchMode string   `json:"matchMode"`
}

func (voicePhraseWorkflowEventMatcher) Match(_ context.Context, event WorkflowTriggerEvent, binding TriggerBinding, _ WorkflowTriggerSecretResolver) (WorkflowEventMatchResult, error) {
	var cfg voicePhraseMatcherConfig
	if err := unmarshalTriggerConfig(binding.Config, &cfg); err != nil {
		return WorkflowEventMatchResult{}, err
	}
	var payload map[string]any
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return WorkflowEventMatchResult{}, fmt.Errorf("workflow voice phrase payload: %w", err)
	}
	transcript := firstString(payload, "transcript", "text", "finalTranscript")
	if transcript == "" || len(cfg.Phrases) == 0 {
		return WorkflowEventMatchResult{}, nil
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.MatchMode))
	for _, phrase := range cfg.Phrases {
		if mode == "exact" {
			if transcript == phrase {
				return WorkflowEventMatchResult{Matched: true, Payload: event.Payload}, nil
			}
			continue
		}
		if normalizePhrase(transcript) == normalizePhrase(phrase) {
			return WorkflowEventMatchResult{Matched: true, Payload: event.Payload}, nil
		}
	}
	return WorkflowEventMatchResult{}, nil
}

type appForegroundWorkflowEventMatcher struct {
	mu   sync.Mutex
	last map[string]time.Time
}

type appForegroundMatcherConfig struct {
	Packages   []string `json:"packages"`
	CooldownMS int64    `json:"cooldownMs"`
}

func newAppForegroundWorkflowEventMatcher() *appForegroundWorkflowEventMatcher {
	return &appForegroundWorkflowEventMatcher{last: make(map[string]time.Time)}
}

func (m *appForegroundWorkflowEventMatcher) Match(_ context.Context, event WorkflowTriggerEvent, binding TriggerBinding, _ WorkflowTriggerSecretResolver) (WorkflowEventMatchResult, error) {
	var cfg appForegroundMatcherConfig
	if err := unmarshalTriggerConfig(binding.Config, &cfg); err != nil {
		return WorkflowEventMatchResult{}, err
	}
	var payload map[string]any
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return WorkflowEventMatchResult{}, fmt.Errorf("workflow app foreground payload: %w", err)
	}
	current := firstString(payload, "packageName", "foregroundPackageName", "currentPackage")
	previous := firstString(payload, "previousPackageName", "previousPackage")
	if current == "" || current == previous || (len(cfg.Packages) > 0 && !containsExact(cfg.Packages, current)) {
		return WorkflowEventMatchResult{}, nil
	}
	if cfg.CooldownMS > 0 {
		occurredAt := event.OccurredAt
		if occurredAt.IsZero() {
			occurredAt = time.Now()
		}
		m.mu.Lock()
		last := m.last[binding.BindingID]
		if !last.IsZero() && occurredAt.Sub(last) < time.Duration(cfg.CooldownMS)*time.Millisecond {
			m.mu.Unlock()
			return WorkflowEventMatchResult{}, nil
		}
		m.last[binding.BindingID] = occurredAt
		m.mu.Unlock()
	}
	return WorkflowEventMatchResult{Matched: true, Payload: event.Payload}, nil
}

func (m *appForegroundWorkflowEventMatcher) Rollback(event WorkflowTriggerEvent, binding TriggerBinding) {
	if m == nil {
		return
	}
	var cfg appForegroundMatcherConfig
	if unmarshalTriggerConfig(binding.Config, &cfg) != nil || cfg.CooldownMS <= 0 {
		return
	}
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		return
	}
	m.mu.Lock()
	if last := m.last[binding.BindingID]; last.Equal(occurredAt) {
		delete(m.last, binding.BindingID)
	}
	m.mu.Unlock()
}

func unmarshalTriggerConfig(raw json.RawMessage, target any) error {
	if len(raw) == 0 || string(raw) == "null" {
		raw = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("workflow trigger config: %w", err)
	}
	return nil
}

func containsExact(items []string, target string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}

func containsAll(haystack, needles []string) bool {
	set := make(map[string]struct{}, len(haystack))
	for _, item := range haystack {
		set[item] = struct{}{}
	}
	for _, item := range needles {
		if _, ok := set[strings.TrimSpace(item)]; !ok {
			return false
		}
	}
	return true
}

func matchesMimeTypeList(patterns []string, mimeType string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == mimeType || pattern == "*/*" {
			return true
		}
		if strings.HasSuffix(pattern, "/*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(mimeType, prefix) {
				return true
			}
		}
	}
	return false
}

func normalizePhrase(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func firstString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key].(string); ok {
			if value = strings.TrimSpace(value); value != "" {
				return value
			}
		}
	}
	return ""
}
