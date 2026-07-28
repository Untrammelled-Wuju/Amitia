package com.amitia.feature.capability

import android.content.Context
import android.net.Uri
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.database.dao.ExtensionInstallationDao
import com.amitia.core.database.entity.ExtensionInstallationEntity
import com.amitia.core.designsystem.component.AmitiaStatusType
import com.amitia.runtime.extension.ExtensionApiClient
import com.amitia.runtime.extension.ExtensionHost
import com.amitia.runtime.extension.ExtensionHostEvent
import com.amitia.runtime.extension.ExtensionHostEventType
import com.amitia.runtime.extension.ExtensionState
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive

sealed class ImportState {
    object Idle : ImportState()
    data class Importing(val progress: Float, val message: String) : ImportState()
    data class Success(val extensionId: String) : ImportState()
    data class Error(val message: String) : ImportState()
}

@HiltViewModel
class CapabilityViewModel @Inject constructor(
    @ApplicationContext private val context: Context,
    private val extensionHost: ExtensionHost,
    private val installationDao: ExtensionInstallationDao,
    private val apiClient: ExtensionApiClient
) : ViewModel() {

    private val _overview = MutableStateFlow<CapabilityOverview?>(null)
    val overview: StateFlow<CapabilityOverview?> = _overview.asStateFlow()

    private val _enabledCapabilities = MutableStateFlow<List<EnabledCapability>>(emptyList())
    val enabledCapabilities: StateFlow<List<EnabledCapability>> = _enabledCapabilities.asStateFlow()

    private val _plugins = MutableStateFlow<List<PluginInfo>>(emptyList())
    val plugins: StateFlow<List<PluginInfo>> = _plugins.asStateFlow()

    private val _skills = MutableStateFlow<List<SkillInfo>>(emptyList())
    val skills: StateFlow<List<SkillInfo>> = _skills.asStateFlow()

    private val _mcpServers = MutableStateFlow<List<McpServerInfo>>(emptyList())
    val mcpServers: StateFlow<List<McpServerInfo>> = _mcpServers.asStateFlow()

    private val _logs = MutableStateFlow<List<ExtensionLogEntry>>(emptyList())
    val logs: StateFlow<List<ExtensionLogEntry>> = _logs.asStateFlow()

    private val _updates = MutableStateFlow<List<ExtensionUpdateInfo>>(emptyList())
    val updates: StateFlow<List<ExtensionUpdateInfo>> = _updates.asStateFlow()

    private val _loading = MutableStateFlow(true)
    val loading: StateFlow<Boolean> = _loading.asStateFlow()

    private val _importState = MutableStateFlow<ImportState>(ImportState.Idle)
    val importState: StateFlow<ImportState> = _importState.asStateFlow()

    private val logBuffer = mutableListOf<ExtensionLogEntry>()
    private var logCounter = 0L

    init {
        viewModelScope.launch {
            extensionHost.restoreFromDatabase()
            collectEvents()
            loadAll()
        }
    }

    private suspend fun collectEvents() {
        extensionHost.events.collect { event ->
            val entry = mapEventToLog(event)
            logBuffer.add(0, entry)
            if (logBuffer.size > 100) {
                logBuffer.removeAt(logBuffer.lastIndex)
            }
            _logs.value = logBuffer.toList()
        }
    }

    private fun mapEventToLog(event: ExtensionHostEvent): ExtensionLogEntry {
        val level = when (event.type) {
            ExtensionHostEventType.LoadFailed,
            ExtensionHostEventType.ToolFailed -> ExtensionLogLevel.Error
            ExtensionHostEventType.LoadStarted -> ExtensionLogLevel.Debug
            ExtensionHostEventType.LoadSucceeded,
            ExtensionHostEventType.Activated,
            ExtensionHostEventType.Deactivated,
            ExtensionHostEventType.Unloaded,
            ExtensionHostEventType.ToolExecuted,
            ExtensionHostEventType.ContributionRegistered -> ExtensionLogLevel.Info
        }
        val source = when (event.type) {
            ExtensionHostEventType.ToolExecuted,
            ExtensionHostEventType.ToolFailed -> "Tool"
            ExtensionHostEventType.ContributionRegistered -> "Contribution"
            else -> "Extension"
        }
        logCounter++
        return ExtensionLogEntry(
            id = logCounter.toString(),
            timestamp = formatTimestamp(event.timestamp),
            level = level,
            source = source,
            message = event.message ?: event.type.name,
            extensionName = event.extensionId
        )
    }

    private fun formatTimestamp(ts: Long): String {
        val sdf = java.text.SimpleDateFormat("HH:mm:ss", java.util.Locale.getDefault())
        return sdf.format(java.util.Date(ts))
    }

    fun loadAll() {
        viewModelScope.launch {
            _loading.value = true
            try {
                val installations = installationDao.getAll()
                val loadedExtensions = extensionHost.listExtensions()

                loadPlugins(installations, loadedExtensions)
                loadSkills(loadedExtensions)
                loadMcpServers(loadedExtensions)
                loadEnabledCapabilities(installations, loadedExtensions)
                computeOverview(installations)

                loadBackendUpdates()
            } catch (e: Exception) {
                _logs.value = logBuffer.toList()
            }
            _loading.value = false
        }
    }

    private fun loadPlugins(
        installations: List<ExtensionInstallationEntity>,
        loadedExtensions: List<com.amitia.runtime.extension.LoadedExtension>
    ) {
        val pluginList = installations.map { entity ->
            val loaded = loadedExtensions.find { it.extensionId == entity.extensionId }
            val isActive = loaded?.state == ExtensionState.Activated
            PluginInfo(
                id = entity.extensionId,
                name = loaded?.displayName ?: entity.extensionId,
                description = loaded?.manifest?.extension?.description?.default ?: "",
                version = entity.version,
                author = loaded?.manifest?.publisher?.displayName ?: "",
                source = "已安装",
                enabled = isActive,
                isSystem = false,
                canUninstall = true,
                status = if (isActive) AmitiaStatusType.Running else AmitiaStatusType.Idle,
                tools = loaded?.tools?.map { it.toolId } ?: emptyList(),
                permissions = loaded?.manifest?.permissions?.map { perm ->
                    PluginPermission(
                        name = perm.id,
                        description = perm.reason ?: "",
                        granted = true,
                        riskLevel = mapPermissionRisk(perm.scope),
                        category = mapPermissionCategory(perm.id)
                    )
                } ?: emptyList(),
                uiContributions = loaded?.contributions
                    ?.filter { it.type == com.amitia.runtime.extension.ContributionType.Ui }
                    ?.map { it.id } ?: emptyList(),
                tasks = loaded?.contributions
                    ?.filter { it.type == com.amitia.runtime.extension.ContributionType.BackgroundTask }
                    ?.map { it.id } ?: emptyList()
            )
        }
        _plugins.value = pluginList
    }

    private fun loadSkills(loadedExtensions: List<com.amitia.runtime.extension.LoadedExtension>) {
        val skillList = loadedExtensions.flatMap { ext ->
            ext.contributions.filter { it.type == com.amitia.runtime.extension.ContributionType.AgentSkill }.map { contrib ->
                SkillInfo(
                    id = contrib.id,
                    name = ext.displayName,
                    description = ext.manifest.extension.description?.default ?: "",
                    source = SkillSource.User,
                    version = ext.version,
                    inputSchema = "object",
                    outputSchema = "object",
                    roles = emptyList()
                )
            }
        }
        _skills.value = skillList
    }

    private fun loadMcpServers(loadedExtensions: List<com.amitia.runtime.extension.LoadedExtension>) {
        val mcpList = loadedExtensions.flatMap { ext ->
            ext.contributions.filter { it.type == com.amitia.runtime.extension.ContributionType.Mcp }.map { contrib ->
                val isActive = ext.state == ExtensionState.Activated
                McpServerInfo(
                    id = contrib.id,
                    name = ext.displayName,
                    connectionType = "stdio",
                    status = if (isActive) AmitiaStatusType.Connected else AmitiaStatusType.Disconnected,
                    toolCount = ext.tools.size,
                    sourceSkill = ext.extensionId,
                    tools = ext.tools.map { com.amitia.feature.capability.McpTool(it.toolId, it.description) }
                )
            }
        }
        _mcpServers.value = mcpList
    }

    private fun loadEnabledCapabilities(
        installations: List<ExtensionInstallationEntity>,
        loadedExtensions: List<com.amitia.runtime.extension.LoadedExtension>
    ) {
        val capabilities = mutableListOf<EnabledCapability>()
        loadedExtensions.forEach { ext ->
            val isActive = ext.state == ExtensionState.Activated
            ext.contributions.forEach { contrib ->
                val type = when (contrib.type) {
                    com.amitia.runtime.extension.ContributionType.Tool -> CapabilityType.Plugin
                    com.amitia.runtime.extension.ContributionType.AgentSkill -> CapabilityType.Skill
                    com.amitia.runtime.extension.ContributionType.Mcp -> CapabilityType.Mcp
                    else -> CapabilityType.Plugin
                }
                capabilities.add(
                    EnabledCapability(
                        id = contrib.id,
                        name = ext.displayName,
                        description = contrib.id,
                        type = type,
                        status = if (isActive) AmitiaStatusType.Running else AmitiaStatusType.Idle
                    )
                )
            }
        }
        _enabledCapabilities.value = capabilities
    }

    private fun computeOverview(installations: List<ExtensionInstallationEntity>) {
        val plugins = _plugins.value
        val skills = _skills.value
        val mcps = _mcpServers.value
        _overview.value = CapabilityOverview(
            skillCount = skills.size,
            pluginCount = plugins.size,
            mcpCount = mcps.size,
            enabledCount = plugins.count { it.enabled } + skills.size + mcps.size,
            systemCapabilityCount = 0
        )
    }

    private suspend fun loadBackendUpdates() {
        val result = runCatching { apiClient.listPlugins() }.getOrNull()
        if (result != null) {
            val items = result["items"]?.jsonArray ?: result["plugins"]?.jsonArray
            if (items != null) {
                val updatesList = items.mapNotNull { item ->
                    val obj = item.jsonObject
                    val extId = obj["id"]?.jsonPrimitive?.contentOrNull ?: return@mapNotNull null
                    val currentVersion = obj["version"]?.jsonPrimitive?.contentOrNull ?: return@mapNotNull null
                    val updateVersion = obj["updateVersion"]?.jsonPrimitive?.contentOrNull
                        ?: obj["latestVersion"]?.jsonPrimitive?.contentOrNull
                        ?: return@mapNotNull null
                    if (updateVersion != currentVersion) {
                        ExtensionUpdateInfo(
                            extensionId = extId,
                            name = obj["name"]?.jsonPrimitive?.contentOrNull ?: extId,
                            currentVersion = currentVersion,
                            newVersion = updateVersion,
                            changelog = obj["changelog"]?.jsonPrimitive?.contentOrNull ?: "",
                            updateMethod = UpdateMethod.Manual
                        )
                    } else null
                }
                _updates.value = updatesList
            }
        }
    }

    fun togglePlugin(pluginId: String, enabled: Boolean) {
        viewModelScope.launch {
            if (enabled) {
                extensionHost.activate(pluginId)
            } else {
                extensionHost.deactivate(pluginId)
            }
            loadAll()
        }
    }

    fun toggleSkill(skillId: String) {
        viewModelScope.launch {
            val ext = extensionHost.listExtensions().find { it.extensionId == skillId }
            if (ext != null) {
                if (ext.state == ExtensionState.Activated) {
                    extensionHost.deactivate(skillId)
                } else {
                    extensionHost.activate(skillId)
                }
                loadAll()
            }
        }
    }

    fun toggleMcp(serverId: String, enabled: Boolean) {
        viewModelScope.launch {
            if (enabled) {
                extensionHost.activate(serverId)
            } else {
                extensionHost.deactivate(serverId)
            }
            loadAll()
        }
    }

    fun filterLogsByExtension(name: String) {
        _logs.value = if (name.isBlank()) logBuffer.toList() else logBuffer.filter { it.extensionName == name }
    }

    fun importExtension(uri: Uri) {
        viewModelScope.launch {
            _importState.value = ImportState.Importing(0.2f, "正在读取文件...")
            try {
                val stream = context.contentResolver.openInputStream(uri)
                if (stream == null) {
                    _importState.value = ImportState.Error("无法打开文件")
                    return@launch
                }
                _importState.value = ImportState.Importing(0.5f, "正在上传到后端安装...")
                val result = stream.use { extensionHost.loadPackage(it) }
                result.onSuccess { ext ->
                    _importState.value = ImportState.Importing(0.8f, "正在激活...")
                    extensionHost.activate(ext.extensionId)
                    _importState.value = ImportState.Success(ext.extensionId)
                    loadAll()
                }.onFailure { e ->
                    _importState.value = ImportState.Error(e.message ?: "导入失败")
                }
            } catch (e: Exception) {
                _importState.value = ImportState.Error(e.message ?: "导入失败")
            }
        }
    }

    fun resetImportState() {
        _importState.value = ImportState.Idle
    }

    fun restoreExtensions() {
        viewModelScope.launch {
            extensionHost.restoreFromDatabase()
            loadAll()
        }
    }

    private fun mapPermissionRisk(scopes: List<String>): PermissionRiskLevel {
        if (scopes.any { it.contains("critical", ignoreCase = true) }) return PermissionRiskLevel.Critical
        if (scopes.any { it.contains("high", ignoreCase = true) || it.contains("network", ignoreCase = true) }) return PermissionRiskLevel.High
        if (scopes.any { it.contains("file", ignoreCase = true) || it.contains("data", ignoreCase = true) }) return PermissionRiskLevel.Medium
        return PermissionRiskLevel.Low
    }

    private fun mapPermissionCategory(permId: String): PermissionCategory {
        return when {
            permId.contains("network", ignoreCase = true) -> PermissionCategory.Network
            permId.contains("file", ignoreCase = true) || permId.contains("fs", ignoreCase = true) -> PermissionCategory.File
            permId.contains("data", ignoreCase = true) -> PermissionCategory.DataAccess
            permId.contains("background", ignoreCase = true) || permId.contains("task", ignoreCase = true) -> PermissionCategory.BackgroundTask
            permId.contains("ui", ignoreCase = true) -> PermissionCategory.UiContribution
            permId.contains("system", ignoreCase = true) -> PermissionCategory.SystemControl
            else -> PermissionCategory.DataAccess
        }
    }
}
