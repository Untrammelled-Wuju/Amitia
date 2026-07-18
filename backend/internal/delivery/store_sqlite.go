package delivery

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"
)

type DeliveryIntentModel struct {
	ID               string `gorm:"primaryKey;column:id"`
	InteractionID    string `gorm:"column:interaction_id;index"`
	ResponseGroupID  string `gorm:"column:response_group_id;index:idx_delivery_message_plan,priority:1"`
	DeliverySequence int    `gorm:"column:delivery_sequence;index:idx_delivery_message_plan,priority:2"`
	Channel          string `gorm:"column:channel"`
	PeerID           string `gorm:"column:peer_id"`
	ContentType      string `gorm:"column:content_type"`
	Payload          []byte `gorm:"column:payload"`
	Status           string `gorm:"column:status;index"`
	CreatedAt        string `gorm:"column:created_at"`
	SentAt           string `gorm:"column:sent_at"`
	DeliveredAt      string `gorm:"column:delivered_at"`
	RetryCount       int    `gorm:"column:retry_count"`
	MaxRetries       int    `gorm:"column:max_retries"`
	LastError        string `gorm:"column:last_error"`
	LeaseOwner       string `gorm:"column:lease_owner"`
	LeaseToken       string `gorm:"column:lease_token"`
	LeaseUntil       string `gorm:"column:lease_until"`
	NextRetry        string `gorm:"column:next_retry"`
}

func (DeliveryIntentModel) TableName() string {
	return "delivery_intents"
}

type OutputLeaseModel struct {
	ID            string `gorm:"primaryKey;column:id"`
	InteractionID string `gorm:"column:interaction_id;index"`
	CharacterID   string `gorm:"column:character_id;index"`
	UserID        string `gorm:"column:user_id"`
	Channel       string `gorm:"column:channel"`
	OwnerToken    string `gorm:"column:owner_token"`
	Generation    int    `gorm:"column:generation"`
	Status        string `gorm:"column:status"`
	AcquiredAt    string `gorm:"column:acquired_at"`
	ExpiresAt     string `gorm:"column:expires_at"`
	ReleasedAt    string `gorm:"column:released_at"`
	PreemptedBy   string `gorm:"column:preempted_by"`
}

func (OutputLeaseModel) TableName() string {
	return "output_leases"
}

type SQLiteDeliveryStore struct {
	db *gorm.DB
}

func NewSQLiteDeliveryStore(db *gorm.DB) *SQLiteDeliveryStore {
	return &SQLiteDeliveryStore{db: db}
}

func (s *SQLiteDeliveryStore) InitSchema() error {
	return s.db.AutoMigrate(&DeliveryIntentModel{}, &OutputLeaseModel{})
}

