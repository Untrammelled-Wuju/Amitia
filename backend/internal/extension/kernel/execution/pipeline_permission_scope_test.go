package execution

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

func TestBuildPermissionRequirementsPreservesInvocationScope(t *testing.T) {
	tool := capability.ToolDefinition{
		ID:          "web_run",
		ExtensionID: "com.amitia.builtin.search",
		Permissions: []capability.PermissionRequirement{{Capability: "network.request"}},
	}
	invocation := capability.ToolInvocationContext{
		CharacterID:    "character-1",
		ConversationID: "conversation-1",
	}
	requirements := buildPermissionRequirements(tool, invocation)
	require.Len(t, requirements, 1)
	require.Equal(t, permission.ScopeCharacter, requirements[0].Scope.Type)
	require.Equal(t, "character-1", requirements[0].Scope.CharacterID)
}
