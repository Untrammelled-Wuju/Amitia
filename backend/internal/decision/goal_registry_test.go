package decision

import (
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
