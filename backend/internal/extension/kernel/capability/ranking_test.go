package capability_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type rankingFixtureTool struct {
	ID        string   `json:"id"`
	ModelName string   `json:"modelName"`
	Keywords  []string `json:"keywords"`
	Domains   []string `json:"domains"`
}

type rankingFixtureUsage struct {
	ToolID    string `json:"toolId"`
	Calls     int64  `json:"calls"`
	Successes int64  `json:"successes"`
	Failures  int64  `json:"failures"`
}

type rankingFixtureQuery struct {
	Text string `json:"text"`
	Mode string `json:"mode"`
	Top1 string `json:"top1"`
}

type rankingFixture struct {
	SchemaVersion int                       `json:"schemaVersion"`
	Weights       capability.RankingWeights `json:"weights"`
	Tools         []rankingFixtureTool      `json:"tools"`
	Usage         []rankingFixtureUsage     `json:"usage"`
	Queries       []rankingFixtureQuery     `json:"queries"`
}

type rankingUsage struct {
	stats map[string]capability.UsageStat
	err   error
}

func (u rankingUsage) Usage(ctx context.Context, toolID string) (capability.UsageStat, bool, error) {
	if u.err != nil {
		return capability.UsageStat{}, false, u.err
	}
	stat, ok := u.stats[toolID]
	return stat, ok, nil
}

func loadRankingFixture(t *testing.T) rankingFixture {
	t.Helper()
	body, err := os.ReadFile("testdata/ranking_fixture.json")
	if err != nil {
		t.Fatalf("read ranking fixture: %v", err)
	}
	var fixture rankingFixture
	if err := json.Unmarshal(body, &fixture); err != nil {
		t.Fatalf("decode ranking fixture: %v", err)
	}
	return fixture
}

func rankingUsageProvider(fixture rankingFixture) rankingUsage {
	stats := map[string]capability.UsageStat{}
	for _, item := range fixture.Usage {
		stats[item.ToolID] = capability.UsageStat{Calls: item.Calls, Successes: item.Successes, Failures: item.Failures}
	}
	return rankingUsage{stats: stats}
}

func rankingTools(fixture rankingFixture) []capability.ToolDefinition {
	defs := make([]capability.ToolDefinition, 0, len(fixture.Tools))
	for _, tool := range fixture.Tools {
		metadata := map[string]any{"keywords": tool.Keywords, "domains": tool.Domains}
		def := taskGateTool(tool.ID, tool.ModelName, tool.Keywords...)
		def.Metadata = metadata
		defs = append(defs, def)
	}
	return defs
}

func rankingRegistry(t *testing.T, fixture rankingFixture) *capability.ToolRegistry {
	t.Helper()
	return taskGateRegistry(t, rankingTools(fixture)...)
}

func rankingHybrid(t *testing.T, fixture rankingFixture, usage capability.UsageProvider) *capability.HybridStrategy {
	t.Helper()
	hybrid, err := capability.NewHybridStrategy(fixture.Weights, usage)
	if err != nil {
		t.Fatalf("NewHybridStrategy: %v", err)
	}
	return hybrid
}

func rankingPolicy(t *testing.T, fixture rankingFixture) *capability.RankingPolicy {
	t.Helper()
	policy, err := capability.NewRankingPolicy(capability.LexicalStrategy{}, rankingHybrid(t, fixture, rankingUsageProvider(fixture)))
	if err != nil {
		t.Fatalf("NewRankingPolicy: %v", err)
	}
	if err := policy.Activate(context.Background(), capability.RankingVersionHybrid); err != nil {
		t.Fatalf("Activate hybrid: %v", err)
	}
	return policy
}

func rankingTop1(t *testing.T, selector *capability.TaskSelector, query string) capability.SelectionReport {
	t.Helper()
	report, err := selector.Select(context.Background(), capability.ToolInvocationContext{InvocationID: "inv-ranking", SpaceID: "space-1"}, query, 0)
	if err != nil {
		t.Fatalf("Select %q: %v", query, err)
	}
	if len(report.Selected) == 0 {
		t.Fatalf("Select %q returned no candidate", query)
	}
	return report
}

func TestRankingExplicitQueryHitsTop1(t *testing.T) {
	fixture := loadRankingFixture(t)
	selector := capability.NewTaskSelector(rankingRegistry(t, fixture), nil).WithRanking(rankingPolicy(t, fixture))
	explicit := map[string]string{}
	for _, query := range fixture.Queries {
		if query.Mode != "explicit" {
			continue
		}
		report := rankingTop1(t, selector, query.Text)
		if report.Selected[0].Tool.ID != query.Top1 {
			t.Fatalf("explicit query %q top1=%s want %s", query.Text, report.Selected[0].Tool.ID, query.Top1)
		}
		explicit[query.Top1] = report.Selected[0].Tool.ID
	}
	if len(explicit) != 2 {
		t.Fatalf("explicit queries=%d want 2", len(explicit))
	}
	t.Logf("m05-s04 explicit_writer_top1=%s explicit_reader_top1=%s", explicit["tool-report-writer"], explicit["tool-report-reader"])
}

