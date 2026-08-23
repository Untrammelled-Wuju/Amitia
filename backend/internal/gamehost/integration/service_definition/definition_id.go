package service_definition

import (
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

const (
	DefinitionIDSeparator = "/"
	ServiceRuntimeType    = "service"
	JavaScriptRuntimeType = "javascript"
	GoRuntimeType         = "go"
)

func BuildServiceDefinitionID(extensionID, moduleID string) string {
	return fmt.Sprintf("%s%s%s", extensionID, DefinitionIDSeparator, moduleID)
}

func ParseServiceDefinitionID(definitionID string) (extensionID, moduleID string, err error) {
	parts := strings.SplitN(definitionID, DefinitionIDSeparator, 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid service definition id format: %s", definitionID)
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid service definition id: empty component in %s", definitionID)
	}
	return parts[0], parts[1], nil
}

func DefinitionIDFromServiceRuntime(def *trusted_service.ServiceRuntimeDefinition) string {
	if def == nil {
		return ""
	}
	return BuildServiceDefinitionID(def.ExtensionID, def.ModuleID)
}

func IsValidServiceRuntimeType(runtimeType string) bool {
	switch runtimeType {
	case ServiceRuntimeType, JavaScriptRuntimeType, GoRuntimeType:
		return true
	default:
		return false
	}
}
