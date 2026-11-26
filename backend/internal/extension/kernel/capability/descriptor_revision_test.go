package capability_test

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func revisionLog(t *testing.T, line string) {
	t.Helper()
	t.Logf("m05-s02 %s", line)
}

func registerEvents(tools []capability.ToolDefinition, from uint64) []capability.DescriptorEvent {
	events := make([]capability.DescriptorEvent, 0, len(tools))
	for position, tool := range tools {
		events = append(events, capability.DescriptorEvent{
			Revision: from + uint64(position),
			Kind:     capability.DescriptorRegister,
			Tool:     tool,
		})
	}
	return events
}

func unregisterEvents(tools []capability.ToolDefinition, from uint64) []capability.DescriptorEvent {
	events := make([]capability.DescriptorEvent, 0, len(tools))
	for position, tool := range tools {
		events = append(events, capability.DescriptorEvent{
			Revision: from + uint64(position),
			Kind:     capability.DescriptorUnregister,
			ToolID:   tool.ID,
		})
	}
	return events
}

func emptyIndex(t *testing.T) (*capability.SchemaStore, *capability.DescriptorIndex) {
	t.Helper()
	store := capability.NewSchemaStore(0)
	index, err := capability.BuildDescriptorIndex(store, nil)
	if err != nil {
		t.Fatalf("BuildDescriptorIndex: %v", err)
	}
	return store, index
}

func TestCapabilityRevisionBatchUninstallDropsTools(t *testing.T) {
	store, index := emptyIndex(t)
	tools := descriptorTools(200)
	report, err := index.Apply(context.Background(), store, registerEvents(tools, 2))
	if err != nil {
		t.Fatalf("注册: %v", err)
	}
	if report.Added != 200 || index.Len() != 200 || index.Revision() != 201 {
		t.Fatalf("注册结果 added=%d len=%d revision=%d", report.Added, index.Len(), index.Revision())
	}

	removed, err := index.Apply(context.Background(), store, unregisterEvents(tools[:150], 202))
	if err != nil {
		t.Fatalf("卸载: %v", err)
	}
	if removed.Removed != 150 || index.Len() != 50 {
		t.Fatalf("卸载结果 removed=%d len=%d", removed.Removed, index.Len())
	}
	for _, tool := range tools[:150] {
		if _, ok := index.Entry(tool.ID); ok {
			t.Fatalf("已卸载工具仍可查到: %s", tool.ID)
		}
	}
	refs := index.Refs()
	if len(refs) != 50 {
		t.Fatalf("引用数 %d", len(refs))
	}
	revisionLog(t, "registered=200 uninstalled=150 remaining=50 stale_entries=0 revision=351")
}

func TestCapabilityRevisionDuplicateNotificationsDoNotGrow(t *testing.T) {
	store, index := emptyIndex(t)
	tools := descriptorTools(200)
	events := registerEvents(tools, 2)
	if _, err := index.Apply(context.Background(), store, events); err != nil {
		t.Fatalf("首次注册: %v", err)
	}
	mutations := index.Mutations()
	again, err := index.Apply(context.Background(), store, events)
	if err != nil {
		t.Fatalf("重复通知: %v", err)
	}
	if again.Skipped != 200 || again.Applied != 0 {
		t.Fatalf("重复通知未跳过: skipped=%d applied=%d", again.Skipped, again.Applied)
	}
	if index.Len() != 200 || index.Revision() != 201 || index.Mutations() != mutations {
		t.Fatalf("重复通知扩大了索引: len=%d revision=%d mutations=%d/%d", index.Len(), index.Revision(), index.Mutations(), mutations)
	}
	uninstall := unregisterEvents(tools[:10], 202)
	if _, err := index.Apply(context.Background(), store, uninstall); err != nil {
		t.Fatalf("卸载: %v", err)
	}
	replay, err := index.Apply(context.Background(), store, uninstall)
	if err != nil {
		t.Fatalf("重复卸载: %v", err)
	}
	if replay.Skipped != 10 || index.Len() != 190 {
		t.Fatalf("重复卸载改变了索引: skipped=%d len=%d", replay.Skipped, index.Len())
	}
	revisionLog(t, "replay=200 skipped=200 len=200 uninstall_replay=10 len_after=190")
}

func TestCapabilityRevisionGapRequiresSnapshotRepair(t *testing.T) {
	store, index := emptyIndex(t)
	tools := descriptorTools(6)
	if _, err := index.Apply(context.Background(), store, registerEvents(tools[:2], 2)); err != nil {
		t.Fatalf("注册: %v", err)
	}
	gapped := capability.DescriptorEvent{Revision: 5, Kind: capability.DescriptorRegister, Tool: tools[2]}
	report, err := index.Apply(context.Background(), store, []capability.DescriptorEvent{gapped})
	if !errors.Is(err, capability.ErrDescriptorRevisionGap) {
		t.Fatalf("断档未被拒绝: %v", err)
	}
	if report.Gap == nil || report.Gap.Expected != 4 || report.Gap.Received != 5 {
		t.Fatalf("断档区间不符: %+v", report.Gap)
	}
	if index.Len() != 2 || index.Revision() != 3 {
		t.Fatalf("断档事件污染了索引: len=%d revision=%d", index.Len(), index.Revision())
	}
	repair, err := index.Repair(context.Background(), store, tools, 5)
	if err != nil {
		t.Fatalf("修复: %v", err)
	}
	if repair.Added != 4 || repair.Kept != 2 || repair.Revision != 5 || index.Len() != 6 || index.Rebuilds() != 1 {
		t.Fatalf("修复结果 %+v len=%d rebuilds=%d", repair, index.Len(), index.Rebuilds())
	}
	if _, err := index.Repair(context.Background(), store, tools, 5); !errors.Is(err, capability.ErrDescriptorRevisionStale) {
		t.Fatalf("旧 revision 修复未被拒绝: %v", err)
	}
	revisionLog(t, "gap_expected=4 gap_received=5 index_untouched=true repaired_added=4 kept=2 revision=5 rebuilds=1 stale_rejected=true")
}

