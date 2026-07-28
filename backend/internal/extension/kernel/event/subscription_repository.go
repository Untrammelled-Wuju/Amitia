package event

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type SubscriptionRepository interface {
	Upsert(ctx context.Context, def EventSubscriptionDefinition) error
	UpsertTx(ctx context.Context, tx *sql.Tx, def EventSubscriptionDefinition) error
	Get(ctx context.Context, contributionID string) (EventSubscriptionDefinition, error)
	ListActive(ctx context.Context) ([]EventSubscriptionDefinition, error)
	ListByExtension(ctx context.Context, extensionID string) ([]EventSubscriptionDefinition, error)
	ListByEventType(ctx context.Context, typeID EventTypeID) ([]EventSubscriptionDefinition, error)
	Delete(ctx context.Context, contributionID string) error
	DeleteByExtension(ctx context.Context, extensionID string) (int, error)
	UpdateGeneration(ctx context.Context, extensionID string, newGeneration int64, defs []EventSubscriptionDefinition) error
	SetEnabled(ctx context.Context, contributionID string, enabled bool) error
}

var ErrSubscriptionRepoNotFound = errors.New("event: subscription not found in repository")

type SQLiteSubscriptionRepository struct {
	db *sql.DB
}

func NewSQLiteSubscriptionRepository(db *sql.DB) *SQLiteSubscriptionRepository {
	return &SQLiteSubscriptionRepository{db: db}
}

func (r *SQLiteSubscriptionRepository) Upsert(ctx context.Context, def EventSubscriptionDefinition) error {
	return r.upsertWithExec(ctx, r.db, def)
}

func (r *SQLiteSubscriptionRepository) UpsertTx(ctx context.Context, tx *sql.Tx, def EventSubscriptionDefinition) error {
	return r.upsertWithExec(ctx, tx, def)
}

func (r *SQLiteSubscriptionRepository) upsertWithExec(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, def EventSubscriptionDefinition) error {
	if def.ContributionID == "" {
		return fmt.Errorf("event: contribution id required for upsert")
	}
	now := time.Now().UTC()
	if def.CreatedAt.IsZero() {
		def.CreatedAt = now
	}
	def.UpdatedAt = now

	filterJSON, _ := json.Marshal(def.Filter)
	projectionJSON, _ := json.Marshal(def.Projection)
	deliveryPolicyJSON, _ := json.Marshal(def.DeliveryPolicy)
	retryPolicyJSON, _ := json.Marshal(def.RetryPolicy)
	permReqJSON, _ := json.Marshal(def.PermissionRequirements)
	scopeRuleJSON, _ := json.Marshal(def.ScopeRule)
	depReqJSON, _ := json.Marshal(def.DependencyRequirements)
	runtimeBindingJSON, _ := json.Marshal(def.RuntimeBinding)

	enabledInt := 0
	if def.Enabled {
		enabledInt = 1
	}
	timeoutMs := int64(def.Timeout / time.Millisecond)
	if timeoutMs == 0 {
		timeoutMs = 5000
	}

	_, err := exec.ExecContext(ctx, `
		INSERT INTO extension_event_subscriptions
		(contribution_id, extension_id, module_id, event_type_id, event_version_range, entry,
		 filter_json, projection_json, delivery_policy_json, retry_policy_json,
		 ordering_requirement, timeout_ms, max_in_flight,
		 permission_requirements_json, scope_rule_json, dependency_requirements_json, runtime_binding_json,
		 definition_hash, generation, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(contribution_id) DO UPDATE SET
		 extension_id=excluded.extension_id, module_id=excluded.module_id, event_type_id=excluded.event_type_id,
		 event_version_range=excluded.event_version_range, entry=excluded.entry,
		 filter_json=excluded.filter_json, projection_json=excluded.projection_json,
		 delivery_policy_json=excluded.delivery_policy_json, retry_policy_json=excluded.retry_policy_json,
		 ordering_requirement=excluded.ordering_requirement, timeout_ms=excluded.timeout_ms, max_in_flight=excluded.max_in_flight,
		 permission_requirements_json=excluded.permission_requirements_json, scope_rule_json=excluded.scope_rule_json,
		 dependency_requirements_json=excluded.dependency_requirements_json, runtime_binding_json=excluded.runtime_binding_json,
		 definition_hash=excluded.definition_hash, generation=excluded.generation, enabled=excluded.enabled,
		 updated_at=excluded.updated_at
	`,
		def.ContributionID, def.ExtensionID, def.ModuleID, string(def.EventTypeID), def.EventVersionRange, def.Entry,
		string(filterJSON), string(projectionJSON), string(deliveryPolicyJSON), string(retryPolicyJSON),
		string(def.OrderingRequirement), timeoutMs, def.MaxInFlight,
		string(permReqJSON), string(scopeRuleJSON), string(depReqJSON), string(runtimeBindingJSON),
		def.DefinitionHash, def.Generation, enabledInt, def.CreatedAt, def.UpdatedAt,
	)
	return err
}

