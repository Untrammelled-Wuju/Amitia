package decision

import (
	"sync"
	"testing"
	"time"
)

func TestGoalRegistryRegisterAndGet(t *testing.T) {
	registry := NewGoalRegistry()
	goal := Goal{
		ID:        "goal-1",
		UserID:    "user-1",
		Type:      GoalTypeConnection,
		Priority:  GoalPriorityHigh,
		Status:    GoalStatusActive,
		Progress:  0.3,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	registry.Register(goal)

	retrieved, ok := registry.Get("goal-1")
	if !ok {
		t.Fatal("已注册的目标无法获取")
	}
	if retrieved.ID != "goal-1" || retrieved.Type != GoalTypeConnection {
		t.Fatalf("目标数据不一致: %#v", retrieved)
	}
}

func TestGoalRegistryUpdateStatus(t *testing.T) {
	registry := NewGoalRegistry()
	goal := Goal{
		ID:        "goal-2",
		Status:    GoalStatusPending,
		Progress:  0,
		UpdatedAt: time.Now().UTC(),
	}
	registry.Register(goal)

	ok := registry.UpdateStatus("goal-2", GoalStatusAchieved, 1.0)
	if !ok {
		t.Fatal("UpdateStatus 应成功")
	}

	updated, _ := registry.Get("goal-2")
	if updated.Status != GoalStatusAchieved || updated.Progress != 1.0 {
		t.Fatalf("状态/进度未更新: status=%s progress=%f", updated.Status, updated.Progress)
	}
}

func TestGoalRegistryUpdateStatusNonExistent(t *testing.T) {
	registry := NewGoalRegistry()
	ok := registry.UpdateStatus("nonexistent", GoalStatusAchieved, 1.0)
	if ok {
		t.Fatal("不存在的目标更新应返回 false")
	}
}

func TestGoalRegistryRemove(t *testing.T) {
	registry := NewGoalRegistry()
	registry.Register(Goal{ID: "goal-3"})

	if !registry.Remove("goal-3") {
		t.Fatal("Remove 应成功")
	}
	if registry.Remove("goal-3") {
		t.Fatal("重复 Remove 应返回 false")
	}
	if _, ok := registry.Get("goal-3"); ok {
		t.Fatal("移除后不应获取到目标")
	}
}

func TestGoalRegistryByUser(t *testing.T) {
	registry := NewGoalRegistry()
	registry.Register(Goal{ID: "u1-g1", UserID: "u1", Priority: GoalPriorityNormal})
	registry.Register(Goal{ID: "u1-g2", UserID: "u1", Priority: GoalPriorityHigh})
	registry.Register(Goal{ID: "u2-g1", UserID: "u2", Priority: GoalPriorityLow})

	u1Goals := registry.ByUser("u1")
	if len(u1Goals) != 2 {
		t.Fatalf("用户 u1 应有 2 个目标, 实际 %d", len(u1Goals))
	}
	if u1Goals[0].Priority != GoalPriorityHigh {
		t.Fatalf("排序错误, 高优先级应在前面: %#v", u1Goals)
	}
}

func TestGoalRegistryActive(t *testing.T) {
	registry := NewGoalRegistry()
	registry.Register(Goal{ID: "g1", Status: GoalStatusActive, Priority: GoalPriorityNormal})
	registry.Register(Goal{ID: "g2", Status: GoalStatusPending, Priority: GoalPriorityLow})
	registry.Register(Goal{ID: "g3", Status: GoalStatusAchieved, Priority: GoalPriorityCritical})
	registry.Register(Goal{ID: "g4", Status: GoalStatusSuspended, Priority: GoalPriorityHigh})

	active := registry.Active()
	if len(active) != 2 {
		t.Fatalf("活跃目标应为 2 个, 实际 %d", len(active))
	}
	ids := make(map[string]bool)
	for _, g := range active {
		ids[g.ID] = true
	}
	if !ids["g1"] || !ids["g2"] {
		t.Fatal("活跃目标 ID 集错误")
	}
}

func TestGoalRegistryWishes(t *testing.T) {
	registry := NewGoalRegistry()
	now := time.Now().UTC()
	registry.Register(Goal{ID: "w1", Status: GoalStatusWish, UpdatedAt: now})
	registry.Register(Goal{ID: "g1", Status: GoalStatusActive, UpdatedAt: now})

	wishes := registry.Wishes()
	if len(wishes) != 1 {
		t.Fatalf("wish 数量应为 1, 实际 %d", len(wishes))
	}
	if wishes[0].ID != "w1" {
		t.Fatalf("wish ID 错误: %s", wishes[0].ID)
	}
	if wishes[0].Goal.Status != GoalStatusWish {
		t.Fatalf("wish goal 状态应是 wish, 实际 %s", wishes[0].Goal.Status)
	}
}

func TestGoalRegistryExpireStale(t *testing.T) {
	registry := NewGoalRegistry()
	now := time.Now().UTC()
	registry.Register(Goal{ID: "expired", ExpiresAt: now.Add(-1 * time.Hour), CreatedAt: now})
	registry.Register(Goal{ID: "valid", ExpiresAt: now.Add(1 * time.Hour), CreatedAt: now})
	registry.Register(Goal{ID: "no-expiry", CreatedAt: now})
	removed := registry.ExpireStale(now)
	if len(removed) != 1 || removed[0] != "expired" {
		t.Fatalf("应只移除过期目标 expired, 实际 %#v", removed)
	}
	if registry.Len() != 2 {
		t.Fatalf("过期清理后应剩余 2 个目标, 实际 %d", registry.Len())
	}
}

func TestGoalRegistryPromoteToWish(t *testing.T) {
	registry := NewGoalRegistry()
	now := time.Now().UTC()
	registry.Register(Goal{ID: "g1", Status: GoalStatusActive, UpdatedAt: now.Add(-24 * time.Hour)})
	if !registry.PromoteToWish("g1", now) {
		t.Fatal("PromoteToWish 应成功")
	}
	g, _ := registry.Get("g1")
	if g.Status != GoalStatusWish {
		t.Fatalf("状态应为 wish, 实际 %s", g.Status)
	}
}

func TestGoalRegistryLen(t *testing.T) {
	registry := NewGoalRegistry()
	if registry.Len() != 0 {
		t.Fatal("新注册表长度应为 0")
	}
	registry.Register(Goal{ID: "g1"})
	registry.Register(Goal{ID: "g2"})
	if registry.Len() != 2 {
		t.Fatalf("长度应为 2, 实际 %d", registry.Len())
	}
}

func TestUpdateStatusTerminalGuard(t *testing.T) {
	registry := NewGoalRegistry()
	registry.Register(Goal{ID: "g1", Status: GoalStatusAchieved, Progress: 1, Revision: 3})
	if registry.UpdateStatus("g1", GoalStatusActive, 0.5) {
		t.Fatal("achieved → active should not be allowed")
	}
	g, _ := registry.Get("g1")
	if g.Status != GoalStatusAchieved || g.Progress != 1 {
		t.Fatalf("goal changed: status=%s progress=%f", g.Status, g.Progress)
	}
	if g.Revision != 3 {
		t.Fatalf("revision changed on rejected UpdateStatus: %d", g.Revision)
	}
}

func TestUpdateStatusAbandonedTerminal(t *testing.T) {
	registry := NewGoalRegistry()
	registry.Register(Goal{ID: "g1", Status: GoalStatusAbandoned, Progress: 0.3, Revision: 2})
	if registry.UpdateStatus("g1", GoalStatusActive, 0) {
		t.Fatal("abandoned → active should not be allowed")
	}
	g, _ := registry.Get("g1")
	if g.Status != GoalStatusAbandoned {
		t.Fatal("status changed")
	}
}

func TestUpdateStatusAchievedRequiresProgress1(t *testing.T) {
	registry := NewGoalRegistry()
	registry.Register(Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.5})
	if registry.UpdateStatus("g1", GoalStatusAchieved, 0.9) {
		t.Fatal("achieved with progress!=1 must be rejected")
	}
	g, _ := registry.Get("g1")
	if g.Status != GoalStatusActive {
		t.Fatal("status changed")
	}
}

