package business

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type SpeechInference struct {
	Scope coordination.ExecutionScope
	Role  coordination.Role
	Text  string
}

type SpeechModel interface {
	GenerateOwnedSpeech(context.Context, SpeechInference) ([]byte, error)
}

type SpeechAudio struct {
	MIME string `json:"mime"`
	Data string `json:"data"`
	Hash string `json:"sha256"`
}

type SpeechResponse struct {
	RequestID       string                       `json:"requestId"`
	Scope           coordination.ExecutionScope  `json:"executionScope"`
	Audio           SpeechAudio                  `json:"audio"`
	Saved           bool                         `json:"saved"`
	Acknowledgement coordination.Acknowledgement `json:"acknowledgement"`
}

type speechReceipt struct {
	Hash     string          `json:"hash"`
	State    string          `json:"state"`
	Response *SpeechResponse `json:"response,omitempty"`
}

func validateSpeechACK(ack coordination.Acknowledgement, scope coordination.ExecutionScope, resourceID string, started bool) error {
	if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != scope.RequestID {
		return coordination.ErrWrongOwner
	}
	if started {
		if len(ack.Versions) != 1 || ack.Versions["checkpoint/"+resourceID] != 1 {
			return coordination.ErrResourceVersion
		}
	} else if len(ack.Versions) != 2 || ack.Versions["checkpoint/"+resourceID] != 2 || ack.Versions["tool-result/"+resourceID] != 1 {
		return coordination.ErrResourceVersion
	}
	return nil
}

func validateStoredSpeech(result SpeechResponse, scope coordination.ExecutionScope, requestID string) error {
	if result.Scope != scope || result.RequestID != requestID || result.Audio.MIME != "audio/mpeg" || len(result.Audio.Hash) != 64 || len(result.Audio.Data) > base64.StdEncoding.EncodedLen(1<<20) {
		return coordination.ErrWrongOwner
	}
	data, err := base64.StdEncoding.Strict().DecodeString(result.Audio.Data)
	if err != nil || len(data) == 0 || len(data) > 1<<20 {
		return coordination.ErrResourceVersion
	}
	digest := sha256.Sum256(data)
	if result.Audio.Hash != hex.EncodeToString(digest[:]) {
		return coordination.ErrResourceVersion
	}
	return nil
}

func (e *Engine) currentSpeechResult(ctx context.Context, result SpeechResponse, scope coordination.ExecutionScope, requestID string, ack coordination.Acknowledgement, resourceID string) (SpeechResponse, error) {
	if err := validateStoredSpeech(result, scope, requestID); err != nil {
		return SpeechResponse{}, err
	}
	if err := validateSpeechACK(ack, scope, resourceID, false); err != nil {
		return SpeechResponse{}, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return SpeechResponse{}, err
	}
	if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
		return SpeechResponse{}, err
	}
	result.Saved, result.Acknowledgement = true, ack
	return result, nil
}

