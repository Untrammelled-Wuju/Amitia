package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	sdk "github.com/u-ai/game-plugin-sdk-go"
	"github.com/u-ai/game-plugin-sdk-go/protocol"
)

type CoreService struct {
	counter int
	mu      sync.Mutex
}

func NewCoreService() *CoreService {
	return &CoreService{}
}

func (s *CoreService) HandleEcho(ctx context.Context, request protocol.Envelope) (any, error) {
	type echoPayload struct {
		Message string `json:"message"`
	}

	var reqPayload echoPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid echo payload: %w", err)
		}
	}

	s.mu.Lock()
	s.counter++
	currentCount := s.counter
	s.mu.Unlock()

	return map[string]any{
		"message": reqPayload.Message,
		"count":   currentCount,
	}, nil
}

func (s *CoreService) HandleIncrement(ctx context.Context, request protocol.Envelope) (any, error) {
	type incrementPayload struct {
		Delta int `json:"delta"`
	}

	var reqPayload incrementPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid increment payload: %w", err)
		}
	}

	if reqPayload.Delta == 0 {
		reqPayload.Delta = 1
	}

	s.mu.Lock()
	s.counter += reqPayload.Delta
	currentCount := s.counter
	s.mu.Unlock()

	return map[string]any{
		"counter": currentCount,
	}, nil
}

func (s *CoreService) HandleGenerate(ctx context.Context, request protocol.Envelope) (any, error) {
	type genPayload struct {
		Prompt string `json:"prompt"`
	}

	var reqPayload genPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid generate payload: %w", err)
		}
	}

	return map[string]any{
		"result": fmt.Sprintf("echo: %s", reqPayload.Prompt),
		"tokens": len(reqPayload.Prompt),
	}, nil
}

type StateService struct {
	mu     sync.Mutex
	states map[string]json.RawMessage
}

func NewStateService() *StateService {
	return &StateService{
		states: make(map[string]json.RawMessage),
	}
}

func (s *StateService) HandleSetState(ctx context.Context, request protocol.Envelope) (any, error) {
	type setStatePayload struct {
		StateId string          `json:"stateId"`
		Value   json.RawMessage `json:"value"`
	}

	var reqPayload setStatePayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid setstate payload: %w", err)
		}
	}

	s.mu.Lock()
	s.states[reqPayload.StateId] = reqPayload.Value
	s.mu.Unlock()

	return map[string]any{
		"acked":   true,
		"stateId": reqPayload.StateId,
	}, nil
}

func (s *StateService) HandleGetState(ctx context.Context, request protocol.Envelope) (any, error) {
	type getStatePayload struct {
		StateId string `json:"stateId"`
	}

	var reqPayload getStatePayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid getstate payload: %w", err)
		}
	}

	s.mu.Lock()
	val, ok := s.states[reqPayload.StateId]
	s.mu.Unlock()

	if !ok {
		return map[string]any{
			"stateId": reqPayload.StateId,
			"found":   false,
		}, nil
	}

	return map[string]any{
		"stateId": reqPayload.StateId,
		"found":   true,
		"value":   val,
	}, nil
}

type DataService struct {
	mu          sync.Mutex
	binaryCount int
}

func NewDataService() *DataService {
	return &DataService{}
}

func (s *DataService) HandleStoreBinary(ctx context.Context, request protocol.Envelope) (any, error) {
	type storePayload struct {
		BinaryId string `json:"binaryId"`
		Data     string `json:"data"`
	}

	var reqPayload storePayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid store payload: %w", err)
		}
	}

	s.mu.Lock()
	s.binaryCount++
	count := s.binaryCount
	s.mu.Unlock()

	return map[string]any{
		"stored":        true,
		"binaryId":      reqPayload.BinaryId,
		"size":          len(reqPayload.Data),
		"totalBinaries": count,
	}, nil
}

func (s *DataService) HandleStreamAppend(ctx context.Context, request protocol.Envelope) (any, error) {
	type appendPayload struct {
		StreamId string `json:"streamId"`
		Data     string `json:"data"`
	}

	var reqPayload appendPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid append payload: %w", err)
		}
	}

	return map[string]any{
		"appended": true,
		"streamId": reqPayload.StreamId,
		"sequence": time.Now().UnixNano(),
	}, nil
}