func TestUpdateStatusProgress1RequiresAchieved(t *testing.T) {
	registry := NewGoalRegistry()
	registry.Register(Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.5})
	if registry.UpdateStatus("g1", GoalStatusActive, 1) {
		t.Fatal("progress=1 with active status must be rejected")
	}
}

func TestApplyProgressBatchBasic(t *testing.T) {
	registry := NewGoalRegistry()
	registry.Register(Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.2, Revision: 1, UserID: "u1"})
	registry.Register(Goal{ID: "g2", Status: GoalStatusPending, Progress: 0, Revision: 1, UserID: "u1"})

	appliedAt := time.Now().UTC()
	updates := []GoalProgressUpdate{
		{GoalRef: GoalRef{ID: "g1", Revision: 1}, ObservationID: "o1", ExpectedRevision: 1, NextStatus: GoalStatusActive, NextProgress: 0.5, Apply: true},
		{GoalRef: GoalRef{ID: "g2", Revision: 1}, ObservationID: "o2", ExpectedRevision: 1, NextStatus: GoalStatusActive, NextProgress: 0, Apply: true},
	}

	results, err := registry.ApplyProgressBatch(updates, appliedAt)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if !r.Changed {
			t.Errorf("expected changed=true for %s", r.GoalID)
		}
	}

	g1, _ := registry.Get("g1")
	if g1.Progress != 0.5 {
		t.Errorf("g1 progress: got %f, want 0.5", g1.Progress)
	}
	if g1.Revision != 2 {
		t.Errorf("g1 revision: got %d, want 2", g1.Revision)
	}
	if g1.LastObservationID != "o1" {
		t.Errorf("g1 LastObservationID: got %s, want o1", g1.LastObservationID)
	}
	if !g1.LastObservedAt.Equal(appliedAt) {
		t.Errorf("g1 LastObservedAt mismatch")
	}

	g2, _ := registry.Get("g2")
	if g2.Status != GoalStatusActive {
		t.Errorf("g2 status: got %s, want active", g2.Status)
	}
	if g2.Revision != 2 {
		t.Errorf("g2 revision: got %d", g2.Revision)
	}
}

