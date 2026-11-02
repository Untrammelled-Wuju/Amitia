package business

import (
	"context"
	"time"

	"github.com/google/uuid"
	amitialog "log"
)

func (e *Engine) RunMemoryConsolidationWorker(ctx context.Context, base Request) {
	timer := time.NewTimer(2 * time.Minute)
	defer timer.Stop()
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			e.runMemoryConsolidationPass(ctx, base)
		case <-ticker.C:
			e.runMemoryConsolidationPass(ctx, base)
		}
	}
}

func (e *Engine) runMemoryConsolidationPass(ctx context.Context, base Request) {
	request := base
	request.RequestID = uuid.NewString()
	roleCtx, scope, finish, err := e.coordination.Begin(ctx, request.SpaceID, request.DeviceID, request.TargetDeviceID, request.CoreID, "", request.RequestID)
	if err != nil {
		amitialog.Printf("[original-error] %v", err)
		return
	}
	roles, err := e.data.Roles(roleCtx, scope)
	finish()
	if err != nil {
		amitialog.Printf("[original-error] %v", err)
		return
	}
	for _, role := range roles {
		request.RoleID, request.RequestID = role.ID, uuid.NewString()
		runCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		_, err := e.ConsolidateOwnedMemories(runCtx, request)
		cancel()
		if err != nil && ctx.Err() == nil {
			amitialog.Printf("[original-error] role=%s %v", role.ID, err)
		}
	}
}
