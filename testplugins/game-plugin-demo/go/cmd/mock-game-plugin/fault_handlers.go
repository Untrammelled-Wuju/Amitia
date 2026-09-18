package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/u-ai/game-plugin-sdk-go/protocol"
)

type FaultInjectionService struct {
	mu      sync.RWMutex
	state   *FaultState
	barrier FaultBarrier
}

func NewFaultInjectionService() *FaultInjectionService {
	return &FaultInjectionService{
		state:   NewFaultState(),
		barrier: NewFaultBarrier(),
	}
}

func (s *FaultInjectionService) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Reset()
	s.barrier.Reset()
}

func (s *FaultInjectionService) GetState() *FaultState {
	return s.state
}

func (s *FaultInjectionService) GetBarrier() FaultBarrier {
	return s.barrier
}

type crashPayload struct {
	ExitCode int `json:"exitCode"`
	DelayMs  int `json:"delayMs"`
}

func (s *FaultInjectionService) HandleCrash(ctx context.Context, request protocol.Envelope) (any, error) {
	var reqPayload crashPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid crash payload: %w", err)
		}
	}

	if reqPayload.ExitCode == 0 {
		reqPayload.ExitCode = 1
	}

	if reqPayload.DelayMs > 0 {
		time.Sleep(time.Duration(reqPayload.DelayMs) * time.Millisecond)
	}

	os.Exit(reqPayload.ExitCode)

	return nil, nil
}

type handshakeDelayPayload struct {
	DelayMs   int `json:"delayMs"`
	DropCount int `json:"dropCount"`
}

func (s *FaultInjectionService) HandleHandshakeDelay(ctx context.Context, request protocol.Envelope) (any, error) {
	var reqPayload handshakeDelayPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid handshake delay payload: %w", err)
		}
	}

	s.mu.Lock()
	s.state.HandshakeDelay = time.Duration(reqPayload.DelayMs) * time.Millisecond
	s.state.HandshakeDropCnt = reqPayload.DropCount
	s.mu.Unlock()

	return map[string]any{
		"configured": true,
		"delayMs":    reqPayload.DelayMs,
		"dropCount":  reqPayload.DropCount,
	}, nil
}

type heartbeatPausePayload struct {
	PauseMs int `json:"pauseMs"`
}

func (s *FaultInjectionService) HandleHeartbeatPause(ctx context.Context, request protocol.Envelope) (any, error) {
	var reqPayload heartbeatPausePayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid heartbeat pause payload: %w", err)
		}
	}

	s.mu.Lock()
	if reqPayload.PauseMs < 0 {
		reqPayload.PauseMs = 0
	}
	s.state.HeartbeatPaused = true
	if reqPayload.PauseMs == 0 {
		s.state.HeartbeatPauseEnd = time.Time{}
	} else {
		s.state.HeartbeatPauseEnd = time.Now().Add(time.Duration(reqPayload.PauseMs) * time.Millisecond)
	}
	s.mu.Unlock()

	durationStr := fmt.Sprintf("%dms", reqPayload.PauseMs)
	if reqPayload.PauseMs == 0 {
		durationStr = "indefinite"
	}

	return map[string]any{
		"paused":   true,
		"pauseMs":  reqPayload.PauseMs,
		"duration": durationStr,
	}, nil
}

func (s *FaultInjectionService) HandleHeartbeatResume(ctx context.Context, request protocol.Envelope) (any, error) {
	s.mu.Lock()
	wasPaused := s.state.HeartbeatPaused
	s.state.HeartbeatPaused = false
	s.state.HeartbeatPauseEnd = time.Time{}
	select {
	case s.state.HeartbeatUnpause <- struct{}{}:
	default:
	}
	s.mu.Unlock()

	return map[string]any{
		"resumed":   true,
		"wasPaused": wasPaused,
	}, nil
}

type heartbeatDropPayload struct {
	Count int `json:"count"`
}

func (s *FaultInjectionService) HandleHeartbeatDrop(ctx context.Context, request protocol.Envelope) (any, error) {
	var reqPayload heartbeatDropPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid heartbeat drop payload: %w", err)
		}
	}

	s.mu.Lock()
	s.state.HeartbeatDropCnt = reqPayload.Count
	s.mu.Unlock()

	return map[string]any{
		"configured": true,
		"dropCount":  reqPayload.Count,
	}, nil
}

