package com.amitia.runtime.extension

import java.io.File
import java.io.InputStream
import java.util.concurrent.ConcurrentHashMap
import javax.inject.Singleton
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

interface ToolExecutor {
    suspend fun execute(request: ToolInvocationRequest, tool: ToolDefinition): Result<ToolInvocationResult>
}

class LocalToolExecutor : ToolExecutor {

    override suspend fun execute(
        request: ToolInvocationRequest,
        tool: ToolDefinition
    ): Result<ToolInvocationResult> = runCatching {
        val start = System.currentTimeMillis()
        val output = buildJsonObject {
            put("toolId", request.toolId)
            put("extensionId", request.extensionId)
            put("status", "executed_locally")
            put("message", "Tool executed via local skeleton executor")
            put("arguments", request.arguments.toString())
        }
        ToolInvocationResult(
            success = true,
            output = output,
            durationMs = System.currentTimeMillis() - start
        )
    }
}

@Singleton
class ExtensionHostImpl(
    private val packageLoader: AmitiaxPackageLoader,
    private val toolExecutor: ToolExecutor
) : ExtensionHost {

    private val extensions = ConcurrentHashMap<String, LoadedExtension>()
    private val contributionIndex = ConcurrentHashMap<String, ExtensionContribution>()
    private val toolIndex = ConcurrentHashMap<String, ToolDefinition>()
    private val extensionLocks = ConcurrentHashMap<String, Mutex>()

    private val _events = MutableSharedFlow<ExtensionHostEvent>(
        replay = 0,
        extraBufferCapacity = 64
    )
    override val events: Flow<ExtensionHostEvent> = _events.asSharedFlow()

    override val loadedExtensions: Map<String, LoadedExtension>
        get() = extensions.toMap()

    override suspend fun loadPackage(file: File): Result<LoadedExtension> {
        return packageLoader.loadFromFile(file).mapCatching { pkg ->
            buildAndStoreExtension(pkg)
        }.onFailure { err ->
            val extId = extractExtensionIdFromFileName(file.name)
            _events.tryEmit(
                ExtensionHostEvent(
                    type = ExtensionHostEventType.LoadFailed,
                    extensionId = extId,
                    timestamp = System.currentTimeMillis(),
                    message = err.message
                )
            )
        }
    }

    override suspend fun loadPackage(stream: InputStream): Result<LoadedExtension> {
        return runCatching { packageLoader.loadFromStream(stream) }.mapCatching { pkg ->
            buildAndStoreExtension(pkg)
        }.onFailure { err ->
            _events.tryEmit(
                ExtensionHostEvent(
                    type = ExtensionHostEventType.LoadFailed,
                    extensionId = "",
                    timestamp = System.currentTimeMillis(),
                    message = err.message
                )
            )
        }
    }

    private fun buildAndStoreExtension(pkg: AmitiaxPackage): LoadedExtension {
        val manifest = pkg.manifest
        val extensionId = manifest.extension.id
        val lock = extensionLocks.computeIfAbsent(extensionId) { Mutex() }

        val contributions = manifest.modules.flatMap { module ->
            module.contributions.map { meta ->
                ExtensionContribution(
                    id = meta.id,
                    type = ContributionType.fromValue(meta.kind) ?: ContributionType.Tool,
                    extensionId = extensionId,
                    moduleId = module.id,
                    enabled = meta.exposure?.visibleByDefault ?: true,
                    metadata = meta.spec,
                    toolId = if (meta.kind == "tool") meta.id else null,
                    surfaceId = if (meta.kind == "ui") meta.id else null,
                    event = if (meta.kind == "hook") meta.spec["event"]?.let {
                        (it as? JsonPrimitive)?.content
                    } else null,
                    handler = if (meta.kind == "hook") meta.spec["handler"]?.let {
                        (it as? JsonPrimitive)?.content
                    } else null,
                    eventType = if (meta.kind == "event_subscription") meta.spec["eventType"]?.let {
                        (it as? JsonPrimitive)?.content
                    } else null
                )
            }
        }

        val tools = contributions.filter { it.type == ContributionType.Tool }.map { contrib ->
            val module = manifest.modules.first { it.id == contrib.moduleId }
            ToolDefinition(
                toolId = contrib.toolId ?: contrib.id,
                extensionId = extensionId,
                moduleId = contrib.moduleId,
                name = module.name.default,
                description = module.description?.default ?: "",
                entryPoint = module.runtime?.entryPoint ?: "",
                timeout = parseTimeout(module.runtime?.timeout),
                permissions = module.runtime?.permissions ?: emptyList()
            )
        }

        val loaded = LoadedExtension(
            extensionId = extensionId,
            version = manifest.extension.version,
            displayName = manifest.extension.name.default,
            manifest = manifest,
            packageHash = pkg.packageHash,
            state = ExtensionState.Loaded,
            contributions = contributions,
            tools = tools,
            loadedAt = System.currentTimeMillis()
        )

        extensions[extensionId] = loaded

        contributions.forEach { contrib ->
            contributionIndex[contrib.id] = contrib
            _events.tryEmit(
                ExtensionHostEvent(
                    type = ExtensionHostEventType.ContributionRegistered,
                    extensionId = extensionId,
                    timestamp = System.currentTimeMillis(),
                    message = "Contribution ${contrib.id} (${contrib.type}) registered"
                )
            )
        }

        tools.forEach { tool ->
            toolIndex[tool.toolId] = tool
        }

        _events.tryEmit(
            ExtensionHostEvent(
                type = ExtensionHostEventType.LoadSucceeded,
                extensionId = extensionId,
                timestamp = System.currentTimeMillis(),
                message = "Extension $extensionId v${loaded.version} loaded with ${contributions.size} contributions and ${tools.size} tools"
            )
        )

        return loaded
    }

    override suspend fun activate(extensionId: String): Result<LoadedExtension> {
        val lock = extensionLocks.computeIfAbsent(extensionId) { Mutex() }
        return lock.withLock {
            val current = extensions[extensionId]
                ?: return@withLock Result.failure(IllegalStateException("Extension $extensionId not loaded"))

            val activated = current.copy(state = ExtensionState.Activated)
            extensions[extensionId] = activated

            _events.tryEmit(
                ExtensionHostEvent(
                    type = ExtensionHostEventType.Activated,
                    extensionId = extensionId,
                    timestamp = System.currentTimeMillis(),
                    message = "Extension $extensionId activated"
                )
            )

            Result.success(activated)
        }
    }

    override suspend fun deactivate(extensionId: String): Result<LoadedExtension> {
        val lock = extensionLocks.computeIfAbsent(extensionId) { Mutex() }
        return lock.withLock {
            val current = extensions[extensionId]
                ?: return@withLock Result.failure(IllegalStateException("Extension $extensionId not loaded"))

            val deactivated = current.copy(state = ExtensionState.Deactivated)
            extensions[extensionId] = deactivated

            _events.tryEmit(
                ExtensionHostEvent(
                    type = ExtensionHostEventType.Deactivated,
                    extensionId = extensionId,
                    timestamp = System.currentTimeMillis(),
                    message = "Extension $extensionId deactivated"
                )
            )

            Result.success(deactivated)
        }
    }

    override suspend fun unload(extensionId: String): Result<Unit> {
        val lock = extensionLocks.computeIfAbsent(extensionId) { Mutex() }
        return lock.withLock {
            val current = extensions.remove(extensionId)
            if (current != null) {
                current.contributions.forEach { contrib ->
                    contributionIndex.remove(contrib.id)
                }
                current.tools.forEach { tool ->
                    toolIndex.remove(tool.toolId)
                }
            }
            extensionLocks.remove(extensionId)

            _events.tryEmit(
                ExtensionHostEvent(
                    type = ExtensionHostEventType.Unloaded,
                    extensionId = extensionId,
                    timestamp = System.currentTimeMillis(),
                    message = "Extension $extensionId unloaded"
                )
            )

            Result.success(Unit)
        }
    }

    override suspend fun executeTool(request: ToolInvocationRequest): Result<ToolInvocationResult> {
        val tool = toolIndex[request.toolId]
            ?: return Result.failure(IllegalArgumentException("Tool ${request.toolId} not found"))

        val ext = extensions[request.extensionId]
            ?: return Result.failure(IllegalArgumentException("Extension ${request.extensionId} not loaded"))

        if (ext.state != ExtensionState.Activated && ext.state != ExtensionState.Loaded) {
            return Result.failure(IllegalStateException("Extension ${request.extensionId} is not in an executable state: ${ext.state}"))
        }

        val result = toolExecutor.execute(request, tool)

        result.onSuccess {
            _events.tryEmit(
                ExtensionHostEvent(
                    type = ExtensionHostEventType.ToolExecuted,
                    extensionId = request.extensionId,
                    timestamp = System.currentTimeMillis(),
                    message = "Tool ${request.toolId} executed in ${it.durationMs}ms"
                )
            )
        }.onFailure { err ->
            _events.tryEmit(
                ExtensionHostEvent(
                    type = ExtensionHostEventType.ToolFailed,
                    extensionId = request.extensionId,
                    timestamp = System.currentTimeMillis(),
                    message = "Tool ${request.toolId} failed: ${err.message}"
                )
            )
        }

        return result
    }

    override fun getExtension(extensionId: String): LoadedExtension? = extensions[extensionId]

    override fun listExtensions(): List<LoadedExtension> = extensions.values.toList()

    override fun listContributions(extensionId: String): List<ExtensionContribution> =
        extensions[extensionId]?.contributions ?: emptyList()

    override fun listTools(extensionId: String): List<ToolDefinition> =
        extensions[extensionId]?.tools ?: emptyList()

    override fun findTool(toolId: String): ToolDefinition? = toolIndex[toolId]

    override fun findContribution(contributionId: String): ExtensionContribution? =
        contributionIndex[contributionId]

    override suspend fun reload(extensionId: String): Result<LoadedExtension> {
        val lock = extensionLocks.computeIfAbsent(extensionId) { Mutex() }
        return lock.withLock {
            val current = extensions[extensionId]
                ?: return@withLock Result.failure(IllegalStateException("Extension $extensionId not loaded for reload"))

            current.contributions.forEach { contrib ->
                contributionIndex.remove(contrib.id)
            }
            current.tools.forEach { tool ->
                toolIndex.remove(tool.toolId)
            }

            val reloaded = current.copy(
                state = ExtensionState.Loaded,
                loadedAt = System.currentTimeMillis(),
                error = null
            )
            extensions[extensionId] = reloaded

            reloaded.contributions.forEach { contrib ->
                contributionIndex[contrib.id] = contrib
            }
            reloaded.tools.forEach { tool ->
                toolIndex[tool.toolId] = tool
            }

            _events.tryEmit(
                ExtensionHostEvent(
                    type = ExtensionHostEventType.LoadSucceeded,
                    extensionId = extensionId,
                    timestamp = System.currentTimeMillis(),
                    message = "Extension $extensionId reloaded"
                )
            )

            Result.success(reloaded)
        }
    }

    private fun parseTimeout(timeout: String?): Long {
        if (timeout.isNullOrBlank()) return 30000L
        return try {
            if (timeout.endsWith("s")) {
                (timeout.dropLast(1).toLong()) * 1000
            } else if (timeout.endsWith("ms")) {
                timeout.dropLast(2).toLong()
            } else {
                timeout.toLong()
            }
        } catch (e: NumberFormatException) {
            30000L
        }
    }

    private fun extractExtensionIdFromFileName(fileName: String): String {
        return fileName.removeSuffix(".amitiax")
            .substringBeforeLast('-')
            .ifBlank { fileName }
    }
}
