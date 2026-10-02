package prompt

import (
	"strings"
	"testing"
)

func TestChatDeliveryContractIsSystemInstruction(t *testing.T) {
	request := BuildRequest{CharacterConfig: "测试角色", ChatDeliveryContract: "使用 [AMITIA_BR] 拆分消息", CurrentUserInput: "你好"}
	messages, _, err := NewGateway().BuildMessages(request)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(messages[0].Content, "使用 [AMITIA_BR] 拆分消息") || messages[0].Role != "system" {
		t.Fatal("delivery contract missing from system instructions")
	}
	for _, message := range messages[1:] {
		if strings.Contains(message.Content, "使用 [AMITIA_BR] 拆分消息") {
			t.Fatal("delivery contract leaked into user messages")
		}
	}
	request.ChatDeliveryContract = ""
	messages, _, err = NewGateway().BuildMessages(request)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(messages[0].Content, "[AMITIA_BR]") {
		t.Fatal("disabled contract is still present")
	}
}
