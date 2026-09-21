package chat

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/conversationstream"
	"gorm.io/gorm"
)

const (
	streamCheckpointInterval = 750 * time.Millisecond
	streamCheckpointBytes    = 4096
)

type liveTurnBlock struct {
	item           AssistantTurnItem
	blockType      string
	revision       int64
	content        strings.Builder
	arguments      strings.Builder
	lastCheckpoint time.Time
	checkpointSize int
}

type modelEventProjector struct {
	recorder  *assistantTurnRecorder
	mu        sync.Mutex
	text      *liveTurnBlock
	reasoning *liveTurnBlock
	tools     map[string]*liveTurnBlock
}

func newModelEventProjector(recorder *assistantTurnRecorder) *modelEventProjector {
	return &modelEventProjector{recorder: recorder, tools: map[string]*liveTurnBlock{}}
}

func (p *modelEventProjector) Emit(ctx context.Context, event ModelEvent) error {
	if p == nil || p.recorder == nil || strings.TrimSpace(p.recorder.ConversationID) == "" {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	switch event.Type {
	case ModelEventTextDelta:
		block, err := p.ensureBlock(ctx, "text", "", "")
		if err != nil {
			return err
		}
		p.text = block
		return p.appendDelta(ctx, block, event.TextDelta, false)
	case ModelEventTextDone:
		return p.completeBlock(ctx, p.text, assistantTurnStatusCompleted)
	case ModelEventReasoningSummaryDelta:
		block, err := p.ensureBlock(ctx, "reasoning", "", "")
		if err != nil {
			return err
		}
		p.reasoning = block
		return p.appendDelta(ctx, block, event.TextDelta, false)
	case ModelEventReasoningSummaryDone:
		return p.completeBlock(ctx, p.reasoning, assistantTurnStatusCompleted)
	case ModelEventToolCallStarted:
		_, err := p.ensureBlock(ctx, "tool_call", event.ToolCallID, event.ToolName)
		return err
	case ModelEventToolCallArgumentsDelta:
		block, err := p.ensureBlock(ctx, "tool_call", event.ToolCallID, event.ToolName)
		if err != nil {
			return err
		}
		return p.appendDelta(ctx, block, event.ArgumentsDelta, true)
	case ModelEventToolCallDone:
		block := p.tools[strings.TrimSpace(event.ToolCallID)]
		if block == nil {
			var err error
			block, err = p.ensureBlock(ctx, "tool_call", event.ToolCallID, event.ToolName)
			if err != nil {
				return err
			}
		}
		return p.argumentsCompleted(ctx, block)
	case ModelEventFailed:
		if err := p.completeOpenBlocks(ctx, assistantTurnStatusFailed); err != nil {
			return err
		}
	case ModelEventCancelled:
		if err := p.completeOpenBlocks(ctx, assistantTurnStatusInterrupted); err != nil {
			return err
		}
	case ModelEventCompleted:
		return p.completeOpenContentBlocks(ctx)
	}
	return nil
}

func (p *modelEventProjector) Text() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.text == nil {
		return ""
	}
	return p.text.content.String()
}

func (p *modelEventProjector) Reasoning() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.reasoning == nil {
		return ""
	}
	return p.reasoning.content.String()
}

func (p *modelEventProjector) EnsureText(ctx context.Context, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.text != nil && strings.TrimSpace(p.text.content.String()) != "" {
		return nil
	}
	if p.text != nil && (p.text.item.Status == assistantTurnStatusCompleted || p.text.item.Status == assistantTurnStatusFailed || p.text.item.Status == assistantTurnStatusInterrupted) {
		p.text = nil
	}
	block, err := p.ensureBlock(ctx, "text", "", "")
	if err != nil {
		return err
	}
	if err := p.appendDelta(ctx, block, text, false); err != nil {
		return err
	}
	return p.completeBlock(ctx, block, assistantTurnStatusCompleted)
}

func (p *modelEventProjector) Complete(ctx context.Context, status string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.completeOpenBlocks(ctx, status)
}

