package worker

import (
	"github.com/u-ai/backend/internal/desktoppet/generation"
	"github.com/u-ai/backend/internal/desktoppet/generation/activebinding"
	"github.com/u-ai/backend/internal/desktoppet/generation/commit"
	"gorm.io/gorm"
)

type activeBindingFinalizerAdapter struct {
	repo activebinding.Repository
}

func NewActiveBindingFinalizerAdapter(repo activebinding.Repository) generation.ActiveBindingFinalizerRepo {
	return &activeBindingFinalizerAdapter{repo: repo}
}

func (a *activeBindingFinalizerAdapter) GetByActionID(actionID string) (revision int, exists bool, err error) {
	binding, err := a.repo.GetByActionID(actionID)
	if err != nil {
		return 0, false, err
	}
	if binding == nil {
		return 0, false, nil
	}
	return binding.BindingRevision, true, nil
}

func (a *activeBindingFinalizerAdapter) CASUpdate(tx *gorm.DB, actionID string, expectedRevision int, attemptID, artifactID, hash, reason string) (bool, error) {
	newBinding := &activebinding.ActiveBinding{
		GenerationActionID:      actionID,
		ActiveAttemptID:         attemptID,
		ActivePrimaryArtifactID: artifactID,
		ArtifactContentHash:     hash,
		BoundReason:             reason,
	}
	return a.repo.CASUpdate(tx, actionID, expectedRevision, newBinding)
}

func makeCommitFunc(committer *commit.ArtifactCommitter) generation.ArtifactCommitFunc {
	return func(input generation.ArtifactCommitInput) (*generation.GenerationArtifact, error) {
		commitInput := commit.CommitInput{
			Tx:                  input.Tx,
			TaskID:              input.TaskID,
			TaskActionID:        input.TaskActionID,
			AttemptID:           input.AttemptID,
			ReferenceAssetID:    input.ReferenceAssetID,
			PromptHash:          input.PromptHash,
			CandidateData:       input.CandidateData,
			CandidateMIME:       input.CandidateMIME,
			SegmentIndex:        input.SegmentIndex,
			CandidateIndex:      input.CandidateIndex,
			ArtifactType:        input.ArtifactType,
			ArtifactRole:        input.ArtifactRole,
			IsPrimary:           input.IsPrimary,
			LayoutJSON:          input.LayoutJSON,
			MetadataJSON:        input.MetadataJSON,
			ProviderRequestID:   input.ProviderRequestID,
			ProviderOperationID: input.ProviderOperationID,
			StorageKey:          input.StorageKey,
			DataDir:             input.DataDir,
		}
		result, err := committer.Commit(commitInput)
		if err != nil {
			return nil, err
		}
		return result.Artifact, nil
	}
}