type rpcStallPayload struct {
	StallMs int `json:"stallMs"`
}

func (s *FaultInjectionService) HandleRPCStall(ctx context.Context, request protocol.Envelope) (any, error) {
	var reqPayload rpcStallPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid rpc stall payload: %w", err)
		}
	}

	if reqPayload.StallMs < 0 {
		reqPayload.StallMs = 0
	}
	if reqPayload.StallMs > 0 {
		time.Sleep(time.Duration(reqPayload.StallMs) * time.Millisecond)
	}

	return map[string]any{
		"stalled": true,
		"stallMs": reqPayload.StallMs,
	}, nil
}

type slowResponsePayload struct {
	ChunkCount   int `json:"chunkCount"`
	ChunkDelayMs int `json:"chunkDelayMs"`
}

func (s *FaultInjectionService) HandleRPCSlowResponse(ctx context.Context, request protocol.Envelope) (any, error) {
	var reqPayload slowResponsePayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid slow response payload: %w", err)
		}
	}

	if reqPayload.ChunkCount < 0 {
		reqPayload.ChunkCount = 0
	}
	if reqPayload.ChunkDelayMs < 0 {
		reqPayload.ChunkDelayMs = 0
	}

	chunks := make([]string, reqPayload.ChunkCount)
	for i := 0; i < reqPayload.ChunkCount; i++ {
		chunks[i] = fmt.Sprintf("chunk-%d", i)
		if reqPayload.ChunkDelayMs > 0 && i < reqPayload.ChunkCount-1 {
			time.Sleep(time.Duration(reqPayload.ChunkDelayMs) * time.Millisecond)
		}
	}

	return map[string]any{
		"slow":       true,
		"chunks":     chunks,
		"chunkCount": reqPayload.ChunkCount,
	}, nil
}

type queueFillPayload struct {
	Count int `json:"count"`
}

func (s *FaultInjectionService) HandleQueueFill(ctx context.Context, request protocol.Envelope) (any, error) {
	var reqPayload queueFillPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid queue fill payload: %w", err)
		}
	}

	if reqPayload.Count <= 0 {
		reqPayload.Count = 1
	}

	var wg sync.WaitGroup
	for i := 0; i < reqPayload.Count; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			time.Sleep(time.Duration(idx*10) * time.Millisecond)
		}(i)
	}
	wg.Wait()

	return map[string]any{
		"filled": true,
		"count":  reqPayload.Count,
	}, nil
}

type queueConsumeSlowPayload struct {
	Count  int `json:"count"`
	SlowMs int `json:"slowMs"`
}

func (s *FaultInjectionService) HandleQueueConsumeSlow(ctx context.Context, request protocol.Envelope) (any, error) {
	var reqPayload queueConsumeSlowPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid queue consume slow payload: %w", err)
		}
	}

	s.mu.Lock()
	s.state.QueueSlowCnt = reqPayload.Count
	s.state.QueueSlowDelay = time.Duration(reqPayload.SlowMs) * time.Millisecond
	s.mu.Unlock()

	return map[string]any{
		"configured": true,
		"count":      reqPayload.Count,
		"slowMs":     reqPayload.SlowMs,
	}, nil
}

func (s *FaultInjectionService) HandleSecretRevokeAndCheck(ctx context.Context, request protocol.Envelope) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]any{
		"leaseStatus": "granted",
		"revoked":     false,
	}, nil
}

type ignoreShutdownPayload struct {
	IgnoreMs int `json:"ignoreMs"`
}

func (s *FaultInjectionService) HandleIgnoreShutdown(ctx context.Context, request protocol.Envelope) (any, error) {
	var reqPayload ignoreShutdownPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid ignore shutdown payload: %w", err)
		}
	}

	s.mu.Lock()
	if reqPayload.IgnoreMs <= 0 {
		s.state.IgnoreShutdown = true
		s.state.IgnoreShutdownEnd = time.Time{}
	} else {
		s.state.IgnoreShutdown = false
		s.state.IgnoreShutdownEnd = time.Now().Add(time.Duration(reqPayload.IgnoreMs) * time.Millisecond)
	}
	s.mu.Unlock()

	durationStr := fmt.Sprintf("%dms", reqPayload.IgnoreMs)
	if reqPayload.IgnoreMs <= 0 {
		durationStr = "indefinite"
	}

	return map[string]any{
		"ignored":  true,
		"ignoreMs": reqPayload.IgnoreMs,
		"duration": durationStr,
	}, nil
}

