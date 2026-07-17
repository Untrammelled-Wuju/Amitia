package extension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type SkillSource string

const (
	SkillSourceBuiltin  SkillSource = "builtin"
	SkillSourceLegacy   SkillSource = "legacy_tool"
	SkillSourceWorkflow SkillSource = "workflow"
)

type SkillTrigger string

const (
	TriggerLLM         SkillTrigger = "llm"
	TriggerManual      SkillTrigger = "manual"
	TriggerSchedule    SkillTrigger = "schedule"
	TriggerSystemEvent SkillTrigger = "system_event"
)

type RunStatus string

const (
	RunPending            RunStatus = "pending"
	RunRunning            RunStatus = "running"
	RunSucceeded          RunStatus = "succeeded"
	RunFailed             RunStatus = "failed"
	RunCancelled          RunStatus = "cancelled"
	RunTimedOut           RunStatus = "timed_out"
	RunPartiallySucceeded RunStatus = "partially_succeeded"
)

type PermissionDecision string

const (
	DecisionDeny           PermissionDecision = "deny"
	DecisionAllowOnce      PermissionDecision = "allow_once"
	DecisionAllowSession   PermissionDecision = "allow_session"
	DecisionAllowCharacter PermissionDecision = "allow_character"
	DecisionAllowAlways    PermissionDecision = "allow_always"
)

type ScopeType string

const (
	ScopeGlobal       ScopeType = "global"
	ScopeCharacter    ScopeType = "character"
	ScopeConversation ScopeType = "conversation"
	ScopeChannel      ScopeType = "channel"
	ScopeSession      ScopeType = "session"
)

const (
	ErrSkillNotFound            = "SKILL_NOT_FOUND"
	ErrSkillDisabled            = "SKILL_DISABLED"
	ErrSkillIncompatible        = "SKILL_INCOMPATIBLE"
	ErrSkillTriggerNotAllowed   = "SKILL_TRIGGER_NOT_ALLOWED"
	ErrSkillInputInvalid        = "SKILL_INPUT_INVALID"
	ErrSkillOutputInvalid       = "SKILL_OUTPUT_INVALID"
	ErrSkillPermissionDenied    = "SKILL_PERMISSION_DENIED"
	ErrSkillTimeout             = "SKILL_TIMEOUT"
	ErrSkillCancelled           = "SKILL_CANCELLED"
	ErrSkillExecutionFailed     = "SKILL_EXECUTION_FAILED"
	ErrSkillDuplicateID         = "SKILL_DUPLICATE_ID"
	ErrSkillManifestInvalid     = "SKILL_MANIFEST_INVALID"
	ErrSkillIdempotencyConflict = "SKILL_IDEMPOTENCY_CONFLICT"
)

type Manifest struct {
	Schema        string                `json:"$schema"`
	APIVersion    string                `json:"apiVersion"`
	Kind          string                `json:"kind"`
	Metadata      ManifestMetadata      `json:"metadata"`
	Compatibility ManifestCompatibility `json:"compatibility"`
	Entry         SkillEntry            `json:"entry"`
	Capabilities  []string              `json:"capabilities"`
	Triggers      []SkillTrigger        `json:"triggers"`
	Execution     ManifestExecution     `json:"execution"`
	InputSchema   json.RawMessage       `json:"inputSchema"`
	OutputSchema  json.RawMessage       `json:"outputSchema"`
	ConfigSchema  json.RawMessage       `json:"configSchema,omitempty"`
	DefaultConfig json.RawMessage       `json:"defaultConfig,omitempty"`
	Enabled       bool                  `json:"enabled"`
	AllowLLM      bool                  `json:"allowLLM"`
	AllowManual   bool                  `json:"allowManual"`
}

