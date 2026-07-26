package com.amitia.runtime.bootstrap

import com.amitia.runtime.api.RuntimeStageInfo
import kotlinx.coroutines.flow.Flow

interface ShutdownSequence {

    val isAccepting: Boolean

    fun rejectNewRequests()

    fun allowNewRequests()

    suspend fun execute(): Flow<ShutdownStep>

    suspend fun stop(): Result<Unit>
}

sealed class ShutdownStep {

    abstract val stageInfo: RuntimeStageInfo

    data class RejectRequests(override val stageInfo: RuntimeStageInfo) : ShutdownStep()

    data class StopBackend(
        override val stageInfo: RuntimeStageInfo,
        val exitCode: Int? = null
    ) : ShutdownStep()

    data class StopQdrant(
        override val stageInfo: RuntimeStageInfo,
        val exitCode: Int? = null
    ) : ShutdownStep()

    data class StopSurreal(
        override val stageInfo: RuntimeStageInfo,
        val exitCode: Int? = null
    ) : ShutdownStep()

    data class FlushState(override val stageInfo: RuntimeStageInfo) : ShutdownStep()

    data class Complete(override val stageInfo: RuntimeStageInfo) : ShutdownStep()

    companion object {
        fun rejectRequests(progress: Float, message: String): RejectRequests = RejectRequests(
            stageInfo = RuntimeStageInfo(
                stage = "RejectRequests",
                progress = progress,
                readableMessage = message
            )
        )

        fun stopBackend(progress: Float, exitCode: Int? = null): StopBackend = StopBackend(
            stageInfo = RuntimeStageInfo(
                stage = "StopBackend",
                progress = progress,
                readableMessage = "停止 Go 后端"
            ),
            exitCode = exitCode
        )

        fun stopQdrant(progress: Float, exitCode: Int? = null): StopQdrant = StopQdrant(
            stageInfo = RuntimeStageInfo(
                stage = "StopQdrant",
                progress = progress,
                readableMessage = "停止 Qdrant"
            ),
            exitCode = exitCode
        )

        fun stopSurreal(progress: Float, exitCode: Int? = null): StopSurreal = StopSurreal(
            stageInfo = RuntimeStageInfo(
                stage = "StopSurreal",
                progress = progress,
                readableMessage = "停止 SurrealDB"
            ),
            exitCode = exitCode
        )

        fun flushState(progress: Float): FlushState = FlushState(
            stageInfo = RuntimeStageInfo(
                stage = "FlushState",
                progress = progress,
                readableMessage = "刷新日志与状态"
            )
        )

        fun complete(): Complete = Complete(
            stageInfo = RuntimeStageInfo(
                stage = "ShutdownComplete",
                progress = 1f,
                readableMessage = "Runtime 已停止"
            )
        )
    }
}
