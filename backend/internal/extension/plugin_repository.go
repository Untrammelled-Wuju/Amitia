package extension

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type pluginStateRecord struct {
	ID, ExtensionID, ScopeType, ScopeID, SchemaVersion, StateJSON, CreatedAt, UpdatedAt string
	Revision                                                                            int64
}

func (pluginStateRecord) TableName() string { return "extension_states" }

type pluginStateRevisionRecord struct {
	ID, ExtensionID, ScopeType, ScopeID, SchemaVersion, StateJSON, CreatedAt string
	Revision                                                                 int64
}

func (pluginStateRevisionRecord) TableName() string { return "extension_state_revisions" }

type pluginEventRecord struct {
	ID, EventID, Source, Type, Subject, DataJSON, TraceID, CorrelationID, CausationID, CreatedAt string
	Depth                                                                                        int
}

func (pluginEventRecord) TableName() string { return "extension_events" }

type pluginDeliveryRecord struct {
	ID, EventID, PluginID, Status, NextAttemptAt, LastErrorCode, LastErrorDetail, ProcessedAt, CreatedAt, UpdatedAt string
	Attempts                                                                                                        int
}

func (pluginDeliveryRecord) TableName() string { return "extension_event_deliveries" }

type pluginScheduleRecord struct {
	ID, PluginID, ScheduleID, ScopeType, ScopeID, ScheduleType, Expression, Timezone, PayloadJSON, NextRunAt, LastRunAt, LastStatus, CreatedAt, UpdatedAt string
	Enabled                                                                                                                                               int
}

func (pluginScheduleRecord) TableName() string { return "extension_schedules" }

type pluginRunRecord struct {
	RunID, PluginID, PluginVersion, Hook, CharacterID, ConversationID, Channel, Status, ErrorCode, TraceID, CircuitState, CreatedAt string
	DurationMS                                                                                                                      int64
}

func (pluginRunRecord) TableName() string { return "extension_plugin_runs" }

type pluginAuditRecord struct {
	ID, ExtensionID, Action, ScopeType, ScopeID, DetailJSON, TraceID, CreatedAt string
}

func (pluginAuditRecord) TableName() string { return "extension_audits" }

func (r *Repository) UpsertPlugin(ctx context.Context, registered RegisteredPlugin, lifecycle PluginLifecycleStatus, health string) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	manifest := registered.Manifest
	var existing extensionRecord
	err := r.db.WithContext(ctx).Where("extension_id = ?", manifest.Metadata.ID).First(&existing).Error
	enabled := manifest.Enabled
	createdAt := now
	id := uuid.NewString()
	if err == nil {
		enabled = existing.Enabled == 1
		createdAt = existing.CreatedAt
		id = existing.ID
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	record := extensionRecord{ID: id, ExtensionID: manifest.Metadata.ID, Kind: "Plugin", Name: manifest.Metadata.Name, CurrentVersion: manifest.Metadata.Version, Source: "builtin", Enabled: boolNumber(enabled), ManifestJSON: string(registered.RawManifest), NormalizedManifestJSON: string(registered.NormalizedManifest), CreatedAt: createdAt, UpdatedAt: now}
	updates := map[string]any{"kind": "Plugin", "name": record.Name, "current_version": record.CurrentVersion, "source": "builtin", "manifest_json": record.ManifestJSON, "normalized_manifest_json": record.NormalizedManifestJSON, "lifecycle_status": string(lifecycle), "health_status": health, "updated_at": now}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "extension_id"}}, DoUpdates: clause.Assignments(updates)}).Create(&record).Error; err != nil {
		return false, err
	}
	normalized, _ := normalizeRawJSON(registered.RawManifest)
	hash := sha256.Sum256(normalized)
	version := extensionVersionRecord{ID: uuid.NewString(), ExtensionID: manifest.Metadata.ID, Version: manifest.Metadata.Version, ManifestJSON: string(normalized), Checksum: fmt.Sprintf("%x", hash[:]), CreatedAt: now}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&version).Error; err != nil {
		return false, err
	}
	return enabled, nil
}

