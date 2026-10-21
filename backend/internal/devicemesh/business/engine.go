package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/conversationstream"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/memorypolicy"
)

var ErrUncertainExecution = errors.New("上次执行的结果尚未确认，请查询结果后再继续，系统不会重复执行已发起的动作")
var ErrInterrupted = errors.New("当前回复已主动停止")

type Request struct {
	queuedHash         string
	RetryRequestID     string                         `json:"retryRequestId,omitempty"`
	RecallEvidence     []coordination.ResourceVersion `json:"-"`
	MemoryView         string                         `json:"memoryView,omitempty"`
	Quote              *QuoteReference                `json:"quote,omitempty"`
	ConversationOrigin *ConversationOrigin            `json:"conversationOrigin,omitempty"`
	Attachments        []Attachment                   `json:"attachments,omitempty"`
	ExpectedScope      *coordination.ExecutionScope   `json:"expectedExecutionScope,omitempty"`
	Context            *ForwardedContext              `json:"context,omitempty"`
	ProjectID          string                         `json:"projectId,omitempty"`
	SpaceID            string                         `json:"-"`
	DeviceID           string                         `json:"-"`
	CoreID             string                         `json:"-"`
	ProjectWorkspaces  []ProjectWorkspace             `json:"-"`
	PrimaryWorkspaceID string                         `json:"-"`
	TargetDeviceID     string                         `json:"targetDeviceId,omitempty"`
	RoleID             string                         `json:"characterId,omitempty"`
	HistoricalRoleID   string                         `json:"historicalRoleId,omitempty"`
	ConversationID     string                         `json:"conversationId,omitempty"`
	PermissionMode     string                         `json:"permissionMode,omitempty"`
	ModelConfigID      int                            `json:"modelConfigId,omitempty"`
	ReasoningEffort    string                         `json:"reasoningEffort,omitempty"`
	ReasoningEnabled   *bool                          `json:"reasoningEnabled,omitempty"`
	MessageStyle       string                         `json:"messageStyle,omitempty"`
	RequestID          string                         `json:"requestId"`
	Message            string                         `json:"message"`
}

type ContextMessage struct {
	ID      string `json:"id"`
	OwnerID string `json:"ownerId,omitempty"`
	Role    string `json:"role"`
	Content string `json:"content"`
	Status  string `json:"status,omitempty"`
}

type ForwardedContext struct {
	PreviousCoreID string           `json:"previousCoreId"`
	ConversationID string           `json:"conversationId"`
	Summary        string           `json:"summary,omitempty"`
	Messages       []ContextMessage `json:"messages"`
}

func validateForwardedContext(request Request) error {
	if request.Context == nil {
		return nil
	}
	context := request.Context
	if context.PreviousCoreID == "" || context.ConversationID == "" || context.ConversationID != request.ConversationID || len(context.Summary) > 64<<10 || len(context.Messages) > 128 || len(body(context)) > 1<<20 {
		return errors.New("传递的对话上下文无效或超过上限")
	}
	seen := map[string]bool{}
	for _, message := range context.Messages {
		key := message.OwnerID + "\x00" + message.ID
		if message.ID == "" || len(message.ID) > 512 || len(message.OwnerID) > 512 || strings.ContainsRune(message.OwnerID, '\x00') || strings.ContainsRune(message.ID, '\x00') || seen[key] || (message.Role != "user" && message.Role != "assistant") || len(message.Content) > 128<<10 || message.Status == "sending" || message.Status == "failed" {
			return errors.New("传递的对话上下文包含无效消息")
		}
		seen[key] = true
	}
	return nil
}

type Generation struct {
	StateEffects        json.RawMessage `json:"-"`
	CitationSnapshot    json.RawMessage `json:"citationSnapshot,omitempty"`
	ReasoningDurationMS int64           `json:"reasoningDurationMs,omitempty"`
	Suppressed          bool            `json:"suppressed,omitempty"`
	Outputs             json.RawMessage `json:"outputs,omitempty"`
	ForceVoice          bool            `json:"forceVoice,omitempty"`
	Text                string          `json:"reply"`
	Reasoning           string          `json:"reasoning,omitempty"`
	Tokens              int             `json:"tokens"`
	Partial             bool            `json:"-"`
}

type Event struct {
	Progress       json.RawMessage             `json:"progress,omitempty"`
	Task           *TaskEvent                  `json:"task,omitempty"`
	Type           string                      `json:"type"`
	ConversationID string                      `json:"conversationId,omitempty"`
	Scope          coordination.ExecutionScope `json:"executionScope"`
	Text           string                      `json:"text,omitempty"`
	Reasoning      bool                        `json:"reasoning,omitempty"`
}

type TaskEvent struct {
	CoordinationID string `json:"coordinationId"`
	Status         string `json:"status"`
}

type Inference struct {
	RecoveredRuntime   *conversationstream.ActiveTurnState                  `json:"-"`
	Preview            bool                                                 `json:"-"`
	Ingress            *Ingress                                             `json:"-"`
	RegisterCompletion func(func(context.Context, Generation, error) error) `json:"-"`
	InternalWorker     bool
	ScheduledMessage   bool
	ConsolidateMemory  bool
	Quote              *ReviewedQuote
	Attachments        []Attachment
	Context            *ForwardedContext
	Scope              coordination.ExecutionScope
	ConversationID     string
	ProjectID          string
	Workspaces         []ProjectWorkspace
	PrimaryWorkspaceID string
	Snapshot           coordination.DataSnapshot
	HistoricalSnapshot *coordination.DataSnapshot
	Emit               func(Event) error `json:"-"`
	PermissionMode     string
	ModelConfigID      int
	ReasoningEffort    string
	ReasoningEnabled   *bool
	MessageStyle       string
	Message            string
}

type DerivedMemory struct {
	SourceKind string          `json:"sourceKind,omitempty"`
	Kind       string          `json:"kind"`
	Key        string          `json:"key"`
	Body       json.RawMessage `json:"body"`
}

