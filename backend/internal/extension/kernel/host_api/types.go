package host_api

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type Method string

const (
	MethodToolExecute    Method = "host.tool.execute"
	MethodStateGet       Method = "host.state.get"
	MethodStateCAS       Method = "host.state.cas"
	MethodStateDelete    Method = "host.state.delete"
	MethodStateList      Method = "host.state.list"
	MethodSecretGet      Method = "host.secret.get"
	MethodResourceOpen   Method = "host.resource.open"
	MethodResourceRead   Method = "host.resource.read"
	MethodResourceWrite  Method = "host.resource.write"
	MethodResourceClose  Method = "host.resource.close"
	MethodResourceStat   Method = "host.resource.stat"
	MethodEventEmit      Method = "host.event.emit"
	MethodEventSubscribe Method = "host.event.subscribe"
	MethodEventUnsubscribe Method = "host.event.unsubscribe"
	MethodScheduleCreate Method = "host.schedule.create"
	MethodScheduleCancel Method = "host.schedule.cancel"
	MethodScheduleList   Method = "host.schedule.list"
	MethodUINotify        Method = "host.ui.notify"
	MethodUIDialog        Method = "host.ui.dialog"
	MethodUINavigate      Method = "host.ui.navigate"
	MethodClipboardWrite  Method = "host.clipboard.write"
	MethodCharacterRead   Method = "host.character.read"
	MethodConversationRead Method = "host.conversation.read"
	MethodMemoryQuery    Method = "host.memory.query"
	MethodProviderInvoke Method = "host.provider.invoke"
	MethodRuntimeHealth  Method = "host.runtime.health"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type SideEffectLevel string

const (
	SideEffectNone     SideEffectLevel = "none"
	SideEffectReadOnly SideEffectLevel = "read_only"
	SideEffectWrite    SideEffectLevel = "write"
	SideEffectExternal SideEffectLevel = "external"
)

type ScopePolicy struct {
	RequireRoles   []string
	AllowNarrowing bool
	Namespaced     bool
}

type RateLimit struct {
	MaxPerSecond int
	MaxPerMinute int
	Burst        int
}

type PermissionRequirement struct {
	Name     string
	Resource string
}

type CallRequest struct {
	CallID               string
	RuntimeIdentity      runtime_supervisor.RuntimeIdentity
	Method               Method
	Version              int
	Input                json.RawMessage
	ScopeSnapshotID      string
	PermissionSnapshotID string
	TraceID              string
	InvocationID         string
	ParentID             string
	Deadline             time.Time
}

type CallResult struct {
	Status       string
	Output       json.RawMessage
	Error        *Error
	ResourceRefs []ResourceReference
	SideEffects  []RecordedSideEffect
	Metadata     map[string]any
}

type ResourceReference struct {
	HandleID  string
	Type      string
	Namespace string
	OwnerID   string
	ExpiresAt *time.Time
}

type RecordedSideEffect struct {
	Kind    string
	Target  string
	Detail  string
}

type Error struct {
	Code    string
	Message string
	Detail  string
}

type Route struct {
	Method          Method
	Version         int
	InputSchema     json.RawMessage
	OutputSchema    json.RawMessage
	Permission      []PermissionRequirement
	ScopePolicy     ScopePolicy
	RiskLevel       RiskLevel
	SideEffectLevel SideEffectLevel
	RateLimit       RateLimit
	Timeout         time.Duration
	Handler         Handler
}

type Handler func(ctx context.Context, request CallRequest) (CallResult, error)

type Session struct {
	SessionID        string
	RuntimeIdentity  runtime_supervisor.RuntimeIdentity
	Generation       int64
	AllowedVersions  map[Method]int
	CreatedAt        time.Time
	ExpiresAt        *time.Time
	Active           bool
}

type Gateway interface {
	RegisterRoute(route Route) error
	OpenSession(ctx context.Context, identity runtime_supervisor.RuntimeIdentity, allowedVersions map[Method]int) (Session, error)
	CloseSession(ctx context.Context, sessionID string) error
	Call(ctx context.Context, request CallRequest) CallResult
	QueryCapability(ctx context.Context, method Method) (Route, bool)
	ListMethods(ctx context.Context) []Method
}

var (
	ErrRouteExists         = errors.New("host_api: route already registered")
	ErrRouteNotFound       = errors.New("host_api: route not found")
	ErrSessionNotFound     = errors.New("host_api: session not found")
	ErrSessionExpired      = errors.New("host_api: session expired")
	ErrIdentityInvalid     = errors.New("host_api: runtime identity invalid")
	ErrGenerationStale     = errors.New("host_api: generation stale")
	ErrPermissionDenied    = errors.New("host_api: permission denied")
	ErrScopeDenied         = errors.New("host_api: scope denied")
	ErrInputInvalid        = errors.New("host_api: input invalid")
	ErrOutputInvalid       = errors.New("host_api: output invalid")
	ErrRateLimited         = errors.New("host_api: rate limited")
	ErrTimeout             = errors.New("host_api: timeout")
	ErrCancelled           = errors.New("host_api: cancelled")
	ErrMethodNotFound      = errors.New("host_api: method not found")
	ErrVersionUnsupported  = errors.New("host_api: version unsupported")
	ErrHostUnavailable     = errors.New("host_api: host unavailable")
)

const (
	StatusSuccess    = "success"
	StatusFailed     = "failed"
	StatusRejected   = "rejected"
	StatusTimeout    = "timeout"
	StatusCancelled  = "cancelled"
	StatusRateLimit  = "rate_limited"
)

const (
	ErrorCodeMethodNotFound     = "method_not_found"
	ErrorCodeVersionUnsupported = "version_unsupported"
	ErrorCodeIdentityInvalid    = "identity_invalid"
	ErrorCodeGenerationStale    = "generation_stale"
	ErrorCodePermissionDenied   = "permission_denied"
	ErrorCodeScopeDenied        = "scope_denied"
	ErrorCodeApprovalRequired   = "approval_required"
	ErrorCodeInputInvalid       = "input_invalid"
	ErrorCodeOutputInvalid      = "output_invalid"
	ErrorCodeRateLimited        = "rate_limited"
	ErrorCodeTimeout            = "timeout"
	ErrorCodeCancelled          = "cancelled"
	ErrorCodeResourceNotFound   = "resource_not_found"
	ErrorCodeStateConflict      = "state_conflict"
	ErrorCodeHostUnavailable         = "host_unavailable"
	ErrorCodeUIHostUnavailable       = "ui_host_unavailable"
	ErrorCodeDialogHostUnavailable   = "dialog_host_unavailable"
	ErrorCodeNavigationHostUnavailable = "navigation_host_unavailable"
	ErrorCodeInternal           = "internal_error"
)

type PermissionChecker interface {
	Check(ctx context.Context, identity runtime_supervisor.RuntimeIdentity, requirements []PermissionRequirement) error
}

type PermissionSnapshotStore interface {
	Get(ctx context.Context, snapshotID string) (*permission.PermissionSnapshot, error)
}

type PermissionSnapshotReader interface {
	GetSnapshot(ctx context.Context, snapshotID string) (permission.PermissionSnapshot, error)
}

type PermissionSnapshotChecker interface {
	Check(ctx context.Context, identity runtime_supervisor.RuntimeIdentity, permissionSnapshotID string, requirements []PermissionRequirement) error
}

type ScopeChecker interface {
	Check(ctx context.Context, identity runtime_supervisor.RuntimeIdentity, scopeSnapshotID string, policy ScopePolicy) error
}

type AuditWriter interface {
	RecordCallStart(ctx context.Context, request CallRequest) error
	RecordCall(ctx context.Context, request CallRequest, result CallResult)
}

type PermissionCheckerFunc func(ctx context.Context, identity runtime_supervisor.RuntimeIdentity, requirements []PermissionRequirement) error
type ScopeCheckerFunc func(ctx context.Context, identity runtime_supervisor.RuntimeIdentity, scopeSnapshotID string, policy ScopePolicy) error
type PermissionSnapshotCheckerFunc func(ctx context.Context, identity runtime_supervisor.RuntimeIdentity, permissionSnapshotID string, requirements []PermissionRequirement) error

func (f PermissionCheckerFunc) Check(ctx context.Context, id runtime_supervisor.RuntimeIdentity, req []PermissionRequirement) error {
	return f(ctx, id, req)
}

func (f ScopeCheckerFunc) Check(ctx context.Context, id runtime_supervisor.RuntimeIdentity, sid string, p ScopePolicy) error {
	return f(ctx, id, sid, p)
}

func (f PermissionSnapshotCheckerFunc) Check(ctx context.Context, id runtime_supervisor.RuntimeIdentity, sid string, req []PermissionRequirement) error {
	return f(ctx, id, sid, req)
}

var _ domain.ExtensionID
