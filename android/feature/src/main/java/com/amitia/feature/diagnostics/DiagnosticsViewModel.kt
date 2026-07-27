package com.amitia.feature.diagnostics

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaStatusType
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

data class DiagnosticsUiState(
    val overview: ScreenState<List<DiagSummaryItem>> = ScreenState.Loading,
    val services: ScreenState<List<DiagServiceStatus>> = ScreenState.Loading,
    val databases: ScreenState<List<DiagDatabaseStatus>> = ScreenState.Loading,
    val tasks: ScreenState<List<DiagTaskInfo>> = ScreenState.Loading,
    val trustedServices: ScreenState<List<DiagTrustedService>> = ScreenState.Loading,
    val wasmModules: ScreenState<List<DiagWasmModule>> = ScreenState.Loading,
    val hooks: ScreenState<List<DiagHook>> = ScreenState.Loading,
    val events: ScreenState<List<DiagEvent>> = ScreenState.Loading,
    val schedules: ScreenState<List<DiagSchedule>> = ScreenState.Loading,
    val uiContributions: ScreenState<List<DiagUiContribution>> = ScreenState.Loading,
    val restrictedWebUis: ScreenState<List<DiagRestrictedWebUi>> = ScreenState.Loading,
    val updates: ScreenState<List<DiagUpdateInfo>> = ScreenState.Loading,
    val migrations: ScreenState<List<DiagMigration>> = ScreenState.Loading,
    val audit: ScreenState<List<DiagAuditEntry>> = ScreenState.Loading,
    val performance: ScreenState<List<DiagPerformanceMetric>> = ScreenState.Loading,
    val logs: ScreenState<List<DiagLogEntry>> = ScreenState.Loading,
    val featureFlags: ScreenState<List<DiagFeatureFlag>> = ScreenState.Loading
)

@HiltViewModel
class DiagnosticsViewModel @Inject constructor() : ViewModel() {

    private val _state = MutableStateFlow(DiagnosticsUiState())
    val state: StateFlow<DiagnosticsUiState> = _state.asStateFlow()

    init {
        loadAll()
    }

    fun loadAll() {
        loadOverview()
        loadServices()
        loadDatabases()
        loadTasks()
        loadTrustedServices()
        loadWasmModules()
        loadHooks()
        loadEvents()
        loadSchedules()
        loadUiContributions()
        loadRestrictedWebUis()
        loadUpdates()
        loadMigrations()
        loadAudit()
        loadPerformance()
        loadLogs()
        loadFeatureFlags()
    }

    fun loadOverview() = viewModelScope.launch {
        _state.value = _state.value.copy(overview = ScreenState.Loading)
        delay(300)
        _state.value = _state.value.copy(overview = ScreenState.Content(mockOverview()))
    }

    fun loadServices() = viewModelScope.launch {
        _state.value = _state.value.copy(services = ScreenState.Loading)
        delay(300)
        _state.value = _state.value.copy(services = ScreenState.Content(mockServices()))
    }

    fun loadDatabases() = viewModelScope.launch {
        _state.value = _state.value.copy(databases = ScreenState.Loading)
        delay(300)
        _state.value = _state.value.copy(databases = ScreenState.Content(mockDatabases()))
    }

    fun loadTasks() = viewModelScope.launch {
        _state.value = _state.value.copy(tasks = ScreenState.Loading)
        delay(300)
        _state.value = _state.value.copy(tasks = ScreenState.Content(mockTasks()))
    }

    fun loadTrustedServices() = viewModelScope.launch {
        _state.value = _state.value.copy(trustedServices = ScreenState.Loading)
        delay(300)
        _state.value = _state.value.copy(trustedServices = ScreenState.Content(mockTrustedServices()))
    }

    fun loadWasmModules() = viewModelScope.launch {
        _state.value = _state.value.copy(wasmModules = ScreenState.Loading)
        delay(300)
        _state.value = _state.value.copy(wasmModules = ScreenState.Content(mockWasmModules()))
    }

    fun loadHooks() = viewModelScope.launch {
        _state.value = _state.value.copy(hooks = ScreenState.Loading)
        delay(300)
        _state.value = _state.value.copy(hooks = ScreenState.Content(mockHooks()))
    }

    fun loadEvents() = viewModelScope.launch {
        _state.value = _state.value.copy(events = ScreenState.Loading)
        delay(300)
        _state.value = _state.value.copy(events = ScreenState.Content(mockEvents()))
    }

    fun loadSchedules() = viewModelScope.launch {
        _state.value = _state.value.copy(schedules = ScreenState.Loading)
        delay(300)
        _state.value = _state.value.copy(schedules = ScreenState.Content(mockSchedules()))
    }