type Model interface {
	GenerateOwnedReply(context.Context, Inference) (Generation, error)
	ExtractOwnedMemory(context.Context, Inference, Generation) ([]DerivedMemory, error)
}

type OwnedStateMutator interface {
	OwnedStateMutations(context.Context, Inference, Generation) ([]coordination.Mutation, error)
}

type AudioModel interface {
	TranscribeOwnedAudio(context.Context, Inference, Attachment) (string, error)
}

type CommittedModel interface {
	OwnedReplyCommitted(context.Context, Inference, Generation, string) error
}

type SemanticModel interface {
	OwnedQueryVector(context.Context, string) ([]float32, string, error)
}

type Response struct {
	ConversationOrigin *ConversationOrigin            `json:"conversationOrigin,omitempty"`
	Transcription      string                         `json:"transcription,omitempty"`
	UserRevision       int64                          `json:"userRevision,omitempty"`
	Evidence           []coordination.ResourceVersion `json:"evidence,omitempty"`
	ConversationID     string                         `json:"conversationId"`
	RequestID          string                         `json:"requestId"`
	TurnID             string                         `json:"turnId"`
	ExecutionID        string                         `json:"executionId"`
	Scope              coordination.ExecutionScope    `json:"executionScope"`
	Generation
	Saved        bool   `json:"saved"`
	MemoryStatus string `json:"memoryStatus"`
	MemoryError  string `json:"memoryError,omitempty"`
	Interrupted  bool   `json:"interrupted,omitempty"`
}

type checkpoint struct {
	Ingress        *Ingress                            `json:"ingress,omitempty"`
	Runtime        *conversationstream.ActiveTurnState `json:"runtime,omitempty"`
	Context        *ForwardedContext                   `json:"context,omitempty"`
	Hash           string                              `json:"hash"`
	Status         string                              `json:"status"`
	Response       *Response                           `json:"response,omitempty"`
	ConversationID string                              `json:"conversationId"`
	Scope          *coordination.ExecutionScope        `json:"scope,omitempty"`
}

type lane struct {
	mu    sync.Mutex
	users int
}

type Engine struct {
	coordination    *coordination.Service
	data            coordination.DataPort
	model           Model
	mu              sync.Mutex
	lanes           map[string]*lane
	active          map[string]context.CancelCauseFunc
	continuitySlots chan struct{}
}

func NewEngine(service *coordination.Service, data coordination.DataPort, model Model) *Engine {
	return &Engine{coordination: service, data: data, model: model, lanes: make(map[string]*lane), active: make(map[string]context.CancelCauseFunc), continuitySlots: make(chan struct{}, 4)}
}

func (e *Engine) ValidateWiring() error {
	if e == nil || e.coordination == nil || ownedDependencyMissing(e.data) || ownedDependencyMissing(e.model) {
		return errors.New("设备归属业务的协调、数据或计算端口未接通")
	}
	return nil
}

func (e *Engine) lock(key string) func() {
	e.mu.Lock()
	l := e.lanes[key]
	if l == nil {
		l = &lane{}
		e.lanes[key] = l
	}
	l.users++
	e.mu.Unlock()
	l.mu.Lock()
	return func() {
		l.mu.Unlock()
		e.mu.Lock()
		l.users--
		if l.users == 0 {
			delete(e.lanes, key)
		}
		e.mu.Unlock()
	}
}

func body(value any) json.RawMessage { encoded, _ := json.Marshal(value); return encoded }

func (e *Engine) commit(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	if proof := coordination.CommitLease(ctx); proof != nil {
		if commit.LeaseProof == nil {
			commit.LeaseProof = proof
		} else if commit.LeaseProof.ContinuityID != proof.ContinuityID || commit.LeaseProof.LeaseID != proof.LeaseID || commit.LeaseProof.RequestID != proof.RequestID {
			return coordination.Acknowledgement{}, coordination.ErrRequestConflict
		}
	}
	dependencies, err := coordination.CommitDependencies(ctx)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	for _, dependency := range dependencies {
		found := false
		for _, existing := range commit.Dependencies {
			if existing.Kind == dependency.Kind && existing.ID == dependency.ID {
				if existing.Revision != dependency.Revision {
					return coordination.Acknowledgement{}, coordination.ErrResourceVersion
				}
				found = true
				break
			}
		}
		if !found {
			commit.Dependencies = append(commit.Dependencies, dependency)
		}
	}
	return e.data.Commit(ctx, commit)
}

func version(snapshot coordination.DataSnapshot, kind, id string) int64 {
	for _, row := range snapshot.Resources {
		if row.Kind == kind && row.ID == id {
			return row.Revision
		}
	}
	return 0
}

func (e *Engine) submit(ctx context.Context, scope coordination.ExecutionScope, stage string, mutations []coordination.Mutation, dependencies ...coordination.ResourceVersion) error {
	copyScope := scope
	copyScope.RequestID = scope.RequestID + "|" + stage
	_, err := e.commit(ctx, coordination.Commit{Scope: copyScope, Mutations: mutations, Dependencies: dependencies})
	return err
}

func conversationProjectID(snapshot coordination.DataSnapshot, conversationID string) string {
	for _, row := range snapshot.Resources {
		if row.Kind != "conversation" || row.ID != conversationID {
			continue
		}
		var document struct {
			ProjectID string `json:"projectId"`
		}
		if json.Unmarshal(row.Body, &document) != nil {
			return ""
		}
		return strings.TrimSpace(document.ProjectID)
	}
	return ""
}

