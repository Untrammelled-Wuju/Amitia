// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import (
	"time"
)

type ProfileRegistry struct {
	base          QualityProfileSnapshot
	categories    map[string]QualityProfileSnapshot
	actions       map[string]string
	artOverrides  map[string]QualityProfileSnapshot
	bgOverrides   map[string]QualityProfileSnapshot
	modeOverrides map[string]QualityProfileSnapshot
}

func NewProfileRegistry() *ProfileRegistry {
	r := &ProfileRegistry{
		categories:    make(map[string]QualityProfileSnapshot),
		actions:       make(map[string]string),
		artOverrides:  make(map[string]QualityProfileSnapshot),
		bgOverrides:   make(map[string]QualityProfileSnapshot),
		modeOverrides: make(map[string]QualityProfileSnapshot),
	}
	r.base = baseProfile()
	r.initCategoryProfiles()
	r.initActionMappings()
	r.initArtOverrides()
	r.initBgOverrides()
	r.initModeOverrides()
	return r
}

func baseProfile() QualityProfileSnapshot {
	return QualityProfileSnapshot{
		SchemaVersion:  ProfileSchemaVersion,
		ProfileID:      "base",
		ProfileVersion: 1,
		EngineVersion:  EngineVersion,
		Detectors:      defaultDetectors(),
		Rules:          defaultRules(),
		Dimensions:     defaultDimensions(),
		HardGateRuleCodes: []string{
			RuleFileMissing, RuleFileUndecodable, RuleFileHashMismatch,
			RuleFrameCountMismatch, RuleFrameIndexGap,
			RuleAlphaAllTransparent, RuleSubjectEmpty, RuleSubjectClipped,
		},
		RequiredActionPolicy: ActionPolicy{GateRule: "block_on_rejected", BlockOnRejected: true, BlockOnReview: true},
		OptionalActionPolicy: ActionPolicy{GateRule: "excluded_candidate", BlockOnRejected: false, BlockOnReview: false},
		ArtStyle:             ArtStyleIllustration,
		BackgroundPolicy:     BackgroundPolicyRemoveBackground,
		QualityMode:          QualityModeBalanced,
		MotionPolicy: MotionPolicy{
			MaxAnchorJitter: 0.04,
			MaxScaleJitter:  0.08,
			MaxMotionJump:   0.06,
		},
	}
}

func defaultDetectors() map[string]DetectorConfig {
	return map[string]DetectorConfig{
		"integrity":  {Enabled: true, Version: "1.0", CanDegrade: false},
		"alpha":      {Enabled: true, Version: "1.0", CanDegrade: false},
		"subject":    {Enabled: true, Version: "1.0", CanDegrade: false},
		"edge":       {Enabled: true, Version: "1.0", CanDegrade: false},
		"background": {Enabled: true, Version: "1.0", CanDegrade: false},
		"stability":  {Enabled: true, Version: "1.0", CanDegrade: false},
		"identity":   {Enabled: true, Version: "1.0", CanDegrade: true},
		"motion":     {Enabled: true, Version: "1.0", CanDegrade: false},
		"duplicate":  {Enabled: true, Version: "1.0", CanDegrade: false},
		"loop":       {Enabled: true, Version: "1.0", CanDegrade: false},
		"color":      {Enabled: true, Version: "1.0", CanDegrade: false},
	}
}

