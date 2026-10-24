package mindruntime

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/migration"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ModelUsageRecord struct {
	Sequence                int64  `gorm:"primaryKey;autoIncrement" json:"sequence"`
	ID                      string `gorm:"uniqueIndex;not null" json:"id"`
	SpaceID                 string `gorm:"index:idx_model_usage_scope,priority:1;index:idx_model_usage_coord,priority:1;not null" json:"spaceId"`
	DeviceID                string `gorm:"index:idx_model_usage_scope,priority:2" json:"deviceId"`
	OwnerID                 string `json:"ownerId"`
	RoleID                  string `json:"roleId"`
	RequestID               string `json:"requestId"`
	Model                   string `json:"model"`
	Provider                string `json:"provider"`
	Source                  string `json:"source"`
	Status                  string `json:"status"`
	StartedAt               string `gorm:"index" json:"startedAt"`
	CompletedAt             string `json:"completedAt"`
	InputTokens             int64  `json:"inputTokens"`
	OutputTokens            int64  `json:"outputTokens"`
	CachedTokens            int64  `json:"cachedTokens"`
	TotalTokens             int64  `json:"totalTokens"`
	UsageReported           bool   `json:"usageReported"`
	CoordinationID          string `gorm:"index:idx_model_usage_coord,priority:2" json:"coordinationId,omitempty"`
	ParentInteractionID     string `json:"parentInteractionId,omitempty"`
	AssignmentID            string `json:"assignmentId,omitempty"`
	ReservedTokens          int64  `gorm:"not null;default:0" json:"reservedTokens,omitempty"`
	ReservedCostMicroUSD    int64  `gorm:"column:reserved_cost_micro_usd;not null;default:0" json:"reservedCostMicroUSD"`
	BudgetCostLimitMicroUSD int64  `gorm:"column:budget_cost_limit_micro_usd;not null;default:0" json:"budgetCostLimitMicroUSD"`
	BudgetInputRate         int64  `gorm:"column:budget_input_rate;not null;default:0" json:"budgetInputRate"`
	BudgetOutputRate        int64  `gorm:"column:budget_output_rate;not null;default:0" json:"budgetOutputRate"`
	BudgetRateVersion       string `gorm:"column:budget_rate_version" json:"budgetRateVersion"`
	BudgetOverrun           bool   `gorm:"column:budget_overrun;not null;default:false" json:"budgetOverrun"`
	ReviewEvidenceRef       string `gorm:"column:review_evidence_ref;not null;default:''" json:"reviewEvidenceRef,omitempty"`
	ReviewActorID           string `gorm:"column:review_actor_id;not null;default:''" json:"reviewActorId,omitempty"`
	ReviewRecordedAt        string `gorm:"column:review_recorded_at;not null;default:''" json:"reviewRecordedAt,omitempty"`
}

func (ModelUsageRecord) TableName() string { return "runtime_model_usage" }

type UsageBaseline struct {
	SpaceID   string `gorm:"primaryKey"`
	DeviceID  string `gorm:"primaryKey"`
	Sequence  int64
	UpdatedAt string
}

func (UsageBaseline) TableName() string { return "runtime_usage_baselines" }

type UsageFilter struct {
	SpaceID  string `json:"coreId"`
	DeviceID string `json:"deviceId,omitempty"`
}

type ModelUsageStore struct {
	db              *gorm.DB
	SpaceID         string
	coreOwnerReview func(context.Context, coordination.ExecutionScope) error
}

func NewModelUsageStore(db *gorm.DB, spaceID string) (*ModelUsageStore, error) {
	if db == nil || spaceID == "" {
		return nil, errors.New("模型计量数据库或空间未就绪")
	}
	if err := (migration.Runner{DB: db}).Apply([]migration.Migration{migration.RuntimeModelBudgetMigration()}); err != nil {
		return nil, err
	}
	if err := db.Model(&ModelUsageRecord{}).Where("space_id=? AND status='running' AND (coordination_id IS NULL OR coordination_id='')", spaceID).Updates(map[string]any{"status": "interrupted", "completed_at": time.Now().UTC().Format(time.RFC3339Nano)}).Error; err != nil {
		return nil, err
	}
	state := UsageBaseline{SpaceID: spaceID, DeviceID: "@tracking-start", UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&state).Error; err != nil {
		return nil, err
	}
	return &ModelUsageStore{db: db, SpaceID: spaceID}, nil
}

func (s *ModelUsageStore) Begin(ctx context.Context, record ModelUsageRecord) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("multi_agent: model usage store is unavailable")
	}
	if record.SpaceID != "" && record.SpaceID != s.SpaceID {
		return "", errors.New("multi_agent: model usage space conflicts with store")
	}
	record.ID = uuid.NewString()
	record.SpaceID = s.SpaceID
	record.Status = "running"
	record.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if budget, exists := OwnedModelBudgetFromContext(ctx); exists {
		if err := s.reserveOwnedModelTokens(ctx, &record, budget); err != nil {
			return "", err
		}
		return record.ID, nil
	}
	return record.ID, s.db.WithContext(ctx).Create(&record).Error
}

