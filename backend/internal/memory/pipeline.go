// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"context"
	"sync"
	"time"
)

const (
	LayerWorkingMemory  = 1
	LayerProfile        = 2
	LayerEpisodic       = 3
	LayerStructuredFact = 4
	LayerVector         = 5
	LayerGraph          = 6
)

var LayerNames = map[int]string{
	LayerWorkingMemory:  "工作记忆",
	LayerProfile:        "用户画像",
	LayerEpisodic:       "情景记忆",
	LayerStructuredFact: "结构化事实",
	LayerVector:         "向量同步",
	LayerGraph:          "图谱关系",
}

type PipelineLayer interface {
	Name() string
	Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error
}

var ErrSkip = &SkipError{}

type SkipError struct{}

func (e *SkipError) Error() string { return "skip" }

type RetryableError struct {
	Layer string
	Err   error
}

func (e *RetryableError) Error() string { return e.Layer + ": " + e.Err.Error() }

type FatalError struct {
	Layer string
	Err   error
}

func (e *FatalError) Error() string { return e.Layer + ": " + e.Err.Error() }

type LayerResult struct {
	Layer      int    `json:"layer"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	DurationMs int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
}

type PipelineRun struct {
	ID         string        `json:"id"`
	ConvID     string        `json:"convId"`
	StartedAt  string        `json:"startedAt"`
	EndedAt    string        `json:"endedAt"`
	DurationMs int64         `json:"durationMs"`
	Layers     []LayerResult `json:"layers"`
}

type Pipeline struct {
	layers  []PipelineLayer
	lastRun *PipelineRun
	mu      sync.RWMutex
}

func NewPipeline(layers ...PipelineLayer) *Pipeline {
	return &Pipeline{layers: layers}
}

func (p *Pipeline) Execute(ctx context.Context, convID string, messages []map[string]string, newReply string) {
	run := &PipelineRun{
		ID:        time.Now().Format("20060102150405") + "-" + convID[:min(8, len(convID))],
		ConvID:    convID,
		StartedAt: time.Now().Format("2006-01-02 15:04:05"),
		Layers:    make([]LayerResult, 0, len(p.layers)),
	}
	for i, layer := range p.layers {
		lr := LayerResult{
			Layer: i + 1,
			Name:  layer.Name(),
		}
		start := time.Now()
		err := layer.Process(ctx, convID, messages, newReply)
		elapsed := time.Since(start).Milliseconds()
		lr.DurationMs = elapsed
		switch {
		case err == nil:
			lr.Status = "completed"
		case err == ErrSkip:
			lr.Status = "skipped"
		default:
			lr.Status = "failed"
			lr.Error = err.Error()
		}
		run.Layers = append(run.Layers, lr)
	}
	run.EndedAt = time.Now().Format("2006-01-02 15:04:05")
	run.DurationMs = time.Since(time.Now().Add(-time.Duration(run.DurationMs) * time.Millisecond)).Milliseconds()

	// Recalculate total
	var total int64
	for _, lr := range run.Layers {
		total += lr.DurationMs
	}
	run.DurationMs = total

	p.mu.Lock()
	p.lastRun = run
	p.mu.Unlock()
}

func (p *Pipeline) LastRun() *PipelineRun {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.lastRun == nil {
		return nil
	}
	cp := *p.lastRun
	cp.Layers = make([]LayerResult, len(p.lastRun.Layers))
	copy(cp.Layers, p.lastRun.Layers)
	return &cp
}

var globalPipeline *Pipeline

func SetGlobalPipeline(p *Pipeline) { globalPipeline = p }

func GetGlobalPipeline() *Pipeline { return globalPipeline }
