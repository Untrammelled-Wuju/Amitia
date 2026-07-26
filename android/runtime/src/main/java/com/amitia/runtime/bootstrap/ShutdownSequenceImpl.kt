package com.amitia.runtime.bootstrap

import com.amitia.runtime.api.RuntimeEvent
import com.amitia.runtime.api.RuntimeState
import com.amitia.runtime.manager.RuntimeStateMachine
import com.amitia.runtime.process.LinuxProcessManager
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.withContext
import java.util.concurrent.atomic.AtomicBoolean
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class ShutdownSequenceImpl @Inject constructor(
    private val processManager: LinuxProcessManager,
    private val stateMachine: RuntimeStateMachine
) : ShutdownSequence {

    private val accepting = AtomicBoolean(true)

    override val isAccepting: Boolean
        get() = accepting.get()

    override fun rejectNewRequests() {
        accepting.set(false)
    }

    override fun allowNewRequests() {
        accepting.set(true)
    }

    override suspend fun execute(): Flow<ShutdownStep> = flow {
        emit(ShutdownStep.rejectRequests(0.05f, "停止接受新请求"))
        stateMachine.transition(RuntimeState.Stopping(stage = "reject"))
        rejectNewRequests()

        emit(ShutdownStep.stopBackend(0.3f))
        val backendResult = runCatching { processManager.stop(BACKEND_PROC, timeoutMs = 5000L) }
        backendResult.onFailure {
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.WARN,
                TAG,
                "Go 后端优雅停止失败,执行强制停止: ${it.message}"
            )
            runCatching { processManager.forceStop(BACKEND_PROC) }
        }

        emit(ShutdownStep.stopQdrant(0.6f))
        val qdrantResult = runCatching { processManager.stop(QDRANT_PROC, timeoutMs = 5000L) }
        qdrantResult.onFailure {
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.WARN,
                TAG,
                "Qdrant 优雅停止失败,执行强制停止: ${it.message}"
            )
            runCatching { processManager.forceStop(QDRANT_PROC) }
        }

        emit(ShutdownStep.stopSurreal(0.85f))
        val surrealResult = runCatching { processManager.stop(SURREAL_PROC, timeoutMs = 5000L) }
        surrealResult.onFailure {
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.WARN,
                TAG,
                "SurrealDB 优雅停止失败,执行强制停止: ${it.message}"
            )
            runCatching { processManager.forceStop(SURREAL_PROC) }
        }

        emit(ShutdownStep.flushState(0.95f))
        stateMachine.emitLog(
            RuntimeEvent.LogEmitted.Level.INFO,
            TAG,
            "Runtime 停止流程已刷新日志与状态"
        )

        stateMachine.transition(RuntimeState.Stopped)
        emit(ShutdownStep.complete())
    }

    override suspend fun stop(): Result<Unit> = withContext(Dispatchers.IO) {
        try {
            execute().collect { step ->
                stateMachine.emitLog(
                    RuntimeEvent.LogEmitted.Level.INFO,
                    TAG,
                    "Shutdown step: ${step.stageInfo.stage} - ${step.stageInfo.readableMessage}"
                )
            }
            allowNewRequests()
            Result.success(Unit)
        } catch (e: Exception) {
            stateMachine.emitError(
                error = "停止失败: ${e.message}",
                retryable = true,
                requiresUserAction = false,
                cause = e
            )
            Result.failure(e)
        }
    }

    companion object {
        private const val TAG = "Shutdown"
        private const val SURREAL_PROC = "surrealdb"
        private const val QDRANT_PROC = "qdrant"
        private const val BACKEND_PROC = "amitia-backend"
    }
}
