// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

type GateEvaluator struct {
	repo QualityRepository
}

func NewGateEvaluator(repo QualityRepository) *GateEvaluator {
	return &GateEvaluator{repo: repo}
}

func (g *GateEvaluator) EvaluateTaskGate(ctx context.Context, processingTaskID string, actionVerdicts []ActionVerdictSummary, profile QualityProfileSnapshot) (*QualityGateResult, error) {
	required := 0
	accepted := 0
	warning := 0
	review := 0
	rejected := 0
	failed := 0

	for _, av := range actionVerdicts {
		if av.Required {
			required++
		}
		if av.ExecutionStatus == EvalFailed || av.ExecutionStatus == EvalCancelled {
			failed++
			continue
		}
		switch av.Verdict {
		case VerdictAccepted:
			accepted++
		case VerdictAcceptedWithWarning:
			accepted++
			warning++
		case VerdictNeedsReview:
			review++
		case VerdictRejected:
			rejected++
		}
	}

	gateStatus := g.resolveGateStatus(required, accepted, warning, review, rejected, failed, profile)

	snapshot := gateSnapshot{
		ProcessingTaskID: processingTaskID,
		ActionVerdicts:   actionVerdicts,
	}

	snapshotJSON, _ := json.Marshal(snapshot)
	snapshotHash := hashSnapshot(snapshotJSON)

	result := &QualityGateResult{
		ProcessingTaskID:      processingTaskID,
		GateStatus:            gateStatus,
		RequiredActionCount:   required,
		AcceptedActionCount:   accepted,
		WarningActionCount:    warning,
		ReviewActionCount:     review,
		RejectedActionCount:   rejected,
		FailedEvaluationCount: failed,
		ActionVerdicts:        actionVerdicts,
	}

	record := &QualityGateResultRecord{
		ProcessingTaskID:      processingTaskID,
		GateStatus:            string(gateStatus),
		RequiredActionCount:   required,
		AcceptedActionCount:   accepted,
		WarningActionCount:    warning,
		ReviewActionCount:     review,
		RejectedActionCount:   rejected,
		FailedEvaluationCount: failed,
		SnapshotJSON:          string(snapshotJSON),
		SnapshotHash:          snapshotHash,
	}

	if err := g.repo.UpsertGateResult(ctx, record); err != nil {
		return nil, NewQualityError(ErrCodeDatabaseCommitFailed, "failed to upsert gate result", err)
	}

	return result, nil
}

func (g *GateEvaluator) resolveGateStatus(required, accepted, warning, review, rejected, failed int, profile QualityProfileSnapshot) GateStatus {
	if failed > 0 && required > 0 {
		return GateBlocked
	}

	if rejected > 0 && profile.RequiredActionPolicy.BlockOnRejected {
		return GateBlocked
	}

	if review > 0 && profile.RequiredActionPolicy.BlockOnReview {
		return GateReviewRequired
	}

	if required > 0 && accepted < required {
		return GatePartialCandidate
	}

	if review > 0 {
		return GateReviewRequired
	}

	if warning > 0 {
		return GatePassedWithWarnings
	}

	return GatePassed
}

func (g *GateEvaluator) InvalidateGate(ctx context.Context, processingTaskID string) error {
	if processingTaskID == "" {
		return nil
	}
	if err := g.repo.DeleteGateResult(ctx, processingTaskID); err != nil {
		return NewQualityError(ErrCodeDatabaseCommitFailed, "failed to invalidate gate result", err)
	}
	return nil
}

func (g *GateEvaluator) GetGateStatus(ctx context.Context, processingTaskID string) (*QualityGateResult, error) {
	record, err := g.repo.GetGateResult(ctx, processingTaskID)
	if err != nil {
		return nil, err
	}

	var snapshot gateSnapshot
	if err := json.Unmarshal([]byte(record.SnapshotJSON), &snapshot); err != nil {
		return &QualityGateResult{
			ProcessingTaskID:      processingTaskID,
			GateStatus:            GateStatus(record.GateStatus),
			RequiredActionCount:   record.RequiredActionCount,
			AcceptedActionCount:   record.AcceptedActionCount,
			WarningActionCount:    record.WarningActionCount,
			ReviewActionCount:     record.ReviewActionCount,
			RejectedActionCount:   record.RejectedActionCount,
			FailedEvaluationCount: record.FailedEvaluationCount,
		}, nil
	}

	return &QualityGateResult{
		ProcessingTaskID:      processingTaskID,
		GateStatus:            GateStatus(record.GateStatus),
		RequiredActionCount:   record.RequiredActionCount,
		AcceptedActionCount:   record.AcceptedActionCount,
		WarningActionCount:    record.WarningActionCount,
		ReviewActionCount:     record.ReviewActionCount,
		RejectedActionCount:   record.RejectedActionCount,
		FailedEvaluationCount: record.FailedEvaluationCount,
		ActionVerdicts:        snapshot.ActionVerdicts,
	}, nil
}

type gateSnapshot struct {
	ProcessingTaskID string                 `json:"processingTaskId"`
	ActionVerdicts   []ActionVerdictSummary `json:"actionVerdicts"`
}

func hashSnapshot(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func SortActionVerdicts(verdicts []ActionVerdictSummary) {
	sort.SliceStable(verdicts, func(i, j int) bool {
		if verdicts[i].Required != verdicts[j].Required {
			return verdicts[i].Required
		}
		return verdicts[i].ActionKey < verdicts[j].ActionKey
	})
}
