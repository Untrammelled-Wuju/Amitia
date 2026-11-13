package main

import (
	"context"
	"log"
	"time"

	"github.com/u-ai/backend/internal/character"
)

func runRolePurgeWorker(ctx context.Context, services *AppServices) {
	if services == nil || services.DB == nil {
		return
	}
	if err := character.EnsureRolePurgeJobsSchema(services.DB); err != nil {
		log.Printf("[original-error] 初始化角色数据清理任务表失败: %v", err)
		return
	}
	purger := roleDataPurger{services: services}
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	sweepTimer := time.NewTimer(60 * time.Second)
	defer sweepTimer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-sweepTimer.C:
			if count := character.SweepOrphanRolePurgeJobs(ctx, services.DB, purger); count > 0 {
				log.Printf("[RoleDataPurge] 发现 %d 个需要补偿的角色数据清理任务", count)
			}
			character.ProcessDueRolePurgeJobs(ctx, services.DB, purger, 20)
		case <-ticker.C:
			character.ProcessDueRolePurgeJobs(ctx, services.DB, purger, 20)
		case <-character.RolePurgeWake():
			character.ProcessDueRolePurgeJobs(ctx, services.DB, purger, 20)
		}
	}
}
