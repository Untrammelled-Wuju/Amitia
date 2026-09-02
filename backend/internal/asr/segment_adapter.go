// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package asr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

type SegmentASRAdapter struct {
	config    *AsrConfig
	mu        sync.Mutex
	audioBuf  []byte
	sessionID string
	language  string
}

func NewSegmentASRAdapter(config *AsrConfig) *SegmentASRAdapter {
	return &SegmentASRAdapter{
		config: config,
	}
}

func (a *SegmentASRAdapter) AppendPCM(pcm []byte) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.audioBuf = append(a.audioBuf, pcm...)
}

func (a *SegmentASRAdapter) SetSession(sessionID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessionID = sessionID
}

func (a *SegmentASRAdapter) SetLanguage(lang string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.language = lang
}

func (a *SegmentASRAdapter) BufferedDurationMS() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.config == nil {
		return 0
	}
	sampleRate := 16000
	channels := 1
	bytesPerFrame := 2 * channels
	frames := len(a.audioBuf) / bytesPerFrame
	return frames * 1000 / sampleRate
}

func (a *SegmentASRAdapter) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.audioBuf = a.audioBuf[:0]
}

func (a *SegmentASRAdapter) Recognize(ctx context.Context) (string, error) {
	a.mu.Lock()
	if len(a.audioBuf) == 0 {
		a.mu.Unlock()
		return "", fmt.Errorf("segment asr: no audio buffered")
	}
	audioCopy := make([]byte, len(a.audioBuf))
	copy(audioCopy, a.audioBuf)
	config := a.config
	lang := a.language
	a.mu.Unlock()

	if config == nil {
		return "", fmt.Errorf("segment asr: config is nil")
	}

	tmpFile := filepath.Join(os.TempDir(), "amitia_segment_"+uuid.New().String()+".wav")
	if err := writeWAV(tmpFile, audioCopy, 16000, 1); err != nil {
		return "", fmt.Errorf("segment asr: write temp audio failed: %w", err)
	}
	defer os.Remove(tmpFile)

	taskID, err := SubmitTask(config, "file://"+tmpFile, lang)
	if err != nil {
		return "", fmt.Errorf("segment asr: submit failed: %w", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}

		resp, err := QueryTask(config, taskID)
		if err != nil {
			continue
		}
		if resp.Status == "success" && resp.Result != "" {
			return resp.Result, nil
		}
	}

	return "", fmt.Errorf("segment asr: recognition timeout")
}

func writeWAV(path string, pcm []byte, sampleRate int, channels int) error {
	header := make([]byte, 44)

	dataSize := len(pcm)
	fileSize := 36 + dataSize

	header[0] = 'R'
	header[1] = 'I'
	header[2] = 'F'
	header[3] = 'F'
	header[4] = byte(fileSize)
	header[5] = byte(fileSize >> 8)
	header[6] = byte(fileSize >> 16)
	header[7] = byte(fileSize >> 24)
	header[8] = 'W'
	header[9] = 'A'
	header[10] = 'V'
	header[11] = 'E'
	header[12] = 'f'
	header[13] = 'm'
	header[14] = 't'
	header[15] = ' '
	header[16] = 16
	header[17] = 0
	header[18] = 0
	header[19] = 0
	header[20] = 1
	header[21] = 0
	header[22] = byte(channels)
	header[23] = 0
	header[24] = byte(sampleRate)
	header[25] = byte(sampleRate >> 8)
	header[26] = byte(sampleRate >> 16)
	header[27] = byte(sampleRate >> 24)
	byteRate := sampleRate * channels * 2
	header[28] = byte(byteRate)
	header[29] = byte(byteRate >> 8)
	header[30] = byte(byteRate >> 16)
	header[31] = byte(byteRate >> 24)
	blockAlign := channels * 2
	header[32] = byte(blockAlign)
	header[33] = byte(blockAlign >> 8)
	header[34] = 16
	header[35] = 0
	header[36] = 'd'
	header[37] = 'a'
	header[38] = 't'
	header[39] = 'a'
	header[40] = byte(dataSize)
	header[41] = byte(dataSize >> 8)
	header[42] = byte(dataSize >> 16)
	header[43] = byte(dataSize >> 24)

	var buf bytes.Buffer
	buf.Write(header)
	buf.Write(pcm)

	return os.WriteFile(path, buf.Bytes(), 0600)
}

type SegmentASRSession struct {
	adapter   *SegmentASRAdapter
	events    chan ASRStreamEvent
	cancel    context.CancelFunc
	mu        sync.Mutex
	closed    bool
	done      sync.Once
	sessionID string
	language  string
}

func NewSegmentASRSession(adapter *SegmentASRAdapter, cancel context.CancelFunc, sessionID, language string) *SegmentASRSession {
	return &SegmentASRSession{
		adapter:   adapter,
		events:    make(chan ASRStreamEvent, 16),
		cancel:    cancel,
		sessionID: sessionID,
		language:  language,
	}
}

func (s *SegmentASRSession) PushPCM(ctx context.Context, pcm []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("segment asr: session closed")
	}
	s.adapter.AppendPCM(pcm)
	return nil
}

func (s *SegmentASRSession) EndUtterance(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("segment asr: session closed")
	}
	s.mu.Unlock()

	go func() {
		text, err := s.adapter.Recognize(ctx)
		if err != nil {
			s.emit(ASRStreamEvent{
				Type:      ASRStreamEventError,
				SessionID: s.sessionID,
				Error:     err.Error(),
			})
			return
		}
		s.emit(ASRStreamEvent{
			Type:      ASRStreamEventFinal,
			SessionID: s.sessionID,
			Text:      text,
			IsFinal:   true,
		})
	}()

	return nil
}

func (s *SegmentASRSession) Events() <-chan ASRStreamEvent {
	return s.events
}

func (s *SegmentASRSession) Cancel() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

func (s *SegmentASRSession) Close() error {
	s.done.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		close(s.events)
		s.adapter.Reset()
	})
	return nil
}

func (s *SegmentASRSession) emit(event ASRStreamEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.events <- event:
	default:
	}
}

func (s *SegmentASRSession) emitStarted() {
	s.emit(ASRStreamEvent{
		Type:      ASRStreamEventSessionStarted,
		SessionID: s.sessionID,
		Timestamp: time.Now().UnixMilli(),
	})
}

func init() {
	_ = json.Marshal
}