    fun loadUiContributions() = viewModelScope.launch {
        _state.value = _state.value.copy(uiContributions = ScreenState.Loading)
        delay(300)
        _state.value = _state.value.copy(uiContributions = ScreenState.Content(mockUiContributions()))
    }

    fun loadRestrictedWebUis() = viewModelScope.launch {
        _state.value = _state.value.copy(restrictedWebUis = ScreenState.Loading)
        delay(300)
        _state.value = _state.value.copy(restrictedWebUis = ScreenState.Content(mockRestrictedWebUis()))
    }

    fun loadUpdates() = viewModelScope.launch {
        _state.value = _state.value.copy(updates = ScreenState.Loading)
        delay(300)
        _state.value = _state.value.copy(updates = ScreenState.Content(mockUpdates()))
    }

    fun loadMigrations() = viewModelScope.launch {
        _state.value = _state.value.copy(migrations = ScreenState.Loading)
        delay(300)
        _state.value = _state.value.copy(migrations = ScreenState.Content(mockMigrations()))
    }

    fun loadAudit() = viewModelScope.launch {
        _state.value = _state.value.copy(audit = ScreenState.Loading)
        delay(300)
        _state.value = _state.value.copy(audit = ScreenState.Content(mockAudit()))
    }

    fun loadPerformance() = viewModelScope.launch {
        _state.value = _state.value.copy(performance = ScreenState.Loading)
        delay(300)
        _state.value = _state.value.copy(performance = ScreenState.Content(mockPerformance()))
    }

    fun loadLogs() = viewModelScope.launch {
        _state.value = _state.value.copy(logs = ScreenState.Loading)
        delay(300)
        _state.value = _state.value.copy(logs = ScreenState.Content(mockLogs()))
    }

    fun loadFeatureFlags() = viewModelScope.launch {
        _state.value = _state.value.copy(featureFlags = ScreenState.Loading)
        delay(300)
        _state.value = _state.value.copy(featureFlags = ScreenState.Content(mockFeatureFlags()))
    }

    fun toggleSchedule(id: String) {
        val current = _state.value.schedules
        if (current is ScreenState.Content) {
            val updated = current.data.map {
                if (it.id == id) it.copy(enabled = !it.enabled) else it
            }
            _state.value = _state.value.copy(schedules = ScreenState.Content(updated))
        }
    }

    fun toggleFeatureFlag(key: String) {
        val current = _state.value.featureFlags
        if (current is ScreenState.Content) {
            val updated = current.data.map {
                if (it.key == key) it.copy(enabled = !it.enabled) else it
            }
            _state.value = _state.value.copy(featureFlags = ScreenState.Content(updated))
        }
    }

    fun resetFeatureFlag(key: String) {
        val current = _state.value.featureFlags
        if (current is ScreenState.Content) {
            val updated = current.data.map {
                if (it.key == key) it.copy(currentValue = it.defaultValue) else it
            }
            _state.value = _state.value.copy(featureFlags = ScreenState.Content(updated))
        }
    }

    private fun mockOverview(): List<DiagSummaryItem> = listOf(
        DiagSummaryItem("backend", "Amitia Backend", AmitiaStatusType.Running, "运行正常"),
        DiagSummaryItem("sqlite", "SQLite", AmitiaStatusType.Running, "已连接"),
        DiagSummaryItem("qdrant", "Qdrant", AmitiaStatusType.Running, "端口 6333"),
        DiagSummaryItem("surreal", "SurrealDB", AmitiaStatusType.Degraded, "响应延迟较高"),
        DiagSummaryItem("channel", "渠道", AmitiaStatusType.Running, "3 个渠道在线"),
        DiagSummaryItem("model", "模型", AmitiaStatusType.Running, "4 个模型可用"),
        DiagSummaryItem("extension", "扩展运行时", AmitiaStatusType.Running, "12 个扩展已加载"),
        DiagSummaryItem("task", "任务系统", AmitiaStatusType.Running, "5 个任务在运行")
    )

    private fun mockServices(): List<DiagServiceStatus> = listOf(
        DiagServiceStatus("Go 后端", 12345, AmitiaStatusType.Running, "08:30:00", 0, 8080, "健康", "1.0.0", "核心后端服务"),
        DiagServiceStatus("Qdrant", 12346, AmitiaStatusType.Running, "08:30:01", 0, 6333, "健康", "1.7.0", "向量数据库"),
        DiagServiceStatus("SurrealDB", 12347, AmitiaStatusType.Degraded, "08:30:02", 1, 8000, "延迟高", "1.5.0", "图数据库"),
        DiagServiceStatus("SQLite", null, AmitiaStatusType.Running, "08:30:00", 0, null, "内嵌", "3.45.0", "本地数据库")
    )

