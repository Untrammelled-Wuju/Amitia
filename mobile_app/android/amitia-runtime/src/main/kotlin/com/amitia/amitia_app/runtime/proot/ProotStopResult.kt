package com.amitia.amitia_app.runtime.proot

sealed class ProotStopResult {
    abstract val sessionId: String
    data class Graceful(override val sessionId: String, val exitCode: Int) : ProotStopResult()
    data class Forced(override val sessionId: String, val exitCode: Int?) : ProotStopResult()
    data class AlreadyStopped(override val sessionId: String, val exitCode: Int?) : ProotStopResult()
    data class Failed(override val sessionId: String, val errorCode: ProotErrorCode, val message: String) : ProotStopResult()
}