func (e *Engine) resolveRunProject(ctx context.Context, scope coordination.ExecutionScope, request Request, snapshot coordination.DataSnapshot) (string, []ProjectWorkspace, string, error) {
	if worker, ok := ctx.Value(workerContextKey{}).(workerContext); ok {
		return "", append([]ProjectWorkspace(nil), worker.Workspaces...), worker.PrimaryID, nil
	}
	conversationProject := conversationProjectID(snapshot, request.ConversationID)
	projectID := strings.TrimSpace(request.ProjectID)
	if projectID == "" {
		projectID = conversationProject
	}
	if projectID == "" {
		return "", nil, "", nil
	}
	if conversationProject != "" && conversationProject != projectID {
		return "", nil, "", errors.New("现有对话不能移入或移出项目")
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return "", nil, "", errors.New("数据来源不支持项目工作区解析")
	}
	resource, err := port.Resource(ctx, scope, "project", projectID)
	if err != nil {
		return "", nil, "", err
	}
	if resource == nil || resource.Deleted || resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID {
		return "", nil, "", coordination.ErrWrongOwner
	}
	var project Project
	if json.Unmarshal(resource.Body, &project) != nil || project.ID != projectID {
		return "", nil, "", errors.New("项目数据格式无效")
	}
	if project.HiddenAt != nil {
		return "", nil, "", errors.New("该项目已从侧边栏移除")
	}
	workspaces, primaryID, err := normalizeProjectWorkspaces(project.Workspaces, project.PrimaryWorkspaceID)
	if err != nil || !projectWorkspacesBelongToDevice(workspaces, scope.TargetDeviceID) {
		return "", nil, "", errors.New("项目工作区无效或不属于当前执行设备")
	}
	return projectID, workspaces, primaryID, nil
}

func (e *Engine) Run(ctx context.Context, request Request) (Response, error) {
	return e.RunEvents(ctx, request, nil)
}

func (e *Engine) Interrupt(space, device, request string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	cancel, exists := e.active[space+"\x00"+device+"\x00"+request]
	if exists {
		cancel(ErrInterrupted)
	}
	return exists
}

