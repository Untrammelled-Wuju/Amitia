package extension

import (
	"encoding/json"
	"errors"
	"fmt"

	coreexec "github.com/u-ai/backend/internal/execution"
	"strings"

	coreexec "github.com/u-ai/backend/internal/execution"
)

type SkillTrigger string

const (
	TriggerLLM         SkillTrigger = "llm"
	TriggerManual      SkillTrigger = "manual"
	TriggerSchedule    SkillTrigger = "schedule"
	TriggerSystemEvent SkillTrigger = "system_event"
)

type ScopeType string

const (
	ScopeGlobal    ScopeType = "global"
	ScopeCharacter ScopeType = "character"
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
	ErrSkillNotExecutable       = "SKILL_NOT_EXECUTABLE"
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
	Path       string `json:"path,omitempty"`
}

type ManifestExecution struct {
	TimeoutMS      int64 `json:"timeoutMs"`
	HasSideEffects bool  `json:"hasSideEffects"`
	Retryable      bool  `json:"retryable"`
	Idempotent     bool  `json:"idempotent"`
}

type ExecutionScope struct {
	SpaceID          string                     `json:"spaceId"`
	CharacterID      string                     `json:"characterId"`
	ConversationID   string                     `json:"conversationId"`
	Channel          string                     `json:"channel"`
	SessionID        string                     `json:"sessionId"`
	Trigger          SkillTrigger               `json:"trigger"`
	TraceID          string                     `json:"traceId"`
	RequestID        string                     `json:"requestId"`
	ToolCallID       string                     `json:"toolCallId"`
	CorrelationID    string                     `json:"correlationId"`
	CausationID      string                     `json:"causationId"`
	ExtensionID      string                     `json:"extensionId,omitempty"`
	ExtensionVersion string                     `json:"extensionVersion,omitempty"`
	RunID            string                     `json:"runId,omitempty"`
	ExecContext      *coreexec.ExecutionContext `json:"-"`
}

type PermissionScope struct {
	Type ScopeType `json:"type"`
	ID   string    `json:"id"`
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

type ProblemDetail struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
	Code     string `json:"code"`
	TraceID  string `json:"traceId"`
}

func normalizeJSON(input json.RawMessage) json.RawMessage {
	if len(input) == 0 {
		return json.RawMessage(`{}`)
	}
	return input
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
