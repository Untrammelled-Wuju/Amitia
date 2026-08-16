package sdk

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/u-ai/backend/pkg/gameplugin/protocol/contracts"
)

const (
	MethodAuthoritySnapshot   = "control.authority.snapshot"
	MethodControlRegisterSink = "control.sink.register"
	MethodControlOutput       = "control.output"
	MethodControlAuthorityTakeover = "control.authority.takeover"
	MethodControlAuthorityRelease  = "control.authority.release"
	MethodEmergencyStopInitiate   = "emergency.stop.initiate"
	MethodEmergencyStopStatus     = "emergency.stop.status"
)

const (
	ControlModeObserve     = "observe"
	ControlModeAssist      = "assist"
	ControlModeShared      = "shared"
	ControlModePlugin      = "plugin"
	ControlModeUser        = "user"
	ControlModeSuspended   = "suspended"
)

const (
	OutputKindCustomRPC = "custom_rpc"
	OutputKindChannel   = "channel"
	OutputKindBinary    = "binary"
	OutputKindEffect    = "effect"
)

const (
	OutputDeniedStaleEpoch      = "stale_epoch"
	OutputDeniedPermission      = "permission_denied"
	OutputDeniedAuthorityMode   = "authority_mode_denied"
	OutputDeniedHostPolicy      = "host_policy_denied"
	OutputDeniedGateClosed      = "gate_closed"
	OutputDeniedRuntimeNotFound = "runtime_not_found"
	OutputDeniedServiceNotFound = "service_not_found"
	OutputDeniedInvalidPeer     = "invalid_peer"
	OutputDeniedNotEligible     = "runtime_not_eligible"
	OutputDeniedNotReady        = "not_ready"
)

const (
	TakeoverActorUser   = "user"
	TakeoverActorHost   = "host"
	TakeoverActorSystem = "system"
)

const (
	EmergencyStopStateRequested         = "requested"
	EmergencyStopStateClosingGate       = "closing_gate"
	EmergencyStopStateSuspending        = "suspending"
	EmergencyStopStateCancellingWork    = "cancelling_work"
	EmergencyStopStateStoppingRuntime   = "stopping_runtime"
	EmergencyStopStateClosingConnection = "closing_connection"
	EmergencyStopStateRevokingLeases    = "revoking_leases"
	EmergencyStopStateCleaningResources = "cleaning_resources"
	EmergencyStopStateVerifying         = "verifying"
	EmergencyStopStateCompleted         = "completed"
	EmergencyStopStateFailed            = "failed"
)