func (s *DataService) HandleChannelSend(ctx context.Context, request protocol.Envelope) (any, error) {
	type sendPayload struct {
		ChannelId string          `json:"channelId"`
		Message   json.RawMessage `json:"message"`
	}

	var reqPayload sendPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid send payload: %w", err)
		}
	}

	return map[string]any{
		"sent":      true,
		"channelId": reqPayload.ChannelId,
	}, nil
}

type ControlService struct {
	mu              sync.Mutex
	authorityMode   string
	authorityEpoch  uint64
	sinkRegistered  bool
	secretLeaseID   string
	lastOutputID    string
	permissionCache map[string]string
	emergencyActive bool
}

func NewControlService() *ControlService {
	return &ControlService{
		authorityMode:   sdk.ControlModeObserve,
		authorityEpoch:  0,
		permissionCache: make(map[string]string),
	}
}

func (s *ControlService) HandleControlProbe(ctx context.Context, request protocol.Envelope) (any, error) {
	type controlPayload struct {
		Action string `json:"action"`
		Value  int    `json:"value,omitempty"`
	}

	var reqPayload controlPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid control probe payload: %w", err)
		}
	}

	s.mu.Lock()
	mode := s.authorityMode
	epoch := s.authorityEpoch
	s.mu.Unlock()

	return map[string]any{
		"probe":       reqPayload.Action,
		"mode":        mode,
		"epoch":       epoch,
		"description": "opaque action",
	}, nil
}

func (s *ControlService) HandleAuthoritySnapshot(ctx context.Context, client *sdk.Client) (any, error) {
	result, err := client.GetAuthoritySnapshot(ctx)
	if err != nil {
		return map[string]any{"error": fmt.Sprintf("snapshot fetch failed: %v", err)}, nil
	}

	s.mu.Lock()
	s.authorityMode = result.Mode
	s.authorityEpoch = result.Epoch
	mode := s.authorityMode
	epoch := s.authorityEpoch
	s.mu.Unlock()

	return map[string]any{
		"mode":      mode,
		"epoch":     epoch,
		"updatedAt": result.UpdatedAt,
		"fetched":   true,
	}, nil
}

func (s *ControlService) HandleSubmitControlEffect(ctx context.Context, client *sdk.Client, request protocol.Envelope) (any, error) {
	type effectPayload struct {
		SinkID  string `json:"sinkId,omitempty"`
		Payload string `json:"payload,omitempty"`
		Value   int    `json:"value,omitempty"`
	}

	var reqPayload effectPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid effect payload: %w", err)
		}
	}

	s.mu.Lock()
	currentEpoch := s.authorityEpoch
	currentMode := s.authorityMode
	emergency := s.emergencyActive
	s.mu.Unlock()

	if emergency {
		return map[string]any{
			"allowed": false,
			"reason":  "emergency_stop_active",
		}, nil
	}

	if currentMode != sdk.ControlModeShared && currentMode != sdk.ControlModePlugin {
		return map[string]any{
			"allowed": false,
			"reason":  sdk.OutputDeniedAuthorityMode,
			"mode":    currentMode,
			"epoch":   currentEpoch,
		}, nil
	}

	rawPayload := json.RawMessage(fmt.Sprintf(`{"value": %d}`, reqPayload.Value))

	result, err := client.SubmitControlOutput(ctx, sdk.ControlOutputInput{
		OutputID: fmt.Sprintf("mock-effect-%d", time.Now().UnixNano()),
		SinkID:   reqPayload.SinkID,
		Epoch:    currentEpoch,
		Payload:  rawPayload,
	})
	if err != nil {
		return map[string]any{
			"allowed": false,
			"reason":  err.Error(),
			"epoch":   currentEpoch,
		}, nil
	}

	s.mu.Lock()
	s.lastOutputID = result.OutputID
	s.mu.Unlock()

	return map[string]any{
		"allowed":      result.Allowed,
		"reason":       result.Reason,
		"epoch":        currentEpoch,
		"currentEpoch": result.CurrentEpoch,
		"outputId":     result.OutputID,
	}, nil
}

