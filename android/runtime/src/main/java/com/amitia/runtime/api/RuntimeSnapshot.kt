package com.amitia.runtime.api

data class RuntimeSnapshot(
    val state: RuntimeState,
    val stageInfo: RuntimeStageInfo,
    val ports: Map<String, Int> = emptyMap(),
    val lastStartTime: Long? = null,
    val lastStopReason: String? = null,
    val crashCount: Int = 0,
    val capturedAtMs: Long = System.currentTimeMillis()
) {

    fun isOperational(): Boolean = state.isOperating

    fun isTerminal(): Boolean = state.isTerminal

    fun withState(newState: RuntimeState): RuntimeSnapshot = copy(
        state = newState,
        stageInfo = RuntimeStageInfo.fromStage(newState.toStage()),
        capturedAtMs = System.currentTimeMillis()
    )

    fun withStage(stageInfo: RuntimeStageInfo): RuntimeSnapshot = copy(
        stageInfo = stageInfo,
        capturedAtMs = System.currentTimeMillis()
    )

    fun withPorts(ports: Map<String, Int>): RuntimeSnapshot = copy(
        ports = ports,
        capturedAtMs = System.currentTimeMillis()
    )

    fun recordStart(timeMs: Long): RuntimeSnapshot = copy(
        lastStartTime = timeMs,
        lastStopReason = null,
        capturedAtMs = System.currentTimeMillis()
    )

    fun recordStop(reason: String): RuntimeSnapshot = copy(
        lastStopReason = reason,
        capturedAtMs = System.currentTimeMillis()
    )

    fun recordCrash(): RuntimeSnapshot = copy(
        crashCount = crashCount + 1,
        capturedAtMs = System.currentTimeMillis()
    )

    companion object {
        val EMPTY: RuntimeSnapshot = RuntimeSnapshot(
            state = RuntimeState.NotInstalled,
            stageInfo = RuntimeStageInfo.EMPTY,
            ports = emptyMap(),
            lastStartTime = null,
            lastStopReason = null,
            crashCount = 0,
            capturedAtMs = 0L
        )
    }
}