func defaultRules() map[string]RuleConfig {
	return map[string]RuleConfig{
		RuleFileMissing:                {RuleVersion: 1, Severity: SeverityCritical, HardGate: true, Comparison: "==", MaxPenalty: 100},
		RuleFileUndecodable:            {RuleVersion: 1, Severity: SeverityCritical, HardGate: true, Comparison: "==", MaxPenalty: 100},
		RuleFileHashMismatch:           {RuleVersion: 1, Severity: SeverityCritical, HardGate: true, Comparison: "!=", MaxPenalty: 100},
		RuleFrameCountMismatch:         {RuleVersion: 1, Severity: SeverityCritical, HardGate: true, Comparison: "!=", MaxPenalty: 100},
		RuleFrameIndexGap:              {RuleVersion: 1, Severity: SeverityCritical, HardGate: true, Comparison: "gap", MaxPenalty: 100},
		RuleFrameDimensionMismatch:     {RuleVersion: 1, Severity: SeverityError, HardGate: true, Comparison: "!=", MaxPenalty: 100},
		RuleAlphaAllTransparent:        {RuleVersion: 1, Severity: SeverityCritical, HardGate: true, Comparison: "==", MaxPenalty: 100},
		RuleAlphaPolicyViolation:       {RuleVersion: 1, Severity: SeverityError, HardGate: false, Comparison: "policy", MaxPenalty: 40},
		RuleSubjectEmpty:               {RuleVersion: 1, Severity: SeverityCritical, HardGate: true, Comparison: "==", MaxPenalty: 100},
		RuleSubjectTooSmall:            {RuleVersion: 1, Severity: SeverityReview, WarningThreshold: f64(0.03), ReviewThreshold: f64(0.01), RejectThreshold: f64(0.005), Comparison: "<", MaxPenalty: 30},
		RuleSubjectTooLarge:            {RuleVersion: 1, Severity: SeverityReview, WarningThreshold: f64(0.90), ReviewThreshold: f64(0.95), RejectThreshold: f64(0.98), Comparison: ">", MaxPenalty: 30},
		RuleSubjectFragmented:          {RuleVersion: 1, Severity: SeverityReview, WarningThreshold: f64(0.15), ReviewThreshold: f64(0.25), Comparison: ">", MaxPenalty: 25},
		RuleSubjectClipped:             {RuleVersion: 1, Severity: SeverityError, HardGate: true, Comparison: "clip", MaxPenalty: 100},
		RuleUnexpectedEdgeContact:      {RuleVersion: 1, Severity: SeverityWarning, WarningThreshold: f64(0.02), ReviewThreshold: f64(0.05), Comparison: ">", MaxPenalty: 15},
		RuleBackgroundResidueComponent: {RuleVersion: 1, Severity: SeverityWarning, WarningThreshold: f64(0.02), ReviewThreshold: f64(0.05), Comparison: ">", MaxPenalty: 25},
		RuleAlphaHalo:                  {RuleVersion: 1, Severity: SeverityWarning, WarningThreshold: f64(3), ReviewThreshold: f64(8), Comparison: ">", MaxPenalty: 15},
		RuleAnchorJitter:               {RuleVersion: 1, Severity: SeverityWarning, WarningThreshold: f64(0.04), ReviewThreshold: f64(0.06), RejectThreshold: f64(0.10), Comparison: ">", MaxPenalty: 20},
		RuleScaleJitter:                {RuleVersion: 1, Severity: SeverityWarning, WarningThreshold: f64(0.08), ReviewThreshold: f64(0.12), RejectThreshold: f64(0.20), Comparison: ">", MaxPenalty: 20},
		RuleIdentityDrift:              {RuleVersion: 1, Severity: SeverityReview, WarningThreshold: f64(0.15), ReviewThreshold: f64(0.25), RejectThreshold: f64(0.40), Comparison: ">", MaxPenalty: 35},
		RuleMotionJump:                 {RuleVersion: 1, Severity: SeverityReview, WarningThreshold: f64(0.06), ReviewThreshold: f64(0.10), RejectThreshold: f64(0.15), Comparison: ">", MaxPenalty: 25},
		RuleMotionDirectionReversal:    {RuleVersion: 1, Severity: SeverityWarning, WarningThreshold: f64(2), ReviewThreshold: f64(3), Comparison: ">", MaxPenalty: 15},
		RuleExactDuplicateFrame:        {RuleVersion: 1, Severity: SeverityInfo, WarningThreshold: f64(1), Comparison: ">=", MaxPenalty: 10},
		RulePerceptualDuplicateFrame:   {RuleVersion: 1, Severity: SeverityWarning, WarningThreshold: f64(0.98), ReviewThreshold: f64(0.995), Comparison: ">", MaxPenalty: 15},
		RuleFrozenSequence:             {RuleVersion: 1, Severity: SeverityReview, WarningThreshold: f64(3), ReviewThreshold: f64(5), RejectThreshold: f64(8), Comparison: ">=", MaxPenalty: 30},
		RuleLoopDiscontinuity:          {RuleVersion: 1, Severity: SeverityWarning, WarningThreshold: f64(0.15), ReviewThreshold: f64(0.25), RejectThreshold: f64(0.40), Comparison: ">", MaxPenalty: 25},
		RuleLoopVelocityDiscontinuity:  {RuleVersion: 1, Severity: SeverityReview, WarningThreshold: f64(0.5), Comparison: ">", MaxPenalty: 20},
		RuleColorFlicker:               {RuleVersion: 1, Severity: SeverityWarning, WarningThreshold: f64(0.15), ReviewThreshold: f64(0.25), Comparison: ">", MaxPenalty: 20},
		RuleLowEvaluationConfidence:    {RuleVersion: 1, Severity: SeverityReview, WarningThreshold: f64(0.55), Comparison: "<", MaxPenalty: 0},
		RuleMissingMeasurement:         {RuleVersion: 1, Severity: SeverityError, HardGate: false, Comparison: "missing", MaxPenalty: 50},
		RuleDetectorFailure:            {RuleVersion: 1, Severity: SeverityCritical, HardGate: false, Comparison: "failure", MaxPenalty: 0},
		RuleLegacyFlagImported:         {RuleVersion: 1, Severity: SeverityInfo, Comparison: "legacy", MaxPenalty: 0},
	}
}

