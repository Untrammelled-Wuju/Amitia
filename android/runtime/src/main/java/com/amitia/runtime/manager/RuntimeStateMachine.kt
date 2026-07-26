package com.amitia.runtime.manager

import com.amitia.runtime.api.RuntimeEvent
import com.amitia.runtime.api.RuntimeSnapshot
import com.amitia.runtime.api.RuntimeStageInfo
import com.amitia.runtime.api.RuntimeState
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class RuntimeStateMachine @Inject constructor() {

    private val _state = MutableStateFlow<RuntimeState>(RuntimeState.NotInstalled)
    val state: StateFlow<RuntimeState> = _state.asStateFlow()

    private val _snapshot = MutableStateFlow<RuntimeSnapshot>(RuntimeSnapshot.EMPTY)
    val snapshot: StateFlow<RuntimeSnapshot> = _snapshot.asStateFlow()

    private val _events = MutableSharedFlow<RuntimeEvent>(
        replay = 64,
        extraBufferCapacity = 64
    )
    val events: Flow<RuntimeEvent> = _events.asSharedFlow()

    private val mutex = Mutex()

    private var lastStartTime: Long? = null
    private var lastStopReason: String? = null
    private var crashCount: Int = 0

    val current: RuntimeState
        get() = _state.value

    val currentSnapshot: RuntimeSnapshot
        get() = _snapshot.value

    fun observe(): Flow<RuntimeState> = state

    fun observeEvents(): Flow<RuntimeEvent> = events

    fun observeSnapshot(): StateFlow<RuntimeSnapshot> = snapshot

    suspend fun transition(target: RuntimeState): Result<RuntimeState> {
        mutex.withLock {
            val from = _state.value
            if (!isTransitionValid(from, target)) {
                val error = IllegalStateException(
                    "非法状态转换: ${from.phase} -> ${target.phase}"
                )
                _events.tryEmit(
                    RuntimeEvent.ErrorOccurred(
                        error = error.message ?: "非法状态转换",
                        retryable = false,
                        requiresUserAction = true,
                        cause = error,
                        timestampMs = System.currentTimeMillis()
                    )
                )
                return Result.failure(error)
            }
            _state.value = target
            updateSnapshotForTransition(from, target)
            _events.tryEmit(
                RuntimeEvent.StateChanged(
                    from = from,
                    to = target,
                    timestampMs = System.currentTimeMillis()
                )
            )
            _events.tryEmit(
                RuntimeEvent.ProgressUpdated(
                    stage = target.toStage(),
                    timestampMs = System.currentTimeMillis()
                )
            )
            return Result.success(target)
        }
    }

    fun emitRuntimeStateChanged(from: RuntimeState, to: RuntimeState, reason: String? = null) {
        _events.tryEmit(
            RuntimeEvent.RuntimeStateChanged(
                from = from,
                to = to,
                reason = reason,
                timestampMs = System.currentTimeMillis()
            )
        )
    }

    fun emitProgress(stage: RuntimeState) {
        _events.tryEmit(
            RuntimeEvent.ProgressUpdated(
                stage = stage.toStage(),
                timestampMs = System.currentTimeMillis()
            )
        )
        updateSnapshotStage(RuntimeStageInfo.fromStage(stage.toStage()))
    }

    fun emitLog(level: RuntimeEvent.LogEmitted.Level, tag: String, message: String) {
        _events.tryEmit(
            RuntimeEvent.LogEmitted(
                level = level,
                tag = tag,
                message = message,
                timestampMs = System.currentTimeMillis()
            )
        )
    }

    fun emitServiceHealth(serviceName: String, serviceState: com.amitia.runtime.api.ServiceState) {
        _events.tryEmit(
            RuntimeEvent.ServiceHealthChanged(
                serviceName = serviceName,
                state = serviceState,
                timestampMs = System.currentTimeMillis()
            )
        )
        when (serviceState) {
            is com.amitia.runtime.api.ServiceState.Healthy ->
                _events.tryEmit(
                    RuntimeEvent.ServiceHealthy(
                        serviceName = serviceName,
                        port = serviceState.port,
                        timestampMs = System.currentTimeMillis()
                    )
                )
            is com.amitia.runtime.api.ServiceState.Unhealthy ->
                _events.tryEmit(
                    RuntimeEvent.ServiceFailed(
                        serviceName = serviceName,
                        error = serviceState.reason,
                        retryable = true,
                        timestampMs = System.currentTimeMillis()
                    )
                )
            else -> Unit
        }
    }

    fun emitError(error: String, retryable: Boolean, requiresUserAction: Boolean, cause: Throwable? = null) {
        _events.tryEmit(
            RuntimeEvent.ErrorOccurred(
                error = error,
                retryable = retryable,
                requiresUserAction = requiresUserAction,
                cause = cause,
                timestampMs = System.currentTimeMillis()
            )
        )
    }

    fun emitRootfsProgress(progress: Float, message: String, bytesProcessed: Long, bytesTotal: Long) {
        _events.tryEmit(
            RuntimeEvent.RootfsInstallProgress(
                progress = progress,
                message = message,
                bytesProcessed = bytesProcessed,
                bytesTotal = bytesTotal,
                timestampMs = System.currentTimeMillis()
            )
        )
    }

    fun emitServiceStarting(serviceName: String, port: Int) {
        _events.tryEmit(
            RuntimeEvent.ServiceStarting(
                serviceName = serviceName,
                port = port,
                timestampMs = System.currentTimeMillis()
            )
        )
    }

    fun recordStart(timeMs: Long) {
        lastStartTime = timeMs
        lastStopReason = null
        refreshSnapshot()
    }

    fun recordStop(reason: String) {
        lastStopReason = reason
        refreshSnapshot()
    }

    fun recordCrash() {
        crashCount += 1
        refreshSnapshot()
    }

    fun setPorts(ports: Map<String, Int>) {
        refreshSnapshot(ports = ports)
    }

    private fun updateSnapshotForTransition(from: RuntimeState, to: RuntimeState) {
        when (to) {
            is RuntimeState.Running -> recordStart(System.currentTimeMillis())
            is RuntimeState.Stopped -> recordStop("user-initiated")
            is RuntimeState.Failed -> {
                if (from is RuntimeState.Running || from is RuntimeState.Degraded) {
                    recordCrash()
                }
            }
            else -> Unit
        }
        refreshSnapshot()
    }

    private fun updateSnapshotStage(stageInfo: RuntimeStageInfo) {
        val current = _snapshot.value
        _snapshot.value = current.withStage(stageInfo)
    }

    private fun refreshSnapshot(ports: Map<String, Int>? = null) {
        val state = _state.value
        val stageInfo = RuntimeStageInfo.fromStage(state.toStage())
        val baseSnapshot = RuntimeSnapshot(
            state = state,
            stageInfo = stageInfo,
            ports = ports ?: _snapshot.value.ports,
            lastStartTime = lastStartTime,
            lastStopReason = lastStopReason,
            crashCount = crashCount,
            capturedAtMs = System.currentTimeMillis()
        )
        _snapshot.value = baseSnapshot
    }

    private fun isTransitionValid(from: RuntimeState, to: RuntimeState): Boolean {
        if (from.phase == to.phase && from == to) return true
        return when (from.phase) {
            RuntimeState.Phase.IDLE -> to.phase in setOf(
                RuntimeState.Phase.INSTALLING,
                RuntimeState.Phase.INSTALLED,
                RuntimeState.Phase.STARTING
            )
            RuntimeState.Phase.INSTALLING -> to.phase in setOf(
                RuntimeState.Phase.INSTALLED,
                RuntimeState.Phase.FAILED,
                RuntimeState.Phase.IDLE
            )
            RuntimeState.Phase.INSTALLED -> to.phase in setOf(
                RuntimeState.Phase.STARTING,
                RuntimeState.Phase.UPDATING,
                RuntimeState.Phase.FAILED,
                RuntimeState.Phase.IDLE
            )
            RuntimeState.Phase.STARTING -> to.phase in setOf(
                RuntimeState.Phase.RUNNING,
                RuntimeState.Phase.DEGRADED,
                RuntimeState.Phase.FAILED,
                RuntimeState.Phase.STOPPING
            )
            RuntimeState.Phase.RUNNING -> to.phase in setOf(
                RuntimeState.Phase.STOPPING,
                RuntimeState.Phase.DEGRADED,
                RuntimeState.Phase.FAILED
            )
            RuntimeState.Phase.DEGRADED -> to.phase in setOf(
                RuntimeState.Phase.RUNNING,
                RuntimeState.Phase.FAILED,
                RuntimeState.Phase.STOPPING
            )
            RuntimeState.Phase.STOPPING -> to.phase in setOf(
                RuntimeState.Phase.STOPPED,
                RuntimeState.Phase.FAILED
            )
            RuntimeState.Phase.STOPPED -> to.phase in setOf(
                RuntimeState.Phase.STARTING,
                RuntimeState.Phase.IDLE
            )
            RuntimeState.Phase.FAILED -> to.phase in setOf(
                RuntimeState.Phase.STARTING,
                RuntimeState.Phase.STOPPED,
                RuntimeState.Phase.IDLE,
                RuntimeState.Phase.INSTALLING
            )
            RuntimeState.Phase.UPDATING -> to.phase in setOf(
                RuntimeState.Phase.INSTALLED,
                RuntimeState.Phase.FAILED
            )
        }
    }
}
