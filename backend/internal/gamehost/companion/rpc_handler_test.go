package artifact

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	kernelpermission "github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/integration"
	ghpermission "github.com/u-ai/backend/internal/gamehost/permission"
	"github.com/u-ai/backend/internal/gamehost/rpc"
)

type artifactRPCPluginResolver struct {
	descriptor domain.PluginDescriptor
}

func (r artifactRPCPluginResolver) Get(ctx context.Context, pluginID domain.PluginID) (domain.PluginDescriptor, error) {
	return r.descriptor, nil
}

type artifactRPCAllowPermission struct{}

func (artifactRPCAllowPermission) CheckServicePermissionTarget(
	ctx context.Context,
	runtimeID, pluginID, serviceID, permID string,
	target kernelpermission.PermissionTarget,
) ghpermission.DecisionResult {
	return ghpermission.DecisionResult{Decision: ghpermission.DecisionAllowed}
}

func TestArtifactRPCMutationCannotCreateManagementTargetGrant(t *testing.T) {
	const extensionID = "example/artifact-rpc"
	generations := &artifactTestGenerationResolver{generation: integration.InstalledGeneration{
		Path: t.TempDir(), GenerationID: "generation-1", Version: "1.0.0",
	}}
	manager, err := NewArtifactManager(t.TempDir(), artifactTestSource{}, generations)
	if err != nil {
		t.Fatalf("NewArtifactManager() error = %v", err)
	}
	handler, err := NewArtifactRPCHandler(manager, artifactRPCPluginResolver{descriptor: domain.PluginDescriptor{
		ID: "plugin-1", ExtensionID: extensionID, Name: "Plugin", Version: "1.0.0", ProtocolVersion: "amitia-game-host/1",
	}}, artifactRPCAllowPermission{})
	if err != nil {
		t.Fatalf("NewArtifactRPCHandler() error = %v", err)
	}
	targetRoot := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"artifactId": "artifact-1",
		"targetRoot": targetRoot,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = handler.Handle(context.Background(), rpc.RPCRequest{
		ID:        "request-1",
		PluginID:  "plugin-1",
		RuntimeID: "runtime-1",
		ServiceID: "service-1",
		Method:    MethodArtifactDeploy,
		Payload:   payload,
	})
	if err == nil || !strings.Contains(err.Error(), "requires management authorization") {
		t.Fatalf("artifact mutation without management grant error = %v", err)
	}
	grants, listErr := manager.ListAuthorizedTargetRoots(context.Background(), extensionID)
	if listErr != nil {
		t.Fatalf("ListAuthorizedTargetRoots() error = %v", listErr)
	}
	if len(grants) != 0 {
		t.Fatalf("plugin RPC created persistent target-root grants: %+v", grants)
	}
}
