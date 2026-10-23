package continuity

import "strings"

func RenderContext(ctx Context) string {
	if strings.TrimSpace(ctx.ThreadID) == "" {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("当前持续事项：")
	builder.WriteString(ctx.Title)
	builder.WriteString("\n")
	if ctx.Goal != "" {
		builder.WriteString("目标：")
		builder.WriteString(ctx.Goal)
		builder.WriteString("\n")
	}
	builder.WriteString("状态：")
	builder.WriteString(string(ctx.Status))
	builder.WriteString("\n")
	if ctx.CurrentState != "" {
		builder.WriteString("当前进度：")
		builder.WriteString(ctx.CurrentState)
		builder.WriteString("\n")
	}
	if ctx.NextAction != "" {
		builder.WriteString("下一步：")
		builder.WriteString(ctx.NextAction)
		builder.WriteString("\n")
	}
	if len(ctx.Waits) > 0 {
		builder.WriteString("当前等待：\n")
		for _, wait := range ctx.Waits {
			builder.WriteString("- ")
			builder.WriteString(wait.Description)
			if wait.ResumeHint != "" {
				builder.WriteString("；恢复后：")
				builder.WriteString(wait.ResumeHint)
			}
			builder.WriteString("\n")
		}
	}
	if len(ctx.RecentEvents) > 0 {
		builder.WriteString("最近关键变化：\n")
		for _, event := range ctx.RecentEvents {
			builder.WriteString("- ")
			if event.Summary != "" {
				builder.WriteString(event.Summary)
			} else {
				builder.WriteString(event.Type)
			}
			builder.WriteString("\n")
		}
	}
	return strings.TrimSpace(builder.String())
}
