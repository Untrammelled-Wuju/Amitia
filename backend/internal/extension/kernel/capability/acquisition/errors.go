package acquisition

import (
	"errors"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

var (
	ErrCapabilityAlreadyAvailable = errors.New("acquisition: capability already available and executable")
	ErrCapabilityRegisteredNotExecutable = errors.New("acquisition: capability registered but not executable")
	ErrNoCandidate           = errors.New("acquisition: no candidate found")
	ErrNoCompatibleCandidate  = errors.New("acquisition: no compatible candidate found")
	ErrCandidateBlocked      = errors.New("acquisition: candidate blocked by policy")
	ErrCandidateUntrusted    = errors.New("acquisition: candidate untrusted and not auto-installable")
	ErrPermissionRequired    = errors.New("acquisition: permission required")
	ErrApprovalRequired      = errors.New("acquisition: user approval required")
	ErrTargetUnavailable     = errors.New("acquisition: deployment target unavailable")
	ErrTargetRuntimeUnavailable = errors.New("acquisition: target runtime unavailable")
	ErrInstallFailed         = errors.New("acquisition: install failed")
	ErrEnableFailed          = errors.New("acquisition: enable failed")
	ErrReconcileFailed       = errors.New("acquisition: provider reconcile failed")
	ErrCapabilityStillUnavailable = errors.New("acquisition: capability still unavailable after acquisition")
	ErrResumeContextMissing  = errors.New("acquisition: resume context missing")
	ErrAcquisitionCancelled  = errors.New("acquisition: cancelled")

	ErrInstallerRegistryUnavailable = errors.New("acquisition: installer registry unavailable")
	ErrNoInstallerForMethod         = errors.New("acquisition: no installer registered for install method")
	ErrProviderRegistryUnavailable  = errors.New("acquisition: provider registry unavailable")
	ErrProviderDefinitionNotFound   = errors.New("acquisition: provider definition not found for capability")
	ErrToolRegistryNil              = errors.New("acquisition: tool registry is nil")
	ErrRuntimeNotLive               = errors.New("acquisition: device runtime not live")
	ErrReconcileTimeout             = errors.New("acquisition: reconcile timed out waiting for executable provider")
)

type AcquisitionError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Cause   error  `json:"-"`
}

func (e AcquisitionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e AcquisitionError) Unwrap() error {
	return e.Cause
}

func NewAcquisitionError(code string, msg string, cause error) AcquisitionError {
	return AcquisitionError{Code: code, Message: msg, Cause: cause}
}

type MissingCapabilityError struct {
	CapabilityID capability.CapabilityID
	Description  string
	Recoverable  bool
}

func (e MissingCapabilityError) Error() string {
	return fmt.Sprintf("missing capability: %s (%s)", e.CapabilityID, e.Description)
}
