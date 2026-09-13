package matrices

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/u-ai/mock-conformance-go"
)

func TestHeartbeat_Suite(t *testing.T) {
	cases := []conformance.TestCase{
		{
			ID:       "HB-001",
			Category: conformance.CatHeartbeat,
			Name:     "pause heartbeat for 500ms returns paused true and correct duration",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatHeartbeat)}
				result, err := heartbeatPauseRPC(ctx, 500)
				if err != nil {
					return tr, err
				}
				if result["paused"] != true {
					return tr, fmt.Errorf("expected paused true, got %v", result["paused"])
				}
				pauseMs, ok := result["pauseMs"].(float64)
				if !ok || pauseMs != 500 {
					return tr, fmt.Errorf("expected pauseMs 500, got %v", result["pauseMs"])
				}
				duration, ok := result["duration"].(string)
				if !ok || duration != "500ms" {
					return tr, fmt.Errorf("expected duration 500ms, got %v", result["duration"])
				}
				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "HB-002",
			Category: conformance.CatHeartbeat,
			Name:     "resume heartbeat after pause returns resumed true and wasPaused true",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatHeartbeat)}
				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}
				h := conformance.NewPseudoHostForBinary(binary)
				if err := h.Start(ctx); err != nil {
					return tr, fmt.Errorf("start host: %w", err)
				}
				defer h.Kill()

				pausePayload := conformance.MustMarshal(map[string]any{"pauseMs": 2000})
				_, err := h.CallRPC(ctx, "mock.fault", "mock.fault.heartbeat.pause", pausePayload)
				if err != nil {
					return tr, fmt.Errorf("heartbeat.pause rpc: %w", err)
				}

				resumeResp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.heartbeat.resume", conformance.MustMarshal(map[string]any{}))
				if err != nil {
					return tr, fmt.Errorf("heartbeat.resume rpc: %w", err)
				}
				var resumeResult map[string]any
				if err := json.Unmarshal(resumeResp.Payload, &resumeResult); err != nil {
					return tr, fmt.Errorf("unmarshal resume response: %w", err)
				}
				if resumeResult["resumed"] != true {
					return tr, fmt.Errorf("expected resumed true, got %v", resumeResult["resumed"])
				}
				if resumeResult["wasPaused"] != true {
					return tr, fmt.Errorf("expected wasPaused true, got %v", resumeResult["wasPaused"])
				}
				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "HB-003",
			Category: conformance.CatHeartbeat,
			Name:     "drop heartbeat with count 5 returns configured true and correct dropCount",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatHeartbeat)}
				result, err := heartbeatDropRPC(ctx, 5)
				if err != nil {
					return tr, err
				}
				if result["configured"] != true {
					return tr, fmt.Errorf("expected configured true, got %v", result["configured"])
				}
				dropCount, ok := result["dropCount"].(float64)
				if !ok || dropCount != 5 {
					return tr, fmt.Errorf("expected dropCount 5, got %v", result["dropCount"])
				}
				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "HB-004",
			Category: conformance.CatHeartbeat,
			Name:     "pause heartbeat indefinite with pauseMs 0 returns duration indefinite",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatHeartbeat)}
				result, err := heartbeatPauseRPC(ctx, 0)
				if err != nil {
					return tr, err
				}
				if result["paused"] != true {
					return tr, fmt.Errorf("expected paused true, got %v", result["paused"])
				}
				duration, ok := result["duration"].(string)
				if !ok || duration != "indefinite" {
					return tr, fmt.Errorf("expected duration indefinite, got %v", result["duration"])
				}
				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "HB-005",
			Category: conformance.CatHeartbeat,
			Name:     "drop heartbeat with count 0 returns configured true and dropCount 0",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatHeartbeat)}
				result, err := heartbeatDropRPC(ctx, 0)
				if err != nil {
					return tr, err
				}
				if result["configured"] != true {
					return tr, fmt.Errorf("expected configured true, got %v", result["configured"])
				}
				dropCount, ok := result["dropCount"].(float64)
				if !ok || dropCount != 0 {
					return tr, fmt.Errorf("expected dropCount 0, got %v", result["dropCount"])
				}
				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "HB-006",
			Category: conformance.CatHeartbeat,
			Name:     "pause then resume then pause again works correctly",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatHeartbeat)}
				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}
				h := conformance.NewPseudoHostForBinary(binary)
				if err := h.Start(ctx); err != nil {
					return tr, fmt.Errorf("start host: %w", err)
				}
				defer h.Kill()

				pausePayload := conformance.MustMarshal(map[string]any{"pauseMs": 1000})
				pauseResp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.heartbeat.pause", pausePayload)
				if err != nil {
					return tr, fmt.Errorf("first pause rpc: %w", err)
				}
				var pauseResult map[string]any
				if err := json.Unmarshal(pauseResp.Payload, &pauseResult); err != nil {
					return tr, fmt.Errorf("unmarshal pause response: %w", err)
				}
				if pauseResult["paused"] != true {
					return tr, fmt.Errorf("expected first pause to return paused true")
				}

				resumeResp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.heartbeat.resume", conformance.MustMarshal(map[string]any{}))
				if err != nil {
					return tr, fmt.Errorf("resume rpc: %w", err)
				}
				var resumeResult map[string]any
				if err := json.Unmarshal(resumeResp.Payload, &resumeResult); err != nil {
					return tr, fmt.Errorf("unmarshal resume response: %w", err)
				}
				if resumeResult["resumed"] != true {
					return tr, fmt.Errorf("expected resumed true")
				}

				pauseResp2, err := h.CallRPC(ctx, "mock.fault", "mock.fault.heartbeat.pause", conformance.MustMarshal(map[string]any{"pauseMs": 2000}))
				if err != nil {
					return tr, fmt.Errorf("second pause rpc: %w", err)
				}
				var pauseResult2 map[string]any
				if err := json.Unmarshal(pauseResp2.Payload, &pauseResult2); err != nil {
					return tr, fmt.Errorf("unmarshal second pause response: %w", err)
				}
				if pauseResult2["paused"] != true {
					return tr, fmt.Errorf("expected second pause to return paused true")
				}
				pauseMs, ok := pauseResult2["pauseMs"].(float64)
				if !ok || pauseMs != 2000 {
					return tr, fmt.Errorf("expected second pauseMs 2000, got %v", pauseResult2["pauseMs"])
				}
				tr.Passed = true
				return tr, nil
			},
		},
	}

	conformance.RunMatrix(t, conformance.CatHeartbeat, cases)
}

