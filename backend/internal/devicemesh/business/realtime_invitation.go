package business

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type RealtimeInvitation struct {
	ID                 string                      `json:"id"`
	RecipientDeviceID  string                      `json:"recipientDeviceId"`
	CharacterID        string                      `json:"characterId"`
	ConversationID     string                      `json:"conversationId"`
	ConversationOrigin *ConversationOrigin         `json:"conversationOrigin,omitempty"`
	HistoricalRoleID   string                      `json:"historicalRoleId,omitempty"`
	Scope              coordination.ExecutionScope `json:"executionScope"`
	Nonce              string                      `json:"nonce"`
	ExpiresAt          time.Time                   `json:"expiresAt"`
	Revision           int64                       `json:"revision"`
	Status             string                      `json:"status"`
	CallType           string                      `json:"callType"`
}

type realtimeInvitationRecord struct {
	Invitation        RealtimeInvitation           `json:"invitation"`
	SenderScope       coordination.ExecutionScope  `json:"senderScope"`
	AcceptedRequestID string                       `json:"acceptedRequestId,omitempty"`
	Acknowledgement   coordination.Acknowledgement `json:"acknowledgement"`
}

func (e *Engine) CreateRealtimeInvitation(ctx context.Context, request Request, callType string) (RealtimeInvitation, coordination.Acknowledgement, error) {
	if _, err := uuid.Parse(request.RequestID); err != nil {
		return RealtimeInvitation{}, coordination.Acknowledgement{}, errors.New("邀请编号必须为 UUID")
	}
	if callType != "audio" && callType != "video" && callType != "screen" {
		return RealtimeInvitation{}, coordination.Acknowledgement{}, errors.New("通话类型无效")
	}
	senderContext, senderScope, senderFinish, err := e.RealtimeAuthority(ctx, request)
	if err != nil {
		return RealtimeInvitation{}, coordination.Acknowledgement{}, err
	}
	defer senderFinish()
	recipientRequest := request
	recipientRequest.DeviceID, recipientRequest.TargetDeviceID = senderScope.TargetDeviceID, senderScope.TargetDeviceID
	recipientRequest.ExpectedScope = nil
	recipientContext, scope, finish, err := e.continuityAuthority(senderContext, recipientRequest)
	if err != nil {
		return RealtimeInvitation{}, coordination.Acknowledgement{}, err
	}
	defer finish()
	recipientContext, err = coordination.WithRequestAuthority(recipientContext, senderContext)
	if err != nil {
		return RealtimeInvitation{}, coordination.Acknowledgement{}, err
	}
	if scope.CoreID != senderScope.CoreID || scope.RoleID != senderScope.RoleID || scope.RoleRevision != senderScope.RoleRevision || scope.RoleOwnerID != senderScope.RoleOwnerID || scope.ResourceOwnerID != senderScope.ResourceOwnerID || scope.TargetProviderEpoch != senderScope.TargetProviderEpoch || scope.TargetPermissionRevision != senderScope.TargetPermissionRevision {
		return RealtimeInvitation{}, coordination.Acknowledgement{}, coordination.ErrScopeExpired
	}
	if request.ConversationOrigin != nil && request.ConversationOrigin.OwnerID != scope.ResourceOwnerID && (!scope.Coordinated || request.ConversationOrigin.OwnerID != scope.TargetDeviceID) {
		return RealtimeInvitation{}, coordination.Acknowledgement{}, errors.New("邀请不能携带未经授权的第三设备对话")
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return RealtimeInvitation{}, coordination.Acknowledgement{}, errors.New("数据所有者不支持通话邀请")
	}
	id := "realtime-invitation/" + request.RequestID
	unlock := e.lock(scope.ResourceOwnerID + "\x00" + id)
	defer unlock()
	previous, err := port.Resource(recipientContext, scope, "checkpoint", id)
	if err != nil {
		return RealtimeInvitation{}, coordination.Acknowledgement{}, err
	}
	if previous != nil {
		var saved realtimeInvitationRecord
		if previous.Deleted || json.Unmarshal(previous.Body, &saved) != nil || saved.Invitation.Status != "pending" || summaryAuthority(saved.SenderScope) != summaryAuthority(senderScope) || summaryAuthority(saved.Invitation.Scope) != summaryAuthority(scope) || saved.Invitation.CallType != callType || saved.Invitation.ConversationOrigin != nil && request.ConversationOrigin == nil || string(body(saved.Invitation.ConversationOrigin)) != string(body(request.ConversationOrigin)) || request.ConversationID != "" && saved.Invitation.ConversationID != request.ConversationID {
			return RealtimeInvitation{}, coordination.Acknowledgement{}, coordination.ErrRequestConflict
		}
		return saved.Invitation, saved.Acknowledgement, nil
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return RealtimeInvitation{}, coordination.Acknowledgement{}, err
	}
	conversationID := request.ConversationID
	if conversationID == "" {
		conversationID = uuid.NewString()
	}
	invitation := RealtimeInvitation{ID: request.RequestID, RecipientDeviceID: scope.InitiatorDeviceID, CharacterID: scope.RoleID, ConversationID: conversationID, ConversationOrigin: request.ConversationOrigin, HistoricalRoleID: request.HistoricalRoleID, Scope: scope, Nonce: base64.RawURLEncoding.EncodeToString(nonce), ExpiresAt: time.Now().UTC().Add(45 * time.Second), Revision: 1, Status: "pending", CallType: callType}
	record := realtimeInvitationRecord{Invitation: invitation, SenderScope: senderScope, Acknowledgement: coordination.Acknowledgement{RequestID: scope.RequestID, OwnerID: scope.ResourceOwnerID, Versions: map[string]int64{"checkpoint/" + id: 1}}}
	if err := coordination.ValidateCurrent(senderContext); err != nil {
		return RealtimeInvitation{}, coordination.Acknowledgement{}, err
	}
	ack, err := e.commit(recipientContext, coordination.Commit{Scope: scope, AdditionalAuthorities: []coordination.ExecutionScope{senderScope}, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: id, RoleID: scope.RoleID, Body: body(record)}}})
	if err != nil {
		return RealtimeInvitation{}, ack, err
	}
	if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != scope.RequestID || len(ack.Versions) != 1 || ack.Versions["checkpoint/"+id] != 1 {
		return RealtimeInvitation{}, ack, errors.New("邀请所有者尚未确认保存")
	}
	return invitation, ack, nil
}

