package chat

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

func cachePromptFixture(t *testing.T, namespace string, round int) (context.Context, business.Inference, *interaction.RuntimeAssembly) {
	t.Helper()
	profile, _ := json.Marshal(map[string]any{"characterId": "role", "name": namespace, "identity": "负责项目资料整理的中文助手", "basePrompt": strings.Repeat("回答只依据已提供资料，不编造事实。识别用户当前任务，保留资料中的约束、日期和验收条件。\n", 12)})
	scope := coordination.ExecutionScope{SpaceID: "space", RoleID: "role", RoleRevision: 1, ResourceOwnerID: "device", RoleOwnerID: "device", RequestID: fmt.Sprintf("round-%d", round)}
	snapshot := coordination.DataSnapshot{OwnerID: "device", Role: coordination.Role{ID: "role", Revision: 1, Profile: profile}}
	for i := 0; i < 12+round; i++ {
		user := fmt.Sprintf("资料段落%d：", i) + strings.Repeat("项目计划要求先核对接口契约，再检查资料归属、输入格式与验收结果。每个阶段由负责人员记录进度；遇到信息不足时先明确缺失条件。交付材料包括变更说明、验证结果和剩余事项。", 3)
		for _, row := range []map[string]string{{"role": "user", "content": user}, {"role": "assistant", "content": fmt.Sprintf("已记录资料段落%d的约束与验收条件。", i)}} {
			raw, _ := json.Marshal(row)
			snapshot.LegacyMessages = append(snapshot.LegacyMessages, raw)
		}
	}
	inference := business.Inference{Scope: scope, Snapshot: snapshot, ConversationID: "conversation", Message: fmt.Sprintf("请检查第%d轮资料的一致性，本轮只回复收到。", round)}
	ctx := coordination.WithScope(t.Context(), scope)
	ctx, err := interaction.WithOwnedContext(ctx, scope, snapshot, "conversation", "web")
	if err != nil {
		t.Fatal(err)
	}
	tracker := interaction.NewInMemoryTracker()
	record := interaction.NewInteractionRecord(interaction.InteractionScope{SpaceID: "space", CharacterID: "role", ConversationID: "conversation", RequestID: scope.RequestID, Channel: "web"})
	if err := tracker.Create(ctx, record); err != nil {
		t.Fatal(err)
	}
	record, err = tracker.TransitionCAS(ctx, record.ID, record.StatusVersion, interaction.InteractionStatusProcessing)
	if err != nil {
		t.Fatal(err)
	}
	runtimeService := interaction.NewRuntimeContextService(tracker, 1)
	ctx, runtime, finish, err := runtimeService.PrepareOwnedRuntime(ctx, record, &interaction.ProcessRequest{SpaceID: "space", CharacterID: "role", ConversationID: "conversation", RequestID: scope.RequestID, Channel: "web", Message: inference.Message})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = finish(context.Background(), true) })
	return ctx, inference, runtime
}

type cacheBenchmarkRow struct {
	Replica      int   `json:"replica"`
	Round        int   `json:"round"`
	InputTokens  int   `json:"inputTokens"`
	CachedTokens int   `json:"cachedTokens"`
	OutputTokens int   `json:"outputTokens"`
	ElapsedMs    int64 `json:"elapsedMs"`
}

func TestOwnedPromptLiveCacheBenchmark(t *testing.T) {
	output := os.Getenv("AMITIA_CACHE_BENCHMARK_OUTPUT")
	if output == "" {
		t.Skip("live cache benchmark requires explicit output and database paths")
	}
	db, err := sql.Open("sqlite", "file:"+os.Getenv("AMITIA_CACHE_BENCHMARK_DB")+"?mode=ro")
	if err != nil {
		t.Fatal("open benchmark configuration failed")
	}
	defer db.Close()
	cfg := &ModelConfig{}
	err = db.QueryRow("SELECT id,base_url,api_key,model_name,api_type,protocol FROM model_configs WHERE is_active=1 LIMIT 1").Scan(&cfg.ID, &cfg.BaseURL, &cfg.APIKey, &cfg.ModelName, &cfg.APIType, &cfg.Protocol)
	if err != nil {
		t.Fatal("read active model failed")
	}
	if cfg.APIType != "deepseek" {
		t.Fatal("benchmark requires the configured DeepSeek provider")
	}
	cfg.MaxTokens = 16
	cfg.MaxOutputTokens = 16
	cfg.TimeoutSeconds = 60
	rows := []cacheBenchmarkRow{}
	save := func() {
		report := map[string]any{"stage": os.Getenv("AMITIA_CACHE_BENCHMARK_STAGE"), "model": cfg.ModelName, "createdAt": time.Now().UTC(), "workload": "synthetic-fixed-history-v1", "replicas": 2, "rounds": 6, "rows": rows}
		raw, e := json.MarshalIndent(report, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.MkdirAll(filepath.Dir(output), 0755); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(output, raw, 0644); e != nil {
			t.Fatal(e)
		}
	}
	for replica := 0; replica < 2; replica++ {
		var nonce [8]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			t.Fatal(err)
		}
		namespace := hex.EncodeToString(nonce[:])
		for round := 0; round < 6; round++ {
			ctx, inference, _ := cachePromptFixture(t, namespace, round)
			messages, err := ownedPromptContext(ctx, inference)
			if err != nil {
				t.Fatal(err)
			}
			start := time.Now()
			result, err := (&service{}).callLLMStreamAdapter(ctx, cfg, messages, nil, false, true, noopEventSink{})
			if err != nil {
				save()
				t.Fatalf("provider request failed at replica %d round %d", replica, round)
			}
			if !result.Usage.CacheUsageReported {
				save()
				t.Fatal("provider omitted cache usage")
			}
			row := cacheBenchmarkRow{Replica: replica, Round: round, InputTokens: result.Usage.InputTokens, CachedTokens: result.Usage.CachedInputTokens, OutputTokens: result.Usage.OutputTokens, ElapsedMs: time.Since(start).Milliseconds()}
			rows = append(rows, row)
			save()
			t.Logf("replica=%d round=%d input=%d cached=%d rate=%.2f%%", replica, round, row.InputTokens, row.CachedTokens, 100*float64(row.CachedTokens)/float64(row.InputTokens))
			select {
			case <-time.After(3 * time.Second):
			case <-t.Context().Done():
				t.Fatal("benchmark cancelled")
			}
		}
	}
}