type AuthoritySnapshot struct {
	RuntimeID string `json:"runtimeId"`
	PluginID  string `json:"pluginId"`
	Mode      string `json:"mode"`
	Epoch     uint64 `json:"epoch"`
	ServiceID string `json:"serviceId,omitempty"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
	Valid     bool   `json:"valid"`
}

type ControlSinkRegisterInput = contracts.SinkRegisterInput

type ControlSinkRegisterResult struct {
	SinkID  string `json:"sinkId"`
	Registered bool `json:"registered"`
}

type ControlOutputInput = contracts.ControlOutputInput

type ControlOutputResult = contracts.ControlOutputResult

type AuthorityTakeoverInput struct {
	TargetMode    string `json:"targetMode"`
	Actor         string `json:"actor"`
	ExpectedEpoch uint64 `json:"expectedEpoch,omitempty"`
	ServiceID     string `json:"serviceId,omitempty"`
}

type AuthorityTakeoverResult struct {
	PreviousMode  string `json:"previousMode"`
	NewMode       string `json:"newMode"`
	PreviousEpoch uint64 `json:"previousEpoch"`
	NewEpoch      uint64 `json:"newEpoch"`
	Success       bool   `json:"success"`
	Reason        string `json:"reason,omitempty"`
}

type AuthorityReleaseInput struct {
	TargetMode    string `json:"targetMode"`
	Actor         string `json:"actor"`
	ExpectedEpoch uint64 `json:"expectedEpoch,omitempty"`
	ServiceID     string `json:"serviceId,omitempty"`
}

type AuthorityReleaseResult struct {
	PreviousMode  string `json:"previousMode"`
	NewMode       string `json:"newMode"`
	PreviousEpoch uint64 `json:"previousEpoch"`
	NewEpoch      uint64 `json:"newEpoch"`
	Success       bool   `json:"success"`
	Reason        string `json:"reason,omitempty"`
}

type EmergencyStopStatusInput struct {
	OperationID string `json:"operationId,omitempty"`
}

type EmergencyStopStatusResult struct {
	OperationID   string `json:"operationId"`
	State         string `json:"state"`
	Active        bool   `json:"active"`
	Reason        string `json:"reason,omitempty"`
	InitiatedAt   int64  `json:"initiatedAt,omitempty"`
	CompletedAt   int64  `json:"completedAt,omitempty"`
}

func (c *Client) GetAuthoritySnapshot(ctx context.Context, runtimeID, serviceID string, opts ...MessageOption) (AuthoritySnapshot, error) {
	input := map[string]any{
		"runtimeId": runtimeID,
		"serviceId": serviceID,
	}
	envelope, err := c.SendReservedRequest(ctx, MethodAuthoritySnapshot, input, opts...)
	if err != nil {
		return AuthoritySnapshot{}, err
	}
	var out AuthoritySnapshot
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return AuthoritySnapshot{}, NewEncodeError("unmarshal authority snapshot: %v", err)
		}
	}
	return out, nil
}

func (c *Client) RegisterControlSink(ctx context.Context, input ControlSinkRegisterInput, opts ...MessageOption) (ControlSinkRegisterResult, error) {
	envelope, err := c.SendReservedRequest(ctx, MethodControlRegisterSink, input, opts...)
	if err != nil {
		return ControlSinkRegisterResult{}, err
	}
	var out ControlSinkRegisterResult
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return ControlSinkRegisterResult{}, NewEncodeError("unmarshal control sink register response: %v", err)
		}
	}
	return out, nil
}

func (c *Client) SubmitControlOutput(ctx context.Context, input ControlOutputInput, opts ...MessageOption) (ControlOutputResult, error) {
	envelope, err := c.SendReservedRequest(ctx, MethodControlOutput, input, opts...)
	if err != nil {
		return ControlOutputResult{}, err
	}
	var out ControlOutputResult
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return ControlOutputResult{}, NewEncodeError("unmarshal control output response: %v", err)
		}
	}
	return out, nil
}

func (c *Client) TakeoverAuthority(ctx context.Context, input AuthorityTakeoverInput, runtimeID string, opts ...MessageOption) (AuthorityTakeoverResult, error) {
	envelope, err := c.SendReservedRequest(ctx, MethodControlAuthorityTakeover, input, opts...)
	if err != nil {
		return AuthorityTakeoverResult{}, err
	}
	var out AuthorityTakeoverResult
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return AuthorityTakeoverResult{}, NewEncodeError("unmarshal authority takeover response: %v", err)
		}
	}
	return out, nil
}

func (c *Client) ReleaseAuthority(ctx context.Context, input AuthorityReleaseInput, runtimeID string, opts ...MessageOption) (AuthorityReleaseResult, error) {
	envelope, err := c.SendReservedRequest(ctx, MethodControlAuthorityRelease, input, opts...)
	if err != nil {
		return AuthorityReleaseResult{}, err
	}
	var out AuthorityReleaseResult
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return AuthorityReleaseResult{}, NewEncodeError("unmarshal authority release response: %v", err)
		}
	}
	return out, nil
}

func (c *Client) GetEmergencyStopStatus(ctx context.Context, input EmergencyStopStatusInput, opts ...MessageOption) (EmergencyStopStatusResult, error) {
	envelope, err := c.SendReservedRequest(ctx, MethodEmergencyStopStatus, input, opts...)
	if err != nil {
		return EmergencyStopStatusResult{}, err
	}
	var out EmergencyStopStatusResult
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return EmergencyStopStatusResult{}, NewEncodeError("unmarshal emergency stop status: %v", err)
		}
	}
	return out, nil
}

const EffectCommitCacheTTL = 5 * time.Minute

type EffectCommitCacheEntry struct {
	Result    SinkEffectCommitResult
	Timestamp time.Time
}

type EffectCommitCache struct {
	mu      sync.Mutex
	entries map[string]EffectCommitCacheEntry
	ttl     time.Duration
}

func NewEffectCommitCache() *EffectCommitCache {
	return &EffectCommitCache{
		entries: make(map[string]EffectCommitCacheEntry),
		ttl:     EffectCommitCacheTTL,
	}
}

func (c *EffectCommitCache) Get(outputID string) (SinkEffectCommitResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[outputID]
	if !ok {
		return SinkEffectCommitResult{}, false
	}
	if time.Since(entry.Timestamp) > c.ttl {
		delete(c.entries, outputID)
		return SinkEffectCommitResult{}, false
	}
	return entry.Result, true
}

func (c *EffectCommitCache) Put(outputID string, result SinkEffectCommitResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[outputID] = EffectCommitCacheEntry{
		Result:    result,
		Timestamp: time.Now(),
	}
}

func (c *EffectCommitCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]EffectCommitCacheEntry)
}

type SinkEffectDispatchPayload = contracts.SinkEffectDispatchPayload

type SinkEffectCommitResult = contracts.SinkEffectCommitResult

type SinkDispatchHandler func(payload SinkEffectDispatchPayload) (SinkEffectCommitResult, error)

func RegisterSinkDispatchHandler(registry interface {
	Register(method string, handler func(request interface{}) (interface{}, error))
}, handler SinkDispatchHandler, cache *EffectCommitCache) {
	registry.Register(MethodSinkDispatch, func(request interface{}) (interface{}, error) {
		payload, ok := request.(SinkEffectDispatchPayload)
		if !ok {
			return SinkEffectCommitResult{
				Accepted:   false,
				Committed:  false,
				EffectID:   "",
				Generation: 0,
				ErrorCode:  "invalid_argument",
				Message:    "invalid sink dispatch payload",
			}, nil
		}
		if cache != nil {
			if cached, hit := cache.Get(payload.OutputID); hit {
				return cached, nil
			}
		}
		result, err := handler(payload)
		if err != nil {
			return SinkEffectCommitResult{
				Accepted:   false,
				Committed:  false,
				EffectID:   "",
				Generation: 0,
				ErrorCode:  "handler_error",
				Message:    err.Error(),
			}, nil
		}
		if cache != nil && result.Committed {
			cache.Put(payload.OutputID, result)
		}
		return result, nil
	})
}

const MethodSinkDispatch = contracts.MethodSinkDispatch
