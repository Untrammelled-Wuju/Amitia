package acquisition

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	kernelmcp "github.com/u-ai/backend/internal/extension/kernel/mcp"
	"github.com/u-ai/backend/internal/extension/kernel/mcp/installer"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/scriptruntime/commandenv"
)

type discoveryFailureSource struct{}

func (discoveryFailureSource) ID() string          { return "broken_catalog" }
func (discoveryFailureSource) Kind() CandidateKind { return CandidateAgentSkill }
func (discoveryFailureSource) Search(context.Context, AcquisitionRequest) ([]CapabilityCandidate, error) {
	return nil, errors.New("HTTP 404")
}

func TestAcquisitionDiscoveryReportsSourceFailures(t *testing.T) {
	registry := NewSourceRegistry()
	registry.Register(discoveryFailureSource{})
	service := &AcquisitionService{registry: registry}
	output, err := NewAgentCapabilityBridge(service).FindCapabilities(context.Background(), FindCapabilitiesInput{CapabilityID: "skill.review", PreferredKind: "skill"}, "space")
	if err != nil || len(output.Errors) != 1 || output.Errors[0].SourceID != "broken_catalog" {
		t.Fatalf("output=%+v err=%v", output, err)
	}
	if _, err := NewSourceSearchService(registry).Search(context.Background(), AcquisitionRequest{CapabilityID: "skill.review"}); err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("error=%v", err)
	}
}

func TestAcquisitionDiscoveryPublicMCPFormats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("search") != "filesystem" || r.URL.Query().Get("version") != "latest" {
			t.Errorf("query=%s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"servers":[{"server":{"name":"io.example/filesystem","version":"1.2.3","packages":[{"registryType":"npm","identifier":"@example/filesystem","version":"1.2.3","environmentVariables":[{"name":"TOKEN","isRequired":true}]}],"remotes":[{"type":"streamable-http","url":"https://example.com/mcp"}]},"_meta":{"io.modelcontextprotocol.registry/official":{"status":"active","isLatest":true}}}]}`))
	}))
	defer server.Close()
	result, err := NewPublicMCPSource(server.URL).Search(context.Background(), AcquisitionRequest{CapabilityID: "mcp.server.filesystem"})
	if err != nil || len(result) != 2 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if result[0].Install.MCP.Transport != "streamable_http" || result[0].Install.MCP.Command != "https://example.com/mcp" {
		t.Fatal(result[0])
	}
	if strings.Join(result[1].Install.MCP.Args, " ") != "-y @example/filesystem@1.2.3" || len(result[1].Install.MCP.RequiredInputs) != 1 {
		t.Fatal(result[1])
	}
	if _, err := NewMCPInstaller(NewMCPPortBridge(nil)).Install(context.Background(), result[1], DeploymentTarget{}); err == nil || !strings.Contains(err.Error(), "TOKEN") {
		t.Fatalf("missing MCP inputs should not install: %v", err)
	}
}

