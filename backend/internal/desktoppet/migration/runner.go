// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

import (
	"context"
	"fmt"
	"time"
)

type Runner struct {
	repo *DBRepository
}

func NewRunner(repo *DBRepository) *Runner {
	return &Runner{repo: repo}
}

func (r *Runner) InitializeSchema(ctx context.Context) error {
	return nil
}

func (r *Runner) RunPlan(ctx context.Context, planID string) (string, error) {
	op := &MigrationOperation{
		ID:             fmt.Sprintf("migop_%d", time.Now().UnixNano()),
		PlanID:         planID,
		Stage:          StagePreflight,
		ProcessedCount: 0,
		StartedAt:      time.Now().Format("2006-01-02 15:04:05"),
		UpdatedAt:      time.Now().Format("2006-01-02 15:04:05"),
	}
	_ = op
	return "", &RunnerError{Code: "NOT_IMPLEMENTED", Message: "迁移运行器尚未实现"}
}

func (r *Runner) GetOperation(ctx context.Context, operationID string) (*MigrationOperation, error) {
	return nil, &RunnerError{Code: "NOT_IMPLEMENTED", Message: "迁移运行器尚未实现"}
}

func (r *Runner) RequestCutover(ctx context.Context, direction string) error {
	return &RunnerError{Code: "NOT_IMPLEMENTED", Message: "切换尚未实现"}
}

type RunnerError struct {
	Code    string
	Message string
	Err     error
}

func (e *RunnerError) Error() string {
	if e.Err != nil {
		return e.Message
	}
	return e.Message
}
