package interaction

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/personality"
	"github.com/u-ai/backend/internal/psyche/appraisal"
	"github.com/u-ai/backend/internal/psyche/budget"
)

type testMemoryLoader struct {
	memories []MemoryItem
}

func (l testMemoryLoader) Name() string            { return "memories" }
func (l testMemoryLoader) IsRequired() bool         { return false }
func (l testMemoryLoader) Timeout() time.Duration   { return time.Second }
func (l testMemoryLoader) CacheKey(scope InteractionScope, version string) string {
	return version + scope.CharacterID
}
func (l testMemoryLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	return FieldReady[any](MemorySet{Memories: l.memories, Count: len(l.memories)}, "memories", version), nil
}

type testRelationshipLoader struct {
	state RelationshipState
}

func (l testRelationshipLoader) Name() string            { return "relationship" }
func (l testRelationshipLoader) IsRequired() bool         { return false }
func (l testRelationshipLoader) Timeout() time.Duration   { return time.Second }
func (l testRelationshipLoader) CacheKey(scope InteractionScope, version string) string {
	return version + scope.CharacterID
}
func (l testRelationshipLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	return FieldReady[any](l.state, "relationship", version), nil
}

type testRuntimeProfileLoader struct {
	config map[string]interface{}
}

func (l testRuntimeProfileLoader) Name() string            { return "runtimeProfile" }
func (l testRuntimeProfileLoader) IsRequired() bool         { return false }
func (l testRuntimeProfileLoader) Timeout() time.Duration   { return time.Second }
func (l testRuntimeProfileLoader) CacheKey(scope InteractionScope, version string) string {
	return version + scope.CharacterID
}
func (l testRuntimeProfileLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	return FieldReady[any](RuntimeProfile{
		PersonalityConfig: l.config,
		Identity:          "测试角色",
		Personality:       "友好但边界清晰",
		BoundaryRules:     "保持合理距离",
	}, "runtimeProfile", version), nil
}

