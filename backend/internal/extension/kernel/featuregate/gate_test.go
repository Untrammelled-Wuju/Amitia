// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package featuregate

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func resetGlobal(t *testing.T) {
	t.Helper()
	globalMu.Lock()
	defer globalMu.Unlock()
	globalRegistry = nil
	globalConfigured = false
	globalFingerprint = ""
}

func TestParseDefaultsDisabled(t *testing.T) {
	registry, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse(nil) 返回错误: %v", err)
	}
	for _, name := range catalog() {
		if registry.Enabled(name) {
			t.Fatalf("%s 默认必须关闭", name)
		}
	}
	snapshot := registry.Snapshot()
	if len(snapshot.Features) != len(catalog()) {
		t.Fatalf("快照条目数 %d，期望 %d", len(snapshot.Features), len(catalog()))
	}
	for _, feature := range snapshot.Features {
		if feature.Enabled || feature.Applied {
			t.Fatalf("%s 默认状态必须为未开启未应用: %+v", feature.Name, feature)
		}
	}
}

func TestParseRejectsNonBoolean(t *testing.T) {
	values := []any{"true", 1, nil, []any{}, map[string]any{}}
	for _, value := range values {
		registry, err := Parse(map[string]any{NameRuntimeCanary: value})
		if registry != nil {
			t.Fatalf("值 %T 不应产生 registry", value)
		}
		if !errors.Is(err, ErrInvalidValue) {
			t.Fatalf("值 %T 错误类型不符: %v", value, err)
		}
		if !strings.Contains(err.Error(), NameRuntimeCanary) {
			t.Fatalf("错误信息缺少开关名: %v", err)
		}
	}
}

func TestParseRejectsUnknownFeature(t *testing.T) {
	_, err := Parse(map[string]any{"runtime-canry": true})
	if !errors.Is(err, ErrUnknownFeature) {
		t.Fatalf("未知开关未拒绝: %v", err)
	}
	if !strings.Contains(err.Error(), NameRuntimeCanary) {
		t.Fatalf("错误信息未列出可用开关: %v", err)
	}
}

func TestParseNormalizesAndRejectsDuplicate(t *testing.T) {
	registry, err := Parse(map[string]any{"  Runtime-Canary  ": true})
	if err != nil {
		t.Fatalf("大小写与空格应归一化: %v", err)
	}
	if !registry.Enabled(NameRuntimeCanary) {
		t.Fatalf("归一化后未开启")
	}
	_, err = Parse(map[string]any{"runtime-canary": true, "Runtime-Canary": false})
	if !errors.Is(err, ErrConfigurationMalformed) {
		t.Fatalf("归一化重复键未拒绝: %v", err)
	}
}

func TestApplyDisabledRegistersNothing(t *testing.T) {
	registry, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	var calls atomic.Int64
	hooks := Hooks{
		Provider:   func() error { calls.Add(1); return nil },
		Subscriber: func() error { calls.Add(1); return nil },
		Worker:     func(context.Context) { calls.Add(1) },
	}
	for index := 0; index < 3; index++ {
		outcome, applyErr := registry.Apply(context.Background(), NameRuntimeCanary, hooks)
		if applyErr != nil {
			t.Fatalf("关闭态 Apply 不应报错: %v", applyErr)
		}
		if outcome.Enabled || outcome.Duplicate || len(outcome.Registered) != 0 {
			t.Fatalf("关闭态结果异常: %+v", outcome)
		}
	}
	time.Sleep(20 * time.Millisecond)
	if calls.Load() != 0 {
		t.Fatalf("关闭态调用了 %d 次注册钩子", calls.Load())
	}
	for _, feature := range registry.Snapshot().Features {
		if feature.Name == NameRuntimeCanary && feature.Applied {
			t.Fatalf("关闭态不应标记已应用")
		}
	}
}

