package continuity

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

type ConversationRoute struct {
	ID      string
	Channel string
	PeerID  string
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) InitSchema() error {
	if r == nil || r.db == nil {
		return errors.New("continuity: database is not configured")
	}
	migrator := r.db.Migrator()
	if !migrator.HasTable(&ThreadBinding{}) {
		if err := migrator.CreateTable(&ThreadBinding{}); err != nil {
			return err
		}
	}
	if err := r.db.AutoMigrate(&Thread{}, &ThreadEvent{}, &Wait{}); err != nil {
		return err
	}
	for _, statement := range []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS uidx_continuity_binding ON continuity_thread_bindings(thread_id, binding_type, binding_id)",
		"CREATE UNIQUE INDEX IF NOT EXISTS uidx_continuity_event_idempotency ON continuity_thread_events(idempotency_key)",
		"CREATE INDEX IF NOT EXISTS idx_continuity_waits_wake_due ON continuity_waits(wake_state, next_wake_at)",
		"CREATE INDEX IF NOT EXISTS idx_continuity_waits_due_status ON continuity_waits(wait_type, status, due_at)",
	} {
		if err := r.db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) CreateThread(thread *Thread) error {
	if thread == nil {
		return errors.New("continuity: thread is nil")
	}
	now := time.Now().UTC()
	if strings.TrimSpace(thread.ID) == "" {
		thread.ID = uuid.New().String()
	}
	thread.SpaceID = strings.TrimSpace(thread.SpaceID)
	thread.CharacterID = strings.TrimSpace(thread.CharacterID)
	thread.Title = strings.TrimSpace(thread.Title)
	thread.Goal = strings.TrimSpace(thread.Goal)
	if thread.SpaceID == "" || thread.Title == "" {
		return errors.New("continuity: thread spaceId and title are required")
	}
	if thread.Status == "" {
		thread.Status = ThreadStatusActive
	}
	if !ValidThreadStatus(thread.Status) {
		return fmt.Errorf("continuity: invalid thread status %q", thread.Status)
	}
	if thread.Confidence <= 0 {
		thread.Confidence = 1
	}
	if thread.Revision <= 0 {
		thread.Revision = 1
	}
	if thread.CreatedAt.IsZero() {
		thread.CreatedAt = now
	}
	if thread.UpdatedAt.IsZero() {
		thread.UpdatedAt = now
	}
	if thread.LastActiveAt.IsZero() {
		thread.LastActiveAt = now
	}
	if thread.Status.IsTerminal() && thread.CompletedAt == nil {
		thread.CompletedAt = &now
	}
	return r.db.Create(thread).Error
}

func (r *Repository) GetThread(id, spaceID string) (*Thread, error) {
	var item Thread
	q := r.db.Where("id = ?", strings.TrimSpace(id))
	if strings.TrimSpace(spaceID) != "" {
		q = q.Where("space_id = ?", strings.TrimSpace(spaceID))
	}
	err := q.First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &item, err
}

func (r *Repository) ListThreads(filter ListThreadsFilter) ([]Thread, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	q := r.db.Where("space_id = ?", strings.TrimSpace(filter.SpaceID))
	if v := strings.TrimSpace(filter.CharacterID); v != "" {
		q = q.Where("character_id = ? OR character_id = ''", v)
	}
	if len(filter.Status) > 0 {
		q = q.Where("status IN ?", filter.Status)
	}
	if v := strings.TrimSpace(filter.Query); v != "" {
		like := "%" + v + "%"
		q = q.Where("title LIKE ? OR goal LIKE ? OR summary LIKE ? OR current_state LIKE ? OR next_action LIKE ?", like, like, like, like, like)
	}
	var items []Thread
	err := q.Order("last_active_at DESC, updated_at DESC").Offset(filter.Offset).Limit(filter.Limit).Find(&items).Error
	return items, err
}

func (r *Repository) ListActiveThreads(spaceID, characterID string, limit int) ([]Thread, error) {
	return r.ListThreads(ListThreadsFilter{
		SpaceID:     spaceID,
		CharacterID: characterID,
		Status:      []ThreadStatus{ThreadStatusActive, ThreadStatusWaiting, ThreadStatusBlocked, ThreadStatusPaused},
		Limit:       limit,
	})
}

func (r *Repository) Bind(threadID, bindingType, bindingID, role, source string, confidence float64) error {
	threadID = strings.TrimSpace(threadID)
	bindingType = strings.TrimSpace(bindingType)
	bindingID = strings.TrimSpace(bindingID)
	if threadID == "" || bindingType == "" || bindingID == "" {
		return nil
	}
	if role == "" {
		role = "context"
	}
	if confidence <= 0 {
		confidence = 1
	}
	now := time.Now().UTC()
	item := ThreadBinding{ID: uuid.New().String(), ThreadID: threadID, BindingType: bindingType, BindingID: bindingID, Role: role, Confidence: confidence, Source: strings.TrimSpace(source), CreatedAt: now, LastActiveAt: now}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "thread_id"}, {Name: "binding_type"}, {Name: "binding_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"role": role, "confidence": confidence, "source": strings.TrimSpace(source), "last_active_at": now}),
	}).Create(&item).Error
}

