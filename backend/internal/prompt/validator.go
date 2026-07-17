package prompt

import (
	"fmt"
	"strings"
)

type Validator struct {
	DisabledFlags []string
}

var flagToSectionTypes = map[string][]GwSectionType{
	"TextlibRawEnabled":      {GwSectionChannelShortRaw},
	"PersonalityRawEnabled":  {GwSectionPersonalityRaw},
	"EmotionFusionEnabled":   {GwSectionEmotionFusionRaw},
	"IntimacyDefaultEnabled": {GwSectionAdultIntimacyRaw},
	"MemoryRawEnabled":       {GwSectionMemoryInjectRaw, GwSectionMemoryExtractRaw},
	"ReplySanitizerEnabled":  {GwSectionOutputShapeRaw, GwSectionAntiRepeatRaw},
	"ProactiveRawEnabled":    {GwSectionProactiveRaw, GwSectionProactiveScene, GwSectionProactivePersonality, GwSectionProactiveTimeContext, GwSectionProactiveRelationship, GwSectionProactiveEmotion, GwSectionProactiveMemory, GwSectionProactiveRecentContext},
}

var flagToSectionNames = map[string]string{
	"TextlibRawEnabled":      "prompt_raw_textlib_enabled",
	"PersonalityRawEnabled":  "prompt_personality_raw_enabled",
	"EmotionFusionEnabled":   "prompt_emotion_fusion_enabled",
	"IntimacyDefaultEnabled": "prompt_intimacy_default_enabled",
	"MemoryRawEnabled":       "prompt_memory_raw_enabled",
	"ReplySanitizerEnabled":  "prompt_reply_sanitizer_enabled",
	"ProactiveRawEnabled":    "prompt_proactive_raw_enabled",
}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) ValidateIR(ir GwIR) error {
	for _, s := range ir.Sections {
		if s.TrustLevel == TrustUntrusted && s.InstructionMode != ModeDataOnly && s.Type != GwSectionCurrentUserMessage {
			return fmt.Errorf("untrusted section must be data_only: %s", s.ID)
		}

		if s.Type == GwSectionCurrentUserMessage && s.InstructionMode != ModeUserRequest {
			return fmt.Errorf("current user message must be user_request")
		}

		switch s.Type {
		case GwSectionMemoryContext, GwSectionProfileContext, GwSectionWorldbookContext, GwSectionPluginContext,
			GwSectionConversationHistory, GwSectionToolResult, GwSectionMultimodalText:
			if s.TrustLevel != TrustUntrusted {
				return fmt.Errorf("context section must be untrusted: %s", s.ID)
			}
		case GwSectionMemoryInjectRaw, GwSectionMemoryExtractRaw:
			if s.TrustLevel != TrustUntrusted {
				return fmt.Errorf("context section must be untrusted: %s", s.ID)
			}
		}
	}

	for _, flag := range v.DisabledFlags {
		sectionTypes, ok := flagToSectionTypes[flag]
		if !ok {
			continue
		}
		for _, st := range sectionTypes {
			for _, s := range ir.Sections {
				if s.Type == st {
					configName := flagToSectionNames[flag]
					if configName == "" {
						configName = flag
					}
					return fmt.Errorf("section type %s is present but feature flag %s (%s) is disabled", s.Type, configName, flag)
				}
			}
		}
	}

	return nil
}

func (v *Validator) ValidateMessages(messages []GwMessage) error {
	if len(messages) == 0 {
		return fmt.Errorf("empty messages")
	}

	if messages[0].Role != "system" {
		return fmt.Errorf("first message must be system")
	}

	for i, m := range messages {
		if i > 0 && m.Role == "system" {
			return fmt.Errorf("only first message can be system")
		}
	}

	last := messages[len(messages)-1]
	if last.Role != "user" || !strings.Contains(last.Content, "<current_user_message>") {
		return fmt.Errorf("last message must be current user message")
	}

	for _, m := range messages {
		if m.Role == "system" {
			forbidden := []string{
				"<untrusted_data",
				"<conversation_history",
				"<memory_context",
				"<worldbook_context",
				"<tool_result",
				"<multimodal_text",
			}
			for _, f := range forbidden {
				if strings.Contains(m.Content, f) {
					return fmt.Errorf("untrusted data leaked into system role: %s", f)
				}
			}
		}
	}

	return nil
}
