package interaction

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/mindruntime"
)

type descriptorRefStaleChecker struct {
	service     *RecoveryDescriptorService
	settleDelay time.Duration
	now         func() time.Time
}

func NewDescriptorRefStaleChecker(service *RecoveryDescriptorService, settleDelay time.Duration) mindruntime.ReconciliationChecker {
	if settleDelay <= 0 {
		settleDelay = mindruntime.DefaultAgentFactSettleDelay()
	}
	return &descriptorRefStaleChecker{
		service:     service,
		settleDelay: settleDelay,
		now:         func() time.Time { return time.Now().UTC() },
	}
}

func (c *descriptorRefStaleChecker) CheckReconciliation(ctx context.Context, req mindruntime.ReconciliationCheckRequest) ([]mindruntime.ReconciliationDiff, error) {
	if c == nil || c.service == nil {
		return nil, nil
	}
	filter := req.Scope
	batch := req.BatchSize
	if batch <= 0 {
		batch = 50
	}
	diffs := make([]mindruntime.ReconciliationDiff, 0)
	scanned := 0
	stopped := false
	err := c.service.tracker.Range(ctx, func(record *InteractionRecord) bool {
		if stopped {
			return false
		}
		if record == nil || record.RecoveryDescriptor == nil {
			return true
		}
		if !descriptorScopeMatch(filter, record.Scope) {
			return true
		}
		desc := record.RecoveryDescriptor
		if !interactionOnly(filter) && !c.settleReady(desc.UpdatedAt) {
			return true
		}
		if scanned >= batch {
			stopped = true
			return false
		}
		result, vErr := c.service.Validate(ctx, *desc)
		if vErr != nil {
			diffs = append(diffs, mindruntime.ReconciliationDiff{
				ScanID:         req.ScanID,
				Source:         "agent_descriptor",
				Target:         string(mindruntime.ReconciliationAgentDescriptorRefStale),
				DiffType:       "descriptor_validate_error",
				SourceKey:      safeInteractionKey(record.ID),
				TargetKey:      safeInteractionKey(record.ID),
				Description:    fmt.Sprintf("descriptor validation error for interaction %s: %v", record.ID, vErr),
				Severity:       "critical",
				AutoRepairable: false,
				RepairAction:   "manual_confirm",
				FoundAt:        c.now(),
			})
			scanned++
			return true
		}
		diffs = append(diffs, c.toDiffs(req.ScanID, record.ID, result)...)
		scanned++
		return true
	})
	if err != nil {
		return nil, err
	}
	return diffs, nil
}

func (c *descriptorRefStaleChecker) toDiffs(scanID string, interactionID string, result RecoveryValidationResult) []mindruntime.ReconciliationDiff {
	if len(result.Issues) == 0 {
		return nil
	}
	diffs := make([]mindruntime.ReconciliationDiff, 0, len(result.Issues))
	for _, i := range result.Issues {
		diffType := string(i.Code)
		switch i.Code {
		case IssueDescriptorVersionUnsupported, IssueDescriptorCorrupt:
			diffType = "schema_" + string(i.Code)
		}
		diffs = append(diffs, mindruntime.ReconciliationDiff{
			ScanID:         scanID,
			Source:         "agent_descriptor",
			Target:         string(mindruntime.ReconciliationAgentDescriptorRefStale),
			DiffType:       diffType,
			SourceKey:      safeInteractionKey(i.ReferenceID),
			TargetKey:      safeInteractionKey(interactionID),
			Description:    fmt.Sprintf("descriptor %s in interaction %s: refType=%s refId=%s", i.Code, interactionID, i.ReferenceType, i.ReferenceID),
			Severity:       i.Severity,
			AutoRepairable: false,
			RepairAction:   "manual_confirm",
			FoundAt:        c.now(),
		})
	}
	return diffs
}

func descriptorScopeMatch(filter *mindruntime.ReconciliationScope, scope InteractionScope) bool {
	if filter == nil {
		return true
	}
	norm := scope.Normalize()
	switch {
	case filter.UserID != "" && norm.UserID != filter.UserID:
		return false
	case filter.CharacterID != "" && norm.CharacterID != filter.CharacterID:
		return false
	case filter.ConversationID != "" && norm.ConversationID != filter.ConversationID:
		return false
	}
	return true
}

func interactionOnly(filter *mindruntime.ReconciliationScope) bool {
	if filter == nil {
		return false
	}
	return filter.InteractionID != ""
}

func (c *descriptorRefStaleChecker) settleReady(ts time.Time) bool {
	if ts.IsZero() {
		return false
	}
	cutoff := c.now().Add(-c.settleDelay)
	return !ts.After(cutoff)
}

func safeInteractionKey(s string) string {
	if s == "" {
		return "-"
	}
	if len(s) > 64 {
		return s[:64]
	}
	return s
}