func defaultDimensions() map[string]DimensionConfig {
	return map[string]DimensionConfig{
		DimensionIntegrity:             {Weight: 0, PassScore: 100, MinConfidence: 1.0, CriticalDimension: true},
		DimensionSubjectIntegrity:      {Weight: 1.5, PassScore: 75, MinConfidence: 0.7, CriticalDimension: true},
		DimensionBackgroundCleanliness: {Weight: 1.0, PassScore: 75, MinConfidence: 0.7},
		DimensionAnchorStability:       {Weight: 1.2, PassScore: 75, MinConfidence: 0.7, CriticalDimension: true},
		DimensionIdentityConsistency:   {Weight: 1.3, PassScore: 75, MinConfidence: 0.65, CriticalDimension: true},
		DimensionMotionContinuity:      {Weight: 1.0, PassScore: 75, MinConfidence: 0.65},
		DimensionLoopContinuity:        {Weight: 0.8, PassScore: 75, MinConfidence: 0.7},
		DimensionVisualConsistency:     {Weight: 1.0, PassScore: 75, MinConfidence: 0.7},
		DimensionEvaluationConfidence:  {Weight: 0.5, PassScore: 75, MinConfidence: 0.75},
	}
}

func (r *ProfileRegistry) initCategoryProfiles() {
	r.categories["idle_subtle_motion"] = idleSubtleMotionProfile()
	r.categories["locomotion_left"] = locomotionLeftProfile()
	r.categories["locomotion_right"] = locomotionRightProfile()
	r.categories["vertical_jump"] = verticalJumpProfile()
	r.categories["vertical_land"] = verticalLandProfile()
	r.categories["pose_transition"] = poseTransitionProfile()
	r.categories["short_reaction"] = shortReactionProfile()
	r.categories["emotion_expression"] = emotionExpressionProfile()
	r.categories["stationary_activity"] = stationaryActivityProfile()
	r.categories["external_motion"] = externalMotionProfile()
	r.categories["vertical_fall"] = verticalFallProfile()
	r.categories["surface_contact_hold"] = surfaceContactHoldProfile()
	r.categories["edge_motion"] = edgeMotionProfile()
}

func idleSubtleMotionProfile() QualityProfileSnapshot {
	p := QualityProfileSnapshot{}
	p.MotionPolicy = MotionPolicy{
		MaxAnchorJitter: 0.02,
		MaxScaleJitter:  0.03,
		MaxMotionJump:   0.03,
	}
	return p
}

func locomotionLeftProfile() QualityProfileSnapshot {
	p := QualityProfileSnapshot{}
	p.MotionPolicy = MotionPolicy{
		AllowHorizontalMotion: true,
		MaxAnchorJitter:       0.06,
		MaxScaleJitter:        0.05,
		MaxMotionJump:         0.08,
	}
	return p
}