type ManifestMetadata struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Author      string   `json:"author,omitempty"`
	License     string   `json:"license,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type ManifestCompatibility struct {
	EngineMin          string `json:"engineMin"`
	EngineMaxExclusive string `json:"engineMaxExclusive,omitempty"`
}

type SkillEntry struct {
	Kind       string `json:"kind"`
	Name       string `json:"name,omitempty"`
	ArtifactID string `json:"artifactId,omitempty"`
}

type ManifestExecution struct {
	TimeoutMS      int64 `json:"timeoutMs"`
	HasSideEffects bool  `json:"hasSideEffects"`
	Retryable      bool  `json:"retryable"`
	Idempotent     bool  `json:"idempotent"`
}

type SkillDefinition struct {
	ID                  string          `json:"id"`
	ModelName           string          `json:"modelName"`
	Name                string          `json:"name"`
	Description         string          `json:"description"`
	Version             string          `json:"version"`
	Source              SkillSource     `json:"source"`
	Entry               SkillEntry      `json:"entry"`
	InputSchema         json.RawMessage `json:"inputSchema"`
	OutputSchema        json.RawMessage `json:"outputSchema"`
	ConfigSchema        json.RawMessage `json:"configSchema,omitempty"`
	DefaultConfig       json.RawMessage `json:"defaultConfig,omitempty"`
	Capabilities        []string        `json:"capabilities"`
	Dependencies        []string        `json:"dependencies,omitempty"`
	Triggers            []SkillTrigger  `json:"triggers"`
	Timeout             time.Duration   `json:"-"`
	TimeoutMS           int64           `json:"timeoutMs"`
	HasSideEffects      bool            `json:"hasSideEffects"`
	Retryable           bool            `json:"retryable"`
	Idempotent          bool            `json:"idempotent"`
	Enabled             bool            `json:"enabled"`
	Compatible          bool            `json:"compatible"`
	CompatibilityReason string          `json:"compatibilityReason,omitempty"`
	Author              string          `json:"author,omitempty"`
	License             string          `json:"license,omitempty"`
	Manifest            json.RawMessage `json:"manifest"`
}

type ExecutionScope struct {
	UserID         string       `json:"userId"`
	CharacterID    string       `json:"characterId"`
	ConversationID string       `json:"conversationId"`
	Channel        string       `json:"channel"`
	SessionID      string       `json:"sessionId"`
	Trigger        SkillTrigger `json:"trigger"`
	TraceID        string       `json:"traceId"`
	RequestID      string       `json:"requestId"`
	ToolCallID     string       `json:"toolCallId"`
	CorrelationID  string       `json:"correlationId"`
	CausationID    string       `json:"causationId"`
}

type PermissionScope struct {
	Type ScopeType `json:"type"`
	ID   string    `json:"id"`
}

type ExtensionIdentity struct {
	ExtensionID string `json:"extensionId"`
	SkillID     string `json:"skillId"`
	Version     string `json:"version"`
}

type SideEffectRecord struct {
	Type      string `json:"type"`
	TargetID  string `json:"targetId,omitempty"`
	Confirmed bool   `json:"confirmed"`
}

type ExtensionError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Detail    string `json:"detail,omitempty"`
	Retryable bool   `json:"retryable"`
	Cause     error  `json:"-"`
}

func (e *ExtensionError) Error() string {
	if e == nil {
		return ""
	}
	if e.Detail != "" {
		return e.Message + ": " + e.Detail
	}
	return e.Message
}

func (e *ExtensionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewExtensionError(code, message, detail string, retryable bool, cause error) *ExtensionError {
	return &ExtensionError{Code: code, Message: message, Detail: detail, Retryable: retryable, Cause: cause}
}

type SkillResult struct {
	RunID       string             `json:"runId"`
	Status      RunStatus          `json:"status"`
	Output      json.RawMessage    `json:"output,omitempty"`
	SideEffects []SideEffectRecord `json:"sideEffects,omitempty"`
	Error       *ExtensionError    `json:"error,omitempty"`
	Duration    time.Duration      `json:"-"`
	DurationMS  int64              `json:"durationMs"`
	VisibleText string             `json:"visibleText,omitempty"`
	ForceVoice  bool               `json:"forceVoice,omitempty"`
}

type ExecuteSkillRequest struct {
	SkillID        string          `json:"skillId"`
	Input          json.RawMessage `json:"input"`
	Config         json.RawMessage `json:"config,omitempty"`
	Scope          ExecutionScope  `json:"scope"`
	IdempotencyKey string          `json:"idempotencyKey,omitempty"`
}

type SkillHandler func(context.Context, ExecuteSkillRequest) (SkillResult, error)

type RegisteredSkill struct {
	Definition SkillDefinition
	Handler    SkillHandler
}

type SkillFilter struct {
	Enabled *bool
	Trigger SkillTrigger
	Source  SkillSource
}

type RunFilter struct {
	SkillID     string
	Status      RunStatus
	CharacterID string
	Channel     string
	Trigger     SkillTrigger
	From        string
	To          string
	Page        int
	PageSize    int
}

type PermissionGrantInput struct {
	Capability string             `json:"capability"`
	Decision   PermissionDecision `json:"decision"`
	ScopeType  ScopeType          `json:"scopeType"`
	ScopeID    string             `json:"scopeId"`
	ExpiresAt  string             `json:"expiresAt,omitempty"`
}

type PermissionGrantView struct {
	ID          string             `json:"id"`
	Capability  string             `json:"capability"`
	Risk        string             `json:"risk"`
	Description string             `json:"description"`
	Decision    PermissionDecision `json:"decision"`
	ScopeType   ScopeType          `json:"scopeType"`
	ScopeID     string             `json:"scopeId"`
	ExpiresAt   string             `json:"expiresAt,omitempty"`
	ConsumedAt  string             `json:"consumedAt,omitempty"`
}

type RunView struct {
	RunID            string             `json:"runId"`
	ExtensionID      string             `json:"extensionId"`
	ExtensionVersion string             `json:"extensionVersion"`
	SkillID          string             `json:"skillId"`
	UserID           string             `json:"userId"`
	CharacterID      string             `json:"characterId"`
	ConversationID   string             `json:"conversationId"`
	Channel          string             `json:"channel"`
	Trigger          SkillTrigger       `json:"trigger"`
	Status           RunStatus          `json:"status"`
	InputSummary     string             `json:"inputSummary"`
	OutputSummary    string             `json:"outputSummary"`
	SideEffects      []SideEffectRecord `json:"sideEffects,omitempty"`
	IdempotencyKey   string             `json:"idempotencyKey,omitempty"`
	StartedAt        string             `json:"startedAt"`
	FinishedAt       string             `json:"finishedAt,omitempty"`
	DurationMS       int64              `json:"durationMs"`
	ErrorCode        string             `json:"errorCode,omitempty"`
	ErrorDetail      string             `json:"errorDetail,omitempty"`
	TraceID          string             `json:"traceId"`
}

type RunPage struct {
	Items    []RunView `json:"items"`
	Total    int64     `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
}

