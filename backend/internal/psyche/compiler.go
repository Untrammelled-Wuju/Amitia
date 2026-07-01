package psyche

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
)

const (
	DefaultSchemaVersion   = "personality-config-v1"
	DefaultCompilerVersion = "psyche-compiler-v1"
)

type defaultSet struct {
	Initiative        float64
	Sensitivity       float64
	Tolerance         float64
	Stability         float64
	Boundary          float64
	Warmth            float64
	Directness        float64
	Humor             float64
	Affection         float64
	Verbosity         float64
	ConflictAvoidance float64
}

func baselineDefaults() defaultSet {
	return defaultSet{
		Initiative:        50,
		Sensitivity:       45,
		Tolerance:         55,
		Stability:         60,
		Boundary:          55,
		Warmth:            65,
		Directness:        50,
		Humor:             40,
		Affection:         45,
		Verbosity:         50,
		ConflictAvoidance: 50,
	}
}

func defaultsForSchemaVersion(version string) defaultSet {
	return baselineDefaults()
}

func DefaultConfig() PersonalityConfig {
	return PersonalityConfig{}
}

func MigratePersonalityConfig(cfg PersonalityConfig) (PersonalityConfig, PersonalityMigration) {
	snapshot := cfg
	from := normalizeSchemaVersion(cfg)
	defaults := defaultsForSchemaVersion(from)
	sources := map[string]string{}
	diagnostics := []string{}

	cfg.SchemaVersion = DefaultSchemaVersion
	cfg.PersonalitySchemaVersion = DefaultSchemaVersion
	sources["schemaVersion"] = schemaSource(snapshot.SchemaVersion, snapshot.PersonalitySchemaVersion)
	sources["personality_schema_version"] = schemaSource(snapshot.PersonalitySchemaVersion, snapshot.SchemaVersion)

	cfg.Initiative = migrateScore("initiative", cfg.Initiative, defaults.Initiative, sources)
	cfg.Sensitivity = migrateScore("sensitivity", cfg.Sensitivity, defaults.Sensitivity, sources)
	cfg.Tolerance = migrateScore("tolerance", cfg.Tolerance, defaults.Tolerance, sources)
	cfg.Stability = migrateScore("stability", cfg.Stability, defaults.Stability, sources)
	cfg.Boundary = migrateScore("boundary", cfg.Boundary, defaults.Boundary, sources)
	cfg.Warmth = migrateScore("warmth", cfg.Warmth, defaults.Warmth, sources)
	cfg.Directness = migrateScore("directness", cfg.Directness, defaults.Directness, sources)
	cfg.Humor = migrateScore("humor", cfg.Humor, defaults.Humor, sources)
	cfg.Affection = migrateScore("affection", cfg.Affection, defaults.Affection, sources)
	cfg.Verbosity = migrateScore("verbosity", cfg.Verbosity, defaults.Verbosity, sources)
	cfg.ConflictAvoidance = migrateScore("conflictAvoidance", cfg.ConflictAvoidance, defaults.ConflictAvoidance, sources)

	if from != "" && from != DefaultSchemaVersion {
		diagnostics = append(diagnostics, "personality_schema_migrated")
	}

	return cfg, PersonalityMigration{
		FromSchema:  displaySchemaVersion(from),
		ToSchema:    DefaultSchemaVersion,
		Snapshot:    snapshot,
		Sources:     sources,
		Diagnostics: diagnostics,
	}
}