func TestCapabilityRevisionUpdateReplacesDescriptor(t *testing.T) {
	store, index := emptyIndex(t)
	tools := descriptorTools(1)
	if _, err := index.Apply(context.Background(), store, registerEvents(tools, 2)); err != nil {
		t.Fatalf("注册: %v", err)
	}
	original, _ := index.Entry(tools[0].ID)
	updated := tools[0]
	updated.Description = "版本二的新描述"
	updated.InputSchema = descriptorSchema(999)
	report, err := index.Apply(context.Background(), store, []capability.DescriptorEvent{{Revision: 3, Kind: capability.DescriptorUpdate, Tool: updated}})
	if err != nil {
		t.Fatalf("更新: %v", err)
	}
	entry, ok := index.Entry(tools[0].ID)
	if !ok || report.Updated != 1 || index.Len() != 1 {
		t.Fatalf("更新结果 %+v len=%d", report, index.Len())
	}
	if entry.SchemaRef == original.SchemaRef || entry.Description != updated.Description {
		t.Fatalf("更新未替换描述符: %+v -> %+v", original, entry)
	}
	if _, err := store.Load(original.SchemaRef, false); err != nil {
		t.Fatalf("历史 Schema 不可读: %v", err)
	}
	revisionLog(t, "updated=1 len=1 new_ref=true old_schema_readable=true")
}

func TestCapabilityRevisionUnknownUnregisterIsSkipped(t *testing.T) {
	store, index := emptyIndex(t)
	report, err := index.Apply(context.Background(), store, []capability.DescriptorEvent{{Revision: 2, Kind: capability.DescriptorUnregister, ToolID: "tool-ghost"}})
	if err != nil {
		t.Fatalf("卸载未知工具: %v", err)
	}
	if report.Skipped != 1 || report.Applied != 0 || index.Revision() != 2 || index.Len() != 0 {
		t.Fatalf("未知卸载处理不符: %+v revision=%d", report, index.Revision())
	}
	revisionLog(t, "unknown_unregister=skipped applied=0 revision_consumed=2")
}

func TestCapabilityRevisionInvalidEventsRejected(t *testing.T) {
	store, index := emptyIndex(t)
	cases := []capability.DescriptorEvent{
		{Revision: 0, Kind: capability.DescriptorRegister, Tool: descriptorTools(1)[0]},
		{Revision: 2, Kind: "explode", Tool: descriptorTools(1)[0]},
		{Revision: 2, Kind: capability.DescriptorUnregister},
		{Revision: 2, Kind: capability.DescriptorRegister, Tool: capability.ToolDefinition{ID: "no-name"}},
	}
	for _, event := range cases {
		if _, err := index.Apply(context.Background(), store, []capability.DescriptorEvent{event}); err == nil {
			t.Fatalf("非法事件未被拒绝: %+v", event)
		}
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := index.Apply(canceled, store, registerEvents(descriptorTools(1), 2)); !errors.Is(err, context.Canceled) {
		t.Fatalf("取消未被透传: %v", err)
	}
	if index.Len() != 0 || index.Revision() != 1 {
		t.Fatalf("非法事件污染索引: len=%d revision=%d", index.Len(), index.Revision())
	}
	revisionLog(t, "zero_revision=rejected unknown_kind=rejected missing_id=rejected nameless=rejected canceled=passed_through")
}

func TestCapabilityRevisionRestartRebuildMatchesRepair(t *testing.T) {
	store, index := emptyIndex(t)
	tools := descriptorTools(40)
	if _, err := index.Apply(context.Background(), store, registerEvents(tools, 2)); err != nil {
		t.Fatalf("注册: %v", err)
	}
	if _, err := index.Apply(context.Background(), store, unregisterEvents(tools[:12], 42)); err != nil {
		t.Fatalf("卸载: %v", err)
	}
	snapshot := append([]capability.ToolDefinition{}, tools[12:]...)
	repaired, err := index.Repair(context.Background(), store, snapshot, 60)
	if err != nil {
		t.Fatalf("修复: %v", err)
	}
	restarted, err := capability.BuildDescriptorIndex(store, snapshot)
	if err != nil {
		t.Fatalf("重建: %v", err)
	}
	left := index.Query(capability.ToolFilter{})
	right := restarted.Query(capability.ToolFilter{})
	if len(left) != len(right) || len(left) != 28 {
		t.Fatalf("修复后 %d 条，重建后 %d 条", len(left), len(right))
	}
	for position := range left {
		if left[position] != right[position] {
			t.Fatalf("第 %d 条不一致", position)
		}
	}
	if repaired.Removed != 0 || index.Rebuilds() != 1 {
		t.Fatalf("修复报告 %+v rebuilds=%d", repaired, index.Rebuilds())
	}
	revisionLog(t, "registered=40 uninstalled=12 repaired=28 restart_match=true rebuilds=1")
}