func heartbeatPauseRPC(ctx context.Context, pauseMs int) (map[string]any, error) {
	binary := conformance.FindPluginBinary()
	if binary == "" {
		return nil, fmt.Errorf("plugin binary not found")
	}

	h := conformance.NewPseudoHostForBinary(binary)
	if err := h.Start(ctx); err != nil {
		return nil, fmt.Errorf("start host: %w", err)
	}
	defer h.Kill()

	payload := conformance.MustMarshal(map[string]any{"pauseMs": pauseMs})
	resp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.heartbeat.pause", payload)
	if err != nil {
		return nil, fmt.Errorf("heartbeat.pause rpc: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Payload, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return result, nil
}

func heartbeatDropRPC(ctx context.Context, count int) (map[string]any, error) {
	binary := conformance.FindPluginBinary()
	if binary == "" {
		return nil, fmt.Errorf("plugin binary not found")
	}

	h := conformance.NewPseudoHostForBinary(binary)
	if err := h.Start(ctx); err != nil {
		return nil, fmt.Errorf("start host: %w", err)
	}
	defer h.Kill()

	payload := conformance.MustMarshal(map[string]any{"count": count})
	resp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.heartbeat.drop", payload)
	if err != nil {
		return nil, fmt.Errorf("heartbeat.drop rpc: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Payload, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return result, nil
}
