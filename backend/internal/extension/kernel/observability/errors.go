package observability

import "errors"

var (
	ErrInvocationNotFound = errors.New("observability: invocation not found")
	ErrOperationNotFound  = errors.New("observability: operation not found")
	ErrTraceNotFound      = errors.New("observability: trace not found")
	ErrAttemptNotFound    = errors.New("observability: attempt not found")
	ErrAuditWriteRequired = errors.New("observability: high-risk audit write required but failed")
	ErrStorageUnavailable = errors.New("observability: storage backend unavailable")
	ErrRetentionViolation = errors.New("observability: retention policy violation")
)
