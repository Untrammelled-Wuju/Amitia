package prompt

import (
	"fmt"
	"strings"
)

type Validator struct{}

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
		case GwSectionMemoryContext, GwSectionProfileContext, GwSectionWorldbookContext,
			GwSectionConversationHistory, GwSectionToolResult, GwSectionMultimodalText:
			if s.TrustLevel != TrustUntrusted {
				return fmt.Errorf("context section must be untrusted: %s", s.ID)
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
