package runtime_supervisor

import (
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func BuildRuntimeDefinitionID(extensionID string, moduleID string, runtimeType domain.RuntimeType) DefinitionID {
	return DefinitionID(fmt.Sprintf("%s/%s/%s", extensionID, moduleID, runtimeType))
}
