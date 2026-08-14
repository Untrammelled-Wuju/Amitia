package com.amitia.amitia_app.runtime.proot

interface ProotSession {
    val sessionId: String
    fun isAlive(): Boolean
    fun awaitExit(timeoutMillis: Long): Int?
    fun stop(graceMillis: Long): ProotStopResult
    fun close()
    fun requestStop()
    val exit: ProotExit?
}
