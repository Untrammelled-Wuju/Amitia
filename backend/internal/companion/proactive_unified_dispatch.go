package companion

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/prompt/textlib"
	"log"
)

var errProactiveUnifiedEntryMissing = errors.New("proactive unified entry is not configured")

type proactiveDeliveryScope struct {
	channel string
	peerID  string
	userID  string
}

func (s *service) submitProactiveMessage(ctx context.Context, characterID, conversationID, channelSetting, prompt, requestID string) (*interaction.OrchestrationResult, error) {
	if s.unifiedEntry == nil {
		return nil, errProactiveUnifiedEntryMissing
	}
	if !s.shouldProactivelyMessage(conversationID) {
		log.Printf("[companion] shouldProactivelyMessage=false, skip proactive dispatch for conversationID=%s", conversationID)
		return nil, nil
	}
	scope := s.resolveProactiveDeliveryScope(conversationID, channelSetting, characterID)

	timeCtx := s.buildProactiveTimeContext()
	recentCtx := s.buildProactiveRecentContext(conversationID, characterID)

	relCtx := s.buildProactiveRelationshipContext(conversationID, characterID)
	emoCtx := s.buildProactiveEmotionContext(conversationID, characterID)
	memCtx := s.buildProactiveMemoryContext(conversationID, characterID)

	req := &interaction.UnifiedEntryRequest{
		Channel:                  scope.channel,
		PeerID:                   scope.peerID,
		UserID:                   scope.userID,
		Source:                   "proactive",
		CharacterID:              characterID,
		ConversationID:           conversationID,
		RequestID:                requestID,
		SessionID:                "proactive:" + characterID,
		IsInternal:               true,
		ProactiveTaskInstruction: prompt,
		ProactiveTimeContext:     timeCtx,
		ProactiveRecentContext:   recentCtx,
		ProactiveRelationship:    relCtx,
		ProactiveEmotion:         emoCtx,
		ProactiveMemory:          memCtx,
	}
	return s.unifiedEntry.Handle(ctx, req)
}

func (s *service) shouldProactivelyMessage(conversationID string) bool {
	var messages []chat.Message
	if err := s.db.Where("conversation_id = ?", conversationID).
		Order("sequence DESC").
		Limit(1).
		Find(&messages).Error; err != nil || len(messages) == 0 {
		return len(messages) == 0
	}

	lastMessage := messages[0]

	if lastMessage.Role != "user" {
		return false
	}

	lastUserMsg := lastMessage.Content

	for _, pattern := range textlib.ProactiveGoodbyePatterns {
		if strings.Contains(lastUserMsg, pattern) {
			return false
		}
	}

	for _, pattern := range textlib.ProactiveStopReplyPatterns {
		if strings.Contains(lastUserMsg, pattern) {
			return false
		}
	}

	for _, pattern := range textlib.ProactiveShortEndingPatterns {
		if strings.TrimSpace(lastUserMsg) == pattern {
			return false
		}
	}

	for _, pattern := range textlib.ProactiveAckEndingPatterns {
		if strings.TrimSpace(lastUserMsg) == pattern {
			return false
		}
	}

	lastMsgTime, err := time.Parse("2006-01-02 15:04:05", lastMessage.CreatedAt)
	if err == nil {
		timeSinceLastMsg := time.Since(lastMsgTime).Milliseconds()
		if timeSinceLastMsg < textlib.ProactiveMinGapMinutes*60*1000 {
			return false
		}
	}

	runes := []rune(strings.TrimSpace(lastUserMsg))
	if len(runes) < 3 && !strings.ContainsAny(lastUserMsg, "?？吗呢什么怎么为什么多少") {
		return false
	}

	return true
}

