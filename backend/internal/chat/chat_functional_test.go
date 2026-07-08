package chat

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func setupChatFunctionalTest(t *testing.T) (*gorm.DB, *service, string, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "chat_func.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	tables := []interface{}{&Conversation{}, &Message{}, &ModelConfig{}, &character.Character{}}
	if err := db.AutoMigrate(tables...); err != nil {
		t.Fatal(err)
	}

	ctx := app.NewAppContext(db, nil)
	repo := NewRepository(ctx)
	charRepo := character.NewRepository(ctx)

	charID := "char-func-" + uuid.New().String()[:8]
	convID := "conv-func-" + uuid.New().String()[:8]

	if err := db.Create(&character.Character{
		ID:                charID,
		Name:              "功能测试角色",
		Identity:          "一位功能测试助手",
		Status:            "enabled",
		Personality:       "测试",
		PersonalityConfig: `{"warmth":60,"initiative":40}`,
		ChatStyleConfig:   "{}",
		SceneRules:        "{}",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Conversation{
		ID:          convID,
		CharacterID: charID,
		Title:       "功能测试对话",
		Channel:     "web",
		Source:      "manual",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ModelConfig{
		Name: "func-model", APIType: "openai-compatible", BaseURL: "http://127.0.0.1",
		APIKey: "func", ModelName: "func-test", Temperature: 0.7, MaxTokens: 128, IsActive: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}

	var llm llmWithToolsFunc
	svc := &service{
		repo:     repo,
		charRepo: charRepo,
		db:       db,
		llmWithTools: func(ctx context.Context, cfg *ModelConfig, msgs []map[string]interface{}, tools []tool.Tool) (string, string, []map[string]interface{}, int, error) {
			return llm(ctx, cfg, msgs, tools)
		},
	}
	_ = llm
	return db, svc, charID, convID
}

func verifyMessagesInDB(t *testing.T, db *gorm.DB, convID string, requestID string, expectedLines []string) {
	t.Helper()
	var msgs []Message
	if err := db.Where("conversation_id = ? AND role = ? AND request_id = ?", convID, "assistant", requestID).
		Order("sequence ASC").Find(&msgs).Error; err != nil {
		t.Fatal(err)
	}
	if len(msgs) != len(expectedLines) {
		t.Fatalf("请求 %s: 期望 %d 条 assistant 消息, 实际 %d 条", requestID, len(expectedLines), len(msgs))
	}
	for i, msg := range msgs {
		if msg.Content != expectedLines[i] {
			t.Fatalf("请求 %s 第 %d 条消息: 期望=%q, 实际=%q", requestID, i+1, expectedLines[i], msg.Content)
		}
	}
}

func verifyUserMessageStatus(t *testing.T, db *gorm.DB, requestID, expectedStatus string) {
	t.Helper()
	var status string
	if err := db.Model(&Message{}).Where("role = ? AND request_id = ?", "user", requestID).
		Select("status").Row().Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != expectedStatus {
		t.Fatalf("请求 %s: 期望用户消息状态=%s, 实际=%s", requestID, expectedStatus, status)
	}
}

func verifyConversationMessageCount(t *testing.T, db *gorm.DB, convID string, expected int64) {
	t.Helper()
	var count int64
	if err := db.Model(&Message{}).Where("conversation_id = ?", convID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != expected {
		t.Fatalf("对话 %s: 期望消息总数=%d, 实际=%d", convID, expected, count)
	}
}

func TestChatFunctional_NormalMultiLineReply(t *testing.T) {
	t.Run("multi_line_web", func(t *testing.T) {
		db, svc, charID, convID := setupChatFunctionalTest(t)

		replyText := "这是第一句回复\n这是第二句回复\n这是第三句回复"

		svc.llmWithTools = func(ctx context.Context, _ *ModelConfig, _ []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
			return replyText, "", nil, 30, nil
		}

		reqID := "req-multi-web"
		resp, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
			CharacterID:    charID,
			ConversationID: convID,
			Channel:        "web",
			Source:         "manual",
			Message:        "你好",
			RequestID:      reqID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Reply != replyText {
			t.Fatalf("Reply不匹配: 期望=%q, 实际=%q", replyText, resp.Reply)
		}
		expectedLines := []string{"这是第一句回复", "这是第二句回复", "这是第三句回复"}
		if len(resp.MessageIDs) != len(expectedLines) {
			t.Fatalf("MessageIDs数量不匹配: 期望=%d, 实际=%d", len(expectedLines), len(resp.MessageIDs))
		}

		verifyMessagesInDB(t, db, convID, reqID, expectedLines)
		verifyUserMessageStatus(t, db, reqID, "sent")
		verifyConversationMessageCount(t, db, convID, 4)
	})
}

func TestChatFunctional_SingleLineReply(t *testing.T) {
	t.Run("single_line", func(t *testing.T) {
		db, svc, charID, convID := setupChatFunctionalTest(t)

		svc.llmWithTools = func(ctx context.Context, _ *ModelConfig, _ []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
			return "只有一句话", "", nil, 5, nil
		}

		reqID := "req-single"
		resp, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
			CharacterID:    charID,
			ConversationID: convID,
			Channel:        "web",
			Source:         "manual",
			Message:        "你好",
			RequestID:      reqID,
		})
		if err != nil {
			t.Fatal(err)
		}
		expectedLines := []string{"只有一句话"}
		if len(resp.MessageIDs) != 1 {
			t.Fatalf("期望1条消息, 实际=%d", len(resp.MessageIDs))
		}

		verifyMessagesInDB(t, db, convID, reqID, expectedLines)
		verifyUserMessageStatus(t, db, reqID, "sent")
		verifyConversationMessageCount(t, db, convID, 2)
	})
}

func TestChatFunctional_LongMultiLineSplit(t *testing.T) {
	t.Run("long_multi_line", func(t *testing.T) {
		db, svc, charID, convID := setupChatFunctionalTest(t)

		var sb strings.Builder
		for i := 0; i < 10; i++ {
			sb.WriteString(fmt.Sprintf("这是一句需要拆分的长文本内容[第%d行]", i))
			sb.WriteString("\n")
		}
		replyText := strings.TrimSuffix(sb.String(), "\n")

		svc.llmWithTools = func(ctx context.Context, _ *ModelConfig, _ []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
			return replyText, "", nil, 200, nil
		}

		reqID := "req-long-multi"
		resp, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
			CharacterID:    charID,
			ConversationID: convID,
			Channel:        "web",
			Source:         "manual",
			Message:        "长文本",
			RequestID:      reqID,
		})
		if err != nil {
			t.Fatal(err)
		}

		var msgs []Message
		if err := db.Where("conversation_id = ? AND role = ? AND request_id = ?", convID, "assistant", reqID).
			Order("sequence ASC").Find(&msgs).Error; err != nil {
			t.Fatal(err)
		}
		if len(msgs) < 2 {
			t.Fatalf("期望至少2条消息, 实际=%d", len(msgs))
		}

		for _, msg := range msgs {
			runeCount := len([]rune(msg.Content))
			if runeCount > 2000 {
				t.Fatalf("拆分后单条消息超过2000个字符: %d", runeCount)
			}
		}

		if resp.Reply != replyText {
			t.Fatalf("Reply被修改")
		}

		t.Logf("多行拆分: %d条消息 (ApplyPostValidation截断后)", len(msgs))
	})
}

