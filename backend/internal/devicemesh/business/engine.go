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
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

var ErrUncertainExecution = errors.New("上次执行的结果尚未确认，请查询结果后再继续，系统不会重复执行已发起的动作")
var ErrInterrupted = errors.New("当前回复已主动停止")

type Request struct {
	Quote              *QuoteReference              `json:"quote,omitempty"`
	ConversationOrigin *ConversationOrigin          `json:"conversationOrigin,omitempty"`
	Attachments        []Attachment                 `json:"attachments,omitempty"`
	ExpectedScope      *coordination.ExecutionScope `json:"expectedExecutionScope,omitempty"`
	Context            *ForwardedContext            `json:"context,omitempty"`
	SpaceID            string                       `json:"-"`
	DeviceID           string                       `json:"-"`
	CoreID             string                       `json:"-"`
	TargetDeviceID     string                       `json:"targetDeviceId,omitempty"`
	RoleID             string                       `json:"characterId,omitempty"`
	HistoricalRoleID   string                       `json:"historicalRoleId,omitempty"`
	ConversationID     string                       `json:"conversationId,omitempty"`
	RequestID          string                       `json:"requestId"`
	Message            string                       `json:"message"`
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
	Text      string `json:"reply"`
	Reasoning string `json:"reasoning,omitempty"`
	Tokens    int    `json:"tokens"`
	Partial   bool   `json:"-"`
}

type Event struct {
	Type           string                      `json:"type"`
	ConversationID string                      `json:"conversationId,omitempty"`
	Scope          coordination.ExecutionScope `json:"executionScope"`
	Text           string                      `json:"text,omitempty"`
	Reasoning      bool                        `json:"reasoning,omitempty"`
}

type Inference struct {
	Quote              *ReviewedQuote
	Attachments        []Attachment
	Context            *ForwardedContext
	Scope              coordination.ExecutionScope
	ConversationID     string
	Snapshot           coordination.DataSnapshot
	HistoricalSnapshot *coordination.DataSnapshot
	Emit               func(Event) error `json:"-"`
	Message            string
}

type DerivedMemory struct {
	Kind string          `json:"kind"`
	Key  string          `json:"key"`
	Body json.RawMessage `json:"body"`
}

type Model interface {
	GenerateOwnedReply(context.Context, Inference) (Generation, error)
	ExtractOwnedMemory(context.Context, Inference, Generation) ([]DerivedMemory, error)
}

