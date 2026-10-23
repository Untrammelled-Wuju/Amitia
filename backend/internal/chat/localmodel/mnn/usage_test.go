package mnn

import "testing"

func TestUsagePreservesLocalUnknownAndReportedZero(t *testing.T) {
	for _, test := range []struct {
		payload  string
		reported bool
	}{
		{`{"choices":[{"message":{"content":"ok"}}]}`, false},
		{`{"choices":[{"message":{"content":"ok"}}],"usage":{}}`, false},
		{`{"choices":[{"message":{"content":"ok"}}],"usage":{"total_tokens":0}}`, true},
	} {
		result, err := parseMNNChatResponse([]byte(test.payload))
		if err != nil || result.Usage.Reported != test.reported {
			t.Fatalf("local usage presence: %+v %v", result, err)
		}
	}
}
