package com.amitia.runtime.api

data class RuntimeStageInfo(
    val stage: String,
    val progress: Float,
    val readableMessage: String,
    val errorCause: String? = null,
    val isRetryable: Boolean = true,
    val needsUserAction: Boolean = false
) {

    fun coercedProgress(): Float = progress.coerceIn(0f, 1f)

    fun toRuntimeStage(): RuntimeStage = RuntimeStage(
        stage = stage,
        progress = coercedProgress(),
        message = readableMessage,
        error = errorCause,
        retryable = isRetryable,
        requiresUserAction = needsUserAction
    )

    companion object {
        val EMPTY: RuntimeStageInfo = RuntimeStageInfo(
            stage = "Idle",
            progress = 0f,
            readableMessage = "",
            errorCause = null,
            isRetryable = true,
            needsUserAction = false
        )

        fun fromStage(stage: RuntimeStage): RuntimeStageInfo = RuntimeStageInfo(
            stage = stage.stage,
            progress = stage.progress,
            readableMessage = stage.message,
            errorCause = stage.error,
            isRetryable = stage.retryable,
            needsUserAction = stage.requiresUserAction
        )
    }
}
