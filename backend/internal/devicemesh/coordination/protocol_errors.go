package coordination

import "errors"

var protocolErrors = []struct {
	code string
	err  error
}{
	{"mesh.owned_scope_expired", ErrScopeExpired},
	{"mesh.owned_wrong_owner", ErrWrongOwner},
	{"mesh.owned_request_conflict", ErrRequestConflict},
	{"mesh.owned_resource_version", ErrResourceVersion},
	{"mesh.owned_role_required", ErrRoleRequired},
	{"mesh.owned_role_selection", ErrRoleSelection},
	{"mesh.owned_policy_revision", ErrRevision},
	{"mesh.owned_capability_denied", ErrCapabilityGrant},
	{"mesh.owned_pending_limit", ErrPendingLimit},
}

func ProtocolErrorCode(err error) string {
	for _, entry := range protocolErrors {
		if errors.Is(err, entry.err) {
			return entry.code
		}
	}
	return ""
}

func ErrorFromProtocol(code string) error {
	for _, entry := range protocolErrors {
		if entry.code == code {
			return entry.err
		}
	}
	return nil
}
