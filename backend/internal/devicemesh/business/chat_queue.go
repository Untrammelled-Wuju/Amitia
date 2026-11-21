package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	amitialog "log"
)

type QueuedChatResponse struct {
	Accepted       bool                        `json:"accepted"`
	Status         string                      `json:"status"`
	RequestID      string                      `json:"requestId"`
	ConversationID string                      `json:"conversationId"`
	Scope          coordination.ExecutionScope `json:"executionScope"`
	Error          string                      `json:"error,omitempty"`
	Message        string                      `json:"message"`
	Position       int64                       `json:"position"`
	Attachments    []QueuedChatAttachment      `json:"attachments,omitempty"`
	Quote          *QueuedChatQuote            `json:"quote,omitempty"`
}

type QueuedChatAttachment struct {
	ArtifactID      string `json:"artifactId,omitempty"`
	OwnerDeviceID   string `json:"ownerDeviceId,omitempty"`
	StorageDeviceID string `json:"storageDeviceId,omitempty"`
	Kind            string `json:"kind"`
	Name            string `json:"name"`
	MIMEType        string `json:"mimeType"`
	SHA256          string `json:"sha256,omitempty"`
	SizeBytes       int64  `json:"sizeBytes,omitempty"`
}

type QueuedChatQuote struct {
	OwnerID          string `json:"ownerId"`
	RoleID           string `json:"characterId"`
	ConversationID   string `json:"conversationId"`
	MessageID        string `json:"messageId"`
	ExpectedRevision int64  `json:"expectedRevision"`
}

type queuedChatDocument struct {
	Actor       *auth.ActorContext          `json:"actor,omitempty"`
	Hash        string                      `json:"hash"`
	Request     Request                     `json:"request"`
	Scope       coordination.ExecutionScope `json:"executionScope"`
	Ingress     *Ingress                    `json:"ingress,omitempty"`
	Status      string                      `json:"status"`
	Error       string                      `json:"error,omitempty"`
	Position    int64                       `json:"position"`
	SteerTurnID string                      `json:"steerTurnId,omitempty"`
}

type queuedChatKey struct {
	Engine               *Engine
	Owner, Role, Request string
}

var activeQueuedChats sync.Map
var queuedChatFailures sync.Map

var errQueuedChatNotNext = errors.New("排队消息尚未轮到执行")
var errQueuedChatRemoved = errors.New("排队消息已被移除")

func (e *Engine) persistentQueuedChat(ctx context.Context, scope coordination.ExecutionScope, requestID string) (queuedConversationDocument, error) {
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return queuedConversationDocument{}, errors.New("Owner 排队存储未接通")
	}
	resource, err := port.Resource(ctx, scope, "checkpoint", queuedChatResourceID(scope, requestID))
	if err != nil {
		return queuedConversationDocument{}, err
	}
	if resource == nil || resource.Deleted {
		return queuedConversationDocument{}, errQueuedChatRemoved
	}
	var document queuedChatDocument
	if err := json.Unmarshal(resource.Body, &document); err != nil {
		return queuedConversationDocument{}, fmt.Errorf("解析排队请求 %s 失败: %w", requestID, err)
	}
	return queuedConversationDocument{resource: *resource, document: document}, nil
}

func (e *Engine) persistQueuedChat(ctx context.Context, item queuedConversationDocument, transition string) (int64, error) {
	scope := item.document.Scope
	scope.RequestID = transition + "/" + item.document.Request.RequestID + "/" + uuid.NewString()
	ack, err := e.commit(ctx, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: item.resource.ID, RoleID: item.resource.RoleID, ExpectedRevision: item.resource.Revision, Body: body(item.document)}}})
	if err != nil {
		return item.resource.Revision, err
	}
	if revision := ack.Versions["checkpoint/"+item.resource.ID]; revision > 0 {
		return revision, nil
	}
	return item.resource.Revision + 1, nil
}

func queuedChatUncertainError(document queuedChatDocument, reason error) error {
	if document.SteerTurnID != "" {
		return fmt.Errorf("排队消息引导状态未确认，目标回合 %s 可能已经收到引导；为避免重复执行，系统不会再次投递: %w", document.SteerTurnID, reason)
	}
	return fmt.Errorf("排队消息已开始执行但结果未确认，系统不会重复执行已发起的动作: %w", reason)
}

func (e *Engine) markQueuedChatUncertain(ctx context.Context, item queuedConversationDocument, reason error) (queuedConversationDocument, error) {
	detail := DescribeRuntimeError(queuedChatUncertainError(item.document, reason))
	encoded, err := json.Marshal(detail)
	if err != nil {
		return item, errors.Join(queuedChatUncertainError(item.document, reason), err)
	}
	item.document.Status = "uncertain"
	item.document.Error = string(encoded)
	revision, err := e.persistQueuedChat(ctx, item, "queue-uncertain")
	if err != nil {
		return item, errors.Join(queuedChatUncertainError(item.document, reason), err)
	}
	item.resource.Revision = revision
	return item, nil
}

