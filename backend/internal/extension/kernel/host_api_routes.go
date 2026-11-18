package kernel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	coreexec "github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

type hostAPIAuditWriter struct {
	auditRepo hostAPIAuditPutter
}

type hostAPIAuditPutter interface {
	PutAuditLog(ctx context.Context, entry sqlite.HostAPIAuditLog) error
}

func newHostAPIAuditWriter(auditRepo hostAPIAuditPutter) *hostAPIAuditWriter {
	return &hostAPIAuditWriter{auditRepo: auditRepo}
}

var sensitiveKeyPatterns = []string{
	"password", "passwd", "token", "secret", "credential", "apikey", "api_key",
	"auth", "authorization", "bearer", "privatekey", "private_key",
	"accesstoken", "access_token", "refreshtoken", "refresh_token",
	"session", "cookie", "otp", "code", "pin",
}

const maxMaskedInputSize = 8192

func maskSensitiveInput(method host_api.Method, input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var parsed interface{}
	if err := json.Unmarshal(input, &parsed); err != nil {
		truncated := string(input)
		if len(truncated) > maxMaskedInputSize {
			truncated = truncated[:maxMaskedInputSize] + "...[truncated]"
		}
		return truncated
	}
	masked := maskSensitiveValue(parsed)
	out, err := json.Marshal(masked)
	if err != nil {
		return "{}"
	}
	result := string(out)
	if len(result) > maxMaskedInputSize {
		result = result[:maxMaskedInputSize] + "...[truncated]"
	}
	return result
}

func maskSensitiveValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		masked := make(map[string]interface{}, len(val))
		for k, vv := range val {
			lk := strings.ToLower(k)
			if isSensitiveKey(lk) {
				masked[k] = "***REDACTED***"
			} else {
				masked[k] = maskSensitiveValue(vv)
			}
		}
		return masked
	case []interface{}:
		masked := make([]interface{}, len(val))
		for i, vv := range val {
			masked[i] = maskSensitiveValue(vv)
		}
		return masked
	default:
		return v
	}
}

func isSensitiveKey(lowerKey string) bool {
	for _, p := range sensitiveKeyPatterns {
		if strings.Contains(lowerKey, p) {
			return true
		}
	}
	return false
}

func (w *hostAPIAuditWriter) RecordCallStart(ctx context.Context, request host_api.CallRequest) error {
	callID := request.CallID
	if callID == "" {
		callID = fmt.Sprintf("audit-%s", uuid.NewString())
	}
	now := time.Now().UTC()
	entry := sqlite.HostAPIAuditLog{
		CallID:               callID,
		TraceID:              request.TraceID,
		OperationID:          callID,
		InvocationID:         request.InvocationID,
		ExtensionID:          string(request.RuntimeIdentity.ExtensionID),
		ModuleID:             string(request.RuntimeIdentity.ModuleID),
		Method:               string(request.Method),
		Generation:           request.RuntimeIdentity.Generation,
		PermissionSnapshotID: request.PermissionSnapshotID,
		ScopeSnapshotID:      request.ScopeSnapshotID,
		StartedAt:            now,
		Result:               "started",
		Phase:                "start",
		InputMasked:          maskSensitiveInput(request.Method, request.Input),
	}
	if w.auditRepo == nil {
		log.Printf("[host-api-audit] START call=%s method=%s ext=%s (no repository, log only)", callID, request.Method, request.RuntimeIdentity.ExtensionID)
		return nil
	}
	if err := w.auditRepo.PutAuditLog(ctx, entry); err != nil {
		log.Printf("[host-api-audit] FAILED to persist start record: call=%s method=%s err=%v", callID, request.Method, err)
		return err
	}
	log.Printf("[host-api-audit] START call=%s method=%s ext=%s", callID, request.Method, request.RuntimeIdentity.ExtensionID)
	return nil
}

func (w *hostAPIAuditWriter) RecordCall(ctx context.Context, request host_api.CallRequest, result host_api.CallResult) {
	callID := request.CallID
	if callID == "" {
		callID = fmt.Sprintf("audit-%s", uuid.NewString())
	}
	now := time.Now().UTC()
	errorCode := ""
	errorMessage := ""
	if result.Error != nil {
		errorCode = result.Error.Code
		errorMessage = result.Error.Message
	}
	sideEffect := ""
	if len(result.SideEffects) > 0 {
		parts := make([]string, 0, len(result.SideEffects))
		for _, se := range result.SideEffects {
			parts = append(parts, fmt.Sprintf("%s:%s:%s", se.Kind, se.Target, se.Detail))
		}
		sideEffect = strings.Join(parts, "; ")
	}
	entry := sqlite.HostAPIAuditLog{
		CallID:               callID,
		TraceID:              request.TraceID,
		OperationID:          callID,
		InvocationID:         request.InvocationID,
		ExtensionID:          string(request.RuntimeIdentity.ExtensionID),
		ModuleID:             string(request.RuntimeIdentity.ModuleID),
		Method:               string(request.Method),
		Generation:           request.RuntimeIdentity.Generation,
		PermissionSnapshotID: request.PermissionSnapshotID,
		ScopeSnapshotID:      request.ScopeSnapshotID,
		StartedAt:            now,
		FinishedAt:           &now,
		Result:               result.Status,
		ErrorCode:            errorCode,
		ErrorMessage:         errorMessage,
		SideEffect:           sideEffect,
		Phase:                "end",
		InputMasked:          maskSensitiveInput(request.Method, request.Input),
	}
	log.Printf("[host-api-audit] END call=%s method=%s ext=%s status=%s",
		callID, request.Method, request.RuntimeIdentity.ExtensionID, result.Status)
	if w.auditRepo == nil {
		return
	}
	if err := w.auditRepo.PutAuditLog(ctx, entry); err != nil {
		log.Printf("[host-api-audit] FAILED to persist end record: call=%s method=%s err=%v", callID, request.Method, err)
	}
}

type ExtensionStateStore interface {
	Get(ctx context.Context, extensionID, moduleID, key string) (json.RawMessage, int64, bool, error)
	Set(ctx context.Context, extensionID, moduleID, key string, value json.RawMessage) (int64, error)
	CAS(ctx context.Context, extensionID, moduleID, key string, expectedVersion int64, newValue json.RawMessage) (bool, int64, error)
	Delete(ctx context.Context, extensionID, moduleID, key string) error
	List(ctx context.Context, extensionID, moduleID string) ([]sqlite.ExtensionStateEntry, error)
}

type CharacterReader interface {
	ReadCharacter(ctx context.Context, characterID string) (json.RawMessage, bool, error)
}

type CharacterRevisionReader func(ctx context.Context, characterID string) (int64, error)

type CharacterLister interface {
	ListCharacters(ctx context.Context, spaceID string, includeDisabled bool) ([]json.RawMessage, error)
}

type ConversationReader interface {
	ReadConversation(ctx context.Context, conversationID string, limit int, offset int) ([]json.RawMessage, bool, error)
}

type ConversationSearcher interface {
	SearchConversations(ctx context.Context, query string, limit int) ([]json.RawMessage, bool, error)
}

type ConversationCreateRequest struct {
	SpaceID        string
	CharacterID    string
	ConversationID string
	RequestID      string
	Title          string
	ProjectID      string
	Channel        string
	PeerID         string
	Draft          bool
}

type ConversationDeleteRequest struct {
	SpaceID        string
	CharacterID    string
	ConversationID string
	RequestID      string
}

type ConversationCreator interface {
	CreateConversation(ctx context.Context, request ConversationCreateRequest) (json.RawMessage, error)
}

type ConversationLister interface {
	ListConversations(ctx context.Context, limit int, offset int, archivedOnly bool) (any, error)
}

type ConversationDeleter interface {
	DeleteConversation(ctx context.Context, request ConversationDeleteRequest) (json.RawMessage, error)
}

type MemoryQueryService interface {
	Query(ctx context.Context, extensionID string, query string, limit int) ([]json.RawMessage, error)
}

type RuntimeHealthReader interface {
	SnapshotByExtension(ctx context.Context, extensionID string, moduleID string) []runtime_supervisor.RuntimeHealthSnapshot
}

type ConversationMessageRequest struct {
	SpaceID        string
	CharacterID    string
	ConversationID string
	Channel        string
	Content        string
	RequestID      string
}

type ConversationMessageResult struct {
	Content   string
	RequestID string
}

type ConversationMessageSender interface {
	SendConversationMessage(ctx context.Context, request ConversationMessageRequest) (ConversationMessageResult, error)
}

type ConversationMessagePart struct {
	Type          string         `json:"type"`
	Content       string         `json:"content,omitempty"`
	ExtensionType string         `json:"extensionType,omitempty"`
	MIMEType      string         `json:"mimeType,omitempty"`
	URL           string         `json:"url,omitempty"`
	FallbackURL   string         `json:"fallbackUrl,omitempty"`
	AltText       string         `json:"altText,omitempty"`
	Width         int            `json:"width,omitempty"`
	Height        int            `json:"height,omitempty"`
	IsAnimated    bool           `json:"isAnimated,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type ConversationMessageAppendRequest struct {
	SpaceID          string
	CharacterID      string
	ConversationID   string
	Channel          string
	Role             string
	Source           string
	ReplyToMessageID *string
	RequestID        string
	Parts            []ConversationMessagePart
}

type ConversationMessageAppendResult struct {
	MessageIDs      []string `json:"messageIds"`
	Sequences       []int64  `json:"sequences"`
	DeliveryGroupID string   `json:"deliveryGroupId"`
	LastSequence    int64    `json:"lastSequence"`
}

type ConversationMessageAppender interface {
	AppendConversationMessages(ctx context.Context, request ConversationMessageAppendRequest) (ConversationMessageAppendResult, error)
}

var ErrConversationTurnUnavailable = errors.New("conversation turn is unavailable")

type ConversationTurnInterruptRequest struct {
	SpaceID        string
	CharacterID    string
	ConversationID string
	TurnID         string
}

type ConversationTurnInterruptResult struct {
	Interrupted bool   `json:"interrupted"`
	TurnID      string `json:"turnId"`
	RequestID   string `json:"requestId,omitempty"`
	ExecutionID string `json:"executionId,omitempty"`
}

type ConversationTurnAwaitResult struct {
	Idle   bool   `json:"idle"`
	TurnID string `json:"turnId,omitempty"`
}

type ConversationTurnController interface {
	InterruptConversationTurn(ctx context.Context, request ConversationTurnInterruptRequest) (ConversationTurnInterruptResult, error)
	AwaitConversationTurnIdle(ctx context.Context, request ConversationTurnInterruptRequest) (ConversationTurnAwaitResult, error)
}

type ProjectWorkspaceInput struct {
	ID       string `json:"id"`
	DeviceID string `json:"deviceId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	RootURI  string `json:"rootUri"`
	ReadOnly bool   `json:"readOnly,omitempty"`
}

type ProjectCreateRequest struct {
	SpaceID            string
	CharacterID        string
	Title              string
	DirectoryPath      string
	Workspaces         []ProjectWorkspaceInput
	PrimaryWorkspaceID string
	RequestID          string
}

type ProjectUpdateRequest struct {
	ProjectID          string
	Title              *string
	Pinned             *bool
	Hidden             *bool
	Workspaces         []ProjectWorkspaceInput
	PrimaryWorkspaceID *string
	RequestID          string
}

type ProjectDeleteRequest struct {
	ProjectID string
	RequestID string
}

type ConversationArchiveRequest struct {
	SpaceID        string
	CharacterID    string
	ConversationID string
	Archived       bool
	RequestID      string
}

type ConversationPinRequest struct {
	SpaceID        string
	CharacterID    string
	ConversationID string
	Pinned         bool
	RequestID      string
}

type ConversationForkRequest struct {
	SpaceID          string
	CharacterID      string
	ConversationID   string
	BoundarySequence int64
	Title            string
	RequestID        string
}

type ConversationForker interface {
	ForkConversation(ctx context.Context, request ConversationForkRequest) (json.RawMessage, error)
}

type ModelDefaultsProvider interface {
	ModelDefaults(ctx context.Context) (map[string]any, error)
}

type ConversationPinner interface {
	PinConversation(ctx context.Context, request ConversationPinRequest) (json.RawMessage, error)
}

const (
	hostDirectoryDefaultLimit = 200
	hostDirectoryMaxLimit     = 1000
)

type DirectoryEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modifiedAt"`
}

type DirectoryListRequest struct {
	Path  string
	Limit int
}

type DirectoryListResult struct {
	Path    string           `json:"path"`
	Entries []DirectoryEntry `json:"entries"`
}

type DirectoryCreateRequest struct {
	Path string
	Name string
}

type DirectoryLister interface {
	ListDirectory(ctx context.Context, request DirectoryListRequest) (DirectoryListResult, error)
}

type DirectoryCreator interface {
	CreateDirectory(ctx context.Context, request DirectoryCreateRequest) (string, error)
}

type ProjectLister interface {
	ListProjects(ctx context.Context, limit int, cursor string) (any, error)
}

type ProjectCreator interface {
	CreateProject(ctx context.Context, request ProjectCreateRequest) (json.RawMessage, error)
}

type ProjectUpdater interface {
	UpdateProject(ctx context.Context, request ProjectUpdateRequest) (json.RawMessage, error)
}

type ProjectDeleter interface {
	DeleteProject(ctx context.Context, request ProjectDeleteRequest) (json.RawMessage, error)
}

type ConversationArchiver interface {
	ArchiveConversation(ctx context.Context, request ConversationArchiveRequest) (json.RawMessage, error)
}

type ExtensionDataSource interface {
	Read(ctx context.Context, sourceID string, params json.RawMessage) (json.RawMessage, error)
}

type HostAPIRouteDeps struct {
	StateStore                  ExtensionStateStore
	CharacterReader             CharacterReader
	CharacterRevisionReader     CharacterRevisionReader
	CharacterLister             CharacterLister
	ConversationReader          ConversationReader
	ConversationSearcher        ConversationSearcher
	MemoryQueryService          MemoryQueryService
	EventService                *event.Service
	ScheduleService             *schedule.ScheduleService
	Coordination                *coordination.Service
	UIExecutionIdentity         func(sessionID string) (spaceID string, deviceID string)
	ToolFacade                  *ToolFacade
	PluginPackages              HostPluginPackageManagerPort
	ExecutionKernel             *execution.ExecutionPipeline
	ToolRegistry                *capability.ToolRegistry
	OperationRepository         sqlite.OperationRepository
	ExtensionRoot               string
	UIHostNotifier              UIHostNotifier
	ClipboardHost               ClipboardHost
	RuntimeSupervisor           RuntimeHealthReader
	ScopeSnapshotStore          host_api.ScopeSnapshotStore
	ScopeSnapshotWriter         ScopeSnapshotWriter
	ScopeSnapshotDeleter        ScopeSnapshotDeleter
	SecretStore                 SecretStore
	ProviderInvoker             ProviderInvoker
	ModelGateway                *ModelGatewayBridge
	ApprovalBroker              *execution.ApprovalBroker
	ConversationCreator         ConversationCreator
	ConversationLister          ConversationLister
	ConversationDeleter         ConversationDeleter
	ConversationMessageSender   ConversationMessageSender
	ConversationMessageAppender ConversationMessageAppender
	ConversationTurnController  ConversationTurnController
	ConversationArchiver        ConversationArchiver
	ConversationPinner          ConversationPinner
	ModelDefaults               ModelDefaultsProvider
	ConversationForker          ConversationForker
	DirectoryLister             DirectoryLister
	DirectoryCreator            DirectoryCreator
	SkillCatalog                *agent_skill.AgentSkillCatalog
	ProjectLister               ProjectLister
	ProjectCreator              ProjectCreator
	ProjectUpdater              ProjectUpdater
	ProjectDeleter              ProjectDeleter
	ExtensionDataSources        ExtensionDataSource
	ResourceLinks               *ResourceLinkManager
	VectorStore                 ExtensionVectorStore
	NetworkClient               *http.Client
	NetworkAllowPrivate         bool
	RuntimeObservability        *RuntimeObservabilityBridge
	RuntimeMaintenance          *RuntimeMaintenanceBridge
	Workspace                   *WorkspaceBridge
	ChatImport                  *ChatImportBridge
	Storage                     *StorageBridge
	ChatLogs                    *ChatLogsBridge
}

type SecretStore interface {
	Get(ctx context.Context, ref string) ([]byte, error)
}

type ProviderInvoker interface {
	Invoke(ctx context.Context, request ProviderInvokeRequest) (ProviderInvokeResponse, error)
}

type ProviderInvokeRequest struct {
	Target  string          `json:"target"`
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload"`
}

type ProviderInvokeResponse struct {
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result"`
}

type ExtensionVectorPoint struct {
	ID      string         `json:"id"`
	Vector  []float32      `json:"vector,omitempty"`
	Text    string         `json:"text,omitempty"`
	Payload map[string]any `json:"payload,omitempty"`
}

type ExtensionVectorSearchResult struct {
	ID      string         `json:"id"`
	Score   float64        `json:"score"`
	Payload map[string]any `json:"payload,omitempty"`
}

type ExtensionVectorStore interface {
	Upsert(ctx context.Context, namespace, collection string, points []ExtensionVectorPoint) error
	Search(ctx context.Context, namespace, collection, query string, vector []float32, limit int, filter map[string]string) ([]ExtensionVectorSearchResult, error)
	Delete(ctx context.Context, namespace, collection string, ids []string) error
}