    private fun mockDatabases(): List<DiagDatabaseStatus> = listOf(
        DiagDatabaseStatus(DiagDbType.SQLite, "amitia.db", AmitiaStatusType.Running, "15", "内嵌", "健康", "12 MB"),
        DiagDatabaseStatus(DiagDbType.Qdrant, "memory_collection", AmitiaStatusType.Running, null, "localhost:6333", "健康", "45 MB", mapOf("集合" to "3", "向量数" to "12500")),
        DiagDatabaseStatus(DiagDbType.SurrealDB, "amitia", AmitiaStatusType.Degraded, null, "localhost:8000", "延迟 200ms", "8 MB", mapOf("表" to "5", "记录" to "3200"))
    )

    private fun mockTasks(): List<DiagTaskInfo> = listOf(
        DiagTaskInfo("t1", "记忆压缩", DiagTaskStatus.Running, "core", null, "30s", 0),
        DiagTaskInfo("t2", "情感分析", DiagTaskStatus.Pending, "emotion-skill", "10分钟后", "60s", 0),
        DiagTaskInfo("t3", "向量重建", DiagTaskStatus.Completed, "qdrant-mgr", null, "120s", 0),
        DiagTaskInfo("t4", "数据备份", DiagTaskStatus.Failed, "backup-ext", null, "300s", 2)
    )

    private fun mockTrustedServices(): List<DiagTrustedService> = listOf(
        DiagTrustedService("ts1", "文件管理服务", DiagLifecycle.Active, listOf("文件读写"), "正常", 0, 0),
        DiagTrustedService("ts2", "通知服务", DiagLifecycle.Active, listOf("通知发送"), "正常", 0, 0),
        DiagTrustedService("ts3", "网络代理", DiagLifecycle.Idle, listOf("网络访问"), "空闲", 1, 1)
    )

    private fun mockWasmModules(): List<DiagWasmModule> = listOf(
        DiagWasmModule("w1", "图像处理", "1.0.0", "64MB", "200ms", listOf("内存", "CPU"), AmitiaStatusType.Running),
        DiagWasmModule("w2", "文本编码", "0.8.0", "32MB", "50ms", listOf("内存"), AmitiaStatusType.Running),
        DiagWasmModule("w3", "音频解码", "1.1.0", "128MB", "100ms", listOf("内存", "CPU", "文件"), AmitiaStatusType.Idle)
    )

    private fun mockHooks(): List<DiagHook> = listOf(
        DiagHook("h1", "消息预处理", "message.beforeSend", "core", 1, "5ms", "记录并继续", true),
        DiagHook("h2", "权限校验", "permission.check", "auth-ext", 2, "3ms", "终止执行", true),
        DiagHook("h3", "日志记录", "event.after", "logger", 10, "1ms", "忽略错误", true),
        DiagHook("h4", "数据脱敏", "data.beforeStore", "privacy-ext", 5, "8ms", "使用原值", false)
    )

    private fun mockEvents(): List<DiagEvent> = listOf(
        DiagEvent("e1", "消息已发送", "chat-module", listOf("logger", "analytics"), "2分钟前", 0),
        DiagEvent("e2", "角色状态变更", "character-core", listOf("ui", "memory"), "5分钟前", 0),
        DiagEvent("e3", "模型切换", "model-router", listOf("analytics"), "1小时前", 1)
    )

    private fun mockSchedules(): List<DiagSchedule> = listOf(
        DiagSchedule("s1", "每 30 分钟", "30分钟后", "10:00:00", DiagMissedPolicy.Skip, true),
        DiagSchedule("s2", "每天 00:00", "明天 00:00", "今天 00:00", DiagMissedPolicy.ExecuteImmediately, true),
        DiagSchedule("s3", "每周一", "下周一", "上周一", DiagMissedPolicy.Queue, false)
    )

    private fun mockUiContributions(): List<DiagUiContribution> = listOf(
        DiagUiContribution("u1", "天气卡片扩展", "message.card", "2.0", listOf("网络访问"), true, emptyList()),
        DiagUiContribution("u2", "日程视图", "settings.tab", "1.1", listOf("存储读取"), false, listOf("主题不兼容")),
        DiagUiContribution("u3", "表情面板", "chat.panel", "1.0", emptyList(), true, emptyList())
    )