func (r *Repository) UpdatePluginLifecycle(ctx context.Context, pluginID string, enabled bool, lifecycle PluginLifecycleStatus, health, errorCode string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	updates := map[string]any{"enabled": boolNumber(enabled), "lifecycle_status": string(lifecycle), "health_status": health, "last_error_code": errorCode, "updated_at": now}
	if errorCode != "" {
		updates["last_error_at"] = now
	}
	if enabled {
		updates["enabled_at"] = now
	} else if lifecycle == PluginDisabled {
		updates["disabled_at"] = now
	}
	result := r.db.WithContext(ctx).Model(&extensionRecord{}).Where("extension_id = ? AND kind = 'Plugin'", pluginID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return NewExtensionError(ErrPluginNotFound, "Plugin not found", pluginID, false, nil)
	}
	return nil
}

func (r *Repository) PluginRecord(ctx context.Context, pluginID string) (extensionRecord, error) {
	var record extensionRecord
	err := r.db.WithContext(ctx).Where("extension_id = ? AND kind = 'Plugin'", pluginID).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return record, NewExtensionError(ErrPluginNotFound, "Plugin not found", pluginID, false, nil)
	}
	return record, err
}

func (r *Repository) ReadPluginState(ctx context.Context, pluginID string, scope PluginStateScope, defaults json.RawMessage, schemaVersion string) (PluginState, error) {
	if r.configCipherErr != nil {
		return PluginState{}, r.configCipherErr
	}
	if err := validatePluginStateScope(scope); err != nil {
		return PluginState{}, err
	}
	var record pluginStateRecord
	err := r.db.WithContext(ctx).Where("extension_id = ? AND scope_type = ? AND scope_id = ?", pluginID, scope.Type, scope.ID).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PluginState{PluginID: pluginID, ScopeType: scope.Type, ScopeID: scope.ID, SchemaVersion: schemaVersion, Revision: 0, Data: append(json.RawMessage(nil), normalizeJSON(defaults)...)}, nil
	}
	if err != nil {
		return PluginState{}, err
	}
	plain, _, err := r.configCipher.decrypt(record.StateJSON)
	if err != nil {
		return PluginState{}, err
	}
	return PluginState{PluginID: pluginID, ScopeType: ScopeType(record.ScopeType), ScopeID: record.ScopeID, SchemaVersion: record.SchemaVersion, Revision: record.Revision, Data: json.RawMessage(plain), UpdatedAt: record.UpdatedAt}, nil
}

