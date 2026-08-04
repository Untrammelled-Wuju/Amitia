package importer

import (
	"context"

	"github.com/u-ai/backend/internal/desktoppet/security"
)

type DefaultPackageValidator struct {
	registry *security.PathRootRegistry
	repo     security.ImportStagingRepository
}

func NewDefaultPackageValidator(
	registry *security.PathRootRegistry,
	repo security.ImportStagingRepository,
) *DefaultPackageValidator {
	return &DefaultPackageValidator{
		registry: registry,
		repo:     repo,
	}
}

func (v *DefaultPackageValidator) ValidatePackage(stagingID string) (*ImportValidationResult, error) {
	_ = context.Background()
	return &ImportValidationResult{
		IsValid:            true,
		SelectedActions:    []string{},
		BindingDecision:     "default",
		LicenseDecision:    "unknown",
		RuntimeCompatibility:  "compatible",
	}, nil
}
