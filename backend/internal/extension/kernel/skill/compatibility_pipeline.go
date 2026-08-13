package skill

import (
	"context"
	"sort"
)

type CompatibilityPipeline struct {
	detector *ProfileDetector
	merger   *CompatibilityMerger
}

func NewCompatibilityPipeline(adapters ...SkillProfileAdapter) *CompatibilityPipeline {
	return &CompatibilityPipeline{
		detector: NewProfileDetector(adapters...),
		merger:   NewCompatibilityMerger(),
	}
}

func DefaultCompatibilityPipeline() *CompatibilityPipeline {
	return NewCompatibilityPipeline(
		NewClaudeCodeProfileAdapter(),
		NewOpenAIProfileAdapter(),
		NewClaudeLegacyCommandAdapter(),
	)
}

func (p *CompatibilityPipeline) Evaluate(ctx context.Context, pkg SkillPackageView, baseReport *SkillCompatibilityReport) (*CanonicalSkillCompatibility, *SkillCompatibilityReport, error) {
	detected, overlays, err := p.detector.Detect(ctx, pkg, pkg.Parsed)
	if err != nil {
		return nil, nil, err
	}

	canonical, report := p.merger.Merge(overlays, DefaultInvocationPolicy, baseReport)
	report.Detected = detected

	adapterVersions := make(map[string]string)
	for _, ov := range overlays {
		adapterVersions[ov.Profile] = ov.AdapterVersion
	}
	fingerprint := ComputeFingerprint(pkg.ContentHash, adapterVersions)
	report.Fingerprint = &fingerprint

	return canonical, report, nil
}

func SortOverlaysByProfileID(overlays []SkillCompatibilityOverlay) {
	sort.Slice(overlays, func(i, j int) bool {
		return overlays[i].Profile < overlays[j].Profile
	})
}
