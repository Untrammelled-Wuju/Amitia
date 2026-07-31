package worker

import (
	"github.com/u-ai/backend/internal/desktoppet/generation"
	"github.com/u-ai/backend/internal/desktoppet/generation/commit"
)

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
