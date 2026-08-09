package com.amitia.amitia_app.runtime.startup

import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
import java.io.BufferedReader
import java.io.InputStreamReader
import java.net.HttpURLConnection
import java.net.InetSocketAddress
import java.net.Socket
import java.net.URL

internal class HttpRuntimeHealthProbe(
    private val connectTimeoutMs: Int = 2000,
    private val readTimeoutMs: Int = 2000
) : RuntimeHealthProbe {

    override fun checkLiveness(endpoint: BackendEndpointPolicy): RuntimeHealthProbeResult {
        return probe(endpoint, "/livez")
    }

    override fun checkReadiness(endpoint: BackendEndpointPolicy): RuntimeHealthProbeResult {
        return probe(endpoint, "/readyz")
    }

    private fun probe(endpoint: BackendEndpointPolicy, path: String): RuntimeHealthProbeResult {
        if (endpoint.host != "127.0.0.1") {
            return RuntimeHealthProbeResult.Failure(RuntimeHealthProbeError.IOError("invalid host"))
        }

        val url = URL("${endpoint.httpScheme}://${endpoint.host}:${endpoint.port}$path")
        var connection: HttpURLConnection? = null
        try {
            connection = (url.openConnection() as HttpURLConnection).apply {
                requestMethod = "GET"
                connectTimeout = connectTimeoutMs
                readTimeout = readTimeoutMs
                setRequestProperty("Accept", "application/json")
            }
            connection.connect()
            val statusCode = connection.responseCode
            return RuntimeHealthProbeResult.Success(statusCode)
        } catch (e: java.net.ConnectException) {
            return RuntimeHealthProbeResult.Failure(RuntimeHealthProbeError.ConnectionRefused)
        } catch (e: java.net.SocketTimeoutException) {
            return RuntimeHealthProbeResult.Failure(RuntimeHealthProbeError.ConnectionTimeout)
        } catch (e: java.io.IOException) {
            return RuntimeHealthProbeResult.Failure(RuntimeHealthProbeError.IOError(e.message ?: "io error"))
        } finally {
            connection?.disconnect()
        }
    }
}
