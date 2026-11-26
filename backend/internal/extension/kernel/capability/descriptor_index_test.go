package capability_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func descriptorLog(t *testing.T, line string) {
	t.Helper()
	t.Logf("m05-s01 %s", line)
}

func descriptorSchema(index int) []byte {
	return []byte(fmt.Sprintf(`{"type":"object","properties":{"query_%d":{"type":"string","description":"参数 %d 的说明，用于撑开 Schema 体积"}},"required":["query_%d"]}`, index, index, index))
}

func descriptorTools(count int) []capability.ToolDefinition {
	tools := make([]capability.ToolDefinition, 0, count)
	for index := 0; index < count; index++ {
		tools = append(tools, capability.ToolDefinition{
			ID:           fmt.Sprintf("tool-%04d", index),
			ModelName:    fmt.Sprintf("tool_%04d", index),
			Description:  fmt.Sprintf("第 %d 个工具的简短描述", index),
			Source:       capability.ToolSourcePlugin,
			CapabilityID: capability.CapabilityID(fmt.Sprintf("cap-%04d", index)),
			InputSchema:  descriptorSchema(index),
			Enabled:      true,
		})
	}
	return tools
}

func buildIndex(t *testing.T, count int) (*capability.SchemaStore, *capability.DescriptorIndex) {
	t.Helper()
	store := capability.NewSchemaStore(time.Hour)
	index, err := capability.BuildDescriptorIndex(store, descriptorTools(count))
	if err != nil {
		t.Fatalf("BuildDescriptorIndex: %v", err)
	}
	return store, index
}

func TestCapabilityDescriptorBuildKeepsDescriptorsOnly(t *testing.T) {
	store, index := buildIndex(t, 1000)
	if index.Len() != 1000 {
		t.Fatalf("index len=%d", index.Len())
	}
	if store.Writes() != 1000 || store.Reads() != 0 {
		t.Fatalf("writes=%d reads=%d", store.Writes(), store.Reads())
	}
	for _, entry := range index.Query(capability.ToolFilter{}) {
		if !entry.SchemaRef.Valid() {
			t.Fatalf("tool %s 的 schemaRef 非法: %q", entry.ToolID, entry.SchemaRef)
		}
		if entry.SchemaBytes <= 0 {
			t.Fatalf("tool %s 缺少 Schema 体积", entry.ToolID)
		}
	}
	if _, err := capability.BuildDescriptorIndex(nil, descriptorTools(1)); !errors.Is(err, capability.ErrDescriptorInvalid) {
		t.Fatalf("空 store 未被拒绝: %v", err)
	}
	descriptorLog(t, "tools=1000 entries=1000 schema_reads=0 schema_writes=1000 refs_valid=true")
}

func TestCapabilityDescriptorQueryDoesNotReadSchemas(t *testing.T) {
	store, index := buildIndex(t, 1000)
	before := store.Reads()
	plugins := index.Query(capability.ToolFilter{Source: capability.ToolSourcePlugin})
	if len(plugins) != 1000 {
		t.Fatalf("插件条目=%d", len(plugins))
	}
	builtin := index.Query(capability.ToolFilter{Source: capability.ToolSourceBuiltin})
	if len(builtin) != 0 {
		t.Fatalf("按来源过滤失效: %d", len(builtin))
	}
	for position := 1; position < len(plugins); position++ {
		if plugins[position-1].ToolID > plugins[position].ToolID {
			t.Fatalf("目录未按 ToolID 排序: %s > %s", plugins[position-1].ToolID, plugins[position].ToolID)
		}
	}
	if store.Reads() != before {
		t.Fatalf("目录查询读取了 Schema: %d -> %d", before, store.Reads())
	}
	descriptorLog(t, "queries=2 entries=1000 schema_reads=0 sorted=true")
}

