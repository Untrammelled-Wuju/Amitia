package mindruntime

import (
	"context"
	"errors"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type GormReconciliationSource struct {
	DB     *gorm.DB
	Store  string
	Tables []GormReconciliationTable
}

type GormReconciliationTable struct {
	Table             string
	Kind              string
	KeyColumns        []string
	VersionColumn     string
	StatusColumn      string
	DeletedColumn     string
	LeasedUntilColumn string
	HashColumns       []string
	FieldColumns      []string
	ReferenceColumns  map[string]string
}

func RegisterDefaultRuntimeReconciliationCheckers(engine *ReconciliationEngine, db *gorm.DB) error {
	if engine == nil {
		return errors.New("reconciliation engine is nil")
	}
	if db == nil {
		return errors.New("reconciliation db is nil")
	}
	engine.RegisterChecker(ReconciliationTombstoneDerivedData, NewTombstoneDerivedDataReconciliationChecker(db))
	registerGormReconciliationChecker(engine, db, ReconciliationLeaseDelivery, leaseDeliverySource(db), deliveryIntentSource(db))
	registerGormReconciliationChecker(engine, db, ReconciliationOutboxSideEffect, outboxSideEffectSource(db), outboxChannelReceiptSource(db))
	registerGormReconciliationChecker(engine, db, ReconciliationInteractionRunMsg, interactionRunSource(db), interactionMessageSource(db))
	return nil
}

func registerGormReconciliationChecker(engine *ReconciliationEngine, db *gorm.DB, target ReconciliationTarget, source GormReconciliationSource, targetSource GormReconciliationSource) {
	source.Tables = existingReconciliationTables(db, source.Tables)
	targetSource.Tables = existingReconciliationTables(db, targetSource.Tables)
	if len(source.Tables) == 0 || len(targetSource.Tables) == 0 {
		return
	}
	engine.RegisterChecker(target, NewRuntimeReconciliationChecker(source, targetSource))
}

func existingReconciliationTables(db *gorm.DB, tables []GormReconciliationTable) []GormReconciliationTable {
	existing := make([]GormReconciliationTable, 0, len(tables))
	for _, table := range tables {
		if strings.TrimSpace(table.Table) != "" && db.Migrator().HasTable(table.Table) {
			existing = append(existing, table)
		}
	}
	return existing
}

func leaseDeliverySource(db *gorm.DB) GormReconciliationSource {
	return GormReconciliationSource{
		DB:    db,
		Store: "sqlite",
		Tables: []GormReconciliationTable{
			{
				Table:             "outbox_records",
				Kind:              "outbox",
				KeyColumns:        []string{"id"},
				StatusColumn:      "status",
				VersionColumn:     "retry_count",
				LeasedUntilColumn: "leased_until",
				HashColumns:       []string{"aggregate_id", "event_type", "status", "retry_count", "last_error"},
				FieldColumns:      []string{"aggregate_id", "event_type", "status", "last_error"},
			},
		},
	}
}

func outboxSideEffectSource(db *gorm.DB) GormReconciliationSource {
	return GormReconciliationSource{
		DB:    db,
		Store: "sqlite",
		Tables: []GormReconciliationTable{
			{
				Table:         "outbox_records",
				Kind:          "outbox_side_effect",
				KeyColumns:    []string{"aggregate_id", "event_type"},
				StatusColumn:  "status",
				VersionColumn: "retry_count",
				HashColumns:   []string{"payload", "status", "retry_count"},
				FieldColumns:  []string{"aggregate_id", "event_type", "status", "last_error"},
			},
		},
	}
}

func interactionRunSource(db *gorm.DB) GormReconciliationSource {
	return GormReconciliationSource{
		DB:    db,
		Store: "sqlite",
		Tables: []GormReconciliationTable{
			{
				Table:         "interaction_records",
				Kind:          "interaction",
				KeyColumns:    []string{"request_id"},
				StatusColumn:  "status",
				VersionColumn: "status_version",
				HashColumns:   []string{"status", "status_version", "result_ref", "error_code", "error_message"},
				FieldColumns:  []string{"user_id", "character_id", "conversation_id", "request_id", "status", "result_ref"},
			},
		},
	}
}

func interactionMessageSource(db *gorm.DB) GormReconciliationSource {
	return GormReconciliationSource{
		DB:    db,
		Store: "sqlite",
		Tables: []GormReconciliationTable{
			{
				Table:         "messages",
				Kind:          "interaction",
				KeyColumns:    []string{"request_id"},
				StatusColumn:  "status",
				VersionColumn: "updated_at",
				HashColumns:   []string{"content", "status", "updated_at"},
				FieldColumns:  []string{"conversation_id", "character_id", "request_id", "status"},
			},
		},
	}
}

func (s GormReconciliationSource) ListReconciliationEntities(ctx context.Context, req ReconciliationCheckRequest) ([]ReconciliationEntity, error) {
	if s.DB == nil {
		return nil, errors.New("gorm reconciliation source requires db")
	}
	entities := make([]ReconciliationEntity, 0)
	for _, table := range s.Tables {
		if strings.TrimSpace(table.Table) == "" {
			return nil, errors.New("gorm reconciliation table name is required")
		}
		rows := make([]map[string]interface{}, 0)
		query := s.DB.WithContext(ctx).Table(table.Table)
		if req.BatchSize > 0 {
			query = query.Limit(req.BatchSize)
		}
		if err := query.Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			entity := table.entityFromRow(s.Store, row)
			if entity.Key == "" {
				continue
			}
			entities = append(entities, entity)
		}
	}
	sort.Slice(entities, func(i, j int) bool {
		return reconciliationEntityIdentity(entities[i]) < reconciliationEntityIdentity(entities[j])
	})
	return entities, nil
}

func (t GormReconciliationTable) entityFromRow(store string, row map[string]interface{}) ReconciliationEntity {
	fields := make(map[string]string)
	for _, column := range t.FieldColumns {
		if value, ok := reconciliationRowValue(row, column); ok {
			fields[column] = reconciliationValueString(value)
		}
	}
	references := make(map[string]string)
	for name, column := range t.ReferenceColumns {
		if value, ok := reconciliationRowValue(row, column); ok {
			references[name] = reconciliationValueString(value)
		}
	}
	keyParts := make([]string, 0, len(t.KeyColumns))
	for _, column := range t.KeyColumns {
		value, _ := reconciliationRowValue(row, column)
		keyParts = append(keyParts, reconciliationValueString(value))
	}
	statusValue, _ := reconciliationRowValue(row, t.StatusColumn)
	versionValue, _ := reconciliationRowValue(row, t.VersionColumn)
	deletedValue, _ := reconciliationRowValue(row, t.DeletedColumn)
	leasedUntilValue, _ := reconciliationRowValue(row, t.LeasedUntilColumn)
	status := reconciliationValueString(statusValue)
	version := reconciliationValueString(versionValue)
	deleted := reconciliationBool(deletedValue)
	leasedUntil := reconciliationTime(leasedUntilValue)
	hashColumns := t.HashColumns
	if len(hashColumns) == 0 {
		hashColumns = t.FieldColumns
	}
	return ReconciliationEntity{
		Store:       store,
		Kind:        firstNonEmpty(t.Kind, t.Table),
		Key:         strings.Join(keyParts, ":"),
		Version:     version,
		Status:      status,
		Hash:        reconciliationRowHash(row, hashColumns),
		Deleted:     deleted,
		LeasedUntil: leasedUntil,
		Fields:      fields,
		References:  references,
	}
}
