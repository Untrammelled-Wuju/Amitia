package chat

import (
	"fmt"
	"strings"
)

const bubbleMessageInstruction = "【本轮聊天界面：气泡消息】\n使用聊天消息方式组织回复。需要发送多条独立文本消息时，在消息之间插入精确分隔符 [AMITIA_BR]，客户端会将每段作为独立气泡发送；单条消息不需要分隔符。不要在开头、结尾或连续插入分隔符，不要解释分隔符。代码块、行内代码、结构化数据、链接和多媒体地址内部禁止插入分隔符。思考过程、工具调用及语音、图片、视频、文件由协议独立呈现，不要伪造这些内容或用分隔符代替工具调用。"

func NormalizeMessageStyle(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "flow":
		return "flow", nil
	case "bubble":
		return "bubble", nil
	default:
		return "", fmt.Errorf("聊天界面风格无效")
	}
}
