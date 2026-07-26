package com.amitia.core.network.connection

import com.amitia.core.network.api.HealthApi
import com.amitia.core.network.client.AmitiaApiClient
import com.amitia.core.network.client.AmitiaApiException
import com.amitia.core.network.endpoint.RuntimeEndpointProvider
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.withContext

@Singleton
class ConnectionManager @Inject constructor(
    private val apiClient: AmitiaApiClient,
    private val endpointProvider: RuntimeEndpointProvider
) {

    private val _connectionState = MutableStateFlow(ConnectionState.DISCONNECTED)

    val connectionState: StateFlow<ConnectionState> = _connectionState.asStateFlow()

    fun observeConnectionState(): Flow<ConnectionState> = _connectionState.asStateFlow()

    suspend fun testConnection(): Result<ConnectionTestResult> = withContext(Dispatchers.IO) {
        _connectionState.value = ConnectionState.CONNECTING
        val startedAt = System.currentTimeMillis()
        runCatching {
            val healthApi = apiClient.service(HealthApi::class.java)
            val response = healthApi.health()
            val latency = System.currentTimeMillis() - startedAt
            val mode = endpointProvider.getCurrentMode()
            _connectionState.value = ConnectionState.CONNECTED
            ConnectionTestResult(
                success = true,
                latencyMs = latency,
                serverVersion = response.version,
                mode = mode
            )
        }.onFailure { _ ->
            _connectionState.value = ConnectionState.ERROR
        }
    }

    fun markConnected() {
        _connectionState.value = ConnectionState.CONNECTED
    }

    fun markDisconnected() {
        _connectionState.value = ConnectionState.DISCONNECTED
    }

    fun markError() {
        _connectionState.value = ConnectionState.ERROR
    }

    fun mapFailure(throwable: Throwable): AmitiaApiException {
        return if (throwable is AmitiaApiException) throwable
        else AmitiaApiException.Unknown(throwable)
    }

    data class ConnectionTestResult(
        val success: Boolean,
        val latencyMs: Long,
        val serverVersion: String?,
        val mode: RuntimeEndpointProvider.RuntimeMode
    )
}
