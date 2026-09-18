// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxVisualFrameBytes = 2 * 1024 * 1024
	visualQueueDepth    = 1
)

type VisualFrame struct {
	CallID           string           `json:"callId,omitempty"`
	Source           VisualSourceType `json:"source"`
	Sequence         uint64           `json:"sequence"`
	CaptureTimestamp time.Time        `json:"captureTimestamp"`
	IngressTimestamp time.Time        `json:"ingressTimestamp"`
	MIME             string           `json:"mime"`
	Width            int              `json:"width,omitempty"`
	Height           int              `json:"height,omitempty"`
	CursorX          *float64         `json:"cursorX,omitempty"`
	CursorY          *float64         `json:"cursorY,omitempty"`
	FocusHint        string           `json:"focusHint,omitempty"`
	Immediate        bool             `json:"immediate,omitempty"`
	Data             []byte           `json:"-"`
}

type VisualFrameEnvelope struct {
	Source           VisualSourceType `json:"source"`
	Sequence         uint64           `json:"sequence"`
	CaptureTimestamp string           `json:"captureTimestamp,omitempty"`
	MIME             string           `json:"mime"`
	Width            int              `json:"width,omitempty"`
	Height           int              `json:"height,omitempty"`
	CursorX          *float64         `json:"cursorX,omitempty"`
	CursorY          *float64         `json:"cursorY,omitempty"`
	FocusHint        string           `json:"focusHint,omitempty"`
	Immediate        bool             `json:"immediate,omitempty"`
	Data             string           `json:"data"`
}

type VisualPipelineUpdate struct {
	Source     VisualSourceType `json:"source"`
	Sequence   uint64           `json:"sequence"`
	CapturedAt time.Time        `json:"capturedAt"`
	AnalyzedAt time.Time        `json:"analyzedAt"`
	Context    string           `json:"context"`
	Dropped    uint64           `json:"dropped"`
	AnalysisMS int64            `json:"analysisMs"`
}

type VisualPipeline struct {
	call     *RealtimeCallSession
	analyzer VisualAnalyzer
	updates  chan VisualPipelineUpdate
	errors   chan error
	input    chan VisualFrame
	cancel   context.CancelFunc
	wg       sync.WaitGroup

	dropped      atomic.Uint64
	mu           sync.Mutex
	lastHash     map[VisualSourceType][32]byte
	lastAccepted map[VisualSourceType]time.Time
}

func NewVisualPipeline(parent context.Context, call *RealtimeCallSession, analyzer VisualAnalyzer) *VisualPipeline {
	ctx, cancel := context.WithCancel(parent)
	p := &VisualPipeline{
		call:         call,
		analyzer:     analyzer,
		updates:      make(chan VisualPipelineUpdate, 8),
		errors:       make(chan error, 4),
		input:        make(chan VisualFrame, visualQueueDepth),
		cancel:       cancel,
		lastHash:     make(map[VisualSourceType][32]byte),
		lastAccepted: make(map[VisualSourceType]time.Time),
	}
	p.wg.Add(1)
	go p.run(ctx)
	return p
}

func (p *VisualPipeline) Updates() <-chan VisualPipelineUpdate { return p.updates }
func (p *VisualPipeline) Errors() <-chan error                 { return p.errors }

func (p *VisualPipeline) Close() {
	p.cancel()
	p.wg.Wait()
}