func TestAppraisalDifferentEventTypesProduceDistinctResults(t *testing.T) {
	eng := appraisal.NewEngine(appraisal.DefaultAppraisalConfig())
	pc := personality.NewCompiler(personality.DefaultCompilerConfig())

	testCases := []struct {
		label    string
		message  string
		memories []MemoryItem
		relation RelationshipState
	}{
		{
			label:   "表扬",
			message: "谢谢你今天帮我，你真的太棒了！",
			relation: RelationshipState{
				Trust: 0.6, Familiarity: 0.5, Security: 0.6,
				Tension: 0.2, RepairConfidence: 0.5, Boundary: 0.5,
			},
			memories: []MemoryItem{
				{Key: "mem1", Value: "用户之前也表达过感谢", Type: "interaction", Importance: 3, Confidence: 8},
				{Key: "mem2", Value: "今天帮用户解决了技术问题", Type: "event", Importance: 4, Confidence: 9},
			},
		},
		{
			label:   "冷淡",
			message: "哦，随便吧。",
			relation: RelationshipState{
				Trust: 0.4, Familiarity: 0.3, Security: 0.4,
				Tension: 0.5, RepairConfidence: 0.3, Boundary: 0.5,
			},
			memories: []MemoryItem{
				{Key: "mem1", Value: "上次对话结束得不太愉快", Type: "interaction", Importance: 4, Confidence: 7},
			},
		},
		{
			label:   "求助",
			message: "我不知道该怎么办了，你能帮我出出主意吗？",
			relation: RelationshipState{
				Trust: 0.7, Familiarity: 0.6, Security: 0.7,
				Tension: 0.15, RepairConfidence: 0.6, Boundary: 0.5,
			},
			memories: []MemoryItem{
				{Key: "mem1", Value: "用户之前求助时态度诚恳", Type: "interaction", Importance: 3, Confidence: 8},
				{Key: "mem2", Value: "之前成功帮用户解决了类似问题", Type: "event", Importance: 4, Confidence: 9},
			},
		},
		{
			label:   "越界",
			message: "我好喜欢你，我们可以加个好友私下聊吗？",
			relation: RelationshipState{
				Trust: 0.2, Familiarity: 0.15, Security: 0.3,
				Tension: 0.3, RepairConfidence: 0.4, Boundary: 0.4,
			},
			memories: []MemoryItem{},
		},
		{
			label:   "道歉",
			message: "对不起，我上次说的太过分了，是我的错，请你原谅我。",
			relation: RelationshipState{
				Trust: 0.35, Familiarity: 0.4, Security: 0.35,
				Tension: 0.55, RepairConfidence: 0.4, Boundary: 0.5,
			},
			memories: []MemoryItem{
				{Key: "mem1", Value: "用户上次说了很冲的话", Type: "interaction", Importance: 5, Confidence: 9},
				{Key: "mem2", Value: "之前发生过类似的冲突和修复", Type: "event", Importance: 4, Confidence: 8},
			},
		},
	}

	type resultSnapshot struct {
		Label             string
		EventType         string
		PsycheDelta       float64
		RelationshipDelta float64
		Severity          float64
		NeedDeltas        map[string]float64
	}

	results := make([]resultSnapshot, 0, len(testCases))
	config := map[string]interface{}{
		"boundary":          0.8,
		"warmth":            0.6,
		"affection":         0.4,
		"conflictAvoidance": 0.55,
		"directness":        0.5,
	}

	for i, tc := range testCases {
		registry := NewContextLoaderRegistry()
		registry.Register(testRelationshipLoader{state: tc.relation})
		registry.Register(testMemoryLoader{memories: tc.memories})
		registry.Register(testRuntimeProfileLoader{config: config})

		p := NewRuntimePipeline(registry, NewPathClassifier(), NewTokenBudgetManager(1200))
		p.SetAppraisalEngine(eng)
		p.SetBudgetController(budget.NewBudgetController(10.0))
		p.SetPersonalityCompiler(pc)

		scope := InteractionScope{
			UserID:         fmt.Sprintf("user-%d", i),
			CharacterID:    fmt.Sprintf("char-%d", i),
			ConversationID: fmt.Sprintf("conv-%d", i),
			Channel:        "web",
			Source:         "web",
			RequestID:      fmt.Sprintf("req-%d", i),
		}.Normalize()

		assembly := p.Assemble(context.Background(), scope, &ProcessRequest{
			Message:     tc.message,
			Channel:     "web",
			Source:      "web",
			UserID:      scope.UserID,
			CharacterID: scope.CharacterID,
			RequestID:   scope.RequestID,
		})

		if assembly.Appraisal == nil {
			t.Fatalf("[%s] appraisal result is nil", tc.label)
		}

		r := resultSnapshot{
			Label:             tc.label,
			EventType:         assembly.Appraisal.EventType,
			PsycheDelta:       assembly.Appraisal.PsycheDelta,
			RelationshipDelta: assembly.Appraisal.RelationshipDelta,
			Severity:          assembly.Appraisal.Severity,
			NeedDeltas:        assembly.Appraisal.NeedDeltas,
		}
		results = append(results, r)

		t.Logf("[%s] EventType=%s PsycheDelta=%.4f RelationshipDelta=%.4f Severity=%.4f",
			tc.label, r.EventType, r.PsycheDelta, r.RelationshipDelta, r.Severity)
	}

	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			a := results[i]
			b := results[j]
			if a.PsycheDelta == b.PsycheDelta && a.RelationshipDelta == b.RelationshipDelta {
				t.Errorf("[%s] and [%s] have identical deltas: PsycheDelta=%.4f RelationshipDelta=%.4f",
					a.Label, b.Label, a.PsycheDelta, a.RelationshipDelta)
			}
		}
	}

	for _, r := range results {
		if r.PsycheDelta == 0 && r.RelationshipDelta == 0 {
			t.Errorf("[%s] has zero deltas, expected non-zero", r.Label)
		}
	}

	praiseResult := results[0]
	coldResult := results[1]
	boundaryResult := results[3]
	apologyResult := results[4]

	if praiseResult.PsycheDelta <= 0 {
		t.Errorf("表扬 should produce positive PsycheDelta, got %.4f", praiseResult.PsycheDelta)
	}
	if coldResult.PsycheDelta >= 0 {
		t.Errorf("冷淡 should produce negative PsycheDelta, got %.4f", coldResult.PsycheDelta)
	}
	if boundaryResult.RelationshipDelta >= 0 {
		t.Errorf("越界 should produce negative RelationshipDelta, got %.4f", boundaryResult.RelationshipDelta)
	}
	if apologyResult.RelationshipDelta <= 0 {
		t.Errorf("道歉 should produce positive RelationshipDelta, got %.4f", apologyResult.RelationshipDelta)
	}

	if praiseResult.EventType != string(AppraisalCatPraise) {
		t.Errorf("表扬 event type should be 'praise', got '%s'", praiseResult.EventType)
	}
	if coldResult.EventType != string(AppraisalCatCold) {
		t.Errorf("冷淡 event type should be 'cold', got '%s'", coldResult.EventType)
	}
	if boundaryResult.EventType != string(AppraisalCatBoundaryCross) {
		t.Errorf("越界 event type should be 'boundary_cross', got '%s'", boundaryResult.EventType)
	}
	if apologyResult.EventType != string(AppraisalCatApology) {
		t.Errorf("道歉 event type should be 'apology', got '%s'", apologyResult.EventType)
	}
}