func (s *ModelUsageStore) Finish(ctx context.Context, id string, usage ModelUsageRecord) error {
	if s == nil || s.db == nil || s.SpaceID == "" || id == "" {
		return errors.New("multi_agent: model usage settlement requires an initialized store and record")
	}
	verified := validOwnedModelSettlement(usage)
	updates := map[string]any{"status": usage.Status, "completed_at": time.Now().UTC().Format(time.RFC3339Nano), "input_tokens": usage.InputTokens, "output_tokens": usage.OutputTokens, "cached_tokens": usage.CachedTokens, "total_tokens": usage.TotalTokens}
	if verified {
		costSQL := "((?*budget_input_rate+?*budget_output_rate+(?-?-?)*MAX(budget_input_rate,budget_output_rate)+999999)/1000000)"
		costArgs := []any{usage.InputTokens, usage.OutputTokens, usage.TotalTokens, usage.InputTokens, usage.OutputTokens}
		overrunSQL := "(reserved_tokens>0 AND ?>reserved_tokens OR budget_cost_limit_micro_usd>0 AND " + costSQL + ">reserved_cost_micro_usd)"
		overrunArgs := append([]any{usage.TotalTokens}, costArgs...)
		updates["budget_overrun"] = gorm.Expr("CASE WHEN COALESCE(coordination_id,'')<>'' AND "+overrunSQL+" THEN 1 ELSE 0 END", overrunArgs...)
		usageArgs := append(append([]any{}, overrunArgs...), true, usage.UsageReported)
		updates["usage_reported"] = gorm.Expr("CASE WHEN COALESCE(coordination_id,'')<>'' AND "+overrunSQL+" THEN 0 WHEN COALESCE(coordination_id,'')<>'' THEN ? ELSE ? END", usageArgs...)
		tokenArgs := append(append([]any{}, overrunArgs...), usage.TotalTokens, usage.TotalTokens, usage.TotalTokens)
		updates["reserved_tokens"] = gorm.Expr("CASE WHEN COALESCE(coordination_id,'')<>'' AND "+overrunSQL+" THEN CASE WHEN reserved_tokens>? THEN reserved_tokens ELSE ? END WHEN COALESCE(coordination_id,'')<>'' THEN ? ELSE reserved_tokens END", tokenArgs...)
		costSettlementArgs := append(append([]any{}, overrunArgs...), usage.TotalTokens)
		costSettlementArgs = append(costSettlementArgs, costArgs...)
		updates["reserved_cost_micro_usd"] = gorm.Expr("CASE WHEN COALESCE(coordination_id,'')<>'' AND budget_cost_limit_micro_usd>0 AND "+overrunSQL+" THEN budget_cost_limit_micro_usd WHEN COALESCE(coordination_id,'')<>'' AND budget_cost_limit_micro_usd>0 AND ?<=reserved_tokens THEN MIN(budget_cost_limit_micro_usd,"+costSQL+") WHEN COALESCE(coordination_id,'')<>'' AND budget_cost_limit_micro_usd>0 THEN budget_cost_limit_micro_usd ELSE reserved_cost_micro_usd END", costSettlementArgs...)
	} else {
		updates["usage_reported"] = gorm.Expr("CASE WHEN COALESCE(coordination_id,'')<>'' THEN 0 ELSE ? END", usage.UsageReported)
	}
	result := s.db.WithContext(ctx).Model(&ModelUsageRecord{}).Where("id=? AND space_id=? AND status='running'", id, s.SpaceID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		var saved ModelUsageRecord
		if err := s.db.WithContext(ctx).Where("id=? AND space_id=?", id, s.SpaceID).First(&saved).Error; err != nil {
			return err
		}
		if saved.CoordinationID != "" {
			return errors.New("multi_agent: model usage reservation is already settled or interrupted")
		}
	}
	return nil
}

func (s *ModelUsageStore) Clear(ctx context.Context, filter UsageFilter) (int64, error) {
	if filter.SpaceID == "" {
		return 0, errors.New("统计空间缺失")
	}
	var cutoff int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&ModelUsageRecord{}).Where("space_id=?", filter.SpaceID).Select("COALESCE(MAX(sequence),0)").Scan(&cutoff).Error; err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "space_id"}, {Name: "device_id"}}, DoUpdates: clause.Assignments(map[string]any{"sequence": gorm.Expr("MAX(sequence,?)", cutoff), "updated_at": time.Now().UTC().Format(time.RFC3339Nano)})}).Create(&UsageBaseline{SpaceID: filter.SpaceID, DeviceID: filter.DeviceID, Sequence: cutoff, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}).Error
	})
	return cutoff, err
}

