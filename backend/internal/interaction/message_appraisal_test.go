package interaction

import "testing"

func TestEvaluateMessageAppraisalClassifiesRealtimeTurn(t *testing.T) {
	tests := []struct {
		message string
		want    string
	}{
		{"谢谢你，今天真的帮到我了", "praise"},
		{"对不起，是我错了", "apology"},
		{"我有点失望，也有点生气", "complaint"},
	}
	for _, test := range tests {
		result := EvaluateMessageAppraisal(test.message, 0.5)
		if result == nil || result.EventType != test.want {
			t.Fatalf("EvaluateMessageAppraisal(%q) = %+v, want event %q", test.message, result, test.want)
		}
	}
}

func TestEvaluateMessageAppraisalProducesNeedAndRelationshipDeltas(t *testing.T) {
	result := EvaluateMessageAppraisal("对不起，是我错了", 0.6)
	if result.RelationshipHurtDelta >= 0 || result.RelationshipAngerDelta >= 0 {
		t.Fatalf("apology should reduce hurt and anger: %+v", result)
	}
	if result.NeedDeltas == nil || len(result.NeedDeltas) == 0 {
		t.Fatalf("expected need deltas: %+v", result)
	}
}
