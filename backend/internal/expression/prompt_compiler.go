package expression

import (
	"strings"
)

type CompiledPrompt struct {
	SystemInstruction string
	StyleInstruction  string
}

func CompileChannelPrompt(kind ChannelKind) CompiledPrompt {
	policy := GetChannelPolicy(kind)
	return compileWithPolicy(policy)
}

func CompileChannelPromptWithPolicy(kind ChannelKind, policy ChannelPolicy) CompiledPrompt {
	return compileWithPolicy(policy)
}

func compileWithPolicy(policy ChannelPolicy) CompiledPrompt {
	var instructionParts []string
	var styleParts []string

	instructionParts = append(instructionParts, "【回复格式 - 系统固定规则】")

	switch policy.SegmentHint {
	case "short_per_line":
		instructionParts = append(instructionParts,
			"每句话必须单独一行，用换行符分隔。",
			"每句话尽量短，像微信连续消息一样。",
			"能一句说完就一句，不要写长段落。",
			"不要把多句话连成一段。",
			"不要用句号连接多个意思。",
		)
		styleParts = append(styleParts,
			"你和用户是比较熟悉的长期对话关系，不需要像客服或正式助手一样说话。",
			"回复要自然、有反应、有一点态度，可以适当使用「嗯？、喔、奥奥、ok、好、行、确实、懂了」等语气词。",
			"用户随口聊，你就自然接话；用户认真问问题，你再认真回答。",
			"不要客服腔，不要过度正式，不要每次都完整总结，也不要动不动分点讲大道理。",
			"回复格式要像微信连续消息：",
			"用户发一句话时，你可以回复 1 到 4 句短句。",
			"不要写成一整段长文。",
			"整体目标是：像一个熟悉用户、说话自然、有判断力的人。该短就短，该认真就认真，不端着，也不表演过头。",
		)
	case "full_paragraph":
		instructionParts = append(instructionParts,
			"回复可以写成完整段落，不需要刻意拆分短句。",
			"使用自然流畅的中文表达，可适当使用Markdown格式增强可读性。",
			"保持条理清晰，但避免过度正式的公文腔。",
		)
		styleParts = append(styleParts,
			"你和用户是自然的对话关系，回复要有温度。",
			"可以比微信更完整地表达，但仍然保持亲切自然。",
			"遇到复杂问题时可以适度展开说明，但不要啰嗦。",
			"整体目标是：像一位体贴且有知识的朋友，既能深入交流也能轻松聊天。",
		)
	case "single_utterance":
		instructionParts = append(instructionParts,
			"回复应为单一简短语句，适合语音播报。",
			"不要分段，不要使用任何格式标记。",
			"控制在不超过120字的长度内。",
		)
		styleParts = append(styleParts,
			"用口语化方式回复，自然流畅像真人说话。",
			"使用简短的句子，符合语音对话的习惯。",
		)
	default:
		instructionParts = append(instructionParts,
			"回复应简洁自然，不分段过长。",
			"避免长篇大论，优先聚焦用户当下关心的事。",
		)
		styleParts = append(styleParts,
			"以自然友好的方式回复，保持适度温暖。",
		)
	}

	if !policy.Capabilities.SupportsMarkdown {
		instructionParts = append(instructionParts, "回复中不要使用任何emoji表情符号。", "不能使用markdown格式。")
	}

	if !policy.Capabilities.SupportsVoice {
		instructionParts = append(instructionParts, "【工具使用规则 - 严格遵守】",
			"create_schedule 仅在用户明确要求\"提醒\"、\"叫\"、\"通知\"、\"叫醒\"、\"定时\"等场景时调用。",
			"禁止在用户只问时间、闲聊、打招呼、问天气等日常对话中调用 create_schedule。",
			"get_current_time 仅在用户明确询问当前时间时调用。",
			"不要在用户没有明确要求的情况下自动创建任何提醒。",
			"force_voice_reply 仅在用户明确要求\"用语音回复\"、\"发语音\"、\"语音回答\"、\"说语音\"、\"讲语音\"时调用。调用后本次回复会以语音形式发送。",
		)
	}

	return CompiledPrompt{
		SystemInstruction: strings.Join(instructionParts, "\n"),
		StyleInstruction:  strings.Join(styleParts, "\n"),
	}
}

func ApplyPostValidation(raw string, kind ChannelKind) string {
	policy := GetChannelPolicy(kind)
	result := raw

	if !policy.Capabilities.SupportsMarkdown {
		result = stripMarkdown(result)
	}

	if policy.MaxCharacters > 0 {
		runes := []rune(result)
		if len(runes) > policy.MaxCharacters {
			result = string(runes[:policy.MaxCharacters])
		}
	}

	return result
}

func stripMarkdown(input string) string {
	replacements := []struct{ old, new string }{
		{"**", ""},
		{"__", ""},
		{"*", ""},
		{"_", ""},
		{"`", ""},
		{"#", ""},
	}
	result := input
	for _, r := range replacements {
		result = strings.ReplaceAll(result, r.old, r.new)
	}
	return result
}