func (e *Engine) claimQueuedChat(ctx context.Context, request Request) error {
	if request.ExpectedScope == nil {
		return coordination.ErrWrongOwner
	}
	scope := *request.ExpectedScope
	unlock, err := e.lockContext(ctx, "queue-control\x00"+RuntimeKey(scope, request.ConversationID))
	if err != nil {
		return err
	}
	defer unlock()
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return errors.New("Owner 排队存储未接通")
	}
	row, err := port.Resource(ctx, scope, "checkpoint", queuedChatResourceID(scope, request.RequestID))
	if err != nil {
		return err
	}
	if row == nil || row.Deleted {
		return errQueuedChatRemoved
	}
	var document queuedChatDocument
	if err := json.Unmarshal(row.Body, &document); err != nil {
		return fmt.Errorf("解析排队请求 %s 失败: %w", request.RequestID, err)
	}
	if !sameQueueAuthority(scope, document.Scope) || document.Request.RequestID != request.RequestID || document.Request.ConversationID != request.ConversationID || document.Hash != request.queuedHash {
		return coordination.ErrRequestConflict
	}
	if document.Status == "running" {
		return nil
	}
	if document.Status != "queued" {
		return errQueuedChatRemoved
	}
	items, err := e.queuedConversationDocuments(ctx, request)
	if err != nil {
		return err
	}
	next := ""
	for _, item := range items {
		if item.document.Status == "queued" {
			next = item.document.Request.RequestID
			break
		}
	}
	if next != request.RequestID {
		return errQueuedChatNotNext
	}
	document.Status = "running"
	commitScope := scope
	commitScope.RequestID = "queue-claim/" + request.RequestID
	_, err = e.commit(ctx, coordination.Commit{Scope: commitScope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: row.ID, RoleID: scope.RoleID, ExpectedRevision: row.Revision, Body: body(document)}}})
	return err
}

func queuedChatResourceID(scope coordination.ExecutionScope, requestID string) string {
	hash := sha256.Sum256([]byte(scope.InitiatorDeviceID + "\x00" + requestID))
	return "queued-chat/" + hex.EncodeToString(hash[:])
}

func (e *Engine) QueueChat(ctx context.Context, expected coordination.ExecutionScope, request Request) (QueuedChatResponse, error) {
	actor, err := auth.RequireActor(ctx)
	if err != nil {
		return QueuedChatResponse{}, err
	}
	if actor.SpaceID.String() != expected.SpaceID || actor.DeviceID.String() != expected.InitiatorDeviceID {
		return QueuedChatResponse{}, coordination.ErrWrongOwner
	}
	if request.RequestID == "" {
		request.RequestID = uuid.NewString()
	}
	if len(request.RequestID) > 128 {
		return QueuedChatResponse{}, coordination.ErrPendingLimit
	}
	request.SpaceID, request.DeviceID, request.CoreID, request.TargetDeviceID, request.RoleID = expected.SpaceID, expected.InitiatorDeviceID, expected.CoreID, expected.TargetDeviceID, expected.RoleID
	if err := validateInboundRequest(&request); err != nil {
		return QueuedChatResponse{}, err
	}
	if err := ValidateAttachments(request.Attachments); err != nil {
		return QueuedChatResponse{}, err
	}
	if request.Quote != nil && request.ConversationOrigin != nil && request.ConversationOrigin.ID != request.ConversationID {
		return QueuedChatResponse{}, coordination.ErrWrongOwner
	}
	childScope := expected
	childScope.RequestID = request.RequestID
	childScope.TurnID, childScope.ExecutionID, childScope.ContinuityRoleID = uuid.NewSHA1(uuid.NameSpaceOID, []byte("queued-turn/"+request.RequestID)).String(), uuid.NewSHA1(uuid.NameSpaceOID, []byte("queued-execution/"+request.RequestID)).String(), ""
	request.ExpectedScope = &childScope
	if err := e.validateChatAttachments(ctx, childScope, request.Attachments); err != nil {
		return QueuedChatResponse{}, err
	}
	unlock, err := e.lockContext(ctx, "queue-control\x00"+RuntimeKey(expected, request.ConversationID))
	if err != nil {
		return QueuedChatResponse{}, err
	}
	defer unlock()
	position := time.Now().UnixMicro()
	var queuedItems []queuedConversationDocument
	if request.ConversationID != "" {
		items, err := e.queuedConversationDocuments(ctx, request)
		if err != nil {
			return QueuedChatResponse{}, err
		}
		queuedItems = items
		if len(items) > 0 && items[len(items)-1].document.Position >= position {
			position = items[len(items)-1].document.Position + 1
		}
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return QueuedChatResponse{}, errors.New("Owner 排队存储未接通")
	}
	fingerprint := sha256.Sum256(body(request))
	hash := hex.EncodeToString(fingerprint[:])
	var document queuedChatDocument
	err = e.WithConversationExecutionScope(ctx, expected, request.ConversationID, func(current context.Context) error {
		id := queuedChatResourceID(expected, request.RequestID)
		record, err := port.Resource(current, expected, "checkpoint", id)
		if err != nil {
			return err
		}
		if record != nil && !record.Deleted {
			if err := json.Unmarshal(record.Body, &document); err != nil {
				return err
			}
			if document.Hash != hash {
				return coordination.ErrRequestConflict
			}
			return nil
		}
		if len(queuedItems) >= 128 {
			return coordination.ErrPendingLimit
		}
		if _, _, err := e.reviewQuote(current, expected, request, request.Quote); err != nil {
			return fmt.Errorf("校验排队消息引用失败: %w", err)
		}
		if err := e.coordination.RegisterQueuedRequest(current, childScope, hash, request.ConversationID); err != nil {
			return err
		}
		document = queuedChatDocument{Actor: actor.Clone(), Hash: hash, Request: request, Scope: childScope, Status: "queued", Position: position}
		if ingress, ok := IngressFromContext(ctx); ok {
			document.Ingress = &ingress
		}
		scope := expected
		scope.RequestID = "queue/" + request.RequestID
		_, err = e.commit(current, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: id, RoleID: expected.RoleID, Body: body(document)}}})
		return err
	})
	if err != nil {
		return QueuedChatResponse{}, err
	}
	response := e.queuedChatResponse(document, expected)
	if document.Status == "queued" {
		e.dispatchQueuedChat(actor.Clone(), document)
	}
	return response, nil
}

