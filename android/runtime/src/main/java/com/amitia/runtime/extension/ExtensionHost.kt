package com.amitia.runtime.extension

import java.io.File
import java.io.InputStream
import kotlinx.coroutines.flow.Flow

enum class ExtensionState {
    Unloaded,
    Loading,
    Loaded,
    Activated,
    Deactivated,
    Error
}

data class LoadedExtension(
    val extensionId: String,
    val version: String,
    val displayName: String,
    val manifest: ExtensionManifest,
    val packageHash: String,
    val state: ExtensionState,
    val contributions: List<ExtensionContribution>,
    val tools: List<ToolDefinition>,
    val loadedAt: Long,
    val error: String? = null
)

data class ExtensionHostEvent(
    val type: ExtensionHostEventType,
    val extensionId: String,
    val timestamp: Long,
    val message: String? = null
)

enum class ExtensionHostEventType {
    LoadStarted,
    LoadSucceeded,
    LoadFailed,
    Activated,
    Deactivated,
    Unloaded,
    ToolExecuted,
    ToolFailed,
    ContributionRegistered
}

interface ExtensionHost {

    val events: Flow<ExtensionHostEvent>

    val loadedExtensions: Map<String, LoadedExtension>

    suspend fun initialize()

    suspend fun loadPackage(file: File): Result<LoadedExtension>

    suspend fun loadPackage(stream: InputStream): Result<LoadedExtension>

    suspend fun activate(extensionId: String): Result<LoadedExtension>

    suspend fun deactivate(extensionId: String): Result<LoadedExtension>

    suspend fun unload(extensionId: String): Result<Unit>

    suspend fun executeTool(request: ToolInvocationRequest): Result<ToolInvocationResult>

    fun getExtension(extensionId: String): LoadedExtension?

    fun listExtensions(): List<LoadedExtension>

    fun listContributions(extensionId: String): List<ExtensionContribution>

    fun listTools(extensionId: String): List<ToolDefinition>

    fun findTool(toolId: String): ToolDefinition?

    fun findContribution(contributionId: String): ExtensionContribution?

    suspend fun reload(extensionId: String): Result<LoadedExtension>

    suspend fun restoreFromDatabase(): Result<List<LoadedExtension>>

    fun getPermissionChecker(): ExtensionPermissionChecker
}
