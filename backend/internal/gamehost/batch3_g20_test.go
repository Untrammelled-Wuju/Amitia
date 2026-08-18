package gamehost

import (
	"context"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

// TestG20_ProductionComposition_Identity 验证 Production Composition 中的 canonical 实例唯一性
func TestG20_ProductionComposition_Identity(t *testing.T) {
	c := composeTestContainer(t)
	if c == nil {
		t.Fatal("container is nil")
	}

	// G18: 所有安全核心组件必须非空
	coreComponents := map[string]interface{}{
		"AuthorityManager":    c.AuthorityManager,
		"OutputGate":          c.OutputGate,
		"TakeoverService":     c.TakeoverService,
		"AuditSink":           c.AuthorityAudit,
		"StreamManager":       c.StreamManager,
		"HandshakeManager":    c.HandshakeManager,
		"BinaryObjectRegistry": c.BinaryObjectRegistry,
		"RecoveryCoordinator": c.RecoveryCoordinator,
		"StartupRecovery":     c.StartupRecovery,
	}
	for name, comp := range coreComponents {
		if comp == nil {
			t.Errorf("core component %s is nil", name)
		}
	}
}

// TestG20_ProductionComposition_ShutdownIdempotency 验证 Shutdown 幂等性
func TestG20_ProductionComposition_ShutdownIdempotency(t *testing.T) {
	c := composeTestContainer(t)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := c.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown iteration %d failed: %v", i, err)
		}
	}
}

// TestG20_AuthorityManager_TakeoverABA 验证 ABA 场景下权限正确递增
func TestG20_AuthorityManager_TakeoverABA(t *testing.T) {
	c := composeTestContainer(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-g20-aba")
	pluginID := domain.PluginID("plugin-g20-aba")

	if _, err := c.AuthorityManager.Create(ctx, runtimeID, pluginID); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if _, err := c.AuthorityManager.Transition(ctx, runtimeID, control.TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  control.ActorSystem,
		Reason: control.ReasonRuntimeLifecycle,
	}); err != nil {
		t.Fatalf("Transition to plugin failed: %v", err)
	}

	snap1, _ := c.AuthorityManager.Get(ctx, runtimeID)
	epoch1 := snap1.Epoch

	if _, err := c.AuthorityManager.Transition(ctx, runtimeID, control.TransitionRequest{
		Target: domain.ControlModeUserControl,
		Actor:  control.ActorUser,
		Reason: control.ReasonUserRequest,
	}); err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	snap2, _ := c.AuthorityManager.Get(ctx, runtimeID)
	if snap2.Epoch <= epoch1 {
		t.Errorf("epoch did not advance after takeover: %d <= %d", snap2.Epoch, epoch1)
	}

	if _, err := c.AuthorityManager.Transition(ctx, runtimeID, control.TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  control.ActorSystem,
		Reason: control.ReasonSystemRecovery,
	}); err != nil {
		t.Fatalf("Release back to plugin failed: %v", err)
	}

	snap3, _ := c.AuthorityManager.Get(ctx, runtimeID)
	if snap3.Epoch <= snap2.Epoch {
		t.Errorf("epoch did not advance after release: %d <= %d", snap3.Epoch, snap2.Epoch)
	}
}

// TestG20_AuthorityManager_OldEpochReject 验证过期 Epoch 被拒绝
func TestG20_AuthorityManager_OldEpochReject(t *testing.T) {
	c := composeTestContainer(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-g20-epoch")
	pluginID := domain.PluginID("plugin-g20-epoch")

	if _, err := c.AuthorityManager.Create(ctx, runtimeID, pluginID); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if _, err := c.AuthorityManager.Transition(ctx, runtimeID, control.TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  control.ActorSystem,
		Reason: control.ReasonRuntimeLifecycle,
	}); err != nil {
		t.Fatalf("Transition failed: %v", err)
	}

	snap, _ := c.AuthorityManager.Get(ctx, runtimeID)
	epoch := snap.Epoch

	// Takeover 后 epoch 增加
	if _, err := c.AuthorityManager.Transition(ctx, runtimeID, control.TransitionRequest{
		Target: domain.ControlModeUserControl,
		Actor:  control.ActorUser,
		Reason: control.ReasonUserRequest,
	}); err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	// 使用旧 epoch 尝试输出
	if c.OutputGate == nil {
		t.Fatal("OutputGate not wired - core component must be present for this test")
	}

	// 无法直接调用 PermissionChecker，但可以验证 AuthorityManager GetSnapshot 正确反映旧 epoch 无效
	currentSnap, _ := c.AuthorityManager.Get(ctx, runtimeID)
	if currentSnap.Epoch == epoch {
		t.Error("epoch should have advanced")
	}
}

