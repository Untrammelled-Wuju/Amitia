package com.amitia.feature.computeruse

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
class ComputerUseViewModel @Inject constructor() : ViewModel() {

    private val _overview = MutableStateFlow(ComputerUseOverview())
    val overview: StateFlow<ComputerUseOverview> = _overview.asStateFlow()

    private val _devices = MutableStateFlow<List<ControllableDevice>>(emptyList())
    val devices: StateFlow<List<ControllableDevice>> = _devices.asStateFlow()

    private val _sessions = MutableStateFlow<List<ComputerUseSession>>(emptyList())
    val sessions: StateFlow<List<ComputerUseSession>> = _sessions.asStateFlow()

    private val _pendingApprovals = MutableStateFlow<List<PendingApproval>>(emptyList())
    val pendingApprovals: StateFlow<List<PendingApproval>> = _pendingApprovals.asStateFlow()

    private val _history = MutableStateFlow<List<OperationHistoryEntry>>(emptyList())
    val history: StateFlow<List<OperationHistoryEntry>> = _history.asStateFlow()

    private val _permissions = MutableStateFlow<List<SystemPermissionState>>(emptyList())
    val permissions: StateFlow<List<SystemPermissionState>> = _permissions.asStateFlow()

    private val _safetyRules = MutableStateFlow<List<SafetyRule>>(emptyList())
    val safetyRules: StateFlow<List<SafetyRule>> = _safetyRules.asStateFlow()

    private val _currentSession = MutableStateFlow<ComputerUseSession?>(null)
    val currentSession: StateFlow<ComputerUseSession?> = _currentSession.asStateFlow()

    private val _historyMasked = MutableStateFlow(true)
    val historyMasked: StateFlow<Boolean> = _historyMasked.asStateFlow()

    private val _loading = MutableStateFlow(true)
    val loading: StateFlow<Boolean> = _loading.asStateFlow()

    init {
        loadAll()
    }

    fun loadAll() {
        viewModelScope.launch {
            _loading.value = true
            _overview.value = ComputerUseOverview(
                enabled = true,
                currentMode = PermissionMode.ManualApproval,
                deviceCount = 2,
                recentSessionCount = 3,
                pendingApprovalCount = 2,
                riskDescription = "Computer Use 涉及设备控制，请谨慎配置权限模式与安全规则"
            )
            _devices.value = sampleDevices()
            _sessions.value = sampleSessions()
            _pendingApprovals.value = sampleApprovals()
            _history.value = sampleHistory()
            _permissions.value = samplePermissions()
            _safetyRules.value = sampleSafetyRules()
            _currentSession.value = sampleSessions().firstOrNull()
            _loading.value = false
        }
    }

    fun toggleComputerUse(enabled: Boolean) {
        _overview.update { it.copy(enabled = enabled) }
    }

    fun setPermissionMode(mode: PermissionMode) {
        _overview.update { it.copy(currentMode = mode) }
    }

    fun approveOnce(approvalId: String) {
        _pendingApprovals.update { list -> list.filter { it.id != approvalId } }
        _overview.update { it.copy(pendingApprovalCount = it.pendingApprovalCount - 1) }
    }

    fun alwaysAllow(approvalId: String) {
        _pendingApprovals.update { list -> list.filter { it.id != approvalId } }
        _overview.update { it.copy(pendingApprovalCount = it.pendingApprovalCount - 1) }
    }

    fun denyApproval(approvalId: String) {
        _pendingApprovals.update { list -> list.filter { it.id != approvalId } }
        _overview.update { it.copy(pendingApprovalCount = it.pendingApprovalCount - 1) }
    }

    fun toggleSafetyRule(ruleId: String, enabled: Boolean) {
        _safetyRules.update { list ->
            list.map { if (it.id == ruleId) it.copy(enabled = enabled) else it }
        }
    }

    fun updateRulePriority(ruleId: String, newPriority: Int) {
        _safetyRules.update { list ->
            list.map { if (it.id == ruleId) it.copy(priority = newPriority) else it }
        }
    }

    fun pauseSession() {
        _currentSession.update { it?.copy(status = SessionStatus.Paused) }
    }

    fun resumeSession() {
        _currentSession.update { it?.copy(status = SessionStatus.Running) }
    }

    fun stopSession() {
        _currentSession.update { it?.copy(status = SessionStatus.Stopped) }
    }

