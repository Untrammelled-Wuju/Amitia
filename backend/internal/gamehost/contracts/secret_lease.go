package contracts

import (
	kernelsecret "github.com/u-ai/backend/internal/extension/kernel/secret"
)

type RuntimeSecretLeaseSession struct {
	SessionID  string
	RuntimeID  string
	ServiceID  string
	Generation int64
	LeaseIDs   []kernelsecret.LeaseID
}
