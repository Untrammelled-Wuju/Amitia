package com.amitia.feature.capability

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.designsystem.component.AmitiaStatusType
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

@HiltViewModel
class CapabilityViewModel @Inject constructor() : ViewModel() {

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

    init {
        loadAll()
    }

    fun loadAll() {
        viewModelScope.launch {
            _loading.value = true
            _overview.value = CapabilityOverview(
                skillCount = 8,
                pluginCount = 12,
                mcpCount = 3,
                enabledCount = 18,
                systemCapabilityCount = 5
            )
            _enabledCapabilities.value = sampleEnabledCapabilities()
            _plugins.value = samplePlugins()
            _skills.value = sampleSkills()
            _mcpServers.value = sampleMcpServers()
            _logs.value = sampleLogs()
            _updates.value = sampleUpdates()
            _loading.value = false
        }
    }

    fun togglePlugin(pluginId: String, enabled: Boolean) {
        _plugins.update { list ->
            list.map { if (it.id == pluginId) it.copy(enabled = enabled) else it }
        }
    }

    fun toggleSkill(skillId: String) {
        _skills.update { list -> list.map { if (it.id == skillId) it else it } }
    }

    fun toggleMcp(serverId: String, enabled: Boolean) {
        _mcpServers.update { list ->
            list.map {
                if (it.id == serverId) it.copy(
                    status = if (enabled) AmitiaStatusType.Connected else AmitiaStatusType.Disconnected
                ) else it
            }
        }
    }

    fun filterLogsByExtension(name: String) {
        _logs.update { list -> if (name.isBlank()) sampleLogs() else sampleLogs().filter { it.extensionName == name } }
    }

    private fun sampleEnabledCapabilities() = listOf(
        EnabledCapability("1", "对话记忆", "长期记忆与上下文管理", CapabilityType.System, AmitiaStatusType.Running),
        EnabledCapability("2", "天气查询", "实时天气信息查询插件", CapabilityType.Plugin, AmitiaStatusType.Running),
        EnabledCapability("3", "文件搜索", "MCP 文件检索服务", CapabilityType.Mcp, AmitiaStatusType.Connected),
        EnabledCapability("4", "意图识别", "对话意图分类 Skill", CapabilityType.Skill, AmitiaStatusType.Running)
    )

    private fun samplePlugins() = listOf(
        PluginInfo(
            id = "sys-1", name = "对话记忆", description = "管理长期记忆与上下文窗口",
            version = "1.0.0", author = "Amitia", source = "系统内置", enabled = true,
            isSystem = true, canUninstall = false, status = AmitiaStatusType.Running,
            tools = listOf("recall", "store"), events = listOf("onMessage"),
            impactDescription = "禁用后角色将无法记忆对话内容"
        ),
        PluginInfo(
            id = "sys-2", name = "渠道接入", description = "微信、QQ、Web 渠道连接",
            version = "1.0.0", author = "Amitia", source = "系统内置", enabled = true,
            isSystem = true, canUninstall = false, status = AmitiaStatusType.Running,
            impactDescription = "禁用后所有渠道将断开"
        ),
        PluginInfo(
            id = "pub-1", name = "天气查询", description = "提供实时天气信息查询",
            version = "1.2.0", author = "社区", source = "公共插件", enabled = true,
            isSystem = false, canUninstall = true, status = AmitiaStatusType.Running,
            updateAvailable = true,
            permissions = listOf(
                PluginPermission("网络访问", "查询天气数据", true, PermissionRiskLevel.Low, PermissionCategory.Network)
            )
        ),
        PluginInfo(
            id = "pub-2", name = "图片生成", description = "基于文本生成图像",
            version = "0.9.3", author = "社区", source = "公共插件", enabled = false,
            isSystem = false, canUninstall = true, status = AmitiaStatusType.Idle,
            permissions = listOf(
                PluginPermission("网络访问", "调用图像生成接口", false, PermissionRiskLevel.Medium, PermissionCategory.Network),
                PluginPermission("文件写入", "保存生成图片", false, PermissionRiskLevel.Medium, PermissionCategory.File)
            )
        )
    )

    private fun sampleSkills() = listOf(
        SkillInfo("s1", "意图识别", "识别用户对话意图", SkillSource.System, "1.0.0",
            "text:string", "intent:string", roles = listOf("艾米")),
        SkillInfo("s2", "情绪分析", "分析用户情绪倾向", SkillSource.User, "0.8.0",
            "text:string", "emotion:string", declaredMcp = listOf("emotion-api"),
            roles = listOf("艾米", "助手")),
        SkillInfo("s3", "代码解释", "解释代码片段含义", SkillSource.Community, "2.1.0",
            "code:string", "explanation:string", updateAvailable = true)
    )

    private fun sampleMcpServers() = listOf(
        McpServerInfo(
            id = "m1", name = "文件搜索服务", connectionType = "stdio",
            status = AmitiaStatusType.Connected, toolCount = 4, sourceSkill = "文件检索",
            roles = listOf("艾米"),
            tools = listOf(McpTool("search", "搜索文件"), McpTool("read", "读取文件内容")),
            recentCalls = listOf(McpCallRecord("search", "14:30", true, "120ms"))
        ),
        McpServerInfo(
            id = "m2", name = "数据库查询", connectionType = "sse",
            status = AmitiaStatusType.Disconnected, toolCount = 2, sourceSkill = "数据助手",
            errors = listOf("连接超时")
        )
    )

    private fun sampleLogs() = listOf(
        ExtensionLogEntry("1", "14:30:01", ExtensionLogLevel.Info, "Plugin", "插件已启动", "天气查询"),
        ExtensionLogEntry("2", "14:30:15", ExtensionLogLevel.Warning, "Hook", "Hook 执行耗时较长", "天气查询"),
        ExtensionLogEntry("3", "14:31:00", ExtensionLogLevel.Error, "MCP", "连接超时", "数据库查询"),
        ExtensionLogEntry("4", "14:32:10", ExtensionLogLevel.Debug, "Task", "任务调度检查", "对话记忆")
    )

    private fun sampleUpdates() = listOf(
        ExtensionUpdateInfo("pub-1", "天气查询", "1.2.0", "1.3.0",
            permissionChanges = listOf("新增位置信息访问权限"),
            changelog = "优化查询速度，新增降雨预警", updateMethod = UpdateMethod.Manual)
    )
}