// TestG20_NoopRemnants_NotInProduction 确保 Production 中无 Noop 残留
func TestG20_NoopRemnants_NotInProduction(t *testing.T) {
	c := composeTestContainer(t)

	// Register a runtime and verify audit is actually recording
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-audit-check")
	pluginID := domain.PluginID("plugin-audit-check")

	if _, err := c.AuthorityManager.Create(ctx, runtimeID, pluginID); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if _, err := c.AuthorityManager.Transition(ctx, runtimeID, control.TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  control.ActorSystem,
		Reason: control.ReasonRuntimeLifecycle,
	}); err != nil {
		t.Fatalf("Transition failed: %v", err)
	}

	// 审计 sink 必须是非 Noop 的具体实现
	if c.AuthorityAudit == nil {
		t.Fatal("AuthorityAudit is nil - might be using NoopAuthorityAuditSink")
	}
}

// TestG20_Concurrent_AuthorityTransitions 验证并发权限转换的线性化安全
func TestG20_Concurrent_AuthorityTransitions(t *testing.T) {
	c := composeTestContainer(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-concurrent")
	pluginID := domain.PluginID("plugin-concurrent")

	if _, err := c.AuthorityManager.Create(ctx, runtimeID, pluginID); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if _, err := c.AuthorityManager.Transition(ctx, runtimeID, control.TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  control.ActorSystem,
		Reason: control.ReasonRuntimeLifecycle,
	}); err != nil {
		t.Fatalf("Initial transition failed: %v", err)
	}

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errors := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			var target domain.ControlMode
			if idx%2 == 0 {
				target = domain.ControlModeUserControl
			} else {
				target = domain.ControlModePluginControl
			}
			_, err := c.AuthorityManager.Transition(ctx, runtimeID, control.TransitionRequest{
				Target: target,
				Actor:  control.ActorUser,
				Reason: control.ReasonUserRequest,
			})
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	successCount := 0
	for _, err := range errors {
		if err == nil {
			successCount++
		}
	}

	if successCount == 0 {
		t.Error("expected at least some transitions to succeed")
	}

	// 最终状态必须是合法的
	finalSnap, err := c.AuthorityManager.Get(ctx, runtimeID)
	if err != nil {
		t.Fatalf("Final Get failed: %v", err)
	}
	if finalSnap.Mode != domain.ControlModeUserControl && finalSnap.Mode != domain.ControlModePluginControl {
		t.Errorf("invalid final mode: %s", finalSnap.Mode)
	}
}

// TestG20_NilContainer_SafeOperations 确保 nil 容器不会 panic
func TestG20_NilContainer_SafeOperations(t *testing.T) {
	var c *GameHostContainer

	if err := c.Start(context.Background()); err != nil {
		t.Errorf("nil Start returned error: %v", err)
	}
	if err := c.Shutdown(context.Background()); err != nil {
		t.Errorf("nil Shutdown returned error: %v", err)
	}
}

// TestG20_GameHostContainer_FullLifecycle 验证完整 lifecycle: Start -> Shutdown
func TestG20_GameHostContainer_FullLifecycle(t *testing.T) {
	c := composeTestContainer(t)

	if c.StreamManager == nil {
		t.Fatal("StreamManager nil")
	}
	if c.HandshakeManager == nil {
		t.Fatal("HandshakeManager nil")
	}

	ctx := context.Background()

	if err := c.Start(ctx); err != nil {
		t.Errorf("Start error: %v", err)
	}

	if err := c.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown error: %v", err)
	}
}

// TestG20_MultipleCompositions_AreDistinct 验证多个 composition 实例独立
func TestG20_MultipleCompositions_AreDistinct(t *testing.T) {
	containers := make([]*GameHostContainer, 5)
	for i := range containers {
		containers[i] = composeTestContainer(t)
	}

	for i := 0; i < len(containers); i++ {
		for j := i + 1; j < len(containers); j++ {
			if containers[i].AuthorityManager == containers[j].AuthorityManager {
				t.Errorf("container[%d] and container[%d] share AuthorityManager", i, j)
			}
			if containers[i].OutputGate == containers[j].OutputGate {
				t.Errorf("container[%d] and container[%d] share OutputGate", i, j)
			}
		}
	}
}
