package devicemesh

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	meshaudit "github.com/u-ai/backend/internal/devicemesh/audit"
)

func (rt *Runtime) StartSecurityAudit(canonical *sql.DB, space string) error {
	rt.deliveryMu.Lock()
	defer rt.deliveryMu.Unlock()
	if rt.auditCancel != nil {
		return nil
	}
	if canonical == nil || rt.DB == nil || space == "" {
		return errors.New("安全审计生产依赖缺失")
	}
	rows, err := canonical.Query(`SELECT event_id FROM security_audit_events WHERE 1=0`)
	if err != nil {
		return err
	}
	rows.Close()
	ctx, cancel := context.WithCancel(context.Background())
	rt.auditCancel = cancel
	rt.auditDone = make(chan struct{})
	bridge := meshaudit.Bridge{Kernel: rt.DB, Canonical: canonical, SpaceID: space}
	go func() {
		defer close(rt.auditDone)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			if err := bridge.Flush(ctx); err != nil && ctx.Err() == nil {
				log.Printf("device-mesh security audit delivery: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return nil
}