func TestChatFunctional_TenRoundConsistency(t *testing.T) {
	t.Run("ten_round", func(t *testing.T) {
		db, svc, charID, _ := setupChatFunctionalTest(t)

		convWeb := "conv-ten-web-" + uuid.New().String()[:8]
		convWechat := "conv-ten-wechat-" + uuid.New().String()[:8]
		convQQ := "conv-ten-qq-" + uuid.New().String()[:8]

		for _, c := range []struct{ id, channel string }{
			{convWeb, "web"},
			{convWechat, "wechat"},
			{convQQ, "qq"},
		} {
			if err := db.Create(&Conversation{
				ID:          c.id,
				CharacterID: charID,
				Title:       c.channel + "十轮测试",
				Channel:     c.channel,
				Source:      "manual",
			}).Error; err != nil {
				t.Fatal(err)
			}
		}

		type roundSpec struct {
			label   string
			reply   string
			channel string
			convID  string
		}
		rounds := []roundSpec{
			{"简单招呼", "你好呀\n今天天气不错", "web", convWeb},
			{"多句回复", "嗯，这个问题很有意思\n让我想想\n大概是这样\n你觉得呢", "web", convWeb},
			{"单句回复", "没问题！", "web", convWeb},
			{"多行中等长度", "文本行A\n文本行B\n文本行C\n文本行D\n文本行E", "web", convWeb},
			{"微信渠道", "收到你的消息了\n马上处理", "wechat", convWechat},
			{"QQ渠道", "好\n行\nok", "qq", convQQ},
			{"带空行", "第一句\n第二句\n第三句", "web", convWeb},
			{"Emoji混合", "好的😊\n明白了👌\n稍等", "wechat", convWechat},
			{"纯换行", "只有\n换行\n分隔", "web", convWeb},
			{"最终轮", "测试即将完成\n一切正常", "web", convWeb},
		}

		var totalAssistantMsgs, totalUserMsgs int64

		for i, round := range rounds {
			roundReply := round.reply
			roundChannel := round.channel
			roundConvID := round.convID
			svc.llmWithTools = func(ctx context.Context, _ *ModelConfig, _ []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
				return roundReply, "", nil, len([]rune(roundReply)) / 2, nil
			}

			reqID := fmt.Sprintf("req-round-%d", i+1)
			resp, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
				CharacterID:    charID,
				ConversationID: roundConvID,
				Channel:        roundChannel,
				Source:         "manual",
				Message:        fmt.Sprintf("第%d轮: %s", i+1, round.label),
				RequestID:      reqID,
			})
			if err != nil {
				t.Fatalf("第%d轮(%s)失败: %v", i+1, round.label, err)
			}
			if resp.Reply != roundReply {
				t.Fatalf("第%d轮(%s) Reply不匹配: 期望=%q, 实际=%q", i+1, round.label, roundReply, resp.Reply)
			}

			var assistantMsgs []Message
			if err := db.Where("conversation_id = ? AND role = ? AND request_id = ?", roundConvID, "assistant", reqID).
				Order("sequence ASC").Find(&assistantMsgs).Error; err != nil {
				t.Fatal(err)
			}
			if len(assistantMsgs) == 0 {
				t.Fatalf("第%d轮(%s): 没有assistant消息", i+1, round.label)
			}
			if len(resp.MessageIDs) != len(assistantMsgs) {
				t.Fatalf("第%d轮(%s): MessageIDs数量(%d)与DB消息数(%d)不匹配", i+1, round.label, len(resp.MessageIDs), len(assistantMsgs))
			}

			totalAssistantMsgs += int64(len(assistantMsgs))
			totalUserMsgs++
			t.Logf("第%d轮(%s/%s): 拆分为%d条, 回复长度=%d字符", i+1, round.label, roundChannel, len(assistantMsgs), len([]rune(roundReply)))
		}

		for _, cid := range []string{convWeb, convWechat, convQQ} {
			var count int64
			if err := db.Model(&Message{}).Where("conversation_id = ?", cid).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			var userCount int64
			if err := db.Model(&Message{}).Where("conversation_id = ? AND role = ?", cid, "user").Count(&userCount).Error; err != nil {
				t.Fatal(err)
			}
			t.Logf("对话 %s: 总消息=%d, 用户消息=%d", cid, count, userCount)
		}

		t.Logf("10轮测试通过: 总assistant=%d, 总user=%d", totalAssistantMsgs, totalUserMsgs)
	})
}

