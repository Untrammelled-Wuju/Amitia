package modelprotocol

import "testing"

func TestUsagePresenceDistinguishesUnknownAndZero(t *testing.T) {
	for _, data := range []string{`{}`, `{"usage":null}`, `{"usage":{}}`} {
		if UsageReported([]byte(data), "usage") {
			t.Fatalf("unknown usage counted as zero: %s", data)
		}
	}
	for _, data := range []string{`{"usage":{"total_tokens":0}}`, `{"usage":{"input_tokens":3}}`} {
		if !UsageReported([]byte(data), "usage") {
			t.Fatalf("reported usage lost: %s", data)
		}
	}
}
