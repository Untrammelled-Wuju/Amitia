package interaction

import (
	"context"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/androidnative/uitree"
)

type Verifier interface {
	Verify(
		ctx context.Context,
		before InteractionContext,
		result InteractionResult,
	) (VerificationResult, error)
}

type DefaultVerifier struct {
	snapshotResolver uitree.SnapshotResolver
	screenshot       ScreenshotProvider
	policy           Policy
}

func NewDefaultVerifier(
	snapshotResolver uitree.SnapshotResolver,
	screenshot ScreenshotProvider,
	policy Policy,
) *DefaultVerifier {
	return &DefaultVerifier{
		snapshotResolver: snapshotResolver,
		screenshot:       screenshot,
		policy:           policy,
	}
}

func (v *DefaultVerifier) Verify(
	ctx context.Context,
	before InteractionContext,
	result InteractionResult,
) (VerificationResult, error) {
	verifyCtx, cancel := context.WithTimeout(ctx, DefaultVerificationTimeoutMS*time.Millisecond)
	defer cancel()

	if result.Strategy == StrategyAccessibilityAction || result.Strategy == StrategyNodeBounds || result.Strategy == StrategyCoordinate {
		return v.verifyByUITree(verifyCtx, before, result)
	}

	if result.Strategy == StrategyVisualOCR || result.Strategy == StrategyVisualUnderstand {
		return v.verifyByScreenshot(verifyCtx, before, result)
	}

	return VerificationResult{Verified: false, Reason: "unsupported strategy for verification"}, nil
}

func (v *DefaultVerifier) verifyByUITree(
	ctx context.Context,
	before InteractionContext,
	result InteractionResult,
) (VerificationResult, error) {
	if v.snapshotResolver == nil {
		return VerificationResult{Verified: false, Reason: "snapshot resolver not available"}, nil
	}
	if result.SnapshotID == "" {
		return VerificationResult{Verified: false, Reason: "no baseline snapshot ID in result"}, nil
	}

	baseline, err := v.snapshotResolver.GetSnapshot(ctx, result.SnapshotID)
	if err != nil {
		return VerificationResult{Verified: false, Reason: "failed to get baseline snapshot: " + err.Error()}, nil
	}
	after, err := v.snapshotResolver.Latest(ctx)
	if err != nil {
		return VerificationResult{Verified: false, Reason: "failed to get latest snapshot: " + err.Error()}, nil
	}
	changed := after.SnapshotID != baseline.SnapshotID || after.Generation > baseline.Generation
	if changed {
		return VerificationResult{
			Verified: true,
			Method:   "ui_tree_changed",
			Changed:  true,
			Reason:   "UI tree snapshot changed after action",
		}, nil
	}
	return VerificationResult{
		Verified: false,
		Method:   "ui_tree_unchanged",
		Changed:  false,
		Reason:   "UI tree snapshot unchanged after action",
	}, nil
}

func (v *DefaultVerifier) verifyByScreenshot(
	ctx context.Context,
	before InteractionContext,
	result InteractionResult,
) (VerificationResult, error) {
	if v.screenshot == nil {
		return VerificationResult{Verified: false, Reason: "screenshot provider not available"}, nil
	}
	after, err := v.screenshot.Capture(ctx, result.DisplayID)
	if err != nil {
		return VerificationResult{Verified: false, Reason: "failed to capture after screenshot: " + err.Error()}, nil
	}
	if strings.TrimSpace(result.BaselineScreenStateToken) == "" {
		return VerificationResult{Verified: false, Method: "screen_state_baseline_missing", Reason: "no pre-action screen state token"}, nil
	}
	changed := strings.TrimSpace(after.StateToken) != "" && after.StateToken != result.BaselineScreenStateToken
	if changed {
		return VerificationResult{Verified: true, Method: "screen_state_changed", Changed: true, Reason: "screen state changed after action"}, nil
	}
	return VerificationResult{Verified: false, Method: "screen_state_unchanged", Changed: false, Reason: "screen state unchanged after action"}, nil
}