func (s *ControlService) HandleTakeover(ctx context.Context, client *sdk.Client, request protocol.Envelope) (any, error) {
	type takeoverPayload struct {
		ExpectedEpoch *uint64 `json:"expectedEpoch,omitempty"`
	}

	var reqPayload takeoverPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid takeover payload: %w", err)
		}
	}

	result, err := client.TakeoverAuthority(ctx, sdk.AuthorityTakeoverInput{ExpectedEpoch: reqPayload.ExpectedEpoch})
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}

	s.mu.Lock()
	s.authorityMode = result.NewMode
	s.authorityEpoch = result.NewEpoch
	mode := s.authorityMode
	epoch := s.authorityEpoch
	s.mu.Unlock()

	return map[string]any{
		"previousMode":  result.PreviousMode,
		"newMode":       result.NewMode,
		"previousEpoch": result.PreviousEpoch,
		"newEpoch":      result.NewEpoch,
		"mode":          mode,
		"epoch":         epoch,
		"success":       true,
	}, nil
}

func (s *ControlService) HandleRelease(ctx context.Context, client *sdk.Client, request protocol.Envelope) (any, error) {
	type releasePayload struct {
		TargetMode    string  `json:"targetMode,omitempty"`
		ExpectedEpoch *uint64 `json:"expectedEpoch,omitempty"`
	}

	var reqPayload releasePayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid release payload: %w", err)
		}
	}
	if reqPayload.TargetMode == "" {
		reqPayload.TargetMode = sdk.ControlModeObserveOnly
	}

	result, err := client.ReleaseAuthority(ctx, sdk.AuthorityReleaseInput{
		TargetMode:    reqPayload.TargetMode,
		ExpectedEpoch: reqPayload.ExpectedEpoch,
	})
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}

	s.mu.Lock()
	s.authorityMode = result.NewMode
	s.authorityEpoch = result.NewEpoch
	mode := s.authorityMode
	epoch := s.authorityEpoch
	s.mu.Unlock()

	return map[string]any{
		"previousMode":  result.PreviousMode,
		"newMode":       result.NewMode,
		"previousEpoch": result.PreviousEpoch,
		"newEpoch":      result.NewEpoch,
		"mode":          mode,
		"epoch":         epoch,
		"success":       true,
	}, nil
}

func (s *ControlService) HandleEmergencyStop(ctx context.Context, client *sdk.Client) (any, error) {
	result, err := client.EmergencyStop(ctx, sdk.EmergencyStopInput{})
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}

	s.mu.Lock()
	s.emergencyActive = !result.Success
	if result.Success || result.State == sdk.EmergencyStopStateCompleted {
		s.authorityMode = sdk.ControlModeSuspended
	}
	mode := s.authorityMode
	epoch := s.authorityEpoch
	emergency := s.emergencyActive
	s.mu.Unlock()

	return map[string]any{
		"operationId": result.OperationID,
		"state":       result.State,
		"success":     result.Success,
		"active":      emergency,
		"mode":        mode,
		"epoch":       epoch,
	}, nil
}

func (s *ControlService) SetEmergencyActive(active bool) {
	s.mu.Lock()
	s.emergencyActive = active
	s.mu.Unlock()
}

func (s *ControlService) UpdateAuthority(mode string, epoch uint64) {
	s.mu.Lock()
	s.authorityMode = mode
	s.authorityEpoch = epoch
	s.mu.Unlock()
}

func (s *ControlService) HandleHostAPIPermissionCheck(ctx context.Context, client *sdk.Client, request protocol.Envelope) (any, error) {
	type checkPayload struct {
		PermissionID string `json:"permissionId"`
	}

	var reqPayload checkPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid check payload: %w", err)
		}
	}

	if reqPayload.PermissionID == "" {
		reqPayload.PermissionID = sdk.PermGameHostControl
	}

	result, err := client.CheckPermission(ctx, sdk.PermissionCheckInput{PermissionID: reqPayload.PermissionID})
	if err != nil {
		return map[string]any{
			"error":      err.Error(),
			"permission": reqPayload.PermissionID,
		}, nil
	}

	s.mu.Lock()
	s.permissionCache[reqPayload.PermissionID] = result.Decision
	s.mu.Unlock()

	return map[string]any{
		"permission": reqPayload.PermissionID,
		"decision":   result.Decision,
		"reason":     result.Reason,
	}, nil
}