func (r *Repository) ListBindings(threadID string) ([]ThreadBinding, error) {
	var items []ThreadBinding
	err := r.db.Where("thread_id = ?", strings.TrimSpace(threadID)).Order("last_active_at DESC").Find(&items).Error
	return items, err
}

func (r *Repository) MostRecentBindingID(threadID, bindingType string) (string, error) {
	var item ThreadBinding
	err := r.db.Where("thread_id = ? AND binding_type = ?", strings.TrimSpace(threadID), strings.TrimSpace(bindingType)).Order("last_active_at DESC").First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return item.BindingID, err
}

func (r *Repository) MostRecentBoundThread(spaceID, bindingType, bindingID string) (*Thread, error) {
	var item Thread
	statuses := []ThreadStatus{ThreadStatusActive, ThreadStatusWaiting, ThreadStatusBlocked, ThreadStatusPaused}
	err := r.db.Table("continuity_threads AS t").
		Select("t.*").
		Joins("JOIN continuity_thread_bindings AS b ON b.thread_id = t.id").
		Where("t.space_id = ? AND b.binding_type = ? AND b.binding_id = ? AND t.status IN ?", strings.TrimSpace(spaceID), strings.TrimSpace(bindingType), strings.TrimSpace(bindingID), statuses).
		Order("b.last_active_at DESC, t.last_active_at DESC").
		Limit(1).
		Scan(&item).Error
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(item.ID) == "" {
		return nil, nil
	}
	return &item, nil
}

type BindingLookup struct {
	Type string
	ID   string
}

func (r *Repository) FindThreadByBindings(spaceID string, lookups ...BindingLookup) (*Thread, error) {
	for _, lookup := range lookups {
		if strings.TrimSpace(lookup.Type) == "" || strings.TrimSpace(lookup.ID) == "" {
			continue
		}
		thread, err := r.MostRecentBoundThread(spaceID, lookup.Type, lookup.ID)
		if err != nil || thread != nil {
			return thread, err
		}
	}
	return nil, nil
}

func (r *Repository) TouchThread(id string) error {
	now := time.Now().UTC()
	return r.db.Model(&Thread{}).Where("id = ?", strings.TrimSpace(id)).Updates(map[string]interface{}{"last_active_at": now, "updated_at": now}).Error
}

func (r *Repository) AppendEvent(event *ThreadEvent) (bool, error) {
	if event == nil {
		return false, errors.New("continuity: event is nil")
	}
	if strings.TrimSpace(event.ThreadID) == "" || strings.TrimSpace(event.IdempotencyKey) == "" {
		return false, errors.New("continuity: threadId and idempotencyKey are required")
	}
	if strings.TrimSpace(event.ID) == "" {
		event.ID = uuid.New().String()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if strings.TrimSpace(event.PayloadJSON) == "" {
		event.PayloadJSON = "{}"
	}
	result := r.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "idempotency_key"}}, DoNothing: true}).Create(event)
	return result.RowsAffected > 0, result.Error
}

func (r *Repository) GetEventByIdempotencyKey(key string) (*ThreadEvent, error) {
	var item ThreadEvent
	result := r.db.Where("idempotency_key = ?", strings.TrimSpace(key)).Limit(1).Find(&item)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &item, nil
}

func (r *Repository) ListRecentEvents(threadID string, limit int) ([]ThreadEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var items []ThreadEvent
	err := r.db.Where("thread_id = ?", strings.TrimSpace(threadID)).Order("occurred_at DESC").Limit(limit).Find(&items).Error
	return items, err
}

func (r *Repository) GetWait(id string) (*Wait, error) {
	var item Wait
	err := r.db.Where("id = ?", strings.TrimSpace(id)).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &item, err
}

func (r *Repository) ListWaits(threadID string, status *WaitStatus, limit int) ([]Wait, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	q := r.db.Where("thread_id = ?", strings.TrimSpace(threadID))
	if status != nil {
		q = q.Where("status = ?", *status)
	}
	var items []Wait
	err := q.Order("created_at DESC").Limit(limit).Find(&items).Error
	return items, err
}

func (r *Repository) ListOpenWaits(threadID string, limit int) ([]Wait, error) {
	status := WaitStatusWaiting
	return r.ListWaits(threadID, &status, limit)
}

func (r *Repository) ListWaitingWaitsByType(waitType, spaceID string, offset, limit int) ([]Wait, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := r.db.Table("continuity_waits AS w").
		Select("w.*").
		Joins("JOIN continuity_threads AS t ON t.id = w.thread_id").
		Where("w.status = ? AND w.wait_type = ?", WaitStatusWaiting, strings.TrimSpace(waitType))
	if strings.TrimSpace(spaceID) != "" {
		q = q.Where("t.space_id = ?", strings.TrimSpace(spaceID))
	}
	var items []Wait
	err := q.Order("w.created_at ASC, w.id ASC").Offset(offset).Limit(limit).Scan(&items).Error
	return items, err
}

