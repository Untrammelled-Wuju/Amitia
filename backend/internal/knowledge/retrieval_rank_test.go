package knowledge

import "testing"

func testState(id string, fusion float64) *fusionState {
	return &fusionState{chunkID: id, fusion: fusion, denseScore: fusion, sparseScore: fusion / 2}
}

func TestPrepareEvidenceCandidatesCollapsesSiblingHitsToParent(t *testing.T) {
	parent := Chunk{ID: "p1", SourceID: "s1", SourceRevision: 1, UnitID: "u1", ChunkIndex: -1, Content: "完整父块证据"}
	children := map[string]Chunk{
		"c1": {ID: "c1", SourceID: "s1", SourceRevision: 1, UnitID: "u1", ChunkIndex: 0, ParentChunkID: "p1", Content: "child one"},
		"c2": {ID: "c2", SourceID: "s1", SourceRevision: 1, UnitID: "u1", ChunkIndex: 1, ParentChunkID: "p1", Content: "child two"},
	}
	fused := []*fusionState{
		{chunkID: "c1", fusion: 0.4, denseScore: 0.9, sparseScore: 0.1},
		{chunkID: "c2", fusion: 0.6, denseScore: 0.2, sparseScore: 0.8, exactScore: 1},
	}
	got := prepareEvidenceCandidates(fused, children, 0, 10000, func(id string) (*Chunk, error) { return &parent, nil })
	if len(got) != 1 {
		t.Fatalf("expected one parent-expanded candidate, got %d", len(got))
	}
	if got[0].readID != parent.ID || got[0].matched.ID != "c2" {
		t.Fatalf("unexpected parent aggregation: %#v", got[0])
	}
	if got[0].state.fusion != 0.6 || got[0].state.denseScore != 0.9 || got[0].state.sparseScore != 0.8 || got[0].state.exactScore != 1 {
		t.Fatalf("sibling modality signals were not merged: %#v", got[0].state)
	}
}

func TestDedupeCrossSourceExactDuplicateKeepsProvenance(t *testing.T) {
	mk := func(source, chunk string, score float64) evidenceCandidate {
		c := Chunk{ID: chunk, SourceID: source, SourceRevision: 1, UnitID: "u-" + chunk, Content: "同一条事实：Amitia 的知识库与角色无关。"}
		return evidenceCandidate{
			state: testState(chunk, score), matched: c, read: c, readID: chunk,
			fingerprint: contentFingerprint(c.Content), features: similarityFeatures(c.Content), relevance: score,
		}
	}
	got := dedupeEvidenceCandidates([]evidenceCandidate{mk("source-a", "a", 1), mk("source-b", "b", 0.8)})
	if len(got) != 1 {
		t.Fatalf("exact duplicate should collapse to one context item, got %d", len(got))
	}
	if len(got[0].duplicates) != 1 || got[0].duplicates[0].SourceID != "source-b" {
		t.Fatalf("cross-source provenance was lost: %#v", got[0].duplicates)
	}
}

func TestDedupeDoesNotDropDistinctSameTopicEvidence(t *testing.T) {
	aText := "知识库采用全局共享 RAG，所有角色可以查询同一份知识文档，但数据仍受当前租户边界保护。"
	bText := "知识库采用全局共享 RAG。世界书则只按角色隔离，并通过关键词或正则规则触发，两者不能混成同一数据域。"
	mk := func(source, chunk, text string, score float64) evidenceCandidate {
		c := Chunk{ID: chunk, SourceID: source, SourceRevision: 1, UnitID: "u-" + chunk, Content: text}
		return evidenceCandidate{state: testState(chunk, score), matched: c, read: c, readID: chunk, fingerprint: contentFingerprint(text), features: similarityFeatures(text), relevance: score}
	}
	got := dedupeEvidenceCandidates([]evidenceCandidate{mk("a", "a", aText, 1), mk("b", "b", bText, 0.9)})
	if len(got) != 2 {
		t.Fatalf("distinct corroborating/contrasting evidence must survive dedupe, got %d", len(got))
	}
}