func (r *Repository) CompareAndSwapPluginState(ctx context.Context, pluginID, schemaVersion string, request WritePluginStateRequest) (PluginState, error) {
	if r.configCipherErr != nil {
		return PluginState{}, r.configCipherErr
	}
	if err := validatePluginStateScope(request.Scope); err != nil {
		return PluginState{}, err
	}
	if len(request.Data) > 262144 {
		return PluginState{}, NewExtensionError(ErrPluginStateInvalid, "Plugin state is too large", pluginID, false, nil)
	}
	secured, err := r.configCipher.encrypt(request.Data)
	if err != nil {
		return PluginState{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var output PluginState
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing pluginStateRecord
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("extension_id = ? AND scope_type = ? AND scope_id = ?", pluginID, request.Scope.Type, request.Scope.ID).First(&existing).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			if request.ExpectedRevision != 0 {
				return NewExtensionError(ErrPluginStateConflict, "Plugin state revision conflict", pluginID, false, nil)
			}
			record := pluginStateRecord{ID: uuid.NewString(), ExtensionID: pluginID, ScopeType: string(request.Scope.Type), ScopeID: request.Scope.ID, SchemaVersion: schemaVersion, Revision: 1, StateJSON: secured, CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
			output = PluginState{PluginID: pluginID, ScopeType: request.Scope.Type, ScopeID: request.Scope.ID, SchemaVersion: schemaVersion, Revision: 1, Data: append(json.RawMessage(nil), request.Data...), UpdatedAt: now}
			return nil
		}
		if findErr != nil {
			return findErr
		}
		if existing.Revision != request.ExpectedRevision {
			return NewExtensionError(ErrPluginStateConflict, "Plugin state revision conflict", pluginID, false, nil)
		}
		backup := pluginStateRevisionRecord{ID: uuid.NewString(), ExtensionID: pluginID, ScopeType: existing.ScopeType, ScopeID: existing.ScopeID, SchemaVersion: existing.SchemaVersion, Revision: existing.Revision, StateJSON: existing.StateJSON, CreatedAt: now}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&backup).Error; err != nil {
			return err
		}
		result := tx.Model(&pluginStateRecord{}).Where("id = ? AND revision = ?", existing.ID, existing.Revision).Updates(map[string]any{"schema_version": schemaVersion, "revision": existing.Revision + 1, "state_json": secured, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return NewExtensionError(ErrPluginStateConflict, "Plugin state revision conflict", pluginID, false, nil)
		}
		output = PluginState{PluginID: pluginID, ScopeType: request.Scope.Type, ScopeID: request.Scope.ID, SchemaVersion: schemaVersion, Revision: existing.Revision + 1, Data: append(json.RawMessage(nil), request.Data...), UpdatedAt: now}
		return nil
	})
	return output, err
}

func (r *Repository) ListPluginStates(ctx context.Context, pluginID, characterID string) ([]PluginState, error) {
	if r.configCipherErr != nil {
		return nil, r.configCipherErr
	}
	var records []pluginStateRecord
	query := r.db.WithContext(ctx).Where("extension_id = ?", pluginID)
	if characterID != "" {
		query = query.Where("scope_type = 'global' OR (scope_type = 'character' AND scope_id = ?)", characterID)
	} else {
		query = query.Where("scope_type = 'global'")
	}
	if err := query.Order("scope_type, scope_id").Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]PluginState, 0, len(records))
	for _, record := range records {
		plain, _, err := r.configCipher.decrypt(record.StateJSON)
		if err != nil {
			return nil, err
		}
		items = append(items, PluginState{PluginID: pluginID, ScopeType: ScopeType(record.ScopeType), ScopeID: record.ScopeID, SchemaVersion: record.SchemaVersion, Revision: record.Revision, Data: redactJSON(json.RawMessage(plain)), UpdatedAt: record.UpdatedAt})
	}
	return items, nil
}

func (r *Repository) AllPluginStates(ctx context.Context, pluginID string) ([]PluginState, error) {
	if r.configCipherErr != nil {
		return nil, r.configCipherErr
	}
	var records []pluginStateRecord
	if err := r.db.WithContext(ctx).Where("extension_id = ?", pluginID).Order("scope_type, scope_id").Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]PluginState, 0, len(records))
	for _, record := range records {
		plain, _, err := r.configCipher.decrypt(record.StateJSON)
		if err != nil {
			return nil, err
		}
		items = append(items, PluginState{PluginID: pluginID, ScopeType: ScopeType(record.ScopeType), ScopeID: record.ScopeID, SchemaVersion: record.SchemaVersion, Revision: record.Revision, Data: json.RawMessage(plain), UpdatedAt: record.UpdatedAt})
	}
	return items, nil
}

