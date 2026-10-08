package coordination_test

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRequestAuthorityRechecksCallerInsideTaskCommitBarrier(t *testing.T) {
	_, service := setup(t)
	request, _, finish, err := service.Begin(t.Context(), "space", "a", "a", "space", "", "management")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	linked, err := coordination.WithRequestAuthority(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if _, owned := coordination.FromContext(linked); owned {
		t.Fatal("管理请求改变了任务数据归属")
	}
	task, expected, endTask, err := service.Begin(linked, "space", "b", "b", "space", "role", "task")
	if err != nil {
		t.Fatal(err)
	}
	defer endTask()
	if actual, ok := coordination.FromContext(task); !ok || actual != expected {
		t.Fatal("任务授权被管理员请求覆盖")
	}
	committed := false
	if err := coordination.CommitCurrent(task, func() error { committed = true; return nil }); err != nil || !committed {
		t.Fatalf("有效管理请求不能提交: %v", err)
	}
	window := coordination.WithAdditionalGuard(task, func(context.Context) error {
		_, err := service.ChangeMode(t.Context(), "space", "a", 1, true, "role")
		return err
	})
	committed = false
	if err := coordination.CommitRequestCurrent(window, func() error { committed = true; return nil }); !errors.Is(err, coordination.ErrScopeExpired) || committed {
		t.Fatalf("调用者在最终提交前撤销权限后仍被提交: %v %v", err, committed)
	}
	if err := service.Validate(t.Context(), expected); err != nil {
		t.Fatalf("被管理任务自身授权不应被调用者变更影响: %v", err)
	}
}

func TestRequestAuthorityCollectionWritesAndForeignRealmAreFenced(t *testing.T) {
	_, service := setup(t)
	request, _, finish, err := service.Begin(t.Context(), "space", "a", "a", "space", "", "management")
	if err != nil {
		t.Fatal(err)
	}
	linked, err := coordination.WithRequestAuthority(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	if err := coordination.CommitCurrent(linked, func() error { called = true; return nil }); err != nil || !called {
		t.Fatalf("集合写入丢失提交授权: %v", err)
	}
	_, other := setup(t)
	foreign, _, endForeign, err := other.Begin(t.Context(), "space", "b", "b", "space", "", "foreign")
	if err != nil {
		t.Fatal(err)
	}
	defer endForeign()
	if _, err := coordination.WithRequestAuthority(foreign, request); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("允许将不同权限服务组合提交: %v", err)
	}
	finish()
	called = false
	if err := coordination.CommitCurrent(linked, func() error { called = true; return nil }); err == nil || called {
		t.Fatalf("管理请求结束后仍可修改集合: %v", err)
	}
	if _, err := coordination.WithRequestAuthority(t.Context(), t.Context()); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("未认证上下文被当作管理授权: %v", err)
	}
}
