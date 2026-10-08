package notificationruntime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func isNotificationCleanupEnvelope(envelope PushEnvelope) bool {
	eventType := strings.ToLower(strings.TrimSpace(envelope.Type))
	switch eventType {
	case "call.ended", "run.end", "run.dismiss", "liveactivity.end":
		return true
	default:
		return false
	}
}

func (r *Repository) InitSchema() error {
	if r == nil || r.db == nil {
		return errors.New("notification repository unavailable")
	}
	// Pre-space notification runtimes used device_id + run_id as the unique
	// Live Activity key. AutoMigrate does not remove obsolete indexes, so an
	// upgraded shared Cloud Core would otherwise keep rejecting the same
	// device/run pair in a second space even after the model changed.
	migrator := r.db.Migrator()
	if migrator.HasIndex(&LiveActivityToken{}, "idx_notification_live_run") {
		if err := migrator.DropIndex(&LiveActivityToken{}, "idx_notification_live_run"); err != nil {
			return err
		}
	}
	return r.db.AutoMigrate(
		&DeviceEndpoint{},
		&LiveActivityToken{},
		&DeliveryLog{},
		&NotificationOutbox{},
		&NotificationRunTerminal{},
	)
}

func (r *Repository) UpsertDevice(ctx context.Context, endpoint *DeviceEndpoint) error {
	if r == nil || r.db == nil || endpoint == nil {
		return errors.New("notification repository unavailable")
	}
	endpoint.SpaceID = strings.TrimSpace(endpoint.SpaceID)
	endpoint.DeviceID = strings.TrimSpace(endpoint.DeviceID)
	endpoint.Platform = strings.ToLower(strings.TrimSpace(endpoint.Platform))
	if endpoint.SpaceID == "" || endpoint.DeviceID == "" || (endpoint.Platform != "android" && endpoint.Platform != "ios") {
		return errors.New("invalid notification device identity")
	}
	now := time.Now().UTC()
	if endpoint.ID == "" {
		endpoint.ID = uuid.NewString()
	}
	if endpoint.PreviewMode == "" {
		endpoint.PreviewMode = "full"
	}
	endpoint.LastSeenAt = now
	endpoint.UpdatedAt = now
	if endpoint.CreatedAt.IsZero() {
		endpoint.CreatedAt = now
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "space_id"}, {Name: "device_id"}, {Name: "platform"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"device_name", "app_version", "os_version", "locale", "appearance", "timezone", "preferred_provider", "native_data_providers",
			"fcm_token", "apns_token", "voip_token", "hms_token", "mipush_token", "oppo_token", "vivo_token", "honor_token",
			"live_activity_push_to_start_token", "push_enabled", "system_notifications_enabled", "message_push_enabled", "execution_activity_enabled",
			"call_push_enabled", "reminder_push_enabled", "preview_mode", "sound_enabled", "live_activity_supported",
			"dynamic_island_supported", "progress_style_supported", "communication_supported", "last_seen_at",
			"token_updated_at", "updated_at", "revoked_at",
		}),
	}).Create(endpoint).Error
}

func (r *Repository) ListDevices(ctx context.Context, spaceID string) ([]DeviceEndpoint, error) {
	var rows []DeviceEndpoint
	err := r.db.WithContext(ctx).Where("space_id = ? AND revoked_at IS NULL", strings.TrimSpace(spaceID)).Order("last_seen_at DESC").Find(&rows).Error
	return rows, err
}

func (r *Repository) ListPushDevices(ctx context.Context, spaceID string) ([]DeviceEndpoint, error) {
	var rows []DeviceEndpoint
	err := r.db.WithContext(ctx).Where("space_id = ? AND push_enabled = 1 AND revoked_at IS NULL", strings.TrimSpace(spaceID)).Find(&rows).Error
	return rows, err
}

