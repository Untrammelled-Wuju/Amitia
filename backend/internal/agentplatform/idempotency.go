package agentplatform

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BeginIdempotentOperationInput struct {
	AgentID        string         `json:"agentId"`
	ScopeKey       string         `json:"scopeKey"`
	IdempotencyKey string         `json:"idempotencyKey"`
	Fingerprint    string         `json:"fingerprint"`
	Input          map[string]any `json:"input"`
}

type CompleteIdempotentOperationInput struct {
	Result           any    `json:"result"`
	Error            string `json:"error"`
	ExpectedRevision int64  `json:"expectedRevision"`
}

type IdempotentBeginResult struct {
	Operation IdempotentOperation `json:"operation"`
	Created   bool                `json:"created"`
}

func (s *Service) BeginIdempotentOperation(ctx context.Context, spaceID string, input BeginIdempotentOperationInput) (IdempotentBeginResult, error) {
	scopeKey := strings.TrimSpace(input.ScopeKey)
	key := strings.TrimSpace(input.IdempotencyKey)
	if scopeKey == "" || key == "" || strings.TrimSpace(input.AgentID) == "" {
		return IdempotentBeginResult{}, errors.New("Agent、作用域和幂等键不能为空")
	}
	if strings.TrimSpace(input.Fingerprint) == "" {
		_, fingerprint, err := hashJSON(input.Input)
		if err != nil {
			return IdempotentBeginResult{}, err
		}
		input.Fingerprint = fingerprint
	}
	now := time.Now().UTC()
	operation := IdempotentOperation{
		ID: newID("operation"), SpaceID: spaceID, AgentID: input.AgentID,
		IdempotencyKey: key, ScopeKey: scopeKey, Fingerprint: input.Fingerprint,
		Status: "running", Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	created := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing IdempotentOperation
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id = ? AND scope_key = ? AND idempotency_key = ?", spaceID, scopeKey, key).First(&existing).Error
		if err == nil {
			if existing.Fingerprint != input.Fingerprint {
				return errors.New("同一幂等键对应了不同输入")
			}
			operation = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("读取幂等操作: %w", err)
		}
		if err := tx.Create(&operation).Error; err != nil {
			return fmt.Errorf("创建幂等操作: %w", err)
		}
		created = true
		return nil
	})
	if err != nil {
		return IdempotentBeginResult{}, err
	}
	return IdempotentBeginResult{Operation: operation, Created: created}, nil
}

func (s *Service) CompleteIdempotentOperation(ctx context.Context, spaceID, operationID string, input CompleteIdempotentOperationInput) (IdempotentOperation, error) {
	resultJSON, _, err := hashJSON(input.Result)
	if err != nil {
		return IdempotentOperation{}, err
	}
	status := "completed"
	if strings.TrimSpace(input.Error) != "" {
		status = "failed"
	}
	now := time.Now().UTC()
	result := s.db.WithContext(ctx).Model(&IdempotentOperation{}).Where("id = ? AND space_id = ? AND revision = ? AND status = ?", operationID, spaceID, input.ExpectedRevision, "running").Updates(map[string]any{
		"status": status, "result_json": resultJSON, "original_error": input.Error,
		"completed_at": now, "updated_at": now, "revision": input.ExpectedRevision + 1,
	})
	if result.Error != nil {
		return IdempotentOperation{}, fmt.Errorf("完成幂等操作: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return IdempotentOperation{}, errors.New("幂等操作不存在或版本已变化")
	}
	var operation IdempotentOperation
	if err := s.db.WithContext(ctx).Where("id = ?", operationID).First(&operation).Error; err != nil {
		return IdempotentOperation{}, fmt.Errorf("读取幂等操作: %w", err)
	}
	return operation, nil
}