func (e *Engine) prepareQueuedChatDispatch(ctx context.Context, document *queuedChatDocument) (bool, error) {
	if document == nil || document.Request.ConversationID == "" || document.Request.RequestID == "" {
		return false, coordination.ErrWrongOwner
	}
	unlock, err := e.lockContext(ctx, "queue-control\x00"+RuntimeKey(document.Scope, document.Request.ConversationID))
	if err != nil {
		return false, err
	}
	defer unlock()
	item, err := e.persistentQueuedChat(ctx, document.Scope, document.Request.RequestID)
	if err != nil {
		return false, err
	}
	if item.document.Hash != document.Hash || !sameQueueAuthority(item.document.Scope, document.Scope) {
		return false, coordination.ErrRequestConflict
	}
	*document = item.document
	switch item.document.Status {
	case "queued":
		return false, nil
	case "steering":
		item, err = e.markQueuedChatUncertain(ctx, item, errors.New("进程在引导提交后、运行时确认前中断"))
		if err != nil {
			return false, err
		}
		*document = item.document
		if err := e.coordination.FinishQueuedRequest(ctx, item.document.Scope); err != nil {
			return false, err
		}
		return true, nil
	case "running":
		if item.document.Request.Group != nil {
			round, err := itemResource(e, ctx, item.document.Scope, "group-round", groupRoundID(item.document.Scope.SpaceID, item.document.Scope.InitiatorDeviceID, item.document.Request.RequestID))
			if err != nil {
				return false, err
			}
			if round == nil || round.Deleted {
				if err := e.coordination.ResetUnstartedQueuedRequest(ctx, item.document.Scope, item.document.Hash); err != nil {
					return false, err
				}
			}
			return false, nil
		}
		checkpointResource, err := itemResource(e, ctx, item.document.Scope, "checkpoint", "turn/"+item.document.Request.RequestID)
		if err != nil {
			return false, err
		}
		if checkpointResource == nil || checkpointResource.Deleted {
			if err := e.coordination.ResetUnstartedQueuedRequest(ctx, item.document.Scope, item.document.Hash); err != nil {
				return false, err
			}
			return false, nil
		}
		var saved checkpoint
		if err := json.Unmarshal(checkpointResource.Body, &saved); err != nil {
			reason := fmt.Errorf("解析排队消息执行检查点失败 raw=%s: %w", string(checkpointResource.Body), err)
			item, markErr := e.markQueuedChatUncertain(ctx, item, reason)
			*document = item.document
			if markErr != nil {
				return false, errors.Join(reason, markErr)
			}
			return true, e.coordination.FinishQueuedRequest(ctx, item.document.Scope)
		}
		if saved.Status != "completed" || saved.Response == nil {
			reason := fmt.Errorf("%w: checkpoint status=%s raw=%s", ErrUncertainExecution, saved.Status, string(checkpointResource.Body))
			if saved.Error != nil {
				encoded, marshalErr := json.Marshal(saved.Error)
				if marshalErr != nil {
					reason = errors.Join(reason, marshalErr)
				} else {
					reason = fmt.Errorf("%w: checkpoint error=%s", reason, string(encoded))
				}
			}
			item, markErr := e.markQueuedChatUncertain(ctx, item, reason)
			*document = item.document
			if markErr != nil {
				return false, errors.Join(reason, markErr)
			}
			return true, e.coordination.FinishQueuedRequest(ctx, item.document.Scope)
		}
		if err := e.coordination.CompleteRequest(ctx, item.document.Scope); err != nil {
			item, markErr := e.markQueuedChatUncertain(ctx, item, err)
			*document = item.document
			if markErr != nil {
				return false, errors.Join(err, markErr)
			}
			return true, e.coordination.FinishQueuedRequest(ctx, item.document.Scope)
		}
		item.document.Status = "completed"
		item.document.Error = ""
		revision, err := e.persistQueuedChat(ctx, item, "queue-recover-completed")
		if err != nil {
			return false, err
		}
		item.resource.Revision = revision
		*document = item.document
		return true, e.coordination.FinishQueuedRequest(ctx, item.document.Scope)
	default:
		*document = item.document
		if err := e.coordination.FinishQueuedRequest(ctx, item.document.Scope); err != nil {
			return false, err
		}
		return true, nil
	}
}

