package mindruntime

import (
	"context"
	"time"
)

type ReconciliationRepairer interface {
	Repair(ctx context.Context, diff ReconciliationDiff) error
}

type noopRepairer struct{}

func (noopRepairer) Repair(ctx context.Context, diff ReconciliationDiff) error {
	return nil
}

var defaultNoopRepairer ReconciliationRepairer = noopRepairer{}

func NowUTC() time.Time {
	return time.Now().UTC()
}