func TestRankingConfusableQueryPicksUsedService(t *testing.T) {
	fixture := loadRankingFixture(t)
	var confusable rankingFixtureQuery
	for _, query := range fixture.Queries {
		if query.Mode == "confusable" {
			confusable = query
		}
	}
	if confusable.Text == "" {
		t.Fatal("fixture has no confusable query")
	}
	lexicalPolicy, err := capability.NewRankingPolicy(capability.LexicalStrategy{})
	if err != nil {
		t.Fatalf("NewRankingPolicy lexical: %v", err)
	}
	registry := rankingRegistry(t, fixture)
	lexicalReport := rankingTop1(t, capability.NewTaskSelector(registry, nil).WithRanking(lexicalPolicy), confusable.Text)
	if lexicalReport.Selected[0].Tool.ID != "tool-report-reader" {
		t.Fatalf("lexical tie-break top1=%s want tool-report-reader", lexicalReport.Selected[0].Tool.ID)
	}
	hybridReport := rankingTop1(t, capability.NewTaskSelector(registry, nil).WithRanking(rankingPolicy(t, fixture)), confusable.Text)
	if hybridReport.Selected[0].Tool.ID != confusable.Top1 {
		t.Fatalf("hybrid top1=%s want %s", hybridReport.Selected[0].Tool.ID, confusable.Top1)
	}
	prior := rankingUsageProvider(fixture).stats["tool-report-writer"]
	if prior.Calls != 40 {
		t.Fatalf("fixture usage for tool-report-writer calls=%d want 40", prior.Calls)
	}
	t.Logf("m05-s04 confusable_lexical_top1=%s hybrid_top1=%s usage_prior=%.2f", lexicalReport.Selected[0].Tool.ID, hybridReport.Selected[0].Tool.ID, prior.Reliability()*(1+prior.Volume()))
}

func TestRankingSnapshotRollback(t *testing.T) {
	ctx := context.Background()
	fixture := loadRankingFixture(t)
	policy, err := capability.NewRankingPolicy(capability.LexicalStrategy{}, rankingHybrid(t, fixture, rankingUsageProvider(fixture)))
	if err != nil {
		t.Fatalf("NewRankingPolicy: %v", err)
	}
	selector := capability.NewTaskSelector(rankingRegistry(t, fixture), nil).WithRanking(policy)
	before := policy.Snapshot()
	if before.Version != capability.RankingVersionLexical {
		t.Fatalf("initial version=%s want %s", before.Version, capability.RankingVersionLexical)
	}
	if !strings.HasPrefix(before.Digest, "sha256:") || len(before.Digest) != len("sha256:")+64 {
		t.Fatalf("snapshot digest=%q is not a sha256 digest", before.Digest)
	}
	if top := rankingTop1(t, selector, "把周报整理成报告").Selected[0].Tool.ID; top != "tool-report-reader" {
		t.Fatalf("lexical top1=%s want tool-report-reader", top)
	}
	if err := policy.Activate(ctx, capability.RankingVersionHybrid); err != nil {
		t.Fatalf("Activate hybrid: %v", err)
	}
	if top := rankingTop1(t, selector, "把周报整理成报告").Selected[0].Tool.ID; top != "tool-report-writer" {
		t.Fatalf("hybrid top1=%s want tool-report-writer", top)
	}
	restored, err := policy.Rollback(ctx)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if restored.Version != capability.RankingVersionLexical || restored.Previous != capability.RankingVersionHybrid {
		t.Fatalf("rollback snapshot version=%s previous=%s", restored.Version, restored.Previous)
	}
	after := rankingTop1(t, selector, "把周报整理成报告")
	if after.Selected[0].Tool.ID != "tool-report-reader" || after.Strategy.Version != capability.RankingVersionLexical {
		t.Fatalf("rollback top1=%s strategy=%s", after.Selected[0].Tool.ID, after.Strategy.Version)
	}
	t.Logf("m05-s04 snapshot_version=%s digest_sha256=true rollback_version=%s rollback_restored=true", before.Version, restored.Version)
}