func (r *SQLiteSubscriptionRepository) Get(ctx context.Context, contributionID string) (EventSubscriptionDefinition, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT contribution_id, extension_id, module_id, event_type_id, event_version_range, entry,
		 filter_json, projection_json, delivery_policy_json, retry_policy_json,
		 ordering_requirement, timeout_ms, max_in_flight,
		 permission_requirements_json, scope_rule_json, dependency_requirements_json, runtime_binding_json,
		 definition_hash, generation, enabled, created_at, updated_at
		FROM extension_event_subscriptions
		WHERE contribution_id = ?
	`, contributionID)
	def, err := scanSubscriptionRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EventSubscriptionDefinition{}, ErrSubscriptionRepoNotFound
		}
		return EventSubscriptionDefinition{}, err
	}
	return def, nil
}

func (r *SQLiteSubscriptionRepository) ListActive(ctx context.Context) ([]EventSubscriptionDefinition, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT contribution_id, extension_id, module_id, event_type_id, event_version_range, entry,
		 filter_json, projection_json, delivery_policy_json, retry_policy_json,
		 ordering_requirement, timeout_ms, max_in_flight,
		 permission_requirements_json, scope_rule_json, dependency_requirements_json, runtime_binding_json,
		 definition_hash, generation, enabled, created_at, updated_at
		FROM extension_event_subscriptions
		WHERE enabled = 1
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubscriptionRows(rows)
}

func (r *SQLiteSubscriptionRepository) ListByExtension(ctx context.Context, extensionID string) ([]EventSubscriptionDefinition, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT contribution_id, extension_id, module_id, event_type_id, event_version_range, entry,
		 filter_json, projection_json, delivery_policy_json, retry_policy_json,
		 ordering_requirement, timeout_ms, max_in_flight,
		 permission_requirements_json, scope_rule_json, dependency_requirements_json, runtime_binding_json,
		 definition_hash, generation, enabled, created_at, updated_at
		FROM extension_event_subscriptions
		WHERE extension_id = ?
		ORDER BY created_at ASC
	`, extensionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubscriptionRows(rows)
}

func (r *SQLiteSubscriptionRepository) ListByEventType(ctx context.Context, typeID EventTypeID) ([]EventSubscriptionDefinition, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT contribution_id, extension_id, module_id, event_type_id, event_version_range, entry,
		 filter_json, projection_json, delivery_policy_json, retry_policy_json,
		 ordering_requirement, timeout_ms, max_in_flight,
		 permission_requirements_json, scope_rule_json, dependency_requirements_json, runtime_binding_json,
		 definition_hash, generation, enabled, created_at, updated_at
		FROM extension_event_subscriptions
		WHERE event_type_id = ?
		ORDER BY created_at ASC
	`, string(typeID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubscriptionRows(rows)
}

func (r *SQLiteSubscriptionRepository) Delete(ctx context.Context, contributionID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM extension_event_subscriptions WHERE contribution_id = ?`, contributionID)
	return err
}

