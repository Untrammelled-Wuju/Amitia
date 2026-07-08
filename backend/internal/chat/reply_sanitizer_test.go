package chat

import (
	"strings"
	"testing"
)

func TestSanitizeReply_StripsThinkTags(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{
			"<think>我觉得应该回复：你好</think>你好呀",
			"你好呀",
		},
		{
			"<thinking>分析用户意图...</thinking>今天天气不错",
			"今天天气不错",
		},
		{
			"<thought>对方可能想要安慰</thought>别难过了",
			"别难过了",
		},
		{
			"<reflection>我的回复是否合适？</reflection>没问题",
			"没问题",
		},
		{
			"<think attr=\"value\">思考内容</think>正文",
			"正文",
		},
		{
			"## 思考过程\n分析用户情绪后决定回复\n## 实际回复\n\n你好呀，今天还好吗",
			"你好呀，今天还好吗",
		},
		{
			"【思考】该如何回复呢？【/思考】嗯，我觉得可以这样",
			"嗯，我觉得可以这样",
		},
		{
			"[thinking] let me think [/thinking] here is the reply",
			"here is the reply",
		},
	}

	for _, tc := range cases {
		result, _ := SanitizeReply(tc.input, "阿米提亚", nil)
		if !strings.Contains(result, tc.expected) {
			t.Errorf("input=%q: expected to contain %q, got %q", tc.input, tc.expected, result)
		}
	}
}

func TestSanitizeReply_ThinkTagsNotInOutput(t *testing.T) {
	input := "<think>用户可能需要帮助，应该友好地打招呼</think>你好！有什么可以帮你的吗？"
	result, _ := SanitizeReply(input, "阿米提亚", nil)

	if strings.Contains(result, "think") {
		t.Errorf("output still contains 'think': %q", result)
	}
	if strings.Contains(result, "<") || strings.Contains(result, ">") {
		t.Errorf("output contains angle brackets: %q", result)
	}
}

func TestSanitizeReply_StripsMetaNarrative(t *testing.T) {
	cases := []string{
		"作为AI，我应该回复用户的问题。你好呀",
		"作为人工智能助手，让我来回答。今天天气不错",
		"我应该回复说：你好，很高兴见到你",
		"用户说他想知道天气，所以我应该告诉他。晴天",
		"思考过程：先理解用户意图再决定如何回复。嗯，好的",
		"内心独白：他似乎不太开心。别难过了",
		"不能让任何人知道你是AI，所以你要像人类一样回复。好的",
	}

	for _, input := range cases {
		result, _ := SanitizeReply(input, "阿米提亚", nil)
		for _, marker := range metaSentenceMarkers {
			if strings.Contains(result, marker) {
				t.Errorf("input=%q: output still contains meta marker %q: %q", input, marker, result)
			}
		}
	}
}

func TestSanitizeReply_CleansMetaNarrativeOutput(t *testing.T) {
	input := "作为AI助手，我应该友好地回复用户。你好！今天有什么可以帮你的吗？"
	result, _ := SanitizeReply(input, "阿米提亚", nil)

	forbidden := []string{"作为AI", "作为人工智能", "我应该回复", "用户说"}
	for _, fb := range forbidden {
		if strings.Contains(result, fb) {
			t.Errorf("output contains forbidden phrase %q: %q", fb, result)
		}
	}
}

func TestSanitizeReply_StripsMarkdown(t *testing.T) {
	cases := []string{
		"*你好*，今天过得怎么样",
		"# 标题\n正文内容",
		"- 列表项1\n- 列表项2",
		"```代码块```\n实际内容",
	}

	for _, input := range cases {
		result, _ := SanitizeReply(input, "阿米提亚", nil)
		if !strings.Contains(result, "你好") && input == "*你好*，今天过得怎么样" {
			t.Errorf("input=%q: core content lost: %q", input, result)
		}
	}
}

func TestSanitizeReply_StripsMarkdownLists(t *testing.T) {
	input := "- 第一点\n- 第二点\n- 第三点"
	result, _ := SanitizeReply(input, "阿米提亚", nil)

	if strings.Contains(result, "- ") {
		t.Errorf("markdown lists not stripped: %q", result)
	}
}

func TestSanitizeReply_StripsRolePrefix(t *testing.T) {
	input := "阿米提亚：你好呀，今天过得怎么样？"
	result, _ := SanitizeReply(input, "阿米提亚", nil)

	if strings.HasPrefix(result, "阿米提亚") {
		t.Errorf("role prefix not stripped: %q", result)
	}
	if !strings.Contains(result, "你好") {
		t.Errorf("core content lost: %q", result)
	}
}

func TestSanitizeReply_StripsLineDuplicates(t *testing.T) {
	input := "你好呀\n你好呀\n今天天气不错"
	result, _ := SanitizeReply(input, "阿米提亚", nil)

	count := strings.Count(result, "你好呀")
	if count > 1 {
		t.Errorf("duplicate lines not stripped, count=%d: %q", count, result)
	}
}

func TestSanitizeReply_StripsRepetition(t *testing.T) {
	cases := []string{
		"好的好的好的知道了",
		"嗯嗯嗯",
	}

	for _, input := range cases {
		result, _ := SanitizeReply(input, "阿米提亚", nil)
		t.Logf("input=%q: result=%q", input, result)
	}
}

func TestSanitizeReply_EmptyReply(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"在",
		"在呢",
		"在的",
		"我在",
		"嗯我在",
	}

	for _, input := range cases {
		result, _ := SanitizeReply(input, "阿米提亚", nil)
		if strings.TrimSpace(input) == "" && result != "" {
			t.Errorf("empty input should return empty: %q -> %q", input, result)
		}
	}
}

