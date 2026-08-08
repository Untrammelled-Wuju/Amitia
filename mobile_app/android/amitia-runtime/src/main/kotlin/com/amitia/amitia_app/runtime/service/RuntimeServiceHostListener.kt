package com.amitia.amitia_app.runtime.service

internal enum class RuntimeServiceTerminationCause {
    FOREGROUND_FAILED,
    NOTIFICATION_FAILED,
    SERVICE_INTERNAL_ERROR
}

internal sealed interface RuntimeServiceHostEvent {
    data object ForegroundStarted : RuntimeServiceHostEvent
    data object ExpectedStopped : RuntimeServiceHostEvent
    data class UnexpectedTermination(
        val cause: RuntimeServiceTerminationCause,
    ) : RuntimeServiceHostEvent
}

internal fun interface RuntimeServiceHostListener {
    fun onServiceHostEvent(event: RuntimeServiceHostEvent)
}
