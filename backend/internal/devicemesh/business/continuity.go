package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type OwnedContinuity struct {
	MemoryPartition    string                       `json:"memoryPartition,omitempty"`
	ReadOnly           bool                         `json:"readOnly,omitempty"`
	ManagementScope    *coordination.ExecutionScope `json:"managementExecutionScope,omitempty"`
	PausedReason       string                       `json:"pausedReason,omitempty"`
	Scope              coordination.ExecutionScope  `json:"executionScope"`
	Thread             continuity.Thread            `json:"thread"`
	Bindings           []continuity.ThreadBinding   `json:"bindings,omitempty"`
	Waits              []continuity.Wait            `json:"waits"`
	Events             []continuity.ThreadEvent     `json:"events"`
	CoreID             string                       `json:"coreId"`
	OwnerID            string                       `json:"ownerId"`
	ProviderEpoch      int64                        `json:"providerEpoch"`
	ModeRevision       int64                        `json:"modeRevision"`
	PermissionRevision int64                        `json:"permissionRevision"`
	RoleRevision       int64                        `json:"roleRevision"`
	Lease              *ContinuityLease             `json:"lease,omitempty"`
}

type ContinuityLease struct {
	ID        string    `json:"id"`
	RequestID string    `json:"requestId"`
	WaitID    string    `json:"waitId"`
	State     string    `json:"state"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type ContinuityMutation struct {
	rawRead              bool
	ExpectedScope        *coordination.ExecutionScope `json:"expectedExecutionScope,omitempty"`
	ExpectedCoreID       string                       `json:"expectedCoreId,omitempty"`
	ExpectedOwnerID      string                       `json:"expectedOwnerId,omitempty"`
	ExpectedModeRevision int64                        `json:"expectedModeRevision,omitempty"`
	ID                   string                       `json:"id,omitempty"`
	ExpectedRevision     int64                        `json:"expectedRevision"`
	Action               string                       `json:"action"`
	Title                string                       `json:"title,omitempty"`
	Goal                 string                       `json:"goal,omitempty"`
	NextAction           string                       `json:"nextAction,omitempty"`
	UpdateGoal           bool                         `json:"updateGoal,omitempty"`
	Resume               bool                         `json:"resume,omitempty"`
	Status               continuity.ThreadStatus      `json:"status,omitempty"`
	Wait                 *continuity.Wait             `json:"wait,omitempty"`
	WaitID               string                       `json:"waitId,omitempty"`
	LeaseID              string                       `json:"leaseId,omitempty"`
	Result               string                       `json:"result,omitempty"`
	Outcome              string                       `json:"outcome,omitempty"`
	SignalHash           string                       `json:"signalHash,omitempty"`
	UpdateNextAction     bool                         `json:"updateNextAction,omitempty"`
	Summary              string                       `json:"summary,omitempty"`
	CurrentState         string                       `json:"currentState,omitempty"`
	UpdateSummary        bool                         `json:"updateSummary,omitempty"`
	UpdateCurrentState   bool                         `json:"updateCurrentState,omitempty"`
	Binding              *continuity.ThreadBinding    `json:"binding,omitempty"`
	Observation          *continuity.ObservePayload   `json:"observation,omitempty"`
	ThreadPatch          *continuity.ThreadPatch      `json:"threadPatch,omitempty"`
	RuntimeEvent         *ContinuityRuntimeEvent      `json:"runtimeEvent,omitempty"`
}

type ContinuityRuntimeEvent struct {
	Bindings       []continuity.BindingLookup `json:"-"`
	EventType      string                     `json:"eventType"`
	SourceType     string                     `json:"sourceType,omitempty"`
	SourceID       string                     `json:"sourceId,omitempty"`
	ConversationID string                     `json:"conversationId,omitempty"`
	ExecutionID    string                     `json:"executionId,omitempty"`
	PayloadJSON    string                     `json:"payloadJson,omitempty"`
	ThreadStatus   continuity.ThreadStatus    `json:"threadStatus,omitempty"`
	CurrentState   string                     `json:"currentState,omitempty"`
	NextAction     string                     `json:"nextAction,omitempty"`
	Wait           *continuity.Wait           `json:"wait,omitempty"`
	Signal         *continuity.Signal         `json:"signal,omitempty"`
}

func (e *Engine) continuityAuthority(ctx context.Context, request Request) (context.Context, coordination.ExecutionScope, func(), error) {
	if request.RequestID == "" || len(request.RequestID) > 128 {
		return ctx, coordination.ExecutionScope{}, func() {}, errors.New("持续事项请求编号无效")
	}
	ctx, scope, finish, err := e.coordination.Begin(ctx, request.SpaceID, request.DeviceID, request.TargetDeviceID, request.CoreID, request.RoleID, request.RequestID)
	if err != nil {
		return ctx, scope, func() {}, err
	}
	if err = e.coordination.RequireCapability(ctx, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, "ai.chat"); err != nil {
		finish()
		return ctx, scope, func() {}, err
	}
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		finish()
		return ctx, scope, func() {}, err
	}
	requested := request.RoleID
	if requested == "" {
		policy, policyErr := e.coordination.Get(ctx, scope.SpaceID, scope.TargetDeviceID)
		if policyErr != nil {
			finish()
			return ctx, scope, func() {}, policyErr
		}
		requested = policy.SelectedRole
	}
	role, err := coordination.ResolveRole(requested, roles)
	if err != nil {
		finish()
		return ctx, scope, func() {}, err
	}
	scope.RoleID, scope.RoleRevision = role.ID, role.Revision
	view := request.MemoryView
	if request.ExpectedScope != nil {
		if request.ExpectedScope.MemoryViewAll {
			view = "all"
		} else if request.ExpectedScope.MemoryRoleID == coordination.GlobalMemoryRole {
			view = "global"
		} else if request.ExpectedScope.MemoryRoleID != "" {
			view = "role:" + request.ExpectedScope.MemoryRoleID
		}
	}
	if err := coordination.ResolveMemoryScope(&scope, role, view); err != nil {
		finish()
		return ctx, scope, func() {}, err
	}
	if request.ExpectedScope != nil {
		scope.ContinuityRoleID = request.ExpectedScope.ContinuityRoleID
		if len(scope.ContinuityRoleID) > 512 {
			finish()
			return ctx, scope, func() {}, coordination.ErrWrongOwner
		}
	}
	return coordination.WithScope(ctx, scope), scope, finish, nil
}

func (e *Engine) Continuity(ctx context.Context, request Request, mutation ContinuityMutation) (OwnedContinuity, coordination.Acknowledgement, error) {
	if mutation.ExpectedScope != nil {
		request.ExpectedScope = mutation.ExpectedScope
	}
	ctx, scope, finish, err := e.continuityAuthority(ctx, request)
	if err != nil {
		return OwnedContinuity{}, coordination.Acknowledgement{}, err
	}
	defer finish()
	if mutation.ExpectedScope != nil && summaryAuthority(*mutation.ExpectedScope) != summaryAuthority(scope) {
		return OwnedContinuity{}, coordination.Acknowledgement{}, coordination.ErrScopeExpired
	}
	if mutation.ExpectedCoreID != "" && mutation.ExpectedCoreID != scope.CoreID || mutation.ExpectedOwnerID != "" && mutation.ExpectedOwnerID != scope.ResourceOwnerID || mutation.ExpectedModeRevision > 0 && mutation.ExpectedModeRevision != scope.ModeRevision {
		return OwnedContinuity{}, coordination.Acknowledgement{}, coordination.ErrScopeExpired
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return OwnedContinuity{}, coordination.Acknowledgement{}, errors.New("数据来源不支持持续事项")
	}
	if mutation.ID == "" && mutation.Action == "create" {
		mutation.ID = "continuity/" + request.RequestID
	}
	if mutation.ID == "" || len(mutation.ID) > 512 || len(body(mutation)) > 256<<10 {
		return OwnedContinuity{}, coordination.Acknowledgement{}, errors.New("持续事项参数无效")
	}
	unlock := e.lock(scope.ResourceOwnerID + "\x00continuity\x00" + mutation.ID)
	defer unlock()
	digest := sha256.Sum256(body(struct {
		Scope    coordination.ExecutionScope
		Mutation ContinuityMutation
	}{scope, mutation}))
	fingerprint := hex.EncodeToString(digest[:])
	receiptID := "continuity-operation/" + request.RequestID
	if mutation.Action != "read" {
		if pending, proof, err := e.pendingMutation(ctx, scope, receiptID, fingerprint); err != nil {
			return OwnedContinuity{}, coordination.Acknowledgement{}, err
		} else if pending != nil {
			var saved struct {
				Document OwnedContinuity `json:"document"`
			}
			if json.Unmarshal(proof, &saved) != nil {
				return OwnedContinuity{}, coordination.Acknowledgement{}, coordination.ErrRequestConflict
			}
			ack, err := e.commit(ctx, *pending)
			return saved.Document, ack, err
		}
		receipt, receiptErr := port.Resource(ctx, scope, "checkpoint", receiptID)
		if receiptErr != nil {
			return OwnedContinuity{}, coordination.Acknowledgement{}, receiptErr
		}
		if receipt != nil {
			var saved struct {
				Hash     string                       `json:"hash"`
				Document OwnedContinuity              `json:"document"`
				Ack      coordination.Acknowledgement `json:"ack"`
			}
			if receipt.Deleted || json.Unmarshal(receipt.Body, &saved) != nil || saved.Hash != fingerprint {
				return OwnedContinuity{}, coordination.Acknowledgement{}, coordination.ErrRequestConflict
			}
			if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
				return OwnedContinuity{}, coordination.Acknowledgement{}, err
			}
			return saved.Document, saved.Ack, e.coordination.Validate(ctx, scope)
		}
	}
	resource, err := port.Resource(ctx, scope, "continuity", mutation.ID)
	if err != nil {
		return OwnedContinuity{}, coordination.Acknowledgement{}, err
	}
	var document OwnedContinuity
	if resource != nil {
		if resource.Deleted || json.Unmarshal(resource.Body, &document) != nil || document.OwnerID != scope.ResourceOwnerID {
			return document, coordination.Acknowledgement{}, coordination.ErrWrongOwner
		}
	}
	if mutation.Action == "read" {
		if resource == nil {
			return document, coordination.Acknowledgement{}, errors.New("持续事项不存在")
		}
		if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
			return document, coordination.Acknowledgement{}, err
		}
		if !mutation.rawRead {
			document = e.PresentManagedContinuity(ctx, document, scope, resource.RoleID)
		}
		return document, coordination.Acknowledgement{}, e.coordination.Validate(ctx, scope)
	}
	if resource != nil && document.Thread.CharacterID != scope.RoleID {
		return document, coordination.Acknowledgement{}, errors.New("此事项由其他角色执行，请使用原执行角色管理")
	}
	if resource == nil && mutation.Action != "create" || resource != nil && (mutation.Action == "create" || resource.Revision != mutation.ExpectedRevision) {
		return document, coordination.Acknowledgement{}, coordination.ErrResourceVersion
	}
	now := time.Now().UTC()
	if resource != nil && (document.Scope.TargetDeviceID != scope.TargetDeviceID || document.Scope.InitiatorDeviceID != scope.InitiatorDeviceID) {
		return document, coordination.Acknowledgement{}, coordination.ErrWrongOwner
	}
	bound := document.CoreID == scope.CoreID && document.ProviderEpoch == scope.ProviderEpoch && document.ModeRevision == scope.ModeRevision && document.PermissionRevision == scope.PermissionRevision && document.RoleRevision == scope.RoleRevision
	bound = bound && document.Scope.TargetPermissionRevision == scope.TargetPermissionRevision && document.Scope.TargetProviderEpoch == scope.TargetProviderEpoch
	if resource != nil && !bound && mutation.Action != "resume" && mutation.Action != "pause" && mutation.Action != "cancel" && mutation.Action != "confirm_execution" {
		return document, coordination.Acknowledgement{}, coordination.ErrScopeExpired
	}
	if document.Lease != nil && document.Lease.State == "running" && mutation.Action != "heartbeat" && mutation.Action != "finish" && mutation.Action != "pause" && mutation.Action != "cancel" && mutation.Action != "mark_unknown" && mutation.Action != "confirm_execution" {
		return document, coordination.Acknowledgement{}, ErrUncertainExecution
	}
	switch mutation.Action {
	case "create":
		if strings.TrimSpace(mutation.Title) == "" || len(mutation.Title) > 4096 || len(mutation.Goal) > 128<<10 {
			return document, coordination.Acknowledgement{}, errors.New("事项标题或目标无效")
		}
		if len(mutation.NextAction) > 8192 {
			return document, coordination.Acknowledgement{}, errors.New("下一步内容超过上限")
		}
		document = OwnedContinuity{Thread: continuity.Thread{ID: mutation.ID, SpaceID: scope.SpaceID, CharacterID: scope.RoleID, Title: mutation.Title, Goal: mutation.Goal, NextAction: mutation.NextAction, Status: continuity.ThreadStatusActive, Confidence: 1, CreatedAt: now, LastActiveAt: now}, Bindings: []continuity.ThreadBinding{}, Waits: []continuity.Wait{}, Events: []continuity.ThreadEvent{}}
	case "update":
		if document.Thread.Status.IsTerminal() {
			return document, coordination.Acknowledgement{}, errors.New("已结束的事项不能修改")
		}
		if mutation.Title != "" {
			if len(mutation.Title) > 4096 {
				return document, coordination.Acknowledgement{}, errors.New("事项标题超过上限")
			}
			document.Thread.Title = mutation.Title
		}
		if len(mutation.Goal) > 128<<10 {
			return document, coordination.Acknowledgement{}, errors.New("事项目标超过上限")
		}
		if mutation.UpdateGoal {
			document.Thread.Goal = mutation.Goal
		}
		if mutation.UpdateNextAction {
			if len(mutation.NextAction) > 8192 {
				return document, coordination.Acknowledgement{}, errors.New("下一步内容超过上限")
			}
			document.Thread.NextAction = mutation.NextAction
		}
		if mutation.UpdateSummary {
			if len(mutation.Summary) > 128<<10 {
				return document, coordination.Acknowledgement{}, errors.New("事项摘要超过上限")
			}
			document.Thread.Summary = mutation.Summary
		}
		if mutation.UpdateCurrentState {
			if len(mutation.CurrentState) > 128<<10 {
				return document, coordination.Acknowledgement{}, errors.New("当前状态超过上限")
			}
			document.Thread.CurrentState = mutation.CurrentState
		}
	case "bind":
		if document.Thread.Status.IsTerminal() || mutation.Binding == nil || mutation.Binding.BindingType == "" || mutation.Binding.BindingID == "" {
			return document, coordination.Acknowledgement{}, errors.New("持续事项绑定参数无效")
		}
		binding := *mutation.Binding
		binding.ThreadID = document.Thread.ID
		document.Bindings = continuity.MergeThreadBinding(document.Bindings, binding)
	case "observe":
		if mutation.Observation == nil || document.Thread.Status.IsTerminal() {
			return document, coordination.Acknowledgement{}, errors.New("持续事项观察参数无效或事项已结束")
		}
		if err := applyContinuityObservation(&document, *mutation.Observation, mutation.ThreadPatch, request.RequestID, now); err != nil {
			return document, coordination.Acknowledgement{}, err
		}
	case "runtime_event":
		if mutation.RuntimeEvent == nil || document.Thread.Status.IsTerminal() {
			return document, coordination.Acknowledgement{}, errors.New("持续事项运行时事件参数无效或事项已结束")
		}
		applyContinuityRuntimeEvent(&document, *mutation.RuntimeEvent, request.RequestID, now)
	case "confirm_execution":
		if document.Lease == nil || document.Lease.ID != mutation.LeaseID || !(document.Lease.State == "unknown" || document.Lease.State == "running" && !document.Lease.ExpiresAt.After(now)) {
			return document, coordination.Acknowledgement{}, ErrUncertainExecution
		}
		if (mutation.Outcome != "completed" && mutation.Outcome != "abandoned") || len(mutation.Result) > 128<<10 {
			return document, coordination.Acknowledgement{}, errors.New("必须明确确认执行已完成或放弃本次执行")
		}
		document.Lease.State = mutation.Outcome
		if mutation.Outcome == "completed" {
			document.Thread.Summary = mutation.Result
		}
		if !document.Thread.Status.IsTerminal() {
			document.Thread.Status = continuity.ThreadStatusPaused
		}
		for index := range document.Waits {
			wait := &document.Waits[index]
			if wait.ID != document.Lease.WaitID {
				continue
			}
			wait.Status, wait.WakeState = continuity.WaitStatusResolved, continuity.WakeStateDelivered
			if mutation.Outcome == "abandoned" {
				wait.Status = continuity.WaitStatusCancelled
			}
			wait.WakeRequestID = document.Lease.RequestID
			wait.ResolvedAt, wait.WakeDeliveredAt = &now, &now
			wait.ResolvedBy = scope.InitiatorDeviceID
			wait.AutoResume = false
			wait.UpdatedAt = now
		}
	case "pause":
		document.Thread.Status = continuity.ThreadStatusPaused
		if document.Lease != nil && document.Lease.State == "running" {
			e.Interrupt(scope.SpaceID, document.Scope.InitiatorDeviceID, document.Lease.RequestID)
			document.Lease.State = "unknown"
		}
	case "cancel":
		document.Thread.Status = continuity.ThreadStatusCancelled
		document.Thread.CompletedAt = &now
		if document.Lease != nil && document.Lease.State == "running" {
			e.Interrupt(scope.SpaceID, document.Scope.InitiatorDeviceID, document.Lease.RequestID)
			document.Lease.State = "unknown"
		}
	case "mark_unknown":
		if document.Lease == nil || document.Lease.State != "running" || document.Lease.ExpiresAt.After(now) {
			return document, coordination.Acknowledgement{}, coordination.ErrResourceVersion
		}
		document.Lease.State = "unknown"
		document.Thread.Status = continuity.ThreadStatusPaused
	case "resume":
		if document.Thread.Status.IsTerminal() || document.Lease != nil && document.Lease.State == "unknown" {
			return document, coordination.Acknowledgement{}, ErrUncertainExecution
		}
		document.Thread.Status = continuity.ThreadStatusActive
	case "complete":
		if document.Lease != nil && document.Lease.State == "unknown" {
			return document, coordination.Acknowledgement{}, ErrUncertainExecution
		}
		document.Thread.Status = continuity.ThreadStatusCompleted
		document.Thread.CompletedAt = &now
	case "add_wait":
		if document.Thread.Status.IsTerminal() || mutation.Wait == nil || !continuity.ValidWaitType(mutation.Wait.WaitType) || len(document.Waits) >= 256 {
			return document, coordination.Acknowledgement{}, errors.New("等待条件无效或超过上限")
		}
		wait := *mutation.Wait
		if len(wait.Description) > 8192 || len(wait.ResumeHint) > 8192 || len(wait.ConditionJSON) > 64<<10 {
			return document, coordination.Acknowledgement{}, errors.New("等待条件超过上限")
		}
		if wait.WaitType == continuity.WaitTypeTime && wait.DueAt == nil {
			return document, coordination.Acknowledgement{}, errors.New("定时等待必须指定时间")
		}
		if wait.ConditionJSON == "" {
			wait.ConditionJSON = "{}"
		}
		if !json.Valid([]byte(wait.ConditionJSON)) {
			return document, coordination.Acknowledgement{}, errors.New("等待条件格式无效")
		}
		wait.ID = "wait/" + request.RequestID
		wait.ThreadID = document.Thread.ID
		wait.Status = continuity.WaitStatusWaiting
		wait.WakeState = ""
		wait.WakeAttempts = 0
		wait.WakeRequestID = ""
		wait.ResolvedAt = nil
		wait.ResolvedBy = ""
		wait.ResolutionJSON = "{}"
		wait.SourceExecutionID = ""
		wait.NextWakeAt = nil
		wait.WakeDeliveredAt = nil
		wait.LastWakeError = ""
		wait.CreatedAt = now
		wait.UpdatedAt = now
		document.Waits = append(document.Waits, wait)
		document.Thread.Status = continuity.ThreadStatusWaiting
	case "resolve_wait", "cancel_wait", "claim":
		found := false
		for index := range document.Waits {
			wait := &document.Waits[index]
			if wait.ID != mutation.WaitID {
				continue
			}
			found = true
			if mutation.Action == "claim" {
				timeReady := wait.Status == continuity.WaitStatusWaiting && wait.WaitType == continuity.WaitTypeTime && wait.DueAt != nil && !wait.DueAt.After(now)
				resolvedReady := wait.Status == continuity.WaitStatusResolved && wait.WakeState == continuity.WakeStatePending
				if document.Thread.Status.IsTerminal() || document.Thread.Status == continuity.ThreadStatusPaused || !wait.AutoResume || (!timeReady && !resolvedReady) {
					return document, coordination.Acknowledgement{}, errors.New("该事项当前不允许自动执行")
				}
				if document.Lease != nil && document.Lease.State == "unknown" {
					return document, coordination.Acknowledgement{}, ErrUncertainExecution
				}
				document.Lease = &ContinuityLease{ID: uuid.NewString(), RequestID: "continuity/" + uuid.NewString(), WaitID: wait.ID, State: "running", ExpiresAt: now.Add(90 * time.Second)}
				document.Thread.Status = continuity.ThreadStatusActive
			} else {
				if wait.Status != continuity.WaitStatusWaiting {
					return document, coordination.Acknowledgement{}, coordination.ErrResourceVersion
				}
				wait.Status = continuity.WaitStatusResolved
				if mutation.Action == "cancel_wait" {
					wait.Status = continuity.WaitStatusCancelled
				}
				wait.ResolvedAt = &now
				wait.ResolvedBy = scope.InitiatorDeviceID
				wait.UpdatedAt = now
				if mutation.Action == "resolve_wait" && mutation.Resume {
					wait.AutoResume = true
					wait.WakeState = continuity.WakeStatePending
				}
			}
			break
		}
		if !found {
			return document, coordination.Acknowledgement{}, errors.New("等待条件不存在")
		}
	case "heartbeat", "finish":
		if document.Lease == nil || document.Lease.ID != mutation.LeaseID || document.Lease.State != "running" || !document.Lease.ExpiresAt.After(now) {
			return document, coordination.Acknowledgement{}, ErrUncertainExecution
		}
		if mutation.Action == "heartbeat" {
			document.Lease.ExpiresAt = now.Add(90 * time.Second)
		} else {
			if len(mutation.Result) > 128<<10 {
				return document, coordination.Acknowledgement{}, errors.New("执行结果超过上限")
			}
			document.Lease.State = "completed"
			document.Thread.Summary = mutation.Result
			document.Thread.Status = continuity.ThreadStatusWaiting
			for index := range document.Waits {
				wait := &document.Waits[index]
				if wait.ID == document.Lease.WaitID {
					wait.Status = continuity.WaitStatusResolved
					wait.WakeState = continuity.WakeStateDelivered
					wait.WakeRequestID = document.Lease.RequestID
					wait.ResolvedAt = &now
					wait.WakeDeliveredAt = &now
					wait.UpdatedAt = now
				}
			}
		}
	default:
		return document, coordination.Acknowledgement{}, errors.New("不支持的持续事项操作")
	}
	document.Scope = scope
	document.MemoryPartition = coordination.StorageRole(scope, "continuity")
	document.PausedReason = ""
	document.CoreID = scope.CoreID
	document.OwnerID = scope.ResourceOwnerID
	document.ProviderEpoch = scope.ProviderEpoch
	document.ModeRevision = scope.ModeRevision
	document.PermissionRevision = scope.PermissionRevision
	document.RoleRevision = scope.RoleRevision
	document.Thread.Revision = mutation.ExpectedRevision + 1
	document.Thread.UpdatedAt = now
	document.Thread.LastActiveAt = now
	if mutation.Action != "observe" && mutation.Action != "runtime_event" {
		document.Events = append(document.Events, continuity.ThreadEvent{ID: "event/" + request.RequestID, ThreadID: document.Thread.ID, EventType: mutation.Action, RequestID: request.RequestID, PayloadJSON: "{}", IdempotencyKey: "event|" + request.RequestID, OccurredAt: now})
	}
	if len(document.Events) > 256 {
		document.Events = document.Events[len(document.Events)-256:]
	}
	proofAck := coordination.Acknowledgement{RequestID: scope.RequestID, OwnerID: scope.ResourceOwnerID, Versions: map[string]int64{"continuity/" + document.Thread.ID: document.Thread.Revision, "checkpoint/" + receiptID: 1}}
	proof := body(map[string]any{"hash": fingerprint, "document": document, "ack": proofAck})
	if err := e.coordination.TrackContinuity(ctx, scope, document.Thread.ID, document.Thread.Revision); err != nil {
		return document, coordination.Acknowledgement{}, err
	}
	ack, err := e.commit(ctx, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "continuity", ID: document.Thread.ID, RoleID: coordination.StorageRole(scope, "continuity"), ExpectedRevision: mutation.ExpectedRevision, Body: body(document)}, {Kind: "checkpoint", ID: receiptID, RoleID: scope.RoleID, Body: proof}}})
	return document, ack, err
}

func applyContinuityObservation(document *OwnedContinuity, observation continuity.ObservePayload, patch *continuity.ThreadPatch, requestID string, now time.Time) error {
	if document == nil {
		return errors.New("持续事项不存在")
	}
	if observation.ThreadID != "" && observation.ThreadID != document.Thread.ID {
		return coordination.ErrWrongOwner
	}
	observation.ThreadID = document.Thread.ID
	if patch == nil {
		heuristic := continuity.BuildHeuristicThreadPatch(&document.Thread, observation)
		patch = &heuristic
	}
	if len(observation.UserMessage) > 128<<10 || len(observation.AssistantReply) > 128<<10 {
		return errors.New("持续事项观察内容超过上限")
	}
	if userMessage := strings.TrimSpace(observation.UserMessage); userMessage != "" {
		for index := range document.Waits {
			wait := &document.Waits[index]
			if wait.WaitType != continuity.WaitTypeUser || wait.Status != continuity.WaitStatusWaiting {
				continue
			}
			resolved := now
			wait.Status = continuity.WaitStatusResolved
			wait.ResolvedAt = &resolved
			wait.ResolvedBy = "user_message"
			wait.UpdatedAt = now
		}
	}
	continuity.ApplyThreadPatch(&document.Thread, *patch, now)
	for _, waitPatch := range patch.Waits {
		switch waitPatch.Action {
		case "create":
			wait, ok := continuity.WaitFromPatch(document.Thread.ID, waitPatch, observation.ExecutionID, now)
			if !ok {
				continue
			}
			duplicate := false
			for _, existing := range document.Waits {
				if existing.Status != continuity.WaitStatusWaiting || existing.WaitType != wait.WaitType || existing.Description != wait.Description || existing.ConditionJSON != wait.ConditionJSON {
					continue
				}
				duplicate = true
				break
			}
			if !duplicate && len(document.Waits) < 256 {
				document.Waits = append(document.Waits, wait)
			}
		case "resolve", "cancel":
			for index := range document.Waits {
				wait := &document.Waits[index]
				if wait.Status != continuity.WaitStatusWaiting {
					continue
				}
				if waitPatch.WaitID != "" {
					if wait.ID != waitPatch.WaitID {
						continue
					}
				} else if waitPatch.Type == "" || wait.WaitType != waitPatch.Type {
					continue
				}
				resolved := now
				wait.Status = continuity.WaitStatusResolved
				if waitPatch.Action == "cancel" {
					wait.Status = continuity.WaitStatusCancelled
				}
				wait.ResolvedAt = &resolved
				wait.ResolvedBy = "continuity_intelligence"
				wait.UpdatedAt = now
				if waitPatch.Action == "resolve" && wait.AutoResume {
					wait.WakeState = continuity.WakeStatePending
					wait.WakeRequestID = "continuity-wake/" + requestID
					wait.NextWakeAt = &resolved
				}
				break
			}
		}
	}
	if strings.TrimSpace(observation.ConversationID) != "" {
		document.Bindings = continuity.MergeThreadBinding(document.Bindings, continuity.NewThreadBinding(document.Thread.ID, "conversation", observation.ConversationID, "context", "observer", 1))
	}
	open := false
	for _, wait := range document.Waits {
		if wait.Status == continuity.WaitStatusWaiting {
			open = true
			break
		}
	}
	if open && document.Thread.Status == continuity.ThreadStatusActive {
		document.Thread.Status = continuity.ThreadStatusWaiting
	}
	if !open && document.Thread.Status == continuity.ThreadStatusWaiting {
		document.Thread.Status = continuity.ThreadStatusActive
	}
	if patch.EventSummary != "" {
		payload, _ := json.Marshal(map[string]any{"summary": patch.EventSummary, "confidence": patch.Confidence})
		eventType := patch.EventType
		if eventType == "" {
			eventType = "turn.heuristic"
		}
		document.Events = append(document.Events, continuity.ThreadEvent{ID: "event/" + requestID + "/observed", ThreadID: document.Thread.ID, EventType: eventType, SourceType: observation.Source, ConversationID: observation.ConversationID, RequestID: requestID, ExecutionID: observation.ExecutionID, PayloadJSON: string(payload), IdempotencyKey: requestID + "|continuity.observe", OccurredAt: now})
	}
	if len(document.Events) > 256 {
		document.Events = document.Events[len(document.Events)-256:]
	}
	return nil
}

func applyContinuityRuntimeEvent(document *OwnedContinuity, event ContinuityRuntimeEvent, requestID string, now time.Time) {
	if document == nil {
		return
	}
	if event.Wait != nil && continuity.ValidWaitType(event.Wait.WaitType) && len(document.Waits) < 256 {
		wait := *event.Wait
		wait.ThreadID = document.Thread.ID
		wait.Status = continuity.WaitStatusWaiting
		wait.CreatedAt = now
		wait.UpdatedAt = now
		if wait.ID == "" {
			wait.ID = "wait/runtime/" + requestID
		}
		duplicate := false
		for _, existing := range document.Waits {
			if existing.Status == continuity.WaitStatusWaiting && existing.WaitType == wait.WaitType && existing.ConditionJSON == wait.ConditionJSON {
				duplicate = true
				break
			}
		}
		if !duplicate {
			document.Waits = append(document.Waits, wait)
		}
	}
	if event.Signal != nil && continuity.ValidWaitType(event.Signal.WaitType) {
		signal := *event.Signal
		if signal.OccurredAt.IsZero() {
			signal.OccurredAt = now
		}
		for index := range document.Waits {
			wait := &document.Waits[index]
			if wait.Status != continuity.WaitStatusWaiting || !continuity.WaitMatchesSignal(*wait, signal) {
				continue
			}
			resolved := now
			wait.Status = continuity.WaitStatusResolved
			wait.ResolvedAt = &resolved
			wait.ResolvedBy = strings.TrimSpace(signal.Source)
			if wait.ResolvedBy == "" {
				wait.ResolvedBy = signal.WaitType
			}
			wait.ResolutionJSON = event.PayloadJSON
			wait.UpdatedAt = now
			if wait.AutoResume {
				wait.WakeState = continuity.WakeStatePending
				wait.WakeRequestID = "continuity-wake/" + requestID + "/" + wait.ID
				wait.NextWakeAt = &resolved
			}
		}
	}
	if event.ThreadStatus != "" && continuity.ValidThreadStatus(event.ThreadStatus) {
		document.Thread.Status = event.ThreadStatus
		if event.ThreadStatus.IsTerminal() {
			completed := now
			document.Thread.CompletedAt = &completed
		}
	}
	if event.CurrentState != "" {
		document.Thread.CurrentState = event.CurrentState
	}
	if event.NextAction != "" || event.ThreadStatus.IsTerminal() {
		document.Thread.NextAction = event.NextAction
	}
	payload := event.PayloadJSON
	if payload == "" {
		payload = "{}"
	}
	document.Events = append(document.Events, continuity.ThreadEvent{ID: "event/" + requestID + "/runtime", ThreadID: document.Thread.ID, EventType: event.EventType, SourceType: event.SourceType, SourceID: event.SourceID, ConversationID: event.ConversationID, RequestID: requestID, ExecutionID: event.ExecutionID, PayloadJSON: payload, IdempotencyKey: "runtime|" + requestID, OccurredAt: now})
	if len(document.Events) > 256 {
		document.Events = document.Events[len(document.Events)-256:]
	}
	open := false
	for _, wait := range document.Waits {
		if wait.Status == continuity.WaitStatusWaiting {
			open = true
			break
		}
	}
	if open && document.Thread.Status == continuity.ThreadStatusActive {
		document.Thread.Status = continuity.ThreadStatusWaiting
	}
	if !open && document.Thread.Status == continuity.ThreadStatusWaiting {
		document.Thread.Status = continuity.ThreadStatusActive
	}
}

func PresentContinuity(document OwnedContinuity, scope coordination.ExecutionScope, now time.Time) OwnedContinuity {
	current := scope
	if scope.MemoryViewAll {
		if document.MemoryPartition == "" {
			document.MemoryPartition = document.Thread.CharacterID
		}
		current.MemoryViewAll = false
		current.MemoryRoleID = document.MemoryPartition
	} else {
		document.MemoryPartition = coordination.StorageRole(scope, "continuity")
	}
	document.ManagementScope = &current
	if document.Thread.CharacterID != scope.RoleID {
		document.ReadOnly = true
		document.PausedReason = "此事项由其他角色执行，迁移分区不会更换执行角色。"
		return document
	}
	if document.Thread.Status.IsTerminal() {
		return document
	}
	if document.Scope.TargetDeviceID != scope.TargetDeviceID || document.Scope.InitiatorDeviceID != scope.InitiatorDeviceID {
		document.Thread.Status = continuity.ThreadStatusPaused
		document.PausedReason = "该事项绑定其他调用设备，请通过原设备管理执行。"
		return document
	}
	if document.Lease != nil && (document.Lease.State == "unknown" || document.Lease.State == "running" && !document.Lease.ExpiresAt.After(now)) {
		lease := *document.Lease
		lease.State = "unknown"
		document.Lease = &lease
		document.Thread.Status = continuity.ThreadStatusPaused
		document.PausedReason = "上次执行的结果尚未确认，事项已暂停，系统不会自动重新执行。"
		return document
	}
	if document.CoreID != scope.CoreID || document.ProviderEpoch != scope.ProviderEpoch || document.ModeRevision != scope.ModeRevision || document.PermissionRevision != scope.PermissionRevision || document.RoleRevision != scope.RoleRevision || document.Scope.TargetPermissionRevision != scope.TargetPermissionRevision || document.Scope.TargetProviderEpoch != scope.TargetProviderEpoch {
		document.Thread.Status = continuity.ThreadStatusPaused
		document.PausedReason = "服务提供者、统筹模式、权限或角色已变化，事项已暂停，请确认后恢复。"
	}
	return document
}

func (e *Engine) PresentManagedContinuity(ctx context.Context, document OwnedContinuity, scope coordination.ExecutionScope, partition string) OwnedContinuity {
	scope.ContinuityRoleID = partition
	scope.MemoryViewAll = false
	document.MemoryPartition = partition
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		document.ReadOnly = true
		document.PausedReason = "无法核验原执行角色，暂时只读"
		return PresentContinuity(document, scope, time.Now())
	}
	role, err := coordination.ResolveRole(document.Thread.CharacterID, roles)
	if err != nil {
		document.ReadOnly = true
		document.PausedReason = "原执行角色不可用，事项只读"
		return PresentContinuity(document, scope, time.Now())
	}
	scope.RoleID, scope.RoleRevision = role.ID, role.Revision
	_ = coordination.ResolveMemoryScope(&scope, role, "")
	scope.ContinuityRoleID = partition
	document.MemoryPartition = partition
	document.ReadOnly = false
	return PresentContinuity(document, scope, time.Now())
}
