// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestReevaluatePreservesImmutableInputFence(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:quality-reevaluate?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&QualityEvaluation{}))
	repo := NewRepository(db)
	ctx := context.Background()

	original := &QualityEvaluation{
		ID:                   "eval-original",
		UserID:               "user-1",
		CharacterID:          "character-1",
		ProcessingTaskID:     "task-1",
		ProcessingActionID:   "processing-action-1",
		ActionRevisionID:     "action-revision-1",
		ActionContentHash:    "content-hash-1",
		ProcessingRevisionID: "processing-revision-1",
		MeasurementSetID:     "measurement-set-1",
		ActionKey:            "idle",
		ExecutionStatus:      EvalSucceeded,
		ProfileID:            "profile-1",
		ProfileVersion:       "7",
		RuleSetVersion:       "ruleset-3",
		RulesetContentHash:   "ruleset-hash",
		MeasurementVersion:   "measurement-v2",
		QualityMode:          QualityModeBalanced,
	}
	require.NoError(t, repo.CreateEvaluation(ctx, original))

	svc, err := NewQualityService(ServiceConfig{DB: db, Repo: repo})
	require.NoError(t, err)
	recreated, err := svc.Reevaluate(ctx, ReevaluateRequest{
		EvaluationID: original.ID,
		QualityMode:  QualityModeStrict,
	})
	require.NoError(t, err)

	require.NotEqual(t, original.ID, recreated.ID)
	require.Equal(t, original.UserID, recreated.UserID)
	require.Equal(t, original.CharacterID, recreated.CharacterID)
	require.Equal(t, original.ProcessingTaskID, recreated.ProcessingTaskID)
	require.Equal(t, original.ProcessingActionID, recreated.ProcessingActionID)
	require.Equal(t, original.ActionRevisionID, recreated.ActionRevisionID)
	require.Equal(t, original.ActionContentHash, recreated.ActionContentHash)
	require.Equal(t, original.ProcessingRevisionID, recreated.ProcessingRevisionID)
	require.Equal(t, original.MeasurementSetID, recreated.MeasurementSetID)
	require.Equal(t, original.ProfileID, recreated.ProfileID)
	require.Equal(t, original.ProfileVersion, recreated.ProfileVersion)
	require.Equal(t, original.RuleSetVersion, recreated.RuleSetVersion)
	require.Equal(t, original.RulesetContentHash, recreated.RulesetContentHash)
	require.Equal(t, original.MeasurementVersion, recreated.MeasurementVersion)
	require.Equal(t, original.ID, recreated.SupersedesEvaluationID)
	require.Equal(t, QualityModeStrict, recreated.QualityMode)
	require.Equal(t, EvalPending, recreated.ExecutionStatus)
}