func (r *Repository) CreatePluginEvent(ctx context.Context, event ExtensionEvent, pluginIDs []string) error {
	if len(event.Data) > 131072 {
		return NewExtensionError(ErrPluginEventInvalid, "Plugin event is too large", event.Type, false, nil)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := pluginEventRecord{ID: uuid.NewString(), EventID: event.ID, Source: event.Source, Type: event.Type, Subject: event.Subject, DataJSON: string(redactJSON(event.Data)), TraceID: event.TraceID, CorrelationID: event.CorrelationID, CausationID: event.CausationID, Depth: event.Depth, CreatedAt: now}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&record)
		if result.Error != nil {
			return result.Error
		}
		for _, pluginID := range pluginIDs {
			delivery := pluginDeliveryRecord{ID: uuid.NewString(), EventID: event.ID, PluginID: pluginID, Status: "pending", NextAttemptAt: now, CreatedAt: now, UpdatedAt: now}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&delivery).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) PendingPluginDeliveries(ctx context.Context, limit int) ([]pluginDeliveryRecord, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	var records []pluginDeliveryRecord
	now := time.Now().UTC().Format(time.RFC3339Nano)
	err := r.db.WithContext(ctx).Where("status IN ('pending','failed') AND next_attempt_at <= ?", now).Order("created_at").Limit(limit).Find(&records).Error
	return records, err
}

func (r *Repository) PluginEvent(ctx context.Context, eventID string) (ExtensionEvent, error) {
	var record pluginEventRecord
	if err := r.db.WithContext(ctx).Where("event_id = ?", eventID).First(&record).Error; err != nil {
		return ExtensionEvent{}, err
	}
	created, _ := time.Parse(time.RFC3339Nano, record.CreatedAt)
	return ExtensionEvent{SpecVersion: "1.0", ID: record.EventID, Source: record.Source, Type: record.Type, Subject: record.Subject, Time: created, DataContentType: "application/json", Data: json.RawMessage(record.DataJSON), TraceID: record.TraceID, CorrelationID: record.CorrelationID, CausationID: record.CausationID, Depth: record.Depth}, nil
}

func (r *Repository) UpdatePluginDelivery(ctx context.Context, delivery pluginDeliveryRecord, status, code, detail string, next time.Time) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	updates := map[string]any{"status": status, "attempts": delivery.Attempts + 1, "last_error_code": code, "last_error_detail": detail, "updated_at": now}
	if !next.IsZero() {
		updates["next_attempt_at"] = next.UTC().Format(time.RFC3339Nano)
	}
	if status == "completed" || status == "dead_letter" {
		updates["processed_at"] = now
	}
	return r.db.WithContext(ctx).Model(&pluginDeliveryRecord{}).Where("id = ?", delivery.ID).Updates(updates).Error
}

func (r *Repository) ListPluginEvents(ctx context.Context, pluginID, characterID, status string, page, pageSize int) ([]ExtensionEvent, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	query := r.db.WithContext(ctx).Table("extension_events e").Joins("JOIN extension_event_deliveries d ON d.event_id = e.event_id").Where("d.plugin_id = ?", pluginID)
	if characterID != "" {
		query = query.Where("(e.subject = '' OR e.subject LIKE ?)", "%character/"+characterID+"%")
	}
	if status != "" {
		query = query.Where("d.status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []pluginEventRecord
	if err := query.Select("e.*").Order("e.created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&records).Error; err != nil {
		return nil, 0, err
	}
	items := make([]ExtensionEvent, 0, len(records))
	for _, record := range records {
		created, _ := time.Parse(time.RFC3339Nano, record.CreatedAt)
		items = append(items, ExtensionEvent{SpecVersion: "1.0", ID: record.EventID, Source: record.Source, Type: record.Type, Subject: record.Subject, Time: created, DataContentType: "application/json", Data: json.RawMessage(record.DataJSON), TraceID: record.TraceID, CorrelationID: record.CorrelationID, CausationID: record.CausationID, Depth: record.Depth})
	}
	return items, total, nil
}

func (r *Repository) RetryPluginEvent(ctx context.Context, pluginID, eventID string) error {
	result := r.db.WithContext(ctx).Model(&pluginDeliveryRecord{}).Where("plugin_id = ? AND event_id = ? AND status = 'dead_letter'", pluginID, eventID).Updates(map[string]any{"status": "pending", "attempts": 0, "next_attempt_at": time.Now().UTC().Format(time.RFC3339Nano), "last_error_code": "", "last_error_detail": "", "processed_at": ""})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return NewExtensionError(ErrPluginEventDeadLetter, "Dead-letter event not found", eventID, false, nil)
	}
	return nil
}

func (r *Repository) UpsertPluginSchedule(ctx context.Context, pluginID string, definition PluginScheduleDefinition) error {
	var existing pluginScheduleRecord
	findErr := r.db.WithContext(ctx).Where("plugin_id = ? AND schedule_id = ?", pluginID, definition.ScheduleID).First(&existing).Error
	if findErr == nil && (existing.ScopeType != string(definition.Scope.Type) || existing.ScopeID != definition.Scope.ID) {
		return NewExtensionError(ErrPluginScheduleInvalid, "Plugin schedule ID already belongs to another scope", definition.ScheduleID, false, nil)
	}
	if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return findErr
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := pluginScheduleRecord{ID: uuid.NewString(), PluginID: pluginID, ScheduleID: definition.ScheduleID, ScopeType: string(definition.Scope.Type), ScopeID: definition.Scope.ID, ScheduleType: definition.Type, Expression: definition.Expression, Timezone: definition.Timezone, PayloadJSON: compactSensitiveJSON(definition.Payload), Enabled: boolNumber(definition.Enabled), NextRunAt: definition.NextRunAt, CreatedAt: now, UpdatedAt: now}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "plugin_id"}, {Name: "schedule_id"}, {Name: "scope_type"}, {Name: "scope_id"}}, DoUpdates: clause.AssignmentColumns([]string{"schedule_type", "expression", "timezone", "payload_json", "enabled", "next_run_at", "updated_at"})}).Create(&record).Error
}

