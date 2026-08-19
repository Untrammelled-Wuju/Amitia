package accountsession

import (
	"fmt"

	"gorm.io/gorm"
)

type AuditLogger interface {
	LogLoginSuccess(userID int, sessionID, ip, ua, username string) error
	LogLoginFailed(ip, ua, username, reason string) error
	LogLoginRateLimited(ip, ua, username string) error
	LogRefreshSuccess(sessionID string, userID int, ip, ua string) error
	LogRefreshFailed(sessionID string, userID int, ip, ua, reason string) error
	LogRefreshReuseDetected(sessionID string, userID int, ip, ua string) error
	LogSessionRevoked(sessionID string, userID int64, reason string) error
	LogSessionsRevoked(actorSessionID string, userID int64, count int) error
	LogLogoutAll(userID int64) error
	LogPasswordChanged(userID int64, sessionID string) error
	LogRecoveryCodesGenerated(userID int64, count int) error
	LogRecoveryCodeUsed(userID int64, codeID string) error
	LogRecoveryCodeFailed(userID int64, reason string) error
}

type AuditRepository interface {
	Insert(event *AuditEvent) error
	ListUserEvents(userID int64, limit int, cursor string) ([]AuditEvent, error)
}

type auditRepository struct {
	db *gorm.DB
}

func NewAuditRepository(db *gorm.DB) AuditRepository {
	return &auditRepository{db: db}
}

func (r *auditRepository) Insert(event *AuditEvent) error {
	return r.db.Create(event).Error
}

func (r *auditRepository) ListUserEvents(userID int64, limit int, cursor string) ([]AuditEvent, error) {
	var events []AuditEvent
	query := r.db.Where("user_id = ?", fmt.Sprintf("%d", userID)).Order("occurred_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if cursor != "" {
		query = query.Where("occurred_at < ?", cursor)
	}
	err := query.Find(&events).Error
	return events, err
}

type AuditEvent struct {
	EventID       string `gorm:"column:event_id;primaryKey;size:64" json:"eventId"`
	EventType     string `gorm:"column:event_type;size:100;not null;index:idx_audit_type" json:"eventType"`
	Severity      string `gorm:"column:severity;size:20;not null;default:info" json:"severity"`
	Outcome       string `gorm:"column:outcome;size:20;not null" json:"outcome"`
	UserID        string `gorm:"column:user_id;size:50;index:idx_audit_user" json:"userId"`
	SessionID     string `gorm:"column:session_id;size:64;index:idx_audit_session" json:"sessionId"`
	ActorType     string `gorm:"column:actor_type;size:30" json:"actorType"`
	AuthMethod    string `gorm:"column:auth_method;size:30" json:"authMethod"`
	IPAddress     string `gorm:"column:ip_address;size:50" json:"ipAddress"`
	UserAgent     string `gorm:"column:user_agent;size:500" json:"userAgent"`
	ReasonCode    string `gorm:"column:reason_code;size:100" json:"reasonCode"`
	DetailsJSON   string `gorm:"column:details_json" json:"detailsJson"`
	OccurredAt    string `gorm:"column:occurred_at;not null;index:idx_audit_time" json:"occurredAt"`
}

func (AuditEvent) TableName() string { return "security_audit_events" }

type auditLogger struct {
	repo AuditRepository
}

func NewAuditLogger(repo AuditRepository) AuditLogger {
	return &auditLogger{repo: repo}
}

func (a *auditLogger) LogLoginSuccess(userID int, sessionID, ip, ua, username string) error {
	event := &AuditEvent{
		EventID:    GeneratePublicID("ae_"),
		EventType:  "auth.login_succeeded",
		Severity:   AuditSeverityInfo,
		Outcome:    AuditOutcomeSuccess,
		UserID:     itoa(userID),
		SessionID:  sessionID,
		ActorType:  "user",
		AuthMethod: "jwt",
		IPAddress:  ip,
		UserAgent:  ua,
	}
	return a.repo.Insert(event)
}

func (a *auditLogger) LogLoginFailed(ip, ua, username, reason string) error {
	event := &AuditEvent{
		EventID:    GeneratePublicID("ae_"),
		EventType:  "auth.login_failed",
		Severity:   AuditSeverityWarning,
		Outcome:    AuditOutcomeFailure,
		ActorType:  "user",
		AuthMethod: "jwt",
		IPAddress:  ip,
		UserAgent:  ua,
		ReasonCode: reason,
	}
	return a.repo.Insert(event)
}

func (a *auditLogger) LogLoginRateLimited(ip, ua, username string) error {
	event := &AuditEvent{
		EventID:    GeneratePublicID("ae_"),
		EventType:  "auth.login_rate_limited",
		Severity:   AuditSeverityWarning,
		Outcome:    AuditOutcomeFailure,
		ActorType:  "user",
		AuthMethod: "jwt",
		IPAddress:  ip,
		UserAgent:  ua,
	}
	return a.repo.Insert(event)
}