func itemResource(e *Engine, ctx context.Context, scope coordination.ExecutionScope, kind, id string) (*coordination.Resource, error) {
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return nil, errors.New("Owner 排队存储未接通")
	}
	return port.Resource(ctx, scope, kind, id)
}

func (e *Engine) dispatchQueuedChat(actor *auth.ActorContext, document queuedChatDocument) {
	key := queuedChatKey{Engine: e, Owner: document.Scope.ResourceOwnerID, Role: document.Scope.RoleID, Request: document.Request.RequestID}
	if _, loaded := activeQueuedChats.LoadOrStore(key, true); loaded {
		return
	}
	go func() {
		recordFailure := func(stage string, err error) {
			detail := fmt.Sprintf("%s: %+v\n%s", stage, err, errorCauses(err))
			queuedChatFailures.Store(key, detail)
			amitialog.Printf("[queue-recovery] request=%s conversation=%s stage=%s error=%s", document.Request.RequestID, document.Request.ConversationID, stage, detail)
		}
		amitialog.Printf("[queue-recovery] start request=%s conversation=%s status=%s", document.Request.RequestID, document.Request.ConversationID, document.Status)
		retry := false
		defer func() {
			activeQueuedChats.Delete(key)
			if retry {
				time.AfterFunc(25*time.Millisecond, func() { e.dispatchQueuedChat(actor, document) })
			}
		}()
		ctx := auth.WithActor(context.Background(), actor)
		if document.Ingress != nil {
			ctx = WithIngress(ctx, *document.Ingress)
		}
		request := document.Request
		request.queuedHash = document.Hash
		request.SpaceID, request.DeviceID, request.CoreID = document.Scope.SpaceID, document.Scope.InitiatorDeviceID, document.Scope.CoreID
		request.TargetDeviceID, request.ResourceOwnerID, request.RoleOwnerID = document.Scope.TargetDeviceID, document.Scope.ResourceOwnerID, document.Scope.RoleOwnerID
		current, finish, err := e.coordination.Restore(ctx, document.Scope, e.data)
		if err != nil {
			recordFailure("恢复排队请求失败", err)
			return
		}
		defer finish()
		port := e.data.(coordination.ResourcePort)
		terminal, err := e.prepareQueuedChatDispatch(current, &document)
		if err != nil {
			recordFailure("恢复排队请求执行状态失败", err)
			return
		}
		if terminal {
			return
		}
		if request.Group != nil {
			_, err = e.RunGroupEvents(current, request, nil)
		} else {
			_, err = e.Run(current, request)
		}
		if errors.Is(err, errQueuedChatNotNext) {
			retry = true
			return
		}
		if errors.Is(err, errQueuedChatRemoved) {
			amitialog.Printf("[queue-recovery] removed request=%s conversation=%s", document.Request.RequestID, document.Request.ConversationID)
			return
		}
		if err == nil {
			document.Status = "completed"
			queuedChatFailures.Delete(key)
		} else {
			recordFailure("执行排队请求失败", err)
			document.Status = "failed"
			if errors.Is(err, ErrUncertainExecution) {
				document.Status = "uncertain"
			}
			detail, marshalErr := json.Marshal(DescribeRuntimeError(err))
			if marshalErr != nil {
				recordFailure("序列化排队请求错误失败", errors.Join(err, marshalErr))
				return
			}
			document.Error = string(detail)
		}
		err = e.WithConversationExecutionScope(ctx, document.Scope, document.Request.ConversationID, func(current context.Context) error {
			id := queuedChatResourceID(document.Scope, request.RequestID)
			record, err := port.Resource(current, document.Scope, "checkpoint", id)
			if err != nil {
				return err
			}
			if record == nil || record.Deleted {
				return coordination.ErrResourceVersion
			}
			scope := document.Scope
			scope.RequestID = "queue-finish/" + request.RequestID
			_, err = e.commit(current, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: id, RoleID: scope.RoleID, ExpectedRevision: record.Revision, Body: body(document)}}})
			return err
		})
		if err != nil {
			if document.Error != "" {
				err = errors.Join(errors.New(document.Error), err)
			}
			recordFailure("保存排队请求结果失败", err)
			return
		}
		amitialog.Printf("[queue-recovery] settled request=%s conversation=%s status=%s", document.Request.RequestID, document.Request.ConversationID, document.Status)
		if err := e.coordination.FinishQueuedRequest(ctx, document.Scope); err != nil {
			recordFailure("结束排队请求失败", err)
		}
	}()
}