func TestChatFunctional_IdempotentRequest(t *testing.T) {
	t.Run("idempotent", func(t *testing.T) {
		_, svc, charID, convID := setupChatFunctionalTest(t)

		callCount := 0
		svc.llmWithTools = func(ctx context.Context, _ *ModelConfig, _ []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
			callCount++
			return "第一句\n第二句", "", nil, 10, nil
		}

		reqID := "req-idempotent"
		resp1, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
			CharacterID:    charID,
			ConversationID: convID,
			Channel:        "web",
			Source:         "manual",
			Message:        "测试幂等",
			RequestID:      reqID,
		})
		if err != nil {
			t.Fatal(err)
		}

		resp2, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
			CharacterID:    charID,
			ConversationID: convID,
			Channel:        "web",
			Source:         "manual",
			Message:        "测试幂等",
			RequestID:      reqID,
		})
		if err != nil {
			t.Fatal(err)
		}

		if resp2.Sequence != resp1.Sequence {
			t.Fatalf("幂等请求返回不同Sequence: %d != %d", resp1.Sequence, resp2.Sequence)
		}
		if callCount > 2 {
			t.Fatalf("LLM调用次数异常: 期望<=2, 实际=%d", callCount)
		}
	})
}

func TestChatFunctional_ChannelSpecificSplit(t *testing.T) {
	t.Run("wechat_split", func(t *testing.T) {
		db, svc, charID, _ := setupChatFunctionalTest(t)

		convWechat := "conv-wechat-test"
		db.Create(&Conversation{
			ID:          convWechat,
			CharacterID: charID,
			Title:       "微信对话",
			Channel:     "wechat",
			Source:      "sidecar",
		})

		replyText := "微信消息1\n微信消息2\n微信消息3"

		svc.llmWithTools = func(ctx context.Context, _ *ModelConfig, _ []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
			return replyText, "", nil, 15, nil
		}

		reqID := "req-wechat"
		resp, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
			CharacterID:    charID,
			ConversationID: convWechat,
			Channel:        "wechat",
			Source:         "sidecar",
			Message:        "微信测试",
			RequestID:      reqID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.MessageIDs) != 3 {
			t.Fatalf("微信渠道期望3条消息, 实际=%d", len(resp.MessageIDs))
		}
	})

	t.Run("qq_split", func(t *testing.T) {
		db, svc, charID, _ := setupChatFunctionalTest(t)

		convQQ := "conv-qq-test"
		db.Create(&Conversation{
			ID:          convQQ,
			CharacterID: charID,
			Title:       "QQ对话",
			Channel:     "qq",
			Source:      "sidecar",
		})

		replyText := "QQ消息1\nQQ消息2\nQQ消息3\nQQ消息4"

		svc.llmWithTools = func(ctx context.Context, _ *ModelConfig, _ []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
			return replyText, "", nil, 20, nil
		}

		reqID := "req-qq"
		resp, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
			CharacterID:    charID,
			ConversationID: convQQ,
			Channel:        "qq",
			Source:         "sidecar",
			Message:        "QQ测试",
			RequestID:      reqID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.MessageIDs) != 4 {
			t.Fatalf("QQ渠道期望4条消息, 实际=%d", len(resp.MessageIDs))
		}
	})
}