func CompilePersonality(cfg PersonalityConfig) CompiledProfile {
	migrated, migration := MigratePersonalityConfig(cfg)
	cfg = migrated
	defaults := defaultsForSchemaVersion(cfg.SchemaVersion)
	sources := compileSourcesFromMigration(migration.Sources)
	diagnostics := append([]string{}, migration.Diagnostics...)
	resolved := ResolvedConfig{
		SchemaVersion:     resolveSchemaVersion(cfg.SchemaVersion, cfg.PersonalitySchemaVersion, sources),
		Initiative:        resolveScore("initiative", cfg.Initiative, defaults.Initiative, sources, &diagnostics),
		Sensitivity:       resolveScore("sensitivity", cfg.Sensitivity, defaults.Sensitivity, sources, &diagnostics),
		Tolerance:         resolveScore("tolerance", cfg.Tolerance, defaults.Tolerance, sources, &diagnostics),
		Stability:         resolveScore("stability", cfg.Stability, defaults.Stability, sources, &diagnostics),
		Boundary:          resolveScore("boundary", cfg.Boundary, defaults.Boundary, sources, &diagnostics),
		Warmth:            resolveScore("warmth", cfg.Warmth, defaults.Warmth, sources, &diagnostics),
		Directness:        resolveScore("directness", cfg.Directness, defaults.Directness, sources, &diagnostics),
		Humor:             resolveScore("humor", cfg.Humor, defaults.Humor, sources, &diagnostics),
		Affection:         resolveScore("affection", cfg.Affection, defaults.Affection, sources, &diagnostics),
		Verbosity:         resolveScore("verbosity", cfg.Verbosity, defaults.Verbosity, sources, &diagnostics),
		ConflictAvoidance: resolveScore("conflictAvoidance", cfg.ConflictAvoidance, defaults.ConflictAvoidance, sources, &diagnostics),
	}

	initiative := normalize(resolved.Initiative)
	sensitivity := normalize(resolved.Sensitivity)
	tolerance := normalize(resolved.Tolerance)
	stability := normalize(resolved.Stability)
	boundary := normalize(resolved.Boundary)
	warmth := normalize(resolved.Warmth)
	directness := normalize(resolved.Directness)
	humor := normalize(resolved.Humor)
	affection := normalize(resolved.Affection)
	verbosity := normalize(resolved.Verbosity)
	conflictAvoidance := normalize(resolved.ConflictAvoidance)

	internal := InternalModel{
		StableCore: StableCoreLayer{
			SocialInitiative:     round4(initiative),
			RejectionSensitivity: round4(clamp01(sensitivity*0.65 + (1-stability)*0.35)),
			UncertaintyTolerance: round4(tolerance),
			EmotionStability:     round4(stability),
			BoundaryStrength:     round4(boundary),
		},
		Growth: GrowthLayer{
			Warmth:      round4(warmth),
			Humor:       round4(humor),
			Affection:   round4(affection),
			SupportBias: round4(clamp01(warmth*0.5 + affection*0.35 + stability*0.15)),
		},
		Situational: SituationalLayer{
			Directness:        round4(directness),
			Verbosity:         round4(verbosity),
			ConflictAvoidance: round4(conflictAvoidance),
		},
	}

	appraisal := compileAppraisal(internal, warmth, affection, directness)
	recovery := compileRecovery(internal, sensitivity, tolerance, verbosity)
	behavior := compileBehavior(internal, warmth, humor, affection, sensitivity, directness, boundary, conflictAvoidance)
	expression := compileExpression(internal, warmth, humor, affection, sensitivity, tolerance, directness, boundary, verbosity)

	return CompiledProfile{
		CompilerVersion: DefaultCompilerVersion,
		Resolved:        resolved,
		Internal:        internal,
		Appraisal:       appraisal,
		Recovery:        recovery,
		Behavior:        behavior,
		Expression:      expression,
		Sources:         sources,
		Diagnostics:     diagnostics,
		Migration:       migration,
	}
}

func MigratePersonalityConfigJSON(raw []byte) (PersonalityConfig, PersonalityMigration, error) {
	var cfg PersonalityConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return PersonalityConfig{}, PersonalityMigration{}, err
	}
	migrated, migration := MigratePersonalityConfig(cfg)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return PersonalityConfig{}, PersonalityMigration{}, err
	}
	for _, name := range knownPersonalityFields() {
		delete(fields, name)
	}
	if len(fields) > 0 {
		migration.UnknownFields = fields
	}
	return migrated, migration, nil
}

func compileAppraisal(internal InternalModel, warmth, affection, directness float64) AppraisalCoefficients {
	rejection := clampRange(0.2, 0.95, 0.55+(internal.StableCore.RejectionSensitivity-0.5)*0.8)
	relationship := clampRange(0.2, 0.95, 0.35+warmth*0.22+affection*0.24+internal.StableCore.SocialInitiative*0.1-internal.StableCore.BoundaryStrength*0.08)
	expectation := clampRange(0.2, 0.95, 0.32+directness*0.24+internal.StableCore.SocialInitiative*0.14+(1-internal.StableCore.UncertaintyTolerance)*0.18)
	uncertainty := clampRange(0.2, 0.95, 0.25+(1-internal.StableCore.UncertaintyTolerance)*0.52+internal.StableCore.RejectionSensitivity*0.12)
	boundary := clampRange(0.2, 0.95, 0.28+internal.StableCore.BoundaryStrength*0.48+(1-warmth)*0.08+directness*0.08)
	amplification := clampRange(1.15, 2.2, 1.12+rejection*0.3+relationship*0.17+expectation*0.15+uncertainty*0.14+boundary*0.12)
	parts := []string{
		"rejection",
		"relationship",
		"expectation",
		"uncertainty",
		"boundary",
	}
	return AppraisalCoefficients{
		Version:               DefaultCompilerVersion,
		Rejection:             round4(rejection),
		RelationshipRelevance: round4(relationship),
		ExpectationGap:        round4(expectation),
		Uncertainty:           round4(uncertainty),
		Boundary:              round4(boundary),
		AmplificationCap:      round4(amplification),
		Explanation:           strings.Join(parts, ","),
	}
}