type resourceHandle struct {
	handleID        string
	realPath        string
	file            *os.File
	readOnly        bool
	extensionID     string
	moduleID        string
	scopeSnapshotID string
	generation      int64
	owner           string
	expiresAt       time.Time
}

type resourceHandleTable struct {
	mu      sync.Mutex
	handles map[string]*resourceHandle
}

func newResourceHandleTable() *resourceHandleTable {
	return &resourceHandleTable{
		handles: make(map[string]*resourceHandle),
	}
}

func (t *resourceHandleTable) put(h *resourceHandle) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handles[h.handleID] = h
}

func (t *resourceHandleTable) get(handleID string) (*resourceHandle, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	h, ok := t.handles[handleID]
	return h, ok
}

func (t *resourceHandleTable) remove(handleID string) (*resourceHandle, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	h, ok := t.handles[handleID]
	if ok {
		delete(t.handles, handleID)
	}
	return h, ok
}

func (t *resourceHandleTable) closeAll(extensionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id, h := range t.handles {
		if h.extensionID == extensionID {
			if h.file != nil {
				if err := h.file.Close(); err != nil {
					log.Printf("[original-error] %v", err)
				}
			}
			delete(t.handles, id)
		}
	}
}

func (t *resourceHandleTable) cleanupExpired() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	for id, h := range t.handles {
		if !h.expiresAt.IsZero() && now.After(h.expiresAt) {
			if h.file != nil {
				if err := h.file.Close(); err != nil {
					log.Printf("[original-error] %v", err)
				}
			}
			delete(t.handles, id)
		}
	}
}

const maxToolExecutionDepth = 8
const resourceHandleTTL = 30 * time.Minute
const clipboardRouteTimeout = 12 * time.Second
const maxClipboardPayloadSize = 1 * 1024 * 1024

func conversationMessageRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		var p struct {
			SpaceID        string `json:"spaceId"`
			CharacterID    string `json:"characterId"`
			ConversationID string `json:"conversationId"`
			Channel        string `json:"channel"`
			Content        string `json:"content"`
			RequestID      string `json:"requestId"`
		}
		if err := json.Unmarshal(req.Input, &p); err != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
			}, nil
		}
		content := strings.TrimSpace(p.Content)
		if content == "" {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "content is required"},
			}, nil
		}
		ctx = resolveHostAPIScope(ctx, deps.ScopeSnapshotStore, req.ScopeSnapshotID)
		scope := GetHostAPIScope(ctx)
		targetScope, targetErr := resolveConversationTargetScope(ctx, deps, req, p.ConversationID)
		if targetErr != nil {
			return host_api.CallResult{
				Status: host_api.StatusRejected,
				Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: targetErr.Error()},
			}, nil
		}
		scope = targetScope
		if req.SpaceID != "" {
			if scope.SpaceID != "" && scope.SpaceID != req.SpaceID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "spaceId does not match current scope"},
				}, nil
			}
			scope.SpaceID = req.SpaceID
		}
		if req.CharacterID != "" {
			if scope.CharacterID != "" && scope.CharacterID != req.CharacterID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "characterId does not match current scope"},
				}, nil
			}
			scope.CharacterID = req.CharacterID
		}
		if req.ConversationID != "" && req.TargetScopeSnapshotID == "" {
			if scope.ConversationID != "" && scope.ConversationID != req.ConversationID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "conversationId does not match current scope"},
				}, nil
			}
			scope.ConversationID = req.ConversationID
		}
		ctx = WithHostAPIScope(ctx, scope)
		if scope.SpaceID != "" {
			if p.SpaceID != "" && p.SpaceID != scope.SpaceID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "spaceId does not match current scope"},
				}, nil
			}
			p.SpaceID = scope.SpaceID
		}
		if scope.CharacterID != "" {
			if p.CharacterID != "" && p.CharacterID != scope.CharacterID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "characterId does not match current scope"},
				}, nil
			}
			p.CharacterID = scope.CharacterID
		}
		if scope.ConversationID != "" {
			if p.ConversationID != "" && p.ConversationID != scope.ConversationID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "conversationId does not match current scope"},
				}, nil
			}
			p.ConversationID = scope.ConversationID
		}
		if deps.ConversationMessageSender == nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "conversation message sender not configured"},
			}, nil
		}
		requestID := strings.TrimSpace(p.RequestID)
		if requestID == "" {
			requestID = uuid.NewString()
		}
		result, err := deps.ConversationMessageSender.SendConversationMessage(ctx, ConversationMessageRequest{
			SpaceID:        p.SpaceID,
			CharacterID:    p.CharacterID,
			ConversationID: p.ConversationID,
			Channel:        p.Channel,
			Content:        content,
			RequestID:      requestID,
		})
		if err != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error: &host_api.Error{
					Code:    host_api.ErrorCodeInternal,
					Message: err.Error(),
				},
			}, nil
		}
		if strings.TrimSpace(result.RequestID) == "" {
			result.RequestID = requestID
		}
		output, _ := json.Marshal(map[string]any{
			"content":   result.Content,
			"requestId": result.RequestID,
		})
		return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
	}
}

func conversationMessageAppendRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		var p struct {
			SpaceID          string                    `json:"spaceId"`
			CharacterID      string                    `json:"characterId"`
			ConversationID   string                    `json:"conversationId"`
			Channel          string                    `json:"channel"`
			Role             string                    `json:"role"`
			Source           string                    `json:"source"`
			ReplyToMessageID *string                   `json:"replyToMessageId"`
			RequestID        string                    `json:"requestId"`
			Parts            []ConversationMessagePart `json:"parts"`
		}
		if err := json.Unmarshal(req.Input, &p); err != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
			}, nil
		}
		if len(p.Parts) == 0 || len(p.Parts) > 16 {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "parts must contain between 1 and 16 items"},
			}, nil
		}
		ctx = resolveHostAPIScope(ctx, deps.ScopeSnapshotStore, req.ScopeSnapshotID)
		scope := GetHostAPIScope(ctx)
		targetScope, targetErr := resolveConversationTargetScope(ctx, deps, req, p.ConversationID)
		if targetErr != nil {
			return host_api.CallResult{
				Status: host_api.StatusRejected,
				Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: targetErr.Error()},
			}, nil
		}
		scope = targetScope
		if req.SpaceID != "" {
			if scope.SpaceID != "" && scope.SpaceID != req.SpaceID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "spaceId does not match current scope"},
				}, nil
			}
			scope.SpaceID = req.SpaceID
		}
		if req.CharacterID != "" {
			if scope.CharacterID != "" && scope.CharacterID != req.CharacterID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "characterId does not match current scope"},
				}, nil
			}
			scope.CharacterID = req.CharacterID
		}
		if req.ConversationID != "" && req.TargetScopeSnapshotID == "" {
			if scope.ConversationID != "" && scope.ConversationID != req.ConversationID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "conversationId does not match current scope"},
				}, nil
			}
			scope.ConversationID = req.ConversationID
		}
		ctx = WithHostAPIScope(ctx, scope)
		if scope.SpaceID != "" {
			if p.SpaceID != "" && p.SpaceID != scope.SpaceID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "spaceId does not match current scope"},
				}, nil
			}
			p.SpaceID = scope.SpaceID
		}
		if scope.CharacterID != "" {
			if p.CharacterID != "" && p.CharacterID != scope.CharacterID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "characterId does not match current scope"},
				}, nil
			}
			p.CharacterID = scope.CharacterID
		}
		if scope.ConversationID != "" {
			if p.ConversationID != "" && p.ConversationID != scope.ConversationID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "conversationId does not match current scope"},
				}, nil
			}
			p.ConversationID = scope.ConversationID
		}
		if deps.ConversationMessageAppender == nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "conversation message appender not configured"},
			}, nil
		}
		requestID := strings.TrimSpace(p.RequestID)
		if requestID == "" {
			requestID = uuid.NewString()
		}
		result, err := deps.ConversationMessageAppender.AppendConversationMessages(ctx, ConversationMessageAppendRequest{
			SpaceID:          p.SpaceID,
			CharacterID:      p.CharacterID,
			ConversationID:   p.ConversationID,
			Channel:          p.Channel,
			Role:             p.Role,
			Source:           p.Source,
			ReplyToMessageID: p.ReplyToMessageID,
			RequestID:        requestID,
			Parts:            append([]ConversationMessagePart(nil), p.Parts...),
		})
		if err != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
			}, nil
		}
		output, _ := json.Marshal(result)
		return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
	}
}

func conversationCreateRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		var p struct {
			SpaceID        string `json:"spaceId"`
			CharacterID    string `json:"characterId"`
			ConversationID string `json:"conversationId"`
			RequestID      string `json:"requestId"`
			Title          string `json:"title"`
			ProjectID      string `json:"projectId"`
			Channel        string `json:"channel"`
			PeerID         string `json:"peerId"`
		}
		if err := json.Unmarshal(req.Input, &p); err != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
			}, nil
		}
		p.SpaceID = strings.TrimSpace(p.SpaceID)
		p.CharacterID = strings.TrimSpace(p.CharacterID)
		p.ConversationID = strings.TrimSpace(p.ConversationID)
		p.RequestID = strings.TrimSpace(p.RequestID)
		p.Title = strings.TrimSpace(p.Title)
		p.ProjectID = strings.TrimSpace(p.ProjectID)
		p.Channel = strings.TrimSpace(p.Channel)
		p.PeerID = strings.TrimSpace(p.PeerID)
		if len(p.Title) > 1024 || len(p.ConversationID) > 512 || len(p.RequestID) > 128 || len(p.ProjectID) > 512 || len(p.Channel) > 128 || len(p.PeerID) > 512 {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "conversation create input is too long"},
			}, nil
		}
		ctx = resolveHostAPIScope(ctx, deps.ScopeSnapshotStore, req.ScopeSnapshotID)
		scope := GetHostAPIScope(ctx)
		if req.SpaceID != "" {
			if scope.SpaceID != "" && scope.SpaceID != req.SpaceID {
				return host_api.CallResult{Status: host_api.StatusRejected, Error: &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "spaceId does not match current scope"}}, nil
			}
			scope.SpaceID = req.SpaceID
		}
		if req.CharacterID != "" {
			if scope.CharacterID != "" && scope.CharacterID != req.CharacterID {
				return host_api.CallResult{Status: host_api.StatusRejected, Error: &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "characterId does not match current scope"}}, nil
			}
			scope.CharacterID = req.CharacterID
		}
		ctx = WithHostAPIScope(ctx, scope)
		if scope.SpaceID != "" {
			if p.SpaceID != "" && p.SpaceID != scope.SpaceID {
				return host_api.CallResult{Status: host_api.StatusRejected, Error: &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "spaceId does not match current scope"}}, nil
			}
			p.SpaceID = scope.SpaceID
		}
		if scope.CharacterID != "" {
			if p.CharacterID != "" && p.CharacterID != scope.CharacterID {
				return host_api.CallResult{Status: host_api.StatusRejected, Error: &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "characterId does not match current scope"}}, nil
			}
			p.CharacterID = scope.CharacterID
		}
		if p.SpaceID == "" || p.CharacterID == "" {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "spaceId and characterId are required"}}, nil
		}
		if deps.ConversationCreator == nil {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "conversation creator not configured"}}, nil
		}
		if p.RequestID == "" {
			p.RequestID = uuid.NewString()
		}
		output, err := deps.ConversationCreator.CreateConversation(ctx, ConversationCreateRequest{
			SpaceID:        p.SpaceID,
			CharacterID:    p.CharacterID,
			ConversationID: p.ConversationID,
			RequestID:      p.RequestID,
			Title:          p.Title,
			ProjectID:      p.ProjectID,
			Channel:        p.Channel,
			PeerID:         p.PeerID,
		})
		if err != nil {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()}}, nil
		}
		if len(output) == 0 {
			output = json.RawMessage(`{}`)
		}
		if !json.Valid(output) {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeOutputInvalid, Message: "conversation creator returned invalid JSON"}}, nil
		}
		var created map[string]any
		if err := json.Unmarshal(output, &created); err != nil {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeOutputInvalid, Message: err.Error()}}, nil
		}
		createdConversationID, _ := created["conversationId"].(string)
		if strings.TrimSpace(createdConversationID) == "" {
			createdConversationID = p.ConversationID
		}
		targetSnapshotID, err := createConversationTargetSnapshot(ctx, deps, req, createdConversationID)
		if err != nil {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()}}, nil
		}
		created["scopeSnapshotId"] = targetSnapshotID
		output, err = json.Marshal(created)
		if err != nil {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()}}, nil
		}
		return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
	}
}

func conversationDeleteRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		var p struct {
			SpaceID        string `json:"spaceId"`
			CharacterID    string `json:"characterId"`
			ConversationID string `json:"conversationId"`
			RequestID      string `json:"requestId"`
		}
		if err := json.Unmarshal(req.Input, &p); err != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
			}, nil
		}
		p.SpaceID = strings.TrimSpace(p.SpaceID)
		p.CharacterID = strings.TrimSpace(p.CharacterID)
		p.ConversationID = strings.TrimSpace(p.ConversationID)
		p.RequestID = strings.TrimSpace(p.RequestID)
		if p.ConversationID == "" || len(p.ConversationID) > 512 || len(p.RequestID) > 128 {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "conversationId is required and conversation delete input is too long"},
			}, nil
		}
		ctx = resolveHostAPIScope(ctx, deps.ScopeSnapshotStore, req.ScopeSnapshotID)
		scope := GetHostAPIScope(ctx)
		targetScope, targetErr := resolveConversationTargetScope(ctx, deps, req, p.ConversationID)
		if targetErr != nil {
			return host_api.CallResult{
				Status: host_api.StatusRejected,
				Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: targetErr.Error()},
			}, nil
		}
		scope = targetScope
		if p.SpaceID != "" && scope.SpaceID != "" && p.SpaceID != scope.SpaceID {
			return host_api.CallResult{Status: host_api.StatusRejected, Error: &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "spaceId does not match current scope"}}, nil
		}
		if p.CharacterID != "" && scope.CharacterID != "" && p.CharacterID != scope.CharacterID {
			return host_api.CallResult{Status: host_api.StatusRejected, Error: &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "characterId does not match current scope"}}, nil
		}
		if scope.SpaceID == "" {
			scope.SpaceID = p.SpaceID
		}
		if scope.CharacterID == "" {
			scope.CharacterID = p.CharacterID
		}
		if scope.SpaceID == "" || scope.CharacterID == "" {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "spaceId and characterId are required"},
			}, nil
		}
		scope.ConversationID = p.ConversationID
		ctx = WithHostAPIScope(ctx, scope)
		if deps.ConversationDeleter == nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "conversation deleter not configured"},
			}, nil
		}
		if p.RequestID == "" {
			p.RequestID = uuid.NewString()
		}
		output, err := deps.ConversationDeleter.DeleteConversation(ctx, ConversationDeleteRequest{
			SpaceID:        scope.SpaceID,
			CharacterID:    scope.CharacterID,
			ConversationID: p.ConversationID,
			RequestID:      p.RequestID,
		})
		if err != nil {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()}}, nil
		}
		if len(output) == 0 {
			output = json.RawMessage(`{"deleted":true}`)
		}
		if !json.Valid(output) {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeOutputInvalid, Message: "conversation deleter returned invalid JSON"}}, nil
		}
		if err := deleteConversationTargetSnapshot(ctx, deps, req); err != nil {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()}}, nil
		}
		return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
	}
}

func hostRouteInputInvalid(message string) host_api.CallResult {
	return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: message}}
}

func hostRouteScopeDenied(message string) host_api.CallResult {
	return host_api.CallResult{Status: host_api.StatusRejected, Error: &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: message}}
}

func hostRouteUnavailable(message string) host_api.CallResult {
	return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: message}}
}

func hostRouteInternal(err error) host_api.CallResult {
	return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()}}
}

func hostRouteOutput(value json.RawMessage, fallback string) host_api.CallResult {
	output := value
	if len(output) == 0 {
		output = json.RawMessage(fallback)
	}
	if !json.Valid(output) {
		return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeOutputInvalid, Message: "host route produced invalid JSON"}}
	}
	return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}
}

func resolveProjectHostScope(ctx context.Context, deps HostAPIRouteDeps, req host_api.CallRequest, inputSpaceID string, inputCharacterID string) (context.Context, HostAPIScopeContext, host_api.CallResult, bool) {
	ctx = resolveHostAPIScope(ctx, deps.ScopeSnapshotStore, req.ScopeSnapshotID)
	scope := GetHostAPIScope(ctx)
	for _, pair := range []struct {
		scopeValue *string
		requestVal string
		inputVal   string
		label      string
	}{
		{&scope.SpaceID, req.SpaceID, inputSpaceID, "spaceId"},
		{&scope.CharacterID, req.CharacterID, inputCharacterID, "characterId"},
	} {
		if pair.requestVal != "" {
			if *pair.scopeValue != "" && *pair.scopeValue != pair.requestVal {
				return ctx, scope, hostRouteScopeDenied(pair.label + " does not match current scope"), false
			}
			*pair.scopeValue = pair.requestVal
		}
		if pair.inputVal != "" {
			if *pair.scopeValue != "" && *pair.scopeValue != pair.inputVal {
				return ctx, scope, hostRouteScopeDenied(pair.label + " does not match current scope"), false
			}
			*pair.scopeValue = pair.inputVal
		}
	}
	if scope.SpaceID == "" || scope.CharacterID == "" {
		return ctx, scope, hostRouteInputInvalid("spaceId and characterId are required"), false
	}
	ctx = WithHostAPIScope(ctx, scope)
	return ctx, scope, host_api.CallResult{}, true
}

func projectListRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		var p struct {
			SpaceID     string `json:"spaceId"`
			CharacterID string `json:"characterId"`
			Limit       int    `json:"limit"`
			Cursor      string `json:"cursor"`
		}
		if len(req.Input) > 0 {
			if err := json.Unmarshal(req.Input, &p); err != nil {
				return hostRouteInputInvalid(err.Error()), nil
			}
		}
		p.SpaceID = strings.TrimSpace(p.SpaceID)
		p.CharacterID = strings.TrimSpace(p.CharacterID)
		p.Cursor = strings.TrimSpace(p.Cursor)
		if len(p.Cursor) > 4096 {
			return hostRouteInputInvalid("project cursor is too long"), nil
		}
		ctx, _, rejected, ok := resolveProjectHostScope(ctx, deps, req, p.SpaceID, p.CharacterID)
		if !ok {
			return rejected, nil
		}
		if deps.ProjectLister == nil {
			return hostRouteUnavailable("project lister not configured"), nil
		}
		if p.Limit <= 0 {
			p.Limit = 50
		}
		if p.Limit > 128 {
			p.Limit = 128
		}
		result, err := deps.ProjectLister.ListProjects(ctx, p.Limit, p.Cursor)
		if err != nil {
			return hostRouteInternal(err), nil
		}
		output, err := json.Marshal(result)
		if err != nil {
			return hostRouteInternal(err), nil
		}
		return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
	}
}

func projectCreateRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		var p struct {
			SpaceID            string                  `json:"spaceId"`
			CharacterID        string                  `json:"characterId"`
			Title              string                  `json:"title"`
			DirectoryPath      string                  `json:"directoryPath"`
			Workspaces         []ProjectWorkspaceInput `json:"workspaces"`
			PrimaryWorkspaceID string                  `json:"primaryWorkspaceId"`
			RequestID          string                  `json:"requestId"`
		}
		if err := json.Unmarshal(req.Input, &p); err != nil {
			return hostRouteInputInvalid(err.Error()), nil
		}
		p.SpaceID = strings.TrimSpace(p.SpaceID)
		p.CharacterID = strings.TrimSpace(p.CharacterID)
		p.Title = strings.TrimSpace(p.Title)
		p.DirectoryPath = strings.TrimSpace(p.DirectoryPath)
		p.PrimaryWorkspaceID = strings.TrimSpace(p.PrimaryWorkspaceID)
		p.RequestID = strings.TrimSpace(p.RequestID)
		if len(p.Title) > 1024 || len(p.PrimaryWorkspaceID) > 512 || len(p.RequestID) > 128 || len(p.Workspaces) > 32 {
			return hostRouteInputInvalid("project create input is too long"), nil
		}
		if p.DirectoryPath != "" {
			if len(p.DirectoryPath) > 4096 {
				return hostRouteInputInvalid("project directoryPath is too long"), nil
			}
			if len(p.Workspaces) > 0 || p.PrimaryWorkspaceID != "" {
				return hostRouteInputInvalid("project create accepts either directoryPath or explicit workspaces, not both"), nil
			}
		}
		ctx, scope, rejected, ok := resolveProjectHostScope(ctx, deps, req, p.SpaceID, p.CharacterID)
		if !ok {
			return rejected, nil
		}
		if deps.ProjectCreator == nil {
			return hostRouteUnavailable("project creator not configured"), nil
		}
		output, err := deps.ProjectCreator.CreateProject(ctx, ProjectCreateRequest{
			SpaceID:            scope.SpaceID,
			CharacterID:        scope.CharacterID,
			Title:              p.Title,
			DirectoryPath:      p.DirectoryPath,
			Workspaces:         p.Workspaces,
			PrimaryWorkspaceID: p.PrimaryWorkspaceID,
			RequestID:          p.RequestID,
		})
		if err != nil {
			return hostRouteInternal(err), nil
		}
		return hostRouteOutput(output, `{}`), nil
	}
}

func projectUpdateRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		var p struct {
			SpaceID            string                  `json:"spaceId"`
			CharacterID        string                  `json:"characterId"`
			ProjectID          string                  `json:"projectId"`
			Title              *string                 `json:"title"`
			Pinned             *bool                   `json:"pinned"`
			Hidden             *bool                   `json:"hidden"`
			Workspaces         []ProjectWorkspaceInput `json:"workspaces"`
			PrimaryWorkspaceID *string                 `json:"primaryWorkspaceId"`
			RequestID          string                  `json:"requestId"`
		}
		if err := json.Unmarshal(req.Input, &p); err != nil {
			return hostRouteInputInvalid(err.Error()), nil
		}
		p.SpaceID = strings.TrimSpace(p.SpaceID)
		p.CharacterID = strings.TrimSpace(p.CharacterID)
		p.ProjectID = strings.TrimSpace(p.ProjectID)
		p.RequestID = strings.TrimSpace(p.RequestID)
		if p.ProjectID == "" || len(p.ProjectID) > 512 || len(p.RequestID) > 128 {
			return hostRouteInputInvalid("projectId is required and project update input is too long"), nil
		}
		if p.Title == nil && p.Pinned == nil && p.Hidden == nil && p.Workspaces == nil && p.PrimaryWorkspaceID == nil {
			return hostRouteInputInvalid("project update requires at least one of title, pinned, hidden, workspaces or primaryWorkspaceId"), nil
		}
		if p.Title != nil {
			trimmed := strings.TrimSpace(*p.Title)
			if len(trimmed) > 1024 {
				return hostRouteInputInvalid("project title is too long"), nil
			}
			p.Title = &trimmed
		}
		if p.PrimaryWorkspaceID != nil {
			trimmed := strings.TrimSpace(*p.PrimaryWorkspaceID)
			if len(trimmed) > 512 {
				return hostRouteInputInvalid("project primaryWorkspaceId is too long"), nil
			}
			p.PrimaryWorkspaceID = &trimmed
		}
		if len(p.Workspaces) > 32 {
			return hostRouteInputInvalid("project workspaces are too many"), nil
		}
		ctx, _, rejected, ok := resolveProjectHostScope(ctx, deps, req, p.SpaceID, p.CharacterID)
		if !ok {
			return rejected, nil
		}
		if deps.ProjectUpdater == nil {
			return hostRouteUnavailable("project updater not configured"), nil
		}
		output, err := deps.ProjectUpdater.UpdateProject(ctx, ProjectUpdateRequest{
			ProjectID:          p.ProjectID,
			Title:              p.Title,
			Pinned:             p.Pinned,
			Hidden:             p.Hidden,
			Workspaces:         p.Workspaces,
			PrimaryWorkspaceID: p.PrimaryWorkspaceID,
			RequestID:          p.RequestID,
		})
		if err != nil {
			return hostRouteInternal(err), nil
		}
		return hostRouteOutput(output, `{}`), nil
	}
}

func projectDeleteRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		var p struct {
			SpaceID     string `json:"spaceId"`
			CharacterID string `json:"characterId"`
			ProjectID   string `json:"projectId"`
			RequestID   string `json:"requestId"`
		}
		if err := json.Unmarshal(req.Input, &p); err != nil {
			return hostRouteInputInvalid(err.Error()), nil
		}
		p.SpaceID = strings.TrimSpace(p.SpaceID)
		p.CharacterID = strings.TrimSpace(p.CharacterID)
		p.ProjectID = strings.TrimSpace(p.ProjectID)
		p.RequestID = strings.TrimSpace(p.RequestID)
		if p.ProjectID == "" || len(p.ProjectID) > 512 || len(p.RequestID) > 128 {
			return hostRouteInputInvalid("projectId is required and project delete input is too long"), nil
		}
		ctx, _, rejected, ok := resolveProjectHostScope(ctx, deps, req, p.SpaceID, p.CharacterID)
		if !ok {
			return rejected, nil
		}
		if deps.ProjectDeleter == nil {
			return hostRouteUnavailable("project deleter not configured"), nil
		}
		output, err := deps.ProjectDeleter.DeleteProject(ctx, ProjectDeleteRequest{ProjectID: p.ProjectID, RequestID: p.RequestID})
		if err != nil {
			return hostRouteInternal(err), nil
		}
		return hostRouteOutput(output, `{"deleted":true}`), nil
	}
}

func conversationArchiveRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		var p struct {
			SpaceID        string `json:"spaceId"`
			CharacterID    string `json:"characterId"`
			ConversationID string `json:"conversationId"`
			Archived       *bool  `json:"archived"`
			StopActivity   bool   `json:"stopActivity"`
			RequestID      string `json:"requestId"`
		}
		if err := json.Unmarshal(req.Input, &p); err != nil {
			return hostRouteInputInvalid(err.Error()), nil
		}
		p.SpaceID = strings.TrimSpace(p.SpaceID)
		p.CharacterID = strings.TrimSpace(p.CharacterID)
		p.ConversationID = strings.TrimSpace(p.ConversationID)
		p.RequestID = strings.TrimSpace(p.RequestID)
		if p.Archived == nil || p.ConversationID == "" || len(p.ConversationID) > 512 || len(p.RequestID) > 128 {
			return hostRouteInputInvalid("conversationId and archived are required and conversation archive input is too long"), nil
		}
		ctx = resolveHostAPIScope(ctx, deps.ScopeSnapshotStore, req.ScopeSnapshotID)
		scope := GetHostAPIScope(ctx)
		targetScope, targetErr := resolveConversationTargetScope(ctx, deps, req, p.ConversationID)
		if targetErr != nil {
			return hostRouteScopeDenied(targetErr.Error()), nil
		}
		scope = targetScope
		if req.SpaceID != "" {
			if scope.SpaceID != "" && scope.SpaceID != req.SpaceID {
				return hostRouteScopeDenied("spaceId does not match current scope"), nil
			}
			scope.SpaceID = req.SpaceID
		}
		if req.CharacterID != "" {
			if scope.CharacterID != "" && scope.CharacterID != req.CharacterID {
				return hostRouteScopeDenied("characterId does not match current scope"), nil
			}
			scope.CharacterID = req.CharacterID
		}
		if p.SpaceID != "" && scope.SpaceID != "" && p.SpaceID != scope.SpaceID {
			return hostRouteScopeDenied("spaceId does not match current scope"), nil
		}
		if p.CharacterID != "" && scope.CharacterID != "" && p.CharacterID != scope.CharacterID {
			return hostRouteScopeDenied("characterId does not match current scope"), nil
		}
		if scope.SpaceID == "" {
			scope.SpaceID = p.SpaceID
		}
		if scope.CharacterID == "" {
			scope.CharacterID = p.CharacterID
		}
		if scope.SpaceID == "" || scope.CharacterID == "" {
			return hostRouteInputInvalid("spaceId and characterId are required"), nil
		}
		scope.ConversationID = p.ConversationID
		ctx = WithHostAPIScope(ctx, scope)
		if deps.ConversationArchiver == nil {
			return hostRouteUnavailable("conversation archiver not configured"), nil
		}
		if p.StopActivity && *p.Archived {
			if deps.ConversationTurnController == nil {
				return hostRouteUnavailable("conversation turn controller not configured"), nil
			}
			if _, err := deps.ConversationTurnController.InterruptConversationTurn(ctx, ConversationTurnInterruptRequest{SpaceID: scope.SpaceID, CharacterID: scope.CharacterID, ConversationID: p.ConversationID}); err != nil && !errors.Is(err, ErrConversationTurnUnavailable) {
				return hostRouteInternal(err), nil
			}
		}
		output, err := deps.ConversationArchiver.ArchiveConversation(ctx, ConversationArchiveRequest{
			SpaceID:        scope.SpaceID,
			CharacterID:    scope.CharacterID,
			ConversationID: p.ConversationID,
			Archived:       *p.Archived,
			RequestID:      p.RequestID,
		})
		if err != nil {
			return hostRouteInternal(err), nil
		}
		archivedJSON := "false"
		if *p.Archived {
			archivedJSON = "true"
		}
		return hostRouteOutput(output, `{"conversationId":"`+p.ConversationID+`","archived":`+archivedJSON+`}`), nil
	}
}

func conversationForkRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		var p struct {
			SpaceID          string `json:"spaceId"`
			CharacterID      string `json:"characterId"`
			ConversationID   string `json:"conversationId"`
			BoundarySequence int64  `json:"boundarySequence"`
			Title            string `json:"title"`
			RequestID        string `json:"requestId"`
		}
		if err := json.Unmarshal(req.Input, &p); err != nil {
			return hostRouteInputInvalid(err.Error()), nil
		}
		p.SpaceID = strings.TrimSpace(p.SpaceID)
		p.CharacterID = strings.TrimSpace(p.CharacterID)
		p.ConversationID = strings.TrimSpace(p.ConversationID)
		p.RequestID = strings.TrimSpace(p.RequestID)
		p.Title = strings.TrimSpace(p.Title)
		if p.ConversationID == "" || len(p.ConversationID) > 512 || len(p.Title) > 200 || p.BoundarySequence < 0 {
			return hostRouteInputInvalid("conversationId is required and fork input is too long"), nil
		}
		ctx = resolveHostAPIScope(ctx, deps.ScopeSnapshotStore, req.ScopeSnapshotID)
		scope := GetHostAPIScope(ctx)
		targetScope, targetErr := resolveConversationTargetScope(ctx, deps, req, p.ConversationID)
		if targetErr != nil {
			return hostRouteScopeDenied(targetErr.Error()), nil
		}
		scope = targetScope
		if p.SpaceID != "" && scope.SpaceID != "" && p.SpaceID != scope.SpaceID {
			return hostRouteScopeDenied("spaceId does not match current scope"), nil
		}
		if p.CharacterID != "" && scope.CharacterID != "" && p.CharacterID != scope.CharacterID {
			return hostRouteScopeDenied("characterId does not match current scope"), nil
		}
		if scope.SpaceID == "" || scope.CharacterID == "" {
			return hostRouteInputInvalid("spaceId and characterId are required"), nil
		}
		scope.ConversationID = p.ConversationID
		ctx = WithHostAPIScope(ctx, scope)
		if deps.ConversationForker == nil {
			return hostRouteUnavailable("conversation forker not configured"), nil
		}
		output, err := deps.ConversationForker.ForkConversation(ctx, ConversationForkRequest{
			SpaceID:          scope.SpaceID,
			CharacterID:      scope.CharacterID,
			ConversationID:   p.ConversationID,
			BoundarySequence: p.BoundarySequence,
			Title:            p.Title,
			RequestID:        p.RequestID,
		})
		if err != nil {
			return hostRouteInternal(err), nil
		}
		return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
	}
}

func modelDefaultsRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		if deps.ModelDefaults == nil {
			return hostRouteUnavailable("model defaults provider not configured"), nil
		}
		defaults, err := deps.ModelDefaults.ModelDefaults(ctx)
		if err != nil {
			return hostRouteInternal(err), nil
		}
		output, err := json.Marshal(defaults)
		if err != nil {
			return hostRouteInternal(err), nil
		}
		return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
	}
}

func skillListRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		var p struct {
			Query   string `json:"query"`
			Enabled *bool  `json:"enabled"`
			Limit   int    `json:"limit"`
		}
		if len(req.Input) > 0 {
			if err := json.Unmarshal(req.Input, &p); err != nil {
				return hostRouteInputInvalid(err.Error()), nil
			}
		}
		if deps.SkillCatalog == nil {
			return hostRouteUnavailable("skill catalog not configured"), nil
		}
		ctx = resolveHostAPIScope(ctx, deps.ScopeSnapshotStore, req.ScopeSnapshotID)
		scope := GetHostAPIScope(ctx)
		limit := p.Limit
		if limit <= 0 {
			limit = 50
		}
		if limit > 200 {
			limit = 200
		}
		definitions := deps.SkillCatalog.List(agent_skill.CatalogFilter{
			Enabled: p.Enabled,
			Query:   strings.TrimSpace(p.Query),
			SpaceID: scope.SpaceID,
		})
		items := make([]map[string]any, 0, len(definitions))
		for _, definition := range definitions {
			items = append(items, map[string]any{
				"skillId":        definition.ID,
				"id":             definition.ID,
				"name":           definition.Name,
				"displayName":    definition.DisplayName,
				"description":    definition.Description,
				"version":        definition.Version,
				"extensionId":    definition.ExtensionID,
				"contributionId": definition.ContributionID,
				"enabled":        definition.Enabled,
			})
			if len(items) >= limit {
				break
			}
		}
		output, err := json.Marshal(map[string]any{"items": items})
		if err != nil {
			return hostRouteInternal(err), nil
		}
		return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
	}
}

func conversationPinRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		var p struct {
			SpaceID        string `json:"spaceId"`
			CharacterID    string `json:"characterId"`
			ConversationID string `json:"conversationId"`
			Pinned         *bool  `json:"pinned"`
			RequestID      string `json:"requestId"`
		}
		if err := json.Unmarshal(req.Input, &p); err != nil {
			return hostRouteInputInvalid(err.Error()), nil
		}
		p.SpaceID = strings.TrimSpace(p.SpaceID)
		p.CharacterID = strings.TrimSpace(p.CharacterID)
		p.ConversationID = strings.TrimSpace(p.ConversationID)
		p.RequestID = strings.TrimSpace(p.RequestID)
		if p.Pinned == nil || p.ConversationID == "" || len(p.ConversationID) > 512 || len(p.RequestID) > 128 {
			return hostRouteInputInvalid("conversationId and pinned are required and conversation pin input is too long"), nil
		}
		ctx = resolveHostAPIScope(ctx, deps.ScopeSnapshotStore, req.ScopeSnapshotID)
		scope := GetHostAPIScope(ctx)
		targetScope, targetErr := resolveConversationTargetScope(ctx, deps, req, p.ConversationID)
		if targetErr != nil {
			return hostRouteScopeDenied(targetErr.Error()), nil
		}
		scope = targetScope
		if req.SpaceID != "" {
			if scope.SpaceID != "" && scope.SpaceID != req.SpaceID {
				return hostRouteScopeDenied("spaceId does not match current scope"), nil
			}
			scope.SpaceID = req.SpaceID
		}
		if req.CharacterID != "" {
			if scope.CharacterID != "" && scope.CharacterID != req.CharacterID {
				return hostRouteScopeDenied("characterId does not match current scope"), nil
			}
			scope.CharacterID = req.CharacterID
		}
		if p.SpaceID != "" && scope.SpaceID != "" && p.SpaceID != scope.SpaceID {
			return hostRouteScopeDenied("spaceId does not match current scope"), nil
		}
		if p.CharacterID != "" && scope.CharacterID != "" && p.CharacterID != scope.CharacterID {
			return hostRouteScopeDenied("characterId does not match current scope"), nil
		}
		if scope.SpaceID == "" {
			scope.SpaceID = p.SpaceID
		}
		if scope.CharacterID == "" {
			scope.CharacterID = p.CharacterID
		}
		if scope.SpaceID == "" || scope.CharacterID == "" {
			return hostRouteInputInvalid("spaceId and characterId are required"), nil
		}
		scope.ConversationID = p.ConversationID
		ctx = WithHostAPIScope(ctx, scope)
		if deps.ConversationPinner == nil {
			return hostRouteUnavailable("conversation pinner not configured"), nil
		}
		output, err := deps.ConversationPinner.PinConversation(ctx, ConversationPinRequest{
			SpaceID:        scope.SpaceID,
			CharacterID:    scope.CharacterID,
			ConversationID: p.ConversationID,
			Pinned:         *p.Pinned,
			RequestID:      p.RequestID,
		})
		if err != nil {
			return hostRouteInternal(err), nil
		}
		pinnedJSON := "false"
		if *p.Pinned {
			pinnedJSON = "true"
		}
		return hostRouteOutput(output, `{"conversationId":"`+p.ConversationID+`","pinned":`+pinnedJSON+`}`), nil
	}
}

func directoryListRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		var p struct {
			Path  string `json:"path"`
			Limit int    `json:"limit"`
		}
		if len(req.Input) > 0 {
			if err := json.Unmarshal(req.Input, &p); err != nil {
				return hostRouteInputInvalid(err.Error()), nil
			}
		}
		path, err := resolveHostDirectoryPath(p.Path)
		if err != nil {
			return hostRouteInputInvalid(err.Error()), nil
		}
		limit := p.Limit
		if limit <= 0 {
			limit = hostDirectoryDefaultLimit
		}
		if limit > hostDirectoryMaxLimit {
			limit = hostDirectoryMaxLimit
		}
		if deps.DirectoryLister == nil {
			return hostRouteUnavailable("directory lister not configured"), nil
		}
		result, err := deps.DirectoryLister.ListDirectory(ctx, DirectoryListRequest{Path: path, Limit: limit})
		if err != nil {
			return hostRouteInternal(err), nil
		}
		result.Path = path
		output, err := json.Marshal(result)
		if err != nil {
			return hostRouteInternal(err), nil
		}
		return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
	}
}

func directoryCreateRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		var p struct {
			Path string `json:"path"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(req.Input, &p); err != nil {
			return hostRouteInputInvalid(err.Error()), nil
		}
		if strings.TrimSpace(p.Path) == "" {
			return hostRouteInputInvalid("directory path is required"), nil
		}
		path, err := resolveHostDirectoryPath(p.Path)
		if err != nil {
			return hostRouteInputInvalid(err.Error()), nil
		}
		name := strings.TrimSpace(p.Name)
		if err := validateHostDirectoryName(name); err != nil {
			return hostRouteInputInvalid(err.Error()), nil
		}
		if deps.DirectoryCreator == nil {
			return hostRouteUnavailable("directory creator not configured"), nil
		}
		created, err := deps.DirectoryCreator.CreateDirectory(ctx, DirectoryCreateRequest{Path: path, Name: name})
		if err != nil {
			return hostRouteInternal(err), nil
		}
		output, err := json.Marshal(map[string]string{"path": created})
		if err != nil {
			return hostRouteInternal(err), nil
		}
		return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
	}
}

func resolveHostDirectoryPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		trimmed = home
	}
	if !filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("directory path must be absolute: %s", trimmed)
	}
	for _, segment := range strings.FieldsFunc(trimmed, func(r rune) bool { return r == '/' || r == '\\' }) {
		if segment == ".." {
			return "", fmt.Errorf("directory path must not contain parent traversal: %s", trimmed)
		}
	}
	return filepath.Clean(trimmed), nil
}

func validateHostDirectoryName(name string) error {
	if name == "" {
		return nil
	}
	if name == "." || name == ".." {
		return fmt.Errorf("directory name must not be a traversal segment: %s", name)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("directory name must not contain path separators: %s", name)
	}
	return nil
}

func conversationTurnInterruptRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		var p struct {
			SpaceID        string `json:"spaceId"`
			CharacterID    string `json:"characterId"`
			ConversationID string `json:"conversationId"`
			TurnID         string `json:"turnId"`
		}
		if err := json.Unmarshal(req.Input, &p); err != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
			}, nil
		}
		p.SpaceID = strings.TrimSpace(p.SpaceID)
		p.CharacterID = strings.TrimSpace(p.CharacterID)
		p.ConversationID = strings.TrimSpace(p.ConversationID)
		p.TurnID = strings.TrimSpace(p.TurnID)
		if len(p.ConversationID) > 512 || len(p.TurnID) > 512 {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "conversationId or turnId is too long"},
			}, nil
		}
		ctx = resolveHostAPIScope(ctx, deps.ScopeSnapshotStore, req.ScopeSnapshotID)
		scope := GetHostAPIScope(ctx)
		targetScope, targetErr := resolveConversationTargetScope(ctx, deps, req, p.ConversationID)
		if targetErr != nil {
			return host_api.CallResult{
				Status: host_api.StatusRejected,
				Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: targetErr.Error()},
			}, nil
		}
		scope = targetScope
		if req.SpaceID != "" {
			if scope.SpaceID != "" && scope.SpaceID != req.SpaceID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "spaceId does not match current scope"},
				}, nil
			}
			scope.SpaceID = req.SpaceID
		}
		if req.CharacterID != "" {
			if scope.CharacterID != "" && scope.CharacterID != req.CharacterID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "characterId does not match current scope"},
				}, nil
			}
			scope.CharacterID = req.CharacterID
		}
		if req.ConversationID != "" && req.TargetScopeSnapshotID == "" {
			if scope.ConversationID != "" && scope.ConversationID != req.ConversationID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "conversationId does not match current scope"},
				}, nil
			}
			scope.ConversationID = req.ConversationID
		}
		ctx = WithHostAPIScope(ctx, scope)
		if scope.SpaceID != "" {
			if p.SpaceID != "" && p.SpaceID != scope.SpaceID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "spaceId does not match current scope"},
				}, nil
			}
			p.SpaceID = scope.SpaceID
		}
		if scope.CharacterID != "" {
			if p.CharacterID != "" && p.CharacterID != scope.CharacterID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "characterId does not match current scope"},
				}, nil
			}
			p.CharacterID = scope.CharacterID
		}
		if scope.ConversationID != "" {
			if p.ConversationID != "" && p.ConversationID != scope.ConversationID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "conversationId does not match current scope"},
				}, nil
			}
			p.ConversationID = scope.ConversationID
		}
		if p.SpaceID == "" || p.CharacterID == "" || p.ConversationID == "" {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "spaceId, characterId and conversationId are required"},
			}, nil
		}
		if deps.ConversationTurnController == nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "conversation turn controller not configured"},
			}, nil
		}
		result, err := deps.ConversationTurnController.InterruptConversationTurn(ctx, ConversationTurnInterruptRequest{
			SpaceID:        p.SpaceID,
			CharacterID:    p.CharacterID,
			ConversationID: p.ConversationID,
			TurnID:         p.TurnID,
		})
		if err != nil {
			status, code := host_api.StatusFailed, host_api.ErrorCodeInternal
			if errors.Is(err, ErrConversationTurnUnavailable) {
				status, code = host_api.StatusRejected, host_api.ErrorCodeStateConflict
			}
			return host_api.CallResult{
				Status: status,
				Error:  &host_api.Error{Code: code, Message: err.Error()},
			}, nil
		}
		if !result.Interrupted {
			return host_api.CallResult{
				Status: host_api.StatusRejected,
				Error:  &host_api.Error{Code: host_api.ErrorCodeStateConflict, Message: ErrConversationTurnUnavailable.Error()},
			}, nil
		}
		output, err := json.Marshal(result)
		if err != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeOutputInvalid, Message: err.Error()},
			}, nil
		}
		return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
	}
}

func conversationTurnAwaitRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		var p struct {
			SpaceID        string `json:"spaceId"`
			CharacterID    string `json:"characterId"`
			ConversationID string `json:"conversationId"`
			TurnID         string `json:"turnId"`
		}
		if err := json.Unmarshal(req.Input, &p); err != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
			}, nil
		}
		p.SpaceID = strings.TrimSpace(p.SpaceID)
		p.CharacterID = strings.TrimSpace(p.CharacterID)
		p.ConversationID = strings.TrimSpace(p.ConversationID)
		p.TurnID = strings.TrimSpace(p.TurnID)
		if len(p.ConversationID) > 512 || len(p.TurnID) > 512 {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "conversationId or turnId is too long"},
			}, nil
		}
		ctx = resolveHostAPIScope(ctx, deps.ScopeSnapshotStore, req.ScopeSnapshotID)
		scope := GetHostAPIScope(ctx)
		targetScope, targetErr := resolveConversationTargetScope(ctx, deps, req, p.ConversationID)
		if targetErr != nil {
			return host_api.CallResult{
				Status: host_api.StatusRejected,
				Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: targetErr.Error()},
			}, nil
		}
		scope = targetScope
		if req.SpaceID != "" {
			if scope.SpaceID != "" && scope.SpaceID != req.SpaceID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "spaceId does not match current scope"},
				}, nil
			}
			scope.SpaceID = req.SpaceID
		}
		if req.CharacterID != "" {
			if scope.CharacterID != "" && scope.CharacterID != req.CharacterID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "characterId does not match current scope"},
				}, nil
			}
			scope.CharacterID = req.CharacterID
		}
		if req.ConversationID != "" && req.TargetScopeSnapshotID == "" {
			if scope.ConversationID != "" && scope.ConversationID != req.ConversationID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "conversationId does not match current scope"},
				}, nil
			}
			scope.ConversationID = req.ConversationID
		}
		ctx = WithHostAPIScope(ctx, scope)
		if scope.SpaceID != "" {
			if p.SpaceID != "" && p.SpaceID != scope.SpaceID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "spaceId does not match current scope"},
				}, nil
			}
			p.SpaceID = scope.SpaceID
		}
		if scope.CharacterID != "" {
			if p.CharacterID != "" && p.CharacterID != scope.CharacterID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "characterId does not match current scope"},
				}, nil
			}
			p.CharacterID = scope.CharacterID
		}
		if scope.ConversationID != "" {
			if p.ConversationID != "" && p.ConversationID != scope.ConversationID {
				return host_api.CallResult{
					Status: host_api.StatusRejected,
					Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "conversationId does not match current scope"},
				}, nil
			}
			p.ConversationID = scope.ConversationID
		}
		if p.SpaceID == "" || p.CharacterID == "" || p.ConversationID == "" {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "spaceId, characterId and conversationId are required"},
			}, nil
		}
		if deps.ConversationTurnController == nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "conversation turn controller not configured"},
			}, nil
		}
		result, err := deps.ConversationTurnController.AwaitConversationTurnIdle(ctx, ConversationTurnInterruptRequest{
			SpaceID:        p.SpaceID,
			CharacterID:    p.CharacterID,
			ConversationID: p.ConversationID,
			TurnID:         p.TurnID,
		})
		if err != nil {
			status, code := host_api.StatusFailed, host_api.ErrorCodeInternal
			if errors.Is(err, ErrConversationTurnUnavailable) {
				status, code = host_api.StatusRejected, host_api.ErrorCodeStateConflict
			}
			return host_api.CallResult{
				Status: status,
				Error:  &host_api.Error{Code: code, Message: err.Error()},
			}, nil
		}
		if !result.Idle {
			return host_api.CallResult{
				Status: host_api.StatusRejected,
				Error:  &host_api.Error{Code: host_api.ErrorCodeStateConflict, Message: "conversation did not become idle"},
			}, nil
		}
		output, err := json.Marshal(result)
		if err != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeOutputInvalid, Message: err.Error()},
			}, nil
		}
		return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
	}
}

func extensionVectorNamespace(req host_api.CallRequest) string {
	return string(req.RuntimeIdentity.ExtensionID) + "/" + string(req.RuntimeIdentity.ModuleID)
}

func validVectorCollection(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 64 {
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_' || char == '-' || char == '.' {
			continue
		}
		return false
	}
	return true
}

func vectorUpsertRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		if deps.VectorStore == nil {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "vector store not configured"}}, nil
		}
		var p struct {
			Collection string                 `json:"collection"`
			Points     []ExtensionVectorPoint `json:"points"`
		}
		if err := json.Unmarshal(req.Input, &p); err != nil {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()}}, nil
		}
		if !validVectorCollection(p.Collection) {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "invalid vector collection"}}, nil
		}
		if len(p.Points) == 0 || len(p.Points) > 128 {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "points must contain between 1 and 128 items"}}, nil
		}
		for index := range p.Points {
			p.Points[index].ID = strings.TrimSpace(p.Points[index].ID)
			p.Points[index].Text = strings.TrimSpace(p.Points[index].Text)
			if p.Points[index].ID == "" || len(p.Points[index].ID) > 256 {
				return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: fmt.Sprintf("point %d id is invalid", index)}}, nil
			}
			if len(p.Points[index].Vector) > 8192 {
				return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: fmt.Sprintf("point %d vector is too large", index)}}, nil
			}
			if len(p.Points[index].Vector) == 0 && p.Points[index].Text == "" {
				return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: fmt.Sprintf("point %d requires vector or text", index)}}, nil
			}
		}
		if err := deps.VectorStore.Upsert(ctx, extensionVectorNamespace(req), p.Collection, p.Points); err != nil {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()}}, nil
		}
		output, _ := json.Marshal(map[string]any{"upserted": len(p.Points)})
		return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
	}
}

func vectorSearchRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		if deps.VectorStore == nil {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "vector store not configured"}}, nil
		}
		var p struct {
			Collection string            `json:"collection"`
			Query      string            `json:"query"`
			Vector     []float32         `json:"vector"`
			Limit      int               `json:"limit"`
			Filter     map[string]string `json:"filter"`
		}
		if err := json.Unmarshal(req.Input, &p); err != nil {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()}}, nil
		}
		if !validVectorCollection(p.Collection) {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "invalid vector collection"}}, nil
		}
		if len(p.Vector) == 0 && strings.TrimSpace(p.Query) == "" {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "query or vector is required"}}, nil
		}
		if p.Limit <= 0 || p.Limit > 100 {
			p.Limit = 10
		}
		results, err := deps.VectorStore.Search(ctx, extensionVectorNamespace(req), p.Collection, p.Query, p.Vector, p.Limit, p.Filter)
		if err != nil {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()}}, nil
		}
		output, _ := json.Marshal(map[string]any{"items": results})
		return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
	}
}

func vectorDeleteRouteHandler(deps HostAPIRouteDeps) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		if deps.VectorStore == nil {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "vector store not configured"}}, nil
		}
		var p struct {
			Collection string   `json:"collection"`
			IDs        []string `json:"ids"`
		}
		if err := json.Unmarshal(req.Input, &p); err != nil {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()}}, nil
		}
		if !validVectorCollection(p.Collection) {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "invalid vector collection"}}, nil
		}
		if len(p.IDs) == 0 || len(p.IDs) > 1000 {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "ids must contain between 1 and 1000 items"}}, nil
		}
		if err := deps.VectorStore.Delete(ctx, extensionVectorNamespace(req), p.Collection, p.IDs); err != nil {
			return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()}}, nil
		}
		output, _ := json.Marshal(map[string]any{"deleted": len(p.IDs)})
		return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
	}
}

func setupDefaultHostAPIRoutes(gateway *host_api.DefaultGateway, deps HostAPIRouteDeps) error {
	handleTable := newResourceHandleTable()
	conversationMessageHandler := conversationMessageRouteHandler(deps)
	conversationMessageAppendHandler := conversationMessageAppendRouteHandler(deps)
	conversationCreateHandler := conversationCreateRouteHandler(deps)
	conversationTurnInterruptHandler := conversationTurnInterruptRouteHandler(deps)
	conversationTurnAwaitHandler := conversationTurnAwaitRouteHandler(deps)
	conversationArchiveHandler := conversationArchiveRouteHandler(deps)
	projectListHandler := projectListRouteHandler(deps)
	projectCreateHandler := projectCreateRouteHandler(deps)
	projectUpdateHandler := projectUpdateRouteHandler(deps)
	projectDeleteHandler := projectDeleteRouteHandler(deps)
	vectorUpsertHandler := vectorUpsertRouteHandler(deps)
	vectorSearchHandler := vectorSearchRouteHandler(deps)
	vectorDeleteHandler := vectorDeleteRouteHandler(deps)

	type routeDef struct {
		method          host_api.Method
		riskLevel       host_api.RiskLevel
		sideEffectLevel host_api.SideEffectLevel
		timeout         time.Duration
		handler         host_api.Handler
	}

	routes := []routeDef{
		{
			method:          host_api.MethodStateGet,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					Key string `json:"key"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.StateStore == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "state store not configured"},
					}, nil
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				modID := string(req.RuntimeIdentity.ModuleID)
				value, version, found, err := deps.StateStore.Get(ctx, extID, modID, p.Key)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"key":     p.Key,
					"value":   json.RawMessage(value),
					"found":   found,
					"version": version,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodStateCAS,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					Key             string          `json:"key"`
					ExpectedVersion int64           `json:"expectedVersion"`
					NewValue        json.RawMessage `json:"newValue"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.StateStore == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "state store not configured"},
					}, nil
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				modID := string(req.RuntimeIdentity.ModuleID)
				swapped, newVersion, err := deps.StateStore.CAS(ctx, extID, modID, p.Key, p.ExpectedVersion, p.NewValue)
				if err != nil {
					if errors.Is(err, sqlite.ErrExtensionStateQuotaExceeded) {
						return host_api.CallResult{
							Status: host_api.StatusRejected,
							Error:  &host_api.Error{Code: host_api.ErrorCodeQuotaExceeded, Message: err.Error()},
						}, nil
					}
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"swapped": swapped,
					"version": newVersion,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodStateDelete,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					Key string `json:"key"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.StateStore == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "state store not configured"},
					}, nil
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				modID := string(req.RuntimeIdentity.ModuleID)
				if err := deps.StateStore.Delete(ctx, extID, modID, p.Key); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{"ok": true})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodStateList,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.StateStore == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "state store not configured"},
					}, nil
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				modID := string(req.RuntimeIdentity.ModuleID)
				entries, err := deps.StateStore.List(ctx, extID, modID)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				keys := make([]map[string]any, 0, len(entries))
				for _, e := range entries {
					keys = append(keys, map[string]any{
						"key":     e.Key,
						"value":   json.RawMessage(e.Value),
						"version": e.Version,
					})
				}
				output, _ := json.Marshal(map[string]any{
					"keys":  keys,
					"total": len(keys),
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodResourceOpen,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					Path  string `json:"path"`
					Mode  string `json:"mode"`
					Scope string `json:"scope"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.ExtensionRoot == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "extension root not configured"},
					}, nil
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				readOnly := p.Mode == "r" || p.Mode == "read"
				var realPath string
				var err error
				if strings.EqualFold(strings.TrimSpace(p.Scope), ResourceLinkScopeData) {
					if deps.ResourceLinks == nil {
						return host_api.CallResult{
							Status: host_api.StatusFailed,
							Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "resource link manager not configured"},
						}, nil
					}
					realPath, err = deps.ResourceLinks.DataPath(extID, p.Path, !readOnly)
				} else {
					extDir := resolveExtensionBundlePath(deps.ExtensionRoot, extID)
					if extDir == "" {
						return host_api.CallResult{
							Status: host_api.StatusFailed,
							Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: "extension package root not found"},
						}, nil
					}
					realPath = filepath.Join(extDir, p.Path)
					if !isPathSafe(extDir, realPath) {
						realPath = ""
						err = errors.New("path escapes extension directory")
					}
				}
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				var f *os.File
				if readOnly {
					f, err = os.Open(realPath)
				} else {
					flags := os.O_RDWR | os.O_CREATE
					if strings.EqualFold(strings.TrimSpace(p.Mode), "a") || strings.EqualFold(strings.TrimSpace(p.Mode), "append") {
						flags |= os.O_APPEND
					} else {
						flags |= os.O_TRUNC
					}
					f, err = os.OpenFile(realPath, flags, 0644)
				}
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: err.Error()},
					}, nil
				}
				handleID := fmt.Sprintf("res-%s", uuid.NewString())
				h := &resourceHandle{
					handleID:        handleID,
					realPath:        realPath,
					file:            f,
					readOnly:        readOnly,
					extensionID:     extID,
					moduleID:        string(req.RuntimeIdentity.ModuleID),
					scopeSnapshotID: req.ScopeSnapshotID,
					generation:      req.RuntimeIdentity.Generation,
					owner:           extID,
					expiresAt:       time.Now().UTC().Add(resourceHandleTTL),
				}
				handleTable.put(h)
				output, _ := json.Marshal(map[string]any{
					"handleId":  handleID,
					"path":      p.Path,
					"mode":      p.Mode,
					"expiresAt": h.expiresAt.Format(time.RFC3339),
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodResourceRead,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         10 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					HandleID string `json:"handleId"`
					Length   int    `json:"length"`
					Encoding string `json:"encoding"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				h, ok := handleTable.get(p.HandleID)
				if !ok {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: "handle not found"},
					}, nil
				}
				if h.extensionID != string(req.RuntimeIdentity.ExtensionID) {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: "handle does not belong to caller"},
					}, nil
				}
				if !h.expiresAt.IsZero() && time.Now().UTC().After(h.expiresAt) {
					if h.file != nil {
						if err := h.file.Close(); err != nil {
							log.Printf("[original-error] %v", err)
						}
					}
					handleTable.remove(p.HandleID)
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: "handle expired"},
					}, nil
				}
				if p.Length <= 0 || p.Length > 1024*1024 {
					p.Length = 4096
				}
				buf := make([]byte, p.Length)
				n, err := h.file.Read(buf)
				eof := false
				if err != nil {
					if errors.Is(err, io.EOF) {
						eof = true
					} else {
						return host_api.CallResult{
							Status: host_api.StatusFailed,
							Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
						}, nil
					}
				}
				if n < p.Length {
					eof = true
				}
				data := string(buf[:n])
				encoding := strings.ToLower(strings.TrimSpace(p.Encoding))
				if encoding == "base64" {
					data = base64.StdEncoding.EncodeToString(buf[:n])
				}
				output, _ := json.Marshal(map[string]any{
					"data":     data,
					"eof":      eof,
					"encoding": encoding,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodResourceWrite,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         10 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					HandleID string `json:"handleId"`
					Data     string `json:"data"`
					Encoding string `json:"encoding"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				h, ok := handleTable.get(p.HandleID)
				if !ok {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: "handle not found"},
					}, nil
				}
				if h.extensionID != string(req.RuntimeIdentity.ExtensionID) {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: "handle does not belong to caller"},
					}, nil
				}
				if h.readOnly {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: "handle is read-only"},
					}, nil
				}
				data := []byte(p.Data)
				if strings.EqualFold(strings.TrimSpace(p.Encoding), "base64") {
					decoded, err := base64.StdEncoding.DecodeString(p.Data)
					if err != nil {
						return host_api.CallResult{
							Status: host_api.StatusFailed,
							Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
						}, nil
					}
					data = decoded
				}
				n, err := h.file.Write(data)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"written": n,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodResourceClose,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectNone,
			timeout:         3 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					HandleID string `json:"handleId"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				h, ok := handleTable.remove(p.HandleID)
				if !ok {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: "handle not found"},
					}, nil
				}
				if h.extensionID != string(req.RuntimeIdentity.ExtensionID) {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: "handle does not belong to caller"},
					}, nil
				}
				if h.file != nil {
					if err := h.file.Close(); err != nil {
						log.Printf("[original-error] %v", err)
					}
				}
				output, _ := json.Marshal(map[string]any{"ok": true})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodResourceStat,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         3 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					Path  string `json:"path"`
					Scope string `json:"scope"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.ExtensionRoot == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "extension root not configured"},
					}, nil
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				var realPath string
				var err error
				if strings.EqualFold(strings.TrimSpace(p.Scope), ResourceLinkScopeData) {
					if deps.ResourceLinks == nil {
						return host_api.CallResult{
							Status: host_api.StatusFailed,
							Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "resource link manager not configured"},
						}, nil
					}
					realPath, err = deps.ResourceLinks.DataPath(extID, p.Path, false)
				} else {
					extDir := resolveExtensionBundlePath(deps.ExtensionRoot, extID)
					realPath = filepath.Join(extDir, p.Path)
					if extDir == "" || !isPathSafe(extDir, realPath) {
						err = errors.New("path escapes extension directory")
					}
				}
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				info, err := os.Stat(realPath)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"path":    p.Path,
					"size":    info.Size(),
					"isDir":   info.IsDir(),
					"modTime": info.ModTime().UTC().Format(time.RFC3339),
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodResourceLink,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectNone,
			timeout:         3 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.ResourceLinks == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "resource link manager not configured"},
					}, nil
				}
				var p struct {
					Path  string `json:"path"`
					Scope string `json:"scope"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				url, err := deps.ResourceLinks.Sign(string(req.RuntimeIdentity.ExtensionID), p.Scope, p.Path)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{"url": url})
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
			},
		},
		{
			method:          host_api.MethodResourceDelete,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.ResourceLinks == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "resource link manager not configured"},
					}, nil
				}
				var p struct {
					Path  string `json:"path"`
					Scope string `json:"scope"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if !strings.EqualFold(strings.TrimSpace(p.Scope), ResourceLinkScopeData) {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: "only extension data resources can be deleted"},
					}, nil
				}
				realPath, err := deps.ResourceLinks.DataPath(string(req.RuntimeIdentity.ExtensionID), p.Path, false)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				info, err := os.Stat(realPath)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: err.Error()},
					}, nil
				}
				if info.IsDir() {
					err = os.RemoveAll(realPath)
				} else {
					err = os.Remove(realPath)
				}
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{"deleted": true})
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
			},
		},
		{
			method:          host_api.MethodCharacterRead,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					CharacterID string `json:"characterId"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.CharacterReader == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "character reader not configured"},
					}, nil
				}
				ctx = resolveHostAPIScope(ctx, deps.ScopeSnapshotStore, req.ScopeSnapshotID)
				data, available, err := deps.CharacterReader.ReadCharacter(ctx, p.CharacterID)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"characterId": p.CharacterID,
					"available":   available,
					"data":        json.RawMessage(data),
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodCharacterList,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.CharacterLister == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "character lister not configured"},
					}, nil
				}
				var p struct {
					IncludeDisabled bool `json:"includeDisabled"`
				}
				if len(req.Input) > 0 {
					if err := json.Unmarshal(req.Input, &p); err != nil {
						return host_api.CallResult{
							Status: host_api.StatusFailed,
							Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
						}, nil
					}
				}
				ctx = resolveHostAPIScope(ctx, deps.ScopeSnapshotStore, req.ScopeSnapshotID)
				scope := GetHostAPIScope(ctx)
				items, err := deps.CharacterLister.ListCharacters(ctx, scope.SpaceID, p.IncludeDisabled)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{"items": items})
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
			},
		},
		{
			method:          host_api.MethodConversationList,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         10 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					Limit    int  `json:"limit"`
					Offset   int  `json:"offset"`
					Archived bool `json:"archived"`
				}
				if len(req.Input) > 0 {
					if err := json.Unmarshal(req.Input, &p); err != nil {
						return host_api.CallResult{
							Status: host_api.StatusFailed,
							Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
						}, nil
					}
				}
				if deps.ConversationLister == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "conversation lister not configured"},
					}, nil
				}
				if p.Limit <= 0 {
					p.Limit = 50
				}
				if p.Limit > 128 {
					p.Limit = 128
				}
				if p.Offset < 0 {
					p.Offset = 0
				}
				ctx = resolveHostAPIScope(ctx, deps.ScopeSnapshotStore, req.ScopeSnapshotID)
				result, err := deps.ConversationLister.ListConversations(ctx, p.Limit, p.Offset, p.Archived)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, err := json.Marshal(result)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
			},
		},
		{
			method:          host_api.MethodConversationScope,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					ConversationID string `json:"conversationId"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				conversationID := strings.TrimSpace(p.ConversationID)
				if conversationID == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "conversationId is required"},
					}, nil
				}
				scopeSnapshotID, err := createConversationTargetSnapshot(ctx, deps, req, conversationID)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, err := json.Marshal(map[string]any{
					"conversationId":  conversationID,
					"scopeSnapshotId": scopeSnapshotID,
				})
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
			},
		},
		{
			method:          host_api.MethodConversationRead,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					ConversationID string `json:"conversationId"`
					Limit          int    `json:"limit"`
					Offset         int    `json:"offset"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.ConversationReader == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "conversation reader not configured"},
					}, nil
				}
				if p.Limit <= 0 || p.Limit > 100 {
					p.Limit = 50
				}
				ctx = resolveHostAPIScope(ctx, deps.ScopeSnapshotStore, req.ScopeSnapshotID)
				targetScope, targetErr := resolveConversationTargetScope(ctx, deps, req, p.ConversationID)
				if targetErr != nil {
					return host_api.CallResult{
						Status: host_api.StatusRejected,
						Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: targetErr.Error()},
					}, nil
				}
				ctx = WithHostAPIScope(ctx, targetScope)
				msgs, hasMore, err := deps.ConversationReader.ReadConversation(ctx, p.ConversationID, p.Limit, p.Offset)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"messages": msgs,
					"hasMore":  hasMore,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodConversationSearch,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					Query string `json:"query"`
					Limit int    `json:"limit"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.ConversationSearcher == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "conversation searcher not configured"},
					}, nil
				}
				p.Query = strings.TrimSpace(p.Query)
				if p.Query == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "conversation search query is required"},
					}, nil
				}
				if p.Limit <= 0 || p.Limit > 20 {
					p.Limit = 20
				}
				ctx = resolveHostAPIScope(ctx, deps.ScopeSnapshotStore, req.ScopeSnapshotID)
				items, hasMore, err := deps.ConversationSearcher.SearchConversations(ctx, p.Query, p.Limit)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, err := json.Marshal(map[string]any{
					"items":   items,
					"hasMore": hasMore,
				})
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
			},
		},
		{
			method:          host_api.MethodMemoryQuery,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         10 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					Query string `json:"query"`
					Limit int    `json:"limit"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.MemoryQueryService == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "memory query service not configured"},
					}, nil
				}
				if p.Limit <= 0 || p.Limit > 50 {
					p.Limit = 10
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				ctx = resolveHostAPIScope(ctx, deps.ScopeSnapshotStore, req.ScopeSnapshotID)
				results, err := deps.MemoryQueryService.Query(ctx, extID, p.Query, p.Limit)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"results": results,
					"total":   len(results),
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodEventEmit,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					EventType string          `json:"eventType"`
					Payload   json.RawMessage `json:"payload"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.EventService == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "event service not configured"},
					}, nil
				}
				if p.EventType == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "eventType is required"},
					}, nil
				}
				opts := event.PublishOptions{
					ProducerID:    string(req.RuntimeIdentity.ExtensionID),
					ProducerType:  event.EventProducerTypeExtension,
					AggregateType: p.EventType,
					AggregateID:   req.CallID,
					TraceID:       req.TraceID,
				}
				pubResult, err := deps.EventService.Publish(ctx, event.EventTypeID(p.EventType), 1, p.Payload, opts)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"ok":        pubResult.Accepted,
					"eventType": p.EventType,
					"eventId":   pubResult.EventID,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodEventSubscribe,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectNone,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					EventType string `json:"eventType"`
					Entry     string `json:"entry"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.EventService == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "event service not configured"},
					}, nil
				}
				if p.EventType == "" || p.Entry == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "eventType and entry are required"},
					}, nil
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				modID := string(req.RuntimeIdentity.ModuleID)
				subID := fmt.Sprintf("sub-%s", uuid.NewString())
				def := event.EventSubscriptionDefinition{
					ContributionID: subID,
					ExtensionID:    extID,
					ModuleID:       modID,
					EventTypeID:    event.EventTypeID(p.EventType),
					Entry:          p.Entry,
					Enabled:        true,
					Generation:     req.RuntimeIdentity.Generation,
					CreatedAt:      time.Now().UTC(),
					UpdatedAt:      time.Now().UTC(),
				}
				if err := def.Validate(); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if err := deps.EventService.RegisterSubscription(ctx, def); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"ok":             true,
					"subscriptionId": subID,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodEventUnsubscribe,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectNone,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					SubscriptionID string `json:"subscriptionId"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.EventService == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "event service not configured"},
					}, nil
				}
				sub, ok := deps.EventService.GetSubscription(ctx, p.SubscriptionID)
				if !ok {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: "subscription not found"},
					}, nil
				}
				if sub.Definition.ExtensionID != string(req.RuntimeIdentity.ExtensionID) {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: "subscription does not belong to caller"},
					}, nil
				}
				if err := deps.EventService.UnregisterSubscription(ctx, p.SubscriptionID); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{"ok": true})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodScheduleCreate,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         10 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var def schedule.ScheduleContributionDefinition
				if err := json.Unmarshal(req.Input, &def); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.ScheduleService == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "schedule service not configured"},
					}, nil
				}
				def.ExtensionID = string(req.RuntimeIdentity.ExtensionID)
				def.ModuleID = string(req.RuntimeIdentity.ModuleID)
				if def.ContributionID == "" {
					def.ContributionID = fmt.Sprintf("sched-contrib-%s", uuid.NewString())
				}
				if def.Target.Type == schedule.TargetTypeSessionPrompt {
					var targetMetadata struct {
						TargetKind        string `json:"dshTargetKind"`
						ReservedSessionID string `json:"dshReservedSessionId"`
						ProjectID         string `json:"dshProjectId"`
					}
					_ = json.Unmarshal(def.Target.InputTemplate, &targetMetadata)
					targetKind := strings.TrimSpace(targetMetadata.TargetKind)
					reservedSessionID := strings.TrimSpace(targetMetadata.ReservedSessionID)
					if targetKind == "" {
						targetKind = "session"
					}
					if targetKind != "session" && targetKind != "new_once" && targetKind != "new" {
						return host_api.CallResult{
							Status: host_api.StatusFailed,
							Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: fmt.Sprintf("unsupported session prompt target kind %q", targetKind)},
						}, nil
					}
					if deps.ScopeSnapshotStore == nil || req.ScopeSnapshotID == "" {
						return host_api.CallResult{
							Status: host_api.StatusFailed,
							Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "session prompt schedule requires an owner scope snapshot"},
						}, nil
					}
					snapshot, snapshotErr := deps.ScopeSnapshotStore.Get(ctx, req.ScopeSnapshotID)
					if snapshotErr != nil || snapshot == nil {
						message := "owner scope snapshot not found"
						if snapshotErr != nil {
							message = snapshotErr.Error()
						}
						return host_api.CallResult{
							Status: host_api.StatusFailed,
							Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: message},
						}, nil
					}
					if snapshot.SpaceID == "" || snapshot.CharacterID == "" {
						return host_api.CallResult{
							Status: host_api.StatusFailed,
							Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "session prompt schedule requires space and character owner scope"},
						}, nil
					}
					if targetKind == "new_once" && (reservedSessionID == "" || reservedSessionID != def.Target.TargetID) {
						return host_api.CallResult{
							Status: host_api.StatusFailed,
							Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "new_once schedule requires its reserved session id as targetId"},
						}, nil
					}
					if targetKind == "session" && (snapshot.ConversationID == "" || snapshot.ConversationID != def.Target.TargetID) {
						return host_api.CallResult{
							Status: host_api.StatusFailed,
							Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "session prompt target does not match owner conversation"},
						}, nil
					}
					ownedExecutionScope := append(json.RawMessage(nil), snapshot.OwnedExecutionScope...)
					executionSpaceID := snapshot.SpaceID
					executionDeviceID := req.ExecutionContext.DeviceID.String()
					if actor, ok := auth.FromContext(ctx); ok && actor != nil {
						if actor.SpaceID.String() != "" {
							executionSpaceID = actor.SpaceID.String()
						}
						if actor.DeviceID.String() != "" {
							executionDeviceID = actor.DeviceID.String()
						}
					}
					if deps.UIExecutionIdentity != nil {
						uiSessionID := req.ExecutionContext.RuntimeSessionID.String()
						if uiSessionID == "" {
							uiSessionID = req.RuntimeIdentity.InstanceID
						}
						if uiSpaceID, uiDeviceID := deps.UIExecutionIdentity(uiSessionID); uiSpaceID != "" && uiDeviceID != "" {
							executionSpaceID = uiSpaceID
							executionDeviceID = uiDeviceID
						}
					}
					if (len(ownedExecutionScope) == 0 || strings.EqualFold(strings.TrimSpace(string(ownedExecutionScope)), "null")) && deps.Coordination != nil && executionSpaceID != "" && executionDeviceID != "" {
						requestID := strings.TrimSpace(req.InvocationID)
						if requestID == "" {
							requestID = req.CallID
						}
						_, owned, finish, resolveErr := deps.Coordination.BeginWithResourceOwner(ctx, executionSpaceID, executionDeviceID, executionDeviceID, executionSpaceID, snapshot.CharacterID, requestID, executionDeviceID)
						if resolveErr != nil {
							return host_api.CallResult{
								Status: host_api.StatusFailed,
								Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: fmt.Sprintf("resolve session prompt owner execution scope: %v", resolveErr)},
							}, nil
						}
						if deps.CharacterRevisionReader != nil {
							owned.RoleRevision, resolveErr = deps.CharacterRevisionReader(ctx, snapshot.CharacterID)
							if resolveErr != nil {
								finish()
								return host_api.CallResult{
									Status: host_api.StatusFailed,
									Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: fmt.Sprintf("resolve session prompt owner role revision: %v", resolveErr)},
								}, nil
							}
						}
						if resolveErr = coordination.ResolveMemoryScope(&owned, ""); resolveErr != nil {
							finish()
							return host_api.CallResult{
								Status: host_api.StatusFailed,
								Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: fmt.Sprintf("resolve session prompt owner memory scope: %v", resolveErr)},
							}, nil
						}
						ownedExecutionScope, resolveErr = json.Marshal(owned)
						finish()
						if resolveErr != nil {
							return host_api.CallResult{
								Status: host_api.StatusFailed,
								Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: fmt.Sprintf("encode session prompt owner execution scope: %v", resolveErr)},
							}, nil
						}
					}
					def.SessionPromptScope = &schedule.SessionPromptScope{
						SpaceID:                snapshot.SpaceID,
						CharacterID:            snapshot.CharacterID,
						ProjectID:              strings.TrimSpace(targetMetadata.ProjectID),
						ConversationID:         snapshot.ConversationID,
						ReservedConversationID: reservedSessionID,
						TargetKind:             targetKind,
						OwnedExecutionScope:    ownedExecutionScope,
					}
				}
				if err := deps.ScheduleService.InstallDefinition(ctx, &def); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"ok":         true,
					"scheduleId": def.ScheduleID,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodScheduleCancel,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					ScheduleID        string `json:"scheduleId"`
					ExpectedUpdatedAt string `json:"expectedUpdatedAt"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.ScheduleService == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "schedule service not configured"},
					}, nil
				}
				scheduleDef, state, err := deps.ScheduleService.GetSchedule(ctx, p.ScheduleID)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: err.Error()},
					}, nil
				}
				if scheduleDef == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: "schedule not found"},
					}, nil
				}
				if scheduleDef.Target.Type == schedule.TargetTypeHostWorkload {
					return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "业务任务须通过统一任务接口管理"}}, nil
				}
				if scheduleDef.ExtensionID != string(req.RuntimeIdentity.ExtensionID) {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: "schedule does not belong to caller"},
					}, nil
				}
				if expected := strings.TrimSpace(p.ExpectedUpdatedAt); expected != "" && (state == nil || expected != state.UpdatedAt.UTC().Format(time.RFC3339Nano)) {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeStateConflict, Message: "schedule_conflict: expectedUpdatedAt does not match current revision"},
					}, nil
				}
				if err := deps.ScheduleService.Uninstall(ctx, p.ScheduleID); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{"ok": true})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodScheduleUpdate,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         10 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					ScheduleID        string                                  `json:"scheduleId"`
					ExpectedUpdatedAt string                                  `json:"expectedUpdatedAt"`
					Definition        schedule.ScheduleContributionDefinition `json:"definition"`
					Enabled           *bool                                   `json:"enabled"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.ScheduleService == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "schedule service not configured"},
					}, nil
				}
				existing, state, err := deps.ScheduleService.GetSchedule(ctx, p.ScheduleID)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: err.Error()},
					}, nil
				}
				if existing == nil || state == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: "schedule not found"},
					}, nil
				}
				if existing.ExtensionID != string(req.RuntimeIdentity.ExtensionID) {
					return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: "schedule does not belong to caller"}}, nil
				}
				if existing.Target.Type == schedule.TargetTypeHostWorkload || p.Definition.Target.Type != schedule.TargetTypeSessionPrompt {
					return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "business schedules must use the unified task interface"}}, nil
				}
				if expected := strings.TrimSpace(p.ExpectedUpdatedAt); expected != "" && expected != state.UpdatedAt.UTC().Format(time.RFC3339Nano) {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeStateConflict, Message: "schedule_conflict: expectedUpdatedAt does not match current revision"},
					}, nil
				}
				if existing.SessionPromptScope == nil {
					return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "session prompt schedule is missing its owner scope"}}, nil
				}
				var targetMetadata struct {
					TargetKind        string `json:"dshTargetKind"`
					ReservedSessionID string `json:"dshReservedSessionId"`
					ProjectID         string `json:"dshProjectId"`
				}
				if err := json.Unmarshal(p.Definition.Target.InputTemplate, &targetMetadata); err != nil {
					return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()}}, nil
				}
				targetKind := strings.TrimSpace(targetMetadata.TargetKind)
				reservedSessionID := strings.TrimSpace(targetMetadata.ReservedSessionID)
				if targetKind == "" {
					targetKind = "session"
				}
				if targetKind == "session" && p.Definition.Target.TargetID != existing.SessionPromptScope.ConversationID {
					return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "session prompt schedules can only target their owner conversation"}}, nil
				}
				if targetKind == "new_once" && (reservedSessionID == "" || reservedSessionID != p.Definition.Target.TargetID) {
					return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "new_once schedule requires its reserved session id as targetId"}}, nil
				}
				if targetKind != "session" && targetKind != "new_once" && targetKind != "new" {
					return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: fmt.Sprintf("unsupported session prompt target kind %q", targetKind)}}, nil
				}
				p.Definition.ScheduleID = p.ScheduleID
				p.Definition.ExtensionID = existing.ExtensionID
				p.Definition.ModuleID = existing.ModuleID
				scope := *existing.SessionPromptScope
				scope.TargetKind = targetKind
				scope.ReservedConversationID = reservedSessionID
				scope.ProjectID = strings.TrimSpace(targetMetadata.ProjectID)
				p.Definition.SessionPromptScope = &scope
				if err := deps.ScheduleService.UpdateHostDefinitionState(ctx, p.ScheduleID, state.Generation, &p.Definition, p.Enabled); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: json.RawMessage(`{"ok":true}`)}, nil
			},
		},
		{
			method:          host_api.MethodScheduleRun,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         10 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					ScheduleID string `json:"scheduleId"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.ScheduleService == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "schedule service not configured"},
					}, nil
				}
				scheduleDef, _, err := deps.ScheduleService.GetSchedule(ctx, p.ScheduleID)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: err.Error()},
					}, nil
				}
				if scheduleDef == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: "schedule not found"},
					}, nil
				}
				if scheduleDef.Target.Type == schedule.TargetTypeHostWorkload {
					return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "业务任务须通过统一任务接口管理"}}, nil
				}
				if scheduleDef.ExtensionID != string(req.RuntimeIdentity.ExtensionID) {
					return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: "schedule does not belong to caller"}}, nil
				}
				trigger, err := deps.ScheduleService.RunNow(ctx, p.ScheduleID)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"ok":         true,
					"scheduleId": p.ScheduleID,
					"triggerId":  trigger.TriggerID,
				})
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
			},
		},
		{
			method:          host_api.MethodScheduleHistory,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         10 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					ScheduleID string `json:"scheduleId"`
					Limit      int    `json:"limit"`
					Before     string `json:"before"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.ScheduleService == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "schedule service not configured"},
					}, nil
				}
				def, _, err := deps.ScheduleService.GetSchedule(ctx, p.ScheduleID)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: err.Error()},
					}, nil
				}
				if def == nil {
					return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: "schedule not found"}}, nil
				}
				if def.ExtensionID != string(req.RuntimeIdentity.ExtensionID) {
					return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: "schedule does not belong to caller"}}, nil
				}
				if def.Target.Type != schedule.TargetTypeSessionPrompt {
					return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "schedule history is available only for session prompt targets"}}, nil
				}
				limit := p.Limit
				if limit <= 0 || limit > 100 {
					limit = 100
				}
				triggers, err := deps.ScheduleService.GetAllTriggers(ctx, p.ScheduleID)
				if err != nil {
					return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()}}, nil
				}
				runs, err := deps.ScheduleService.GetAllRuns(ctx, p.ScheduleID)
				if err != nil {
					return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()}}, nil
				}
				triggerTimes := make(map[string]time.Time, len(triggers))
				for _, trigger := range triggers {
					triggerTimes[trigger.TriggerID] = trigger.ScheduledAt
				}
				records := make([]map[string]any, 0, len(runs))
				for _, run := range runs {
					if run.Status != schedule.RunStatusCompleted || len(run.ResultJSON) == 0 {
						continue
					}
					var receipt struct {
						MessageIDs      []string `json:"messageIds"`
						Prompt          string   `json:"prompt"`
						TargetSessionID string   `json:"targetSessionId"`
					}
					if err := json.Unmarshal(run.ResultJSON, &receipt); err != nil || len(receipt.MessageIDs) == 0 {
						continue
					}
					deliveredAt := run.UpdatedAt
					if run.FinishedAt != nil {
						deliveredAt = *run.FinishedAt
					}
					scheduledAt := triggerTimes[run.TriggerID]
					if scheduledAt.IsZero() {
						scheduledAt = run.StartedAt
					}
					record := map[string]any{
						"scheduledAt":     scheduledAt.UTC().Format(time.RFC3339Nano),
						"deliveredAt":     deliveredAt.UTC().Format(time.RFC3339Nano),
						"messageId":       receipt.MessageIDs[0],
						"targetSessionId": receipt.TargetSessionID,
					}
					if receipt.TargetSessionID == "" {
						record["targetSessionId"] = def.Target.TargetID
					}
					if receipt.Prompt != "" {
						record["prompt"] = receipt.Prompt
					}
					records = append(records, record)
				}
				sort.Slice(records, func(i, j int) bool {
					left, _ := records[i]["deliveredAt"].(string)
					right, _ := records[j]["deliveredAt"].(string)
					return left > right
				})
				if p.Before != "" {
					index := -1
					for i, record := range records {
						if record["messageId"] == p.Before {
							index = i
							break
						}
					}
					if index < 0 {
						output, _ := json.Marshal(map[string]any{"id": p.ScheduleID, "code": "delivery_cursor_not_found"})
						return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
					}
					records = records[index+1:]
				}
				nextBefore := ""
				if len(records) > limit {
					nextBefore = records[limit-1]["messageId"].(string)
					records = records[:limit]
				}
				output, _ := json.Marshal(map[string]any{
					"id":                        p.ScheduleID,
					"records":                   records,
					"earlierRecordsUnavailable": false,
					"earlierRecordsPruned":      false,
					"retention": map[string]any{
						"days":    30,
						"records": 200,
					},
					"nextBefore": nextBefore,
				})
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
			},
		},
		{
			method:          host_api.MethodScheduleList,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.ScheduleService == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "schedule service not configured"},
					}, nil
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				defs, err := deps.ScheduleService.ListSchedules(ctx, extID)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				technical := make([]*schedule.ScheduleContributionDefinition, 0, len(defs))
				states := make(map[string]*schedule.ScheduleState, len(defs))
				for _, definition := range defs {
					if definition.Target.Type != schedule.TargetTypeHostWorkload {
						technical = append(technical, definition)
					}
				}
				for _, definition := range technical {
					state, stateErr := deps.ScheduleService.GetScheduleState(ctx, definition.ScheduleID)
					if stateErr != nil {
						return host_api.CallResult{
							Status: host_api.StatusFailed,
							Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: stateErr.Error()},
						}, nil
					}
					states[definition.ScheduleID] = state
				}
				output, _ := json.Marshal(map[string]any{
					"schedules": technical,
					"states":    states,
					"total":     len(technical),
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodToolExecute,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectExternal,
			timeout:         30 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					ToolID string          `json:"toolId"`
					Input  json.RawMessage `json:"input"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.ExecutionKernel == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "execution kernel not configured"},
					}, nil
				}
				if p.ToolID == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "toolId is required"},
					}, nil
				}
				ctx = resolveHostAPIScope(ctx, deps.ScopeSnapshotStore, req.ScopeSnapshotID)
				hostScope := GetHostAPIScope(ctx)
				if req.SpaceID != "" {
					if hostScope.SpaceID != "" && hostScope.SpaceID != req.SpaceID {
						return host_api.CallResult{
							Status: host_api.StatusRejected,
							Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "spaceId does not match current scope"},
						}, nil
					}
					hostScope.SpaceID = req.SpaceID
				}
				if req.CharacterID != "" {
					if hostScope.CharacterID != "" && hostScope.CharacterID != req.CharacterID {
						return host_api.CallResult{
							Status: host_api.StatusRejected,
							Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "characterId does not match current scope"},
						}, nil
					}
					hostScope.CharacterID = req.CharacterID
				}
				if req.ConversationID != "" {
					if hostScope.ConversationID != "" && hostScope.ConversationID != req.ConversationID {
						return host_api.CallResult{
							Status: host_api.StatusRejected,
							Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "conversationId does not match current scope"},
						}, nil
					}
					hostScope.ConversationID = req.ConversationID
				}
				ctx = WithHostAPIScope(ctx, hostScope)
				if hostScope.SpaceID != "" && req.ExecutionContext.SpaceID != "" && hostScope.SpaceID != req.ExecutionContext.SpaceID.String() {
					return host_api.CallResult{
						Status: host_api.StatusRejected,
						Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "execution space does not match current scope"},
					}, nil
				}
				depth := 0
				if req.ParentID != "" {
					depth = extractDepth(req.ParentID) + 1
				}
				if depth >= maxToolExecutionDepth {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: fmt.Sprintf("tool execution depth limit (%d) exceeded", maxToolExecutionDepth)},
					}, nil
				}
				if deps.ToolFacade == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "tool facade not configured"},
					}, nil
				}
				invocationID := req.InvocationID
				if invocationID == "" {
					invocationID = fmt.Sprintf("hostapi-%s", uuid.NewString())
				}
				source := capability.InvocationSourcePlugin
				permissionMode := ""
				if req.UserInitiated {
					source = capability.InvocationSourceUser
					permissionMode = string(capability.ApprovalModeSession)
				}
				executionContext := &coreexec.ExecutionContext{
					ExecutionID:          invocationID,
					SpaceID:              req.ExecutionContext.SpaceID,
					ConversationID:       hostScope.ConversationID,
					InvocationID:         invocationID,
					TraceID:              req.TraceID,
					ScopeSnapshotID:      req.ScopeSnapshotID,
					PermissionSnapshotID: req.PermissionSnapshotID,
					ExtensionID:          string(req.RuntimeIdentity.ExtensionID),
					ModuleID:             string(req.RuntimeIdentity.ModuleID),
					RuntimeTarget: &coreexec.RuntimeTarget{
						Placement: string(req.ExecutionContext.Placement),
						SpaceID:   req.ExecutionContext.SpaceID,
						DeviceID:  req.ExecutionContext.DeviceID,
						RuntimeID: req.ExecutionContext.RuntimeID,
					},
					Source:    string(source),
					CreatedAt: time.Now().UTC(),
				}
				toolScope := InvocationScope{
					SpaceID:        hostScope.SpaceID,
					DeviceID:       req.ExecutionContext.DeviceID.String(),
					RuntimeID:      req.ExecutionContext.RuntimeID.String(),
					CharacterID:    hostScope.CharacterID,
					ConversationID: hostScope.ConversationID,
					Channel:        req.Channel,
					SessionID:      req.RuntimeIdentity.InstanceID,
					Source:         string(source),
					TraceID:        req.TraceID,
					RequestID:      invocationID,
					ToolCallID:     invocationID,
					PermissionMode: permissionMode,
					ExecContext:    executionContext,
				}
				result, found := deps.ToolFacade.ExecuteTool(ctx, capability.CapabilityID(p.ToolID), p.Input, toolScope, invocationID, "")
				if !found {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeMethodNotFound, Message: "tool not found"},
					}, nil
				}
				var content []map[string]any
				if strings.TrimSpace(result.VisibleText) != "" {
					content = append(content, map[string]any{"type": "text", "text": result.VisibleText})
				}
				var resultError *capability.ToolError
				if result.Error != nil {
					resultError = &capability.ToolError{Code: result.Error.Code, Message: result.Error.Message, Retryable: result.Error.Retryable}
				}
				output, _ := json.Marshal(map[string]any{
					"invocationId": invocationID,
					"status":       result.Status,
					"content":      content,
					"structured":   json.RawMessage(result.Output),
					"error":        resultError,
				})
				if result.Status != string(capability.ToolResultStatusSuccess) {
					message := fmt.Sprintf("tool execution failed: %s", result.Status)
					code := host_api.ErrorCodeInternal
					if result.Error != nil {
						code = result.Error.Code
						if strings.TrimSpace(result.Error.Message) != "" {
							message = fmt.Sprintf("%s (%s: %s)", message, result.Error.Code, result.Error.Message)
						}
					}
					return host_api.CallResult{
						Status: toolFailureStatus(code),
						Output: output,
						Error: &host_api.Error{
							Code:    code,
							Message: message,
						},
					}, nil
				}
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodPluginList,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         30 * time.Second,
			handler: func(ctx context.Context, _ host_api.CallRequest) (host_api.CallResult, error) {
				if deps.PluginPackages == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "plugin package manager not configured"},
					}, nil
				}
				result, err := deps.PluginPackages.List(ctx)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, err := json.Marshal(result)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
			},
		},
		{
			method:          host_api.MethodPluginInstall,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectExternal,
			timeout:         30 * time.Minute,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.PluginPackages == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "plugin package manager not configured"},
					}, nil
				}
				var input struct {
					ExtensionID      string          `json:"extensionId"`
					Version          string          `json:"version"`
					PreviewSessionID string          `json:"previewSessionId"`
					Confirmations    map[string]bool `json:"confirmations"`
					IdempotencyKey   string          `json:"idempotencyKey,omitempty"`
				}
				if err := json.Unmarshal(req.Input, &input); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				input.ExtensionID = strings.TrimSpace(input.ExtensionID)
				input.Version = strings.TrimSpace(input.Version)
				input.PreviewSessionID = strings.TrimSpace(input.PreviewSessionID)
				if input.ExtensionID == "" || input.Version == "" || input.PreviewSessionID == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "extensionId, version and previewSessionId are required"},
					}, nil
				}
				result, err := deps.PluginPackages.Install(ctx, InstallHostPluginPackageRequest{
					SpaceID: req.SpaceID, ExtensionID: input.ExtensionID, Version: input.Version,
					PreviewSessionID: input.PreviewSessionID, Confirmations: input.Confirmations,
					IdempotencyKey: input.IdempotencyKey,
				})
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, err := json.Marshal(result)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
			},
		},
		{
			method:          host_api.MethodPluginUninstall,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectExternal,
			timeout:         30 * time.Minute,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.PluginPackages == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "plugin package manager not configured"},
					}, nil
				}
				var input struct {
					ExtensionID    string `json:"extensionId"`
					IdempotencyKey string `json:"idempotencyKey,omitempty"`
				}
				if err := json.Unmarshal(req.Input, &input); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				input.ExtensionID = strings.TrimSpace(input.ExtensionID)
				if input.ExtensionID == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "extensionId is required"},
					}, nil
				}
				result, err := deps.PluginPackages.Uninstall(ctx, UninstallHostPluginPackageRequest{
					SpaceID: req.SpaceID, ExtensionID: input.ExtensionID, IdempotencyKey: input.IdempotencyKey,
				})
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, err := json.Marshal(result)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
			},
		},
		{
			method:          host_api.MethodPluginPreview,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectExternal,
			timeout:         30 * time.Minute,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.PluginPackages == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "plugin package manager not configured"},
					}, nil
				}
				var input struct {
					ExtensionID string                  `json:"extensionId"`
					Version     string                  `json:"version"`
					PackageURI  string                  `json:"packageUri"`
					SHA256      string                  `json:"sha256"`
					FileName    string                  `json:"fileName,omitempty"`
					Source      PackageSourceProvenance `json:"source,omitempty"`
				}
				if err := json.Unmarshal(req.Input, &input); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				preview, err := deps.PluginPackages.Prepare(ctx, PrepareHostPluginPackageRequest{
					SpaceID: req.SpaceID, ExtensionID: input.ExtensionID, Version: input.Version,
					PackageURI: input.PackageURI, SHA256: input.SHA256, FileName: input.FileName, Source: input.Source,
				})
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, err := json.Marshal(preview)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
			},
		},
		{
			method:          host_api.MethodNetworkRequest,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectExternal,
			timeout:         2 * time.Minute,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.NetworkClient == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "network requester not configured"},
					}, nil
				}
				var input struct {
					URL               string            `json:"url"`
					Method            string            `json:"method"`
					Headers           map[string]string `json:"headers"`
					Body              json.RawMessage   `json:"body"`
					TimeoutMS         int64             `json:"timeoutMs"`
					IdempotencyKey    string            `json:"idempotencyKey,omitempty"`
					FailOnServerError bool              `json:"failOnServerError,omitempty"`
				}
				if err := json.Unmarshal(req.Input, &input); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				target, err := url.Parse(strings.TrimSpace(input.URL))
				if err != nil || (target.Scheme != "https" && target.Scheme != "http") || target.Hostname() == "" || target.User != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "url must be an absolute HTTP or HTTPS URL without user information"},
					}, nil
				}
				if !deps.NetworkAllowPrivate {
					if err := rejectPrivateNetworkURL(ctx, target); err != nil {
						return host_api.CallResult{
							Status: host_api.StatusFailed,
							Error:  &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: err.Error()},
						}, nil
					}
				}
				method := strings.ToUpper(strings.TrimSpace(input.Method))
				if method == "" {
					method = http.MethodGet
				}
				switch method {
				case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
				default:
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "unsupported HTTP method: " + method},
					}, nil
				}
				var body io.Reader
				if len(input.Body) > 0 && string(input.Body) != "null" {
					var encoded string
					if json.Unmarshal(input.Body, &encoded) == nil {
						body = strings.NewReader(encoded)
					} else {
						body = strings.NewReader(string(input.Body))
					}
				}
				timeout := 2 * time.Minute
				if input.TimeoutMS > 0 && input.TimeoutMS < timeout.Milliseconds() {
					timeout = time.Duration(input.TimeoutMS) * time.Millisecond
				}
				requestCtx, cancel := context.WithTimeout(ctx, timeout)
				defer cancel()
				request, err := http.NewRequestWithContext(requestCtx, method, target.String(), body)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				for key, value := range input.Headers {
					if strings.EqualFold(strings.TrimSpace(key), "host") {
						continue
					}
					request.Header.Set(key, value)
				}
				if request.Header.Get("User-Agent") == "" {
					request.Header.Set("User-Agent", "Amitia-Plugin-Host")
				}
				client := *deps.NetworkClient
				if !deps.NetworkAllowPrivate {
					guardedClient, guardErr := privateNetworkGuardedHTTPClient(&client)
					if guardErr != nil {
						return host_api.CallResult{
							Status: host_api.StatusFailed,
							Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: guardErr.Error()},
						}, nil
					}
					client = *guardedClient
				}
				client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
					if len(via) >= 5 {
						return errors.New("network redirect limit exceeded")
					}
					if !deps.NetworkAllowPrivate {
						if err := rejectPrivateNetworkURL(next.Context(), next.URL); err != nil {
							return err
						}
					}
					for key := range next.Header {
						if strings.EqualFold(key, "Authorization") || strings.EqualFold(key, "Cookie") {
							next.Header.Del(key)
						}
					}
					return nil
				}
				response, err := client.Do(request)
				if err != nil {
					return host_api.CallResult{
						Status: transportFailureStatus(err),
						Error:  &host_api.Error{Code: transportFailureCode(err), Message: err.Error()},
					}, nil
				}
				defer response.Body.Close()
				payload, err := io.ReadAll(io.LimitReader(response.Body, (16<<20)+1))
				if err != nil {
					return host_api.CallResult{
						Status: transportFailureStatus(err),
						Error:  &host_api.Error{Code: transportFailureCode(err), Message: err.Error()},
					}, nil
				}
				if len(payload) > 16<<20 {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeOutputInvalid, Message: "network response exceeds 16 MiB"},
					}, nil
				}
				if input.FailOnServerError && (response.StatusCode < 200 || response.StatusCode >= 300) {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error: &host_api.Error{
							Code:    host_api.ErrorCodeUpstream,
							Message: upstreamFailureMessage(response.StatusCode, payload),
							Detail:  string(payload),
						},
					}, nil
				}
				headers := make(map[string]string, len(response.Header))
				for key := range response.Header {
					headers[key] = response.Header.Get(key)
				}
				var responseBody any = string(payload)
				if strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "json") && len(payload) > 0 {
					if err := json.Unmarshal(payload, &responseBody); err != nil {
						return host_api.CallResult{
							Status: host_api.StatusFailed,
							Error:  &host_api.Error{Code: host_api.ErrorCodeOutputInvalid, Message: err.Error()},
						}, nil
					}
				}
				output, err := json.Marshal(map[string]any{
					"status": response.StatusCode, "headers": headers, "body": responseBody,
					"ok": response.StatusCode >= 200 && response.StatusCode < 300,
				})
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
			},
		},
		{
			method:          host_api.MethodUINotify,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectNone,
			timeout:         3 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.UIHostNotifier == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeUIHostUnavailable, Message: "ui host notifier not configured"},
					}, nil
				}
				var p struct {
					Title    string `json:"title"`
					Body     string `json:"body"`
					Severity string `json:"severity"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if p.Title == "" {
					p.Title = "Extension Notification"
				}
				if p.Severity == "" {
					p.Severity = "info"
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				if err := deps.UIHostNotifier.Notify(ctx, extID, p.Title, p.Body, p.Severity); err != nil {
					code := host_api.ErrorCodeInternal
					if errors.Is(err, ErrUIHostUnavailable) {
						code = host_api.ErrorCodeUIHostUnavailable
					}
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: code, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{"ok": true})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodUIDialog,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectNone,
			timeout:         30 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.UIHostNotifier == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeDialogHostUnavailable, Message: "dialog host notifier not configured"},
					}, nil
				}
				var p struct {
					DialogID string   `json:"dialogId"`
					Message  string   `json:"message"`
					Buttons  []string `json:"buttons"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if p.Message == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "message is required"},
					}, nil
				}
				if p.DialogID == "" {
					p.DialogID = fmt.Sprintf("dialog-%s", uuid.NewString())
				}
				if len(p.Buttons) == 0 {
					p.Buttons = []string{"OK"}
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				result, err := deps.UIHostNotifier.Dialog(ctx, extID, p.DialogID, p.Message, p.Buttons)
				if err != nil {
					code := host_api.ErrorCodeInternal
					if errors.Is(err, ErrDialogHostUnavailable) {
						code = host_api.ErrorCodeDialogHostUnavailable
					}
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: code, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"dialogId": p.DialogID,
					"result":   result,
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodUINavigate,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectNone,
			timeout:         3 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.UIHostNotifier == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeNavigationHostUnavailable, Message: "navigation host notifier not configured"},
					}, nil
				}
				var p struct {
					Target string `json:"target"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if p.Target == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "target is required"},
					}, nil
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				if err := deps.UIHostNotifier.Navigate(ctx, extID, p.Target); err != nil {
					code := host_api.ErrorCodeInternal
					if errors.Is(err, ErrNavigationHostUnavailable) {
						code = host_api.ErrorCodeNavigationHostUnavailable
					}
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: code, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{"ok": true, "target": p.Target})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodClipboardWrite,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         clipboardRouteTimeout,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.ClipboardHost == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "clipboard host not configured"},
					}, nil
				}
				var p struct {
					Text string `json:"text"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if len(p.Text) > maxClipboardPayloadSize {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "clipboard text exceeds maximum size"},
					}, nil
				}
				if err := deps.ClipboardHost.WriteText(ctx, p.Text); err != nil {
					code := host_api.ErrorCodeHostUnavailable
					if errors.Is(err, ErrClipboardTextTooLarge) {
						code = host_api.ErrorCodeInputInvalid
					} else if errors.Is(err, ErrClipboardHostTimeout) {
						code = host_api.ErrorCodeTimeout
					}
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: code, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{"ok": true})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodClipboardRead,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         clipboardRouteTimeout,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.ClipboardHost == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "clipboard host not configured"},
					}, nil
				}
				text, err := deps.ClipboardHost.ReadText(ctx)
				if err != nil {
					code := host_api.ErrorCodeHostUnavailable
					if errors.Is(err, ErrClipboardHostTimeout) {
						code = host_api.ErrorCodeTimeout
					}
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: code, Message: err.Error()},
					}, nil
				}
				if len(text) > maxClipboardPayloadSize {
					text = text[:maxClipboardPayloadSize]
				}
				output, _ := json.Marshal(map[string]any{"text": text})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodRuntimeHealth,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.RuntimeSupervisor == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "runtime supervisor not configured"},
					}, nil
				}
				extID := string(req.RuntimeIdentity.ExtensionID)
				modID := string(req.RuntimeIdentity.ModuleID)
				snapshots := deps.RuntimeSupervisor.SnapshotByExtension(ctx, extID, modID)
				if len(snapshots) == 0 {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: "no runtime instances found for extension"},
					}, nil
				}
				instances := make([]map[string]any, 0, len(snapshots))
				for _, snap := range snapshots {
					instances = append(instances, map[string]any{
						"instanceId":  snap.InstanceID,
						"generation":  snap.Generation,
						"health":      string(snap.Health),
						"circuit":     string(snap.Circuit),
						"actual":      string(snap.Actual),
						"quarantined": snap.Quarantined,
					})
				}
				output, _ := json.Marshal(map[string]any{
					"extensionId": extID,
					"moduleId":    modID,
					"instances":   instances,
					"total":       len(instances),
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodRuntimeSnapshot,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         15 * time.Second,
			handler: func(ctx context.Context, _ host_api.CallRequest) (host_api.CallResult, error) {
				if deps.RuntimeObservability == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "runtime observability not configured"},
					}, nil
				}
				output, err := json.Marshal(deps.RuntimeObservability.Snapshot(ctx))
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
			},
		},
		{
			method:          host_api.MethodWorkspaceRead,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         30 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.Workspace == nil {
					return workspaceHostResult(nil, errors.New("workspace service is unavailable"))
				}
				return workspaceHostResult(deps.Workspace.Read(ctx, req.Input))
			},
		},
		{
			method:          host_api.MethodWorkspaceWrite,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         2 * time.Minute,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.Workspace == nil {
					return workspaceHostResult(nil, errors.New("workspace service is unavailable"))
				}
				return workspaceHostResult(deps.Workspace.Write(ctx, req.Input))
			},
		},
		{
			method:          host_api.MethodImportExecute,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         2 * time.Minute,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.ChatImport == nil {
					return workspaceHostResult(nil, errors.New("chat import service is unavailable"))
				}
				var input struct {
					Operation string          `json:"operation"`
					Input     json.RawMessage `json:"input,omitempty"`
				}
				if err := json.Unmarshal(req.Input, &input); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if input.Operation == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "operation is required"},
					}, nil
				}
				if len(input.Input) == 0 {
					input.Input = json.RawMessage(`{}`)
				}
				return workspaceHostResult(deps.ChatImport.Execute(ctx, req, input.Operation, input.Input))
			},
		},
		storageHostRoute(host_api.MethodStorageCleanup, deps.Storage),
		storageHostRoute(host_api.MethodStorageBackup, deps.Storage),
		storageHostRoute(host_api.MethodStorageRestore, deps.Storage),
		chatLogsHostRoute(host_api.MethodConversationLogsRead, deps.ChatLogs, host_api.RiskLow, host_api.SideEffectReadOnly),
		chatLogsHostRoute(host_api.MethodConversationLogsManage, deps.ChatLogs, host_api.RiskHigh, host_api.SideEffectWrite),
		chatLogsHostRoute(host_api.MethodConversationLogsExport, deps.ChatLogs, host_api.RiskMedium, host_api.SideEffectReadOnly),
		{
			method:          host_api.MethodRuntimeMaintenanceStatus,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         10 * time.Second,
			handler: func(ctx context.Context, _ host_api.CallRequest) (host_api.CallResult, error) {
				if deps.RuntimeMaintenance == nil {
					return runtimeMaintenanceResult(nil, errors.New("runtime maintenance service is unavailable"))
				}
				return runtimeMaintenanceResult(deps.RuntimeMaintenance.Status(ctx))
			},
		},
		{
			method:          host_api.MethodRuntimeMaintenanceOverview,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         15 * time.Second,
			handler: func(ctx context.Context, _ host_api.CallRequest) (host_api.CallResult, error) {
				if deps.RuntimeMaintenance == nil {
					return runtimeMaintenanceResult(nil, errors.New("runtime maintenance service is unavailable"))
				}
				return runtimeMaintenanceResult(deps.RuntimeMaintenance.Overview(ctx))
			},
		},
		{
			method:          host_api.MethodRuntimeMaintenanceConfig,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         10 * time.Second,
			handler: func(ctx context.Context, _ host_api.CallRequest) (host_api.CallResult, error) {
				if deps.RuntimeMaintenance == nil {
					return runtimeMaintenanceResult(nil, errors.New("runtime maintenance service is unavailable"))
				}
				return runtimeMaintenanceResult(deps.RuntimeMaintenance.Config(ctx))
			},
		},
		{
			method:          host_api.MethodRuntimeMaintenanceUpdate,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         10 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				if deps.RuntimeMaintenance == nil {
					return runtimeMaintenanceResult(nil, errors.New("runtime maintenance service is unavailable"))
				}
				return runtimeMaintenanceResult(deps.RuntimeMaintenance.UpdateConfig(ctx, req.Input))
			},
		},
		{
			method:          host_api.MethodRuntimeCleanupTemp,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         2 * time.Minute,
			handler: func(ctx context.Context, _ host_api.CallRequest) (host_api.CallResult, error) {
				if deps.RuntimeMaintenance == nil {
					return runtimeMaintenanceResult(nil, errors.New("runtime maintenance service is unavailable"))
				}
				return runtimeMaintenanceResult(deps.RuntimeMaintenance.CleanupTemp(ctx))
			},
		},
		{
			method:          host_api.MethodRuntimeRotateLogs,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         2 * time.Minute,
			handler: func(ctx context.Context, _ host_api.CallRequest) (host_api.CallResult, error) {
				if deps.RuntimeMaintenance == nil {
					return runtimeMaintenanceResult(nil, errors.New("runtime maintenance service is unavailable"))
				}
				return runtimeMaintenanceResult(deps.RuntimeMaintenance.RotateLogs(ctx))
			},
		},
		{
			method:          host_api.MethodRuntimeCheckDBIntegrity,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         2 * time.Minute,
			handler: func(ctx context.Context, _ host_api.CallRequest) (host_api.CallResult, error) {
				if deps.RuntimeMaintenance == nil {
					return runtimeMaintenanceResult(nil, errors.New("runtime maintenance service is unavailable"))
				}
				return runtimeMaintenanceResult(deps.RuntimeMaintenance.CheckDBIntegrity(ctx))
			},
		},
		{
			method:          host_api.MethodSecretGet,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         5 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					Ref string `json:"ref"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.SecretStore == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "secret store not configured"},
					}, nil
				}
				value, err := deps.SecretStore.Get(ctx, p.Ref)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"ref":   p.Ref,
					"value": string(value),
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodProviderInvoke,
			riskLevel:       host_api.RiskCritical,
			sideEffectLevel: host_api.SideEffectExternal,
			timeout:         30 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p ProviderInvokeRequest
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.ProviderInvoker == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "provider invoker not configured"},
					}, nil
				}
				resp, err := deps.ProviderInvoker.Invoke(ctx, p)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, _ := json.Marshal(map[string]any{
					"success": resp.Success,
					"result":  json.RawMessage(resp.Result),
				})
				return host_api.CallResult{
					Status: host_api.StatusSuccess,
					Output: output,
				}, nil
			},
		},
		{
			method:          host_api.MethodModelInvoke,
			riskLevel:       host_api.RiskCritical,
			sideEffectLevel: host_api.SideEffectExternal,
			timeout:         5 * time.Minute,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var input ModelInvokeRequest
				if err := json.Unmarshal(req.Input, &input); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if deps.ModelGateway == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "model gateway not configured"},
					}, nil
				}
				result, err := deps.ModelGateway.Invoke(ctx, input)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, err := json.Marshal(result)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
			},
		},
		{
			method:          host_api.MethodApprovalRequest,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectNone,
			timeout:         0,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					ToolName       string `json:"toolName"`
					Arguments      string `json:"arguments"`
					RiskLevel      string `json:"riskLevel"`
					ToolCallID     string `json:"toolCallId"`
					RequestID      string `json:"requestId"`
					ConversationID string `json:"conversationId"`
					TimeoutMS      int    `json:"timeoutMs"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				p.ToolName = strings.TrimSpace(p.ToolName)
				if p.ToolName == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "toolName is required"},
					}, nil
				}
				if deps.ApprovalBroker == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "approval broker not configured"},
					}, nil
				}
				ctx = resolveHostAPIScope(ctx, deps.ScopeSnapshotStore, req.ScopeSnapshotID)
				scope, scopeErr := resolveConversationTargetScope(ctx, deps, req, p.ConversationID)
				if scopeErr != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: scopeErr.Error()},
					}, nil
				}
				if strings.TrimSpace(scope.ConversationID) == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "host.approval.request requires an active conversation in scope"},
					}, nil
				}
				timeout := 5 * time.Minute
				if p.TimeoutMS > 0 {
					timeout = time.Duration(p.TimeoutMS) * time.Millisecond
				}
				approved, err := deps.ApprovalBroker.Await(ctx, execution.ApprovalRequest{
					SpaceID:        scope.SpaceID,
					CharacterID:    scope.CharacterID,
					ConversationID: scope.ConversationID,
					RequestID:      strings.TrimSpace(p.RequestID),
					ToolCallID:     strings.TrimSpace(p.ToolCallID),
					ToolName:       p.ToolName,
					Arguments:      p.Arguments,
					RiskLevel:      strings.TrimSpace(p.RiskLevel),
				}, timeout)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				output, err := json.Marshal(map[string]any{"approved": approved})
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
			},
		},
		{
			method:          host_api.MethodVectorUpsert,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         30 * time.Second,
			handler:         vectorUpsertHandler,
		},
		{
			method:          host_api.MethodVectorSearch,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         30 * time.Second,
			handler:         vectorSearchHandler,
		},
		{
			method:          host_api.MethodVectorDelete,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         30 * time.Second,
			handler:         vectorDeleteHandler,
		},
		{
			method:          host_api.MethodConversationMessageSend,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectExternal,
			timeout:         2 * time.Minute,
			handler:         conversationMessageHandler,
		},
		{
			method:          host_api.MethodConversationMessageAppend,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         30 * time.Second,
			handler:         conversationMessageAppendHandler,
		},
		{
			method:          host_api.MethodConversationTurnInterrupt,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         10 * time.Second,
			handler:         conversationTurnInterruptHandler,
		},
		{
			method:          host_api.MethodConversationCreate,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         30 * time.Second,
			handler:         conversationCreateHandler,
		},
		{
			method:          host_api.MethodConversationDelete,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         30 * time.Second,
			handler:         conversationDeleteRouteHandler(deps),
		},
		{
			method:          host_api.MethodConversationTurnAwait,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         0,
			handler:         conversationTurnAwaitHandler,
		},
		{
			method:          host_api.MethodConversationArchive,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         30 * time.Second,
			handler:         conversationArchiveHandler,
		},
		{
			method:          host_api.MethodConversationFork,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         30 * time.Second,
			handler:         conversationForkRouteHandler(deps),
		},
		{
			method:          host_api.MethodModelDefaults,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         10 * time.Second,
			handler:         modelDefaultsRouteHandler(deps),
		},
		{
			method:          host_api.MethodSkillList,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         10 * time.Second,
			handler:         skillListRouteHandler(deps),
		},
		{
			method:          host_api.MethodConversationPin,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         30 * time.Second,
			handler:         conversationPinRouteHandler(deps),
		},
		{
			method:          host_api.MethodDirectoryList,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         15 * time.Second,
			handler:         directoryListRouteHandler(deps),
		},
		{
			method:          host_api.MethodDirectoryCreate,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         15 * time.Second,
			handler:         directoryCreateRouteHandler(deps),
		},
		{
			method:          host_api.MethodProjectList,
			riskLevel:       host_api.RiskLow,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         10 * time.Second,
			handler:         projectListHandler,
		},
		{
			method:          host_api.MethodProjectCreate,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         30 * time.Second,
			handler:         projectCreateHandler,
		},
		{
			method:          host_api.MethodProjectUpdate,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         30 * time.Second,
			handler:         projectUpdateHandler,
		},
		{
			method:          host_api.MethodProjectDelete,
			riskLevel:       host_api.RiskHigh,
			sideEffectLevel: host_api.SideEffectWrite,
			timeout:         30 * time.Second,
			handler:         projectDeleteHandler,
		},
		{
			method:          host_api.MethodExtensionDataRead,
			riskLevel:       host_api.RiskMedium,
			sideEffectLevel: host_api.SideEffectReadOnly,
			timeout:         10 * time.Second,
			handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
				var p struct {
					SourceID string          `json:"sourceId"`
					Params   json.RawMessage `json:"params"`
				}
				if err := json.Unmarshal(req.Input, &p); err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
					}, nil
				}
				if strings.TrimSpace(p.SourceID) == "" {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "sourceId is required"},
					}, nil
				}
				if deps.ExtensionDataSources == nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "extension data source registry not configured"},
					}, nil
				}
				snapshot, err := deps.ExtensionDataSources.Read(ctx, p.SourceID, p.Params)
				if err != nil {
					return host_api.CallResult{
						Status: host_api.StatusFailed,
						Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
					}, nil
				}
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: snapshot}, nil
			},
		},
	}

	for _, rd := range routes {
		perm := host_api.RoutePermissionForMethod(rd.method)
		if perm == nil {
			return fmt.Errorf("host_api: no permission mapping for method %s, refusing to register", rd.method)
		}
		scopePolicy := host_api.RouteScopeForMethod(rd.method)
		if host_api.IsDataRouteMethod(rd.method) {
			if scopePolicy.Namespaced == false && len(scopePolicy.RequireRoles) == 0 {
				return fmt.Errorf("host_api: empty scope policy for data route %s, refusing to register", rd.method)
			}
		}
		route := host_api.Route{
			Method:          rd.method,
			Version:         1,
			Permission:      perm,
			ScopePolicy:     scopePolicy,
			RiskLevel:       rd.riskLevel,
			SideEffectLevel: rd.sideEffectLevel,
			Timeout:         rd.timeout,
			Handler:         rd.handler,
		}
		if err := gateway.RegisterRoute(route); err != nil {
			return fmt.Errorf("host_api: register route %s v%d: %w", route.Method, route.Version, err)
		}
	}

	return nil
}

