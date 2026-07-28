package com.amitia.runtime.extension

import kotlinx.serialization.Serializable

@Serializable
data class ExtensionManifest(
    val manifestVersion: Int = 2,
    val extension: ExtensionMeta,
    val publisher: PublisherMeta? = null,
    val compatibility: Compatibility? = null,
    val modules: List<ModuleMeta> = emptyList(),
    val dependencies: List<Dependency> = emptyList(),
    val permissions: List<PermissionReq> = emptyList(),
    val resources: List<ResourceMeta> = emptyList(),
    val lifecycle: LifecycleMeta? = null,
    val integrity: IntegrityMeta? = null,
    val development: DevelopmentMeta? = null
)

@Serializable
data class ExtensionMeta(
    val id: String,
    val name: LocalizedText,
    val description: LocalizedText? = null,
    val version: String,
    val license: String? = null,
    val homepage: String? = null,
    val repository: String? = null,
    val categories: List<String> = emptyList(),
    val keywords: List<String> = emptyList(),
    val icon: String? = null,
    val metadata: Map<String, kotlinx.serialization.json.JsonElement> = emptyMap()
)

@Serializable
data class LocalizedText(
    val default: String,
    val translations: Map<String, String> = emptyMap()
)

@Serializable
data class PublisherMeta(
    val id: String,
    val displayName: String,
    val trustLevel: String? = null,
    val contact: String? = null,
    val website: String? = null
)

@Serializable
data class Compatibility(
    val minHostVersion: String? = null,
    val maxHostVersion: String? = null,
    val platforms: List<String> = emptyList(),
    val featureFlags: List<String> = emptyList()
)

@Serializable
data class ModuleMeta(
    val id: String,
    val name: LocalizedText,
    val description: LocalizedText? = null,
    val type: String,
    val version: String? = null,
    val runtime: RuntimeMeta? = null,
    val contributions: List<ContributionMeta> = emptyList(),
    val dependencies: List<Dependency> = emptyList(),
    val compatibility: ModuleCompatibility? = null,
    val policies: ModulePolicies? = null
)

@Serializable
data class ModuleCompatibility(
    val minHostVersion: String? = null,
    val platforms: List<String> = emptyList()
)

@Serializable
data class ModulePolicies(
    val isolation: String? = null,
    val networkAccess: Boolean = false,
    val fileSystemAccess: Boolean = false
)

@Serializable
data class RuntimeMeta(
    val type: String,
    val entryPoint: String? = null,
    val workerCount: Int = 1,
    val timeout: String? = null,
    val memory: Long = 0,
    val permissions: List<String> = emptyList(),
    val capabilities: Map<String, Boolean> = emptyMap(),
    val env: Map<String, String> = emptyMap()
)

@Serializable
data class ContributionMeta(
    val id: String,
    val kind: String,
    val name: LocalizedText,
    val description: LocalizedText? = null,
    val version: String? = null,
    val spec: Map<String, kotlinx.serialization.json.JsonElement> = emptyMap(),
    val requiredPermissions: List<String> = emptyList(),
    val requiredScope: List<String> = emptyList(),
    val exposure: ExposureMeta? = null,
    val runtimeBinding: RuntimeBindingMeta? = null,
    val dependencies: List<Dependency> = emptyList()
)

@Serializable
data class ExposureMeta(
    val visibleByDefault: Boolean = true,
    val hiddenFromDiscovery: Boolean = false,
    val requiredRoles: List<String> = emptyList()
)

@Serializable
data class RuntimeBindingMeta(
    val runtimeId: String,
    val generation: Long = 0
)

@Serializable
data class Dependency(
    val id: String,
    val version: String? = null,
    val optional: Boolean = false
)

@Serializable
data class PermissionReq(
    val id: String,
    val reason: String? = null,
    val scope: List<String> = emptyList()
)

@Serializable
data class ResourceMeta(
    val id: String,
    val type: String,
    val path: String,
    val description: String? = null
)

@Serializable
data class LifecycleMeta(
    val activateOn: String? = null,
    val deactivateOn: String? = null,
    val priority: Int = 0
)

@Serializable
data class IntegrityMeta(
    val algorithm: String = "sha256",
    val contentTree: String? = null,
    val filesManifest: String? = null
)

@Serializable
data class DevelopmentMeta(
    val devMode: Boolean = false,
    val hotReload: Boolean = false,
    val sourceMap: Boolean = true
)