func (s *service) buildProactiveTimeContext() string {
	now := time.Now()
	hour := now.Hour()
	minute := now.Minute()
	second := now.Second()
	weekday := now.Weekday()

	weekdayNames := map[time.Weekday]string{
		time.Monday:    "周一",
		time.Tuesday:   "周二",
		time.Wednesday: "周三",
		time.Thursday:  "周四",
		time.Friday:    "周五",
		time.Saturday:  "周六",
		time.Sunday:    "周日",
	}
	weekdayName := weekdayNames[weekday]

	timeStr := fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)

	var timeScenario string
	switch {
	case hour >= 5 && hour <= 7:
		hint := "凌晨了"
		if hour == 6 {
			hint = "天快亮了"
		} else if hour >= 7 {
			hint = "早上了"
		}
		timeScenario = fmt.Sprintf("%s（%s），用户可能刚醒或还没醒。可以关心对方有没有起床、早安、问要不要一起吃早餐、提醒今天有什么安排。", hint, timeStr)
	case hour >= 8 && hour <= 10:
		timeScenario = fmt.Sprintf("上午（%s），用户可能在上班/上学路上或刚开始工作。可以聊早上发生了什么、吃了没、今天心情怎么样、提醒别迟到。", timeStr)
	case hour >= 11 && hour <= 12:
		timeScenario = fmt.Sprintf("快到午饭时间了（%s），用户肚子应该饿了。可以问吃什么、要不要一起点外卖、中午休息一下、吐槽食堂/外卖难吃。", timeStr)
	case hour >= 13 && hour <= 14:
		timeScenario = fmt.Sprintf("午休时间（%s），用户可能在犯困打盹。可以问睡醒了没、下午要干嘛、分享自己也在犯困、叫对方起来活动一下。", timeStr)
	case hour >= 15 && hour <= 17:
		timeScenario = fmt.Sprintf("下午（%s），工作时间过半，用户可能累了或在摸鱼。可以聊下班还有多久、想不想喝奶茶、摸鱼中吗、等下一起去吃点什么。", timeStr)
	case hour >= 18 && hour <= 19:
		timeScenario = fmt.Sprintf("下班/放学时间（%s），用户在回家路上或刚到家。可以问到家了没、路上堵不堵、晚上想干什么、要不要一起打游戏/看剧/吃饭。", timeStr)
	case hour >= 20 && hour <= 22:
		timeScenario = fmt.Sprintf("晚间休闲时间（%s），用户在放松。可以聊今天过得怎么样、分享有趣的事、撒娇求关注、催对方早点洗澡、一起追剧/打游戏。", timeStr)
	case hour >= 23 || hour <= 4:
		timeScenario = fmt.Sprintf("深夜/凌晨（%s），用户还没睡。可以问怎么还不睡、明天不用早起吗、陪对方聊天、温柔地哄睡觉、说晚安。", timeStr)
	default:
		timeScenario = timeStr
	}

	isWeekend := weekday == time.Saturday || weekday == time.Sunday
	var weekendHint string
	switch {
	case isWeekend && hour >= 9 && hour <= 11:
		weekendHint = fmt.Sprintf("今天是%s 周末，用户可以睡懒觉。", weekdayName)
	case isWeekend && hour >= 12 && hour <= 14:
		weekendHint = "周末中午，用户可能在享受慵懒时光。"
	case isWeekend && hour >= 18 && hour <= 21:
		weekendHint = "周末晚上，适合约会或宅家放松。"
	case !isWeekend && hour >= 7 && hour <= 9:
		weekendHint = fmt.Sprintf("今天是%s 工作日，用户可能要赶时间出门。", weekdayName)
	case !isWeekend && hour >= 17 && hour <= 19:
		weekendHint = "工作日傍晚，用户可能刚结束一天的工作比较疲惫。"
	}

	var sb strings.Builder
	sb.WriteString(textlib.ProactiveTimeAwareHeader)
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("当前精确时间：%s %s", weekdayName, timeStr))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("场景：%s", timeScenario))
	if weekendHint != "" {
		sb.WriteString("\n")
		sb.WriteString(weekendHint)
	}
	sb.WriteString("\n")
	sb.WriteString(textlib.ProactiveTimeAwareFooter)
	return sb.String()
}

