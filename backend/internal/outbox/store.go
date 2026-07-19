package outbox

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrLeaseConflict     = errors.New("outbox: lease conflict")
	ErrInvalidTransition = errors.New("outbox: invalid transition")
	ErrLeaseExpired      = errors.New("outbox: lease expired")
)

type OutboxRecordModel struct {
	ID             string `gorm:"primaryKey;column:id"`
	AggregateID    string `gorm:"column:aggregate_id"`
	EventType      string `gorm:"column:event_type"`
	Payload        string `gorm:"column:payload"`
	PayloadVersion string `gorm:"column:payload_version"`
	Status         string `gorm:"column:status"`
	LeaseOwner     string `gorm:"column:lease_owner"`
	LeaseToken     string `gorm:"column:lease_token"`
	LeasedUntil    string `gorm:"column:leased_until"`
	AvailableAt    string `gorm:"column:available_at"`
	PublishedAt    string `gorm:"column:published_at"`
	UpdatedAt      string `gorm:"column:updated_at"`
	RetryCount     int    `gorm:"column:retry_count"`
	MaxRetries     int    `gorm:"column:max_retries"`
	NextRetryAt    string `gorm:"column:next_retry_at"`
	LastError      string `gorm:"column:last_error"`
	IdempotencyKey string `gorm:"column:idempotency_key"`
	CreatedAt      string `gorm:"column:created_at"`
}

func (OutboxRecordModel) TableName() string {
	return "outbox_records"
}

type DeadLetterRecordModel struct {
	ID          string `gorm:"primaryKey;column:id"`
	OutboxID    string `gorm:"uniqueIndex;column:outbox_id"`
	EventType   string `gorm:"column:event_type"`
	Payload     string `gorm:"column:payload"`
	Status      string `gorm:"column:status"`
	RetryCount  int    `gorm:"column:retry_count"`
	MaxRetries  int    `gorm:"column:max_retries"`
	NextRetryAt string `gorm:"column:next_retry_at"`
	LastError   string `gorm:"column:last_error"`
	CreatedAt   string `gorm:"column:created_at"`
	UpdatedAt   string `gorm:"column:updated_at"`
}

func (DeadLetterRecordModel) TableName() string {
	return "dead_letter_records"
}

type SQLiteOutboxStore struct {
	db  *gorm.DB
	mu  sync.Mutex
	cfg OutboxStoreConfig
}

type OutboxStoreConfig struct {
	LeaseTTL     time.Duration
	RenewWindow  time.Duration
	MaxRetries   int
	RetryBackoff time.Duration
}

func DefaultOutboxStoreConfig() OutboxStoreConfig {
	return OutboxStoreConfig{
		LeaseTTL:     DefaultLeaseTTL,
		RenewWindow:  DefaultRenewWindow,
		MaxRetries:   DefaultMaxRetries,
		RetryBackoff: 2 * time.Second,
	}
}

func NewSQLiteOutboxStore(db *gorm.DB, cfg OutboxStoreConfig) *SQLiteOutboxStore {
	return &SQLiteOutboxStore{db: db, cfg: cfg}
}

func (s *SQLiteOutboxStore) DB() *gorm.DB {
	return s.db
}