func TestRankingSnapshotRestoreAfterRestart(t *testing.T) {
	ctx := context.Background()
	fixture := loadRankingFixture(t)
	policy := rankingPolicy(t, fixture)
	snapshot := policy.Snapshot()
	if snapshot.Version != capability.RankingVersionHybrid {
		t.Fatalf("active version=%s want %s", snapshot.Version, capability.RankingVersionHybrid)
	}
	fresh, err := capability.NewRankingPolicy(capability.LexicalStrategy{}, rankingHybrid(t, fixture, rankingUsageProvider(fixture)))
	if err != nil {
		t.Fatalf("NewRankingPolicy fresh: %v", err)
	}
	if err := fresh.Restore(ctx, snapshot); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if fresh.Version() != snapshot.Version {
		t.Fatalf("restored version=%s want %s", fresh.Version(), snapshot.Version)
	}
	selector := capability.NewTaskSelector(rankingRegistry(t, fixture), nil).WithRanking(fresh)
	if top := rankingTop1(t, selector, "把周报整理成报告").Selected[0].Tool.ID; top != "tool-report-writer" {
		t.Fatalf("restored top1=%s want tool-report-writer", top)
	}
	tampered := snapshot
	tampered.Weights.Usage = snapshot.Weights.Usage + 1
	if err := fresh.Restore(ctx, tampered); !errors.Is(err, capability.ErrRankingSnapshotDrift) {
		t.Fatalf("tampered Restore err=%v want ErrRankingSnapshotDrift", err)
	}
	t.Logf("m05-s04 restart_restored=true snapshot_version=%s corrupted_snapshot=drift", snapshot.Version)
}

func TestRankingRejectsInvalidInputs(t *testing.T) {
	ctx := context.Background()
	if _, err := capability.NewRankingPolicy(); !errors.Is(err, capability.ErrRankingStrategyMissing) {
		t.Fatalf("empty policy err=%v want ErrRankingStrategyMissing", err)
	}
	if _, err := capability.NewRankingPolicy(nil); !errors.Is(err, capability.ErrRankingStrategyMissing) {
		t.Fatalf("nil policy err=%v want ErrRankingStrategyMissing", err)
	}
	if _, err := capability.NewRankingPolicy(capability.LexicalStrategy{}, capability.LexicalStrategy{}); !errors.Is(err, capability.ErrRankingStrategyMissing) {
		t.Fatalf("duplicate version err=%v want ErrRankingStrategyMissing", err)
	}
	if _, err := capability.NewHybridStrategy(capability.RankingWeights{Lexical: -1, Domain: 1}, nil); !errors.Is(err, capability.ErrRankingWeightsInvalid) {
		t.Fatalf("negative weights err=%v want ErrRankingWeightsInvalid", err)
	}
	if _, err := capability.NewHybridStrategy(capability.RankingWeights{}, nil); !errors.Is(err, capability.ErrRankingWeightsInvalid) {
		t.Fatalf("zero weights err=%v want ErrRankingWeightsInvalid", err)
	}
	policy, err := capability.NewRankingPolicy(capability.LexicalStrategy{})
	if err != nil {
		t.Fatalf("NewRankingPolicy: %v", err)
	}
	if err := policy.Activate(ctx, "hybrid-v9"); !errors.Is(err, capability.ErrRankingVersionUnknown) {
		t.Fatalf("unknown version err=%v want ErrRankingVersionUnknown", err)
	}
	if _, err := policy.Rollback(ctx); !errors.Is(err, capability.ErrRankingRollbackUnavailable) {
		t.Fatalf("rollback without previous err=%v want ErrRankingRollbackUnavailable", err)
	}
	t.Logf("m05-s04 missing_strategy=rejected invalid_weights=rejected unknown_version=rejected empty_rollback=rejected")
}

func TestRankingRepeatAndCancel(t *testing.T) {
	ctx := context.Background()
	fixture := loadRankingFixture(t)
	policy := rankingPolicy(t, fixture)
	candidates := []capability.RankCandidate{
		{Tool: taskGateTool("tool-report-writer", "report_writer", "周报"), Lexical: 12, Domains: []capability.TaskDomain{capability.TaskDomainDocument}},
		{Tool: taskGateTool("tool-report-reader", "report_reader", "周报"), Lexical: 12, Domains: []capability.TaskDomain{capability.TaskDomainDocument}},
	}
	profile := capability.ClassifyTask("把周报整理成报告")
	first, firstSnapshot, err := policy.Rank(ctx, profile, "把周报整理成报告", candidates)
	if err != nil {
		t.Fatalf("first Rank: %v", err)
	}
	second, secondSnapshot, err := policy.Rank(ctx, profile, "把周报整理成报告", candidates)
	if err != nil {
		t.Fatalf("second Rank: %v", err)
	}
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("ranked sizes=%d/%d want 2/2", len(first), len(second))
	}
	for index := range first {
		if first[index].Tool.ID != second[index].Tool.ID {
			t.Fatalf("repeat order drifted at %d: %s vs %s", index, first[index].Tool.ID, second[index].Tool.ID)
		}
	}
	if firstSnapshot != secondSnapshot {
		t.Fatalf("repeat snapshot drifted: %+v vs %+v", firstSnapshot, secondSnapshot)
	}
	if policy.Rankings() != 2 {
		t.Fatalf("rankings=%d want 2", policy.Rankings())
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	ranked, _, err := policy.Rank(canceled, profile, "把周报整理成报告", candidates)
	if !errors.Is(err, context.Canceled) || ranked != nil {
		t.Fatalf("canceled Rank err=%v ranked=%v want context.Canceled and nil", err, ranked)
	}
	if policy.Rankings() != 2 {
		t.Fatalf("canceled rank changed counter to %d", policy.Rankings())
	}
	t.Logf("m05-s04 repeat_orders_equal=true rankings=%d canceled=passed_through", policy.Rankings())
}

