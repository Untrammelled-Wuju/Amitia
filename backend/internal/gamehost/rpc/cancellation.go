package rpc

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

const (
	CancelMethod         = "control.request.cancel"
	MetadataCancelReason = "rpc.cancel_reason"
	MaxCancelReasonLen   = 128
)

type CancelRequest struct {
	RequestID string `json:"requestId"`
	Reason    string `json:"reason,omitempty"`
}

func ParseCancelEnvelope(env *protocol.Envelope) (CancelRequest, bool) {
	if env.Method != CancelMethod {
		return CancelRequest{}, false
	}

	var req CancelRequest
	if len(env.Payload) == 0 {
		return CancelRequest{}, false
	}

	if err := json.Unmarshal(env.Payload, &req); err != nil {
		return CancelRequest{}, false
	}

	req.RequestID = strings.TrimSpace(req.RequestID)
	if req.RequestID == "" {
		return CancelRequest{}, false
	}

	if len(req.Reason) > MaxCancelReasonLen {
		req.Reason = req.Reason[:MaxCancelReasonLen]
	}

	return req, true
}

func BuildCancelEnvelope(requestID, reason string) protocol.Envelope {
	payload := CancelRequest{
		RequestID: requestID,
		Reason:    reason,
	}
	data, _ := json.Marshal(payload)
	return protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeNotification,
		Method:   CancelMethod,
		Payload:  data,
	}
}

func ValidateCancelSender(key RequestKey, sourcePeer ipc.Peer) error {
	if key.RuntimeID == domain.RuntimeInstanceID(sourcePeer.RuntimeID) &&
		key.ServiceID == domain.ServiceID(sourcePeer.ServiceID) {
		return nil
	}
	return NewRPCErrorWithCause(
		"unauthorized",
		domain.ErrPermissionDenied,
		fmt.Sprintf("peer cannot cancel request owned by %s", key),
		nil,
	)
}
