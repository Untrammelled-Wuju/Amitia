package accountsession

import "errors"

var (
	ErrInvalidCredentials  = errors.New("auth.invalid_credentials")
	ErrLoginRateLimited    = errors.New("auth.login_rate_limited")
	ErrAccessExpired       = errors.New("auth.access_expired")
	ErrAccessInvalid       = errors.New("auth.access_invalid")
	ErrSessionNotFound     = errors.New("auth.session_not_found")
	ErrSessionRevoked      = errors.New("auth.session_revoked")
	ErrSessionExpired      = errors.New("auth.session_expired")
	ErrRefreshInvalid      = errors.New("auth.refresh_invalid")
	ErrRefreshExpired      = errors.New("auth.refresh_expired")
	ErrRefreshReused       = errors.New("auth.refresh_reused")
	ErrRefreshRevoked      = errors.New("auth.refresh_revoked")
	ErrReauthRequired      = errors.New("auth.reauth_required")
	ErrRecoveryInvalid     = errors.New("auth.recovery_invalid")
	ErrRecoveryUsed        = errors.New("auth.recovery_used")
	ErrRecoveryExpired     = errors.New("auth.recovery_expired")
	ErrUserInactive        = errors.New("auth.user_inactive")
	ErrMaxSessionsReached  = errors.New("auth.max_sessions_reached")
	ErrSessionOwnership    = errors.New("auth.session_ownership")
	ErrInvalidClientType   = errors.New("auth.invalid_client_type")
)
