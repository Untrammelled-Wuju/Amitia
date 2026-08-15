package migration

import (
	"bytes"
	"context"
	"fmt"
)

type PreconditionValidator struct{}

func NewPreconditionValidator() *PreconditionValidator {
	return &PreconditionValidator{}
}

func (v *PreconditionValidator) ValidatePreconditions(ctx context.Context, conditions []MigrationCondition) (*ValidationResult, error) {
	return v.validateConditions(conditions)
}

func (v *PreconditionValidator) ValidatePostconditions(ctx context.Context, conditions []MigrationCondition) (*ValidationResult, error) {
	return v.validateConditions(conditions)
}

func (v *PreconditionValidator) validateConditions(conditions []MigrationCondition) (*ValidationResult, error) {
	result := &ValidationResult{
		Passed:   true,
		Errors:   []string{},
		Warnings: []string{},
	}

	for _, cond := range conditions {
		if !bytes.Equal(cond.Expected, cond.Actual) {
			result.Passed = false
			result.Errors = append(result.Errors, fmt.Sprintf("condition %s: expected %s, got %s", cond.Name, string(cond.Expected), string(cond.Actual)))
		}
	}

	return result, nil
}

func (v *PreconditionValidator) CheckForbiddenDomains(domains []DataDomain) error {
	for _, domain := range domains {
		if ForbiddenDomains[ForbiddenMigrationDomain(domain.Domain)] {
			return fmt.Errorf("domain %s is forbidden", domain.Domain)
		}
	}
	return nil
}

func (v *PreconditionValidator) CheckPermissions(reqs []PermissionRequirement) error {
	for _, req := range reqs {
		if req.PermissionID == "" {
			return fmt.Errorf("permission requirement has empty permission_id")
		}
		if req.Scope.Type == "" {
			return fmt.Errorf("permission requirement %s has empty scope", req.PermissionID)
		}
	}
	return nil
}

func (v *PreconditionValidator) CheckResourceLimits(limits TaskResourceLimits) error {
	if limits.MaxMemoryMB <= 0 {
		return fmt.Errorf("max_memory_mb must be > 0")
	}
	if limits.MaxCPUPercent <= 0 || limits.MaxCPUPercent > 100 {
		return fmt.Errorf("max_cpu_percent must be > 0 and <= 100")
	}
	if limits.MaxDiskMB <= 0 {
		return fmt.Errorf("max_disk_mb must be > 0")
	}
	if limits.MaxDurationSecs <= 0 {
		return fmt.Errorf("max_duration_secs must be > 0")
	}
	return nil
}

func (v *PreconditionValidator) CheckReversibility(reversibility Reversibility) bool {
	return reversibility == ReversibilityFullyReversible || reversibility == ReversibilitySnapshotReversible
}

func (v *PreconditionValidator) CheckIdempotency(idempotency Idempotency) bool {
	return idempotency == IdempotencyIdempotent || idempotency == IdempotencyCheckpointIdempotent
}

func (v *PreconditionValidator) ValidateMigrationDefinition(def *MigrationDefinition) (*ValidationResult, error) {
	result := &ValidationResult{
		Passed:   true,
		Errors:   []string{},
		Warnings: []string{},
	}

	if err := v.CheckForbiddenDomains(def.DataDomains); err != nil {
		result.Passed = false
		result.Errors = append(result.Errors, err.Error())
	}

	if err := v.CheckPermissions(def.PermissionRequirements); err != nil {
		result.Passed = false
		result.Errors = append(result.Errors, err.Error())
	}

	if err := v.CheckResourceLimits(def.ResourceLimits); err != nil {
		result.Passed = false
		result.Errors = append(result.Errors, err.Error())
	}

	if !v.CheckReversibility(def.Reversibility) {
		result.Warnings = append(result.Warnings, fmt.Sprintf("migration is not reversible: %s", def.Reversibility))
	}

	if !v.CheckIdempotency(def.Idempotency) {
		result.Warnings = append(result.Warnings, fmt.Sprintf("migration is not idempotent: %s", def.Idempotency))
	}

	return result, nil
}