    fun takeoverSession() {
        _currentSession.update { it?.copy(status = SessionStatus.Stopped) }
    }

    fun toggleHistoryMasking() {
        _historyMasked.update { !it }
    }

    fun moveRuleUp(ruleId: String) {
        _safetyRules.update { list ->
            val sorted = list.sortedBy { it.priority }
            val index = sorted.indexOfFirst { it.id == ruleId }
            if (index > 0) {
                val swapped = sorted.toMutableList()
                val higher = swapped[index - 1]
                swapped[index - 1] = sorted[index].copy(priority = higher.priority)
                swapped[index] = higher.copy(priority = sorted[index].priority)
                swapped
            } else list
        }
    }

    fun moveRuleDown(ruleId: String) {
        _safetyRules.update { list ->
            val sorted = list.sortedBy { it.priority }
            val index = sorted.indexOfFirst { it.id == ruleId }
            if (index in 0 until sorted.lastIndex) {
                val swapped = sorted.toMutableList()
                val lower = swapped[index + 1]
                swapped[index + 1] = sorted[index].copy(priority = lower.priority)
                swapped[index] = lower.copy(priority = sorted[index].priority)
                swapped
            } else list
        }
    }

    private fun sampleDevices() = listOf(
        ControllableDevice("d1", "本机", DeviceType.Android, AmitiaStatusType.Connected, "2 分钟前"),
        ControllableDevice("d2", "桌面端", DeviceType.Desktop, AmitiaStatusType.Disconnected, null)
    )

    private fun sampleSessions() = listOf(
        ComputerUseSession(
            "s1", "整理桌面文件", SessionStatus.Running, "正在扫描文件",
            "桌面显示 12 个待整理文件夹",
            listOf(
                SessionStep("1", "扫描桌面", StepStatus.Done, "14:30"),
                SessionStep("2", "分类文件", StepStatus.Running, "14:31"),
                SessionStep("3", "移动到对应目录", StepStatus.Pending, "")
            ),
            "14:30"
        )
    )

    private fun sampleApprovals() = listOf(
        PendingApproval(
            "a1", "删除文件 config.ini", ApprovalRisk.High,
            "艾米", "清理临时文件", "请求删除 Downloads 目录下的配置文件", "14:32"
        ),
        PendingApproval(
            "a2", "打开支付宝", ApprovalRisk.Critical,
            "艾米", "代为支付账单", "请求打开支付宝应用", "14:35"
        )
    )

    private fun sampleHistory() = listOf(
        OperationHistoryEntry("h1", "14:30", "艾米", "文件管理器", "创建文件夹", OperationResult.Success, ApprovalMethod.ManualOnce),
        OperationHistoryEntry("h2", "14:28", "艾米", "浏览器", "打开网页", OperationResult.Success, ApprovalMethod.Auto),
        OperationHistoryEntry("h3", "14:25", "艾米", "支付宝", "发起转账", OperationResult.Blocked, ApprovalMethod.Denied, sensitive = true)
    )

    private fun samplePermissions() = listOf(
        SystemPermissionState("无障碍服务", "允许控制和读取屏幕内容", false, true, "无障碍设置"),
        SystemPermissionState("屏幕捕获", "捕获屏幕截图供 AI 分析", false, true, "权限管理"),
        SystemPermissionState("通知读取", "读取应用通知内容", false, true, "通知访问设置"),
        SystemPermissionState("精确位置", "获取设备精确位置", false, true, "位置权限")
    )

    private fun sampleSafetyRules() = listOf(
        SafetyRule("r1", "禁止操作支付应用", "拦截支付宝、微信支付等金融操作", true, 1, SafetyRuleType.PaymentProtection),
        SafetyRule("r2", "禁止读取密码输入", "在密码输入框时不执行任何操作", true, 2, SafetyRuleType.PrivacyInput),
        SafetyRule("r3", "夜间限制", "23:00 - 07:00 禁止 Computer Use", false, 3, SafetyRuleType.NightRestriction),
        SafetyRule("r4", "禁止卸载应用", "禁止执行应用卸载操作", true, 4, SafetyRuleType.BlockedOperation),
        SafetyRule("r5", "连续失败自动停止", "连续 3 次操作失败后自动停止", true, 5, SafetyRuleType.AutoStop)
    )
}