type SecurityService struct {
	mu            sync.Mutex
	leaseID       string
	secretRef     sdk.SecretRef
	outputCounter uint64
}

func NewSecurityService() *SecurityService {
	return &SecurityService{
		secretRef: "secret://mock.provider.credential",
	}
}

func (s *SecurityService) AcquireLease(ctx context.Context, client *sdk.Client) error {
	result, err := client.AcquireSecret(ctx, sdk.SecretAcquireInput{
		Ref:      s.secretRef,
		Purpose:  sdk.SecretPurposeStartup,
		Required: false,
	})
	if err != nil {
		return fmt.Errorf("lease acquire failed: %w", err)
	}

	s.mu.Lock()
	s.leaseID = result.LeaseID
	s.mu.Unlock()

	return nil
}

func (s *SecurityService) HasActiveLease() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.leaseID != ""
}

func (s *SecurityService) HandleSecretProbe(ctx context.Context, request protocol.Envelope) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return map[string]any{
		"leaseId":     s.leaseID,
		"hasLease":    s.leaseID != "",
		"leaseStatus": "granted",
		"ref":         s.secretRef,
	}, nil
}

func (s *SecurityService) HandleReleaseLease(ctx context.Context, client *sdk.Client, request protocol.Envelope) (any, error) {
	s.mu.Lock()
	leaseID := s.leaseID
	s.mu.Unlock()

	if leaseID == "" {
		return map[string]any{
			"released": false,
			"reason":   "no_active_lease",
		}, nil
	}

	result, err := client.ReleaseSecret(ctx, sdk.SecretReleaseInput{
		LeaseID: leaseID,
		Reason:  "test_revoke",
	})
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}

	s.mu.Lock()
	s.leaseID = ""
	s.mu.Unlock()

	return map[string]any{
		"released": result.Released,
		"reason":   result.Reason,
	}, nil
}

