package chat

import (
	"errors"
	"math"
	"strings"
)

func validateModelBudgetRateConfig(raw map[string]interface{}) error {
	for _, key := range []string{"budgetInputRate", "budgetOutputRate"} {
		value, exists := raw[key]
		if !exists {
			continue
		}
		n, ok := value.(float64)
		if !ok || math.IsNaN(n) || n < 0 || n > 100000000 || math.Trunc(n) != n {
			return errors.New("model budget rate must be an integer from 0 to 100000000 microUSD per million tokens")
		}
	}
	if value, exists := raw["budgetRateVersion"]; exists {
		name, ok := value.(string)
		if !ok || len(name) > 128 || strings.TrimSpace(name) != name {
			return errors.New("model budget rate version must be a trimmed string of at most 128 bytes")
		}
	}
	return nil
}