func (e *Engine) RunEvents(ctx context.Context, request Request, emit func(Event) error) (finalResponse Response, finalErr error) {
	if e.coordination == nil || e.data == nil || e.model == nil {
		return Response{}, errors.New("业务服务尚未初始化")
	}
	request.Message = strings.TrimSpace(request.Message)
	if request.RequestID == "" || len(request.RequestID) > 128 || request.Message == "" || len(request.Message) > 128<<10 {
		return Response{}, errors.New("请求编号和消息内容无效")
	}
	if err := validateForwardedContext(request); err != nil {
		return Response{}, err
	}
	if err := validateConversationSettings(request); err != nil {
		return Response{}, err
	}
	if err := ValidateAttachments(request.Attachments); err != nil {
		return Response{}, err
	}
	if request.ConversationID == "" {
		request.ConversationID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(request.SpaceID+"\x00"+request.DeviceID+"\x00"+request.RequestID)).String()
	}
	if request.TargetDeviceID == "" {
		request.TargetDeviceID = request.DeviceID
	}
	unlock := e.lock(request.SpaceID + "\x00" + request.TargetDeviceID + "\x00" + request.ConversationID)
	defer unlock()
	ctx, scope, finish, err := e.coordination.Begin(ctx, request.SpaceID, request.DeviceID, request.TargetDeviceID, request.CoreID, request.RoleID, request.RequestID)
	if err != nil {
		return Response{}, err
	}
	defer finish()
	if err := e.coordination.RequireCapability(ctx, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, "ai.chat"); err != nil {
		return Response{}, err
	}
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		return Response{}, err
	}
	requestedRole := request.RoleID
	if requestedRole == "" {
		policy, err := e.coordination.Get(ctx, request.SpaceID, request.TargetDeviceID)
		if err != nil {
			return Response{}, err
		}
		requestedRole = policy.SelectedRole
	}
	role, err := coordination.ResolveRole(requestedRole, roles)
	if err != nil {
		return Response{}, err
	}
	scope.RoleID = role.ID
	scope.RoleRevision = role.Revision
	if err := coordination.ResolveMemoryScope(&scope, role, ""); err != nil {
		return Response{}, err
	}
	if request.ExpectedScope != nil && request.ExpectedScope.ContinuityRoleID != "" {
		if coordination.CommitLease(ctx) == nil {
			return Response{}, coordination.ErrWrongOwner
		}
		scope.ContinuityRoleID = request.ExpectedScope.ContinuityRoleID
	}
	if request.ExpectedScope != nil {
		expected := *request.ExpectedScope
		expected.RequestID, expected.TurnID, expected.ExecutionID = scope.RequestID, scope.TurnID, scope.ExecutionID
		if expected != scope {
			return Response{}, coordination.ErrScopeExpired
		}
	}
	scope.TurnID = uuid.NewString()
	scope.ExecutionID = uuid.NewString()
	ctx = coordination.WithScope(ctx, scope)
	conversationRoute, err := e.resolveRunConversationRoute(ctx, scope, request)
	if err != nil {
		return Response{}, err
	}
	quoteRequest := request
	request.ConversationID = conversationRoute.CurrentID
	dataQuery := coordination.DataQuery{RequestID: request.RequestID, ConversationID: request.ConversationID, Query: request.Message, Limit: 128}
	dataQuery.ChatRuntime = &coordination.OwnedChatRuntimeQuery{Source: "manual", Channel: "web"}
	if ingress, ok := IngressFromContext(ctx); ok {
		dataQuery.ChatRuntime = &coordination.OwnedChatRuntimeQuery{Source: ingress.Source, Channel: ingress.Channel, PeerID: ingress.PeerID, IsInternal: ingress.IsInternal, DeviceTimezone: ingress.DeviceTimezone}
	}
	if _, ok := ctx.Value(workerContextKey{}).(workerContext); ok {
		dataQuery.ChatRuntime.IsInternal = true
	}
	previousCheckpoint := false
	if port, ok := e.data.(coordination.ResourcePort); ok {
		resource, err := port.Resource(ctx, scope, "checkpoint", "turn/"+request.RequestID)
		if err != nil {
			return Response{}, err
		}
		previousCheckpoint = resource != nil
	}
	hasAudio := false
	for _, attachment := range request.Attachments {
		hasAudio = hasAudio || attachment.Kind == "audio"
	}
	if model, ok := e.model.(SemanticModel); ok && !previousCheckpoint && !hasAudio {
		dataQuery.Vector, dataQuery.VectorModel, err = model.OwnedQueryVector(ctx, request.Message)
		if err != nil {
			return Response{}, err
		}
	}
	snapshot, err := e.data.Snapshot(ctx, scope, dataQuery)
	if err != nil {
		return Response{}, err
	}
	if err := coordination.ValidateSnapshot(scope, snapshot); err != nil {
		return Response{}, err
	}
	var historicalSnapshot *coordination.DataSnapshot
	if port, ok := e.data.(coordination.HistoricalDataPort); ok && scope.Coordinated && conversationRoute.HistoricalID != "" {
		historicalQuery := dataQuery
		historicalQuery.ConversationID = conversationRoute.HistoricalID
		historicalQuery.HistoricalRoleID = request.HistoricalRoleID
		historicalSnapshot, err = port.HistoricalSnapshot(ctx, scope, historicalQuery)
		if err != nil {
			return Response{}, err
		}
	}
	if err := rejectAmbiguousConversation(conversationRoute, snapshot, historicalSnapshot); err != nil {
		return Response{}, err
	}
	projectID, workspaces, primaryWorkspaceID, err := e.resolveRunProject(ctx, scope, request, snapshot)
	if err != nil {
		return Response{}, err
	}
	request.ProjectID, request.ProjectWorkspaces, request.PrimaryWorkspaceID = projectID, workspaces, primaryWorkspaceID
	if err := resolveConversationSettings(&request, snapshot); err != nil {
		return Response{}, err
	}
	fingerprintData := map[string]any{"conversationId": request.ConversationID, "message": request.Message, "roleId": request.RoleID, "historicalRoleId": request.HistoricalRoleID, "targetDeviceId": request.TargetDeviceID, "context": request.Context, "projectId": projectID, "workspaces": workspaces, "primaryWorkspaceId": primaryWorkspaceID}
	fingerprintData["conversationSettings"] = settingsFromRequest(request)
	fingerprintData["retryRequestId"] = request.RetryRequestID
	if ingress, ok := IngressFromContext(ctx); ok {
		fingerprintData["ingress"] = ingress
	}
	if request.Quote != nil {
		fingerprintData["quote"] = request.Quote
	}
	if conversationRoute.Origin != nil {
		fingerprintData["conversationOrigin"] = conversationRoute.Origin
	}
	if len(request.Attachments) > 0 {
		fingerprintData["attachments"] = request.Attachments
	}
	digest := sha256.Sum256(body(fingerprintData))
	fingerprint := hex.EncodeToString(digest[:])
	location, firstRequest, err := e.coordination.RegisterRequest(ctx, scope, fingerprint, request.ConversationID, request.queuedHash)
	if err != nil {
		if errors.Is(err, coordination.ErrScopeExpired) && location.State == "started" {
			return Response{}, ErrUncertainExecution
		}
		return Response{}, err
	}
	checkpointID := "turn/" + request.RequestID
	for _, row := range snapshot.Resources {
		if row.Kind != "checkpoint" || row.ID != checkpointID {
			continue
		}
		var recorded checkpoint
		if err := json.Unmarshal(row.Body, &recorded); err != nil {
			return Response{}, err
		}
		if recorded.Hash != fingerprint {
			return Response{}, coordination.ErrRequestConflict
		}
		if recorded.Status == "completed" && recorded.Response != nil {
			if recorded.Response.MemoryStatus != "saved" && recorded.Response.MemoryStatus != "skipped" {
				message := request.Message
				if recorded.Response.Transcription != "" {
					message = recorded.Response.Transcription
				}
				inference := Inference{Context: request.Context, Scope: recorded.Response.Scope, ConversationID: request.ConversationID, ProjectID: projectID, Workspaces: workspaces, PrimaryWorkspaceID: primaryWorkspaceID, Snapshot: snapshot, HistoricalSnapshot: historicalSnapshot, PermissionMode: request.PermissionMode, Message: message}
				inference.Ingress = recorded.Ingress
				return e.completeMemory(coordination.WithScope(ctx, recorded.Response.Scope), inference, fingerprint, *recorded.Response, row.Revision)
			}
			return *recorded.Response, nil
		}
		return Response{}, ErrUncertainExecution
	}
	if !firstRequest {
		return Response{}, ErrUncertainExecution
	}
	ingress, hasIngress := IngressFromContext(ctx)
	quote, quoteDependencies, err := e.reviewQuote(ctx, scope, quoteRequest, request.Quote)
	if err != nil {
		return Response{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	conversation := map[string]any{"id": request.ConversationID, "conversationId": request.ConversationID, "title": request.Message, "characterId": role.ID, "createdAt": now, "ownerId": scope.ResourceOwnerID, "coreId": scope.CoreID}
	if hasIngress {
		if ingress.Channel != "" {
			conversation["channel"] = ingress.Channel
		}
		if ingress.Source != "" {
			conversation["source"] = ingress.Source
		}
		if ingress.PeerID != "" {
			conversation["peerId"] = ingress.PeerID
		}
	}
	if projectID != "" {
		conversation["projectId"] = projectID
	}
	if conversationRoute.Origin != nil {
		conversation["conversationOrigin"] = conversationRoute.Origin
	}
	for _, row := range snapshot.Resources {
		if row.Kind == "conversation" && row.ID == request.ConversationID {
			if err := json.Unmarshal(row.Body, &conversation); err != nil {
				return Response{}, err
			}
			break
		}
	}
	conversation["updatedAt"] = now
	previousMessageSequence := conversation["lastMessageSequence"]
	userSequence := nextOwnedMessageSequence(snapshot, request.ConversationID)
	conversation["lastMessageSequence"] = userSequence + 1
	applyConversationSettings(conversation, request)
	input := []coordination.Mutation{
		{Kind: "conversation", ID: request.ConversationID, RoleID: role.ID, ExpectedRevision: version(snapshot, "conversation", request.ConversationID), Body: body(conversation)},
		{Kind: "message", ID: request.RequestID + "/user", RoleID: role.ID, Body: body(map[string]any{"id": request.RequestID + "/user", "conversationId": request.ConversationID, "characterId": role.ID, "role": "user", "content": request.Message, "requestId": request.RequestID, "createdAt": now, "executionScope": scope, "attachments": request.Attachments, "quote": quote})},
		{Kind: "checkpoint", ID: checkpointID, RoleID: role.ID, Body: body(checkpoint{Hash: fingerprint, Status: "running", ConversationID: request.ConversationID, Scope: &scope, Context: request.Context})},
	}
	var userDocument map[string]any
	if err := json.Unmarshal(input[1].Body, &userDocument); err != nil {
		return Response{}, err
	}
	userDocument["sequence"] = userSequence
	input[1].Body = body(userDocument)
	var storedIngress *Ingress
	if hasIngress {
		storedIngress = &ingress
	}
	input[len(input)-1].Body = body(checkpoint{Hash: fingerprint, Status: "running", ConversationID: request.ConversationID, Scope: &scope, Context: request.Context, Ingress: storedIngress})
	if hasIngress && ingress.SuppressReplyPersistence {
		if previousMessageSequence == nil {
			delete(conversation, "lastMessageSequence")
		} else {
			conversation["lastMessageSequence"] = previousMessageSequence
		}
		input[0].Body = body(conversation)
		input = append(input[:1], input[2:]...)
	}
	if err := e.submit(ctx, scope, "input", input, quoteDependencies...); err != nil {
		return Response{}, fmt.Errorf("输入尚未确认保存: %w", err)
	}
	ctx, cancel := context.WithCancelCause(ctx)
	activeKey := scope.SpaceID + "\x00" + scope.InitiatorDeviceID + "\x00" + scope.RequestID
	e.mu.Lock()
	e.active[activeKey] = cancel
	e.mu.Unlock()
	defer func() { e.mu.Lock(); delete(e.active, activeKey); e.mu.Unlock(); cancel(nil) }()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := coordination.ValidateCurrent(ctx); err != nil {
					cancel(err)
					return
				}
				if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
					cancel(err)
					return
				}
			}
		}
	}()
	emitCurrent := func(event Event) error {
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return err
		}
		if err := e.coordination.Validate(ctx, scope); err != nil {
			return err
		}
		event.Scope = scope
		event.ConversationID = request.ConversationID
		if err := publishRuntime(ctx, Inference{Scope: scope, ConversationID: request.ConversationID}, event); err != nil {
			return err
		}
		if event.Type == "tool-started" || event.Type == "tool-completed" || event.Type == "task" {
			if err := e.persistOwnedRuntime(ctx, scope, request.ConversationID); err != nil {
				return err
			}
		}
		if emit != nil {
			return emit(event)
		}
		return nil
	}
	inference := Inference{Quote: quote, Attachments: request.Attachments, Context: request.Context, Scope: scope, ConversationID: request.ConversationID, ProjectID: projectID, Workspaces: workspaces, PrimaryWorkspaceID: primaryWorkspaceID, Snapshot: snapshot, HistoricalSnapshot: historicalSnapshot, PermissionMode: request.PermissionMode, Message: request.Message, Emit: emitCurrent}
	inference.ModelConfigID, inference.ReasoningEffort, inference.ReasoningEnabled = request.ModelConfigID, request.ReasoningEffort, request.ReasoningEnabled
	inference.MessageStyle = request.MessageStyle
	if request.RetryRequestID != "" {
		inference.RecoveredRuntime, err = e.loadOwnedRetryRuntime(ctx, scope, request)
		if err != nil {
			return Response{}, err
		}
	}
	if ingress, ok := IngressFromContext(ctx); ok {
		inference.Ingress = &ingress
	}
	finishRuntime, err := beginRuntime(ctx, inference, cancel)
	if err != nil {
		return Response{}, err
	}
	defer finishRuntime()
	defer func() {
		status, eventType := "completed", "turn.completed"
		if finalErr != nil || finalResponse.Interrupted || !finalResponse.Saved {
			status, eventType = "failed", "turn.failed"
			if finalResponse.Interrupted {
				status, eventType = "interrupted", "turn.interrupted"
			}
		}
		payload := map[string]any{"saved": finalResponse.Saved}
		if finalErr != nil {
			payload["message"] = finalErr.Error()
		}
		_, _ = ownedRuntimeManager.Publish(context.WithoutCancel(ctx), conversationstream.AgentUIEvent{ConversationID: RuntimeKey(scope, request.ConversationID), RequestID: scope.RequestID, TurnID: scope.TurnID, ExecutionID: scope.ExecutionID, Type: eventType, Status: status, Payload: payload}, false)
		saveContext, saveCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer saveCancel()
		if err := e.persistOwnedRuntime(saveContext, scope, request.ConversationID); err != nil {
			finalErr = errors.Join(finalErr, err)
		}
	}()
	_, inference.InternalWorker = ctx.Value(workerContextKey{}).(workerContext)
	var completeTurn func(context.Context, Generation, error) error
	inference.RegisterCompletion = func(finish func(context.Context, Generation, error) error) { completeTurn = finish }
	defer func() {
		if completeTurn == nil {
			return
		}
		cause := finalErr
		if current := context.Cause(ctx); current != nil {
			cause = current
		}
		if cause == nil && (!finalResponse.Saved || finalResponse.Interrupted) {
			cause = ErrInterrupted
		}
		finishContext, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer finishCancel()
		if err := completeTurn(finishContext, finalResponse.Generation, cause); err != nil && finalErr == nil {
			finalErr = err
		}
	}()
	if err := emitCurrent(Event{Type: "started"}); err != nil {
		cancel(err)
	}
	var transcription string
	var userRevision int64
	if !hasIngress || !ingress.SuppressReplyPersistence {
		transcription, userRevision, err = e.transcribeAudio(ctx, &inference, input[1])
	}
	var generation Generation
	if err == nil {
		generation, err = e.model.GenerateOwnedReply(ctx, inference)
	}
	if err != nil {
		if ctx.Err() != nil {
			response := Response{ConversationID: request.ConversationID, RequestID: request.RequestID, TurnID: scope.TurnID, ExecutionID: scope.ExecutionID, Scope: scope, Interrupted: true, MemoryStatus: "interrupted"}
			if generation.Partial {
				response.Generation = generation
			}
			if port, ok := e.data.(coordination.InterruptionPort); ok && ctx.Value(realtimeContextKey{}) != true {
				saveContext, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
				saveErr := port.SaveInterrupted(saveContext, coordination.InterruptedReply{Scope: scope, ConversationID: request.ConversationID, Text: response.Text, Reasoning: response.Reasoning, Reason: context.Cause(ctx).Error()})
				saveCancel()
				response.Saved = saveErr == nil
				if saveErr != nil {
					return response, errors.Join(context.Cause(ctx), saveErr)
				}
			}
			return response, context.Cause(ctx)
		}
		return Response{}, err
	}
	if err := e.coordination.Validate(ctx, scope); err != nil {
		return Response{}, err
	}
	if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
		return Response{}, err
	}
	var stateMutations []coordination.Mutation
	if mutator, ok := e.model.(OwnedStateMutator); ok && !generation.Suppressed && (!hasIngress || !ingress.SuppressReplyPersistence) {
		stateMutations, err = mutator.OwnedStateMutations(ctx, inference, generation)
		if err != nil {
			return Response{}, fmt.Errorf("角色运行状态尚未确认保存: %w", err)
		}
	}
	response := Response{Transcription: transcription, UserRevision: userRevision, ConversationID: request.ConversationID, RequestID: request.RequestID, TurnID: scope.TurnID, ExecutionID: scope.ExecutionID, Scope: scope, Generation: generation, Saved: true, MemoryStatus: "pending"}
	if generation.Suppressed || hasIngress && ingress.SuppressReplyPersistence {
		response.MemoryStatus = "skipped"
	}
	response.ConversationOrigin = conversationRoute.Origin
	for _, resource := range inference.Snapshot.Resources {
		if resource.Kind == "message" {
			response.Evidence = append(response.Evidence, coordination.ResourceVersion{Kind: "message", ID: resource.ID, Revision: resource.Revision})
		}
	}
	if !hasIngress || !ingress.SuppressReplyPersistence {
		response.Evidence = append(response.Evidence, coordination.ResourceVersion{Kind: "message", ID: request.RequestID + "/user", Revision: userRevision})
	}
	checkpointRevision, err := e.ownedCheckpointRevision(ctx, scope)
	if err != nil {
		return response, err
	}
	output := []coordination.Mutation{
		{Kind: "message", ID: request.RequestID + "/assistant", RoleID: role.ID, Body: body(map[string]any{"id": request.RequestID + "/assistant", "conversationId": request.ConversationID, "characterId": role.ID, "role": "assistant", "content": generation.Text, "reasoningContent": generation.Reasoning, "tokens": generation.Tokens, "outputs": generation.Outputs, "forceVoice": generation.ForceVoice, "requestId": request.RequestID, "createdAt": time.Now().UTC().Format(time.RFC3339Nano), "executionScope": scope})},
		{Kind: "checkpoint", ID: checkpointID, RoleID: role.ID, ExpectedRevision: checkpointRevision, Body: body(checkpoint{Hash: fingerprint, Status: "completed", ConversationID: request.ConversationID, Response: &response, Context: request.Context, Runtime: ownedRuntimeManager.RuntimeSnapshot(RuntimeKey(scope, request.ConversationID)).ActiveTurn})},
	}
	output[1].Body = body(checkpoint{Hash: fingerprint, Status: "completed", ConversationID: request.ConversationID, Response: &response, Context: request.Context, Runtime: ownedRuntimeManager.RuntimeSnapshot(RuntimeKey(scope, request.ConversationID)).ActiveTurn, Ingress: storedIngress})
	var assistantDocument map[string]any
	if err := json.Unmarshal(output[0].Body, &assistantDocument); err != nil {
		return response, err
	}
	assistantDocument["sequence"], assistantDocument["reasoningDurationMs"] = userSequence+1, generation.ReasoningDurationMS
	if len(generation.CitationSnapshot) == 0 {
		assistantDocument["citationSnapshot"] = json.RawMessage("[]")
	} else {
		assistantDocument["citationSnapshot"] = generation.CitationSnapshot
	}
	output[0].Body = body(assistantDocument)
	if generation.Suppressed || hasIngress && ingress.SuppressReplyPersistence {
		output = output[1:]
	}
	output = append(output, stateMutations...)
	if hasIngress && !generation.Suppressed && !ingress.SuppressReplyPersistence && ingress.Channel != "" && ingress.Channel != "web" && ingress.Channel != "voice" && ingress.Channel != "device-mesh" && ingress.PeerID != "" {
		pending := coordination.PendingChannelDelivery{Pending: true, Scope: scope, Request: coordination.ChannelDeliveryRequest{ConversationID: request.ConversationID, RequestID: scope.RequestID, Channel: ingress.Channel, PeerID: ingress.PeerID, Text: generation.Text, Outputs: generation.Outputs}}
		output = append(output, coordination.Mutation{Kind: "checkpoint", ID: "channel-delivery/" + scope.RequestID, RoleID: scope.RoleID, Body: body(pending)})
	}
	if err := e.submit(ctx, scope, "reply", output, response.Evidence...); err != nil {
		response.Saved = false
		return response, fmt.Errorf("回复尚未确认保存: %w", err)
	}
	if err := e.coordination.CompleteRequest(ctx, scope); err != nil {
		return response, err
	}
	if committed, ok := e.model.(CommittedModel); ok && !generation.Suppressed && (!hasIngress || !ingress.SuppressReplyPersistence) {
		if err := committed.OwnedReplyCommitted(ctx, inference, generation, request.RequestID+"/assistant"); err != nil {
			return response, fmt.Errorf("回复已保存，回复后处理失败: %w", err)
		}
	}
	if generation.Suppressed || hasIngress && ingress.SuppressReplyPersistence {
		response.MemoryStatus = "skipped"
		return response, nil
	}
	response, err = e.completeMemory(ctx, inference, fingerprint, response, checkpointRevision+1)
	if err != nil {
		return response, err
	}
	if err := e.finishOwnedTitle(ctx, inference, generation); err != nil {
		return response, fmt.Errorf("回复已保存，会话标题更新失败: %w", err)
	}
	return response, nil
}

