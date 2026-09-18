package emotionstate

import (
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/psyche"
	"github.com/u-ai/backend/internal/relationship"
)

func TestComposeCurrentExpressionPrioritizesUserConditionWithoutResettingRelationship(t *testing.T) {
	psycheState := &psyche.PsycheState{
		Energy: 0.7,
		Emotion: psyche.EmotionDimensions{
			Valence:   0.45,
			Arousal:   0.4,
			Dominance: 0.55,
		},
	}
	relation := &relationship.RelationshipState{
		Trust:            0.6,
		Familiarity:      0.5,
		Security:         0.5,
		Tension:          0.6,
		RepairConfidence: 0.4,
	}
	transient := RelationshipEmotion{Anger: 0.7, Hurt: 0.5, Concern: 0.2}
	affect := UserAffect{PrimaryEmotion: "sadness", Severity: 0.8, Stress: 0.7, Need: "comfort", Confidence: 0.9}
	current := Compose(psycheState, relation, transient, affect, Signals{})
	if current.Primary != "restrained_concern" {
		t.Fatalf("expected restrained concern, got %+v", current)
	}
	if current.AngerDisplay >= transient.Anger || current.HurtDisplay >= transient.Hurt {
		t.Fatalf("relationship emotion was not suppressed for display: %+v", current)
	}
	if current.Concern < 0.7 {
		t.Fatalf("concern did not increase: %+v", current)
	}
}

func TestBuildContextRendersPromptAndVoiceInstruction(t *testing.T) {
	state := defaultState("user-1", "char-1", time.Now().UTC())
	state.UserAffect = UserAffect{PrimaryEmotion: "anxiety", Severity: 0.6, Stress: 0.7, Need: "comfort", Confidence: 0.8}
	state.RelationshipEmotion = RelationshipEmotion{Concern: 0.7, Warmth: 0.5}
	contextData := BuildContext(nil, nil, state, "角色正在忙")
	if !strings.Contains(contextData.Prompt, "用户当前情绪") || !strings.Contains(contextData.Prompt, "角色正在忙") {
		t.Fatalf("unexpected prompt: %s", contextData.Prompt)
	}
	if !strings.Contains(contextData.VoiceInstruction, "语速") || !strings.Contains(contextData.VoiceInstruction, "停顿") {
		t.Fatalf("unexpected voice instruction: %s", contextData.VoiceInstruction)
	}
}
