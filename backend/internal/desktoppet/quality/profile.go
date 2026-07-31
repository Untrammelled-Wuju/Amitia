// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

const (
	QualityModeFast     = "fast"
	QualityModeBalanced = "balanced"
	QualityModeStrict   = "strict"

	BackgroundPolicyRemoveBackground = "remove_background"
	BackgroundPolicyKeepAlpha        = "keep_alpha"
	BackgroundPolicyOpaqueAllowed    = "opaque_allowed"

	ArtStyleIllustration    = "illustration"
	ArtStylePixelArt        = "pixel_art"
	ArtStyleChibi           = "chibi"
	ArtStyleFlat            = "flat"
	ArtStyleSemiTransparent = "semi_transparent"

	ProfileSchemaVersion = 1
	EngineVersion        = "quality-engine/1.0.0"
)

type QualityProfileSnapshot struct {
	SchemaVersion  int    `json:"schemaVersion"`
	ProfileID      string `json:"profileId"`
	ProfileVersion int    `json:"profileVersion"`
	CreatedAt      string `json:"createdAt"`
	EngineVersion  string `json:"engineVersion"`

	ActionSpecHash string `json:"actionSpecHash"`
	LoopType       string `json:"loopType"`
	FrameCount     int    `json:"frameCount"`
	AnchorProfile  string `json:"anchorProfile"`
	MotionPolicy   MotionPolicy `json:"motionPolicy"`

	Detectors  map[string]DetectorConfig  `json:"detectors"`
	Rules      map[string]RuleConfig      `json:"rules"`
	Dimensions map[string]DimensionConfig `json:"dimensions"`

	HardGateRuleCodes []string `json:"hardGateRuleCodes"`

	RequiredActionPolicy ActionPolicy `json:"requiredActionPolicy"`
	OptionalActionPolicy ActionPolicy `json:"optionalActionPolicy"`

	ArtStyle         string `json:"artStyle"`
	BackgroundPolicy string `json:"backgroundPolicy"`
	QualityMode      string `json:"qualityMode"`

	CanonicalJSON string `json:"canonicalJson"`
	Hash          string `json:"hash"`
}

type MotionPolicy struct {
	AllowHorizontalMotion bool      `json:"allowHorizontalMotion"`
	AllowVerticalMotion   bool      `json:"allowVerticalMotion"`
	AllowScaleChange      bool      `json:"allowScaleChange"`
	MaxAnchorJitter       float64   `json:"maxAnchorJitter"`
	MaxScaleJitter        float64   `json:"maxScaleJitter"`
	MaxMotionJump         float64   `json:"maxMotionJump"`
	AllowedEdges          []string  `json:"allowedEdges"`
}

type DetectorConfig struct {
	Enabled    bool              `json:"enabled"`
	Version    string            `json:"version"`
	Parameters map[string]float64 `json:"parameters"`
	CanDegrade bool              `json:"canDegrade"`
}

type RuleConfig struct {
	RuleVersion     int     `json:"ruleVersion"`
	WarningThreshold  *float64 `json:"warningThreshold,omitempty"`
	ReviewThreshold   *float64 `json:"reviewThreshold,omitempty"`
	RejectThreshold   *float64 `json:"rejectThreshold,omitempty"`
	Comparison      string  `json:"comparison"`
	Severity        Severity `json:"severity"`
	HardGate        bool    `json:"hardGate"`
	MaxPenalty      float64 `json:"maxPenalty"`
}

type DimensionConfig struct {
	Weight            float64 `json:"weight"`
	PassScore         float64 `json:"passScore"`
	MinConfidence     float64 `json:"minConfidence"`
	CriticalDimension bool    `json:"criticalDimension"`
}

type ActionPolicy struct {
	GateRule        string `json:"gateRule"`
	BlockOnRejected bool   `json:"blockOnRejected"`
	BlockOnReview   bool   `json:"blockOnReview"`
}

func (p QualityProfileSnapshot) DetectorEnabled(key string) bool {
	if cfg, ok := p.Detectors[key]; ok {
		return cfg.Enabled
	}
	return true
}

func (p QualityProfileSnapshot) CanDegrade(key string) bool {
	if cfg, ok := p.Detectors[key]; ok {
		return cfg.CanDegrade
	}
	return false
}

func (p QualityProfileSnapshot) IsHardGate(ruleCode string) bool {
	for _, rc := range p.HardGateRuleCodes {
		if rc == ruleCode {
			return true
		}
	}
	if cfg, ok := p.Rules[ruleCode]; ok {
		return cfg.HardGate
	}
	return false
}

func (p QualityProfileSnapshot) GetRuleConfig(ruleCode string) (RuleConfig, bool) {
	cfg, ok := p.Rules[ruleCode]
	return cfg, ok
}

func (p QualityProfileSnapshot) GetDimensionConfig(dim string) (DimensionConfig, bool) {
	cfg, ok := p.Dimensions[dim]
	return cfg, ok
}

func (p QualityProfileSnapshot) GetDetectorConfig(key string) (DetectorConfig, bool) {
	cfg, ok := p.Detectors[key]
	return cfg, ok
}

func CanonicalProfileJSON(p QualityProfileSnapshot) string {
	stable := QualityProfileSnapshot{
		SchemaVersion:    p.SchemaVersion,
		ProfileID:        p.ProfileID,
		ProfileVersion:   p.ProfileVersion,
		EngineVersion:    p.EngineVersion,
		ActionSpecHash:   p.ActionSpecHash,
		LoopType:         p.LoopType,
		FrameCount:       p.FrameCount,
		AnchorProfile:    p.AnchorProfile,
		MotionPolicy:     p.MotionPolicy,
		QualityMode:      p.QualityMode,
	}
	stable.Detectors = make(map[string]DetectorConfig, len(p.Detectors))
	for k, v := range p.Detectors {
		stable.Detectors[k] = v
	}
	stable.Rules = make(map[string]RuleConfig, len(p.Rules))
	for k, v := range p.Rules {
		stable.Rules[k] = v
	}
	stable.Dimensions = make(map[string]DimensionConfig, len(p.Dimensions))
	for k, v := range p.Dimensions {
		stable.Dimensions[k] = v
	}
	stable.HardGateRuleCodes = make([]string, len(p.HardGateRuleCodes))
	copy(stable.HardGateRuleCodes, p.HardGateRuleCodes)
	stable.RequiredActionPolicy = p.RequiredActionPolicy
	stable.OptionalActionPolicy = p.OptionalActionPolicy

	keys := make([]string, 0, len(stable.Detectors))
	for k := range stable.Detectors {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ruleKeys := make([]string, 0, len(stable.Rules))
	for k := range stable.Rules {
		ruleKeys = append(ruleKeys, k)
	}
	sort.Strings(ruleKeys)

	dimKeys := make([]string, 0, len(stable.Dimensions))
	for k := range stable.Dimensions {
		dimKeys = append(dimKeys, k)
	}
	sort.Strings(dimKeys)

	hardGates := make([]string, len(stable.HardGateRuleCodes))
	copy(hardGates, stable.HardGateRuleCodes)
	sort.Strings(hardGates)

	stable.HardGateRuleCodes = hardGates

	data, _ := json.Marshal(stable)
	return string(data)
}

func ComputeProfileHash(p QualityProfileSnapshot) string {
	canonical := CanonicalProfileJSON(p)
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:])
}

func FreezeProfile(p QualityProfileSnapshot) QualityProfileSnapshot {
	p.CanonicalJSON = CanonicalProfileJSON(p)
	p.Hash = ComputeProfileHash(p)
	return p
}