func (e *Engine) completeMemory(ctx context.Context, inference Inference, fingerprint string, response Response, checkpointRevision int64) (Response, error) {
	ctx = context.WithValue(ctx, forwardedMemoryContextKey{}, inference.Context)
	if inference.Ingress != nil {
		ctx = WithIngress(ctx, *inference.Ingress)
	}
	scope := inference.Scope
	if err := e.coordination.Validate(ctx, scope); err != nil {
		return response, err
	}
	if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
		return response, err
	}
	if err := e.finishOwnedEffects(ctx, inference, response.Generation); err != nil {
		return response, fmt.Errorf("回复已保存，Owner后处理失败: %w", err)
	}
	var planned *coordination.Commit
	for _, resource := range inference.Snapshot.Resources {
		if resource.Kind == "checkpoint" && resource.ID == "memory/"+scope.RequestID {
			var commit coordination.Commit
			if err := json.Unmarshal(resource.Body, &commit); err != nil {
				return response, err
			}
			if commit.Scope != memoryScope(scope) {
				return response, coordination.ErrRequestConflict
			}
			planned = &commit
			break
		}
	}
	if planned != nil {
		if _, err := e.commit(ctx, *planned); err != nil {
			response.MemoryStatus, response.MemoryError = memoryFailureStatus(err), err.Error()
			return e.recordMemoryStatus(ctx, scope, fingerprint, response, checkpointRevision)
		}
		response.MemoryStatus, response.MemoryError = "saved", ""
		return e.recordMemoryStatus(ctx, scope, fingerprint, response, checkpointRevision, true)
	}
	derived, err := e.model.ExtractOwnedMemory(ctx, inference, response.Generation)
	if err != nil {
		response.MemoryStatus = "failed"
		response.MemoryError = err.Error()
		return e.recordMemoryStatus(ctx, scope, fingerprint, response, checkpointRevision)
	}
	if err := e.coordination.Validate(ctx, scope); err != nil {
		return response, err
	}
	mutations, err := e.prepareMemoryMutations(ctx, scope, response.ConversationID, modelMemoryDerived(derived), &inference.Snapshot)
	if err != nil {
		response.MemoryStatus = "failed"
		if memoryFailureStatus(err) == "paused" {
			response.MemoryStatus = "paused"
		}
		response.MemoryError = err.Error()
		return e.recordMemoryStatus(ctx, scope, fingerprint, response, checkpointRevision)
	}
	mutations = reinforceMemoryMutations(mutations, inference.Snapshot, scope, time.Now())
	if len(mutations) > 0 {
		dependencies := append([]coordination.ResourceVersion(nil), response.Evidence...)
		for _, resource := range inference.Snapshot.Resources {
			switch resource.Kind {
			case "memory", "fact", "vector", "graph", "profile", "episodic", "working", "summary":
				dependencies = append(dependencies, coordination.ResourceVersion{Kind: resource.Kind, ID: resource.ID, Revision: resource.Revision})
			}
		}
		planned = &coordination.Commit{Scope: memoryScope(scope), Mutations: mutations, Dependencies: dependencies}
		if proof := coordination.CommitLease(ctx); proof != nil {
			proof.AllowCompleted = true
			planned.LeaseProof = proof
		}
		leaseDependencies, err := coordination.CommitDependencies(ctx)
		if err != nil {
			return response, err
		}
		planned.Dependencies = append(planned.Dependencies, leaseDependencies...)
		plan := coordination.Mutation{Kind: "checkpoint", ID: "memory/" + scope.RequestID, RoleID: scope.RoleID, Body: body(planned)}
		if err := e.submit(ctx, scope, "memory-plan", []coordination.Mutation{plan}); err != nil {
			response.MemoryStatus, response.MemoryError = "pending", err.Error()
			return e.recordMemoryStatus(ctx, scope, fingerprint, response, checkpointRevision)
		}
		if _, err := e.commit(ctx, *planned); err != nil {
			response.MemoryStatus = memoryFailureStatus(err)
			response.MemoryError = err.Error()
			return e.recordMemoryStatus(ctx, scope, fingerprint, response, checkpointRevision)
		}
	}
	response.MemoryStatus = "saved"
	response.MemoryError = ""
	return e.recordMemoryStatus(ctx, scope, fingerprint, response, checkpointRevision, len(mutations) > 0)
}

