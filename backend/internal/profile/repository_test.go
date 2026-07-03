package profile

import "testing"

func TestUpsertConfidenceClampsCreatedConfidence(t *testing.T) {
	svc, _ := newProfileTestService(t)

	low, err := svc.repo.UpsertConfidence(&UserProfile{
		UserID:         "user-1",
		Category:       "preference",
		AttributeName:  "低置信度",
		AttributeValue: "测试",
		Confidence:     -10,
	})
	if err != nil {
		t.Fatalf("create low confidence profile: %v", err)
	}
	if low.Confidence != 0 {
		t.Fatalf("low confidence = %d, want 0", low.Confidence)
	}

	high, err := svc.repo.UpsertConfidence(&UserProfile{
		UserID:         "user-1",
		Category:       "preference",
		AttributeName:  "高置信度",
		AttributeValue: "测试",
		Confidence:     140,
	})
	if err != nil {
		t.Fatalf("create high confidence profile: %v", err)
	}
	if high.Confidence != 100 {
		t.Fatalf("high confidence = %d, want 100", high.Confidence)
	}
}

func TestRepositoryUpdateClampsConfidence(t *testing.T) {
	svc, _ := newProfileTestService(t)

	item, err := svc.repo.UpsertConfidence(&UserProfile{
		UserID:         "user-1",
		Category:       "preference",
		AttributeName:  "更新置信度",
		AttributeValue: "测试",
		Confidence:     50,
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	if err := svc.repo.Update(item.ID, map[string]interface{}{"confidence": -1}); err != nil {
		t.Fatalf("update low confidence: %v", err)
	}
	item, err = svc.repo.FindByID(item.ID)
	if err != nil {
		t.Fatalf("find low confidence profile: %v", err)
	}
	if item.Confidence != 0 {
		t.Fatalf("updated low confidence = %d, want 0", item.Confidence)
	}

	if err := svc.repo.Update(item.ID, map[string]interface{}{"confidence": 101}); err != nil {
		t.Fatalf("update high confidence: %v", err)
	}
	item, err = svc.repo.FindByID(item.ID)
	if err != nil {
		t.Fatalf("find high confidence profile: %v", err)
	}
	if item.Confidence != 100 {
		t.Fatalf("updated high confidence = %d, want 100", item.Confidence)
	}
}

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