func TestApplyProgressBatchStaleRevisionAtomicity(t *testing.T) {
	registry := NewGoalRegistry()
	registry.Register(Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.2, Revision: 5})
	registry.Register(Goal{ID: "g2", Status: GoalStatusActive, Progress: 0.2, Revision: 2})

	appliedAt := time.Now().UTC()
	updates := []GoalProgressUpdate{
		{GoalRef: GoalRef{ID: "g1", Revision: 5}, ObservationID: "o1", ExpectedRevision: 5, NextStatus: GoalStatusActive, NextProgress: 0.5, Apply: true},
		{GoalRef: GoalRef{ID: "g2", Revision: 1}, ObservationID: "o1", ExpectedRevision: 1, NextStatus: GoalStatusActive, NextProgress: 0.5, Apply: true},
	}

	_, err := registry.ApplyProgressBatch(updates, appliedAt)
	if err == nil {
		t.Fatal("expected stale revision error")
	}

	g1, _ := registry.Get("g1")
	if g1.Progress != 0.2 {
		t.Errorf("g1 should not change on batch atomic failure: progress=%f", g1.Progress)
	}
	g2, _ := registry.Get("g2")
	if g2.Progress != 0.2 {
		t.Errorf("g2 should not register: progress=%f", g2.Progress)
	}
}