func (s *SecurityService) HandleHostAPIProbe(ctx context.Context, client *sdk.Client, request protocol.Envelope) (any, error) {
	type apiPayload struct {
		Route string `json:"route"`
	}

	var reqPayload apiPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid api probe payload: %w", err)
		}
	}

	if reqPayload.Route == "" {
		reqPayload.Route = "host.runtime.health"
	}

	result, err := client.InvokeHostAPI(ctx, sdk.HostInvokeInput{
		Method: reqPayload.Route,
		Input:  json.RawMessage(`{}`),
	})
	if err != nil {
		return map[string]any{
			"error":  err.Error(),
			"route":  reqPayload.Route,
			"status": "failed",
		}, nil
	}

	return map[string]any{
		"route":     reqPayload.Route,
		"status":    result.Status,
		"hasOutput": result.Output != nil,
	}, nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		cancel()
	}()

	coreService := NewCoreService()
	stateService := NewStateService()
	dataService := NewDataService()
	controlService := NewControlService()
	securityService := NewSecurityService()
	faultService := NewFaultInjectionService()

	transport := sdk.NewDefaultStdioTransport()
	client := sdk.NewClient(transport)

	coreRegistry := sdk.NewHandlerRegistry()
	coreRegistry.RegisterRequest("mock.core.echo", coreService.HandleEcho)
	coreRegistry.RegisterRequest("mock.core.increment", coreService.HandleIncrement)
	coreRegistry.RegisterRequest("mock.core.generate", coreService.HandleGenerate)

	stateRegistry := coreRegistry
	stateRegistry.RegisterRequest("mock.state.set", stateService.HandleSetState)
	stateRegistry.RegisterRequest("mock.state.get", stateService.HandleGetState)

	dataRegistry := coreRegistry
	dataRegistry.RegisterRequest("mock.data.store", dataService.HandleStoreBinary)
	dataRegistry.RegisterRequest("mock.data.stream.append", dataService.HandleStreamAppend)
	dataRegistry.RegisterRequest("mock.data.channel.send", dataService.HandleChannelSend)

	runner := sdk.NewRunner(client, sdk.RunnerConfig{
		PluginID:         "mock-game-plugin-go",
		DefaultServiceID: "mock-go-runtime",
		Hello: sdk.HelloConfiguration{
			SupportedProtocols: []string{protocol.ProtocolVersion},
			Capabilities: []string{
				"realtime_control",
				"state_streaming",
				"event_streaming",
				"custom_rpc",
				"host_api",
				"shared_control",
			},
			RPCNamespaces: []string{"mock.core", "mock.state", "mock.data", "mock.control", "mock.security", "mock.fault"},
			Channels: []sdk.ChannelHelloDescriptor{
				{ID: "mock-events"},
				{ID: "mock-state"},
			},
			Sinks: []sdk.SinkHelloDescriptor{
				{SinkID: "mock.control.effect", Kind: sdk.OutputKindEffect, ServiceID: "mock-go-runtime"},
			},
			SDK: &sdk.SDKInfo{
				Name:    "@amitia/game-plugin-sdk-go",
				Version: "0.1.0",
			},
			Metadata: map[string]json.RawMessage{
				"mode":     json.RawMessage(`"g36-mock"`),
				"security": json.RawMessage(`"full"`),
			},
		},
		OnReady: func(ctx context.Context, client *sdk.Client) func(ctx context.Context) {
			if err := securityService.AcquireLease(ctx, client); err != nil {
				fmt.Fprintf(os.Stderr, "security lease warning: %v\n", err)
			}

			result, err := client.RegisterControlSink(ctx, sdk.ControlSinkRegisterInput{
				SinkID: "mock.control.effect",
				Kind:   sdk.OutputKindEffect,
			})
			_ = result
			_ = err

			client.SendNotification(ctx, "mock.ready", map[string]any{
				"status":           "ready",
				"services":         []string{"mock-go-runtime"},
				"sinks":            []string{"mock.control.effect"},
				"plugin":           "mock-game-plugin-go",
				"security_ready":   true,
				"has_secret_lease": securityService.HasActiveLease(),
			})

			return func(ctx context.Context) {
				securityService.HandleReleaseLease(ctx, client, protocol.Envelope{})
				client.SendNotification(ctx, "mock.shutdown", map[string]any{
					"status": "shutdown",
					"plugin": "mock-game-plugin-go",
				})
			}
		},
	})

	faultCrashHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return faultService.HandleCrash(ctx, request)
	}
	faultHandshakeDelayHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return faultService.HandleHandshakeDelay(ctx, request)
	}
	faultHeartbeatPauseHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return faultService.HandleHeartbeatPause(ctx, request)
	}
	faultHeartbeatResumeHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return faultService.HandleHeartbeatResume(ctx, request)
	}
	faultHeartbeatDropHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return faultService.HandleHeartbeatDrop(ctx, request)
	}
	faultRPCStallHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return faultService.HandleRPCStall(ctx, request)
	}
	faultRPCSlowResponseHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return faultService.HandleRPCSlowResponse(ctx, request)
	}
	faultQueueFillHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return faultService.HandleQueueFill(ctx, request)
	}
	faultQueueConsumeSlowHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return faultService.HandleQueueConsumeSlow(ctx, request)
	}
	faultSecretRevokeHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return faultService.HandleSecretRevokeAndCheck(ctx, request)
	}
	faultIgnoreShutdownHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return faultService.HandleIgnoreShutdown(ctx, request)
	}
	faultEmergencyHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return faultService.HandleEmergencyResponse(ctx, request)
	}
	faultUpgradeQuiesceAckHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return faultService.HandleUpgradeQuiesceAck(ctx, request)
	}
	faultBackendSimRestartHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return faultService.HandleBackendSimulateRestart(ctx, request)
	}
	faultResidueCountsHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return faultService.HandleResidueGetCounts(ctx, request)
	}
	hostAPIPermissionHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return controlService.HandleHostAPIPermissionCheck(ctx, client, request)
	}
	controlProbeHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return controlService.HandleControlProbe(ctx, request)
	}
	submitEffectHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return controlService.HandleSubmitControlEffect(ctx, client, request)
	}
	takeoverHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return controlService.HandleTakeover(ctx, client, request)
	}
	releaseHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return controlService.HandleRelease(ctx, client, request)
	}
	emergencyStopHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return controlService.HandleEmergencyStop(ctx, client)
	}
	secretProbeHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return securityService.HandleSecretProbe(ctx, request)
	}
	releaseLeaseHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return securityService.HandleReleaseLease(ctx, client, request)
	}
	hostAPIProbeHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return securityService.HandleHostAPIProbe(ctx, client, request)
	}
	authoritySnapshotHandler := func(ctx context.Context, request protocol.Envelope) (any, error) {
		return controlService.HandleAuthoritySnapshot(ctx, client)
	}

	controlRegistry := coreRegistry
	controlRegistry.RegisterRequest("mock.security.hostapi_check", hostAPIPermissionHandler)
	controlRegistry.RegisterRequest("mock.control.probe", controlProbeHandler)
	controlRegistry.RegisterRequest("mock.control.submit_effect", submitEffectHandler)
	controlRegistry.RegisterRequest("mock.control.authority.takeover", takeoverHandler)
	controlRegistry.RegisterRequest("mock.control.authority.release", releaseHandler)
	controlRegistry.RegisterRequest("mock.control.authority.snapshot", authoritySnapshotHandler)
	controlRegistry.RegisterRequest("mock.control.emergency_status", emergencyStopHandler)
	controlRegistry.RegisterRequest("mock.security.secret_probe", secretProbeHandler)
	controlRegistry.RegisterRequest("mock.security.release_lease", releaseLeaseHandler)
	controlRegistry.RegisterRequest("mock.security.hostapi_probe", hostAPIProbeHandler)

	controlRegistry.RegisterNotification("host.authority.changed", func(ctx context.Context, notification protocol.Envelope) error {
		var payload struct {
			Mode  string `json:"mode"`
			Epoch uint64 `json:"epoch"`
		}
		if len(notification.Payload) > 0 {
			json.Unmarshal(notification.Payload, &payload)
		}
		controlService.UpdateAuthority(payload.Mode, payload.Epoch)
		return nil
	})

	controlRegistry.RegisterNotification("host.authority.emergency", func(ctx context.Context, notification protocol.Envelope) error {
		controlService.SetEmergencyActive(true)
		return nil
	})

	faultRegistry := coreRegistry
	faultRegistry.RegisterRequest("mock.fault.crash", faultCrashHandler)
	faultRegistry.RegisterRequest("mock.fault.handshake.delay", faultHandshakeDelayHandler)
	faultRegistry.RegisterRequest("mock.fault.heartbeat.pause", faultHeartbeatPauseHandler)
	faultRegistry.RegisterRequest("mock.fault.heartbeat.resume", faultHeartbeatResumeHandler)
	faultRegistry.RegisterRequest("mock.fault.heartbeat.drop", faultHeartbeatDropHandler)
	faultRegistry.RegisterRequest("mock.fault.rpc.stall", faultRPCStallHandler)
	faultRegistry.RegisterRequest("mock.fault.rpc.slow_response", faultRPCSlowResponseHandler)
	faultRegistry.RegisterRequest("mock.fault.queue.fill", faultQueueFillHandler)
	faultRegistry.RegisterRequest("mock.fault.queue.consume_slow", faultQueueConsumeSlowHandler)
	faultRegistry.RegisterRequest("mock.fault.secret.revoke_and_check", faultSecretRevokeHandler)
	faultRegistry.RegisterRequest("mock.fault.control.ignore_shutdown", faultIgnoreShutdownHandler)
	faultRegistry.RegisterRequest("mock.fault.control.emergency_response", faultEmergencyHandler)
	faultRegistry.RegisterRequest("mock.fault.upgrade.quiesce_ack", faultUpgradeQuiesceAckHandler)
	faultRegistry.RegisterRequest("mock.fault.backend.simulate_restart", faultBackendSimRestartHandler)
	faultRegistry.RegisterRequest("mock.fault.residue.get_counts", faultResidueCountsHandler)
	faultRegistry.RegisterNotification("host.authority.emergency", func(ctx context.Context, notification protocol.Envelope) error {
		controlService.SetEmergencyActive(true)
		_, _ = faultService.HandleEmergencyResponse(ctx, notification)
		return nil
	})

	runner.AddService("mock-go-runtime", coreRegistry)

	if err := runner.Run(ctx, coreRegistry); err != nil {
		fmt.Fprintf(os.Stderr, "plugin error: %v\n", err)
		os.Exit(1)
	}
}
