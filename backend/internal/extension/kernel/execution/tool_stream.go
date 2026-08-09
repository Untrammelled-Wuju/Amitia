package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

var ErrToolStreamClosed = fmt.Errorf("tool stream session closed")

func newToolStreamSession(invocationID string, sink capability.ToolStreamSink, policy capability.ToolStreamingPolicy, sanitizer *Sanitizer, cancelCtrl *CancellationController) *toolStreamSession {
	return &toolStreamSession{
		invocationID: invocationID,
		sink:         sink,
		policy:       policy,
		sanitizer:    sanitizer,
		cancelCtrl:   cancelCtrl,
		abortFired:   false,
	}
}

type toolStreamSession struct {
	mu           sync.Mutex
	invocationID string
	sink         capability.ToolStreamSink
	nextSequence uint64
	eventCount   int
	totalBytes   int64
	visible      bool
	closed       bool
	deliveryErr  error
	policy       capability.ToolStreamingPolicy
	sanitizer    *Sanitizer
	cancelCtrl   *CancellationController
	abortFired   bool
}

var _ capability.ToolStreamEmitter = (*toolStreamSession)(nil)

func (s *toolStreamSession) Emit(ctx context.Context, emission capability.ToolStreamEmission) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("%w: stream already closed", ErrToolStreamClosed)
	}

	if s.deliveryErr != nil {
		return s.deliveryErr
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%w: context done: %s", ErrToolStreamClosed, err.Error())
	}

	if emission.Type == capability.ToolStreamEventTerminal {
		return s.recordDeliveryErr(&capability.ToolError{
			Code:     capability.ErrorCodeStreamProtocol,
			Category: capability.ToolErrorCategoryStream,
			Message:  "runtime cannot emit terminal event",
		})
	}

	if !emission.Type.Valid() {
		return s.recordDeliveryErr(&capability.ToolError{
			Code:     capability.ErrorCodeStreamProtocol,
			Category: capability.ToolErrorCategoryStream,
			Message:  "invalid stream event type",
		})
	}

	if emission.Type == capability.ToolStreamEventContent {
		if emission.Content == nil {
			return s.recordDeliveryErr(&capability.ToolError{
				Code:     capability.ErrorCodeStreamProtocol,
				Category: capability.ToolErrorCategoryStream,
				Message:  "content event requires content",
			})
		}
		if emission.Progress != nil {
			return s.recordDeliveryErr(&capability.ToolError{
				Code:     capability.ErrorCodeStreamProtocol,
				Category: capability.ToolErrorCategoryStream,
				Message:  "content event must not have progress",
			})
		}
	}

	if emission.Type == capability.ToolStreamEventProgress {
		if emission.Progress == nil {
			return s.recordDeliveryErr(&capability.ToolError{
				Code:     capability.ErrorCodeStreamProtocol,
				Category: capability.ToolErrorCategoryStream,
				Message:  "progress event requires progress",
			})
		}
		if emission.Content != nil {
			return s.recordDeliveryErr(&capability.ToolError{
				Code:     capability.ErrorCodeStreamProtocol,
				Category: capability.ToolErrorCategoryStream,
				Message:  "progress event must not have content",
			})
		}
	}

	if emission.Content != nil && emission.Content.Type == capability.ToolContentStructured && len(emission.Content.Data) > 0 {
		if !json.Valid(emission.Content.Data) {
			return s.recordDeliveryErr(&capability.ToolError{
				Code:     capability.ErrorCodeStreamProtocol,
				Category: capability.ToolErrorCategoryStream,
				Message:  "structured content is not valid JSON",
			})
		}
	}

	if emission.Content != nil && emission.Content.Type == capability.ToolContentStream {
		return s.recordDeliveryErr(&capability.ToolError{
			Code:     capability.ErrorCodeStreamProtocol,
			Category: capability.ToolErrorCategoryStream,
			Message:  "stream content type not allowed in stream emission",
		})
	}

	if emission.Progress != nil && !emission.Progress.Indeterminate {
		f := emission.Progress.Fraction
		if math.IsNaN(f) || math.IsInf(f, 0) || f < 0 || f > 1 {
			return s.recordDeliveryErr(&capability.ToolError{
				Code:     capability.ErrorCodeStreamProtocol,
				Category: capability.ToolErrorCategoryStream,
				Message:  "progress fraction must be between 0 and 1",
			})
		}
	}

	s.nextSequence++
	event := capability.ToolStreamEvent{
		InvocationID: s.invocationID,
		Sequence:     s.nextSequence,
		Type:         emission.Type,
		Metadata:     cloneStringMap(emission.Metadata),
	}

	if emission.Content != nil {
		contentCopy := *emission.Content
		if emission.Content.Data != nil {
			contentCopy.Data = append(json.RawMessage(nil), emission.Content.Data...)
		}
		event.Content = &contentCopy
	}

	if emission.Progress != nil {
		progressCopy := *emission.Progress
		event.Progress = &progressCopy
	}

	if s.sanitizer != nil {
		event.Content = s.sanitizer.sanitizeStreamContent(event.Content)
	}

	payloadSize := estimateStreamEventSize(event)
	if payloadSize > s.policy.EffectiveMaxEventBytes() {
		return s.recordDeliveryErr(&capability.ToolError{
			Code:     capability.ErrorCodeStreamLimitExceeded,
			Category: capability.ToolErrorCategoryStream,
			Message:  fmt.Sprintf("event size %d exceeds limit %d", payloadSize, s.policy.EffectiveMaxEventBytes()),
		})
	}

	maxEvents := s.policy.EffectiveMaxEvents()
	if maxEvents > 0 && s.eventCount >= maxEvents {
		return s.recordDeliveryErr(&capability.ToolError{
			Code:     capability.ErrorCodeStreamLimitExceeded,
			Category: capability.ToolErrorCategoryStream,
			Message:  fmt.Sprintf("max events %d exceeded", maxEvents),
		})
	}

	maxBytes := s.policy.EffectiveMaxTotalBytes()
	if maxBytes > 0 && s.totalBytes+int64(payloadSize) > maxBytes {
		return s.recordDeliveryErr(&capability.ToolError{
			Code:     capability.ErrorCodeStreamLimitExceeded,
			Category: capability.ToolErrorCategoryStream,
			Message:  fmt.Sprintf("max total stream bytes %d exceeded", maxBytes),
		})
	}

	err := s.sink.Emit(ctx, event)
	if err != nil {
		toolErr := &capability.ToolError{
			Code:     capability.ErrorCodeStreamDeliveryFailed,
			Category: capability.ToolErrorCategoryStream,
			Message:  fmt.Sprintf("stream delivery failed: %s", err.Error()),
		}
		return s.recordDeliveryErr(toolErr)
	}

	s.visible = true
	s.eventCount++
	s.totalBytes += int64(payloadSize)
	return nil
}

