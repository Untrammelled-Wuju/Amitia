// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	applog "github.com/u-ai/backend/log"
)

type GenerationStatus string

const (
	GenStatusIdle       GenerationStatus = "idle"
	GenStatusCollecting GenerationStatus = "collecting"
	GenStatusProcessing GenerationStatus = "processing"
	GenStatusCompleted  GenerationStatus = "completed"
	GenStatusFailed     GenerationStatus = "failed"
	GenStatusCancelled  GenerationStatus = "cancelled"
)

type ConversationGenerationRuntime struct {
	ConversationID     string
	GenerationStatus   GenerationStatus
	ActiveGenerationID string
	GenerationRunning  bool
	CancelFunc         context.CancelFunc
	GenerationIDs      []string
	Mu                 sync.Mutex
}

type GenerationQueue struct {
	mu       sync.Mutex
	runtimes map[string]*ConversationGenerationRuntime
	handler  func(ctx context.Context, convID, generationID string, msgIDs []string) error
}

var generationQueue *GenerationQueue

func InitGenerationQueue(handler func(ctx context.Context, convID, generationID string, msgIDs []string) error) {
	generationQueue = &GenerationQueue{
		runtimes: make(map[string]*ConversationGenerationRuntime),
		handler:  handler,
	}
}

func GetGenerationQueue() *GenerationQueue {
	if generationQueue == nil {
		generationQueue = &GenerationQueue{
			runtimes: make(map[string]*ConversationGenerationRuntime),
		}
	}
	return generationQueue
}

func (gq *GenerationQueue) getOrCreate(convID string) *ConversationGenerationRuntime {
	gq.mu.Lock()
	defer gq.mu.Unlock()
	rt, exists := gq.runtimes[convID]
	if !exists {
		rt = &ConversationGenerationRuntime{
			ConversationID:   convID,
			GenerationStatus: GenStatusIdle,
		}
		gq.runtimes[convID] = rt
	}
	return rt
}

func (gq *GenerationQueue) GetStatus(convID string) GenerationStatus {
	rt := gq.getOrCreate(convID)
	rt.Mu.Lock()
	defer rt.Mu.Unlock()
	return rt.GenerationStatus
}

func (gq *GenerationQueue) StartCollection(convID string) string {
	rt := gq.getOrCreate(convID)
	rt.Mu.Lock()
	defer rt.Mu.Unlock()
	genID := uuid.New().String()
	rt.GenerationIDs = append(rt.GenerationIDs, genID)
	if rt.GenerationStatus == GenStatusIdle || rt.GenerationStatus == GenStatusCompleted || rt.GenerationStatus == GenStatusFailed || rt.GenerationStatus == GenStatusCancelled {
		rt.GenerationStatus = GenStatusCollecting
	}
	return genID
}

func (gq *GenerationQueue) StartProcessing(convID, genID string) {
	rt := gq.getOrCreate(convID)
	rt.Mu.Lock()
	defer rt.Mu.Unlock()
	rt.GenerationStatus = GenStatusProcessing
	rt.GenerationRunning = true
	rt.ActiveGenerationID = genID
}

func (gq *GenerationQueue) FinishProcessing(convID string) {
	rt := gq.getOrCreate(convID)
	rt.Mu.Lock()
	defer rt.Mu.Unlock()
	rt.GenerationRunning = false
	rt.ActiveGenerationID = ""
	if rt.GenerationStatus == GenStatusProcessing {
		rt.GenerationStatus = GenStatusCompleted
	}
	if rt.CancelFunc != nil {
		rt.CancelFunc()
		rt.CancelFunc = nil
	}
}

func (gq *GenerationQueue) MarkFailed(convID string) {
	rt := gq.getOrCreate(convID)
	rt.Mu.Lock()
	defer rt.Mu.Unlock()
	rt.GenerationRunning = false
	rt.ActiveGenerationID = ""
	rt.GenerationStatus = GenStatusFailed
	if rt.CancelFunc != nil {
		rt.CancelFunc()
		rt.CancelFunc = nil
	}
}

func (gq *GenerationQueue) Cancel(convID string) {
	rt := gq.getOrCreate(convID)
	rt.Mu.Lock()
	defer rt.Mu.Unlock()
	if rt.CancelFunc != nil {
		rt.CancelFunc()
		rt.CancelFunc = nil
	}
	rt.GenerationRunning = false
	rt.ActiveGenerationID = ""
	rt.GenerationStatus = GenStatusCancelled
}

func (gq *GenerationQueue) SetCancelFunc(convID string, cf context.CancelFunc) {
	rt := gq.getOrCreate(convID)
	rt.Mu.Lock()
	defer rt.Mu.Unlock()
	rt.CancelFunc = cf
}

func (gq *GenerationQueue) RunGeneration(ctx context.Context, convID, genID string, msgIDs []string) error {
	rt := gq.getOrCreate(convID)
	rt.Mu.Lock()
	if rt.GenerationRunning {
		rt.Mu.Unlock()
		applog.Info("[GenerationQueue] generation already running, queuing conversation " + convID)
		for {
			time.Sleep(500 * time.Millisecond)
			rt.Mu.Lock()
			if !rt.GenerationRunning {
				break
			}
			rt.Mu.Unlock()
		}
	}
	rt.GenerationStatus = GenStatusProcessing
	rt.GenerationRunning = true
	rt.ActiveGenerationID = genID
	genCtx, cancel := context.WithCancel(ctx)
	rt.CancelFunc = cancel
	rt.Mu.Unlock()

	defer func() {
		rt.Mu.Lock()
		rt.GenerationRunning = false
		rt.ActiveGenerationID = ""
		rt.CancelFunc = nil
		if rt.GenerationStatus == GenStatusProcessing {
			rt.GenerationStatus = GenStatusCompleted
		}
		rt.Mu.Unlock()
	}()

	if gq.handler != nil {
		return gq.handler(genCtx, convID, genID, msgIDs)
	}
	return nil
}

func (gq *GenerationQueue) AcquireSlot(ctx context.Context, convID, genID string) (context.Context, context.CancelFunc, error) {
	rt := gq.getOrCreate(convID)
	rt.Mu.Lock()
	if rt.GenerationStatus == GenStatusCancelled {
		rt.Mu.Unlock()
		return nil, nil, fmt.Errorf("generation cancelled for %s", convID)
	}
	if rt.GenerationRunning {
		rt.Mu.Unlock()
		applog.Info("[GenerationQueue] generation already running, queuing conversation " + convID)
		for {
			time.Sleep(500 * time.Millisecond)
			rt.Mu.Lock()
			if rt.GenerationStatus == GenStatusCancelled {
				rt.Mu.Unlock()
				return nil, nil, fmt.Errorf("generation cancelled for %s", convID)
			}
			if !rt.GenerationRunning {
				break
			}
			rt.Mu.Unlock()
		}
	}
	if rt.GenerationStatus == GenStatusCancelled {
		rt.Mu.Unlock()
		return nil, nil, fmt.Errorf("generation cancelled for %s", convID)
	}
	rt.GenerationStatus = GenStatusProcessing
	rt.GenerationRunning = true
	rt.ActiveGenerationID = genID
	genCtx, cancel := context.WithCancel(ctx)
	rt.CancelFunc = cancel
	rt.Mu.Unlock()
	return genCtx, cancel, nil
}

func (gq *GenerationQueue) ShouldCancel(convID string) bool {
	rt := gq.getOrCreate(convID)
	rt.Mu.Lock()
	defer rt.Mu.Unlock()
	return rt.GenerationStatus == GenStatusCancelled
}

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
