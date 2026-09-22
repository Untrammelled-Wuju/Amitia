package continuity

import (
	"context"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Resolver struct {
	repo          *Repository
	semantic      SemanticMatcher
	disambiguator ThreadDisambiguator
}

func NewResolver(repo *Repository) *Resolver { return &Resolver{repo: repo} }

func (r *Resolver) SetSemanticMatcher(matcher SemanticMatcher) { r.semantic = matcher }
func (r *Resolver) SetDisambiguator(disambiguator ThreadDisambiguator) {
	r.disambiguator = disambiguator
}

func (r *Resolver) Resolve(ctx context.Context, input ResolveInput) (Resolution, error) {
	if r == nil || r.repo == nil {
		return Resolution{Method: "none"}, nil
	}
	input.SpaceID = strings.TrimSpace(input.SpaceID)
	input.CharacterID = strings.TrimSpace(input.CharacterID)
	input.ConversationID = strings.TrimSpace(input.ConversationID)
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.ThreadID = strings.TrimSpace(input.ThreadID)
	input.Message = strings.TrimSpace(input.Message)
	var err error

	if input.ThreadID != "" {
		thread, err := r.repo.GetThread(input.ThreadID, input.SpaceID)
		if err != nil {
			return Resolution{}, err
		}
		if thread != nil && !thread.Status.IsTerminal() && threadCompatibleCharacter(thread, input.CharacterID) {
			r.bindScope(thread.ID, input, "explicit", 1)
			_ = r.repo.TouchThread(thread.ID)
			return Resolution{Thread: thread, Method: "explicit", Confidence: 1}, nil
		}
	}

	var bound *Thread
	if input.ConversationID != "" {
		bound, err = r.repo.MostRecentBoundThread(input.SpaceID, "conversation", input.ConversationID)
		if err != nil {
			return Resolution{}, err
		}
	}
	if bound == nil && input.WorkspaceID != "" && isContinuationCue(input.Message) {
		bound, err = r.repo.MostRecentBoundThread(input.SpaceID, "workspace", input.WorkspaceID)
		if err != nil {
			return Resolution{}, err
		}
	}

	active, err := r.repo.ListActiveThreads(input.SpaceID, input.CharacterID, 24)
	if err != nil {
		return Resolution{}, err
	}
	semanticScores := map[string]float64{}
	// Do not put an embedding/API call on every turn of an already-bound thread.
	// Semantic resolution is needed for new conversations, explicit continuation/switch
	// cues, or messages that look like the start of another persistent goal.
	needsSemantic := bound == nil || isContinuationCue(input.Message) || shouldCreateThread(input.Message)
	if needsSemantic && r.semantic != nil && len(active) > 0 && len([]rune(input.Message)) >= 3 {
		if scores, semanticErr := r.semantic.Scores(ctx, input.Message, active); semanticErr == nil {
			semanticScores = scores
		}
	}
	ranked := rankThreadCandidates(input.Message, active, semanticScores)
	if len(ranked) > 0 {
		best := ranked[0]
		margin := best.score
		if len(ranked) > 1 {
			margin = best.score - ranked[1].score
		}
		// Automatic attachment requires strong evidence and separation from the
		// next candidate. This intentionally favors a miss over a false attach.
		if best.score >= 0.88 && (len(ranked) == 1 || margin >= 0.10) {
			thread := best.thread
			r.bindScope(thread.ID, input, "semantic", best.score)
			_ = r.repo.TouchThread(thread.ID)
			return Resolution{Thread: &thread, Method: "semantic", Confidence: best.score}, nil
		}

		if r.disambiguator != nil && best.score >= 0.55 {
			candidates := make([]Thread, 0, minInt(5, len(ranked)))
			for i := 0; i < len(ranked) && i < 5; i++ {
				candidates = append(candidates, ranked[i].thread)
			}
			threadID, confidence, chooseErr := r.disambiguator.ChooseThread(ctx, input.Message, candidates)
			if chooseErr == nil && threadID != "" && confidence >= 0.72 {
				for i := range candidates {
					if candidates[i].ID == threadID {
						thread := candidates[i]
						r.bindScope(thread.ID, input, "llm_disambiguation", confidence)
						_ = r.repo.TouchThread(thread.ID)
						return Resolution{Thread: &thread, Method: "llm_disambiguation", Confidence: confidence}, nil
					}
				}
			}
		}

		if bound != nil && best.thread.ID != bound.ID && isContinuationCue(input.Message) && best.score >= 0.70 && (len(ranked) == 1 || margin >= 0.20) {
			thread := best.thread
			r.bindScope(thread.ID, input, "conversation_switch", best.score)
			_ = r.repo.TouchThread(thread.ID)
			return Resolution{Thread: &thread, Method: "conversation_switch", Confidence: best.score}, nil
		}
	}

	if bound != nil {
		if !input.SuppressCreate && shouldCreateThread(input.Message) {
			boundScore := scoreForThread(input.Message, *bound)
			if semanticScores[bound.ID] > boundScore {
				boundScore = semanticScores[bound.ID]
			}
			if boundScore < 0.18 {
				return r.createThread(input)
			}
		}
		r.bindScope(bound.ID, input, "conversation_binding", 0.99)
		_ = r.repo.TouchThread(bound.ID)
		return Resolution{Thread: bound, Method: "conversation_binding", Confidence: 0.99}, nil
	}

	// A vague continuation cue may use a single unambiguous active thread, but
	// never guesses when multiple unrelated threads exist.
	if isContinuationCue(input.Message) && len(active) == 1 {
		thread := active[0]
		r.bindScope(thread.ID, input, "recent", 0.86)
		_ = r.repo.TouchThread(thread.ID)
		return Resolution{Thread: &thread, Method: "recent", Confidence: 0.86}, nil
	}

	if !input.SuppressCreate && shouldCreateThread(input.Message) {
		return r.createThread(input)
	}
	return Resolution{Method: "none"}, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (r *Resolver) createThread(input ResolveInput) (Resolution, error) {
	thread := &Thread{SpaceID: input.SpaceID, CharacterID: input.CharacterID, Title: deriveTitle(input.Message), Goal: input.Message, Status: ThreadStatusActive, Summary: input.Message, CurrentState: "已识别为持续事项，等待本轮处理结果", NextAction: "根据当前目标继续处理", Confidence: 0.82}
	if err := r.repo.CreateThread(thread); err != nil {
		return Resolution{}, err
	}
	r.bindScope(thread.ID, input, "inception", 0.82)
	_, _ = r.repo.AppendEvent(&ThreadEvent{ThreadID: thread.ID, EventType: "thread.created", SourceType: "user_message", ConversationID: input.ConversationID, PayloadJSON: `{"summary":"持续事项已创建"}`, IdempotencyKey: "thread-created|" + thread.ID})
	return Resolution{Thread: thread, Method: "new", Confidence: 0.82, Created: true}, nil
}

func threadCompatibleCharacter(thread *Thread, characterID string) bool {
	if thread == nil {
		return false
	}
	characterID = strings.TrimSpace(characterID)
	return characterID == "" || strings.TrimSpace(thread.CharacterID) == "" || strings.TrimSpace(thread.CharacterID) == characterID
}

func (r *Resolver) Bind(threadID, bindingType, bindingID, source string, confidence float64) error {
	if r == nil || r.repo == nil {
		return nil
	}
	return r.repo.Bind(threadID, bindingType, bindingID, "context", source, confidence)
}

func (r *Resolver) bindScope(threadID string, input ResolveInput, source string, confidence float64) {
	_ = r.Bind(threadID, "conversation", input.ConversationID, source, confidence)
	_ = r.Bind(threadID, "workspace", input.WorkspaceID, source, confidence)
}

var continuationCues = []string{"继续", "接着", "上次", "昨天那个", "之前那个", "那个继续", "继续做", "继续弄", "continue", "resume", "pick up", "carry on"}

func isContinuationCue(message string) bool {
	v := strings.ToLower(strings.TrimSpace(message))
	for _, cue := range continuationCues {
		if strings.Contains(v, cue) {
			return true
		}
	}
	return utf8.RuneCountInString(v) <= 8 && (strings.Contains(v, "那个") || strings.Contains(v, "这个"))
}

var inceptionCues = []string{
	// Thread inception deliberately requires action/commitment language. Generic
	// nouns such as “项目/开发/部署” are not enough, otherwise ordinary
	// questions like “介绍一下这个项目” become persistent work by accident.
	"我要", "我准备", "我计划", "我打算", "我决定",
	"帮我做", "帮我开发", "帮我实现", "帮我修改", "帮我重构", "帮我迁移", "帮我排查", "帮我修复", "帮我部署", "帮我安排", "帮我规划", "帮我跟进",
	"需要完成", "接下来要", "以后要", "持续跟进", "继续跟进", "直到完成", "完成以后", "完成后继续",
	"i want to", "i need to", "i plan to", "help me build", "help me implement", "help me migrate", "help me fix", "help me deploy", "help me plan", "follow this until",
}

func shouldCreateThread(message string) bool {
	v := strings.ToLower(strings.TrimSpace(message))
	if v == "" || utf8.RuneCountInString(v) < 8 || isSmallTalk(v) {
		return false
	}
	for _, cue := range inceptionCues {
		if strings.Contains(v, cue) {
			return true
		}
	}
	return false
}

func isSmallTalk(v string) bool {
	clean := strings.TrimSpace(strings.Trim(v, "。！!？?，,~～"))
	values := map[string]bool{"你好": true, "早上好": true, "中午好": true, "晚上好": true, "哈哈": true, "哈哈哈": true, "谢谢": true, "ok": true, "okay": true, "hi": true, "hello": true}
	return values[strings.ToLower(clean)]
}

func deriveTitle(message string) string {
	v := strings.TrimSpace(message)
	v = regexp.MustCompile(`^[，,。.!！?？\s]+`).ReplaceAllString(v, "")
	runes := []rune(v)
	if len(runes) > 32 {
		runes = runes[:32]
	}
	return strings.TrimSpace(string(runes))
}

type scoredThread struct {
	idx   int
	score float64
}

func bestThreadMatch(message string, items []Thread) (*Thread, float64) {
	msgTokens := textTokens(message)
	if len(msgTokens) == 0 {
		return nil, 0
	}
	queryPhrase := normalizedResolutionPhrase(message)
	scores := make([]scoredThread, 0, len(items))
	for i := range items {
		text := strings.Join([]string{items[i].Title, items[i].Goal, items[i].Summary, items[i].CurrentState, items[i].NextAction}, " ")
		textLower := strings.ToLower(text)
		score := tokenSimilarity(msgTokens, textTokens(text))
		if strings.Contains(textLower, strings.ToLower(strings.TrimSpace(message))) && utf8.RuneCountInString(strings.TrimSpace(message)) >= 4 {
			score = math.Max(score, 0.9)
		}
		if utf8.RuneCountInString(queryPhrase) >= 4 && strings.Contains(textLower, queryPhrase) {
			score = math.Max(score, 0.95)
		}
		score = math.Max(score, queryTokenCoverage(message, text))
		scores = append(scores, scoredThread{idx: i, score: score})
	}
	sort.SliceStable(scores, func(i, j int) bool { return scores[i].score > scores[j].score })
	if len(scores) == 0 || scores[0].score <= 0 {
		return nil, 0
	}
	best := items[scores[0].idx]
	return &best, scores[0].score
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
	v := strings.ToLower(strings.TrimSpace(message))
	for _, cue := range continuationCues {
		v = strings.ReplaceAll(v, cue, "")
	}
	for _, filler := range []string{"那个", "这个", "一下", "一下吧", "吧", "的", "事情", "事项"} {
		v = strings.ReplaceAll(v, filler, "")
	}
	v = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Han, r) {
			return r
		}
		return -1
	}, v)
	return strings.TrimSpace(v)
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

func textTokens(s string) map[string]struct{} {
	s = strings.ToLower(strings.TrimSpace(s))
	out := map[string]struct{}{}
	ascii := strings.FieldsFunc(s, func(r rune) bool {
		return r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	for _, token := range ascii {
		token = strings.TrimSpace(token)
		if len(token) >= 2 {
			out[token] = struct{}{}
		}
	}
	runes := []rune(s)
	for i := 0; i < len(runes)-1; i++ {
		if unicode.Is(unicode.Han, runes[i]) && unicode.Is(unicode.Han, runes[i+1]) {
			out[string(runes[i:i+2])] = struct{}{}
		}
	}
	return out
}

func tokenSimilarity(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	if inter == 0 {
		return 0
	}
	return float64(inter) / math.Sqrt(float64(len(a)*len(b)))
}
