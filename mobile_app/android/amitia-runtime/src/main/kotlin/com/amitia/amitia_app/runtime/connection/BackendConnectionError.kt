package com.amitia.amitia_app.runtime.connection

internal enum class BackendConnectionErrorCode {
    RUNTIME_NOT_READY,
    BACKEND_NOT_READY,
    ENDPOINT_UNAVAILABLE,
    ENDPOINT_INVALID,
    CREDENTIAL_UNAVAILABLE,
    CREDENTIAL_INVALID,
    GENERATION_INVALID,
    BRIDGE_UNAVAILABLE,
    BRIDGE_PAYLOAD_INVALID,
    UNSUPPORTED_CONNECTION_MODE,
    INTERNAL_ERROR,
}

internal class BackendConnectionError(
    val code: BackendConnectionErrorCode,
    override val message: String,
) : Exception(message)
