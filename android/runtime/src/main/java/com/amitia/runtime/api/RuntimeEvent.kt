package com.amitia.runtime.api

sealed interface RuntimeEvent {

    val timestampMs: Long

    data class StateChanged(
        val from: RuntimeState,
        val to: RuntimeState,
        override val timestampMs: Long
    ) : RuntimeEvent

    data class ProgressUpdated(
        val stage: RuntimeStage,
        override val timestampMs: Long
    ) : RuntimeEvent

    data class ServiceHealthChanged(
        val serviceName: String,
        val state: ServiceState,
        override val timestampMs: Long
    ) : RuntimeEvent

    data class LogEmitted(
        val level: Level,
        val tag: String,
        val message: String,
        override val timestampMs: Long
    ) : RuntimeEvent {
        enum class Level { TRACE, DEBUG, INFO, WARN, ERROR }
    }

    data class ErrorOccurred(
        val error: String,
        val retryable: Boolean,
        val requiresUserAction: Boolean,
        val cause: Throwable? = null,
        override val timestampMs: Long
    ) : RuntimeEvent

    data class RootfsInstallProgress(
        val progress: Float,
        val message: String,
        val bytesProcessed: Long,
        val bytesTotal: Long,
        override val timestampMs: Long
    ) : RuntimeEvent

    data class ServiceStarting(
        val serviceName: String,
        val port: Int,
        override val timestampMs: Long
    ) : RuntimeEvent

    data class ServiceHealthy(
        val serviceName: String,
        val port: Int,
        override val timestampMs: Long
    ) : RuntimeEvent

    data class ServiceFailed(
        val serviceName: String,
        val error: String,
        val retryable: Boolean,
        override val timestampMs: Long
    ) : RuntimeEvent

    data class RuntimeStateChanged(
        val from: RuntimeState,
        val to: RuntimeState,
        val reason: String? = null,
        override val timestampMs: Long
    ) : RuntimeEvent
}
