package update

import (
	"context"
)

type ContributionSideEffectInfo struct {
	ContributionID  string
	SideEffectClass string
	Actions         []string
}

type SideEffectAssessor struct{}

func NewSideEffectAssessor() *SideEffectAssessor {
	return &SideEffectAssessor{}
}

func (a *SideEffectAssessor) Assess(ctx context.Context, extensionID string, contributions []ContributionSideEffectInfo) ([]SideEffectAssessment, error) {
	assessments := make([]SideEffectAssessment, 0, len(contributions))
	for _, c := range contributions {
		assessment := SideEffectAssessment{
			ContributionID:  c.ContributionID,
			SideEffectClass: c.SideEffectClass,
		}
		assessment.Reversibility, assessment.CanCompensate, assessment.CompensationAction = a.classify(c.SideEffectClass, c.Actions)
		assessment.Evidence = a.buildEvidence(c.SideEffectClass, c.Actions)
		assessments = append(assessments, assessment)
	}
	return assessments, nil
}

func (a *SideEffectAssessor) classify(class string, actions []string) (string, bool, string) {
	switch class {
	case "message_send", "notification", "email", "sms":
		return "non_reversible", false, ""
	case "external_api_write", "webhook", "api_call":
		return "compensatable", true, "call_compensation_endpoint"
	case "file_write", "file_delete", "file_create":
		return "reversible", true, "restore_from_snapshot"
	case "local_index", "cache", "search_index", "local_cache":
		return "idempotent", true, "rebuild_index"
	case "database_write", "db_insert", "db_update":
		return "compensatable", true, "execute_reverse_migration"
	default:
		return "unknown", false, ""
	}
}

func (a *SideEffectAssessor) buildEvidence(class string, actions []string) string {
	if len(actions) == 0 {
		return class
	}
	result := class + ":"
	for i, act := range actions {
		if i > 0 {
			result += ","
		}
		result += act
	}
	return result
}

func (a *SideEffectAssessor) HasNonReversible(ctx context.Context, assessments []SideEffectAssessment) bool {
	for _, assess := range assessments {
		if assess.Reversibility == "non_reversible" {
			return true
		}
	}
	return false
}

func (a *SideEffectAssessor) RequiresManualIntervention(ctx context.Context, assessments []SideEffectAssessment) bool {
	for _, assess := range assessments {
		if assess.Reversibility == "non_reversible" || assess.Reversibility == "unknown" {
			return true
		}
	}
	return false
}