func TestChatFunctional_EmptyLinesFiltered(t *testing.T) {
	t.Run("empty_lines", func(t *testing.T) {
		db, svc, charID, convID := setupChatFunctionalTest(t)

		replyText := "  \n\n第一句\n\n  \n第二句\n  \n"

		svc.llmWithTools = func(ctx context.Context, _ *ModelConfig, _ []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
			return replyText, "", nil, 10, nil
		}

		reqID := "req-empty"
		resp, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
			CharacterID:    charID,
			ConversationID: convID,
			Channel:        "web",
			Source:         "manual",
			Message:        "空行测试",
			RequestID:      reqID,
		})
		if err != nil {
			t.Fatal(err)
		}
		expectedLines := []string{"第一句", "第二句"}
		if len(resp.MessageIDs) != len(expectedLines) {
			t.Fatalf("期望 %d 条消息(空行被过滤), 实际=%d", len(expectedLines), len(resp.MessageIDs))
		}
		verifyMessagesInDB(t, db, convID, reqID, expectedLines)
	})
}

func TestChatFunctional_MessageSequenceMonotonic(t *testing.T) {
	t.Run("sequence_monotonic", func(t *testing.T) {
		db, svc, charID, convID := setupChatFunctionalTest(t)

		replies := []string{
			"第一轮回复",
			"第二轮回复A\n第二轮回复B",
			"第三轮回复",
			"第四轮回复A\n第四轮回复B\n第四轮回复C",
			"第五轮回复",
		}

		var lastSeq int64 = 0
		for i, reply := range replies {
			r := reply
			svc.llmWithTools = func(ctx context.Context, _ *ModelConfig, _ []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
				return r, "", nil, 10, nil
			}

			reqID := fmt.Sprintf("req-seq-%d", i+1)
			resp, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
				CharacterID:    charID,
				ConversationID: convID,
				Channel:        "web",
				Source:         "manual",
				Message:        fmt.Sprintf("第%d轮", i+1),
				RequestID:      reqID,
			})
			if err != nil {
				t.Fatalf("第%d轮失败: %v", i+1, err)
			}
			if resp.Sequence <= lastSeq {
				t.Fatalf("Sequence非单调递增: %d <= %d (第%d轮)", resp.Sequence, lastSeq, i+1)
			}
			lastSeq = resp.Sequence
		}

		var allMsgs []Message
		if err := db.Where("conversation_id = ?", convID).Order("sequence ASC").Find(&allMsgs).Error; err != nil {
			t.Fatal(err)
		}
		for i := 1; i < len(allMsgs); i++ {
			if allMsgs[i].Sequence <= allMsgs[i-1].Sequence {
				t.Fatalf("DB中Sequence非单调: 第%d条=%d, 第%d条=%d", i, allMsgs[i-1].Sequence, i+1, allMsgs[i].Sequence)
			}
		}
		t.Logf("Sequence单调性验证通过: 共%d条消息, 最大Sequence=%d", len(allMsgs), lastSeq)
	})
}

