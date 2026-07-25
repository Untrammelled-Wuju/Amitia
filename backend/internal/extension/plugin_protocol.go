// Deprecated: Legacy extension architecture.
// Do not add new capabilities. This implementation is retained only for
// compatibility, maintenance, testing, and migration to Extension Kernel.

package extension

import (
	"context"
	"encoding/json"
	"time"
)

type PluginHook string

const (
	HookOnLoad       PluginHook = "on_load"
	HookOnEnable     PluginHook = "on_enable"
	HookBeforePrompt PluginHook = "before_prompt"
	HookAfterReply   PluginHook = "after_reply"
	HookOnEvent      PluginHook = "on_event"
	HookOnSchedule   PluginHook = "on_schedule"
	HookOnDisable    PluginHook = "on_disable"
	HookOnUnload     PluginHook = "on_unload"
)

type PluginLifecycleStatus string

const (
	PluginRegistered  PluginLifecycleStatus = "registered"
	PluginLoaded      PluginLifecycleStatus = "loaded"
	PluginEnabled     PluginLifecycleStatus = "enabled"
	PluginDisabled    PluginLifecycleStatus = "disabled"
	PluginError       PluginLifecycleStatus = "error"
	PluginCircuitOpen PluginLifecycleStatus = "circuit_open"
	PluginUnloading   PluginLifecycleStatus = "unloading"
	PluginUnloaded    PluginLifecycleStatus = "unloaded"
)

type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

const (
	ErrPluginNotFound           = "PLUGIN_NOT_FOUND"
	ErrPluginDisabled           = "PLUGIN_DISABLED"
	ErrPluginIncompatible       = "PLUGIN_INCOMPATIBLE"
	ErrPluginManifestInvalid    = "PLUGIN_MANIFEST_INVALID"
	ErrPluginLoadFailed         = "PLUGIN_LOAD_FAILED"
	ErrPluginEnableFailed       = "PLUGIN_ENABLE_FAILED"
	ErrPluginDisableFailed      = "PLUGIN_DISABLE_FAILED"
	ErrPluginHookTimeout        = "PLUGIN_HOOK_TIMEOUT"
	ErrPluginHookFailed         = "PLUGIN_HOOK_FAILED"
	ErrPluginCircuitOpen        = "PLUGIN_CIRCUIT_OPEN"
	ErrPluginStateInvalid       = "PLUGIN_STATE_INVALID"
	ErrPluginStateConflict      = "PLUGIN_STATE_CONFLICT"
	ErrPluginStateMigration     = "PLUGIN_STATE_MIGRATION_FAILED"
	ErrPluginConfigInvalid      = "PLUGIN_CONFIG_INVALID"
	ErrPluginEventInvalid       = "PLUGIN_EVENT_INVALID"
	ErrPluginEventDepthExceeded = "PLUGIN_EVENT_DEPTH_EXCEEDED"
	ErrPluginEventDeadLetter    = "PLUGIN_EVENT_DEAD_LETTER"
	ErrPluginScheduleInvalid    = "PLUGIN_SCHEDULE_INVALID"
	ErrPluginSurfaceInvalid     = "PLUGIN_SURFACE_INVALID"
	ErrPluginActionNotAllowed   = "PLUGIN_ACTION_NOT_ALLOWED"
)

type PluginManifest struct {
	Schema           string                `json:"$schema"`
	APIVersion       string                `json:"apiVersion"`
	Kind             string                `json:"kind"`
	Metadata         ManifestMetadata      `json:"metadata"`
	Compatibility    ManifestCompatibility `json:"compatibility"`
	Entry            SkillEntry            `json:"entry"`
	Capabilities     []string              `json:"capabilities"`
	Hooks            []PluginHook          `json:"hooks"`
	Subscriptions    []string              `json:"subscriptions"`
	RegisteredSkills []string              `json:"registeredSkills"`
	Execution        PluginExecution       `json:"execution"`
	ConfigSchema     json.RawMessage       `json:"configSchema,omitempty"`
	DefaultConfig    json.RawMessage       `json:"defaultConfig,omitempty"`
	State            PluginStateManifest   `json:"state"`
	Surface          json.RawMessage       `json:"surface,omitempty"`
	Enabled          bool                  `json:"enabled"`
}

type PluginExecution struct {
	HookTimeoutMS      int64 `json:"hookTimeoutMs"`
	MaxConcurrency     int   `json:"maxConcurrency"`
	FailureThreshold   int   `json:"failureThreshold"`
	CircuitOpenMS      int64 `json:"circuitOpenMs"`
	HalfOpenMaxRequest int   `json:"halfOpenMaxRequests,omitempty"`
}