func (e *Engine) realtimeInvitation(ctx context.Context, request Request, id string) (context.Context, coordination.ExecutionScope, func(), realtimeInvitationRecord, error) {
	if _, err := uuid.Parse(id); err != nil {
		return ctx, coordination.ExecutionScope{}, func() {}, realtimeInvitationRecord{}, errors.New("邀请编号无效")
	}
	current, scope, finish, err := e.continuityAuthority(ctx, request)
	if err != nil {
		return ctx, scope, finish, realtimeInvitationRecord{}, err
	}
	fail := func(err error) (context.Context, coordination.ExecutionScope, func(), realtimeInvitationRecord, error) {
		finish()
		return ctx, scope, func() {}, realtimeInvitationRecord{}, err
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return fail(errors.New("所有者无法读取通话邀请"))
	}
	row, err := port.Resource(current, scope, "checkpoint", "realtime-invitation/"+id)
	if err != nil {
		return fail(err)
	}
	var record realtimeInvitationRecord
	if row == nil || row.Deleted || json.Unmarshal(row.Body, &record) != nil || record.Invitation.ID != id || record.Invitation.RecipientDeviceID != request.DeviceID || row.Revision != record.Invitation.Revision || !record.Invitation.ExpiresAt.After(time.Now()) || summaryAuthority(record.Invitation.Scope) != summaryAuthority(scope) {
		return fail(coordination.ErrScopeExpired)
	}
	sender := Request{SpaceID: record.SenderScope.SpaceID, DeviceID: record.SenderScope.InitiatorDeviceID, TargetDeviceID: record.SenderScope.TargetDeviceID, CoreID: request.CoreID, RoleID: record.SenderScope.RoleID, RequestID: uuid.NewString(), ExpectedScope: &record.SenderScope}
	senderContext, _, senderFinish, err := e.RealtimeAuthority(current, sender)
	if err != nil {
		return fail(err)
	}
	if err := coordination.ValidateCurrent(senderContext); err != nil {
		senderFinish()
		return fail(err)
	}
	linked, err := coordination.WithRequestAuthority(current, senderContext)
	if err != nil {
		senderFinish()
		return fail(err)
	}
	linked, cancel := context.WithCancelCause(linked)
	stop := context.AfterFunc(senderContext, func() { cancel(context.Cause(senderContext)) })
	return linked, scope, func() { stop(); cancel(context.Canceled); senderFinish(); finish() }, record, nil
}