type emergencyPayload struct {
	OperationID string `json:"operationId"`
	Reason      string `json:"reason"`
}

func (s *FaultInjectionService) HandleEmergencyResponse(ctx context.Context, request protocol.Envelope) (any, error) {
	var reqPayload emergencyPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid emergency payload: %w", err)
		}
	}

	s.mu.Lock()
	s.state.LastEmergencyOpID = reqPayload.OperationID
	s.mu.Unlock()

	return map[string]any{
		"received":     true,
		"operationId":  reqPayload.OperationID,
		"acknowledged": true,
	}, nil
}

type quiesceAckPayload struct {
	UpgradeID string `json:"upgradeId"`
}

func (s *FaultInjectionService) HandleUpgradeQuiesceAck(ctx context.Context, request protocol.Envelope) (any, error) {
	var reqPayload quiesceAckPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid upgrade quiesce ack payload: %w", err)
		}
	}

	return map[string]any{
		"ack":       true,
		"upgradeId": reqPayload.UpgradeID,
	}, nil
}

type simulateRestartPayload struct {
	SleepMs int `json:"sleepMs"`
}

func (s *FaultInjectionService) HandleBackendSimulateRestart(ctx context.Context, request protocol.Envelope) (any, error) {
	var reqPayload simulateRestartPayload
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &reqPayload); err != nil {
			return nil, fmt.Errorf("invalid simulate restart payload: %w", err)
		}
	}
	_ = reqPayload

	return map[string]any{
		"intent":    "persist_save_and_sleep",
		"sleepMs":   reqPayload.SleepMs,
		"simulated": true,
	}, nil
}

type residueTarget struct {
	ExtensionID string `json:"extensionId"`
	PluginID    string `json:"pluginId"`
	RuntimeID   string `json:"runtimeId"`
}

func (s *FaultInjectionService) HandleResidueGetCounts(ctx context.Context, request protocol.Envelope) (any, error) {
	var target residueTarget
	if len(request.Payload) > 0 {
		_ = json.Unmarshal(request.Payload, &target)
	}

	s.mu.RLock()
	activeConns := s.state.ActiveConns
	openChannels := s.state.OpenChannels
	s.mu.RUnlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	heapAllocMB := float64(memStats.HeapAlloc) / (1024 * 1024)

	return map[string]any{
		"target": map[string]any{
			"extensionId": target.ExtensionID,
			"pluginId":    target.PluginID,
			"runtimeId":   target.RuntimeID,
		},
		"residue": map[string]any{
			"extension":       0,
			"plugin":          0,
			"runtime":         0,
			"topology":        0,
			"process":         0,
			"connection":      activeConns,
			"ready":           0,
			"pendingRPC":      0,
			"hostapiInflight": 0,
			"leaseSession":    0,
			"sink":            0,
			"channel":         openChannels,
			"stream":          0,
			"binary":          0,
			"temp":            0,
			"lifecycleIntent": 0,
			"emergencyLatch":  0,
		},
		"goroutines":  runtime.NumGoroutine(),
		"heapAllocMB": heapAllocMB,
	}, nil
}

func (s *FaultInjectionService) ApplyHandshakeDelay() time.Duration {
	s.mu.RLock()
	delay := s.state.HandshakeDelay
	dropCnt := s.state.HandshakeDropCnt
	s.mu.RUnlock()

	if dropCnt > 0 {
		s.mu.Lock()
		s.state.HandshakeDropCnt--
		s.mu.Unlock()
		return -1
	}

	if delay > 0 {
		time.Sleep(delay)
	}

	return delay
}

func (s *FaultInjectionService) ShouldDropHeartbeat() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.HeartbeatDropCnt > 0 {
		s.state.HeartbeatDropCnt--
		return true
	}
	return false
}

func (s *FaultInjectionService) ShouldQueueSlow() (bool, time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.QueueSlowCnt > 0 {
		s.state.QueueSlowCnt--
		return true, s.state.QueueSlowDelay
	}
	return false, 0
}