func (e *Engine) Speech(ctx context.Context, request Request, text string) (SpeechResponse, error) {
	model, ok := e.model.(SpeechModel)
	if !ok {
		return SpeechResponse{}, errors.New("当前 Core 没有私有语音合成服务")
	}
	if len(request.RequestID) > 96 || request.RequestID == "" || request.ExpectedScope == nil || len(text) > 8192 || strings.TrimSpace(text) == "" {
		return SpeechResponse{}, errors.New("朗读请求缺少当前角色、页面权限版本或有效文本")
	}
	originalID := request.RequestID
	request.RequestID = "speech/" + originalID
	ctx, scope, finish, err := e.continuityAuthority(ctx, request)
	if err != nil {
		return SpeechResponse{}, err
	}
	defer finish()
	if summaryAuthority(scope) != summaryAuthority(*request.ExpectedScope) {
		return SpeechResponse{}, coordination.ErrScopeExpired
	}
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		return SpeechResponse{}, err
	}
	role, err := coordination.ResolveRole(scope.RoleID, roles)
	if err != nil || role.Revision != scope.RoleRevision {
		return SpeechResponse{}, coordination.ErrRoleRequired
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return SpeechResponse{}, errors.New("语音数据所有者不支持确认保存")
	}
	scope.TurnID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(scope.CoreID+"\x00"+scope.InitiatorDeviceID+"\x00"+request.RequestID)).String()
	scope.ExecutionID = scope.TurnID
	ctx = coordination.WithScope(ctx, scope)
	resourceDigest := sha256.Sum256([]byte(scope.CoreID + "\x00" + scope.InitiatorDeviceID + "\x00" + originalID))
	resourceID := "speech/" + hex.EncodeToString(resourceDigest[:16])
	unlock := e.lock(scope.ResourceOwnerID + "\x00" + resourceID)
	defer unlock()
	digest := sha256.Sum256(body(struct {
		Scope   coordination.ExecutionScope
		Text    string
		Profile json.RawMessage
	}{summaryAuthority(scope), text, role.Profile}))
	fingerprint := hex.EncodeToString(digest[:])
	if pending, proof, err := e.pendingMutation(ctx, scope, resourceID, fingerprint); err != nil {
		return SpeechResponse{}, err
	} else if pending != nil {
		var receipt speechReceipt
		if json.Unmarshal(proof, &receipt) != nil || receipt.Response == nil {
			return SpeechResponse{}, coordination.ErrRequestConflict
		}
		var result SpeechResponse
		found := false
		for _, mutation := range pending.Mutations {
			if mutation.Kind == "tool-result" && mutation.ID == resourceID && !mutation.Deleted {
				if json.Unmarshal(mutation.Body, &result) != nil {
					return SpeechResponse{}, coordination.ErrRequestConflict
				}
				found = true
			}
		}
		if !found || result.Audio.Hash == "" {
			return SpeechResponse{}, coordination.ErrRequestConflict
		}
		if err := validateStoredSpeech(result, scope, originalID); err != nil {
			return SpeechResponse{}, err
		}
		ack, err := e.commit(ctx, *pending)
		if err != nil {
			return SpeechResponse{}, err
		}
		return e.currentSpeechResult(ctx, result, scope, originalID, ack, resourceID)
	}
	recorded, err := port.Resource(ctx, scope, "checkpoint", resourceID)
	if err != nil {
		return SpeechResponse{}, err
	}
	if recorded != nil {
		var receipt speechReceipt
		if recorded.Deleted || json.Unmarshal(recorded.Body, &receipt) != nil || receipt.Hash != fingerprint {
			return SpeechResponse{}, coordination.ErrRequestConflict
		}
		if receipt.State != "completed" || receipt.Response == nil {
			return SpeechResponse{}, ErrUncertainExecution
		}
		resource, err := port.Resource(ctx, scope, "tool-result", resourceID)
		if err != nil {
			return SpeechResponse{}, err
		}
		if resource == nil || resource.Deleted || resource.Revision != 1 || recorded.Revision != 2 || resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID || resource.Kind != "tool-result" || resource.ID != resourceID {
			return SpeechResponse{}, coordination.ErrResourceVersion
		}
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return SpeechResponse{}, err
		}
		var result SpeechResponse
		if json.Unmarshal(resource.Body, &result) != nil || result.Audio.Hash == "" {
			return SpeechResponse{}, coordination.ErrResourceVersion
		}
		ack := coordination.Acknowledgement{RequestID: scope.RequestID, OwnerID: scope.ResourceOwnerID, Versions: map[string]int64{"tool-result/" + resourceID: 1, "checkpoint/" + resourceID: recorded.Revision}}
		return e.currentSpeechResult(ctx, result, scope, originalID, ack, resourceID)
	}
	start := scope
	start.RequestID += "|start"
	startACK, err := e.commit(ctx, coordination.Commit{Scope: start, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: resourceID, RoleID: scope.RoleID, Body: body(speechReceipt{Hash: fingerprint, State: "started"})}}})
	if err != nil {
		return SpeechResponse{}, err
	}
	if err := validateSpeechACK(startACK, start, resourceID, true); err != nil {
		return SpeechResponse{}, err
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
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
	data, err := model.GenerateOwnedSpeech(ctx, SpeechInference{Scope: scope, Role: role, Text: text})
	if err != nil {
		return SpeechResponse{}, err
	}
	if ctx.Err() != nil {
		return SpeechResponse{}, context.Cause(ctx)
	}
	if len(data) == 0 || len(data) > 1<<20 {
		return SpeechResponse{}, errors.New("合成音频为空或超过大小上限")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return SpeechResponse{}, err
	}
	if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
		return SpeechResponse{}, err
	}
	audioDigest := sha256.Sum256(data)
	result := SpeechResponse{RequestID: originalID, Scope: scope, Audio: SpeechAudio{MIME: "audio/mpeg", Data: base64.StdEncoding.EncodeToString(data), Hash: hex.EncodeToString(audioDigest[:])}}
	receiptResult := result
	receiptResult.Audio = SpeechAudio{}
	ack, err := e.commit(ctx, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "tool-result", ID: resourceID, RoleID: scope.RoleID, Body: body(result)}, {Kind: "checkpoint", ID: resourceID, RoleID: scope.RoleID, ExpectedRevision: 1, Body: body(speechReceipt{Hash: fingerprint, State: "completed", Response: &receiptResult})}}})
	if err != nil {
		return SpeechResponse{}, err
	}
	return e.currentSpeechResult(ctx, result, scope, originalID, ack, resourceID)
}