func (e *Engine) ResumePendingQueuedChats(ctx context.Context) error {
	var failures error
	for cursor := int64(0); ; {
		locations, err := e.coordination.PendingQueuedRequests(ctx, cursor)
		if err != nil {
			return errors.Join(failures, err)
		}
		if len(locations) == 0 {
			return failures
		}
		amitialog.Printf("[queue-recovery] scanned locations=%d cursor=%d", len(locations), cursor)
		for _, location := range locations {
			cursor = location.Cursor
			key := queuedChatKey{Engine: e, Owner: location.Scope.ResourceOwnerID, Role: location.Scope.RoleID, Request: location.Scope.RequestID}
			recordFailure := func(err error) {
				detail := fmt.Errorf("恢复排队请求 %s 失败: %w", location.Scope.RequestID, err)
				queuedChatFailures.Store(key, detail.Error())
				failures = errors.Join(failures, detail)
			}
			current, finish, err := e.coordination.Restore(ctx, location.Scope, e.data)
			if err != nil {
				if ctx.Err() != nil {
					return errors.Join(failures, err, ctx.Err())
				}
				recordFailure(err)
				continue
			}
			port, ok := e.data.(coordination.ResourcePort)
			if !ok {
				finish()
				return coordination.ErrWrongOwner
			}
			unlockQueue, err := e.lockContext(ctx, "queue-control\x00"+RuntimeKey(location.Scope, location.ConversationID))
			if err != nil {
				finish()
				return errors.Join(failures, err)
			}
			row, err := port.Resource(current, location.Scope, "checkpoint", queuedChatResourceID(location.Scope, location.Scope.RequestID))
			unlockQueue()
			finish()
			if err != nil {
				recordFailure(err)
				continue
			}
			if row == nil || row.Deleted {
				if err := e.coordination.FinishQueuedRequest(ctx, location.Scope); err != nil {
					recordFailure(err)
				}
				continue
			}
			var document queuedChatDocument
			if err := json.Unmarshal(row.Body, &document); err != nil {
				recordFailure(err)
				continue
			}
			if !sameQueueAuthority(document.Scope, location.Scope) || document.Request.RequestID != location.Scope.RequestID || document.Request.ConversationID != location.ConversationID {
				recordFailure(coordination.ErrWrongOwner)
				continue
			}
			if document.Status != "queued" && document.Status != "running" {
				if document.Status == "steering" {
					item := queuedConversationDocument{
						resource: coordination.Resource{ID: row.ID, RoleID: row.RoleID, Revision: row.Revision},
						document: document,
					}
					err := e.WithConversationExecutionScope(ctx, document.Scope, document.Request.ConversationID, func(current context.Context) error {
						recovered, err := e.markQueuedChatUncertain(current, item, errors.New("进程在引导提交后、运行时确认前中断"))
						if err != nil {
							return err
						}
						return e.coordination.FinishQueuedRequest(current, recovered.document.Scope)
					})
					if err != nil {
						recordFailure(err)
					}
					continue
				}
				if err := e.coordination.FinishQueuedRequest(ctx, location.Scope); err != nil {
					recordFailure(err)
				}
				continue
			}
			if document.Actor == nil || document.Actor.SpaceID.String() != location.Scope.SpaceID || document.Actor.DeviceID.String() != location.Scope.InitiatorDeviceID {
				recordFailure(coordination.ErrWrongOwner)
				continue
			}
			amitialog.Printf("[queue-recovery] dispatch request=%s conversation=%s status=%s", document.Request.RequestID, document.Request.ConversationID, document.Status)
			e.dispatchQueuedChat(document.Actor.Clone(), document)
		}
	}
}

func (e *Engine) ResumeQueuedChat(ctx context.Context, expected coordination.ExecutionScope, requestID string, conversation ...string) (QueuedChatResponse, error) {
	actor, err := auth.RequireActor(ctx)
	if err != nil {
		return QueuedChatResponse{}, err
	}
	if actor.SpaceID.String() != expected.SpaceID || actor.DeviceID.String() != expected.InitiatorDeviceID {
		return QueuedChatResponse{}, coordination.ErrWrongOwner
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return QueuedChatResponse{}, coordination.ErrWrongOwner
	}
	var document queuedChatDocument
	conversationID := ""
	if len(conversation) > 0 {
		conversationID = conversation[0]
	}
	err = e.WithConversationExecutionScope(ctx, expected, conversationID, func(current context.Context) error {
		resource, err := port.Resource(current, expected, "checkpoint", queuedChatResourceID(expected, requestID))
		if err != nil {
			return err
		}
		if resource == nil || resource.Deleted {
			return ErrConversationUnavailable
		}
		return json.Unmarshal(resource.Body, &document)
	})
	if err != nil {
		return QueuedChatResponse{}, err
	}
	if !sameQueueAuthority(expected, document.Scope) || document.Request.RequestID != requestID {
		return QueuedChatResponse{}, coordination.ErrScopeExpired
	}
	if len(conversation) > 0 && document.Request.ConversationID != conversation[0] {
		return QueuedChatResponse{}, coordination.ErrWrongOwner
	}
	if document.Status == "steering" {
		var recovered queuedConversationDocument
		err := e.WithConversationExecutionScope(ctx, document.Scope, document.Request.ConversationID, func(current context.Context) error {
			item, err := e.persistentQueuedChat(current, document.Scope, requestID)
			if err != nil {
				return err
			}
			recovered, err = e.markQueuedChatUncertain(current, item, errors.New("进程在引导提交后、运行时确认前中断"))
			if err != nil {
				return err
			}
			return e.coordination.FinishQueuedRequest(current, recovered.document.Scope)
		})
		if err != nil {
			return QueuedChatResponse{}, err
		}
		document = recovered.document
	} else if document.Status == "queued" || document.Status == "running" {
		e.dispatchQueuedChat(actor.Clone(), document)
	}
	return e.queuedChatResponse(document, expected), nil
}

