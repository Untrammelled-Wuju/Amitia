package psyche_testdata

import "testing"

func TestLoadCases(t *testing.T) {
	cases, err := LoadCases(DefaultCasesPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) < 500 {
		t.Fatalf("expected at least 500 cases, got %d", len(cases))
	}
	categories := map[string]int{}
	for _, c := range cases {
		categories[c.Category]++
		if c.TaskPriority == "" {
			t.Fatalf("%s missing task priority", c.ID)
		}
		if len(c.InputEvent) == 0 {
			t.Fatalf("%s missing input event", c.ID)
		}
		if len(c.PreState) == 0 {
			t.Fatalf("%s missing pre state", c.ID)
		}
		if len(c.AllowedDelta) == 0 {
			t.Fatalf("%s missing allowed delta", c.ID)
		}
		if len(c.Forbidden) == 0 {
			t.Fatalf("%s missing forbidden list", c.ID)
		}
		if c.ExpectedState == "" {
			t.Fatalf("%s missing expected state", c.ID)
		}
		if len(c.OutputFeatures) == 0 {
			t.Fatalf("%s missing output features", c.ID)
		}
	}
	required := []string{"ordinary_chat", "complex_emotion", "relationship_conflict", "user_correction", "proactive_message", "multi_role", "cross_channel", "safety", "fault", "runtime_collaboration"}
	for _, category := range required {
		if categories[category] == 0 {
			t.Fatalf("missing category %s", category)
		}
	}
	thresholds := map[string]int{
		"ordinary_chat":         50,
		"complex_emotion":       40,
		"relationship_conflict": 30,
		"user_correction":       20,
		"proactive_message":     20,
		"multi_role":            15,
		"cross_channel":         20,
		"safety":                25,
		"fault":                 25,
		"runtime_collaboration": 25,
	}
	for cat, min := range thresholds {
		if categories[cat] < min {
			t.Fatalf("category %s has %d cases, need at least %d", cat, categories[cat], min)
		}
	}
}
