package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"

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
}

type queuedChatDocument struct {
	Actor   *auth.ActorContext          `json:"actor,omitempty"`
	Hash    string                      `json:"hash"`
	Request Request                     `json:"request"`
	Scope   coordination.ExecutionScope `json:"executionScope"`
	Ingress *Ingress                    `json:"ingress,omitempty"`
	Status  string                      `json:"status"`
	Error   string                      `json:"error,omitempty"`
}

type queuedChatKey struct {
	Engine               *Engine
	Owner, Role, Request string
}

var activeQueuedChats sync.Map

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
	childScope := expected
	childScope.RequestID = request.RequestID
	childScope.TurnID, childScope.ExecutionID, childScope.ContinuityRoleID = uuid.NewSHA1(uuid.NameSpaceOID, []byte("queued-turn/"+request.RequestID)).String(), uuid.NewSHA1(uuid.NameSpaceOID, []byte("queued-execution/"+request.RequestID)).String(), ""
	request.ExpectedScope = &childScope
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return QueuedChatResponse{}, errors.New("Owner 排队存储未接通")
	}
	fingerprint := sha256.Sum256(body(request))
	hash := hex.EncodeToString(fingerprint[:])
	if err := e.coordination.RegisterQueuedRequest(ctx, childScope, hash, request.ConversationID); err != nil {
		return QueuedChatResponse{}, err
	}
	var document queuedChatDocument
	err = e.WithExecutionScope(ctx, expected, func(current context.Context) error {
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
		document = queuedChatDocument{Actor: actor.Clone(), Hash: hash, Request: request, Scope: childScope, Status: "queued"}
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
	response := QueuedChatResponse{Accepted: true, Status: document.Status, RequestID: request.RequestID, ConversationID: document.Request.ConversationID, Scope: expected, Error: document.Error}
	if document.Status == "queued" {
		e.dispatchQueuedChat(actor.Clone(), document)
	}
	return response, nil
}

func (e *Engine) dispatchQueuedChat(actor *auth.ActorContext, document queuedChatDocument) {
	key := queuedChatKey{Engine: e, Owner: document.Scope.ResourceOwnerID, Role: document.Scope.RoleID, Request: document.Request.RequestID}
	if _, loaded := activeQueuedChats.LoadOrStore(key, true); loaded {
		return
	}
	go func() {
		defer activeQueuedChats.Delete(key)
		ctx := auth.WithActor(context.Background(), actor)
		if document.Ingress != nil {
			ctx = WithIngress(ctx, *document.Ingress)
		}
		request := document.Request
		request.queuedHash = document.Hash
		request.SpaceID, request.DeviceID, request.CoreID = document.Scope.SpaceID, document.Scope.InitiatorDeviceID, document.Scope.CoreID
		current, finish, err := e.coordination.Restore(ctx, document.Scope, e.data)
		if err != nil {
			return
		}
		defer finish()
		port := e.data.(coordination.ResourcePort)
		checkpoint, err := port.Resource(current, document.Scope, "checkpoint", "turn/"+request.RequestID)
		if err != nil {
			return
		}
		if checkpoint == nil {
			if err := e.coordination.ResetUnstartedQueuedRequest(current, document.Scope, document.Hash); err != nil {
				return
			}
		}
		_, err = e.Run(current, request)
		if err == nil {
			document.Status = "completed"
		} else {
			document.Status = "failed"
			if errors.Is(err, ErrUncertainExecution) {
				document.Status = "uncertain"
			}
			document.Error = err.Error()
		}
		err = e.WithExecutionScope(ctx, document.Scope, func(current context.Context) error {
			port := e.data.(coordination.ResourcePort)
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
		if err == nil {
			if err := e.coordination.FinishQueuedRequest(ctx, document.Scope); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
		}
	}()
}

func (e *Engine) ResumePendingQueuedChats(ctx context.Context) error {
	for cursor := int64(0); ; {
		locations, err := e.coordination.PendingQueuedRequests(ctx, cursor)
		if err != nil {
			return err
		}
		if len(locations) == 0 {
			return nil
		}
		for _, location := range locations {
			cursor = location.Cursor
			current, finish, err := e.coordination.Restore(ctx, location.Scope, e.data)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				continue
			}
			port, ok := e.data.(coordination.ResourcePort)
			if !ok {
				finish()
				return coordination.ErrWrongOwner
			}
			row, err := port.Resource(current, location.Scope, "checkpoint", queuedChatResourceID(location.Scope, location.Scope.RequestID))
			finish()
			if err != nil || row == nil || row.Deleted {
				continue
			}
			var document queuedChatDocument
			if json.Unmarshal(row.Body, &document) != nil || !sameQueueAuthority(document.Scope, location.Scope) || document.Request.RequestID != location.Scope.RequestID || document.Request.ConversationID != location.ConversationID {
				continue
			}
			if document.Status != "queued" {
				if err := e.coordination.FinishQueuedRequest(ctx, location.Scope); err != nil {
					amitialog.Printf("[original-error] %v", err)
				}
				continue
			}
			if document.Actor == nil || document.Actor.SpaceID.String() != location.Scope.SpaceID || document.Actor.DeviceID.String() != location.Scope.InitiatorDeviceID {
				continue
			}
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
	err = e.WithExecutionScope(ctx, expected, func(current context.Context) error {
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
	if document.Status == "queued" {
		e.dispatchQueuedChat(actor.Clone(), document)
	}
	return QueuedChatResponse{Accepted: true, Status: document.Status, RequestID: requestID, ConversationID: document.Request.ConversationID, Scope: expected, Error: document.Error}, nil
}

func sameQueueAuthority(a, b coordination.ExecutionScope) bool {
	a.RequestID, a.TurnID, a.ExecutionID, a.ContinuityRoleID = "", "", "", ""
	b.RequestID, b.TurnID, b.ExecutionID, b.ContinuityRoleID = "", "", "", ""
	return a == b
}

func (e *Engine) ResumeConversationQueuedChats(ctx context.Context, request Request) ([]QueuedChatResponse, error) {
	if request.ExpectedScope == nil || request.ConversationID == "" {
		return nil, coordination.ErrWrongOwner
	}
	responses := make([]QueuedChatResponse, 0)
	query := coordination.DataQuery{ConversationID: request.ConversationID, Management: true, QueuedChats: true, ResourceKind: "checkpoint", Limit: 128}
	for {
		view, err := e.Query(ctx, request, query)
		if err != nil {
			return nil, err
		}
		for _, resource := range view.Snapshot.Resources {
			var document queuedChatDocument
			if resource.Kind != "checkpoint" || json.Unmarshal(resource.Body, &document) != nil || document.Status != "queued" || document.Request.ConversationID != view.ConversationID || !sameQueueAuthority(*request.ExpectedScope, document.Scope) {
				continue
			}
			response, err := e.ResumeQueuedChat(ctx, *request.ExpectedScope, document.Request.RequestID, view.ConversationID)
			if err != nil {
				return nil, err
			}
			responses = append(responses, response)
		}
		cursor := view.Snapshot.NextCursors["checkpoint"]
		if cursor == "" {
			return responses, nil
		}
		if cursor == query.Cursor {
			return nil, coordination.ErrRequestConflict
		}
		query.Cursor = cursor
	}
}
