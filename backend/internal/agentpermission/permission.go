package agentpermission

import "strings"

const (
	RequestApproval = "request_approval"
	FullAccess      = "full_access"
)

func Normalize(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case FullAccess:
		return FullAccess
	default:
		return RequestApproval
	}
}

func Valid(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case RequestApproval, FullAccess:
		return true
	default:
		return false
	}
}