type PluginStateManifest struct {
	SchemaVersion string          `json:"schemaVersion"`
	Schema        json.RawMessage `json:"schema,omitempty"`
	Default       json.RawMessage `json:"default,omitempty"`
}

type Plugin interface {
	Manifest() PluginManifest
}

type PluginFactory func() Plugin

type LoadHook interface {
	OnLoad(context.Context, PluginHost) error
}

type EnableHook interface {
	OnEnable(context.Context) error
}

type BeforePromptHook interface {
	BeforePrompt(context.Context, ExtensionSnapshot) ([]ContextContribution, error)
}

type AfterReplyHook interface {
	AfterReply(context.Context, ExtensionSnapshot, ReplyView) error
}

type EventHook interface {
	OnEvent(context.Context, ExtensionEvent) error
}

type ScheduleHook interface {
	OnSchedule(context.Context, PluginScheduleInvocation) error
}

type DisableHook interface {
	OnDisable(context.Context) error
}

type UnloadHook interface {
	OnUnload(context.Context) error
}

type StateMigrator interface {
	CurrentVersion() string
	Migrate(context.Context, string, json.RawMessage) (string, json.RawMessage, error)
}

type PluginStateScope struct {
	Type ScopeType `json:"type"`
	ID   string    `json:"id"`
}

type PluginState struct {
	PluginID      string          `json:"pluginId"`
	ScopeType     ScopeType       `json:"scopeType"`
	ScopeID       string          `json:"scopeId"`
	SchemaVersion string          `json:"schemaVersion"`
	Revision      int64           `json:"revision"`
	Data          json.RawMessage `json:"data"`
	UpdatedAt     string          `json:"updatedAt"`
}

type WritePluginStateRequest struct {
	Scope            PluginStateScope `json:"scope"`
	ExpectedRevision int64            `json:"expectedRevision"`
	Data             json.RawMessage  `json:"data"`
}

type ExtensionSnapshot struct {
	SchemaVersion string          `json:"schemaVersion"`
	User          SnapshotUser    `json:"user"`
	Character     SnapshotEntity  `json:"character"`
	Conversation  SnapshotEntity  `json:"conversation"`
	Channel       SnapshotChannel `json:"channel"`
	Relationship  map[string]any  `json:"relationship,omitempty"`
	Emotion       map[string]any  `json:"emotion,omitempty"`
	Life          map[string]any  `json:"life,omitempty"`
	CapturedAt    time.Time       `json:"capturedAt"`
}

type SnapshotUser struct {
	ID string `json:"id"`
}

type SnapshotEntity struct {
	ID string `json:"id"`
}

type SnapshotChannel struct {
	Name string `json:"name"`
}

type ContextContribution struct {
	Source     string            `json:"source"`
	Priority   int               `json:"priority"`
	Content    string            `json:"content"`
	TokenLimit int               `json:"tokenLimit"`
	ExpiresAt  *time.Time        `json:"expiresAt,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type ReplyView struct {
	MessageID      string    `json:"messageId"`
	CharacterID    string    `json:"characterId"`
	ConversationID string    `json:"conversationId"`
	Channel        string    `json:"channel"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"createdAt"`
}

type ExtensionEvent struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	Subject         string          `json:"subject,omitempty"`
	Time            time.Time       `json:"time"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
	TraceID         string          `json:"traceId,omitempty"`
	CorrelationID   string          `json:"correlationId,omitempty"`
	CausationID     string          `json:"causationId,omitempty"`
	Depth           int             `json:"depth"`
}

type PluginScheduleDefinition struct {
	ScheduleID string           `json:"scheduleId"`
	Scope      PluginStateScope `json:"scope"`
	Type       string           `json:"type"`
	Expression string           `json:"expression"`
	Timezone   string           `json:"timezone"`
	Payload    json.RawMessage  `json:"payload"`
	Enabled    bool             `json:"enabled"`
	NextRunAt  string           `json:"nextRunAt,omitempty"`
}

type PluginScheduleInvocation struct {
	PluginID     string           `json:"pluginId"`
	ScheduleID   string           `json:"scheduleId"`
	InvocationID string           `json:"invocationId"`
	Scope        PluginStateScope `json:"scope"`
	Payload      json.RawMessage  `json:"payload"`
	TriggeredAt  time.Time        `json:"triggeredAt"`
}

