package chat

func conservativeModelOutputCeiling(configured, maxTokens int, anthropic bool, thinking int) int64 {
	output := configured
	if maxTokens > output {
		output = maxTokens
	}
	if output <= 0 {
		output = 4096
	}
	if anthropic && thinking > 0 && output <= thinking {
		output = thinking + 2048
	}
	return int64(output)
}