func (s *service) buildProactiveRecentContext(conversationID, characterID string) string {
	if s.db == nil {
		return ""
	}

	var messages []chat.Message
	if err := s.db.Where("conversation_id = ?", conversationID).
		Order("sequence DESC").
		Limit(10).
		Find(&messages).Error; err != nil || len(messages) == 0 {
		if len(messages) == 0 {
			return textlib.ProactiveDefaultNoHistory
		}
		return ""
	}

	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	now := time.Now()

	var sb strings.Builder
	sb.WriteString(textlib.ProactiveRecentContextHeader)
	sb.WriteString("\n")

	var characterName string
	s.db.Table("characters").Select("name").Where("id = ?", characterID).Limit(1).Row().Scan(&characterName)
	if characterName == "" {
		characterName = "AI"
	}

	for _, msg := range messages {
		role := characterName
		if msg.Role == "user" {
			role = "用户"
		}
		msgTime, err := time.Parse("2006-01-02 15:04:05", msg.CreatedAt)
		var timeAgo string
		if err == nil {
			timeAgo = formatTimeAgo(now, msgTime)
		} else {
			timeAgo = "未知"
		}
		sb.WriteString(fmt.Sprintf("%s（%s前）: %s\n", role, timeAgo, msg.Content))
	}

	lastMsg := messages[len(messages)-1]
	lastMsgTime, _ := time.Parse("2006-01-02 15:04:05", lastMsg.CreatedAt)
	totalGapMs := now.Sub(lastMsgTime).Milliseconds()
	gapMinutes := totalGapMs / 60000
	gapSeconds := totalGapMs / 1000

	sb.WriteString("\n")
	sb.WriteString(textlib.ProactiveTimeInfoHeader)
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("当前精确时间：%s", now.Format("2006-01-02 15:04:05")))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("上一条消息时间距今：%s（精确值）", formatGapDuration(totalGapMs)))

	switch {
	case gapMinutes < 1:
		sb.WriteString(fmt.Sprintf("\n"+textlib.ProactiveGap0to1Min, gapSeconds))
	case gapMinutes < 5:
		sb.WriteString(fmt.Sprintf("\n"+textlib.ProactiveGap1to5Min, gapMinutes, gapSeconds%60))
	case gapMinutes < 15:
		sb.WriteString(fmt.Sprintf("\n"+textlib.ProactiveGap5to15Min, gapMinutes, gapSeconds%60))
	case gapMinutes < 60:
		sb.WriteString(fmt.Sprintf("\n"+textlib.ProactiveGap15to60Min, gapMinutes, gapSeconds%60))
	default:
		hours := gapMinutes / 60
		remainMins := gapMinutes % 60
		if hours >= 24 {
			days := hours / 24
			remainHours := hours % 24
			sb.WriteString(fmt.Sprintf("\n"+textlib.ProactiveGap24HPlus, days, remainHours, remainMins))
		} else {
			sb.WriteString(fmt.Sprintf("\n"+textlib.ProactiveGap60MinPlus, hours, remainMins, gapSeconds%60))
		}
	}

	if gapMinutes >= 10 {
		sb.WriteString("\n")
		sb.WriteString(textlib.ProactiveGapLongReminder)
	}

	var lastUserMsg *chat.Message
	var lastAiMsg *chat.Message
	for i := len(messages) - 1; i >= 0; i-- {
		if lastUserMsg == nil && messages[i].Role == "user" {
			lastUserMsg = &messages[i]
		}
		if lastAiMsg == nil && messages[i].Role != "user" {
			lastAiMsg = &messages[i]
		}
	}

	if lastUserMsg != nil && lastAiMsg != nil {
		sb.WriteString("\n\n")
		sb.WriteString(textlib.ProactiveImportantHeader)
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf(textlib.ProactiveUserLastSaid, lastUserMsg.Content))
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf(textlib.ProactiveAssistantLastSaid, lastAiMsg.Content))

		if strings.ContainsAny(lastUserMsg.Content, "?？吗呢什么怎么为什么多少") {
			sb.WriteString("\n")
			sb.WriteString(textlib.ProactiveQuestionReminder)
		}

		if len(messages) >= 4 {
			var userMessages []chat.Message
			for _, msg := range messages {
				if msg.Role == "user" {
					userMessages = append(userMessages, msg)
				}
			}
			if len(userMessages) >= 2 {
				prevTopic := userMessages[len(userMessages)-2].Content
				lastTopic := userMessages[len(userMessages)-1].Content
				sb.WriteString("\n")
				sb.WriteString(fmt.Sprintf(textlib.ProactiveTopicContinuity, prevTopic, lastTopic))
				sb.WriteString("\n")
				sb.WriteString(textlib.ProactiveEnsureConnect)
			}
		}
	}

	return sb.String()
}

func formatTimeAgo(now, msgTime time.Time) string {
	d := now.Sub(msgTime)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d秒", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d分钟", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d小时", int(d.Hours()))
	default:
		return fmt.Sprintf("%d天", int(d.Hours()/24))
	}
}

