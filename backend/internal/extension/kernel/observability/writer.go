package observability

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type RecordWriter interface {
	WriteOperation(ctx context.Context, op OperationRecord) error
	WriteInvocation(ctx context.Context, inv InvocationRecord) error
	WriteAttempt(ctx context.Context, att ExecutionAttempt) error
	WriteRuntimeEvent(ctx context.Context, evt RuntimeEventRecord) error
	WriteAuditEvent(ctx context.Context, evt AuditEvent) error
	WriteErrorRecord(ctx context.Context, rec ErrorRecord) error
	Flush(ctx context.Context) error
	Close() error
}

type WriterConfig struct {
	HighRiskSyncWrite bool
	BatchSize         int
	FlushInterval     int
	MaxQueueSize      int
}

func DefaultWriterConfig() WriterConfig {
	return WriterConfig{
		HighRiskSyncWrite: true,
		BatchSize:         100,
		FlushInterval:     5,
		MaxQueueSize:      10000,
	}
}

type writerEntry struct {
	entryType string
	data      any
}

type DefaultRecordWriter struct {
	store  StorageBackend
	config WriterConfig
	queue  chan writerEntry
}

func NewRecordWriter(store StorageBackend, config WriterConfig) *DefaultRecordWriter {
	return &DefaultRecordWriter{
		store:  store,
		config: config,
		queue:  make(chan writerEntry, config.MaxQueueSize),
	}
}

func (w *DefaultRecordWriter) WriteOperation(ctx context.Context, op OperationRecord) error {
	return w.enqueueOrWrite(ctx, "operation", op, false)
}

func (w *DefaultRecordWriter) WriteInvocation(ctx context.Context, inv InvocationRecord) error {
	return w.enqueueOrWrite(ctx, "invocation", inv, false)
}

func (w *DefaultRecordWriter) WriteAttempt(ctx context.Context, att ExecutionAttempt) error {
	return w.enqueueOrWrite(ctx, "attempt", att, false)
}

func (w *DefaultRecordWriter) WriteRuntimeEvent(ctx context.Context, evt RuntimeEventRecord) error {
	return w.enqueueOrWrite(ctx, "event", evt, false)
}

func (w *DefaultRecordWriter) WriteAuditEvent(ctx context.Context, evt AuditEvent) error {
	isHighRisk := evt.RiskLevel == "high" || evt.RiskLevel == "critical"
	syncWrite := isHighRisk && w.config.HighRiskSyncWrite
	return w.enqueueOrWrite(ctx, "audit", evt, syncWrite)
}

func (w *DefaultRecordWriter) WriteErrorRecord(ctx context.Context, rec ErrorRecord) error {
	return w.enqueueOrWrite(ctx, "error", rec, false)
}

func (w *DefaultRecordWriter) enqueueOrWrite(ctx context.Context, entryType string, data any, syncWrite bool) error {
	if syncWrite {
		return w.writeDirect(ctx, entryType, data)
	}

	select {
	case w.queue <- writerEntry{entryType: entryType, data: data}:
		return nil
	default:
		return w.writeDirect(ctx, entryType, data)
	}
}

func (w *DefaultRecordWriter) writeDirect(ctx context.Context, entryType string, data any) error {
	switch entryType {
	case "operation":
		if op, ok := data.(OperationRecord); ok {
			return w.store.SaveOperation(ctx, op)
		}
	case "invocation":
		if inv, ok := data.(InvocationRecord); ok {
			return w.store.SaveInvocation(ctx, inv)
		}
	case "attempt":
		if att, ok := data.(ExecutionAttempt); ok {
			return w.store.SaveAttempt(ctx, att)
		}
	case "event":
		if evt, ok := data.(RuntimeEventRecord); ok {
			return w.store.SaveRuntimeEvent(ctx, evt)
		}
	case "audit":
		if evt, ok := data.(AuditEvent); ok {
			return w.store.SaveAuditEvent(ctx, evt)
		}
	case "error":
		if rec, ok := data.(ErrorRecord); ok {
			return w.store.SaveErrorRecord(ctx, rec)
		}
	}
	return fmt.Errorf("observability: unknown entry type %q", entryType)
}

func (w *DefaultRecordWriter) Flush(ctx context.Context) error {
	for {
		select {
		case entry := <-w.queue:
			if err := w.writeDirect(ctx, entry.entryType, entry.data); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

func (w *DefaultRecordWriter) Close() error {
	close(w.queue)
	return nil
}

func HashInput(data string) string {
	if data == "" {
		return ""
	}
	h := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", h)
}

func HashOutput(data string) string {
	return HashInput(data)
}

func NewTraceID() string {
	return capability.NewTraceID()
}

func NewOperationID() string {
	return capability.NewOperationID()
}

func NewInvocationID() string {
	return capability.NewInvocationID()
}

func NewAttemptID() string {
	return capability.NewInvocationID()
}

func NewEventID() string {
	return capability.NewInvocationID()
}

func NewAuditID() string {
	return capability.NewInvocationID()
}

func NewErrorID() string {
	return capability.NewInvocationID()
}
