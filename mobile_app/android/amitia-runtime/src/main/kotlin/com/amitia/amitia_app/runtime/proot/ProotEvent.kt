package com.amitia.amitia_app.runtime.proot

data class ProotExit(
    val generation: Long,
    val sessionId: String,
    val exitCode: Int,
    val stopRequested: Boolean,
)

sealed class ProotEvent {
    abstract val sessionId: String
    data class Started(override val sessionId: String, val timestamp: Long) : ProotEvent()
    data class Stdout(override val sessionId: String, val data: String, val sequence: Long) : ProotEvent()
    data class Stderr(override val sessionId: String, val data: String, val sequence: Long) : ProotEvent()
    data class Exited(val exit: ProotExit) : ProotEvent() {
        override val sessionId: String get() = exit.sessionId
    }
}