func TestChatFunctional_ReplyContainsComputedMetadata(t *testing.T) {
	t.Run("metadata", func(t *testing.T) {
		db, svc, charID, convID := setupChatFunctionalTest(t)

		svc.llmWithTools = func(ctx context.Context, _ *ModelConfig, _ []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
			return "AI回复内容", "", nil, 6, nil
		}

		reqID := "req-meta"
		resp, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
			CharacterID:    charID,
			ConversationID: convID,
			Channel:        "web",
			Source:         "manual",
			Message:        "元数据测试",
			RequestID:      reqID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if resp.ConversationID != convID {
			t.Fatalf("ConversationID不匹配: 期望=%s, 实际=%s", convID, resp.ConversationID)
		}
		if resp.CharacterID != charID {
			t.Fatalf("CharacterID不匹配: 期望=%s, 实际=%s", charID, resp.CharacterID)
		}
		if resp.Reply != "AI回复内容" {
			t.Fatalf("Reply不匹配")
		}
		if len(resp.MessageIDs) != 1 {
			t.Fatalf("期望1条消息, 实际=%d", len(resp.MessageIDs))
		}

		var msg Message
		if err := db.Where("id = ?", resp.MessageIDs[0]).First(&msg).Error; err != nil {
			t.Fatal(err)
		}
		if msg.Role != "assistant" {
			t.Fatalf("消息角色错误: %s", msg.Role)
		}
		if msg.MsgType != "text" {
			t.Fatalf("消息类型错误: %s", msg.MsgType)
		}
	})
}

func TestChatFunctional_SequentialRequests(t *testing.T) {
	t.Run("sequential", func(t *testing.T) {
		_, svc, charID, convID := setupChatFunctionalTest(t)

		callCount := 0
		var mu sync.Mutex
		svc.llmWithTools = func(ctx context.Context, _ *ModelConfig, _ []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
			mu.Lock()
			callCount++
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			return "并发回复", "", nil, 4, nil
		}

		for i := 0; i < 3; i++ {
			reqID := fmt.Sprintf("req-seq-concurrent-%d", i)
			resp, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
				CharacterID:    charID,
				ConversationID: convID,
				Channel:        "web",
				Source:         "manual",
				Message:        fmt.Sprintf("顺序%d", i),
				RequestID:      reqID,
			})
			if err != nil {
				t.Fatalf("顺序请求 %s 失败: %v", reqID, err)
			}
			if resp.Reply != "并发回复" {
				t.Fatalf("顺序请求 %s Reply不匹配", reqID)
			}
		}
		if callCount != 3 {
			t.Fatalf("期望3次LLM调用, 实际=%d", callCount)
		}
	})
}

