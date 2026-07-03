package affect

import (
	"encoding/json"
	"testing"
)

func TestChineseEmotionLabel_HighPleasure_HighArousal_HighDominance_Excited(t *testing.T) {
	tag := ChineseEmotionLabel(0.8, 0.7, 0.75)
	if tag.Tag != "兴奋" {
		t.Fatalf("expected 兴奋, got %s (P=%.2f A=%.2f D=%.2f)", tag.Tag, tag.Pleasure, tag.Arousal, tag.Dominance)
	}
}

func TestChineseEmotionLabel_HighPleasure_LowArousal_Relaxed(t *testing.T) {
	tag := ChineseEmotionLabel(0.7, 0.2, 0.45)
	if tag.Tag != "平静" {
		t.Fatalf("expected 平静, got %s (P=%.2f A=%.2f D=%.2f)", tag.Tag, tag.Pleasure, tag.Arousal, tag.Dominance)
	}
}

func TestChineseEmotionLabel_LowPleasure_HighArousal_LowDominance_Fear(t *testing.T) {
	tag := ChineseEmotionLabel(0.2, 0.7, 0.2)
	if tag.Tag != "恐惧" {
		t.Fatalf("expected 恐惧, got %s (P=%.2f A=%.2f D=%.2f)", tag.Tag, tag.Pleasure, tag.Arousal, tag.Dominance)
	}
}

func TestChineseEmotionLabel_LowPleasure_LowArousal_LowDominance_Sad(t *testing.T) {
	tag := ChineseEmotionLabel(0.2, 0.2, 0.2)
	if tag.Tag != "悲伤" {
		t.Fatalf("expected 悲伤, got %s (P=%.2f A=%.2f D=%.2f)", tag.Tag, tag.Pleasure, tag.Arousal, tag.Dominance)
	}
}

func TestChineseEmotionLabel_LowPleasure_HighArousal_MidDominance_Anxious(t *testing.T) {
	tag := ChineseEmotionLabel(0.2, 0.7, 0.45)
	if tag.Tag != "焦虑" {
		t.Fatalf("expected 焦虑, got %s (P=%.2f A=%.2f D=%.2f)", tag.Tag, tag.Pleasure, tag.Arousal, tag.Dominance)
	}
}

func TestChineseEmotionLabel_LowPleasure_LowArousal_MidDominance_Depressed(t *testing.T) {
	tag := ChineseEmotionLabel(0.2, 0.2, 0.45)
	if tag.Tag != "消沉" {
		t.Fatalf("expected 消沉, got %s (P=%.2f A=%.2f D=%.2f)", tag.Tag, tag.Pleasure, tag.Arousal, tag.Dominance)
	}
}

func TestChineseEmotionLabel_MidPleasure_MidArousal_MidDominance_Calm(t *testing.T) {
	tag := ChineseEmotionLabel(0.5, 0.45, 0.5)
	if tag.Tag != "平静" {
		t.Fatalf("expected 平静, got %s (P=%.2f A=%.2f D=%.2f)", tag.Tag, tag.Pleasure, tag.Arousal, tag.Dominance)
	}
}

func TestChineseEmotionLabel_HighPleasure_LowArousal_LowDominance_Gentle(t *testing.T) {
	tag := ChineseEmotionLabel(0.7, 0.2, 0.2)
	if tag.Tag != "温和" {
		t.Fatalf("expected 温和, got %s (P=%.2f A=%.2f D=%.2f)", tag.Tag, tag.Pleasure, tag.Arousal, tag.Dominance)
	}
}

func TestChineseEmotionLabel_RangeBoundary_BorderlineValues(t *testing.T) {
	labelHigh := ChineseEmotionLabel(0.55, 0.55, 0.55)
	if labelHigh.Tag == "" || labelHigh.Tag == "低愉悦_" {
		t.Fatalf("expected valid tag at boundary, got %s", labelHigh.Tag)
	}
	labelLow := ChineseEmotionLabel(0.35, 0.35, 0.35)
	if labelLow.Tag == "" || labelLow.Tag == "低愉悦_" {
		t.Fatalf("expected valid tag at boundary, got %s", labelLow.Tag)
	}
}

func TestChineseEmotionLabel_NeverReturnsEmpty(t *testing.T) {
	values := []float64{0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0}
	for _, p := range values {
		for _, a := range values {
			for _, d := range values {
				tag := ChineseEmotionLabel(p, a, d)
				if tag.Tag == "" {
					t.Fatalf("empty tag at P=%.2f A=%.2f D=%.2f", p, a, d)
				}
			}
		}
	}
}

func TestChineseEmotionLabel_JSONSerialization(t *testing.T) {
	tag := ChineseEmotionLabel(0.8, 0.7, 0.75)
	data, err := json.Marshal(tag)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ChineseEmotionTag
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Tag != tag.Tag || decoded.Pleasure != tag.Pleasure {
		t.Fatalf("JSON round-trip mismatch: %#v vs %#v", decoded, tag)
	}
}

func TestChineseEmotionLabel_PADLabelCoherence(t *testing.T) {
	eng := PADLabel(0.8, 0.7, 0.75)
	cn := ChineseEmotionLabel(0.8, 0.7, 0.75)
	if eng == "" || cn.Tag == "" {
		t.Fatalf("both labels should be non-empty: eng=%s cn=%s", eng, cn.Tag)
	}
}