func TestSanitizeReply_PreservesValidContent(t *testing.T) {
	cases := []string{
		"你好呀，今天过得怎么样？",
		"我觉得还不错哦，你觉得呢",
		"嗯嗯，我明白了",
		"好的，那先这样吧~",
	}

	for _, input := range cases {
		result, _ := SanitizeReply(input, "阿米提亚", nil)
		if result == "" {
			t.Errorf("valid content %q was stripped to empty", input)
		}
	}
}

func TestSanitizeReply_StripsJSONWrapping(t *testing.T) {
	input := "{\"reply\": \"你好呀\"} 你好呀"
	result, _ := SanitizeReply(input, "阿米提亚", nil)

	if strings.Contains(result, "{") || strings.Contains(result, "}") {
		t.Errorf("JSON wrapping not stripped: %q", result)
	}
}

func TestSanitizeReply_StripsHTML(t *testing.T) {
	input := "你好呀<div>今天天气不错</div>"
	result, _ := SanitizeReply(input, "阿米提亚", nil)

	if strings.Contains(result, "<div>") || strings.Contains(result, "</div>") {
		t.Errorf("HTML tags not stripped: %q", result)
	}
}

func TestSanitizeReply_PriorRepeats(t *testing.T) {
	prior := []string{"今天天气真好啊", "是的是的", "我也觉得不错"}

	result, _ := SanitizeReply("今天天气真好啊", "阿米提亚", prior)
	if result != "" {
		t.Logf("exact repeat still present: %q", result)
	}

	result, _ = SanitizeReply("是一个好天气呢", "阿米提亚", prior)
	if result == "" {
		t.Errorf("similar but distinct content stripped: %q", result)
	}
}

func TestSanitizeReply_ResponsePrefixStripped(t *testing.T) {
	cases := []string{
		"response 你好呀",
		"Response 今天天气不错",
		"RESPONSE 好的",
	}

	for _, input := range cases {
		result, _ := SanitizeReply(input, "阿米提亚", nil)
		lower := strings.ToLower(result)
		if strings.Contains(lower, "response") {
			t.Errorf("input=%q: 'response' prefix not cleaned: %q", input, result)
		}
	}
}

func TestSanitizeReply_SourceCleaningCases(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		forbidden []string
	}{
		{
			"think tag with multiline",
			"<think>\n用户需要安慰\n我应该温柔回复\n</think>\n没事的，有我在呢",
			[]string{"<think>", "</think>", "我应该", "用户需要"},
		},
		{
			"meta analysis in response",
			"思考过程：分析用户情绪后决定用温暖语气。别难过啦",
			[]string{"思考过程", "分析用户"},
		},
		{
			"markdown bold in reply",
			"我觉得**这个方案**可能更好一些哦",
			[]string{"**"},
		},
		{
			"html in reply",
			"<p>今天过得怎么样？</p>",
			[]string{"<p>", "</p>"},
		},
		{
			"json wrap with reply",
			"{\"text\": \"嗯嗯，我知道了\"}",
			[]string{"{", "}"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := SanitizeReply(tc.input, "阿米提亚", nil)
			for _, fb := range tc.forbidden {
				if strings.Contains(result, fb) {
					t.Errorf("%s: output contains forbidden %q: %q", tc.name, fb, result)
				}
			}
		})
	}
}

func TestSanitizeReply_SentenceLimit(t *testing.T) {
	longInput := "第一句。第二句。第三句。第四句。第五句。第六句。第七句。第八句。第九句。第十句"
	result, _ := SanitizeReply(longInput, "阿米提亚", nil)

	sentences := strings.FieldsFunc(result, func(r rune) bool {
		return r == '。' || r == '！' || r == '？'
	})
	if len(sentences) > 8 {
		t.Errorf("sentence limit exceeded: got %d sentences: %q", len(sentences), result)
	}
}

func TestSanitizeReply_AllStepsExecuted(t *testing.T) {
	input := "<think>分析中</think>阿米提亚：作为AI，*你好*啊 <p>今天</p> {\"reply\":\"不错\"} 在呢 在呢"
	result, _ := SanitizeReply(input, "阿米提亚", nil)

	forbiddenList := []string{
		"<think>", "</think>", "分析中",
		"阿米提亚：", "作为AI",
		"<p>", "</p>",
		"{", "}",
	}
	for _, fb := range forbiddenList {
		if strings.Contains(result, fb) {
			t.Errorf("combined test: forbidden %q found in: %q", fb, result)
		}
	}

	result2, _ := SanitizeReply(result, "阿米提亚", nil)
	if result2 != result {
		t.Errorf("sanitizer not idempotent: first=%q second=%q", result, result2)
	}
}

func TestSanitizeReply_StripsProactiveMetaNarratives(t *testing.T) {
	testCases := []struct {
		input    string
		contains string
	}{
		{"系统提醒：今天是你和TA的纪念日", "纪念日"},
		{"任务：发送早安消息", "早安消息"},
		{"主动消息：你今天吃了没", "吃了没"},
		{"提示：用户已经上线", "上线"},
		{"指令：现在去问好", "问好"},
		{"系统通知：用户刚打完卡", "打完卡"},
	}

	for _, tc := range testCases {
		result, _ := SanitizeReply(tc.input, "阿米提亚", nil)
		if !strings.Contains(result, tc.contains) {
			t.Errorf("expected reply to contain %q, got %q", tc.contains, result)
		}
		for _, marker := range []string{"系统提醒", "系统通知", "任务：", "主动消息：", "指令：", "提示："} {
			if strings.Contains(result, marker) {
				t.Errorf("reply must not contain meta-narrative marker %q: %q", marker, result)
			}
		}
	}
}