func TestChatFunctional_ComputeInteractionOnlySplitsOnce(t *testing.T) {
	t.Run("split_once", func(t *testing.T) {
		_, svc, charID, convID := setupChatFunctionalTest(t)

		replyText := "拆分测试1\n拆分测试2\n拆分测试3"
		svc.llmWithTools = func(ctx context.Context, _ *ModelConfig, _ []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
			return replyText, "", nil, 15, nil
		}

		computeResult, err := svc.ComputeInteraction(context.Background(), &ProcessMessageRequest{
			CharacterID:    charID,
			ConversationID: convID,
			Channel:        "web",
			Source:         "manual",
			Message:        "拆分测试",
			RequestID:      "req-split-once",
		})
		if err != nil {
			t.Fatal(err)
		}

		expectedLines := []string{"拆分测试1", "拆分测试2", "拆分测试3"}
		if len(computeResult.Lines) != 3 {
			t.Fatalf("ComputeInteraction拆分结果数量不对: 期望3, 实际=%d", len(computeResult.Lines))
		}
		for i, line := range computeResult.Lines {
			if line != expectedLines[i] {
				t.Fatalf("ComputeInteraction第%d条不匹配: 期望=%q, 实际=%q", i+1, expectedLines[i], line)
			}
		}
		if computeResult.Reply != replyText {
			t.Fatalf("原始Reply被修改: 期望=%q, 实际=%q", replyText, computeResult.Reply)
		}
		t.Logf("拆分点唯一验证通过: Lines=%v, Reply=%q", computeResult.Lines, computeResult.Reply)
	})
}

func setupChatFunctionalTestWithCapture(t *testing.T, personalityCfg string, capture *[]map[string]interface{}, reply string) (*gorm.DB, *service, string, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "chat-flag.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&Conversation{}, &Message{}, &ModelConfig{}, &character.Character{}); err != nil {
		t.Fatal(err)
	}
	ctx := app.NewAppContext(db, nil)
	repo := NewRepository(ctx)
	charRepo := character.NewRepository(ctx)
	charID := "char-cap-" + t.Name()
	convID := "conv-cap-" + t.Name()
	if err := db.Create(&character.Character{
		ID: charID, Name: "测试角色", Identity: "一位伙伴",
		Status: "enabled", PersonalityConfig: personalityCfg,
		ChatStyleConfig: "{}", SceneRules: "{}",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Conversation{ID: convID, CharacterID: charID, Title: "capture", Channel: "web", Source: "manual"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ModelConfig{
		Name: "cap-model", APIType: "openai-compatible", BaseURL: "http://127.0.0.1",
		APIKey: "cap", ModelName: "cap", Temperature: 0.7, MaxTokens: 128, IsActive: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	llm := func(_ context.Context, _ *ModelConfig, messages []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		*capture = messages
		if reply == "" {
			reply = "收到"
		}
		return reply, "", nil, len(reply) / 3, nil
	}
	svc := &service{repo: repo, charRepo: charRepo, db: db, llmWithTools: llm}
	return db, svc, charID, convID
}

func TestChatFunctional_FeatureFlagPersonalityDisabled(t *testing.T) {
	t.Run("personality_flag_off", func(t *testing.T) {
		var capturedMessages []map[string]interface{}
		_, svc, charID, convID := setupChatFunctionalTestWithCapture(t, `{"warmth":70}`, &capturedMessages, "测试回复")

		if config.AppCfg == nil {
			config.AppCfg = &config.Config{}
		}
		config.AppCfg.Prompt.PersonalityRawEnabled = false
		t.Cleanup(func() { config.AppCfg.Prompt.PersonalityRawEnabled = true })

		_, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
			CharacterID:    charID,
			ConversationID: convID,
			Channel:        "web",
			Source:         "manual",
			Message:        "你好",
			RequestID:      "req-personality-off",
		})
		if err != nil {
			t.Fatal(err)
		}

		for _, msg := range capturedMessages {
			if content, ok := msg["content"].(string); ok {
				if strings.Contains(content, "<personality_raw") {
					t.Error("PersonalityRawEnabled=false 时 prompt 不应包含 <personality_raw>")
				}
			}
		}
	})
}

func TestChatFunctional_FeatureFlagEmotionFusionDisabled(t *testing.T) {
	t.Run("emotion_fusion_flag_off", func(t *testing.T) {
		var capturedMessages []map[string]interface{}
		_, svc, charID, convID := setupChatFunctionalTestWithCapture(t, `{"warmth":70}`, &capturedMessages, "测试回复")

		if config.AppCfg == nil {
			config.AppCfg = &config.Config{}
		}
		config.AppCfg.Prompt.EmotionFusionEnabled = false
		t.Cleanup(func() { config.AppCfg.Prompt.EmotionFusionEnabled = true })

		_, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
			CharacterID:    charID,
			ConversationID: convID,
			Channel:        "web",
			Source:         "manual",
			Message:        "你好",
			RequestID:      "req-emotion-off",
		})
		if err != nil {
			t.Fatal(err)
		}

		for _, msg := range capturedMessages {
			if content, ok := msg["content"].(string); ok {
				if strings.Contains(content, "<emotion_fusion_raw") {
					t.Error("EmotionFusionEnabled=false 时 prompt 不应包含 <emotion_fusion_raw>")
				}
			}
		}
	})
}