func runtimeMaintenanceResult(value any, err error) (host_api.CallResult, error) {
	if err != nil {
		return host_api.CallResult{
			Status: host_api.StatusFailed,
			Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
		}, nil
	}
	output, err := json.Marshal(value)
	if err != nil {
		return host_api.CallResult{
			Status: host_api.StatusFailed,
			Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
		}, nil
	}
	return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
}

func workspaceHostResult(value any, err error) (host_api.CallResult, error) {
	if err != nil {
		return host_api.CallResult{
			Status: host_api.StatusFailed,
			Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
		}, nil
	}
	output, err := json.Marshal(value)
	if err != nil {
		return host_api.CallResult{
			Status: host_api.StatusFailed,
			Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
		}, nil
	}
	return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
}

func storageHostRoute(method host_api.Method, bridge *StorageBridge) struct {
	method          host_api.Method
	riskLevel       host_api.RiskLevel
	sideEffectLevel host_api.SideEffectLevel
	timeout         time.Duration
	handler         host_api.Handler
} {
	return struct {
		method          host_api.Method
		riskLevel       host_api.RiskLevel
		sideEffectLevel host_api.SideEffectLevel
		timeout         time.Duration
		handler         host_api.Handler
	}{
		method:          method,
		riskLevel:       host_api.RiskHigh,
		sideEffectLevel: host_api.SideEffectWrite,
		timeout:         10 * time.Minute,
		handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
			if bridge == nil {
				return workspaceHostResult(nil, errors.New("storage service is unavailable"))
			}
			var input struct {
				Operation string          `json:"operation"`
				Input     json.RawMessage `json:"input,omitempty"`
			}
			if err := json.Unmarshal(req.Input, &input); err != nil {
				return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()}}, nil
			}
			if input.Operation == "" {
				return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "operation is required"}}, nil
			}
			if len(input.Input) == 0 {
				input.Input = json.RawMessage(`{}`)
			}
			return workspaceHostResult(bridge.Execute(ctx, req, method, input.Operation, input.Input))
		},
	}
}

