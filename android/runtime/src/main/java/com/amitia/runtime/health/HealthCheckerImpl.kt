package com.amitia.runtime.health

import com.amitia.runtime.api.ServiceState
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext
import okhttp3.Request
import java.net.InetSocketAddress
import java.net.Socket
import java.util.concurrent.TimeoutException
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class HealthCheckerImpl @Inject constructor(
    private val httpClientProvider: OkHttpClientProvider
) : HealthChecker {

    override suspend fun checkPort(host: String, port: Int, timeoutMs: Long): Boolean =
        withContext(Dispatchers.IO) {
            try {
                Socket().use { socket ->
                    socket.connect(InetSocketAddress(host, port), timeoutMs.toInt().coerceAtLeast(1))
                    socket.isConnected && !socket.isClosed
                }
            } catch (_: Exception) {
                false
            }
        }

    override suspend fun checkHttp(
        url: String,
        timeoutMs: Long,
        expectedStatusRange: IntRange
    ): Boolean = withContext(Dispatchers.IO) {
        try {
            val request = Request.Builder()
                .url(url)
                .get()
                .build()
            val client = httpClientProvider.clientWithTimeout(timeoutMs)
            client.newCall(request).execute().use { response ->
                response.code in expectedStatusRange
            }
        } catch (_: Exception) {
            false
        }
    }

    override suspend fun checkProcess(pid: Int): Boolean = withContext(Dispatchers.IO) {
        if (pid <= 0) return@withContext false
        try {
            val probe = Runtime.getRuntime().exec(arrayOf("sh", "-c", "kill -0 $pid"))
            probe.waitFor() == 0
        } catch (_: Throwable) {
            false
        }
    }

    override suspend fun waitForHealthy(
        name: String,
        check: suspend () -> Boolean,
        intervalMs: Long,
        timeoutMs: Long
    ): Result<Unit> = withContext(Dispatchers.IO) {
        val deadline = System.currentTimeMillis() + timeoutMs
        var attempt = 0
        var lastError: String? = null
        while (System.currentTimeMillis() < deadline) {
            attempt++
            try {
                if (check()) {
                    return@withContext Result.success(Unit)
                }
            } catch (e: Exception) {
                lastError = e.message
            }
            val remaining = deadline - System.currentTimeMillis()
            if (remaining <= 0) break
            delay(intervalMs.coerceAtMost(remaining).coerceAtLeast(50L))
        }
        Result.failure(
            TimeoutException(
                "健康检查超时: $name attempts=$attempt timeout=${timeoutMs}ms lastError=$lastError"
            )
        )
    }

    override suspend fun waitForPort(
        host: String,
        port: Int,
        timeoutMs: Long
    ): Result<Unit> = waitForHealthy(
        name = "port:$host:$port",
        check = { checkPort(host, port) },
        timeoutMs = timeoutMs
    )

    override suspend fun waitForHttp(
        url: String,
        timeoutMs: Long
    ): Result<Unit> = waitForHealthy(
        name = "http:$url",
        check = { checkHttp(url) },
        timeoutMs = timeoutMs
    )

    override fun buildServiceState(
        name: String,
        healthy: Boolean,
        port: Int?,
        reason: String?
    ): ServiceState {
        return when {
            healthy -> ServiceState.Healthy(port ?: 0)
            reason != null -> ServiceState.Unhealthy(reason)
            else -> ServiceState.Stopped
        }
    }
}