func (s *toolStreamSession) Finish(ctx context.Context, result capability.UnifiedToolResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	if s.deliveryErr != nil {
		return nil
	}

	s.nextSequence++
	event := capability.ToolStreamEvent{
		InvocationID: s.invocationID,
		Sequence:     s.nextSequence,
		Type:         capability.ToolStreamEventTerminal,
		Result:       &result,
	}

	err := s.sink.Emit(ctx, event)
	if err != nil {
		s.deliveryErr = &capability.ToolError{
			Code:     capability.ErrorCodeStreamDeliveryFailed,
			Category: capability.ToolErrorCategoryStream,
			Message:  fmt.Sprintf("terminal delivery failed: %s", err.Error()),
		}
		return s.deliveryErr
	}

	s.eventCount++
	return nil
}

func (s *toolStreamSession) HasVisibleOutput() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.visible
}

func (s *toolStreamSession) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deliveryErr
}

func (s *toolStreamSession) StreamEventCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.eventCount
}

func (s *toolStreamSession) recordDeliveryErr(err *capability.ToolError) error {
	s.deliveryErr = err
	s.tryAbortStream()
	return err
}

func (s *toolStreamSession) tryAbortStream() {
	if s.abortFired || s.cancelCtrl == nil {
		return
	}
	s.abortFired = true
	reason := capability.ToolCancellationReason{
		Code: capability.CancellationReasonStreamConsumerGone,
	}
	go s.cancelCtrl.CancelInvocation(context.Background(), s.invocationID, reason)
}

func cloneStringMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func estimateStreamEventSize(event capability.ToolStreamEvent) int {
	size := 256
	if event.Content != nil {
		size += len(event.Content.Text)
		size += len(event.Content.Data)
		size += len(event.Content.URI)
		size += len(event.Content.MIMEType)
	}
	if event.Result != nil {
		size += len(event.Result.Structured)
		for _, c := range event.Result.Content {
			size += len(c.Text)
			size += len(c.Data)
		}
	}
	return size
}
