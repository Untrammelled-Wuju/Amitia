package com.amitia.amitia_app.runtime.service

enum class RuntimeServicePhase {
    CREATED,
    FOREGROUND,
    UNOBSERVABLE,
    DESTROYED,
}

enum class RuntimeProcessPhase {
    CREATED,
    STARTED,
    READY,
    EXITING,
    EXITED,
    UNKNOWN,
}

enum class RuntimeStartupPhase {
    NOT_STARTED,
    DETECTING,
    READY,
    FAILED,
}

enum class RuntimeTerminalState {
    EXPECTED_STOPPED,
    UNEXPECTED_TERMINATION,
    STARTUP_FAILURE_CLEANUP,
}

data class RuntimeServiceLifecycleSnapshot(
    val generation: Long,
    val sessionId: String?,
    val servicePhase: RuntimeServicePhase,
    val processPhase: RuntimeProcessPhase,
    val startupPhase: RuntimeStartupPhase,
    val terminalState: RuntimeTerminalState?,
    val latestStartId: Int,
    val stopRequested: Boolean,
    val terminationCause: RuntimeServiceTerminationCause? = null,
    val teardownResult: ServiceTeardownResult? = null,
    val startupFailureMessage: String? = null,
) {
    val isTerminal: Boolean get() = terminalState != null
    val isProcessObservedDead: Boolean get() = processPhase == RuntimeProcessPhase.EXITED
}