func (r *SQLiteSubscriptionRepository) DeleteByExtension(ctx context.Context, extensionID string) (int, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM extension_event_subscriptions WHERE extension_id = ?`, extensionID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (r *SQLiteSubscriptionRepository) UpdateGeneration(ctx context.Context, extensionID string, newGeneration int64, defs []EventSubscriptionDefinition) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM extension_event_subscriptions WHERE extension_id = ?`, extensionID); err != nil {
		return fmt.Errorf("event: delete old subscriptions: %w", err)
	}

	for i := range defs {
		defs[i].Generation = newGeneration
		defs[i].Enabled = true
		if err := defs[i].Validate(); err != nil {
			return fmt.Errorf("event: validate %s: %w", defs[i].ContributionID, err)
		}
		if err := r.UpsertTx(ctx, tx, defs[i]); err != nil {
			return fmt.Errorf("event: upsert %s: %w", defs[i].ContributionID, err)
		}
	}

	return tx.Commit()
}

func (r *SQLiteSubscriptionRepository) SetEnabled(ctx context.Context, contributionID string, enabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE extension_event_subscriptions SET enabled = ?, updated_at = ? WHERE contribution_id = ?
	`, enabledInt, time.Now().UTC(), contributionID)
	return err
}

type subscriptionScanner interface {
	Scan(dest ...any) error
}

func scanSubscriptionRow(scanner subscriptionScanner) (EventSubscriptionDefinition, error) {
	var def EventSubscriptionDefinition
	var eventTypeIDStr string
	var filterJSON, projectionJSON, deliveryPolicyJSON, retryPolicyJSON string
	var permReqJSON, scopeRuleJSON, depReqJSON, runtimeBindingJSON string
	var orderingReq string
	var timeoutMs int64
	var enabledInt int

	err := scanner.Scan(
		&def.ContributionID, &def.ExtensionID, &def.ModuleID, &eventTypeIDStr, &def.EventVersionRange, &def.Entry,
		&filterJSON, &projectionJSON, &deliveryPolicyJSON, &retryPolicyJSON,
		&orderingReq, &timeoutMs, &def.MaxInFlight,
		&permReqJSON, &scopeRuleJSON, &depReqJSON, &runtimeBindingJSON,
		&def.DefinitionHash, &def.Generation, &enabledInt, &def.CreatedAt, &def.UpdatedAt,
	)
	if err != nil {
		return def, err
	}

	def.EventTypeID = EventTypeID(eventTypeIDStr)
	def.OrderingRequirement = EventOrderingRequirement(orderingReq)
	def.Timeout = time.Duration(timeoutMs) * time.Millisecond
	def.Enabled = enabledInt != 0

	_ = json.Unmarshal([]byte(filterJSON), &def.Filter)
	_ = json.Unmarshal([]byte(projectionJSON), &def.Projection)
	_ = json.Unmarshal([]byte(deliveryPolicyJSON), &def.DeliveryPolicy)
	_ = json.Unmarshal([]byte(retryPolicyJSON), &def.RetryPolicy)
	_ = json.Unmarshal([]byte(permReqJSON), &def.PermissionRequirements)
	_ = json.Unmarshal([]byte(scopeRuleJSON), &def.ScopeRule)
	_ = json.Unmarshal([]byte(depReqJSON), &def.DependencyRequirements)
	_ = json.Unmarshal([]byte(runtimeBindingJSON), &def.RuntimeBinding)

	return def, nil
}

func scanSubscriptionRows(rows *sql.Rows) ([]EventSubscriptionDefinition, error) {
	var result []EventSubscriptionDefinition
	for rows.Next() {
		def, err := scanSubscriptionRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, def)
	}
	return result, rows.Err()
}