func TestAcquisitionDiscoveryPinsSkillCommitAndCarriesSource(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/search/"):
			w.Write([]byte(`{"items":[]}`))
		case strings.Contains(r.URL.Path, "/commits/"):
			w.Write([]byte(`{"sha":"commit-sha"}`))
		case strings.Contains(r.URL.Path, "/git/trees/commit-sha"):
			w.Write([]byte(`{"sha":"tree-sha","tree":[{"path":"skills/.curated/review/SKILL.md","type":"blob"}]}`))
		default:
			t.Errorf("unexpected URL %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer api.Close()
	source := NewPublicSkillSource()
	source.endpoint = api.URL
	candidates, err := source.Search(context.Background(), AcquisitionRequest{CapabilityID: "skill.review"})
	if err != nil || len(candidates) != 1 {
		t.Fatalf("candidates=%+v err=%v", candidates, err)
	}
	if !strings.Contains(candidates[0].Source.URI, "/tree/commit-sha/") || strings.Contains(candidates[0].Source.URI, "tree-sha") {
		t.Fatal(candidates[0].Source)
	}
	request := AcquisitionRequest{CapabilityID: "skill.review", RequestedCandidateID: candidates[0].ID, Install: &candidates[0].Install}
	explicit, err := NewExplicitSource().Search(context.Background(), request)
	if err != nil || len(explicit) != 1 || explicit[0].ID != candidates[0].ID {
		t.Fatalf("explicit=%+v err=%v", explicit, err)
	}
}

func TestAcquisitionDiscoveryLocalSkillIncludesResources(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "review")
	os.MkdirAll(filepath.Join(directory, "references"), 0700)
	raw := []byte("---\nname: review\ndescription: Review source code.\n---\nRead references/checklist.md.")
	os.WriteFile(filepath.Join(directory, "SKILL.md"), raw, 0600)
	os.WriteFile(filepath.Join(directory, "references", "checklist.md"), []byte("Check correctness."), 0600)
	source := NewLocalSkillSource(filepath.Dir(directory))
	candidates, err := source.Search(context.Background(), AcquisitionRequest{CapabilityID: "skill.review"})
	if err != nil || len(candidates) != 1 {
		t.Fatalf("candidates=%+v err=%v", candidates, err)
	}
	bundle, err := LoadSkillBundle(context.Background(), candidates[0].Source.URI, "review", "")
	if err != nil || string(bundle.Files["references/checklist.md"]) != "Check correctness." {
		t.Fatalf("bundle=%+v err=%v", bundle, err)
	}
	if _, err := LoadSkillBundle(context.Background(), directory, "review", strings.Repeat("0", 64)); err == nil {
		t.Fatal("hash mismatch accepted")
	}
}

type discoveryMCPRuntime struct {
	requests     []MCPRuntimeConnectRequest
	failure      error
	disconnected bool
}

func (r *discoveryMCPRuntime) ConnectAndDiscover(_ context.Context, request MCPRuntimeConnectRequest) ([]capability.MCPToolDescriptor, error) {
	r.requests = append(r.requests, request)
	return []capability.MCPToolDescriptor{{Name: "inspect", InputSchema: json.RawMessage(`{"type":"object"}`)}}, r.failure
}
func (r *discoveryMCPRuntime) Disconnect(context.Context, string) error {
	r.disconnected = true
	return nil
}

type discoveryMCPSync struct{ count int }

func (s *discoveryMCPSync) SyncMCPTools(_ context.Context, _ string, tools []capability.MCPToolDescriptor) (*MCPToolSyncResult, error) {
	s.count = len(tools)
	return &MCPToolSyncResult{Total: len(tools)}, nil
}
func (*discoveryMCPSync) ListMCPTools(context.Context, string) ([]capability.MCPToolDescriptor, error) {
	return nil, nil
}

type discoveryMCPPersistence struct {
	saved   bool
	ready   bool
	removed bool
}

func (p *discoveryMCPPersistence) SaveMCPConfiguration(context.Context, string, string, string, []string, map[string]string) (string, error) {
	p.saved = true
	return "persistent-server-id", nil
}
func (p *discoveryMCPPersistence) MarkMCPReady(context.Context, string) error {
	p.ready = true
	return nil
}
func (p *discoveryMCPPersistence) RemoveMCPConfiguration(context.Context, string) error {
	p.removed = true
	return nil
}

func TestAcquisitionDiscoveryMCPPersistConnectVerifyAndCleanup(t *testing.T) {
	for _, fails := range []bool{false, true} {
		lifecycle := kernelmcp.NewMCPLifecycle(installer.NewDefaultProvisioner(), installer.NewDefaultInstaller())
		runtime := &discoveryMCPRuntime{}
		if fails {
			runtime.failure = errors.New("network unavailable")
		}
		syncer := &discoveryMCPSync{}
		persistence := &discoveryMCPPersistence{}
		port := NewPersistentMCPPortBridge(lifecycle, runtime, syncer, persistence)
		install := NewMCPInstaller(port)
		candidate := CapabilityCandidate{ID: "candidate", Kind: CandidateMCP, Install: CandidateInstallDescriptor{Method: InstallMCP, MCP: &MCPInstallDescriptor{ServerName: "named", Transport: "streamable_http", Command: "https://example.com/mcp"}}}
		installed, err := install.Install(context.Background(), candidate, DeploymentTarget{SpaceID: runtimeidentity.SpaceID("space")})
		if fails {
			if err == nil || !persistence.removed || !runtime.disconnected || syncer.count != 0 {
				t.Fatalf("failed cleanup: %+v %+v err=%v", persistence, runtime, err)
			}
			continue
		}
		if err != nil || !persistence.saved || !persistence.ready || installed.TransactionID != "persistent-server-id" || syncer.count != 1 {
			t.Fatalf("installed=%+v persistence=%+v err=%v", installed, persistence, err)
		}
		ready, err := install.VerifyInstalledCapability(context.Background(), installed)
		if err != nil || !ready {
			t.Fatalf("ready=%v err=%v", ready, err)
		}
		if err := install.Rollback(context.Background(), installed); err != nil || !persistence.removed {
			t.Fatalf("rollback=%v", err)
		}
	}
}

