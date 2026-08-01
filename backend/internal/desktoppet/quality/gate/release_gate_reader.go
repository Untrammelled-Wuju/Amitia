package gate

import (
	"context"

	"github.com/u-ai/backend/internal/desktoppet/quality"
	"github.com/u-ai/backend/internal/desktoppet/release"
	"gorm.io/gorm"
)

func NewQualityGateReader(db *gorm.DB) release.ReleaseQualityGateReader {
	repo := quality.NewRepository(db)
	return &releaseQualityGateReader{
		svc: NewTaskGateService(repo, quality.NewGateEvaluator(repo)),
	}
}

type releaseQualityGateReader struct {
	svc *TaskGateService
}

func (r *releaseQualityGateReader) GetValidGateForRelease(
	ctx context.Context,
	userID string,
	processingTaskID string,
	activeRevisionSetHash string,
) (*release.QualityGateResult, error) {
	result, err := r.svc.GetValidGateForRelease(ctx, quality.GetValidGateForReleaseRequest{
		UserID:                userID,
		ProcessingTaskID:      processingTaskID,
				ActiveRevisionSetHash: activeRevisionSetHash,
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}

	included := make([]string, 0, len(result.ActionVerdicts))
	required := make([]string, 0, len(result.ActionVerdicts))
	verdicts := make([]release.GateActionVerdict, 0, len(result.ActionVerdicts))

	for _, av := range result.ActionVerdicts {
		if av.Verdict == quality.VerdictAccepted || av.Verdict == quality.VerdictAcceptedWithWarning {
			included = append(included, av.ActionKey)
		}
		if av.Required {
			required = append(required, av.ActionKey)
		}
		verdicts = append(verdicts, release.GateActionVerdict{
			ActionKey:        av.ActionKey,
			ActionName:       av.ActionName,
			Required:         av.Required,
			Verdict:          string(av.Verdict),
			ExecutionStatus:  string(av.ExecutionStatus),
			OverallScore:     av.OverallScore,
			FindingCount:     av.FindingCount,
			HardGateCount:    av.HardGateCount,
			ActionRevisionID: av.ActionRevisionID,
		})
	}

	return &release.QualityGateResult{
		GateStatus:            release.GateStatus(string(result.GateStatus)),
		GateID:                result.ProfileID,
		GateHash:              "",
		IncludedActionKeys:    included,
		RequiredActionKeys:    required,
		ExcludedActionKeys:    []string{},
		ActiveRevisionSetHash: result.ActiveRevisionSetHash,
		EvaluationSetHash:     result.EvaluationSetHash,
		ProfileID:             result.ProfileID,
		RuleSetVersion:        result.RuleSetVersion,
		RuleSetContentHash:    "",
		ActionVerdicts:        verdicts,
	}, nil
}