func TestApplyProgressBatchMissingGoal(t *testing.T) {
	registry := NewGoalRegistry()
	registry.Register(Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.2, Revision: 1})

	appliedAt := time.Now().UTC()
	updates := []GoalProgressUpdate{
		{GoalRef: GoalRef{ID: "g1", Revision: 1}, ObservationID: "o1", ExpectedRevision: 1, NextStatus: GoalStatusActive, NextProgress: 0.5, Apply: true},
		{GoalRef: GoalRef{ID: "missing", Revision: 1}, ObservationID: "o1", ExpectedRevision: 1, NextStatus: GoalStatusActive, NextProgress: 0.5, Apply: true},
	}

	_, err := registry.ApplyProgressBatch(updates, appliedAt)
	if err == nil {
		t.Fatal("expected missing goal error")
	}

	g1, _ := registry.Get("g1")
	if g1.Progress != 0.2 {
		t.Errorf("atomic failure should not commit: progress=%f", g1.Progress)
	}
}

func TestApplyProgressBatchTerminalNoop(t *testing.T) {
	registry := NewGoalRegistry()
	registry.Register(Goal{ID: "g1", Status: GoalStatusAchieved, Progress: 1, Revision: 3})
	registry.Register(Goal{ID: "g2", Status: GoalStatusActive, Progress: 0.5, Revision: 2})

	appliedAt := time.Now().UTC()
	updates := []GoalProgressUpdate{
		{GoalRef: GoalRef{ID: "g1", Revision: 3}, ObservationID: "o1", ExpectedRevision: 3, NextStatus: GoalStatusActive, NextProgress: 0.9, Apply: true},
		{GoalRef: GoalRef{ID: "g2", Revision: 2}, ObservationID: "o1", ExpectedRevision: 2, NextStatus: GoalStatusAchieved, NextProgress: 1, Apply: true},
	}

	results, err := registry.ApplyProgressBatch(updates, appliedAt)
	if err != nil {
		t.Fatal(err)
	}

	g1, _ := registry.Get("g1")
	if g1.Revision != 3 || g1.Progress != 1 || g1.Status != GoalStatusAchieved {
		t.Fatalf("g1 terminal should not change: status=%s progress=%f rev=%d", g1.Status, g1.Progress, g1.Revision)
	}

	g2, _ := registry.Get("g2")
	if g2.Status != GoalStatusAchieved || g2.Progress != 1 {
		t.Fatalf("g2 should be achieved: status=%s progress=%f", g2.Status, g2.Progress)
	}

	for _, r := range results {
		if r.GoalID == "g1" && r.Disposition != GoalProgressTerminalIgnore {
			t.Errorf("g1 disposition should be terminal_ignored, got %s", r.Disposition)
		}
		if r.GoalID == "g2" && !r.Changed {
			t.Errorf("g2 should be changed")
		}
	}
}

func TestApplyProgressBatchProgressMonotonicNeverBackward(t *testing.T) {
	registry := NewGoalRegistry()
	registry.Register(Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.8, Revision: 1})

	appliedAt := time.Now().UTC()
	updates := []GoalProgressUpdate{
		{GoalRef: GoalRef{ID: "g1", Revision: 1}, ObservationID: "o1", ExpectedRevision: 1, NextStatus: GoalStatusActive, NextProgress: 0.4, Apply: true},
	}

	results, err := registry.ApplyProgressBatch(updates, appliedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !results[0].Changed {
		t.Error("batch should commit")
	}

	g1, _ := registry.Get("g1")
	if g1.Progress != 0.4 {
		t.Errorf("g1 progress: got %f, want 0.4 (batch only)", g1.Progress)
	}
}

func TestGoalRegistryDefensiveCloneGet(t *testing.T) {
	registry := NewGoalRegistry()
	registry.Register(Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.3, Revision: 1, Metadata: map[string]any{"k": "v"}})

	g, _ := registry.Get("g1")
	g.Metadata["k"] = "mutated"
	g.Progress = 0.9

	original, _ := registry.Get("g1")
	if original.Metadata["k"] != "v" {
		t.Errorf("registry internal metadata mutated: %v", original.Metadata)
	}
	if original.Progress != 0.3 {
		t.Errorf("registry internal progress mutated: %f", original.Progress)
	}
}

