package chat

func BuildMessageExcerpt(msg *Message) string {
	if msg == nil {
		return ""
	}
	if msg.MsgType == "image" || (msg.ImageUrl != "" && msg.Content == "[图片]") {
		return "[图片]"
	}
	if msg.MsgType == "voice" || (msg.AudioUrl != "" && msg.Content == "[语音]") {
		return "[语音消息]"
	}
	if msg.MsgType == "video" || (msg.VideoUrl != "" && msg.Content == "[视频]") {
		return "[视频]"
	}
	runes := []rune(msg.Content)
	if len(runes) <= 200 {
		return msg.Content
	}
	return string(runes[:200])
}

func (s *service) BuildReplyContext(targetMsg *Message) (replyToRole *string, replyToExcerpt *string) {
	if targetMsg == nil {
		return nil, nil
	}
	role := targetMsg.Role
	excerpt := BuildMessageExcerpt(targetMsg)
	return &role, &excerpt
}
