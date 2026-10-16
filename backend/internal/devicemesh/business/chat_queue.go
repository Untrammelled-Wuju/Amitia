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
	childScope.TurnID, childScope.ExecutionID, childScope.ContinuityRoleID = "", "", ""
	request.ExpectedScope = &childScope
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return QueuedChatResponse{}, errors.New("Owner 排队存储未接通")
	}
	fingerprint := sha256.Sum256(body(request))
	hash := hex.EncodeToString(fingerprint[:])
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
		document = queuedChatDocument{Hash: hash, Request: request, Scope: childScope, Status: "queued"}
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
		request.SpaceID, request.DeviceID, request.CoreID = document.Scope.SpaceID, document.Scope.InitiatorDeviceID, document.Scope.CoreID
		err := e.WithExecutionScope(ctx, document.Scope, func(current context.Context) error { _, err := e.Run(current, request); return err })
		if err == nil {
			document.Status = "completed"
		} else {
			document.Status = "failed"
			if errors.Is(err, ErrUncertainExecution) {
				document.Status = "uncertain"
			}
			document.Error = err.Error()
		}
		_ = e.WithExecutionScope(ctx, document.Scope, func(current context.Context) error {
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
	}()
}

func (e *Engine) ResumeQueuedChat(ctx context.Context, expected coordination.ExecutionScope, requestID string) (QueuedChatResponse, error) {
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