func chatLogsHostRoute(
	method host_api.Method,
	bridge *ChatLogsBridge,
	risk host_api.RiskLevel,
	sideEffect host_api.SideEffectLevel,
) struct {
	method          host_api.Method
	riskLevel       host_api.RiskLevel
	sideEffectLevel host_api.SideEffectLevel
	timeout         time.Duration
	handler         host_api.Handler
} {
	return struct {
		method          host_api.Method
		riskLevel       host_api.RiskLevel
		sideEffectLevel host_api.SideEffectLevel
		timeout         time.Duration
		handler         host_api.Handler
	}{
		method:          method,
		riskLevel:       risk,
		sideEffectLevel: sideEffect,
		timeout:         2 * time.Minute,
		handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
			if bridge == nil {
				return workspaceHostResult(nil, errors.New("conversation archive service is unavailable"))
			}
			var input struct {
				Operation string          `json:"operation"`
				Input     json.RawMessage `json:"input,omitempty"`
			}
			if err := json.Unmarshal(req.Input, &input); err != nil {
				return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()}}, nil
			}
			if input.Operation == "" {
				return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "operation is required"}}, nil
			}
			if len(input.Input) == 0 {
				input.Input = json.RawMessage(`{}`)
			}
			return workspaceHostResult(bridge.Execute(ctx, req, method, input.Operation, input.Input))
		},
	}
}

