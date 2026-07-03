package personality

import (
	"encoding/json"
	"time"
)

type CompiledPersonality struct {
	Version         string                 `json:"version"`
	CharacterID     string                 `json:"characterId"`
	SchemaVersion   string                 `json:"schemaVersion"`
	CognitiveSens   float64                `json:"cognitiveSens"`
	RecoverySpeed   float64                `json:"recoverySpeed"`
	BehaviorBias    map[string]float64     `json:"behaviorBias"`
	ExpressionStyle map[string]float64     `json:"expressionStyle"`
	ImmutableCore   map[string]string      `json:"immutableCore"`
	CompiledAt      time.Time              `json:"compiledAt"`
	SourceConfig    map[string]interface{} `json:"sourceConfig"`
	Diagnostics     []string               `json:"diagnostics"`
}

type CompilerConfig struct {
	DefaultSensitivity float64
	DefaultRecovery    float64
}

func DefaultCompilerConfig() CompilerConfig {
	return CompilerConfig{
		DefaultSensitivity: 0.5,
		DefaultRecovery:    0.5,
	}
}

type Compiler struct {
	config CompilerConfig
}

func NewCompiler(cfg CompilerConfig) *Compiler {
	return &Compiler{config: cfg}
}

func (c *Compiler) Compile(characterID string, rawConfig map[string]interface{}) CompiledPersonality {
	cp := CompiledPersonality{
		Version:       "personality-compiled-v1",
		CharacterID:   characterID,
		SchemaVersion: "v1",
		CompiledAt:    time.Now().UTC(),
		SourceConfig:  rawConfig,
		Diagnostics:   []string{},
	}

	cp.CognitiveSens = extractFloat(rawConfig, "cognitiveSensitivity", c.config.DefaultSensitivity)
	cp.RecoverySpeed = extractFloat(rawConfig, "recoverySpeed", c.config.DefaultRecovery)

	cp.BehaviorBias = map[string]float64{
		"warmth":            extractFloat(rawConfig, "warmth", 0.5),
		"directness":        extractFloat(rawConfig, "directness", 0.5),
		"humor":             extractFloat(rawConfig, "humor", 0.4),
		"affection":         extractFloat(rawConfig, "affection", 0.45),
		"conflictAvoidance": extractFloat(rawConfig, "conflictAvoidance", 0.5),
		"initiative":        extractFloat(rawConfig, "initiative", 0.5),
	}

	cp.ExpressionStyle = map[string]float64{
		"verbosity":    extractFloat(rawConfig, "verbosity", 0.5),
		"formality":    extractFloat(rawConfig, "formality", 0.5),
		"emotionality": extractFloat(rawConfig, "emotionality", 0.5),
	}

	cp.ImmutableCore = map[string]string{
		"identity":     extractString(rawConfig, "identity", ""),
		"coreBoundary": extractString(rawConfig, "coreBoundary", ""),
	}

	if raw, ok := rawConfig["personality"]; ok {
		if s, ok := raw.(string); ok && s != "" {
			cp.ImmutableCore["personalityDesc"] = s
		}
	}

	raw, _ := json.Marshal(rawConfig)
	if len(raw) == 0 || string(raw) == "{}" {
		cp.Diagnostics = append(cp.Diagnostics, "empty_config_defaults_applied")
	}

	return cp
}

func extractFloat(m map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return clamp01(val)
		case int:
			return clamp01(float64(val))
		case json.Number:
			f, _ := val.Float64()
			return clamp01(f)
		}
	}
	return defaultVal
}

func extractString(m map[string]interface{}, key, defaultVal string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