func (r *Repository) DeletePluginSchedule(ctx context.Context, pluginID, scheduleID string) error {
	return r.db.WithContext(ctx).Where("plugin_id = ? AND schedule_id = ?", pluginID, scheduleID).Delete(&pluginScheduleRecord{}).Error
}

func (r *Repository) SetPluginScheduleEnabled(ctx context.Context, pluginID, scheduleID string, enabled bool) error {
	result := r.db.WithContext(ctx).Model(&pluginScheduleRecord{}).Where("plugin_id = ? AND schedule_id = ?", pluginID, scheduleID).Updates(map[string]any{"enabled": boolNumber(enabled), "updated_at": time.Now().UTC().Format(time.RFC3339Nano)})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return NewExtensionError(ErrPluginScheduleInvalid, "Plugin schedule not found", scheduleID, false, nil)
	}
	return nil
}

func (r *Repository) PluginScheduleScope(ctx context.Context, pluginID, scheduleID string) (PluginStateScope, error) {
	var record pluginScheduleRecord
	if err := r.db.WithContext(ctx).Where("plugin_id = ? AND schedule_id = ?", pluginID, scheduleID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PluginStateScope{}, NewExtensionError(ErrPluginScheduleInvalid, "Plugin schedule not found", scheduleID, false, nil)
		}
		return PluginStateScope{}, err
	}
	return PluginStateScope{Type: ScopeType(record.ScopeType), ID: record.ScopeID}, nil
}

func (r *Repository) ListPluginSchedules(ctx context.Context, pluginID string) ([]PluginScheduleDefinition, error) {
	var records []pluginScheduleRecord
	if err := r.db.WithContext(ctx).Where("plugin_id = ?", pluginID).Order("schedule_id, scope_type, scope_id").Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]PluginScheduleDefinition, 0, len(records))
	for _, record := range records {
		items = append(items, scheduleDefinitionFromRecord(record))
	}
	return items, nil
}

func (r *Repository) DuePluginSchedules(ctx context.Context, limit int) ([]pluginScheduleRecord, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	var records []pluginScheduleRecord
	err := r.db.WithContext(ctx).Where("enabled = 1 AND next_run_at != '' AND next_run_at <= ?", time.Now().UTC().Format(time.RFC3339Nano)).Order("next_run_at").Limit(limit).Find(&records).Error
	return records, err
}

