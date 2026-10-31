package safety

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"gorm.io/gorm"
	amitialog "log"
)

type Incident struct {
	ID             string `json:"id"`
	SpaceID        string `json:"spaceId"`
	OwnerID        string `json:"ownerId"`
	CoreID         string `json:"coreId"`
	RoleID         string `json:"roleId"`
	ConversationID string `json:"conversationId"`
	MessageID      string `json:"messageId"`
	FeedbackID     *int64 `json:"feedbackId,omitempty"`
	RequestID      string `json:"requestId"`
	EventType      string `json:"eventType"`
	Severity       string `json:"severity"`
	Description    string `json:"description"`
	Status         string `json:"status"`
	PolicyRevision int64  `json:"policyRevision"`
	Revision       int64  `json:"revision"`
	DetailsJSON    string `json:"details"`
	LegacyEventID  string `json:"-"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
	ResolvedAt     string `json:"resolvedAt,omitempty"`
	ResolvedBy     string `json:"resolvedBy,omitempty"`
}

func (Incident) TableName() string {
	return "safety_incidents"
}

type IncidentRecorder interface {
	RecordIncident(context.Context, Incident) error
}

func (r *Repository) RecordIncident(ctx context.Context, incident Incident) error {
	if r == nil || r.db == nil {
		return errors.New("安全事件存储不可用")
	}
	if strings.TrimSpace(incident.SpaceID) == "" || strings.TrimSpace(incident.OwnerID) == "" || strings.TrimSpace(incident.CoreID) == "" || strings.TrimSpace(incident.EventType) == "" {
		return errors.New("安全事件归属不完整")
	}
	if _, scoped := coordination.FromContext(ctx); scoped {
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return err
		}
	}
	if incident.ID == "" {
		incident.ID = "safety-incident-" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "-")
	}
	if incident.Status == "" {
		incident.Status = "open"
	}
	if incident.Severity == "" {
		incident.Severity = "medium"
	}
	if incident.Revision < 1 {
		incident.Revision = 1
	}
	if incident.DetailsJSON == "" || !json.Valid([]byte(incident.DetailsJSON)) {
		incident.DetailsJSON = "{}"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if incident.CreatedAt == "" {
		incident.CreatedAt = now
	}
	incident.UpdatedAt = now
	return r.db.WithContext(ctx).Exec(`INSERT INTO safety_incidents(id,space_id,owner_id,core_id,role_id,conversation_id,message_id,feedback_id,request_id,event_type,severity,description,status,policy_revision,revision,details_json,legacy_event_id,created_at,updated_at,resolved_at,resolved_by) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(feedback_id) WHERE feedback_id IS NOT NULL DO NOTHING`,
		incident.ID, incident.SpaceID, incident.OwnerID, incident.CoreID, incident.RoleID, incident.ConversationID, incident.MessageID, incident.FeedbackID, incident.RequestID, incident.EventType, incident.Severity, incident.Description, incident.Status, incident.PolicyRevision, incident.Revision, incident.DetailsJSON, incident.LegacyEventID, incident.CreatedAt, incident.UpdatedAt, incident.ResolvedAt, incident.ResolvedBy).Error
}

func (r *Repository) ListIncidents(ctx context.Context, scope PolicyScope, page, pageSize int, status string) ([]Incident, int64, error) {
	if err := validatePolicyScope(scope); err != nil {
		return nil, 0, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, 0, err
	}
	policy, err := r.ensurePolicy(ctx, scope)
	if err != nil {
		return nil, 0, err
	}
	retentionDays := policy.Config.AuditLogRetentionDays
	if retentionDays > 0 {
		cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays).Format(time.RFC3339Nano)
		if err := r.db.WithContext(ctx).Where("space_id=? AND owner_id=? AND core_id=? AND status=? AND resolved_at<>'' AND resolved_at<?", scope.SpaceID, scope.OwnerID, scope.CoreID, "resolved", cutoff).Delete(&Incident{}).Error; err != nil {
			return nil, 0, err
		}
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	query := r.db.WithContext(ctx).Model(&Incident{}).Where("space_id=? AND owner_id=? AND core_id=?", scope.SpaceID, scope.OwnerID, scope.CoreID)
	if strings.TrimSpace(status) != "" {
		query = query.Where("status=?", strings.TrimSpace(status))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []Incident
	if err := query.Order("created_at DESC,id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *Repository) AllIncidents(ctx context.Context, scope PolicyScope) ([]Incident, error) {
	if err := validatePolicyScope(scope); err != nil {
		return nil, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	items := []Incident{}
	err := r.db.WithContext(ctx).Where("space_id=? AND owner_id=? AND core_id=?", scope.SpaceID, scope.OwnerID, scope.CoreID).Order("created_at,id").Find(&items).Error
	return items, err
}

func (r *Repository) HandleIncident(ctx context.Context, scope PolicyScope, id string, expectedRevision int64, resolvedBy string) (Incident, error) {
	if err := validatePolicyScope(scope); err != nil {
		return Incident{}, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return Incident{}, err
	}
	if strings.TrimSpace(id) == "" || expectedRevision < 1 {
		return Incident{}, errors.New("安全事件版本无效")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result := r.db.WithContext(ctx).Model(&Incident{}).Where("id=? AND space_id=? AND owner_id=? AND core_id=? AND revision=?", id, scope.SpaceID, scope.OwnerID, scope.CoreID, expectedRevision).Updates(map[string]any{"status": "resolved", "revision": gorm.Expr("revision + 1"), "resolved_at": now, "resolved_by": strings.TrimSpace(resolvedBy), "updated_at": now})
	if result.Error != nil {
		return Incident{}, result.Error
	}
	if result.RowsAffected != 1 {
		return Incident{}, gorm.ErrRecordNotFound
	}
	var incident Incident
	if err := r.db.WithContext(ctx).Where("id=? AND space_id=? AND owner_id=? AND core_id=?", id, scope.SpaceID, scope.OwnerID, scope.CoreID).Take(&incident).Error; err != nil {
		return Incident{}, err
	}
	return incident, nil
}

func (r *Repository) DeleteIncidents(ctx context.Context, scope PolicyScope) (int64, error) {
	if err := validatePolicyScope(scope); err != nil {
		return 0, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return 0, err
	}
	result := r.db.WithContext(ctx).Where("space_id=? AND owner_id=? AND core_id=?", scope.SpaceID, scope.OwnerID, scope.CoreID).Delete(&Incident{})
	return result.RowsAffected, result.Error
}

func (r *Repository) MigrateLegacyEvents(ctx context.Context, scope PolicyScope) error {
	if err := validatePolicyScope(scope); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if !tx.Migrator().HasTable("safety_events") {
			return nil
		}
		var rows []struct {
			ID             string `gorm:"column:id"`
			ConversationID string `gorm:"column:conversation_id"`
			EventType      string `gorm:"column:event_type"`
			Description    string `gorm:"column:description"`
			Direction      string `gorm:"column:direction"`
			Handled        int    `gorm:"column:handled"`
			CreatedAt      string `gorm:"column:created_at"`
		}
		if err := tx.Table("safety_events").Select("id,conversation_id,event_type,description,direction,handled,created_at").Order("created_at,id").Scan(&rows).Error; err != nil {
			return err
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		for _, row := range rows {
			status, resolvedAt := "open", ""
			if row.Handled != 0 {
				status, resolvedAt = "resolved", now
			}
			var roleID string
			if err := tx.Table("messages").Select("character_id").Where("conversation_id=? AND character_id<>''", row.ConversationID).Limit(1).Scan(&roleID).Error; err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			if row.EventType == "" {
				row.EventType = "legacy_safety_event"
			}
			if row.CreatedAt == "" {
				row.CreatedAt = now
			}
			details, _ := json.Marshal(map[string]any{"source": "legacy.safety_events", "direction": row.Direction})
			if err := tx.Exec(`INSERT INTO safety_incidents(id,space_id,owner_id,core_id,role_id,conversation_id,message_id,feedback_id,request_id,event_type,severity,description,status,policy_revision,revision,details_json,legacy_event_id,created_at,updated_at,resolved_at,resolved_by) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(legacy_event_id) WHERE legacy_event_id <> '' DO NOTHING`,
				"legacy-safety/"+row.ID, scope.SpaceID, scope.OwnerID, scope.CoreID, roleID, row.ConversationID, "", nil, "", row.EventType, "medium", row.Description, status, 0, 1, string(details), row.ID, row.CreatedAt, now, resolvedAt, "").Error; err != nil {
				return err
			}
		}
		return tx.Migrator().DropTable("safety_events")
	})
}
