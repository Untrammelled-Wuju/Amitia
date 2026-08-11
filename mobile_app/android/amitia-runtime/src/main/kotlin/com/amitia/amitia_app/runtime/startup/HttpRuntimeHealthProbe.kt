package com.amitia.amitia_app.runtime.startup

import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
import org.json.JSONObject
import java.io.ByteArrayOutputStream
import java.net.HttpURLConnection
import java.net.InetSocketAddress
import java.net.Proxy
import java.net.URL

internal class HttpRuntimeHealthProbe(
    private val perProbeConnectTimeoutMs: Int = 2000,
    private val perProbeReadTimeoutMs: Int = 2000,
    private val maxResponseSizeBytes: Int = 8192
) : RuntimeHealthProbe {

    override fun checkReadiness(endpoint: BackendEndpointPolicy): RuntimeHealthProbeResult {
        if (!isLoopbackHost(endpoint.host)) {
            return RuntimeHealthProbeResult.Failure(
                RuntimeHealthProbeError.IOError("readiness probe only targets loopback, host=${sanitizeHostForLog(endpoint.host)}")
            )
        }
        if (endpoint.port <= 0 || endpoint.port > 65535) {
            return RuntimeHealthProbeResult.Failure(
                RuntimeHealthProbeError.IOError("invalid port for readiness probe: ${endpoint.port}")
            )
        }

        val url = URL("${endpoint.httpScheme}://${endpoint.host}:${endpoint.port}/readyz")
        var connection: HttpURLConnection? = null
        var inputStream: java.io.InputStream? = null
        try {
            connection = (url.openConnection(Proxy.NO_PROXY) as HttpURLConnection).apply {
                requestMethod = "GET"
                connectTimeout = perProbeConnectTimeoutMs
                readTimeout = perProbeReadTimeoutMs
                instanceFollowRedirects = false
                useCaches = false
                setRequestProperty("Accept", "application/json")
                setRequestProperty("Connection", "close")
            }
            connection.connect()
            val statusCode = connection.responseCode

            inputStream = try {
                connection.inputStream
            } catch (_: java.io.IOException) {
                connection.errorStream
            }

            val body = drainLimited(inputStream)
            if (statusCode == 301 || statusCode == 302 || statusCode == 303 || statusCode == 307 || statusCode == 308) {
                return RuntimeHealthProbeResult.Failure(
                    RuntimeHealthProbeError.IOError("readiness probe rejected redirect: statusCode=$statusCode")
                )
            }

            return RuntimeHealthProbeResult.Success(statusCode, body)
        } catch (e: java.net.ConnectException) {
            return RuntimeHealthProbeResult.Failure(RuntimeHealthProbeError.ConnectionRefused)
        } catch (e: java.net.SocketTimeoutException) {
            return RuntimeHealthProbeResult.Failure(RuntimeHealthProbeError.ConnectionTimeout)
        } catch (e: java.io.IOException) {
            return RuntimeHealthProbeResult.Failure(RuntimeHealthProbeError.IOError(e.message ?: "io error"))
        } catch (e: RuntimeException) {
            return RuntimeHealthProbeResult.Failure(RuntimeHealthProbeError.IOError(e.message ?: "runtime error"))
        } finally {
            try { inputStream?.close() } catch (_: Throwable) {}
            try { connection?.disconnect() } catch (_: Throwable) {}
        }
    }

    private fun drainLimited(inputStream: java.io.InputStream?): String {
        if (inputStream == null) return ""
        return try {
            val buffer = ByteArray(1024)
            val total = ByteArrayOutputStream(maxResponseSizeBytes)
            var remaining = maxResponseSizeBytes
            var read = inputStream.read(buffer, 0, minOf(buffer.size, remaining))
            while (read > 0 && remaining > 0) {
                total.write(buffer, 0, read)
                remaining -= read
                if (remaining <= 0) break
                read = inputStream.read(buffer, 0, minOf(buffer.size, remaining))
            }
            total.toString("UTF-8")
        } catch (_: Throwable) {
            ""
        }
    }

    companion object {
        fun readBackendStatus(body: String?): String? {
            if (body.isNullOrBlank()) return null
            return try {
                val root = JSONObject(body)
                val data = root.optJSONObject("data") ?: return null
                val status = data.optString("status", "").takeIf { it.isNotBlank() } ?: return null
                status.lowercase()
            } catch (_: Throwable) {
                null
            }
        }

        fun isLoopbackHost(host: String): Boolean {
            val normalized = host.trim().lowercase()
            if (normalized == "127.0.0.1" || normalized == "localhost" || normalized == "[::1]" || normalized == "::1") return true
            return try {
                val addr = InetSocketAddress(host, 9).address
                addr.isLoopbackAddress
            } catch (_: Throwable) {
                false
            }
        }

        fun sanitizeHostForLog(host: String): String {
            val trimmed = host.trim()
            return if (trimmed.length <= 32) trimmed else trimmed.take(16) + "..." + trimmed.takeLast(8)
        }
    }
}
