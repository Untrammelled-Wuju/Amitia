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
	ManagementScope    *coordination.ExecutionScope `json:"managementExecutionScope,omitempty"`
	PausedReason       string                       `json:"pausedReason,omitempty"`
	Scope              coordination.ExecutionScope  `json:"executionScope"`
	Thread             continuity.Thread            `json:"thread"`
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
	return coordination.WithScope(ctx, scope), scope, finish, nil
}

func (e *Engine) Continuity(ctx context.Context, request Request, mutation ContinuityMutation) (OwnedContinuity, coordination.Acknowledgement, error) {
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
		if resource.Deleted || json.Unmarshal(resource.Body, &document) != nil || document.OwnerID != scope.ResourceOwnerID || document.Thread.CharacterID != scope.RoleID {
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
			document = PresentContinuity(document, scope, time.Now())
		}
		return document, coordination.Acknowledgement{}, e.coordination.Validate(ctx, scope)
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
		document = OwnedContinuity{Thread: continuity.Thread{ID: mutation.ID, SpaceID: scope.SpaceID, CharacterID: scope.RoleID, Title: mutation.Title, Goal: mutation.Goal, NextAction: mutation.NextAction, Status: continuity.ThreadStatusActive, Confidence: 1, CreatedAt: now, LastActiveAt: now}, Waits: []continuity.Wait{}, Events: []continuity.ThreadEvent{}}
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
	document.Events = append(document.Events, continuity.ThreadEvent{ID: "event/" + request.RequestID, ThreadID: document.Thread.ID, EventType: mutation.Action, RequestID: request.RequestID, PayloadJSON: "{}", OccurredAt: now})
	if len(document.Events) > 256 {
		document.Events = document.Events[len(document.Events)-256:]
	}
	proofAck := coordination.Acknowledgement{RequestID: scope.RequestID, OwnerID: scope.ResourceOwnerID, Versions: map[string]int64{"continuity/" + document.Thread.ID: document.Thread.Revision, "checkpoint/" + receiptID: 1}}
	proof := body(map[string]any{"hash": fingerprint, "document": document, "ack": proofAck})
	if err := e.coordination.TrackContinuity(ctx, scope, document.Thread.ID, document.Thread.Revision); err != nil {
		return document, coordination.Acknowledgement{}, err
	}
	ack, err := e.commit(ctx, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "continuity", ID: document.Thread.ID, RoleID: scope.RoleID, ExpectedRevision: mutation.ExpectedRevision, Body: body(document)}, {Kind: "checkpoint", ID: receiptID, RoleID: scope.RoleID, Body: proof}}})
	return document, ack, err
}

func PresentContinuity(document OwnedContinuity, scope coordination.ExecutionScope, now time.Time) OwnedContinuity {
	current := scope
	document.ManagementScope = &current
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
