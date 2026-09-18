package matrices

import (
	"context"
	"fmt"
	"testing"

	"github.com/u-ai/mock-conformance-go"
)

func TestProcessCrash_Suite(t *testing.T) {
	cases := []conformance.TestCase{
		{
			ID:       "CRASH-001",
			Category: conformance.CatProcessCrash,
			Name:     "crash with exitCode 1 and no delay exits with code 1",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatProcessCrash)}
				if err := runCrash(ctx, 1, 0); err != nil {
					return tr, err
				}
				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "CRASH-002",
			Category: conformance.CatProcessCrash,
			Name:     "crash with exitCode 42 exits with code 42",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatProcessCrash)}
				if err := runCrash(ctx, 42, 0); err != nil {
					return tr, err
				}
				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "CRASH-003",
			Category: conformance.CatProcessCrash,
			Name:     "crash with exitCode 0 defaults to exitCode 1",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatProcessCrash)}
				if err := runCrash(ctx, 0, 0); err != nil {
					return tr, err
				}
				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "CRASH-004",
			Category: conformance.CatProcessCrash,
			Name:     "crash with delayMs 100 and exitCode 7 exits with code 7",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatProcessCrash)}
				if err := runCrash(ctx, 7, 100); err != nil {
					return tr, err
				}
				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "CRASH-005",
			Category: conformance.CatProcessCrash,
			Name:     "crash with exitCode 255 exits with code 255",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatProcessCrash)}
				if err := runCrash(ctx, 255, 0); err != nil {
					return tr, err
				}
				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "CRASH-006",
			Category: conformance.CatProcessCrash,
			Name:     "crash with delayMs 50 and exitCode 3 exits with code 3",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatProcessCrash)}
				if err := runCrash(ctx, 3, 50); err != nil {
					return tr, err
				}
				tr.Passed = true
				return tr, nil
			},
		},
	}

	conformance.RunMatrix(t, conformance.CatProcessCrash, cases)
}

func runCrash(ctx context.Context, exitCode int, delayMs int) error {
	binary := conformance.FindPluginBinary()
	if binary == "" {
		return fmt.Errorf("plugin binary not found")
	}

	h := conformance.NewPseudoHostForBinary(binary)
	if err := h.Start(ctx); err != nil {
		return fmt.Errorf("start host: %w", err)
	}

	crashPayload := conformance.MustMarshal(map[string]any{
		"exitCode": exitCode,
		"delayMs":  delayMs,
	})

	go func() {
		_, _ = h.CallRPC(ctx, "mock.fault", "mock.fault.crash", crashPayload)
	}()

	h.WaitExit()

	actualExitCode := h.ExitCode()
	if actualExitCode == -1 {
		return fmt.Errorf("could not determine exit code")
	}

	expectedExitCode := exitCode
	if expectedExitCode == 0 {
		expectedExitCode = 1
	}

	if actualExitCode != expectedExitCode {
		return fmt.Errorf("expected exit code %d, got %d", expectedExitCode, actualExitCode)
	}

	return nil
}
