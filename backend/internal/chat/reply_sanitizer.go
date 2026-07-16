package chat

import (
	"strings"

	promptir "github.com/u-ai/backend/internal/prompt"
)

func SanitizeReply(raw string, charName string, priorReplies []string) (string, promptir.QualityFlags) {
	if raw == "" {
		return "", promptir.QualityFlags{}
	}

	result := raw

	var flags promptir.QualityFlags

	result = stripThinkingTags(result)
	flags.ThinkRemoved = result != raw

	preJSON := result
	result = stripJSONWrap(result)
	flags.JSONWrapperRemoved = result != preJSON

	preHTML := result
	result = stripHTMLTags(result)
	flags.HTMLRemoved = result != preHTML

	preMarkdown := result
	result = stripMarkdownFormatting(result)
	flags.MarkdownRemoved = result != preMarkdown

	result = stripResponsePrefix(result)

	preRole := result
	result = stripRoleNamePrefix(result, charName)
	flags.RolePrefixRemoved = result != preRole

	preDup := result
	result = stripLineDuplicates(result)
	flags.DuplicateTrimmed = result != preDup

	preMeta := result
	result = extractDirectReply(result)
	flags.MetaSentenceRemoved = result != preMeta

	result = stripRepeatPhrases(result)

	preTrim := result
	result = trimToSentenceLimit(result, 8, 500)
	flags.ChannelLimitApplied = result != preTrim

	result = stripPriorRepeats(result, priorReplies)

	result = strings.TrimSpace(result)

	return result, flags
}
