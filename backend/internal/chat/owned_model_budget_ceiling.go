package chat

func ownedModelOutputCeiling(cfg *ModelConfig) int64 {
	if cfg == nil {
		return 0
	}
	return conservativeModelOutputCeiling(cfg.MaxOutputTokens, cfg.MaxTokens, resolveProtocol(cfg) == ProtocolAnthropicMessages, reasoningBudget(cfg.ReasoningEffort))
}
