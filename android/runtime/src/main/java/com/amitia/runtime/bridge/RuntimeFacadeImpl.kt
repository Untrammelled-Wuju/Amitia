package com.amitia.runtime.bridge

import com.amitia.runtime.api.RuntimeEvent
import com.amitia.runtime.api.RuntimeFacade
import com.amitia.runtime.api.RuntimeServices
import com.amitia.runtime.api.RuntimeState
import com.amitia.runtime.bootstrap.BootstrapSequence
import com.amitia.runtime.linux.LinuxRootfsManager
import com.amitia.runtime.manager.RuntimeStateMachine
import com.amitia.runtime.process.LinuxProcessManager
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class RuntimeFacadeImpl @Inject constructor(
    private val stateMachine: RuntimeStateMachine,
    private val bootstrapSequence: BootstrapSequence,
    private val rootfsManager: LinuxRootfsManager,
    private val processManager: LinuxProcessManager
) : RuntimeFacade {

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)

    private val _services = MutableStateFlow(RuntimeServices.ALL_STOPPED)
    override val services: StateFlow<RuntimeServices> = _services.asStateFlow()

    private val _uptimeMs = MutableStateFlow(0L)
    override val uptimeMs: StateFlow<Long> = _uptimeMs.asStateFlow()

    private var runningSinceMs: Long = 0L

    override val state: StateFlow<RuntimeState>
        get() = stateMachine.state

    override val events: kotlinx.coroutines.flow.Flow<RuntimeEvent>
        get() = stateMachine.events

    init {
        scope.launch {
            stateMachine.state.collect { s ->
                when (s) {
                    is RuntimeState.Running -> {
                        if (runningSinceMs == 0L) {
                            runningSinceMs = System.currentTimeMillis()
                        }
                        _services.value = s.services
                    }
                    is RuntimeState.Degraded -> {
                        _services.value = s.services
                    }
                    is RuntimeState.Stopped, is RuntimeState.Failed, is RuntimeState.NotInstalled -> {
                        runningSinceMs = 0L
                        _uptimeMs.value = 0L
                        _services.value = RuntimeServices.ALL_STOPPED
                    }
                    else -> Unit
                }
                if (runningSinceMs > 0L) {
                    _uptimeMs.value = System.currentTimeMillis() - runningSinceMs
                }
            }
        }
    }

    override suspend fun start(): Result<RuntimeServices> {
        return bootstrapSequence.start { }
    }

    override suspend fun stop(): Result<Unit> {
        return bootstrapSequence.stop { }
    }

    override suspend fun restart(): Result<RuntimeServices> {
        return bootstrapSequence.restart()
    }

    override suspend fun repair(): Result<Unit> {
        return bootstrapSequence.repair()
    }

    override suspend fun refresh() {
        stateMachine.emitProgress(stateMachine.current)
    }

    override suspend fun update(): Result<Unit> {
        return try {
            stateMachine.transition(
                RuntimeState.Updating(progressValue = 0f, message = "升级 RootFS")
            )
            val flow = rootfsManager.upgrade()
            flow.collect { progress ->
                stateMachine.emitProgress(
                    RuntimeState.Updating(
                        progressValue = progress.percent,
                        message = progress.message
                    )
                )
            }
            stateMachine.transition(RuntimeState.Installed)
            Result.success(Unit)
        } catch (e: Exception) {
            stateMachine.transition(
                RuntimeState.Failed(
                    errorMessage = e.message ?: "升级失败",
                    retryable = true,
                    requiresUserAction = false
                )
            )
            Result.failure(e)
        }
    }

    override suspend fun cleanup(confirm: Boolean): Result<Unit> {
        return rootfsManager.cleanup(requireConfirmation = !confirm)
    }

    override fun snapshot(): RuntimeFacade.RuntimeSnapshot {
        val s = stateMachine.current
        val components = listOf(BACKEND_PROC, QDRANT_PROC, SURREALDB_PROC)
        val crashCounts = components.associateWith { processManager.crashCount(it) }
        val lastErrors = components.associateWith { processManager.lastExitReason(it) }
        return RuntimeFacade.RuntimeSnapshot(
            state = s,
            services = _services.value,
            uptimeMs = _uptimeMs.value,
            rootfsVersion = null,
            manifestVersion = null,
            crashCounts = crashCounts,
            lastErrors = lastErrors
        )
    }

    companion object {
        private const val BACKEND_PROC = "amitia-backend"
        private const val QDRANT_PROC = "qdrant"
        private const val SURREALDB_PROC = "surrealdb"
    }
}