func TestCapabilityDescriptorHydrateLoadsRequestedOnly(t *testing.T) {
	store, index := buildIndex(t, 1000)
	wanted := []string{"tool-0000", "tool-0007", "tool-0100", "tool-0500", "tool-0999"}
	views, err := index.Hydrate(context.Background(), store, wanted, capability.HydrateOptions{RequireFresh: true})
	if err != nil {
		t.Fatalf("Hydrate: %v", err)
	}
	if len(views) != len(wanted) {
		t.Fatalf("views=%d", len(views))
	}
	if store.Reads() != int64(len(wanted)) {
		t.Fatalf("只应读取 %d 个 Schema，实际 %d", len(wanted), store.Reads())
	}
	if string(views[1].InputSchema) != string(descriptorSchema(7)) {
		t.Fatalf("加载回的 Schema 与原始不一致")
	}
	if _, err := index.Hydrate(context.Background(), store, []string{"tool-missing"}, capability.HydrateOptions{}); !errors.Is(err, capability.ErrDescriptorUnknown) {
		t.Fatalf("未知工具未被拒绝: %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := index.Hydrate(canceled, store, wanted, capability.HydrateOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("取消未被透传: %v", err)
	}
	descriptorLog(t, "requested=5 hydrated=5 schema_reads=5 unknown=rejected canceled=passed_through")
}

func TestCapabilityDescriptorSchemaRefIsContentAddressed(t *testing.T) {
	store := capability.NewSchemaStore(time.Hour)
	body := descriptorSchema(1)
	first, created, err := store.Put(body)
	if err != nil || !created {
		t.Fatalf("首次写入: created=%v err=%v", created, err)
	}
	second, createdAgain, err := store.Put(body)
	if err != nil {
		t.Fatalf("重复写入: %v", err)
	}
	if first != second || createdAgain {
		t.Fatalf("重复写入未幂等: first=%s second=%s created=%v", first, second, createdAgain)
	}
	other, _, err := store.Put(descriptorSchema(2))
	if err != nil {
		t.Fatalf("写入另一内容: %v", err)
	}
	if other == first {
		t.Fatalf("不同内容得到同一 ref")
	}
	if store.Writes() != 2 || store.Len() != 2 {
		t.Fatalf("writes=%d len=%d", store.Writes(), store.Len())
	}
	if capability.NewSchemaRef(body) != first {
		t.Fatalf("ref 与内容哈希不一致")
	}
	descriptorLog(t, "ref=content_hash repeat=idempotent writes=2 distinct_refs=true")
}

func TestCapabilityDescriptorHistoricalRefNotOverwritable(t *testing.T) {
	store := capability.NewSchemaStore(time.Hour)
	body := descriptorSchema(3)
	ref, _, err := store.Put(body)
	if err != nil {
		t.Fatalf("写入: %v", err)
	}
	if _, err := store.PutAt(ref, descriptorSchema(4)); !errors.Is(err, capability.ErrSchemaRefMismatch) {
		t.Fatalf("哈希不一致未被拒绝: %v", err)
	}
	if _, err := store.PutAt(capability.SchemaRef("sha256:zz"), body); !errors.Is(err, capability.ErrSchemaRefInvalid) {
		t.Fatalf("非法 ref 未被拒绝: %v", err)
	}
	stored, ok := store.Get(ref)
	if !ok || string(stored) != string(body) {
		t.Fatalf("历史 ref 内容被改动")
	}
	if _, err := store.Load(capability.NewSchemaRef(descriptorSchema(5)), false); !errors.Is(err, capability.ErrSchemaRefMissing) {
		t.Fatalf("缺失 ref 未被拒绝: %v", err)
	}
	descriptorLog(t, "overwrite=mismatch_rejected invalid_ref=rejected historical_read=identical missing=rejected")
}

func TestCapabilityDescriptorTTLRequiresReindex(t *testing.T) {
	now := time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)
	store := capability.NewSchemaStoreWithClock(time.Minute, func() time.Time { return now })
	body := descriptorSchema(6)
	ref, _, err := store.Put(body)
	if err != nil {
		t.Fatalf("写入: %v", err)
	}
	if fresh, found := store.Fresh(ref, now); !fresh || !found {
		t.Fatalf("刚写入即过期")
	}
	now = now.Add(2 * time.Minute)
	if fresh, found := store.Fresh(ref, now); fresh || !found {
		t.Fatalf("TTL 未生效")
	}
	if _, err := store.Load(ref, true); !errors.Is(err, capability.ErrSchemaExpired) {
		t.Fatalf("过期未被拒绝: %v", err)
	}
	stale, err := store.Load(ref, false)
	if err != nil || string(stale) != string(body) {
		t.Fatalf("过期历史 Schema 应仍可读: err=%v", err)
	}
	if ttl, ok := store.RefTTL(ref); !ok || !ttl.Equal(now.Add(-time.Minute)) {
		t.Fatalf("TTL 截止时间不符: %v ok=%v", ttl, ok)
	}
	descriptorLog(t, "ttl=1m fresh_then_expired=true require_fresh=rejected historical=readable")
}

func TestCapabilityDescriptorInvalidInputsRejected(t *testing.T) {
	store := capability.NewSchemaStore(time.Hour)
	if _, _, err := store.Put(nil); !errors.Is(err, capability.ErrSchemaBodyInvalid) {
		t.Fatalf("空内容未被拒绝: %v", err)
	}
	if _, _, err := store.Put([]byte("{not json")); !errors.Is(err, capability.ErrSchemaBodyInvalid) {
		t.Fatalf("非法 JSON 未被拒绝: %v", err)
	}
	duplicated := descriptorTools(2)
	duplicated[1].ID = duplicated[0].ID
	if _, err := capability.BuildDescriptorIndex(store, duplicated); !errors.Is(err, capability.ErrDescriptorInvalid) {
		t.Fatalf("重复 ToolID 未被拒绝: %v", err)
	}
	nameless := descriptorTools(1)
	nameless[0].ModelName = ""
	if _, err := capability.BuildDescriptorIndex(store, nameless); !errors.Is(err, capability.ErrDescriptorInvalid) {
		t.Fatalf("缺少模型名未被拒绝: %v", err)
	}
	descriptorLog(t, "empty_body=rejected malformed_json=rejected duplicate_id=rejected nameless=rejected")
}

func TestCapabilityDescriptorRepeatedBuildIsStable(t *testing.T) {
	store, index := buildIndex(t, 50)
	first := index.Query(capability.ToolFilter{})
	again, err := capability.BuildDescriptorIndex(store, descriptorTools(50))
	if err != nil {
		t.Fatalf("重建: %v", err)
	}
	if store.Writes() != 50 {
		t.Fatalf("重建不应重复写入: writes=%d", store.Writes())
	}
	second := again.Query(capability.ToolFilter{})
	if len(first) != len(second) {
		t.Fatalf("重建条目数不一致")
	}
	for position := range first {
		if first[position] != second[position] {
			t.Fatalf("第 %d 条重建后不一致", position)
		}
	}
	descriptorLog(t, "tools=50 rebuild_writes=50 entries_equal=true")
}
