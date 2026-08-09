package rpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type RequestState string

const (
	RequestStateCreated   RequestState = "created"
	RequestStatePending   RequestState = "pending"
	RequestStateRunning   RequestState = "running"
	RequestStateCompleted RequestState = "completed"
	RequestStateFailed    RequestState = "failed"
	RequestStateTimedOut  RequestState = "timed_out"
	RequestStateCancelled RequestState = "cancelled"
)

var terminalStates = map[RequestState]bool{
	RequestStateCompleted: true,
	RequestStateFailed:    true,
	RequestStateTimedOut:  true,
	RequestStateCancelled: true,
}

func (s RequestState) IsTerminal() bool {
	return terminalStates[s]
}

type RequestKey struct {
	RuntimeID domain.RuntimeInstanceID
	ServiceID domain.ServiceID
	RequestID string
}

func (k RequestKey) String() string {
	return fmt.Sprintf("%s/%s/%s", k.RuntimeID, k.ServiceID, k.RequestID)
}

func (k RequestKey) Validate() error {
	if k.RuntimeID == "" {
		return fmt.Errorf("request key runtime id must not be empty")
	}
	if k.ServiceID == "" {
		return fmt.Errorf("request key service id must not be empty")
	}
	if k.RequestID == "" {
		return fmt.Errorf("request key request id must not be empty")
	}
	return nil
}

func RequestKeyFromIPC(id string, peer ipc.Peer) RequestKey {
	return RequestKey{
		RuntimeID: domain.RuntimeInstanceID(peer.RuntimeID),
		ServiceID: domain.ServiceID(peer.ServiceID),
		RequestID: id,
	}
}

func RequestKeyFromProtocol(id string, env *protocol.Envelope) RequestKey {
	return RequestKey{
		RuntimeID: domain.RuntimeInstanceID(env.RuntimeID),
		ServiceID: domain.ServiceID(env.ServiceID),
		RequestID: id,
	}
}

type PendingRequest struct {
	Key RequestKey

	Method Method

	RequestID string

	Namespace Namespace

	Request protocol.Envelope

	State     RequestState
	CreatedAt time.Time

	Ctx        context.Context
	CancelFunc context.CancelFunc

	Target *RequestKey

	Done chan struct{}

	Result protocol.Envelope

	Error error

	Fingerprint RequestFingerprint

	IsUpstream bool

	CorrelationID string
}

func (r *PendingRequest) IsTerminal() bool {
	return r.State.IsTerminal()
}

type RequestFingerprint string

func ComputeFingerprint(method string, payload []byte) RequestFingerprint {
	h := sha256.New()
	h.Write([]byte(method))
	h.Write(payload)
	return RequestFingerprint(hex.EncodeToString(h.Sum(nil)))
}

func ComputeRequestFingerprint(req protocol.Envelope) RequestFingerprint {
	payload := req.Payload
	if payload == nil {
		payload = []byte{}
	}
	return ComputeFingerprint(req.Method, payload)
}

var _ = domain.ErrInvalidArgument
