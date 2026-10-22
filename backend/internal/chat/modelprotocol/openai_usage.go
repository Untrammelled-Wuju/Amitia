package modelprotocol

type openAIChatUsage struct {
	PromptTokens          int  `json:"prompt_tokens"`
	CompletionTokens      int  `json:"completion_tokens"`
	TotalTokens           int  `json:"total_tokens"`
	PromptCacheHitTokens  *int `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens *int `json:"prompt_cache_miss_tokens"`
	PromptTokensDetails   struct {
		CachedTokens *int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}

func (u openAIChatUsage) modelUsage() ModelUsage {
	usage := ModelUsage{InputTokens: u.PromptTokens, OutputTokens: u.CompletionTokens, TotalTokens: u.TotalTokens, ReasoningTokens: u.CompletionTokensDetails.ReasoningTokens}
	cached := u.PromptTokensDetails.CachedTokens
	if u.PromptCacheHitTokens != nil {
		cached = u.PromptCacheHitTokens
		if usage.InputTokens == 0 && u.PromptCacheMissTokens != nil {
			usage.InputTokens = *cached + *u.PromptCacheMissTokens
		}
	}
	if cached != nil && *cached >= 0 && usage.InputTokens >= *cached {
		usage.CachedInputTokens = *cached
		usage.CacheUsageReported = true
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}
	return usage
}