func TestSelectMMRUsesSoftSourceDiversityBudget(t *testing.T) {
	mk := func(source, id, text string, relevance float64) evidenceCandidate {
		c := Chunk{ID: id, SourceID: source, SourceRevision: 1, UnitID: "u-" + id, Content: text}
		return evidenceCandidate{state: testState(id, relevance), matched: c, read: c, readID: id, relevance: relevance, features: similarityFeatures(text)}
	}
	candidates := []evidenceCandidate{
		mk("source-a", "a1", "数据库事务与 canonical 状态切换", 1.00),
		mk("source-a", "a2", "向量索引 outbox 与 lease", 0.98),
		mk("source-a", "a3", "解析任务 checkpoint 与 heartbeat", 0.96),
		mk("source-b", "b1", "另一份文档提供租户权限隔离证据", 0.80),
	}
	got := selectMMR(candidates, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %d", len(got))
	}
	seenB := false
	countA := 0
	for _, c := range got {
		if c.matched.SourceID == "source-b" {
			seenB = true
		}
		if c.matched.SourceID == "source-a" {
			countA++
		}
	}
	if !seenB || countA > 2 {
		t.Fatalf("diversity budget did not prevent source monopoly: %#v", got)
	}
}

func TestSelectMMRSoftCapDoesNotDiscardMuchStrongerEvidence(t *testing.T) {
	mk := func(source, id, text string, relevance float64) evidenceCandidate {
		c := Chunk{ID: id, SourceID: source, SourceRevision: 1, UnitID: "u-" + id, Content: text}
		return evidenceCandidate{state: testState(id, relevance), matched: c, read: c, readID: id, relevance: relevance, features: similarityFeatures(text)}
	}
	got := selectMMR([]evidenceCandidate{
		mk("source-a", "a1", "高质量证据甲 完整事务原子切换", 1.00),
		mk("source-a", "a2", "高质量证据乙 outbox 最终一致", 0.99),
		mk("source-a", "a3", "高质量证据丙 lease 防重复消费", 0.98),
		mk("source-b", "b1", "弱相关信息 天气与咖啡", 0.20),
	}, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %d", len(got))
	}
	for _, c := range got {
		if c.matched.SourceID == "source-b" {
			t.Fatalf("soft diversity cap replaced materially stronger evidence with weak evidence: %#v", got)
		}
	}
}

func TestContentFingerprintIgnoresFormattingNoise(t *testing.T) {
	a := contentFingerprint("Hello， World!\nAmitia")
	b := contentFingerprint("hello world amitia")
	if a == "" || a != b {
		t.Fatalf("format-normalised fingerprints differ: %q vs %q", a, b)
	}
}

func TestPrepareEvidenceCandidatesRespectsParentExpansionBudget(t *testing.T) {
	fused := []*fusionState{
		{chunkID: "c1", fusion: 1.0},
		{chunkID: "c2", fusion: 0.9},
	}
	children := map[string]Chunk{
		"c1": {ID: "c1", SourceID: "s1", SourceRevision: 1, ParentChunkID: "p1", Content: "child one", TokenCount: 10},
		"c2": {ID: "c2", SourceID: "s2", SourceRevision: 1, ParentChunkID: "p2", Content: "child two", TokenCount: 10},
	}
	parents := map[string]Chunk{
		"p1": {ID: "p1", SourceID: "s1", SourceRevision: 1, Content: "parent one", TokenCount: 80},
		"p2": {ID: "p2", SourceID: "s2", SourceRevision: 1, Content: "parent two", TokenCount: 80},
	}
	got := prepareEvidenceCandidates(fused, children, 0, 100, func(id string) (*Chunk, error) {
		p := parents[id]
		return &p, nil
	})
	if len(got) != 2 {
		t.Fatalf("expected two candidates, got %d", len(got))
	}
	if got[0].readID != "p1" {
		t.Fatalf("highest ranked candidate should receive parent expansion, got %q", got[0].readID)
	}
	if got[1].readID != "c2" {
		t.Fatalf("second candidate should fall back to child after budget exhaustion, got %q", got[1].readID)
	}
}
