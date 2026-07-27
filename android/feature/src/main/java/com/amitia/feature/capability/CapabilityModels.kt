package com.amitia.feature.capability

import com.amitia.core.designsystem.component.AmitiaStatusType

data class CapabilityOverview(
    val skillCount: Int = 0,
    val pluginCount: Int = 0,
    val mcpCount: Int = 0,
    val enabledCount: Int = 0,
    val systemCapabilityCount: Int = 0
)

data class EnabledCapability(
    val id: String,
    val name: String,
    val description: String,
    val type: CapabilityType,
    val status: AmitiaStatusType
)

enum class CapabilityType { Skill, Plugin, Mcp, System }

data class PluginInfo(
    val id: String,
    val name: String,
    val description: String,
    val version: String,
    val author: String,
    val source: String,
    val enabled: Boolean,
    val isSystem: Boolean,
    val canUninstall: Boolean,
    val status: AmitiaStatusType,
    val permissions: List<PluginPermission> = emptyList(),
    val tools: List<String> = emptyList(),
    val events: List<String> = emptyList(),
    val hooks: List<String> = emptyList(),
    val tasks: List<String> = emptyList(),
    val uiContributions: List<String> = emptyList(),
    val roles: List<String> = emptyList(),
    val updateAvailable: Boolean = false,
    val impactDescription: String? = null
)

data class PluginPermission(
    val name: String,
    val description: String,
    val granted: Boolean,
    val riskLevel: PermissionRiskLevel,
    val category: PermissionCategory
)

enum class PermissionRiskLevel(val label: String, val weight: Int) {
    Low("低风险", 0), Medium("中风险", 1), High("高风险", 2), Critical("严重风险", 3)
}

enum class PermissionCategory(val label: String) {
    DataAccess("数据访问"),
    Network("网络"),
    File("文件"),
    BackgroundTask("后台任务"),
    UiContribution("UI Contribution"),
    SystemControl("系统控制")
}

data class SkillInfo(
    val id: String,
    val name: String,
    val description: String,
    val source: SkillSource,
    val version: String,
    val inputSchema: String,
    val outputSchema: String,
    val declaredMcp: List<String> = emptyList(),
    val requiredPermissions: List<String> = emptyList(),
    val roles: List<String> = emptyList(),
    val updateAvailable: Boolean = false
)

enum class SkillSource(val label: String) {
    System("系统"), User("用户导入"), Community("社区")
}

data class McpServerInfo(
    val id: String,
    val name: String,
    val connectionType: String,
    val status: AmitiaStatusType,
    val toolCount: Int,
    val sourceSkill: String,
    val roles: List<String> = emptyList(),
    val config: Map<String, String> = emptyMap(),
    val tools: List<McpTool> = emptyList(),
    val resources: List<String> = emptyList(),
    val promptTemplates: List<String> = emptyList(),
    val recentCalls: List<McpCallRecord> = emptyList(),
    val errors: List<String> = emptyList()
)

data class McpTool(val name: String, val description: String)

data class McpCallRecord(
    val toolName: String,
    val timestamp: String,
    val success: Boolean,
    val duration: String
)

data class ExtensionLogEntry(
    val id: String,
    val timestamp: String,
    val level: ExtensionLogLevel,
    val source: String,
    val message: String,
    val extensionName: String,
    val masked: Boolean = true
)

enum class ExtensionLogLevel(val label: String) {
    Debug("DEBUG"), Info("INFO"), Warning("WARN"), Error("ERROR")
}

data class ExtensionUpdateInfo(
    val extensionId: String,
    val name: String,
    val currentVersion: String,
    val newVersion: String,
    val permissionChanges: List<String> = emptyList(),
    val changelog: String,
    val updateMethod: UpdateMethod,
    val failedRollback: Boolean = false
)

enum class UpdateMethod(val label: String) {
    Auto("自动更新"), Manual("手动更新"), Scheduled("定时更新")
}

data class ImportProgress(
    val step: ImportStep,
    val progress: Float,
    val message: String,
    val permissions: List<PluginPermission> = emptyList()
)

enum class ImportStep(val label: String) {
    SelectFile("选择文件"),
    Verify("校验签名与结构"),
    ShowPermissions("展示权限"),
    Install("安装"),
    Done("完成")
}

data class TestResult(
    val target: String,
    val input: String,
    val output: String,
    val success: Boolean,
    val duration: String,
    val timestamp: String
)

data class CapabilityTestTarget(
    val id: String,
    val name: String,
    val type: CapabilityType,
    val description: String
)
