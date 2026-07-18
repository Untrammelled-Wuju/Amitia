package temporal

import (
	"os"
	"strconv"
	"strings"
)

type FeatureFlags struct {
	TemporalCoreEnabled     bool `json:"temporalCoreEnabled"`
	RelationshipTimeEnabled bool `json:"relationshipTimeEnabled"`
}

func FeatureFlagsFromEnvironment() FeatureFlags {
	return FeatureFlags{
		TemporalCoreEnabled:     environmentBool("AMITIA_TEMPORAL_CORE_ENABLED", true),
		RelationshipTimeEnabled: environmentBool("AMITIA_RELATIONSHIP_TIME_ENABLED", false),
	}
}

func environmentBool(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