func TestRankingSelectorKeepsHardFilter(t *testing.T) {
	fixture := loadRankingFixture(t)
	blocked := taskGateTool("tool-deploy-blocked", "deploy_service", "周报", "部署")
	blocked.Metadata = map[string]any{"keywords": []string{"周报", "部署"}, "domains": []string{"document"}}
	registry := rankingRegistry(t, fixture)
	if err := registry.Register(context.Background(), blocked); err != nil {
		t.Fatalf("Register blocked: %v", err)
	}
	selector := capability.NewTaskSelector(registry, nil).
		WithGrantResolver(taskGateGrants{denied: map[string]bool{blocked.ID: true}}).
		WithRanking(rankingPolicy(t, fixture))
	report := rankingTop1(t, selector, "把周报整理成报告")
	if report.Claimed(blocked.ID) {
		t.Fatalf("denied tool %s appeared in selection", blocked.ID)
	}
	if report.Selected[0].Tool.ID != "tool-report-writer" {
		t.Fatalf("selector top1=%s want tool-report-writer", report.Selected[0].Tool.ID)
	}
	if report.Strategy.Version != capability.RankingVersionHybrid {
		t.Fatalf("report strategy=%s want %s", report.Strategy.Version, capability.RankingVersionHybrid)
	}
	reasons := report.Reasons(blocked.ID)
	if !taskGateHasReason(reasons, capability.ReasonPermissionMissing) {
		t.Fatalf("denied tool reasons=%v want permission missing", reasons)
	}
	if len(report.Selected[0].Signals) == 0 {
		t.Fatal("selected tool exposes no ranking signals")
	}
	t.Logf("m05-s04 selector_strategy=%s selector_top1=%s denied_filtered=true", report.Strategy.Version, report.Selected[0].Tool.ID)
}

func TestRankingUsageProviderErrorIsChained(t *testing.T) {
	sentinel := errors.New("usage store unavailable")
	hybrid, err := capability.NewHybridStrategy(capability.RankingWeights{Lexical: 1, Usage: 1}, rankingUsage{err: sentinel})
	if err != nil {
		t.Fatalf("NewHybridStrategy: %v", err)
	}
	policy, err := capability.NewRankingPolicy(hybrid)
	if err != nil {
		t.Fatalf("NewRankingPolicy: %v", err)
	}
	candidates := []capability.RankCandidate{{Tool: taskGateTool("tool-report-writer", "report_writer", "周报"), Lexical: 12}}
	ranked, _, err := policy.Rank(context.Background(), capability.ClassifyTask("周报"), "周报", candidates)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Rank err=%v want sentinel", err)
	}
	if ranked != nil {
		t.Fatalf("Rank returned %d rows on usage error", len(ranked))
	}
	t.Logf("m05-s04 usage_error=chained top1_empty=true")
}

func TestRankingFixtureIntegrity(t *testing.T) {
	fixture := loadRankingFixture(t)
	if fixture.SchemaVersion != 1 {
		t.Fatalf("schemaVersion=%d want 1", fixture.SchemaVersion)
	}
	if len(fixture.Tools) != 4 {
		t.Fatalf("fixture tools=%d want 4", len(fixture.Tools))
	}
	if len(fixture.Queries) != 3 {
		t.Fatalf("fixture queries=%d want 3", len(fixture.Queries))
	}
	if !fixture.Weights.Valid() {
		t.Fatalf("fixture weights invalid: %+v", fixture.Weights)
	}
	ids := map[string]bool{}
	for _, tool := range fixture.Tools {
		ids[tool.ID] = true
	}
	modes := map[string]bool{}
	for _, query := range fixture.Queries {
		modes[query.Mode] = true
		if !ids[query.Top1] {
			t.Fatalf("query %q expects unknown tool %s", query.Text, query.Top1)
		}
	}
	if !modes["explicit"] || !modes["confusable"] {
		t.Fatalf("fixture modes=%v want explicit and confusable", modes)
	}
	t.Logf("m05-s04 fixture_tools=%d fixture_queries=%d modes=explicit,confusable top1_valid=true", len(fixture.Tools), len(fixture.Queries))
}
