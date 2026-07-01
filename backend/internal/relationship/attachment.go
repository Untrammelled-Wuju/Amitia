package relationship

import (
	"math"
)

func DefaultAttachmentProfile() AttachmentProfile {
	return AttachmentProfile{
		Style:               AttachmentSecure,
		RecoverySpeed:       1.0,
		ConflictSensitivity: 0.5,
		ProtestIntensity:    0.2,
	}
}

func ComputeSecurityScore(dims RelationshipDimensions) float64 {
	trustW := dims.Trust.Value / 100
	intimacyW := dims.Intimacy.Value / 100
	dependencyW := dims.Dependency.Value / 100
	conflictW := dims.Conflict.Value / 100
	repairW := dims.Repair.Value / 100

	base := trustW*0.30 + intimacyW*0.30 + dependencyW*0.15
	penalty := conflictW * 0.15
	repairBoost := repairW * 0.10 * (1 - conflictW)
	score := base - penalty + repairBoost
	return round4(clamp01(score))
}

func ComputeSecurityFromState(state RelationshipState) float64 {
	trustW := state.Trust
	intimacyW := state.Familiarity
	dependencyW := state.Security
	conflictW := state.Tension
	repairW := state.RepairConfidence

	base := trustW*0.30 + intimacyW*0.30 + dependencyW*0.15
	penalty := conflictW * 0.15
	repairBoost := repairW * 0.10 * (1 - conflictW)
	score := base - penalty + repairBoost
	return round4(clamp01(score))
}

func AttachmentRecoveryMultiplier(profile AttachmentProfile) float64 {
	switch profile.Style {
	case AttachmentSecure:
		return 1.0
	case AttachmentAnxious:
		return clampRange(0.5, 0.9, profile.RecoverySpeed)
	case AttachmentDismiss:
		return 1.2
	case AttachmentFearful:
		return clampRange(0.3, 0.7, profile.RecoverySpeed)
	default:
		return 1.0
	}
}

func AttachmentConflictModifier(profile AttachmentProfile) float64 {
	switch profile.Style {
	case AttachmentSecure:
		return 1.0
	case AttachmentAnxious:
		mult := profile.ConflictSensitivity
		if mult <= 0 {
			mult = 1.3
		}
		return mult
	case AttachmentDismiss:
		mult := profile.ConflictSensitivity
		if mult <= 0 {
			mult = 0.7
		}
		return mult
	case AttachmentFearful:
		mult := profile.ConflictSensitivity
		if mult <= 0 {
			mult = 1.5
		}
		return mult
	default:
		return 1.0
	}
}

func AttachmentProtestBehavior(profile AttachmentProfile, security float64) bool {
	if profile.Style == AttachmentSecure {
		return false
	}
	threshold := 0.35
	if profile.Style == AttachmentAnxious {
		threshold = 0.50
	}
	if profile.Style == AttachmentFearful {
		threshold = 0.40
	}
	if profile.ProtestIntensity > 0.6 {
		threshold += (profile.ProtestIntensity - 0.6) * 0.2
	}
	return security < threshold
}

func AdjustTensionDecayForAttachment(profile AttachmentProfile, baseDecay float64) float64 {
	recovery := AttachmentRecoveryMultiplier(profile)
	adjusted := baseDecay * recovery
	return math.Max(0.005, adjusted)
}
