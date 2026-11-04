package agentplatform

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/u-ai/backend/internal/extension/kernel/packageverify"
)

func newTestPlatform(t *testing.T) (*Service, string, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(dir, "agent-platform.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(AllModels()...); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE characters (
		id TEXT PRIMARY KEY,
		space_id TEXT NOT NULL DEFAULT '',
		name TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'enabled',
		base_prompt TEXT NOT NULL DEFAULT '',
		generated_prompt TEXT NOT NULL DEFAULT '',
		revision INTEGER NOT NULL DEFAULT 1,
		deleted_at TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT ''
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE model_configs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL DEFAULT '',
		model_name TEXT NOT NULL DEFAULT '',
		is_active INTEGER NOT NULL DEFAULT 0
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO model_configs(name, model_name, is_active) VALUES('test-model', 'test-model', 1)`).Error; err != nil {
		t.Fatal(err)
	}
	service, err := NewService(db, ServiceConfig{DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	service.SetWorkspaceRegistrar(testWorkspaceRegistrar{})
	owner, err := service.EnsureLocalOwner(context.Background(), "space_test", "测试用户")
	if err != nil {
		t.Fatal(err)
	}
	return service, "space_test", owner.ID
}

func createTestAgent(t *testing.T, service *Service, spaceID, ownerID, name string, teamIDs ...string) AgentDetail {
	t.Helper()
	characterID := newID("character")
	if err := service.db.Exec(
		`INSERT INTO characters(id,space_id,name,description,base_prompt,status,revision) VALUES(?,?,?,?,?,?,1)`,
		characterID, spaceID, name, "测试角色", "验证任务之后再报告完成。", "enabled",
	).Error; err != nil {
		t.Fatal(err)
	}
	profile, err := service.SyncRoleAgent(service.db, RoleAgentSyncInput{
		SpaceID: spaceID, CharacterID: characterID, Name: name,
		Description: "测试角色", Instructions: "验证任务之后再报告完成。",
		Status: "enabled", ModelConfigID: 1, ReasoningEffort: "medium",
	})
	if err != nil {
		t.Fatal(err)
	}
	teamID := ""
	if len(teamIDs) > 0 {
		teamID = teamIDs[0]
	}
	if err := service.db.Model(&AgentProfile{}).Where("id = ?", profile.ID).Updates(map[string]any{
		"owner_id": ownerID, "archetype": "worker", "team_id": teamID,
	}).Error; err != nil {
		t.Fatal(err)
	}
	detail, err := service.GetAgent(context.Background(), spaceID, profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	return detail
}

type testWorkspaceRegistrar struct{}

func (testWorkspaceRegistrar) RegisterRuntimeWorkspace(_ context.Context, agentID, name, localRoot string) (RuntimeWorkspace, error) {
	return RuntimeWorkspace{ID: "workspace-" + agentID, Name: name, RootURI: "amitia://workspace/@" + agentID + "/"}, nil
}

func TestCreateTeamWithSelectedMembers(t *testing.T) {
	service, spaceID, ownerID := newTestPlatform(t)
	ctx := context.Background()
	lead := createTestAgent(t, service, spaceID, ownerID, "负责人角色")
	member := createTestAgent(t, service, spaceID, ownerID, "执行角色")
	team, err := service.CreateTeam(ctx, spaceID, CreateTeamInput{
		Name:        "成员选择团队",
		OwnerID:     ownerID,
		LeadAgentID: lead.Profile.ID,
		MemberIDs:   []string{member.Profile.ID, member.Profile.ID, lead.Profile.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := service.GetTeam(ctx, spaceID, team.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Members) != 2 {
		t.Fatalf("expected lead and selected member, got %d", len(detail.Members))
	}
	roles := map[string]string{}
	for _, item := range detail.Members {
		roles[item.MemberID] = item.Role
	}
	if roles[lead.Profile.ID] != "lead" || roles[member.Profile.ID] != "member" {
		t.Fatalf("unexpected member roles: %#v", roles)
	}
}

func TestApprovalBrokerPersistsAndResumesWithoutLiveWaiter(t *testing.T) {
	service, spaceID, ownerID := newTestPlatform(t)
	ctx := context.Background()
	agent := createTestAgent(t, service, spaceID, ownerID, "持久审批")
	broker := execution.NewApprovalBroker()
	broker.SetPersistence(service)
	service.SetApprovalBroker(broker)
	request := execution.ApprovalRequest{
		SpaceID: spaceID, AgentID: agent.Profile.ID, CharacterID: agent.Profile.CharacterID,
		ConversationID: "conversation", ToolCallID: "call", ToolName: "email.send",
		Arguments: `{"to":"test@example.com"}`, RiskLevel: "high",
	}
	result := make(chan bool, 1)
	go func() {
		approved, awaitErr := broker.Await(ctx, request, time.Minute)
		if awaitErr != nil {
			result <- false
			return
		}
		result <- approved
	}()
	var pending []ApprovalRecord
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := service.db.WithContext(ctx).Where(
			"space_id = ? AND agent_id = ? AND conversation_id = ? AND status = ?",
			spaceID, agent.Profile.ID, "conversation", "pending",
		).Order("created_at DESC").Limit(10).Find(&pending).Error; err != nil {
			t.Fatal(err)
		}
		if len(pending) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(pending) != 1 {
		t.Fatal("approval request was not persisted")
	}
	if err := broker.ResolveWithActor(pending[0].ID, true, execution.ApprovalActor{
		ID: ownerID, PrincipalType: "platform_user", AuthMethod: "platform_session", SessionID: ownerID,
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case approved := <-result:
		if !approved {
			t.Fatal("active approval waiter was not resumed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("active approval waiter did not resume")
	}
	restartedBroker := execution.NewApprovalBroker()
	restartedBroker.SetPersistence(service)
	approved, err := restartedBroker.Await(ctx, request, 100*time.Millisecond)
	if err != nil || !approved {
		t.Fatalf("persisted approval did not resume after restart: %t %v", approved, err)
	}
}

func TestResourceLeaseAndIdempotency(t *testing.T) {
	service, spaceID, ownerID := newTestPlatform(t)
	ctx := context.Background()
	agentA := createTestAgent(t, service, spaceID, ownerID, "A")
	agentB := createTestAgent(t, service, spaceID, ownerID, "B")
	lease, err := service.AcquireResourceLease(ctx, spaceID, AcquireResourceLeaseInput{AgentID: agentA.Profile.ID, ResourceType: "calendar", ResourceID: "primary", Mode: "exclusive", TTLSeconds: 60})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AcquireResourceLease(ctx, spaceID, AcquireResourceLeaseInput{AgentID: agentB.Profile.ID, ResourceType: "calendar", ResourceID: "primary", Mode: "shared", TTLSeconds: 60}); err == nil {
		t.Fatal("expected exclusive resource conflict")
	}
	if _, err := service.ReleaseResourceLease(ctx, spaceID, lease.ID, agentA.Profile.ID); err != nil {
		t.Fatal(err)
	}
	begin, err := service.BeginIdempotentOperation(ctx, spaceID, BeginIdempotentOperationInput{AgentID: agentA.Profile.ID, ScopeKey: "calendar", IdempotencyKey: "send-1", Input: map[string]any{"event": "x"}})
	if err != nil || !begin.Created {
		t.Fatalf("begin idempotency failed: %+v %v", begin, err)
	}
	again, err := service.BeginIdempotentOperation(ctx, spaceID, BeginIdempotentOperationInput{AgentID: agentA.Profile.ID, ScopeKey: "calendar", IdempotencyKey: "send-1", Input: map[string]any{"event": "x"}})
	if err != nil || again.Created || again.Operation.ID != begin.Operation.ID {
		t.Fatalf("idempotent replay failed: %+v %v", again, err)
	}
	if _, err := service.CompleteIdempotentOperation(ctx, spaceID, begin.Operation.ID, CompleteIdempotentOperationInput{Result: map[string]any{"ok": true}, ExpectedRevision: begin.Operation.Revision}); err != nil {
		t.Fatal(err)
	}
}

func TestModelUsageActivityAndTotals(t *testing.T) {
	service, spaceID, ownerID := newTestPlatform(t)
	ctx := WithPlatformActor(context.Background(), ownerID, "platform_user")
	team, err := service.CreateTeam(ctx, spaceID, CreateTeamInput{Name: "模型预算团队", OwnerID: ownerID, SharedWorkspace: false})
	if err != nil {
		t.Fatal(err)
	}
	agent := createTestAgent(t, service, spaceID, ownerID, "模型预算 Agent", team.ID)
	input := ModelUsageActivityInput{
		UsageID: "model-usage-1", CharacterID: agent.Profile.CharacterID, Model: "test", Provider: "openai",
		Status: "succeeded", InputTokens: 120, OutputTokens: 30, TotalTokens: 150,
		UsageReported: true, InputRate: 1000000, OutputRate: 2000000,
	}
	if err := service.RecordModelUsageActivity(ctx, spaceID, input); err != nil {
		t.Fatal(err)
	}
	if err := service.RecordModelUsageActivity(ctx, spaceID, input); err != nil {
		t.Fatal(err)
	}
	var events []ActivityEvent
	if err := service.db.WithContext(ctx).Where("kind = ? AND payload_json LIKE ?", "model.usage", "%model-usage-1%").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ActorID != ownerID || events[0].ActorType != "platform_user" {
		t.Fatalf("model usage activity should be idempotent and attributed, got %+v", events)
	}
	if err := service.db.Exec(`CREATE TABLE runtime_model_usage (
		sequence INTEGER PRIMARY KEY AUTOINCREMENT,
		id TEXT NOT NULL,
		space_id TEXT NOT NULL,
		role_id TEXT NOT NULL,
		request_id TEXT NOT NULL,
		status TEXT NOT NULL,
		input_tokens INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		total_tokens INTEGER NOT NULL DEFAULT 0,
		completed_at TEXT NOT NULL DEFAULT ''
	)`).Error; err != nil {
		t.Fatal(err)
	}
	for _, usage := range []struct {
		id, requestID string
		input, output int64
		cost          int64
	}{
		{id: "model-usage-a", requestID: "request-summary", input: 10, output: 5, cost: 2},
		{id: "model-usage-b", requestID: "request-summary", input: 12, output: 3, cost: 3},
	} {
		if err := service.db.Exec(
			"INSERT INTO runtime_model_usage (id, space_id, role_id, request_id, status, input_tokens, output_tokens, total_tokens, completed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
			usage.id, spaceID, agent.Profile.CharacterID, usage.requestID, "succeeded", usage.input, usage.output, usage.input+usage.output, time.Now().UTC().Format(time.RFC3339Nano),
		).Error; err != nil {
			t.Fatal(err)
		}
		if err := service.db.Create(&ActivityEvent{
			ID: stableID("activity", "model:"+usage.id), SpaceID: spaceID,
			AgentID: agent.Profile.ID, TeamID: team.ID, Kind: "model.usage",
			State: "succeeded", Title: "Agent 模型调用", CostMicroUSD: usage.cost,
			PayloadJSON: mustJSON(map[string]any{"usageId": usage.id}),
			OccurredAt:  time.Now().UTC(),
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	totals, err := service.ModelUsageTotalsByRequest(ctx, spaceID, "request-summary", agent.Profile.CharacterID)
	if err != nil {
		t.Fatal(err)
	}
	if totals.Calls != 2 || totals.InputTokens != 22 || totals.OutputTokens != 8 || totals.TotalTokens != 30 || totals.CostMicroUSD != 5 {
		t.Fatalf("unexpected model usage totals: %+v", totals)
	}
	windowTotals, err := service.ModelUsageTotalsByWindow(ctx, spaceID, agent.Profile.CharacterID, time.Now().UTC().Add(-time.Minute), time.Now().UTC().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if windowTotals.Calls != 2 || windowTotals.TotalTokens != 30 || windowTotals.CostMicroUSD != 5 {
		t.Fatalf("unexpected model usage window totals: %+v", windowTotals)
	}
}

func TestTeamInvitationLoginAndSharedContext(t *testing.T) {
	service, spaceID, ownerID := newTestPlatform(t)
	ctx := context.Background()
	if err := service.SeedAgentTemplates(ctx); err != nil {
		t.Fatal(err)
	}
	team, err := service.CreateTeam(ctx, spaceID, CreateTeamInput{Name: "研究团队", OwnerID: ownerID, SharedMemory: true, SharedWorkspace: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateTeamContext(ctx, spaceID, team.ID, CreateTeamContextInput{Kind: "instruction", Key: "tone", Content: "结论优先", Visibility: "team"}); err != nil {
		t.Fatal(err)
	}
	invite, err := service.InviteUser(ctx, spaceID, ownerID, InviteUserInput{Email: "member@example.com", Role: "member", TeamID: team.ID})
	if err != nil {
		t.Fatal(err)
	}
	user, err := service.AcceptInvitation(ctx, spaceID, invite.Token, "成员一", "password-123")
	if err != nil {
		t.Fatal(err)
	}
	login, err := service.LoginPlatformUser(ctx, spaceID, PlatformLoginInput{Email: "member@example.com", Password: "password-123"})
	if err != nil || login.Token == "" || login.User.ID != user.ID {
		t.Fatalf("login failed: %+v %v", login, err)
	}
	resolved, memberships, err := service.ResolvePlatformSession(ctx, login.Token)
	if err != nil || resolved.ID != user.ID || len(memberships) == 0 {
		t.Fatalf("resolve session failed: %+v %+v %v", resolved, memberships, err)
	}
	if err := service.Authorize(ctx, spaceID, user.ID, "platform.write"); err == nil {
		t.Fatal("read-only member unexpectedly received platform.write")
	}
	operatorInvite, err := service.InviteUser(ctx, spaceID, ownerID, InviteUserInput{Email: "operator@example.com", Role: "operator", TeamID: team.ID})
	if err != nil {
		t.Fatal(err)
	}
	operatorUser, err := service.AcceptInvitation(ctx, spaceID, operatorInvite.Token, "操作员", "password-456")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Authorize(ctx, spaceID, operatorUser.ID, "platform.write"); err != nil {
		t.Fatalf("operator permission missing: %v", err)
	}
	detail, err := service.GetTeam(ctx, spaceID, team.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundHuman := false
	for _, member := range detail.Members {
		if member.MemberType == "user" && member.MemberID == user.ID {
			foundHuman = true
		}
	}
	if !foundHuman || team.OwnerID != ownerID || team.WorkspaceID == "" {
		t.Fatalf("team ownership or human membership missing: %+v %+v", team, detail.Members)
	}
	templates, err := service.ListTeamTemplates(ctx)
	if err != nil || len(templates) == 0 {
		t.Fatalf("team templates missing: %+v %v", templates, err)
	}
}

func TestRuntimeSnapshotResetAndMarketSeed(t *testing.T) {
	service, spaceID, ownerID := newTestPlatform(t)
	ctx := context.Background()
	agent := createTestAgent(t, service, spaceID, ownerID, "电脑助手")
	runtime, err := service.Runtime().Create(ctx, spaceID, CreateRuntimeInput{AgentID: agent.Profile.ID, Provider: "local"})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err = service.Runtime().Start(ctx, spaceID, runtime.ID)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err = service.Runtime().Pause(ctx, spaceID, runtime.ID)
	if err != nil || runtime.State != "paused" {
		t.Fatalf("runtime pause failed: %+v %v", runtime, err)
	}
	runtime, err = service.Runtime().Start(ctx, spaceID, runtime.ID)
	if err != nil || runtime.State != "running" {
		t.Fatalf("runtime start from paused failed: %+v %v", runtime, err)
	}
	var pauseResumeEvents int64
	if err := service.db.WithContext(ctx).Model(&ActivityEvent{}).Where(
		"space_id = ? AND agent_id = ? AND kind = ? AND state IN ?",
		spaceID, agent.Profile.ID, "runtime.lifecycle", []string{"paused", "resumed"},
	).Count(&pauseResumeEvents).Error; err != nil {
		t.Fatal(err)
	}
	if pauseResumeEvents != 2 {
		t.Fatalf("expected pause and resume lifecycle events, got %d", pauseResumeEvents)
	}
	execResult, err := service.Runtime().Exec(ctx, spaceID, runtime.ID, RuntimeExecInput{Command: "printf runtime-exec-ok"})
	if err != nil || execResult.ExitCode != 0 || !strings.Contains(execResult.Output, "runtime-exec-ok") {
		t.Fatalf("runtime exec failed: %+v %v", execResult, err)
	}
	if err := os.WriteFile(filepath.Join(runtime.RootPath, "workspace", "state.txt"), []byte("snapshot-value"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.Runtime().Snapshot(ctx, spaceID, runtime.ID, "baseline")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtime.RootPath, "workspace", "state.txt"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	reset, err := service.Runtime().Reset(ctx, spaceID, runtime.ID, snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(reset.RootPath, "workspace", "state.txt"))
	if err != nil || string(body) != "snapshot-value" {
		t.Fatalf("snapshot reset mismatch: %q %v", body, err)
	}
	var restoreEvents int64
	if err := service.db.WithContext(ctx).Model(&ActivityEvent{}).Where(
		"space_id = ? AND agent_id = ? AND kind = ? AND state = ?",
		spaceID, agent.Profile.ID, "runtime.snapshot", "restored",
	).Count(&restoreEvents).Error; err != nil {
		t.Fatal(err)
	}
	if restoreEvents != 1 {
		t.Fatalf("expected one snapshot restore event, got %d", restoreEvents)
	}
}

func TestOfficialIdentityPluginPackagesVerify(t *testing.T) {
	for _, name := range []string{
		"email-1.0.0.amitiax",
		"calendar-1.0.0.amitiax",
		"phone-1.0.0.amitiax",
		"wallet-1.0.0.amitiax",
		"browser-profile-1.0.0.amitiax",
	} {
		path := filepath.Join("..", "..", "..", "plugin", "agent-identities", name)
		if err := packageverify.VerifyOfficialPublisherPackage(context.Background(), path); err != nil {
			t.Fatalf("official plugin %s failed verification: %v", name, err)
		}
	}
}

func TestRuntimeContainerWorkingDirectoryContainment(t *testing.T) {
	for _, testCase := range []struct {
		input string
		want  string
	}{
		{input: "", want: "/workspace"},
		{input: "src", want: "/workspace/src"},
		{input: "/src/app", want: "/workspace/src/app"},
		{input: "src/../tests", want: "/workspace/tests"},
	} {
		got, err := containerWorkingDir(testCase.input)
		if err != nil {
			t.Fatalf("containerWorkingDir(%q): %v", testCase.input, err)
		}
		if got != testCase.want {
			t.Fatalf("containerWorkingDir(%q) = %q, want %q", testCase.input, got, testCase.want)
		}
	}
	for _, value := range []string{"..", "../escape", "src/../../escape"} {
		if _, err := containerWorkingDir(value); err == nil {
			t.Fatalf("containerWorkingDir(%q) should reject traversal", value)
		}
	}
}

func TestIsolatedRuntimeProviderRequiresOCIBoundary(t *testing.T) {
	service, spaceID, ownerID := newTestPlatform(t)
	agent := createTestAgent(t, service, spaceID, ownerID, "隔离电脑")
	runtime, err := service.Runtime().Create(context.Background(), spaceID, CreateRuntimeInput{AgentID: agent.Profile.ID, Provider: "isolated"})
	if err != nil {
		t.Fatal(err)
	}
	if runtime.Provider != "oci" || runtime.Image != "ubuntu:24.04" {
		t.Fatalf("isolated runtime did not use OCI boundary: %+v", runtime)
	}
	if _, err := service.Runtime().Create(context.Background(), spaceID, CreateRuntimeInput{AgentID: "agent_missing", Provider: "local"}); err == nil {
		t.Fatal("runtime creation should reject an unknown agent")
	}
	remote, err := service.Runtime().RegisterRemote(context.Background(), spaceID, CreateRuntimeInput{AgentID: agent.Profile.ID}, "https://runtime.example.test")
	if err != nil || remote.Provider != "remote" || remote.State != "running" {
		t.Fatalf("remote runtime registration failed: %+v %v", remote, err)
	}
	if _, err := service.Runtime().RegisterRemote(context.Background(), spaceID, CreateRuntimeInput{AgentID: agent.Profile.ID}, "file:///tmp/runtime"); err == nil {
		t.Fatal("remote runtime registration should reject non-http endpoints")
	}
}

type testA2AExecutor struct {
	input any
}

func (e *testA2AExecutor) ExecuteA2ATask(_ context.Context, _, _ string, input any) (any, error) {
	e.input = input
	return map[string]any{"reply": "received", "input": input}, nil
}

func TestA2AInboundOutboundAndEvaluation(t *testing.T) {
	if err := validateA2AResolvedEndpoint(context.Background(), "http://127.0.0.1:18899/api/public/a2a/tasks"); err != nil {
		t.Fatalf("loopback A2A endpoint should be allowed for local integration: %v", err)
	}
	service, spaceID, ownerID := newTestPlatform(t)
	ctx := context.Background()
	agent := createTestAgent(t, service, spaceID, ownerID, "互联助手")
	executor := &testA2AExecutor{}
	service.SetA2AExecutor(executor)
	card, err := service.RegisterA2ACard(ctx, spaceID, RegisterA2ACardInput{AgentID: agent.Profile.ID, Name: "互联助手", URL: "https://example.test/a2a", Skills: []string{"research"}, Security: map[string]any{"approval": true}})
	if err != nil {
		t.Fatal(err)
	}
	if card.ID == "" {
		t.Fatal("missing A2A card")
	}
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	nonce := "test-nonce-1"
	payload := map[string]any{
		"agentId": agent.Profile.ID, "senderAgentId": agent.Profile.ID,
		"input": map[string]any{"message": "hello"}, "requestId": "request-1",
		"timestamp": timestamp, "nonce": nonce,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	privateKey, err := service.a2aSigningKey()
	if err != nil {
		t.Fatal(err)
	}
	signature := base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, body))
	inbound := InboundA2ATaskInput{
		AgentID: agent.Profile.ID, Input: payload["input"], RequestID: "request-1",
		SenderAgentID: agent.Profile.ID, Timestamp: timestamp, Nonce: nonce,
		Signature: signature, RawBody: body,
	}
	result, err := service.HandleInboundA2ATask(ctx, spaceID, inbound)
	if err != nil || result.Task.Status != "completed" || executor.input == nil {
		t.Fatalf("inbound A2A failed: %+v %v", result, err)
	}
	if _, err := service.HandleInboundA2ATask(ctx, spaceID, inbound); err == nil {
		t.Fatal("expected replayed A2A request to be rejected")
	}
	suite, err := service.CreateEvaluationSuite(ctx, spaceID, CreateEvaluationSuiteInput{AgentID: agent.Profile.ID, Name: "门禁", GatePolicy: map[string]any{"taskSuccessRate": 0.8, "toolAccuracy": 0.8, "falseSuccessRate": 0.1, "recoveryRate": 0.8}})
	if err != nil {
		t.Fatal(err)
	}
	caseItem, err := service.CreateEvaluationCase(ctx, spaceID, suite.ID, CreateEvaluationCaseInput{Name: "安全工具", Category: "safety", Input: map[string]any{"prompt": "x"}, Expected: map[string]any{"pass": true}, SafetyCritical: true, Weight: 2})
	if err != nil {
		t.Fatal(err)
	}
	run, err := service.RunEvaluation(ctx, spaceID, agent.Profile.ID, suite.ID, RunEvaluationInput{RevisionID: agent.Revision.ID, Results: []SubmitEvaluationResult{{CaseID: caseItem.ID, Passed: true, Score: 1, Tokens: 10, CostMicroUSD: 5, LatencyMS: 50, ToolTrace: []string{"search"}}}})
	if err != nil || run.GateStatus != "passed" {
		t.Fatalf("evaluation failed: %+v %v", run, err)
	}
	raw, err := json.Marshal(result.Task)
	if err != nil || len(raw) == 0 {
		t.Fatalf("serialize A2A task: %v", err)
	}
}

func TestHostControlCapabilityMapping(t *testing.T) {
	cases := map[string]string{
		"screen.capture":  "screen.capture",
		"mouse.click":     "input.mouse",
		"keyboard.type":   "input.keyboard",
		"window.activate": "window.control",
		"unknown":         "",
	}
	for action, expected := range cases {
		if actual := hostCapabilityForAction(action); actual != expected {
			t.Fatalf("%s: expected %q got %q", action, expected, actual)
		}
	}
}

func TestHostControlRejectsUnsupportedPermissions(t *testing.T) {
	service, spaceID, ownerID := newTestPlatform(t)
	agent := createTestAgent(t, service, spaceID, ownerID, "桌面助手")
	if _, err := service.Runtime().CreateHostControl(context.Background(), spaceID, CreateHostControlInput{
		AgentID: agent.Profile.ID, Capabilities: []string{"screen.capture"},
	}); err != nil {
		t.Fatalf("valid host control permission rejected: %v", err)
	}
	if _, err := service.Runtime().CreateHostControl(context.Background(), spaceID, CreateHostControlInput{
		AgentID: agent.Profile.ID, Capabilities: []string{"shell.unrestricted"},
	}); err == nil {
		t.Fatal("unsupported host control permission should be rejected")
	}
	if _, err := service.Runtime().CreateHostControl(context.Background(), spaceID, CreateHostControlInput{
		AgentID: "agent_missing", Capabilities: []string{"screen.capture"},
	}); err == nil {
		t.Fatal("host control should reject an unknown agent")
	}
}

func TestEmptyAgentPlatformCollectionsSerializeAsArrays(t *testing.T) {
	service, spaceID, _ := newTestPlatform(t)
	ctx := context.Background()
	events, _, err := service.ListActivity(ctx, spaceID, ActivityFilter{})
	if err != nil {
		t.Fatal(err)
	}
	agents, err := service.ListAgents(ctx, spaceID, true, 20)
	if err != nil {
		t.Fatal(err)
	}
	teams, err := service.ListTeams(ctx, spaceID)
	if err != nil {
		t.Fatal(err)
	}
	runtimes, err := service.Runtime().List(ctx, spaceID, "")
	if err != nil {
		t.Fatal(err)
	}
	collections := map[string]any{
		"activity": events, "agents": agents, "teams": teams, "runtimes": runtimes,
	}
	for name, value := range collections {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if string(encoded) != "[]" {
			t.Fatalf("%s: expected empty JSON array, got %s", name, encoded)
		}
	}
}