func compileRecovery(internal InternalModel, sensitivity, tolerance, verbosity float64) RecoveryProfile {
	emotionHalfLife := clampRange(6, 24, 16-internal.StableCore.EmotionStability*7+sensitivity*5.5)
	moodHalfLife := clampRange(10, 36, 24-internal.StableCore.EmotionStability*8+(1-tolerance)*5)
	stressHalfLife := clampRange(8, 28, 18-internal.StableCore.EmotionStability*5.5+(1-internal.StableCore.UncertaintyTolerance)*6)
	needHalfLife := clampRange(4, 18, 11+verbosity*3-internal.StableCore.EmotionStability*4)
	minRecovery := clampRange(0.03, 0.2, 0.06+internal.StableCore.EmotionStability*0.05-internal.StableCore.RejectionSensitivity*0.02)
	maxRecovery := clampRange(0.2, 0.65, 0.28+internal.StableCore.EmotionStability*0.22+internal.StableCore.UncertaintyTolerance*0.08)
	if maxRecovery < minRecovery {
		maxRecovery = minRecovery
	}
	resilience := clampRange(0.15, 0.95, 0.26+internal.StableCore.EmotionStability*0.34+internal.StableCore.UncertaintyTolerance*0.18-internal.StableCore.RejectionSensitivity*0.12)
	return RecoveryProfile{
		Version:              DefaultCompilerVersion,
		EmotionHalfLifeHours: round4(emotionHalfLife),
		MoodHalfLifeHours:    round4(moodHalfLife),
		StressHalfLifeHours:  round4(stressHalfLife),
		NeedHalfLifeHours:    round4(needHalfLife),
		MinRecoveryRate:      round4(minRecovery),
		MaxRecoveryRate:      round4(maxRecovery),
		ResilienceBias:       round4(resilience),
	}
}

func compileBehavior(internal InternalModel, warmth, humor, affection, sensitivity, directness, boundary, conflictAvoidance float64) BehaviorProfile {
	initiate := clampRange(0.15, 0.95, 0.22+internal.StableCore.SocialInitiative*0.48+warmth*0.16)
	direct := clampRange(0.15, 0.95, 0.24+directness*0.5+boundary*0.1-conflictAvoidance*0.16)
	humorWeight := clampRange(0.05, 0.9, 0.12+humor*0.58+warmth*0.14-directness*0.04)
	conflictAvoid := clampRange(0.1, 0.95, 0.18+conflictAvoidance*0.56+warmth*0.09-directness*0.12)
	support := clampRange(0.2, 0.95, 0.24+warmth*0.33+affection*0.21+internal.StableCore.EmotionStability*0.11)
	initiationThreshold := clampRange(0.15, 0.85, 0.72-internal.StableCore.SocialInitiative*0.34-warmth*0.08+boundary*0.09+sensitivity*0.08)
	return BehaviorProfile{
		Version:             DefaultCompilerVersion,
		InitiateWeight:      round4(initiate),
		DirectWeight:        round4(direct),
		HumorWeight:         round4(humorWeight),
		ConflictAvoidWeight: round4(conflictAvoid),
		SupportWeight:       round4(support),
		InitiationThreshold: round4(initiationThreshold),
	}
}

