package execution

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func NewMetricsRecorder() *MetricsRecorder {
	return &MetricsRecorder{
		counts: make(map[string]*MetricCounter),
	}
}

type MetricCounter struct {
	TotalCalls     int64
	SuccessCalls   int64
	FailedCalls    int64
	DeniedCalls    int64
	CancelledCalls int64
	TimedOutCalls  int64
	TotalDuration  time.Duration
	MaxDuration    time.Duration
	MinDuration    time.Duration
}

type MetricsRecorder struct {
	counts map[string]*MetricCounter
	mu     sync.RWMutex
}

func (m *MetricsRecorder) Record(ctx context.Context, tool capability.ToolDefinition, result capability.UnifiedToolResult, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	toolKey := tool.ID
	if tool.ExtensionID != "" {
		toolKey = tool.ExtensionID + "/" + tool.ID
	}

	c, ok := m.counts[toolKey]
	if !ok {
		c = &MetricCounter{}
		m.counts[toolKey] = c
	}

	c.TotalCalls++
	c.TotalDuration += duration
	if duration > c.MaxDuration {
		c.MaxDuration = duration
	}
	if c.MinDuration == 0 || duration < c.MinDuration {
		c.MinDuration = duration
	}

	switch result.Status {
	case capability.ToolResultStatusSuccess:
		c.SuccessCalls++
	case capability.ToolResultStatusFailed:
		c.FailedCalls++
	case capability.ToolResultStatusCancelled:
		c.CancelledCalls++
	case capability.ToolResultStatusTimedOut:
		c.TimedOutCalls++
	}
}

func (m *MetricsRecorder) RecordDenied(ctx context.Context, toolID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.counts[toolID]
	if !ok {
		c = &MetricCounter{}
		m.counts[toolID] = c
	}
	c.DeniedCalls++
}

func (m *MetricsRecorder) GetCounters() map[string]*MetricCounter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*MetricCounter, len(m.counts))
	for k, v := range m.counts {
		result[k] = v
	}
	return result
}

func (m *MetricsRecorder) GetCounter(toolID string) *MetricCounter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.counts[toolID]
}