func (a *auditLogger) LogRefreshSuccess(sessionID string, userID int, ip, ua string) error {
	event := &AuditEvent{
		EventID:    GeneratePublicID("ae_"),
		EventType:  "auth.refresh_succeeded",
		Severity:   AuditSeverityInfo,
		Outcome:    AuditOutcomeSuccess,
		SessionID:  sessionID,
		UserID:     itoa(userID),
		ActorType:  "user",
		AuthMethod: "jwt",
		IPAddress:  ip,
		UserAgent:  ua,
	}
	return a.repo.Insert(event)
}

func (a *auditLogger) LogRefreshFailed(sessionID string, userID int, ip, ua, reason string) error {
	event := &AuditEvent{
		EventID:    GeneratePublicID("ae_"),
		EventType:  "auth.refresh_failed",
		Severity:   AuditSeverityWarning,
		Outcome:    AuditOutcomeFailure,
		SessionID:  sessionID,
		UserID:     itoa(userID),
		ActorType:  "user",
		AuthMethod: "jwt",
		IPAddress:  ip,
		UserAgent:  ua,
		ReasonCode: reason,
	}
	return a.repo.Insert(event)
}

func (a *auditLogger) LogRefreshReuseDetected(sessionID string, userID int, ip, ua string) error {
	event := &AuditEvent{
		EventID:    GeneratePublicID("ae_"),
		EventType:  "auth.refresh_reuse_detected",
		Severity:   AuditSeverityCritical,
		Outcome:    AuditOutcomeFailure,
		SessionID:  sessionID,
		UserID:     itoa(userID),
		ActorType:  "user",
		AuthMethod: "jwt",
		IPAddress:  ip,
		UserAgent:  ua,
	}
	return a.repo.Insert(event)
}

func (a *auditLogger) LogSessionRevoked(sessionID string, userID int64, reason string) error {
	event := &AuditEvent{
		EventID:    GeneratePublicID("ae_"),
		EventType:  "auth.session_revoked",
		Severity:   AuditSeverityInfo,
		Outcome:    AuditOutcomeSuccess,
		SessionID:  sessionID,
		UserID:     itoa64(userID),
		ActorType:  "user",
		AuthMethod: "jwt",
		ReasonCode: reason,
	}
	return a.repo.Insert(event)
}

func (a *auditLogger) LogSessionsRevoked(actorSessionID string, userID int64, count int) error {
	event := &AuditEvent{
		EventID:    GeneratePublicID("ae_"),
		EventType:  "auth.sessions_revoked",
		Severity:   AuditSeverityInfo,
		Outcome:    AuditOutcomeSuccess,
		SessionID:  actorSessionID,
		UserID:     itoa64(userID),
		ActorType:  "user",
		AuthMethod: "jwt",
	}
	return a.repo.Insert(event)
}

func (a *auditLogger) LogLogoutAll(userID int64) error {
	event := &AuditEvent{
		EventID:    GeneratePublicID("ae_"),
		EventType:  "auth.logout_all",
		Severity:   AuditSeverityInfo,
		Outcome:    AuditOutcomeSuccess,
		UserID:     itoa64(userID),
		ActorType:  "user",
		AuthMethod: "jwt",
	}
	return a.repo.Insert(event)
}

func (a *auditLogger) LogPasswordChanged(userID int64, sessionID string) error {
	event := &AuditEvent{
		EventID:    GeneratePublicID("ae_"),
		EventType:  "auth.password_changed",
		Severity:   AuditSeverityInfo,
		Outcome:    AuditOutcomeSuccess,
		UserID:     itoa64(userID),
		SessionID:  sessionID,
		ActorType:  "user",
		AuthMethod: "jwt",
	}
	return a.repo.Insert(event)
}

func (a *auditLogger) LogRecoveryCodesGenerated(userID int64, count int) error {
	event := &AuditEvent{
		EventID:    GeneratePublicID("ae_"),
		EventType:  "auth.recovery_codes_generated",
		Severity:   AuditSeverityInfo,
		Outcome:    AuditOutcomeSuccess,
		UserID:     itoa64(userID),
		ActorType:  "user",
		AuthMethod: "jwt",
	}
	return a.repo.Insert(event)
}

func (a *auditLogger) LogRecoveryCodeUsed(userID int64, codeID string) error {
	event := &AuditEvent{
		EventID:    GeneratePublicID("ae_"),
		EventType:  "auth.recovery_code_used",
		Severity:   AuditSeverityInfo,
		Outcome:    AuditOutcomeSuccess,
		UserID:     itoa64(userID),
		ActorType:  "user",
		AuthMethod: "jwt",
	}
	return a.repo.Insert(event)
}

func (a *auditLogger) LogRecoveryCodeFailed(userID int64, reason string) error {
	event := &AuditEvent{
		EventID:    GeneratePublicID("ae_"),
		EventType:  "auth.recovery_code_failed",
		Severity:   AuditSeverityWarning,
		Outcome:    AuditOutcomeFailure,
		UserID:     itoa64(userID),
		ActorType:  "user",
		AuthMethod: "jwt",
		ReasonCode: reason,
	}
	return a.repo.Insert(event)
}

func itoa(i int) string {
	if i == 0 {
		return ""
	}
	return itoa64(int64(i))
}

func itoa64(i int64) string {
	if i == 0 {
		return ""
	}
	return fmt.Sprintf("%d", i)
}
