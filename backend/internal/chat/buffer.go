// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"fmt"
	"strings"
	"sync"
	"time"

	applog "github.com/u-ai/backend/log"
)

var ErrBufferAborted = fmt.Errorf("buffer aborted by newer message")

var globalBuffer *MessageBuffer

func InitBuffer(waitMs int) {
	if globalBuffer == nil {
		globalBuffer = NewMessageBuffer(waitMs)
	}
}

func GetBuffer() *MessageBuffer {
	if globalBuffer == nil {
		globalBuffer = NewMessageBuffer(6000)
	}
	return globalBuffer
}

type MessageBuffer struct {
	mu      sync.Mutex
	buffers map[string]*conversationBuffer
	wait    time.Duration
}

type conversationBuffer struct {
	mu            sync.Mutex
	messages      []string
	imageContexts []string
	timer         *time.Timer
	waiters       []chan []string
}

func NewMessageBuffer(waitMs int) *MessageBuffer {
	return &MessageBuffer{
		buffers: make(map[string]*conversationBuffer),
		wait:    time.Duration(waitMs) * time.Millisecond,
	}
}

func (mb *MessageBuffer) getOrCreate(convID string) *conversationBuffer {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	buf, exists := mb.buffers[convID]
	if !exists {
		buf = &conversationBuffer{}
		mb.buffers[convID] = buf
	}
	return buf
}

func (mb *MessageBuffer) Buffer(convID, text string) ([]string, error) {
	buf := mb.getOrCreate(convID)

	buf.mu.Lock()
	buf.messages = append(buf.messages, text)

	if buf.timer != nil {
		buf.timer.Stop()
	}

	for _, ch := range buf.waiters {
		close(ch)
	}
	buf.waiters = nil

	ch := make(chan []string, 1)
	buf.waiters = append(buf.waiters, ch)

	buf.timer = time.AfterFunc(mb.wait, func() {
		buf.mu.Lock()
		defer buf.mu.Unlock()
		msgs := make([]string, len(buf.messages))
		copy(msgs, buf.messages)
		buf.messages = nil
		buf.timer = nil
		for _, w := range buf.waiters {
			w <- msgs
		}
		buf.waiters = nil
	})
	buf.mu.Unlock()

	msgs, ok := <-ch
	if !ok {
		return nil, ErrBufferAborted
	}
	return msgs, nil
}

func (mb *MessageBuffer) AnalyzeImage(convID, imageUrl string) {
	if imageUrl == "" {
		return
	}
	buf := mb.getOrCreate(convID)

	applog.Info(fmt.Sprintf("[Image] Analyzing image: %s", imageUrl[:min(len(imageUrl), 80)]))
	desc, errDetail := analyzeImageInternal(imageUrl)
	if desc == "" && errDetail != "" {
		applog.Warn(fmt.Sprintf("[Image] Analysis failed: %s", errDetail))
		desc = "图片解析失败：" + errDetail
	} else if desc != "" {
		applog.Info(fmt.Sprintf("[Image] Analysis success, descLen=%d", len(desc)))
	}
	if desc != "" {
		ctx := "[图片描述：" + desc + "]"
		buf.mu.Lock()
		buf.imageContexts = append(buf.imageContexts, ctx)
		buf.mu.Unlock()
		applog.Info(fmt.Sprintf("[Image] Context appended, total contexts=%d", len(buf.imageContexts)))
	}
}

func (mb *MessageBuffer) AnalyzeVideo(convID, videoUrl string) {
	if videoUrl == "" {
		return
	}
	buf := mb.getOrCreate(convID)

	applog.Info(fmt.Sprintf("[Video] Analyzing video: %s", videoUrl[:min(len(videoUrl), 80)]))
	desc, errDetail := analyzeVideoInternal(videoUrl)
	if desc == "" && errDetail != "" {
		applog.Warn(fmt.Sprintf("[Video] Analysis failed: %s", errDetail))
		desc = "视频解析失败：" + errDetail
	} else if desc != "" {
		applog.Info(fmt.Sprintf("[Video] Analysis success, descLen=%d desc=%s", len(desc), desc[:min(len(desc), 120)]))
	} else {
		applog.Warn("[Video] Analysis returned empty without error")
	}
	if desc != "" {
		ctx := "[视频描述：" + desc + "]"
		buf.mu.Lock()
		buf.imageContexts = append(buf.imageContexts, ctx)
		buf.mu.Unlock()
		applog.Info(fmt.Sprintf("[Video] Context appended to imageContexts, total contexts=%d", len(buf.imageContexts)))
	}
}

func (mb *MessageBuffer) GetImageContexts(convID string) string {
	buf := mb.getOrCreate(convID)
	buf.mu.Lock()
	defer buf.mu.Unlock()
	if len(buf.imageContexts) == 0 {
		return ""
	}
	return strings.Join(buf.imageContexts, "\n")
}

func (mb *MessageBuffer) GetPending(convID string) []string {
	mb.mu.Lock()
	buf, exists := mb.buffers[convID]
	mb.mu.Unlock()
	if !exists {
		return nil
	}
	buf.mu.Lock()
	defer buf.mu.Unlock()
	return append([]string{}, buf.messages...)
}

func (mb *MessageBuffer) Flush(convID string) string {
	mb.mu.Lock()
	buf, exists := mb.buffers[convID]
	mb.mu.Unlock()
	if !exists {
		return ""
	}
	buf.mu.Lock()
	defer buf.mu.Unlock()
	if buf.timer != nil {
		buf.timer.Stop()
		buf.timer = nil
	}
	if len(buf.messages) == 0 {
		return ""
	}
	combined := strings.Join(buf.messages, "\n")
	buf.messages = nil
	buf.imageContexts = nil
	return combined
}
