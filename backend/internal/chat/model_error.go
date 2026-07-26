package chat

type TextModelCallError struct {
	RawError string
}

func (e *TextModelCallError) Error() string {
	return "AI 调用失败: " + e.RawError
}
