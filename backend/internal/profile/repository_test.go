package profile

import "testing"

func TestUpsertConfidenceDoesNotIncreaseForSameEvidence(t *testing.T) {
	svc, _ := newProfileTestService(t)

	item, err := svc.repo.UpsertConfidence(&UserProfile{
		UserID:         "user-1",
		Category:       "preference",
		AttributeName:  "饮料",
		AttributeValue: "茶",
		Confidence:     60,
		Source:         "extract",
		SourceConvID:   "conv-a",
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if item.Confidence != 60 {
		t.Fatalf("initial confidence = %d, want 60", item.Confidence)
	}

	item, err = svc.repo.UpsertConfidence(&UserProfile{
		UserID:         "user-1",
		Category:       "preference",
		AttributeName:  "饮料",
		AttributeValue: "茶",
		Confidence:     90,
		Source:         "extract",
		SourceConvID:   "conv-a",
	})
	if err != nil {
		t.Fatalf("upsert same evidence: %v", err)
	}
	if item.Confidence != 60 {
		t.Fatalf("same evidence confidence = %d, want 60", item.Confidence)
	}
}

func TestUpsertConfidenceDoesNotIncreaseWithoutEvidence(t *testing.T) {
	svc, _ := newProfileTestService(t)

	_, err := svc.repo.UpsertConfidence(&UserProfile{
		UserID:         "user-1",
		Category:       "preference",
		AttributeName:  "颜色",
		AttributeValue: "蓝色",
		Confidence:     70,
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	item, err := svc.repo.UpsertConfidence(&UserProfile{
		UserID:         "user-1",
		Category:       "preference",
		AttributeName:  "颜色",
		AttributeValue: "蓝色",
		Confidence:     95,
	})
	if err != nil {
		t.Fatalf("upsert without evidence: %v", err)
	}
	if item.Confidence != 70 {
		t.Fatalf("no evidence confidence = %d, want 70", item.Confidence)
	}
}

func TestUpsertConfidenceIncreasesForNewConversationEvidence(t *testing.T) {
	svc, _ := newProfileTestService(t)

	_, err := svc.repo.UpsertConfidence(&UserProfile{
		UserID:         "user-1",
		Category:       "preference",
		AttributeName:  "音乐",
		AttributeValue: "爵士",
		Confidence:     60,
		SourceConvID:   "conv-a",
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	item, err := svc.repo.UpsertConfidence(&UserProfile{
		UserID:         "user-1",
		Category:       "preference",
		AttributeName:  "音乐",
		AttributeValue: "爵士",
		Confidence:     60,
		SourceConvID:   "conv-b",
	})
	if err != nil {
		t.Fatalf("upsert new conversation evidence: %v", err)
	}
	if item.Confidence <= 60 {
		t.Fatalf("new conversation evidence confidence = %d, want greater than 60", item.Confidence)
	}
}

func TestUpsertConfidenceIncreasesForDifferentSourceWhenNoConversationEvidence(t *testing.T) {
	svc, _ := newProfileTestService(t)

	_, err := svc.repo.UpsertConfidence(&UserProfile{
		UserID:         "user-1",
		Category:       "habit",
		AttributeName:  "运动",
		AttributeValue: "跑步",
		Confidence:     65,
		Source:         "tool",
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	item, err := svc.repo.UpsertConfidence(&UserProfile{
		UserID:         "user-1",
		Category:       "habit",
		AttributeName:  "运动",
		AttributeValue: "跑步",
		Confidence:     65,
		Source:         "manual",
	})
	if err != nil {
		t.Fatalf("upsert different source evidence: %v", err)
	}
	if item.Confidence <= 65 {
		t.Fatalf("different source evidence confidence = %d, want greater than 65", item.Confidence)
	}
}
