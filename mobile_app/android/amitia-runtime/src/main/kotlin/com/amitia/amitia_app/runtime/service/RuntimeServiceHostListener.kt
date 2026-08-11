package com.amitia.amitia_app.runtime.service

internal enum class RuntimeServiceTerminationCause {
    FOREGROUND_FAILED,
    NOTIFICATION_FAILED,
    SERVICE_INTERNAL_ERROR,
    SESSION_EXITED
}

internal sealed interface RuntimeServiceHostEvent {
    data object ForegroundStarted : RuntimeServiceHostEvent
    data class SessionReady(
        val generation: Long,
        val sessionId: String
    ) : RuntimeServiceHostEvent
    data class SessionExited(
        val sessionId: String,
        val exitCode: Int?,
        val forced: Boolean
    ) : RuntimeServiceHostEvent
    data object ExpectedStopped : RuntimeServiceHostEvent
    data class UnexpectedTermination(
        val cause: RuntimeServiceTerminationCause,
    ) : RuntimeServiceHostEvent
}

internal fun interface RuntimeServiceHostListener {
    fun onServiceHostEvent(event: RuntimeServiceHostEvent)
}
