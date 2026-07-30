// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import (
	"context"

	"github.com/u-ai/backend/log"
)

type NoopEventPublisher struct{}

func NewNoopEventPublisher() *NoopEventPublisher {
	return &NoopEventPublisher{}
}

func (p *NoopEventPublisher) PublishQualityEvent(ctx context.Context, event QualityEvent) error {
	return nil
}

type LogEventPublisher struct{}

func NewLogEventPublisher() *LogEventPublisher {
	return &LogEventPublisher{}
}

func (p *LogEventPublisher) PublishQualityEvent(ctx context.Context, event QualityEvent) error {
	log.Logger.Infof("quality event: jobId=%s task=%s actionKey=%s eval=%s stage=%s progress=%d status=%s message=%s",
		event.JobID, event.ProcessingTaskID, event.ActionKey, event.EvaluationID,
		event.Stage, event.Progress, event.Status, event.Message)
	return nil
}
