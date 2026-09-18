package matrices

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/u-ai/mock-conformance-go"
)

func TestHandshake_Suite(t *testing.T) {
	cases := []conformance.TestCase{
		{
			ID:       "HS-001",
			Category: conformance.CatHandshake,
			Name:     "configure handshake delay 200ms dropCount 0 returns configured true",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatHandshake)}
				result, err := handshakeRPC(ctx, 200, 0)
				if err != nil {
					return tr, err
				}
				if result["configured"] != true {
					return tr, fmt.Errorf("expected configured true, got %v", result["configured"])
				}
				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "HS-002",
			Category: conformance.CatHandshake,
			Name:     "configure handshake delay 0ms dropCount 3 returns configured true",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatHandshake)}
				result, err := handshakeRPC(ctx, 0, 3)
				if err != nil {
					return tr, err
				}
				if result["configured"] != true {
					return tr, fmt.Errorf("expected configured true, got %v", result["configured"])
				}
				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "HS-003",
			Category: conformance.CatHandshake,
			Name:     "configure handshake delay 500ms dropCount 1 returns delayMs 500 dropCount 1",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatHandshake)}
				result, err := handshakeRPC(ctx, 500, 1)
				if err != nil {
					return tr, err
				}
				delayMs, ok := result["delayMs"].(float64)
				if !ok || delayMs != 500 {
					return tr, fmt.Errorf("expected delayMs 500, got %v", result["delayMs"])
				}
				dropCount, ok := result["dropCount"].(float64)
				if !ok || dropCount != 1 {
					return tr, fmt.Errorf("expected dropCount 1, got %v", result["dropCount"])
				}
				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "HS-004",
			Category: conformance.CatHandshake,
			Name:     "configure handshake delay 0ms dropCount 0 returns zero values",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatHandshake)}
				result, err := handshakeRPC(ctx, 0, 0)
				if err != nil {
					return tr, err
				}
				delayMs, ok := result["delayMs"].(float64)
				if !ok || delayMs != 0 {
					return tr, fmt.Errorf("expected delayMs 0, got %v", result["delayMs"])
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
			ID:       "HS-005",
			Category: conformance.CatHandshake,
			Name:     "configure handshake dropCount 10 then verify subsequent config overrides to dropCount 5",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatHandshake)}
				result, err := handshakeRPC(ctx, 100, 10)
				if err != nil {
					return tr, err
				}
				drop1, ok := result["dropCount"].(float64)
				if !ok || drop1 != 10 {
					return tr, fmt.Errorf("expected dropCount 10, got %v", result["dropCount"])
				}
				result2, err := handshakeRPC(ctx, 50, 5)
				if err != nil {
					return tr, err
				}
				drop2, ok := result2["dropCount"].(float64)
				if !ok || drop2 != 5 {
					return tr, fmt.Errorf("expected dropCount 5 on override, got %v", result2["dropCount"])
				}
				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "HS-006",
			Category: conformance.CatHandshake,
			Name:     "configure handshake delay 1000ms returns correct response structure",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatHandshake)}
				result, err := handshakeRPC(ctx, 1000, 2)
				if err != nil {
					return tr, err
				}
				if result["configured"] != true {
					return tr, fmt.Errorf("expected configured true, got %v", result["configured"])
				}
				delayMs, ok := result["delayMs"].(float64)
				if !ok || delayMs != 1000 {
					return tr, fmt.Errorf("expected delayMs 1000, got %v", result["delayMs"])
				}
				dropCount, ok := result["dropCount"].(float64)
				if !ok || dropCount != 2 {
					return tr, fmt.Errorf("expected dropCount 2, got %v", result["dropCount"])
				}
				tr.Passed = true
				return tr, nil
			},
		},
	}

	conformance.RunMatrix(t, conformance.CatHandshake, cases)
}

func handshakeRPC(ctx context.Context, delayMs int, dropCount int) (map[string]any, error) {
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
		"delayMs":   delayMs,
		"dropCount": dropCount,
	})

	resp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.handshake.delay", payload)
	if err != nil {
		return nil, fmt.Errorf("handshake.delay rpc: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Payload, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return result, nil
}