func locomotionRightProfile() QualityProfileSnapshot {
	p := QualityProfileSnapshot{}
	p.MotionPolicy = MotionPolicy{
		AllowHorizontalMotion: true,
		MaxAnchorJitter:       0.06,
		MaxScaleJitter:        0.05,
		MaxMotionJump:         0.08,
	}
	return p
}

func verticalJumpProfile() QualityProfileSnapshot {
	p := QualityProfileSnapshot{}
	p.MotionPolicy = MotionPolicy{
		AllowVerticalMotion: true,
		MaxAnchorJitter:     0.05,
		MaxScaleJitter:      0.08,
		MaxMotionJump:       0.10,
	}
	return p
}

func verticalLandProfile() QualityProfileSnapshot {
	p := QualityProfileSnapshot{}
	p.MotionPolicy = MotionPolicy{
		AllowVerticalMotion: true,
		AllowScaleChange:    true,
		MaxAnchorJitter:     0.06,
		MaxScaleJitter:      0.12,
		MaxMotionJump:       0.10,
	}
	return p
}

func poseTransitionProfile() QualityProfileSnapshot {
	p := QualityProfileSnapshot{}
	p.MotionPolicy = MotionPolicy{
		AllowScaleChange: true,
		MaxAnchorJitter:  0.05,
		MaxScaleJitter:   0.15,
		MaxMotionJump:    0.08,
	}
	return p
}

func shortReactionProfile() QualityProfileSnapshot {
	p := QualityProfileSnapshot{}
	p.MotionPolicy = MotionPolicy{
		MaxAnchorJitter: 0.04,
		MaxScaleJitter:  0.06,
		MaxMotionJump:   0.06,
	}
	return p
}

func emotionExpressionProfile() QualityProfileSnapshot {
	p := QualityProfileSnapshot{}
	p.MotionPolicy = MotionPolicy{
		MaxAnchorJitter: 0.03,
		MaxScaleJitter:  0.04,
		MaxMotionJump:   0.05,
	}
	return p
}

func stationaryActivityProfile() QualityProfileSnapshot {
	p := QualityProfileSnapshot{}
	p.MotionPolicy = MotionPolicy{
		MaxAnchorJitter: 0.03,
		MaxScaleJitter:  0.04,
		MaxMotionJump:   0.04,
	}
	return p
}

func externalMotionProfile() QualityProfileSnapshot {
	p := QualityProfileSnapshot{}
	p.MotionPolicy = MotionPolicy{
		AllowHorizontalMotion: true,
		AllowVerticalMotion:   true,
		MaxAnchorJitter:       0.15,
		MaxScaleJitter:        0.10,
		MaxMotionJump:         0.12,
	}
	return p
}

func verticalFallProfile() QualityProfileSnapshot {
	p := QualityProfileSnapshot{}
	p.MotionPolicy = MotionPolicy{
		AllowVerticalMotion: true,
		MaxAnchorJitter:     0.08,
		MaxScaleJitter:      0.06,
		MaxMotionJump:       0.12,
	}
	return p
}

func surfaceContactHoldProfile() QualityProfileSnapshot {
	p := QualityProfileSnapshot{}
	p.MotionPolicy = MotionPolicy{
		MaxAnchorJitter: 0.03,
		MaxScaleJitter:  0.04,
		MaxMotionJump:   0.04,
		AllowedEdges:    []string{"bottom"},
	}
	return p
}

func edgeMotionProfile() QualityProfileSnapshot {
	p := QualityProfileSnapshot{}
	p.MotionPolicy = MotionPolicy{
		AllowVerticalMotion: true,
		MaxAnchorJitter:     0.06,
		MaxScaleJitter:      0.05,
		MaxMotionJump:       0.08,
		AllowedEdges:        []string{"bottom", "left", "right"},
	}
	return p
}

