package com.amitia.runtime.extension

import com.amitia.core.database.dao.ExtensionInstallationDao
import com.amitia.core.database.entity.ExtensionInstallationEntity
import com.amitia.runtime.extension.security.ArchivePolicy
import com.amitia.runtime.extension.security.SizeLimitExceededException
import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.io.File
import java.io.InputStream
import java.util.concurrent.ConcurrentHashMap
import javax.inject.Singleton
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.put

interface ToolExecutor {
    suspend fun execute(request: ToolInvocationRequest, tool: ToolDefinition): Result<ToolInvocationResult>
}

@Singleton
class ExtensionHostImpl(
    private val packageLoader: AmitiaxPackageLoader,
    private val toolExecutor: ToolExecutor,
    private val apiClient: ExtensionApiClient,
    private val installationDao: ExtensionInstallationDao,
    private val permissionChecker: ExtensionPermissionChecker,
    private val json: Json,
    private val maxPackageBytes: Long = ArchivePolicy.default().maxArchiveBytes
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
        return runCatching {
            val fileSize = file.length()
            if (fileSize > maxPackageBytes) {
                throw SizeLimitExceededException(
                    "package file size $fileSize exceeds limit $maxPackageBytes"
                )
            }
            val packageBytes = file.readBytes()
            val pkg = packageLoader.loadFromStream(ByteArrayInputStream(packageBytes))
            apiClient.installExtension(packageBytes, file.name)
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
        return runCatching {
            val packageBytes = readLimitedStream(stream, maxPackageBytes)
            val pkg = packageLoader.loadFromStream(ByteArrayInputStream(packageBytes))
            apiClient.installExtension(packageBytes)
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

    private suspend fun buildAndStoreExtension(pkg: AmitiaxPackage): LoadedExtension {
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

        installationDao.insert(
            ExtensionInstallationEntity(
                extensionId = extensionId,
                version = loaded.version,
                manifestHash = loaded.packageHash,
                installedAt = loaded.loadedAt,
                status = ExtensionState.Loaded.name,
                contributionIds = contributions.map { it.id }
            )
        )

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

            runCatching { apiClient.enableExtension(extensionId) }
                .onFailure { return@withLock Result.failure(it) }

            val activated = current.copy(state = ExtensionState.Activated)
            extensions[extensionId] = activated

            installationDao.updateStatus(extensionId, ExtensionState.Activated.name)

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

            runCatching { apiClient.disableExtension(extensionId) }
                .onFailure { return@withLock Result.failure(it) }

            val deactivated = current.copy(state = ExtensionState.Deactivated)
            extensions[extensionId] = deactivated

            installationDao.updateStatus(extensionId, ExtensionState.Deactivated.name)

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
            runCatching { apiClient.uninstallExtension(extensionId) }

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

            installationDao.deleteByExtensionId(extensionId)

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

        val hasPermission = permissionChecker.checkPermission(
            request.extensionId,
            tool.permissions.firstOrNull() ?: "tool.execute"
        )
        if (!hasPermission) {
            _events.tryEmit(
                ExtensionHostEvent(
                    type = ExtensionHostEventType.ToolFailed,
                    extensionId = request.extensionId,
                    timestamp = System.currentTimeMillis(),
                    message = "Tool ${request.toolId} denied: permission not granted"
                )
            )
            return Result.failure(SecurityException("Permission denied for tool ${request.toolId}"))
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

            runCatching {
                apiClient.disableExtension(extensionId)
                apiClient.enableExtension(extensionId)
            }.onFailure { return@withLock Result.failure(it) }

            current.contributions.forEach { contrib ->
                contributionIndex.remove(contrib.id)
            }
            current.tools.forEach { tool ->
                toolIndex.remove(tool.toolId)
            }

            val reloaded = current.copy(
                state = ExtensionState.Activated,
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

            installationDao.updateStatus(extensionId, ExtensionState.Activated.name)

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

    override suspend fun restoreFromDatabase(): Result<List<LoadedExtension>> = runCatching {
        val entities = installationDao.getAll()
        if (entities.isEmpty()) return@runCatching emptyList()

        val backendResponse = runCatching { apiClient.listExtensions() }.getOrNull()
        val backendExtensions = backendResponse?.get("extensions")?.jsonArray
            ?.associateBy { it.jsonObject["extensionId"]?.jsonPrimitive?.contentOrNull }
            ?: emptyMap()

        val restored = mutableListOf<LoadedExtension>()

        entities.forEach { entity ->
            val backendItem = backendExtensions[entity.extensionId]?.jsonObject
            val enablement = backendItem?.get("enablement")?.jsonPrimitive?.contentOrNull

            val state = when (enablement) {
                "enabled" -> ExtensionState.Activated
                "disabled" -> ExtensionState.Deactivated
                else -> ExtensionState.Loaded
            }

            val detail = runCatching {
                apiClient.getExtensionDetail(entity.extensionId)
            }.getOrNull()

            val modules = detail?.get("modules")?.jsonArray?.map { modElem ->
                val mod = modElem.jsonObject
                ModuleMeta(
                    id = mod["id"]?.jsonPrimitive?.contentOrNull ?: "",
                    name = LocalizedText(default = mod["id"]?.jsonPrimitive?.contentOrNull ?: ""),
                    type = mod["type"]?.jsonPrimitive?.contentOrNull ?: "",
                    runtime = mod["runtime"]?.jsonPrimitive?.contentOrNull?.let { rt ->
                        RuntimeMeta(type = rt, entryPoint = mod["entryPoint"]?.jsonPrimitive?.contentOrNull)
                    }
                )
            } ?: emptyList()

            val contributionList = detail?.get("contributions")?.jsonArray?.map { contribElem ->
                val contrib = contribElem.jsonObject
                ExtensionContribution(
                    id = contrib["id"]?.jsonPrimitive?.contentOrNull ?: "",
                    type = ContributionType.fromValue(
                        contrib["kind"]?.jsonPrimitive?.contentOrNull ?: "tool"
                    ) ?: ContributionType.Tool,
                    extensionId = entity.extensionId,
                    moduleId = contrib["moduleId"]?.jsonPrimitive?.contentOrNull ?: "",
                    toolId = if (contrib["kind"]?.jsonPrimitive?.contentOrNull == "tool")
                        contrib["id"]?.jsonPrimitive?.contentOrNull else null
                )
            } ?: emptyList()

            val tools = contributionList.filter { it.type == ContributionType.Tool }.map { contrib ->
                val module = modules.firstOrNull { it.id == contrib.moduleId }
                ToolDefinition(
                    toolId = contrib.toolId ?: contrib.id,
                    extensionId = entity.extensionId,
                    moduleId = contrib.moduleId,
                    name = module?.name?.default ?: contrib.id,
                    description = "",
                    entryPoint = module?.runtime?.entryPoint ?: "",
                    timeout = parseTimeout(module?.runtime?.timeout),
                    permissions = module?.runtime?.permissions ?: emptyList()
                )
            }

            val manifest = ExtensionManifest(
                extension = ExtensionMeta(
                    id = entity.extensionId,
                    name = LocalizedText(default = entity.extensionId),
                    version = entity.version
                ),
                modules = modules
            )

            val loaded = LoadedExtension(
                extensionId = entity.extensionId,
                version = entity.version,
                displayName = entity.extensionId,
                manifest = manifest,
                packageHash = entity.manifestHash,
                state = state,
                contributions = contributionList,
                tools = tools,
                loadedAt = entity.installedAt
            )

            extensions[entity.extensionId] = loaded
            extensionLocks.computeIfAbsent(entity.extensionId) { Mutex() }

            contributionList.forEach { contrib ->
                contributionIndex[contrib.id] = contrib
            }
            tools.forEach { tool ->
                toolIndex[tool.toolId] = tool
            }

            restored.add(loaded)
        }

        restored
    }

    override fun getPermissionChecker(): ExtensionPermissionChecker = permissionChecker

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

    private fun readLimitedStream(stream: InputStream, maxBytes: Long): ByteArray {
        val buffer = ByteArray(8192)
        val output = ByteArrayOutputStream()
        var totalRead = 0L
        while (totalRead < maxBytes) {
            val toRead = minOf(buffer.size.toLong(), maxBytes - totalRead).toInt()
            val read = stream.read(buffer, 0, toRead)
            if (read == -1) break
            output.write(buffer, 0, read)
            totalRead += read
        }
        if (totalRead >= maxBytes) {
            val peek = stream.read()
            if (peek != -1) {
                throw SizeLimitExceededException(
                    "stream exceeds max package size $maxBytes"
                )
            }
        }
        return output.toByteArray()
    }
}