type UsageBucket struct {
	TodayCalls   int64
	TodayTokens  int64
	TodayUnknown int64
	Interrupted  int64  `json:"interrupted"`
	Day          string `json:"date,omitempty"`
	Model        string `json:"name,omitempty"`
	Provider     string `json:"provider,omitempty"`
	Source       string `json:"source,omitempty"`
	Calls        int64  `json:"modelCalls"`
	Succeeded    int64  `json:"succeeded"`
	Failed       int64  `json:"failed"`
	Cancelled    int64  `json:"cancelled"`
	Running      int64  `json:"running"`
	Input        int64  `json:"inputTokens"`
	Output       int64  `json:"outputTokens"`
	Cached       int64  `json:"cachedInputTokens"`
	Tokens       int64  `json:"knownTokens"`
	Unknown      int64  `json:"unknownUsageCalls"`
}

func (s *ModelUsageStore) Since(ctx context.Context, filter UsageFilter) (UsageBaseline, error) {
	var rows []UsageBaseline
	err := s.db.WithContext(ctx).Where("space_id=? AND (device_id='' OR device_id=?)", filter.SpaceID, filter.DeviceID).Order("sequence DESC,updated_at DESC").Limit(1).Find(&rows).Error
	if err != nil {
		return UsageBaseline{}, err
	}
	if len(rows) == 0 {
		return UsageBaseline{}, nil
	}
	return rows[0], nil
}

func (s *ModelUsageStore) Aggregate(ctx context.Context, filter UsageFilter, group string) (rows []UsageBucket, baseline UsageBaseline, err error) {
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		snapshot := &ModelUsageStore{db: tx, SpaceID: s.SpaceID}
		var queryErr error
		rows, baseline, queryErr = snapshot.aggregate(ctx, filter, group)
		return queryErr
	})
	return
}

func (s *ModelUsageStore) aggregate(ctx context.Context, filter UsageFilter, group string) ([]UsageBucket, UsageBaseline, error) {
	baseline, err := s.Since(ctx, filter)
	if err != nil {
		return nil, baseline, err
	}
	query := s.db.WithContext(ctx).Model(&ModelUsageRecord{}).Where("space_id=? AND sequence>?", filter.SpaceID, baseline.Sequence)
	if filter.DeviceID != "" {
		query = query.Where("device_id=?", filter.DeviceID)
	}
	selectSQL := `COUNT(*) AS calls, COALESCE(SUM(CASE WHEN status='succeeded' THEN 1 ELSE 0 END),0) AS succeeded, COALESCE(SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END),0) AS failed, COALESCE(SUM(CASE WHEN status='cancelled' THEN 1 ELSE 0 END),0) AS cancelled, COALESCE(SUM(CASE WHEN status='running' THEN 1 ELSE 0 END),0) AS running, COALESCE(SUM(CASE WHEN status='interrupted' THEN 1 ELSE 0 END),0) AS interrupted, COALESCE(SUM(input_tokens),0) AS input, COALESCE(SUM(output_tokens),0) AS output, COALESCE(SUM(cached_tokens),0) AS cached, COALESCE(SUM(total_tokens),0) AS tokens, COALESCE(SUM(CASE WHEN usage_reported=0 THEN 1 ELSE 0 END),0) AS unknown`
	switch group {
	case "overview":
		selectSQL += ",COALESCE(SUM(CASE WHEN substr(started_at,1,10)=date('now') THEN 1 ELSE 0 END),0) AS today_calls, COALESCE(SUM(CASE WHEN substr(started_at,1,10)=date('now') THEN total_tokens ELSE 0 END),0) AS today_tokens, COALESCE(SUM(CASE WHEN substr(started_at,1,10)=date('now') AND usage_reported=0 THEN 1 ELSE 0 END),0) AS today_unknown"
	case "today":
		query = query.Where("substr(started_at,1,10)=?", time.Now().UTC().Format("2006-01-02"))
	case "daily":
		query = query.Where("substr(started_at,1,10)>=date('now','-29 days')")
		selectSQL = "substr(started_at,1,10) AS day," + selectSQL
		query = query.Group("substr(started_at,1,10)").Order("day DESC").Limit(30)
	case "models":
		selectSQL = "model,provider," + selectSQL
		query = query.Group("model,provider").Order("calls DESC,model,provider")
	case "sources":
		selectSQL = "source," + selectSQL
		query = query.Group("source").Order("calls DESC,source")
	default:
		return nil, baseline, errors.New("统计分组无效")
	}
	rows := []UsageBucket{}
	err = query.Select(selectSQL).Scan(&rows).Error
	return rows, baseline, err
}

func (s *ModelUsageStore) TrackingStartedAt(ctx context.Context) (string, error) {
	var row UsageBaseline
	err := s.db.WithContext(ctx).Where("space_id=? AND device_id='@tracking-start'", s.SpaceID).First(&row).Error
	return row.UpdatedAt, err
}
