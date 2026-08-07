package com.amitia.amitia_app.runtime.bridge

import com.amitia.amitia_app.runtime.connection.BackendConnectionAvailability
import com.amitia.amitia_app.runtime.connection.BackendConnectionProvider
import com.amitia.amitia_app.runtime.connection.internal.BackendConnectionMapper

internal class RuntimeBridge(
    private val connectionProvider: BackendConnectionProvider,
) {
    fun getBackendConnection(): Map<String, Any?> {
        val availability = connectionProvider.current()
        return when (availability) {
            is BackendConnectionAvailability.Available -> BackendConnectionMapper.toPayload(
                available = true,
                descriptor = availability.descriptor,
                error = null,
            )
            is BackendConnectionAvailability.Unavailable -> BackendConnectionMapper.toPayload(
                available = false,
                descriptor = null,
                error = null,
            )
            is BackendConnectionAvailability.Resolving -> BackendConnectionMapper.toPayload(
                available = false,
                descriptor = null,
                error = null,
            )
        }
    }
}