func (e *Engine) queuedChatResponse(document queuedChatDocument, scope coordination.ExecutionScope) QueuedChatResponse {
	detail := document.Error
	key := queuedChatKey{Engine: e, Owner: document.Scope.ResourceOwnerID, Role: document.Scope.RoleID, Request: document.Request.RequestID}
	if failure, ok := queuedChatFailures.Load(key); ok {
		if detail != "" {
			detail += "\n"
		}
		detail += failure.(string)
	}
	attachments := make([]QueuedChatAttachment, 0, len(document.Request.Attachments))
	for _, attachment := range document.Request.Attachments {
		attachments = append(attachments, QueuedChatAttachment{
			ArtifactID:      attachment.ArtifactID,
			OwnerDeviceID:   attachment.OwnerDeviceID,
			StorageDeviceID: attachment.StorageDeviceID,
			Kind:            attachment.Kind,
			Name:            attachment.Name,
			MIMEType:        attachment.MIMEType,
			SHA256:          attachment.SHA256,
			SizeBytes:       attachment.SizeBytes,
		})
	}
	var quote *QueuedChatQuote
	if document.Request.Quote != nil {
		quote = &QueuedChatQuote{
			OwnerID:          document.Request.Quote.OwnerID,
			RoleID:           document.Request.Quote.RoleID,
			ConversationID:   document.Request.Quote.ConversationID,
			MessageID:        document.Request.Quote.MessageID,
			ExpectedRevision: document.Request.Quote.ExpectedRevision,
		}
	}
	return QueuedChatResponse{Accepted: true, Status: document.Status, RequestID: document.Request.RequestID, ConversationID: document.Request.ConversationID, Scope: scope, Error: detail, Message: document.Request.Message, Position: document.Position, Attachments: attachments, Quote: quote}
}

func (e *Engine) ReorderQueuedChats(ctx context.Context, request Request, requestIDs []string) ([]QueuedChatResponse, error) {
	if request.ExpectedScope == nil || request.ConversationID == "" {
		return nil, coordination.ErrWrongOwner
	}
	scope := *request.ExpectedScope
	unlock, err := e.lockContext(ctx, "queue-control\x00"+RuntimeKey(scope, request.ConversationID))
	if err != nil {
		return nil, err
	}
	defer unlock()
	var responses []QueuedChatResponse
	err = e.WithConversationExecutionScope(ctx, scope, request.ConversationID, func(current context.Context) error {
		items, err := e.queuedConversationDocuments(current, request)
		if err != nil {
			return err
		}
		if len(items) != len(requestIDs) {
			return errors.New("排队消息已变化，请刷新后重试")
		}
		byID := make(map[string]queuedConversationDocument, len(items))
		for _, item := range items {
			byID[item.document.Request.RequestID] = item
		}
		mutations := make([]coordination.Mutation, 0, len(items))
		responses = make([]QueuedChatResponse, 0, len(items))
		position := time.Now().UnixMicro()
		for index, id := range requestIDs {
			item, ok := byID[id]
			if !ok {
				return errors.New("排队消息已变化或包含重复项，请刷新后重试")
			}
			delete(byID, id)
			item.document.Position = position + int64(index)
			mutations = append(mutations, coordination.Mutation{Kind: "checkpoint", ID: item.resource.ID, RoleID: scope.RoleID, ExpectedRevision: item.resource.Revision, Body: body(item.document)})
			responses = append(responses, e.queuedChatResponse(item.document, scope))
		}
		if len(mutations) == 0 {
			return nil
		}
		commitScope := scope
		commitScope.RequestID = "queue-order/" + uuid.NewString()
		_, err = e.commit(current, coordination.Commit{Scope: commitScope, Mutations: mutations})
		return err
	})
	if err != nil {
		return nil, err
	}
	return responses, nil
}