type SkillView struct {
	SkillDefinition
	LatestRun *RunView `json:"latestRun,omitempty"`
}

type SkillDetailView struct {
	SkillView
	Permissions []PermissionGrantView  `json:"permissions"`
	Config      json.RawMessage        `json:"config"`
	RecentRuns  []RunView              `json:"recentRuns"`
	Versions    []ExtensionVersionView `json:"versions"`
}

type ExtensionVersionView struct {
	Version   string          `json:"version"`
	Checksum  string          `json:"checksum"`
	Manifest  json.RawMessage `json:"manifest"`
	CreatedAt time.Time       `json:"createdAt"`
}

type ProblemDetail struct {
	Type     string       `json:"type"`
	Title    string       `json:"title"`
	Status   int          `json:"status"`
	Detail   string       `json:"detail"`
	Instance string       `json:"instance"`
	Code     string       `json:"code"`
	TraceID  string       `json:"traceId"`
	Result   *SkillResult `json:"result,omitempty"`
}

func hasTrigger(triggers []SkillTrigger, trigger SkillTrigger) bool {
	for _, item := range triggers {
		if item == trigger {
			return true
		}
	}
	return false
}

func normalizeJSON(input json.RawMessage) json.RawMessage {
	if len(input) == 0 {
		return json.RawMessage(`{}`)
	}
	return input
}

func compactSensitiveJSON(input json.RawMessage) string {
	if len(input) == 0 {
		return "{}"
	}
	var value interface{}
	if json.Unmarshal(input, &value) != nil {
		return "[invalid json]"
	}
	redactValue(value)
	out, _ := json.Marshal(value)
	if len(out) > 2048 {
		out = out[:2048]
	}
	return string(out)
}

func redactValue(value interface{}) {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, item := range typed {
			if isSensitiveKey(key) {
				typed[key] = "[REDACTED]"
				continue
			}
			redactValue(item)
		}
	case []interface{}:
		for _, item := range typed {
			redactValue(item)
		}
	}
}

func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	return strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "apikey") || strings.Contains(lower, "api_key") || strings.Contains(lower, "authorization") || strings.Contains(lower, "password") || strings.Contains(lower, "credential")
}

func hasPlaintextSecret(value interface{}) bool {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, item := range typed {
			if isSensitiveKey(key) {
				if text, ok := item.(string); !ok || (strings.TrimSpace(text) != "" && text != "[REDACTED]") {
					return true
				}
			}
			if hasPlaintextSecret(item) {
				return true
			}
		}
	case []interface{}:
		for _, item := range typed {
			if hasPlaintextSecret(item) {
				return true
			}
		}
	}
	return false
}

func redactJSON(input json.RawMessage) json.RawMessage {
	var value interface{}
	if json.Unmarshal(input, &value) != nil {
		return json.RawMessage(`{}`)
	}
	redactValue(value)
	output, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return output
}

func restoreRedactedValue(stored interface{}, incoming interface{}) interface{} {
	storedMap, storedOK := stored.(map[string]interface{})
	incomingMap, incomingOK := incoming.(map[string]interface{})
	if !storedOK || !incomingOK {
		return incoming
	}
	for key, value := range incomingMap {
		if value == "[REDACTED]" {
			if existing, ok := storedMap[key]; ok {
				incomingMap[key] = existing
			}
			continue
		}
		if existing, ok := storedMap[key]; ok {
			incomingMap[key] = restoreRedactedValue(existing, value)
		}
	}
	return incomingMap
}

func asExtensionError(err error) *ExtensionError {
	if err == nil {
		return nil
	}
	var extErr *ExtensionError
	if errors.As(err, &extErr) {
		return extErr
	}
	return NewExtensionError(ErrSkillExecutionFailed, "Skill execution failed", "", false, err)
}

func validateScopeID(scope PermissionScope) error {
	if scope.Type == ScopeGlobal {
		return nil
	}
	if strings.TrimSpace(scope.ID) == "" {
		return fmt.Errorf("scope id is required for %s", scope.Type)
	}
	return nil
}
