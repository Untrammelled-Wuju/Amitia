package permission

import (
	"encoding/json"
	"testing"
)

func TestInputConditionsFailClosedAndUseActualValues(t *testing.T) {
	for _, test := range []struct {
		name, conditions, input string
		allowed                 bool
	}{
		{"matching", `[{"field":"path","operator":"eq","value":"allowed"}]`, `{"path":"allowed"}`, true},
		{"escaped", `[{"field":"path","operator":"eq","value":"allowed"}]`, `{"path":"private"}`, false},
		{"missing", `[{"field":"path","operator":"ne","value":"private"}]`, `{}`, false},
		{"exists", `[{"field":"nested.value","operator":"exists","value":true}]`, `{"nested":{"value":null}}`, true},
		{"numeric", `[{"field":"count","operator":"eq","value":1200}]`, `{"count":1.20e+3}`, true},
		{"in", `[{"field":"text","operator":"in","value":["<>&中文","other"]}]`, `{"text":"<>&中文"}`, true},
		{"unknown operator", `[{"field":"path","operator":"wildcard","value":"*"}]`, `{"path":"private"}`, false},
		{"unknown format", `{"paths":["allowed"]}`, `{"path":"allowed"}`, false},
		{"null", `null`, `{}`, false},
		{"unknown field", `[{"field":"path","operator":"eq","value":"allowed","ignore":true}]`, `{"path":"allowed"}`, false},
		{"trailing", `[] {}`, `{}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateInputConditions(json.RawMessage(test.conditions), json.RawMessage(test.input))
			if (err == nil) != test.allowed {
				t.Fatalf("allowed=%v error=%v", test.allowed, err)
			}
		})
	}
}
