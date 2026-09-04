package com.amitia.amitia_app.runtime.startup

import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
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
                // Current backend contract nests readiness under data.status.
                // Accept the legacy top-level status as a compatibility fallback
                // so mixed mobile/backend rollouts cannot strand the runtime.
                val dataValue = extractJsonObjectField(body, "data")
                val nestedStatus = dataValue?.let { extractJsonStringField(it, "status") }
                val status = nestedStatus ?: extractJsonStringField(body, "status")
                status?.takeIf { it.isNotBlank() }?.lowercase()
            } catch (_: Throwable) {
                null
            }
        }

        private fun extractJsonObjectField(json: String, fieldName: String): String? {
            val keyPattern = "\"" + fieldName + "\""
            val keyIndex = json.indexOf(keyPattern)
            if (keyIndex < 0) return null
            val colonIndex = json.indexOf(':', keyIndex + keyPattern.length)
            if (colonIndex < 0) return null
            var i = colonIndex + 1
            while (i < json.length && json[i].isWhitespace()) i++
            if (i >= json.length || json[i] != '{') return null
            var depth = 0
            val start = i
            while (i < json.length) {
                when (json[i]) {
                    '{' -> depth++
                    '}' -> {
                        depth--
                        if (depth == 0) return json.substring(start, i + 1)
                    }
                }
                i++
            }
            return null
        }

        private fun extractJsonStringField(json: String, fieldName: String): String? {
            val keyPattern = "\"" + fieldName + "\""
            val keyIndex = json.indexOf(keyPattern)
            if (keyIndex < 0) return null
            val colonIndex = json.indexOf(':', keyIndex + keyPattern.length)
            if (colonIndex < 0) return null
            var i = colonIndex + 1
            while (i < json.length && json[i].isWhitespace()) i++
            if (i >= json.length || json[i] != '"') return null
            i++
            val sb = StringBuilder()
            while (i < json.length) {
                val c = json[i]
                if (c == '"') return sb.toString()
                if (c == '\\' && i + 1 < json.length) {
                    val next = json[i + 1]
                    when (next) {
                        '"' -> sb.append('"')
                        '\\' -> sb.append('\\')
                        '/' -> sb.append('/')
                        'n' -> sb.append('\n')
                        'r' -> sb.append('\r')
                        't' -> sb.append('\t')
                        else -> { sb.append('\\'); sb.append(next) }
                    }
                    i += 2
                } else {
                    sb.append(c)
                    i++
                }
            }
            return null
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
