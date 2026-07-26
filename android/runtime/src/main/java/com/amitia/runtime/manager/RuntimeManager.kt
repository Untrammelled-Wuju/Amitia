package com.amitia.runtime.manager

import com.amitia.runtime.api.RuntimeEvent
import com.amitia.runtime.api.RuntimeFacade
import com.amitia.runtime.api.RuntimeServices
import com.amitia.runtime.api.RuntimeState
import com.amitia.runtime.bootstrap.BootstrapSequence
import com.amitia.runtime.linux.LinuxRootfsManager
import com.amitia.runtime.linux.RootfsInstallPhase
import com.amitia.runtime.process.LinuxProcessManager
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import javax.inject.Inject
import javax.inject.Singleton

interface RuntimeManager : RuntimeFacade {

    override val state: StateFlow<RuntimeState>

    override val events: Flow<RuntimeEvent>

    override val services: StateFlow<RuntimeServices>

    override val uptimeMs: StateFlow<Long>

    fun observeState(): StateFlow<RuntimeState>

    override fun snapshot(): RuntimeFacade.RuntimeSnapshot
}

@Singleton
class RuntimeManagerImpl @Inject constructor(
    private val bootstrap: BootstrapSequence,
    private val stateMachine: RuntimeStateMachine,
    private val rootfsManager: LinuxRootfsManager,
    private val processManager: LinuxProcessManager
) : RuntimeManager {

    private val mutex = Mutex()

    private val _services = MutableStateFlow(RuntimeServices.ALL_STOPPED)
    override val services: StateFlow<RuntimeServices> = _services.asStateFlow()

    private val _uptimeMs = MutableStateFlow(0L)
    override val uptimeMs: StateFlow<Long> = _uptimeMs.asStateFlow()

    private var runningSinceMs: Long = 0L

    override val state: StateFlow<RuntimeState>
        get() = stateMachine.state

    override val events: Flow<RuntimeEvent>
        get() = stateMachine.events

    override fun observeState(): StateFlow<RuntimeState> = stateMachine.state

    override suspend fun start(): Result<RuntimeServices> = mutex.withLock {
        val result = bootstrap.start { }
        result.onSuccess { services ->
            _services.value = services
            runningSinceMs = System.currentTimeMillis()
            _uptimeMs.value = 0L
        }
        result
    }

    override suspend fun stop(): Result<Unit> = mutex.withLock {
        val result = bootstrap.stop { }
        result.onSuccess {
            _services.value = RuntimeServices.ALL_STOPPED
            _uptimeMs.value = 0L
            runningSinceMs = 0L
        }
        result
    }

    override suspend fun restart(): Result<RuntimeServices> = mutex.withLock {
        val result = bootstrap.restart()
        result.onSuccess { services ->
            _services.value = services
            runningSinceMs = System.currentTimeMillis()
            _uptimeMs.value = 0L
        }
        result
    }

    override suspend fun repair(): Result<Unit> = mutex.withLock {
        val result = bootstrap.repair()
        result.onFailure {
            _services.value = RuntimeServices.ALL_STOPPED
        }
        result
    }

    override suspend fun refresh() {
        val current = stateMachine.current
        if (current is RuntimeState.Running) {
            val since = if (runningSinceMs > 0L) runningSinceMs else System.currentTimeMillis()
            _uptimeMs.value = System.currentTimeMillis() - since
        }
    }

    override suspend fun update(): Result<Unit> {
        stateMachine.transition(
            RuntimeState.Updating(progressValue = 0f, message = "正在升级 RootFS")
        )
        var failed: String? = null
        try {
            rootfsManager.upgrade().collect { p ->
                stateMachine.emitProgress(
                    RuntimeState.Updating(progressValue = p.percent, message = p.message)
                )
                if (p.phase == RootfsInstallPhase.FAILED) {
                    failed = p.error ?: p.message
                }
            }
        } catch (e: Exception) {
            failed = e.message ?: "RootFS 升级异常"
        }
        if (failed != null) {
            stateMachine.transition(
                RuntimeState.Failed(
                    errorMessage = failed,
                    retryable = true,
                    requiresUserAction = false
                )
            )
            return Result.failure(IllegalStateException(failed))
        }
        stateMachine.transition(RuntimeState.Installed)
        return Result.success(Unit)
    }

    override suspend fun cleanup(confirm: Boolean): Result<Unit> {
        val result = rootfsManager.cleanup(requireConfirmation = !confirm)
        result.onSuccess {
            if (confirm) {
                _services.value = RuntimeServices.ALL_STOPPED
                stateMachine.transition(RuntimeState.NotInstalled)
            }
        }
        return result
    }

    override fun snapshot(): RuntimeFacade.RuntimeSnapshot {
        val current = stateMachine.current
        val services = if (current is RuntimeState.Running) current.services
            else if (current is RuntimeState.Degraded) current.services
            else _services.value
        val crashCounts = mapOf(
            "amitia-backend" to processManager.crashCount("amitia-backend"),
            "qdrant" to processManager.crashCount("qdrant"),
            "surrealdb" to processManager.crashCount("surrealdb")
        )
        val lastErrors = mapOf(
            "amitia-backend" to processManager.lastExitReason("amitia-backend"),
            "qdrant" to processManager.lastExitReason("qdrant"),
            "surrealdb" to processManager.lastExitReason("surrealdb")
        )
        return RuntimeFacade.RuntimeSnapshot(
            state = current,
            services = services,
            uptimeMs = _uptimeMs.value,
            rootfsVersion = null,
            manifestVersion = null,
            crashCounts = crashCounts,
            lastErrors = lastErrors
        )
    }
}
