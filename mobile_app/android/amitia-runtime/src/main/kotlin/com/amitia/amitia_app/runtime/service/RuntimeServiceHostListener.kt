package com.amitia.amitia_app.runtime.service

enum class RuntimeServiceTerminationCause {
    FOREGROUND_FAILED,
    NOTIFICATION_FAILED,
    SERVICE_INTERNAL_ERROR,
    SESSION_EXITED,
    PROOT_COMPONENT_MISSING,
    ROOTFS_MISSING,
    ASSEMBLER_MISSING,
    NO_ACTIVE_RUNTIME,
    ACTIVE_PROGRAM_ROOT_MISSING,
    ACTIVE_PROGRAM_ROOT_INVALID,
    ENVIRONMENT_BUILD_FAILED,
    MOUNT_CONTRACT_INVALID
}

sealed interface RuntimeServiceHostEvent {
    data object ForegroundStarted : RuntimeServiceHostEvent
    data class SessionReady(
        val generation: Long,
        val sessionId: String
    ) : RuntimeServiceHostEvent
    data class SessionExited(
        val generation: Long,
        val sessionId: String,
        val exitCode: Int?,
        val forced: Boolean
    ) : RuntimeServiceHostEvent
    data class ExpectedStopped(
        val generation: Long
    ) : RuntimeServiceHostEvent
    data class UnexpectedTermination(
        val generation: Long,
        val cause: RuntimeServiceTerminationCause,
    ) : RuntimeServiceHostEvent
    data class LaunchFailed(
        val generation: Long,
        val cause: RuntimeServiceTerminationCause,
        val message: String,
    ) : RuntimeServiceHostEvent
}

fun interface RuntimeServiceHostListener {
    fun onServiceHostEvent(event: RuntimeServiceHostEvent)
}