type AudioModel interface {
	TranscribeOwnedAudio(context.Context, Inference, Attachment) (string, error)
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
	Context        *ForwardedContext            `json:"context,omitempty"`
	Hash           string                       `json:"hash"`
	Status         string                       `json:"status"`
	Response       *Response                    `json:"response,omitempty"`
	ConversationID string                       `json:"conversationId"`
	Scope          *coordination.ExecutionScope `json:"scope,omitempty"`
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

func (e *Engine) RunEvents(ctx context.Context, request Request, emit func(Event) error) (Response, error) {
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
	fingerprintData := map[string]any{"conversationId": request.ConversationID, "message": request.Message, "roleId": request.RoleID, "historicalRoleId": request.HistoricalRoleID, "targetDeviceId": request.TargetDeviceID, "context": request.Context}
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
	location, firstRequest, err := e.coordination.RegisterRequest(ctx, scope, fingerprint, request.ConversationID)
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
			if recorded.Response.MemoryStatus != "saved" {
				message := request.Message
				if recorded.Response.Transcription != "" {
					message = recorded.Response.Transcription
				}
				inference := Inference{Context: request.Context, Scope: recorded.Response.Scope, ConversationID: request.ConversationID, Snapshot: snapshot, HistoricalSnapshot: historicalSnapshot, Message: message}
				return e.completeMemory(coordination.WithScope(ctx, recorded.Response.Scope), inference, fingerprint, *recorded.Response, row.Revision)
			}
			return *recorded.Response, nil
		}
		return Response{}, ErrUncertainExecution
	}
	if !firstRequest {
		return Response{}, ErrUncertainExecution
	}
	quote, quoteDependencies, err := e.reviewQuote(ctx, scope, quoteRequest, request.Quote)
	if err != nil {
		return Response{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	conversation := map[string]any{"id": request.ConversationID, "conversationId": request.ConversationID, "title": request.Message, "characterId": role.ID, "createdAt": now, "ownerId": scope.ResourceOwnerID, "coreId": scope.CoreID}
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
	input := []coordination.Mutation{
		{Kind: "conversation", ID: request.ConversationID, RoleID: role.ID, ExpectedRevision: version(snapshot, "conversation", request.ConversationID), Body: body(conversation)},
		{Kind: "message", ID: request.RequestID + "/user", RoleID: role.ID, Body: body(map[string]any{"id": request.RequestID + "/user", "conversationId": request.ConversationID, "characterId": role.ID, "role": "user", "content": request.Message, "requestId": request.RequestID, "createdAt": now, "executionScope": scope, "attachments": request.Attachments, "quote": quote})},
		{Kind: "checkpoint", ID: checkpointID, RoleID: role.ID, Body: body(checkpoint{Hash: fingerprint, Status: "running", ConversationID: request.ConversationID, Scope: &scope, Context: request.Context})},
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
		if emit != nil {
			return emit(event)
		}
		return nil
	}
	inference := Inference{Quote: quote, Attachments: request.Attachments, Context: request.Context, Scope: scope, ConversationID: request.ConversationID, Snapshot: snapshot, HistoricalSnapshot: historicalSnapshot, Message: request.Message, Emit: emitCurrent}
	if err := emitCurrent(Event{Type: "started"}); err != nil {
		cancel(err)
	}
	transcription, userRevision, err := e.transcribeAudio(ctx, &inference, input[1])
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
	response := Response{Transcription: transcription, UserRevision: userRevision, ConversationID: request.ConversationID, RequestID: request.RequestID, TurnID: scope.TurnID, ExecutionID: scope.ExecutionID, Scope: scope, Generation: generation, Saved: true, MemoryStatus: "pending"}
	response.ConversationOrigin = conversationRoute.Origin
	for _, resource := range inference.Snapshot.Resources {
		if resource.Kind == "message" {
			response.Evidence = append(response.Evidence, coordination.ResourceVersion{Kind: "message", ID: resource.ID, Revision: resource.Revision})
		}
	}
	response.Evidence = append(response.Evidence, coordination.ResourceVersion{Kind: "message", ID: request.RequestID + "/user", Revision: userRevision})
	output := []coordination.Mutation{
		{Kind: "message", ID: request.RequestID + "/assistant", RoleID: role.ID, Body: body(map[string]any{"id": request.RequestID + "/assistant", "conversationId": request.ConversationID, "characterId": role.ID, "role": "assistant", "content": generation.Text, "reasoningContent": generation.Reasoning, "tokens": generation.Tokens, "requestId": request.RequestID, "createdAt": time.Now().UTC().Format(time.RFC3339Nano), "executionScope": scope})},
		{Kind: "checkpoint", ID: checkpointID, RoleID: role.ID, ExpectedRevision: 1, Body: body(checkpoint{Hash: fingerprint, Status: "completed", ConversationID: request.ConversationID, Response: &response, Context: request.Context})},
	}
	if err := e.submit(ctx, scope, "reply", output, response.Evidence...); err != nil {
		response.Saved = false
		return response, fmt.Errorf("回复尚未确认保存: %w", err)
	}
	if err := e.coordination.CompleteRequest(ctx, scope); err != nil {
		return response, err
	}
	return e.completeMemory(ctx, inference, fingerprint, response, 2)
}

func (e *Engine) completeMemory(ctx context.Context, inference Inference, fingerprint string, response Response, checkpointRevision int64) (Response, error) {
	ctx = context.WithValue(ctx, forwardedMemoryContextKey{}, inference.Context)
	scope := inference.Scope
	if err := e.coordination.Validate(ctx, scope); err != nil {
		return response, err
	}
	if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
		return response, err
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
	mutations, err := e.prepareMemoryMutations(ctx, scope, response.ConversationID, derived, &inference.Snapshot)
	if err != nil {
		response.MemoryStatus = "failed"
		if memoryFailureStatus(err) == "paused" {
			response.MemoryStatus = "paused"
		}
		response.MemoryError = err.Error()
		return e.recordMemoryStatus(ctx, scope, fingerprint, response, checkpointRevision)
	}
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
	mutation := coordination.Mutation{Kind: "checkpoint", ID: "turn/" + scope.RequestID, RoleID: scope.RoleID, ExpectedRevision: checkpointRevision, Body: body(checkpoint{Hash: fingerprint, Status: "completed", ConversationID: response.ConversationID, Response: &response, Context: forwarded})}
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
	if len(derived) > 80 {
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
		key := item.Kind + "/" + item.Key
		if sources[key] != "" {
			return nil, coordination.ErrRequestConflict
		}
		digest := sha256.Sum256([]byte(scope.RoleID + "\x00memory\x00" + key))
		id := hex.EncodeToString(digest[:16])
		sources[key] = id
		result = append(result, coordination.Mutation{Kind: "memory", ID: id, RoleID: scope.RoleID, ExpectedRevision: version(snapshot, "memory", id), Body: body(map[string]any{"conversationId": conversation, "key": item.Key, "kind": item.Kind, "content": item.Body, "executionScope": scope})})
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
		digest := sha256.Sum256([]byte(scope.RoleID + "\x00" + item.Kind + "\x00" + item.Key))
		id := hex.EncodeToString(digest[:16])
		if item.Kind == "working" || item.Kind == "summary" {
			id = conversation + "/" + item.Kind
		}
		if seen[item.Kind+"/"+id] {
			return nil, coordination.ErrRequestConflict
		}
		seen[item.Kind+"/"+id] = true
		wrapped := body(map[string]any{"conversationId": conversation, "key": item.Key, "content": item.Body, "executionScope": scope})
		mutation := coordination.Mutation{Kind: item.Kind, ID: id, RoleID: scope.RoleID, ExpectedRevision: version(snapshot, item.Kind, id), Body: wrapped}
		if item.Kind == "fact" || item.Kind == "profile" || item.Kind == "episodic" {
			mutation.SourceID = sources[item.Kind+"/"+item.Key]
		}
		if item.Kind == "vector" || item.Kind == "graph" {
			sourceID := sources["fact/"+item.Key]
			if sourceID == "" {
				return nil, errors.New("向量或图谱必须引用本轮明确的事实来源")
			}
			mutation.SourceID = sourceID
		}
		result = append(result, mutation)
	}
	return result, nil
}
