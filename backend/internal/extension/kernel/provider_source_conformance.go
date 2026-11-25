package kernel

import (
	"fmt"
	"sort"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type ProviderConformanceGapKind string

const (
	ProviderGapRuntimeUndeclared ProviderConformanceGapKind = "runtime.undeclared"
	ProviderGapDeclaredMissing   ProviderConformanceGapKind = "declared.missing"
)

type ProviderConformanceGap struct {
	Kind         ProviderConformanceGapKind `json:"kind"`
	ExtensionID  string                     `json:"extensionId"`
	ModuleID     string                     `json:"moduleId,omitempty"`
	CapabilityID string                     `json:"capabilityId,omitempty"`
	ProviderID   string                     `json:"providerId,omitempty"`
	Detail       string                     `json:"detail"`
}

type ProviderConformanceReport struct {
	DeclaredProviders int                      `json:"declaredProviders"`
	RuntimeProviders  int                      `json:"runtimeProviders"`
	Gaps              []ProviderConformanceGap `json:"gaps"`
}

func (r ProviderConformanceReport) Conformant() bool {
	return len(r.Gaps) == 0
}

func (r ProviderConformanceReport) BlockingGaps() []ProviderConformanceGap {
	return filterProviderGaps(r.Gaps, ProviderGapRuntimeUndeclared)
}

func (r ProviderConformanceReport) AdvisoryGaps() []ProviderConformanceGap {
	return filterProviderGaps(r.Gaps, ProviderGapDeclaredMissing)
}

func filterProviderGaps(gaps []ProviderConformanceGap, kind ProviderConformanceGapKind) []ProviderConformanceGap {
	filtered := make([]ProviderConformanceGap, 0, len(gaps))
	for _, gap := range gaps {
		if gap.Kind == kind {
			filtered = append(filtered, gap)
		}
	}
	return filtered
}

func providerConformanceKey(extensionID, moduleID, capabilityID string) string {
	return strings.TrimSpace(extensionID) + "|" + strings.TrimSpace(moduleID) + "|" + strings.TrimSpace(capabilityID)
}

func CheckProviderConformance(defs []domain.ExtensionDefinition, registry *capability.ProviderRegistry) (ProviderConformanceReport, error) {
	if registry == nil {
		return ProviderConformanceReport{}, fmt.Errorf("provider conformance: registry is nil")
	}
	declared := make(map[string]domain.ProvidedCapability)
	report := ProviderConformanceReport{}
	for _, def := range defs {
		for _, module := range def.Modules {
			for _, provided := range module.ProvidedCapabilities {
				capabilityID := strings.TrimSpace(provided.ID)
				if capabilityID == "" {
					continue
				}
				key := providerConformanceKey(string(def.ID), string(module.ID), capabilityID)
				declared[key] = provided
			}
		}
	}
	report.DeclaredProviders = len(declared)
	observed := make(map[string]*capability.CapabilityProviderDefinition)
	for _, definition := range registry.ListAllProviders() {
		if definition == nil {
			continue
		}
		extensionID := strings.TrimSpace(definition.ExtensionID)
		if extensionID == "" {
			continue
		}
		capabilityID := strings.TrimSpace(string(definition.CapabilityID))
		report.RuntimeProviders++
		key := providerConformanceKey(extensionID, definition.ModuleID, capabilityID)
		if _, exists := observed[key]; !exists {
			observed[key] = definition
		}
		if _, ok := declared[key]; ok {
			continue
		}
		report.Gaps = append(report.Gaps, ProviderConformanceGap{
			Kind:         ProviderGapRuntimeUndeclared,
			ExtensionID:  extensionID,
			ModuleID:     strings.TrimSpace(definition.ModuleID),
			CapabilityID: capabilityID,
			ProviderID:   string(definition.ID),
			Detail:       fmt.Sprintf("runtime provider %s has no manifest declaration for capability %s", definition.ID, capabilityID),
		})
	}
	for key := range declared {
		if _, ok := observed[key]; ok {
			continue
		}
		extensionID, moduleID, capabilityID := splitProviderConformanceKey(key)
		report.Gaps = append(report.Gaps, ProviderConformanceGap{
			Kind:         ProviderGapDeclaredMissing,
			ExtensionID:  extensionID,
			ModuleID:     moduleID,
			CapabilityID: capabilityID,
			Detail:       fmt.Sprintf("manifest declares capability %s for module %s/%s but no runtime provider is registered", capabilityID, extensionID, moduleID),
		})
	}
	sort.Slice(report.Gaps, func(i, j int) bool {
		if report.Gaps[i].Kind != report.Gaps[j].Kind {
			return report.Gaps[i].Kind < report.Gaps[j].Kind
		}
		if report.Gaps[i].ExtensionID != report.Gaps[j].ExtensionID {
			return report.Gaps[i].ExtensionID < report.Gaps[j].ExtensionID
		}
		if report.Gaps[i].ModuleID != report.Gaps[j].ModuleID {
			return report.Gaps[i].ModuleID < report.Gaps[j].ModuleID
		}
		return report.Gaps[i].CapabilityID < report.Gaps[j].CapabilityID
	})
	return report, nil
}

func splitProviderConformanceKey(key string) (string, string, string) {
	parts := strings.SplitN(key, "|", 3)
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	return parts[0], parts[1], parts[2]
}