func (r *Repository) GetDevice(ctx context.Context, spaceID, deviceID string) (*DeviceEndpoint, error) {
	var row DeviceEndpoint
	err := r.db.WithContext(ctx).Where("space_id = ? AND device_id = ? AND revoked_at IS NULL", strings.TrimSpace(spaceID), strings.TrimSpace(deviceID)).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repository) GetDeviceForPlatform(ctx context.Context, spaceID, deviceID, platform string) (*DeviceEndpoint, error) {
	var row DeviceEndpoint
	err := r.db.WithContext(ctx).
		Where("space_id = ? AND device_id = ? AND platform = ? AND revoked_at IS NULL",
			strings.TrimSpace(spaceID), strings.TrimSpace(deviceID), strings.ToLower(strings.TrimSpace(platform))).
		First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repository) UpdatePreferences(ctx context.Context, spaceID, deviceID string, values map[string]any) error {
	allowed := map[string]struct{}{
		"push_enabled": {}, "message_push_enabled": {}, "execution_activity_enabled": {}, "call_push_enabled": {},
		"reminder_push_enabled": {}, "preview_mode": {}, "sound_enabled": {},
	}
	update := map[string]any{"updated_at": time.Now().UTC()}
	for key, value := range values {
		if _, ok := allowed[key]; ok {
			update[key] = value
		}
	}
	if len(update) == 1 {
		return nil
	}
	result := r.db.WithContext(ctx).Model(&DeviceEndpoint{}).Where("space_id = ? AND device_id = ? AND revoked_at IS NULL", strings.TrimSpace(spaceID), strings.TrimSpace(deviceID)).Updates(update)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) CancelPendingOutboxByPreferences(
	ctx context.Context,
	spaceID, deviceID string,
	disableAll, disableMessages, disableExecution, disableCalls, disableReminders bool,
) error {
	if r == nil || r.db == nil {
		return errors.New("notification repository unavailable")
	}
	if !disableAll && !disableMessages && !disableExecution && !disableCalls && !disableReminders {
		return nil
	}
	var rows []NotificationOutbox
	if err := r.db.WithContext(ctx).
		Select("id", "envelope_json").
		Where(
			"space_id = ? AND device_id = ? AND status IN ?",
			strings.TrimSpace(spaceID),
			strings.TrimSpace(deviceID),
			[]string{"queued", "retry", "processing"},
		).
		Find(&rows).Error; err != nil {
		return err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		var envelope PushEnvelope
		if err := json.Unmarshal([]byte(row.EnvelopeJSON), &envelope); err != nil {
			continue
		}
		if isNotificationCleanupEnvelope(envelope) {
			continue
		}
		if disableAll {
			ids = append(ids, row.ID)
			continue
		}
		eventType := strings.ToLower(strings.TrimSpace(envelope.Type))
		disabled := false
		switch {
		case strings.HasPrefix(eventType, "message."):
			disabled = disableMessages
		case strings.HasPrefix(eventType, "run."),
			strings.HasPrefix(eventType, "liveactivity."):
			disabled = disableExecution
		case strings.HasPrefix(eventType, "call."):
			disabled = disableCalls
		case strings.HasPrefix(eventType, "reminder."),
			strings.HasPrefix(eventType, "proactive."):
			disabled = disableReminders
		}
		if disabled {
			ids = append(ids, row.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	now := time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(&NotificationOutbox{}).
		Where("id IN ? AND status IN ?", ids, []string{"queued", "retry", "processing"}).
		Updates(map[string]any{
			"status":     "failed",
			"last_error": "notification preference disabled",
			"updated_at": now,
		}).Error
}

func (r *Repository) ListActiveLiveActivities(
	ctx context.Context,
	spaceID, deviceID string,
) ([]LiveActivityToken, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("notification repository unavailable")
	}
	var rows []LiveActivityToken
	err := r.db.WithContext(ctx).
		Where(
			"space_id = ? AND device_id = ? AND ended_at IS NULL",
			strings.TrimSpace(spaceID),
			strings.TrimSpace(deviceID),
		).
		Order("updated_at ASC").
		Find(&rows).Error
	return rows, err
}

func (r *Repository) ListActiveLiveActivitiesForConversation(
	ctx context.Context,
	spaceID, deviceID, conversationID string,
) ([]LiveActivityToken, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("notification repository unavailable")
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, nil
	}
	var rows []LiveActivityToken
	err := r.db.WithContext(ctx).
		Where(
			"space_id = ? AND device_id = ? AND conversation_id = ? AND ended_at IS NULL",
			strings.TrimSpace(spaceID),
			strings.TrimSpace(deviceID),
			conversationID,
		).
		Order("updated_at ASC").
		Find(&rows).Error
	return rows, err
}

func (r *Repository) UpdatePresence(ctx context.Context, spaceID, deviceID string, foreground bool, conversationID string) error {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&DeviceEndpoint{}).Where("space_id = ? AND device_id = ? AND revoked_at IS NULL", strings.TrimSpace(spaceID), strings.TrimSpace(deviceID)).Updates(map[string]any{
		"foreground": foreground, "active_conversation_id": strings.TrimSpace(conversationID), "presence_updated_at": now, "last_seen_at": now, "updated_at": now,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) RevokeDevice(ctx context.Context, spaceID, deviceID string) error {
	if r == nil || r.db == nil {
		return errors.New("notification repository unavailable")
	}
	spaceID = strings.TrimSpace(spaceID)
	deviceID = strings.TrimSpace(deviceID)
	if spaceID == "" || deviceID == "" {
		return errors.New("notification device identity is required")
	}
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&DeviceEndpoint{}).
			Where("space_id = ? AND device_id = ?", spaceID, deviceID).
			Updates(map[string]any{
				"revoked_at":   now,
				"push_enabled": false,
				"foreground":   false,
				"updated_at":   now,
			}).Error; err != nil {
			return err
		}
		if err := tx.Model(&LiveActivityToken{}).
			Where("space_id = ? AND device_id = ? AND ended_at IS NULL", spaceID, deviceID).
			Updates(map[string]any{
				"ended_at":   now,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
		return tx.Model(&NotificationOutbox{}).
			Where("space_id = ? AND device_id = ? AND status IN ?", spaceID, deviceID, []string{"queued", "retry", "processing"}).
			Updates(map[string]any{
				"status":     "failed",
				"last_error": "notification device revoked",
				"updated_at": now,
			}).Error
	})
}

func (r *Repository) DisableToken(ctx context.Context, endpoint *DeviceEndpoint, provider string) error {
	if endpoint == nil {
		return nil
	}
	column := map[string]string{"fcm": "fcm_token", "apns": "apns_token", "apns-voip": "voip_token", "hms": "hms_token", "mipush": "mipush_token", "oppo": "oppo_token", "vivo": "vivo_token", "honor": "honor_token"}[provider]
	if column == "" {
		return nil
	}
	return r.db.WithContext(ctx).Model(&DeviceEndpoint{}).Where("id = ?", endpoint.ID).Updates(map[string]any{column: "", "updated_at": time.Now().UTC()}).Error
}

func (r *Repository) UpsertLiveActivityToken(ctx context.Context, value *LiveActivityToken) error {
	if value == nil || strings.TrimSpace(value.SpaceID) == "" ||
		strings.TrimSpace(value.DeviceID) == "" ||
		strings.TrimSpace(value.RunID) == "" ||
		strings.TrimSpace(value.ActivityID) == "" ||
		strings.TrimSpace(value.UpdateToken) == "" {
		return errors.New("invalid live activity token")
	}
	value.SpaceID = strings.TrimSpace(value.SpaceID)
	value.DeviceID = strings.TrimSpace(value.DeviceID)
	value.RunID = strings.TrimSpace(value.RunID)
	value.ActivityID = strings.TrimSpace(value.ActivityID)
	value.UpdateToken = strings.TrimSpace(value.UpdateToken)
	value.ConversationID = strings.TrimSpace(value.ConversationID)

	// ActivityKit may emit a token refresh with revision=0 long after the
	// server has already observed newer run revisions. Never let token churn
	// move the durable presentation revision backwards.
	var existing LiveActivityToken
	if err := r.db.WithContext(ctx).
		Where(
			"space_id = ? AND device_id = ? AND run_id = ?",
			value.SpaceID,
			value.DeviceID,
			value.RunID,
		).
		First(&existing).Error; err == nil {
		// A run ID is immutable. Once the activity is terminal, a late
		// ActivityKit token callback is an idempotent no-op and must never
		// clear ended_at or recreate the activity.
		if existing.EndedAt != nil {
			return nil
		}
		if value.ConversationID == "" {
			value.ConversationID = existing.ConversationID
		}
		if value.Revision < existing.Revision {
			value.Revision = existing.Revision
		}
	}
	now := time.Now().UTC()
	if value.ID == "" {
		value.ID = uuid.NewString()
	}
	if value.CreatedAt.IsZero() {
		value.CreatedAt = now
	}
	value.UpdatedAt = now
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "space_id"}, {Name: "device_id"}, {Name: "run_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"space_id", "conversation_id", "activity_id", "update_token", "revision", "updated_at", "ended_at"}),
	}).Create(value).Error
}

