package chat

import (
	"encoding/json"
	"testing"
)

func TestMemoryAllowsProactiveMention(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{name: "absent", body: `{"key":"k","content":{"value":"v"}}`, want: true},
		{name: "top-level-off", body: `{"allowProactiveMention":false,"content":{"value":"v"}}`, want: false},
		{name: "top-level-on", body: `{"allowProactiveMention":true,"content":{"value":"v"}}`, want: true},
		{name: "content-off", body: `{"content":{"allowProactiveMention":false,"value":"v"}}`, want: false},
		{name: "invalid", body: `not-json`, want: true},
	}
	for _, testCase := range cases {
		if got := memoryAllowsProactiveMention(json.RawMessage(testCase.body)); got != testCase.want {
			t.Fatalf("%s: got %v want %v", testCase.name, got, testCase.want)
		}
	}
}
