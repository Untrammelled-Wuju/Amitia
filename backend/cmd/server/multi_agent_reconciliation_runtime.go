package main

import (
	"context"
	"time"

	"github.com/u-ai/backend/log"
)

func runMultiAgentReconciliation(ctx context.Context, interval time.Duration, reconcile func(context.Context) error) {
	if ctx == nil || reconcile == nil {
		return
	}
	if interval <= 0 {
		interval = 4 * time.Second
	}
	lastError := ""
	run := func() {
		if ctx.Err() != nil {
			return
		}
		checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := reconcile(checkCtx); err != nil {
			if err.Error() != lastError {
				log.Warn("multi_agent.reconciliation.requires_attention: ", err)
				lastError = err.Error()
			}
		} else {
			lastError = ""
		}
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
