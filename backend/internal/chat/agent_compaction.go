package chat

func agentCompactionBoundary(messages []map[string]interface{}, baseMessageCount, requestedBoundary int) int {
	if baseMessageCount < 0 {
		baseMessageCount = 0
	}
	if requestedBoundary <= baseMessageCount {
		return baseMessageCount
	}
	if requestedBoundary >= len(messages) {
		return len(messages)
	}
	if messages[requestedBoundary]["role"] != "tool" {
		return requestedBoundary
	}
	start := requestedBoundary - 1
	for start >= baseMessageCount && messages[start]["role"] == "tool" {
		start--
	}
	if start < baseMessageCount || messages[start]["role"] != "assistant" || messages[start]["tool_calls"] == nil {
		return baseMessageCount
	}
	return start
}
