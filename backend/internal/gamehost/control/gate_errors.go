package control

import (
	"github.com/u-ai/backend/internal/gamehost/domain"
)

func errOutputInvalidPeer(identity TrustedPluginIdentity) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrPermissionDenied,
		Message: "control output invalid trusted peer identity",
	}
}

func errOutputRuntimeMismatch(peerRuntime domain.RuntimeInstanceID, intentRuntime domain.RuntimeInstanceID) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrPermissionDenied,
		Message: "runtime mismatch between trusted peer and output intent",
	}
}

func errOutputServiceMismatch(peerService domain.ServiceID, intentService domain.ServiceID) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrPermissionDenied,
		Message: "service mismatch between trusted peer and output intent",
	}
}

func errOutputRuntimeNotFound(runtimeID domain.RuntimeInstanceID) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrNotFound,
		Message: "control output runtime not registered: " + string(runtimeID),
	}
}

func errOutputServiceNotFound(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrNotFound,
		Message: "service not found in runtime: " + string(runtimeID) + "/" + string(serviceID),
	}
}

func errOutputRuntimeNotEligible(runtimeID domain.RuntimeInstanceID, state string) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrRuntimeUnavailable,
		Message: "runtime not eligible for control output: " + string(runtimeID) + " state=" + state,
	}
}

func errOutputRuntimeNotReady(runtimeID domain.RuntimeInstanceID) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrRuntimeUnavailable,
		Message: "runtime not ready for control output: " + string(runtimeID),
	}
}

func errOutputPermissionDenied(runtimeID domain.RuntimeInstanceID) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrPermissionDenied,
		Message: "control output permission denied for runtime: " + string(runtimeID),
	}
}

func errOutputAuthorityModeDenied(mode domain.ControlMode) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrPermissionDenied,
		Message: "control output blocked by authority mode: " + string(mode),
	}
}

func errOutputStaleEpoch(intentEpoch uint64, currentEpoch uint64) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrInvalidState,
		Message: "control output stale epoch",
	}
}

func errOutputHostPolicyDenied(runtimeID domain.RuntimeInstanceID) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrPermissionDenied,
		Message: "control output denied by host policy for runtime: " + string(runtimeID),
	}
}

func errOutputGateClosed(runtimeID domain.RuntimeInstanceID) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrRuntimeUnavailable,
		Message: "control output gate closed for runtime: " + string(runtimeID),
	}
}

func errOutputGeneration(staleGeneration uint64, currentGeneration uint64) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrInvalidState,
		Message: "stale process/connection generation",
	}
}
