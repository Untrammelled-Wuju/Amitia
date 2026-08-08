package domain

const (
	legacyRuntimeTypeTrustedService RuntimeType = "trusted_service"
	legacyRuntimeTypePluginService  RuntimeType = "plugin_service"
)

func NormalizeRuntimeType(runtimeType RuntimeType) RuntimeType {
	switch runtimeType {
	case RuntimeTypeService, legacyRuntimeTypeTrustedService, legacyRuntimeTypePluginService:
		return RuntimeTypeService
	default:
		return runtimeType
	}
}
