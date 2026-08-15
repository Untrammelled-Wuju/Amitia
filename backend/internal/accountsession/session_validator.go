package accountsession

import (
	"time"
)

type UserLookup interface {
	GetUserByID(userID int) (UserAccount, error)
}

type UserAccount struct {
	ID       int
	Username string
	Role     string
	IsActive int
}

type Validator struct {
	sessions SessionRepository
	users    UserLookup
}

func NewValidator(sessions SessionRepository, users UserLookup) *Validator {
	return &Validator{sessions: sessions, users: users}
}

type ValidationResult struct {
	Valid   bool
	Session *Session
	User    *UserAccount
	Reason  error
}

func (v *Validator) ValidateAccessSession(sessionPublicID string, claimsUserID int) ValidationResult {
	session, err := v.sessions.GetByPublicID(sessionPublicID)
	if err != nil {
		return ValidationResult{Valid: false, Reason: ErrAccessInvalid}
	}
	if session == nil {
		return ValidationResult{Valid: false, Reason: ErrSessionNotFound}
	}
	if session.UserID != int64(claimsUserID) {
		return ValidationResult{Valid: false, Reason: ErrSessionOwnership}
	}
	if session.Status == SessionStatusRevoked {
		return ValidationResult{Valid: false, Reason: ErrSessionRevoked, Session: session}
	}
	if session.Status == SessionStatusCompromised {
		return ValidationResult{Valid: false, Reason: ErrSessionRevoked, Session: session}
	}
	if session.Status == SessionStatusExpired {
		return ValidationResult{Valid: false, Reason: ErrSessionExpired, Session: session}
	}
	if session.Status != SessionStatusActive {
		return ValidationResult{Valid: false, Reason: ErrSessionRevoked, Session: session}
	}
	if session.ExpiresAt != nil && session.ExpiresAt.Before(time.Now().UTC()) {
		return ValidationResult{Valid: false, Reason: ErrSessionExpired, Session: session}
	}
	if session.AbsoluteExpiresAt != nil && session.AbsoluteExpiresAt.Before(time.Now().UTC()) {
		return ValidationResult{Valid: false, Reason: ErrSessionExpired, Session: session}
	}
	if v.users != nil {
		user, err := v.users.GetUserByID(int(session.UserID))
		if err != nil {
			return ValidationResult{Valid: false, Reason: ErrAccessInvalid}
		}
		if user.IsActive != 1 {
			return ValidationResult{Valid: false, Reason: ErrUserInactive, Session: session}
		}
		return ValidationResult{Valid: true, Session: session, User: &user}
	}
	return ValidationResult{Valid: true, Session: session}
}

func (v *Validator) TouchSession(sessionPublicID string) {
	_ = v.sessions.TouchLastActive(sessionPublicID)
}
