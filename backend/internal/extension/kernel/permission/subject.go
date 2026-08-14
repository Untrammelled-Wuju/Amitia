package permission

import (
	"strings"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type SubjectType string

const (
	SubjectSystem    SubjectType = "system"
	SubjectUser      SubjectType = "user"
	SubjectExtension SubjectType = "extension"
	SubjectModule    SubjectType = "module"
	SubjectTool      SubjectType = "tool"
	SubjectWorkflow  SubjectType = "workflow"
	SubjectMCPServer SubjectType = "mcp_server"
	SubjectProvider  SubjectType = "provider"
	SubjectRuntime   SubjectType = "runtime"
)

type PermissionSubject struct {
	Type        SubjectType `json:"type"`
	ID          string      `json:"id"`
	ExtensionID string      `json:"extensionId,omitempty"`
	ModuleID    string      `json:"moduleId,omitempty"`
	ToolID      string      `json:"toolId,omitempty"`
}

func SubjectForExtension(extID string) PermissionSubject {
	return PermissionSubject{Type: SubjectExtension, ID: extID, ExtensionID: extID}
}

func SubjectForTool(extID, toolID string) PermissionSubject {
	return PermissionSubject{Type: SubjectTool, ID: toolID, ExtensionID: extID, ToolID: toolID}
}

func SubjectForUser(userID string) PermissionSubject {
	return PermissionSubject{Type: SubjectUser, ID: userID}
}

func SubjectForMCPServer(serverID string) PermissionSubject {
	return PermissionSubject{Type: SubjectMCPServer, ID: serverID}
}

func SubjectForProvider(providerID string) PermissionSubject {
	return PermissionSubject{
		Type: SubjectProvider,
		ID:   strings.TrimSpace(providerID),
	}
}

func SubjectForRuntime(runtimeID runtimeidentity.RuntimeID) PermissionSubject {
	return PermissionSubject{
		Type: SubjectRuntime,
		ID:   runtimeID.String(),
	}
}