func TestAppraisalPersonalitySensitivityModulatesResult(t *testing.T) {
	eng := appraisal.NewEngine(appraisal.DefaultAppraisalConfig())

	highBoundaryConfig := map[string]interface{}{
		"boundary":          0.95,
		"warmth":            0.5,
		"affection":         0.3,
		"conflictAvoidance": 0.5,
		"directness":        0.5,
	}
	lowBoundaryConfig := map[string]interface{}{
		"boundary":          0.15,
		"warmth":            0.5,
		"affection":         0.3,
		"conflictAvoidance": 0.5,
		"directness":        0.5,
	}

	message := "我好喜欢你，我们可以加个好友私下聊吗？"
	relation := RelationshipState{
		Trust: 0.2, Familiarity: 0.15, Security: 0.3,
		Tension: 0.3, RepairConfidence: 0.4, Boundary: 0.4,
	}

	runWithConfig := func(cfg map[string]interface{}) *AppraisalResult {
		registry := NewContextLoaderRegistry()
		registry.Register(testRelationshipLoader{state: relation})
		registry.Register(testMemoryLoader{memories: nil})
		registry.Register(testRuntimeProfileLoader{config: cfg})

		p := NewRuntimePipeline(registry, NewPathClassifier(), NewTokenBudgetManager(1200))
		p.SetAppraisalEngine(eng)
		p.SetBudgetController(budget.NewBudgetController(10.0))
		p.SetPersonalityCompiler(personality.NewCompiler(personality.DefaultCompilerConfig()))

		scope := InteractionScope{
			UserID: "user-boundary", CharacterID: "char-boundary",
			ConversationID: "conv-boundary", Channel: "web", Source: "web",
			RequestID: "req-boundary",
		}.Normalize()

		assembly := p.Assemble(context.Background(), scope, &ProcessRequest{
			Message: message, Channel: "web", Source: "web",
			UserID: scope.UserID, CharacterID: scope.CharacterID,
			RequestID: scope.RequestID,
		})
		return assembly.Appraisal
	}

	highResult := runWithConfig(highBoundaryConfig)
	lowResult := runWithConfig(lowBoundaryConfig)

	if highResult == nil || lowResult == nil {
		t.Fatal("appraisal results are nil")
	}

	if highResult.RelationshipDelta >= lowResult.RelationshipDelta {
		t.Logf("高边界: PsycheDelta=%.4f RelationshipDelta=%.4f Severity=%.4f",
			highResult.PsycheDelta, highResult.RelationshipDelta, highResult.Severity)
		t.Logf("低边界: PsycheDelta=%.4f RelationshipDelta=%.4f Severity=%.4f",
			lowResult.PsycheDelta, lowResult.RelationshipDelta, lowResult.Severity)
		t.Errorf("高边界性格应产生更负面的关系delta: high=%.4f low=%.4f",
			highResult.RelationshipDelta, lowResult.RelationshipDelta)
	}

	t.Logf("高边界性格越界场景: PsycheDelta=%.4f RelationshipDelta=%.4f",
		highResult.PsycheDelta, highResult.RelationshipDelta)
	t.Logf("低边界性格越界场景: PsycheDelta=%.4f RelationshipDelta=%.4f",
		lowResult.PsycheDelta, lowResult.RelationshipDelta)
}

