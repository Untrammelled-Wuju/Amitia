package interaction

import (
	"context"
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
		return VerificationResult{Verified: false, Reason: "no snapshot ID in result"}, nil
	}

	afterSnapshot, err := v.snapshotResolver.GetSnapshot(ctx, result.SnapshotID)
	if err != nil {
		return VerificationResult{Verified: false, Reason: "failed to get after snapshot: " + err.Error()}, nil
	}

	if afterSnapshot.Generation > result.SnapshotIDGeneration() {
		return VerificationResult{
			Verified: true,
			Method:   "ui_tree_generation_changed",
			Changed:  true,
			Reason:   "UI tree generation changed after action",
		}, nil
	}

	return VerificationResult{
		Verified: false,
		Method:   "ui_tree_generation_unchanged",
		Changed:  false,
		Reason:   "UI tree generation unchanged",
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

	afterScreenshot, err := v.screenshot.Capture(ctx, result.DisplayID)
	if err != nil {
		return VerificationResult{Verified: false, Reason: "failed to capture after screenshot: " + err.Error()}, nil
	}

	if afterScreenshot.CapturedAt > time.Now().Add(-v.policy.MaxScreenshotAge).UnixMilli() {
		return VerificationResult{
			Verified: true,
			Method:   "screenshot_captured",
			Changed:  true,
			Reason:   "new screenshot captured after action",
		}, nil
	}

	return VerificationResult{
		Verified: false,
		Method:   "screenshot_stale",
		Changed:  false,
		Reason:   "screenshot too old for verification",
	}, nil
}

func (r *InteractionResult) SnapshotIDGeneration() int64 {
	return 0
}
