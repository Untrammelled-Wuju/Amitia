package matrices

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/u-ai/mock-conformance-go"
)

func TestRPCTimeout_Suite(t *testing.T) {
	cases := []conformance.TestCase{
		{
			ID:       "RPC-001",
			Category: conformance.CatRPCTimeout,
			Name:     "slow response with chunkCount 1 chunkDelayMs 0 returns single chunk",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatRPCTimeout)}
				result, err := slowResponseRPC(ctx, 1, 0)
				if err != nil {
					return tr, err
				}
				if result["slow"] != true {
					return tr, fmt.Errorf("expected slow true, got %v", result["slow"])
				}
				chunkCount, ok := result["chunkCount"].(float64)
				if !ok || chunkCount != 1 {
					return tr, fmt.Errorf("expected chunkCount 1, got %v", result["chunkCount"])
				}
				chunks, ok := result["chunks"].([]any)
				if !ok || len(chunks) != 1 {
					return tr, fmt.Errorf("expected 1 chunk, got %v", result["chunks"])
				}
				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "RPC-002",
			Category: conformance.CatRPCTimeout,
			Name:     "slow response with chunkCount 3 chunkDelayMs 50 returns 3 chunks",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatRPCTimeout)}
				result, err := slowResponseRPC(ctx, 3, 50)
				if err != nil {
					return tr, err
				}
				if result["slow"] != true {
					return tr, fmt.Errorf("expected slow true, got %v", result["slow"])
				}
				chunkCount, ok := result["chunkCount"].(float64)
				if !ok || chunkCount != 3 {
					return tr, fmt.Errorf("expected chunkCount 3, got %v", result["chunkCount"])
				}
				chunks, ok := result["chunks"].([]any)
				if !ok || len(chunks) != 3 {
					return tr, fmt.Errorf("expected 3 chunks, got %v", result["chunks"])
				}
				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "RPC-003",
			Category: conformance.CatRPCTimeout,
			Name:     "slow response with chunkCount 0 returns empty chunks array",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatRPCTimeout)}
				result, err := slowResponseRPC(ctx, 0, 0)
				if err != nil {
					return tr, err
				}
				if result["slow"] != true {
					return tr, fmt.Errorf("expected slow true, got %v", result["slow"])
				}
				chunkCount, ok := result["chunkCount"].(float64)
				if !ok || chunkCount != 0 {
					return tr, fmt.Errorf("expected chunkCount 0, got %v", result["chunkCount"])
				}
				chunks, ok := result["chunks"].([]any)
				if !ok || len(chunks) != 0 {
					return tr, fmt.Errorf("expected 0 chunks, got %v", result["chunks"])
				}
				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "RPC-004",
			Category: conformance.CatRPCTimeout,
			Name:     "stall with stallMs 0 returns stalled true immediately",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatRPCTimeout)}
				result, err := stallRPC(ctx, 0)
				if err != nil {
					return tr, err
				}
				if result["stalled"] != true {
					return tr, fmt.Errorf("expected stalled true, got %v", result["stalled"])
				}
				stallMs, ok := result["stallMs"].(float64)
				if !ok || stallMs != 0 {
					return tr, fmt.Errorf("expected stallMs 0, got %v", result["stallMs"])
				}
				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "RPC-005",
			Category: conformance.CatRPCTimeout,
			Name:     "stall with stallMs 5000 and 3s context timeout triggers rpc timeout error",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatRPCTimeout)}
				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}
				h := conformance.NewPseudoHostForBinary(binary)
				if err := h.Start(ctx); err != nil {
					return tr, fmt.Errorf("start host: %w", err)
				}
				defer h.Kill()

				stallCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
				defer cancel()

				payload := conformance.MustMarshal(map[string]any{"stallMs": 5000})
				_, err := h.CallRPC(stallCtx, "mock.fault", "mock.fault.rpc.stall", payload)
				if err == nil {
					return tr, fmt.Errorf("expected timeout error, got nil")
				}
				if err != context.DeadlineExceeded {
					return tr, fmt.Errorf("expected DeadlineExceeded error, got: %v", err)
				}
				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "RPC-006",
			Category: conformance.CatRPCTimeout,
			Name:     "slow response with chunkCount 5 chunkDelayMs 20 returns correct chunks",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatRPCTimeout)}
				result, err := slowResponseRPC(ctx, 5, 20)
				if err != nil {
					return tr, err
				}
				if result["slow"] != true {
					return tr, fmt.Errorf("expected slow true, got %v", result["slow"])
				}
				chunkCount, ok := result["chunkCount"].(float64)
				if !ok || chunkCount != 5 {
					return tr, fmt.Errorf("expected chunkCount 5, got %v", result["chunkCount"])
				}
				chunks, ok := result["chunks"].([]any)
				if !ok || len(chunks) != 5 {
					return tr, fmt.Errorf("expected 5 chunks, got %v", result["chunks"])
				}
				if chunks[0] != "chunk-0" {
					return tr, fmt.Errorf("expected first chunk-0, got %v", chunks[0])
				}
				if chunks[4] != "chunk-4" {
					return tr, fmt.Errorf("expected last chunk-4, got %v", chunks[4])
				}
				tr.Passed = true
				return tr, nil
			},
		},
	}

	conformance.RunMatrix(t, conformance.CatRPCTimeout, cases)
}

func slowResponseRPC(ctx context.Context, chunkCount int, chunkDelayMs int) (map[string]any, error) {
	binary := conformance.FindPluginBinary()
	if binary == "" {
		return nil, fmt.Errorf("plugin binary not found")
	}

	h := conformance.NewPseudoHostForBinary(binary)
	if err := h.Start(ctx); err != nil {
		return nil, fmt.Errorf("start host: %w", err)
	}
	defer h.Kill()

	payload := conformance.MustMarshal(map[string]any{
		"chunkCount":   chunkCount,
		"chunkDelayMs": chunkDelayMs,
	})

	resp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.rpc.slow_response", payload)
	if err != nil {
		return nil, fmt.Errorf("rpc.slow_response rpc: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Payload, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return result, nil
}

func stallRPC(ctx context.Context, stallMs int) (map[string]any, error) {
	binary := conformance.FindPluginBinary()
	if binary == "" {
		return nil, fmt.Errorf("plugin binary not found")
	}

	h := conformance.NewPseudoHostForBinary(binary)
	if err := h.Start(ctx); err != nil {
		return nil, fmt.Errorf("start host: %w", err)
	}
	defer h.Kill()

	payload := conformance.MustMarshal(map[string]any{"stallMs": stallMs})
	resp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.rpc.stall", payload)
	if err != nil {
		return nil, fmt.Errorf("rpc.stall rpc: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Payload, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return result, nil
}
