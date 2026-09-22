package continuity

import (
	"context"
	"testing"
)

type fakeSemanticMatcher struct{ scores map[string]float64 }

func (m fakeSemanticMatcher) Scores(context.Context, string, []Thread) (map[string]float64, error) {
	return m.scores, nil
}

type fakeDisambiguator struct {
	id         string
	confidence float64
}

func (d fakeDisambiguator) ChooseThread(context.Context, string, []Thread) (string, float64, error) {
	return d.id, d.confidence, nil
}

func TestResolverUsesSemanticEvidenceAcrossConversations(t *testing.T) {
	repo := testRepository(t)
	server := &Thread{SpaceID: "space-1", Title: "服务器迁移", Goal: "迁移生产服务器", Status: ThreadStatusActive}
	study := &Thread{SpaceID: "space-1", Title: "专升本", Goal: "学习计划", Status: ThreadStatusActive}
	if err := repo.CreateThread(server); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateThread(study); err != nil {
		t.Fatal(err)
	}
	resolver := NewResolver(repo)
	resolver.SetSemanticMatcher(fakeSemanticMatcher{scores: map[string]float64{server.ID: 0.98, study.ID: 0.11}})
	resolution, err := resolver.Resolve(context.Background(), ResolveInput{SpaceID: "space-1", ConversationID: "new-conv", Message: "DNS 已经生效了"})
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Thread == nil || resolution.Thread.ID != server.ID || resolution.Method != "semantic" {
		t.Fatalf("unexpected resolution: %#v", resolution)
	}
}

func TestResolverUsesLLMOnlyForAmbiguousSemanticCandidates(t *testing.T) {
	repo := testRepository(t)
	a := &Thread{SpaceID: "space-1", Title: "Amitia 后端部署", Status: ThreadStatusActive}
	b := &Thread{SpaceID: "space-1", Title: "Amitia 桌面端部署", Status: ThreadStatusActive}
	if err := repo.CreateThread(a); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateThread(b); err != nil {
		t.Fatal(err)
	}
	resolver := NewResolver(repo)
	resolver.SetSemanticMatcher(fakeSemanticMatcher{scores: map[string]float64{a.ID: 0.72, b.ID: 0.70}})
	resolver.SetDisambiguator(fakeDisambiguator{id: b.ID, confidence: 0.91})
	resolution, err := resolver.Resolve(context.Background(), ResolveInput{SpaceID: "space-1", ConversationID: "new-conv", Message: "继续那个部署"})
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Thread == nil || resolution.Thread.ID != b.ID || resolution.Method != "llm_disambiguation" {
		t.Fatalf("unexpected resolution: %#v", resolution)
	}
}
