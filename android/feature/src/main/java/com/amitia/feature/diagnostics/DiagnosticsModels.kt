package com.amitia.feature.diagnostics

import com.amitia.core.designsystem.component.AmitiaStatusType

data class DiagSummaryItem(
    val key: String,
    val label: String,
    val status: AmitiaStatusType,
    val detail: String
)

data class DiagServiceStatus(
    val name: String,
    val pid: Int?,
    val status: AmitiaStatusType,
    val startTime: String,
    val restartCount: Int,
    val port: Int?,
    val healthCheck: String,
    val version: String,
    val description: String,
    val advancedCollapsed: Boolean = true
)

enum class DiagDbType(val label: String) {
    SQLite("SQLite"),
    Qdrant("Qdrant"),
    SurrealDB("SurrealDB")
}

data class DiagDatabaseStatus(
    val type: DiagDbType,
    val name: String,
    val status: AmitiaStatusType,
    val migrationVersion: String?,
    val connection: String,
    val healthCheck: String,
    val storageSize: String,
    val details: Map<String, String> = emptyMap()
)

enum class DiagTaskStatus(val label: String) {
    Running("运行中"),
    Pending("等待中"),
    Completed("已完成"),
    Failed("失败"),
    Cancelled("已取消")
}

data class DiagTaskInfo(
    val id: String,
    val name: String,
    val status: DiagTaskStatus,
    val extension: String,
    val nextRun: String?,
    val timeout: String,
    val retryCount: Int
)

enum class DiagLifecycle(val label: String) {
    Active("活跃"),
    Idle("空闲"),
    Stopped("已停止"),
    Crashed("已崩溃")
}

data class DiagTrustedService(
    val id: String,
    val name: String,
    val lifecycle: DiagLifecycle,
    val permissions: List<String>,
    val callStatus: String,
    val crashes: Int,
    val restarts: Int
)

data class DiagWasmModule(
    val id: String,
    val name: String,
    val version: String,
    val memoryLimit: String,
    val cpuTime: String,
    val permissions: List<String>,
    val instanceStatus: AmitiaStatusType
)

data class DiagHook(
    val id: String,
    val name: String,
    val triggerPoint: String,
    val extension: String,
    val order: Int,
    val duration: String,
    val failurePolicy: String,
    val enabled: Boolean
)

data class DiagEvent(
    val id: String,
    val type: String,
    val publisher: String,
    val subscribers: List<String>,
    val lastTrigger: String,
    val failures: Int
)

enum class DiagMissedPolicy(val label: String) {
    Skip("跳过"),
    ExecuteImmediately("立即执行"),
    Queue("排队执行")
}

data class DiagSchedule(
    val id: String,
    val rule: String,
    val nextRun: String,
    val lastRun: String?,
    val missedPolicy: DiagMissedPolicy,
    val enabled: Boolean
)

data class DiagUiContribution(
    val id: String,
    val source: String,
    val mountPoint: String,
    val schemaVersion: String,
    val permissions: List<String>,
    val themeCompatible: Boolean,
    val renderErrors: List<String>
)

data class DiagRestrictedWebUi(
    val id: String,
    val source: String,
    val isolation: String,
    val domain: String,
    val permissions: List<String>,
    val csp: String,
    val status: AmitiaStatusType
)

data class DiagUpdateInfo(
    val id: String,
    val target: String,
    val currentVersion: String,
    val availableVersion: String?,
    val status: AmitiaStatusType,
    val compatible: Boolean,
    val lastCheck: String
)

data class DiagMigration(
    val id: String,
    val script: String,
    val version: String,
    val batch: Int,
    val successRate: String,
    val rollbackPoint: String,
    val rollbackStatus: String?
)

enum class DiagAuditType(val label: String) {
    Permission("权限审计"),
    ExtensionCall("扩展调用"),
    DataAccess("数据访问"),
    ComputerUse("Computer Use"),
    ChannelDelivery("渠道投递"),
    ConfigChange("敏感配置变更")
}

enum class DiagSeverity(val label: String) {
    Info("信息"),
    Warning("警告"),
    Error("错误"),
    Critical("严重")
}

data class DiagAuditEntry(
    val id: String,
    val type: DiagAuditType,
    val action: String,
    val timestamp: String,
    val user: String,
    val details: String,
    val severity: DiagSeverity
)

enum class DiagMetricCategory(val label: String) {
    Fps("Compose 帧率"),
    Recomposition("重组热点"),
    Memory("内存"),
    Runtime("本地运行时资源"),
    ModelLatency("模型延迟"),
    DbLatency("数据库延迟")
}

data class DiagPerformanceMetric(
    val id: String,
    val name: String,
    val value: String,
    val unit: String,
    val threshold: String?,
    val trend: String,
    val category: DiagMetricCategory
)

enum class DiagLogLevel(val label: String) {
    Debug("DEBUG"),
    Info("INFO"),
    Warning("WARN"),
    Error("ERROR")
}

data class DiagLogEntry(
    val id: String,
    val level: DiagLogLevel,
    val source: String,
    val timestamp: String,
    val message: String,
    val sanitized: Boolean
)

data class DiagFeatureFlag(
    val key: String,
    val description: String,
    val source: String,
    val currentValue: String,
    val defaultValue: String,
    val enabled: Boolean,
    val grayRelease: Boolean
)
