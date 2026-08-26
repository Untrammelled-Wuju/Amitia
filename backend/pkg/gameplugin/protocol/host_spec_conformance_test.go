package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type hostSpecValidationCase struct {
	Name  string          `json:"name"`
	Valid bool            `json:"valid"`
	Spec  json.RawMessage `json:"spec"`
}

func TestPluginHostSpecSharedGoTypeScriptValidationFixture(t *testing.T) {
	fixturePath := filepath.Join("..", "testdata", "conformance", "host_spec_validation_cases.json")
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read shared host-spec fixture: %v", err)
	}
	var cases []hostSpecValidationCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("decode shared host-spec fixture: %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("shared host-spec fixture is empty")
	}

	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			var raw map[string]any
			if err := json.Unmarshal(testCase.Spec, &raw); err != nil {
				t.Fatalf("decode spec: %v", err)
			}
			spec, parseErr := ParsePluginHostSpec(raw)
			valid := parseErr == nil && spec.Validate() == nil
			if valid != testCase.Valid {
				if parseErr != nil {
					t.Fatalf("valid=%v want=%v; parse error: %v", valid, testCase.Valid, parseErr)
				}
				t.Fatalf("valid=%v want=%v; validation error: %v", valid, testCase.Valid, spec.Validate())
			}
		})
	}
}
