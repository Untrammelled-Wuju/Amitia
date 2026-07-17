package prompt

import (
	"strings"
	"testing"
)

func TestPluginContextIsOrderedAndRenderedAsUntrustedData(t *testing.T) {
	ir := NewBuilder().Build(BuildRequest{Worldbook: "world", PluginContext: "plugin fact", History: "history", CurrentUserInput: "hello", DropEmptySections: true})
	indices := map[GwSectionType]int{}
	for index, section := range ir.Sections {
		indices[section.Type] = index
		if section.Type == GwSectionPluginContext && (section.TrustLevel != TrustUntrusted || section.InstructionMode != ModeDataOnly) {
			t.Fatalf("plugin context is not data-only untrusted content: %#v", section)
		}
	}
	if !(indices[GwSectionWorldbookContext] < indices[GwSectionPluginContext] && indices[GwSectionPluginContext] < indices[GwSectionConversationHistory]) {
		t.Fatalf("plugin context order is invalid: %#v", indices)
	}
	messages, err := NewRenderer().Render(ir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, message := range messages {
		if strings.Contains(message.Content, `type="plugin_context"`) && strings.Contains(message.Content, `instruction_mode="data_only"`) {
			if message.Role != "user" {
				t.Fatal("plugin context leaked into system role")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("plugin context was not rendered")
	}
}
