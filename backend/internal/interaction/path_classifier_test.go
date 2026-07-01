package interaction

import "testing"

func TestPathClassifier_FastPath_SimpleMessage(t *testing.T) {
	c := NewPathClassifier()
	input := PathClassifyInput{
		MessageContent: "你好",
		RoleState: PsycheState{
			Fatigue: 0.1,
			Stress:  0.1,
			Arousal: 0.8,
		},
		Urgency:      1,
		IsEmotional:  false,
		Attachments:  0,
		HasCommands:  false,
		PreviousPath: "",
	}

	result := c.Classify(input)
	if result != PathTypeFast {
		t.Errorf("expected Fast path for simple message, got %s", result)
	}
}

func TestPathClassifier_DeepPath_Emotional(t *testing.T) {
	c := NewPathClassifier()
	input := PathClassifyInput{
		MessageContent: "我真的很伤心，不知道该怎么办了",
		RoleState: PsycheState{
			Fatigue: 0.5,
			Stress:  0.8,
			Arousal: 0.3,
		},
		Urgency:      8,
		IsEmotional:  true,
		Attachments:  0,
		HasCommands:  true,
		PreviousPath: PathTypeDeep,
	}

	result := c.Classify(input)
	if result != PathTypeDeep {
		t.Errorf("expected Deep path for emotional high-urgency message, got %s", result)
	}
}

func TestPathClassifier_StandardPath(t *testing.T) {
	c := NewPathClassifier()
	input := PathClassifyInput{
		MessageContent: "今天天气不错，有什么好玩的地方推荐吗",
		RoleState: PsycheState{
			Fatigue: 0.3,
			Stress:  0.4,
			Arousal: 0.6,
		},
		Urgency:      4,
		IsEmotional:  false,
		Attachments:  1,
		HasCommands:  false,
		PreviousPath: PathTypeStandard,
	}

	result := c.Classify(input)
	if result != PathTypeStandard {
		t.Errorf("expected Standard path for normal message, got %s", result)
	}
}

func TestPathClassifier_ValidPaths(t *testing.T) {
	c := NewPathClassifier()
	validPaths := map[PathType]bool{
		PathTypeFast:     true,
		PathTypeStandard: true,
		PathTypeDeep:     true,
	}

	tests := []struct {
		name  string
		input PathClassifyInput
	}{
		{"empty_message", PathClassifyInput{
			MessageContent: "", RoleState: PsycheState{}, Urgency: 0,
		}},
		{"long_emotional_urgent", PathClassifyInput{
			MessageContent: "非常紧急！系统出问题了，需要立即处理",
			RoleState:      PsycheState{Fatigue: 0.7, Stress: 0.9, Arousal: 0.2, SocialLoad: 0.8},
			Urgency:        10, IsEmotional: true, Attachments: 3, HasCommands: true,
		}},
		{"simple_greeting", PathClassifyInput{
			MessageContent: "hi", RoleState: PsycheState{Fatigue: 0.0, Stress: 0.0, Arousal: 1.0},
			Urgency: 0, IsEmotional: false, Attachments: 0, HasCommands: false,
		}},
		{"complex_discussion", PathClassifyInput{
			MessageContent: "请详细分析一下人工智能的未来发展趋势及其对社会的影响",
			RoleState:      PsycheState{Fatigue: 0.2, Stress: 0.2, Arousal: 0.7},
			Urgency:        3, IsEmotional: false, Attachments: 0, HasCommands: true,
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.Classify(tt.input)
			if !validPaths[result] {
				t.Errorf("invalid path type: %s", result)
			}
		})
	}
}

func TestPathClassifier_DeepWithHighFatigue(t *testing.T) {
	c := NewPathClassifier()
	input := PathClassifyInput{
		MessageContent: "你能帮我看看这个吗",
		RoleState: PsycheState{
			Fatigue:    0.9,
			Stress:     0.9,
			Arousal:    0.1,
			SocialLoad: 0.9,
		},
		Urgency:     7,
		IsEmotional: true,
		Attachments: 0,
		HasCommands: false,
	}

	result := c.Classify(input)
	if result != PathTypeDeep {
		t.Errorf("expected Deep path when fatigue/stress are high, got %s", result)
	}
}

func TestPathClassifier_FastWithLowEverything(t *testing.T) {
	c := NewPathClassifier()
	input := PathClassifyInput{
		MessageContent: "ok",
		RoleState: PsycheState{
			Fatigue:    0.0,
			Stress:     0.0,
			Arousal:    1.0,
			SocialLoad: 0.0,
		},
		Urgency:      0,
		IsEmotional:  false,
		Attachments:  0,
		HasCommands:  false,
		PreviousPath: PathTypeFast,
	}

	result := c.Classify(input)
	if result != PathTypeFast {
		t.Errorf("expected Fast path when all signals are low, got %s", result)
	}
}