func TestGoalRegistryDefensiveCloneRegister(t *testing.T) {
	registry := NewGoalRegistry()
	g := Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.3, Revision: 1, Metadata: map[string]any{"k": "v"}}
	registry.Register(g)

	g.Metadata["k"] = "mutated"
	g.Progress = 0.9

	original, _ := registry.Get("g1")
	if original.Metadata["k"] != "v" {
		t.Errorf("registry internal metadata mutated: %v", original.Metadata)
	}
	if original.Progress != 0.3 {
		t.Errorf("registry internal progress mutated: %f", original.Progress)
	}
}

func TestApplyProgressBatchConcurrentDuplicateObservation(t *testing.T) {
	registry := NewGoalRegistry()
	registry.Register(Goal{ID: "g1", Status: GoalStatusActive, Progress: 0, Revision: 1, UserID: "u1"})

	const N = 100
	var wg sync.WaitGroup
	results := make([]GoalProgressResult, N)
	var mu sync.Mutex

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			appliedAt := time.Now().UTC()
			updates := []GoalProgressUpdate{
				{GoalRef: GoalRef{ID: "g1", Revision: 1}, ObservationID: "dup", ExpectedRevision: 1, NextStatus: GoalStatusAchieved, NextProgress: 1, Apply: true},
			}
			res, err := registry.ApplyProgressBatch(updates, appliedAt)
			mu.Lock()
			if err == nil && len(res) > 0 {
				results[i] = res[0]
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	changedCount := 0
	for _, r := range results {
		if r.Changed {
			changedCount++
		}
	}
	if changedCount != 1 {
		t.Errorf("expected exactly 1 successful mutation, got %d", changedCount)
	}

	g1, _ := registry.Get("g1")
	if g1.Revision != 2 {
		t.Errorf("revision after all goroutines should be 2, got %d", g1.Revision)
	}
}

func TestApplyProgressBatchDuplicateObservationIdempotent(t *testing.T) {
	registry := NewGoalRegistry()
	registry.Register(Goal{ID: "g1", Status: GoalStatusActive, Progress: 0, Revision: 1})

	appliedAt := time.Now().UTC()
	updates := []GoalProgressUpdate{
		{GoalRef: GoalRef{ID: "g1", Revision: 1}, ObservationID: "dup", ExpectedRevision: 1, NextStatus: GoalStatusActive, NextProgress: 0.3, Apply: true},
	}
	results, err := registry.ApplyProgressBatch(updates, appliedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !results[0].Changed {
		t.Error("first apply should succeed")
	}

	g1, _ := registry.Get("g1")
	if g1.Revision != 2 {
		t.Errorf("revision should be 2 after first apply: %d", g1.Revision)
	}
	if g1.Progress != 0.3 {
		t.Errorf("progress should be 0.3: %f", g1.Progress)
	}

	updates2 := []GoalProgressUpdate{
		{GoalRef: GoalRef{ID: "g1", Revision: 1}, ObservationID: "dup", ExpectedRevision: 1, NextStatus: GoalStatusActive, NextProgress: 0.5, Apply: true},
	}
	results2, err := registry.ApplyProgressBatch(updates2, appliedAt)
	if err == nil {
		t.Error("second duplicate should fail with stale revision")
	}
	if len(results2) != 1 {
		t.Fatalf("should return 1 stale result for inspection: got %d", len(results2))
	}
	if results2[0].Disposition != GoalProgressStaleRevision {
		t.Errorf("disposition should be stale_revision: got %s", results2[0].Disposition)
	}

	g1final, _ := registry.Get("g1")
	if g1final.Revision != 2 {
		t.Errorf("revision should stay at 2: %d", g1final.Revision)
	}
}

func TestUpdateStatusNaNProgressRejected(t *testing.T) {
	registry := NewGoalRegistry()
	registry.Register(Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.5})
	if registry.UpdateStatus("g1", GoalStatusActive, 1.5) {
		t.Fatal("progress=1.5 should be rejected")
	}
}