func isPathSafe(baseDir, targetPath string) bool {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return false
	}
	if strings.HasPrefix(rel, "..") {
		return false
	}
	return true
}

func extractDepth(parentID string) int {
	if parentID == "" {
		return 0
	}
	parts := strings.Split(parentID, ":")
	if len(parts) < 2 {
		return 0
	}
	var d int
	for _, c := range parts[len(parts)-1] {
		if c >= '0' && c <= '9' {
			d = d*10 + int(c-'0')
		} else {
			break
		}
	}
	return d
}

func toolFailureStatus(code string) string {
	switch code {
	case capability.ErrorCodeTimeout:
		return host_api.StatusTimeout
	case capability.ErrorCodeCancelled:
		return host_api.StatusCancelled
	case capability.ErrorCodePermissionDenied, capability.ErrorCodeScopeDenied:
		return host_api.StatusRejected
	}
	return host_api.StatusFailed
}

func transportFailureCode(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return host_api.ErrorCodeTimeout
	case errors.Is(err, context.Canceled):
		return host_api.ErrorCodeCancelled
	}
	return host_api.ErrorCodeInternal
}

func transportFailureStatus(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return host_api.StatusTimeout
	case errors.Is(err, context.Canceled):
		return host_api.StatusCancelled
	}
	return host_api.StatusFailed
}

func upstreamFailureMessage(status int, payload []byte) string {
	message := fmt.Sprintf("upstream HTTP %d %s", status, http.StatusText(status))
	body := strings.TrimSpace(string(payload))
	if body == "" {
		return message
	}
	if len(body) <= 4096 {
		return message + ": " + body
	}
	return fmt.Sprintf("%s: %d bytes body, complete text preserved in detail", message, len(body))
}

func rejectPrivateNetworkURL(ctx context.Context, target *url.URL) error {
	if target == nil || target.Hostname() == "" {
		return errors.New("network destination host is required")
	}
	host := target.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateNetworkIP(ip) {
			return errors.New("network destination cannot target a private or local address")
		}
		return nil
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("resolve network destination: %w", err)
	}
	for _, address := range addresses {
		if isPrivateNetworkIP(address.IP) {
			return errors.New("network destination resolves to a private or local address")
		}
	}
	return nil
}

func isPrivateNetworkIP(ip net.IP) bool {
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}