func (e *Engine) ReadRealtimeInvitation(ctx context.Context, request Request, id string) (RealtimeInvitation, error) {
	current, scope, finish, record, err := e.realtimeInvitation(ctx, request, id)
	if err != nil {
		return RealtimeInvitation{}, err
	}
	defer finish()
	if err := coordination.ValidateCurrent(current); err != nil {
		return RealtimeInvitation{}, err
	}
	if err := coordination.ValidateRoleRevision(current, e.data, scope); err != nil {
		return RealtimeInvitation{}, err
	}
	return record.Invitation, nil
}

func (e *Engine) AcceptRealtimeInvitation(ctx context.Context, request Request, id, nonce string, revision int64) (RealtimeInvitation, coordination.Acknowledgement, error) {
	if _, err := uuid.Parse(request.RequestID); err != nil || request.ExpectedScope == nil {
		return RealtimeInvitation{}, coordination.Acknowledgement{}, errors.New("接听缺少原邀请范围与新操作编号")
	}
	unlock := e.lock(request.CoreID + "\x00invitation\x00" + id)
	defer unlock()
	current, scope, finish, record, err := e.realtimeInvitation(ctx, request, id)
	if err != nil {
		return RealtimeInvitation{}, coordination.Acknowledgement{}, err
	}
	defer finish()
	if summaryAuthority(*request.ExpectedScope) != summaryAuthority(record.Invitation.Scope) || subtle.ConstantTimeCompare([]byte(nonce), []byte(record.Invitation.Nonce)) != 1 || revision != 1 {
		return RealtimeInvitation{}, coordination.Acknowledgement{}, coordination.ErrScopeExpired
	}
	if record.Invitation.Status == "accepted" {
		if record.AcceptedRequestID != request.RequestID {
			return RealtimeInvitation{}, coordination.Acknowledgement{}, coordination.ErrRequestConflict
		}
		return record.Invitation, record.Acknowledgement, nil
	}
	if record.Invitation.Status != "pending" || record.Invitation.Revision != revision {
		return RealtimeInvitation{}, coordination.Acknowledgement{}, coordination.ErrResourceVersion
	}
	record.Invitation.Status, record.Invitation.Revision, record.AcceptedRequestID = "accepted", 2, request.RequestID
	record.Acknowledgement = coordination.Acknowledgement{RequestID: scope.RequestID, OwnerID: scope.ResourceOwnerID, Versions: map[string]int64{"checkpoint/realtime-invitation/" + id: 2}}
	ack, err := e.commit(current, coordination.Commit{Scope: scope, AdditionalAuthorities: []coordination.ExecutionScope{record.SenderScope}, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: "realtime-invitation/" + id, RoleID: scope.RoleID, ExpectedRevision: revision, Body: body(record)}}})
	if err != nil {
		return RealtimeInvitation{}, ack, err
	}
	if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != scope.RequestID || len(ack.Versions) != 1 || ack.Versions["checkpoint/realtime-invitation/"+id] != 2 {
		return RealtimeInvitation{}, ack, errors.New("接听所有者尚未确认保存")
	}
	return record.Invitation, ack, nil
}