func (e *Engine) DeleteQueuedChat(ctx context.Context, request Request, queuedRequestID string) error {
	if request.ExpectedScope == nil || request.ConversationID == "" || queuedRequestID == "" {
		return coordination.ErrWrongOwner
	}
	scope := *request.ExpectedScope
	unlock, err := e.lockContext(ctx, "queue-control\x00"+RuntimeKey(scope, request.ConversationID))
	if err != nil {
		return err
	}
	defer unlock()
	return e.WithConversationExecutionScope(ctx, scope, request.ConversationID, func(current context.Context) error {
		items, err := e.queuedConversationDocuments(current, request)
		if err != nil {
			return err
		}
		for _, item := range items {
			if item.document.Request.RequestID != queuedRequestID {
				continue
			}
			if item.document.Status == "steering" || item.document.Status == "uncertain" {
				return queuedChatUncertainError(item.document, errors.New("引导状态尚未确认"))
			}
			item.document.Status = "deleted"
			commitScope := scope
			commitScope.RequestID = "queue-delete/" + uuid.NewString()
			_, err := e.commit(current, coordination.Commit{Scope: commitScope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: item.resource.ID, RoleID: scope.RoleID, ExpectedRevision: item.resource.Revision, Body: body(item.document)}}})
			if err != nil {
				return err
			}
			queuedChatFailures.Delete(queuedChatKey{Engine: e, Owner: item.document.Scope.ResourceOwnerID, Role: item.document.Scope.RoleID, Request: queuedRequestID})
			return e.coordination.FinishQueuedRequest(current, item.document.Scope)
		}
		return errors.New("排队消息已开始执行或不存在")
	})
}

// UpdateQueuedChat 原位替换尚未执行的排队消息内容与附件。
// 保留 requestId、position 与创建顺序，不改变队列位置，也不触发重新排队。
func (e *Engine) UpdateQueuedChat(ctx context.Context, request Request, queuedRequestID, message string, attachments []Attachment) (QueuedChatResponse, error) {
	if request.ExpectedScope == nil || request.ConversationID == "" || queuedRequestID == "" {
		return QueuedChatResponse{}, coordination.ErrWrongOwner
	}
	message = strings.TrimSpace(message)
	switch {
	case message == "":
		return QueuedChatResponse{}, fmt.Errorf("%w: 消息内容为空", ErrInvalidRequest)
	case len(message) > 128<<10:
		return QueuedChatResponse{}, fmt.Errorf("%w: 消息内容长度 %d 超过上限 %d", ErrInvalidRequest, len(message), 128<<10)
	}
	if err := ValidateAttachments(attachments); err != nil {
		return QueuedChatResponse{}, err
	}
	scope := *request.ExpectedScope
	unlock, err := e.lockContext(ctx, "queue-control\x00"+RuntimeKey(scope, request.ConversationID))
	if err != nil {
		return QueuedChatResponse{}, err
	}
	defer unlock()
	var response QueuedChatResponse
	err = e.WithConversationExecutionScope(ctx, scope, request.ConversationID, func(current context.Context) error {
		items, err := e.queuedConversationDocuments(current, request)
		if err != nil {
			return err
		}
		for _, item := range items {
			if item.document.Request.RequestID != queuedRequestID {
				continue
			}
			switch item.document.Status {
			case "queued":
			case "steering", "uncertain":
				return queuedChatUncertainError(item.document, errors.New("引导状态尚未确认"))
			default:
				return errors.New("排队消息已开始执行，无法编辑")
			}
			if err := e.validateChatAttachments(current, item.document.Scope, attachments); err != nil {
				return err
			}
			item.document.Request.Message = message
			item.document.Request.Attachments = attachments
			commitScope := scope
			commitScope.RequestID = "queue-update/" + uuid.NewString()
			if _, err := e.commit(current, coordination.Commit{Scope: commitScope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: item.resource.ID, RoleID: scope.RoleID, ExpectedRevision: item.resource.Revision, Body: body(item.document)}}}); err != nil {
				return err
			}
			response = e.queuedChatResponse(item.document, scope)
			return nil
		}
		return errors.New("排队消息已开始执行或不存在")
	})
	if err != nil {
		return QueuedChatResponse{}, err
	}
	return response, nil
}