func TestAcquisitionDiscoveryToolSchemas(t *testing.T) {
	for _, raw := range []json.RawMessage{inputSchemaFindCapabilities(), inputSchemaAcquireCapability()} {
		var schema map[string]any
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		properties := schema["properties"].(map[string]any)
		for _, name := range []string{"sourceUri", "install", "query"} {
			if _, ok := properties[name]; !ok {
				t.Fatal(name)
			}
		}
	}
	if normalizeCandidateKind("skill") != CandidateAgentSkill || normalizeCandidateKind("plugin") != CandidateExtensionPackage {
		t.Fatal("kind aliases not normalized")
	}
}

type discoveryCommandResolver struct{ commands []string }

func (r *discoveryCommandResolver) Resolve(_ context.Context, request commandenv.Request) (commandenv.Invocation, error) {
	r.commands = append(r.commands, request.Command)
	return commandenv.Invocation{Executable: "managed-runtime"}, nil
}

func TestAcquisitionDiscoveryPinnedMCPUsesManagedRuntime(t *testing.T) {
	for _, entry := range []struct{ command, spec string }{{"npx", "@example/server@1.2.3"}, {"uvx", "example-server==1.2.3"}} {
		resolver := &discoveryCommandResolver{}
		lifecycle := kernelmcp.NewMCPLifecycle(installer.NewDefaultProvisioner(), installer.NewDefaultInstaller(resolver))
		port := NewPersistentMCPPortBridge(lifecycle, &discoveryMCPRuntime{}, &discoveryMCPSync{}, &discoveryMCPPersistence{})
		id, err := port.InstallMCP(context.Background(), "test", "stdio", entry.command, []string{entry.spec}, nil)
		if err != nil || id == "" || len(resolver.commands) != 1 || resolver.commands[0] != entry.command {
			t.Fatalf("command=%s id=%s err=%v resolved=%v", entry.command, id, err, resolver.commands)
		}
	}
}

func TestAcquisitionDiscoveryResumeRejectsAnotherSpace(t *testing.T) {
	service := &AcquisitionService{resumeContexts: map[string]CapabilityResumeContext{
		"approval": {State: ResumePending, SpaceID: "owner", CapabilityID: "skill.review"},
	}}
	if _, err := service.ResumeAcquire(context.Background(), "approval", "another-space"); err == nil {
		t.Fatal("cross-space approval accepted")
	}
	if _, exists := service.resumeContexts["approval"]; !exists {
		t.Fatal("rejected request consumed approval")
	}
}

func TestAcquisitionDiscoveryApprovalMetadataExcludesSecrets(t *testing.T) {
	request := AcquisitionRequest{CapabilityID: "mcp.server.test", ExecContext: &execution.ExecutionContext{}, Install: &CandidateInstallDescriptor{Method: InstallMCP, MCP: &MCPInstallDescriptor{Env: map[string]string{"TOKEN": "isolated-secret"}}}}
	if _, safe := approvalRequestMetadata(request); safe {
		t.Fatal("credential-bearing request can be saved in plaintext approval metadata")
	}
	request.Install.MCP.Env = nil
	saved, safe := approvalRequestMetadata(request)
	if !safe || saved.ExecContext != nil || saved.CapabilityID != request.CapabilityID {
		t.Fatal("approval metadata did not retain source selection while excluding execution context")
	}
}
