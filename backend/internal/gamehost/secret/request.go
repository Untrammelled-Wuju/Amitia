package secret

import "github.com/u-ai/backend/internal/extension/kernel/secret"

type Purpose string

const (
	PurposeStartup Purpose = "startup"
	PurposeRuntime Purpose = "runtime"
)

type LeaseRequest struct {
	RuntimeID       string
	ServiceID       string
	SecretRef       secret.SecretRef
	Purpose         Purpose
	Required        bool
	TTLSeconds      int
	MaxUses         int
	Generation      int64
	PermissionToken string
}

type LeaseResult struct {
	LeaseID   secret.LeaseID
	Ref       secret.SecretRef
	Purpose   Purpose
	ExpiresAt int64
}

type SecretAcquireRequest struct {
	RuntimeID  string
	PluginID   string
	ServiceID  string
	Ref        secret.SecretRef
	Purpose    Purpose
	Required   bool
	Generation int64
}

type SecretAcquireResult struct {
	LeaseID   secret.LeaseID
	Ref       secret.SecretRef
	Purpose   Purpose
	ExpiresAt int64
	Granted   bool
	Reason    string
}