type SurfaceDocument struct {
	Schema   string           `json:"$schema"`
	Version  string           `json:"version"`
	Title    string           `json:"title"`
	Sections []SurfaceSection `json:"sections"`
}

type SurfaceSection struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Title   string         `json:"title,omitempty"`
	Label   string         `json:"label,omitempty"`
	Source  string         `json:"source,omitempty"`
	Skill   string         `json:"skill,omitempty"`
	Fields  []SurfaceField `json:"fields,omitempty"`
	Columns []SurfaceField `json:"columns,omitempty"`
}

type SurfaceField struct {
	Key       string   `json:"key"`
	Label     string   `json:"label"`
	Component string   `json:"component"`
	Minimum   *float64 `json:"minimum,omitempty"`
	Maximum   *float64 `json:"maximum,omitempty"`
	Required  bool     `json:"required,omitempty"`
	Options   []string `json:"options,omitempty"`
}

type PluginHealth struct {
	PluginID      string                     `json:"pluginId"`
	Lifecycle     PluginLifecycleStatus      `json:"lifecycle"`
	Health        string                     `json:"health"`
	Compatible    bool                       `json:"compatible"`
	LastErrorCode string                     `json:"lastErrorCode,omitempty"`
	LastErrorAt   string                     `json:"lastErrorAt,omitempty"`
	Circuits      map[PluginHook]CircuitView `json:"circuits"`
}

type CircuitView struct {
	State       CircuitState `json:"state"`
	Failures    int          `json:"failures"`
	OpenedAt    string       `json:"openedAt,omitempty"`
	NextProbeAt string       `json:"nextProbeAt,omitempty"`
}

type PluginRunView struct {
	RunID          string       `json:"runId"`
	PluginID       string       `json:"pluginId"`
	PluginVersion  string       `json:"pluginVersion"`
	Hook           PluginHook   `json:"hook"`
	CharacterID    string       `json:"characterId,omitempty"`
	ConversationID string       `json:"conversationId,omitempty"`
	Channel        string       `json:"channel,omitempty"`
	Status         string       `json:"status"`
	DurationMS     int64        `json:"durationMs"`
	ErrorCode      string       `json:"errorCode,omitempty"`
	TraceID        string       `json:"traceId,omitempty"`
	CircuitState   CircuitState `json:"circuitState"`
	CreatedAt      string       `json:"createdAt"`
}

type PluginView struct {
	Manifest        PluginManifest        `json:"manifest"`
	Source          string                `json:"source"`
	Lifecycle       PluginLifecycleStatus `json:"lifecycle"`
	Health          string                `json:"health"`
	Compatible      bool                  `json:"compatible"`
	Enabled         bool                  `json:"enabled"`
	CurrentCircuits int                   `json:"currentCircuits"`
	LastErrorCode   string                `json:"lastErrorCode,omitempty"`
	LastErrorAt     string                `json:"lastErrorAt,omitempty"`
}

type PluginDetailView struct {
	PluginView
	Permissions  []PermissionGrantView      `json:"permissions"`
	Config       json.RawMessage            `json:"config"`
	States       []PluginState              `json:"states"`
	Schedules    []PluginScheduleDefinition `json:"schedules"`
	RecentRuns   []PluginRunView            `json:"recentRuns"`
	RecentEvents []ExtensionEvent           `json:"recentEvents"`
}

type PluginPage struct {
	Items    []PluginView `json:"items"`
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
}

type PluginEventPage struct {
	Items    []ExtensionEvent `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
}

type PluginHost interface {
	PluginID() string
	RegisterSkill(context.Context, SkillDefinition, SkillHandler) error
	CallSkill(context.Context, ExecuteSkillRequest) (SkillResult, error)
	ReadSnapshot(context.Context, ExecutionScope) (ExtensionSnapshot, error)
	ReadConfig(context.Context, PluginStateScope) (json.RawMessage, error)
	ReadState(context.Context, PluginStateScope) (PluginState, error)
	WriteState(context.Context, WritePluginStateRequest) (PluginState, error)
	EmitEvent(context.Context, ExtensionEvent) error
	RegisterSchedule(context.Context, PluginScheduleDefinition) error
	RemoveSchedule(context.Context, string) error
	Logger() PluginLogger
	Tracer() PluginTracer
}

type PluginLogger interface {
	Info(context.Context, string, map[string]any)
	Warn(context.Context, string, map[string]any)
	Error(context.Context, string, map[string]any)
}

type PluginTracer interface {
	Start(context.Context, string) (context.Context, func(error))
}
