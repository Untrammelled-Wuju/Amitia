package com.amitia.runtime.api

sealed interface RuntimeState {

    val phase: Phase
    val progress: Float
    val readableMessage: String
    val error: String?
    val retryable: Boolean
    val requiresUserAction: Boolean

    enum class Phase {
        IDLE, INSTALLING, INSTALLED, STARTING, RUNNING, DEGRADED,
        STOPPING, STOPPED, FAILED, UPDATING
    }

    fun toStage(): RuntimeStage

    data object NotInstalled : RuntimeState {
        override val phase: Phase = Phase.IDLE
        override val progress: Float = 0f
        override val readableMessage: String = "运行时未安装"
        override val error: String? = null
        override val retryable: Boolean = true
        override val requiresUserAction: Boolean = true

        override fun toStage(): RuntimeStage = RuntimeStage(
            stage = "NotInstalled",
            progress = progress,
            message = readableMessage,
            error = error,
            retryable = retryable,
            requiresUserAction = requiresUserAction
        )
    }

    data class Installing(
        val progressValue: Float,
        val message: String
    ) : RuntimeState {
        override val phase: Phase = Phase.INSTALLING
        override val progress: Float = progressValue
        override val readableMessage: String = message
        override val error: String? = null
        override val retryable: Boolean = true
        override val requiresUserAction: Boolean = false

        override fun toStage(): RuntimeStage = RuntimeStage(
            stage = "Installing",
            progress = progressValue,
            message = message,
            error = null,
            retryable = true,
            requiresUserAction = false
        )
    }

    data object Installed : RuntimeState {
        override val phase: Phase = Phase.INSTALLED
        override val progress: Float = 1f
        override val readableMessage: String = "运行时已就绪"
        override val error: String? = null
        override val retryable: Boolean = true
        override val requiresUserAction: Boolean = false

        override fun toStage(): RuntimeStage = RuntimeStage(
            stage = "Installed",
            progress = 1f,
            message = readableMessage,
            error = null,
            retryable = true,
            requiresUserAction = false
        )
    }

    data class Starting(
        val stage: String,
        val progressValue: Float
    ) : RuntimeState {
        override val phase: Phase = Phase.STARTING
        override val progress: Float = progressValue
        override val readableMessage: String = "正在启动运行时: $stage"
        override val error: String? = null
        override val retryable: Boolean = true
        override val requiresUserAction: Boolean = false

        override fun toStage(): RuntimeStage = RuntimeStage(
            stage = stage,
            progress = progressValue,
            message = readableMessage,
            error = null,
            retryable = true,
            requiresUserAction = false
        )
    }

    data class Running(
        val uptimeMs: Long,
        val services: RuntimeServices
    ) : RuntimeState {
        override val phase: Phase = Phase.RUNNING
        override val progress: Float = 1f
        override val readableMessage: String = "运行时已启动"
        override val error: String? = null
        override val retryable: Boolean = true
        override val requiresUserAction: Boolean = false

        override fun toStage(): RuntimeStage = RuntimeStage(
            stage = "Running",
            progress = 1f,
            message = readableMessage,
            error = null,
            retryable = true,
            requiresUserAction = false
        )
    }

    data class Degraded(
        val reason: String,
        val services: RuntimeServices
    ) : RuntimeState {
        override val phase: Phase = Phase.DEGRADED
        override val progress: Float = 1f
        override val readableMessage: String = "运行时降级: $reason"
        override val error: String? = reason
        override val retryable: Boolean = true
        override val requiresUserAction: Boolean = false

        override fun toStage(): RuntimeStage = RuntimeStage(
            stage = "Degraded",
            progress = 1f,
            message = readableMessage,
            error = reason,
            retryable = true,
            requiresUserAction = false
        )
    }

    data class Stopping(
        val stage: String
    ) : RuntimeState {
        override val phase: Phase = Phase.STOPPING
        override val progress: Float = 0.5f
        override val readableMessage: String = "正在停止运行时: $stage"
        override val error: String? = null
        override val retryable: Boolean = false
        override val requiresUserAction: Boolean = false

        override fun toStage(): RuntimeStage = RuntimeStage(
            stage = stage,
            progress = 0.5f,
            message = readableMessage,
            error = null,
            retryable = false,
            requiresUserAction = false
        )
    }

    data object Stopped : RuntimeState {
        override val phase: Phase = Phase.STOPPED
        override val progress: Float = 0f
        override val readableMessage: String = "运行时已停止"
        override val error: String? = null
        override val retryable: Boolean = true
        override val requiresUserAction: Boolean = false

        override fun toStage(): RuntimeStage = RuntimeStage(
            stage = "Stopped",
            progress = 0f,
            message = readableMessage,
            error = null,
            retryable = true,
            requiresUserAction = false
        )
    }

    data class Failed(
        val errorMessage: String,
        override val retryable: Boolean,
        override val requiresUserAction: Boolean
    ) : RuntimeState {
        override val phase: Phase = Phase.FAILED
        override val progress: Float = 0f
        override val readableMessage: String = "运行时失败: $errorMessage"
        override val error: String? = errorMessage

        override fun toStage(): RuntimeStage = RuntimeStage(
            stage = "Failed",
            progress = 0f,
            message = readableMessage,
            error = errorMessage,
            retryable = retryable,
            requiresUserAction = requiresUserAction
        )
    }

    data class Updating(
        val progressValue: Float,
        val message: String
    ) : RuntimeState {
        override val phase: Phase = Phase.UPDATING
        override val progress: Float = progressValue
        override val readableMessage: String = message
        override val error: String? = null
        override val retryable: Boolean = true
        override val requiresUserAction: Boolean = false

        override fun toStage(): RuntimeStage = RuntimeStage(
            stage = "Updating",
            progress = progressValue,
            message = message,
            error = null,
            retryable = true,
            requiresUserAction = false
        )
    }

    val isTerminal: Boolean
        get() = this is Stopped || this is Failed

    val isOperating: Boolean
        get() = this is Running || this is Degraded

    val isTransitioning: Boolean
        get() = this is Installing || this is Starting || this is Stopping || this is Updating
}

data class RuntimeServices(
    val surrealDb: ServiceState,
    val qdrant: ServiceState,
    val backend: ServiceState
) {
    companion object {
        val ALL_STOPPED: RuntimeServices = RuntimeServices(
            surrealDb = ServiceState.Stopped,
            qdrant = ServiceState.Stopped,
            backend = ServiceState.Stopped
        )
    }
}

sealed interface ServiceState {
    data class Healthy(val port: Int) : ServiceState
    data class Unhealthy(val reason: String) : ServiceState
    data object Stopped : ServiceState
    data object Starting : ServiceState
}

data class RuntimeStage(
    val stage: String,
    val progress: Float,
    val message: String,
    val error: String?,
    val retryable: Boolean,
    val requiresUserAction: Boolean
)
