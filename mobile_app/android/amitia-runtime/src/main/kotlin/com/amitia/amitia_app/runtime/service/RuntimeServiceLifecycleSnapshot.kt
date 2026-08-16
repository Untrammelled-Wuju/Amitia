package com.amitia.amitia_app.runtime.service

internal enum class RuntimeServicePhase {
    CREATED,
    FOREGROUND,
    UNOBSERVABLE,
    DESTROYED,
}

internal enum class RuntimeProcessPhase {
    CREATED,
    STARTED,
    READY,
    EXITING,
    EXITED,
    UNKNOWN,
}

internal enum class RuntimeStartupPhase {
    NOT_STARTED,
    DETECTING,
    READY,
    FAILED,
}

internal enum class RuntimeTerminalState {
    EXPECTED_STOPPED,
    UNEXPECTED_TERMINATION,
    STARTUP_FAILURE_CLEANUP,
}

internal data class RuntimeServiceLifecycleSnapshot(
    val generation: Long,
    val sessionId: String?,
    val servicePhase: RuntimeServicePhase,
    val processPhase: RuntimeProcessPhase,
    val startupPhase: RuntimeStartupPhase,
    val terminalState: RuntimeTerminalState?,
    val latestStartId: Int,
    val stopRequested: Boolean,
)
