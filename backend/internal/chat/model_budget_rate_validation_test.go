package chat

import "testing"

func TestRound79ConfiguredModelRateValidation(t *testing.T) {
	for _, rates := range []map[string]interface{}{
		{"budgetInputRate": float64(1), "budgetOutputRate": float64(100000000), "budgetRateVersion": "v1"},
		{"budgetInputRate": float64(0), "budgetOutputRate": float64(0)},
	} {
		if err := validateModelBudgetRateConfig(rates); err != nil {
			t.Fatal(err)
		}
	}
	for _, rates := range []map[string]interface{}{
		{"budgetInputRate": -1.0}, {"budgetOutputRate": float64(100000001)},
		{"budgetInputRate": float64(1.5)}, {"budgetOutputRate": "cheap"},
		{"budgetRateVersion": " an invalid value "}, {"budgetRateVersion": 123},
	} {
		if err := validateModelBudgetRateConfig(rates); err == nil {
			t.Fatalf("invalid rate admitted: %+v", rates)
		}
	}
}