func compileExpression(internal InternalModel, warmth, humor, affection, sensitivity, tolerance, directness, boundary, verbosity float64) ExpressionPolicy {
	targetChars := 44 + int(math.Round(verbosity*120+warmth*24))
	minChars := maxInt(18, int(math.Round(float64(targetChars)*0.6)))
	maxChars := minInt(240, int(math.Round(float64(targetChars)*1.45)))
	minSentences := 1
	maxSentences := minInt(6, maxInt(2, 2+int(math.Round(verbosity*3))))
	shortSentenceBias := clampRange(0.2, 0.9, 0.66+directness*0.18-verbosity*0.28)
	warmthValue := clampRange(0.2, 0.95, 0.24+warmth*0.34+affection*0.22)
	rationality := clampRange(0.2, 0.95, 0.26+tolerance*0.16+internal.StableCore.EmotionStability*0.27+directness*0.13-humor*0.04)
	teasing := clampRange(0.02, 0.85, 0.06+humor*0.56+affection*0.12-sensitivity*0.14)
	intimacy := clampRange(0.05, 0.9, 0.08+warmth*0.28+affection*0.38-boundary*0.16)
	suggestion := clampRange(0.1, 0.9, 0.16+directness*0.23+internal.Growth.SupportBias*0.24-boundary*0.1)
	disclosure := clampRange(0.08, 0.9, 0.12+warmth*0.22+affection*0.24+humor*0.08-boundary*0.08)

	styles := []string{}
	if boundary >= 0.7 {
		styles = append(styles, "overfamiliar_escalation")
	}
	if humor <= 0.25 {
		styles = append(styles, "constant_teasing")
	}
	if directness <= 0.35 {
		styles = append(styles, "abrupt_commands")
	}
	if sensitivity >= 0.75 {
		styles = append(styles, "harsh_confrontation")
	}
	sort.Strings(styles)

	return ExpressionPolicy{
		Version:             DefaultCompilerVersion,
		MinReplyChars:       minChars,
		MaxReplyChars:       maxChars,
		MinSentences:        minSentences,
		MaxSentences:        maxSentences,
		ShortSentenceBias:   round4(shortSentenceBias),
		Warmth:              round4(warmthValue),
		Rationality:         round4(rationality),
		Teasing:             round4(teasing),
		Intimacy:            round4(intimacy),
		SuggestionBias:      round4(suggestion),
		EmotionalDisclosure: round4(disclosure),
		ForbiddenStyles:     styles,
	}
}

func normalizeSchemaVersion(cfg PersonalityConfig) string {
	if strings.TrimSpace(cfg.PersonalitySchemaVersion) != "" {
		return strings.TrimSpace(cfg.PersonalitySchemaVersion)
	}
	if strings.TrimSpace(cfg.SchemaVersion) != "" {
		return strings.TrimSpace(cfg.SchemaVersion)
	}
	return ""
}

func displaySchemaVersion(version string) string {
	if strings.TrimSpace(version) == "" {
		return "missing"
	}
	return version
}

func schemaSource(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return "user"
	}
	if strings.TrimSpace(fallback) != "" {
		return "alias"
	}
	return "default"
}

func migrateScore(name string, value *float64, fallback float64, sources map[string]string) *float64 {
	if value != nil {
		sources[name] = "user"
		return value
	}
	sources[name] = "default:" + DefaultSchemaVersion
	resolved := fallback
	return &resolved
}

func resolveSchemaVersion(value, alias string, sources map[string]string) string {
	if strings.TrimSpace(alias) != "" {
		sources["schemaVersion"] = sourceOrExisting("schemaVersion", sources, "alias")
		return strings.TrimSpace(alias)
	}
	if strings.TrimSpace(value) == "" {
		sources["schemaVersion"] = sourceOrExisting("schemaVersion", sources, "default")
		return DefaultSchemaVersion
	}
	sources["schemaVersion"] = sourceOrExisting("schemaVersion", sources, "user")
	return strings.TrimSpace(value)
}

func resolveScore(name string, value *float64, fallback float64, sources map[string]string, diagnostics *[]string) float64 {
	if value == nil {
		sources[name] = sourceOrExisting(name, sources, "default")
		return fallback
	}
	raw := *value
	clamped := clampRange(0, 100, raw)
	if clamped != raw {
		sources[name] = "user_clamped"
		*diagnostics = append(*diagnostics, name+"_clamped")
		return clamped
	}
	sources[name] = sourceOrExisting(name, sources, "user")
	return raw
}

func sourceOrExisting(name string, sources map[string]string, fallback string) string {
	if existing := sources[name]; existing != "" {
		return existing
	}
	return fallback
}

func compileSourcesFromMigration(input map[string]string) map[string]string {
	output := map[string]string{}
	for key, value := range input {
		if strings.HasPrefix(value, "default:") {
			output[key] = "default"
			continue
		}
		if key == "personality_schema_version" {
			continue
		}
		output[key] = value
	}
	return output
}

func knownPersonalityFields() []string {
	return []string{
		"schemaVersion",
		"personality_schema_version",
		"initiative",
		"sensitivity",
		"tolerance",
		"stability",
		"boundary",
		"warmth",
		"directness",
		"humor",
		"affection",
		"verbosity",
		"conflictAvoidance",
	}
}

func normalize(score float64) float64 {
	return round4(clampRange(0, 1, score/100))
}

func clamp01(value float64) float64 {
	return clampRange(0, 1, value)
}

func clampRange(minimum, maximum, value float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
