package accountsession

import "time"

const (
	SessionStatusActive     = "active"
	SessionStatusRevoked    = "revoked"
	SessionStatusExpired    = "expired"
	SessionStatusCompromised = "compromised"

	RefreshStatusActive   = "active"
	RefreshStatusUsed     = "used"
	RefreshStatusRevoked  = "revoked"
	RefreshStatusExpired  = "expired"

	RecoveryStatusActive  = "active"
	RecoveryStatusUsed    = "used"
	RecoveryStatusRevoked = "revoked"
	RecoveryStatusExpired = "expired"

	GrantStatusActive   = "active"
	GrantStatusConsumed = "consumed"
	GrantStatusExpired  = "expired"

	AuditOutcomeSuccess = "success"
	AuditOutcomeFailure = "failure"

	AuditSeverityInfo     = "info"
	AuditSeverityWarning  = "warning"
	AuditSeverityCritical = "critical"
)

type Session struct {
	ID               int64      `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	PublicID         string     `gorm:"column:public_id;uniqueIndex;size:64;not null" json:"sessionId"`
	UserID           int64      `gorm:"column:user_id;not null;index:idx_session_user_status" json:"-"`
	Username         string     `gorm:"column:username;size:100;not null;default:''" json:"username"`
	Role             string     `gorm:"column:role;size:20;not null;default:'user'" json:"role"`
	Status           string     `gorm:"column:status;size:20;not null;index:idx_session_user_status" json:"status"`
	DeviceName       string     `gorm:"column:device_name;size:100" json:"deviceName"`
	IPAddress        string     `gorm:"column:ip_address;size:50" json:"ipAddress"`
	UserAgent        string     `gorm:"column:user_agent;size:500" json:"userAgent"`
	Revision         int64      `gorm:"column:revision;not null;default:1" json:"-"`
	TokenHash        *string    `gorm:"column:token_hash;size:255" json:"-"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"createdAt"`
	LastActiveAt     *time.Time `gorm:"column:last_active_at" json:"lastActiveAt"`
	LastRefreshedAt  *time.Time `gorm:"column:last_refreshed_at" json:"lastRefreshedAt"`
	ExpiresAt        *time.Time `gorm:"column:expires_at" json:"expiresAt"`
	AbsoluteExpiresAt *time.Time `gorm:"column:absolute_expires_at" json:"-"`
	RevokedAt        *time.Time `gorm:"column:revoked_at" json:"-"`
	RevokeReason     string     `gorm:"column:revoke_reason;size:100" json:"-"`
}

func (Session) TableName() string { return "auth_sessions" }

type RefreshToken struct {
	TokenID          string     `gorm:"column:token_id;primaryKey;size:64" json:"-"`
	SessionID        string     `gorm:"column:session_id;size:64;not null;index:idx_refresh_session" json:"-"`
	TokenHash        string     `gorm:"column:token_hash;size:255;not null;uniqueIndex" json:"-"`
	Status           string     `gorm:"column:status;size:20;not null;index:idx_refresh_status" json:"status"`
	IssuedAt         time.Time  `gorm:"column:issued_at;not null" json:"issuedAt"`
	ExpiresAt        time.Time  `gorm:"column:expires_at;not null;index:idx_refresh_status" json:"expiresAt"`
	UsedAt           *time.Time `gorm:"column:used_at" json:"usedAt"`
	RevokedAt        *time.Time `gorm:"column:revoked_at" json:"-"`
	ReplacedByTokenID *string   `gorm:"column:replaced_by_token_id;size:64" json:"-"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"-"`
}

func (RefreshToken) TableName() string { return "auth_refresh_tokens" }

type LoginGuard struct {
	GuardKey       string    `gorm:"column:guard_key;primaryKey;size:255" json:"-"`
	Dimension      string    `gorm:"column:dimension;size:20;primaryKey" json:"-"`
	FailureCount   int64     `gorm:"column:failure_count;not null;default:0" json:"-"`
	WindowStartedAt time.Time `gorm:"column:window_started_at;not null" json:"-"`
	BlockedUntil   *time.Time `gorm:"column:blocked_until" json:"-"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP" json:"-"`
}

func (LoginGuard) TableName() string { return "auth_login_guards" }

type RecoveryCode struct {
	CodeID    string     `gorm:"column:code_id;primaryKey;size:64" json:"-"`
	UserID    int64      `gorm:"column:user_id;not null;index:idx_recovery_user" json:"-"`
	CodeHash  string     `gorm:"column:code_hash;size:255;not null;uniqueIndex" json:"-"`
	Status    string     `gorm:"column:status;size:20;not null;index:idx_recovery_user" json:"-"`
	CreatedAt time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"-"`
	UsedAt    *time.Time `gorm:"column:used_at" json:"-"`
	ExpiresAt *time.Time `gorm:"column:expires_at" json:"-"`
	Generation int64     `gorm:"column:generation;not null;default:1" json:"-"`
}

func (RecoveryCode) TableName() string { return "auth_recovery_codes" }

type RecoveryGrant struct {
	GrantID   string     `gorm:"column:grant_id;primaryKey;size:64" json:"-"`
	UserID    int64      `gorm:"column:user_id;not null;index:idx_grant_user" json:"-"`
	GrantHash string     `gorm:"column:grant_hash;size:255;not null;uniqueIndex" json:"-"`
	Status    string     `gorm:"column:status;size:20;not null" json:"-"`
	CreatedAt time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"-"`
	ExpiresAt time.Time  `gorm:"column:expires_at;not null" json:"-"`
	ConsumedAt *time.Time `gorm:"column:consumed_at" json:"-"`
}

func (RecoveryGrant) TableName() string { return "auth_recovery_grants" }