func (r *Repository) CompletePluginSchedule(ctx context.Context, record pluginScheduleRecord, status, nextRunAt string) error {
	return r.db.WithContext(ctx).Model(&pluginScheduleRecord{}).Where("id = ?", record.ID).Updates(map[string]any{"last_run_at": time.Now().UTC().Format(time.RFC3339Nano), "last_status": status, "next_run_at": nextRunAt, "enabled": boolNumber(nextRunAt != ""), "updated_at": time.Now().UTC().Format(time.RFC3339Nano)}).Error
}

func (r *Repository) CreatePluginRun(ctx context.Context, run PluginRunView) error {
	record := pluginRunRecord{RunID: run.RunID, PluginID: run.PluginID, PluginVersion: run.PluginVersion, Hook: string(run.Hook), CharacterID: run.CharacterID, ConversationID: run.ConversationID, Channel: run.Channel, Status: run.Status, DurationMS: run.DurationMS, ErrorCode: run.ErrorCode, TraceID: run.TraceID, CircuitState: string(run.CircuitState), CreatedAt: run.CreatedAt}
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *Repository) ListPluginRuns(ctx context.Context, pluginID, characterID string, limit int) ([]PluginRunView, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	var records []pluginRunRecord
	query := r.db.WithContext(ctx).Where("plugin_id = ?", pluginID)
	if characterID != "" {
		query = query.Where("character_id = '' OR character_id = ?", characterID)
	} else {
		query = query.Where("character_id = ''")
	}
	if err := query.Order("created_at DESC").Limit(limit).Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]PluginRunView, 0, len(records))
	for _, record := range records {
		items = append(items, PluginRunView{RunID: record.RunID, PluginID: record.PluginID, PluginVersion: record.PluginVersion, Hook: PluginHook(record.Hook), CharacterID: record.CharacterID, ConversationID: record.ConversationID, Channel: record.Channel, Status: record.Status, DurationMS: record.DurationMS, ErrorCode: record.ErrorCode, TraceID: record.TraceID, CircuitState: CircuitState(record.CircuitState), CreatedAt: record.CreatedAt})
	}
	return items, nil
}

func (r *Repository) AuditPlugin(ctx context.Context, pluginID, action string, scope PluginStateScope, traceID string, detail any) error {
	raw, _ := json.Marshal(detail)
	record := pluginAuditRecord{ID: uuid.NewString(), ExtensionID: pluginID, Action: action, ScopeType: string(scope.Type), ScopeID: scope.ID, DetailJSON: compactSensitiveJSON(raw), TraceID: traceID, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	return r.db.WithContext(ctx).Create(&record).Error
}

func validatePluginStateScope(scope PluginStateScope) error {
	if scope.Type != ScopeGlobal && scope.Type != ScopeCharacter && scope.Type != ScopeConversation {
		return NewExtensionError(ErrPluginStateInvalid, "Plugin state scope is invalid", string(scope.Type), false, nil)
	}
	if scope.Type != ScopeGlobal && scope.ID == "" {
		return NewExtensionError(ErrPluginStateInvalid, "Plugin state scope ID is required", string(scope.Type), false, nil)
	}
	if scope.Type == ScopeGlobal && scope.ID != "" {
		return NewExtensionError(ErrPluginStateInvalid, "Global plugin state scope ID must be empty", scope.ID, false, nil)
	}
	return nil
}

func scheduleDefinitionFromRecord(record pluginScheduleRecord) PluginScheduleDefinition {
	return PluginScheduleDefinition{ScheduleID: record.ScheduleID, Scope: PluginStateScope{Type: ScopeType(record.ScopeType), ID: record.ScopeID}, Type: record.ScheduleType, Expression: record.Expression, Timezone: record.Timezone, Payload: json.RawMessage(record.PayloadJSON), Enabled: record.Enabled == 1, NextRunAt: record.NextRunAt}
}
