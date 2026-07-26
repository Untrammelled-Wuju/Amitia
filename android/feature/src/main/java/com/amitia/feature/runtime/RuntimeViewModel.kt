package com.amitia.feature.runtime

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.network.endpoint.RuntimeEndpointProvider
import com.amitia.platform.notification.ProactiveMessageObserver
import com.amitia.runtime.api.RuntimeEvent
import com.amitia.runtime.api.RuntimeServices
import com.amitia.runtime.api.RuntimeState
import com.amitia.runtime.api.ServiceState
import com.amitia.runtime.manager.RuntimeDirectories
import com.amitia.runtime.manager.RuntimeManager
import dagger.hilt.android.lifecycle.HiltViewModel
import java.io.File
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

@HiltViewModel
class RuntimeViewModel @Inject constructor(
    private val runtimeManager: RuntimeManager,
    private val runtimeDirectories: RuntimeDirectories,
    private val endpointProvider: RuntimeEndpointProvider,
    private val proactiveMessageObserver: ProactiveMessageObserver
) : ViewModel() {

    private val _state = MutableStateFlow(RuntimeUiState())
    val state: StateFlow<RuntimeUiState> = _state.asStateFlow()

    private val logBuffer = ArrayDeque<LogEntry>()
    private val maxLogs = 200
    private var lastUptimeMs: Long = 0L
    private var observerActive: Boolean = false

    init {
        observeRuntimeState()
        observeRuntimeEvents()
        refresh()
    }

    private fun observeRuntimeState() {
        viewModelScope.launch {
            runtimeManager.observeState().collect { rs ->
                lastUptimeMs = if (rs is RuntimeState.Running) rs.uptimeMs else lastUptimeMs
                _state.value = _state.value.copy(
                    runtimeState = rs,
                    services = (rs as? RuntimeState.Running)?.services
                        ?: (rs as? RuntimeState.Degraded)?.services
                        ?: RuntimeServices.ALL_STOPPED,
                    uptimeMs = lastUptimeMs,
                    lastError = rs.error,
                    ports = buildPorts()
                )
                syncProactiveObserver(rs)
            }
        }
    }

    private fun syncProactiveObserver(state: RuntimeState) {
        val shouldRun = state is RuntimeState.Running || state is RuntimeState.Degraded
        if (shouldRun && !observerActive) {
            observerActive = true
            viewModelScope.launch {
                runCatching { proactiveMessageObserver.start() }
                    .onFailure { appendLog(LogEntry("WARN", "proactive", "主动消息监听启动失败：${it.message}")) }
            }
        } else if (!shouldRun && observerActive) {
            observerActive = false
            viewModelScope.launch {
                runCatching { proactiveMessageObserver.stop() }
            }
        }
    }

    private fun observeRuntimeEvents() {
        viewModelScope.launch {
            runtimeManager.observeState().collect { rs ->
                if (rs is RuntimeState.Failed) {
                    appendLog(LogEntry(level = "ERROR", tag = "runtime", message = rs.errorMessage))
                } else {
                    appendLog(
                        LogEntry(
                            level = "INFO",
                            tag = "runtime",
                            message = rs.readableMessage
                        )
                    )
                }
            }
        }
    }

    fun refresh() {
        viewModelScope.launch {
            runtimeManager.refresh()
            _state.value = _state.value.copy(
                dataUsage = computeDataUsage(),
                rootfsVersion = readRootfsVersion()
            )
        }
    }

    fun start() {
        viewModelScope.launch {
            runCatching { runtimeManager.start() }
                .onFailure { e ->
                    _state.value = _state.value.copy(lastError = e.message ?: "启动失败")
                }
        }
    }

    fun stop() {
        viewModelScope.launch {
            runCatching { runtimeManager.stop() }
                .onFailure { e ->
                    _state.value = _state.value.copy(lastError = e.message ?: "停止失败")
                }
        }
    }

    fun restart() {
        viewModelScope.launch {
            runCatching { runtimeManager.restart() }
                .onFailure { e ->
                    _state.value = _state.value.copy(lastError = e.message ?: "重启失败")
                }
        }
    }

    fun repair() {
        viewModelScope.launch {
            runCatching { runtimeManager.repair() }
                .onFailure { e ->
                    _state.value = _state.value.copy(lastError = e.message ?: "修复失败")
                }
        }
    }

    fun update() {
        viewModelScope.launch {
            runtimeManager.observeState().let { _ ->
                appendLog(LogEntry(level = "INFO", tag = "update", message = "更新已请求"))
            }
        }
    }

    fun exportDiagnostics(onExported: (File) -> Unit) {
        viewModelScope.launch {
            val file = DiagnosticsExporter.export(
                directory = runtimeDirectories.tmpDir(),
                state = _state.value,
                logs = logBuffer.toList()
            )
            onExported(file)
        }
    }

    fun cleanup(confirmed: Boolean) {
        if (!confirmed) {
            _state.value = _state.value.copy(lastError = "清理需要确认")
            return
        }
        viewModelScope.launch {
            runCatching {
                val rootfs = runtimeDirectories.rootfsDir()
                if (rootfs.exists()) rootfs.deleteRecursively()
                runtimeDirectories.ensureAllCreated()
                appendLog(LogEntry(level = "WARN", tag = "cleanup", message = "已清理 rootfs，保留 amitia-data"))
            }.onFailure { e ->
                _state.value = _state.value.copy(lastError = e.message ?: "清理失败")
            }
        }
    }

    fun backup() {
        viewModelScope.launch {
            runCatching {
                val backups = runtimeDirectories.backupsDir()
                if (!backups.exists()) backups.mkdirs()
                appendLog(LogEntry(level = "INFO", tag = "backup", message = "备份已请求"))
            }.onFailure { e ->
                _state.value = _state.value.copy(lastError = e.message ?: "备份失败")
            }
        }
    }

    fun restore() {
        viewModelScope.launch {
            runCatching {
                appendLog(LogEntry(level = "WARN", tag = "restore", message = "恢复已请求"))
            }.onFailure { e ->
                _state.value = _state.value.copy(lastError = e.message ?: "恢复失败")
            }
        }
    }

    private fun appendLog(entry: LogEntry) {
        logBuffer.addLast(entry)
        while (logBuffer.size > maxLogs) logBuffer.removeFirst()
        _state.value = _state.value.copy(logs = logBuffer.toList())
    }

    private fun buildPorts(): PortsInfo {
        val endpoint = endpointProvider.currentEndpoint.value
        return when (endpoint) {
            is com.amitia.core.network.endpoint.RuntimeEndpoint.Local -> PortsInfo(
                backend = endpoint.port,
                qdrant = com.amitia.core.common.Constants.QDRANT_PORT,
                surrealdb = com.amitia.core.common.Constants.SURREALDB_PORT
            )
            is com.amitia.core.network.endpoint.RuntimeEndpoint.Remote -> PortsInfo(
                backend = null,
                qdrant = null,
                surrealdb = null
            )
        }
    }

    private fun computeDataUsage(): DataInfo {
        val dataRoot = runtimeDirectories.amitiaDataRoot()
        val rootfs = runtimeDirectories.rootfsDir()
        return DataInfo(
            dataBytes = dirSize(dataRoot),
            rootfsBytes = dirSize(rootfs)
        )
    }

    private fun readRootfsVersion(): String? {
        val versionFile = File(runtimeDirectories.versionsDir(), "current.txt")
        return runCatching { versionFile.readText().trim() }.getOrNull()
    }

    private fun dirSize(file: File): Long {
        if (!file.exists()) return 0L
        if (file.isFile) return file.length()
        return file.walkTopDown().filter { it.isFile }.sumOf { it.length() }
    }

    override fun onCleared() {
        super.onCleared()
        logBuffer.clear()
    }
}

data class RuntimeUiState(
    val runtimeState: RuntimeState = RuntimeState.NotInstalled,
    val services: RuntimeServices = RuntimeServices.ALL_STOPPED,
    val ports: PortsInfo = PortsInfo(),
    val uptimeMs: Long = 0L,
    val memoryUsage: MemoryInfo? = null,
    val dataUsage: DataInfo = DataInfo(),
    val logs: List<LogEntry> = emptyList(),
    val lastError: String? = null,
    val rootfsVersion: String? = null,
    val actions: List<RuntimeAction> = emptyList()
)

data class PortsInfo(
    val backend: Int? = null,
    val qdrant: Int? = null,
    val surrealdb: Int? = null
)

data class MemoryInfo(
    val usedMb: Long,
    val totalMb: Long
)

data class DataInfo(
    val dataBytes: Long = 0L,
    val rootfsBytes: Long = 0L
)

data class LogEntry(
    val level: String,
    val tag: String,
    val message: String,
    val timestampMs: Long = System.currentTimeMillis()
)

enum class RuntimeAction {
    START, STOP, RESTART, REPAIR, UPDATE, EXPORT_DIAGNOSTICS, CLEANUP, BACKUP, RESTORE
}
