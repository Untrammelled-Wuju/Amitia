package prompt

import (
	"sort"
	"strings"
)

const CompilerVersionV1 = "prompt-compiler-v1"

func CompileIR(sections []Section, options CompileOptions) IR {
	options = normalizeOptions(options)
	diagnostics := []string{}
	forceDataOnly := sectionTypeSet(options.ForceDataOnlyTypes)
	compiled := make([]Section, 0, len(sections))
	for _, section := range sections {
		next := normalizeSection(section, options, forceDataOnly, &diagnostics)
		if options.DropEmptySections && strings.TrimSpace(next.Content) == "" {
			diagnostics = append(diagnostics, "empty_section_dropped:"+string(next.Type))
			continue
		}
		compiled = append(compiled, next)
	}
	sort.SliceStable(compiled, func(i, j int) bool {
		if compiled[i].Priority == compiled[j].Priority {
			return sectionRank(compiled[i].Type) < sectionRank(compiled[j].Type)
		}
		return compiled[i].Priority > compiled[j].Priority
	})
	if len(compiled) > options.MaxSections {
		compiled = compiled[:options.MaxSections]
		diagnostics = append(diagnostics, "section_window_truncated")
	}
	return IR{
		Version:  IRVersionV1,
		Sections: compiled,
		Audit: Audit{
			CompilerVersion: CompilerVersionV1,
			Diagnostics:     uniqueStrings(diagnostics),
		},
	}
}

func RenderIR(ir IR) string {
	parts := make([]string, 0, len(ir.Sections))
	for _, section := range ir.Sections {
		content := strings.TrimSpace(section.Content)
		if content == "" {
			continue
		}
		header := "[" + string(section.Type) + "]"
		if section.DataOnly {
			header += "[data_only]"
		}
		parts = append(parts, header+"\n"+content)
	}
	return strings.Join(parts, "\n\n")
}

func SnapshotIR(ir IR) IR {
	next := ir
	next.Sections = make([]Section, 0, len(ir.Sections))
	for _, section := range ir.Sections {
		item := section
		if item.Sensitivity == SensitivityPrivate || item.Sensitivity == SensitivitySecret {
			item.Content = "[redacted]"
		}
		next.Sections = append(next.Sections, item)
	}
	return next
}

func normalizeOptions(options CompileOptions) CompileOptions {
	if options.MaxSections <= 0 {
		options.MaxSections = 64
	}
	if options.MinTokenBudget <= 0 {
		options.MinTokenBudget = 16
	}
	if options.MaxTokenBudget <= 0 {
		options.MaxTokenBudget = 2048
	}
	if options.MaxTokenBudget < options.MinTokenBudget {
		options.MaxTokenBudget = options.MinTokenBudget
	}
	if len(options.ForceDataOnlyTypes) == 0 {
		options.ForceDataOnlyTypes = []SectionType{SectionTypeMemory, SectionTypeHistory, SectionTypeWorldbook}
	}
	return options
}

func normalizeSection(section Section, options CompileOptions, forceDataOnly map[SectionType]bool, diagnostics *[]string) Section {
	if section.Type == "" {
		section.Type = SectionTypeSystem
		*diagnostics = append(*diagnostics, "default_type")
	}
	if section.Sensitivity == "" {
		section.Sensitivity = SensitivityInternal
		*diagnostics = append(*diagnostics, "default_sensitivity:"+string(section.Type))
	}
	if section.Source == "" {
		section.Source = "unknown"
		*diagnostics = append(*diagnostics, "default_source:"+string(section.Type))
	}
	if section.TokenBudget < options.MinTokenBudget {
		section.TokenBudget = options.MinTokenBudget
		*diagnostics = append(*diagnostics, "clamp_min_token_budget:"+string(section.Type))
	}
	if section.TokenBudget > options.MaxTokenBudget {
		section.TokenBudget = options.MaxTokenBudget
		*diagnostics = append(*diagnostics, "clamp_max_token_budget:"+string(section.Type))
	}
	if forceDataOnly[section.Type] && !section.DataOnly {
		section.DataOnly = true
		*diagnostics = append(*diagnostics, "forced_data_only:"+string(section.Type))
	}
	if section.DataOnly && !section.Trimmable && (section.Type == SectionTypeMemory || section.Type == SectionTypeHistory || section.Type == SectionTypeWorldbook) {
		section.Trimmable = true
		*diagnostics = append(*diagnostics, "forced_trimmable:"+string(section.Type))
	}
	if options.RedactSensitive && (section.Sensitivity == SensitivityPrivate || section.Sensitivity == SensitivitySecret) {
		section.Content = "[redacted]"
		*diagnostics = append(*diagnostics, "redacted:"+string(section.Type))
	}
	return section
}

func sectionTypeSet(values []SectionType) map[SectionType]bool {
	result := make(map[SectionType]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func sectionRank(sectionType SectionType) int {
	switch sectionType {
	case SectionTypeSystem:
		return 10
	case SectionTypeBaseIdentity:
		return 15
	case SectionTypeIdentity:
		return 20
	case SectionTypePersonalityRaw:
		return 25
	case SectionTypeBehaviorPlan:
		return 30
	case SectionTypeEmotionFusionRaw:
		return 35
	case SectionTypeAdultIntimacyRaw:
		return 36
	case SectionTypeOutputShapeRaw:
		return 37
	case SectionTypeAntiRepeatRaw:
		return 38
	case SectionTypeProactiveRaw:
		return 39
	case SectionTypePsyche:
		return 40
	case SectionTypeTemporalContext:
		return 41
	case SectionTypeRelationshipTime:
		return 42
	case SectionTypeMemoryInjectRaw:
		return 45
	case SectionTypeMemoryExtractRaw:
		return 46
	case SectionTypeMemory:
		return 50
	case SectionTypeChannelShortRaw:
		return 55
	case SectionTypeWorldbook:
		return 60
	case SectionTypeHistory:
		return 70
	case SectionTypeCurrentInput:
		return 80
	case SectionTypeTraceOnly:
		return 99
	default:
		return 100
	}
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	out := values[:0]
	var last string
	for i, value := range values {
		if i == 0 || value != last {
			out = append(out, value)
			last = value
		}
	}
	return out
}
