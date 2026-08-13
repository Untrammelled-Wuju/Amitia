package skill

import (
	"context"
	"sort"
	"strings"
)

type ProfileDetector struct {
	adapters []SkillProfileAdapter
}

func NewProfileDetector(adapters ...SkillProfileAdapter) *ProfileDetector {
	return &ProfileDetector{adapters: adapters}
}

func (d *ProfileDetector) Detect(ctx context.Context, pkg SkillPackageView, parsed ParsedSkill) ([]SkillEcosystemProfile, []SkillCompatibilityOverlay, error) {
	var detected []SkillEcosystemProfile
	var overlays []SkillCompatibilityOverlay

	for _, adapter := range d.adapters {
		detection, err := adapter.Detect(ctx, pkg, parsed)
		if err != nil {
			continue
		}
		if len(detection.Detected) == 0 {
			continue
		}
		for _, prof := range detection.Detected {
			detected = append(detected, prof)
		}
		overlay, err := adapter.Analyze(ctx, pkg, parsed)
		if err != nil {
			overlay = SkillCompatibilityOverlay{
				Profile:        adapter.ID(),
				AdapterVersion: adapter.Version(),
				Errors: []SkillError{
					{Code: "ADAPTER_ANALYZE_ERROR", Message: err.Error(), Path: pkg.RootURI},
				},
			}
		}
		overlays = append(overlays, overlay)
	}

	sort.Slice(detected, func(i, j int) bool {
		return detected[i].ID < detected[j].ID
	})
	return detected, overlays, nil
}

func detectExtraFields(extra map[string]interface{}, keys ...string) []string {
	var evidence []string
	for _, k := range keys {
		if _, ok := extra[k]; ok {
			evidence = append(evidence, "extra:"+k)
		}
	}
	return evidence
}

func hasFileType(files map[string][]byte, path string) bool {
	_, ok := files[path]
	return ok
}

func bodyContains(body string, needles ...string) []string {
	lower := strings.ToLower(body)
	var found []string
	for _, n := range needles {
		if strings.Contains(lower, strings.ToLower(n)) {
			found = append(found, n)
		}
	}
	return found
}