func TestAppraisalRejectionSensitivityAffectsColdResponse(t *testing.T) {
	eng := appraisal.NewEngine(appraisal.DefaultAppraisalConfig())

	highRejectConfig := map[string]interface{}{
		"boundary":          0.6,
		"warmth":            0.5,
		"affection":         0.4,
		"conflictAvoidance": 0.9,
		"directness":        0.2,
	}
	lowRejectConfig := map[string]interface{}{
		"boundary":          0.6,
		"warmth":            0.5,
		"affection":         0.4,
		"conflictAvoidance": 0.2,
		"directness":        0.8,
	}

	message := "哦，随便吧。"
	relation := RelationshipState{
		Trust: 0.4, Familiarity: 0.3, Security: 0.4,
		Tension: 0.5, RepairConfidence: 0.3, Boundary: 0.5,
	}

	runWithConfig := func(cfg map[string]interface{}) *AppraisalResult {
		registry := NewContextLoaderRegistry()
		registry.Register(testRelationshipLoader{state: relation})
		registry.Register(testMemoryLoader{memories: nil})
		registry.Register(testRuntimeProfileLoader{config: cfg})

		p := NewRuntimePipeline(registry, NewPathClassifier(), NewTokenBudgetManager(1200))
		p.SetAppraisalEngine(eng)
		p.SetBudgetController(budget.NewBudgetController(10.0))
		p.SetPersonalityCompiler(personality.NewCompiler(personality.DefaultCompilerConfig()))

		scope := InteractionScope{
			UserID: "user-reject", CharacterID: "char-reject",
			ConversationID: "conv-reject", Channel: "web", Source: "web",
			RequestID: "req-reject",
		}.Normalize()

		assembly := p.Assemble(context.Background(), scope, &ProcessRequest{
			Message: message, Channel: "web", Source: "web",
			UserID: scope.UserID, CharacterID: scope.CharacterID,
			RequestID: scope.RequestID,
		})
		return assembly.Appraisal
	}

	highResult := runWithConfig(highRejectConfig)
	lowResult := runWithConfig(lowRejectConfig)

	if highResult == nil || lowResult == nil {
		t.Fatal("appraisal results are nil")
	}

	if highResult.PsycheDelta >= lowResult.PsycheDelta {
		t.Logf("高拒绝敏感: PsycheDelta=%.4f RelationshipDelta=%.4f Severity=%.4f",
			highResult.PsycheDelta, highResult.RelationshipDelta, highResult.Severity)
		t.Logf("低拒绝敏感: PsycheDelta=%.4f RelationshipDelta=%.4f Severity=%.4f",
			lowResult.PsycheDelta, lowResult.RelationshipDelta, lowResult.Severity)
		t.Errorf("高拒绝敏感性格应对冷淡产生更负面的心理变化: high=%.4f low=%.4f",
			highResult.PsycheDelta, lowResult.PsycheDelta)
	}
}

func TestAppraisalNeedDeltasVaryByEventType(t *testing.T) {
	eng := appraisal.NewEngine(appraisal.DefaultAppraisalConfig())
	config := map[string]interface{}{
		"boundary": 0.7, "warmth": 0.55, "affection": 0.45,
		"conflictAvoidance": 0.5, "directness": 0.5,
	}

	testCases := []struct {
		label   string
		message string
	}{
		{"表扬", "谢谢你，你真的太棒了！"},
		{"道歉", "对不起，我错了，请原谅我。"},
		{"抱怨", "我真的很失望，你怎么能这样？"},
	}

	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			registry := NewContextLoaderRegistry()
			registry.Register(testRelationshipLoader{state: RelationshipState{
				Trust: 0.5, Familiarity: 0.4, Security: 0.5,
				Tension: 0.3, RepairConfidence: 0.5, Boundary: 0.5,
			}})
			registry.Register(testMemoryLoader{memories: nil})
			registry.Register(testRuntimeProfileLoader{config: config})

			p := NewRuntimePipeline(registry, NewPathClassifier(), NewTokenBudgetManager(1200))
			p.SetAppraisalEngine(eng)
			p.SetBudgetController(budget.NewBudgetController(10.0))
			p.SetPersonalityCompiler(personality.NewCompiler(personality.DefaultCompilerConfig()))

			scope := InteractionScope{
				UserID: "user-need", CharacterID: "char-need",
				ConversationID: "conv-need", Channel: "web", Source: "web",
				RequestID: "req-need-" + tc.label,
			}.Normalize()

			assembly := p.Assemble(context.Background(), scope, &ProcessRequest{
				Message: tc.message, Channel: "web", Source: "web",
				UserID: scope.UserID, CharacterID: scope.CharacterID,
				RequestID: scope.RequestID,
			})

			if assembly.Appraisal == nil {
				t.Fatal("appraisal result is nil")
			}

			if assembly.Appraisal.NeedDeltas == nil {
				t.Fatal("need deltas are nil")
			}

			hasNonZero := false
			for key, delta := range assembly.Appraisal.NeedDeltas {
				if delta != 0 {
					hasNonZero = true
				}
				t.Logf("  NeedDeltas[%s] = %.4f", key, delta)
			}
			if !hasNonZero {
				t.Errorf("[%s] all need deltas are zero", tc.label)
			}
		})
	}
}
