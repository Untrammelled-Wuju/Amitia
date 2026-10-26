package decision

import "testing"

func TestPromoteCandidatesMovesTargetToFront(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "chat_reply", FinalScore: 0.8},
		{ID: "offer_support", FinalScore: 0.3},
		{ID: "proactive_greet", FinalScore: 0.5},
	}
	promoted := PromoteCandidates(candidates, []string{"offer_support"})
	if promoted[1].FinalScore <= promoted[0].FinalScore {
		t.Fatalf("promotion did not raise score: %+v", promoted)
	}
	if promoted[0].FinalScore != 0.8 {
		t.Fatalf("unrelated candidate score changed: %+v", promoted[0])
	}
	last := promoted[1].Reasons[len(promoted[1].Reasons)-1]
	if last.Source != "safety_bdi" || last.Key != "override" {
		t.Fatalf("missing promotion reason: %+v", promoted[1].Reasons)
	}
	if len(PromoteCandidates(candidates, nil)) != 3 {
		t.Fatalf("empty ids must be a no-op")
	}
}

func TestIsKnownPreferenceDimension(t *testing.T) {
	if !IsKnownPreferenceDimension("chat_reply") || !IsKnownPreferenceDimension("reply") {
		t.Fatalf("known dimension rejected")
	}
	if IsKnownPreferenceDimension("unknown") || IsKnownPreferenceDimension("") {
		t.Fatalf("unknown dimension accepted")
	}
}
