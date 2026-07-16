package mindruntime

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

type ExternalReconciliationService interface {
	CheckSideEffectExists(ctx context.Context, aggregateID, eventType string) (bool, error)
	Name() string
}

type externalSideEffectSource struct {
	service ExternalReconciliationService
	db      *gorm.DB
	store   string
}

func (s externalSideEffectSource) ListReconciliationEntities(ctx context.Context, req ReconciliationCheckRequest) ([]ReconciliationEntity, error) {
	if s.db == nil {
		return nil, errors.New("external side effect source requires db")
	}
	if !s.db.Migrator().HasTable("outbox_records") {
		return nil, nil
	}
	rows := make([]map[string]interface{}, 0)
	query := s.db.WithContext(ctx).Table("outbox_records").Where("status = ?", "published")
	if req.BatchSize > 0 {
		query = query.Limit(req.BatchSize)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	entities := make([]ReconciliationEntity, 0, len(rows))
	for _, row := range rows {
		aggregateID, _ := reconciliationRowValue(row, "aggregate_id")
		eventType, _ := reconciliationRowValue(row, "event_type")
		aggID := reconciliationValueString(aggregateID)
		evtType := reconciliationValueString(eventType)
		exists, checkErr := s.service.CheckSideEffectExists(ctx, aggID, evtType)
		fieldStatus := "verified"
		if checkErr != nil {
			fieldStatus = "check_error"
		} else if !exists {
			fieldStatus = "missing"
		}
		entities = append(entities, ReconciliationEntity{
			Store:  s.store,
			Kind:   "external_side_effect",
			Key:    aggID + ":" + evtType,
			Status: fieldStatus,
			Fields: map[string]string{
				"aggregate_id": aggID,
				"event_type":   evtType,
				"service":      s.service.Name(),
			},
		})
	}
	return entities, nil
}

func RegisterRuntimeReconciliationCheckers(engine *ReconciliationEngine, db *gorm.DB, externalServices ...ExternalReconciliationService) error {
	if err := RegisterDefaultRuntimeReconciliationCheckers(engine, db); err != nil {
		return err
	}
	for _, svc := range externalServices {
		if svc == nil {
			continue
		}
		externalSource := externalSideEffectSource{db: db, service: svc, store: svc.Name()}
		sourceTables := []GormReconciliationTable{{Table: "outbox_records", Kind: "outbox", KeyColumns: []string{"aggregate_id", "event_type"}, StatusColumn: "status"}}
		sourceTables = existingReconciliationTables(db, sourceTables)
		if len(sourceTables) == 0 {
			continue
		}
		sqliteSource := GormReconciliationSource{DB: db, Store: "sqlite", Tables: sourceTables}
		target := ReconciliationTarget("external_" + ReconciliationTarget(svc.Name()))
		engine.RegisterChecker(target, NewRuntimeReconciliationChecker(sqliteSource, externalSource))
	}
	return nil
}
