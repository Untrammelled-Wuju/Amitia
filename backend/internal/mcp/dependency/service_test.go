package dependency

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/mcp"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func dependencyTestService(t *testing.T) (*Service, *mcp.Repository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	models := []any{&mcp.Server{}, &mcp.ServerScopeBinding{}, &mcp.ServerCredential{}, &mcp.ServerCapability{}, &mcp.ToolDefinition{}, &mcp.ResourceDefinition{}, &mcp.ResourceTemplate{}, &mcp.PromptDefinition{}, &mcp.DependencyLink{}, &mcp.Operation{}, &mcp.OAuthSession{}, &mcp.Task{}}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatal(err)
	}
	repository := mcp.NewRepository(db)
	return New(repository, nil, nil, nil, nil), repository
}

func httpDependency(id string, required bool) extension.AgentSkillMCPDependency {
	return extension.AgentSkillMCPDependency{ID: id, Required: required, Transport: "streamable_http", URL: "https://example.com/" + id, AuthType: "none", DefaultScope: "global", AutoConfigure: true}
}

func TestPreviewPlansHTTPReuseOAuthAndMissingStdio(t *testing.T) {
	service, repository := dependencyTestService(t)
	existing := httpDependency("existing", true)
	server, err := repository.CreateServer(context.Background(), serverInput(existing))
	if err != nil {
		t.Fatal(err)
	}
	oauth := httpDependency("oauth", true)
	oauth.AuthType = "oauth"
	stdio := extension.AgentSkillMCPDependency{ID: "stdio", Required: true, Transport: "stdio", Command: "amitia-command-that-does-not-exist-42", DefaultScope: "global"}
	plan, err := service.Preview(context.Background(), PreviewRequest{AgentSkillExtensionID: "skill", Dependencies: []extension.AgentSkillMCPDependency{existing, oauth, stdio}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 3 || !plan.RequiredMissing || plan.RiskLevel != "high" {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	if !plan.Items[0].Installed || plan.Items[0].ServerID != server.ID {
		t.Fatalf("existing server not reused: %#v", plan.Items[0])
	}
	if !plan.Items[1].AuthorizationRequired || !plan.Items[1].CanAutoConfigure {
		t.Fatalf("oauth plan invalid: %#v", plan.Items[1])
	}
	if plan.Items[2].CommandAvailable || !plan.Items[2].StartsLocalProcess {
		t.Fatalf("stdio plan invalid: %#v", plan.Items[2])
	}
}

func TestInstallCreatesLinkAndUninstallPreservesServer(t *testing.T) {
	service, repository := dependencyTestService(t)
	dependency := httpDependency("remote", true)
	plan, err := service.Preview(context.Background(), PreviewRequest{AgentSkillExtensionID: "skill", Dependencies: []extension.AgentSkillMCPDependency{dependency}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Install(context.Background(), InstallRequest{Plan: plan, ConfirmHTTP: true, EnableServers: false})
	if err != nil || result.Status != "completed" || len(result.Links) != 1 {
		t.Fatalf("unexpected install=%#v err=%v", result, err)
	}
	serverID := result.Links[0].ServerID
	ids, err := service.Uninstall(context.Background(), "skill")
	if err != nil || len(ids) != 1 || ids[0] != serverID {
		t.Fatalf("unexpected uninstall ids=%#v err=%v", ids, err)
	}
	if _, err := repository.GetServer(context.Background(), serverID); err != nil {
		t.Fatal("shared server was removed")
	}
}

func TestInstallRollsBackCreatedServersAndLinks(t *testing.T) {
	service, repository := dependencyTestService(t)
	first := httpDependency("first", true)
	second := httpDependency("second", true)
	second.DefaultScope = "character"
	plan, err := service.Preview(context.Background(), PreviewRequest{AgentSkillExtensionID: "skill", Dependencies: []extension.AgentSkillMCPDependency{first, second}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Install(context.Background(), InstallRequest{Plan: plan, ConfirmHTTP: true}); err == nil {
		t.Fatal("expected character scope failure")
	}
	servers, err := repository.ListServers(context.Background())
	if err != nil || len(servers) != 0 {
		t.Fatalf("rollback left servers=%#v err=%v", servers, err)
	}
	links, err := repository.ListDependencyLinks(context.Background(), "skill")
	if err != nil || len(links) != 0 {
		t.Fatalf("rollback left links=%#v err=%v", links, err)
	}
}

func TestOAuthInstallResumesOperationAfterAuthorization(t *testing.T) {
	service, repository := dependencyTestService(t)
	dependency := httpDependency("oauth", true)
	dependency.AuthType = "oauth"
	plan, err := service.Preview(context.Background(), PreviewRequest{AgentSkillExtensionID: "skill", Dependencies: []extension.AgentSkillMCPDependency{dependency}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Install(context.Background(), InstallRequest{Plan: plan, ConfirmHTTP: true})
	if err != nil || result.Status != "awaiting_authorization" {
		t.Fatalf("unexpected result=%#v err=%v", result, err)
	}
	if err := service.AuthorizationCompleted(context.Background(), result.AuthorizationServerIDs[0]); err != nil {
		t.Fatal(err)
	}
	links, err := repository.ListDependencyLinks(context.Background(), "skill")
	if err != nil || len(links) != 1 || links[0].InstallStatus != "installed" {
		t.Fatalf("unexpected links=%#v err=%v", links, err)
	}
	operations, err := repository.ListAgentSkillOperations(context.Background(), "skill", "completed")
	if err != nil || len(operations) != 1 {
		t.Fatalf("operation did not resume: %#v err=%v", operations, err)
	}
}

func TestInstallRevalidatesForgedStdioPlan(t *testing.T) {
	service, repository := dependencyTestService(t)
	dependency := extension.AgentSkillMCPDependency{ID: "stdio", Required: true, Transport: "stdio", Command: "amitia-command-that-does-not-exist-99", DefaultScope: "global"}
	forged := Plan{AgentSkillExtensionID: "skill", Items: []PlanItem{{Dependency: dependency, CommandAvailable: true, CanAutoConfigure: true, StartsLocalProcess: true}}}
	if _, err := service.Install(context.Background(), InstallRequest{Plan: forged, ConfirmStdio: true}); err == nil {
		t.Fatal("expected unavailable command rejection")
	}
	servers, err := repository.ListServers(context.Background())
	if err != nil || len(servers) != 0 {
		t.Fatalf("forged plan created servers=%#v err=%v", servers, err)
	}
}
