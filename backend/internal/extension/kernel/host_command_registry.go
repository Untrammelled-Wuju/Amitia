package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
)

const ErrCodeHostCommandNotFound = "HOST_COMMAND_NOT_FOUND"
const ErrCodeHostCommandInputInvalid = "HOST_COMMAND_INPUT_INVALID"
const ErrCodeHostCommandPermissionDenied = "HOST_COMMAND_PERMISSION_DENIED"

type HostCommandScope string

const (
	HostCommandScopeGlobal    HostCommandScope = "global"
	HostCommandScopeExtension HostCommandScope = "extension"
	HostCommandScopeModule    HostCommandScope = "module"
)

type HostCommandExecContext struct {
	ExtensionID          string
	ModuleID             string
	Generation           int64
	SessionID            string
	ScopeSnapshotID      string
	PermissionSnapshotID string
}

type HostCommandHandler func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error)

type HostCommandDefinition struct {
	CommandID   string
	Description string
	Permission  string
	Scope       HostCommandScope
	Risk        host_api.RiskLevel
	InputSchema json.RawMessage
	Handler     HostCommandHandler
}

type HostCommandError struct {
	Code    string
	Message string
	Cause   error
}

func (e *HostCommandError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *HostCommandError) Unwrap() error {
	return e.Cause
}

func NewHostCommandError(code, message string, cause error) *HostCommandError {
	return &HostCommandError{Code: code, Message: message, Cause: cause}
}

func AsHostCommandError(err error, target **HostCommandError) bool {
	return errors.As(err, target)
}

type HostCommandRegistry struct {
	mu       sync.RWMutex
	commands map[string]HostCommandDefinition
}

func NewHostCommandRegistry() *HostCommandRegistry {
	return &HostCommandRegistry{
		commands: make(map[string]HostCommandDefinition),
	}
}

func (r *HostCommandRegistry) Register(def HostCommandDefinition) error {
	if def.CommandID == "" {
		return fmt.Errorf("host command: command_id is required")
	}
	if def.Handler == nil {
		return fmt.Errorf("host command %s: handler is required", def.CommandID)
	}
	if def.Permission == "" {
		return fmt.Errorf("host command %s: permission is required", def.CommandID)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.commands[def.CommandID]; exists {
		return fmt.Errorf("host command %s: already registered", def.CommandID)
	}
	r.commands[def.CommandID] = def
	return nil
}

func (r *HostCommandRegistry) Unregister(commandID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.commands, commandID)
}

func (r *HostCommandRegistry) Get(commandID string) (HostCommandDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.commands[commandID]
	return def, ok
}

func (r *HostCommandRegistry) Execute(ctx context.Context, commandID string, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
	r.mu.RLock()
	def, ok := r.commands[commandID]
	r.mu.RUnlock()
	if !ok {
		return nil, NewHostCommandError(
			ErrCodeHostCommandNotFound,
			fmt.Sprintf("host command not registered: %s", commandID),
			nil,
		)
	}
	return def.Handler(ctx, execCtx, input)
}

func (r *HostCommandRegistry) IsRegistered(commandID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.commands[commandID]
	return ok
}

func (r *HostCommandRegistry) List() []HostCommandDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]HostCommandDefinition, 0, len(r.commands))
	for _, def := range r.commands {
		result = append(result, def)
	}
	return result
}

func (r *HostCommandRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.commands)
}