func (e *Engine) SteerQueuedChat(ctx context.Context, request Request, queuedRequestID, turnID string) (string, error) {
	if request.ExpectedScope == nil || request.ConversationID == "" || queuedRequestID == "" || turnID == "" {
		return "", coordination.ErrWrongOwner
	}
	scope := *request.ExpectedScope
	key := RuntimeKey(scope, request.ConversationID)
	unlock, err := e.lockContext(ctx, "queue-control\x00"+key)
	if err != nil {
		return "", err
	}
	defer unlock()
	var content string
	err = e.WithConversationExecutionScope(ctx, scope, request.ConversationID, func(current context.Context) error {
		items, err := e.queuedConversationDocuments(current, request)
		if err != nil {
			return err
		}
		for _, item := range items {
			if item.document.Request.RequestID != queuedRequestID {
				continue
			}
			if item.document.Status != "queued" {
				if item.document.Status == "steering" || item.document.Status == "uncertain" {
					return queuedChatUncertainError(item.document, errors.New("引导状态尚未确认"))
				}
				return errors.New("失败或状态不确定的排队消息无法引导当前执行")
			}
			if len(item.document.Request.Attachments) != 0 {
				return errors.New("带附件的排队消息不能作为引导发送")
			}
			content = item.document.Request.Message
			err = ownedRuntimeManager.SteerWithCommit(key, turnID, content, func() error {
				item.document.Status = "steering"
				item.document.SteerTurnID = turnID
				item.document.Error = ""
				steeringRevision, err := e.persistQueuedChat(current, item, "queue-steer-prepare")
				if err != nil {
					return fmt.Errorf("保存引导提交状态失败: %w", err)
				}
				item.resource.Revision = steeringRevision
				return nil
			})
			if err != nil {
				return err
			}
			item.document.Status = "steered"
			if _, err := e.persistQueuedChat(current, item, "queue-steer"); err != nil {
				uncertain := queuedChatUncertainError(item.document, fmt.Errorf("引导已送入当前运行时，但最终状态保存失败: %w", err))
				detail, marshalErr := json.Marshal(DescribeRuntimeError(uncertain))
				if marshalErr != nil {
					return errors.Join(uncertain, marshalErr)
				}
				item.document.Status = "uncertain"
				item.document.Error = string(detail)
				if _, persistErr := e.persistQueuedChat(current, item, "queue-steer-uncertain"); persistErr != nil {
					return errors.Join(uncertain, persistErr)
				}
				_ = e.coordination.FinishQueuedRequest(current, item.document.Scope)
				return uncertain
			}
			queuedChatFailures.Delete(queuedChatKey{Engine: e, Owner: item.document.Scope.ResourceOwnerID, Role: item.document.Scope.RoleID, Request: queuedRequestID})
			return e.coordination.FinishQueuedRequest(current, item.document.Scope)
		}
		return errors.New("排队消息已开始执行或不存在")
	})
	return content, err
}

func sameQueueAuthority(a, b coordination.ExecutionScope) bool {
	a.RequestID, a.TurnID, a.ExecutionID, a.ContinuityRoleID = "", "", "", ""
	b.RequestID, b.TurnID, b.ExecutionID, b.ContinuityRoleID = "", "", "", ""
	return a == b
}

func (e *Engine) ResumeConversationQueuedChats(ctx context.Context, request Request) ([]QueuedChatResponse, error) {
	items, err := e.queuedConversationDocuments(ctx, request)
	if err != nil {
		return nil, err
	}
	responses := make([]QueuedChatResponse, 0, len(items))
	for _, item := range items {
		response, err := e.ResumeQueuedChat(ctx, *request.ExpectedScope, item.document.Request.RequestID, request.ConversationID)
		if err != nil {
			return nil, err
		}
		if response.Status == "queued" || response.Status == "failed" || response.Status == "uncertain" {
			responses = append(responses, response)
		}
	}
	return responses, nil
}

type queuedConversationDocument struct {
	resource coordination.Resource
	document queuedChatDocument
}

func (e *Engine) queuedConversationDocuments(ctx context.Context, request Request) ([]queuedConversationDocument, error) {
	if request.ExpectedScope == nil || request.ConversationID == "" {
		return nil, coordination.ErrWrongOwner
	}
	items := make([]queuedConversationDocument, 0)
	query := coordination.DataQuery{ConversationID: request.ConversationID, Management: true, QueuedChats: true, ResourceKind: "checkpoint", Limit: 128}
	for {
		view, err := e.Query(ctx, request, query)
		if err != nil {
			return nil, fmt.Errorf("读取排队会话快照失败: %w", err)
		}
		for _, resource := range view.Snapshot.Resources {
			var document queuedChatDocument
			if resource.Kind != "checkpoint" || !strings.HasPrefix(resource.ID, "queued-chat/") {
				continue
			}
			if err := json.Unmarshal(resource.Body, &document); err != nil {
				return nil, fmt.Errorf("解析排队请求 %s 失败: %w", resource.ID, err)
			}
			if document.Status != "queued" && document.Status != "steering" && document.Status != "failed" && document.Status != "uncertain" || document.Request.ConversationID != view.ConversationID || !sameQueueAuthority(*request.ExpectedScope, document.Scope) {
				continue
			}
			items = append(items, queuedConversationDocument{resource: resource, document: document})
		}
		cursor := view.Snapshot.NextCursors["checkpoint"]
		if cursor == "" {
			sort.Slice(items, func(i, j int) bool {
				if items[i].document.Position == items[j].document.Position {
					return items[i].document.Request.RequestID < items[j].document.Request.RequestID
				}
				return items[i].document.Position < items[j].document.Position
			})
			return items, nil
		}
		if cursor == query.Cursor {
			return nil, coordination.ErrRequestConflict
		}
		query.Cursor = cursor
	}
}