func (p *modelEventProjector) ensureBlock(ctx context.Context, blockType, callID, toolName string) (*liveTurnBlock, error) {
	if blockType == "text" && p.text != nil {
		return p.text, nil
	}
	if blockType == "reasoning" && p.reasoning != nil {
		return p.reasoning, nil
	}
	if blockType == "tool_call" {
		callID = strings.TrimSpace(callID)
		if callID != "" && p.tools[callID] != nil {
			return p.tools[callID], nil
		}
	}
	itemType := blockType
	item := AssistantTurnItem{ID: uuid.NewString(), TurnID: p.recorder.TurnID, ConversationID: p.recorder.ConversationID, ItemType: itemType, Status: assistantTurnStatusRunning, Revision: 1, CallID: strings.TrimSpace(callID), ToolName: strings.TrimSpace(toolName), CreatedAt: nowString(), UpdatedAt: nowString()}
	if p.recorder.enabled && p.recorder.db != nil {
		if err := p.recorder.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return appendAssistantTurnItemTx(tx, &item) }); err != nil {
			return nil, err
		}
	}
	block := &liveTurnBlock{item: item, blockType: blockType, revision: 1, lastCheckpoint: time.Now()}
	if blockType == "text" {
		p.text = block
	} else if blockType == "reasoning" {
		p.reasoning = block
	} else if blockType == "tool_call" && callID != "" {
		p.tools[callID] = block
	}
	eventType := blockType + ".started"
	if blockType == "tool_call" {
		eventType = "tool.started"
	}
	payload := map[string]any{"blockType": blockType}
	if toolName != "" {
		payload["toolName"] = toolName
	}
	if _, err := conversationstream.DefaultManager().Publish(ctx, p.recorder.event(block, eventType, assistantTurnStatusRunning, payload), true); err != nil {
		return nil, err
	}
	return block, nil
}

func (p *modelEventProjector) appendDelta(ctx context.Context, block *liveTurnBlock, delta string, arguments bool) error {
	if block == nil || delta == "" {
		return nil
	}
	block.revision++
	if arguments {
		block.arguments.WriteString(delta)
	} else {
		block.content.WriteString(delta)
	}
	payload := map[string]any{"blockType": block.blockType, "delta": delta}
	eventType := block.blockType + ".delta"
	if arguments {
		eventType = "tool.arguments.delta"
		payload["field"] = "arguments"
	}
	if _, err := conversationstream.DefaultManager().Publish(ctx, p.recorder.event(block, eventType, assistantTurnStatusRunning, payload), false); err != nil {
		return err
	}
	currentSize := block.content.Len() + block.arguments.Len()
	if currentSize-block.checkpointSize >= streamCheckpointBytes || time.Since(block.lastCheckpoint) >= streamCheckpointInterval {
		return p.checkpoint(ctx, block)
	}
	return nil
}

func (p *modelEventProjector) checkpoint(ctx context.Context, block *liveTurnBlock) error {
	if block == nil || !p.recorder.enabled || p.recorder.db == nil {
		return nil
	}
	block.revision++
	updates := map[string]any{"revision": block.revision, "updated_at": nowString()}
	if block.content.Len() > 0 {
		updates["content"] = block.content.String()
	}
	if block.arguments.Len() > 0 {
		updates["arguments_json"] = block.arguments.String()
	}
	if err := p.recorder.db.WithContext(ctx).Model(&AssistantTurnItem{}).Where("id = ?", block.item.ID).Updates(updates).Error; err != nil {
		block.revision--
		return err
	}
	block.checkpointSize = block.content.Len() + block.arguments.Len()
	block.lastCheckpoint = time.Now()
	payload := map[string]any{
		"blockType":          block.blockType,
		"content":            block.content.String(),
		"recoveryCheckpoint": true,
	}
	if block.arguments.Len() > 0 {
		payload["arguments"] = block.arguments.String()
	}
	if block.item.ToolName != "" {
		payload["toolName"] = block.item.ToolName
	}
	_, err := conversationstream.DefaultManager().Publish(ctx, p.recorder.event(block, "block.checkpoint", assistantTurnStatusRunning, payload), true)
	return err
}

