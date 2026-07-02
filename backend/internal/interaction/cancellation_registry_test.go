package interaction

import (
	"context"
	"testing"
	"time"
)

func TestCancellationRegistryCancelKeepsEntry(t *testing.T) {
	registry := NewCancellationRegistry()
	ctx, cancel := context.WithCancel(context.Background())

	registry.Register("interaction-1", cancel)

	if !registry.Cancel("interaction-1") {
		t.Fatalf("应能取消已注册交互")
	}

	select {
	case <-ctx.Done():
	default:
		t.Fatalf("取消函数未被触发")
	}

	if registry.Len() != 1 {
		t.Fatalf("Cancel 不应移除注册项, 实际长度 %d", registry.Len())
	}
}

func TestCancellationRegistryCleanupStaleAtRemovesOnlyExpiredEntries(t *testing.T) {
	registry := NewCancellationRegistry()
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)

	registry.entries["expired"] = cancellationEntry{
		cancel:    func() {},
		createdAt: now.Add(-2 * time.Hour),
	}
	registry.entries["fresh"] = cancellationEntry{
		cancel:    func() {},
		createdAt: now.Add(-30 * time.Minute),
	}

	cleaned := registry.CleanupStaleAt(time.Hour, now)

	if cleaned != 1 {
		t.Fatalf("应清理 1 个陈旧项, 实际 %d", cleaned)
	}
	if registry.Len() != 1 {
		t.Fatalf("清理后应剩余 1 项, 实际 %d", registry.Len())
	}
	if registry.Cancel("expired") {
		t.Fatalf("陈旧项应已被移除")
	}
	if !registry.Cancel("fresh") {
		t.Fatalf("未过期项不应被移除")
	}
}

func TestCancellationRegistryCleanupStaleAtKeepsBoundaryEntry(t *testing.T) {
	registry := NewCancellationRegistry()
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)

	registry.entries["boundary"] = cancellationEntry{
		cancel:    func() {},
		createdAt: now.Add(-time.Hour),
	}

	cleaned := registry.CleanupStaleAt(time.Hour, now)

	if cleaned != 0 {
		t.Fatalf("刚达到最大年龄的注册项不应被清理, 实际清理 %d", cleaned)
	}
	if !registry.Cancel("boundary") {
		t.Fatalf("边界项应仍可取消")
	}
}

func TestCancellationRegistryCleanupStaleAtIgnoresInvalidMaxAge(t *testing.T) {
	registry := NewCancellationRegistry()
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)

	registry.entries["expired"] = cancellationEntry{
		cancel:    func() {},
		createdAt: now.Add(-2 * time.Hour),
	}

	if cleaned := registry.CleanupStaleAt(0, now); cleaned != 0 {
		t.Fatalf("无效最大年龄不应清理注册项, 实际清理 %d", cleaned)
	}
	if registry.Len() != 1 {
		t.Fatalf("无效最大年龄不应改变注册表长度, 实际 %d", registry.Len())
	}
}

func TestCancellationRegistryRegisterCleansStaleEntries(t *testing.T) {
	registry := NewCancellationRegistry()
	registry.entries["expired"] = cancellationEntry{
		cancel:    func() {},
		createdAt: time.Now().Add(-cancellationRegistryMaxAge - time.Minute),
	}

	_, cancel := context.WithCancel(context.Background())
	registry.Register("fresh", cancel)

	if registry.Cancel("expired") {
		t.Fatalf("注册新项时应清理陈旧项")
	}
	if !registry.Cancel("fresh") {
		t.Fatalf("新注册项应保留")
	}
	if registry.Len() != 1 {
		t.Fatalf("注册清理后应只剩新项, 实际 %d", registry.Len())
	}
}