func (r *ProfileRegistry) initActionMappings() {
	m := map[string]string{
		"idle_normal":      "idle_subtle_motion",
		"idle_breathing":   "idle_subtle_motion",
		"idle_blink":       "idle_subtle_motion",
		"idle_look_around": "idle_subtle_motion",
		"idle_sway":        "idle_subtle_motion",
		"walk_left":        "locomotion_left",
		"walk_right":       "locomotion_right",
		"run_left":         "locomotion_left",
		"run_right":        "locomotion_right",
		"jump":             "vertical_jump",
		"land":             "vertical_land",
		"turn_around":      "pose_transition",
		"wave":             "short_reaction",
		"nod":              "short_reaction",
		"shake_head":       "short_reaction",
		"clap":             "short_reaction",
		"point":            "short_reaction",
		"stretch":          "pose_transition",
		"bow":              "pose_transition",
		"happy":            "emotion_expression",
		"excited":          "emotion_expression",
		"shy":              "emotion_expression",
		"sad":              "emotion_expression",
		"cry":              "emotion_expression",
		"angry":            "emotion_expression",
		"surprised":        "emotion_expression",
		"confused":         "emotion_expression",
		"embarrassed":      "emotion_expression",
		"scared":           "emotion_expression",
		"proud":            "emotion_expression",
		"tired":            "emotion_expression",
		"sit":              "pose_transition",
		"sleep":            "stationary_activity",
		"wake_up":          "pose_transition",
		"eat":              "stationary_activity",
		"drink":            "stationary_activity",
		"read":             "stationary_activity",
		"write":            "stationary_activity",
		"use_phone":        "stationary_activity",
		"work":             "stationary_activity",
		"study":            "stationary_activity",
		"clicked":          "short_reaction",
		"double_clicked":   "short_reaction",
		"hovered":          "short_reaction",
		"dragged":          "external_motion",
		"picked_up":        "external_motion",
		"dropped":          "external_motion",
		"fall":             "vertical_fall",
		"edge_sit":         "surface_contact_hold",
		"edge_climb":       "edge_motion",
		"sleep_on_desktop": "surface_contact_hold",
		"listening":        "stationary_activity",
		"thinking":         "stationary_activity",
		"speaking":         "stationary_activity",
		"agreeing":         "short_reaction",
		"disagreeing":      "short_reaction",
		"waiting":          "stationary_activity",
		"greeting":         "short_reaction",
		"goodbye":          "short_reaction",
	}
	r.actions = m
}

func (r *ProfileRegistry) initArtOverrides() {
	r.artOverrides[ArtStylePixelArt] = QualityProfileSnapshot{}
	r.artOverrides[ArtStyleSemiTransparent] = QualityProfileSnapshot{}
}

func (r *ProfileRegistry) initBgOverrides() {
	r.bgOverrides[BackgroundPolicyOpaqueAllowed] = QualityProfileSnapshot{}
	r.bgOverrides[BackgroundPolicyKeepAlpha] = QualityProfileSnapshot{}
}

func (r *ProfileRegistry) initModeOverrides() {
	r.modeOverrides[QualityModeFast] = QualityProfileSnapshot{}
	r.modeOverrides[QualityModeStrict] = QualityProfileSnapshot{}
}

func (r *ProfileRegistry) ComposeProfile(actionKey, actionSpecHash, loopType, anchorProfile string, frameCount int, backgroundPolicy, artStyle, qualityMode string) QualityProfileSnapshot {
	p := r.base

	p.ActionSpecHash = actionSpecHash
	p.LoopType = loopType
	p.AnchorProfile = anchorProfile
	p.FrameCount = frameCount
	p.BackgroundPolicy = backgroundPolicy
	p.ArtStyle = artStyle
	p.QualityMode = qualityMode
	p.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	if catKey, ok := r.actions[actionKey]; ok {
		if cat, ok2 := r.categories[catKey]; ok2 {
			p.MotionPolicy = cat.MotionPolicy
		}
	}

	p = FreezeProfile(p)
	return p
}

func (r *ProfileRegistry) GetActionCategory(actionKey string) string {
	if cat, ok := r.actions[actionKey]; ok {
		return cat
	}
	return "idle_subtle_motion"
}

func f64(v float64) *float64 {
	return &v
}
