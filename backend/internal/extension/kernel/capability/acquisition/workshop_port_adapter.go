package acquisition

import (
	"context"

	"github.com/u-ai/backend/internal/extension"
)

type workshopGeneratorPortAdapter struct {
	generator *extension.WorkshopGenerator
}

func NewWorkshopGeneratorPortAdapter(gen *extension.WorkshopGenerator) WorkshopGeneratePort {
	return &workshopGeneratorPortAdapter{generator: gen}
}

func (a *workshopGeneratorPortAdapter) GenerateInstruction(ctx context.Context, requirement string) (WorkshopInstructionDraft, error) {
	draft, err := a.generator.GenerateInstruction(ctx, requirement)
	if err != nil {
		return WorkshopInstructionDraft{}, err
	}
	return WorkshopInstructionDraft{
		Name:        draft.Name,
		Description: draft.Description,
		DisplayName: draft.DisplayName,
		Body:        draft.Body,
		References:  draft.References,
		Assets:      draft.Assets,
	}, nil
}