func (r *Repository) GetLiveActivityToken(ctx context.Context, spaceID, deviceID, runID string) (*LiveActivityToken, error) {
	var value LiveActivityToken
	err := r.db.WithContext(ctx).Where(
		"space_id = ? AND device_id = ? AND run_id = ? AND ended_at IS NULL",
		strings.TrimSpace(spaceID),
		strings.TrimSpace(deviceID),
		strings.TrimSpace(runID),
	).First(&value).Error
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func (r *Repository) EndLiveActivity(ctx context.Context, spaceID, deviceID, runID string, revision int64) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&LiveActivityToken{}).Where(
		"space_id = ? AND device_id = ? AND run_id = ?",
		strings.TrimSpace(spaceID),
		strings.TrimSpace(deviceID),
		strings.TrimSpace(runID),
	).Updates(map[string]any{
		"ended_at": now, "revision": revision, "updated_at": now,
	}).Error
}

func (r *Repository) ClearLiveActivityPushToStartToken(ctx context.Context, spaceID, deviceID string) error {
	return r.db.WithContext(ctx).
		Model(&DeviceEndpoint{}).
		Where(
			"space_id = ? AND device_id = ? AND revoked_at IS NULL",
			strings.TrimSpace(spaceID),
			strings.TrimSpace(deviceID),
		).
		Updates(map[string]any{
			"live_activity_push_to_start_token": "",
			"updated_at":                        time.Now().UTC(),
		}).Error
}

