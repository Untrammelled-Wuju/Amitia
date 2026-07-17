package extension

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCloneDefinitionNormalizesRequiredSlices(t *testing.T) {
	cloned := cloneDefinition(SkillDefinition{})
	if cloned.Capabilities == nil || cloned.Triggers == nil {
		t.Fatalf("required slices must not be nil: %#v", cloned)
	}
	payload, err := json.Marshal(cloned)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if !strings.Contains(text, `"capabilities":[]`) || !strings.Contains(text, `"triggers":[]`) {
		t.Fatalf("required slices must serialize as arrays: %s", text)
	}
}