func (s *SQLiteOutboxStore) Append(record OutboxRecord) error {
	model := OutboxRecordModel{
		ID:             record.ID,
		AggregateID:    record.AggregateID,
		EventType:      record.EventType,
		Payload:        string(record.Payload),
		PayloadVersion: record.PayloadVersion,
		Status:         string(record.Status),
		AvailableAt:    record.AvailableAt.Format("2006-01-02 15:04:05"),
		MaxRetries:     record.MaxRetries,
		NextRetryAt:    record.AvailableAt.Format("2006-01-02 15:04:05"),
		IdempotencyKey: record.IdempotencyKey,
		CreatedAt:      record.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:      record.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if record.IdempotencyKey != "" {
		var existing OutboxRecordModel
		err := s.db.Where("idempotency_key = ?", record.IdempotencyKey).Select("id").Take(&existing).Error
		if err == nil {
			return nil
		}
	}
	return s.db.Create(&model).Error
}

func (s *SQLiteOutboxStore) AppendWithTx(tx *gorm.DB, record OutboxRecord) error {
	model := OutboxRecordModel{
		ID:             record.ID,
		AggregateID:    record.AggregateID,
		EventType:      record.EventType,
		Payload:        string(record.Payload),
		PayloadVersion: record.PayloadVersion,
		Status:         string(record.Status),
		AvailableAt:    record.AvailableAt.Format("2006-01-02 15:04:05"),
		MaxRetries:     record.MaxRetries,
		NextRetryAt:    record.AvailableAt.Format("2006-01-02 15:04:05"),
		IdempotencyKey: record.IdempotencyKey,
		CreatedAt:      record.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:      record.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if record.IdempotencyKey != "" {
		var existing OutboxRecordModel
		err := tx.Where("idempotency_key = ?", record.IdempotencyKey).Select("id").Take(&existing).Error
		if err == nil {
			return nil
		}
	}
	return tx.Create(&model).Error
}

func (s *SQLiteOutboxStore) ClaimNext(batchSize int, owner string) ([]OutboxRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	availableAt := now.Format("2006-01-02 15:04:05")
	leasedUntil := now.Add(s.cfg.LeaseTTL).Format("2006-01-02 15:04:05")

	var models []OutboxRecordModel
	err := s.db.Where("(status = ? AND available_at <= ?) OR (status = ? AND next_retry_at <= ?) OR (status = ? AND leased_until <= ?)",
		string(OutboxStatusPending), availableAt,
		string(OutboxStatusRetry), availableAt,
		string(OutboxStatusLeased), availableAt).
		Limit(batchSize).
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	records := make([]OutboxRecord, 0, len(models))
	for _, m := range models {
		token := generateLeaseToken()
		res := s.db.Model(&OutboxRecordModel{}).
			Where("id = ? AND (status = ? OR status = ? OR status = ?)", m.ID, string(OutboxStatusPending), string(OutboxStatusRetry), string(OutboxStatusLeased)).
			Updates(map[string]interface{}{
				"status":       string(OutboxStatusLeased),
				"lease_owner":  owner,
				"lease_token":  token,
				"leased_until": leasedUntil,
				"updated_at":   now.Format("2006-01-02 15:04:05"),
			})
		if res.Error != nil || res.RowsAffected != 1 {
			continue
		}
		records = append(records, modelToRecord(m, owner, token, now.Add(s.cfg.LeaseTTL)))
	}
	return records, nil
}

func (s *SQLiteOutboxStore) MarkPublished(id, leaseToken string) error {
	if !s.validateLease(id, leaseToken) {
		return ErrLeaseConflict
	}
	now := time.Now().UTC()
	res := s.db.Model(&OutboxRecordModel{}).
		Where("id = ? AND lease_token = ? AND status = ?", id, leaseToken, string(OutboxStatusLeased)).
		Updates(map[string]interface{}{
			"status":       string(OutboxStatusPublished),
			"published_at": now.Format("2006-01-02 15:04:05"),
			"updated_at":   now.Format("2006-01-02 15:04:05"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrLeaseConflict
	}
	return nil
}

func (s *SQLiteOutboxStore) MarkFailed(id, leaseToken, errMsg string) error {
	if !s.validateLease(id, leaseToken) {
		return ErrLeaseConflict
	}
	now := time.Now().UTC()
	return s.db.Transaction(func(tx *gorm.DB) error {
		var m OutboxRecordModel
		if err := tx.Where("id = ? AND lease_token = ? AND status = ?", id, leaseToken, string(OutboxStatusLeased)).Take(&m).Error; err != nil {
			return ErrLeaseConflict
		}
		newCount := m.RetryCount + 1
		newStatus := OutboxStatusRetry
		if newCount >= m.MaxRetries {
			newStatus = OutboxStatusDead
		}
		nextRetry := now.Add(s.cfg.RetryBackoff * time.Duration(newCount))
		res := tx.Model(&OutboxRecordModel{}).
			Where("id = ? AND lease_token = ? AND status = ?", id, leaseToken, string(OutboxStatusLeased)).
			Updates(map[string]interface{}{
				"status":        string(newStatus),
				"retry_count":   newCount,
				"next_retry_at": nextRetry.Format("2006-01-02 15:04:05"),
				"last_error":    errMsg,
				"updated_at":    now.Format("2006-01-02 15:04:05"),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrLeaseConflict
		}
		if newStatus == OutboxStatusDead {
			dl := DeadLetterRecordModel{
				ID:         uuid.New().String(),
				OutboxID:   id,
				EventType:  m.EventType,
				Payload:    m.Payload,
				Status:     string(DeadLetterStatusPending),
				MaxRetries: 3,
				LastError:  errMsg,
				CreatedAt:  now.Format("2006-01-02 15:04:05"),
				UpdatedAt:  now.Format("2006-01-02 15:04:05"),
			}
			if err := tx.Create(&dl).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *SQLiteOutboxStore) RenewLease(id, leaseToken string) error {
	now := time.Now().UTC()
	leasedUntil := now.Add(s.cfg.LeaseTTL).Format("2006-01-02 15:04:05")
	res := s.db.Model(&OutboxRecordModel{}).
		Where("id = ? AND lease_token = ? AND status = ? AND leased_until > ?",
			id, leaseToken, string(OutboxStatusLeased), now.Format("2006-01-02 15:04:05")).
		Updates(map[string]interface{}{
			"leased_until": leasedUntil,
			"updated_at":   now.Format("2006-01-02 15:04:05"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrLeaseExpired
	}
	return nil
}

func (s *SQLiteOutboxStore) ReleaseExpiredLeases() (int64, error) {
	now := time.Now().UTC()
	res := s.db.Model(&OutboxRecordModel{}).
		Where("status = ? AND leased_until <= ?", string(OutboxStatusLeased), now.Format("2006-01-02 15:04:05")).
		Updates(map[string]interface{}{
			"status":       string(OutboxStatusPending),
			"lease_owner":  "",
			"lease_token":  "",
			"leased_until": "",
			"updated_at":   now.Format("2006-01-02 15:04:05"),
		})
	return res.RowsAffected, res.Error
}

func (s *SQLiteOutboxStore) validateLease(id, token string) bool {
	var m OutboxRecordModel
	if err := s.db.Where("id = ? AND status = ?", id, string(OutboxStatusLeased)).Take(&m).Error; err != nil {
		return false
	}
	return m.LeaseToken == token && m.LeasedUntil > time.Now().UTC().Format("2006-01-02 15:04:05")
}

func generateLeaseToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func modelToRecord(m OutboxRecordModel, owner, token string, leasedUntil time.Time) OutboxRecord {
	var payload []byte
	if m.Payload != "" {
		payload = []byte(m.Payload)
	}
	pubAt := parseTimePtr(m.PublishedAt)
	availAt := parseTime(m.AvailableAt)
	crAt := parseTime(m.CreatedAt)
	upAt := parseTime(m.UpdatedAt)
	nrAt := parseTime(m.NextRetryAt)

	return OutboxRecord{
		ID:             m.ID,
		AggregateID:    m.AggregateID,
		EventType:      m.EventType,
		Payload:        payload,
		PayloadVersion: m.PayloadVersion,
		Status:         OutboxStatusLeased,
		LeaseOwner:     owner,
		LeaseToken:     token,
		LeasedUntil:    leasedUntil,
		AvailableAt:    availAt,
		PublishedAt:    pubAt,
		UpdatedAt:      upAt,
		RetryCount:     m.RetryCount,
		MaxRetries:     m.MaxRetries,
		NextRetryAt:    nrAt,
		LastError:      m.LastError,
		IdempotencyKey: m.IdempotencyKey,
		CreatedAt:      crAt,
	}
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse("2006-01-02 15:04:05", s)
	return t
}

func parseTimePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		return nil
	}
	return &t
}