func (r *Repository) CreateOutbox(ctx context.Context, row *NotificationOutbox) error {
	if r == nil || r.db == nil || row == nil {
		return errors.New("notification repository unavailable")
	}
	now := time.Now().UTC()
	if row.ID == "" {
		row.ID = uuid.NewString()
	}
	if row.Status == "" {
		row.Status = "queued"
	}
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}
	row.UpdatedAt = now
	if row.NextAttemptAt.IsZero() {
		row.NextAttemptAt = now
	}
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(row)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	var existing NotificationOutbox
	if err := r.db.WithContext(ctx).
		Select("id", "status", "next_attempt_at", "expires_at").
		Where(
			"notification_id = ? AND space_id = ? AND device_id = ?",
			row.NotificationID,
			row.SpaceID,
			row.DeviceID,
		).
		First(&existing).Error; err != nil {
		return err
	}
	row.ID = existing.ID
	row.Status = existing.Status
	row.NextAttemptAt = existing.NextAttemptAt
	row.ExpiresAt = existing.ExpiresAt
	return nil
}

func (r *Repository) MarkRunTerminal(
	ctx context.Context,
	spaceID, deviceID, runID string,
	revision int64,
) error {
	if r == nil || r.db == nil {
		return errors.New("notification repository unavailable")
	}
	spaceID = strings.TrimSpace(spaceID)
	deviceID = strings.TrimSpace(deviceID)
	runID = strings.TrimSpace(runID)
	if spaceID == "" || deviceID == "" || runID == "" {
		return nil
	}
	now := time.Now().UTC()
	row := &NotificationRunTerminal{
		ID:       uuid.NewString(),
		SpaceID:  spaceID,
		DeviceID: deviceID,
		RunID:    runID,
		Revision: revision,
		EndedAt:  now,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "space_id"},
			{Name: "device_id"},
			{Name: "run_id"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"revision": revision,
			"ended_at": now,
		}),
	}).Create(row).Error
}

func (r *Repository) IsRunTerminal(
	ctx context.Context,
	spaceID, deviceID, runID string,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, errors.New("notification repository unavailable")
	}
	var count int64
	err := r.db.WithContext(ctx).
		Model(&NotificationRunTerminal{}).
		Where(
			"space_id = ? AND device_id = ? AND run_id = ?",
			strings.TrimSpace(spaceID),
			strings.TrimSpace(deviceID),
			strings.TrimSpace(runID),
		).
		Count(&count).Error
	return count > 0, err
}