func memoryFailureStatus(err error) string {
	if errors.Is(err, coordination.ErrResourceVersion) || errors.Is(err, coordination.ErrScopeExpired) || errors.Is(err, coordination.ErrRoleRequired) || errors.Is(err, coordination.ErrWrongOwner) {
		return "paused"
	}
	return "pending"
}

func memoryScope(scope coordination.ExecutionScope) coordination.ExecutionScope {
	scope.RequestID += "|memory"
	return scope
}

func (e *Engine) recordMemoryStatus(ctx context.Context, scope coordination.ExecutionScope, fingerprint string, response Response, checkpointRevision int64, completePlan ...bool) (Response, error) {
	forwarded, _ := ctx.Value(forwardedMemoryContextKey{}).(*ForwardedContext)
	runtime, err := e.savedRuntime(ctx, scope, response.ConversationID)
	if err != nil {
		return response, err
	}
	var ingress *Ingress
	if current, ok := IngressFromContext(ctx); ok {
		ingress = &current
	}
	mutation := coordination.Mutation{Kind: "checkpoint", ID: "turn/" + scope.RequestID, RoleID: scope.RoleID, ExpectedRevision: checkpointRevision, Body: body(checkpoint{Hash: fingerprint, Status: "completed", ConversationID: response.ConversationID, Response: &response, Context: forwarded, Runtime: runtime, Ingress: ingress})}
	mutations := []coordination.Mutation{mutation}
	if len(completePlan) > 0 && completePlan[0] {
		mutations = append(mutations, coordination.Mutation{Kind: "checkpoint", ID: "memory/" + scope.RequestID, RoleID: scope.RoleID, ExpectedRevision: 1, Deleted: true, Body: json.RawMessage(`null`)})
	}
	if err := e.submit(ctx, scope, fmt.Sprintf("memory-status/%d", checkpointRevision), mutations); err != nil {
		return response, err
	}
	if response.MemoryStatus == "saved" {
		if err := e.coordination.FinishMemoryJob(ctx, scope); err != nil {
			return response, err
		}
	}
	return response, nil
}

