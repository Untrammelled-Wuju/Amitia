package com.amitia.feature.computeruse

import com.amitia.core.designsystem.component.AmitiaStatusType

data class ComputerUseOverview(
    val enabled: Boolean = false,
    val currentMode: PermissionMode = PermissionMode.ManualApproval,
    val deviceCount: Int = 0,
    val recentSessionCount: Int = 0,
    val pendingApprovalCount: Int = 0,
    val riskDescription: String = "Computer Use 涉及设备控制，请谨慎配置权限"
)

enum class PermissionMode(val label: String, val description: String, val risk: String) {
    FullControl("完全控制", "AI 可自主执行所有操作，无需逐次确认", "最高风险，AI 可执行任意操作"),
    AutoApproval("自动审批", "符合安全规则的操作自动执行，其余需确认", "中等风险，受安全规则约束"),
    ManualApproval("手动审批", "所有操作均需人工逐次批准", "最低风险，完全由用户控制")
}

data class ControllableDevice(
    val id: String,
    val name: String,
    val type: DeviceType,
    val status: AmitiaStatusType,
    val lastActive: String?
)

enum class DeviceType(val label: String) {
    Android("本机 Android"), Desktop("桌面端"), Web("浏览器")
}

data class ComputerUseSession(
    val id: String,
    val target: String,
    val status: SessionStatus,
    val currentStep: String,
    val screenSummary: String,
    val steps: List<SessionStep>,
    val startedAt: String
)

enum class SessionStatus(val label: String) {
    Running("运行中"), Paused("已暂停"), Stopped("已停止"), Pending("等待审批")
}

data class SessionStep(
    val id: String,
    val description: String,
    val status: StepStatus,
    val timestamp: String
)

enum class StepStatus(val label: String) {
    Done("已完成"), Running("执行中"), Pending("待执行"), Failed("失败")
}

data class PendingApproval(
    val id: String,
    val operation: String,
    val risk: ApprovalRisk,
    val sourceRole: String,
    val sourceTask: String,
    val description: String,
    val timestamp: String
)

enum class ApprovalRisk(val label: String) {
    Low("低风险"), Medium("中风险"), High("高风险"), Critical("严重风险")
}

data class OperationHistoryEntry(
    val id: String,
    val time: String,
    val role: String,
    val app: String,
    val operation: String,
    val result: OperationResult,
    val approvalMethod: ApprovalMethod,
    val sensitive: Boolean = true
)

enum class OperationResult(val label: String) {
    Success("成功"), Failed("失败"), Blocked("已拦截"), Cancelled("已取消")
}

enum class ApprovalMethod(val label: String) {
    Auto("自动审批"), ManualOnce("允许一次"), AlwaysAllow("始终允许"), Denied("已拒绝")
}

data class SystemPermissionState(
    val name: String,
    val description: String,
    val granted: Boolean,
    val systemLevel: Boolean,
    val settingsAction: String
)

data class SafetyRule(
    val id: String,
    val name: String,
    val description: String,
    val enabled: Boolean,
    val priority: Int,
    val type: SafetyRuleType,
    val config: Map<String, String> = emptyMap()
)

enum class SafetyRuleType(val label: String) {
    BlockedApp("禁止应用"),
    BlockedOperation("禁止操作"),
    PaymentProtection("金融/支付保护"),
    PrivacyInput("隐私输入保护"),
    NightRestriction("夜间限制"),
    AutoStop("自动停止条件")
}
