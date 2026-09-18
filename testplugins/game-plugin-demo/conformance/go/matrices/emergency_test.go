package matrices

import (
	"context"
	"encoding/json"
	"testing"

	conformance "github.com/u-ai/mock-conformance-go"
)

func TestEmergency_Suite(t *testing.T) {
	cases := []conformance.TestCase{
		{
			ID:       "emergency_response_received",
			Category: conformance.CatEmergency,
			Name:     "Emergency response returns received true with operationId",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "control.emergency_response", conformance.MustMarshal(map[string]any{"operationId": "op-001", "reason": "fire"}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					received, ok := resp["received"].(bool)
					if !ok || !received {
						tr.Error = "expected received to be true"
						return nil
					}
					operationId, ok := resp["operationId"].(string)
					if !ok || operationId != "op-001" {
						tr.Error = "expected operationId to be 'op-001'"
						return nil
					}
					tr.Passed = true
					return nil
				})
				if err != nil {
					tr.Passed = false
					tr.Error = err.Error()
				}
				return tr, nil
			},
		},
		{
			ID:       "emergency_response_acknowledged",
			Category: conformance.CatEmergency,
			Name:     "Emergency response returns acknowledged true",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "control.emergency_response", conformance.MustMarshal(map[string]any{"operationId": "op-002", "reason": "intrusion"}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					acknowledged, ok := resp["acknowledged"].(bool)
					if !ok || !acknowledged {
						tr.Error = "expected acknowledged to be true"
						return nil
					}
					tr.Passed = true
					return nil
				})
				if err != nil {
					tr.Passed = false
					tr.Error = err.Error()
				}
				return tr, nil
			},
		},
		{
			ID:       "emergency_response_operation_id_echo",
			Category: conformance.CatEmergency,
			Name:     "Emergency response echoes the provided operationId",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "control.emergency_response", conformance.MustMarshal(map[string]any{"operationId": "test-echo-id-42", "reason": ""}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					operationId, ok := resp["operationId"].(string)
					if !ok || operationId != "test-echo-id-42" {
						tr.Error = "expected operationId to be 'test-echo-id-42'"
						return nil
					}
					tr.Passed = true
					return nil
				})
				if err != nil {
					tr.Passed = false
					tr.Error = err.Error()
				}
				return tr, nil
			},
		},
		{
			ID:       "emergency_response_has_all_keys",
			Category: conformance.CatEmergency,
			Name:     "Emergency response contains received, operationId, and acknowledged keys",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "control.emergency_response", conformance.MustMarshal(map[string]any{"operationId": "op-keys", "reason": "verify"}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					if _, ok := resp["received"]; !ok {
						tr.Error = "missing received key"
						return nil
					}
					if _, ok := resp["operationId"]; !ok {
						tr.Error = "missing operationId key"
						return nil
					}
					if _, ok := resp["acknowledged"]; !ok {
						tr.Error = "missing acknowledged key"
						return nil
					}
					tr.Passed = true
					return nil
				})
				if err != nil {
					tr.Passed = false
					tr.Error = err.Error()
				}
				return tr, nil
			},
		},
		{
			ID:       "emergency_response_multiple_operations",
			Category: conformance.CatEmergency,
			Name:     "Emergency response handles multiple distinct operationIds correctly",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					ids := []string{"op-alpha", "op-beta", "op-gamma"}
					for _, id := range ids {
						env, err := h.CallRPC(ctx, "mock.fault", "control.emergency_response", conformance.MustMarshal(map[string]any{"operationId": id, "reason": "multi"}))
						if err != nil {
							tr.Error = "rpc error: " + err.Error()
							return nil
						}
						var resp map[string]any
						if err := json.Unmarshal(env.Payload, &resp); err != nil {
							tr.Error = "unmarshal: " + err.Error()
							return nil
						}
						operationId, ok := resp["operationId"].(string)
						if !ok || operationId != id {
							tr.Error = "operationId mismatch for " + id
							return nil
						}
						received, ok := resp["received"].(bool)
						if !ok || !received {
							tr.Error = "received not true for " + id
							return nil
						}
					}
					tr.Passed = true
					return nil
				})
				if err != nil {
					tr.Passed = false
					tr.Error = err.Error()
				}
				return tr, nil
			},
		},
		{
			ID:       "emergency_response_received_type",
			Category: conformance.CatEmergency,
			Name:     "Emergency response received field is boolean type",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "control.emergency_response", conformance.MustMarshal(map[string]any{"operationId": "op-type", "reason": ""}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					received := resp["received"]
					if received == nil {
						tr.Error = "received is nil"
						return nil
					}
					_, isBool := received.(bool)
					if !isBool {
						tr.Error = "received is not bool type"
						return nil
					}
					tr.Passed = true
					return nil
				})
				if err != nil {
					tr.Passed = false
					tr.Error = err.Error()
				}
				return tr, nil
			},
		},
	}
	conformance.RunMatrix(t, conformance.CatEmergency, cases)
}
