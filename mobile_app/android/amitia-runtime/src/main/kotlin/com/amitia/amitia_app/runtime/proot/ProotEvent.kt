package com.amitia.amitia_app.runtime.proot

sealed class ProotEvent {
    abstract val sessionId: String
    data class Started(override val sessionId: String, val timestamp: Long) : ProotEvent()
    data class Stdout(override val sessionId: String, val data: String, val sequence: Long) : ProotEvent()
    data class Stderr(override val sessionId: String, val data: String, val sequence: Long) : ProotEvent()
    data class Exited(override val sessionId: String, val exitCode: Int, val forced: Boolean) : ProotEvent()
}