    private fun mockRestrictedWebUis(): List<DiagRestrictedWebUi> = listOf(
        DiagRestrictedWebUi("r1", "地图服务", "沙箱隔离", "maps.example.com", listOf("定位"), "default-src 'self'", AmitiaStatusType.Running),
        DiagRestrictedWebUi("r2", "支付页面", "严格隔离", "pay.example.com", listOf("网络访问"), "default-src 'none'", AmitiaStatusType.Idle)
    )

    private fun mockUpdates(): List<DiagUpdateInfo> = listOf(
        DiagUpdateInfo("u1", "Amitia Android", "1.0.0", "1.1.0", AmitiaStatusType.Pending, true, "1小时前"),
        DiagUpdateInfo("u2", "天气技能", "1.2.0", null, AmitiaStatusType.Running, true, "30分钟前"),
        DiagUpdateInfo("u3", "消息卡片", "0.8.0", "0.9.0", AmitiaStatusType.Pending, true, "2小时前")
    )

    private fun mockMigrations(): List<DiagMigration> = listOf(
        DiagMigration("m1", "user_prefs_v2", "v2", 1, "100%", "v1", null),
        DiagMigration("m2", "memory_index", "v3", 3, "98%", "v2", "批次 2 部分回滚")
    )

    private fun mockAudit(): List<DiagAuditEntry> = listOf(
        DiagAuditEntry("a1", DiagAuditType.Permission, "授予网络权限", "10:30:00", "user", "天气技能请求网络访问", DiagSeverity.Info),
        DiagAuditEntry("a2", DiagAuditType.ExtensionCall, "天气查询失败", "10:25:00", "weather-skill", "API 超时", DiagSeverity.Warning),
        DiagAuditEntry("a3", DiagAuditType.DataAccess, "读取记忆文件", "10:20:00", "memory-skill", "读取 5 条记忆", DiagSeverity.Info),
        DiagAuditEntry("a4", DiagAuditType.ConfigChange, "修改模型路由", "10:15:00", "user", "切换默认文本模型", DiagSeverity.Warning)
    )

    private fun mockPerformance(): List<DiagPerformanceMetric> = listOf(
        DiagPerformanceMetric("p1", "平均帧率", "59", "fps", ">= 60", "稳定", DiagMetricCategory.Fps),
        DiagPerformanceMetric("p2", "内存占用", "256", "MB", "< 512", "稳定", DiagMetricCategory.Memory),
        DiagPerformanceMetric("p3", "模型响应延迟", "320", "ms", "< 500", "上升", DiagMetricCategory.ModelLatency),
        DiagPerformanceMetric("p4", "数据库查询延迟", "5", "ms", "< 50", "稳定", DiagMetricCategory.DbLatency),
        DiagPerformanceMetric("p5", "重组次数", "12", "次", "< 20", "下降", DiagMetricCategory.Recomposition),
        DiagPerformanceMetric("p6", "CPU 占用", "15", "%", "< 80", "稳定", DiagMetricCategory.Runtime)
    )

    private fun mockLogs(): List<DiagLogEntry> = listOf(
        DiagLogEntry("l1", DiagLogLevel.Info, "Runtime", "10:30:00.123", "服务启动完成", true),
        DiagLogEntry("l2", DiagLogLevel.Warning, "Qdrant", "10:30:01.456", "连接池接近上限", true),
        DiagLogEntry("l3", DiagLogLevel.Error, "SurrealDB", "10:30:02.789", "查询超时 (200ms)", true),
        DiagLogEntry("l4", DiagLogLevel.Info, "Extension", "10:30:05.001", "扩展 weather-skill 已加载", true),
        DiagLogEntry("l5", DiagLogLevel.Debug, "Memory", "10:30:06.234", "记忆压缩任务完成", true),
        DiagLogEntry("l6", DiagLogLevel.Warning, "Channel", "10:30:10.567", "微信渠道重连", true),
        DiagLogEntry("l7", DiagLogLevel.Error, "Task", "10:30:15.890", "备份任务失败: 磁盘空间不足", true),
        DiagLogEntry("l8", DiagLogLevel.Info, "Model", "10:30:20.111", "模型路由切换到 GPT-4o", true)
    )

    private fun mockFeatureFlags(): List<DiagFeatureFlag> = listOf(
        DiagFeatureFlag("ff1", "新版记忆引擎", "实验功能", "true", "false", true, true),
        DiagFeatureFlag("ff2", "多模态输入", "实验功能", "false", "false", false, false),
        DiagFeatureFlag("ff3", "本地向量搜索", "灰度", "true", "true", true, true),
        DiagFeatureFlag("ff4", "高级审计日志", "正式", "true", "true", true, false),
        DiagFeatureFlag("ff5", "实验性 UI 动画", "实验功能", "false", "false", false, false)
    )
}