func TestChatFunctional_FeatureFlagReplySanitizerDisabled(t *testing.T) {
	t.Run("reply_sanitizer_flag_off", func(t *testing.T) {
		var capturedMessages []map[string]interface{}
		_, svc, charID, convID := setupChatFunctionalTestWithCapture(t, `{"warmth":70}`, &capturedMessages, "测试回复")

		if config.AppCfg == nil {
			config.AppCfg = &config.Config{}
		}
		config.AppCfg.Prompt.ReplySanitizerEnabled = false
		t.Cleanup(func() { config.AppCfg.Prompt.ReplySanitizerEnabled = true })

		_, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
			CharacterID:    charID,
			ConversationID: convID,
			Channel:        "web",
			Source:         "manual",
			Message:        "你好",
			RequestID:      "req-sanitizer-off",
		})
		if err != nil {
			t.Fatal(err)
		}

		for _, msg := range capturedMessages {
			if content, ok := msg["content"].(string); ok {
				if strings.Contains(content, "<output_shape_raw") {
					t.Error("ReplySanitizerEnabled=false 时 prompt 不应包含 <output_shape_raw>")
				}
				if strings.Contains(content, "<anti_repeat_raw") {
					t.Error("ReplySanitizerEnabled=false 时 prompt 不应包含 <anti_repeat_raw>")
				}
			}
		}
	})
}

func TestChatFunctional_FeatureFlagAllFlagsOff(t *testing.T) {
	t.Run("all_flags_off", func(t *testing.T) {
		var capturedMessages []map[string]interface{}
		_, svc, charID, convID := setupChatFunctionalTestWithCapture(t, `{"warmth":70}`, &capturedMessages, "测试回复")

		if config.AppCfg == nil {
			config.AppCfg = &config.Config{}
		}
		config.AppCfg.Prompt = config.PromptFeatureFlags{}
		t.Cleanup(func() {
			config.AppCfg.Prompt = config.PromptFeatureFlags{
				TextlibRawEnabled: true, PersonalityRawEnabled: true, EmotionFusionEnabled: true,
				IntimacyDefaultEnabled: true, MemoryRawEnabled: true,
				ReplySanitizerEnabled: true, ProactiveRawEnabled: true,
			}
		})

		_, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
			CharacterID:    charID,
			ConversationID: convID,
			Channel:        "web",
			Source:         "manual",
			Message:        "你好",
			RequestID:      "req-all-off",
		})
		if err != nil {
			t.Fatal(err)
		}

		for _, msg := range capturedMessages {
			if content, ok := msg["content"].(string); ok {
				forbiddenTags := []string{"<personality_raw", "<emotion_fusion_raw", "<adult_intimacy_raw", "<memory_inject_raw", "<output_shape_raw", "<anti_repeat_raw", "<proactive_scene", "<channel_short_raw"}
				for _, tag := range forbiddenTags {
					if strings.Contains(content, tag) {
						t.Errorf("全部关闭后 prompt 不应包含 %s", tag)
					}
				}
			}
		}

		hasSystem := false
		for _, msg := range capturedMessages {
			if role, ok := msg["role"].(string); ok && role == "system" {
				hasSystem = true
				break
			}
		}
		if !hasSystem {
			t.Error("全部关闭后仍应有 system 消息（旧链路可运行）")
		}
	})
}
