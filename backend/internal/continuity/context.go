package continuity

import (
	"encoding/json"
	"strings"
)

func BuildContext(repo *Repository, threadID, spaceID string) (Context, error) {
	if repo == nil || strings.TrimSpace(threadID) == "" {
		return Context{}, nil
	}
	thread, err := repo.GetThread(threadID, spaceID)
	if err != nil || thread == nil {
		return Context{}, err
	}
	waits, err := repo.ListOpenWaits(thread.ID, 6)
	if err != nil {
		return Context{}, err
	}
	events, err := repo.ListRecentEvents(thread.ID, 6)
	if err != nil {
		return Context{}, err
	}
	ctx := Context{ThreadID: thread.ID, Title: thread.Title, Goal: thread.Goal, Status: thread.Status, Summary: thread.Summary, CurrentState: thread.CurrentState, NextAction: thread.NextAction, Revision: thread.Revision}
	for _, w := range waits {
		ctx.Waits = append(ctx.Waits, WaitSummary{ID: w.ID, Type: w.WaitType, Description: w.Description, ResumeHint: w.ResumeHint, DueAt: w.DueAt})
	}
	for _, event := range events {
		summary := ""
		var payload map[string]interface{}
		if json.Unmarshal([]byte(event.PayloadJSON), &payload) == nil {
			if raw, ok := payload["summary"].(string); ok {
				summary = strings.TrimSpace(raw)
			}
		}
		ctx.RecentEvents = append(ctx.RecentEvents, EventSummary{Type: event.EventType, Summary: summary, OccurredAt: event.OccurredAt})
	}
	return ctx, nil
}

func RenderContext(ctx Context) string {
	if strings.TrimSpace(ctx.ThreadID) == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("当前持续事项：")
	b.WriteString(ctx.Title)
	b.WriteString("\n")
	if ctx.Goal != "" {
		b.WriteString("目标：")
		b.WriteString(ctx.Goal)
		b.WriteString("\n")
	}
	b.WriteString("状态：")
	b.WriteString(string(ctx.Status))
	b.WriteString("\n")
	if ctx.CurrentState != "" {
		b.WriteString("当前进度：")
		b.WriteString(ctx.CurrentState)
		b.WriteString("\n")
	}
	if ctx.NextAction != "" {
		b.WriteString("下一步：")
		b.WriteString(ctx.NextAction)
		b.WriteString("\n")
	}
	if len(ctx.Waits) > 0 {
		b.WriteString("当前等待：\n")
		for _, wait := range ctx.Waits {
			b.WriteString("- ")
			b.WriteString(wait.Description)
			if wait.ResumeHint != "" {
				b.WriteString("；恢复后：")
				b.WriteString(wait.ResumeHint)
			}
			b.WriteString("\n")
		}
	}
	if len(ctx.RecentEvents) > 0 {
		b.WriteString("最近关键变化：\n")
		for _, event := range ctx.RecentEvents {
			b.WriteString("- ")
			if event.Summary != "" {
				b.WriteString(event.Summary)
			} else {
				b.WriteString(event.Type)
			}
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}