func TestApplyEnabledRegistersOnce(t *testing.T) {
	registry, err := Parse(map[string]any{NameRuntimeCanary: true})
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	var provider, subscriber, worker atomic.Int64
	hooks := Hooks{
		Provider:   func() error { provider.Add(1); return nil },
		Subscriber: func() error { subscriber.Add(1); return nil },
		Worker:     func(ctx context.Context) { worker.Add(1); <-ctx.Done() },
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	outcome, err := registry.Apply(ctx, NameRuntimeCanary, hooks)
	if err != nil {
		t.Fatalf("开启态 Apply 失败: %v", err)
	}
	if !outcome.Enabled || outcome.Duplicate {
		t.Fatalf("开启态结果异常: %+v", outcome)
	}
	if strings.Join(outcome.Registered, ",") != "provider,subscriber,worker" {
		t.Fatalf("注册顺序异常: %+v", outcome.Registered)
	}
	duplicate, err := registry.Apply(ctx, NameRuntimeCanary, hooks)
	if err != nil {
		t.Fatalf("重复 Apply 不应报错: %v", err)
	}
	if !duplicate.Duplicate {
		t.Fatalf("重复 Apply 未标记 duplicate: %+v", duplicate)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && worker.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if provider.Load() != 1 || subscriber.Load() != 1 || worker.Load() != 1 {
		t.Fatalf("钩子调用次数异常 provider=%d subscriber=%d worker=%d", provider.Load(), subscriber.Load(), worker.Load())
	}
	var applied bool
	for _, feature := range registry.Snapshot().Features {
		if feature.Name == NameRuntimeCanary {
			applied = feature.Applied
		}
	}
	if !applied {
		t.Fatalf("开启态未标记已应用")
	}
}

func TestApplyUnknownFeatureRejected(t *testing.T) {
	registry, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	_, err = registry.Apply(context.Background(), "runtime-canry", Hooks{})
	if !errors.Is(err, ErrUnknownFeature) {
		t.Fatalf("Apply 未拒绝未知开关: %v", err)
	}
}

func TestApplyProviderFailurePropagatesCause(t *testing.T) {
	registry, err := Parse(map[string]any{NameRuntimeCanary: true})
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	cause := errors.New("capability registry rejected candidate binding")
	var subscriber, worker, providerCalls atomic.Int64
	hooks := Hooks{
		Provider: func() error {
			providerCalls.Add(1)
			return cause
		},
		Subscriber: func() error { subscriber.Add(1); return nil },
		Worker:     func(context.Context) { worker.Add(1) },
	}
	outcome, err := registry.Apply(context.Background(), NameRuntimeCanary, hooks)
	if err == nil {
		t.Fatalf("provider 失败必须返回错误")
	}
	if !errors.Is(err, cause) {
		t.Fatalf("原始 cause 丢失: %v", err)
	}
	if !strings.Contains(err.Error(), "已应用：none") {
		t.Fatalf("错误信息缺少已应用清单: %v", err)
	}
	if len(outcome.Registered) != 0 {
		t.Fatalf("失败后不应报告已注册: %+v", outcome)
	}
	if subscriber.Load() != 0 || worker.Load() != 0 {
		t.Fatalf("provider 失败后不得继续注册 subscriber/worker")
	}
	if _, retryErr := registry.Apply(context.Background(), NameRuntimeCanary, hooks); retryErr == nil {
		t.Fatalf("失败后的重试应再次执行 provider 并失败")
	}
	if providerCalls.Load() != 2 {
		t.Fatalf("失败后 applied 未释放，provider 调用 %d 次", providerCalls.Load())
	}
}

func TestWorkerObservesCancellation(t *testing.T) {
	registry, err := Parse(map[string]any{NameRuntimeCanary: true})
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	started := make(chan struct{})
	stopped := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	_, err = registry.Apply(ctx, NameRuntimeCanary, Hooks{Worker: func(workerCtx context.Context) {
		close(started)
		<-workerCtx.Done()
		close(stopped)
	}})
	if err != nil {
		t.Fatalf("Apply 失败: %v", err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatalf("worker 未启动")
	}
	cancel()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatalf("worker 未响应取消")
	}
}

func TestConfigureSingleton(t *testing.T) {
	resetGlobal(t)
	defer resetGlobal(t)
	if err := Configure(map[string]any{NameRuntimeCanary: true}); err != nil {
		t.Fatalf("首次 Configure 失败: %v", err)
	}
	if !Enabled(NameRuntimeCanary) {
		t.Fatalf("单例未开启 runtime-canary")
	}
	if err := Configure(map[string]any{NameRuntimeCanary: true}); err != nil {
		t.Fatalf("相同配置重复 Configure 应幂等: %v", err)
	}
	if err := Configure(map[string]any{NameRuntimeCanary: false}); !errors.Is(err, ErrConfigurationConflict) {
		t.Fatalf("不一致的重复 Configure 未拒绝: %v", err)
	}
	if !Enabled(NameRuntimeCanary) {
		t.Fatalf("冲突后原配置被篡改")
	}
}

func TestActiveFailClosedBeforeConfigure(t *testing.T) {
	resetGlobal(t)
	defer resetGlobal(t)
	var calls atomic.Int64
	if Enabled(NameRuntimeCanary) || Enabled(NameRouteInventory) {
		t.Fatalf("未配置时必须默认关闭")
	}
	outcome, err := Apply(context.Background(), NameRuntimeCanary, Hooks{Provider: func() error { calls.Add(1); return nil }})
	if err != nil {
		t.Fatalf("未配置时 Apply 不应报错: %v", err)
	}
	if outcome.Enabled || calls.Load() != 0 {
		t.Fatalf("未配置时不得注册任何钩子: %+v calls=%d", outcome, calls.Load())
	}
}
