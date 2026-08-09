package notification

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type CompositeSink struct {
	sinks []NotificationSink
}

func NewCompositeSink(sinks ...NotificationSink) *CompositeSink {
	return &CompositeSink{sinks: sinks}
}

func (s *CompositeSink) Publish(ctx context.Context, n Notification) error {
	if len(s.sinks) == 0 {
		return nil
	}
	var errs []string
	for i, sink := range s.sinks {
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
