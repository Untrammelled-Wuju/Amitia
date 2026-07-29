package observability

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

type ExportFormat string

const (
	ExportFormatJSON ExportFormat = "json"
	ExportFormatCSV  ExportFormat = "csv"
)

type ExportOptions struct {
	Format        ExportFormat
	IncludeSecret bool
	Since         *time.Time
	Until         *time.Time
	MaxRecords    int
}

type AuditExporter struct {
	store StorageBackend
}

func NewAuditExporter(store StorageBackend) *AuditExporter {
	return &AuditExporter{store: store}
}

func (e *AuditExporter) ExportAuditEvents(ctx context.Context, w io.Writer, opts ExportOptions) error {
	filter := AuditFilter{
		Since:       opts.Since,
		Until:       opts.Until,
		ListOptions: ListOptions{Limit: opts.MaxRecords},
	}

	events, _, err := e.store.ListAuditEvents(ctx, filter)
	if err != nil {
		return fmt.Errorf("observability: export failed: %w", err)
	}

	switch opts.Format {
	case ExportFormatJSON:
		return e.exportJSON(w, events, opts)
	case ExportFormatCSV:
		return e.exportCSV(w, events, opts)
	default:
		return fmt.Errorf("observability: unsupported export format: %s", opts.Format)
	}
}

func (e *AuditExporter) exportJSON(w io.Writer, events []AuditEvent, opts ExportOptions) error {
	if !opts.IncludeSecret {
		for i := range events {
			events[i].Metadata = sanitizeExportMetadata(events[i].Metadata)
		}
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(events)
}

func (e *AuditExporter) exportCSV(w io.Writer, events []AuditEvent, opts ExportOptions) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	header := []string{"audit_id", "trace_id", "operation_id", "invocation_id", "actor_type", "actor_id",
		"subject_type", "subject_id", "action", "decision", "risk_level", "result", "error_code", "created_at"}
	if err := writer.Write(header); err != nil {
		return err
	}

	for _, evt := range events {
		row := []string{
			evt.AuditID, evt.TraceID, evt.OperationID, evt.InvocationID,
			string(evt.ActorType), evt.ActorID,
			string(evt.SubjectType), evt.SubjectID,
			evt.Action, evt.Decision, evt.RiskLevel,
			evt.Result, evt.ErrorCode,
			evt.CreatedAt.Format(time.RFC3339),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

func (e *AuditExporter) ExportInvocationChain(ctx context.Context, w io.Writer, rootInvocationID string, opts ExportOptions) error {
	tree, err := NewQueryService(e.store).GetInvocationTree(ctx, rootInvocationID)
	if err != nil {
		return fmt.Errorf("observability: export chain failed: %w", err)
	}

	switch opts.Format {
	case ExportFormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(tree)
	case ExportFormatCSV:
		return e.exportInvocationTreeCSV(w, tree)
	default:
		return fmt.Errorf("observability: unsupported export format: %s", opts.Format)
	}
}

func (e *AuditExporter) exportInvocationTreeCSV(w io.Writer, tree *InvocationNode) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	header := []string{"invocation_id", "parent_id", "capability_id", "status", "error_code", "side_effect_count", "created_at"}
	if err := writer.Write(header); err != nil {
		return err
	}

	e.writeNodeCSV(writer, tree, "")
	return nil
}

func (e *AuditExporter) writeNodeCSV(writer *csv.Writer, node *InvocationNode, parentID string) {
	inv := node.Invocation
	row := []string{
		inv.InvocationID, parentID, inv.CapabilityID,
		string(inv.Status), inv.ErrorCode,
		fmt.Sprintf("%d", inv.SideEffectCount),
		inv.CreatedAt.Format(time.RFC3339),
	}
	_ = writer.Write(row)

	for i := range node.Children {
		e.writeNodeCSV(writer, &node.Children[i], inv.InvocationID)
	}
}

func sanitizeExportMetadata(meta map[string]any) map[string]any {
	if meta == nil {
		return nil
	}
	cleaned := make(map[string]any)
	redactedKeys := map[string]bool{
		"token": true, "api_key": true, "secret": true, "password": true,
		"authorization": true, "cookie": true, "credential": true,
	}
	for k, v := range meta {
		lower := strings.ToLower(k)
		if redactedKeys[lower] {
			cleaned[k] = "[redacted]"
		} else {
			cleaned[k] = v
		}
	}
	return cleaned
}
