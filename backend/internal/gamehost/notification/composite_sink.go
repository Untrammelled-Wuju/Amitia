package notification

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

type CompositeSink struct {
	mu    sync.RWMutex
	sinks []NotificationSink
}

func NewCompositeSink(sinks ...NotificationSink) *CompositeSink {
	return &CompositeSink{sinks: append([]NotificationSink(nil), sinks...)}
}

func (s *CompositeSink) Add(sink NotificationSink) {
	if s == nil || sink == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sinks = append(s.sinks, sink)
}

func (s *CompositeSink) Publish(ctx context.Context, n Notification) error {
	s.mu.RLock()
	sinks := append([]NotificationSink(nil), s.sinks...)
	s.mu.RUnlock()
	if len(sinks) == 0 {
		return nil
	}
	var errs []string
	for i, sink := range sinks {
		if sink == nil {
			continue
		}
		if err := sink.Publish(ctx, n); err != nil {
			errs = append(errs, fmt.Sprintf("sink[%d]: %v", i, err))
		}
	}
	if len(errs) > 0 {
		return errors.New("composite: " + strings.Join(errs, "; "))
	}
	return nil
}
