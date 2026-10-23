package continuity

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

func ThreadCompatibleCharacter(thread *Thread, characterID string) bool {
	if thread == nil {
		return false
	}
	characterID = strings.TrimSpace(characterID)
	return characterID == "" || strings.TrimSpace(thread.CharacterID) == "" || strings.TrimSpace(thread.CharacterID) == characterID
}

var continuationCues = []string{"继续", "接着", "上次", "昨天那个", "之前那个", "那个继续", "继续做", "继续弄", "continue", "resume", "pick up", "carry on"}

func IsContinuationCue(message string) bool {
	value := strings.ToLower(strings.TrimSpace(message))
	for _, cue := range continuationCues {
		if strings.Contains(value, cue) {
			return true
		}
	}
	return utf8.RuneCountInString(value) <= 8 && (strings.Contains(value, "那个") || strings.Contains(value, "这个"))
}

var inceptionCues = []string{
	"我要", "我准备", "我计划", "我打算", "我决定",
	"帮我做", "帮我开发", "帮我实现", "帮我修改", "帮我重构", "帮我迁移", "帮我排查", "帮我修复", "帮我部署", "帮我安排", "帮我规划", "帮我跟进",
	"需要完成", "接下来要", "以后要", "持续跟进", "继续跟进", "直到完成", "完成以后", "完成后继续",
	"i want to", "i need to", "i plan to", "help me build", "help me implement", "help me migrate", "help me fix", "help me deploy", "help me plan", "follow this until",
}

func ShouldCreateThread(message string) bool {
	value := strings.ToLower(strings.TrimSpace(message))
	if value == "" || utf8.RuneCountInString(value) < 8 || isSmallTalk(value) {
		return false
	}
	for _, cue := range inceptionCues {
		if strings.Contains(value, cue) {
			return true
		}
	}
	return false
}

func isSmallTalk(value string) bool {
	clean := strings.TrimSpace(strings.Trim(value, "。！!？?，,~～"))
	values := map[string]bool{"你好": true, "早上好": true, "中午好": true, "晚上好": true, "哈哈": true, "哈哈哈": true, "谢谢": true, "ok": true, "okay": true, "hi": true, "hello": true}
	return values[strings.ToLower(clean)]
}

func DeriveThreadTitle(message string) string {
	value := strings.TrimSpace(message)
	value = regexp.MustCompile(`^[，,。.!！?？\s]+`).ReplaceAllString(value, "")
	runes := []rune(value)
	if len(runes) > 32 {
		runes = runes[:32]
	}
	return strings.TrimSpace(string(runes))
}

func RankThreads(message string, threads []Thread, semantic map[string]float64) []RankedThread {
	ranked := rankThreadCandidates(message, threads, semantic)
	result := make([]RankedThread, 0, len(ranked))
	for _, item := range ranked {
		result = append(result, RankedThread{Thread: item.thread, Score: item.score})
	}
	return result
}

func scoreForThread(message string, item Thread) float64 {
	text := strings.Join([]string{item.Title, item.Goal, item.Summary, item.CurrentState, item.NextAction}, " ")
	score := tokenSimilarity(textTokens(message), textTokens(text))
	queryPhrase := normalizedResolutionPhrase(message)
	if utf8.RuneCountInString(queryPhrase) >= 4 && strings.Contains(strings.ToLower(text), queryPhrase) {
		score = math.Max(score, 0.95)
	}
	return math.Max(score, queryTokenCoverage(message, text))
}

func normalizedResolutionPhrase(message string) string {
	value := strings.ToLower(strings.TrimSpace(message))
	for _, cue := range continuationCues {
		value = strings.ReplaceAll(value, cue, "")
	}
	for _, filler := range []string{"那个", "这个", "一下", "一下吧", "吧", "的", "事情", "事项"} {
		value = strings.ReplaceAll(value, filler, "")
	}
	value = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Han, r) {
			return r
		}
		return -1
	}, value)
	return strings.TrimSpace(value)
}

func queryTokenCoverage(message, text string) float64 {
	query := textTokens(normalizedResolutionPhrase(message))
	if len(query) == 0 {
		return 0
	}
	target := textTokens(text)
	matched := 0
	for token := range query {
		if _, ok := target[token]; ok {
			matched++
		}
	}
	if matched == 0 {
		return 0
	}
	coverage := float64(matched) / float64(len(query))
	if matched < 2 && utf8.RuneCountInString(normalizedResolutionPhrase(message)) < 4 {
		return 0
	}
	return coverage
}

func textTokens(value string) map[string]struct{} {
	value = strings.ToLower(strings.TrimSpace(value))
	out := map[string]struct{}{}
	ascii := strings.FieldsFunc(value, func(r rune) bool {
		return r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	for _, token := range ascii {
		token = strings.TrimSpace(token)
		if len(token) >= 2 {
			out[token] = struct{}{}
		}
	}
	runes := []rune(value)
	for index := 0; index < len(runes)-1; index++ {
		if unicode.Is(unicode.Han, runes[index]) && unicode.Is(unicode.Han, runes[index+1]) {
			out[string(runes[index:index+2])] = struct{}{}
		}
	}
	return out
}

func tokenSimilarity(left, right map[string]struct{}) float64 {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	intersection := 0
	for token := range left {
		if _, ok := right[token]; ok {
			intersection++
		}
	}
	if intersection == 0 {
		return 0
	}
	return float64(intersection) / math.Sqrt(float64(len(left)*len(right)))
}

func sortRankedThreads(items []RankedThread) {
	sort.SliceStable(items, func(left, right int) bool { return items[left].Score > items[right].Score })
}