func (s *SQLiteDeliveryStore) SubmitIntent(intent DeliveryIntent) (bool, error) {
	now := intent.CreatedAt.Format("2006-01-02 15:04:05")
	model := DeliveryIntentModel{
		ID:               intent.ID,
		InteractionID:    intent.InteractionID,
		ResponseGroupID:  intent.ResponseGroupID,
		DeliverySequence: intent.DeliverySequence,
		Channel:          intent.Channel,
		PeerID:           intent.PeerID,
		ContentType:      intent.ContentType,
		Payload:          intent.Payload,
		Status:           string(intent.Status),
		CreatedAt:        now,
		MaxRetries:       intent.MaxRetries,
	}
	result := s.db.Exec("INSERT OR IGNORE INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries, response_group_id, delivery_sequence) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		model.ID, model.InteractionID, model.Channel, model.PeerID, model.ContentType, model.Payload, model.Status, model.CreatedAt, model.MaxRetries, model.ResponseGroupID, model.DeliverySequence)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func (s *SQLiteDeliveryStore) CreateIntent(intent DeliveryIntent) error {
	now := intent.CreatedAt.Format("2006-01-02 15:04:05")
	model := DeliveryIntentModel{
		ID:               intent.ID,
		InteractionID:    intent.InteractionID,
		ResponseGroupID:  intent.ResponseGroupID,
		DeliverySequence: intent.DeliverySequence,
		Channel:          intent.Channel,
		PeerID:           intent.PeerID,
		ContentType:      intent.ContentType,
		Payload:          intent.Payload,
		Status:           string(intent.Status),
		CreatedAt:        now,
		MaxRetries:       intent.MaxRetries,
	}
	return s.db.Create(&model).Error
}

func (s *SQLiteDeliveryStore) GetIntent(id string) (*DeliveryIntent, error) {
	var model DeliveryIntentModel
	err := s.db.Where("id = ?", id).Take(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return modelToIntent(&model), nil
}

func (s *SQLiteDeliveryStore) UpdateStatus(id string, status DeliveryStatus, errMsg string) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	updates := map[string]interface{}{
		"status":     string(status),
		"last_error": errMsg,
	}
	if status == DeliveryStatusSent {
		updates["sent_at"] = now
	}
	if status == DeliveryStatusDelivered {
		updates["delivered_at"] = now
	}
	if status == DeliveryStatusFailed || status == DeliveryStatusRetry {
		updates["retry_count"] = gorm.Expr("retry_count + 1")
	}
	return s.db.Model(&DeliveryIntentModel{}).Where("id = ?", id).Updates(updates).Error
}

const DeliveryStatusRetry DeliveryStatus = "retry"

func (s *SQLiteDeliveryStore) ListPending(limit int) ([]DeliveryIntent, error) {
	var models []DeliveryIntentModel
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	err := s.db.Where("status = ? OR (status = ? AND (next_retry IS NULL OR next_retry = '' OR next_retry <= ?))", string(DeliveryStatusPending), string(DeliveryStatusRetry), now).
		Order("created_at ASC, response_group_id ASC, CASE WHEN status = 'pending' THEN 0 ELSE 1 END ASC, delivery_sequence ASC").Limit(limit).Find(&models).Error
	if err != nil {
		return nil, err
	}
	intents := make([]DeliveryIntent, 0, len(models))
	for _, m := range models {
		intents = append(intents, *modelToIntent(&m))
	}
	return intents, nil
}

func (s *SQLiteDeliveryStore) CreateLease(lease OutputLease) error {
	model := leaseToModel(&lease)
	return s.db.Create(&model).Error
}

func (s *SQLiteDeliveryStore) GetActiveLease(characterID, userID, channel string) (*OutputLease, error) {
	var model OutputLeaseModel
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	err := s.db.Where("character_id = ? AND user_id = ? AND channel = ? AND status = ? AND expires_at > ?",
		characterID, userID, channel, "active", now).Order("acquired_at DESC").Take(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return modelToLease(&model), nil
}

func (s *SQLiteDeliveryStore) PreemptLease(id, byID string) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	result := s.db.Model(&OutputLeaseModel{}).Where("id = ? AND status = ?", id, "active").
		Updates(map[string]interface{}{
			"status":       "preempted",
			"preempted_by": byID,
			"released_at":  now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("lease not found or not active")
	}
	return nil
}

func (s *SQLiteDeliveryStore) ReleaseLease(id, ownerToken string) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	result := s.db.Model(&OutputLeaseModel{}).Where("id = ? AND status = ? AND owner_token = ?", id, "active", ownerToken).
		Updates(map[string]interface{}{
			"status":      "released",
			"released_at": now,
			"generation":  gorm.Expr("generation + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (s *SQLiteDeliveryStore) RenewLease(id, ownerToken string, newExpiry time.Time) error {
	expiresStr := newExpiry.UTC().Format("2006-01-02 15:04:05")
	result := s.db.Model(&OutputLeaseModel{}).Where("id = ? AND status = ? AND owner_token = ?", id, "active", ownerToken).
		Update("expires_at", expiresStr)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("lease not found or not active")
	}
	return nil
}

func (s *SQLiteDeliveryStore) ClaimNextIntents(batchSize int) ([]DeliveryIntent, error) {
	now := time.Now().UTC()
	leasedUntil := now.Add(60 * time.Second).Format("2006-01-02 15:04:05")
	nowStr := now.Format("2006-01-02 15:04:05")
	owner := "delivery-worker"

	var models []DeliveryIntentModel
	err := s.db.Where("(status = ? OR (status = ? AND (next_retry IS NULL OR next_retry = '' OR next_retry <= ?))) AND (lease_until IS NULL OR lease_until = '' OR lease_until <= ?)",
		string(DeliveryStatusPending), string(DeliveryStatusRetry), nowStr, nowStr).
		Order("created_at ASC, response_group_id ASC, CASE WHEN status = 'pending' THEN 0 ELSE 1 END ASC, delivery_sequence ASC").Limit(batchSize).Find(&models).Error
	if err != nil {
		return nil, err
	}

	intents := make([]DeliveryIntent, 0, len(models))
	for _, m := range models {
		token := generateDeliveryLeaseToken()
		res := s.db.Model(&DeliveryIntentModel{}).Where("id = ? AND status IN ? AND (lease_until IS NULL OR lease_until = '' OR lease_until <= ?)",
			m.ID, []string{string(DeliveryStatusPending), string(DeliveryStatusRetry)}, nowStr).
			Updates(map[string]interface{}{
				"status":      string(DeliveryStatusLeased),
				"lease_owner": owner,
				"lease_token": token,
				"lease_until": leasedUntil,
			})
		if res.Error != nil || res.RowsAffected != 1 {
			continue
		}
		intent := modelToIntent(&m)
		intent.Status = DeliveryStatusLeased
		intent.LeaseOwner = owner
		intent.LeaseToken = token
		leaseUntilTime := now.Add(60 * time.Second)
		intent.LeaseUntil = &leaseUntilTime
		intents = append(intents, *intent)
	}
	return intents, nil
}

func (s *SQLiteDeliveryStore) MarkSent(id, leaseToken string) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	res := s.db.Model(&DeliveryIntentModel{}).
		Where("id = ? AND status = ? AND lease_token = ?",
			id, string(DeliveryStatusLeased), leaseToken).
		Updates(map[string]interface{}{
			"status":      string(DeliveryStatusSent),
			"sent_at":     now,
			"last_error":  "",
			"lease_owner": "",
			"lease_token": "",
			"lease_until": "",
			"next_retry":  "",
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	_ = s.db.Table("emote_send_records").Where("delivery_key = ?", id).Updates(map[string]interface{}{"status": "sent", "sent_at": now, "failure_reason": ""}).Error
	_ = s.db.Table("messages").Where("id = (SELECT message_id FROM emote_send_records WHERE delivery_key = ?)", id).Updates(map[string]interface{}{"status": "sent", "emote_decision_status": "sent", "updated_at": now}).Error
	return nil
}

func (s *SQLiteDeliveryStore) MarkDelivered(id string) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	res := s.db.Model(&DeliveryIntentModel{}).
		Where("id = ? AND status IN ? AND lease_until > ?",
			id, []string{string(DeliveryStatusLeased), string(DeliveryStatusSent)}, now).
		Updates(map[string]interface{}{
			"status":       string(DeliveryStatusDelivered),
			"delivered_at": now,
		})
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (s *SQLiteDeliveryStore) MarkFailed(id, leaseToken, errMsg string) error {
	now := time.Now().UTC()
	nowStr := now.Format("2006-01-02 15:04:05")
	return s.db.Transaction(func(tx *gorm.DB) error {
		var m DeliveryIntentModel
		if err := tx.Where("id = ? AND status = ? AND lease_token = ? AND lease_until > ?",
			id, string(DeliveryStatusLeased), leaseToken, nowStr).Take(&m).Error; err != nil {
			return errors.New("lease conflict or expired")
		}
		newCount := m.RetryCount + 1
		newStatus := DeliveryStatusRetry
		if newCount >= m.MaxRetries {
			newStatus = DeliveryStatusFailed
		}
		nextRetry := now.Add(time.Duration(newCount) * 2 * time.Second).Format("2006-01-02 15:04:05")
		res := tx.Model(&DeliveryIntentModel{}).
			Where("id = ? AND status = ? AND lease_token = ?", id, string(DeliveryStatusLeased), leaseToken).
			Updates(map[string]interface{}{
				"status":      string(newStatus),
				"retry_count": newCount,
				"next_retry":  nextRetry,
				"last_error":  errMsg,
				"lease_owner": "",
				"lease_token": "",
				"lease_until": "",
			})
		if res.Error != nil {
			return res.Error
		}
		recordStatus := "retrying"
		if newStatus == DeliveryStatusFailed {
			recordStatus = "failed"
		}
		_ = tx.Table("emote_send_records").Where("delivery_key = ?", id).Updates(map[string]interface{}{"status": recordStatus, "failure_reason": errMsg}).Error
		if newStatus == DeliveryStatusFailed {
			_ = tx.Table("messages").Where("id = (SELECT message_id FROM emote_send_records WHERE delivery_key = ?)", id).Updates(map[string]interface{}{"status": "failed", "emote_decision_status": "failed", "updated_at": nowStr}).Error
		}
		return nil
	})
}

func (s *SQLiteDeliveryStore) ReleaseExpiredClaims() (int64, error) {

	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	res := s.db.Model(&DeliveryIntentModel{}).
		Where("status = ? AND lease_until != '' AND lease_until <= ?", string(DeliveryStatusLeased), now).
		Updates(map[string]interface{}{
			"status":      string(DeliveryStatusPending),
			"lease_owner": "",
			"lease_token": "",
			"lease_until": "",
		})
	return res.RowsAffected, res.Error
}

func (s *SQLiteDeliveryStore) PreemptActiveLeasesByCharacter(characterID string) (int64, error) {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	res := s.db.Model(&OutputLeaseModel{}).
		Where("character_id = ? AND status = ? AND expires_at > ?", characterID, "active", now).
		Updates(map[string]interface{}{
			"status":       "preempted",
			"preempted_by": "user_input",
			"released_at":  now,
		})
	return res.RowsAffected, res.Error
}

func generateDeliveryLeaseToken() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func parseOptionalTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		return nil
	}
	return &t
}

func modelToIntent(m *DeliveryIntentModel) *DeliveryIntent {
	var sentAt, deliveredAt *time.Time
	if m.SentAt != "" {
		t, err := time.Parse("2006-01-02 15:04:05", m.SentAt)
		if err == nil {
			sentAt = &t
		}
	}
	if m.DeliveredAt != "" {
		t, err := time.Parse("2006-01-02 15:04:05", m.DeliveredAt)
		if err == nil {
			deliveredAt = &t
		}
	}
	createdAt, _ := time.Parse("2006-01-02 15:04:05", m.CreatedAt)
	return &DeliveryIntent{
		ID:               m.ID,
		InteractionID:    m.InteractionID,
		ResponseGroupID:  m.ResponseGroupID,
		DeliverySequence: m.DeliverySequence,
		Channel:          m.Channel,
		PeerID:           m.PeerID,
		ContentType:      m.ContentType,
		Payload:          m.Payload,
		Status:           DeliveryStatus(m.Status),
		CreatedAt:        createdAt,
		SentAt:           sentAt,
		DeliveredAt:      deliveredAt,
		RetryCount:       m.RetryCount,
		MaxRetries:       m.MaxRetries,
		LastError:        m.LastError,
		LeaseOwner:       m.LeaseOwner,
		LeaseToken:       m.LeaseToken,
		LeaseUntil:       parseOptionalTime(m.LeaseUntil),
		NextRetry:        parseOptionalTime(m.NextRetry),
	}
}

func leaseToModel(l *OutputLease) *OutputLeaseModel {
	model := &OutputLeaseModel{
		ID:            l.ID,
		InteractionID: l.InteractionID,
		CharacterID:   l.CharacterID,
		UserID:        l.UserID,
		Channel:       l.Channel,
		OwnerToken:    l.OwnerToken,
		Generation:    l.Generation,
		Status:        l.Status,
		AcquiredAt:    l.AcquiredAt.UTC().Format("2006-01-02 15:04:05"),
		ExpiresAt:     l.ExpiresAt.UTC().Format("2006-01-02 15:04:05"),
		PreemptedBy:   l.PreemptedBy,
	}
	if l.ReleasedAt != nil {
		released := l.ReleasedAt.UTC().Format("2006-01-02 15:04:05")
		model.ReleasedAt = released
	}
	return model
}

func modelToLease(m *OutputLeaseModel) *OutputLease {
	acquiredAt, _ := time.Parse("2006-01-02 15:04:05", m.AcquiredAt)
	expiresAt, _ := time.Parse("2006-01-02 15:04:05", m.ExpiresAt)
	lease := &OutputLease{
		ID:            m.ID,
		InteractionID: m.InteractionID,
		CharacterID:   m.CharacterID,
		UserID:        m.UserID,
		Channel:       m.Channel,
		OwnerToken:    m.OwnerToken,
		Generation:    m.Generation,
		Status:        m.Status,
		AcquiredAt:    acquiredAt,
		ExpiresAt:     expiresAt,
		PreemptedBy:   m.PreemptedBy,
	}
	if m.ReleasedAt != "" {
		t, err := time.Parse("2006-01-02 15:04:05", m.ReleasedAt)
		if err == nil {
			lease.ReleasedAt = &t
		}
	}
	return lease
}
