package acquisition

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

// MissingCapabilityDetector inspects errors and resolution failures to determine
// whether a missing-capability condition occurred and produces the resume context
// needed for automatic recovery.
type MissingCapabilityDetector interface {
	DetectFromError(ctx context.Context, err error, invocation capability.ToolInvocationContext) (*CapabilityResumeContext, error)
	DetectFromResolution(ctx context.Context, failure capability.ResolutionFailure, invocation capability.ToolInvocationContext) (*CapabilityResumeContext, error)
}

type defaultMissingDetector struct{}

// NewMissingCapabilityDetector returns the default MissingCapabilityDetector.
func NewMissingCapabilityDetector() MissingCapabilityDetector {
	return &defaultMissingDetector{}
}

// DetectFromError inspects an execution error and, when it wraps a
// MissingCapabilityError, returns a CapabilityResumeContext describing the
// missing capability and resuming in the ResumePending state.
func (d *defaultMissingDetector) DetectFromError(ctx context.Context, err error, invocation capability.ToolInvocationContext) (*CapabilityResumeContext, error) {
	if err == nil {
		return nil, nil
	}

	var missingErr MissingCapabilityError
	if errors.As(err, &missingErr) {
		return &CapabilityResumeContext{
			ConversationID: conversationIDFromContext(ctx),
			CapabilityID:   missingErr.CapabilityID,
			State:          ResumePending,
		}, nil
	}

	return nil, nil
}

// DetectFromResolution inspects a capability resolution failure and, when the
// failure is actionable (i.e. not ResolutionFailureNone and recoverable), returns a
// CapabilityResumeContext in the ResumePending state.
// Non-recoverable failures (device_offline, credential_required, permission_denied)
// do NOT trigger recovery to avoid repeated install attempts.
func (d *defaultMissingDetector) DetectFromResolution(ctx context.Context, failure capability.ResolutionFailure, invocation capability.ToolInvocationContext) (*CapabilityResumeContext, error) {
	if failure == capability.ResolutionFailureNone {
		return nil, nil
	}

	// Non-recoverable failures: do NOT trigger recovery
	switch failure {
	case capability.ResolutionFailureDeviceOffline,
		capability.ResolutionFailureDeviceUnavailable,
		capability.ResolutionFailureRuntimeUnavailable:
		return nil, nil
	}

	capID := capability.CapabilityID("")
	if invocation.Metadata != nil {
		if v, ok := invocation.Metadata["capabilityId"]; ok {
			if s, ok := v.(string); ok {
				capID = capability.CapabilityID(s)
			}
		}
	}

	return &CapabilityResumeContext{
		ConversationID: conversationIDFromContext(ctx),
		CapabilityID:   capID,
		State:          ResumePending,
	}, nil
}

// conversationIDFromContext extracts the ConversationID from the supplied
// context. It first checks for the "conversation_id" context value, then falls
// back to "conversationId". It returns an empty string when the value is absent
// or not a string.
func conversationIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value("conversation_id").(string); ok && v != "" {
		return v
	}
	if v, ok := ctx.Value("conversationId").(string); ok && v != "" {
		return v
	}
	return ""
}

// --- Resume Helpers ---

// GenerateResumeToken produces a unique resume token for a capability candidate
// and acquisition request. The format is: acq_{base64url(sha256(timestamp:capID:candidateID))[:16]}
func GenerateResumeToken(candidate CapabilityCandidate, request AcquisitionRequest) string {
	data := fmt.Sprintf("%d:%s:%s", time.Now().UnixNano(), request.CapabilityID, candidate.ID)
	h := sha256.Sum256([]byte(data))
	return "acq_" + base64.RawURLEncoding.EncodeToString(h[:])[:16]
}

// ValidateResumeToken checks whether a resume token has the expected "acq_"
// prefix and minimum length.
func ValidateResumeToken(token string) bool {
	if len(token) < 5 || token[:4] != "acq_" {
		return false
	}
	return true
}

// GenerateIdempotencyKey produces a deterministic idempotency key for a given
// acquisition request and candidate. The format is:
// base64url(sha256(capabilityId|userID|candidateID))
func GenerateIdempotencyKey(request AcquisitionRequest, candidateID string) string {
	data := string(request.CapabilityID) + "|" + string(request.UserID) + "|" + candidateID
	h := sha256.Sum256([]byte(data))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