func (r *Repository) GetConversationRoute(conversationID string) (*ConversationRoute, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, nil
	}
	if !r.db.Migrator().HasTable("conversations") {
		return nil, nil
	}
	var route ConversationRoute
	err := r.db.Table("conversations").
		Select("id, channel, peer_id").
		Where("id = ?", conversationID).
		Take(&route).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &route, err
}

func (r *Repository) CreateWait(wait *Wait) error {
	if wait == nil {
		return errors.New("continuity: wait is nil")
	}
	wait.ThreadID = strings.TrimSpace(wait.ThreadID)
	wait.WaitType = strings.TrimSpace(wait.WaitType)
	if wait.ThreadID == "" || !ValidWaitType(wait.WaitType) {
		return errors.New("continuity: wait threadId and valid waitType are required")
	}
	now := time.Now().UTC()
	if strings.TrimSpace(wait.ID) == "" {
		wait.ID = uuid.New().String()
	}
	if wait.Status == "" {
		wait.Status = WaitStatusWaiting
	}
	if strings.TrimSpace(wait.ConditionJSON) == "" {
		wait.ConditionJSON = "{}"
	}
	if strings.TrimSpace(wait.ResolutionJSON) == "" {
		wait.ResolutionJSON = "{}"
	}
	if wait.CreatedAt.IsZero() {
		wait.CreatedAt = now
	}
	wait.UpdatedAt = now
	return r.db.Create(wait).Error
}

func (r *Repository) ResolveOpenWaits(threadID, waitType string) error {
	now := time.Now().UTC()
	q := r.db.Model(&Wait{}).Where("thread_id = ? AND status = ?", strings.TrimSpace(threadID), WaitStatusWaiting)
	if strings.TrimSpace(waitType) != "" {
		q = q.Where("wait_type = ?", strings.TrimSpace(waitType))
	}
	return q.Updates(map[string]interface{}{"status": WaitStatusResolved, "resolved_at": now, "resolved_by": "legacy", "updated_at": now}).Error
}

func (r *Repository) ListDueTimeWaits(now time.Time, limit int) ([]Wait, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var items []Wait
	err := r.db.Where("status = ? AND wait_type = ? AND due_at IS NOT NULL AND due_at <= ?", WaitStatusWaiting, WaitTypeTime, now.UTC()).Order("due_at ASC").Limit(limit).Find(&items).Error
	return items, err
}

func (r *Repository) ListPendingWakes(now time.Time, limit int) ([]Wait, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var items []Wait
	err := r.db.Where("status = ? AND wake_state IN ? AND (next_wake_at IS NULL OR next_wake_at <= ?)", WaitStatusResolved, []string{WakeStatePending, WakeStateFailed}, now.UTC()).Order("next_wake_at ASC, updated_at ASC").Limit(limit).Find(&items).Error
	return items, err
}

func (r *Repository) UpdateWait(id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	updates["updated_at"] = time.Now().UTC()
	return r.db.Model(&Wait{}).Where("id = ?", strings.TrimSpace(id)).Updates(updates).Error
}

func (r *Repository) CancelOpenWaits(threadID, resolvedBy string) error {
	now := time.Now().UTC()
	return r.db.Model(&Wait{}).
		Where("thread_id = ? AND (status = ? OR wake_state IN ?)", strings.TrimSpace(threadID), WaitStatusWaiting, []string{WakeStatePending, WakeStateFailed}).
		Updates(map[string]interface{}{
			"status": WaitStatusCancelled, "resolved_at": now, "resolved_by": strings.TrimSpace(resolvedBy),
			"wake_state": WakeStateSkipped, "next_wake_at": nil, "updated_at": now,
		}).Error
}

func (r *Repository) UpdateThreadCAS(id string, expectedRevision int64, updates map[string]interface{}) (*Thread, error) {
	if updates == nil {
		updates = map[string]interface{}{}
	}
	updates["updated_at"] = time.Now().UTC()
	updates["revision"] = gorm.Expr("revision + 1")
	result := r.db.Model(&Thread{}).Where("id = ? AND revision = ?", strings.TrimSpace(id), expectedRevision).Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return r.GetThread(id, "")
}

func (r *Repository) UpdateThread(id, spaceID string, updates map[string]interface{}) (*Thread, error) {
	thread, err := r.GetThread(id, spaceID)
	if err != nil || thread == nil {
		return thread, err
	}
	return r.UpdateThreadCAS(thread.ID, thread.Revision, updates)
}

func (r *Repository) WithTransaction(fn func(tx *Repository) error) error {
	if r == nil || r.db == nil {
		return errors.New("continuity: database is not configured")
	}
	return r.db.Transaction(func(dbtx *gorm.DB) error { return fn(NewRepository(dbtx)) })
}
