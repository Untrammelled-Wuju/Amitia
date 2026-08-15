package nativebridge

import (
	"fmt"
	"log"
	"time"
)

type NativeTraceContext struct {
	RequestID     string
	Platform      string
	Operation     string
	Generation    uint64
	ConnectionID  uint64
	StartTime     time.Time
}

func (c *NativeTraceContext) Duration() time.Duration {
	return time.Since(c.StartTime)
}

func (c *NativeTraceContext) LogSuccess(extra string) {
	log.Printf("[nativebridge] requestId=%s platform=%s operation=%s generation=%d conn=%d duration=%s %s",
		c.RequestID, c.Platform, c.Operation, c.Generation, c.ConnectionID, c.Duration(), extra)
}

func (c *NativeTraceContext) LogError(code, message string) {
	log.Printf("[nativebridge] requestId=%s platform=%s operation=%s generation=%d conn=%d duration=%s error=%s: %s",
		c.RequestID, c.Platform, c.Operation, c.Generation, c.ConnectionID, c.Duration(), code, message)
}

func (c *NativeTraceContext) LogEvent(eventType string) {
	log.Printf("[nativebridge] event=%s platform=%s generation=%d event_type=%s",
		c.RequestID, c.Platform, c.Generation, eventType)
}

func (c *NativeTraceContext) String() string {
	return fmt.Sprintf("requestId=%s platform=%s operation=%s generation=%d conn=%d",
		c.RequestID, c.Platform, c.Operation, c.Generation, c.ConnectionID)
}

func NewTraceContext(requestID, platform, operation string, generation uint64, connID uint64) *NativeTraceContext {
	return &NativeTraceContext{
		RequestID:    requestID,
		Platform:     platform,
		Operation:    operation,
		Generation:   generation,
		ConnectionID: connID,
		StartTime:    time.Now().UTC(),
	}
}
