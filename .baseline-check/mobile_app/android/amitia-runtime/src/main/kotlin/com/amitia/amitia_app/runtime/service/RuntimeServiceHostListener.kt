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
    MOUNT_CONTRACT_INVALID,
    EXIT_WATCHER_FAILED,
    STOP_RESULT_FAILED,
}

sealed interface ServiceTeardownResult {
    data class FullyStopped(val startId: Int) : ServiceTeardownResult
    data object SupersededByNewStart : ServiceTeardownResult
    data object Failed : ServiceTeardownResult
}

sealed interface RuntimeServiceHostEvent {
    data object ForegroundStarted : RuntimeServiceHostEvent
    data class SessionReady(
        val generation: Long,
        val sessionId: String
    ) : RuntimeServiceHostEvent
    data class ExpectedStopped(
        val generation: Long,
        val result: ServiceTeardownResult,
    ) : RuntimeServiceHostEvent
    data class UnexpectedTermination(
        val generation: Long,
        val cause: RuntimeServiceTerminationCause,
        val message: String? = null,
    ) : RuntimeServiceHostEvent
    data class StartupFailed(
        val generation: Long,
        val cause: RuntimeServiceTerminationCause,
        val message: String,
        val sessionId: String?,
        val launchStartId: Int,
        val phase: String,
    ) : RuntimeServiceHostEvent
    data class SnapshotUpdated(
        val snapshot: RuntimeServiceLifecycleSnapshot
    ) : RuntimeServiceHostEvent
}

fun interface RuntimeServiceHostListener {
    fun onServiceHostEvent(event: RuntimeServiceHostEvent)
}
