package modelprotocol

import "testing"

func TestApplyOpenAIChatControlsEnablesJSONAndDisablesDeepSeekThinking(t *testing.T) {
	requestBody := map[string]interface{}{}
	applyOpenAIChatControls(requestBody, ProviderConfig{
		BaseURL: "https://api.deepseek.com/v1",
	}, ModelRequest{
		ResponseFormat:  ModelResponseFormat{Type: "json_object"},
		DisableThinking: true,
	})

	responseFormat, ok := requestBody["response_format"].(map[string]string)
	if !ok || responseFormat["type"] != "json_object" {
		t.Fatalf("response_format was not enabled: %#v", requestBody["response_format"])
	}
	thinking, ok := requestBody["thinking"].(map[string]string)
	if !ok || thinking["type"] != "disabled" {
		t.Fatalf("deepseek thinking was not disabled: %#v", requestBody["thinking"])
	}
}

func TestApplyOpenAIChatControlsDoesNotSendDeepSeekThinkingToOtherProviders(t *testing.T) {
	requestBody := map[string]interface{}{}
	applyOpenAIChatControls(requestBody, ProviderConfig{
		BaseURL: "https://api.openai.com/v1",
	}, ModelRequest{
		ResponseFormat:  ModelResponseFormat{Type: "json_object"},
		DisableThinking: true,
	})

	if _, ok := requestBody["thinking"]; ok {
		t.Fatalf("unexpected thinking control: %#v", requestBody["thinking"])
	}
}

func TestApplyOpenAIChatControlsSendsReasoningEffort(t *testing.T) {
	for _, effort := range []string{"low", "medium", "high", "xhigh"} {
		requestBody := map[string]interface{}{}
		applyOpenAIChatControls(requestBody, ProviderConfig{
			BaseURL: "https://api.openai.com/v1",
		}, ModelRequest{ReasoningEffort: effort})

		if requestBody["reasoning_effort"] != effort {
			t.Fatalf("effort %q: unexpected request value %#v", effort, requestBody["reasoning_effort"])
		}
	}
}