func (p *modelEventProjector) argumentsCompleted(ctx context.Context, block *liveTurnBlock) error {
	if block == nil {
		return nil
	}
	if err := p.checkpoint(ctx, block); err != nil {
		return err
	}
	block.revision++
	if p.recorder.enabled && p.recorder.db != nil {
		if err := p.recorder.db.WithContext(ctx).Model(&AssistantTurnItem{}).Where("id = ?", block.item.ID).Updates(map[string]any{
			"arguments_json": block.arguments.String(),
			"revision":       block.revision,
			"updated_at":     nowString(),
		}).Error; err != nil {
			block.revision--
			return err
		}
	}
	payload := map[string]any{"blockType": block.blockType, "arguments": block.arguments.String(), "toolName": block.item.ToolName, "recoveryCheckpoint": true}
	_, err := conversationstream.DefaultManager().Publish(ctx, p.recorder.event(block, "tool.arguments.completed", assistantTurnStatusRunning, payload), true)
	return err
}

func (p *modelEventProjector) completeBlock(ctx context.Context, block *liveTurnBlock, status string) error {
	if block == nil || block.item.Status == assistantTurnStatusCompleted || block.item.Status == assistantTurnStatusFailed || block.item.Status == assistantTurnStatusInterrupted {
		return nil
	}
	block.item.Status = status
	block.revision++
	if p.recorder.enabled && p.recorder.db != nil {
		now := nowString()
		updates := map[string]any{"status": status, "revision": block.revision, "updated_at": now}
		if block.content.Len() > 0 {
			updates["content"] = block.content.String()
		}
		if block.arguments.Len() > 0 {
			updates["arguments_json"] = block.arguments.String()
		}
		if err := p.recorder.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return tx.Model(&AssistantTurnItem{}).Where("id = ?", block.item.ID).Updates(updates).Error
		}); err != nil {
			return err
		}
	}
	payload := map[string]any{"blockType": block.blockType, "content": block.content.String(), "recoveryCheckpoint": true}
	if block.arguments.Len() > 0 {
		payload["arguments"] = block.arguments.String()
	}
	if block.item.ToolName != "" {
		payload["toolName"] = block.item.ToolName
	}
	eventType := block.blockType + ".completed"
	if block.blockType == "tool_call" {
		eventType = "tool.completed"
	}
	if status == assistantTurnStatusFailed {
		eventType = block.blockType + ".failed"
		if block.blockType == "tool_call" {
			eventType = "tool.failed"
		}
	} else if status == assistantTurnStatusInterrupted {
		eventType = block.blockType + ".interrupted"
		if block.blockType == "tool_call" {
			eventType = "tool.interrupted"
		}
	}
	_, err := conversationstream.DefaultManager().Publish(ctx, p.recorder.event(block, eventType, status, payload), true)
	return err
}

func (p *modelEventProjector) completeOpenContentBlocks(ctx context.Context) error {
	if err := p.completeBlock(ctx, p.reasoning, assistantTurnStatusCompleted); err != nil {
		return err
	}
	return p.completeBlock(ctx, p.text, assistantTurnStatusCompleted)
}

func (p *modelEventProjector) completeOpenBlocks(ctx context.Context, status string) error {
	if err := p.completeBlock(ctx, p.reasoning, status); err != nil {
		return err
	}
	if err := p.completeBlock(ctx, p.text, status); err != nil {
		return err
	}
	if status == assistantTurnStatusFailed || status == assistantTurnStatusInterrupted {
		for _, block := range p.tools {
			if err := p.completeBlock(ctx, block, status); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *assistantTurnRecorder) event(block *liveTurnBlock, eventType, status string, payload map[string]any) conversationstream.AgentUIEvent {
	event := conversationstream.AgentUIEvent{ConversationID: r.ConversationID, RequestID: r.RequestID, ExecutionID: r.ExecutionID, TurnID: r.TurnID, TurnSequence: r.TurnSequence, Type: eventType, Status: status, Payload: payload}
	if block != nil {
		event.BlockID = block.item.ID
		event.BlockSequence = block.item.Sequence
		event.CallID = block.item.CallID
		event.Revision = block.revision
	}
	return event
}