func formatGapDuration(totalGapMs int64) string {
	totalSeconds := totalGapMs / 1000
	if totalSeconds < 60 {
		return fmt.Sprintf("%d 秒", totalSeconds)
	}
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	if minutes < 60 {
		return fmt.Sprintf("%d 分 %d 秒", minutes, seconds)
	}
	hours := minutes / 60
	remainMins := minutes % 60
	if hours < 24 {
		return fmt.Sprintf("%d 小时 %d 分 %d 秒", hours, remainMins, seconds)
	}
	days := hours / 24
	remainHours := hours % 24
	return fmt.Sprintf("%d 天 %d 小时 %d 分 %d 秒", days, remainHours, remainMins, seconds)
}

func (s *service) resolveProactiveDeliveryScope(conversationID, channelSetting, characterID string) proactiveDeliveryScope {
	scope := proactiveDeliveryScope{channel: normalizeProactiveChannel(channelSetting)}
	var channel, peerID, userID string
	s.db.Table("conversations").Select("channel, peer_id, user_id").Where("id = ?", conversationID).Limit(1).Row().Scan(&channel, &peerID, &userID)
	if strings.TrimSpace(channel) != "" {
		scope.channel = normalizeProactiveChannel(channel)
	}
	scope.peerID = strings.TrimSpace(peerID)
	if userIDFromDB := strings.TrimSpace(userID); userIDFromDB != "" {
		scope.userID = userIDFromDB
	} else {
		if trimmedPeerID := strings.TrimSpace(peerID); trimmedPeerID != "" {
			scope.userID = trimmedPeerID
		} else {
			scope.userID = "character:" + strings.TrimSpace(characterID)
		}
	}
	if scope.channel == "" {
		scope.channel = "web"
	}
	if scope.channel == "all" {
		scope.channel = "web"
	}
	return scope
}

func normalizeProactiveChannel(channel string) string {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" {
		return "web"
	}
	if channel == "all" {
		return "web"
	}
	if strings.Contains(channel, "wechat") {
		return "wechat"
	}
	if strings.Contains(channel, "qq") {
		return "qq"
	}
	if strings.Contains(channel, "voice") {
		return "voice"
	}
	return channel
}

func proactiveRequestID(prefix string, id interface{}) string {
	return fmt.Sprintf("%s-%v", prefix, id)
}

func (s *service) DispatchProactiveMessage(ctx context.Context, characterID, conversationID, channel, prompt, requestID string) (string, error) {
	result, err := s.submitProactiveMessage(ctx, characterID, conversationID, channel, prompt, requestID)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}
	if result.Response != nil {
		return result.Response.Reply, nil
	}
	return "", nil
}

func (s *service) buildProactiveRelationshipContext(conversationID, characterID string) string {
	var relationType string
	var relationData string

	s.db.Table("relationship_states").Select("relation_type, relation_data").
		Where("character_id = ?", characterID).Limit(1).
		Row().Scan(&relationType, &relationData)

	if relationType == "" {
		relationType = "ACQUAINTANCE"
	}

	return fmt.Sprintf("关系类型：%s\n关系数据：%s", relationType, relationData)
}

func (s *service) buildProactiveEmotionContext(conversationID, characterID string) string {
	var emotionJSON string
	var moodJSON string
	var stress float64
	var energy float64

	s.db.Table("psyche_states").Select("emotion, mood, stress, energy").
		Where("character_id = ?", characterID).Limit(1).
		Row().Scan(&emotionJSON, &moodJSON, &stress, &energy)

	if emotionJSON == "" {
		emotionJSON = "{}"
	}
	if moodJSON == "" {
		moodJSON = "{}"
	}

	return fmt.Sprintf("情绪：%s\n心情：%s\n压力：%.0f\n精力：%.0f", emotionJSON, moodJSON, stress, energy)
}

func (s *service) buildProactiveMemoryContext(conversationID, characterID string) string {
	type memoryRow struct {
		Key   string
		Value string
	}
	var memories []memoryRow

	rows, err := s.db.Table("memories").Select("key, value").
		Where("character_id = ?", characterID).
		Order("updated_at DESC").
		Limit(3).
		Rows()
	if err != nil {
		return ""
	}
	defer rows.Close()

	for rows.Next() {
		var m memoryRow
		rows.Scan(&m.Key, &m.Value)
		memories = append(memories, m)
	}

	if len(memories) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("最近记忆（可轻点提到，不要机械复述）：\n")
	for _, m := range memories {
		sb.WriteString(fmt.Sprintf("- %s\n", m.Value))
	}
	return sb.String()
}
