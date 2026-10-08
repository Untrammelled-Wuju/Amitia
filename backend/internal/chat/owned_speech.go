package chat

import (
	"context"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/tts"
)

func (s *service) GenerateOwnedSpeech(ctx context.Context, input business.SpeechInference) ([]byte, error) {
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	return tts.SynthesizePrivateRole(ctx, s.db, input.Scope.SpaceID, input.Role.Profile, input.Scope.RoleOwnerID == input.Scope.CoreID, input.Text)
}