func (p *VisualPipeline) Submit(frame VisualFrame) bool {
	if p == nil || len(frame.Data) == 0 || len(frame.Data) > maxVisualFrameBytes {
		return false
	}
	if !validVisualSource(frame.Source) || !validVisualMIME(frame.MIME) {
		return false
	}
	if frame.CaptureTimestamp.IsZero() {
		frame.CaptureTimestamp = time.Now().UTC()
	}
	frame.IngressTimestamp = time.Now().UTC()

	sum := sha256.Sum256(frame.Data)
	p.mu.Lock()
	previous, hasPrevious := p.lastHash[frame.Source]
	lastAccepted := p.lastAccepted[frame.Source]
	minInterval := 330 * time.Millisecond
	if !frame.Immediate && time.Since(lastAccepted) < minInterval {
		p.mu.Unlock()
		p.dropped.Add(1)
		return false
	}
	if hasPrevious && previous == sum && !frame.Immediate {
		p.mu.Unlock()
		p.dropped.Add(1)
		return false
	}
	p.lastHash[frame.Source] = sum
	p.lastAccepted[frame.Source] = time.Now()
	p.mu.Unlock()

	select {
	case p.input <- frame:
		return true
	default:
		// Visual realtime semantics are latest-frame-wins. Never build an
		// unbounded queue behind a slower visual model.
		select {
		case <-p.input:
			p.dropped.Add(1)
		default:
		}
		select {
		case p.input <- frame:
			return true
		default:
			p.dropped.Add(1)
			return false
		}
	}
}

func (p *VisualPipeline) run(ctx context.Context) {
	defer p.wg.Done()
	defer close(p.updates)
	defer close(p.errors)
	for {
		select {
		case <-ctx.Done():
			return
		case frame := <-p.input:
			if p.analyzer == nil || !p.analyzer.Available() {
				p.emitError(fmt.Errorf("visual model unavailable; frame accepted but semantic context was not updated"))
				continue
			}
			started := time.Now()
			analysisCtx, cancel := context.WithTimeout(ctx, 22*time.Second)
			text, err := p.analyzer.Analyze(analysisCtx, frame)
			cancel()
			if err != nil {
				p.emitError(err)
				continue
			}
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			p.call.UpdateVisualContext(frame.Source, frame.CaptureTimestamp, text)
			update := VisualPipelineUpdate{
				Source:     frame.Source,
				Sequence:   frame.Sequence,
				CapturedAt: frame.CaptureTimestamp,
				AnalyzedAt: time.Now().UTC(),
				Context:    text,
				Dropped:    p.dropped.Load(),
				AnalysisMS: time.Since(started).Milliseconds(),
			}
			select {
			case p.updates <- update:
			default:
			}
		}
	}
}

func (p *VisualPipeline) emitError(err error) {
	select {
	case p.errors <- err:
	default:
	}
}

func ParseVisualFrame(callID string, envelope VisualFrameEnvelope) (VisualFrame, error) {
	if !validVisualSource(envelope.Source) {
		return VisualFrame{}, fmt.Errorf("unsupported visual source: %s", envelope.Source)
	}
	mime := strings.ToLower(strings.TrimSpace(envelope.MIME))
	if !validVisualMIME(mime) {
		return VisualFrame{}, fmt.Errorf("unsupported visual mime: %s", mime)
	}
	raw, err := base64.StdEncoding.DecodeString(envelope.Data)
	if err != nil {
		return VisualFrame{}, fmt.Errorf("decode visual frame: %w", err)
	}
	if len(raw) == 0 || len(raw) > maxVisualFrameBytes {
		return VisualFrame{}, fmt.Errorf("visual frame size must be between 1 and %d bytes", maxVisualFrameBytes)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return VisualFrame{}, fmt.Errorf("decode visual image: %w", err)
	}
	capturedAt := time.Now().UTC()
	if strings.TrimSpace(envelope.CaptureTimestamp) != "" {
		if parsed, parseErr := time.Parse(time.RFC3339Nano, envelope.CaptureTimestamp); parseErr == nil {
			capturedAt = parsed.UTC()
		}
	}
	return VisualFrame{
		CallID:           callID,
		Source:           envelope.Source,
		Sequence:         envelope.Sequence,
		CaptureTimestamp: capturedAt,
		MIME:             mime,
		Width:            cfg.Width,
		Height:           cfg.Height,
		CursorX:          envelope.CursorX,
		CursorY:          envelope.CursorY,
		FocusHint:        strings.TrimSpace(envelope.FocusHint),
		Immediate:        envelope.Immediate,
		Data:             raw,
	}, nil
}

func validVisualSource(source VisualSourceType) bool {
	return source == VisualSourceCamera || source == VisualSourceScreen
}

func validVisualMIME(mime string) bool {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/jpeg", "image/png":
		return true
	default:
		return false
	}
}
