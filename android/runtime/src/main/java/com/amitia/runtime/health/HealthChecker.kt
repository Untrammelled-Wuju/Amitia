package com.amitia.runtime.health

import com.amitia.runtime.api.ServiceState

interface HealthChecker {

    suspend fun checkPort(host: String, port: Int, timeoutMs: Long = DEFAULT_PORT_TIMEOUT_MS): Boolean

    suspend fun checkHttp(
        url: String,
        timeoutMs: Long = DEFAULT_HTTP_TIMEOUT_MS,
        expectedStatusRange: IntRange = 200..299
    ): Boolean

    suspend fun checkProcess(pid: Int): Boolean

    suspend fun waitForHealthy(
        name: String,
        check: suspend () -> Boolean,
        intervalMs: Long = DEFAULT_INTERVAL_MS,
        timeoutMs: Long = DEFAULT_TOTAL_TIMEOUT_MS
    ): Result<Unit>

    suspend fun waitForPort(
        host: String,
        port: Int,
        timeoutMs: Long = DEFAULT_TOTAL_TIMEOUT_MS
    ): Result<Unit>

    suspend fun waitForHttp(
        url: String,
        timeoutMs: Long = DEFAULT_TOTAL_TIMEOUT_MS
    ): Result<Unit>

    fun buildServiceState(
        name: String,
        healthy: Boolean,
        port: Int? = null,
        reason: String? = null
    ): ServiceState

    companion object {
        const val DEFAULT_PORT_TIMEOUT_MS = 1000L
        const val DEFAULT_HTTP_TIMEOUT_MS = 2000L
        const val DEFAULT_INTERVAL_MS = 1000L
        const val DEFAULT_TOTAL_TIMEOUT_MS = 60_000L
    }
}