func memoryMutations(scope coordination.ExecutionScope, conversation string, derived []DerivedMemory, snapshot coordination.DataSnapshot) ([]coordination.Mutation, error) {
	derived = partitionMemoryDerived(scope, derived)
	if len(derived) > 160 {
		return nil, coordination.ErrPendingLimit
	}
	result := make([]coordination.Mutation, 0, len(derived)*2)
	seen := make(map[string]bool)
	sources := make(map[string]string)
	for _, item := range derived {
		if item.Kind != "fact" && item.Kind != "profile" && item.Kind != "episodic" {
			continue
		}
		if item.Key == "" || !json.Valid(item.Body) {
			return nil, errors.New("记忆计算结果无效")
		}
		var content map[string]json.RawMessage
		if json.Unmarshal(item.Body, &content) != nil || strings.TrimSpace(MemoryValue(content)) == "" {
			return nil, errors.New("记忆正文没有可用内容")
		}
		key := item.Kind + "/" + item.Key
		if sources[key] != "" {
			return nil, coordination.ErrRequestConflict
		}
		digest := sha256.Sum256([]byte(coordination.StorageRole(scope, "memory") + "\x00memory\x00" + key))
		id := hex.EncodeToString(digest[:16])
		sources[key] = id
		document := map[string]json.RawMessage{"conversationId": body(conversation), "key": body(item.Key), "kind": body(item.Kind), "content": item.Body, "executionScope": body(scope), "createdAt": body(time.Now().UTC().Format(time.RFC3339Nano)), "updatedAt": body(time.Now().UTC().Format(time.RFC3339Nano))}
		var metadata struct {
			RetentionLevel int    `json:"retentionLevel"`
			Pinned         bool   `json:"pinned"`
			Importance     int    `json:"importance"`
			MemoryType     string `json:"memoryType"`
			MemorySubtype  string `json:"memorySubtype"`
		}
		_ = json.Unmarshal(item.Body, &metadata)
		state := memorypolicy.NewLifecycle(metadata.RetentionLevel, metadata.Pinned, metadata.Importance, time.Now())
		if metadata.RetentionLevel == 0 {
			state = memorypolicy.AssignLifecycle(metadata.MemoryType, metadata.MemorySubtype, metadata.Importance, metadata.Pinned, time.Now())
		}
		for _, previous := range snapshot.Resources {
			if previous.Kind == "memory" && previous.ID == id {
				var old map[string]json.RawMessage
				if json.Unmarshal(previous.Body, &old) == nil {
					state = memorypolicy.ReadLifecycle(old, time.Now())
					for _, field := range []string{"allowContextUse", "expiresAt", "archivedAt", "createdAt"} {
						if value, found := old[field]; found {
							document[field] = value
						}
					}
				}
			}
		}
		if err := state.Validate(); err != nil {
			return nil, err
		}
		memorypolicy.WriteLifecycle(document, state)
		result = append(result, coordination.Mutation{Kind: "memory", ID: id, RoleID: coordination.StorageRole(scope, "memory"), ExpectedRevision: version(snapshot, "memory", id), Body: body(document)})
	}
	for _, item := range derived {
		if item.Key == "" || !json.Valid(item.Body) {
			return nil, errors.New("记忆计算结果无效")
		}
		switch item.Kind {
		case "working", "profile", "episodic", "fact", "vector", "graph", "summary":
		default:
			return nil, errors.New("记忆计算结果类型无效")
		}
		identityKey := item.Key
		if (item.Kind == "vector" || item.Kind == "graph") && item.SourceKind != "" && item.SourceKind != "fact" {
			identityKey = item.SourceKind + "\x00" + item.Key
		}
		digest := sha256.Sum256([]byte(coordination.StorageRole(scope, item.Kind) + "\x00" + item.Kind + "\x00" + identityKey))
		id := hex.EncodeToString(digest[:16])
		if item.Kind == "working" || item.Kind == "summary" {
			id = conversation + "/" + item.Kind
		}
		if seen[item.Kind+"/"+id] {
			return nil, coordination.ErrRequestConflict
		}
		seen[item.Kind+"/"+id] = true
		wrapped := body(map[string]any{"conversationId": conversation, "key": item.Key, "content": item.Body, "executionScope": scope})
		mutation := coordination.Mutation{Kind: item.Kind, ID: id, RoleID: coordination.StorageRole(scope, item.Kind), ExpectedRevision: version(snapshot, item.Kind, id), Body: wrapped}
		if item.Kind == "fact" || item.Kind == "profile" || item.Kind == "episodic" {
			mutation.SourceID = sources[item.Kind+"/"+item.Key]
		}
		if item.Kind == "vector" || item.Kind == "graph" {
			sourceKind := item.SourceKind
			if sourceKind == "" {
				sourceKind = "fact"
			}
			sourceID := sources[sourceKind+"/"+item.Key]
			if sourceID == "" {
				return nil, errors.New("向量或图谱必须引用本轮明确的记忆来源")
			}
			mutation.SourceID = sourceID
		}
		result = append(result, mutation)
	}
	return result, nil
}