func (r *Repository) SupersedePendingLiveActivityStart(
	ctx context.Context,
	spaceID, deviceID, runID string,
) error {
	if r == nil || r.db == nil {
		return errors.New("notification repository unavailable")
	}
	spaceID = strings.TrimSpace(spaceID)
	deviceID = strings.TrimSpace(deviceID)
	runID = strings.TrimSpace(runID)
	if spaceID == "" || deviceID == "" || runID == "" {
		return nil
	}
	now := time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(&NotificationOutbox{}).
		Where(
			"space_id = ? AND device_id = ? AND run_id = ? AND type = ? AND status IN ?",
			spaceID,
			deviceID,
			runID,
			"liveactivity.start",
			[]string{"queued", "retry", "processing"},
		).
		Updates(map[string]any{
			"status":     "failed",
			"last_error": "superseded by live activity end",
			"updated_at": now,
		}).Error
}

func (r *Repository) RecoverOutbox(ctx context.Context) error {
	if r == nil || r.db == nil {
		return errors.New("notification repository unavailable")
	}
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("ended_at < ?", now.Add(-24*time.Hour)).
			Delete(&NotificationRunTerminal{}).Error; err != nil {
			return err
		}
		return tx.Model(&NotificationOutbox{}).
			Where("status = ? AND expires_at > ?", "processing", now).
			Updates(map[string]any{
				"status":          "queued",
				"next_attempt_at": now,
				"updated_at":      now,
			}).Error
	})
}

func (r *Repository) ListDueOutbox(ctx context.Context, limit int) ([]NotificationOutbox, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("notification repository unavailable")
	}
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	now := time.Now().UTC()
	var rows []NotificationOutbox
	err := r.db.WithContext(ctx).
		Where("status IN ? AND next_attempt_at <= ? AND expires_at > ?", []string{"queued", "retry"}, now, now).
		Order("next_attempt_at ASC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func (r *Repository) ClaimOutbox(ctx context.Context, id string) (*NotificationOutbox, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("notification repository unavailable")
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&NotificationOutbox{}).
		Where("id = ? AND status IN ?", strings.TrimSpace(id), []string{"queued", "retry"}).
		Updates(map[string]any{"status": "processing", "updated_at": now})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var row NotificationOutbox
	if err := r.db.WithContext(ctx).Where("id = ?", strings.TrimSpace(id)).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repository) UpdateOutbox(ctx context.Context, id string, values map[string]any) error {
	if r == nil || r.db == nil {
		return errors.New("notification repository unavailable")
	}
	update := make(map[string]any, len(values)+1)
	for key, value := range values {
		update[key] = value
	}
	update["updated_at"] = time.Now().UTC()
	return r.db.WithContext(ctx).Model(&NotificationOutbox{}).
		Where("id = ?", strings.TrimSpace(id)).
		Updates(update).Error
}

func (r *Repository) ExpireOutbox(ctx context.Context) error {
	if r == nil || r.db == nil {
		return errors.New("notification repository unavailable")
	}
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&NotificationOutbox{}).
		Where("status IN ? AND expires_at <= ?", []string{"queued", "retry", "processing"}, now).
		Updates(map[string]any{
			"status":     "failed",
			"last_error": "notification TTL expired",
			"updated_at": now,
		}).Error
}

func (r *Repository) HasRecentLiveActivityStart(
	ctx context.Context,
	spaceID, deviceID string,
	within time.Duration,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, errors.New("notification repository unavailable")
	}
	if within <= 0 {
		return false, nil
	}
	var count int64
	err := r.db.WithContext(ctx).
		Model(&DeliveryLog{}).
		Where(
			"space_id = ? AND device_id = ? AND provider = ? AND status = ? AND type IN ? AND accepted_at >= ?",
			strings.TrimSpace(spaceID),
			strings.TrimSpace(deviceID),
			"apns",
			"accepted",
			[]string{"run.start", "liveactivity.start"},
			time.Now().UTC().Add(-within),
		).
		Count(&count).Error
	return count > 0, err
}

func (r *Repository) CreateDelivery(ctx context.Context, row *DeliveryLog) error {
	if row.ID == "" {
		row.ID = uuid.NewString()
	}
	if row.QueuedAt.IsZero() {
		row.QueuedAt = time.Now().UTC()
	}
	return r.db.WithContext(ctx).Create(row).Error
}

func (r *Repository) UpdateDelivery(ctx context.Context, id string, values map[string]any) error {
	return r.db.WithContext(ctx).Model(&DeliveryLog{}).Where("id = ?", id).Updates(values).Error
}
